package rpc

import (
	"archive/tar"
	"bufio"
	"fmt"
	"io"
	"path/filepath"
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
func (v *Vibranium) Info(context.Context, *pb.Empty) (*pb.CoreInfo, error) {
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
	task := v.newTask(stream.Context(), "WatchServiceStatus", false)
	defer task.done()
	ch, err := v.cluster.WatchServiceStatus(task.context)
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
				v.logUnsentMessages(task.context, "WatchServicesStatus", err, s)
				return grpcstatus.Error(WatchServiceStatus, err.Error())
			}
		case <-task.context.Done():
			return nil
		case <-v.stop:
			return nil
		}
	}
}

// ListNetworks list networks for pod
func (v *Vibranium) ListNetworks(ctx context.Context, opts *pb.ListNetworkOptions) (*pb.Networks, error) {
	task := v.newTask(ctx, "ListNetworks", false)
	defer task.done()
	networks, err := v.cluster.ListNetworks(task.context, opts.Podname, opts.Driver)
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
	task := v.newTask(ctx, "ConnectNetwork", false)
	defer task.done()
	subnets, err := v.cluster.ConnectNetwork(task.context, opts.Network, opts.Target, opts.Ipv4, opts.Ipv6)
	if err != nil {
		return nil, grpcstatus.Error(ConnectNetwork, err.Error())
	}
	return &pb.Network{Name: opts.Network, Subnets: subnets}, nil
}

// DisconnectNetwork disconnect network
func (v *Vibranium) DisconnectNetwork(ctx context.Context, opts *pb.DisconnectNetworkOptions) (*pb.Empty, error) {
	task := v.newTask(ctx, "DisconnectNetwork", false)
	defer task.done()
	if err := v.cluster.DisconnectNetwork(task.context, opts.Network, opts.Target, opts.Force); err != nil {
		return nil, grpcstatus.Error(DisconnectNetwork, err.Error())
	}
	return &pb.Empty{}, nil
}

// AddPod saves a pod, and returns it to client
func (v *Vibranium) AddPod(ctx context.Context, opts *pb.AddPodOptions) (*pb.Pod, error) {
	task := v.newTask(ctx, "AddPod", false)
	defer task.done()
	p, err := v.cluster.AddPod(task.context, opts.Name, opts.Desc)
	if err != nil {
		return nil, grpcstatus.Error(AddPod, err.Error())
	}

	return toRPCPod(p), nil
}

// RemovePod removes a pod only if it's empty
func (v *Vibranium) RemovePod(ctx context.Context, opts *pb.RemovePodOptions) (*pb.Empty, error) {
	task := v.newTask(ctx, "RemovePod", false)
	defer task.done()
	if err := v.cluster.RemovePod(task.context, opts.Name); err != nil {
		return nil, grpcstatus.Error(RemovePod, err.Error())
	}
	return &pb.Empty{}, nil
}

// GetPod show a pod
func (v *Vibranium) GetPod(ctx context.Context, opts *pb.GetPodOptions) (*pb.Pod, error) {
	task := v.newTask(ctx, "GetPod", false)
	defer task.done()
	p, err := v.cluster.GetPod(task.context, opts.Name)
	if err != nil {
		return nil, grpcstatus.Error(GetPod, err.Error())
	}

	return toRPCPod(p), nil
}

