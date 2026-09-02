package rpc

import (
	"archive/tar"
	"bufio"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"
	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/projecteru2/core/version"
)

const copyChunkSize = 64 << 10

type (
	sender[R any]       func(R) error
	converter[T, R any] func(T) R
)

// Vibranium implements the CoreRPC gRPC server.
type Vibranium struct {
	cluster cluster.Cluster
	config  types.Config
	counter sync.WaitGroup
	stop    chan struct{}
}

// New returns a Vibranium serving cluster.
func New(cluster cluster.Cluster, config types.Config, stop chan struct{}) *Vibranium {
	return &Vibranium{cluster: cluster, config: config, stop: stop}
}

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
				logUnsentMessages(task.context, "WatchServiceStatus", err, s)
				return grpcstatus.Error(WatchServiceStatus, err.Error())
			}
		case <-task.context.Done():
			return nil
		case <-v.stop:
			return nil
		}
	}
}

func (v *Vibranium) ListNetworks(ctx context.Context, opts *pb.ListNetworkOptions) (*pb.Networks, error) {
	task := v.newTask(ctx, "ListNetworks", false)
	defer task.done()
	networks, err := v.cluster.ListNetworks(task.context, opts.Podname, opts.Driver)
	if err != nil {
		return nil, grpcstatus.Error(ListNetworks, err.Error())
	}

	return &pb.Networks{Networks: utils.Map(networks, toRPCNetwork)}, nil
}

func (v *Vibranium) ConnectNetwork(ctx context.Context, opts *pb.ConnectNetworkOptions) (*pb.Network, error) {
	task := v.newTask(ctx, "ConnectNetwork", false)
	defer task.done()
	subnets, err := v.cluster.ConnectNetwork(task.context, opts.Network, opts.Target, opts.Ipv4, opts.Ipv6)
	if err != nil {
		return nil, grpcstatus.Error(ConnectNetwork, err.Error())
	}
	return &pb.Network{Name: opts.Network, Subnets: subnets}, nil
}

func (v *Vibranium) DisconnectNetwork(ctx context.Context, opts *pb.DisconnectNetworkOptions) (*pb.Empty, error) {
	task := v.newTask(ctx, "DisconnectNetwork", false)
	defer task.done()
	if err := v.cluster.DisconnectNetwork(task.context, opts.Network, opts.Target, opts.Force); err != nil {
		return nil, grpcstatus.Error(DisconnectNetwork, err.Error())
	}
	return &pb.Empty{}, nil
}

func (v *Vibranium) AddPod(ctx context.Context, opts *pb.AddPodOptions) (*pb.Pod, error) {
	task := v.newTask(ctx, "AddPod", false)
	defer task.done()
	p, err := v.cluster.AddPod(task.context, opts.Name, opts.Desc)
	if err != nil {
		return nil, grpcstatus.Error(AddPod, err.Error())
	}

	return toRPCPod(p), nil
}

func (v *Vibranium) RemovePod(ctx context.Context, opts *pb.RemovePodOptions) (*pb.Empty, error) {
	task := v.newTask(ctx, "RemovePod", false)
	defer task.done()
	if err := v.cluster.RemovePod(task.context, opts.Name); err != nil {
		return nil, grpcstatus.Error(RemovePod, err.Error())
	}
	return &pb.Empty{}, nil
}

func (v *Vibranium) GetPod(ctx context.Context, opts *pb.GetPodOptions) (*pb.Pod, error) {
	task := v.newTask(ctx, "GetPod", false)
	defer task.done()
	p, err := v.cluster.GetPod(task.context, opts.Name)
	if err != nil {
		return nil, grpcstatus.Error(GetPod, err.Error())
	}

	return toRPCPod(p), nil
}

func (v *Vibranium) ListPods(ctx context.Context, _ *pb.Empty) (*pb.Pods, error) {
	task := v.newTask(ctx, "ListPods", false)
	defer task.done()
	ps, err := v.cluster.ListPods(task.context)
	if err != nil {
		return nil, grpcstatus.Error(ListPods, err.Error())
	}

	return &pb.Pods{Pods: utils.Map(ps, toRPCPod)}, nil
}

