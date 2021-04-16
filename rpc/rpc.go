package rpc

import (
	"archive/tar"
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"runtime"
	"sync"
	"time"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"
	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/version"
	"golang.org/x/net/context"
	grpcstatus "google.golang.org/grpc/status"
)

// Vibranium is implementations for grpc server interface
// Many data types should be transformed
type Vibranium struct {
	cluster cluster.Cluster
	config  types.Config
	counter sync.WaitGroup
	rpcch   chan struct{}
	TaskNum int
}

// Info show core info
func (v *Vibranium) Info(ctx context.Context, opts *pb.Empty) (*pb.CoreInfo, error) {
	return &pb.CoreInfo{
		Version:       version.VERSION,
		Revison:       version.REVISION,
		BuildAt:       version.BUILTAT,
		GolangVersion: runtime.Version(),
		OsArch:        fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}, nil
}

// WatchServiceStatus pushes sibling services
func (v *Vibranium) WatchServiceStatus(_ *pb.Empty, stream pb.CoreRPC_WatchServiceStatusServer) (err error) {
	ch, err := v.cluster.WatchServiceStatus(stream.Context())
	if err != nil {
		return grpcstatus.Error(WatchServiceStatus, err.Error())
	}
	for status := range ch {
		s := toRPCServiceStatus(status)
		if err = stream.Send(s); err != nil {
			v.logUnsentMessages("WatchServicesStatus", err, s)
			return grpcstatus.Error(WatchServiceStatus, err.Error())
		}
	}
	return nil
}

// ListNetworks list networks for pod
func (v *Vibranium) ListNetworks(ctx context.Context, opts *pb.ListNetworkOptions) (*pb.Networks, error) {
	networks, err := v.cluster.ListNetworks(ctx, opts.Podname, opts.Driver)
	if err != nil {
		return nil, grpcstatus.Error(ListNetworks, err.Error())
	}

	ns := []*pb.Network{}
	for _, n := range networks {
		ns = append(ns, toRPCNetwork(n))
	}
	return &pb.Networks{Networks: ns}, nil
}

// ConnectNetwork connect network
func (v *Vibranium) ConnectNetwork(ctx context.Context, opts *pb.ConnectNetworkOptions) (*pb.Network, error) {
	subnets, err := v.cluster.ConnectNetwork(ctx, opts.Network, opts.Target, opts.Ipv4, opts.Ipv6)
	if err != nil {
		return nil, grpcstatus.Error(ConnectNetwork, err.Error())
	}
	return &pb.Network{Name: opts.Network, Subnets: subnets}, nil
}

// DisconnectNetwork disconnect network
func (v *Vibranium) DisconnectNetwork(ctx context.Context, opts *pb.DisconnectNetworkOptions) (*pb.Empty, error) {
	if err := v.cluster.DisconnectNetwork(ctx, opts.Network, opts.Target, opts.Force); err != nil {
		return nil, grpcstatus.Error(DisconnectNetwork, err.Error())
	}
	return &pb.Empty{}, nil
}

// AddPod saves a pod, and returns it to client
func (v *Vibranium) AddPod(ctx context.Context, opts *pb.AddPodOptions) (*pb.Pod, error) {
	p, err := v.cluster.AddPod(ctx, opts.Name, opts.Desc)
	if err != nil {
		return nil, grpcstatus.Error(AddPod, err.Error())
	}

	return toRPCPod(p), nil
}

// RemovePod removes a pod only if it's empty
func (v *Vibranium) RemovePod(ctx context.Context, opts *pb.RemovePodOptions) (*pb.Empty, error) {
	if err := v.cluster.RemovePod(ctx, opts.Name); err != nil {
		return nil, grpcstatus.Error(RemovePod, err.Error())
	}
	return &pb.Empty{}, nil
}

// GetPod show a pod
func (v *Vibranium) GetPod(ctx context.Context, opts *pb.GetPodOptions) (*pb.Pod, error) {
	p, err := v.cluster.GetPod(ctx, opts.Name)
	if err != nil {
		return nil, grpcstatus.Error(GetPod, err.Error())
	}

	return toRPCPod(p), nil
}

