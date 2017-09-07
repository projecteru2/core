package calcium

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/types"
)

func TestRunAndWait(t *testing.T) {
	initMockConfig()
	specs := types.Specs{
		Entrypoints: map[string]types.Entrypoint{
			"entry": types.Entrypoint{},
		},
	}
	opts := types.DeployOptions{
		Entrypoint: "entry",
		OpenStdin:  true,
		Count:      10,
	}
	_, err = mockc.RunAndWait(specs, &opts, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Count must be 1 if OpenStdin is true")
}
