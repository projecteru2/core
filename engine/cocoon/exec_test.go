package cocoon

import (
	"testing"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestExecuteRunsThroughTheAgentInPipeMode(t *testing.T) {
	runner := &sshrunnertest.Fake{Started: []*sshrunnertest.Session{{Code: 7}}}
	e := testEngine(t, runner)

	execID, _, stderr, stdin, err := e.Execute(t.Context(), "w1", &enginetypes.ExecConfig{
		Cmd:         []string{"ls", "-l"},
		Env:         []string{"FOO=bar"},
		AttachStdin: true,
		Tty:         true,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	want := sshrunner.Quote([]string{testBinary, "vm", "exec", "-i", "-e", "FOO=bar", "w1", "--", "ls", "-l"})
	if len(runner.Lines()) != 1 || runner.Lines()[0] != want {
		t.Fatalf("got %q, want %q", runner.Lines(), want)
	}
	if opts := runner.Options(); len(opts) != 1 || !opts[0].Stdin || opts[0].TTY {
		t.Errorf("got %+v, want stdin without a pty", opts)
	}
	if stdin == nil || stderr == nil {
		t.Error("pipe mode hands back stdin and stderr")
	}

	code, err := e.ExecExitCode(t.Context(), "w1", execID)
	if err != nil {
		t.Fatalf("exit code: %v", err)
	}
	if code != 7 {
		t.Errorf("got exit code %d, want 7", code)
	}
}

func TestExecuteWithoutStdinClosesItAtOnce(t *testing.T) {
	runner := &sshrunnertest.Fake{Started: []*sshrunnertest.Session{{}}}
	e := testEngine(t, runner)

	_, _, _, stdin, err := e.Execute(t.Context(), "w1", &enginetypes.ExecConfig{Cmd: []string{"uname"}})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	want := sshrunner.Quote([]string{testBinary, "vm", "exec", "w1", "--", "uname"})
	if runner.Lines()[0] != want {
		t.Errorf("got %q, want %q", runner.Lines()[0], want)
	}
	if stdin != nil {
		t.Error("no stdin was asked for")
	}
}

func TestExecResizeNeedsAPty(t *testing.T) {
	e := testEngine(t, &sshrunnertest.Fake{})

	if err := e.ExecResize(t.Context(), "x", 24, 80); !errors.Is(err, coretypes.ErrEngineNotImplemented) {
		t.Errorf("got %v, want ErrEngineNotImplemented", err)
	}
}

func TestExecExitCodeRejectsAnUnknownExec(t *testing.T) {
	e := testEngine(t, &sshrunnertest.Fake{})

	if _, err := e.ExecExitCode(t.Context(), "w1", "missing"); !errors.Is(err, errExecNotFound) {
		t.Errorf("got %v, want errExecNotFound", err)
	}
}
