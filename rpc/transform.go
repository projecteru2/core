package rpc

import (
	"encoding/json"
	"errors"

	"github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
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
	var nodeInfo string
	if info, err := n.Info(); err == nil {
		bytes, _ := json.Marshal(info)
		nodeInfo = string(bytes)
	} else {
		nodeInfo = err.Error()
	}

	return &pb.Node{
		Name:      n.Name,
		Endpoint:  n.Endpoint,
		Podname:   n.Podname,
		Public:    n.Public,
		Cpu:       toRPCCPUMap(n.CPU),
		Info:      nodeInfo,
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

func toCoreBuildOptions(b *pb.BuildImageOptions) (*types.BuildOptions, error) {
	if b.Builds == nil || len(b.Builds.Stages) == 0 {
		return nil, errors.New("no builds")
	}
	builds := &types.Builds{
		Stages: b.Builds.Stages,
	}
	builds.Builds = map[string]*types.Build{}
	for stage, p := range b.Builds.Builds {
		builds.Builds[stage] = &types.Build{
			Base:       p.Base,
			Repo:       p.Repo,
			Version:    p.Version,
			WorkingDir: p.WorkingDir,
			Commands:   p.Commands,
			Artifacts:  p.Artifacts,
			Cache:      p.Cache,
		}
	}
	return &types.BuildOptions{
		Name:   b.Name,
		User:   b.User,
		UID:    int(b.Uid),
		Tag:    b.Tag,
		Builds: builds,
	}, nil
}

func toCoreDeployOptions(d *pb.DeployOptions) (*types.DeployOptions, error) {
	if d.Entrypoint == nil {
		return nil, errors.New("no entry")
	}

	entrypoint := d.Entrypoint

	// covert ports
	ports := []types.Port{}
	for _, port := range entrypoint.Ports {
		ports = append(ports, types.Port(port))
	}

	hook := &types.Hook{}
	if entrypoint.Hook != nil {
		hook.AfterStart = entrypoint.Hook.AfterStart
		hook.BeforeStop = entrypoint.Hook.BeforeStop
	}

	healthcheck := &types.HealthCheck{}
	if entrypoint.Healcheck != nil {
		healthcheck.Port = int(entrypoint.Healcheck.Port)
		healthcheck.URL = entrypoint.Healcheck.Url
		healthcheck.Code = int(entrypoint.Healcheck.Code)
	}

	entry := &types.Entrypoint{
		Name:          entrypoint.Name,
		Command:       entrypoint.Command,
		Privileged:    entrypoint.Privileged,
		WorkingDir:    entrypoint.WorkingDir,
		LogConfig:     entrypoint.LogConfig,
		Ports:         ports,
		HealthCheck:   healthcheck,
		Hook:          hook,
		RestartPolicy: entrypoint.RestartPolicy,
		ExtraHosts:    entrypoint.ExtraHosts,
	}
	return &types.DeployOptions{
		Name:        d.Name,
		Entrypoint:  entry,
		Podname:     d.Podname,
		Nodename:    d.Nodename,
		Image:       d.Image,
		ExtraArgs:   d.ExtraArgs,
		CPUQuota:    d.CpuQuota,
		Memory:      d.Memory,
		Count:       int(d.Count),
		Env:         d.Env,
		DNS:         d.Dns,
		Volumes:     d.Volumes,
		Networks:    d.Networks,
		NetworkMode: d.Networkmode,
		User:        d.User,
		Debug:       d.Debug,
		OpenStdin:   d.OpenStdin,
		Meta:        d.Meta,
	}, nil
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
		Memory:   c.Memory,
		Ips:      c.IPs,
	}
}

func toRPCRemoveImageMessage(r *types.RemoveImageMessage) *pb.RemoveImageMessage {
	return &pb.RemoveImageMessage{
		Image:    r.Image,
		Success:  r.Success,
		Messages: r.Messages,
	}
}

func toRPCReallocResourceMessage(r *types.ReallocResourceMessage) *pb.ReallocResourceMessage {
	return &pb.ReallocResourceMessage{
		Id:      r.ContainerID,
		Success: r.Success,
	}
}

func toRPCRemoveContainerMessage(r *types.RemoveContainerMessage) *pb.RemoveContainerMessage {
	return &pb.RemoveContainerMessage{
		Id:      r.ContainerID,
		Success: r.Success,
		Message: r.Message,
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