func (v *Vibranium) GetPodResource(opts *pb.GetPodOptions, stream pb.CoreRPC_GetPodResourceServer) error {
	task := v.newTask(stream.Context(), "GetPodResource", false)
	defer task.done()
	ch, err := v.cluster.PodResource(task.context, opts.Name)
	if err != nil {
		return grpcstatus.Error(PodResource, err.Error())
	}
	drain(task, "GetPodResource", ch, stream.Send, toRPCNodeResource)
	return nil
}

func (v *Vibranium) GetNodeResource(ctx context.Context, opts *pb.GetNodeResourceOptions) (*pb.NodeResource, error) {
	task := v.newTask(ctx, "GetNodeResource", false)
	defer task.done()
	nr, err := v.cluster.NodeResource(task.context, opts.GetOpts().Nodename, opts.Fix)
	if err != nil {
		return nil, grpcstatus.Error(GetNodeResource, err.Error())
	}

	return toRPCNodeResource(nr), nil
}

func (v *Vibranium) AddNode(ctx context.Context, opts *pb.AddNodeOptions) (*pb.Node, error) {
	task := v.newTask(ctx, "AddNode", false)
	defer task.done()
	addNodeOpts, err := toCoreAddNodeOptions(opts)
	if err != nil {
		return nil, grpcstatus.Error(AddNode, err.Error())
	}
	n, err := v.cluster.AddNode(task.context, addNodeOpts)
	if err != nil {
		return nil, grpcstatus.Error(AddNode, err.Error())
	}

	return toRPCNode(n), nil
}

func (v *Vibranium) RemoveNode(ctx context.Context, opts *pb.RemoveNodeOptions) (*pb.Empty, error) {
	task := v.newTask(ctx, "RemoveNode", false)
	defer task.done()
	if err := v.cluster.RemoveNode(task.context, opts.Nodename); err != nil {
		return nil, grpcstatus.Error(RemoveNode, err.Error())
	}
	return &pb.Empty{}, nil
}

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

	drain(task, "PodNodesStream", ch, stream.Send, toRPCNode)
	return nil
}

func (v *Vibranium) GetNode(ctx context.Context, opts *pb.GetNodeOptions) (*pb.Node, error) {
	task := v.newTask(ctx, "GetNode", false)
	defer task.done()
	n, err := v.cluster.GetNode(task.context, opts.Nodename)
	if err != nil {
		return nil, grpcstatus.Error(GetNode, err.Error())
	}

	return toRPCNode(n), nil
}

func (v *Vibranium) GetNodeEngineInfo(ctx context.Context, opts *pb.GetNodeOptions) (*pb.Engine, error) {
	task := v.newTask(ctx, "GetNodeEngine", false)
	defer task.done()
	e, err := v.cluster.GetNodeEngineInfo(task.context, opts.Nodename)
	if err != nil {
		return nil, grpcstatus.Error(GetNodeEngine, err.Error())
	}

	return toRPCEngine(e), nil
}

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

func (v *Vibranium) SetNodeStatus(ctx context.Context, opts *pb.SetNodeStatusOptions) (*pb.Empty, error) {
	task := v.newTask(ctx, "SetNodeStatus", false)
	defer task.done()
	if err := v.cluster.SetNodeStatus(task.context, opts.Nodename, opts.Ttl); err != nil {
		return nil, grpcstatus.Error(SetNodeStatus, err.Error())
	}
	return &pb.Empty{}, nil
}

