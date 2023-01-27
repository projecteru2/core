package binary

import (
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
	binarytypes "github.com/projecteru2/core/resource/plugins/binary/types"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
)

// AddNode .
func (p Plugin) AddNode(ctx context.Context, nodename string, resource *plugintypes.NodeResourceRequest, info *enginetypes.Info) (*plugintypes.AddNodeResponse, error) {
	req := &binarytypes.AddNodeRequest{
		Nodename: nodename,
		Resource: resource,
		Info:     info,
	}
	resp := &plugintypes.AddNodeResponse{}
	return resp, p.call(ctx, AddNodeCommand, req, resp)
}

// RemoveNode .
func (p Plugin) RemoveNode(ctx context.Context, nodename string) (*plugintypes.RemoveNodeResponse, error) {
	req := &binarytypes.RemoveNodeRequest{
		Nodename: nodename,
	}
	resp := &plugintypes.RemoveNodeResponse{}
	return resp, p.call(ctx, RemoveNodeCommand, req, resp)
}

// GetNodesDeployCapacity .
func (p Plugin) GetNodesDeployCapacity(ctx context.Context, nodenames []string, resource *plugintypes.WorkloadResourceRequest) (*plugintypes.GetNodesDeployCapacityResponse, error) {
	req := &binarytypes.GetNodesDeployCapacityRequest{
		Nodenames:        nodenames,
		WorkloadResource: resource,
	}
	resp := &plugintypes.GetNodesDeployCapacityResponse{}
	return resp, p.call(ctx, GetNodesDeployCapacityCommand, req, resp)
}

// SetNodeResourceCapacity .
func (p Plugin) SetNodeResourceCapacity(ctx context.Context, nodename string, resource *plugintypes.NodeResource, resourceRequest *plugintypes.NodeResourceRequest, delta bool, incr bool) (*plugintypes.SetNodeResourceCapacityResponse, error) {
	req := &binarytypes.SetNodeResourceCapacityRequest{
		Nodename:        nodename,
		Resource:        resource,
		ResourceRequest: resourceRequest,
		Delta:           delta,
		Incr:            incr,
	}

	resp := &plugintypes.SetNodeResourceCapacityResponse{}
	return resp, p.call(ctx, SetNodeResourceCapacityCommand, req, resp)
}

// GetNodeResourceInfo .
func (p Plugin) GetNodeResourceInfo(ctx context.Context, nodename string, workloadsResource []*plugintypes.WorkloadResource) (*plugintypes.GetNodeResourceInfoResponse, error) {
	return p.doGetNodeResourceInfo(ctx, nodename, workloadsResource, false) // Get do not fix the resource
}

// SetNodeResourceInfo .
func (p Plugin) SetNodeResourceInfo(ctx context.Context, nodename string, capacity *plugintypes.NodeResource, usage *plugintypes.NodeResource) (*plugintypes.SetNodeResourceInfoResponse, error) {
	req := &binarytypes.SetNodeResourceInfoRequest{
		Nodename: nodename,
		Capacity: capacity,
		Usage:    usage,
	}
	resp := &plugintypes.SetNodeResourceInfoResponse{}
	return resp, p.call(ctx, SetNodeResourceInfoCommand, req, resp)

}

// SetNodeResourceUsage .
func (p Plugin) SetNodeResourceUsage(ctx context.Context, nodename string, resource *plugintypes.NodeResource, resourceRequest *plugintypes.NodeResourceRequest, workloadsResource []*plugintypes.WorkloadResource, delta bool, incr bool) (*plugintypes.SetNodeResourceUsageResponse, error) {
	req := &binarytypes.SetNodeResourceUsageRequest{
		Nodename:          nodename,
		WorkloadsResource: workloadsResource,
		Resource:          resource,
		ResourceRequest:   resourceRequest,
		Delta:             delta,
		Incr:              incr,
	}

	resp := &plugintypes.SetNodeResourceUsageResponse{}
	return resp, p.call(ctx, SetNodeResourceUsageCommand, req, resp)
}

// GetMostIdleNode .
func (p Plugin) GetMostIdleNode(ctx context.Context, nodenames []string) (*plugintypes.GetMostIdleNodeResponse, error) {
	req := &binarytypes.GetMostIdleNodeRequest{
		Nodenames: nodenames,
	}
	resp := &plugintypes.GetMostIdleNodeResponse{}
	return resp, p.call(ctx, GetMostIdleNodeCommand, req, resp)
}

// FixNodeResource .
func (p Plugin) FixNodeResource(ctx context.Context, nodename string, workloadsResource []*plugintypes.WorkloadResource) (*plugintypes.GetNodeResourceInfoResponse, error) {
	return p.doGetNodeResourceInfo(ctx, nodename, workloadsResource, true)
}

func (p Plugin) doGetNodeResourceInfo(ctx context.Context, nodename string, workloadsResource []*plugintypes.WorkloadResource, fix bool) (*plugintypes.GetNodeResourceInfoResponse, error) {
	req := &binarytypes.GetNodeResourceInfoRequest{
		Nodename:          nodename,
		WorkloadsResource: workloadsResource,
	}
	resp := &plugintypes.GetNodeResourceInfoResponse{}
	if !fix {
		return resp, p.call(ctx, GetNodeResourceInfoCommand, req, resp)
	}
	return resp, p.call(ctx, FixNodeResourceCommand, req, resp)
}
