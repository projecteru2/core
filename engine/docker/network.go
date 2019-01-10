package docker

import (
	"context"
	"net"

	dockertypes "github.com/docker/docker/api/types"
	dockerfilters "github.com/docker/docker/api/types/filters"
	dockernetwork "github.com/docker/docker/api/types/network"

	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// NetworkConnect connect to a network
func (e *Engine) NetworkConnect(ctx context.Context, network, target, ipv4, ipv6 string) error {
	config := &dockernetwork.EndpointSettings{
		IPAMConfig: &dockernetwork.EndpointIPAMConfig{},
	}
	// set specified IP
	// but if IP is empty, just ignore
	if ipv4 != "" {
		ip := net.ParseIP(ipv4)
		if ip == nil {
			return coretypes.NewDetailedErr(coretypes.ErrBadIPAddress, ipv4)
		}

		config.IPAMConfig.IPv4Address = ip.String()
	}

	ipForShow := ipv4
	if ipForShow == "" {
		ipForShow = "[AutoAlloc]"
	}

	log.Infof("[ConnectToNetwork] Connect %v to %v with IP %v", target, network, ipForShow)
	return e.client.NetworkConnect(ctx, network, target, config)
}

// NetworkDisconnect disconnect from a network
func (e *Engine) NetworkDisconnect(ctx context.Context, network, target string, force bool) error {
	return e.client.NetworkDisconnect(ctx, network, target, force)
}

// NetworkList show all networks
func (e *Engine) NetworkList(ctx context.Context, drivers []string) ([]*enginetypes.Network, error) {
	networks := []*enginetypes.Network{}
	filters := dockerfilters.NewArgs()
	for _, driver := range drivers {
		filters.Add("driver", driver)
	}

	ns, err := e.client.NetworkList(ctx, dockertypes.NetworkListOptions{Filters: filters})
	if err != nil {
		return networks, err
	}

	for _, n := range ns {
		subnets := []string{}
		for _, config := range n.IPAM.Config {
			subnets = append(subnets, config.Subnet)
		}
		networks = append(networks, &enginetypes.Network{Name: n.Name, Subnets: subnets})
	}
	return networks, nil
}
