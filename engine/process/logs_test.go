package process

import (
	"io"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestVirtualizationLogsBuffersWhenNotFollowing(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Stdout: "hello\n"} }}
	e := testEngine(t, runner)

	stdout, stderr, err := e.VirtualizationLogs(t.Context(), &enginetypes.VirtualizationLogStreamOptions{ID: "w1"})
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	if stderr != nil {
		t.Error("journald merges the streams, so stderr must stay nil")
	}
	want := sshrunner.Quote([]string{"journalctl", "-u", "eru-w1.service", "-o", "cat"})
	if len(runner.Lines()) != 1 || runner.Lines()[0] != want {
		t.Errorf("got %q, want %q", runner.Lines(), want)
	}
	body, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(body) != "hello\n" {
		t.Errorf("got %q, want %q", body, "hello\n")
	}
}

func TestVirtualizationLogsFollowStopsWithTheUnit(t *testing.T) {
	running := &sshrunnertest.Session{Out: "line\n"}
	runner := &sshrunnertest.Fake{Started: running}
	e := testEngine(t, runner)

	stdout, _, err := e.VirtualizationLogs(t.Context(), &enginetypes.VirtualizationLogStreamOptions{ID: "w1", Follow: true})
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	want := sshrunner.Quote(sshrunner.Shell(followScript, "eru-w1.service", "-f", "-o", "cat"))
	if len(runner.Lines()) != 1 || runner.Lines()[0] != want {
		t.Fatalf("got %q, want %q", runner.Lines(), want)
	}
	if !strings.Contains(followScript, "kill") {
		t.Error("the follow script must kill journalctl once the unit leaves running")
	}
	if err = stdout.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !running.Closed() {
		t.Error("closing the log stream must close the ssh session")
	}
}

func TestVirtualizationAttachRefusesStdin(t *testing.T) {
	e := testEngine(t, &sshrunnertest.Fake{})

	_, _, _, err := e.VirtualizationAttach(t.Context(), "w1", true, true)
	if !errors.Is(err, coretypes.ErrEngineNotImplemented) {
		t.Errorf("got %v, want ErrEngineNotImplemented", err)
	}
}
