package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func NormalVolumeBindingTestcases(t *testing.T) (testcases []*VolumeBinding) {
	vb, err := NewVolumeBinding("/src:/dst:rwm:1000")
	assert.Nil(t, err)
	assert.Equal(t, vb, &VolumeBinding{"/src", "/dst", "rwm", int64(1000)})
	assert.False(t, vb.RequireSchedule())
	assert.False(t, vb.RequireMonopoly())
	testcases = append(testcases, vb)

	vb, err = NewVolumeBinding("/src:/dst:rwm")
	assert.Nil(t, err)
	assert.Equal(t, vb, &VolumeBinding{"/src", "/dst", "rwm", int64(0)})
	assert.False(t, vb.RequireSchedule())
	assert.False(t, vb.RequireMonopoly())
	testcases = append(testcases, vb)

	vb, err = NewVolumeBinding("/src:/dst")
	assert.Nil(t, err)
	assert.Equal(t, vb, &VolumeBinding{"/src", "/dst", "", int64(0)})
	assert.False(t, vb.RequireSchedule())
	assert.False(t, vb.RequireMonopoly())
	testcases = append(testcases, vb)

	return
}

func AutoVolumeBindingTestcases(t *testing.T) (testcases []*VolumeBinding) {
	vb, err := NewVolumeBinding("AUTO:/data:rw:1")
	assert.Nil(t, err)
	assert.True(t, vb.RequireSchedule())
	assert.False(t, vb.RequireMonopoly())
	testcases = append(testcases, vb)

	vb, err = NewVolumeBinding("AUTO:/dir:rwm:1")
	assert.Nil(t, err)
	assert.True(t, vb.RequireSchedule())
	assert.True(t, vb.RequireMonopoly())
	testcases = append(testcases, vb)

	return
}

func TestNewVolumeBinding(t *testing.T) {
	NormalVolumeBindingTestcases(t)
	AutoVolumeBindingTestcases(t)

	_, err := NewVolumeBinding("/src:/dst:rw:1G")
	assert.Error(t, err, "invalid syntax")

	_, err = NewVolumeBinding("/src:/dst:rwm:1:asdf")
	assert.Error(t, err, "invalid volume")

	_, err = NewVolumeBinding("/src:/data:rw:-1")
	assert.Nil(t, err)

	_, err = NewVolumeBinding("AUTO:/data:rw")
	assert.Error(t, err, "size must be provided")
}
