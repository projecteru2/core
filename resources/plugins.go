package resources

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// PluginsManager manages plugins
type PluginsManager struct {
	config  types.Config
	plugins []Plugin
}

// NewPluginsManager creates a plugin manager
func NewPluginsManager(config types.Config) (*PluginsManager, error) {
	pm := &PluginsManager{
		config:  config,
		plugins: []Plugin{},
	}

	return pm, nil
}

// LoadPlugins .
func (pm *PluginsManager) LoadPlugins(ctx context.Context) error {
	if pm.config.ResourcePlugin.Dir == "" {
		return nil
	}

	pluginFiles, err := utils.ListAllExecutableFiles(pm.config.ResourcePlugin.Dir)
	if err != nil {
		log.Errorf(ctx, err, "[LoadPlugins] failed to list all executable files dir: %+v", pm.config.ResourcePlugin.Dir)
		return err
	}

	cache := map[string]struct{}{}
	for _, plugin := range pm.plugins {
		cache[plugin.Name()] = struct{}{}
	}

	for _, file := range pluginFiles {
		log.Infof(ctx, "[LoadPlugins] load binary plugin: %+v", file)
		plugin := &BinaryPlugin{path: file, config: pm.config}
		if _, ok := cache[plugin.Name()]; ok {
			continue
		}
		pm.plugins = append(pm.plugins, plugin)
	}
	return nil
}

// AddPlugins adds a plugin (for test and debug)
func (pm *PluginsManager) AddPlugins(plugins ...Plugin) {
	pm.plugins = append(pm.plugins, plugins...)
}

// GetPlugins is used for mock
func (pm *PluginsManager) GetPlugins() []Plugin {
	return pm.plugins
}

func callPlugins[T any](ctx context.Context, plugins []Plugin, f func(Plugin) (T, error)) (map[Plugin]T, error) {
	// TODO 并行化，意义不大
	var combinedErr error
	results := map[Plugin]T{}

	for _, plugin := range plugins {
		result, err := f(plugin)
		if err != nil {
			log.Errorf(ctx, err, "[callPlugins] failed to call plugin %+v", plugin.Name())
			combinedErr = errors.CombineErrors(combinedErr, errors.Wrap(err, plugin.Name()))
			continue
		}
		results[plugin] = result
	}

	return results, combinedErr
}