// ListPods returns a list of pods
func (v *Vibranium) ListPods(ctx context.Context, _ *pb.Empty) (*pb.Pods, error) {
	task := v.newTask(ctx, "ListPods", false)
	defer task.done()
	ps, err := v.cluster.ListPods(task.context)
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
func (v *Vibranium) GetPodResource(opts *pb.GetPodOptions, stream pb.CoreRPC_GetPodResourceServer) error {
	task := v.newTask(stream.Context(), "GetPodResource", false)
	defer task.done()
	ch, err := v.cluster.PodResource(task.context, opts.Name)
	if err != nil {
		return grpcstatus.Error(PodResource, err.Error())
	}
	for msg := range ch {
		if err := stream.Send(toRPCNodeResource(msg)); err != nil {
			v.logUnsentMessages(task.context, "GetPodResource", err, msg)
		}
	}
	return nil
}

// GetNodeResource check node resource
func (v *Vibranium) GetNodeResource(ctx context.Context, opts *pb.GetNodeResourceOptions) (*pb.NodeResource, error) {
	task := v.newTask(ctx, "GetNodeResource", false)
	defer task.done()
	nr, err := v.cluster.NodeResource(task.context, opts.GetOpts().Nodename, opts.Fix)
	if err != nil {
		return nil, grpcstatus.Error(GetNodeResource, err.Error())
	}

	return toRPCNodeResource(nr), nil
}

// AddNode saves a node and returns it to client
// Method must be called synchronously, or nothing will be returned
func (v *Vibranium) AddNode(ctx context.Context, opts *pb.AddNodeOptions) (*pb.Node, error) {
	task := v.newTask(ctx, "AddNode", false)
	defer task.done()
	addNodeOpts := toCoreAddNodeOptions(opts)
	n, err := v.cluster.AddNode(task.context, addNodeOpts)
	if err != nil {
		return nil, grpcstatus.Error(AddNode, err.Error())
	}

	return toRPCNode(n), nil
}

// RemoveNode removes the node from etcd
func (v *Vibranium) RemoveNode(ctx context.Context, opts *pb.RemoveNodeOptions) (*pb.Empty, error) {
	task := v.newTask(ctx, "RemoveNode", false)
	defer task.done()
	if err := v.cluster.RemoveNode(task.context, opts.Nodename); err != nil {
		return nil, grpcstatus.Error(RemoveNode, err.Error())
	}
	return &pb.Empty{}, nil
}

// ListPodNodes returns a list of node for pod
func (v *Vibranium) ListPodNodes(opts *pb.ListNodesOptions, stream pb.CoreRPC_ListPodNodesServer) error {
	task := v.newTask(stream.Context(), "ListPodNodes", false)
	defer task.done()

	timeout := time.Duration(opts.TimeoutInSecond) * time.Second
	if opts.TimeoutInSecond <= 0 {
		timeout = v.config.ConnectionTimeout
	}
	ctx, cancel := context.WithTimeout(task.context, timeout)
	defer cancel()

	ch, err := v.cluster.ListPodNodes(ctx, toCoreListNodesOptions(opts))
	if err != nil {
		return grpcstatus.Error(ListPodNodes, err.Error())
	}

	for msg := range ch {
		if err := stream.Send(toRPCNode(msg)); err != nil {
			v.logUnsentMessages(task.context, "PodNodesStream", err, msg)
		}
	}
	return nil
}

// GetNode get a node
func (v *Vibranium) GetNode(ctx context.Context, opts *pb.GetNodeOptions) (*pb.Node, error) {
	task := v.newTask(ctx, "GetNode", false)
	defer task.done()
	n, err := v.cluster.GetNode(task.context, opts.Nodename)
	if err != nil {
		return nil, grpcstatus.Error(GetNode, err.Error())
	}

	return toRPCNode(n), nil
}

// GetNodeEngine get a node engine
func (v *Vibranium) GetNodeEngineInfo(ctx context.Context, opts *pb.GetNodeOptions) (*pb.Engine, error) {
	task := v.newTask(ctx, "GetNodeEngine", false)
	defer task.done()
	e, err := v.cluster.GetNodeEngineInfo(task.context, opts.Nodename)
	if err != nil {
		return nil, grpcstatus.Error(GetNodeEngine, err.Error())
	}

	return toRPCEngine(e), nil
}

// SetNode set node meta
func (v *Vibranium) SetNode(ctx context.Context, opts *pb.SetNodeOptions) (*pb.Node, error) {
	task := v.newTask(ctx, "SetNode", false)
	defer task.done()
	setNodeOpts, err := toCoreSetNodeOptions(opts)
	if err != nil {
		return nil, grpcstatus.Error(SetNode, err.Error())
	}
	n, err := v.cluster.SetNode(task.context, setNodeOpts)
	if err != nil {
		return nil, grpcstatus.Error(SetNode, err.Error())
	}
	return toRPCNode(n), nil
}

// GetNodeStatus set status of a node for reporting
func (v *Vibranium) GetNodeStatus(ctx context.Context, opts *pb.GetNodeStatusOptions) (*pb.NodeStatusStreamMessage, error) {
	task := v.newTask(ctx, "GetNodeStatus", false)
	defer task.done()
	status, err := v.cluster.GetNodeStatus(task.context, opts.Nodename)
	if err != nil {
		return nil, grpcstatus.Error(GetNodeStatus, err.Error())
	}
	return &pb.NodeStatusStreamMessage{
		Nodename: status.Nodename,
		Podname:  status.Podname,
		Alive:    status.Alive,
	}, nil
}

// SetNodeStatus set status of a node for reporting
func (v *Vibranium) SetNodeStatus(ctx context.Context, opts *pb.SetNodeStatusOptions) (*pb.Empty, error) {
	task := v.newTask(ctx, "SetNodeStatus", false)
	defer task.done()
	if err := v.cluster.SetNodeStatus(task.context, opts.Nodename, opts.Ttl); err != nil {
		return nil, grpcstatus.Error(SetNodeStatus, err.Error())
	}
	return &pb.Empty{}, nil
}

// NodeStatusStream watch and show deployed status
func (v *Vibranium) NodeStatusStream(_ *pb.Empty, stream pb.CoreRPC_NodeStatusStreamServer) error {
	task := v.newTask(stream.Context(), "NodeStatusStream", true)
	defer task.done()

	ch := v.cluster.NodeStatusStream(task.context)
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
				v.logUnsentMessages(task.context, "NodeStatusStream", err, m)
			}
		case <-v.stop:
			return nil
		}
	}
}

