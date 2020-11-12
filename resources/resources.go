package resources

import (
	"github.com/projecteru2/core/resources/cpumem"
	"github.com/projecteru2/core/resources/storage"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/resources/volume"
	"github.com/projecteru2/core/types"
)

var registeredFactories = []func(types.ResourceOptions) (resourcetypes.ResourceRequest, error){
	cpumem.MakeRequest,
	storage.MakeRequest,
	volume.MakeRequest,
}

// MakeRequests .
func MakeRequests(opts types.ResourceOptions) (resourceRequests resourcetypes.ResourceRequests, err error) {
	for idx, factory := range registeredFactories {
		if resourceRequests[idx], err = factory(opts); err != nil {
			return
		}
	}
	return
}
