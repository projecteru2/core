package cobalt

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
	"github.com/projecteru2/core/resource/plugins/binary"
	"github.com/projecteru2/core/resource/plugins/cpumem"
	"github.com/projecteru2/core/store/etcdv3/embedded"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// Manager fans resource operations out to the loaded plugins.
type Manager struct {
	config  types.Config
	plugins []plugins.Plugin
}

func New(config types.Config) (*Manager, error) {
	m := &Manager{
		config:  config,
		plugins: []plugins.Plugin{},
	}

	return m, nil
}

func (m *Manager) LoadPlugins(ctx context.Context, embeddedETCD *embedded.Cluster) error {
	logger := log.WithFunc("resource.cobalt.LoadPlugins")
	cm, err := cpumem.NewPlugin(ctx, m.config, embeddedETCD)
	if err != nil {
		return err
	}
	m.AddPlugins(cm)

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

// AddPlugins adds a plugin (for test and debug)
func (m *Manager) AddPlugins(ps ...plugins.Plugin) {
	m.plugins = append(m.plugins, ps...)
}
