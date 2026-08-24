package calcium

import (
	"context"

	"github.com/cockroachdb/errors"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

func (c *Calcium) ListNetworks(ctx context.Context, podname, driver string) ([]*enginetypes.Network, error) {
	logger := log.WithFunc("calcium.ListNetworks").WithField("podname", podname).WithField("driver", driver)
	networks := []*enginetypes.Network{}
	nodes, err := c.store.GetNodesByPod(ctx, &types.NodeFilter{Podname: podname})
	if err != nil {
		logger.Error(ctx, err)
		return networks, err
	}

	if len(nodes) == 0 {
		err = errors.Wrapf(types.ErrPodNoNodes, "pod: %s", podname)
		logger.Error(ctx, err)
		return networks, err
	}

	drivers := []string{}
	if driver != "" {
		drivers = append(drivers, driver)
	}

	// every node of a pod reports the same networks
	node := nodes[0]

	networks, err = node.Engine.NetworkList(ctx, drivers)
	logger.Error(ctx, err)
	return networks, err
}

func (c *Calcium) ConnectNetwork(ctx context.Context, network, target, ipv4, ipv6 string) ([]string, error) {
	logger := log.WithFunc("calcium.ConnectNetwork").WithField("network", network).WithField("target", target).WithField("ipv4", ipv4).WithField("ipv6", ipv6)
	workload, err := c.GetWorkload(ctx, target)
	if err != nil {
		return nil, err
	}

	networks, err := workload.Engine.NetworkConnect(ctx, network, target, ipv4, ipv6)
	logger.Error(ctx, err)
	return networks, err
}

func (c *Calcium) DisconnectNetwork(ctx context.Context, network, target string, force bool) error {
	logger := log.WithFunc("calcium.DisconnectNetwork").WithField("network", network).WithField("target", target).WithField("force", force)
	workload, err := c.GetWorkload(ctx, target)
	if err != nil {
		logger.Error(ctx, err)
		return err
	}
	if err = workload.Engine.NetworkDisconnect(ctx, network, target, force); err != nil {
		logger.Error(ctx, err)
	}
	return err
}
