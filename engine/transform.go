package engine

import (
	"github.com/mitchellh/mapstructure"
	"github.com/projecteru2/core/engine/types"
)

// MakeVirtualizationResource .
// TODO Werid, should revise
func MakeVirtualizationResource(engineParams interface{}) (types.VirtualizationResource, error) {
	// trans to Resources first because cycle import not allow here
	t := map[string]map[string]interface{}{}
	var res types.VirtualizationResource
	if err := mapstructure.Decode(engineParams, &t); err != nil {
		return res, err
	}
	r := map[string]interface{}{}
	for _, p := range t {
		for k, v := range p {
			r[k] = v
		}
	}
	if err := mapstructure.Decode(r, &res); err != nil {
		return res, err
	}
	res.EngineParams = engineParams
	return res, nil
}
