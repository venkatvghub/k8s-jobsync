package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	v1beta1 "k8s.io/api/apps/v1beta1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
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

//JobStruct ...
type JobStruct struct {
	Jobs []string `json:"jobs"`
}

const (
	//JobSyncEnabledAnnotation annotation for enabling job sync
	JobSyncEnabledAnnotation = "jobsync.k8s.io/enabled"
	//JobSyncedAnnotation annotation for marking deployment as sycned
	JobSyncedAnnotation = "jobsync.k8s.io/synced"
	//JobAnnotationPrefix annotation prefix for list of jobs
	JobAnnotationPrefix = "jobsync.k8s.io/jobs"
)

// NewDeploymentController creates a new NewDeploymentController
func NewDeploymentController(client *kubernetes.Clientset, opts map[string]string) *DeploymentController {
	deploymentWatcher := &DeploymentController{}

	dryRun, _ := strconv.ParseBool(opts["dryRun"])
	namespace := opts["namespace"]
	if namespace == "" {
		log.Error("Namespace is not defined or empty. Cannot Proceed")
		os.Exit(1)
	}
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
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				deploymentWatcher.deploymentUpdated(cur, dryRun, *version, opts["namespace"])
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

func (c *DeploymentController) deploymentUpdated(cur interface{}, dryRun bool, version version.Info, namespace string) {
	deploymentObj := cur.(*v1beta1.Deployment)
	isDeploymentAnnotated := c.isDeploymentAnnotated(deploymentObj)
	isDeploymentAvailable := false
	if isDeploymentAnnotated && deploymentObj.Status.ObservedGeneration < deploymentObj.Generation {
		for _, d := range deploymentObj.Status.Conditions {
			log.Printf("Deployment Status:%s, Size:%s", d.Status, d.Reason)
			if d.Status == "True" {
				isDeploymentAvailable = true
				break
			}
		}
	}
	if isDeploymentAvailable == true {
		c.applyJobDeployment(deploymentObj, namespace)
	}
}

func (c *DeploymentController) isDeploymentAnnotated(deployment *v1beta1.Deployment) bool {
	_, ok := deployment.ObjectMeta.Annotations[JobSyncEnabledAnnotation]
	return ok
}

func (c *DeploymentController) getJobsForDeployment(deployment *v1beta1.Deployment) *JobStruct {
	annotations := deployment.ObjectMeta.Annotations[JobAnnotationPrefix]
	jobStruct := JobStruct{}
	json.Unmarshal([]byte(annotations), &jobStruct)
	return &jobStruct
}

func (c *DeploymentController) buildJobMap(deployment *v1beta1.Deployment) *map[string]string {
	jobStruct := c.getJobsForDeployment(deployment)
	deploymentImage := deployment.Spec.Template.Spec.Containers[0].Image
	jobs := jobStruct.Jobs
	jobImageMap := make(map[string]string)
	for id := range jobs {
		jobName := jobs[id]
		jobImageMap[string(jobName)] = string(deploymentImage)
	}
	return &jobImageMap
}

func (c *DeploymentController) applyJobDeployment(deployment *v1beta1.Deployment, namespace string) {
	log.Printf("Found Deployment %s with Annotation", deployment.Name)
	jobImageMap := c.buildJobMap(deployment)
	log.Print("CALLING SYNC_CRON_JOB")
	err := c.syncCronJob(*jobImageMap, deployment, namespace)
	if err != nil {
		log.Printf("Sync Cronjob Error:%s", err)
	} else {
		log.Printf("Deployment Occured for Deployment:%s, NameSpace:%s", deployment.Name, deployment.Namespace)
	}
}

func (c *DeploymentController) syncCronJob(jobs map[string]string, deployment *v1beta1.Deployment, namespace string) error {
	k8scli := c.client
	cronList, err := k8scli.BatchV1beta1().CronJobs(namespace).List(metav1.ListOptions{})
	if err != nil {
		log.Printf("failed to list cronjobs %v\n", err)
		return err
	}
	for _, cronJob := range cronList.Items {
		isAnnotated := c.isJobAnnotated(&cronJob)
		log.Printf("JOB NAME:%s, IS_ANNOTATED:%v", cronJob.GetName(), isAnnotated)
		if c.isJobAnnotated(&cronJob) {
			jobName := cronJob.GetName()
			if image, ok := jobs[jobName]; ok {
				cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image = image
				log.Printf("IMAGE INSIDE:%s", image)
				if _, err := k8scli.BatchV1beta1().CronJobs(namespace).Update(&cronJob); err != nil {
					log.Printf("Image updated on Cronjob:%s, Image:%s, Deployment:%s", jobName, image, deployment.Name)
					return err
				}
				log.Printf("JOB_NAME:%s, UPDATED_TO:%s, DEPLOYMENT:%s", jobName, image, deployment.Name)
			}
		}
	}
	return nil
}

func (c *DeploymentController) isJobAnnotated(cronJob *batchv1beta1.CronJob) bool {
	_, ok := cronJob.ObjectMeta.Annotations[JobSyncEnabledAnnotation]
	return ok
}
