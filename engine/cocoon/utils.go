package cocoon

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/projecteru2/core/engine/workloadmeta"
)

const (
	scopePrefix    = "vm-"
	scopeSuffix    = ".scope"
	snapshotPrefix = "eru-"
	metaSuffix     = ".json"
)

func durablePath(root, ID string) string {
	return filepath.Join(root, ID+metaSuffix)
}

func snapshotName(ID string) string {
	return snapshotPrefix + ID
}

// scopePath is where cocoon puts the VMM's cgroup scope, keyed by cocoon's own id.
func scopePath(parent, vmID string) string {
	return filepath.Join(workloadmeta.CgroupRoot, parent, scopePrefix+vmID+scopeSuffix)
}

func decodePair[A, B any](out string) (*A, *B, error) {
	decoder := json.NewDecoder(strings.NewReader(out))
	first, second := new(A), new(B)
	if err := decoder.Decode(first); err != nil {
		return nil, nil, err
	}
	if err := decoder.Decode(second); err != nil {
		return nil, nil, err
	}
	return first, second, nil
}
