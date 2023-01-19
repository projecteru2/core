package engine

import (
	"github.com/mitchellh/mapstructure"
	"github.com/projecteru2/core/engine/types"
)

// MakeVirtualizationResource .
// TODO Werid, should revise
func MakeVirtualizationResource(engineParams interface{}) (types.VirtualizationResource, error) {
	var res types.VirtualizationResource
	if err := mapstructure.Decode(engineParams, &res); err != nil {
		return res, err
	}
	res.EngineParams = engineParams
	return res, nil
}