// ListPods returns a list of pods
func (v *Vibranium) ListPods(ctx context.Context, _ *pb.Empty) (*pb.Pods, error) {
	ps, err := v.cluster.ListPods(ctx)
	if err != nil {
		return nil, grpcstatus.Error(ListPods, err.Error())
	}

	pods := []*pb.Pod{}
	for _, p := range ps {
		pods = append(pods, toRPCPod(p))
	}

	return &pb.Pods{Pods: pods}, nil
}

// GetPodResource get pod nodes resource usage
func (v *Vibranium) GetPodResource(ctx context.Context, opts *pb.GetPodOptions) (*pb.PodResource, error) {
	r, err := v.cluster.PodResource(ctx, opts.Name)
	if err != nil {
		return nil, grpcstatus.Error(PodResource, err.Error())
	}
	return toRPCPodResource(r), nil
}

// AddNode saves a node and returns it to client
// Method must be called synchronously, or nothing will be returned
func (v *Vibranium) AddNode(ctx context.Context, opts *pb.AddNodeOptions) (*pb.Node, error) {
	addNodeOpts := toCoreAddNodeOptions(opts)
	n, err := v.cluster.AddNode(ctx, addNodeOpts)
	if err != nil {
		return nil, grpcstatus.Error(AddNode, err.Error())
	}

	return toRPCNode(ctx, n), nil
}

// RemoveNode removes the node from etcd
func (v *Vibranium) RemoveNode(ctx context.Context, opts *pb.RemoveNodeOptions) (*pb.Empty, error) {
	if err := v.cluster.RemoveNode(ctx, opts.Nodename); err != nil {
		return nil, grpcstatus.Error(RemoveNode, err.Error())
	}
	return &pb.Empty{}, nil
}

// ListPodNodes returns a list of node for pod
func (v *Vibranium) ListPodNodes(ctx context.Context, opts *pb.ListNodesOptions) (*pb.Nodes, error) {
	ns, err := v.cluster.ListPodNodes(ctx, opts.Podname, opts.Labels, opts.All)
	if err != nil {
		return nil, grpcstatus.Error(ListPodNodes, err.Error())
	}

	ctx, cancel := context.WithTimeout(ctx, v.config.GlobalTimeout)
	defer cancel()
	nodes := []*pb.Node{}
	for _, n := range ns {
		nodes = append(nodes, toRPCNode(ctx, n))
	}
	return &pb.Nodes{Nodes: nodes}, nil
}

// GetNode get a node
func (v *Vibranium) GetNode(ctx context.Context, opts *pb.GetNodeOptions) (*pb.Node, error) {
	n, err := v.cluster.GetNode(ctx, opts.Nodename)
	if err != nil {
		return nil, grpcstatus.Error(GetNode, err.Error())
	}

	return toRPCNode(ctx, n), nil
}

// SetNode set node meta
func (v *Vibranium) SetNode(ctx context.Context, opts *pb.SetNodeOptions) (*pb.Node, error) {
	setNodeOpts, err := toCoreSetNodeOptions(opts)
	if err != nil {
		return nil, grpcstatus.Error(SetNode, err.Error())
	}
	n, err := v.cluster.SetNode(ctx, setNodeOpts)
	if err != nil {
		return nil, grpcstatus.Error(SetNode, err.Error())
	}
	return toRPCNode(ctx, n), nil
}

// SetNodeStatus set status of a node for reporting
func (v *Vibranium) SetNodeStatus(ctx context.Context, opts *pb.SetNodeStatusOptions) (*pb.Empty, error) {
	if err := v.cluster.SetNodeStatus(ctx, opts.Nodename, opts.Ttl); err != nil {
		return nil, grpcstatus.Error(SetNodeStatus, err.Error())
	}
	return &pb.Empty{}, nil
}

// GetNodeStatus set status of a node for reporting
func (v *Vibranium) GetNodeStatus(ctx context.Context, opts *pb.GetNodeStatusOptions) (*pb.NodeStatusStreamMessage, error) {
	status, err := v.cluster.GetNodeStatus(ctx, opts.Nodename)
	if err != nil {
		return nil, grpcstatus.Error(GetNodeStatus, err.Error())
	}
	return &pb.NodeStatusStreamMessage{
		Nodename: status.Nodename,
		Podname:  status.Podname,
		Alive:    status.Alive,
	}, nil
}

