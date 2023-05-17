package virt

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
	resourcetypes "github.com/projecteru2/core/resource/types"
	coretypes "github.com/projecteru2/core/types"
	virttypes "github.com/projecteru2/libyavirt/types"
)

const sep = "@"

func (v *Virt) parseVolumes(volumes []string) ([]virttypes.Volume, error) {
	vols := make([]virttypes.Volume, len(volumes))
	// format `/source:/dir0:rw:1024:1000:1000:10M:10M`
	for i, bind := range volumes {
		parts := strings.Split(bind, ":")
		if len(parts) != 4 && len(parts) != 8 {
			return nil, errors.Wrapf(coretypes.ErrInvalidVolumeBind, "bind: %s", bind)
		}

		src := parts[0]
		dest := parts[1]
		if !strings.HasPrefix(dest, "/") {
			dest = filepath.Join("/", parts[1])
		}

		mnt := dest
		// the src part has been translated to real host directory by eru-sched or kept it to empty.
		if len(src) > 0 {
			mnt = fmt.Sprintf("%s:%s", src, dest)
		}

		capacity, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return nil, err
		}

		ioConstraints := ""
		if len(parts) > 4 {
			ioConstraints = strings.Join(parts[4:], ":")
		}

		volume := virttypes.Volume{
			Mount:    mnt,
			Capacity: capacity,
			IO:       ioConstraints,
		}
		vols[i] = volume
	}

	return vols, nil
}

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
		b, _ := json.Marshal(res) // nolint
		r[p] = b
	}
	return r
}
