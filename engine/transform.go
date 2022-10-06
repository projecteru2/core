package engine

import (
	"github.com/mitchellh/mapstructure"
	"github.com/projecteru2/core/engine/types"
)

// MakeVirtualizationResource .
func MakeVirtualizationResource(engineArgs map[string]interface{}) (types.VirtualizationResource, error) {
	var res types.VirtualizationResource
	if err := mapstructure.Decode(engineArgs, &res); err != nil {
		return res, err
	}
	res.EngineArgs = engineArgs
	return res, nil
}
