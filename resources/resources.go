package resources

import (
	"github.com/projecteru2/core/resources/cpumem"
	"github.com/projecteru2/core/resources/storage"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/resources/volume"
	"github.com/projecteru2/core/types"
)

var registeredFactories = []func(types.RawResourceOptions) (resourcetypes.ResourceRequirement, error){
	cpumem.NewResourceRequirement,
	volume.NewResourceRequirement,
	storage.NewResourceRequirement,
}

func NewResourceRequirements(opts types.RawResourceOptions) (rrs resourcetypes.ResourceRequirements, err error) {
	for idx, factory := range registeredFactories {
		if rrs[idx], err = factory(opts); err != nil {
			return
		}
	}
	return
}
