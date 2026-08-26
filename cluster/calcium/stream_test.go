package calcium

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProcessVirtualizationOutStreamCopiesTokens(t *testing.T) {
	c := NewTestCluster()
	const lines = 2048
	buf := bytes.Buffer{}
	for i := range lines {
		fmt.Fprintf(&buf, "line-%06d\n", i)
	}

	got := [][]byte{}
	for bs := range c.processVirtualizationOutStream(context.Background(), io.NopCloser(&buf), bufio.ScanLines, byte('\n')) {
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
	for bs := range c.processVirtualizationOutStream(context.Background(), io.NopCloser(reader), bufio.ScanLines, byte('\n')) {
		got = append(got, bs)
	}

	require.Equal(t, [][]byte{append(line, '\n')}, got)
}

func TestProcessVirtualizationOutStreamBoundsTokens(t *testing.T) {
	c := NewTestCluster()
	c.config.GRPCConfig.MaxRecvMsgSize = 1024
	blob := bytes.Repeat([]byte("x"), 4096)

	got := [][]byte{}
	for bs := range c.processVirtualizationOutStream(context.Background(), io.NopCloser(bytes.NewReader(blob)), bufio.ScanLines, byte('\n')) {
		got = append(got, bs)
	}

	require.Empty(t, got)
}
