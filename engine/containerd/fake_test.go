package containerd

import (
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

func testEngine(t *testing.T, runner *sshrunnertest.Fake) *Engine {
	t.Helper()
	return &Engine{
		runner:    runner,
		config:    coretypes.Config{},
		ep:        enginetypes.NewParams("node1", Prefix+"10.0.0.1", "", "", ""),
		namespace: defaultNamespace,
		socket:    defaultSocket,
		host:      "10.0.0.1",
		platform:  ocispec.Platform{OS: "linux", Architecture: "amd64"},
	}
}
