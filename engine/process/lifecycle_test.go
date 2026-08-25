package process

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	enginetypes "github.com/projecteru2/core/engine/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	coretypes "github.com/projecteru2/core/types"
)

const (
	showOutput = `LoadState=loaded
ActiveState=active
SubState=running
ExecMainPID=42
ExecMainStatus=0
MemoryCurrent=1024
CPUUsageNSec=99
User=app
`

	systemctlStub = `#!/bin/sh
case "$1" in
show)
case "$*" in
*LoadState*) echo "$STUB_LOADSTATE";;
*SubState*) echo "$STUB_SUBSTATE";;
esac
exit 0;;
esac
echo "$@" >> "$STUB_LOG"
for verb in $STUB_FAIL; do
if [ "$verb" = "$1" ]; then
echo "Failed to $1 eru-w1.service: Unit eru-w1.service not loaded." 1>&2
exit 1
fi
done
exit 0
`
)

func TestVirtualizationCreateRecordsTheUnitAndTheMetaFile(t *testing.T) {
	runner := &sshrunnertest.Fake{}
	e := testEngine(t, runner)

	created, err := e.VirtualizationCreate(t.Context(), &enginetypes.VirtualizationCreateOptions{
		Name:  "app_web_xyz",
		Image: "hub.io/ns/app:v1",
		Env:   []string{"ERU_POD=prod"},
		Cmd:   []string{"/bin/server"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(runner.Lines()) != 1 {
		t.Fatalf("got %d commands, want 1", len(runner.Lines()))
	}

	for _, want := range []string{
		"oras pull",
		imageDir(testRoot, "hub.io/ns/app:v1"),
		workloadDir(testRoot, created.ID),
		metaPath(created.ID),
		`"$dir/meta.json"`,
		"--unit=" + unitName(created.ID),
		"--slice=eru-prod.slice",
		"TimeoutStopSec=10",
		`rm -rf "$dir"`,
		`"podname":"prod"`,
		`"root_directory":"` + workloadDir(testRoot, created.ID) + `/merged"`,
	} {
		if !strings.Contains(runner.Lines()[0], want) {
			t.Errorf("create command does not carry %q", want)
		}
	}
}

func TestVirtualizationCreateSkipsTheOverlayForARawWorkload(t *testing.T) {
	runner := &sshrunnertest.Fake{}
	e := testEngine(t, runner)

	created, err := e.VirtualizationCreate(t.Context(), &enginetypes.VirtualizationCreateOptions{
		Name:    "app_web_xyz",
		Image:   "hub.io/ns/app:v1",
		Env:     []string{"ERU_POD=prod"},
		Cmd:     []string{"./server"},
		RawArgs: []byte(`{"raw": true, "tasks_max": 64}`),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if strings.Contains(runner.Lines()[0], "RootDirectory=") {
		t.Error("a raw workload must not get a RootDirectory")
	}
	for _, want := range []string{
		"WorkingDirectory=" + workloadDir(testRoot, created.ID) + "/lower",
		"TasksMax=64",
	} {
		if !strings.Contains(runner.Lines()[0], want) {
			t.Errorf("create command does not carry %q", want)
		}
	}
}

func TestVirtualizationCreateRejectsAnUnusablePodname(t *testing.T) {
	e := testEngine(t, &sshrunnertest.Fake{})

	_, err := e.VirtualizationCreate(t.Context(), &enginetypes.VirtualizationCreateOptions{
		Name:  "app_web_xyz",
		Image: "hub.io/ns/app:v1",
		Env:   []string{"ERU_POD=bad/pod"},
	})
	if !errors.Is(err, coretypes.ErrInvalidEngineArgs) {
		t.Errorf("got %v, want ErrInvalidEngineArgs", err)
	}
}

func TestVirtualizationLifecycleCommandSequence(t *testing.T) {
	runner := &sshrunnertest.Fake{}
	e := testEngine(t, runner)
	ctx := t.Context()

	if err := e.VirtualizationStart(ctx, "w1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := e.VirtualizationStop(ctx, "w1", 0); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if err := e.VirtualizationSuspend(ctx, "w1"); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if err := e.VirtualizationResume(ctx, "w1"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if err := e.VirtualizationRemove(ctx, "w1", true, true); err != nil {
		t.Fatalf("remove: %v", err)
	}

	want := []string{
		sshrunner.Quote([]string{"sh", "-c", startScript, "sh", testRoot + "/w1", "eru-w1.service", "/run/eru/workloads/w1.json"}),
		sshrunner.Quote([]string{"sh", "-c", stopScript, "sh", "eru-w1.service", testRoot + "/w1", "1"}),
		sshrunner.Quote([]string{"systemctl", "freeze", "eru-w1.service"}),
		sshrunner.Quote([]string{"systemctl", "thaw", "eru-w1.service"}),
		sshrunner.Quote([]string{"sh", "-c", removeScript, "sh", "eru-w1.service", testRoot + "/w1", "/run/eru/workloads/w1.json", "1"}),
	}
	if !slices.Equal(runner.Lines(), want) {
		t.Errorf("got %q, want %q", runner.Lines(), want)
	}
}

func TestVirtualizationStopOnlyKillsWhenForced(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		force   string
	}{
		{"the engine default stops gracefully", -1, "0"},
		{"a zero timeout forces the stop", 0, "1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &sshrunnertest.Fake{}
			e := testEngine(t, runner)

			if err := e.VirtualizationStop(t.Context(), "w1", tt.timeout); err != nil {
				t.Fatalf("stop: %v", err)
			}
			want := sshrunner.Quote([]string{"sh", "-c", stopScript, "sh", "eru-w1.service", testRoot + "/w1", tt.force})
			if len(runner.Lines()) != 1 || runner.Lines()[0] != want {
				t.Errorf("got %q, want %q", runner.Lines(), want)
			}
		})
	}
}

func TestVirtualizationRemoveReportsAMissingWorkload(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Code: notExistsCode} }}
	e := testEngine(t, runner)

	if err := e.VirtualizationRemove(t.Context(), "w1", true, false); !errors.Is(err, coretypes.ErrWorkloadNotExists) {
		t.Errorf("got %v, want ErrWorkloadNotExists", err)
	}
}

