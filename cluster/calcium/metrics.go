package calcium

import (
	"context"

	"github.com/sanity-io/litter"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/metrics"
	"github.com/projecteru2/core/types"
)

// InitMetrics .
func (c *Calcium) InitMetrics(ctx context.Context) {
	metricsDescriptions, err := c.rmgr.GetMetricsDescription(ctx)
	if err != nil {
		log.Errorf(ctx, err, "[InitMetrics] failed to get metrics description, err: %v", err)
		return
	}
	if err = metrics.InitMetrics(c.config, metricsDescriptions); err != nil {
		log.Errorf(ctx, err, "[InitMetrics] failed to init metrics, err: %v", err)
		return
	}
	log.Infof(ctx, "[InitMetrics] init metrics %v success", litter.Sdump(metricsDescriptions))
}

func (c *Calcium) doSendNodeMetrics(ctx context.Context, node *types.Node) {
	nodeMetrics, err := c.rmgr.GetNodeMetrics(ctx, node)
	if err != nil {
		log.Errorf(ctx, err, "[SendNodeMetrics] convert node %s resource info to metrics failed, %v", node.Name, err)
		return
	}
	metrics.Client.SendMetrics(nodeMetrics...)
}
