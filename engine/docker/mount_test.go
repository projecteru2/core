package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
)

func TestMakeMountPathsExpandsEnv(t *testing.T) {
	opts := &enginetypes.VirtualizationCreateOptions{
		Env: []string{"BARE_ENTRY", "DATA=/mnt/data", "QUERY=a=b"},
	}
	resourceOpts := &engine.VirtualizationResource{
		Volumes: []string{"/host:${DATA}/app:rw", "/host2:${QUERY}:rw", "/host3:${MISSING}/x:rw"},
	}

	binds, volumes := makeMountPaths(t.Context(), opts, resourceOpts)
	assert.Equal(t, []string{"/host:/mnt/data/app:rw", "/host2:a=b:rw", "/host3:/x:rw"}, binds)
	assert.Contains(t, volumes, "/mnt/data/app")
}
