package sdn

import (
	"context"
	"fmt"
	"net"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	enginefilters "github.com/docker/docker/api/types/filters"
	enginenetwork "github.com/docker/docker/api/types/network"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

type titanium struct{}

// connect to network with ipv4 address
func (t *titanium) ConnectToNetwork(ctx context.Context, containerID, networkID, ipv4 string) error {
	if len(containerID) != 64 {
		return fmt.Errorf("ContainerID must be in length of 64")
	}

	engine, ok := utils.FromDockerContext(ctx)
	if !ok {
		return fmt.Errorf("Not actually a `engineapi.Client` for value engine in context")
	}

	config := &enginenetwork.EndpointSettings{
		IPAMConfig: &enginenetwork.EndpointIPAMConfig{},
	}

	// set specified IP
	// but if IP is empty, just ignore
	if ipv4 != "" {
		ip := net.ParseIP(ipv4)
		if ip == nil {
			return fmt.Errorf("IP Address is not valid: %v", ipv4)
		}

		config.IPAMConfig.IPv4Address = ip.String()
	}

	log.Debugf("[ConnectToNetwork] Connect %v to %v with IP %v", containerID, networkID, ipv4)
	return engine.NetworkConnect(context.Background(), networkID, containerID, config)
}

// disconnect from network
func (t *titanium) DisconnectFromNetwork(ctx context.Context, containerID, networkID string) error {
	if len(containerID) != 64 {
		return fmt.Errorf("ContainerID must be in length of 64")
	}

	engine, ok := utils.FromDockerContext(ctx)
	if !ok {
		return fmt.Errorf("Not actually a `engineapi.Client` for value engine in context")
	}

	log.Debugf("[DisconnectFromNetwork] Disconnect %v from %v", containerID, networkID)
	return engine.NetworkDisconnect(context.Background(), networkID, containerID, false)
}

// list networks from context
func (t *titanium) ListNetworks(ctx context.Context, driver string) ([]*types.Network, error) {
	networks := []*types.Network{}
	engine, ok := utils.FromDockerContext(ctx)
	if !ok {
		return networks, fmt.Errorf("Not actually a `engineapi.Client` for value engine in context")
	}

	filters := enginefilters.NewArgs()
	if driver != "" {
		filters.Add("driver", driver)
	}
	ns, err := engine.NetworkList(context.Background(), enginetypes.NetworkListOptions{Filters: filters})
	if err != nil {
		return networks, err
	}

	for _, n := range ns {
		subnets := []string{}
		for _, config := range n.IPAM.Config {
			subnets = append(subnets, config.Subnet)
		}
		networks = append(networks, &types.Network{Name: n.Name, Subnets: subnets})
	}
	return networks, nil
}

//New a titanium obj
func New() *titanium {
	return &titanium{}
}
