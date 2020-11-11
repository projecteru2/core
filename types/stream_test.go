package types

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetReader(t *testing.T) {
	reader := strings.NewReader("aaa")
	rm, err := NewReaderManager(reader)
	assert.Nil(t, err)
	reader2, err := rm.GetReader()
	assert.Nil(t, err)
	bs, err := ioutil.ReadAll(reader2)
	assert.Equal(t, "aaa", string(bs))
}
