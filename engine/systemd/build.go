package systemd

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
	coresource "github.com/projecteru2/core/source"
	"github.com/projecteru2/core/types"
)

func (e *Engine) BuildRefs(context.Context, *enginetypes.BuildRefOptions) []string {
	return nil
}

func (e *Engine) BuildContent(context.Context, coresource.Source, *enginetypes.BuildContentOptions) (string, io.Reader, error) {
	return "", nil, types.ErrEngineNotImplemented
}