func TestVirtualizationInspectParsesSystemctlShow(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Stdout: showOutput} }}
	e := testEngine(t, runner)

	info, err := e.VirtualizationInspect(t.Context(), "w1")
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if !info.Running {
		t.Error("got a stopped workload, want a running one")
	}
	if info.User != "app" {
		t.Errorf("got user %q, want %q", info.User, "app")
	}
	if info.Networks[hostNetwork] != "10.0.0.1" {
		t.Errorf("got networks %v, want the node address on %q", info.Networks, hostNetwork)
	}
}

func TestVirtualizationInspectReportsAnExitedWorkloadAsStopped(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result {
		return &sshrunner.Result{Stdout: "LoadState=not-found\nActiveState=inactive\nSubState=dead\nUser=\n"}
	}}
	e := testEngine(t, runner)

	info, err := e.VirtualizationInspect(t.Context(), "w1")
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if info.Running {
		t.Error("an unloaded unit whose directory still exists is a stopped workload")
	}
}

func TestVirtualizationInspectReportsAMissingWorkload(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Code: notExistsCode} }}
	e := testEngine(t, runner)

	_, err := e.VirtualizationInspect(t.Context(), "w1")
	if !errors.Is(err, coretypes.ErrWorkloadNotExists) {
		t.Errorf("got %v, want ErrWorkloadNotExists", err)
	}
}

func TestVirtualizationWaitReturnsExecMainStatus(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Stdout: "3\n"} }}
	e := testEngine(t, runner)

	waited, err := e.VirtualizationWait(t.Context(), "w1", "")
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if waited.Code != 3 {
		t.Errorf("got code %d, want 3", waited.Code)
	}
}

