package systemd

import (
	"context"

	"github.com/projecteru2/core/types"
)

// ResourceValidate validates resources
func (e *Engine) ResourceValidate(ctx context.Context, cpu float64, cpumap map[string]int64, memory, storage int64) (err error) {
	return types.ErrEngineNotImplemented
}
