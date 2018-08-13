# Kubernetes Custom Job Sync Controller

## Usecase
In event of a deployment and a corresponding cronjob, this controller updates the cronjob with the image from the deployment.

## How does it work
The controller just watches for deployment events inside a specific namespace and updates the corresponding jobs for a given deployment in that namespace. 

How does the controller know which deployments to watch? 
Every deployment that needs to be controlled via the controller should have an annotation `jobsync.k8s.io/enabled: "True"`. In addition, the deployment also specifies the list of cronjobs to be updated as a list. This gives the app developer greater flexibility in terms of keeping certain cronjobs to be synced with the deployment and certain others to be not synced. In addiiton, the specific cronjob that needs to be synced also needs to have the enabled annotation. Please refer to the deployment.yaml and cronjob.yaml in the sample-app folder. 

Limitations:
- Becuase the controller is watching for events, we want to restrict it at a namespace level
- This can only update cronjobs and not jobs. But that can be easily extended.

## Testing
Build a simple app for monitoring deployment changes. Available in the sample-app folder

Building the sample app
```
eval $(minikube docker-env)
docker build -t venkatvghub/ping_server:v3 -f Dockerfile .
```
Follow updating the image from https://kubernetes.io/docs/tutorials/stateless-application/hello-minikube/


```dep ensure
go build -o <executable> *.go
./<executable>
```

Start a deployment in the minikube context and run the executable to listen for all k8s deployments

