package calcium

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBackup(t *testing.T) {
	initMockConfig()
	srcPath := "/tmp"
	_, err = mockc.Backup(mockID, srcPath)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "BackupDir not set")

	config.BackupDir = "/tmp"
	mockc, err = New(config)
	if err != nil {
		t.Error(err)
		return
	}
	mockc.SetStore(mockStore)
	ch, err := mockc.Backup(mockID, srcPath)
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, ch.Status, "ok")
}