// NodeStatusStream watch and show deployed status
func (v *Vibranium) NodeStatusStream(_ *pb.Empty, stream pb.CoreRPC_NodeStatusStreamServer) error {
	log.Info("[rpc] NodeStatusStream started")
	defer log.Info("[rpc] NodeStatusStream stopped")

	ch := v.cluster.NodeStatusStream(stream.Context())
	for {
		select {
		case m, ok := <-ch:
			if !ok {
				return nil
			}
			r := &pb.NodeStatusStreamMessage{
				Nodename: m.Nodename,
				Podname:  m.Podname,
				Alive:    m.Alive,
			}
			if m.Error != nil {
				r.Error = m.Error.Error()
			}
			if err := stream.Send(r); err != nil {
				v.logUnsentMessages("NodeStatusStream", err, m)
			}
		case <-v.rpcch:
			return nil
		}
	}
}

// GetNodeResource check node resource
func (v *Vibranium) GetNodeResource(ctx context.Context, opts *pb.GetNodeResourceOptions) (*pb.NodeResource, error) {
	nr, err := v.cluster.NodeResource(ctx, opts.GetOpts().Nodename, opts.Fix)
	if err != nil {
		return nil, grpcstatus.Error(GetNodeResource, err.Error())
	}

	return toRPCNodeResource(nr), nil
}

// CalculateCapacity calculates capacity for each node
func (v *Vibranium) CalculateCapacity(ctx context.Context, opts *pb.DeployOptions) (*pb.CapacityMessage, error) {
	v.taskAdd("CalculateCapacity", true)
	defer v.taskDone("CalculateCapacity", true)
	deployOpts, err := toCoreDeployOptions(opts)
	if err != nil {
		return nil, grpcstatus.Error(CalculateCapacity, err.Error())
	}
	m, err := v.cluster.CalculateCapacity(ctx, deployOpts)
	if err != nil {
		return nil, grpcstatus.Error(CalculateCapacity, err.Error())
	}
	return toRPCCapacityMessage(m), nil
}

// GetWorkload get a workload
// More information will be shown
func (v *Vibranium) GetWorkload(ctx context.Context, id *pb.WorkloadID) (*pb.Workload, error) {
	workload, err := v.cluster.GetWorkload(ctx, id.Id)
	if err != nil {
		return nil, grpcstatus.Error(GetWorkload, err.Error())
	}

	return toRPCWorkload(ctx, workload)
}

// GetWorkloads get lots workloads
// like GetWorkload, information should be returned
func (v *Vibranium) GetWorkloads(ctx context.Context, cids *pb.WorkloadIDs) (*pb.Workloads, error) {
	workloads, err := v.cluster.GetWorkloads(ctx, cids.GetIds())
	if err != nil {
		return nil, grpcstatus.Error(GetWorkloads, err.Error())
	}

	return toRPCWorkloads(ctx, workloads, nil), nil
}

// ListWorkloads by appname with optional entrypoint and nodename
func (v *Vibranium) ListWorkloads(opts *pb.ListWorkloadsOptions, stream pb.CoreRPC_ListWorkloadsServer) error {
	lsopts := &types.ListWorkloadsOptions{
		Appname:    opts.Appname,
		Entrypoint: opts.Entrypoint,
		Nodename:   opts.Nodename,
		Limit:      opts.Limit,
		Labels:     opts.Labels,
	}
	workloads, err := v.cluster.ListWorkloads(stream.Context(), lsopts)
	if err != nil {
		return grpcstatus.Error(ListWorkloads, err.Error())
	}

	for _, c := range toRPCWorkloads(stream.Context(), workloads, opts.Labels).Workloads {
		if err = stream.Send(c); err != nil {
			v.logUnsentMessages("ListWorkloads", err, c)
			return grpcstatus.Error(ListWorkloads, err.Error())
		}
	}
	return nil
}

