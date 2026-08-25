package process

import (
	"slices"
	"strings"
	"testing"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	enginetypes "github.com/projecteru2/core/engine/types"
)

func TestExecuteRunsAScopeInTheWorkloadSlice(t *testing.T) {
	runner := &sshrunnertest.Fake{
		Started: []*sshrunnertest.Session{{Code: 7}},
		Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Stdout: "1\n" + overlayMeta} },
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

	want := sshrunner.Quote([]string{
		"systemd-run", "--scope", "--quiet", "--collect", "--slice=eru-prod.slice",
		"--setenv=FOO=bar", "--",
		"chroot", "--userspec=app", "/var/lib/eru/process/w1/merged", "ls", "-l",
	})
	if len(runner.Lines()) != 2 || runner.Lines()[1] != want {
		t.Fatalf("got %q, want the meta read then %q", runner.Lines(), want)
	}
	if !strings.Contains(runner.Lines()[0], workloadDir(testRoot, "w1")) {
		t.Errorf("the first command must read the record under %q", workloadDir(testRoot, "w1"))
	}

	code, err := e.ExecExitCode(t.Context(), "w1", execID)
	if err != nil {
		t.Fatalf("exit Code: %v", err)
	}
	if code != 7 {
		t.Errorf("got exit code %d, want 7", code)
	}
}

func TestScopeArgv(t *testing.T) {
	scope := []string{"systemd-run", "--scope", "--quiet", "--collect", "--slice=eru-prod.slice", "--"}
	overlay := &meta{Podname: "prod", RootDirectory: testRoot + "/w1/merged"}
	raw := &meta{Podname: "prod"}

	tests := []struct {
		name   string
		record *meta
		config *enginetypes.ExecConfig
		want   []string
	}{
		{
			"an overlay workload without a user is entered by chroot alone",
			overlay,
			&enginetypes.ExecConfig{Cmd: []string{"sh"}},
			[]string{"chroot", testRoot + "/w1/merged", "sh"},
		},
		{
			"a working directory is entered inside the chroot",
			overlay,
			&enginetypes.ExecConfig{WorkingDir: "/srv", Cmd: []string{"sh"}},
			[]string{"chroot", testRoot + "/w1/merged", "env", "--chdir=/srv", "sh"},
		},
		{
			"a raw workload drops privileges with setpriv",
			raw,
			&enginetypes.ExecConfig{User: "app:staff", Cmd: []string{"sh"}},
			[]string{"setpriv", "--reuid=app", "--regid=staff", "--init-groups", "--", "sh"},
		},
		{
			"a bare user name reuses itself as the group",
			raw,
			&enginetypes.ExecConfig{User: "app", Cmd: []string{"sh"}},
			[]string{"setpriv", "--reuid=app", "--regid=app", "--init-groups", "--", "sh"},
		},
		{
			"a raw workload without a user runs as the ssh user",
			raw,
			&enginetypes.ExecConfig{Cmd: []string{"sh"}},
			[]string{"sh"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := slices.Concat(scope, tt.want)
			if got := scopeArgv(tt.record, tt.config); !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}
