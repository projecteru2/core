package docker

import (
	"context"
	"net"

	"github.com/cockroachdb/errors"
	dockerfilters "github.com/docker/docker/api/types/filters"
	dockernetwork "github.com/docker/docker/api/types/network"

	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

func (e *Engine) NetworkConnect(ctx context.Context, network, target, ipv4, _ string) ([]string, error) {
	config, err := e.makeIPV4EndpointSetting(ipv4)
	if err != nil {
		return nil, err
	}
	if err = e.client.NetworkConnect(ctx, network, target, config); err != nil {
		return nil, err
	}
	workload, err := e.client.ContainerInspect(ctx, target)
	if err != nil {
		return nil, err
	}
	ns := workload.NetworkSettings.Networks[network]
	if ns == nil {
		return []string{}, nil
	}
	return []string{ns.IPAddress}, nil
}

func (e *Engine) NetworkDisconnect(ctx context.Context, network, target string, force bool) error {
	return e.client.NetworkDisconnect(ctx, network, target, force)
}

func (e *Engine) NetworkList(ctx context.Context, drivers []string) ([]*enginetypes.Network, error) {
	networks := []*enginetypes.Network{}
	filters := dockerfilters.NewArgs()
	for _, driver := range drivers {
		filters.Add("driver", driver)
	}

	ns, err := e.client.NetworkList(ctx, dockernetwork.ListOptions{Filters: filters})
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

func (e *Engine) makeIPV4EndpointSetting(ipv4 string) (*dockernetwork.EndpointSettings, error) {
	config := &dockernetwork.EndpointSettings{
		IPAMConfig: &dockernetwork.EndpointIPAMConfig{},
	}
	if ipv4 != "" {
		ip := net.ParseIP(ipv4)
		if ip == nil {
			return nil, errors.Wrapf(coretypes.ErrInvaildIPAddress, "ip: %s", ipv4)
		}
		config.IPAMConfig.IPv4Address = ip.String()
	}
	return config, nil
}
