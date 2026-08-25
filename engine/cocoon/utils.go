package cocoon

import (
	"encoding/json"
	"path/filepath"
	"strings"

	enginetypes "github.com/projecteru2/core/engine/types"
)

const (
	cgroupRoot     = "/sys/fs/cgroup"
	scopePrefix    = "vm-"
	scopeSuffix    = ".scope"
	snapshotPrefix = "eru-"
	metaSuffix     = ".json"
)

func metaPath(ID string) string {
	return filepath.Join(metaDir, ID+metaSuffix)
}

func durablePath(root, ID string) string {
	return filepath.Join(root, ID+metaSuffix)
}

func snapshotName(ID string) string {
	return snapshotPrefix + ID
}

// scopePath is where cocoon puts the VMM's cgroup scope, keyed by cocoon's own id.
func scopePath(parent, vmID string) string {
	return filepath.Join(cgroupRoot, parent, scopePrefix+vmID+scopeSuffix)
}

func lastEnvValue(env []string, key string) string {
	last := ""
	for _, entry := range env {
		if name, value, ok := strings.Cut(entry, "="); ok && name == key {
			last = value
		}
	}
	return last
}

func isURL(ref string) bool {
	return strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://")
}

// imageDigest renders the digest form core compares; a cloud image url stands for itself.
func imageDigest(image, digest string) string {
	if isURL(image) {
		return image
	}
	name, _ := enginetypes.SplitRef(image)
	return name + "@" + digest
}

func parseDescriptor(out string) (string, error) {
	descriptor := struct {
		Digest string `json:"digest"`
	}{}
	if err := json.Unmarshal([]byte(out), &descriptor); err != nil {
		return "", err
	}
	return descriptor.Digest, nil
}