// GetWorkloadsStatus get workloads status
func (v *Vibranium) GetWorkloadsStatus(ctx context.Context, opts *pb.WorkloadIDs) (*pb.WorkloadsStatus, error) {
	task := v.newTask(ctx, "GetWorkloadsStatus", false)
	defer task.done()

	workloadsStatus, err := v.cluster.GetWorkloadsStatus(task.context, opts.IDs)
	if err != nil {
		return nil, grpcstatus.Error(GetWorkloadsStatus, err.Error())
	}
	return toRPCWorkloadsStatus(workloadsStatus), nil
}

// SetWorkloadsStatus set workloads status
func (v *Vibranium) SetWorkloadsStatus(ctx context.Context, opts *pb.SetWorkloadsStatusOptions) (*pb.WorkloadsStatus, error) {
	task := v.newTask(ctx, "SetWorkloadsStatus", false)
	defer task.done()

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

	status, err := v.cluster.SetWorkloadsStatus(task.context, statusData, ttls)
	if err != nil {
		return nil, grpcstatus.Error(SetWorkloadsStatus, err.Error())
	}
	return toRPCWorkloadsStatus(status), nil
}

// WorkloadStatusStream watch and show deployed status
func (v *Vibranium) WorkloadStatusStream(opts *pb.WorkloadStatusStreamOptions, stream pb.CoreRPC_WorkloadStatusStreamServer) error {
	task := v.newTask(stream.Context(), "WorkloadStatusStream", true)
	defer task.done()
	logger := log.WithFunc("vibranium.WorkloadStatusStream").WithField("app", opts.Appname)

	logger.Info(task.context, "WorkloadStatusStream start")
	defer logger.Info(task.context, "WorkloadStatusStream stop")

	ch := v.cluster.WorkloadStatusStream(
		task.context,
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
				if workload, err := toRPCWorkload(task.context, m.Workload); err != nil {
					r.Error = err.Error()
				} else {
					r.Workload = workload
					r.Status = toRPCWorkloadStatus(m.Workload.StatusMeta)
				}
			}
			if err := stream.Send(r); err != nil {
				v.logUnsentMessages(task.context, "WorkloadStatusStream", err, m)
			}
		case <-v.stop:
			return nil
		}
	}
}