// ListNodeWorkloads list node workloads
func (v *Vibranium) ListNodeWorkloads(ctx context.Context, opts *pb.GetNodeOptions) (*pb.Workloads, error) {
	workloads, err := v.cluster.ListNodeWorkloads(ctx, opts.Nodename, opts.Labels)
	if err != nil {
		return nil, grpcstatus.Error(ListNodeWorkloads, err.Error())
	}
	return toRPCWorkloads(ctx, workloads, nil), nil
}

// GetWorkloadsStatus get workloads status
func (v *Vibranium) GetWorkloadsStatus(ctx context.Context, opts *pb.WorkloadIDs) (*pb.WorkloadsStatus, error) {
	v.taskAdd("GetWorkloadsStatus", false)
	defer v.taskDone("GetWorkloadsStatus", false)

	workloadsStatus, err := v.cluster.GetWorkloadsStatus(ctx, opts.Ids)
	if err != nil {
		return nil, grpcstatus.Error(GetWorkloadsStatus, err.Error())
	}
	return toRPCWorkloadsStatus(workloadsStatus), nil
}

// SetWorkloadsStatus set workloads status
func (v *Vibranium) SetWorkloadsStatus(ctx context.Context, opts *pb.SetWorkloadsStatusOptions) (*pb.WorkloadsStatus, error) {
	v.taskAdd("SetWorkloadsStatus", false)
	defer v.taskDone("SetWorkloadsStatus", false)

	var err error
	statusData := []*types.StatusMeta{}
	ttls := map[string]int64{}
	for _, status := range opts.Status {
		r := &types.StatusMeta{
			ID:        status.Id,
			Running:   status.Running,
			Healthy:   status.Healthy,
			Networks:  status.Networks,
			Extension: status.Extension,
		}
		statusData = append(statusData, r)
		ttls[status.Id] = status.Ttl
	}

	status, err := v.cluster.SetWorkloadsStatus(ctx, statusData, ttls)
	if err != nil {
		return nil, grpcstatus.Error(SetWorkloadsStatus, err.Error())
	}
	return toRPCWorkloadsStatus(status), nil
}

// WorkloadStatusStream watch and show deployed status
func (v *Vibranium) WorkloadStatusStream(opts *pb.WorkloadStatusStreamOptions, stream pb.CoreRPC_WorkloadStatusStreamServer) error {
	log.Infof("[rpc] WorkloadStatusStream start %s", opts.Appname)
	defer log.Infof("[rpc] WorkloadStatusStream stop %s", opts.Appname)

	ch := v.cluster.WorkloadStatusStream(
		stream.Context(),
		opts.Appname, opts.Entrypoint, opts.Nodename, opts.Labels,
	)
	for {
		select {
		case m, ok := <-ch:
			if !ok {
				return nil
			}
			r := &pb.WorkloadStatusStreamMessage{Id: m.ID, Delete: m.Delete}
			if m.Error != nil {
				r.Error = m.Error.Error()
			} else if m.Workload != nil {
				if workload, err := toRPCWorkload(stream.Context(), m.Workload); err != nil {
					r.Error = err.Error()
				} else {
					r.Workload = workload
					r.Status = toRPCWorkloadStatus(m.Workload.StatusMeta)
				}
			}
			if err := stream.Send(r); err != nil {
				v.logUnsentMessages("WorkloadStatusStream", err, m)
			}
		case <-v.rpcch:
			return nil
		}
	}
}

