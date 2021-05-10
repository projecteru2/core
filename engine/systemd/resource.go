package systemd

import (
	"context"

	"github.com/projecteru2/core/types"
)

// ResourceValidate validates resources
func (s *systemdEngine) ResourceValidate(ctx context.Context, cpu float64, cpumap map[string]int64, memory, storage int64) (err error) {
	return types.ErrEngineNotImplemented
}
