FROM golang:1.9.3-alpine
ARG GIT_COMMIT_HASH
ENV GIT_COMMIT_HASH=${GIT_COMMIT_HASH}
COPY main.go /go
RUN go build -o webserver .
ENTRYPOINT ./webserver
EXPOSE 9090
