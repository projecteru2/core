package virt

import (
	"path/filepath"
	"strconv"
	"strings"

	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

func (v *Virt) parseVolumes(opts *enginetypes.VirtualizationCreateOptions) (map[string]int64, error) {
	vols := map[string]int64{}

	for _, bind := range opts.Volumes {
		parts := strings.Split(bind, ":")
		if len(parts) != 4 {
			return nil, coretypes.NewDetailedErr(coretypes.ErrInvalidBind, bind)
		}

		mnt := filepath.Join("/", parts[1])
		
		cap, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return nil, err
		}

		vols[mnt] = cap
	}

	return vols, nil
}
