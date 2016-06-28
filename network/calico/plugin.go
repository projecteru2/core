package calico

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	enginenetwork "github.com/docker/engine-api/types/network"
	"gitlab.ricebook.net/platform/core/utils"
	"golang.org/x/net/context"
)

type Titanium struct{}

func (t *Titanium) Type() string {
	return "plugin"
}

func (t *Titanium) Name() string {
	return "calico"
}

func (t *Titanium) ConnectToNetwork(ctx context.Context, containerID, networkID string) error {
	if len(containerID) != 64 {
		return fmt.Errorf("ContainerID must be in length of 64")
	}

	engine, ok := utils.FromDockerContext(ctx)
	if !ok {
		return fmt.Errorf("Not actually a `engineapi.Client` for value engine in context", containerID)
	}

	// TODO specified IPs
	config := &enginenetwork.EndpointSettings{
		IPAMConfig: &enginenetwork.EndpointIPAMConfig{},
	}

	log.Debugf("Connect %q to %q", containerID, networkID)
	return engine.NetworkConnect(context.Background(), networkID, containerID, config)
}

func (t *Titanium) DisconnectFromNetwork(ctx context.Context, containerID, networkID string) error {
	if len(containerID) != 64 {
		return fmt.Errorf("ContainerID must be in length of 64")
	}

	engine, ok := utils.FromDockerContext(ctx)
	if !ok {
		return fmt.Errorf("Not actually a `engineapi.Client` for value engine in context", containerID)
	}

	log.Debugf("Disconnect %q from %q", containerID, networkID)
	return engine.NetworkDisconnect(context.Background(), networkID, containerID, false)
}
