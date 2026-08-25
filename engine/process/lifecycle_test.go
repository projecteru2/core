package process

import (
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

const showOutput = `LoadState=loaded
ActiveState=active
SubState=running
ExecMainPID=42
ExecMainStatus=0
MemoryCurrent=1024
CPUUsageNSec=99
User=app
`

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

func TestVirtualizationUpdateResourceSetsLiveProperties(t *testing.T) {
	runner := &sshrunnertest.Fake{}
	e := testEngine(t, runner)

	params := resourcetypes.Resources{"cpumem": {"cpu": 2.0, "memory": 1 << 30}}
	if err := e.VirtualizationUpdateResource(t.Context(), "w1", params); err != nil {
		t.Fatalf("update: %v", err)
	}
	want := sshrunner.Quote([]string{
		"systemctl", "set-property", "--runtime", "eru-w1.service",
		"CPUQuota=200%", "AllowedCPUs=", "AllowedMemoryNodes=", "CPUWeight=100",
		"MemoryMax=1073741824", "MemoryLow=536870912", "MemorySwapMax=0",
		"IOReadIOPSMax=", "IOWriteIOPSMax=", "IOReadBandwidthMax=", "IOWriteBandwidthMax=",
	})
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
	want := sshrunner.Quote([]string{
		"systemctl", "set-property", "--runtime", "eru-w1.service",
		"CPUQuota=", "AllowedCPUs=", "AllowedMemoryNodes=", "CPUWeight=100",
		"MemoryMax=infinity", "MemoryLow=0", "MemorySwapMax=0",
		"IOReadIOPSMax=", "IOWriteIOPSMax=", "IOReadBandwidthMax=", "IOWriteBandwidthMax=",
	})
	if len(runner.Lines()) != 1 || runner.Lines()[0] != want {
		t.Errorf("got %q, want %q", runner.Lines(), want)
	}
}
