package virt

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	coretypes "github.com/projecteru2/core/types"
)

const sep = "@"

func (v *Virt) parseVolumes(volumes []string) ([]string, error) {
	vols := []string{}

	// format `/source:/dir0:rw:1G:1000:1000:10M:10M`
	for _, bind := range volumes {
		parts := strings.Split(bind, ":")
		if len(parts) != 4 && len(parts) != 8 {
			return nil, coretypes.NewDetailedErr(coretypes.ErrInvalidBind, bind)
		}

		src := parts[0]
		dest := filepath.Join("/", parts[1])

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
			ioConstraints = fmt.Sprintf(":%s", strings.Join(parts[4:], ":"))
		}

		vols = append(vols, fmt.Sprintf("%s:%d%s", mnt, capacity, ioConstraints))
	}

	return vols, nil
}

func splitUserImage(combined string) (user, imageName string, err error) {
	inputErr := fmt.Errorf("input: \"%s\" not valid", combined)
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
