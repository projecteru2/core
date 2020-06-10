package rpc

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"sync"
	"time"

	"github.com/projecteru2/core/cluster"
	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
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

// AddPod saves a pod, and returns it to client
func (v *Vibranium) AddPod(ctx context.Context, opts *pb.AddPodOptions) (*pb.Pod, error) {
	p, err := v.cluster.AddPod(ctx, opts.Name, opts.Desc)
	if err != nil {
		return nil, err
	}

	return toRPCPod(p), nil
}

// AddNode saves a node and returns it to client
// Method must be called synchronously, or nothing will be returned
func (v *Vibranium) AddNode(ctx context.Context, opts *pb.AddNodeOptions) (*pb.Node, error) {
	addNodeOpts := toCoreAddNodeOptions(opts)
	n, err := v.cluster.AddNode(ctx, addNodeOpts)
	if err != nil {
		return nil, err
	}

	return toRPCNode(ctx, n), nil
}

// RemovePod removes a pod only if it's empty
func (v *Vibranium) RemovePod(ctx context.Context, opts *pb.RemovePodOptions) (*pb.Empty, error) {
	return &pb.Empty{}, v.cluster.RemovePod(ctx, opts.Name)
}

// RemoveNode removes the node from etcd
func (v *Vibranium) RemoveNode(ctx context.Context, opts *pb.RemoveNodeOptions) (*pb.Empty, error) {
	return &pb.Empty{}, v.cluster.RemoveNode(ctx, opts.Nodename)
}

// ListPods returns a list of pods
func (v *Vibranium) ListPods(ctx context.Context, _ *pb.Empty) (*pb.Pods, error) {
	ps, err := v.cluster.ListPods(ctx)
	if err != nil {
		return nil, err
	}

	pods := []*pb.Pod{}
	for _, p := range ps {
		pods = append(pods, toRPCPod(p))
	}

	return &pb.Pods{Pods: pods}, nil

}

// ListPodNodes returns a list of node for pod
func (v *Vibranium) ListPodNodes(ctx context.Context, opts *pb.ListNodesOptions) (*pb.Nodes, error) {
	ns, err := v.cluster.ListPodNodes(ctx, opts.Podname, opts.Labels, opts.All)
	if err != nil {
		return nil, err
	}

	nodes := []*pb.Node{}
	for _, n := range ns {
		nodes = append(nodes, toRPCNode(ctx, n))
	}
	return &pb.Nodes{Nodes: nodes}, nil
}

// ListContainers by appname with optional entrypoint and nodename
func (v *Vibranium) ListContainers(opts *pb.ListContainersOptions, stream pb.CoreRPC_ListContainersServer) error {
	lsopts := &types.ListContainersOptions{
		Appname:    opts.Appname,
		Entrypoint: opts.Entrypoint,
		Nodename:   opts.Nodename,
		Limit:      opts.Limit,
		Labels:     opts.Labels,
	}
	ctx := stream.Context()

	containers, err := v.cluster.ListContainers(ctx, lsopts)
	if err != nil {
		return err
	}

	for _, c := range toRPCContainers(ctx, containers, opts.Labels) {
		if err = stream.Send(c); err != nil {
			v.logUnsentMessages("ListContainers", c)
			return err
		}
	}
	return nil
}

// ListNodeContainers list node containers
func (v *Vibranium) ListNodeContainers(ctx context.Context, opts *pb.GetNodeOptions) (*pb.Containers, error) {
	containers, err := v.cluster.ListNodeContainers(ctx, opts.Nodename, opts.Labels)
	if err != nil {
		return nil, err
	}
	return &pb.Containers{Containers: toRPCContainers(ctx, containers, nil)}, nil
}

// ListNetworks list networks for pod
func (v *Vibranium) ListNetworks(ctx context.Context, opts *pb.ListNetworkOptions) (*pb.Networks, error) {
	networks, err := v.cluster.ListNetworks(ctx, opts.Podname, opts.Driver)
	if err != nil {
		return nil, err
	}

	ns := []*pb.Network{}
	for _, n := range networks {
		ns = append(ns, toRPCNetwork(n))
	}
	return &pb.Networks{Networks: ns}, nil
}

// GetPod show a pod
func (v *Vibranium) GetPod(ctx context.Context, opts *pb.GetPodOptions) (*pb.Pod, error) {
	p, err := v.cluster.GetPod(ctx, opts.Name)
	if err != nil {
		return nil, err
	}

	return toRPCPod(p), nil
}

