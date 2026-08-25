package containerd

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/distribution/reference"
)

const idBytes = 16

func newID() string {
	buf := make([]byte, idBytes)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

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
