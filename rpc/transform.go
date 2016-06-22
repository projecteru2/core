package rpc

import (
	"gitlab.ricebook.net/platform/core/rpc/gen"
	"gitlab.ricebook.net/platform/core/types"
)

func toRPCCPUMap(m types.CPUMap) map[string]int64 {
	cpu := make(map[string]int64)
	for label, value := range m {
		cpu[label] = int64(value)
	}
	return cpu
}

func toRPCPod(p *types.Pod) *pb.Pod {
	return &pb.Pod{Name: p.Name, Desc: p.Desc}
}

func toRPCNode(n *types.Node) *pb.Node {
	return &pb.Node{
		Name:     n.Name,
		Endpoint: n.Endpoint,
		Podname:  n.Podname,
		Public:   n.Public,
		Cpu:      toRPCCPUMap(n.CPU),
	}
}

func toRPCContainer(c *types.Container, info string) *pb.Container {
	return &pb.Container{Id: c.ID, Podname: c.Podname, Nodename: c.Nodename, Name: c.Name, Info: info}
}

func toRPCBuildImageMessage(b *types.BuildImageMessage) *pb.BuildImageMessage {
	return &pb.BuildImageMessage{
		Status:   b.Status,
		Progress: b.Progress,
		Error:    b.Error,
		Stream:   b.Stream,
		ErrorDetail: &pb.ErrorDetail{
			Code:    int64(b.ErrorDetail.Code),
			Message: b.ErrorDetail.Message,
		},
	}
}

func toCoreDeployOptions(d *pb.DeployOptions) *types.DeployOptions {
	return &types.DeployOptions{
		Appname:    d.Appname,
		Image:      d.Image,
		Podname:    d.Podname,
		Nodename:   d.Nodename,
		Entrypoint: d.Entrypoint,
		CPUQuota:   d.CpuQuota,
		Count:      int(d.Count),
		Env:        d.Env,
		Networks:   d.Networks,
		Raw:        d.Raw,
	}
}

func toRPCCreateContainerMessage(c *types.CreateContainerMessage) *pb.CreateContainerMessage {
	return &pb.CreateContainerMessage{
		Podname:  c.Podname,
		Nodename: c.Nodename,
		Id:       c.ContainerID,
		Name:     c.ContainerName,
		Success:  c.Success,
		Cpu:      toRPCCPUMap(c.CPU),
	}
}

func toRPCRemoveImageMessage(r *types.RemoveImageMessage) *pb.RemoveImageMessage {
	return &pb.RemoveImageMessage{
		Image:    r.Image,
		Success:  r.Success,
		Messages: r.Messages,
	}
}

func toRPCRemoveContainerMessage(r *types.RemoveContainerMessage) *pb.RemoveContainerMessage {
	return &pb.RemoveContainerMessage{
		Id:      r.ContainerID,
		Success: r.Success,
		Message: r.Message,
	}
}
