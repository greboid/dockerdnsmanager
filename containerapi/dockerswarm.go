package containerapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
)

type Client struct {
	client            *ContainerAPI
	runningContainers map[string]*SwarmContainer
	runningServices   map[string]*SwarmService
	haveServices      chan bool
	haveContainers chan bool
	ExposeContainers bool
	ExposeServices bool
}

func NewDockerSwarmClient(apiClient *ContainerAPI) *Client {
	return &Client{
		client:            apiClient,
		runningContainers: map[string]*SwarmContainer{},
		runningServices:   map[string]*SwarmService{},
		haveServices:      make(chan bool, 1),
		haveContainers:    make(chan bool, 1),
		ExposeContainers:  true,
		ExposeServices:  true,
	}
}

func (d *Client) GetStream() (<-chan *ContainerEvent, error) {
	exposedEvents := make(chan *ContainerEvent)
	serviceEvents := make(chan *SwarmServiceEvent)
	containerEvents := make(chan *SwarmContainerEvent)
	err := d.startEvents(serviceEvents, containerEvents)
	if err != nil {
		return nil, err
	}
	go func() {
		existing, err := d.getExistingServices()
		if err == nil {
			for index := range existing {
				d.runningServices[existing[index].ID] = existing[index]
				serviceEvents <- &SwarmServiceEvent{
					event:   "create",
					Service: existing[index],
				}
			}
		}
	}()
	go func() {
		existing, err := d.getExistingContainers()
		if err == nil {
			for index := range existing {
				d.runningContainers[existing[index].ID] = existing[index]
				containerEvents <- &SwarmContainerEvent{
					Event:     "create",
					Container: existing[index],
				}
			}
		}
	}()
	go func() {
		for {
			select {
			case event := <- serviceEvents:
				if d.ExposeServices {
					service := d.getServiceForDockerService(event.Service)
					exposedEvent := &ContainerEvent{
						Type:      "service",
						Action:    event.event,
						Container: nil,
						Service:   service,
					}
					exposedEvents <- exposedEvent
				}
			case event := <- containerEvents:
				if d.ExposeContainers {
					container := d.getContainerForDockerContainer(event.Container)
					d.copyLabelsFromService(container)
					d.copyPortsFromService(container)
					exposedEvent := &ContainerEvent{
						Type:      "container",
						Action:    event.Event,
						Container: container,
						Service:   nil,
					}
					exposedEvents <- exposedEvent
				}
			}
		}
	}()
	return exposedEvents, nil
}

func (d *Client) startEvents(serviceEvents chan *SwarmServiceEvent, containerEvents chan *SwarmContainerEvent) error {
	newURL := d.client.Url
	newURL.Path = "/events"
	resp, err := d.client.HttpClient.Get(newURL.String())
	if err != nil {
		close(serviceEvents)
		close(containerEvents)
		return err
	}
	go func() {
		defer func() {
			_ = resp.Body.Close()
			close(serviceEvents)
			close(containerEvents)
		}()
		<-d.haveContainers
		<-d.haveServices
		decoder := json.NewDecoder(resp.Body)
		containerEvent := &SwarmEvent{}
		for decoder.More() {
			err = decoder.Decode(containerEvent)
			if err != nil {
				continue
			}
			switch containerEvent.Type {
			case "container":
				d.handleContainerEvent(containerEvent, containerEvents)
			case "service":
				d.handleServiceEvent(containerEvent, serviceEvents)
			}
		}
	}()
	return nil
}

func (d *Client) handleContainerEvent(event *SwarmEvent, events chan *SwarmContainerEvent) {
	log.Printf("event: %s", event.Action)
	container, err := d.getDockerContainerFromID(event.Actor.ID)
	if err != nil && event.Action == "destroy" {
		container = d.runningContainers[event.Actor.ID]
		delete(d.runningContainers, event.Actor.ID)
	} else if err != nil {
		log.Printf("Unable to get container for event: %s", err)
		return
	}
	if event.Action == "create" {
		d.runningContainers[container.ID] = container
	}
	events <- &SwarmContainerEvent{
		Event:     event.Action,
		Container: container,
	}
}

func (d *Client) handleServiceEvent(event *SwarmEvent, events chan *SwarmServiceEvent) {
	service, err := d.getDockerServiceFromID(event.Actor.ID)
	if err != nil && event.Action == "remove" {
		service = d.runningServices[event.Actor.ID]
		delete(d.runningServices, event.Actor.ID)
	} else if err != nil {
		log.Printf("Unable to get service for event: %s", err)
		return
	}
	if event.Action == "create" {
		d.runningServices[service.ID] = service
	}
	events <- &SwarmServiceEvent{
		event:   event.Action,
		Service: service,
	}
}

func (d *Client) getDockerServiceFromID(ID string) (*SwarmService, error) {
	newURL := d.client.Url
	newURL.RawQuery = url.Values{
		"filters": {string(MustJsonMarshall(map[string][]string{ "id": {ID} }))},
	}.Encode()
	newURL.Path = "/services"
	resp, err := d.client.HttpClient.Get(newURL.String())
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("container not found")
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var dockerService []SwarmService
	err = json.Unmarshal(data, &dockerService)
	if err != nil {
		return nil, err
	}
	if len(dockerService) == 0 {
		return nil, fmt.Errorf("container not found")
	}
	return &dockerService[0], nil
}

