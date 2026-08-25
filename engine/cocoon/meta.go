package cocoon

import (
	"context"
	"path/filepath"

	"github.com/projecteru2/core/cluster"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const kindVM = "vm"

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
}

func newMeta(ctx context.Context, ID string, opts *enginetypes.VirtualizationCreateOptions, vm *vmRecord, nodename string, cocoon coretypes.CocoonConfig) *meta {
	appname, entrypoint, ident, _ := utils.ParseWorkloadName(opts.Name)
	label := utils.DecodeMetaInLabel(ctx, opts.Labels)
	record := &meta{
		ID:         ID,
		Kind:       kindVM,
		Name:       opts.Name,
		Appname:    appname,
		Entrypoint: entrypoint,
		Ident:      ident,
		Podname:    lastEnvValue(opts.Env, podEnvKey),
		Nodename:   nodename,
		CoreID:     opts.Labels[cluster.LabelCoreID],
		Labels:     opts.Labels,
		Publish:    label.Publish,
		Networks:   vm.networks(),
		Cgroup:     scopePath(cocoon.CgroupParent, vm.ID),
		Iface:      vm.tap(),
		Log:        logMeta{ConsoleSocket: filepath.Join(vm.runDir(cocoon.RunDir), consoleSock)},
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
	ConsoleSocket string `json:"console_socket"`
}
