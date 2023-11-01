package goplugin

import (
	"context"
	"fmt"
	"path/filepath"

	goplugin "plugin"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/resource/plugins"
	coretypes "github.com/projecteru2/core/types"
)

// NewPlugin .
func NewPlugin(ctx context.Context, path string, config coretypes.Config) (plugins.Plugin, error) {
	pFname, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	gp, err := goplugin.Open(pFname)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open plugin %s", pFname)
	}
	sym, err := gp.Lookup("NewPlugin")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to lookup NewPlugin %s", pFname)
	}
	fn, ok := sym.(func(context.Context, coretypes.Config) (plugins.Plugin, error))
	if !ok {
		return nil, fmt.Errorf("NewPlugin is not a function")
	}
	return fn(ctx, config)
}
