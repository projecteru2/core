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
	"github.com/projecteru2/core/utils"
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
	stop    chan struct{}
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
		Identifier:    v.cluster.GetIdentifier(),
	}, nil
}

// WatchServiceStatus pushes sibling services
func (v *Vibranium) WatchServiceStatus(_ *pb.Empty, stream pb.CoreRPC_WatchServiceStatusServer) (err error) {
	ctx := v.taskAdd(stream.Context(), "WatchServiceStatus", false)
	defer v.taskDone(ctx, "WatchServiceStatus", false)
	ch, err := v.cluster.WatchServiceStatus(ctx)
	if err != nil {
		return grpcstatus.Error(WatchServiceStatus, err.Error())
	}
	for {
		select {
		case status, ok := <-ch:
			if !ok {
				return nil
			}
			s := toRPCServiceStatus(status)
			if err = stream.Send(s); err != nil {
				v.logUnsentMessages(ctx, "WatchServicesStatus", err, s)
				return grpcstatus.Error(WatchServiceStatus, err.Error())
			}
		case <-v.stop:
			return nil
		}
	}
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
	nodeChan := make(chan *pb.Node, len(ns))
	wg := &sync.WaitGroup{}
	for _, n := range ns {
		wg.Add(1)
		go func(node *types.Node) {
			defer wg.Done()
			nodeChan <- toRPCNode(ctx, node)
		}(n)
	}
	wg.Wait()
	close(nodeChan)
	for node := range nodeChan {
		nodes = append(nodes, node)
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
	ctx := v.taskAdd(stream.Context(), "NodeStatusStream", true)
	defer v.taskDone(ctx, "NodeStatusStream", true)

	ch := v.cluster.NodeStatusStream(ctx)
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
				v.logUnsentMessages(ctx, "NodeStatusStream", err, m)
			}
		case <-v.stop:
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
	ctx = v.taskAdd(ctx, "CalculateCapacity", true)
	defer v.taskDone(ctx, "CalculateCapacity", true)
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
	ctx := v.taskAdd(stream.Context(), "ListWorkloads", true)
	defer v.taskDone(ctx, "ListWorkloads", true)
	lsopts := &types.ListWorkloadsOptions{
		Appname:    opts.Appname,
		Entrypoint: opts.Entrypoint,
		Nodename:   opts.Nodename,
		Limit:      opts.Limit,
		Labels:     opts.Labels,
	}
	workloads, err := v.cluster.ListWorkloads(ctx, lsopts)
	if err != nil {
		return grpcstatus.Error(ListWorkloads, err.Error())
	}

	for _, c := range toRPCWorkloads(ctx, workloads, opts.Labels).Workloads {
		if err = stream.Send(c); err != nil {
			v.logUnsentMessages(ctx, "ListWorkloads", err, c)
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
	ctx = v.taskAdd(ctx, "GetWorkloadsStatus", false)
	defer v.taskDone(ctx, "GetWorkloadsStatus", false)

	workloadsStatus, err := v.cluster.GetWorkloadsStatus(ctx, opts.Ids)
	if err != nil {
		return nil, grpcstatus.Error(GetWorkloadsStatus, err.Error())
	}
	return toRPCWorkloadsStatus(workloadsStatus), nil
}

// SetWorkloadsStatus set workloads status
func (v *Vibranium) SetWorkloadsStatus(ctx context.Context, opts *pb.SetWorkloadsStatusOptions) (*pb.WorkloadsStatus, error) {
	ctx = v.taskAdd(ctx, "SetWorkloadsStatus", false)
	defer v.taskDone(ctx, "SetWorkloadsStatus", false)

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

			Appname:    status.Appname,
			Nodename:   status.Nodename,
			Entrypoint: status.Entrypoint,
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
	ctx := v.taskAdd(stream.Context(), "WorkloadStatusStream", true)
	defer v.taskDone(ctx, "WorkloadStatusStream", true)

	log.Infof(ctx, "[rpc] WorkloadStatusStream start %s", opts.Appname)
	defer log.Infof(ctx, "[rpc] WorkloadStatusStream stop %s", opts.Appname)

	ch := v.cluster.WorkloadStatusStream(
		ctx,
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
				if workload, err := toRPCWorkload(ctx, m.Workload); err != nil {
					r.Error = err.Error()
				} else {
					r.Workload = workload
					r.Status = toRPCWorkloadStatus(m.Workload.StatusMeta)
				}
			}
			if err := stream.Send(r); err != nil {
				v.logUnsentMessages(ctx, "WorkloadStatusStream", err, m)
			}
		case <-v.stop:
			return nil
		}
	}
}

