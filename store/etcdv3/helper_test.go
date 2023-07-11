package etcdv3

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseStatusKey(t *testing.T) {
	key := "/deploy/appname/entry/node/id"
	p1, p2, p3, p4 := parseStatusKey(key)
	assert.Equal(t, p1, "appname")
	assert.Equal(t, p2, "entry")
	assert.Equal(t, p3, "node")
	assert.Equal(t, p4, "id")
}
