package calcium

import (
	"github.com/docker/go-units"
)

const (
	restartAlways = "always"
	minMemory     = units.MiB * 4
	minStorage    = units.GiB * 50
	root          = "root"
)
