package process

import (
	"testing"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	enginetypes "github.com/projecteru2/core/engine/types"
)

const (
	testRoot    = "/var/lib/eru/process"
	overlayMeta = `{"id":"w1","kind":"process","podname":"prod","root_directory":"/var/lib/eru/process/w1/merged"}`
	rawMeta     = `{"id":"w1","kind":"process","podname":"prod","working_dir":"/srv/app"}`
)

func testEngine(t *testing.T, runner *sshrunnertest.Fake) *Engine {
	t.Helper()
	return &Engine{
		ep:          enginetypes.NewParams("node1", Prefix+"10.0.0.1", "", "", ""),
		runner:      runner,
		root:        testRoot,
		host:        "10.0.0.1",
		stopTimeout: defaultStopTimeout,
		execs:       sshrunner.NewExecs(),
	}
}
