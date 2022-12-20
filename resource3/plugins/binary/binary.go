package binary

import (
	"context"

	ppath "path"

	coretypes "github.com/projecteru2/core/types"
)

// Plugin
type Plugin struct {
	name   string
	path   string
	config coretypes.Config
}

func NewPlugin(ctx context.Context, path string, config coretypes.Config) (*Plugin, error) {
	plugin := &Plugin{name: ppath.Base(path), path: path, config: config}
	return plugin, nil
}

func (p Plugin) Name() string {
	return p.name
}
