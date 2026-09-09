package binary

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/core/resource/plugins"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	coretypes "github.com/projecteru2/core/types"
)

const (
	fakePlugin = `#!/bin/sh
case "$1" in
verbs) echo '["remove-node", "get-metrics"]' ;;
remove-node) echo 'a log line' >&2; echo '{}' ;;
get-metrics) cat >&2 ;;
esac
`
	perNodePlugin = `#!/bin/sh
case "$1" in
verbs) echo '["get-node-resource-info"]' ;;
get-node-resource-info) n=$(sed 's/.*"nodename":"\([^"]*\)".*/\1/'); echo "{\"capacity\":{\"node\":\"$n\"},\"usage\":{}}" ;;
esac
`
	batchPlugin = `#!/bin/sh
case "$1" in
verbs) echo '["get-node-resource-info", "get-nodes-resource-info"]' ;;
get-node-resource-info) echo 'the singular verb was called' >&2; exit 1 ;;
get-nodes-resource-info) cat >/dev/null; echo '{"node_resource_info_map":{"n1":{"capacity":{"cpu":1},"usage":{}},"n2":{"capacity":{"cpu":2},"usage":{}}}}' ;;
esac
`
)

func TestCallSpawnsOnlyAdvertisedVerbs(t *testing.T) {
	p := newFakePlugin(t, fakePlugin)
	_, err := p.RemoveNode(t.Context(), "n1")
	assert.NoError(t, err)

	_, err = p.AddNode(t.Context(), "n1", nil, nil)
	assert.ErrorIs(t, err, plugins.ErrVerbNotSupported)
}

func TestCallRejectsAnEmptyResponse(t *testing.T) {
	p := newFakePlugin(t, fakePlugin)
	_, err := p.GetMetrics(t.Context(), []plugintypes.NodeRef{{Podname: "p", Nodename: "n1"}})
	assert.ErrorContains(t, err, "no response")
}

func TestGetNodesResourceInfoUsesThePluralVerb(t *testing.T) {
	p := newFakePlugin(t, batchPlugin)
	resp, err := p.GetNodesResourceInfo(t.Context(), []string{"n1", "n2"})
	require.NoError(t, err)
	assert.Len(t, resp.NodeResourceInfoMap, 2)
	assert.EqualValues(t, 2, resp.NodeResourceInfoMap["n2"].Capacity["cpu"])
}

func TestGetNodesResourceInfoFallsBackToOneCallPerNode(t *testing.T) {
	p := newFakePlugin(t, perNodePlugin)
	resp, err := p.GetNodesResourceInfo(t.Context(), []string{"n1", "n2"})
	require.NoError(t, err)
	assert.Equal(t, "n1", resp.NodeResourceInfoMap["n1"].Capacity["node"])
	assert.Equal(t, "n2", resp.NodeResourceInfoMap["n2"].Capacity["node"])
}

func TestNewPluginNeedsTheVerbList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resource-silent")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0o755))

	_, err := NewPlugin(t.Context(), path, pluginConfig(dir))
	assert.Error(t, err)
}

func newFakePlugin(t *testing.T, script string) *Plugin {
	dir := t.TempDir()
	path := filepath.Join(dir, "resource-fake")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	p, err := NewPlugin(t.Context(), path, pluginConfig(dir))
	require.NoError(t, err)
	return p
}

func pluginConfig(dir string) coretypes.Config {
	return coretypes.Config{ResourcePlugin: coretypes.ResourcePluginConfig{Dir: dir, CallTimeout: 5 * time.Second}}
}
