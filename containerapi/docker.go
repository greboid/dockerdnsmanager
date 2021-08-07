package containerapi

import (
	"encoding/json"
	"io"
)

type DockerClient struct {
	client *ContainerAPI
}

func newDockerClient(apiClient *ContainerAPI) DockerClient {
	return DockerClient{
		client: apiClient,
	}
}

func (d DockerClient) GetContainerEvents() (<-chan ContainerEvent, error) {
	containerEvents := make(chan ContainerEvent)
	url := d.client.url
	url.Path = "/events"
	resp, err := d.client.httpClient.Get(url.String())
	if err != nil {
		close(containerEvents)
		return nil, err
	}
	go func() {
		defer func() {
			_ = resp.Body.Close()
		}()
		decoder := json.NewDecoder(resp.Body)
		containerEvent := &DockerClientEvent{}
		for decoder.More() {
			err = decoder.Decode(containerEvent)
			if err != nil {
				close(containerEvents)
			}
			containerEvents <- ContainerEvent{
				Action:    containerEvent.Action,
				Container: &Container{
					ID:    containerEvent.Actor.ID,
					Name:  containerEvent.Actor.Attributes.Name,
					Image: containerEvent.Actor.Attributes.Image,
					Label: nil,
					Ports: nil,
				},
			}
		}
	}()
	return containerEvents, nil
}

func (d DockerClient) GetExistingContainers() ([]*Container, error) {
	url := d.client.url
	url.Path = "/containers/json"
	resp, err := d.client.httpClient.Get(url.String())
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
	dockerContainers := make([]DockerClientContainer, 0)
	err = json.Unmarshal(data, &dockerContainers)
	if err != nil {
		return nil, err
	}
	containers := make([]*Container, 0)
	for index := range dockerContainers {
		containers = append(containers, &Container{
			ID:    dockerContainers[index].ID,
			Name:  dockerContainers[index].Name[0],
			Image: dockerContainers[index].Image,
			Label: nil,
			Ports: nil,
		})
	}
	return containers, nil
}

func (d DockerClient) GetContainerFromID(_ string) (*Container, error) {
	panic("implement me")
}

type DockerClientContainer struct {
	ID string `json:"Id"`
	Name []string `json:"Names"`
	Image string `json:"Image"`
}

type DockerClientEvent struct {
	Type string `json:"Type"`
	Action string `json:"status"`
	Actor struct {
		ID string `json:"ID"`
		Attributes struct {
			Image string `json:"Image"`
			Name string `json:"Name"`
		} `json:"Attributes"`
	} `json:"Actor"`
}