// Copy copy files from multiple workloads
func (v *Vibranium) Copy(opts *pb.CopyOptions, stream pb.CoreRPC_CopyServer) error {
	v.taskAdd("Copy", true)
	defer v.taskDone("Copy", true)

	copyOpts := toCoreCopyOptions(opts)
	ch, err := v.cluster.Copy(stream.Context(), copyOpts)
	if err != nil {
		return grpcstatus.Error(Copy, err.Error())
	}
	// 4K buffer
	p := make([]byte, 4096)
	for m := range ch {
		msg := &pb.CopyMessage{Id: m.ID, Name: m.Name, Path: m.Path}
		if m.Error != nil {
			msg.Error = m.Error.Error()
			if err := stream.Send(msg); err != nil {
				v.logUnsentMessages("Copy", err, m)
			}
			continue
		}

		r, w := io.Pipe()
		go func() {
			var err error
			defer func() {
				w.CloseWithError(err) // nolint
			}()
			defer m.Data.Close()

			var bs []byte
			tw := tar.NewWriter(w)
			if bs, err = ioutil.ReadAll(m.Data); err != nil {
				log.Errorf("[Copy] Error during extracting copy data: %v", err)
				return
			}
			header := &tar.Header{Name: m.Name, Mode: 0644, Size: int64(len(bs))}
			if err = tw.WriteHeader(header); err != nil {
				log.Errorf("[Copy] Error during writing tarball header: %v", err)
				return
			}
			if _, err = tw.Write(bs); err != nil {
				log.Errorf("[Copy] Error during writing tarball content: %v", err)
				return
			}
		}()

		for {
			n, err := r.Read(p)
			if err != nil {
				if err != io.EOF {
					log.Errorf("[Copy] Error during buffer resp: %v", err)
					msg.Error = err.Error()
					if err = stream.Send(msg); err != nil {
						v.logUnsentMessages("Copy", err, m)
					}
				}
				break
			}
			msg.Data = p[:n]
			if err = stream.Send(msg); err != nil {
				v.logUnsentMessages("Copy", err, m)
			}
		}
	}
	return nil
}

// Send send files to some contaienrs
func (v *Vibranium) Send(opts *pb.SendOptions, stream pb.CoreRPC_SendServer) error {
	v.taskAdd("Send", true)
	defer v.taskDone("Send", true)

	sendOpts, err := toCoreSendOptions(opts)
	if err != nil {
		return grpcstatus.Error(Send, err.Error())
	}

	sendOpts.Data = opts.Data
	ch, err := v.cluster.Send(stream.Context(), sendOpts)
	if err != nil {
		return grpcstatus.Error(Send, err.Error())
	}

	for m := range ch {
		msg := &pb.SendMessage{
			Id:   m.ID,
			Path: m.Path,
		}

		if m.Error != nil {
			msg.Error = m.Error.Error()
		}

		if err := stream.Send(msg); err != nil {
			v.logUnsentMessages("Send", err, m)
		}
	}
	return nil
}

// BuildImage streamed returned functions
func (v *Vibranium) BuildImage(opts *pb.BuildImageOptions, stream pb.CoreRPC_BuildImageServer) error {
	v.taskAdd("BuildImage", true)
	defer v.taskDone("BuildImage", true)

	buildOpts, err := toCoreBuildOptions(opts)
	if err != nil {
		return grpcstatus.Error(BuildImage, err.Error())
	}
	ch, err := v.cluster.BuildImage(stream.Context(), buildOpts)
	if err != nil {
		return grpcstatus.Error(BuildImage, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCBuildImageMessage(m)); err != nil {
			v.logUnsentMessages("BuildImage", err, m)
		}
	}
	return nil
}

// CacheImage cache image
func (v *Vibranium) CacheImage(opts *pb.CacheImageOptions, stream pb.CoreRPC_CacheImageServer) error {
	v.taskAdd("CacheImage", true)
	defer v.taskDone("CacheImage", true)

	ch, err := v.cluster.CacheImage(stream.Context(), toCoreCacheImageOptions(opts))
	if err != nil {
		return grpcstatus.Error(CacheImage, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCCacheImageMessage(m)); err != nil {
			v.logUnsentMessages("CacheImage", err, m)
		}
	}
	return nil
}

// RemoveImage remove image
func (v *Vibranium) RemoveImage(opts *pb.RemoveImageOptions, stream pb.CoreRPC_RemoveImageServer) error {
	v.taskAdd("RemoveImage", true)
	defer v.taskDone("RemoveImage", true)

	ch, err := v.cluster.RemoveImage(stream.Context(), toCoreRemoveImageOptions(opts))
	if err != nil {
		return grpcstatus.Error(RemoveImage, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCRemoveImageMessage(m)); err != nil {
			v.logUnsentMessages("RemoveImage", err, m)
		}
	}
	return nil
}