func (v *Vibranium) NodeStatusStream(_ *pb.Empty, stream pb.CoreRPC_NodeStatusStreamServer) error {
	task := v.newTask(stream.Context(), "NodeStatusStream", true)
	defer task.done()

	return drainUntilStop(task, "NodeStatusStream", NodeStatusStream, v.stop, v.cluster.NodeStatusStream(task.context), stream.Send,
		func(m *types.NodeStatus) *pb.NodeStatusStreamMessage {
			r := &pb.NodeStatusStreamMessage{
				Nodename: m.Nodename,
				Podname:  m.Podname,
				Alive:    m.Alive,
			}
			if m.Error != nil {
				r.Error = m.Error.Error()
			}
			return r
		})
}

func (v *Vibranium) GetWorkloadsStatus(ctx context.Context, opts *pb.WorkloadIDs) (*pb.WorkloadsStatus, error) {
	task := v.newTask(ctx, "GetWorkloadsStatus", false)
	defer task.done()

	workloadsStatus, err := v.cluster.GetWorkloadsStatus(task.context, opts.IDs)
	if err != nil {
		return nil, grpcstatus.Error(GetWorkloadsStatus, err.Error())
	}
	return toRPCWorkloadsStatus(workloadsStatus), nil
}

func (v *Vibranium) SetWorkloadsStatus(ctx context.Context, opts *pb.SetWorkloadsStatusOptions) (*pb.WorkloadsStatus, error) {
	task := v.newTask(ctx, "SetWorkloadsStatus", false)
	defer task.done()

	statusData := make([]*types.StatusMeta, 0, len(opts.Status))
	ttls := make(map[string]int64, len(opts.Status))
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

func (v *Vibranium) WorkloadStatusStream(opts *pb.WorkloadStatusStreamOptions, stream pb.CoreRPC_WorkloadStatusStreamServer) error {
	task := v.newTask(stream.Context(), "WorkloadStatusStream", true)
	defer task.done()
	logger := log.WithFunc("vibranium.WorkloadStatusStream").WithField("app", opts.Appname)

	logger.Info(task.context, "stream started")
	defer logger.Info(task.context, "stream stopped")

	ch := v.cluster.WorkloadStatusStream(
		task.context,
		opts.Appname, opts.Entrypoint, opts.Nodename, opts.Labels,
	)
	return drainUntilStop(task, "WorkloadStatusStream", WorkloadStatusStream, v.stop, ch, stream.Send,
		func(m *types.WorkloadStatus) *pb.WorkloadStatusStreamMessage {
			r := &pb.WorkloadStatusStreamMessage{Id: m.ID, Delete: m.Delete}
			if m.Error != nil {
				r.Error = m.Error.Error()
			} else if m.Workload != nil {
				r.Workload = toRPCWorkload(task.context, m.Workload)
				r.Status = toRPCWorkloadStatus(m.Workload.StatusMeta)
			}
			return r
		})
}

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

func (v *Vibranium) GetWorkload(ctx context.Context, ID *pb.WorkloadID) (*pb.Workload, error) {
	task := v.newTask(ctx, "GetWorkload", false)
	defer task.done()
	workload, err := v.cluster.GetWorkload(task.context, ID.Id)
	if err != nil {
		return nil, grpcstatus.Error(GetWorkload, err.Error())
	}

	return toRPCWorkload(task.context, workload), nil
}

func (v *Vibranium) GetWorkloads(ctx context.Context, cids *pb.WorkloadIDs) (*pb.Workloads, error) {
	task := v.newTask(ctx, "GetWorkloads", false)
	defer task.done()
	workloads, err := v.cluster.GetWorkloads(task.context, cids.GetIDs())
	if err != nil {
		return nil, grpcstatus.Error(GetWorkloads, err.Error())
	}

	return toRPCWorkloads(task.context, workloads, nil), nil
}

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
			logUnsentMessages(task.context, "ListWorkloads", err, c)
			return grpcstatus.Error(ListWorkloads, err.Error())
		}
	}
	return nil
}

