package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTriOption(t *testing.T) {
	assert.False(t, ParseTriOption(TriFalse, true))
	assert.True(t, ParseTriOption(TriTrue, false))
	assert.False(t, ParseTriOption(TriKeep, false))
	assert.True(t, ParseTriOption(TriKeep, true))
}

func TestSetNodeOptions(t *testing.T) {
	o := &SetNodeOptions{
		DeltaVolume:  VolumeMap{"/data": 1, "/data2": 2},
		DeltaStorage: -10,
	}
	o.Normalize(nil)
	assert.EqualValues(t, -7, o.DeltaStorage)

	node := &Node{
		InitVolume: VolumeMap{"/data0": 100, "/data1": 3},
	}
	o = &SetNodeOptions{
		DeltaVolume:  VolumeMap{"/data0": 0, "/data1": 10},
		DeltaStorage: 10,
	}
	o.Normalize(node)
	assert.EqualValues(t, 10-100+10, o.DeltaStorage)
}
