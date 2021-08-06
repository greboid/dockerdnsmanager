package containermonitor

import (
	"context"
	"log"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type ContainerMonitor struct {
	client            *client.Client
	ctx               context.Context
	RunningContainers map[string]*types.ContainerJSON
	createHooks       []func(*types.ContainerJSON)
	destroyHooks      []func(*types.ContainerJSON)
	Debug             bool
}

type ContainerEvent struct {
	ID        string
	Container *types.ContainerJSON
	Action    string
}

func NewContainerMonitor(ctx context.Context) (*ContainerMonitor, error) {
	m := &ContainerMonitor{}
	m.ctx = ctx
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	m.client = cli
	if m.RunningContainers == nil {
		m.RunningContainers = make(map[string]*types.ContainerJSON)
	}
	m.createHooks = make([]func(*types.ContainerJSON), 0)
	m.destroyHooks = make([]func(json *types.ContainerJSON), 0)
	return m, nil
}

func (m *ContainerMonitor) AddCreateHook(function func(json *types.ContainerJSON)) {
	m.createHooks = append(m.createHooks, function)
}

func (m *ContainerMonitor) AddDestroyHook(function func(json *types.ContainerJSON)) {
	m.destroyHooks = append(m.destroyHooks, function)
}

func (m *ContainerMonitor) Start() error {
	m.handleStream(m.listen(m.client.Events(m.ctx, types.EventsOptions{Filters: m.getEventFilter()})))
	return nil
}

func (m *ContainerMonitor) getEventFilter() filters.Args {
	args := filters.NewArgs()
	if !m.Debug {
		args.Add("type", "container")
		args.Add("event", "create")
		args.Add("event", "destroy")
	}
	return args
}

func (m *ContainerMonitor) handleStream(events chan ContainerEvent) {
	for {
		select {
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
			}
		}
	}
}

func (m *ContainerMonitor) listen(containerEvents <-chan events.Message, errors <-chan error) chan ContainerEvent {
	containers := make(chan ContainerEvent)
	timer := time.NewTimer(30 * time.Second)
	go func() {
		for {
			select {
			case event := <-containerEvents:
				switch event.Status {
				case "create":
					container, err := m.getContainerFromID(event.Actor.ID)
					if err == nil {
						m.RunningContainers[event.Actor.ID] = container
						containers <- ContainerEvent{
							ID:        event.Actor.ID,
							Container: container,
							Action:    event.Status,
						}
					}
				case "destroy":
					containers <- ContainerEvent{
						ID:        event.Actor.ID,
						Container: m.RunningContainers[event.Actor.ID],
						Action:    event.Status,
					}
					delete(m.RunningContainers, event.Actor.ID)
				default:
					log.Printf("Unknown action: %+v", event)
				}
			case <-errors:
				timer.Stop()
				close(containers)
			case <-m.ctx.Done():
				timer.Stop()
				close(containers)
			case <-timer.C:
				existing, err := m.getExisting()
				if err == nil {
					for index := range existing {
						containers <- ContainerEvent{
							ID:        existing[index].ID,
							Container: existing[index],
							Action:    "create",
						}
					}
				}
			}
		}
	}()
	return containers
}

func (m *ContainerMonitor) getContainerFromID(id string) (*types.ContainerJSON, error) {
	container, err := m.client.ContainerInspect(m.ctx, id)
	if err != nil {
		return nil, err
	}
	return &container, err
}

func (m *ContainerMonitor) getExisting() ([]*types.ContainerJSON, error) {
	containers, err := m.client.ContainerList(m.ctx, types.ContainerListOptions{})
	if err != nil {
		return nil, err
	}
	list := make([]*types.ContainerJSON, 0)
	for index := range containers {
		container, err := m.getContainerFromID(containers[index].ID)
		if err == nil {
			list = append(list, container)
		}
	}
	return list, nil
}
