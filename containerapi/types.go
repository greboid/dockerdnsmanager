package containerapi

import (
	"encoding/json"
	"log"
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
	tlsVerify     bool
	HttpClient    *http.Client
	requestScheme string
	transportScheme string
	transportHost string
	Url           *url.URL
	APIClient     ApiClient
}

type ApiClient interface {
	GetStream() (<-chan *ContainerEvent, error)
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
	Type string
	Action    string
	Container *Container
	Service *Service
}

type Service struct {
	ID string
	Name string
	Label map[string]string
	Ports []Port
}

type Container struct {
	ID    string
	Name  string
	Image string
	Label map[string]string
	Ports []Port
}

type Port struct {
	PrivatePort int
	PublicPort int
	Type string
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

func MustJsonMarshall(value interface{}) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		log.Panicf("Unable to json marshall: %s", err)
	}
	return data
}