// CreateWorkload create workloads
func (v *Vibranium) CreateWorkload(opts *pb.DeployOptions, stream pb.CoreRPC_CreateWorkloadServer) error {
	v.taskAdd("CreateWorkload", true)
	defer v.taskDone("CreateWorkload", true)

	deployOpts, err := toCoreDeployOptions(opts)
	if err != nil {
		return grpcstatus.Error(CreateWorkload, err.Error())
	}

	ch, err := v.cluster.CreateWorkload(stream.Context(), deployOpts)
	if err != nil {
		return grpcstatus.Error(CreateWorkload, err.Error())
	}
	for m := range ch {
		if err = stream.Send(toRPCCreateWorkloadMessage(m)); err != nil {
			v.logUnsentMessages("CreateWorkload", err, m)
		}
	}
	return nil
}

// ReplaceWorkload replace workloads
func (v *Vibranium) ReplaceWorkload(opts *pb.ReplaceOptions, stream pb.CoreRPC_ReplaceWorkloadServer) error {
	v.taskAdd("ReplaceWorkload", true)
	defer v.taskDone("ReplaceWorkload", true)

	replaceOpts, err := toCoreReplaceOptions(opts)
	if err != nil {
		return grpcstatus.Error(ReplaceWorkload, err.Error())
	}

	ch, err := v.cluster.ReplaceWorkload(stream.Context(), replaceOpts)
	if err != nil {
		return grpcstatus.Error(ReplaceWorkload, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCReplaceWorkloadMessage(m)); err != nil {
			v.logUnsentMessages("ReplaceWorkload", err, m)
		}
	}
	return nil
}

// RemoveWorkload remove workloads
func (v *Vibranium) RemoveWorkload(opts *pb.RemoveWorkloadOptions, stream pb.CoreRPC_RemoveWorkloadServer) error {
	v.taskAdd("RemoveWorkload", true)
	defer v.taskDone("RemoveWorkload", true)

	ids := opts.GetIds()
	force := opts.GetForce()
	step := int(opts.GetStep())

	if len(ids) == 0 {
		return types.ErrNoWorkloadIDs
	}
	ch, err := v.cluster.RemoveWorkload(stream.Context(), ids, force, step)
	if err != nil {
		return grpcstatus.Error(ReplaceWorkload, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCRemoveWorkloadMessage(m)); err != nil {
			v.logUnsentMessages("RemoveWorkload", err, m)
		}
	}

	return nil
}

// DissociateWorkload dissociate workload
func (v *Vibranium) DissociateWorkload(opts *pb.DissociateWorkloadOptions, stream pb.CoreRPC_DissociateWorkloadServer) error {
	v.taskAdd("DissociateWorkload", true)
	defer v.taskDone("DissociateWorkload", true)

	ids := opts.GetIds()
	if len(ids) == 0 {
		return types.ErrNoWorkloadIDs
	}

	ch, err := v.cluster.DissociateWorkload(stream.Context(), ids)
	if err != nil {
		return grpcstatus.Error(DissociateWorkload, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCDissociateWorkloadMessage(m)); err != nil {
			v.logUnsentMessages("DissociateWorkload", err, m)
		}
	}

	return nil
}

// ControlWorkload control workloads
func (v *Vibranium) ControlWorkload(opts *pb.ControlWorkloadOptions, stream pb.CoreRPC_ControlWorkloadServer) error {
	v.taskAdd("ControlWorkload", true)
	defer v.taskDone("ControlWorkload", true)

	ids := opts.GetIds()
	t := opts.GetType()
	force := opts.GetForce()

	if len(ids) == 0 {
		return types.ErrNoWorkloadIDs
	}

	ch, err := v.cluster.ControlWorkload(stream.Context(), ids, t, force)
	if err != nil {
		return grpcstatus.Error(ControlWorkload, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCControlWorkloadMessage(m)); err != nil {
			v.logUnsentMessages("ControlWorkload", err, m)
		}
	}

	return nil
}

