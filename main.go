package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/docker/docker/api/types"
	"github.com/greboid/dockerdnsmanager/containermonitor"
)

func main() {
	cm, err := containermonitor.NewContainerMonitor(context.Background())
	if err != nil {
		log.Fatalf("Unable to create container monitor.")
	}
	cm.AddCreateHook(func(json *types.ContainerJSON) {
		log.Printf("Container created: %s (%s)", json.Name, json.Image)
	})
	cm.AddDestroyHook(func(json *types.ContainerJSON) {
		log.Printf("Container destroyed: %s (%s)", json.Name, json.Image)
	})
	err = cm.Start()
	if err != nil {
		log.Fatalf("Unable to start container monitor.")
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, os.Kill)
	<-stop
}
