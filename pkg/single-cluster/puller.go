package singlecluster

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	conf "github.com/redhat-developer/kubernetes-image-puller/internal/configuration"
	"github.com/redhat-developer/kubernetes-image-puller/internal/utils"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// CacheImages starts and maintains a daemonset to ensure images are
// cached.
func CacheImages() {
	// Set up kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf(err.Error())
	}
	var wg sync.WaitGroup
	wg.Add(1)
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGTERM)

	go cacheImagesLocally(config, shutdownChan, &wg)
	wg.Wait()
	log.Printf("Shutting down cleanly")
}

func cacheImagesLocally(config *rest.Config,
	shutdownChan chan os.Signal,
	wg *sync.WaitGroup) {

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf(err.Error())
	}

	// Clean up existing deployment if necessary
	utils.DeleteDaemonsetIfExists(clientset)
	// Create daemonset to cache images
	utils.CacheImages(clientset)

	for {
		select {
		case <-shutdownChan:
			log.Printf("Received SIGTERM, deleting daemonset")
			utils.DeleteDaemonsetIfExists(clientset)
			wg.Done()
		case <-time.After(time.Duration(conf.Config.CachingInterval) * time.Hour):
			utils.EnsureDaemonsetExists(clientset)
		}
	}
}
