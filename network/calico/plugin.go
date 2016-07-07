package calico

import (
	"fmt"
	"net"

	log "github.com/Sirupsen/logrus"
	enginenetwork "github.com/docker/engine-api/types/network"
	"gitlab.ricebook.net/platform/core/utils"
	"golang.org/x/net/context"
)

type titanium struct{}

// type of the network manager
// if set to "plugin", then it will act like a plugin
// if set to "agent", then it will act like an agent
// main difference is the order of connect/disconnect
func (t *titanium) Type() string {
	return "plugin"
}

// name of the network manager
func (t *titanium) Name() string {
	return "calico"
}

// connect to network with ipv4 address
func (t *titanium) ConnectToNetwork(ctx context.Context, containerID, networkID, ipv4 string) error {
	if len(containerID) != 64 {
		return fmt.Errorf("ContainerID must be in length of 64")
	}

	engine, ok := utils.FromDockerContext(ctx)
	if !ok {
		return fmt.Errorf("Not actually a `engineapi.Client` for value engine in context", containerID)
	}

	config := &enginenetwork.EndpointSettings{
		IPAMConfig: &enginenetwork.EndpointIPAMConfig{},
	}

	// set specified IP
	// but if IP is empty, just ignore
	if ipv4 != "" {
		ip := net.ParseIP(ipv4)
		if ip == nil {
			return fmt.Errorf("IP Address is not valid: %q", ipv4)
		}

		config.IPAMConfig.IPv4Address = ip.String()
	}

	log.Debugf("Connect %q to %q with IP %q", containerID, networkID, ipv4)
	return engine.NetworkConnect(context.Background(), networkID, containerID, config)
}

// disconnect from network
func (t *titanium) DisconnectFromNetwork(ctx context.Context, containerID, networkID string) error {
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

func New() *titanium {
	return &titanium{}
}
