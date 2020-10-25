package v2

import (
	"github.com/projecteru2/core/types"
)

// DispenseOptions .
type DispenseOptions struct {
	*types.Node
	ExistingInstances  []*types.Container
	Index              int
	HardVolumeBindings types.VolumeBindings
}
