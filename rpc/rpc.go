package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
	"golang.org/x/net/context"
)

type vibranium struct {
	cluster cluster.Cluster
	config  types.Config
	counter sync.WaitGroup
	TaskNum int
}

// Implementations for grpc server interface
// Many data types should be transformed

// ListPods returns a list of pods
func (v *vibranium) ListPods(ctx context.Context, _ *pb.Empty) (*pb.Pods, error) {
	ps, err := v.cluster.ListPods()
	if err != nil {
		return nil, err
	}

	pods := []*pb.Pod{}
	for _, p := range ps {
		pods = append(pods, toRPCPod(p))
	}

	return &pb.Pods{Pods: pods}, nil

}

// AddPod saves a pod, and returns it to client
func (v *vibranium) AddPod(ctx context.Context, opts *pb.AddPodOptions) (*pb.Pod, error) {
	p, err := v.cluster.AddPod(opts.Name, opts.Favor, opts.Desc)
	if err != nil {
		return nil, err
	}

	return toRPCPod(p), nil
}

// RemovePod removes a pod only if it's empty
func (v *vibranium) RemovePod(ctx context.Context, opts *pb.RemovePodOptions) (*pb.Empty, error) {
	err := v.cluster.RemovePod(opts.Name)
	if err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

// GetPod
func (v *vibranium) GetPod(ctx context.Context, opts *pb.GetPodOptions) (*pb.Pod, error) {
	p, err := v.cluster.GetPod(opts.Name)
	if err != nil {
		return nil, err
	}

	return toRPCPod(p), nil
}

// AddNode saves a node and returns it to client
// Method must be called synchronously, or nothing will be returned
func (v *vibranium) AddNode(ctx context.Context, opts *pb.AddNodeOptions) (*pb.Node, error) {
	n, err := v.cluster.AddNode(opts.Nodename, opts.Endpoint, opts.Podname, opts.Cafile, opts.Certfile, opts.Keyfile, opts.Public)
	if err != nil {
		return nil, err
	}

	return toRPCNode(n), nil
}

// RemoveNode removes the node from etcd
func (v *vibranium) RemoveNode(ctx context.Context, opts *pb.RemoveNodeOptions) (*pb.Pod, error) {
	p, err := v.cluster.RemoveNode(opts.Nodename, opts.Podname)
	if err != nil {
		return nil, err
	}

	return toRPCPod(p), nil
}

// GetNode
func (v *vibranium) GetNode(ctx context.Context, opts *pb.GetNodeOptions) (*pb.Node, error) {
	n, err := v.cluster.GetNode(opts.Podname, opts.Nodename)
	if err != nil {
		return nil, err
	}

	return toRPCNode(n), nil
}

// GetNodeByName
func (v *vibranium) GetNodeByName(ctx context.Context, opts *pb.GetNodeOptions) (*pb.Node, error) {
	n, err := v.cluster.GetNodeByName(opts.Nodename)
	if err != nil {
		return nil, err
	}

	return toRPCNode(n), nil
}

// ListPodNodes returns a list of node for pod
func (v *vibranium) ListPodNodes(ctx context.Context, opts *pb.ListNodesOptions) (*pb.Nodes, error) {
	ns, err := v.cluster.ListPodNodes(opts.Podname, opts.All)
	if err != nil {
		return nil, err
	}

	nodes := []*pb.Node{}
	for _, n := range ns {
		nodes = append(nodes, toRPCNode(n))
	}
	return &pb.Nodes{Nodes: nodes}, nil
}

// GetContainer
// More information will be shown
func (v *vibranium) GetContainer(ctx context.Context, id *pb.ContainerID) (*pb.Container, error) {
	container, err := v.cluster.GetContainer(id.Id)
	if err != nil {
		return nil, err
	}

	info, err := container.Inspect()
	if err != nil {
		return nil, err
	}

	bytes, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}

	return toRPCContainer(container, string(bytes)), nil
}

//GetContainers
//like GetContainer, information should be returned
func (v *vibranium) GetContainers(ctx context.Context, cids *pb.ContainerIDs) (*pb.Containers, error) {
	containers, err := v.cluster.GetContainers(cids.GetIds())
	if err != nil {
		return nil, err
	}

	cs := []*pb.Container{}
	for _, c := range containers {
		info, err := c.Inspect()
		if err != nil {
			continue
		}

		bytes, err := json.Marshal(info)
		if err != nil {
			continue
		}

		cs = append(cs, toRPCContainer(c, string(bytes)))
	}
	return &pb.Containers{Containers: cs}, nil
}

