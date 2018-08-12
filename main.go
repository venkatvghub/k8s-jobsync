package main

import (
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	// Set logging output to standard console out
	log.SetOutput(os.Stdout)

	sigs := make(chan os.Signal, 1) // Create channel to receive OS signals
	stop := make(chan struct{})     // Create channel to receive stop signal

	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, syscall.SIGINT) // Register the sigs channel to receieve SIGTERM

	wg := &sync.WaitGroup{} // Goroutines can add themselves to this to be waited on so that they finish

	runOutsideCluster := flag.Bool("run-outside-cluster", false, "Set this flag when running outside of the cluster.")
	namespace := flag.String("namespace", "", "Watch only this namespaces")
	dryRun := flag.Bool("dry-run", false, "Print only, do not delete anything.")
	flag.Parse()

	// Create clientset for interacting with the kubernetes cluster
	clientset, err := newClientSet(*runOutsideCluster)

	if err != nil {
		panic(err.Error())
	}

	options := map[string]string{
		"namespace": *namespace,
		"dryRun":    strconv.FormatBool(*dryRun),
	}
	if *dryRun {
		log.Println("Performing dry run...")
	}
	log.Printf("Configured namespace: '%s'", options["namespace"])
	log.Printf("Starting controller...")

	go NewDeploymentController(clientset, options).Run(stop, wg)

	<-sigs // Wait for signals (this hangs until a signal arrives)
	log.Printf("Shutting down...")

	close(stop) // Tell goroutines to stop themselves
	wg.Wait()   // Wait for all to be stopped
}

func newClientSet(runOutsideCluster bool) (*kubernetes.Clientset, error) {
	k8scfg, err := rest.InClusterConfig()
	if err != nil {
		// No in cluster? letr's try locally
		// construct the path to resolve to `~/.kube/config`
		//kubeConfigPath := os.Getenv("HOME") + "/.kube/config"
		kubehome := filepath.Join(homedir.HomeDir(), ".kube", "config")
		k8scfg, err = clientcmd.BuildConfigFromFlags("", kubehome)
		if err != nil {
			log.Errorf("error loading kubernetes configuration: %s", err)
			os.Exit(1)
		}
	}
	// generate the client based off of the config
	client, err := kubernetes.NewForConfig(k8scfg)
	if err != nil {
		log.Errorf("error creating kubernetes client: %s", err)
		os.Exit(1)
	}
	log.Info("Successfully constructed k8s client")
	return client, nil
}
