package binary

import (
	"context"
	ppath "path"
	"path/filepath"

	coretypes "github.com/projecteru2/core/types"
)

// Plugin drives a resource plugin binary.
type Plugin struct {
	name   string
	path   string
	config coretypes.Config
}

func NewPlugin(_ context.Context, path string, config coretypes.Config) (*Plugin, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	plugin := &Plugin{name: ppath.Base(path), path: p, config: config}
	return plugin, nil
}

func (p Plugin) Name() string {
	return p.name
}
