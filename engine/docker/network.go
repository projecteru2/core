package docker

import (
	"context"
	"net/netip"

	"github.com/cockroachdb/errors"
	dockernetwork "github.com/moby/moby/api/types/network"
	dockerapi "github.com/moby/moby/client"

	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

func (e *Engine) NetworkConnect(ctx context.Context, network, target, ipv4, _ string) ([]string, error) {
	config, err := e.makeIPV4EndpointSetting(ipv4)
	if err != nil {
		return nil, err
	}
	if _, err = e.client.NetworkConnect(ctx, network, dockerapi.NetworkConnectOptions{Container: target, EndpointConfig: config}); err != nil {
		return nil, err
	}
	workload, err := e.client.ContainerInspect(ctx, target, dockerapi.ContainerInspectOptions{})
	if err != nil {
		return nil, err
	}
	ns := workload.Container.NetworkSettings.Networks[network]
	if ns == nil {
		return []string{}, nil
	}
	return []string{zeroToEmpty(ns.IPAddress)}, nil
}

func (e *Engine) NetworkDisconnect(ctx context.Context, network, target string, force bool) error {
	_, err := e.client.NetworkDisconnect(ctx, network, dockerapi.NetworkDisconnectOptions{Container: target, Force: force})
	return err
}

func (e *Engine) NetworkList(ctx context.Context, drivers []string) ([]*enginetypes.Network, error) {
	networks := []*enginetypes.Network{}
	filters := dockerapi.Filters{}
	for _, driver := range drivers {
		filters.Add("driver", driver)
	}

	ns, err := e.client.NetworkList(ctx, dockerapi.NetworkListOptions{Filters: filters})
	if err != nil {
		return networks, err
	}

	for _, n := range ns.Items {
		subnets := []string{}
		for _, config := range n.IPAM.Config {
			subnets = append(subnets, zeroToEmpty(config.Subnet))
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
		ip, err := netip.ParseAddr(ipv4)
		if err != nil {
			return nil, errors.Wrapf(coretypes.ErrInvaildIPAddress, "ip: %s", ipv4)
		}
		config.IPAMConfig.IPv4Address = ip
	}
	return config, nil
}
