package resources

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"github.com/hashicorp/go-multierror"
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
	if pm.config.ResourcePluginsDir == "" {
		return nil
	}

	pluginFiles, err := utils.ListAllExecutableFiles(pm.config.ResourcePluginsDir)
	if err != nil {
		log.Errorf(ctx, "[LoadPlugins] failed to list all executable files dir: %v, err: %v", pm.config.ResourcePluginsDir, err)
		return err
	}

	cache := map[string]struct{}{}
	for _, plugin := range pm.plugins {
		cache[plugin.Name()] = struct{}{}
	}

	for _, file := range pluginFiles {
		log.Infof(ctx, "[LoadPlugins] load binary plugin: %v", file)
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
	var combinedErr error
	results := map[Plugin]T{}

	for _, plugin := range plugins {
		result, err := f(plugin)
		if err != nil {
			log.Errorf(ctx, "[callPlugins] failed to call plugin %v, err: %v", plugin.Name(), err)
			combinedErr = multierror.Append(combinedErr, types.NewDetailedErr(err, plugin.Name()))
			continue
		}
		results[plugin] = result
	}

	return results, combinedErr
}
