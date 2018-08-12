package main

import (
	"log"
	"reflect"
	"strconv"
	"sync"
	"time"

	v1beta1 "k8s.io/api/apps/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// DeploymentController watches the kubernetes api for changes to Deployments
// and updates the corresponding annotated Jobs with the same image
type DeploymentController struct {
	deploymentInformer cache.SharedIndexInformer
	client             *kubernetes.Clientset
}

// NewDeploymentController creates a new NewDeploymentController
func NewDeploymentController(client *kubernetes.Clientset, opts map[string]string) *DeploymentController {
	deploymentWatcher := &DeploymentController{}

	dryRun, _ := strconv.ParseBool(opts["dryRun"])
	version, err := client.ServerVersion()

	if err != nil {
		log.Fatalf("Failed to retrieve server version %v", err)
	}

	// Create informer for watching Namespaces
	deploymentInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return client.AppsV1beta1().Deployments(opts["namespace"]).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return client.AppsV1beta1().Deployments(opts["namespace"]).Watch(options)
			},
		},
		&v1beta1.Deployment{},
		time.Second*30,
		cache.Indexers{},
	)
	deploymentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			deploymentWatcher.deploymentAdded(cur, dryRun, *version)
		},
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				deploymentWatcher.deploymentAdded(cur, dryRun, *version)
			}
		},
	})

	deploymentWatcher.client = client
	deploymentWatcher.deploymentInformer = deploymentInformer

	return deploymentWatcher
}

// Run starts the process for listening for deployment changes and acting upon those changes.
func (c *DeploymentController) Run(stopCh <-chan struct{}, wg *sync.WaitGroup) {
	log.Printf("Listening for changes...")
	// When this function completes, mark the go function as done
	defer wg.Done()

	// Increment wait group as we're about to execute a go function
	wg.Add(1)

	// Execute go function
	go c.deploymentInformer.Run(stopCh)

	// Wait till we receive a stop signal
	<-stopCh
}

func (c *DeploymentController) deploymentAdded(cur interface{}, dryRun bool, version version.Info) {
	deploymentObj := cur.(*v1beta1.Deployment)
	isDeploymentAvailable := false
	if deploymentObj.Status.ObservedGeneration < deploymentObj.Generation {
		for _, d := range deploymentObj.Status.Conditions {
			if d.Status == "True" {
				isDeploymentAvailable = true
				break
			}
		}
	}
	if isDeploymentAvailable == true {
		log.Printf("Deployment Added:%s", deploymentObj.ObjectMeta.Name)
	}
}

func (c *DeploymentController) deploymentDeleted(cur interface{}, dryRun bool, version version.Info) {
	deploymentObj := cur.(*v1beta1.Deployment)
	log.Printf("Deployment Deleted:%s", deploymentObj.ObjectMeta.Name)
}

func (c *DeploymentController) deploymentUpdated(cur interface{}, dryRun bool, version version.Info) {
	deploymentObj := cur.(*v1beta1.Deployment)
	log.Printf("Deployment Updated:%s", deploymentObj.ObjectMeta.Name)
}
