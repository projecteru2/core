package binary

import (
	"context"
	"path/filepath"

	ppath "path"

	coretypes "github.com/projecteru2/core/types"
)

// Plugin
type Plugin struct {
	name   string
	path   string
	config coretypes.Config
}

// NewPlugin .
func NewPlugin(ctx context.Context, path string, config coretypes.Config) (*Plugin, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	plugin := &Plugin{name: ppath.Base(path), path: p, config: config}
	return plugin, nil
}

// Name .
func (p Plugin) Name() string {
	return p.name
}
