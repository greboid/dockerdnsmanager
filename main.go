package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/csmith/envflag"
	"github.com/greboid/dockerdnsmanager/containerapi"
	"github.com/greboid/dockerdnsmanager/containermonitor"
)

var (
	debug = flag.Bool("debug", false, "Show debug")
)

func main() {
	envflag.Parse()
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
	log.Printf("Debug: %t", *debug)
	client, err := containerapi.NewClient()
	if err != nil {
		log.Fatalf("Unable to create client: %s", err)
	}
	version, err := client.GetEngineType()
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
	cm.AddCreateHook(func(json *containerapi.Container) {
		log.Printf("Container created: %s (%s) Labels: %s | Ports: %v", json.Name, json.Image, json.Label, json.Ports)
	})
	cm.AddDestroyHook(func(json *containerapi.Container) {
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
