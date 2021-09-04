package containermonitor

import (
	"context"

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
	containerEvents, err := m.client.APIClient.GetStream()
	if err != nil {
		return err
	}
	m.handleStream(containerEvents)
	return nil
}

func (m *ContainerMonitor) handleStream(events <-chan *containerapi.ContainerEvent) {
	for {
		select {
		case event := <-events:
			m.handleContainerEvent(event)
		}
	}
}

func (m *ContainerMonitor) handleContainerEvent(event *containerapi.ContainerEvent) {
	if event.Type == "service" {
		return
	}
	switch event.Action {
	case "create":
		m.RunningContainers[event.Container.ID] = event.Container
		for index := range m.createHooks {
			m.createHooks[index](event.Container)
		}
	case "remove":
		container := m.RunningContainers[event.Container.ID]
		if container == nil {
			return
		}
		for index := range m.destroyHooks {
			m.destroyHooks[index](container)
		}
		delete(m.RunningContainers, event.Container.ID)
	case "destroy":
		container := m.RunningContainers[event.Container.ID]
		if container == nil {
			return
		}
		for index := range m.destroyHooks {
			m.destroyHooks[index](container)
		}
		delete(m.RunningContainers, event.Container.ID)
	}
}
