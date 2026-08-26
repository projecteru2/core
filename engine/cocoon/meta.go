package cocoon

import (
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/engine/workloadmeta"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const kindVM = "vm"

type meta struct {
	workloadmeta.Record
	User string  `json:"user,omitempty"`
	Log  logMeta `json:"log"`
}

func newMeta(ctx context.Context, ID string, opts *enginetypes.VirtualizationCreateOptions, vm *vmRecord, nodename string, cocoon coretypes.CocoonConfig) *meta {
	m := &meta{
		Record: workloadmeta.NewRecord(ctx, ID, kindVM, opts.Name, utils.LastEnvValue(opts.Env, podEnvKey), nodename, opts.Labels),
		User:   opts.User,
		Log:    logMeta{ConsoleSocket: vm.console(cocoon.RunDir)},
	}
	m.Networks = vm.networks()
	m.Cgroup = scopePath(cocoon.CgroupParent, vm.ID)
	m.Iface = vm.tap()
	return m
}

func parseInspect(out string) (*meta, *vmRecord, error) {
	return decodePair[meta, vmRecord](out)
}

type logMeta struct {
	ConsoleSocket string `json:"console_socket"`
}