// Copy copy files from multiple workloads
func (v *Vibranium) Copy(opts *pb.CopyOptions, stream pb.CoreRPC_CopyServer) error {
	ctx := v.taskAdd(stream.Context(), "Copy", true)
	defer v.taskDone(ctx, "Copy", true)

	copyOpts := toCoreCopyOptions(opts)
	ch, err := v.cluster.Copy(ctx, copyOpts)
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
				v.logUnsentMessages(ctx, "Copy", err, m)
			}
			continue
		}

		r, w := io.Pipe()
		utils.SentryGo(func(m *types.CopyMessage) func() {
			return func() {
				var err error
				defer func() {
					w.CloseWithError(err) // nolint
				}()
				defer m.Data.Close()

				var bs []byte
				tw := tar.NewWriter(w)
				defer tw.Close()
				if bs, err = ioutil.ReadAll(m.Data); err != nil {
					log.Errorf(ctx, "[Copy] Error during extracting copy data: %v", err)
					return
				}
				header := &tar.Header{Name: m.Name, Mode: 0644, Size: int64(len(bs))}
				if err = tw.WriteHeader(header); err != nil {
					log.Errorf(ctx, "[Copy] Error during writing tarball header: %v", err)
					return
				}
				if _, err = tw.Write(bs); err != nil {
					log.Errorf(ctx, "[Copy] Error during writing tarball content: %v", err)
					return
				}
			}
		}(m))

		for {
			n, err := r.Read(p)
			if err != nil {
				if err != io.EOF {
					log.Errorf(ctx, "[Copy] Error during buffer resp: %v", err)
					msg.Error = err.Error()
					if err = stream.Send(msg); err != nil {
						v.logUnsentMessages(ctx, "Copy", err, m)
					}
				}
				break
			}
			if n > 0 {
				msg.Data = p[:n]
				if err = stream.Send(msg); err != nil {
					v.logUnsentMessages(ctx, "Copy", err, m)
				}
			}
		}
	}
	return nil
}

// Send send files to some contaienrs
func (v *Vibranium) Send(opts *pb.SendOptions, stream pb.CoreRPC_SendServer) error {
	ctx := v.taskAdd(stream.Context(), "Send", true)
	defer v.taskDone(ctx, "Send", true)

	sendOpts, err := toCoreSendOptions(opts)
	if err != nil {
		return grpcstatus.Error(Send, err.Error())
	}

	sendOpts.Data = opts.Data
	ch, err := v.cluster.Send(ctx, sendOpts)
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
			v.logUnsentMessages(ctx, "Send", err, m)
		}
	}
	return nil
}

// BuildImage streamed returned functions
func (v *Vibranium) BuildImage(opts *pb.BuildImageOptions, stream pb.CoreRPC_BuildImageServer) error {
	ctx := v.taskAdd(stream.Context(), "BuildImage", true)
	defer v.taskDone(ctx, "BuildImage", true)

	buildOpts, err := toCoreBuildOptions(opts)
	if err != nil {
		return grpcstatus.Error(BuildImage, err.Error())
	}
	ch, err := v.cluster.BuildImage(ctx, buildOpts)
	if err != nil {
		return grpcstatus.Error(BuildImage, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCBuildImageMessage(m)); err != nil {
			v.logUnsentMessages(ctx, "BuildImage", err, m)
		}
	}
	return nil
}