func (v *Vibranium) ListNodeWorkloads(ctx context.Context, opts *pb.GetNodeOptions) (*pb.Workloads, error) {
	task := v.newTask(ctx, "ListNodeWorkloads", false)
	defer task.done()
	workloads, err := v.cluster.ListNodeWorkloads(task.context, opts.Nodename, opts.Labels)
	if err != nil {
		return nil, grpcstatus.Error(ListNodeWorkloads, err.Error())
	}
	return toRPCWorkloads(task.context, workloads, nil), nil
}

func (v *Vibranium) Copy(opts *pb.CopyOptions, stream pb.CoreRPC_CopyServer) error {
	task := v.newTask(stream.Context(), "Copy", true)
	defer task.done()
	logger := log.WithFunc("vibranium.Copy")

	copyOpts := toCoreCopyOptions(opts)
	ch, err := v.cluster.Copy(task.context, copyOpts)
	if err != nil {
		return grpcstatus.Error(Copy, err.Error())
	}
	p := make([]byte, copyChunkSize)
	for m := range ch {
		msg := &pb.CopyMessage{
			Id:   m.ID,
			Path: m.Path,
		}
		if m.Error != nil {
			msg.Error = m.Error.Error()
			if err := stream.Send(msg); err != nil {
				logUnsentMessages(task.context, "Copy", err, m)
			}
			continue
		}

		r, w := io.Pipe()
		utils.SentryGo(func(m *types.CopyMessage) func() {
			return func() {
				var err error
				defer func() {
					w.CloseWithError(err) //nolint:errcheck
				}()

				tw := tar.NewWriter(w)
				defer func() {
					if closeErr := tw.Close(); err == nil {
						err = closeErr
					}
				}()
				header := &tar.Header{
					Name: filepath.Base(m.Filename),
					Uid:  m.UID,
					Gid:  m.GID,
					Mode: m.Mode,
					Size: int64(len(m.Content)),
				}
				if err = tw.WriteHeader(header); err != nil {
					logger.Error(task.context, err, "write tarball header")
					return
				}
				if _, err = tw.Write(m.Content); err != nil {
					logger.Error(task.context, err, "write tarball content")
					return
				}
			}
		}(m))

		for {
			n, err := r.Read(p)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					logger.Error(task.context, err, "read copy stream")
					msg.Error = err.Error()
					if err = stream.Send(msg); err != nil {
						logUnsentMessages(task.context, "Copy", err, m)
					}
				}
				break
			}
			if n > 0 {
				msg.Data = p[:n]
				if err = stream.Send(msg); err != nil {
					logUnsentMessages(task.context, "Copy", err, m)
				}
			}
		}
	}
	return nil
}

