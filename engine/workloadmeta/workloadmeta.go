package workloadmeta

import (
	"context"
	"path/filepath"

	"github.com/projecteru2/core/cluster"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/utils"
)

const (
	// Dir is where core writes workload records for eru-agent on SSH-managed nodes.
	Dir        = "/run/eru/workloads"
	CgroupRoot = "/sys/fs/cgroup"
	// NotExistsCode is the exit code node-side scripts use for "no record on the node".
	NotExistsCode = 64

	metaSuffix = ".json"
)

// Record is the workload record core writes next to an SSH-managed workload for eru-agent.
type Record struct {
	ID          string                   `json:"id"`
	Kind        string                   `json:"kind"`
	Name        string                   `json:"name"`
	Appname     string                   `json:"appname"`
	Entrypoint  string                   `json:"entrypoint"`
	Ident       string                   `json:"ident"`
	Podname     string                   `json:"podname"`
	Nodename    string                   `json:"nodename"`
	CoreID      string                   `json:"coreid"`
	Labels      map[string]string        `json:"labels,omitempty"`
	HealthCheck *enginetypes.HealthCheck `json:"healthcheck,omitempty"`
	Publish     []string                 `json:"publish,omitempty"`
	Networks    map[string]string        `json:"networks,omitempty"`
	Cgroup      string                   `json:"cgroup"`
	NetnsPID    int                      `json:"netns_pid"`
	Iface       string                   `json:"iface,omitempty"`
}

func NewRecord(ctx context.Context, ID, kind, name, podname, nodename string, labels map[string]string) Record {
	appname, entrypoint, ident, _ := utils.ParseWorkloadName(name)
	label := utils.DecodeMetaInLabel(ctx, labels)
	return Record{
		ID:          ID,
		Kind:        kind,
		Name:        name,
		Appname:     appname,
		Entrypoint:  entrypoint,
		Ident:       ident,
		Podname:     podname,
		Nodename:    nodename,
		CoreID:      labels[cluster.LabelCoreID],
		Labels:      labels,
		HealthCheck: utils.NewHealthCheck(label.HealthCheck),
		Publish:     label.Publish,
	}
}

// Path is the record file for ID under Dir.
func Path(ID string) string {
	return filepath.Join(Dir, ID+metaSuffix)
}
