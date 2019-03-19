//
// Copyright (c) 2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package utils

import (
	"fmt"
	"log"

	conf "github.com/redhat-developer/kubernetes-image-puller/internal/configuration"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

var propagationPolicy = metav1.DeletePropagationForeground
var terminationGracePeriodSeconds = int64(1)

// Set up watch on daemonset
func watchDaemonset(clientset *kubernetes.Clientset) watch.Interface {
	watch, err := clientset.AppsV1().DaemonSets(conf.Config.Namespace).Watch(metav1.ListOptions{
		FieldSelector:        fmt.Sprintf("metadata.name=%s", conf.Config.DaemonsetName),
		IncludeUninitialized: true,
	})
	if err != nil {
		log.Fatalf("Failed to set up watch on daemonsets: %s", err.Error())
	}
	return watch
}

// Create the daemonset, using to-be-cached images as init containers. Blocks
// until daemonset is ready.
func createDaemonset(clientset *kubernetes.Clientset) error {
	log.Printf("Creating daemonset")
	toCreate := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: conf.Config.DaemonsetName,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test": "daemonset-test",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"test": "daemonset-test",
					},
					Name: "test-po",
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers:                    getContainers(),
				},
			},
		},
	}
	dsWatch := watchDaemonset(clientset)
	defer dsWatch.Stop()
	watchChan := dsWatch.ResultChan()

	_, err := clientset.AppsV1().DaemonSets(conf.Config.Namespace).Create(toCreate)
	if err != nil {
		log.Fatalf("Failed to create daemonset: %s", err.Error())
	} else {
		log.Printf("Created daemonset")
	}
	waitDaemonsetReady(watchChan)
	return err
}

// Wait for daemonset to be ready (MODIFIED event with all nodes scheduled)
func waitDaemonsetReady(c <-chan watch.Event) {
	log.Printf("Waiting for daemonset to be ready")
	for ev := range c {
		log.Printf("(DEBUG) Create watch event received: %s", ev.Type)
		if ev.Type == watch.Modified {
			daemonset := ev.Object.(*appsv1.DaemonSet)
			// TODO: Not sure if this is the correct logic
			if daemonset.Status.NumberReady == daemonset.Status.DesiredNumberScheduled {
				log.Printf("All nodes scheduled in daemonset")
				return
			}
		} else if ev.Type == watch.Deleted {
			log.Fatalf("Error occurred while waiting for daemonset to be ready -- event %s detected", watch.Deleted)
		}
	}
}

// Delete daemonset with metadata.name daemonsetName. Blocks until daemonset
// is deleted.
func deleteDaemonset(clientset *kubernetes.Clientset) {
	log.Println("Deleting daemonset")

	dsWatch := watchDaemonset(clientset)
	defer dsWatch.Stop()
	watchChan := dsWatch.ResultChan()

	err := clientset.AppsV1().DaemonSets(conf.Config.Namespace).Delete(conf.Config.DaemonsetName, &metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil {
		log.Fatalf("Failed to delete daemonset %s", err.Error())
	} else {
		log.Printf("Deleted daemonset %s", conf.Config.DaemonsetName)
	}
	waitDaemonsetDeleted(watchChan)
}

// Use watch channel to wait for DELETED event on daemonset, then return
func waitDaemonsetDeleted(c <-chan watch.Event) {
	for ev := range c {
		log.Printf("(DEBUG) Delete watch event received: %s", ev.Type)
		if ev.Type == watch.Deleted {
			return
		}
	}
}

// Get array of all images in containers to be cached.
func getContainers() []corev1.Container {
	images := conf.Config.Images
	containers := make([]corev1.Container, len(images))
	idx := 0
	for name, image := range images {
		containers[idx] = corev1.Container{
			Name:    name,
			Image:   image,
			Command: []string{"/bin/sh", "-c", "sleep", "infinity"},
		}
		idx++
	}
	return containers
}
