package rpc

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/rpc/gen"
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
	TaskNum int
}

// AddPod saves a pod, and returns it to client
func (v *Vibranium) AddPod(ctx context.Context, opts *pb.AddPodOptions) (*pb.Pod, error) {
	p, err := v.cluster.AddPod(ctx, opts.Name, opts.Favor, opts.Desc)
	if err != nil {
		return nil, err
	}

	return toRPCPod(p), nil
}

// AddNode saves a node and returns it to client
// Method must be called synchronously, or nothing will be returned
func (v *Vibranium) AddNode(ctx context.Context, opts *pb.AddNodeOptions) (*pb.Node, error) {
	n, err := v.cluster.AddNode(
		ctx,
		opts.Nodename,
		opts.Endpoint,
		opts.Podname,
		opts.Ca,
		opts.Cert,
		opts.Key,
		int(opts.Cpu),
		int(opts.Share),
		opts.Memory,
		opts.Labels,
	)
	if err != nil {
		return nil, err
	}

	return toRPCNode(ctx, n), nil
}

// RemovePod removes a pod only if it's empty
func (v *Vibranium) RemovePod(ctx context.Context, opts *pb.RemovePodOptions) (*pb.Empty, error) {
	err := v.cluster.RemovePod(ctx, opts.Name)
	if err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

// RemoveNode removes the node from etcd
func (v *Vibranium) RemoveNode(ctx context.Context, opts *pb.RemoveNodeOptions) (*pb.Pod, error) {
	p, err := v.cluster.RemoveNode(ctx, opts.Nodename, opts.Podname)
	if err != nil {
		return nil, err
	}

	return toRPCPod(p), nil
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
	ns, err := v.cluster.ListPodNodes(ctx, opts.Podname, opts.All)
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
func (v *Vibranium) ListContainers(ctx context.Context, opts *pb.DeployStatusOptions) (*pb.Containers, error) {
	containers, err := v.cluster.ListContainers(ctx, opts.Appname, opts.Entrypoint, opts.Nodename)
	if err != nil {
		return nil, err
	}

	return &pb.Containers{Containers: toRPCContainers(ctx, containers)}, nil
}

// ListNodeContainers list node containers
func (v *Vibranium) ListNodeContainers(ctx context.Context, opts *pb.GetNodeOptions) (*pb.Containers, error) {
	containers, err := v.cluster.ListNodeContainers(ctx, opts.Nodename)
	if err != nil {
		return nil, err
	}
	return &pb.Containers{Containers: toRPCContainers(ctx, containers)}, nil
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

// GetNode get a node
func (v *Vibranium) GetNode(ctx context.Context, opts *pb.GetNodeOptions) (*pb.Node, error) {
	n, err := v.cluster.GetNode(ctx, opts.Podname, opts.Nodename)
	if err != nil {
		return nil, err
	}

	return toRPCNode(ctx, n), nil
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

	return &pb.Containers{Containers: toRPCContainers(ctx, containers)}, nil
}

// SetNodeAvailable set node availability
func (v *Vibranium) SetNodeAvailable(ctx context.Context, opts *pb.NodeAvailable) (*pb.Node, error) {
	n, err := v.cluster.SetNodeAvailable(ctx, opts.Podname, opts.Nodename, opts.Available)
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
			buffer := bufio.NewWriterSize(w, bsize)
			defer buffer.Flush()
			gw := gzip.NewWriter(buffer)
			defer gw.Close()
			_, err = io.Copy(gw, m.Data)
			if err != nil {
				log.Errorf("[Copy] Error during copy resp: %v", err)
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

// BuildImage streamed returned functions
func (v *Vibranium) BuildImage(opts *pb.BuildImageOptions, stream pb.CoreRPC_BuildImageServer) error {
	v.taskAdd("BuildImage", true)
	defer v.taskDone("BuildImage", true)

	buildOpts, err := toCoreBuildOptions(opts)
	if err != nil {
		return err
	}

	ch, err := v.cluster.BuildImage(stream.Context(), buildOpts)
	if err != nil {
		return err
	}

	for m := range ch {
		if err = stream.Send(toRPCBuildImageMessage(m)); err != nil {
			v.logUnsentMessages("BuildImage", m)
		}
	}
	return err
}

// RemoveImage remove image
func (v *Vibranium) RemoveImage(opts *pb.RemoveImageOptions, stream pb.CoreRPC_RemoveImageServer) error {
	v.taskAdd("RemoveImage", true)
	defer v.taskDone("RemoveImage", true)

	ch, err := v.cluster.RemoveImage(stream.Context(), opts.Podname, opts.Nodename, opts.Images, opts.Prune)
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

// DeployStatus watch and show deployed status
func (v *Vibranium) DeployStatus(opts *pb.DeployStatusOptions, stream pb.CoreRPC_DeployStatusServer) error {
	v.taskAdd("DeployStatus", true)
	defer v.taskDone("DeployStatus", true)

	ch := v.cluster.DeployStatusStream(stream.Context(), opts.Appname, opts.Entrypoint, opts.Nodename)
	for m := range ch {
		if m.Err != nil {
			return m.Err
		}
		if err := stream.Send(&pb.DeployStatusMessage{
			Action:     m.Action,
			Appname:    m.Appname,
			Entrypoint: m.Entrypoint,
			Nodename:   m.Nodename,
			Id:         m.ID,
			Data:       []byte(m.Data),
		}); err != nil {
			v.logUnsentMessages("DeployStatus", m)
		}
	}
	return nil
}

// RunAndWait is lambda
func (v *Vibranium) RunAndWait(stream pb.CoreRPC_RunAndWaitServer) error {
	v.taskAdd("RunAndWait", true)
	defer v.taskDone("RunAndWait", true)

	RunAndWaitOptions, err := stream.Recv()
	if err != nil {
		return err
	}

	if RunAndWaitOptions.DeployOptions == nil {
		return errors.New("no deploy options")
	}

	opts := RunAndWaitOptions.DeployOptions
	stdinReader, stdinWriter := io.Pipe()
	defer stdinReader.Close()

	deployOpts, err := toCoreDeployOptions(opts)
	if err != nil {
		return err
	}
	ch, err := v.cluster.RunAndWait(stream.Context(), deployOpts, stdinReader)
	if err != nil {
		// `ch` is nil now
		log.Errorf("[RunAndWait] Start run and wait failed %s", err)
		return err
	}

	if opts.OpenStdin {
		go func() {
			defer stdinWriter.Write([]byte("exit\n"))
			stdinWriter.Write([]byte("echo 'Welcom to NERV...\n'\n"))
			cli := []byte("echo \"`pwd`> \"\n")
			stdinWriter.Write(cli)
			for {
				RunAndWaitOptions, err := stream.Recv()
				if RunAndWaitOptions == nil || err != nil {
					log.Errorf("[RunAndWait] Recv command error: %v", err)
					break
				}
				log.Debugf("[RunAndWait] Recv command: %s", bytes.TrimRight(RunAndWaitOptions.Cmd, "\n"))
				if _, err := stdinWriter.Write(RunAndWaitOptions.Cmd); err != nil {
					log.Errorf("[RunAndWait] Write command error: %v", err)
					break
				}
				stdinWriter.Write(cli)
			}
		}()
	}

	for m := range ch {
		if err = stream.Send(toRPCRunAndWaitMessage(m)); err != nil {
			v.logUnsentMessages("RunAndWait", m)
		}
	}

	return err
}

// CreateContainer create containers
func (v *Vibranium) CreateContainer(opts *pb.DeployOptions, stream pb.CoreRPC_CreateContainerServer) error {
	v.taskAdd("CreateContainer", true)
	defer v.taskDone("CreateContainer", true)

	deployOpts, err := toCoreDeployOptions(opts)
	if err != nil {
		return nil
	}
	//这里考虑用全局 Background
	ch, err := v.cluster.CreateContainer(context.Background(), deployOpts)
	if err != nil {
		return err
	}

	for m := range ch {
		if err = stream.Send(toRPCCreateContainerMessage(m)); err != nil {
			v.logUnsentMessages("CreateContainer", m)
		}
	}

	return err
}

// ReplaceContainer replace containers
func (v *Vibranium) ReplaceContainer(opts *pb.ReplaceOptions, stream pb.CoreRPC_ReplaceContainerServer) error {
	v.taskAdd("ReplaceContainer", true)
	defer v.taskDone("ReplaceContainer", true)

	deployOpts, err := toCoreDeployOptions(opts.DeployOpt)
	if err != nil {
		return err
	}
	//这里考虑用全局 Background
	ch, err := v.cluster.ReplaceContainer(context.Background(), deployOpts, opts.Force)
	if err != nil {
		return err
	}

	for m := range ch {
		if err = stream.Send(toRPCReplaceContainerMessage(m)); err != nil {
			v.logUnsentMessages("ReplaceContainer", m)
		}
	}

	return err
}

// RemoveContainer remove containers
func (v *Vibranium) RemoveContainer(opts *pb.RemoveContainerOptions, stream pb.CoreRPC_RemoveContainerServer) error {
	v.taskAdd("RemoveContainer", true)
	defer v.taskDone("RemoveContainer", true)

	ids := opts.GetIds()
	force := opts.GetForce()

	if len(ids) == 0 {
		return fmt.Errorf("No container ids given")
	}

	//这里不能让 client 打断 remove
	ch, err := v.cluster.RemoveContainer(context.Background(), ids, force)
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

// ReallocResource realloc res for containers
func (v *Vibranium) ReallocResource(opts *pb.ReallocOptions, stream pb.CoreRPC_ReallocResourceServer) error {
	v.taskAdd("ReallocResource", true)
	defer v.taskDone("ReallocResource", true)
	ids := opts.GetIds()
	if len(ids) == 0 {
		return fmt.Errorf("No container ids given")
	}

	//这里不能让 client 打断 remove
	ch, err := v.cluster.ReallocResource(context.Background(), ids, opts.Cpu, opts.Mem)
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

// GetNodeByName get node by name
func (v *Vibranium) GetNodeByName(ctx context.Context, opts *pb.GetNodeOptions) (*pb.Node, error) {
	n, err := v.cluster.GetNodeByName(ctx, opts.Nodename)
	if err != nil {
		return nil, err
	}

	return toRPCNode(ctx, n), nil
}

// ContainerDeployed store deploy status
func (v *Vibranium) ContainerDeployed(ctx context.Context, opts *pb.ContainerDeployedOptions) (*pb.Empty, error) {
	v.taskAdd("ContainerDeployed", false)
	defer v.taskDone("ContainerDeployed", false)
	return &pb.Empty{}, v.cluster.ContainerDeployed(ctx, opts.Id, opts.Appname, opts.Entrypoint, opts.Nodename, string(opts.Data))
}

func (v *Vibranium) logUnsentMessages(msgType string, msg interface{}) {
	log.Infof("[logUnsentMessages] Unsent %s streamed message: %v", msgType, msg)
}

// New will new a new cluster instance
func New(cluster cluster.Cluster, config types.Config) *Vibranium {
	return &Vibranium{cluster: cluster, config: config, counter: sync.WaitGroup{}}
}
