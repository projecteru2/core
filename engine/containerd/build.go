package containerd

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
		return []string{e.config.Docker.ImageTag(opts.Name, utils.DefaultVersion)}
	}
	refs := make([]string, 0, len(opts.Tags))
	for _, tag := range opts.Tags {
		refs = append(refs, e.config.Docker.ImageTag(opts.Name, tag))
	}
	return refs
}

func (e *Engine) BuildContent(context.Context, coresource.Source, *enginetypes.BuildContentOptions) (string, io.Reader, error) {
	return "", nil, coretypes.ErrEngineNotImplemented
}

func (e *Engine) ImageBuild(context.Context, io.Reader, []string, string) (io.ReadCloser, error) {
	return nil, coretypes.ErrEngineNotImplemented
}

func (e *Engine) ImageBuildCachePrune(context.Context, bool) (uint64, error) {
	return 0, nil
}