// CalculateCapacity calculates capacity for each node
func (v *Vibranium) CalculateCapacity(ctx context.Context, opts *pb.DeployOptions) (*pb.CapacityMessage, error) {
	task := v.newTask(ctx, "CalculateCapacity", true)
	defer task.done()
	deployOpts, err := toCoreDeployOptions(opts)
	if err != nil {
		return nil, grpcstatus.Error(CalculateCapacity, err.Error())
	}
	m, err := v.cluster.CalculateCapacity(task.context, deployOpts)
	if err != nil {
		return nil, grpcstatus.Error(CalculateCapacity, err.Error())
	}
	return toRPCCapacityMessage(m), nil
}

// GetWorkload get a workload
// More information will be shown
func (v *Vibranium) GetWorkload(ctx context.Context, ID *pb.WorkloadID) (*pb.Workload, error) {
	task := v.newTask(ctx, "GetWorkload", false)
	defer task.done()
	workload, err := v.cluster.GetWorkload(task.context, ID.Id)
	if err != nil {
		return nil, grpcstatus.Error(GetWorkload, err.Error())
	}

	return toRPCWorkload(task.context, workload)
}

// GetWorkloads get lots workloads
// like GetWorkload, information should be returned
func (v *Vibranium) GetWorkloads(ctx context.Context, cids *pb.WorkloadIDs) (*pb.Workloads, error) {
	task := v.newTask(ctx, "GetWorkloads", false)
	defer task.done()
	workloads, err := v.cluster.GetWorkloads(task.context, cids.GetIDs())
	if err != nil {
		return nil, grpcstatus.Error(GetWorkloads, err.Error())
	}

	return toRPCWorkloads(task.context, workloads, nil), nil
}

// ListWorkloads by appname with optional entrypoint and nodename
func (v *Vibranium) ListWorkloads(opts *pb.ListWorkloadsOptions, stream pb.CoreRPC_ListWorkloadsServer) error {
	task := v.newTask(stream.Context(), "ListWorkloads", true)
	defer task.done()
	lsopts := &types.ListWorkloadsOptions{
		Appname:    opts.Appname,
		Entrypoint: opts.Entrypoint,
		Nodename:   opts.Nodename,
		Limit:      opts.Limit,
		Labels:     opts.Labels,
	}
	workloads, err := v.cluster.ListWorkloads(task.context, lsopts)
	if err != nil {
		return grpcstatus.Error(ListWorkloads, err.Error())
	}

	for _, c := range toRPCWorkloads(task.context, workloads, opts.Labels).Workloads {
		if err = stream.Send(c); err != nil {
			v.logUnsentMessages(task.context, "ListWorkloads", err, c)
			return grpcstatus.Error(ListWorkloads, err.Error())
		}
	}
	return nil
}

// ListNodeWorkloads list node workloads
func (v *Vibranium) ListNodeWorkloads(ctx context.Context, opts *pb.GetNodeOptions) (*pb.Workloads, error) {
	task := v.newTask(ctx, "ListNodeWorkloads", false)
	defer task.done()
	workloads, err := v.cluster.ListNodeWorkloads(task.context, opts.Nodename, opts.Labels)
	if err != nil {
		return nil, grpcstatus.Error(ListNodeWorkloads, err.Error())
	}
	return toRPCWorkloads(task.context, workloads, nil), nil
}

