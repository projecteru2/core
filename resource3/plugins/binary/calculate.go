package binary

import (
	"context"

	binarytypes "github.com/projecteru2/core/resource3/plugins/binary/types"
	plugintypes "github.com/projecteru2/core/resource3/plugins/types"
)

func (p Plugin) CalculateDeploy(ctx context.Context, nodename string, deployCount int, resourceRequest *plugintypes.WorkloadResourceRequest) (*plugintypes.CalculateDeployResponse, error) {
	req := &binarytypes.CalculateDeployRequest{
		Nodename:                nodename,
		DeployCount:             deployCount,
		WorkloadResourceRequest: resourceRequest,
	}
	resp := &plugintypes.CalculateDeployResponse{}
	return resp, p.call(ctx, CalculateDeployCommand, req, resp)
}

func (p Plugin) CalculateRealloc(ctx context.Context, nodename string, resource *plugintypes.WorkloadResource, resourceRequest *plugintypes.WorkloadResourceRequest) (*plugintypes.CalculateReallocResponse, error) {
	req := &binarytypes.CalculateReallocRequest{
		Nodename:                nodename,
		WorkloadResource:        resource,
		WorkloadResourceRequest: resourceRequest,
	}
	resp := &plugintypes.CalculateReallocResponse{}
	return resp, p.call(ctx, CalculateReallocCommand, req, resp)
}

func (p Plugin) CalculateRemap(ctx context.Context, nodename string, workloadsResource map[string]*plugintypes.WorkloadResource) (*plugintypes.CalculateRemapResponse, error) {
	req := &binarytypes.CalculateRemapRequest{
		Nodename:          nodename,
		WorkloadsResource: workloadsResource,
	}
	resp := &plugintypes.CalculateRemapResponse{}
	return resp, p.call(ctx, CalculateRemapCommand, req, resp)
}
