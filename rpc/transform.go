package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"golang.org/x/net/context"
)

func toRPCServiceStatus(status types.ServiceStatus) *pb.ServiceStatus {
	return &pb.ServiceStatus{
		Addresses:        status.Addresses,
		IntervalInSecond: int64(status.Interval / time.Second),
	}
}

func toRPCPod(p *types.Pod) *pb.Pod {
	return &pb.Pod{Name: p.Name, Desc: p.Desc}
}

func toRPCNetwork(n *enginetypes.Network) *pb.Network {
	return &pb.Network{Name: n.Name, Subnets: n.Subnets}
}

func toRPCNode(n *types.Node) *pb.Node {
	node := &pb.Node{
		Name:             n.Name,
		Endpoint:         n.Endpoint,
		Podname:          n.Podname,
		Available:        n.Available,
		Labels:           n.Labels,
		Info:             n.NodeInfo,
		Bypass:           n.Bypass,
		ResourceCapacity: toRPCResources(n.ResourceInfo.Capacity),
		ResourceUsage:    toRPCResources(n.ResourceInfo.Usage),
	}
	return node
}

func toRPCResources(v interface{}) string {
	body, _ := json.Marshal(v)
	return string(body)
}

func toRPCEngine(e *enginetypes.Info) *pb.Engine {
	return &pb.Engine{
		Type: e.Type,
	}
}

