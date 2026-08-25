package cocoon

import (
	"path/filepath"
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
