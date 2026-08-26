package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListAllExecutableFiles(t *testing.T) {
	dir := t.TempDir()

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

	os.Chmod(file.Name(), 0o777)
	fInfo, _ = os.Stat(file.Name())
	assert.True(t, isExecutable(fInfo.Mode().Perm()))

	fs, err := ListAllExecutableFiles(dir)
	assert.NoError(t, err)
	assert.Len(t, fs, 1)
}

func TestListAllExecutableFilesFindsOwnerOnlyExecutables(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "plugin")
	assert.NoError(t, os.WriteFile(name, []byte("#!/bin/sh\n"), 0o700))

	fs, err := ListAllExecutableFiles(dir)
	assert.NoError(t, err)
	assert.Equal(t, []string{name}, fs)
}
