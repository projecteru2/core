package calcium

import (
	"os"
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
	assert.NoError(t, err)

	mockc.SetStore(mockStore)
	msg, err := mockc.Backup(mockID, srcPath)
	assert.NoError(t, err)

	assert.Equal(t, msg.Status, "ok")
	t.Log("Backup to ", msg.Path)

	os.RemoveAll(msg.Path)
}
