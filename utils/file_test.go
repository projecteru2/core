package utils

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListAllExecutableFiles(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "test*")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	file, err := ioutil.TempFile(dir, "abc")
	assert.NoError(t, err)

	subdir, err := ioutil.TempDir(dir, "def")
	assert.NoError(t, err)

	assert.NotNil(t, file)
	assert.NotNil(t, subdir)

	fInfo, err := os.Stat(file.Name())
	assert.NoError(t, err)
	assert.NotNil(t, fInfo)

	//file
	//isExecutable(file.Mode().Perm())
}
