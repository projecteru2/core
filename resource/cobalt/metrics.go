package cobalt

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	"github.com/projecteru2/core/types"
)

func (m Manager) GetMetricsDescription(ctx context.Context) ([]*plugintypes.MetricsDescription, error) {
	var metricsDescriptions []*plugintypes.MetricsDescription
	resps, err := call(ctx, m.plugins, func(plugin plugins.Plugin) (*plugintypes.GetMetricsDescriptionResponse, error) {
		resp, err := plugin.GetMetricsDescription(ctx)
		if err != nil {
			log.Errorf(ctx, err, "plugin %+v failed to get metrics description", plugin.Name())
		}
		return resp, err
	})

	if err != nil {
		log.Error(ctx, err, "failed to get metrics description")
		return nil, err
	}

	for _, resp := range resps {
		metricsDescriptions = append(metricsDescriptions, *resp...)
	}

	return metricsDescriptions, nil
}

func (m Manager) GetNodeMetrics(ctx context.Context, node *types.Node) ([]*plugintypes.Metrics, error) {
	logger := log.WithFunc("resource.cobalt.GetNodeMetrics").WithField("node", node.Name)

	var metrics []*plugintypes.Metrics
	resps, err := call(ctx, m.plugins, func(plugin plugins.Plugin) (*plugintypes.GetMetricsResponse, error) {
		resp, err := plugin.GetMetrics(ctx, node.Podname, node.Name)
		if err != nil {
			logger.Errorf(ctx, err, "plugin %+v failed to convert node resource info to metrics", plugin.Name())
		}
		return resp, err
	})

	if err != nil {
		logger.Error(ctx, err, "failed to convert node resource info to metrics")
		return nil, err
	}

	for _, resp := range resps {
		metrics = append(metrics, *resp...)
	}

	return metrics, nil
}
