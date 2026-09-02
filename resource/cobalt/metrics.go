package cobalt

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (m *Manager) GetMetricsDescription(ctx context.Context) ([]*plugintypes.MetricsDescription, error) {
	var metricsDescriptions []*plugintypes.MetricsDescription
	resps, err := call(ctx, m.plugins, func(plugin plugins.Plugin) (*plugintypes.GetMetricsDescriptionResponse, error) {
		return plugin.GetMetricsDescription(ctx)
	})
	if err != nil {
		return nil, err
	}

	for _, resp := range resps {
		metricsDescriptions = append(metricsDescriptions, *resp...)
	}

	return metricsDescriptions, nil
}

func (m *Manager) GetNodesMetrics(ctx context.Context, nodes []*types.Node) ([]*plugintypes.Metrics, error) {
	if len(nodes) == 0 {
		return nil, nil
	}
	refs := utils.Map(nodes, func(node *types.Node) plugintypes.NodeRef {
		return plugintypes.NodeRef{Podname: node.Podname, Nodename: node.Name}
	})

	var metrics []*plugintypes.Metrics
	resps, err := call(ctx, m.plugins, func(plugin plugins.Plugin) (*plugintypes.GetMetricsResponse, error) {
		return plugin.GetMetrics(ctx, refs)
	})
	if err != nil {
		log.WithFunc("resource.cobalt.GetNodesMetrics").Error(ctx, err, "failed to convert node resource info to metrics")
		return nil, err
	}

	for _, resp := range resps {
		metrics = append(metrics, *resp...)
	}

	return metrics, nil
}
