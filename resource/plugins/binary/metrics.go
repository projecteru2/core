package binary

import (
	"context"

	binarytypes "github.com/projecteru2/core/resource/plugins/binary/types"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
)

func (p Plugin) GetMetricsDescription(ctx context.Context) (*plugintypes.GetMetricsDescriptionResponse, error) {
	req := &binarytypes.GetMetricsDescriptionRequest{}
	resp := &plugintypes.GetMetricsDescriptionResponse{}
	return resp, p.call(ctx, GetMetricsDescriptionCommand, req, resp)
}

func (p Plugin) GetMetrics(ctx context.Context, nodes []plugintypes.NodeRef) (*plugintypes.GetMetricsResponse, error) {
	req := &binarytypes.GetMetricsRequest{Nodes: nodes}
	resp := &plugintypes.GetMetricsResponse{}
	return resp, p.call(ctx, GetMetricsCommand, req, resp)
}
