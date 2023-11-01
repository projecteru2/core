package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListAllExecutableFiles(t *testing.T) {
	dir, err := os.MkdirTemp(os.TempDir(), "test*")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	file, err := os.CreateTemp(dir, "abc")
	assert.NoError(t, err)

	subdir, err := os.MkdirTemp(dir, "def")
	assert.NoError(t, err)

	assert.NotNil(t, file)
	assert.NotNil(t, subdir)

	fInfo, err := os.Stat(file.Name())
	assert.NoError(t, err)
	assert.NotNil(t, fInfo)

	assert.False(t, isExecutable(fInfo.Mode().Perm()))

	os.Chmod(file.Name(), 0777)
	fInfo, _ = os.Stat(file.Name())
	assert.True(t, isExecutable(fInfo.Mode().Perm()))

	fs, err := ListAllExecutableFiles(dir)
	assert.NoError(t, err)
	assert.Len(t, fs, 1)
}

func TestListAllShareLibFiles(t *testing.T) {
	dir, err := os.MkdirTemp(os.TempDir(), "test*")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	_, err = os.Create(filepath.Join(dir, "abc"))
	assert.NoError(t, err)

	_, err = os.Create(filepath.Join(dir, "bcd.so"))
	assert.NoError(t, err)

	subdir, err := os.MkdirTemp(dir, "def")
	assert.NoError(t, err)

	_, err = os.Create(filepath.Join(subdir, "abc1"))
	assert.NoError(t, err)

	_, err = os.Create(filepath.Join(subdir, "bcd1.so"))
	assert.NoError(t, err)

	fs, err := ListAllShareLibFiles(dir)
	assert.NoError(t, err)
	assert.Len(t, fs, 1)
	assert.Equal(t, filepath.Join(dir, "bcd.so"), fs[0])
}