// list networks for pod
func (v *vibranium) ListNetworks(ctx context.Context, opts *pb.GetPodOptions) (*pb.Networks, error) {
	networks, err := v.cluster.ListNetworks(opts.Name)
	if err != nil {
		return nil, err
	}

	ns := []*pb.Network{}
	for _, n := range networks {
		ns = append(ns, toRPCNetwork(n))
	}
	return &pb.Networks{Networks: ns}, nil
}

// set node availability
func (v *vibranium) SetNodeAvailable(ctx context.Context, opts *pb.NodeAvailable) (*pb.Node, error) {
	n, err := v.cluster.SetNodeAvailable(opts.Podname, opts.Nodename, opts.Available)
	if err != nil {
		return nil, err
	}
	return toRPCNode(n), nil
}

// streamed returned functions
// caller must ensure that timeout will not be too short because these actions take a little time
func (v *vibranium) BuildImage(opts *pb.BuildImageOptions, stream pb.CoreRPC_BuildImageServer) error {
	v.taskAdd("BuildImage", true)
	defer v.taskDone("BuildImage", true)

	buildOpts, err := toCoreBuildOptions(opts)
	if err != nil {
		return err
	}

	ch, err := v.cluster.BuildImage(buildOpts)
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

func (v *vibranium) CreateContainer(opts *pb.DeployOptions, stream pb.CoreRPC_CreateContainerServer) error {
	v.taskAdd("CreateContainer", true)
	defer v.taskDone("CreateContainer", true)

	deployOpts, err := toCoreDeployOptions(opts)
	if err != nil {
		return nil
	}
	ch, err := v.cluster.CreateContainer(deployOpts)
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

func (v *vibranium) RunAndWait(stream pb.CoreRPC_RunAndWaitServer) error {
	v.taskAdd("RunAndWait", true)
	defer v.taskDone("RunAndWait", true)

	RunAndWaitOptions, err := stream.Recv()
	if err != nil {
		return err
	}

	opts := RunAndWaitOptions.DeployOptions
	timeout := int(RunAndWaitOptions.Timeout)
	stdinReader, stdinWriter := io.Pipe()
	defer stdinReader.Close()

	deployOpts, err := toCoreDeployOptions(opts)
	if err != nil {
		return err
	}
	ch, err := v.cluster.RunAndWait(deployOpts, timeout, stdinReader)
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
			log.Errorf("[RunAndWait] Send msg error: %s", err)
			v.logUnsentMessages("RunAndWait", m)
		}
	}

	return err
}

func (v *vibranium) RemoveContainer(opts *pb.RemoveContainerOptions, stream pb.CoreRPC_RemoveContainerServer) error {
	v.taskAdd("RemoveContainer", true)
	defer v.taskDone("RemoveContainer", true)

	ids := opts.GetIds()
	force := opts.GetForce()

	if len(ids) == 0 {
		return fmt.Errorf("No container ids given")
	}

	ch, err := v.cluster.RemoveContainer(ids, force)
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

func (v *vibranium) ReallocResource(opts *pb.ReallocOptions, stream pb.CoreRPC_ReallocResourceServer) error {
	v.taskAdd("ReallocResource", true)
	defer v.taskDone("ReallocResource", true)
	ids := opts.GetIds()
	if len(ids) == 0 {
		return fmt.Errorf("No container ids given")
	}

	ch, err := v.cluster.ReallocResource(ids, opts.Cpu, opts.Mem)
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

func (v *vibranium) RemoveImage(opts *pb.RemoveImageOptions, stream pb.CoreRPC_RemoveImageServer) error {
	v.taskAdd("RemoveImage", true)
	defer v.taskDone("RemoveImage", true)

	ch, err := v.cluster.RemoveImage(opts.Podname, opts.Nodename, opts.Images)
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

func (v *vibranium) Backup(ctx context.Context, opts *pb.BackupOptions) (*pb.BackupMessage, error) {
	v.taskAdd("Backup", false)
	defer v.taskDone("Backup", false)

	backupMessage, err := v.cluster.Backup(opts.Id, opts.SrcPath)
	if err != nil {
		return nil, err
	}
	return toRPCBackupMessage(backupMessage), nil
}

func (v *vibranium) ContainerDeployed(ctx context.Context, opts *pb.ContainerDeployedOptions) (*pb.Empty, error) {
	v.taskAdd("ContainerDeployed", false)
	defer v.taskDone("ContainerDeployed", false)
	return &pb.Empty{}, v.cluster.ContainerDeployed(opts.Id, opts.Appname, opts.Entrypoint, opts.Nodename, string(opts.Data))
}

func (v *vibranium) logUnsentMessages(msgType string, msg interface{}) {
	log.Infof("[logUnsentMessages] Unsent %s streamed message: %v", msgType, msg)
}

func New(cluster cluster.Cluster, config types.Config) *vibranium {
	return &vibranium{cluster: cluster, config: config, counter: sync.WaitGroup{}}
}
