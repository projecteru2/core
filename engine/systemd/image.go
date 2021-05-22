package systemd

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
	coresource "github.com/projecteru2/core/source"
	"github.com/projecteru2/core/types"
)

// BuildRefs builds images refs
func (e *Engine) BuildRefs(ctx context.Context, name string, tags []string) (refs []string) {
	return
}

// BuildContent builds image content
func (e *Engine) BuildContent(ctx context.Context, scm coresource.Source, opts *enginetypes.BuildContentOptions) (dir string, reader io.Reader, err error) {
	err = types.ErrEngineNotImplemented
	return
}
