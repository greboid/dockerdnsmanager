package containerapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
)

type ContainerAPI struct {
	apiVersion string
	certPath   string
	tlsVerify  bool
	client     *http.Client
	requestScheme string
	transportScheme string
	transportHost string
	url *url.URL
}

type EngineType int

const (
	Docker = iota
	Podman
	DockerSwarm
	Unknown
)

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

const (
	defaultDockerHost = "unix:///var/run/docker.sock"
)

func NewClient() (*ContainerAPI, error) {
	c := &ContainerAPI{}
	if dockerCertPath := os.Getenv("DOCKER_CERT_PATH"); dockerCertPath != "" {
		options := tlsconfig.Options{
			CAFile:             filepath.Join(dockerCertPath, "ca.pem"),
			CertFile:           filepath.Join(dockerCertPath, "cert.pem"),
			KeyFile:            filepath.Join(dockerCertPath, "key.pem"),
			InsecureSkipVerify: os.Getenv("DOCKER_TLS_VERIFY") == "",
		}
		tlsc, err := tlsconfig.Client(options)
		if err != nil {
			return nil, err
		}

		c.client = &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsc},
		}
	}
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = defaultDockerHost
	}
	if err := c.parseHost(host); err != nil {
		return nil, err
	}
	httpClient, err := c.getHttpClient()
	if err != nil {
		return nil, err
	}
	c.client = httpClient

	if version := os.Getenv("DOCKER_API_VERSION"); version != "" {
		c.apiVersion = version
	}
	return c, nil
}

func (c *ContainerAPI) parseHost(host string) error {
	splitURL := strings.Split(host, "://")
	if len(splitURL) != 2 {
		return fmt.Errorf("invalid DOCKER_HOST URL")
	}

	scheme, host, path := splitURL[0], splitURL[1], ""
	transportScheme := scheme
	transportHost := host
	if scheme == "tcp" {
		parsed, err := url.Parse("tcp://" + host)
		if err != nil {
			return err
		}
		host = parsed.Host
		path = parsed.Path
	} else if scheme == "unix" {
		transportScheme = "unix"
		transportHost = host
		scheme = "http"
		host = "unix"
	}
	c.transportScheme = transportScheme
	c.transportHost = transportHost
	parsedURL, err := url.Parse(scheme + "://" + host + "/" + path)
	if err != nil {
		return err
	}
	c.url = parsedURL
	return nil
}

func (c *ContainerAPI) getHttpClient() (*http.Client, error) {
	transport := new(http.Transport)
	err := sockets.ConfigureTransport(transport, c.transportScheme, c.transportHost)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport:     transport,
	}, nil
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

func (c *ContainerAPI) GetProtocol() (EngineType, error) {
	enginetype := Unknown
	newURL := c.url
	newURL.Path = "/version"
	resp, err := c.client.Get(newURL.String())
	if err != nil {
		return Unknown, err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return Unknown, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	ver := &VersionResponse{}
	err = json.Unmarshal(data, ver)
	if err != nil {
		return Unknown, err
	}
	if len(ver.Components) < 1 {
		return Unknown, err
	}
	if ver.Components[0].Name == "Engine" {
		enginetype = Docker
	} else if ver.Components[0].Name == "Podman Engine" {
		enginetype = Podman
	}
	newURL = c.url
	newURL.Path = "/info"
	resp, err = c.client.Get(newURL.String())
	if err != nil {
		return Unknown, err
	}
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return Unknown, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	inf := &InfoResponse{}
	err = json.Unmarshal(data, inf)
	if err != nil {
		return Unknown, err
	}
	if enginetype == Docker && inf.Swarm.LocalNodeState == "active" {
		return DockerSwarm, nil
	} else if enginetype == Docker {
		return Docker, nil
	} else if enginetype == Podman {
		return Podman, nil
	}
	return Unknown, nil
}
