package rpc

import (
	"encoding/json"
	"fmt"

	"gitlab.ricebook.net/platform/core/cluster"
	"gitlab.ricebook.net/platform/core/rpc/gen"
	"gitlab.ricebook.net/platform/core/types"
	"golang.org/x/net/context"
)

type virbranium struct {
	cluster cluster.Cluster
	config  types.Config
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
// Method must be called syncly, or nothing will be returned
func (v *virbranium) AddNode(ctx context.Context, opts *pb.AddNodeOptions) (*pb.Node, error) {
	n, err := v.cluster.AddNode(opts.Nodename, opts.Endpoint, opts.Podname, opts.Cafile, opts.Certfile, opts.Keyfile, opts.Public)
	if err != nil {
		return nil, err
	}

	return toRPCNode(n), nil
}

// GetNode
func (v *virbranium) GetNode(ctx context.Context, opts *pb.GetNodeOptions) (*pb.Node, error) {
	n, err := v.cluster.GetNode(opts.Podname, opts.Nodename)
	if err != nil {
		return nil, err
	}

	return toRPCNode(n), nil
}

// ListPodNodes returns a list of node for pod
func (v *virbranium) ListPodNodes(ctx context.Context, opts *pb.ListNodesOptions) (*pb.Nodes, error) {
	ns, err := v.cluster.ListPodNodes(opts.Podname)
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

// streamed returned functions
// caller must ensure that timeout will not be too short because these actions take a little time
func (v *virbranium) BuildImage(opts *pb.BuildImageOptions, stream pb.CoreRPC_BuildImageServer) error {
	ch, err := v.cluster.BuildImage(opts.Repo, opts.Version, opts.Uid, opts.Artifact)
	if err != nil {
		return err
	}

	for m := range ch {
		if err := stream.Send(toRPCBuildImageMessage(m)); err != nil {
			return err
		}
	}
	return nil
}

func (v *virbranium) CreateContainer(opts *pb.DeployOptions, stream pb.CoreRPC_CreateContainerServer) error {
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
			return err
		}
	}
	return nil
}

func (v *virbranium) UpgradeContainer(opts *pb.UpgradeOptions, stream pb.CoreRPC_UpgradeContainerServer) error {
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
			return err
		}
	}
	return nil
}

func (v *virbranium) RemoveContainer(cids *pb.ContainerIDs, stream pb.CoreRPC_RemoveContainerServer) error {
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
			return err
		}
	}
	return nil
}

func (v *virbranium) RemoveImage(opts *pb.RemoveImageOptions, stream pb.CoreRPC_RemoveImageServer) error {
	ch, err := v.cluster.RemoveImage(opts.Podname, opts.Nodename, opts.Images)
	if err != nil {
		return err
	}

	for m := range ch {
		if err := stream.Send(toRPCRemoveImageMessage(m)); err != nil {
			return err
		}
	}
	return nil
}

func New(cluster cluster.Cluster, config types.Config) *virbranium {
	return &virbranium{cluster: cluster, config: config}
}
