package utils

import (
	"strconv"
	"strings"

	"github.com/docker/go-units"
)

// ParseRAMInHuman returns int value in bytes of a human readable string
// e.g. 100KB -> 102400
func ParseRAMInHuman(ram string) (int64, error) {
	if ram == "" {
		return 0, nil
	}
	ramInBytes, err := strconv.ParseInt(ram, 10, 64)
	if err == nil {
		return ramInBytes, nil
	}

	flag := int64(1)
	if strings.HasPrefix(ram, "-") {
		flag = int64(-1)
		ram = strings.TrimLeft(ram, "-")
	}
	ramInBytes, err = units.RAMInBytes(ram)
	if err != nil {
		return 0, err
	}
	return ramInBytes * flag, nil
}
