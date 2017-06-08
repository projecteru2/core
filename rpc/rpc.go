package rpc

import (
	"encoding/json"
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"
	"gitlab.ricebook.net/platform/core/cluster"
	"gitlab.ricebook.net/platform/core/rpc/gen"
	"gitlab.ricebook.net/platform/core/types"
	"golang.org/x/net/context"
)

type virbranium struct {
	cluster cluster.Cluster
	config  types.Config
	counter sync.WaitGroup
}

// Implementations for grpc server interface
// Many data types should be transformed

// ListPods returns a list of pods
func (v *virbranium) ListPods(ctx context.Context, empty *pb.Empty) (*pb.Pods, error) {
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
func (v *virbranium) AddPod(ctx context.Context, opts *pb.AddPodOptions) (*pb.Pod, error) {
	p, err := v.cluster.AddPod(opts.Name, opts.Desc)
	if err != nil {
		return nil, err
	}

	return toRPCPod(p), nil
}

// GetPod
func (v *virbranium) GetPod(ctx context.Context, opts *pb.GetPodOptions) (*pb.Pod, error) {
	p, err := v.cluster.GetPod(opts.Name)
	if err != nil {
		return nil, err
	}

	return toRPCPod(p), nil
}

// AddNode saves a node and returns it to client
// Method must be called synchronously, or nothing will be returned
func (v *virbranium) AddNode(ctx context.Context, opts *pb.AddNodeOptions) (*pb.Node, error) {
	n, err := v.cluster.AddNode(opts.Nodename, opts.Endpoint, opts.Podname, opts.Cafile, opts.Certfile, opts.Keyfile, opts.Public)
	if err != nil {
		return nil, err
	}

	return toRPCNode(n, v.cluster.GetZone()), nil
}

// AddNode saves a node and returns it to client
// Method must be called synchronously, or nothing will be returned
func (v *virbranium) RemoveNode(ctx context.Context, opts *pb.RemoveNodeOptions) (*pb.Pod, error) {
	p, err := v.cluster.RemoveNode(opts.Nodename, opts.Podname)
	if err != nil {
		return nil, err
	}

	return toRPCPod(p), nil
}

// GetNode
func (v *virbranium) GetNode(ctx context.Context, opts *pb.GetNodeOptions) (*pb.Node, error) {
	n, err := v.cluster.GetNode(opts.Podname, opts.Nodename)
	if err != nil {
		return nil, err
	}

	return toRPCNode(n, v.cluster.GetZone()), nil
}

// ListPodNodes returns a list of node for pod
func (v *virbranium) ListPodNodes(ctx context.Context, opts *pb.ListNodesOptions) (*pb.Nodes, error) {
	ns, err := v.cluster.ListPodNodes(opts.Podname, opts.All)
	if err != nil {
		return nil, err
	}

	nodes := []*pb.Node{}
	for _, n := range ns {
		nodes = append(nodes, toRPCNode(n, v.cluster.GetZone()))
	}
	return &pb.Nodes{Nodes: nodes}, nil
}

// GetContainer
// More information will be shown
func (v *virbranium) GetContainer(ctx context.Context, id *pb.ContainerID) (*pb.Container, error) {
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

// GetContainers
// like GetContainer, information should be returned
func (v *virbranium) GetContainers(ctx context.Context, cids *pb.ContainerIDs) (*pb.Containers, error) {
	ids := []string{}
	for _, id := range cids.Ids {
		ids = append(ids, id.Id)
	}

	containers, err := v.cluster.GetContainers(ids)
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
func (v *virbranium) ListNetworks(ctx context.Context, opts *pb.GetPodOptions) (*pb.Networks, error) {
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
func (v *virbranium) SetNodeAvailable(ctx context.Context, opts *pb.NodeAvailable) (*pb.Node, error) {
	n, err := v.cluster.SetNodeAvailable(opts.Podname, opts.Nodename, opts.Available)
	if err != nil {
		return nil, err
	}
	return toRPCNode(n, v.cluster.GetZone()), nil
}

// streamed returned functions
// caller must ensure that timeout will not be too short because these actions take a little time
func (v *virbranium) BuildImage(opts *pb.BuildImageOptions, stream pb.CoreRPC_BuildImageServer) error {
	ch, err := v.cluster.BuildImage(opts.Repo, opts.Version, opts.Uid, opts.Artifact)
	if err != nil {
		return err
	}

	for m := range ch {
		if err := stream.Send(toRPCBuildImageMessage(m)); err != nil {
			go func() {
				for r := range ch {
					log.Infof("[BuildImage] Unsent streamed message: %v", r)
				}
			}()
			return err
		}
	}
	return nil
}

func (v *virbranium) CreateContainer(opts *pb.DeployOptions, stream pb.CoreRPC_CreateContainerServer) error {
	v.taskAdd("CreateContainer", true)

	specs, err := types.LoadSpecs(opts.Specs)
	if err != nil {
		return err
	}

	ch, err := v.cluster.CreateContainer(specs, toCoreDeployOptions(opts))
	if err != nil {
		return err
	}

	for m := range ch {
		if err := stream.Send(toRPCCreateContainerMessage(m)); err != nil {
			go func() {
				defer v.taskDone("CreateContainer", true)
				for r := range ch {
					log.Infof("[CreateContainer] Unsent streamed message: %v", r)
				}
			}()
			return err
		}
	}

	v.taskDone("CreateContainer", true)
	return nil
}

func (v *virbranium) RunAndWait(opts *pb.DeployOptions, stream pb.CoreRPC_RunAndWaitServer) error {
	v.taskAdd("RunAndWait", true)

	specs, err := types.LoadSpecs(opts.Specs)
	if err != nil {
		return err
	}

	ch, err := v.cluster.RunAndWait(specs, toCoreDeployOptions(opts))
	if err != nil {
		return err
	}

	for m := range ch {
		if err := stream.Send(toRPCRunAndWaitMessage(m)); err != nil {
			go func() {
				defer v.taskDone("RunAndWait", true)
				for r := range ch {
					log.Infof("[RunAndWait] Unsent streamed message: %v", r.Data)
				}
			}()
			return err
		}
	}

	v.taskDone("RunAndWait", true)
	return nil
}

func (v *virbranium) UpgradeContainer(opts *pb.UpgradeOptions, stream pb.CoreRPC_UpgradeContainerServer) error {
	v.taskAdd("UpgradeContainer", true)

	ids := []string{}
	for _, id := range opts.Ids {
		ids = append(ids, id.Id)
	}

	ch, err := v.cluster.UpgradeContainer(ids, opts.Image)
	if err != nil {
		return err
	}

	for m := range ch {
		if err := stream.Send(toRPCUpgradeContainerMessage(m)); err != nil {
			go func() {
				defer v.taskDone("UpgradeContainer", true)
				for r := range ch {
					log.Infof("[UpgradeContainer] Unsent streamed message: %v", r)
				}
			}()
			return err
		}
	}

	v.taskDone("UpgradeContainer", true)
	return nil
}

func (v *virbranium) RemoveContainer(cids *pb.ContainerIDs, stream pb.CoreRPC_RemoveContainerServer) error {
	v.taskAdd("RemoveContainer", true)

	ids := []string{}
	for _, id := range cids.Ids {
		ids = append(ids, id.Id)
	}

	if len(ids) == 0 {
		return fmt.Errorf("No container ids given")
	}

	ch, err := v.cluster.RemoveContainer(ids)
	if err != nil {
		return err
	}

	for m := range ch {
		if err := stream.Send(toRPCRemoveContainerMessage(m)); err != nil {
			go func() {
				defer v.taskDone("RemoveContainer", true)
				for r := range ch {
					log.Infof("[RemoveContainer] Unsent streamed message: %v", r)
				}
			}()
			return err
		}
	}

	v.taskDone("RemoveContainer", true)
	return nil
}

func (v *virbranium) RemoveImage(opts *pb.RemoveImageOptions, stream pb.CoreRPC_RemoveImageServer) error {
	ch, err := v.cluster.RemoveImage(opts.Podname, opts.Nodename, opts.Images)
	if err != nil {
		return err
	}

	for m := range ch {
		if err := stream.Send(toRPCRemoveImageMessage(m)); err != nil {
			go func() {
				for r := range ch {
					log.Infof("[RemoveImage] Unsent streamed message: %v", r)
				}
			}()
			return err
		}
	}
	return nil
}

func (v *virbranium) Backup(ctx context.Context, opts *pb.BackupOptions) (*pb.BackupMessage, error) {
	v.taskAdd("Backup", false)
	defer v.taskDone("Backup", false)

	backupMessage, err := v.cluster.Backup(opts.Id, opts.SrcPath)
	if err != nil {
		return nil, err
	}
	return toRPCBackupMessage(backupMessage), nil
}

func New(cluster cluster.Cluster, config types.Config) *virbranium {
	return &virbranium{cluster: cluster, config: config, counter: sync.WaitGroup{}}
}
