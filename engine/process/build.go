package process

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
	coresource "github.com/projecteru2/core/source"
	coretypes "github.com/projecteru2/core/types"
)

func (e *Engine) BuildRefs(_ context.Context, opts *enginetypes.BuildRefOptions) []string {
	return e.config.Registry.BuildRefs(opts.Name, opts.Tags)
}

func (e *Engine) BuildContent(context.Context, coresource.Source, *enginetypes.BuildContentOptions) (string, io.Reader, error) {
	return "", nil, coretypes.ErrEngineNotImplemented
}