func toRPCNodeResource(nr *types.NodeResourceInfo) *pb.NodeResource {
	return &pb.NodeResource{
		Name:             nr.Name,
		Diffs:            nr.Diffs,
		ResourceCapacity: toRPCResources(nr.Capacity),
		ResourceUsage:    toRPCResources(nr.Usage),
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

func toCoreListNodesOptions(b *pb.ListNodesOptions) *types.ListNodesOptions {
	return &types.ListNodesOptions{
		Podname:  b.Podname,
		Labels:   b.Labels,
		All:      b.All,
		CallInfo: !b.SkipInfo,
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

func toCoreSendOptions(b *pb.SendOptions) (*types.SendOptions, error) { //nolint
	files := []types.LinuxFile{}
	for filename, content := range b.Data {
		files = append(files, types.LinuxFile{
			Content:  content,
			Filename: filename,
			UID:      int(b.Owners[filename].GetUid()),
			GID:      int(b.Owners[filename].GetGid()),
			Mode:     b.Modes[filename].GetMode(),
		})
	}
	return &types.SendOptions{
		IDs:   b.IDs,
		Files: files,
	}, nil
}

func toCoreAddNodeOptions(b *pb.AddNodeOptions) *types.AddNodeOptions {
	r := &types.AddNodeOptions{
		Nodename:  b.Nodename,
		Endpoint:  b.Endpoint,
		Podname:   b.Podname,
		Ca:        b.Ca,
		Cert:      b.Cert,
		Key:       b.Key,
		Labels:    b.Labels,
		Resources: toCoreResources(b.Resources),
	}
	return r
}

func toCoreSetNodeOptions(b *pb.SetNodeOptions) (*types.SetNodeOptions, error) { //nolint
	r := &types.SetNodeOptions{
		Nodename:      b.Nodename,
		Endpoint:      b.Endpoint,
		Ca:            b.Ca,
		Cert:          b.Cert,
		Key:           b.Key,
		WorkloadsDown: b.WorkloadsDown,
		Resources:     toCoreResources(b.Resources),
		Delta:         b.Delta,
		Labels:        b.Labels,
		Bypass:        types.TriOptions(b.Bypass),
	}
	return r, nil
}

func toCoreBuildOptions(b *pb.BuildImageOptions) (*types.BuildOptions, error) {
	var builds *types.Builds
	if b.GetBuilds() != nil {
		if len(b.GetBuilds().Stages) == 0 {
			return nil, types.ErrNoBuildsInSpec
		}
		builds = &types.Builds{
			Stages: b.GetBuilds().Stages,
		}
		builds.Builds = map[string]*types.Build{}
		for stage, p := range b.GetBuilds().Builds {
			if p == nil {
				return nil, types.ErrNoBuildSpec
			}
			builds.Builds[stage] = &types.Build{
				Base:       p.Base,
				Repo:       p.Repo,
				Version:    p.Version,
				Dir:        p.Dir,
				Submodule:  p.Submodule || false,
				Security:   p.Security || false,
				Commands:   p.Commands,
				Envs:       p.Envs,
				Args:       p.Args,
				Labels:     p.Labels,
				Artifacts:  p.Artifacts,
				Cache:      p.Cache,
				StopSignal: p.StopSignal,
			}
		}
	}

	var buildMethod types.BuildMethod
	switch b.GetBuildMethod() {
	case pb.BuildImageOptions_SCM:
		buildMethod = types.BuildFromSCM
	case pb.BuildImageOptions_RAW:
		buildMethod = types.BuildFromRaw
	case pb.BuildImageOptions_EXIST:
		buildMethod = types.BuildFromExist
	}

	return &types.BuildOptions{
		Name:        b.Name,
		User:        b.User,
		UID:         int(b.Uid),
		Tags:        b.Tags,
		BuildMethod: buildMethod,
		Builds:      builds,
		Tar:         bytes.NewReader(b.Tar),
		ExistID:     b.GetExistId(),
		Platform:    b.Platform,
	}, nil
}

func toCoreReplaceOptions(r *pb.ReplaceOptions) (*types.ReplaceOptions, error) {
	deployOpts, err := toCoreDeployOptions(r.DeployOpt)
	if err != nil {
		return nil, err
	}

	replaceOpts := &types.ReplaceOptions{
		DeployOptions:  *deployOpts,
		NetworkInherit: r.Networkinherit,
		FilterLabels:   r.FilterLabels,
		Copy:           r.Copy,
		IDs:            r.IDs,
	}

	return replaceOpts, err
}

func toCoreDeployOptions(d *pb.DeployOptions) (*types.DeployOptions, error) {
	if d.Entrypoint == nil || d.Entrypoint.Name == "" {
		return nil, types.ErrNoEntryInSpec
	}

	entrypoint := d.Entrypoint
	entry := &types.Entrypoint{
		Name:       entrypoint.Name,
		Commands:   utils.MakeCommandLineArgs(fmt.Sprintf("%s %s", d.Entrypoint.Command, d.ExtraArgs)),
		Privileged: entrypoint.Privileged,
		Dir:        entrypoint.Dir,
		Publish:    entrypoint.Publish,
		Restart:    entrypoint.Restart,
		Sysctls:    entrypoint.Sysctls,
	}

	if len(d.Entrypoint.Commands) > 0 {
		entry.Commands = d.Entrypoint.Commands
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

	files := []types.LinuxFile{}
	for filename, bs := range d.Data {
		file := types.LinuxFile{
			Content:  bs,
			Filename: filename,
			UID:      int(d.Owners[filename].GetUid()),
			GID:      int(d.Owners[filename].GetGid()),
			Mode:     d.Modes[filename].GetMode(),
		}
		if file.Mode == 0 && file.UID == 0 && file.GID == 0 {
			file.Mode = 0755
		}
		files = append(files, file)
	}

	nodeFilter := &types.NodeFilter{
		Podname:  d.Podname,
		Includes: d.Nodenames,
		Labels:   d.Nodelabels,
	}
	if d.NodeFilter != nil {
		nodeFilter.Includes = d.NodeFilter.Includes
		nodeFilter.Excludes = d.NodeFilter.Excludes
		nodeFilter.Labels = d.NodeFilter.Labels
	}

	return &types.DeployOptions{
		Resources:      toCoreResources(d.Resources),
		Name:           d.Name,
		Entrypoint:     entry,
		Podname:        d.Podname,
		NodeFilter:     nodeFilter,
		Image:          d.Image,
		ExtraArgs:      d.ExtraArgs,
		Count:          int(d.Count),
		Env:            d.Env,
		DNS:            d.Dns,
		ExtraHosts:     d.ExtraHosts,
		Networks:       d.Networks,
		User:           d.User,
		Debug:          d.Debug,
		OpenStdin:      d.OpenStdin,
		Labels:         d.Labels,
		DeployStrategy: d.DeployStrategy.String(),
		NodesLimit:     int(d.NodesLimit),
		IgnoreHook:     d.IgnoreHook,
		AfterCreate:    d.AfterCreate,
		RawArgs:        d.RawArgs,
		Files:          files,
	}, nil
}

func toRPCCreateWorkloadMessage(c *types.CreateWorkloadMessage) *pb.CreateWorkloadMessage {
	if c == nil {
		return nil
	}

	msg := &pb.CreateWorkloadMessage{
		Podname:   c.Podname,
		Nodename:  c.Nodename,
		Id:        c.WorkloadID,
		Name:      c.WorkloadName,
		Success:   c.Error == nil,
		Publish:   utils.EncodePublishInfo(c.Publish),
		Hook:      utils.MergeHookOutputs(c.Hook),
		Resources: toRPCResources(c.Resources),
	}
	if c.Error != nil {
		msg.Error = c.Error.Error()
	}
	return msg
}

func toRPCReplaceWorkloadMessage(r *types.ReplaceWorkloadMessage) *pb.ReplaceWorkloadMessage {
	msg := &pb.ReplaceWorkloadMessage{
		Create: toRPCCreateWorkloadMessage(r.Create),
		Remove: toRPCRemoveWorkloadMessage(r.Remove),
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

func toRPCControlWorkloadMessage(c *types.ControlWorkloadMessage) *pb.ControlWorkloadMessage {
	r := &pb.ControlWorkloadMessage{
		Id:   c.WorkloadID,
		Hook: utils.MergeHookOutputs(c.Hook),
	}
	if c.Error != nil {
		r.Error = c.Error.Error()
	}
	return r
}

func toRPCRemoveWorkloadMessage(r *types.RemoveWorkloadMessage) *pb.RemoveWorkloadMessage {
	if r == nil {
		return nil
	}
	return &pb.RemoveWorkloadMessage{
		Id:      r.WorkloadID,
		Success: r.Success,
		Hook:    string(utils.MergeHookOutputs(r.Hook)),
	}
}

func toRPCDissociateWorkloadMessage(r *types.DissociateWorkloadMessage) *pb.DissociateWorkloadMessage {
	resp := &pb.DissociateWorkloadMessage{
		Id: r.WorkloadID,
	}
	if r.Error != nil {
		resp.Error = r.Error.Error()
	}
	return resp
}

func toRPCStdStreamType(stdType types.StdStreamType) pb.StdStreamType {
	switch stdType {
	case types.EruError:
		return pb.StdStreamType_ERUERROR
	case types.TypeWorkloadID:
		return pb.StdStreamType_TYPEWORKLOADID
	case types.Stdout:
		return pb.StdStreamType_STDOUT
	default:
		return pb.StdStreamType_STDERR
	}
}

func toRPCAttachWorkloadMessage(msg *types.AttachWorkloadMessage) *pb.AttachWorkloadMessage {
	return &pb.AttachWorkloadMessage{
		WorkloadId:    msg.WorkloadID,
		Data:          msg.Data,
		StdStreamType: toRPCStdStreamType(msg.StdStreamType),
	}
}

func toRPCWorkloadStatus(workloadStatus *types.StatusMeta) *pb.WorkloadStatus {
	r := &pb.WorkloadStatus{}
	if workloadStatus != nil {
		r.Id = workloadStatus.ID
		r.Healthy = workloadStatus.Healthy
		r.Running = workloadStatus.Running
		r.Networks = workloadStatus.Networks
		r.Extension = workloadStatus.Extension
	}
	return r
}

func toRPCWorkloadsStatus(workloadsStatus []*types.StatusMeta) *pb.WorkloadsStatus {
	ret := &pb.WorkloadsStatus{}
	r := []*pb.WorkloadStatus{}
	for _, cs := range workloadsStatus {
		s := toRPCWorkloadStatus(cs)
		if s != nil {
			r = append(r, s)
		}
	}
	ret.Status = r
	return ret
}

func toRPCWorkloads(ctx context.Context, workloads []*types.Workload, labels map[string]string) *pb.Workloads {
	ret := &pb.Workloads{}
	cs := []*pb.Workload{}
	for _, c := range workloads {
		pWorkload, err := toRPCWorkload(ctx, c)
		if err != nil {
			log.WithFunc("transform.toRPCWorkloads").Error(ctx, err, "trans to pb workload failed")
			continue
		}
		if !utils.LabelsFilter(pWorkload.Labels, labels) {
			continue
		}
		cs = append(cs, pWorkload)
	}
	ret.Workloads = cs
	return ret
}

func toRPCWorkload(ctx context.Context, c *types.Workload) (*pb.Workload, error) {
	publish := map[string]string{}
	if c.StatusMeta != nil && len(c.StatusMeta.Networks) != 0 {
		meta := utils.DecodeMetaInLabel(ctx, c.Labels)
		publish = utils.EncodePublishInfo(
			utils.MakePublishInfo(c.StatusMeta.Networks, meta.Publish),
		)
	}

	return &pb.Workload{
		Id:         c.ID,
		Podname:    c.Podname,
		Nodename:   c.Nodename,
		Name:       c.Name,
		Privileged: c.Privileged,
		Publish:    publish,
		Image:      c.Image,
		Labels:     c.Labels,
		Status:     toRPCWorkloadStatus(c.StatusMeta),
		CreateTime: c.CreateTime,
		Resources:  toRPCResources(c.Resources),
		Env:        c.Env,
	}, nil
}

func toRPCLogStreamMessage(msg *types.LogStreamMessage) *pb.LogStreamMessage {
	r := &pb.LogStreamMessage{
		Id:            msg.ID,
		Data:          msg.Data,
		StdStreamType: toRPCStdStreamType(msg.StdStreamType),
	}
	if msg.Error != nil {
		r.Error = msg.Error.Error()
	}
	return r
}

func toCoreExecuteWorkloadOptions(b *pb.ExecuteWorkloadOptions) (opts *types.ExecuteWorkloadOptions, err error) { //nolint
	return &types.ExecuteWorkloadOptions{
		WorkloadID: b.WorkloadId,
		Commands:   b.Commands,
		Envs:       b.Envs,
		Workdir:    b.Workdir,
		OpenStdin:  b.OpenStdin,
		ReplCmd:    b.ReplCmd,
	}, nil
}

func toRPCCapacityMessage(msg *types.CapacityMessage) *pb.CapacityMessage {
	if msg == nil {
		return nil
	}
	caps := map[string]int64{}
	for nodename, capacity := range msg.NodeCapacities {
		caps[nodename] = int64(capacity)
	}
	return &pb.CapacityMessage{
		Total:          int64(msg.Total),
		NodeCapacities: caps,
	}
}

func toCoreCacheImageOptions(opts *pb.CacheImageOptions) *types.ImageOptions {
	return &types.ImageOptions{
		Podname:   opts.Podname,
		Nodenames: opts.Nodenames,
		Images:    opts.Images,
	}
}

func toCoreRemoveImageOptions(opts *pb.RemoveImageOptions) *types.ImageOptions {
	return &types.ImageOptions{
		Podname:   opts.Podname,
		Nodenames: opts.Nodenames,
		Images:    opts.Images,
		Prune:     opts.Prune,
	}
}

func toCoreResources(resources map[string][]byte) types.Resources {
	r := types.Resources{}
	for k, v := range resources {
		rp := types.RawParams{}
		if err := json.Unmarshal(v, &rp); err != nil {
			log.WithFunc("toCoreResources").Errorf(nil, err, "%v", string(v)) // nolint
			continue
		}
		r[k] = rp
	}
	return r
}

func toRPCListImageMessage(msg *types.ListImageMessage) *pb.ListImageMessage {
	m := &pb.ListImageMessage{
		Images:   []*pb.ImageItem{},
		Nodename: "",
		Err:      "",
	}
	if msg == nil {
		return m
	}
	if msg.Error != nil {
		m.Err = msg.Error.Error()
		return m
	}

	m.Nodename = msg.Nodename
	for _, image := range msg.Images {
		m.Images = append(m.Images, &pb.ImageItem{
			Id:   image.ID,
			Tags: image.Tags,
		})
	}

	return m
}

func toCoreListImageOptions(opts *pb.ListImageOptions) *types.ImageOptions {
	return &types.ImageOptions{
		Podname:   opts.Podname,
		Nodenames: opts.Nodenames,
		Filter:    opts.Filter,
	}
}
