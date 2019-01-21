package docker

import (
	"bytes"
	"io/ioutil"
	"testing"

	coreutils "github.com/projecteru2/core/utils"
	"github.com/stretchr/testify/assert"
)

func TestCreateTarStream(t *testing.T) {
	buff := bytes.NewBufferString("test")
	rc := ioutil.NopCloser(buff)
	fname, err := coreutils.TempFile(rc)
	assert.NoError(t, err)
	_, err = createTarStream(fname)
	assert.NoError(t, err)
}
