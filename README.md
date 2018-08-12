Build a simple app for monitoring deployment changes. Available in the sample-app folder

Building the sample app
```
eval $(minikube docker-env)
docker build -t venkatvghub/ping_server:v3 -f Dockerfile .
```
Follow updating the image from https://kubernetes.io/docs/tutorials/stateless-application/hello-minikube/


```dep ensure
go build -o <executable> main.go
./<executable>
```

Start a deployment in the minikube context and run the executable to listen for all k8s deployments

