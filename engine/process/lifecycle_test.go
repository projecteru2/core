package process

import (
	"slices"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"

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
	runner := &fakeRunner{}
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
	if len(runner.lines) != 1 {
		t.Fatalf("got %d commands, want 1", len(runner.lines))
	}

	for _, want := range []string{
		"oras pull",
		workloadDir(testRoot, created.ID),
		metaPath(created.ID),
		"--unit=" + unitName(created.ID),
		"--slice=eru-prod.slice",
		`"podname":"prod"`,
		`"root_directory":"` + workloadDir(testRoot, created.ID) + `/merged"`,
	} {
		if !strings.Contains(runner.lines[0], want) {
			t.Errorf("create command does not carry %q", want)
		}
	}
}

func TestVirtualizationCreateSkipsTheOverlayForARawWorkload(t *testing.T) {
	runner := &fakeRunner{}
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
	if strings.Contains(runner.lines[0], "RootDirectory=") {
		t.Error("a raw workload must not get a RootDirectory")
	}
	for _, want := range []string{
		"WorkingDirectory=" + workloadDir(testRoot, created.ID) + "/lower",
		"TasksMax=64",
	} {
		if !strings.Contains(runner.lines[0], want) {
			t.Errorf("create command does not carry %q", want)
		}
	}
}

func TestVirtualizationLifecycleCommandSequence(t *testing.T) {
	runner := &fakeRunner{}
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
		quote([]string{"sh", "-c", startScript, "sh", testRoot + "/w1"}),
		quote([]string{"sh", "-c", stopScript, "sh", "eru-w1.service", testRoot + "/w1", "1"}),
		quote([]string{"systemctl", "freeze", "eru-w1.service"}),
		quote([]string{"systemctl", "thaw", "eru-w1.service"}),
		quote([]string{"sh", "-c", removeScript, "sh", "eru-w1.service", testRoot + "/w1", "/run/eru/workloads/w1.json", "1"}),
	}
	if !slices.Equal(runner.lines, want) {
		t.Errorf("got %q, want %q", runner.lines, want)
	}
}

func TestVirtualizationRemoveReportsAMissingWorkload(t *testing.T) {
	runner := &fakeRunner{respond: func(string) *result { return &result{Code: notExistsCode} }}
	e := testEngine(t, runner)

	if err := e.VirtualizationRemove(t.Context(), "w1", true, false); !errors.Is(err, coretypes.ErrWorkloadNotExists) {
		t.Errorf("got %v, want ErrWorkloadNotExists", err)
	}
}

func TestVirtualizationInspectParsesSystemctlShow(t *testing.T) {
	runner := &fakeRunner{respond: func(string) *result { return &result{Stdout: showOutput} }}
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

func TestVirtualizationInspectReportsAMissingUnit(t *testing.T) {
	runner := &fakeRunner{respond: func(string) *result { return &result{Stdout: "LoadState=not-found\nActiveState=inactive\n"} }}
	e := testEngine(t, runner)

	_, err := e.VirtualizationInspect(t.Context(), "w1")
	if !errors.Is(err, coretypes.ErrWorkloadNotExists) {
		t.Errorf("got %v, want ErrWorkloadNotExists", err)
	}
}

func TestVirtualizationWaitReturnsExecMainStatus(t *testing.T) {
	runner := &fakeRunner{respond: func(string) *result { return &result{Stdout: "3\n"} }}
	e := testEngine(t, runner)

	waited, err := e.VirtualizationWait(t.Context(), "w1", "")
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if waited.Code != 3 {
		t.Errorf("got code %d, want 3", waited.Code)
	}
}

func TestVirtualizationUpdateResourceSetsLiveProperties(t *testing.T) {
	runner := &fakeRunner{}
	e := testEngine(t, runner)

	params := resourcetypes.Resources{"cpumem": {"cpu": 2.0, "memory": 1 << 30}}
	if err := e.VirtualizationUpdateResource(t.Context(), "w1", params); err != nil {
		t.Fatalf("update: %v", err)
	}
	want := quote([]string{
		"systemctl", "set-property", "eru-w1.service",
		"CPUQuota=200%", "MemoryMax=1073741824", "MemoryHigh=536870912",
	})
	if len(runner.lines) != 1 || runner.lines[0] != want {
		t.Errorf("got %q, want %q", runner.lines, want)
	}
}
