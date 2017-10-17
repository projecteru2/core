package calcium

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/types"
)

func TestRunAndWait(t *testing.T) {
	initMockConfig()
	opts := types.DeployOptions{
		Entrypoint: &types.Entrypoint{},
		OpenStdin:  true,
		Count:      10,
	}
	_, err = mockc.RunAndWait(context.TODO(), &opts, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Count must be 1 if OpenStdin is true")
}
