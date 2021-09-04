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

func NewClient() (*ContainerAPI, error) {
	c := &ContainerAPI{}
	if dockerCertPath := os.Getenv("DOCKER_CERT_PATH"); dockerCertPath != "" {
		options := tlsconfig.Options{
			CAFile:             filepath.Join(dockerCertPath, "ca.pem"),
			CertFile:           filepath.Join(dockerCertPath, "cert.pem"),
			KeyFile:            filepath.Join(dockerCertPath, "key.pem"),
			InsecureSkipVerify: os.Getenv("DOCKER_TLS_VERIFY") == "",
		}
		tlsClientConfig, err := tlsconfig.Client(options)
		if err != nil {
			return nil, err
		}

		c.HttpClient = &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsClientConfig},
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
	c.HttpClient = httpClient

	if version := os.Getenv("DOCKER_API_VERSION"); version != "" {
		c.apiVersion = version
	}
	engineType, _ := c.GetEngineType()
	switch engineType {
	case Docker:
		c.APIClient = NewDockerClient(c)
	case DockerSwarm:
		//c.APIClient = NewDockerSwarmClient(c)
		c.APIClient = NewDockerClient(c)
	case Podman:
		c.APIClient = NewPodmanClient(c)
	default:
		return nil, fmt.Errorf("unable to determine engine type")
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
	c.Url = parsedURL
	return nil
}

func (c *ContainerAPI) getHttpClient() (*http.Client, error) {
	transport := new(http.Transport)
	err := sockets.ConfigureTransport(transport, c.transportScheme, c.transportHost)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: transport,
	}, nil
}

func (c *ContainerAPI) GetEngineType() (EngineType, error) {
	enginetype := Unknown
	newURL := c.Url
	newURL.Path = "/version"
	resp, err := c.HttpClient.Get(newURL.String())
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
	newURL = c.Url
	newURL.Path = "/info"
	resp, err = c.HttpClient.Get(newURL.String())
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
