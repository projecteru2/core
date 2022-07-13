package types

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"

	engine "github.com/projecteru2/core/engine"
)

// NodeMeta .
type NodeMeta struct {
	Name     string            `json:"name"`
	Endpoint string            `json:"endpoint"`
	Podname  string            `json:"podname"`
	Labels   map[string]string `json:"labels"`

	Ca   string `json:"-"`
	Cert string `json:"-"`
	Key  string `json:"-"`
}

// DeepCopy returns a deepcopy of nodemeta
func (n NodeMeta) DeepCopy() (nn NodeMeta, err error) {
	b, err := json.Marshal(n)
	if err != nil {
		return nn, errors.WithStack(err)
	}
	return nn, errors.WithStack(json.Unmarshal(b, &nn))
}

// NodeResource for resource
type NodeResource struct {
	Name      string                      `json:"-"`
	Capacity  map[string]NodeResourceArgs `json:"capacity,omitempty"`
	Usage     map[string]NodeResourceArgs `json:"usage,omitempty"`
	Diffs     []string                    `json:"diffs,omitempty"`
	Workloads []*Workload                 `json:"-"`
}

// Node store node info
type Node struct {
	NodeMeta
	Resource NodeResource `jason:"resource,omitempty"`
	NodeInfo string       `json:"-"`

	// Bypass if bypass is true, it will not participate in future scheduling
	Bypass    bool       `json:"bypass,omitempty"`
	Available bool       `json:"-"`
	Engine    engine.API `json:"-"`
}

// Info show node info
func (n *Node) Info(ctx context.Context) (err error) {
	info, err := n.Engine.Info(ctx)
	if err != nil {
		n.Available = false
		n.NodeInfo = err.Error()
		return errors.WithStack(err)
	}
	bs, err := json.Marshal(info)
	if err != nil {
		n.NodeInfo = err.Error()
		return errors.WithStack(err)
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