func (v *Vibranium) Send(opts *pb.SendOptions, stream pb.CoreRPC_SendServer) error {
	task := v.newTask(stream.Context(), "Send", true)
	defer task.done()

	sendOpts := toCoreSendOptions(opts)
	if err := sendOpts.Validate(); err != nil {
		return grpcstatus.Error(Send, err.Error())
	}
	for _, file := range sendOpts.Files {
		dc := make(chan *types.SendLargeFileOptions)
		ch := v.cluster.SendLargeFile(task.context, dc)
		utils.SentryGo(func() {
			defer close(dc)
			data := toSendLargeFileChunks(file, sendOpts.IDs)
			for _, chunk := range data {
				select {
				case dc <- chunk:
				case <-task.context.Done():
					return
				}
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
				logUnsentMessages(task.context, "Send", err, m)
			}
		}
	}
	return nil
}

func (v *Vibranium) SendLargeFile(stream pb.CoreRPC_SendLargeFileServer) error {
	task := v.newTask(stream.Context(), "SendLargeFile", true)
	defer task.done()
	logger := log.WithFunc("vibranium.SendLargeFile")

	inputChan := make(chan *types.SendLargeFileOptions)
	resp := v.cluster.SendLargeFile(task.context, inputChan)
	var recvErr error
	utils.SentryGo(func() {
		defer close(inputChan)
		for {
			req, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				logger.Error(task.context, err, "recv from stream")
				recvErr = err
				return
			}
			data, err := toSendLargeFileOptions(req)
			if err != nil {
				logger.Error(task.context, err, "transform file options")
				recvErr = err
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
			logUnsentMessages(task.context, "SendLargeFile", err, m)
		}
	}
	if recvErr != nil {
		return grpcstatus.Error(SendLargeFile, recvErr.Error())
	}
	return nil
}

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

	drain(task, "BuildImage", ch, stream.Send, toRPCBuildImageMessage)
	return nil
}

func (v *Vibranium) CacheImage(opts *pb.CacheImageOptions, stream pb.CoreRPC_CacheImageServer) error {
	task := v.newTask(stream.Context(), "CacheImage", true)
	defer task.done()

	ch, err := v.cluster.CacheImage(task.context, toCoreCacheImageOptions(opts))
	if err != nil {
		return grpcstatus.Error(CacheImage, err.Error())
	}

	drain(task, "CacheImage", ch, stream.Send, toRPCCacheImageMessage)
	return nil
}

func (v *Vibranium) RemoveImage(opts *pb.RemoveImageOptions, stream pb.CoreRPC_RemoveImageServer) error {
	task := v.newTask(stream.Context(), "RemoveImage", true)
	defer task.done()

	ch, err := v.cluster.RemoveImage(task.context, toCoreRemoveImageOptions(opts))
	if err != nil {
		return grpcstatus.Error(RemoveImage, err.Error())
	}

	drain(task, "RemoveImage", ch, stream.Send, toRPCRemoveImageMessage)
	return nil
}

func (v *Vibranium) ListImage(opts *pb.ListImageOptions, stream pb.CoreRPC_ListImageServer) error {
	task := v.newTask(stream.Context(), "ListImage", true)
	defer task.done()

	ch, err := v.cluster.ListImage(task.context, toCoreListImageOptions(opts))
	if err != nil {
		return grpcstatus.Error(ListImage, err.Error())
	}

	drain(task, "ListImage", ch, stream.Send, toRPCListImageMessage)
	return nil
}

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
	drain(task, "CreateWorkload", ch, stream.Send, func(m *types.CreateWorkloadMessage) *pb.CreateWorkloadMessage {
		log.WithFunc("vibranium.CreateWorkload").Debugf(task.context, "create workload message: %+v", m)
		return toRPCCreateWorkloadMessage(m)
	})
	return nil
}

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

	drain(task, "ReplaceWorkload", ch, stream.Send, toRPCReplaceWorkloadMessage)
	return nil
}

func (v *Vibranium) RemoveWorkload(opts *pb.RemoveWorkloadOptions, stream pb.CoreRPC_RemoveWorkloadServer) error {
	task := v.newTask(stream.Context(), "RemoveWorkload", true)
	defer task.done()

	IDs := opts.GetIDs()
	force := opts.GetForce()

	if len(IDs) == 0 {
		return grpcstatus.Error(RemoveWorkload, types.ErrNoWorkloadIDs.Error())
	}
	ch, err := v.cluster.RemoveWorkload(task.context, IDs, force)
	if err != nil {
		return grpcstatus.Error(RemoveWorkload, err.Error())
	}

	drain(task, "RemoveWorkload", ch, stream.Send, toRPCRemoveWorkloadMessage)
	return nil
}

func (v *Vibranium) DissociateWorkload(opts *pb.DissociateWorkloadOptions, stream pb.CoreRPC_DissociateWorkloadServer) error {
	task := v.newTask(stream.Context(), "DissociateWorkload", true)
	defer task.done()

	IDs := opts.GetIDs()
	if len(IDs) == 0 {
		return grpcstatus.Error(DissociateWorkload, types.ErrNoWorkloadIDs.Error())
	}

	ch, err := v.cluster.DissociateWorkload(task.context, IDs)
	if err != nil {
		return grpcstatus.Error(DissociateWorkload, err.Error())
	}

	drain(task, "DissociateWorkload", ch, stream.Send, toRPCDissociateWorkloadMessage)
	return nil
}

