package types

import (
	"context"
	"encoding/json"

	"github.com/mitchellh/mapstructure"
	engine "github.com/projecteru2/core/engine"
	resourcetypes "github.com/projecteru2/core/resource/types"
)

// NodeMeta .
type NodeMeta struct {
	Name     string            `json:"name" mapstructure:"name"`
	Endpoint string            `json:"endpoint" mapstructure:"endpoint"`
	Podname  string            `json:"podname" mapstructure:"podname"`
	Labels   map[string]string `json:"labels" mapstructure:"labels"`

	Ca   string `json:"-" mapstructure:"-"`
	Cert string `json:"-" mapstructure:"-"`
	Key  string `json:"-" mapstructure:"-"`
}

// DeepCopy returns a deepcopy of nodemeta
func (n NodeMeta) DeepCopy() (nn NodeMeta, err error) {
	return nn, mapstructure.Decode(n, &nn)
}

// NodeResourceInfo for node resource info
type NodeResourceInfo struct {
	Name      string                  `json:"-"`
	Capacity  resourcetypes.Resources `json:"capacity,omitempty"`
	Usage     resourcetypes.Resources `json:"usage,omitempty"`
	Diffs     []string                `json:"diffs,omitempty"`
	Workloads []*Workload             `json:"-"`
}

// Node store node info
type Node struct {
	NodeMeta
	// Bypass if bypass is true, it will not participate in future scheduling
	Bypass bool `json:"bypass,omitempty"`
	// Test mean can ignore node health check
	Test bool `json:"test,omitempty"`

	ResourceInfo NodeResourceInfo `json:"-"`
	NodeInfo     string           `json:"-"`
	Available    bool             `json:"-"`
	Engine       engine.API       `json:"-"`
}

// Info show node info
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

// IsDown returns if the node is marked as down.
func (n *Node) IsDown() bool {
	// If `bypass` is true, then even if the node is still healthy, the node will be regarded as `down`.
	// Currently `bypass` will only be set when the cli calls the `up` and `down` commands.
	return n.Bypass || !n.Available
}

// NodeStatus wraps node status
// only used for node status stream
type NodeStatus struct {
	Nodename string
	Podname  string
	Alive    bool
	Error    error
}

// NodeFilter is used to filter nodes in a pod
// names in includes will be used
// names in excludes will not be used
type NodeFilter struct {
	Podname  string
	Includes []string
	Excludes []string
	Labels   map[string]string
	All      bool
}
