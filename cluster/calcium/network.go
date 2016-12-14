package calcium

import (
	"fmt"

	"gitlab.ricebook.net/platform/core/types"
	"gitlab.ricebook.net/platform/core/utils"
)

// list networks for podname
// just get one node from podname
// and call docker network ls
// only get those driven by network driver
func (c *calcium) ListNetworks(podname string) ([]*types.Network, error) {
	networks := []*types.Network{}
	nodes, err := c.ListPodNodes(podname, false)
	if err != nil {
		return networks, err
	}

	if len(nodes) == 0 {
		return networks, fmt.Errorf("Pod %s has no nodes", podname)
	}

	node := nodes[0]
	ctx := utils.ToDockerContext(node.Engine)
	return c.network.ListNetworks(ctx)
}
