package calcium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.ricebook.net/platform/core/types"
)

func TestMakeMountPaths(t *testing.T) {
	config := types.Config{AppDir: "/home"}
	specs := types.Specs{Appname: "foo", Volumes: []string{"/foo-data:$APPDIR/foo-data"}}
	binds, volumes := makeMountPaths(specs, config)
	assert.Equal(t, binds, []string{"/foo-data:/home/foo/foo-data:rw", "/proc/sys:/writable-proc/sys:rw", "/sys/kernel/mm/transparent_hugepage:/writable-sys/kernel/mm/transparent_hugepage:rw"}, "binds should be the same")
	assert.Equal(t, volumes, map[string]struct{}{"/home/foo/foo-data": struct{}{}, "/writable-proc/sys": struct{}{}, "/writable-sys/kernel/mm/transparent_hugepage": struct{}{}})
}