// ExecuteWorkload runs a command in a running workload
func (v *Vibranium) ExecuteWorkload(stream pb.CoreRPC_ExecuteWorkloadServer) error {
	v.taskAdd("ExecuteWorkload", true)
	defer v.taskDone("ExecuteWorkload", true)

	opts, err := stream.Recv()
	if err != nil {
		return grpcstatus.Error(ExecuteWorkload, err.Error())
	}
	var executeWorkloadOpts *types.ExecuteWorkloadOptions
	if executeWorkloadOpts, err = toCoreExecuteWorkloadOptions(opts); err != nil {
		return grpcstatus.Error(ExecuteWorkload, err.Error())
	}

	inCh := make(chan []byte)
	go func() {
		defer close(inCh)
		if opts.OpenStdin {
			for {
				execWorkloadOpt, err := stream.Recv()
				if execWorkloadOpt == nil || err != nil {
					log.Errorf("[ExecuteWorkload] Recv command error: %v", err)
					return
				}
				inCh <- execWorkloadOpt.ReplCmd
			}
		}
	}()

	for m := range v.cluster.ExecuteWorkload(stream.Context(), executeWorkloadOpts, inCh) {
		if err = stream.Send(toRPCAttachWorkloadMessage(m)); err != nil {
			v.logUnsentMessages("ExecuteWorkload", err, m)
		}
	}
	return nil
}

// ReallocResource realloc res for workloads
func (v *Vibranium) ReallocResource(ctx context.Context, opts *pb.ReallocOptions) (msg *pb.ReallocResourceMessage, err error) {
	defer func() {
		errString := ""
		if err != nil {
			errString = err.Error()
		}
		msg = &pb.ReallocResourceMessage{Error: errString}
	}()

	v.taskAdd("ReallocResource", true)
	defer v.taskDone("ReallocResource", true)
	if opts.Id == "" {
		return msg, grpcstatus.Errorf(ReallocResource, "%v", types.ErrNoWorkloadIDs)
	}

	vbsRequest, err := types.NewVolumeBindings(opts.ResourceOpts.VolumesRequest)
	if err != nil {
		return msg, grpcstatus.Error(ReallocResource, err.Error())
	}

	vbsLimit, err := types.NewVolumeBindings(opts.ResourceOpts.VolumesLimit)
	if err != nil {
		return msg, grpcstatus.Error(ReallocResource, err.Error())
	}

	if err := v.cluster.ReallocResource(
		ctx,
		&types.ReallocOptions{
			ID:          opts.Id,
			CPUBindOpts: types.TriOptions(opts.BindCpuOpt),
			ResourceOpts: types.ResourceOptions{
				CPUQuotaRequest: opts.ResourceOpts.CpuQuotaRequest,
				CPUQuotaLimit:   opts.ResourceOpts.CpuQuotaLimit,
				MemoryRequest:   opts.ResourceOpts.MemoryRequest,
				MemoryLimit:     opts.ResourceOpts.MemoryLimit,
				VolumeRequest:   vbsRequest,
				VolumeLimit:     vbsLimit,
				StorageRequest:  opts.ResourceOpts.StorageRequest,
				StorageLimit:    opts.ResourceOpts.StorageLimit,
			},
		},
	); err != nil {
		return msg, grpcstatus.Error(ReallocResource, err.Error())
	}

	return msg, nil
}

// LogStream get workload logs
func (v *Vibranium) LogStream(opts *pb.LogStreamOptions, stream pb.CoreRPC_LogStreamServer) error {
	ID := opts.GetId()
	log.Infof("[LogStream] Get %s log start", ID)
	defer log.Infof("[LogStream] Get %s log done", ID)
	ch, err := v.cluster.LogStream(stream.Context(), &types.LogStreamOptions{
		ID:     ID,
		Tail:   opts.Tail,
		Since:  opts.Since,
		Until:  opts.Until,
		Follow: opts.Follow,
	})
	if err != nil {
		return grpcstatus.Error(LogStream, err.Error())
	}

	for {
		select {
		case m, ok := <-ch:
			if !ok {
				return nil
			}
			if err = stream.Send(toRPCLogStreamMessage(m)); err != nil {
				v.logUnsentMessages("LogStream", err, m)
			}
		case <-v.rpcch:
			return nil
		}
	}
}

