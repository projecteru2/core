package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkloadResourceSubDeltasEveryField(t *testing.T) {
	w := &WorkloadResource{
		CPURequest:    2,
		CPULimit:      4,
		MemoryRequest: 200,
		MemoryLimit:   400,
	}
	w.Sub(&WorkloadResource{
		CPURequest:    1,
		CPULimit:      1,
		MemoryRequest: 50,
		MemoryLimit:   100,
	})

	assert.Equal(t, float64(1), w.CPURequest)
	assert.Equal(t, float64(3), w.CPULimit)
	assert.Equal(t, int64(150), w.MemoryRequest)
	assert.Equal(t, int64(300), w.MemoryLimit)
}
