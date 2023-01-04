package cobalt

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource3/plugins"
	"github.com/projecteru2/core/resource3/plugins/binary"
	"github.com/projecteru2/core/resource3/plugins/cpumem"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// Manager manager plugins
type Manager struct {
	config  types.Config
	plugins []plugins.Plugin
}

// New creates a plugin manager
func New(config types.Config) (*Manager, error) {
	m := &Manager{
		config:  config,
		plugins: []plugins.Plugin{},
	}

	return m, nil
}

// LoadPlugins .
func (m *Manager) LoadPlugins(ctx context.Context) error {
	logger := log.WithFunc("resource.cobalt.LoadPlugins")
	// Load internal
	cm, err := cpumem.NewPlugin(ctx, m.config)
	if err != nil {
		return err
	}
	m.AddPlugins(cm)

	//	st, err := storage.NewPlugin(ctx, m.config)
	//	if err != nil {
	//		return err
	//	}
	//	m.AddPlugins(st)

	if m.config.ResourcePlugin.Dir == "" {
		return nil
	}

	pluginFiles, err := utils.ListAllExecutableFiles(m.config.ResourcePlugin.Dir)
	if err != nil {
		logger.Errorf(ctx, err, "failed to list all executable files dir: %+v", m.config.ResourcePlugin.Dir)
		return err
	}

	cache := map[string]struct{}{}
	for _, plugin := range m.plugins {
		cache[plugin.Name()] = struct{}{}
	}

	for _, file := range pluginFiles {
		logger.Infof(ctx, "load binary plugin: %+v", file)
		b, err := binary.NewPlugin(ctx, file, m.config)
		if err != nil {
			return err
		}
		if _, ok := cache[b.Name()]; ok {
			continue
		}
		m.AddPlugins(b)
	}

	return nil
}

// AddPlugins adds a plugin (for test and debug)
func (m *Manager) AddPlugins(plugins ...plugins.Plugin) {
	m.plugins = append(m.plugins, plugins...)
}

// GetPlugins is used for mock
func (m *Manager) GetPlugins() []plugins.Plugin {
	return m.plugins
}
