package rpc

import (
	"gitlab.ricebook.net/platform/core/rpc/gen"
	"gitlab.ricebook.net/platform/core/types"
)

func toRPCPod(p *types.Pod) *pb.Pod {
	return &pb.Pod{Name: p.Name, Desc: p.Desc}
}

func toRPCNode(n *types.Node) *pb.Node {
	cpu := make(map[string]int64)
	for label, value := range n.CPU {
		cpu[label] = int64(value)
	}

	return &pb.Node{
		Name:     n.Name,
		Endpoint: n.Endpoint,
		Podname:  n.Podname,
		Public:   n.Public,
		Cpu:      cpu,
	}
}

func toRPCContainer(c *types.Container) *pb.Container {
	return &pb.Container{Id: c.ID, Podname: c.Podname, Nodename: c.Nodename, Name: c.Name}
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
