package containerapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
)

type DockerClient struct {
	client *ContainerAPI
	runningContainers map[string]*DockerContainer
	haveContainers chan bool
}

func NewDockerClient(apiClient *ContainerAPI) *DockerClient {
	return &DockerClient{
		client: apiClient,
		runningContainers: map[string]*DockerContainer{},
		haveContainers:    make(chan bool, 1),
	}
}

func (d *DockerClient) GetStream() (<-chan *ContainerEvent, error) {
	exposedEvents := make(chan *ContainerEvent)
	containerEvents := make(chan *DockerContainerEvent)
	err := d.startEvents(containerEvents)
	if err != nil {
		return nil, err
	}
	go func() {
		log.Printf("Getting existing")
		existing, err := d.getExistingContainers()
		if err != nil {
			log.Printf("Error getting existing: %s", err)
		}
		log.Printf("Finished getting existing")
		if err == nil {
			for index := range existing {
				d.runningContainers[existing[index].ID] = existing[index]
				containerEvents <- &DockerContainerEvent{
					Event:     "create",
					Container: existing[index],
				}
			}
		}
	}()
	go func() {
		log.Printf("Starting stream")
		for {
			select {
			case event := <- containerEvents:
					exposedEvent := &ContainerEvent{
						Type:      "container",
						Action:    event.Event,
						Container: d.getContainerForDockerContainer(event.Container),
					}
					exposedEvents <- exposedEvent
			}
		}
	}()
	return exposedEvents, nil
}

func (d *DockerClient) startEvents(containerEvents chan *DockerContainerEvent) error {
	log.Printf("Starting events")
	newURL := d.client.Url
	newURL.Path = "/events"
	resp, err := d.client.HttpClient.Get(newURL.String())
	if err != nil {
		close(containerEvents)
		return err
	}
	go func() {
		defer func() {
			_ = resp.Body.Close()
			close(containerEvents)
		}()
		log.Printf("Waiting on existing containers")
		<-d.haveContainers
		log.Printf("Finished waiting on existing containers")
		decoder := json.NewDecoder(resp.Body)
		containerEvent := &DockerEvent{}
		for decoder.More() {
			err = decoder.Decode(containerEvent)
			if err != nil {
				log.Printf("Error decoding event: %s", err)
				continue
			}
			log.Printf("/events: %+v", containerEvent)
			switch containerEvent.Type {
			case "container":
				d.handleContainerEvent(containerEvent, containerEvents)
			}
		}
	}()
	return nil
}

func (d *DockerClient) getExistingContainers() ([]*DockerContainer, error) {
	newURL := d.client.Url
	newURL.Path = "/containers/json"
	log.Printf("Getting containers/json")
	resp, err := d.client.HttpClient.Get(newURL.String())
	if err != nil {
		log.Printf("Error getting containers/json: %s", err)
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	log.Printf("Reading containers/json")
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading containers/json: %s", err)
		return nil, err
	}
	log.Printf("Unmashalling containers/json")
	dockerContainers := make([]*DockerContainer, 0)
	err = json.Unmarshal(data, &dockerContainers)
	if err != nil {
		log.Printf("Error unmarshalling containers/json: %s", err)
		return nil, err
	}
	log.Printf("Finished getting existing containers")
	d.haveContainers <- true
	return dockerContainers, nil
}

func (d *DockerClient) getContainerForDockerContainer(dc *DockerContainer) *Container {
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

func (d *DockerClient) getDockerContainerFromID(ID string) (*DockerContainer, error) {
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
	var dockerContainer []*DockerContainer
	err = json.Unmarshal(data, &dockerContainer)
	if err != nil {
		return nil, err
	}
	if len(dockerContainer) == 0 {
		return nil, fmt.Errorf("container not found")
	}
	log.Printf("/events -> container: %#+v", dockerContainer[0])
	return dockerContainer[0], nil
}

func (d *DockerClient) GetContainerFromID(ID string) (*Container, error) {
	dockerContainer, err := d.getDockerContainerFromID(ID)
	if err != nil {
		return nil, err
	}
	return d.getContainerForDockerContainer(dockerContainer), nil
}

func (d *DockerClient) handleContainerEvent(event *DockerEvent, events chan *DockerContainerEvent) {
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
	events <- &DockerContainerEvent{
		Event:     event.Action,
		Container: container,
	}
}

type DockerContainerEvent struct {
	Event     string
	Container *DockerContainer
}

type DockerContainer struct {
	ID    string   `json:"Id"`
	Names []string `json:"Names"`
	Image string   `json:"Image"`
	Ports []Port `json:"Ports"`
	Labels map[string]string `json:"Labels"`
}

type DockerEvent struct {
	Type string `json:"Type"`
	Action string          `json:"Action"`
	Actor  DockerEventActor `json:"Actor"`
	Time   int64           `json:"time"`
}

type DockerEventActor struct {
	ID string `json:"ID"`
	Attributes map[string]string `json:"Attributes"`
}