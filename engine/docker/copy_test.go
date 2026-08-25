package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	dockerapi "github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockermocks "github.com/projecteru2/core/engine/docker/mocks"
	coretypes "github.com/projecteru2/core/types"
)

func TestVirtualizationCopyFromClosesResponse(t *testing.T) {
	body := &countingReadCloser{Reader: bytes.NewReader(tarWithFile(t, "hello", []byte("data")))}
	client := dockermocks.NewAPIClient(t)
	client.On("CopyFromContainer", mock.Anything, "wid", dockerapi.CopyFromContainerOptions{SourcePath: "/hello"}).
		Return(dockerapi.CopyFromContainerResult{Content: body}, nil)

	e := &Engine{client: client}
	content, _, _, _, err := e.VirtualizationCopyFrom(t.Context(), "wid", "/hello")
	require.NoError(t, err)
	assert.Equal(t, []byte("data"), content)
	assert.EqualValues(t, 1, body.closes.Load(), "the CopyFromContainer body was never closed")
}

func TestVirtualizationCopyChunkToReleasesWriterOnError(t *testing.T) {
	e := &Engine{client: &copyToClient{err: coretypes.ErrWorkloadNotExists}}
	done := make(chan error, 1)
	go func() {
		done <- e.VirtualizationCopyChunkTo(t.Context(), "wid", "/tmp/f", 4, strings.NewReader("data"), 0, 0, 0o644)
	}()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, coretypes.ErrWorkloadNotExists)
	case <-time.After(5 * time.Second):
		t.Fatal("VirtualizationCopyChunkTo hung: the tar writer was never released")
	}
}

func TestVirtualizationCopyChunkToWritesAWellFormedArchive(t *testing.T) {
	client := &copyToClient{}
	e := &Engine{client: client}
	require.NoError(t, e.VirtualizationCopyChunkTo(t.Context(), "wid", "/tmp/f", 4, strings.NewReader("data"), 7, 9, 0o600))

	tr := tar.NewReader(bytes.NewReader(client.body.Bytes()))
	hdr, err := tr.Next()
	require.NoError(t, err)
	assert.Equal(t, "f", hdr.Name)
	assert.Equal(t, 7, hdr.Uid)
	assert.Equal(t, 9, hdr.Gid)
	payload, err := io.ReadAll(tr)
	require.NoError(t, err)
	assert.Equal(t, []byte("data"), payload)

	_, err = tr.Next()
	assert.ErrorIs(t, err, io.EOF)
	assert.Equal(t, 2048, client.body.Len(), "the archive is missing its two zero end-of-archive blocks")
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

type copyToClient struct {
	dockerapi.APIClient
	err  error
	body bytes.Buffer
}

func (c *copyToClient) CopyToContainer(_ context.Context, _ string, options dockerapi.CopyToContainerOptions) (dockerapi.CopyToContainerResult, error) {
	if c.err != nil {
		return dockerapi.CopyToContainerResult{}, c.err
	}
	_, err := io.Copy(&c.body, options.Content)
	return dockerapi.CopyToContainerResult{}, err
}
