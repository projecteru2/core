package process

import (
	"context"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/utils"
)

// meta is the workload record core writes next to an SSH-managed workload for eru-agent.
type meta struct {
	ID          string            `json:"id"`
	Kind        string            `json:"kind"`
	Name        string            `json:"name"`
	Appname     string            `json:"appname"`
	Entrypoint  string            `json:"entrypoint"`
	Ident       string            `json:"ident"`
	Podname     string            `json:"podname"`
	Nodename    string            `json:"nodename"`
	CoreID      string            `json:"coreid"`
	Labels      map[string]string `json:"labels,omitempty"`
	HealthCheck *healthCheck      `json:"healthcheck,omitempty"`
	Publish     []string          `json:"publish,omitempty"`
	Networks    map[string]string `json:"networks,omitempty"`
	Cgroup      string            `json:"cgroup"`
	NetnsPID    int               `json:"netns_pid"`
	Iface       string            `json:"iface,omitempty"`
	Log         logMeta           `json:"log"`

	RootDirectory string `json:"root_directory,omitempty"` // empty for a raw host process
	WorkingDir    string `json:"working_dir,omitempty"`
}

func newMeta(ctx context.Context, u *unit, nodename, host string) *meta {
	appname, entrypoint, ident, _ := utils.ParseWorkloadName(u.Opts.Name)
	label := utils.DecodeMetaInLabel(ctx, u.Opts.Labels)
	record := &meta{
		ID:            u.ID,
		Kind:          Type,
		Name:          u.Opts.Name,
		Appname:       appname,
		Entrypoint:    entrypoint,
		Ident:         ident,
		Podname:       u.Podname,
		Nodename:      nodename,
		CoreID:        u.Opts.Labels[cluster.LabelCoreID],
		Labels:        u.Opts.Labels,
		Publish:       label.Publish,
		Networks:      map[string]string{hostNetwork: host},
		Cgroup:        cgroupPath(sliceName(u.Podname), unitName(u.ID)),
		Log:           logMeta{JournalUnit: unitName(u.ID)},
		RootDirectory: u.Root,
		WorkingDir:    u.Working,
	}
	if label.HealthCheck != nil {
		record.HealthCheck = &healthCheck{
			TCPPorts: label.HealthCheck.TCPPorts,
			HTTPPort: label.HealthCheck.HTTPPort,
			HTTPURL:  label.HealthCheck.HTTPURL,
			HTTPCode: label.HealthCheck.HTTPCode,
		}
	}
	return record
}

type healthCheck struct {
	TCPPorts []string `json:"tcp_ports,omitempty"`
	HTTPPort string   `json:"http_port,omitempty"`
	HTTPURL  string   `json:"http_url,omitempty"`
	HTTPCode int      `json:"http_code,omitempty"`
}

type logMeta struct {
	JournalUnit string `json:"journal_unit"`
}