// CacheImage cache image
func (v *Vibranium) CacheImage(opts *pb.CacheImageOptions, stream pb.CoreRPC_CacheImageServer) error {
	ctx := v.taskAdd(stream.Context(), "CacheImage", true)
	defer v.taskDone(ctx, "CacheImage", true)

	ch, err := v.cluster.CacheImage(ctx, toCoreCacheImageOptions(opts))
	if err != nil {
		return grpcstatus.Error(CacheImage, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCCacheImageMessage(m)); err != nil {
			v.logUnsentMessages(ctx, "CacheImage", err, m)
		}
	}
	return nil
}

// RemoveImage remove image
func (v *Vibranium) RemoveImage(opts *pb.RemoveImageOptions, stream pb.CoreRPC_RemoveImageServer) error {
	ctx := v.taskAdd(stream.Context(), "RemoveImage", true)
	defer v.taskDone(ctx, "RemoveImage", true)

	ch, err := v.cluster.RemoveImage(ctx, toCoreRemoveImageOptions(opts))
	if err != nil {
		return grpcstatus.Error(RemoveImage, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCRemoveImageMessage(m)); err != nil {
			v.logUnsentMessages(ctx, "RemoveImage", err, m)
		}
	}
	return nil
}

// CreateWorkload create workloads
func (v *Vibranium) CreateWorkload(opts *pb.DeployOptions, stream pb.CoreRPC_CreateWorkloadServer) error {
	ctx := v.taskAdd(stream.Context(), "CreateWorkload", true)
	defer v.taskDone(ctx, "CreateWorkload", true)

	deployOpts, err := toCoreDeployOptions(opts)
	if err != nil {
		return grpcstatus.Error(CreateWorkload, err.Error())
	}

	ch, err := v.cluster.CreateWorkload(ctx, deployOpts)
	if err != nil {
		return grpcstatus.Error(CreateWorkload, err.Error())
	}
	for m := range ch {
		if err = stream.Send(toRPCCreateWorkloadMessage(m)); err != nil {
			v.logUnsentMessages(ctx, "CreateWorkload", err, m)
		}
	}
	return nil
}

// ReplaceWorkload replace workloads
func (v *Vibranium) ReplaceWorkload(opts *pb.ReplaceOptions, stream pb.CoreRPC_ReplaceWorkloadServer) error {
	ctx := v.taskAdd(stream.Context(), "ReplaceWorkload", true)
	defer v.taskDone(ctx, "ReplaceWorkload", true)

	replaceOpts, err := toCoreReplaceOptions(opts)
	if err != nil {
		return grpcstatus.Error(ReplaceWorkload, err.Error())
	}

	ch, err := v.cluster.ReplaceWorkload(ctx, replaceOpts)
	if err != nil {
		return grpcstatus.Error(ReplaceWorkload, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCReplaceWorkloadMessage(m)); err != nil {
			v.logUnsentMessages(ctx, "ReplaceWorkload", err, m)
		}
	}
	return nil
}

// RemoveWorkload remove workloads
func (v *Vibranium) RemoveWorkload(opts *pb.RemoveWorkloadOptions, stream pb.CoreRPC_RemoveWorkloadServer) error {
	ctx := v.taskAdd(stream.Context(), "RemoveWorkload", true)
	defer v.taskDone(ctx, "RemoveWorkload", true)

	ids := opts.GetIds()
	force := opts.GetForce()
	step := int(opts.GetStep())

	if len(ids) == 0 {
		return types.ErrNoWorkloadIDs
	}
	ch, err := v.cluster.RemoveWorkload(ctx, ids, force, step)
	if err != nil {
		return grpcstatus.Error(ReplaceWorkload, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCRemoveWorkloadMessage(m)); err != nil {
			v.logUnsentMessages(ctx, "RemoveWorkload", err, m)
		}
	}

	return nil
}

// DissociateWorkload dissociate workload
func (v *Vibranium) DissociateWorkload(opts *pb.DissociateWorkloadOptions, stream pb.CoreRPC_DissociateWorkloadServer) error {
	ctx := v.taskAdd(stream.Context(), "DissociateWorkload", true)
	defer v.taskDone(ctx, "DissociateWorkload", true)

	ids := opts.GetIds()
	if len(ids) == 0 {
		return types.ErrNoWorkloadIDs
	}

	ch, err := v.cluster.DissociateWorkload(ctx, ids)
	if err != nil {
		return grpcstatus.Error(DissociateWorkload, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCDissociateWorkloadMessage(m)); err != nil {
			v.logUnsentMessages(ctx, "DissociateWorkload", err, m)
		}
	}

	return nil
}

