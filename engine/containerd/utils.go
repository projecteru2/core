package containerd

import (
	"github.com/distribution/reference"
)

func normalizeRef(ref string) string {
	named, err := reference.ParseDockerRef(ref)
	if err != nil {
		return ref
	}
	return named.String()
}
