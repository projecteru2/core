package process

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
	coresource "github.com/projecteru2/core/source"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (e *Engine) BuildRefs(_ context.Context, opts *enginetypes.BuildRefOptions) []string {
	if len(opts.Tags) == 0 {
		return []string{e.config.Registry.ImageTag(opts.Name, utils.DefaultVersion)}
	}
	refs := make([]string, 0, len(opts.Tags))
	for _, tag := range opts.Tags {
		refs = append(refs, e.config.Registry.ImageTag(opts.Name, tag))
	}
	return refs
}

func (e *Engine) BuildContent(context.Context, coresource.Source, *enginetypes.BuildContentOptions) (string, io.Reader, error) {
	return "", nil, coretypes.ErrEngineNotImplemented
}