// Copy copy files from multiple workloads
func (v *Vibranium) Copy(opts *pb.CopyOptions, stream pb.CoreRPC_CopyServer) error {
	task := v.newTask(stream.Context(), "Copy", true)
	defer task.done()
	logger := log.WithFunc("vibranium.Copy")

	copyOpts := toCoreCopyOptions(opts)
	ch, err := v.cluster.Copy(task.context, copyOpts)
	if err != nil {
		return grpcstatus.Error(Copy, err.Error())
	}
	// 4K buffer
	p := make([]byte, 4096)
	for m := range ch {
		msg := &pb.CopyMessage{
			Id:   m.ID,
			Path: m.Path,
		}
		if m.Error != nil {
			msg.Error = m.Error.Error()
			if err := stream.Send(msg); err != nil {
				v.logUnsentMessages(task.context, "Copy", err, m)
			}
			continue
		}

		r, w := io.Pipe()
		utils.SentryGo(func(m *types.CopyMessage) func() {
			return func() {
				var err error
				defer func() {
					w.CloseWithError(err) //nolint
				}()

				tw := tar.NewWriter(w)
				defer tw.Close()
				header := &tar.Header{
					Name: filepath.Base(m.Filename),
					Uid:  m.UID,
					Gid:  m.GID,
					Mode: m.Mode,
					Size: int64(len(m.Content)),
				}
				if err = tw.WriteHeader(header); err != nil {
					logger.Error(task.context, err, "Error during writing tarball header")
					return
				}
				if _, err = tw.Write(m.Content); err != nil {
					logger.Error(task.context, err, "Error during writing tarball content")
					return
				}
			}
		}(m))

		for {
			n, err := r.Read(p)
			if err != nil {
				if err != io.EOF {
					logger.Error(task.context, err, "Error during buffer resp")
					msg.Error = err.Error()
					if err = stream.Send(msg); err != nil {
						v.logUnsentMessages(task.context, "Copy", err, m)
					}
				}
				break
			}
			if n > 0 {
				msg.Data = p[:n]
				if err = stream.Send(msg); err != nil {
					v.logUnsentMessages(task.context, "Copy", err, m)
				}
			}
		}
	}
	return nil
}

// Send send files to some contaienrs
func (v *Vibranium) Send(opts *pb.SendOptions, stream pb.CoreRPC_SendServer) error {
	task := v.newTask(stream.Context(), "Send", true)
	defer task.done()

	sendOpts, err := toCoreSendOptions(opts)
	if err != nil {
		return grpcstatus.Error(Send, err.Error())
	}

	for _, file := range sendOpts.Files {
		dc := make(chan *types.SendLargeFileOptions)
		ch := v.cluster.SendLargeFile(task.context, dc)
		utils.SentryGo(func() {
			defer close(dc)
			data := toSendLargeFileChunks(file, sendOpts.IDs)
			for _, chunk := range data {
				dc <- chunk
			}
		})

		for m := range ch {
			msg := &pb.SendMessage{
				Id:   m.ID,
				Path: m.Path,
			}
			if m.Error != nil {
				msg.Error = m.Error.Error()
			}
			if err := stream.Send(msg); err != nil {
				v.logUnsentMessages(task.context, "Send", err, m)
			}
		}
	}
	return nil
}

func (v *Vibranium) SendLargeFile(stream pb.CoreRPC_SendLargeFileServer) error {
	task := v.newTask(stream.Context(), "SendLargeFile", true)
	defer task.done()

	inputChan := make(chan *types.SendLargeFileOptions)
	resp := v.cluster.SendLargeFile(task.context, inputChan)
	utils.SentryGo(func() {
		defer close(inputChan)
		for {
			req, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Errorf(task.context, err, "[SendLargeFile]receive from rpc err: %v", err)
				return
			}
			data, err := toSendLargeFileOptions(req)
			if err != nil {
				log.Errorf(task.context, err, "[SendLargeFile]transform data err: %v", err)
				return
			}
			inputChan <- data
		}
	})

	for m := range resp {
		msg := &pb.SendMessage{
			Id:   m.ID,
			Path: m.Path,
		}
		if m.Error != nil {
			msg.Error = m.Error.Error()
		}
		if err := stream.Send(msg); err != nil {
			v.logUnsentMessages(task.context, "SendLargeFile", err, m)
		}
	}
	return nil
}

// BuildImage streamed returned functions
func (v *Vibranium) BuildImage(opts *pb.BuildImageOptions, stream pb.CoreRPC_BuildImageServer) error {
	task := v.newTask(stream.Context(), "BuildImage", true)
	defer task.done()

	buildOpts, err := toCoreBuildOptions(opts)
	if err != nil {
		return grpcstatus.Error(BuildImage, err.Error())
	}
	ch, err := v.cluster.BuildImage(task.context, buildOpts)
	if err != nil {
		return grpcstatus.Error(BuildImage, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCBuildImageMessage(m)); err != nil {
			v.logUnsentMessages(task.context, "BuildImage", err, m)
		}
	}
	return nil
}

