package sdn

import (
	"context"
	"fmt"
	"net"

	enginetypes "github.com/docker/docker/api/types"
	enginefilters "github.com/docker/docker/api/types/filters"
	enginenetwork "github.com/docker/docker/api/types/network"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

type titanium struct{}

// connect to network with ipv4 address
func (t *titanium) ConnectToNetwork(ctx context.Context, containerID, networkID, ipv4 string) error {
	if len(containerID) != 64 {
		return types.ErrBadContainerID
	}

	engine, ok := utils.GetDockerEngineFromContext(ctx)
	if !ok {
		return types.NewDetailedErr(types.ErrCannotGetEngine,
			fmt.Sprintf("Not actually a `engineapi.Client` for value engine in context"))
	}

	config := &enginenetwork.EndpointSettings{
		IPAMConfig: &enginenetwork.EndpointIPAMConfig{},
	}

	// set specified IP
	// but if IP is empty, just ignore
	if ipv4 != "" {
		ip := net.ParseIP(ipv4)
		if ip == nil {
			return types.NewDetailedErr(types.ErrBadIPAddress, ipv4)
		}

		config.IPAMConfig.IPv4Address = ip.String()
	}

	log.Debugf("[ConnectToNetwork] Connect %v to %v with IP %v", containerID, networkID, ipv4)
	return engine.NetworkConnect(ctx, networkID, containerID, config)
}

// disconnect from network
func (t *titanium) DisconnectFromNetwork(ctx context.Context, containerID, networkID string) error {
	if len(containerID) != 64 {
		return types.ErrBadContainerID
	}

	engine, ok := utils.GetDockerEngineFromContext(ctx)
	if !ok {
		return types.NewDetailedErr(types.ErrCannotGetEngine,
			fmt.Sprintf("Not actually a `engineapi.Client` for value engine in context"))
	}

	log.Debugf("[DisconnectFromNetwork] Disconnect %v from %v", containerID, networkID)
	return engine.NetworkDisconnect(ctx, networkID, containerID, false)
}

// list networks from context
func (t *titanium) ListNetworks(ctx context.Context, driver string) ([]*types.Network, error) {
	networks := []*types.Network{}
	engine, ok := utils.GetDockerEngineFromContext(ctx)
	if !ok {
		return networks, types.NewDetailedErr(types.ErrCannotGetEngine,
			fmt.Sprintf("Not actually a `engineapi.Client` for value engine in context"))
	}

	filters := enginefilters.NewArgs()
	if driver != "" {
		filters.Add("driver", driver)
	}
	ns, err := engine.NetworkList(ctx, enginetypes.NetworkListOptions{Filters: filters})
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