func TestVirtualizationWaitFailsOnAnUnreadableStatus(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Stdout: "\n"} }}
	e := testEngine(t, runner)

	waited, err := e.VirtualizationWait(t.Context(), "w1", "")
	if err == nil {
		t.Fatal("an unparsable ExecMainStatus must not be reported as a clean exit")
	}
	if waited.Code != -1 {
		t.Errorf("got code %d, want -1", waited.Code)
	}
}

func TestVirtualizationRemoveRefusesARunningWorkload(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Code: runningCode} }}
	e := testEngine(t, runner)

	if err := e.VirtualizationRemove(t.Context(), "w1", true, false); !errors.Is(err, coretypes.ErrInvaildWorkloadOps) {
		t.Errorf("got %v, want ErrInvaildWorkloadOps", err)
	}
}

func TestRemoveScriptToleratesAnUnloadedUnit(t *testing.T) {
	node := newStubNode(t)
	node.loadState, node.fail = "not-found", "stop reset-failed"

	code := node.run(t, removeScript, "eru-w1.service", node.dir, node.record, "1")

	if code != 0 {
		t.Fatalf("got exit %d, want a forced remove to survive a unit systemd has already unloaded", code)
	}
	node.assertGone(t)
}

func TestRemoveScriptToleratesAResetFailedOnAUnitTheStopUnloaded(t *testing.T) {
	node := newStubNode(t)
	node.loadState, node.subState, node.fail = "loaded", "dead", "reset-failed"

	code := node.run(t, removeScript, "eru-w1.service", node.dir, node.record, "1")

	if code != 0 {
		t.Fatalf("got exit %d, want the remove to survive: stopping a transient unit unloads it", code)
	}
	node.assertGone(t)
}

func TestRemoveScriptStillRefusesARunningWorkload(t *testing.T) {
	node := newStubNode(t)
	node.loadState, node.subState = "loaded", subStateRunning

	code := node.run(t, removeScript, "eru-w1.service", node.dir, node.record, "0")

	if code != runningCode {
		t.Fatalf("got exit %d, want %d", code, runningCode)
	}
	if _, err := os.Stat(node.dir); err != nil {
		t.Errorf("a refused remove must leave the workload directory: %v", err)
	}
}

func TestRemoveScriptReportsAMissingWorkloadDirectory(t *testing.T) {
	node := newStubNode(t)
	node.loadState = "not-found"
	if err := os.RemoveAll(node.dir); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if code := node.run(t, removeScript, "eru-w1.service", node.dir, node.record, "1"); code != notExistsCode {
		t.Fatalf("got exit %d, want %d", code, notExistsCode)
	}
}

func TestUpdateScriptIsANoOpOnAnUnloadedUnit(t *testing.T) {
	node := newStubNode(t)
	node.loadState, node.fail = "not-found", "set-property"

	code := node.run(t, updateScript, "eru-w1.service", "CPUQuota=200%")

	if code != 0 {
		t.Fatalf("got exit %d, want a remap on a stopped workload to be a no-op", code)
	}
	if log := node.log(t); log != "" {
		t.Errorf("got %q, want nothing sent to a unit that is not loaded", log)
	}
}

func TestUpdateScriptSetsThePropertiesOfALoadedUnit(t *testing.T) {
	node := newStubNode(t)
	node.loadState = "loaded"

	code := node.run(t, updateScript, "eru-w1.service", "CPUQuota=200%", "MemoryMax=1073741824")

	if code != 0 {
		t.Fatalf("got exit %d, want the properties set", code)
	}
	want := "set-property --runtime eru-w1.service CPUQuota=200% MemoryMax=1073741824\n"
	if log := node.log(t); log != want {
		t.Errorf("got %q, want %q", log, want)
	}
}