func (v *Vibranium) ControlWorkload(opts *pb.ControlWorkloadOptions, stream pb.CoreRPC_ControlWorkloadServer) error {
	task := v.newTask(stream.Context(), "ControlWorkload", true)
	defer task.done()

	IDs := opts.GetIDs()
	t := opts.GetType()
	force := opts.GetForce()

	if len(IDs) == 0 {
		return grpcstatus.Error(ControlWorkload, types.ErrNoWorkloadIDs.Error())
	}

	ch, err := v.cluster.ControlWorkload(task.context, IDs, t, force)
	if err != nil {
		return grpcstatus.Error(ControlWorkload, err.Error())
	}

	drain(task, "ControlWorkload", ch, stream.Send, toRPCControlWorkloadMessage)
	return nil
}

func (v *Vibranium) ExecuteWorkload(stream pb.CoreRPC_ExecuteWorkloadServer) error {
	task := v.newTask(stream.Context(), "ExecuteWorkload", true)
	defer task.done()

	opts, err := stream.Recv()
	if err != nil {
		return grpcstatus.Error(ExecuteWorkload, err.Error())
	}
	executeWorkloadOpts := toCoreExecuteWorkloadOptions(opts)

	inCh := recvStdin(task.context, "ExecuteWorkload", opts.OpenStdin, stream.Recv, (*pb.ExecuteWorkloadOptions).GetReplCmd)
	drain(task, "ExecuteWorkload", v.cluster.ExecuteWorkload(task.context, executeWorkloadOpts, inCh), stream.Send, toRPCAttachWorkloadMessage)
	return nil
}

func (v *Vibranium) ReallocResource(ctx context.Context, opts *pb.ReallocOptions) (*pb.ReallocResourceMessage, error) {
	task := v.newTask(ctx, "ReallocResource", true)
	defer task.done()

	if opts.Id == "" {
		return reallocResult(types.ErrNoWorkloadIDs)
	}

	resources, err := toCoreResources(opts.Resources)
	if err != nil {
		return reallocResult(err)
	}

	return reallocResult(v.cluster.ReallocResource(
		task.context,
		&types.ReallocOptions{
			ID:        opts.Id,
			Resources: resources,
		},
	))
}

func (v *Vibranium) LogStream(opts *pb.LogStreamOptions, stream pb.CoreRPC_LogStreamServer) error {
	task := v.newTask(stream.Context(), "LogStream", true)
	defer task.done()

	ID := opts.GetId()
	logger := log.WithFunc("vibranium.LogStream").WithField("ID", ID)

	logger.Info(task.context, "log stream started")
	defer logger.Info(task.context, "log stream stopped")
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
				logUnsentMessages(task.context, "LogStream", err, m)
			}
		case <-v.stop:
			return nil
		}
	}
}

