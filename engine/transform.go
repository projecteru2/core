package engine

import (
	"encoding/json"

	"github.com/sirupsen/logrus"

	"github.com/projecteru2/core/engine/types"
)

func MakeVirtualizationResource(engineArgs map[string]interface{}) (types.VirtualizationResource, error) {
	var res types.VirtualizationResource
	body, err := json.Marshal(engineArgs)
	if err != nil {
		return res, err
	}
	if err = json.Unmarshal(body, &res); err != nil {
		logrus.Errorf("[MakeVirtualizationResource] failed to unmarshal from engine args %v, err: %v", string(body), err)
		return res, err
	}
	res.EngineArgs = engineArgs
	return res, nil
}