// ControlWorkload control workloads
func (v *Vibranium) ControlWorkload(opts *pb.ControlWorkloadOptions, stream pb.CoreRPC_ControlWorkloadServer) error {
	ctx := v.taskAdd(stream.Context(), "ControlWorkload", true)
	defer v.taskDone(ctx, "ControlWorkload", true)

	ids := opts.GetIds()
	t := opts.GetType()
	force := opts.GetForce()

	if len(ids) == 0 {
		return types.ErrNoWorkloadIDs
	}

	ch, err := v.cluster.ControlWorkload(ctx, ids, t, force)
	if err != nil {
		return grpcstatus.Error(ControlWorkload, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCControlWorkloadMessage(m)); err != nil {
			v.logUnsentMessages(ctx, "ControlWorkload", err, m)
		}
	}

	return nil
}

// ExecuteWorkload runs a command in a running workload
func (v *Vibranium) ExecuteWorkload(stream pb.CoreRPC_ExecuteWorkloadServer) error {
	ctx := v.taskAdd(stream.Context(), "ExecuteWorkload", true)
	defer v.taskDone(ctx, "ExecuteWorkload", true)

	opts, err := stream.Recv()
	if err != nil {
		return grpcstatus.Error(ExecuteWorkload, err.Error())
	}
	var executeWorkloadOpts *types.ExecuteWorkloadOptions
	if executeWorkloadOpts, err = toCoreExecuteWorkloadOptions(opts); err != nil {
		return grpcstatus.Error(ExecuteWorkload, err.Error())
	}

	inCh := make(chan []byte)
	utils.SentryGo(func() {
		defer close(inCh)
		if opts.OpenStdin {
			for {
				execWorkloadOpt, err := stream.Recv()
				if execWorkloadOpt == nil || err != nil {
					log.Errorf(ctx, "[ExecuteWorkload] Recv command error: %v", err)
					return
				}
				inCh <- execWorkloadOpt.ReplCmd
			}
		}
	})

	for m := range v.cluster.ExecuteWorkload(ctx, executeWorkloadOpts, inCh) {
		if err = stream.Send(toRPCAttachWorkloadMessage(m)); err != nil {
			v.logUnsentMessages(ctx, "ExecuteWorkload", err, m)
		}
	}
	return nil
}

// ReallocResource realloc res for workloads
func (v *Vibranium) ReallocResource(ctx context.Context, opts *pb.ReallocOptions) (msg *pb.ReallocResourceMessage, err error) {
	ctx = v.taskAdd(ctx, "ReallocResource", true)
	defer v.taskDone(ctx, "ReallocResource", true)
	defer func() {
		errString := ""
		if err != nil {
			errString = err.Error()
		}
		msg = &pb.ReallocResourceMessage{Error: errString}
	}()

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
				StorageRequest:  opts.ResourceOpts.StorageRequest + vbsRequest.TotalSize(),
				StorageLimit:    opts.ResourceOpts.StorageLimit + vbsLimit.TotalSize(),
			},
		},
	); err != nil {
		return msg, grpcstatus.Error(ReallocResource, err.Error())
	}

	return msg, nil
}

// LogStream get workload logs
func (v *Vibranium) LogStream(opts *pb.LogStreamOptions, stream pb.CoreRPC_LogStreamServer) error {
	ctx := v.taskAdd(stream.Context(), "LogStream", true)
	defer v.taskDone(ctx, "LogStream", true)

	ID := opts.GetId()
	log.Infof(ctx, "[LogStream] Get %s log start", ID)
	defer log.Infof(ctx, "[LogStream] Get %s log done", ID)
	ch, err := v.cluster.LogStream(ctx, &types.LogStreamOptions{
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
				v.logUnsentMessages(ctx, "LogStream", err, m)
			}
		case <-v.stop:
			return nil
		}
	}
}

