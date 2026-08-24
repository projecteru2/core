package docker

import (
	"archive/tar"
	"bytes"
	"io"
	"sync/atomic"
	"testing"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockermocks "github.com/projecteru2/core/engine/docker/mocks"
)

func TestVirtualizationCopyFromClosesResponse(t *testing.T) {
	body := &countingReadCloser{Reader: bytes.NewReader(tarWithFile(t, "hello", []byte("data")))}
	client := dockermocks.NewAPIClient(t)
	client.On("CopyFromContainer", mock.Anything, "wid", "/hello").
		Return(body, dockercontainer.PathStat{}, nil)

	e := &Engine{client: client}
	content, _, _, _, err := e.VirtualizationCopyFrom(t.Context(), "wid", "/hello")
	require.NoError(t, err)
	assert.Equal(t, []byte("data"), content)
	assert.EqualValues(t, 1, body.closes.Load(), "the CopyFromContainer body was never closed")
}

func tarWithFile(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	tw := tar.NewWriter(buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Size: int64(len(data)), Mode: 0o644}))
	_, err := tw.Write(data)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	return buf.Bytes()
}

type countingReadCloser struct {
	io.Reader
	closes atomic.Int64
}

func (c *countingReadCloser) Close() error {
	c.closes.Add(1)
	return nil
}
