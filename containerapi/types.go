package containerapi

import (
	"net"
	"net/http"
	"net/url"
)

const (
	defaultDockerHost = "unix:///var/run/docker.sock"

	Docker = iota
	Podman
	DockerSwarm
	Unknown
)

type EngineType int

type ContainerAPI struct {
	apiVersion      string
	certPath        string
	tlsVerify       bool
	httpClient      *http.Client
	requestScheme   string
	transportScheme string
	transportHost   string
	url             *url.URL
	APIClient       ApiClient
}

type ApiClient interface {
	GetContainerEvents() (<-chan ContainerEvent, error)
	GetExistingContainers() ([]*Container, error)
	GetContainerFromID(ID string) (*Container, error)
}

type VersionResponse struct {
	Components []struct {
		Name string `json:"Name"`
	} `json:"Components"`
}

type InfoResponse struct {
	Swarm struct {
		LocalNodeState string `json:"LocalNodeState"`
	} `json:"Swarm"`
}

type ContainerEvent struct {
	Action    string
	Container *Container
}

type Container struct {
	ID    string
	Name  string
	Image string
	Label []string
	Ports []Port
}

type Port struct {
	BindIP        net.IP
	HostPort      int
	ContainerPort int
}

func (s EngineType) String() string {
	switch s {
	case Docker:
		return "Docker"
	case Podman:
		return "Podman"
	case DockerSwarm:
		return "Docker Swarm"
	default:
		return "Unknown"
	}
}
