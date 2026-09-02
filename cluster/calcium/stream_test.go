package calcium

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"
)

func TestProcessStdStreamStopsWhenTheCallerLeaves(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := NewTestCluster()
		defer c.pool.Release()
		ctx, cancel := context.WithCancel(t.Context())
		stdout := io.NopCloser(bytes.NewBufferString("aaaa\nbbbb\ncccc\n"))
		stderr := io.NopCloser(bytes.NewBufferString("dddd\n"))

		ch := c.processStdStream(ctx, stdout, stderr, bufio.ScanLines, byte('\n'))
		synctest.Wait()
		cancel()
		synctest.Wait()

		_, open := <-ch
		require.False(t, open, "the merged stream must close once its reader is gone")
	})
}

func TestProcessVirtualizationOutStreamCopiesTokens(t *testing.T) {
	c := NewTestCluster()
	const lines = 2048
	buf := bytes.Buffer{}
	for i := range lines {
		fmt.Fprintf(&buf, "line-%06d\n", i)
	}

	got := [][]byte{}
	for bs := range c.processVirtualizationOutStream(t.Context(), io.NopCloser(&buf), bufio.ScanLines, byte('\n')) {
		got = append(got, bs)
	}

	require.Len(t, got, lines)
	for i, bs := range got {
		require.Equal(t, fmt.Sprintf("line-%06d\n", i), string(bs))
	}
}

func TestProcessVirtualizationOutStreamReadsLongLines(t *testing.T) {
	c := NewTestCluster()
	line := bytes.Repeat([]byte("x"), 128*1024)
	reader := bytes.NewReader(append(line, '\n'))

	got := [][]byte{}
	for bs := range c.processVirtualizationOutStream(t.Context(), io.NopCloser(reader), bufio.ScanLines, byte('\n')) {
		got = append(got, bs)
	}

	require.Equal(t, [][]byte{append(line, '\n')}, got)
}

func TestProcessVirtualizationOutStreamBoundsTokens(t *testing.T) {
	c := NewTestCluster()
	c.config.GRPCConfig.MaxRecvMsgSize = 1024
	blob := bytes.Repeat([]byte("x"), 4096)

	got := [][]byte{}
	for bs := range c.processVirtualizationOutStream(t.Context(), io.NopCloser(bytes.NewReader(blob)), bufio.ScanLines, byte('\n')) {
		got = append(got, bs)
	}

	require.Empty(t, got)
}
