package process

import (
	"slices"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/engine/workloadmeta"
	coretypes "github.com/projecteru2/core/types"
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

func TestAWorkloadTheNodeLostDropsItsCachedRecord(t *testing.T) {
	gone := false
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result {
		if gone {
			return &sshrunner.Result{Code: workloadmeta.NotExistsCode}
		}
		return &sshrunner.Result{Stdout: "1\n" + overlayMeta}
	}}
	e := testEngine(t, runner)

	if _, err := e.record(t.Context(), "w1"); err != nil {
		t.Fatalf("record: %v", err)
	}
	if _, ok := e.records.Load("w1"); !ok {
		t.Fatal("the first read must cache the record")
	}

	gone = true
	if _, err := e.VirtualizationInspect(t.Context(), "w1"); !errors.Is(err, coretypes.ErrWorkloadNotExists) {
		t.Fatalf("got %v, want ErrWorkloadNotExists", err)
	}
	if _, ok := e.records.Load("w1"); ok {
		t.Error("a workload the node no longer has must not stay cached")
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
			"a working directory is entered inside the chroot by the shell, not by env",
			overlay,
			&enginetypes.ExecConfig{WorkingDir: "/srv", Cmd: []string{"ls", "-l"}},
			[]string{"chroot", testRoot + "/w1/merged", "sh", "-c", chdirScript, "sh", "/srv", "ls", "-l"},
		},
		{
			"a working directory is entered without a chroot too",
			raw,
			&enginetypes.ExecConfig{WorkingDir: "/srv", Cmd: []string{"ls", "-l"}},
			[]string{"sh", "-c", chdirScript, "sh", "/srv", "ls", "-l"},
		},
		{
			"the root working directory needs no wrapper at all",
			overlay,
			&enginetypes.ExecConfig{WorkingDir: "/", Cmd: []string{"sh"}},
			[]string{"chroot", testRoot + "/w1/merged", "sh"},
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
