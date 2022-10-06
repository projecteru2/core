package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMakeVirtualizationResource(t *testing.T) {
	args := map[string]interface{}{
		"cpu_map": map[string]int64{"1": 100},
		"cpu":     100.0,
		"memory":  10000,
	}
	res, err := MakeVirtualizationResource(args)
	assert.NotNil(t, res)
	assert.NoError(t, err)
	assert.Equal(t, res.Quota, 100.0)
}
