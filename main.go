package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/docker/docker/api/types"
	"github.com/greboid/dockerdnsmanager/containerapi"
	"github.com/greboid/dockerdnsmanager/containermonitor"
	"github.com/kouhin/envflag"
)

var (
	debug = flag.Bool("debug", false, "Show debug")
)

func main() {
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
	err := envflag.Parse()
	if err != nil {
		log.Fatalf("Unable to parse flags: %s", err)
	}
	client, err := containerapi.NewClient()
	if err != nil {
		log.Fatalf("Unable to create client: %s", err)
	}
	version, err := client.GetProtocol()
	if err != nil {
		log.Fatalf("Unable to get version: %s", err)
	}
	log.Printf("Version: %+v", version)
	//docker client
	cm, err := containermonitor.NewContainerMonitor(context.Background())
	if err != nil {
		log.Fatalf("Unable to create container monitor.")
	}
	cm.Debug = *debug
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
