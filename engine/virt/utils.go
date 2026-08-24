package virt

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"

	resourcetypes "github.com/projecteru2/core/resource/types"
)

const sep = "@"

func splitUserImage(combined string) (user, imageName string, err error) {
	inputErr := errors.Newf("input: \"%s\" not valid", combined)
	if len(combined) < 1 {
		return "", "", inputErr
	}

	un := strings.Split(combined, sep)
	switch len(un) {
	case 1:
		return "", combined, nil
	case 2:
		if len(un[0]) < 1 || len(un[1]) < 1 {
			return "", "", inputErr
		}
		return un[0], un[1], nil
	default:
		return "", "", inputErr
	}
}

func combineUserImage(user, imageName string) string {
	if len(imageName) < 1 {
		return ""
	}
	if len(user) < 1 {
		return imageName
	}
	return fmt.Sprintf("%s%s%s", user, sep, imageName)
}

func convertEngineParamsToResources(engineParams resourcetypes.Resources) map[string][]byte {
	r := map[string][]byte{}
	for p, res := range engineParams {
		b, _ := json.Marshal(res)
		r[p] = b
	}
	return r
}
