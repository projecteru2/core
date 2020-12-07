package types

import (
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestGetCapacity(t *testing.T) {
	nodesInfo := []ScheduleInfo{
		{NodeMeta: types.NodeMeta{Name: "1"}, Capacity: 1},
		{NodeMeta: types.NodeMeta{Name: "2"}, Capacity: 1},
	}
	r := GetCapacity(nodesInfo)
	assert.Equal(t, r["1"], 1)
	assert.Equal(t, r["2"], 1)
}
