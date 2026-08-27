package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	resourcetypes "github.com/projecteru2/core/resource/types"
)

func TestWorkloadResourceRequestParseMemoryForms(t *testing.T) {
	req := &WorkloadResourceRequest{}
	if err := req.Parse(resourcetypes.RawParams{"memory-request": "1G", "memory-limit": float64(2147483648)}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.MemRequest != 1073741824 || req.MemLimit != 2147483648 {
		t.Errorf("got %d/%d, want the human string and the plain byte count both read", req.MemRequest, req.MemLimit)
	}
	if err := req.Parse(resourcetypes.RawParams{"memory-request": "1Q"}); err == nil {
		t.Error("got nil, want a junk size reported instead of a silent zero")
	}
}

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
