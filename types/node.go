package types

import (
	"context"
	"encoding/json"

	"github.com/go-viper/mapstructure/v2"

	engine "github.com/projecteru2/core/engine"
	resourcetypes "github.com/projecteru2/core/resource/types"
)

type NodeMeta struct {
	Name     string            `json:"name" mapstructure:"name"`
	Endpoint string            `json:"endpoint" mapstructure:"endpoint"`
	Podname  string            `json:"podname" mapstructure:"podname"`
	Labels   map[string]string `json:"labels" mapstructure:"labels"`

	Ca   string `json:"-" mapstructure:"-"`
	Cert string `json:"-" mapstructure:"-"`
	Key  string `json:"-" mapstructure:"-"`
}

func (n NodeMeta) DeepCopy() (nn NodeMeta, err error) {
	return nn, mapstructure.Decode(n, &nn)
}

type NodeResourceInfo struct {
	Name      string                  `json:"-"`
	Capacity  resourcetypes.Resources `json:"capacity,omitempty"`
	Usage     resourcetypes.Resources `json:"usage,omitempty"`
	Diffs     []string                `json:"diffs,omitempty"`
	Workloads []*Workload             `json:"-"`
}

type Node struct {
	NodeMeta
	// Bypass excludes the node from future scheduling.
	Bypass bool `json:"bypass,omitempty"`
	// Test skips the node health check.
	Test bool `json:"test,omitempty"`

	ResourceInfo NodeResourceInfo `json:"-"`
	NodeInfo     string           `json:"-"`
	Available    bool             `json:"-"`
	Engine       engine.API       `json:"-"`
}

func (n *Node) Info(ctx context.Context) (err error) {
	info, err := n.Engine.Info(ctx)
	if err != nil {
		n.Available = false
		n.NodeInfo = err.Error()
		return err
	}
	bs, err := json.Marshal(info)
	if err != nil {
		n.NodeInfo = err.Error()
		return err
	}
	n.NodeInfo = string(bs)
	return nil
}

func (n *Node) IsDown() bool {
	return n.Bypass || !n.Available
}

// NodeStatus carries one node status stream event.
type NodeStatus struct {
	Nodename string
	Podname  string
	Alive    bool
	Error    error
}

// NodeFilter selects nodes in a pod by Includes, then drops Excludes.
type NodeFilter struct {
	Podname  string
	Includes []string
	Excludes []string
	Labels   map[string]string
	All      bool
}
