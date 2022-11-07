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
	logger := log.WithFunc("calcium.InitMetrics")
	metricsDescriptions, err := c.rmgr.GetMetricsDescription(ctx)
	if err != nil {
		logger.Error(ctx, err, "failed to get metrics description")
		return
	}
	if err = metrics.InitMetrics(c.config, metricsDescriptions); err != nil {
		logger.Error(ctx, err, "failed to init metrics")
		return
	}
	logger.Infof(ctx, "init metrics success \n%+v", litter.Sdump(metricsDescriptions))
}

func (c *Calcium) doSendNodeMetrics(ctx context.Context, node *types.Node) {
	nodeMetrics, err := c.rmgr.GetNodeMetrics(ctx, node)
	if err != nil {
		log.WithFunc("calcium.doSendNodeMetrics").Errorf(ctx, err, "convert node %s resource info to metrics failed", node.Name)
		return
	}
	metrics.Client.SendMetrics(ctx, nodeMetrics...)
}
