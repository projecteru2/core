package containerd

import (
	"strings"

	"github.com/distribution/reference"
)

func normalizeRef(ref string) string {
	named, err := reference.ParseDockerRef(ref)
	if err != nil {
		return ref
	}
	return named.String()
}

// imageName drops an image reference's tag, ignoring a registry port.
func imageName(ref string) string {
	colon := strings.LastIndex(ref, ":")
	if colon < 0 || colon < strings.LastIndex(ref, "/") {
		return ref
	}
	return ref[:colon]
}
