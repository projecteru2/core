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