// CacheImage cache image
func (v *Vibranium) CacheImage(opts *pb.CacheImageOptions, stream pb.CoreRPC_CacheImageServer) error {
	task := v.newTask(stream.Context(), "CacheImage", true)
	defer task.done()

	ch, err := v.cluster.CacheImage(task.context, toCoreCacheImageOptions(opts))
	if err != nil {
		return grpcstatus.Error(CacheImage, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCCacheImageMessage(m)); err != nil {
			v.logUnsentMessages(task.context, "CacheImage", err, m)
		}
	}
	return nil
}

// RemoveImage remove image
func (v *Vibranium) RemoveImage(opts *pb.RemoveImageOptions, stream pb.CoreRPC_RemoveImageServer) error {
	task := v.newTask(stream.Context(), "RemoveImage", true)
	defer task.done()

	ch, err := v.cluster.RemoveImage(task.context, toCoreRemoveImageOptions(opts))
	if err != nil {
		return grpcstatus.Error(RemoveImage, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCRemoveImageMessage(m)); err != nil {
			v.logUnsentMessages(task.context, "RemoveImage", err, m)
		}
	}
	return nil
}

// ListImage list image
func (v *Vibranium) ListImage(opts *pb.ListImageOptions, stream pb.CoreRPC_ListImageServer) error {
	task := v.newTask(stream.Context(), "ListImage", true)
	defer task.done()

	ch, err := v.cluster.ListImage(task.context, toCoreListImageOptions(opts))
	if err != nil {
		return grpcstatus.Error(ListImage, err.Error())
	}

	for msg := range ch {
		if err = stream.Send(toRPCListImageMessage(msg)); err != nil {
			v.logUnsentMessages(task.context, "ListImage", err, msg)
		}
	}

	return nil
}

// CreateWorkload create workloads
func (v *Vibranium) CreateWorkload(opts *pb.DeployOptions, stream pb.CoreRPC_CreateWorkloadServer) error {
	task := v.newTask(stream.Context(), "CreateWorkload", true)
	defer task.done()

	deployOpts, err := toCoreDeployOptions(opts)
	if err != nil {
		return grpcstatus.Error(CreateWorkload, err.Error())
	}

	ch, err := v.cluster.CreateWorkload(task.context, deployOpts)
	if err != nil {
		return grpcstatus.Error(CreateWorkload, err.Error())
	}
	for m := range ch {
		log.WithFunc("vibranium.CreateWorkload").Debugf(task.context, "create workload message: %+v", m)
		if err = stream.Send(toRPCCreateWorkloadMessage(m)); err != nil {
			v.logUnsentMessages(task.context, "CreateWorkload", err, m)
		}
	}
	return nil
}

// ReplaceWorkload replace workloads
func (v *Vibranium) ReplaceWorkload(opts *pb.ReplaceOptions, stream pb.CoreRPC_ReplaceWorkloadServer) error {
	task := v.newTask(stream.Context(), "ReplaceWorkload", true)
	defer task.done()

	replaceOpts, err := toCoreReplaceOptions(opts)
	if err != nil {
		return grpcstatus.Error(ReplaceWorkload, err.Error())
	}

	ch, err := v.cluster.ReplaceWorkload(task.context, replaceOpts)
	if err != nil {
		return grpcstatus.Error(ReplaceWorkload, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCReplaceWorkloadMessage(m)); err != nil {
			v.logUnsentMessages(task.context, "ReplaceWorkload", err, m)
		}
	}
	return nil
}

