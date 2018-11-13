package etcdv3

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/stretchr/testify/assert"
)

func TestDumpCerts(t *testing.T) {
	buff := bytes.NewBuffer([]byte(""))
	rc := ioutil.NopCloser(buff)
	fname1, err := utils.TempFile(rc)
	assert.NoError(t, err)
	fname2, err := utils.TempFile(rc)
	assert.NoError(t, err)
	fname3, err := utils.TempFile(rc)
	assert.NoError(t, err)

	f1, err := os.Create(fname1)
	assert.NoError(t, err)
	f2, err := os.Create(fname2)
	assert.NoError(t, err)
	f3, err := os.Create(fname3)
	assert.NoError(t, err)

	err = dumpFromString(f1, f2, f3, "a", "b", "c")
	assert.NoError(t, err)

	d1, err := ioutil.ReadFile(fname1)
	assert.NoError(t, err)
	assert.Equal(t, string(d1), "a")
	d2, err := ioutil.ReadFile(fname2)
	assert.NoError(t, err)
	assert.Equal(t, string(d2), "b")
	d3, err := ioutil.ReadFile(fname3)
	assert.NoError(t, err)
	assert.Equal(t, string(d3), "c")

	os.Remove(fname1)
	os.Remove(fname2)
	os.Remove(fname3)
}

func TestParseStatusKey(t *testing.T) {
	key := "/deploy/appname/entry/node/id"
	p1, p2, p3, p4 := parseStatusKey(key)
	assert.Equal(t, p1, "appname")
	assert.Equal(t, p2, "entry")
	assert.Equal(t, p3, "node")
	assert.Equal(t, p4, "id")
}

func TestSetCount(t *testing.T) {
	nodesCount := map[string]int{
		"n1": 1,
		"n2": 2,
	}
	nodesInfo := []types.NodeInfo{
		types.NodeInfo{Name: "n1"},
		types.NodeInfo{Name: "n2"},
	}
	nodesInfo = setCount(nodesCount, nodesInfo)
	assert.Equal(t, nodesInfo[0].Count, 1)
	assert.Equal(t, nodesInfo[1].Count, 2)
}
