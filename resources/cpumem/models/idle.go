package models

import (
	"context"
	"math"

	"github.com/sirupsen/logrus"
)

const priority = 100

// GetMostIdleNode .
func (c *CPUMem) GetMostIdleNode(ctx context.Context, nodes []string) (string, int, error) {
	var mostIdleNode string
	var minIdle = math.MaxFloat64

	for _, node := range nodes {
		resourceInfo, err := c.doGetNodeResourceInfo(ctx, node)
		if err != nil {
			logrus.Errorf("[GetMostIdleNode] failed to get node resource info")
			return "", 0, err
		}
		idle := float64(resourceInfo.Usage.CPUMap.TotalPieces()) / float64(resourceInfo.Capacity.CPUMap.TotalPieces())
		idle += float64(resourceInfo.Usage.Memory) / float64(resourceInfo.Capacity.Memory)

		if idle < minIdle {
			mostIdleNode = node
			minIdle = idle
		}
	}

	return mostIdleNode, priority, nil
}