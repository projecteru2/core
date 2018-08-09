package rpc

import (
	"encoding/json"
	"errors"

	"github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

func toRPCCPUMap(m types.CPUMap) map[string]int32 {
	cpu := make(map[string]int32)
	for label, value := range m {
		cpu[label] = int32(value)
	}
	return cpu
}

func toRPCPod(p *types.Pod) *pb.Pod {
	return &pb.Pod{Name: p.Name, Desc: p.Desc}
}

func toRPCNetwork(n *types.Network) *pb.Network {
	return &pb.Network{Name: n.Name, Subnets: n.Subnets}
}

func toRPCNode(ctx context.Context, n *types.Node) *pb.Node {
	var nodeInfo string
	if info, err := n.Info(ctx); err == nil {
		bytes, _ := json.Marshal(info)
		nodeInfo = string(bytes)
	} else {
		nodeInfo = err.Error()
	}

	return &pb.Node{
		Name:      n.Name,
		Endpoint:  n.Endpoint,
		Podname:   n.Podname,
		Available: n.Available,
		Cpu:       toRPCCPUMap(n.CPU),
		Memory:    n.MemCap,
		Labels:    n.Labels,
		Info:      nodeInfo,
	}
}

func toRPCBuildImageMessage(b *types.BuildImageMessage) *pb.BuildImageMessage {
	return &pb.BuildImageMessage{
		Id:       b.ID,
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

func toCoreCopyOptions(b *pb.CopyOptions) *types.CopyOptions {
	r := &types.CopyOptions{Targets: map[string][]string{}}
	for cid, paths := range b.Targets {
		r.Targets[cid] = []string{}
		r.Targets[cid] = append(r.Targets[cid], paths.Paths...)
	}
	return r
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
		if p == nil {
			return nil, errors.New("no build spec")
		}
		builds.Builds[stage] = &types.Build{
			Base:      p.Base,
			Repo:      p.Repo,
			Version:   p.Version,
			Dir:       p.Dir,
			Submodule: p.Submodule || false,
			Commands:  p.Commands,
			Envs:      p.Envs,
			Args:      p.Args,
			Labels:    p.Labels,
			Artifacts: p.Artifacts,
			Cache:     p.Cache,
		}
	}
	return &types.BuildOptions{
		Name:   b.Name,
		User:   b.User,
		UID:    int(b.Uid),
		Tags:   b.Tags,
		Builds: builds,
	}, nil
}

func toCoreDeployOptions(d *pb.DeployOptions) (*types.DeployOptions, error) {
	if d.Entrypoint == nil {
		return nil, errors.New("no entry")
	}

	entrypoint := d.Entrypoint

	entry := &types.Entrypoint{
		Name:          entrypoint.Name,
		Command:       entrypoint.Command,
		Privileged:    entrypoint.Privileged,
		Dir:           entrypoint.Dir,
		Publish:       entrypoint.Publish,
		RestartPolicy: entrypoint.RestartPolicy,
	}

	if entrypoint.Log != nil && entrypoint.Log.Type != "" {
		entry.Log = &types.LogConfig{}
		entry.Log.Type = entrypoint.Log.Type
		entry.Log.Config = entrypoint.Log.Config
	}

	if entrypoint.Healthcheck != nil {
		entry.HealthCheck = &types.HealthCheck{}
		entry.HealthCheck.TCPPorts = entrypoint.Healthcheck.TcpPorts
		entry.HealthCheck.HTTPPort = entrypoint.Healthcheck.HttpPort
		entry.HealthCheck.HTTPURL = entrypoint.Healthcheck.Url
		entry.HealthCheck.HTTPCode = int(entrypoint.Healthcheck.Code)
	}

	if entrypoint.Hook != nil {
		entry.Hook = &types.Hook{}
		entry.Hook.AfterStart = entrypoint.Hook.AfterStart
		entry.Hook.BeforeStop = entrypoint.Hook.BeforeStop
		entry.Hook.Force = entrypoint.Hook.Force
	}

	return &types.DeployOptions{
		Name:         d.Name,
		Entrypoint:   entry,
		Podname:      d.Podname,
		Nodename:     d.Nodename,
		Image:        d.Image,
		ExtraArgs:    d.ExtraArgs,
		CPUQuota:     d.CpuQuota,
		Memory:       d.Memory,
		Count:        int(d.Count),
		Env:          d.Env,
		DNS:          d.Dns,
		ExtraHosts:   d.ExtraHosts,
		Volumes:      d.Volumes,
		Networks:     d.Networks,
		NetworkMode:  d.Networkmode,
		User:         d.User,
		Debug:        d.Debug,
		OpenStdin:    d.OpenStdin,
		Meta:         d.Meta,
		NodeLabels:   d.Nodelabels,
		DeployMethod: d.DeployMethod,
		Data:         d.Data,
		SoftLimit:    d.Softlimit,
	}, nil
}

func toRPCCreateContainerMessage(c *types.CreateContainerMessage) *pb.CreateContainerMessage {
	if c == nil {
		return nil
	}
	msg := &pb.CreateContainerMessage{
		Podname:  c.Podname,
		Nodename: c.Nodename,
		Id:       c.ContainerID,
		Name:     c.ContainerName,
		Success:  c.Success,
		Cpu:      toRPCCPUMap(c.CPU),
		Quota:    c.Quota,
		Memory:   c.Memory,
		Publish:  c.Publish,
		Hook:     c.Hook,
	}
	if c.Error != nil {
		msg.Error = c.Error.Error()
	}
	return msg
}

func toRPCReplaceContainerMessage(r *types.ReplaceContainerMessage) *pb.ReplaceContainerMessage {
	msg := &pb.ReplaceContainerMessage{
		Create: toRPCCreateContainerMessage(r.CreateContainerMessage),
		Id:     r.OldContainerID,
	}
	if r.Error != nil {
		msg.Error = r.Error.Error()
	}
	return msg
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

func toRPCRunAndWaitMessage(msg *types.RunAndWaitMessage) *pb.RunAndWaitMessage {
	return &pb.RunAndWaitMessage{
		ContainerId: msg.ContainerID,
		Data:        msg.Data,
	}
}

func toRPCContainers(ctx context.Context, containers []*types.Container) []*pb.Container {
	cs := []*pb.Container{}
	for _, c := range containers {
		pContainer, err := toRPCContainer(ctx, c)
		if err != nil {
			log.Errorf("[toRPCContainers] trans to pb container failed %v", err)
			continue
		}

		cs = append(cs, pContainer)
	}
	return cs
}

func toRPCContainer(ctx context.Context, c *types.Container) (*pb.Container, error) {
	info, err := c.Inspect(ctx)
	if err != nil {
		return nil, err
	}

	bytes, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}

	cpu := toRPCCPUMap(c.CPU)

	return &pb.Container{
		Id:         c.ID,
		Podname:    c.Podname,
		Nodename:   c.Nodename,
		Name:       c.Name,
		Cpu:        cpu,
		Quota:      c.Quota,
		Memory:     c.Memory,
		Privileged: c.Privileged,
		Inspect:    bytes}, nil
}
