package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func ping(w http.ResponseWriter, r *http.Request) {
	version := os.Getenv("GIT_COMMIT_HASH")
	log.Printf("Hello World Called for Version:%s and Time:%v", version, time.Now().Unix())
	fmt.Fprintf(w, "ping\n") // send data to client side
}

func runCommand(version string) {
	log.Printf("Command Version:%s", version)
}

func main() {
	version := os.Getenv("GIT_COMMIT_HASH")
	isCommand := flag.Bool("command", false, "a bool")
	flag.Parse()
	if *isCommand {
		runCommand(version)
	} else {
		http.HandleFunc("/", ping)               // set router
		err := http.ListenAndServe(":9090", nil) // set listen port
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	}
}
