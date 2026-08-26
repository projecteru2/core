package cocoon

import (
	"slices"
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

func TestExecuteRendersTheGuestCommand(t *testing.T) {
	tests := []struct {
		name   string
		config *enginetypes.ExecConfig
		want   []string
	}{
		{
			"a named user is resolved by the guest's passwd",
			&enginetypes.ExecConfig{User: "app", Cmd: []string{"id"}},
			[]string{"runuser", "-u", "app", "--", "id"},
		},
		{
			"a named user whose group differs from its name",
			&enginetypes.ExecConfig{User: "nobody", Cmd: []string{"id"}},
			[]string{"runuser", "-u", "nobody", "--", "id"},
		},
		{
			"a named user and a group keep both",
			&enginetypes.ExecConfig{User: "app:staff", Cmd: []string{"id"}},
			[]string{"setpriv", "--reuid=app", "--regid=staff", "--clear-groups", "--", "id"},
		},
		{
			"a numeric user needs no passwd lookup",
			&enginetypes.ExecConfig{User: "1000", Cmd: []string{"id"}},
			[]string{"setpriv", "--reuid=1000", "--regid=1000", "--clear-groups", "--", "id"},
		},
		{
			"a numeric user and group",
			&enginetypes.ExecConfig{User: "1000:2000", Cmd: []string{"id"}},
			[]string{"setpriv", "--reuid=1000", "--regid=2000", "--clear-groups", "--", "id"},
		},
		{
			"a working dir is entered as the target user",
			&enginetypes.ExecConfig{User: "app", WorkingDir: "/srv/app", Cmd: []string{"ls", "-l"}},
			[]string{"runuser", "-u", "app", "--", "env", "--chdir=/srv/app", "ls", "-l"},
		},
		{
			"a numeric user with a working dir",
			&enginetypes.ExecConfig{User: "1000", WorkingDir: "/srv/app", Cmd: []string{"ls"}},
			[]string{"setpriv", "--reuid=1000", "--regid=1000", "--clear-groups", "--", "env", "--chdir=/srv/app", "ls"},
		},
		{
			"a working dir without a user",
			&enginetypes.ExecConfig{WorkingDir: "/srv/app", Cmd: []string{"ls"}},
			[]string{"env", "--chdir=/srv/app", "ls"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &sshrunnertest.Fake{Started: []*sshrunnertest.Session{{}}, Respond: runningRecord}
			e := testEngine(t, runner)

			if _, _, _, _, err := e.Execute(t.Context(), "w1", tt.config); err != nil {
				t.Fatalf("execute: %v", err)
			}
			want := sshrunner.Quote(slices.Concat([]string{testBinary, "vm", "exec", "w1", "--"}, tt.want))
			if lines := runner.Lines(); len(lines) != 2 || lines[1] != want {
				t.Errorf("got %q, want the guest check then %q", lines, want)
			}
		})
	}
}

func TestExecuteRefusesAUserOrWorkingDirOnAWindowsGuest(t *testing.T) {
	tests := []struct {
		name   string
		config *enginetypes.ExecConfig
	}{
		{"a user", &enginetypes.ExecConfig{User: "app", Cmd: []string{"whoami"}}},
		{"a working dir", &enginetypes.ExecConfig{WorkingDir: `C:\app`, Cmd: []string{"dir"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result {
				return &sshrunner.Result{Stdout: storedRecord + "\n" + bootedWindowsVM}
			}}
			e := testEngine(t, runner)

			_, _, _, _, err := e.Execute(t.Context(), "w1", tt.config)
			if !errors.Is(err, coretypes.ErrEngineNotImplemented) {
				t.Errorf("got %v, want ErrEngineNotImplemented", err)
			}
		})
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

	if _, err := e.ExecExitCode(t.Context(), "w1", "missing"); !errors.Is(err, sshrunner.ErrExecNotFound) {
		t.Errorf("got %v, want ErrExecNotFound", err)
	}
}
