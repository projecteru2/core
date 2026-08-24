package virt

import (
	"encoding/json"
	"strings"

	"github.com/cockroachdb/errors"

	resourcetypes "github.com/projecteru2/core/resource/types"
)

const sep = "@"

func splitUserImage(combined string) (user, imageName string, err error) {
	un := strings.Split(combined, sep)
	switch {
	case combined == "":
	case len(un) == 1:
		return "", combined, nil
	case len(un) == 2 && un[0] != "" && un[1] != "":
		return un[0], un[1], nil
	}
	return "", "", errors.Newf("input: %q not valid", combined)
}

func combineUserImage(user, imageName string) string {
	if len(imageName) < 1 {
		return ""
	}
	if len(user) < 1 {
		return imageName
	}
	return user + sep + imageName
}

func convertEngineParamsToResources(engineParams resourcetypes.Resources) map[string][]byte {
	r := map[string][]byte{}
	for p, res := range engineParams {
		b, _ := json.Marshal(res)
		r[p] = b
	}
	return r
}
