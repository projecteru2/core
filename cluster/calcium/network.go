package calcium

import (
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
)

// ListNetworks by podname
// get one node from a pod
// and list networks
// only get those driven by network driver
func (c *Calcium) ListNetworks(ctx context.Context, podname string, driver string) ([]*enginetypes.Network, error) {
	networks := []*enginetypes.Network{}
	nodes, err := c.ListPodNodes(ctx, podname, false)
	if err != nil {
		return networks, err
	}

	if len(nodes) == 0 {
		return networks, types.NewDetailedErr(types.ErrPodNoNodes, podname)
	}

	drivers := []string{}
	if driver != "" {
		drivers = append(drivers, driver)
	}

	node := nodes[0]
	return node.Engine.NetworkList(ctx, drivers)
}