// RemoveWorkload remove workloads
func (v *Vibranium) RemoveWorkload(opts *pb.RemoveWorkloadOptions, stream pb.CoreRPC_RemoveWorkloadServer) error {
	task := v.newTask(stream.Context(), "RemoveWorkload", true)
	defer task.done()

	IDs := opts.GetIDs()
	force := opts.GetForce()

	if len(IDs) == 0 {
		return types.ErrNoWorkloadIDs
	}
	ch, err := v.cluster.RemoveWorkload(task.context, IDs, force)
	if err != nil {
		return grpcstatus.Error(ReplaceWorkload, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCRemoveWorkloadMessage(m)); err != nil {
			v.logUnsentMessages(task.context, "RemoveWorkload", err, m)
		}
	}

	return nil
}

// DissociateWorkload dissociate workload
func (v *Vibranium) DissociateWorkload(opts *pb.DissociateWorkloadOptions, stream pb.CoreRPC_DissociateWorkloadServer) error {
	task := v.newTask(stream.Context(), "DissociateWorkload", true)
	defer task.done()

	IDs := opts.GetIDs()
	if len(IDs) == 0 {
		return types.ErrNoWorkloadIDs
	}

	ch, err := v.cluster.DissociateWorkload(task.context, IDs)
	if err != nil {
		return grpcstatus.Error(DissociateWorkload, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCDissociateWorkloadMessage(m)); err != nil {
			v.logUnsentMessages(task.context, "DissociateWorkload", err, m)
		}
	}

	return nil
}

// ControlWorkload control workloads
func (v *Vibranium) ControlWorkload(opts *pb.ControlWorkloadOptions, stream pb.CoreRPC_ControlWorkloadServer) error {
	task := v.newTask(stream.Context(), "ControlWorkload", true)
	defer task.done()

	IDs := opts.GetIDs()
	t := opts.GetType()
	force := opts.GetForce()

	if len(IDs) == 0 {
		return types.ErrNoWorkloadIDs
	}

	ch, err := v.cluster.ControlWorkload(task.context, IDs, t, force)
	if err != nil {
		return grpcstatus.Error(ControlWorkload, err.Error())
	}

	for m := range ch {
		if err = stream.Send(toRPCControlWorkloadMessage(m)); err != nil {
			v.logUnsentMessages(task.context, "ControlWorkload", err, m)
		}
	}

	return nil
}

// ExecuteWorkload runs a command in a running workload
func (v *Vibranium) ExecuteWorkload(stream pb.CoreRPC_ExecuteWorkloadServer) error {
	task := v.newTask(stream.Context(), "ExecuteWorkload", true)
	defer task.done()

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
					log.WithFunc("vibranium.ExecuteWorkload").Error(task.context, err, "Recv command error")
					return
				}
				inCh <- execWorkloadOpt.ReplCmd
			}
		}
	})

	for m := range v.cluster.ExecuteWorkload(task.context, executeWorkloadOpts, inCh) {
		if err = stream.Send(toRPCAttachWorkloadMessage(m)); err != nil {
			v.logUnsentMessages(task.context, "ExecuteWorkload", err, m)
		}
	}
	return nil
}

// ReallocResource realloc res for workloads
func (v *Vibranium) ReallocResource(ctx context.Context, opts *pb.ReallocOptions) (msg *pb.ReallocResourceMessage, err error) {
	task := v.newTask(ctx, "ReallocResource", true)
	defer task.done()
	defer func() {
		errString := ""
		if err != nil {
			errString = err.Error()
		}
		msg = &pb.ReallocResourceMessage{Error: errString}
	}()

	if opts.Id == "" {
		return msg, grpcstatus.Errorf(ReallocResource, "%+v", types.ErrNoWorkloadIDs)
	}

	if err := v.cluster.ReallocResource(
		ctx,
		&types.ReallocOptions{
			ID:        opts.Id,
			Resources: toCoreResources(opts.Resources),
		},
	); err != nil {
		return msg, grpcstatus.Error(ReallocResource, err.Error())
	}

	return msg, nil
}

