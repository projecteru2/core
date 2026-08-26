package process

import (
	"context"

	"github.com/projecteru2/core/engine/workloadmeta"
)

type meta struct {
	workloadmeta.Record
	Log logMeta `json:"log"`

	RootDirectory string `json:"root_directory,omitempty"` // empty for a raw host process
	WorkingDir    string `json:"working_dir,omitempty"`
}

func newMeta(ctx context.Context, u *unit, nodename, host string) *meta {
	m := &meta{
		Record:        workloadmeta.NewRecord(ctx, u.ID, Type, u.Opts.Name, u.Podname, nodename, u.Opts.Labels),
		Log:           logMeta{JournalUnit: unitName(u.ID)},
		RootDirectory: u.Root,
		WorkingDir:    u.Working,
	}
	m.Networks = map[string]string{hostNetwork: host}
	m.Cgroup = cgroupPath(sliceName(u.Podname), unitName(u.ID))
	return m
}

type logMeta struct {
	JournalUnit string `json:"journal_unit"`
}
