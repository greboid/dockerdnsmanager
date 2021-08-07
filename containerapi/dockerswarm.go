package containerapi

type DockerSwarmClient struct {
	client *ContainerAPI
}

func newDockerSwarmClient(apiClient *ContainerAPI) DockerSwarmClient {
	return DockerSwarmClient{
		client: apiClient,
	}
}

func (d DockerSwarmClient) GetContainerEvents() (<-chan ContainerEvent, error) {
	panic("implement me")
}

func (d DockerSwarmClient) GetExistingContainers() ([]*Container, error) {
	panic("implement me")
}

func (d DockerSwarmClient) GetContainerFromID(ID string) (*Container, error) {
	panic("implement me")
}
