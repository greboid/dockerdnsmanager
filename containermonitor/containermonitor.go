package containermonitor

import (
	"context"
	"log"
	"time"

	"github.com/greboid/dockerdnsmanager/containerapi"
)

type ContainerMonitor struct {
	client            *containerapi.ContainerAPI
	ctx               context.Context
	RunningContainers map[string]*containerapi.Container
	createHooks       []func(*containerapi.Container)
	destroyHooks      []func(*containerapi.Container)
	Debug             bool
}

func NewContainerMonitor(ctx context.Context) (*ContainerMonitor, error) {
	m := &ContainerMonitor{}
	m.ctx = ctx
	cli, err := containerapi.NewClient()
	if err != nil {
		return nil, err
	}
	m.client = cli
	if m.RunningContainers == nil {
		m.RunningContainers = make(map[string]*containerapi.Container)
	}
	m.createHooks = make([]func(*containerapi.Container), 0)
	m.destroyHooks = make([]func(json *containerapi.Container), 0)
	return m, nil
}

func (m *ContainerMonitor) AddCreateHook(function func(json *containerapi.Container)) {
	m.createHooks = append(m.createHooks, function)
}

func (m *ContainerMonitor) AddDestroyHook(function func(json *containerapi.Container)) {
	m.destroyHooks = append(m.destroyHooks, function)
}

func (m *ContainerMonitor) Start() error {
	log.Printf("Starting")
	containerEvents, err := m.client.APIClient.GetContainerEvents()
	if err != nil {
		return err
	}
	m.handleStream(containerEvents)
	return nil
}

func (m *ContainerMonitor) handleStream(events <-chan containerapi.ContainerEvent) {
	timer := time.NewTimer(30 * time.Second)
	for {
		select {
		case <-timer.C:
			existing, err := m.client.APIClient.GetExistingContainers()
			if err == nil {
				for index := range existing {
					for hookIndex := range m.createHooks {
						m.createHooks[hookIndex](existing[index])
					}
				}
			}
		case event := <-events:
			switch event.Action {
			case "create":
				for index := range m.createHooks {
					m.createHooks[index](event.Container)
				}
			case "destroy":
				for index := range m.destroyHooks {
					m.destroyHooks[index](event.Container)
				}
			default:
				if m.Debug {
					log.Printf("Uparsed Event: %+v", event)
				}
			}
		}
	}
}
