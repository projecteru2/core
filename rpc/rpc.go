package rpc

import (
	"gitlab.ricebook.net/platform/core/cluster"
	"gitlab.ricebook.net/platform/core/rpc/gen"
	"gitlab.ricebook.net/platform/core/types"
	"golang.org/x/net/context"
)

type Virbranium struct {
	cluster cluster.Cluster
	config  types.Config
}

// Implementations for grpc server interface
// Many data types should be transformed

// ListPods returns a list of pods
func (v *Virbranium) ListPods(ctx context.Context, empty *pb.Empty) (*pb.Pods, error) {
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
func (v *Virbranium) AddPod(ctx context.Context, opts *pb.AddPodOptions) (*pb.Pod, error) {
	p, err := v.cluster.AddPod(opts.Name, opts.Desc)
	if err != nil {
		return nil, err
	}

	return toRPCPod(p), nil
}

// GetPod
func (v *Virbranium) GetPod(ctx context.Context, opts *pb.GetPodOptions) (*pb.Pod, error) {
	p, err := v.cluster.GetPod(opts.Name)
	if err != nil {
		return nil, err
	}

	return toRPCPod(p), nil
}

// AddNode saves a node and returns it to client
// Method must be called syncly, or nothing will be returned
func (v *Virbranium) AddNode(ctx context.Context, opts *pb.AddNodeOptions) (*pb.Node, error) {
	n, err := v.cluster.AddNode(opts.Nodename, opts.Endpoint, opts.Podname, opts.Public)
	if err != nil {
		return nil, err
	}

	return toRPCNode(n), nil
}

// GetNode
func (v *Virbranium) GetNode(ctx context.Context, opts *pb.GetNodeOptions) (*pb.Node, error) {
	n, err := v.cluster.GetNode(opts.Podname, opts.Nodename)
	if err != nil {
		return nil, err
	}

	return toRPCNode(n), nil
}

// ListPodNodes returns a list of node for pod
func (v *Virbranium) ListPodNodes(ctx context.Context, opts *pb.ListNodesOptions) (*pb.Nodes, error) {
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
// TODO information from `inspect` should be returned here, guess it's better to return it as string
func (v *Virbranium) GetContainer(ctx context.Context, id *pb.ContainerID) (*pb.Container, error) {
	container, err := v.cluster.GetContainer(id.Id)
	if err != nil {
		return nil, err
	}

	return toRPCContainer(container), nil
}

// GetContainers
// like GetContainer, information should be returned
func (v *Virbranium) GetContainers(ctx context.Context, cids *pb.ContainerIDs) (*pb.Containers, error) {
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
		cs = append(cs, toRPCContainer(c))
	}
	return &pb.Containers{Containers: cs}, nil
}

// streamed returned functions
func (v *Virbranium) BuildImage(opts *pb.BuildImageOptions, stream pb.CoreRPC_BuildImageServer) error {
	return nil
}

func (v *Virbranium) CreateContainer(opts *pb.DeployOptions, stream pb.CoreRPC_CreateContainerServer) error {
	return nil
}

func (v *Virbranium) RemoveContainer(cids *pb.ContainerIDs, stream pb.CoreRPC_RemoveContainerServer) error {
	return nil
}

func (v *Virbranium) RemoveImage(opts *pb.RemoveImageOptions, stream pb.CoreRPC_RemoveImageServer) error {
	return nil
}

func NewVirbranium(cluster cluster.Cluster, config types.Config) *Virbranium {
	return &Virbranium{cluster: cluster, config: config}
}
