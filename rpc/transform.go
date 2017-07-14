package rpc

import (
	"encoding/json"

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

func toRPCNetwork(n *types.Network) *pb.Network {
	return &pb.Network{Name: n.Name, Subnets: n.Subnets}
}

func toRPCNode(n *types.Node, zone string) *pb.Node {
	bytes := []byte("")
	if info, err := n.Info(); err == nil {
		bytes, _ = json.Marshal(info)
	}

	return &pb.Node{
		Name:      n.Name,
		Endpoint:  n.Endpoint,
		Podname:   n.Podname,
		Public:    n.Public,
		Cpu:       toRPCCPUMap(n.CPU),
		Info:      string(bytes),
		Available: n.Available,
		Zone:      zone,
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
		ExtraArgs:  d.ExtraArgs,
		CPUQuota:   d.CpuQuota,
		Count:      d.Count,
		Memory:     d.Memory,
		Env:        d.Env,
		Networks:   d.Networks,
		Raw:        d.Raw,
		Debug:      d.Debug,
	}
}

func toRPCCreateContainerMessage(c *types.CreateContainerMessage) *pb.CreateContainerMessage {
	return &pb.CreateContainerMessage{
		Podname:  c.Podname,
		Nodename: c.Nodename,
		Id:       c.ContainerID,
		Name:     c.ContainerName,
		Error:    c.Error,
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

func toRPCUpgradeContainerMessage(u *types.UpgradeContainerMessage) *pb.UpgradeContainerMessage {
	return &pb.UpgradeContainerMessage{
		Id:      u.ContainerID,
		NewId:   u.NewContainerID,
		NewName: u.NewContainerName,
		Error:   u.Error,
		Success: u.Success,
	}
}

func toRPCBackupMessage(msg *types.BackupMessage) *pb.BackupMessage {
	return &pb.BackupMessage{
		Status: msg.Status,
		Size:   msg.Size,
		Error:  msg.Error,
	}
}

func toRPCRunAndWaitMessage(msg *types.RunAndWaitMessage) *pb.RunAndWaitMessage {
	return &pb.RunAndWaitMessage{
		ContainerId: msg.ContainerID,
		Data:        msg.Data,
	}
}
