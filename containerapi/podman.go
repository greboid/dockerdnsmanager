package containerapi

type PodmanClient struct {
	client *ContainerAPI
}

func NewPodmanClient(apiClient *ContainerAPI) *PodmanClient {
	return &PodmanClient{
		client: apiClient,
	}
}

func (p PodmanClient) GetStream() (<-chan *ContainerEvent, error) {
	panic("implement me")
}

func (p PodmanClient) GetContainerFromID(ID string) (*Container, error) {
	panic("implement me")
}