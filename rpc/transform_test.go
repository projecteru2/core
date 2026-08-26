package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/core/types"
)

func TestToSendLargeFileChunksIncludesAnEmptyFile(t *testing.T) {
	file := types.LinuxFile{Filename: "/tmp/empty", Mode: 0o644, UID: 1, GID: 2}
	chunks := toSendLargeFileChunks(file, []string{"workload1"})

	require.Len(t, chunks, 1)
	assert.Equal(t, []string{"workload1"}, chunks[0].IDs)
	assert.Equal(t, file.Filename, chunks[0].Dst)
	assert.Zero(t, chunks[0].Size)
	assert.Empty(t, chunks[0].Chunk)
	assert.Equal(t, file.Mode, chunks[0].Mode)
	assert.Equal(t, file.UID, chunks[0].UID)
	assert.Equal(t, file.GID, chunks[0].GID)
}
