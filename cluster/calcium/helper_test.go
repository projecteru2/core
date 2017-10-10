package calcium

import (
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestMakeMountPaths(t *testing.T) {
	opts := &types.DeployOptions{
		Name:    "foo",
		Env:     []string{"A=1"},
		Volumes: []string{"/foo-data:/foo-data"},
	}
	binds, volumes := makeMountPaths(opts)
	assert.Equal(t, binds, []string{"/foo-data:/foo-data:rw", "/proc/sys:/writable-proc/sys:rw", "/sys/kernel/mm/transparent_hugepage:/writable-sys/kernel/mm/transparent_hugepage:rw"})
	assert.Equal(t, volumes, map[string]struct{}{"/foo-data": struct{}{}, "/writable-proc/sys": struct{}{}, "/writable-sys/kernel/mm/transparent_hugepage": struct{}{}})
}