func (d *Client) getServiceFromID(ID string) (*Service, error) {
	service, err := d.getDockerServiceFromID(ID)
	if err != nil {
		return nil, err
	}
	return d.getServiceForDockerService(service), nil
}

func (d *Client) getServiceForDockerService(ss *SwarmService) *Service {
	service := &Service{
		ID:    ss.ID,
		Name:  ss.Spec.Name,
		Label: ss.Spec.Labels,
		Ports: []Port{},
	}
	for index := range ss.Spec.Ports {
		service.Ports = append(service.Ports, Port{
			PrivatePort: ss.Spec.Ports[index].PrivatePort,
			PublicPort:  ss.Spec.Ports[index].PublicPort,
			Type:        ss.Spec.Ports[index].Type,
		})
	}
	return service
}

func (d *Client) getExistingContainers() ([]*SwarmContainer, error) {
	newURL := d.client.Url
	newURL.Path = "/containers/json"
	resp, err := d.client.HttpClient.Get(newURL.String())
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	dockerContainers := make([]*SwarmContainer, 0)
	err = json.Unmarshal(data, &dockerContainers)
	if err != nil {
		return nil, err
	}
	d.haveContainers <- true
	return dockerContainers, nil
}

func (d *Client) getExistingServices() ([]*SwarmService, error) {
	newURL := d.client.Url
	newURL.Path = "/services"
	resp, err := d.client.HttpClient.Get(newURL.String())
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	swarmServices := make([]*SwarmService, 0)
	err = json.Unmarshal(data, &swarmServices)
	if err != nil {
		return nil, err
	}
	d.haveServices <- true
	return swarmServices, nil
}

func (d *Client) getDockerContainerFromID(ID string) (*SwarmContainer, error) {
	newURL := d.client.Url
	newURL.RawQuery = url.Values{
		"filters": {string(MustJsonMarshall(map[string][]string{ "id": {ID} }))},
		"all": {"true"},
	}.Encode()
	newURL.Path = "/containers/json"
	resp, err := d.client.HttpClient.Get(newURL.String())
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("container not found")
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var dockerContainer []*SwarmContainer
	err = json.Unmarshal(data, &dockerContainer)
	if err != nil {
		return nil, err
	}
	if len(dockerContainer) == 0 {
		return nil, fmt.Errorf("container not found")
	}
	return dockerContainer[0], nil
}

func (d *Client) GetContainerFromID(ID string) (*Container, error) {
	dockerContainer, err := d.getDockerContainerFromID(ID)
	if err != nil {
		return nil, err
	}
	return d.getContainerForDockerContainer(dockerContainer), nil
}

func (d *Client) getContainerForDockerContainer(dc *SwarmContainer) *Container {
	if dc.Names == nil {
		dc.Names = []string{dc.Names[0]}
	}
	container := &Container{
		ID:    dc.ID,
		Name:  dc.Names[0],
		Image: dc.Image,
		Label: dc.Labels,
		Ports: []Port{},
	}
	for index := range dc.Ports {
		container.Ports = append(container.Ports, Port{
			PrivatePort: dc.Ports[index].PrivatePort,
			PublicPort:  dc.Ports[index].PublicPort,
			Type:        dc.Ports[index].Type,
		})
	}
	return container
}

func (d *Client) copyLabelsFromService(container *Container) {
	serviceID, ok := container.Label["com.docker.swarm.service.id"]
	if !ok {
		return
	}
	service, ok := d.runningServices[serviceID]
	if !ok {
		return
	}
	for key, value := range service.Spec.Labels {
		if _, ok = container.Label[key]; !ok {
			container.Label[key] = value
		}
	}
}

func (d *Client) copyPortsFromService(container *Container) {
	serviceID, ok := container.Label["com.docker.swarm.service.id"]
	if !ok {
		return
	}
	service, ok := d.runningServices[serviceID]
	if !ok {
		return
	}
	log.Printf("%+v", service)
}

type SwarmServiceEvent struct {
	event string
	Service *SwarmService
}

type SwarmService struct {
	ID string `json:"ID"`
	Spec struct {
		Name string `json:"Name"`
		Labels map[string]string `json:"Labels"`
		Ports []struct {
			PrivatePort int `json:"PrivatePort"`
			PublicPort int `json:"PublicPort"`
			Type string `json:"Type"`
		} `json:"Ports"`
	} `json:"Spec"`
}

type SwarmContainerEvent struct {
	Event     string
	Container *SwarmContainer
}

type SwarmContainer struct {
	ID    string   `json:"Id"`
	Names []string `json:"Names"`
	Image string   `json:"Image"`
	Ports []struct {
		PrivatePort int `json:"PrivatePort"`
		PublicPort int `json:"PublicPort"`
		Type string `json:"Type"`
	} `json:"Ports"`
	Labels map[string]string `json:"Labels"`
}

type SwarmEvent struct {
	Type string `json:"Type"`
	Action string          `json:"Action"`
	Actor  SwarmEventActor `json:"Actor"`
	Time   int64           `json:"time"`
}

type SwarmEventActor struct {
	ID string `json:"ID"`
	Attributes map[string]string `json:"Attributes"`
}