// LogStream get workload logs
func (v *Vibranium) LogStream(opts *pb.LogStreamOptions, stream pb.CoreRPC_LogStreamServer) error {
	task := v.newTask(stream.Context(), "LogStream", true)
	defer task.done()

	ID := opts.GetId()
	logger := log.WithFunc("vibranium.LogStream").WithField("ID", ID)

	logger.Info(task.context, "Get log start")
	defer logger.Info(task.context, "Get log done")
	ch, err := v.cluster.LogStream(task.context, &types.LogStreamOptions{
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
				v.logUnsentMessages(task.context, "LogStream", err, m)
			}
		case <-v.stop:
			return nil
		}
	}
}

// RunAndWait is lambda
func (v *Vibranium) RunAndWait(stream pb.CoreRPC_RunAndWaitServer) error {
	task := v.newTask(stream.Context(), "RunAndWait", true)
	RunAndWaitOptions, err := stream.Recv()
	if err != nil {
		task.done()
		return grpcstatus.Error(RunAndWait, err.Error())
	}
	logger := log.WithFunc("vibranium.RunAndWait")

	if RunAndWaitOptions.DeployOptions == nil {
		task.done()
		return grpcstatus.Error(RunAndWait, types.ErrNoDeployOpts.Error())
	}

	opts := RunAndWaitOptions.DeployOptions
	deployOpts, err := toCoreDeployOptions(opts)
	if err != nil {
		task.done()
		return grpcstatus.Error(RunAndWait, err.Error())
	}

	ctx, cancel := context.WithCancel(task.context)
	if RunAndWaitOptions.Async {
		timeout := v.config.GlobalTimeout
		if RunAndWaitOptions.AsyncTimeout != 0 {
			timeout = time.Second * time.Duration(RunAndWaitOptions.AsyncTimeout)
		}
		ctx, cancel = context.WithTimeout(context.TODO(), timeout) // not use the stream ctx
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
				logger.Error(ctx, err, "Recv command")
				break
			}
			inCh <- RunAndWaitOptions.Cmd
		}
	})

	IDs, ch, err := v.cluster.RunAndWait(ctx, deployOpts, inCh)
	if err != nil {
		task.done()
		cancel()
		return grpcstatus.Error(RunAndWait, err.Error())
	}

	// send workload IDs to client first
	for _, ID := range IDs {
		if err = stream.Send(&pb.AttachWorkloadMessage{
			WorkloadId:    ID,
			Data:          []byte(""),
			StdStreamType: pb.StdStreamType_TYPEWORKLOADID,
		}); err != nil {
			v.logUnsentMessages(ctx, "RunAndWait: first message send failed", err, ID)
		}
	}

	// then deal with the rest messages
	runAndWait := func(f func(<-chan *types.AttachWorkloadMessage)) {
		defer task.done()
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
						logger.Error(ctx, err, "iterate and forward AttachWorkloadMessage")
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
							logger.Error(ctx, err, "read error")
						}
						return
					}
					line = append(line, part...)
					if !isPrefix {
						break
					}
				}
				logger.Info(ctx, line)
			}
		})
	})
	return nil
}

func (v *Vibranium) RawEngine(ctx context.Context, opts *pb.RawEngineOptions) (*pb.RawEngineMessage, error) {
	task := v.newTask(ctx, "RawEngine", true)
	defer task.done()

	rawEngineOpts, err := toCoreRawEngineOptions(opts)
	if err != nil {
		return nil, grpcstatus.Error(RawEngineStatus, err.Error())
	}

	msg, err := v.cluster.RawEngine(task.context, rawEngineOpts)

	if err != nil {
		return nil, grpcstatus.Error(RawEngineStatus, err.Error())
	}
	return toRPCRawEngineMessage(msg), nil
}

func (v *Vibranium) logUnsentMessages(ctx context.Context, msgType string, err error, msg any) {
	log.WithFunc("vibranium.logUnsentMessages").Infof(ctx, "Unsent (%s) streamed message due to (%+v): (%+v)", msgType, err, msg)
}

// New will new a new cluster instance
func New(cluster cluster.Cluster, config types.Config, stop chan struct{}) *Vibranium {
	return &Vibranium{cluster: cluster, config: config, counter: sync.WaitGroup{}, stop: stop}
}
