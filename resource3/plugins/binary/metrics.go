package binary

import (
	"context"

	binarytypes "github.com/projecteru2/core/resource3/plugins/binary/types"
	plugintypes "github.com/projecteru2/core/resource3/plugins/types"
)

// GetMetricsDescription .
func (p Plugin) GetMetricsDescription(ctx context.Context) (*plugintypes.GetMetricsDescriptionResponse, error) {
	req := &binarytypes.GetMetricsDescriptionRequest{}
	resp := &plugintypes.GetMetricsDescriptionResponse{}
	return resp, p.call(ctx, GetMetricsDescriptionCommand, req, resp)
}

// GetMetrics .
func (p Plugin) GetMetrics(ctx context.Context, podname, nodename string) (*plugintypes.GetMetricsResponse, error) {
	req := &binarytypes.GetMetricsRequest{
		Podname:  podname,
		Nodename: nodename,
	}
	resp := &plugintypes.GetMetricsResponse{}
	return resp, p.call(ctx, GetMetricsCommand, req, resp)
}
