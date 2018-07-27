package calcium

import (
	"context"

	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

//ListNetworks by podname
// just get one node from podname
// and call docker network ls
// only get those driven by network driver
func (c *Calcium) ListNetworks(ctx context.Context, podname string, driver string) ([]*types.Network, error) {
	networks := []*types.Network{}
	nodes, err := c.ListPodNodes(ctx, podname, false)
	if err != nil {
		return networks, err
	}

	if len(nodes) == 0 {
		return networks, types.NewDetailedErr(types.ErrPodNoNodes, podname)
	}

	node := nodes[0]
	return c.network.ListNetworks(utils.ContextWithDockerEngine(ctx, node.Engine), driver)
}
