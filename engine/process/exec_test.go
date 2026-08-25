package process

import (
	"slices"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"

	enginetypes "github.com/projecteru2/core/engine/types"
)

const overlayMeta = `{"id":"w1","kind":"process","podname":"prod","root_directory":"/var/lib/eru/process/w1/merged"}`

func TestExecuteRunsAScopeInTheWorkloadSlice(t *testing.T) {
	runner := &fakeRunner{
		started: &fakeSession{code: 7},
		respond: func(string) *result { return &result{Stdout: overlayMeta} },
	}
	e := testEngine(t, runner)

	execID, _, _, _, err := e.Execute(t.Context(), "w1", &enginetypes.ExecConfig{
		User:         "app",
		Cmd:          []string{"ls", "-l"},
		Env:          []string{"FOO=bar"},
		AttachStdout: true,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	want := quote([]string{
		"systemd-run", "--scope", "--quiet", "--collect", "--slice=eru-prod.slice",
		"--uid=app", "-p", "RootDirectory=/var/lib/eru/process/w1/merged",
		"--setenv=FOO=bar", "--", "ls", "-l",
	})
	if len(runner.lines) != 2 || runner.lines[1] != want {
		t.Fatalf("got %q, want the meta read then %q", runner.lines, want)
	}
	if !strings.Contains(runner.lines[0], metaPath("w1")) {
		t.Errorf("the first command must read %q", metaPath("w1"))
	}

	code, err := e.ExecExitCode(t.Context(), "w1", execID)
	if err != nil {
		t.Fatalf("exit code: %v", err)
	}
	if code != 7 {
		t.Errorf("got exit code %d, want 7", code)
	}
}

func TestExecExitCodeRejectsAnUnknownExec(t *testing.T) {
	e := testEngine(t, &fakeRunner{})

	if _, err := e.ExecExitCode(t.Context(), "w1", "missing"); !errors.Is(err, errExecNotFound) {
		t.Errorf("got %v, want errExecNotFound", err)
	}
}

func TestScopeArgvOmitsTheRootForARawWorkload(t *testing.T) {
	record := &meta{Podname: "prod"}

	want := []string{"systemd-run", "--scope", "--quiet", "--collect", "--slice=eru-prod.slice", "--", "sh"}
	if got := scopeArgv(record, &enginetypes.ExecConfig{Cmd: []string{"sh"}}); !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
