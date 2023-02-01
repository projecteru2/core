package engine

import (
	resourcetypes "github.com/projecteru2/core/resource/types"
)

// MakeVirtualizationResource .
// TODO 可以考虑进一步简化，每个 engine 自行处理
func MakeVirtualizationResource[T any](engineParams resourcetypes.Resources, dst T, f func(resourcetypes.Resources, T) error) error {
	return f(engineParams, dst)
}
