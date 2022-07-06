package calcium

import (
	"context"

	"github.com/sanity-io/litter"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/metrics"
	"github.com/projecteru2/core/utils"
)

// InitMetrics .
func (c *Calcium) InitMetrics() {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	metricsDescriptions, err := c.rmgr.GetMetricsDescription(ctx)
	if err != nil {
		log.Errorf(ctx, "[InitMetrics] failed to get metrics description, err: %v", err)
		return
	}
	if err = metrics.InitMetrics(c.config, metricsDescriptions); err != nil {
		log.Errorf(ctx, "[InitMetrics] failed to init metrics, err: %v", err)
		return
	}
	log.Infof(ctx, "[InitMetrics] init metrics %v success", litter.Sdump(metricsDescriptions))
}

// SendNodeMetrics .
func (c *Calcium) SendNodeMetrics(ctx context.Context, nodeName string) {
	ctx, cancel := context.WithTimeout(utils.InheritTracingInfo(ctx, context.TODO()), c.config.GlobalTimeout)
	defer cancel()
	node, err := c.GetNode(ctx, nodeName, nil)
	if err != nil {
		log.Errorf(ctx, "[SendNodeMetrics] get node %s failed, %v", nodeName, err)
		return
	}

	nodeMetrics, err := c.rmgr.ConvertNodeResourceInfoToMetrics(ctx, node.Podname, node.Name, node.ResourceCapacity, node.ResourceUsage)
	if err != nil {
		log.Errorf(ctx, "[SendNodeMetrics] resolve node %s resource info to metrics failed, %v", nodeName, err)
		return
	}
	metrics.Client.SendMetrics(nodeMetrics...)
}
