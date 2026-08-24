package embedded

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCluster(t *testing.T) {
	cluster, err := New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(cluster.Close)

	cli := cluster.Client("/test")
	_, err = cli.Put(t.Context(), "k", "v")
	require.NoError(t, err)
	resp, err := cli.Get(t.Context(), "k")
	require.NoError(t, err)
	require.Len(t, resp.Kvs, 1)
	require.Equal(t, "k", string(resp.Kvs[0].Key))
}