// GetPodResource get pod nodes resource usage
func (v *Vibranium) GetPodResource(ctx context.Context, opts *pb.GetPodOptions) (*pb.PodResource, error) {
	r, err := v.cluster.PodResource(ctx, opts.Name)
	if err != nil {
		return nil, err
	}
	return toRPCPodResource(r), nil
}

// GetNode get a node
func (v *Vibranium) GetNode(ctx context.Context, opts *pb.GetNodeOptions) (*pb.Node, error) {
	n, err := v.cluster.GetNode(ctx, opts.Nodename)
	if err != nil {
		return nil, err
	}

	return toRPCNode(ctx, n), nil
}

// GetNodeResource check node resource
func (v *Vibranium) GetNodeResource(ctx context.Context, opts *pb.GetNodeOptions) (*pb.NodeResource, error) {
	nr, err := v.cluster.NodeResource(ctx, opts.Nodename)
	if err != nil {
		return nil, err
	}

	return toRPCNodeResource(nr), nil
}

// GetContainer get a container
// More information will be shown
func (v *Vibranium) GetContainer(ctx context.Context, id *pb.ContainerID) (*pb.Container, error) {
	container, err := v.cluster.GetContainer(ctx, id.Id)
	if err != nil {
		return nil, err
	}

	return toRPCContainer(ctx, container)
}

// GetContainers get lots containers
// like GetContainer, information should be returned
func (v *Vibranium) GetContainers(ctx context.Context, cids *pb.ContainerIDs) (*pb.Containers, error) {
	containers, err := v.cluster.GetContainers(ctx, cids.GetIds())
	if err != nil {
		return nil, err
	}

	return &pb.Containers{Containers: toRPCContainers(ctx, containers, nil)}, nil
}

// SetNode set node meta
func (v *Vibranium) SetNode(ctx context.Context, opts *pb.SetNodeOptions) (*pb.Node, error) {
	setNodeOpts, err := toCoreSetNodeOptions(opts)
	if err != nil {
		return nil, err
	}
	n, err := v.cluster.SetNode(ctx, setNodeOpts)
	if err != nil {
		return nil, err
	}
	return toRPCNode(ctx, n), nil
}

