package systemd

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
	coresource "github.com/projecteru2/core/source"
	"github.com/projecteru2/core/types"
)

// BuildRefs builds images refs
func (s *systemdEngine) BuildRefs(ctx context.Context, name string, tags []string) (refs []string) {
	return
}

// BuildContent builds image content
func (s *systemdEngine) BuildContent(ctx context.Context, scm coresource.Source, opts *enginetypes.BuildContentOptions) (dir string, reader io.Reader, err error) {
	err = types.ErrEngineNotImplemented
	return
}
