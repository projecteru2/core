package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRawArgs(t *testing.T) {
	assert := assert.New(t)

	r1, err := loadRawArgs([]byte(``))
	assert.NoError(err)
	assert.NotEqual(r1.StorageOpt, nil)
	assert.Equal(len(r1.StorageOpt), 0)
	assert.NotEqual(r1.CapAdd, nil)
	assert.Equal(len(r1.CapAdd), 0)
	assert.NotEqual(r1.CapDrop, nil)
	assert.Equal(len(r1.CapDrop), 0)
	assert.NotEqual(r1.Ulimits, nil)
	assert.Equal(len(r1.Ulimits), 0)

	r2, err := loadRawArgs([]byte(`{"storage_opt": null, "cap_add": null, "cap_drop": null, "ulimits": null}`))
	assert.NoError(err)
	assert.NotEqual(r2.StorageOpt, nil)
	assert.Equal(len(r2.StorageOpt), 0)
	assert.NotEqual(r2.CapAdd, nil)
	assert.Equal(len(r2.CapAdd), 0)
	assert.NotEqual(r2.CapDrop, nil)
	assert.Equal(len(r2.CapDrop), 0)
	assert.NotEqual(r2.Ulimits, nil)
	assert.Equal(len(r2.Ulimits), 0)

	_, err = loadRawArgs([]byte(`{"storage_opt": null, "cap_add": null, "cap_drop": null, "ulimits"}`))
	assert.Error(err)
}
