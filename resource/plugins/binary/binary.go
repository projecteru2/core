package binary

import (
	"context"
	ppath "path"
	"path/filepath"

	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
)

// Plugin drives a resource plugin binary.
type Plugin struct {
	name   string
	path   string
	config coretypes.Config
	verbs  []string
}

// NewPlugin asks the binary which verbs it implements, so a verb it lacks never spawns it.
func NewPlugin(ctx context.Context, path string, config coretypes.Config) (*Plugin, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	plugin := &Plugin{name: ppath.Base(path), path: p, config: config, verbs: []string{VerbsCommand}}
	if err := plugin.call(ctx, VerbsCommand, struct{}{}, &plugin.verbs); err != nil {
		return nil, err
	}
	log.WithFunc("resource.binary.NewPlugin").WithField("plugin", plugin.name).Infof(ctx, "verbs %v", plugin.verbs)
	return plugin, nil
}

func (p Plugin) Name() string {
	return p.name
}
