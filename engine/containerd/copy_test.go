package containerd

import (
	"slices"
	"strings"
	"testing"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	enginetypes "github.com/projecteru2/core/engine/types"
)

func TestMissingPathSeparatesTarsWarningsFromItsFailures(t *testing.T) {
	tests := []struct {
		name string
		res  *sshrunner.Result
		want bool
	}{
		{"a warning about a changed file is not a missing path", &sshrunner.Result{Code: 1, Stderr: "tar: file changed as we read it"}, false},
		{"a fatal error is", &sshrunner.Result{Code: 2, Stderr: "tar: /nope: Cannot stat"}, true},
		{"so is the message, whatever the code", &sshrunner.Result{Code: 1, Stderr: "tar: /nope: No such file or directory"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := missingPath(tt.res); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCopyIntoARunningWorkloadExecsTar(t *testing.T) {
	e := testEngine(t, &sshrunnertest.Fake{})

	config := &enginetypes.ExecConfig{Cmd: []string{"tar", "-x", "-C", "/srv"}}
	argv := e.execArgv("app_web_abc123", "e1", config)

	want := []string{
		ctrBinary, "--address", defaultSocket, "--namespace", defaultNamespace, "tasks", "exec", "--exec-id", "e1",
		"app_web_abc123", "tar", "-x", "-C", "/srv",
	}
	if !slices.Equal(argv, want) {
		t.Errorf("got %q, want %q", argv, want)
	}
}

func TestCopyIntoAWorkloadWithNoTaskMountsItsSnapshot(t *testing.T) {
	e := testEngine(t, &sshrunnertest.Fake{})

	argv := e.snapshotArgv("app_web_abc123", "app_web_abc123", "/srv/app.conf")

	if argv[0] != "sh" || argv[2] != snapshotScript {
		t.Fatalf("got %q, want the snapshot script", argv)
	}
	args := argv[4:]
	want := []string{
		ctrBinary, defaultSocket, defaultNamespace, "app_web_abc123",
		workloadRoot + "/app_web_abc123/" + snapshotMount, "/srv",
	}
	if !slices.Equal(args, want) {
		t.Errorf("got %q, want %q", args, want)
	}
	for _, step := range []string{"snapshots mounts", "tar -x -C", "umount", "rmdir"} {
		if !strings.Contains(snapshotScript, step) {
			t.Errorf("the script must %s", step)
		}
	}
	if !strings.Contains(snapshotScript, "mounts=$(") {
		t.Error("a command substitution inside eval hides the mount's exit status, and tar would unpack onto the node")
	}
}
