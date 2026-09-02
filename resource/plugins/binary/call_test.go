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

const fakePlugin = `#!/bin/sh
case "$1" in
verbs) echo '["remove-node", "get-metrics"]' ;;
remove-node) echo 'a log line' >&2; echo '{}' ;;
get-metrics) cat >&2 ;;
esac
`

func TestCallSpawnsOnlyAdvertisedVerbs(t *testing.T) {
	p := newFakePlugin(t)
	_, err := p.RemoveNode(t.Context(), "n1")
	assert.NoError(t, err)

	_, err = p.AddNode(t.Context(), "n1", nil, nil)
	assert.ErrorIs(t, err, plugins.ErrVerbNotSupported)
}

func TestCallRejectsAnEmptyResponse(t *testing.T) {
	p := newFakePlugin(t)
	_, err := p.GetMetrics(t.Context(), []plugintypes.NodeRef{{Podname: "p", Nodename: "n1"}})
	assert.ErrorContains(t, err, "no response")
}

func TestNewPluginNeedsTheVerbList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resource-silent")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0o755))

	_, err := NewPlugin(t.Context(), path, pluginConfig(dir))
	assert.Error(t, err)
}

func newFakePlugin(t *testing.T) *Plugin {
	dir := t.TempDir()
	path := filepath.Join(dir, "resource-fake")
	require.NoError(t, os.WriteFile(path, []byte(fakePlugin), 0o755))
	p, err := NewPlugin(t.Context(), path, pluginConfig(dir))
	require.NoError(t, err)
	return p
}

func pluginConfig(dir string) coretypes.Config {
	return coretypes.Config{ResourcePlugin: coretypes.ResourcePluginConfig{Dir: dir, CallTimeout: 5 * time.Second}}
}
