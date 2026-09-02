package cobalt

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
	"github.com/projecteru2/core/resource/plugins/binary"
	"github.com/projecteru2/core/resource/plugins/cpumem"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// Manager fans resource operations out to the loaded plugins.
type Manager struct {
	config  types.Config
	plugins []plugins.Plugin
}

func New(config types.Config) *Manager {
	return &Manager{config: config}
}

func (m *Manager) LoadPlugins(ctx context.Context, store store.Store) error {
	logger := log.WithFunc("resource.cobalt.LoadPlugins")
	m.AddPlugins(cpumem.NewPlugin(m.config, store))

	if m.config.ResourcePlugin.Dir == "" {
		return nil
	}

	cache := map[string]struct{}{}
	for _, plugin := range m.plugins {
		cache[plugin.Name()] = struct{}{}
	}

	pluginFiles, err := utils.ListAllExecutableFiles(m.config.ResourcePlugin.Dir)
	if err != nil {
		logger.Errorf(ctx, err, "failed to list executable files in dir %+v", m.config.ResourcePlugin.Dir)
		return err
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
		cache[b.Name()] = struct{}{}
		m.AddPlugins(b)
	}
	return nil
}

func (m *Manager) AddPlugins(ps ...plugins.Plugin) {
	m.plugins = append(m.plugins, ps...)
}
