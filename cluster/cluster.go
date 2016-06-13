package cluster

import (
	"gitlab.ricebook.net/platform/core/store"
	"gitlab.ricebook.net/platform/core/types"
)

type Calcium struct {
	store  *store.Store
	config *types.Config
}

func NewCalcum(config *types.Config) (*Calcium, error) {
	store, err := store.NewStore(config)
	if err != nil {
		return nil, err
	}

	return &Calcium{store: store, config: config}, nil
}
