package rpc

import (
	"strings"
	"testing"

	"os"

	"github.com/stretchr/testify/assert"
)

func TestWithDumpFiles(t *testing.T) {
	data := map[string][]byte{
		"/tmp/test-1": []byte("1"),
		"/tmp/test-2": []byte("2"),
	}
	fp := []string{}
	err := withDumpFiles(data, func(files map[string]string) error {
		for p := range files {
			r := strings.HasPrefix(p, "/tmp/test")
			assert.True(t, r)
			fp = append(fp, files[p])
		}
		return nil
	})
	assert.NoError(t, err)
	for _, path := range fp {
		_, err := os.Stat(path)
		assert.True(t, os.IsNotExist(err))
	}
}