// Copy copy files from multiple containers
func (v *Vibranium) Copy(opts *pb.CopyOptions, stream pb.CoreRPC_CopyServer) error {
	v.taskAdd("Copy", true)
	defer v.taskDone("Copy", true)

	copyOpts := toCoreCopyOptions(opts)
	ch, err := v.cluster.Copy(stream.Context(), copyOpts)
	if err != nil {
		return err
	}

	//4K buffer
	bsize := 4 * 1024

	for m := range ch {
		var copyError string
		if m.Error != nil {
			copyError = fmt.Sprintf("%v", m.Error)
			if err := stream.Send(&pb.CopyMessage{
				Id:     m.ID,
				Status: m.Status,
				Name:   m.Name,
				Path:   m.Path,
				Error:  copyError,
				Data:   []byte{},
			}); err != nil {
				v.logUnsentMessages("Copy", m)
			}
			continue
		}

		r, w := io.Pipe()
		go func() {
			defer w.Close()
			defer m.Data.Close()

			tw := tar.NewWriter(w)
			bs, err := ioutil.ReadAll(m.Data)
			if err != nil {
				log.Errorf("[Copy] Error during extracting copy data: %v", err)
			}
			header := &tar.Header{Name: m.Name, Mode: 0644, Size: int64(len(bs))}
			if err := tw.WriteHeader(header); err != nil {
				log.Errorf("[Copy] Error during writing tarball header: %v", err)
			}
			if _, err := tw.Write(bs); err != nil {
				log.Errorf("[Copy] Error during writing tarball content: %v", err)
			}
		}()

		for {
			p := make([]byte, bsize)
			n, err := r.Read(p)
			if err != nil {
				if err != io.EOF {
					log.Errorf("[Copy] Error during buffer resp: %v", err)
				}
				break
			}
			if err = stream.Send(&pb.CopyMessage{
				Id:     m.ID,
				Status: m.Status,
				Name:   m.Name,
				Path:   m.Path,
				Error:  copyError,
				Data:   p[:n],
			}); err != nil {
				v.logUnsentMessages("Copy", m)
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
		return err
	}

	sendOpts.Data = opts.Data
	ch, err := v.cluster.Send(stream.Context(), sendOpts)
	if err != nil {
		return err
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
			v.logUnsentMessages("Send", m)
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
		return err
	}
	// TODO VM BRANCH
	ch, err := v.cluster.BuildImage(stream.Context(), buildOpts)
	if err != nil {
		log.Errorf("[BuildImage] build image error %+v", err)
		return err
	}

	for m := range ch {
		if err = stream.Send(toRPCBuildImageMessage(m)); err != nil {
			v.logUnsentMessages("BuildImage", m)
		}
	}
	return err
}

// CacheImage cache image
func (v *Vibranium) CacheImage(opts *pb.CacheImageOptions, stream pb.CoreRPC_CacheImageServer) error {
	v.taskAdd("CacheImage", true)
	defer v.taskDone("CacheImage", true)

	ch, err := v.cluster.CacheImage(stream.Context(), opts.Podname, opts.Nodename, opts.Images, int(opts.Step))
	if err != nil {
		return err
	}

	for m := range ch {
		if err = stream.Send(toRPCCacheImageMessage(m)); err != nil {
			v.logUnsentMessages("CacheImage", m)
		}
	}
	return err
}

// RemoveImage remove image
func (v *Vibranium) RemoveImage(opts *pb.RemoveImageOptions, stream pb.CoreRPC_RemoveImageServer) error {
	v.taskAdd("RemoveImage", true)
	defer v.taskDone("RemoveImage", true)

	ch, err := v.cluster.RemoveImage(stream.Context(), opts.Podname, opts.Nodename, opts.Images, int(opts.Step), opts.Prune)
	if err != nil {
		return err
	}

	for m := range ch {
		if err = stream.Send(toRPCRemoveImageMessage(m)); err != nil {
			v.logUnsentMessages("RemoveImage", m)
		}
	}
	return err
}

// RunAndWait is lambda
func (v *Vibranium) RunAndWait(stream pb.CoreRPC_RunAndWaitServer) error {
	RunAndWaitOptions, err := stream.Recv()
	if err != nil {
		return err
	}

	if RunAndWaitOptions.DeployOptions == nil {
		return types.ErrNoDeployOpts
	}

	opts := RunAndWaitOptions.DeployOptions
	deployOpts, err := toCoreDeployOptions(opts)
	if err != nil {
		return err
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
		if opts.OpenStdin {
			for {
				RunAndWaitOptions, err := stream.Recv()
				if RunAndWaitOptions == nil || err != nil {
					log.Errorf("[RunAndWait] Recv command error: %v", err)
					break
				}
				inCh <- RunAndWaitOptions.Cmd
			}
		}
	}()

	runAndWait := func(f func(<-chan *types.AttachContainerMessage)) error {
		defer v.taskDone("RunAndWait", true)
		defer cancel()
		ch, err := v.cluster.RunAndWait(ctx, deployOpts, inCh)
		if err != nil {
			log.Errorf("[RunAndWait] Start run and wait failed %s", err)
			return err
		}
		f(ch)
		return nil
	}

	if !RunAndWaitOptions.Async {
		return runAndWait(func(ch <-chan *types.AttachContainerMessage) {
			for m := range ch {
				if err = stream.Send(toRPCAttachContainerMessage(m)); err != nil {
					v.logUnsentMessages("RunAndWait", m)
				}
			}
		})
	}
	go func() {
		_ = runAndWait(func(ch <-chan *types.AttachContainerMessage) {
			for m := range ch {
				log.Infof("[Async RunAndWait] %v", string(m.Data))
			}
		})
	}()
	return nil
}

// CreateContainer create containers
func (v *Vibranium) CreateContainer(opts *pb.DeployOptions, stream pb.CoreRPC_CreateContainerServer) error {
	v.taskAdd("CreateContainer", true)
	defer v.taskDone("CreateContainer", true)

	deployOpts, err := toCoreDeployOptions(opts)
	if err != nil {
		return err
	}

	ch, err := v.cluster.CreateContainer(stream.Context(), deployOpts)
	if err != nil {
		return err
	}

	for m := range ch {
		if err = stream.Send(toRPCCreateContainerMessage(m)); err != nil {
			v.logUnsentMessages("CreateContainer", m)
		}
	}

	return nil
}

// ReplaceContainer replace containers
func (v *Vibranium) ReplaceContainer(opts *pb.ReplaceOptions, stream pb.CoreRPC_ReplaceContainerServer) error {
	v.taskAdd("ReplaceContainer", true)
	defer v.taskDone("ReplaceContainer", true)

	replaceOpts, err := toCoreReplaceOptions(opts)
	if err != nil {
		return err
	}

	ch, err := v.cluster.ReplaceContainer(stream.Context(), replaceOpts)
	if err != nil {
		return err
	}

	for m := range ch {
		if err = stream.Send(toRPCReplaceContainerMessage(m)); err != nil {
			v.logUnsentMessages("ReplaceContainer", m)
		}
	}

	return nil
}

// RemoveContainer remove containers
func (v *Vibranium) RemoveContainer(opts *pb.RemoveContainerOptions, stream pb.CoreRPC_RemoveContainerServer) error {
	v.taskAdd("RemoveContainer", true)
	defer v.taskDone("RemoveContainer", true)

	ids := opts.GetIds()
	force := opts.GetForce()
	step := int(opts.GetStep())

	if len(ids) == 0 {
		return types.ErrNoContainerIDs
	}
	ch, err := v.cluster.RemoveContainer(stream.Context(), ids, force, step)
	if err != nil {
		return err
	}

	for m := range ch {
		if err = stream.Send(toRPCRemoveContainerMessage(m)); err != nil {
			v.logUnsentMessages("RemoveContainer", m)
		}
	}

	return err
}

// DissociateContainer dissociate container
func (v *Vibranium) DissociateContainer(opts *pb.DissociateContainerOptions, stream pb.CoreRPC_DissociateContainerServer) error {
	v.taskAdd("DissociateContainer", true)
	defer v.taskDone("DissociateContainer", true)

	ids := opts.GetIds()
	if len(ids) == 0 {
		return types.ErrNoContainerIDs
	}

	ch, err := v.cluster.DissociateContainer(stream.Context(), ids)
	if err != nil {
		return err
	}

	for m := range ch {
		if err = stream.Send(toRPCDissociateContainerMessage(m)); err != nil {
			v.logUnsentMessages("DissociateContainer", m)
		}
	}

	return err
}

// ControlContainer control containers
func (v *Vibranium) ControlContainer(opts *pb.ControlContainerOptions, stream pb.CoreRPC_ControlContainerServer) error {
	v.taskAdd("ControlContainer", true)
	defer v.taskDone("ControlContainer", true)

	ids := opts.GetIds()
	t := opts.GetType()
	force := opts.GetForce()

	if len(ids) == 0 {
		return types.ErrNoContainerIDs
	}

	ch, err := v.cluster.ControlContainer(stream.Context(), ids, t, force)
	if err != nil {
		return err
	}

	for m := range ch {
		if err = stream.Send(toRPCControlContainerMessage(m)); err != nil {
			v.logUnsentMessages("ControlContainer", m)
		}
	}

	return err
}

// ReallocResource realloc res for containers
func (v *Vibranium) ReallocResource(opts *pb.ReallocOptions, stream pb.CoreRPC_ReallocResourceServer) error {
	v.taskAdd("ReallocResource", true)
	defer v.taskDone("ReallocResource", true)
	ids := opts.GetIds()
	if len(ids) == 0 {
		return types.ErrNoContainerIDs
	}
	vbs, err := types.MakeVolumeBindings(opts.Volumes)
	if err != nil {
		return err
	}
	bindCPUOpt := types.BindCPUOptions(opts.BindCpuOpt)
	//这里不能让 client 打断 remove
	ch, err := v.cluster.ReallocResource(stream.Context(), ids, opts.Cpu, opts.Memory, vbs, bindCPUOpt)
	if err != nil {
		return err
	}

	for m := range ch {
		if err = stream.Send(toRPCReallocResourceMessage(m)); err != nil {
			v.logUnsentMessages("ReallocResource", m)
		}
	}
	return err
}

// LogStream get container logs
func (v *Vibranium) LogStream(opts *pb.LogStreamOptions, stream pb.CoreRPC_LogStreamServer) error {
	ID := opts.GetId()
	log.Infof("[LogStream] Get %s log start", ID)
	defer log.Infof("[LogStream] Get %s log done", ID)
	ch, err := v.cluster.LogStream(stream.Context(), &types.LogStreamOptions{
		ID: ID, Tail: opts.Tail, Since: opts.Since, Until: opts.Until,
	})
	if err != nil {
		return err
	}

	for {
		select {
		case m, ok := <-ch:
			if !ok {
				return nil
			}
			if err = stream.Send(toRPCLogStreamMessage(m)); err != nil {
				v.logUnsentMessages("LogStream", m)
			}
		case <-v.rpcch:
			return nil
		}
	}
}

// GetContainersStatus get containers status
func (v *Vibranium) GetContainersStatus(ctx context.Context, opts *pb.ContainerIDs) (*pb.ContainersStatus, error) {
	v.taskAdd("GetContainersStatus", false)
	defer v.taskDone("GetContainersStatus", false)

	containersStatus, err := v.cluster.GetContainersStatus(ctx, opts.Ids)
	if err != nil {
		return nil, err
	}
	return &pb.ContainersStatus{Status: toRPCContainersStatus(containersStatus)}, nil
}

// SetContainersStatus set containers status
func (v *Vibranium) SetContainersStatus(ctx context.Context, opts *pb.SetContainersStatusOptions) (*pb.ContainersStatus, error) {
	v.taskAdd("SetContainersStatus", false)
	defer v.taskDone("SetContainersStatus", false)

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

	status, err := v.cluster.SetContainersStatus(ctx, statusData, ttls)
	if err != nil {
		return nil, err
	}
	return &pb.ContainersStatus{Status: toRPCContainersStatus(status)}, nil
}

// ContainerStatusStream watch and show deployed status
func (v *Vibranium) ContainerStatusStream(opts *pb.ContainerStatusStreamOptions, stream pb.CoreRPC_ContainerStatusStreamServer) error {
	log.Infof("[rpc] ContainerStatusStream start %s", opts.Appname)
	defer log.Infof("[rpc] ContainerStatusStream stop %s", opts.Appname)

	ch := v.cluster.ContainerStatusStream(
		stream.Context(),
		opts.Appname, opts.Entrypoint, opts.Nodename, opts.Labels,
	)
	for {
		select {
		case m, ok := <-ch:
			if !ok {
				return nil
			}
			r := &pb.ContainerStatusStreamMessage{Id: m.ID, Delete: m.Delete}
			if m.Error != nil {
				r.Error = m.Error.Error()
			} else if m.Container != nil {
				if container, err := toRPCContainer(stream.Context(), m.Container); err != nil {
					r.Error = err.Error()
				} else {
					r.Container = container
					r.Status = toRPCContainerStatus(m.Container.StatusMeta)
				}
			}
			if err := stream.Send(r); err != nil {
				v.logUnsentMessages("ContainerStatusStream", m)
			}
		case <-v.rpcch:
			return nil
		}
	}
}

// ExecuteContainer runs a command in a running container
func (v *Vibranium) ExecuteContainer(stream pb.CoreRPC_ExecuteContainerServer) (err error) {
	v.taskAdd("ExecuteContainer", true)
	defer v.taskDone("ExecuteContainer", true)

	opts, err := stream.Recv()
	if err != nil {
		return
	}
	var executeContainerOpts *types.ExecuteContainerOptions
	if executeContainerOpts, err = toCoreExecuteContainerOptions(opts); err != nil {
		return
	}

	inCh := make(chan []byte)
	go func() {
		defer close(inCh)
		if opts.OpenStdin {
			for {
				execContainerOpt, err := stream.Recv()
				if execContainerOpt == nil || err != nil {
					log.Errorf("[ExecuteContainer] Recv command error: %v", err)
					return
				}
				inCh <- execContainerOpt.ReplCmd
			}
		}
	}()

	for m := range v.cluster.ExecuteContainer(stream.Context(), executeContainerOpts, inCh) {
		if err = stream.Send(toRPCAttachContainerMessage(m)); err != nil {
			v.logUnsentMessages("ExecuteContainer", m)
		}
	}

	return
}

func (v *Vibranium) logUnsentMessages(msgType string, msg interface{}) {
	log.Infof("[logUnsentMessages] Unsent %s streamed message: %v", msgType, msg)
}

// New will new a new cluster instance
func New(cluster cluster.Cluster, config types.Config, rpcch chan struct{}) *Vibranium {
	return &Vibranium{cluster: cluster, config: config, counter: sync.WaitGroup{}, rpcch: rpcch}
}
