package containerapi

type PodmanClient struct {
	client *ContainerAPI
}

func newPodmanClient(apiClient *ContainerAPI) PodmanClient {
	return PodmanClient{
		client: apiClient,
	}
}

func (p PodmanClient) GetEvents() (<-chan ContainerEvent, error) {
	panic("implement me")
}

func (p PodmanClient) GetContainerEvents() (<-chan ContainerEvent, error) {
	panic("implement me")
}

func (p PodmanClient) GetExistingContainers() ([]*Container, error) {
	panic("implement me")
}

func (p PodmanClient) GetContainerFromID(ID string) (*Container, error) {
	panic("implement me")
}