// RunAndWait is lambda
func (v *Vibranium) RunAndWait(stream pb.CoreRPC_RunAndWaitServer) error {
	RunAndWaitOptions, err := stream.Recv()
	if err != nil {
		return grpcstatus.Error(RunAndWait, err.Error())
	}

	if RunAndWaitOptions.DeployOptions == nil {
		return grpcstatus.Error(RunAndWait, types.ErrNoDeployOpts.Error())
	}

	opts := RunAndWaitOptions.DeployOptions
	deployOpts, err := toCoreDeployOptions(opts)
	if err != nil {
		return grpcstatus.Error(RunAndWait, err.Error())
	}

	ctx, cancel := context.WithCancel(stream.Context())
	if RunAndWaitOptions.Async {
		timeout := v.config.GlobalTimeout
		if RunAndWaitOptions.AsyncTimeout != 0 {
			timeout = time.Second * time.Duration(RunAndWaitOptions.AsyncTimeout)
		}
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		// force mark stdin to false
		opts.OpenStdin = false
	}

	v.taskAdd("RunAndWait", true)

	inCh := make(chan []byte)
	go func() {
		defer close(inCh)
		if !opts.OpenStdin {
			return
		}
		for {
			RunAndWaitOptions, err := stream.Recv()
			if RunAndWaitOptions == nil || err != nil {
				log.Errorf("[RunAndWait] Recv command error: %v", err)
				break
			}
			inCh <- RunAndWaitOptions.Cmd
		}
	}()

	ids, ch, err := v.cluster.RunAndWait(ctx, deployOpts, inCh)
	if err != nil {
		return grpcstatus.Error(RunAndWait, err.Error())
	}

	// send workload ids to client first
	for _, id := range ids {
		if err = stream.Send(&pb.AttachWorkloadMessage{
			WorkloadId:    id,
			Data:          []byte(""),
			StdStreamType: pb.StdStreamType_TYPEWORKLOADID,
		}); err != nil {
			v.logUnsentMessages("RunAndWait: first message send failed", err, id)
		}
	}

	// then deal with the rest messages
	runAndWait := func(f func(<-chan *types.AttachWorkloadMessage)) {
		defer v.taskDone("RunAndWait", true)
		defer cancel()
		f(ch)
	}

	if !RunAndWaitOptions.Async {
		runAndWait(func(ch <-chan *types.AttachWorkloadMessage) {
			for m := range ch {
				if err = stream.Send(toRPCAttachWorkloadMessage(m)); err != nil {
					v.logUnsentMessages("RunAndWait", err, m)
				}
			}
		})
		return nil
	}

	go func() {
		runAndWait(func(ch <-chan *types.AttachWorkloadMessage) {
			r, w := io.Pipe()
			go func() {
				defer w.Close()
				for m := range ch {
					if _, err := w.Write(m.Data); err != nil {
						log.Errorf("[Async RunAndWait] iterate and forward AttachWorkloadMessage error: %v", err)
					}
				}
			}()
			bufReader := bufio.NewReader(r)
			for {
				var (
					line, part []byte
					isPrefix   bool
					err        error
				)
				for {
					if part, isPrefix, err = bufReader.ReadLine(); err != nil {
						if err != io.EOF {
							log.Errorf("[Aysnc RunAndWait] read error: %+v", err)
						}
						return
					}
					line = append(line, part...)
					if !isPrefix {
						break
					}
				}
				log.Infof("[Async RunAndWait] %s", line)
			}
		})
	}()
	return nil
}

func (v *Vibranium) logUnsentMessages(msgType string, err error, msg interface{}) {
	log.Infof("[logUnsentMessages] Unsent %s streamed message due to %+v: %v", msgType, err, msg)
}

// New will new a new cluster instance
func New(cluster cluster.Cluster, config types.Config, rpcch chan struct{}) *Vibranium {
	return &Vibranium{cluster: cluster, config: config, counter: sync.WaitGroup{}, rpcch: rpcch}
}
