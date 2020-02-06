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
	"time"

	"github.com/redhat-developer/kubernetes-image-puller/cfg"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

var propagationPolicy = metav1.DeletePropagationForeground
var terminationGracePeriodSeconds = int64(1)

// Set up watch on daemonset
func watchDaemonset(clientset *kubernetes.Clientset) watch.Interface {
	watch, err := clientset.AppsV1().DaemonSets(cfg.Namespace).Watch(metav1.ListOptions{
		FieldSelector:        fmt.Sprintf("metadata.name=%s", cfg.DaemonsetName),
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
			Name: cfg.DaemonsetName,
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
					NodeSelector:                  cfg.NodeSelector,
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers:                    getContainers(),
				},
			},
		},
	}
	dsWatch := watchDaemonset(clientset)
	defer dsWatch.Stop()
	watchChan := dsWatch.ResultChan()

	_, err := clientset.AppsV1().DaemonSets(cfg.Namespace).Create(toCreate)
	if err != nil {
		log.Fatalf("Failed to create daemonset: %s", err.Error())
	} else {
		log.Printf("Created daemonset")
	}
	watchErr := waitDaemonsetReady(watchChan)
	if watchErr != nil {
		log.Printf("Unable to watch daemonset for readiness, falling back to manually checking.")
		checkDaemonsetReadiness(clientset)
	}
	return err
}

// Wait for daemonset to be ready (MODIFIED event with all nodes scheduled)
func waitDaemonsetReady(c <-chan watch.Event) error {
	log.Printf("Waiting for daemonset to be ready")
	for {
		select {
		case ev, ok := <-c:
			if !ok {
				log.Printf("WARN: Watch closed before deamonset ready")
				return fmt.Errorf("Watch closed before deamonset ready")
			}
			// log.Printf("(DEBUG) Create watch event received: %s", ev.Type)
			if ev.Type == watch.Modified {
				daemonset := ev.Object.(*appsv1.DaemonSet)
				// TODO: Not sure if this is the correct logic
				if daemonset.Status.NumberReady == daemonset.Status.DesiredNumberScheduled {
					log.Printf("%d/%d nodes ready in daemonset", daemonset.Status.NumberReady, daemonset.Status.DesiredNumberScheduled)
					return nil
				}
				log.Printf("%d/%d nodes ready", daemonset.Status.NumberReady, daemonset.Status.DesiredNumberScheduled)
			} else if ev.Type == watch.Deleted || ev.Type == watch.Error {
				log.Fatalf("Error occurred while waiting for daemonset to be ready -- event %s detected", watch.Deleted)
			}
		}
	}
}

func checkDaemonsetReadiness(clientset *kubernetes.Clientset) {
	// Loop 30 times, sleeping for 3 seconds each time -- 90 seconds total wait.
	for i := 0; i < 30; i++ {
		ds, err := clientset.AppsV1().DaemonSets(cfg.Namespace).Get(cfg.DaemonsetName, metav1.GetOptions{
			// IncludeUninitialized: true,
		})
		if err != nil {
			log.Printf("WARN: could not get daemonset: %s", err)
			return
		}
		if ds.Status.DesiredNumberScheduled == 0 {
			// We've received a daemonset during initialization
			continue
		}
		log.Printf("%d/%d nodes ready in daemonset", ds.Status.NumberReady, ds.Status.DesiredNumberScheduled)
		if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled {
			log.Printf("All nodes running")
			return
		}
		time.Sleep(3 * time.Second)
	}
	log.Printf("WARN: maximum duration for readiness checking exceeded.")
}

// Delete daemonset with metadata.name daemonsetName. Blocks until daemonset
// is deleted.
func deleteDaemonset(clientset *kubernetes.Clientset) {
	log.Println("Deleting daemonset")

	dsWatch := watchDaemonset(clientset)
	defer dsWatch.Stop()
	watchChan := dsWatch.ResultChan()

	err := clientset.AppsV1().DaemonSets(cfg.Namespace).Delete(cfg.DaemonsetName, &metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil {
		log.Fatalf("Failed to delete daemonset %s", err.Error())
	} else {
		log.Printf("Marked daemonset %s for removal", cfg.DaemonsetName)
	}
	watchErr := waitDaemonsetDeleted(watchChan)
	if watchErr != nil {
		// The watch was closed prematurely so we don't know if the daemonset has actually been removed
		// We have to ensure it's gone before trying to create a new one.
		log.Printf("WARN: error while waiting for daemonset deletion: %s", watchErr)
		log.Printf("Manually ensuring daemonset is deleted.")
		err = checkDaemonsetDeleted(clientset)
		if err != nil {
			log.Fatalf("FATAL: could not verify that daemonset is deleted: %s", err)
		}
	}
	log.Printf("Deleted daemonset %s", cfg.DaemonsetName)
}

// Use watch channel to wait for DELETED event on daemonset, then return
func waitDaemonsetDeleted(c <-chan watch.Event) error {
	for {
		select {
		case ev, ok := <-c:
			if !ok {
				log.Printf("WARN: Watch closed before deamonset deleted")
				return fmt.Errorf("watch closed before daemonset deleted")
			}
			log.Printf("(DEBUG) Delete watch event received: %s", ev.Type)
			if ev.Type == watch.Deleted {
				return nil
			}
		}
	}
}

func checkDaemonsetDeleted(clientset *kubernetes.Clientset) error {
	// Loop 60 times, sleeping for 3 seconds each time -- 180 seconds maximum wait.
	for i := 0; i < 60; i++ {
		_, err := clientset.AppsV1().DaemonSets(cfg.Namespace).Get(cfg.DaemonsetName, metav1.GetOptions{
			IncludeUninitialized: true,
		})
		if errors.IsNotFound(err) {
			// daemonset has been deleted
			return nil
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("timeout expired while checking that daemonset is deleted")
}

// Get array of all images in containers to be cached.
func getContainers() []corev1.Container {
	images := cfg.Images
	containers := make([]corev1.Container, len(images))
	idx := 0

	cachedImageResources := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			"memory": resource.MustParse(cfg.CachingMemLimit),
		},
		Requests: corev1.ResourceList{
			"memory": resource.MustParse(cfg.CachingMemRequest),
		},
	}

	for name, image := range images {
		containers[idx] = corev1.Container{
			Name:            name,
			Image:           image,
			Command:         []string{"sleep"},
			Args:            []string{"30d"},
			Resources:       cachedImageResources,
			ImagePullPolicy: corev1.PullAlways,
		}
		idx++
	}
	return containers
}
