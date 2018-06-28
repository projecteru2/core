package calcium

import (
	"context"
	"fmt"

	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

//ListNetworks by podname
// just get one node from podname
// and call docker network ls
// only get those driven by network driver
func (c *Calcium) ListNetworks(podname string, driver string) ([]*types.Network, error) {
	networks := []*types.Network{}
	nodes, err := c.ListPodNodes(podname, false)
	if err != nil {
		return networks, err
	}

	if len(nodes) == 0 {
		return networks, fmt.Errorf("Pod %s has no nodes", podname)
	}

	node := nodes[0]
	ctx := utils.ContextWithDockerEngine(context.Background(), node.Engine)
	return c.network.ListNetworks(ctx, driver)
}