func TestVirtualizationUpdateResourceSetsLiveProperties(t *testing.T) {
	runner := &sshrunnertest.Fake{}
	e := testEngine(t, runner)

	params := resourcetypes.Resources{"cpumem": {"cpu": 2.0, "memory": 1 << 30}}
	if err := e.VirtualizationUpdateResource(t.Context(), "w1", params); err != nil {
		t.Fatalf("update: %v", err)
	}
	want := sshrunner.Quote(sshrunner.Shell(updateScript,
		"eru-w1.service",
		"CPUQuota=200%", "AllowedCPUs=", "AllowedMemoryNodes=", "CPUWeight=100",
		"MemoryMax=1073741824", "MemoryLow=536870912", "MemorySwapMax=0",
		"IOReadIOPSMax=", "IOWriteIOPSMax=", "IOReadBandwidthMax=", "IOWriteBandwidthMax=",
	))
	if len(runner.Lines()) != 1 || runner.Lines()[0] != want {
		t.Errorf("got %q, want %q", runner.Lines(), want)
	}
}

func TestVirtualizationUpdateResourceClearsTheOldShape(t *testing.T) {
	runner := &sshrunnertest.Fake{}
	e := testEngine(t, runner)

	if err := e.VirtualizationUpdateResource(t.Context(), "w1", resourcetypes.Resources{}); err != nil {
		t.Fatalf("update: %v", err)
	}
	want := sshrunner.Quote(sshrunner.Shell(updateScript,
		"eru-w1.service",
		"CPUQuota=", "AllowedCPUs=", "AllowedMemoryNodes=", "CPUWeight=100",
		"MemoryMax=infinity", "MemoryLow=0", "MemorySwapMax=0",
		"IOReadIOPSMax=", "IOWriteIOPSMax=", "IOReadBandwidthMax=", "IOWriteBandwidthMax=",
	))
	if len(runner.Lines()) != 1 || runner.Lines()[0] != want {
		t.Errorf("got %q, want %q", runner.Lines(), want)
	}
}

type stubNode struct {
	bin       string
	dir       string
	record    string
	logPath   string
	loadState string
	subState  string
	fail      string
}

func newStubNode(t *testing.T) *stubNode {
	t.Helper()
	root := t.TempDir()
	node := &stubNode{
		bin:     filepath.Join(root, "bin"),
		dir:     filepath.Join(root, "w1"),
		record:  filepath.Join(root, "w1.json"),
		logPath: filepath.Join(root, "systemctl.log"),
	}
	for _, path := range []string{node.bin, node.dir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	if err := os.WriteFile(node.record, []byte("{}"), 0o644); err != nil {
		t.Fatalf("setup record: %v", err)
	}
	for name, body := range map[string]string{
		"systemctl":  systemctlStub,
		"mountpoint": "#!/bin/sh\nexit 1\n",
	} {
		if err := os.WriteFile(filepath.Join(node.bin, name), []byte(body), 0o755); err != nil {
			t.Fatalf("setup %s: %v", name, err)
		}
	}
	return node
}

func (n *stubNode) run(t *testing.T, script string, args ...string) int {
	t.Helper()
	cmd := exec.Command("sh", slices.Concat([]string{"-c", script, "sh"}, args)...)
	cmd.Env = slices.Concat(os.Environ(), []string{
		"PATH=" + n.bin + ":" + os.Getenv("PATH"),
		"STUB_LOADSTATE=" + n.loadState,
		"STUB_SUBSTATE=" + n.subState,
		"STUB_FAIL=" + n.fail,
		"STUB_LOG=" + n.logPath,
	})
	out, err := cmd.CombinedOutput()
	t.Logf("script output: %s", out)
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exitErr):
		return exitErr.ExitCode()
	default:
		t.Fatalf("run: %v", err)
		return -1
	}
}

func (n *stubNode) log(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(n.logPath)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	return string(body)
}

func (n *stubNode) assertGone(t *testing.T) {
	t.Helper()
	for _, path := range []string{n.dir, n.record} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s survived the remove: %v", path, err)
		}
	}
}
