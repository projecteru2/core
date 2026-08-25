package types

import (
	"context"
	"encoding/json"
	"maps"
	"slices"

	"github.com/cockroachdb/errors"

	engine "github.com/projecteru2/core/engine"
	resourcetypes "github.com/projecteru2/core/resource/types"
)

type NodeMeta struct {
	Name     string            `json:"name"`
	Endpoint string            `json:"endpoint"`
	Podname  string            `json:"podname"`
	Labels   map[string]string `json:"labels"`

	Ca   string `json:"-"`
	Cert string `json:"-"`
	Key  string `json:"-"`
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
	Podname  string            `yaml:"podname"`
	Includes []string          `yaml:"includes"`
	Excludes []string          `yaml:"excludes"`
	Labels   map[string]string `yaml:"labels"`
	All      bool              `yaml:"all"`
}

// Narrow intersects other into f on pod, names and labels; other may only shrink the selection.
func (f NodeFilter) Narrow(other *NodeFilter) (*NodeFilter, error) {
	if other == nil {
		return &f, nil
	}
	if other.Podname != "" {
		if f.Podname != "" && f.Podname != other.Podname {
			return nil, errors.Wrapf(ErrInvaildNodeFilter, "pod %s is outside pod %s", other.Podname, f.Podname)
		}
		f.Podname = other.Podname
	}
	includes, err := narrowNames(f.Includes, other.Includes)
	if err != nil {
		return nil, err
	}
	f.Includes = includes
	f.Excludes = slices.Concat(f.Excludes, other.Excludes)

	if len(other.Labels) > 0 {
		labels := maps.Clone(f.Labels)
		if labels == nil {
			labels = map[string]string{}
		}
		for key, value := range other.Labels {
			if kept, ok := labels[key]; ok && kept != value {
				return nil, errors.Wrapf(ErrInvaildNodeFilter, "label %s=%s is outside %s=%s", key, value, key, kept)
			}
			labels[key] = value
		}
		f.Labels = labels
	}
	return &f, nil
}

// narrowNames keeps the requested names the configured list allows; an empty configured list allows every name.
func narrowNames(configured, requested []string) ([]string, error) {
	if len(requested) == 0 {
		return configured, nil
	}
	if len(configured) == 0 {
		return requested, nil
	}
	narrowed := slices.DeleteFunc(slices.Clone(requested), func(name string) bool {
		return !slices.Contains(configured, name)
	})
	if len(narrowed) == 0 {
		return nil, errors.Wrapf(ErrInvaildNodeFilter, "nodes %v are outside %v", requested, configured)
	}
	return narrowed, nil
}