func (v *Vibranium) RunAndWait(stream pb.CoreRPC_RunAndWaitServer) error {
	task := v.newTask(stream.Context(), "RunAndWait", true)
	RunAndWaitOptions, deployOpts, err := runAndWaitOptions(stream)
	if err != nil {
		task.done()
		return grpcstatus.Error(RunAndWait, err.Error())
	}
	logger := log.WithFunc("vibranium.RunAndWait")
	opts := RunAndWaitOptions.DeployOptions

	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	if RunAndWaitOptions.Async {
		timeout := v.config.GlobalTimeout
		if RunAndWaitOptions.AsyncTimeout != 0 {
			timeout = time.Second * time.Duration(RunAndWaitOptions.AsyncTimeout)
		}
		ctx, cancel = context.WithTimeout(context.WithoutCancel(task.context), timeout) // task.done cancels task.context
	} else {
		ctx, cancel = context.WithCancel(task.context)
	}

	inCh := recvStdin(ctx, "RunAndWait", opts.OpenStdin, stream.Recv, (*pb.RunAndWaitOptions).GetCmd)
	IDs, ch, err := v.cluster.RunAndWait(ctx, deployOpts, inCh)
	if err != nil {
		task.done()
		cancel()
		return grpcstatus.Error(RunAndWait, err.Error())
	}

	for _, ID := range IDs {
		if err = stream.Send(&pb.AttachWorkloadMessage{
			WorkloadId:    ID,
			Data:          []byte(""),
			StdStreamType: pb.StdStreamType_TYPEWORKLOADID,
		}); err != nil {
			logUnsentMessages(ctx, "RunAndWait", err, ID)
		}
	}

	runAndWait := func(f func(<-chan *types.AttachWorkloadMessage)) {
		defer task.done()
		defer cancel()
		f(ch)
	}

	if !RunAndWaitOptions.Async {
		runAndWait(func(ch <-chan *types.AttachWorkloadMessage) {
			for m := range ch {
				if err = stream.Send(toRPCAttachWorkloadMessage(m)); err != nil {
					logUnsentMessages(ctx, "RunAndWait", err, m)
				}
			}
		})
		return nil
	}

	utils.SentryGo(func() {
		runAndWait(func(ch <-chan *types.AttachWorkloadMessage) {
			r, w := io.Pipe()
			utils.SentryGo(func() {
				defer func() {
					_ = w.Close()
				}()
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
						if !errors.Is(err, io.EOF) {
							logger.Error(ctx, err, "read line")
						}
						return
					}
					line = append(line, part...)
					if !isPrefix {
						break
					}
				}
				logger.Infof(ctx, "%s", line)
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

func logUnsentMessages(ctx context.Context, msgType string, err error, msg any) {
	log.WithFunc("vibranium.logUnsentMessages").Warnf(ctx, "unsent %s streamed message %+v: %+v", msgType, msg, err)
}

func recvStdin[T any](ctx context.Context, name string, open bool, recv func() (T, error), payload func(T) []byte) <-chan []byte {
	inCh := make(chan []byte)
	if !open {
		close(inCh)
		return inCh
	}
	utils.SentryGo(func() {
		defer close(inCh)
		for {
			msg, err := recv()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					log.WithFunc("vibranium."+name).Error(ctx, err, "recv command")
				}
				return
			}
			select {
			case inCh <- payload(msg):
			case <-ctx.Done():
				return
			}
		}
	})
	return inCh
}

func drain[T, R any](t *task, name string, ch <-chan T, send sender[R], to converter[T, R]) {
	for m := range ch {
		if err := send(to(m)); err != nil {
			logUnsentMessages(t.context, name, err, m)
		}
	}
}

func drainUntilStop[T, R any](t *task, name string, code codes.Code, stop <-chan struct{}, ch <-chan T, send sender[R], to converter[T, R]) error {
	for {
		select {
		case m, ok := <-ch:
			if !ok {
				if t.context.Err() != nil {
					return nil
				}
				return grpcstatus.Error(code, types.ErrMessageChanClosed.Error())
			}
			if err := send(to(m)); err != nil {
				logUnsentMessages(t.context, name, err, m)
			}
		case <-stop:
			return nil
		}
	}
}

func runAndWaitOptions(stream pb.CoreRPC_RunAndWaitServer) (*pb.RunAndWaitOptions, *types.DeployOptions, error) {
	RunAndWaitOptions, err := stream.Recv()
	if err != nil {
		return nil, nil, err
	}
	if RunAndWaitOptions.DeployOptions == nil {
		return nil, nil, types.ErrNoDeployOpts
	}
	opts := RunAndWaitOptions.DeployOptions
	if RunAndWaitOptions.Async {
		opts.OpenStdin = false
	}
	deployOpts, err := toCoreDeployOptions(opts)
	return RunAndWaitOptions, deployOpts, err
}

func reallocResult(err error) (*pb.ReallocResourceMessage, error) {
	if err == nil {
		return &pb.ReallocResourceMessage{}, nil
	}
	err = grpcstatus.Error(ReallocResource, err.Error())
	return &pb.ReallocResourceMessage{Error: err.Error()}, err
}
