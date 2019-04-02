package rpc

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"

	enginetypes "github.com/projecteru2/core/engine/types"
	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
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
	return &pb.Pod{Name: p.Name, Desc: p.Desc, Favor: p.Favor}
}

func toRPCPodResource(p *types.PodResource) *pb.PodResource {
	r := &pb.PodResource{
		Name:   p.Name,
		Cpu:    p.CPUPercent,
		Memory: p.MEMPercent,
		Diff:   p.Diff,
		Detail: p.Detail,
	}
	return r
}

func toRPCNetwork(n *enginetypes.Network) *pb.Network {
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
		Name:       n.Name,
		Endpoint:   n.Endpoint,
		Podname:    n.Podname,
		Cpu:        toRPCCPUMap(n.CPU),
		CpuUsed:    n.CPUUsed,
		Memory:     n.MemCap,
		MemoryUsed: n.InitMemCap - n.MemCap,
		Available:  n.Available,
		Labels:     n.Labels,
		InitCpu:    toRPCCPUMap(n.InitCPU),
		InitMemory: n.InitMemCap,
		Info:       nodeInfo,
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

func toCoreBuildOptions(b *pb.BuildImageOptions) (*enginetypes.BuildOptions, error) {
	var builds *enginetypes.Builds
	if b.Builds != nil {
		if len(b.Builds.Stages) == 0 {
			return nil, types.ErrNoBuildsInSpec
		}
		builds = &enginetypes.Builds{
			Stages: b.Builds.Stages,
		}
		builds.Builds = map[string]*enginetypes.Build{}
		for stage, p := range b.Builds.Builds {
			if p == nil {
				return nil, types.ErrNoBuildSpec
			}
			builds.Builds[stage] = &enginetypes.Build{
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
	}
	return &enginetypes.BuildOptions{
		Name:   b.Name,
		User:   b.User,
		UID:    int(b.Uid),
		Tags:   b.Tags,
		Builds: builds,
		Tar:    ioutil.NopCloser(bytes.NewReader(b.Tar)),
	}, nil
}

func toCoreReplaceOptions(r *pb.ReplaceOptions) (*types.ReplaceOptions, error) {
	deployOpts, err := toCoreDeployOptions(r.DeployOpt)

	replaceOpts := &types.ReplaceOptions{
		DeployOptions:  *deployOpts,
		Force:          r.Force,
		FilterLabels:   r.FilterLabels,
		Copy:           r.Copy,
		IDs:            r.Ids,
		NetworkInherit: r.Networkinherit,
	}

	return replaceOpts, err
}

func toCoreDeployOptions(d *pb.DeployOptions) (*types.DeployOptions, error) {
	if d.Entrypoint == nil {
		return nil, types.ErrNoEntryInSpec
	}

	entrypoint := d.Entrypoint

	entry := &types.Entrypoint{
		Name:          entrypoint.Name,
		Command:       entrypoint.Command,
		Privileged:    entrypoint.Privileged,
		Dir:           entrypoint.Dir,
		Publish:       entrypoint.Publish,
		RestartPolicy: entrypoint.RestartPolicy,
		Sysctls:       entrypoint.Sysctls,
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

	tarFiles := map[string]string{}
	for path, data := range d.Data {
		fname, err := utils.TempTarFile(path, data)
		if err != nil {
			if fname != "" {
				os.RemoveAll(fname)
			}
			return nil, err
		}
		tarFiles[path] = fname
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
		Labels:       d.Labels,
		NodeLabels:   d.Nodelabels,
		DeployMethod: d.DeployMethod,
		Data:         tarFiles,
		SoftLimit:    d.Softlimit,
		NodesLimit:   int(d.NodesLimit),
	}, nil
}

func cleanDeployOptionsDataFile(opts *types.DeployOptions) error {
	var err error
	for _, src := range opts.Data {
		err = os.RemoveAll(src)
		if err != nil {
			log.Errorf("[cleanDeployOptionsDataFile] clean temp files failed %v", err)
		}
	}
	return err
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
		Publish:  utils.EncodePublishInfo(c.Publish),
		Hook:     types.HookOutput(c.Hook),
	}
	if c.Error != nil {
		msg.Error = c.Error.Error()
	}
	return msg
}

func toRPCReplaceContainerMessage(r *types.ReplaceContainerMessage) *pb.ReplaceContainerMessage {
	msg := &pb.ReplaceContainerMessage{
		Create: toRPCCreateContainerMessage(r.Create),
		Remove: toRPCRemoveContainerMessage(r.Remove),
	}
	if r.Error != nil {
		msg.Error = r.Error.Error()
	}
	return msg
}

func toRPCCacheImageMessage(r *types.CacheImageMessage) *pb.CacheImageMessage {
	return &pb.CacheImageMessage{
		Image:    r.Image,
		Success:  r.Success,
		Nodename: r.Nodename,
		Message:  r.Message,
	}
}

func toRPCRemoveImageMessage(r *types.RemoveImageMessage) *pb.RemoveImageMessage {
	return &pb.RemoveImageMessage{
		Image:    r.Image,
		Success:  r.Success,
		Messages: r.Messages,
	}
}

func toRPCControlContainerMessage(c *types.ControlContainerMessage) *pb.ControlContainerMessage {
	r := &pb.ControlContainerMessage{
		Id:   c.ContainerID,
		Hook: types.HookOutput(c.Hook),
	}
	if c.Error != nil {
		r.Error = c.Error.Error()
	}
	return r
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
		Hook:    string(types.HookOutput(r.Hook)),
	}
}

func toRPCRunAndWaitMessage(msg *types.RunAndWaitMessage) *pb.RunAndWaitMessage {
	return &pb.RunAndWaitMessage{
		ContainerId: msg.ContainerID,
		Data:        msg.Data,
	}
}

func toRPCContainers(ctx context.Context, containers []*types.Container, labels map[string]string) []*pb.Container {
	cs := []*pb.Container{}
	for _, c := range containers {
		pContainer, err := toRPCContainer(ctx, c)
		if err != nil {
			log.Errorf("[toRPCContainers] trans to pb container failed %v", err)
			continue
		}
		if utils.FilterContainer(pContainer.Labels, labels) {
			cs = append(cs, pContainer)
		}
	}
	return cs
}

func toRPCContainer(ctx context.Context, c *types.Container) (*pb.Container, error) {
	info, err := c.Inspect(ctx)
	if err != nil {
		return nil, err
	}

	meta := utils.DecodeMetaInLabel(info.Labels)
	publish := map[string]string{}
	if info.Networks != nil && info.Running {
		publish = utils.EncodePublishInfo(
			utils.MakePublishInfo(info.Networks, meta.Publish),
		)
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
		Publish:    publish,
		Image:      info.Image,
		Labels:     info.Labels,
		Inspect:    bytes,
		StatusData: c.StatusData,
	}, nil
}