// RunAndWait is lambda
func (v *Vibranium) RunAndWait(stream pb.CoreRPC_RunAndWaitServer) error {
	ctx := v.taskAdd(stream.Context(), "RunAndWait", true)
	RunAndWaitOptions, err := stream.Recv()
	if err != nil {
		v.taskDone(ctx, "RunAndWait", true)
		return grpcstatus.Error(RunAndWait, err.Error())
	}

	if RunAndWaitOptions.DeployOptions == nil {
		v.taskDone(ctx, "RunAndWait", true)
		return grpcstatus.Error(RunAndWait, types.ErrNoDeployOpts.Error())
	}

	opts := RunAndWaitOptions.DeployOptions
	deployOpts, err := toCoreDeployOptions(opts)
	if err != nil {
		v.taskDone(ctx, "RunAndWait", true)
		return grpcstatus.Error(RunAndWait, err.Error())
	}

	ctx, cancel := context.WithCancel(ctx)
	if RunAndWaitOptions.Async {
		timeout := v.config.GlobalTimeout
		if RunAndWaitOptions.AsyncTimeout != 0 {
			timeout = time.Second * time.Duration(RunAndWaitOptions.AsyncTimeout)
		}
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		// force mark stdin to false
		opts.OpenStdin = false
	}

	inCh := make(chan []byte)
	utils.SentryGo(func() {
		defer close(inCh)
		if !opts.OpenStdin {
			return
		}
		for {
			RunAndWaitOptions, err := stream.Recv()
			if RunAndWaitOptions == nil || err != nil {
				log.Errorf(ctx, "[RunAndWait] Recv command error: %v", err)
				break
			}
			inCh <- RunAndWaitOptions.Cmd
		}
	})

	ids, ch, err := v.cluster.RunAndWait(ctx, deployOpts, inCh)
	if err != nil {
		v.taskDone(ctx, "RunAndWait", true)
		cancel()
		return grpcstatus.Error(RunAndWait, err.Error())
	}

	// send workload ids to client first
	for _, id := range ids {
		if err = stream.Send(&pb.AttachWorkloadMessage{
			WorkloadId:    id,
			Data:          []byte(""),
			StdStreamType: pb.StdStreamType_TYPEWORKLOADID,
		}); err != nil {
			v.logUnsentMessages(ctx, "RunAndWait: first message send failed", err, id)
		}
	}

	// then deal with the rest messages
	runAndWait := func(f func(<-chan *types.AttachWorkloadMessage)) {
		defer v.taskDone(ctx, "RunAndWait", true)
		defer cancel()
		f(ch)
	}

	if !RunAndWaitOptions.Async {
		runAndWait(func(ch <-chan *types.AttachWorkloadMessage) {
			for m := range ch {
				if err = stream.Send(toRPCAttachWorkloadMessage(m)); err != nil {
					v.logUnsentMessages(ctx, "RunAndWait", err, m)
				}
			}
		})
		return nil
	}

	utils.SentryGo(func() {
		runAndWait(func(ch <-chan *types.AttachWorkloadMessage) {
			r, w := io.Pipe()
			utils.SentryGo(func() {
				defer w.Close()
				for m := range ch {
					if _, err := w.Write(m.Data); err != nil {
						log.Errorf(ctx, "[Async RunAndWait] iterate and forward AttachWorkloadMessage error: %v", err)
					}
				}
			})
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
							log.Errorf(ctx, "[Aysnc RunAndWait] read error: %+v", err)
						}
						return
					}
					line = append(line, part...)
					if !isPrefix {
						break
					}
				}
				log.Infof(ctx, "[Async RunAndWait] %s", line)
			}
		})
	})
	return nil
}

func (v *Vibranium) logUnsentMessages(ctx context.Context, msgType string, err error, msg interface{}) {
	log.Infof(ctx, "[logUnsentMessages] Unsent (%s) streamed message due to (%+v): (%v)", msgType, err, msg)
}

// New will new a new cluster instance
func New(cluster cluster.Cluster, config types.Config, stop chan struct{}) *Vibranium {
	return &Vibranium{cluster: cluster, config: config, counter: sync.WaitGroup{}, stop: stop}
}
