package resources

import (
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// ResourceApplication .
type ResourceApplication interface {
	Type() types.ResourceType
	Validate() error
	MakeScheduler() SchedulerV2
	Rate(types.Node) float64
}

// ResourcePlans .
type ResourcePlans interface {
	Type() types.ResourceType
	Capacity() map[string]int
	ApplyChangesOnNode(*types.Node, ...int)
	RollbackChangesOnNode(*types.Node, ...int)
	Dispense(DispenseOptions, *types.Resources) error
}

var registeredFactories []func(types.RawResourceOptions) (ResourceApplication, error)

func RegisterApplicationFactory(f func(types.RawResourceOptions) (ResourceApplication, error)) {
	registeredFactories = append(registeredFactories, f)
}

type ResourceApplications [ResourceQuantity]ResourceApplication

func NewResourceApplications(opts types.RawResourceOptions) (apps ResourceApplications, err error) {
	if len(registeredFactories) != ResourceQuantity {
		log.Panicf("wrong quantity of resource applications: expect %d", ResourceQuantity)
	}

	for idx, factory := range registeredFactories {
		app, err := factory(opts)
		if err != nil {
			return apps, err
		}
		apps[idx] = app
	}
	return
}

const ResourceQuantity = 3
