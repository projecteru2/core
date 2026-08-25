package cocoon

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	resourcetypes "github.com/projecteru2/core/resource/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestVirtualizationStartBootsALinuxGuestAndRecordsItsConsole(t *testing.T) {
	dialed := ""
	runner := &sshrunnertest.Fake{
		Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Stdout: linuxVM + "\n" + runningVM} },
		Dialer:  chAPI(t, "/dev/pts/3", &dialed),
	}
	e := testEngine(t, runner)

	if err := e.VirtualizationStart(t.Context(), "w1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	want := []string{
		sshrunner.Quote(sshrunner.Shell(startScript, testBinary, "w1", testRoot+"/w1.json")),
		sshrunner.Quote(sshrunner.Shell(refreshScript, testRoot+"/w1.json", "/run/eru/workloads/w1.json", "/dev/pts/3", "4242")),
	}
	if !slices.Equal(runner.Lines(), want) {
		t.Errorf("got %q, want %q", runner.Lines(), want)
	}
}

func TestVirtualizationStartProgramsAWindowsGuestOnItsFirstBoot(t *testing.T) {
	dialed := ""
	runner := &sshrunnertest.Fake{
		Respond: func(line string) *sshrunner.Result {
			if strings.Contains(line, "netsh") {
				return &sshrunner.Result{}
			}
			return &sshrunner.Result{Stdout: windowsVM + "\n" + bootedWindowsVM}
		},
		Dialer: chAPI(t, "", &dialed),
	}
	e := testEngine(t, runner)

	if err := e.VirtualizationStart(t.Context(), "w1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	lines := runner.Lines()
	if len(lines) != 3 {
		t.Fatalf("got %d commands, want the start, the console and the address", len(lines))
	}
	want := sshrunner.Quote(sshrunner.Shell(addressScript, testBinary, "w1", "10.22.0.5", "255.255.0.0", "10.22.0.1"))
	if lines[2] != want {
		t.Errorf("got %q, want %q", lines[2], want)
	}
	if !strings.Contains(addressScript, "netsh interface ip set address Ethernet static") {
		t.Error("the address script must drive netsh")
	}
}

func TestVirtualizationStartLeavesABootedWindowsGuestAlone(t *testing.T) {
	dialed := ""
	runner := &sshrunnertest.Fake{
		Respond: func(string) *sshrunner.Result {
			return &sshrunner.Result{Stdout: bootedWindowsVM + "\n" + bootedWindowsVM}
		},
		Dialer: chAPI(t, "", &dialed),
	}
	e := testEngine(t, runner)

	if err := e.VirtualizationStart(t.Context(), "w1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if lines := runner.Lines(); len(lines) != 2 || strings.Contains(lines[1], "netsh") {
		t.Errorf("got %q, want the start and the console only", lines)
	}
}

func TestVirtualizationStartSurvivesAConsoleQueryItCannotRun(t *testing.T) {
	runner := &sshrunnertest.Fake{
		Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Stdout: linuxVM + "\n" + runningVM} },
	}
	e := testEngine(t, runner)

	if err := e.VirtualizationStart(t.Context(), "w1"); err != nil {
		t.Fatalf("a console query core cannot run must not fail the boot: %v", err)
	}
	want := sshrunner.Quote(sshrunner.Shell(refreshScript, testRoot+"/w1.json", "/run/eru/workloads/w1.json",
		testRunDir+"/cloudhypervisor/"+testVMID+"/console.sock", "4242"))
	if lines := runner.Lines(); len(lines) != 2 || lines[1] != want {
		t.Errorf("got %q, want the recorded socket path %q", lines, want)
	}
}

func TestVirtualizationStartReportsAMissingWorkload(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Code: notExistsCode} }}
	e := testEngine(t, runner)

	if err := e.VirtualizationStart(t.Context(), "w1"); !errors.Is(err, coretypes.ErrWorkloadNotExists) {
		t.Errorf("got %v, want ErrWorkloadNotExists", err)
	}
}

func TestVirtualizationStopFlags(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		want    []string
	}{
		{"the engine default is cocoon's grace period", -1, []string{testBinary, "vm", "stop", "w1"}},
		{"a zero timeout forces the stop", 0, []string{testBinary, "vm", "stop", "--force", "w1"}},
		{"a timeout is passed in seconds", 30 * time.Second, []string{testBinary, "vm", "stop", "--timeout", "30", "w1"}},
		{"a sub-second timeout rounds up to one", time.Millisecond, []string{testBinary, "vm", "stop", "--timeout", "1", "w1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &sshrunnertest.Fake{}
			e := testEngine(t, runner)

			if err := e.VirtualizationStop(t.Context(), "w1", tt.timeout); err != nil {
				t.Fatalf("stop: %v", err)
			}
			if want := sshrunner.Quote(tt.want); len(runner.Lines()) != 1 || runner.Lines()[0] != want {
				t.Errorf("got %q, want %q", runner.Lines(), want)
			}
		})
	}
}

func TestVirtualizationLifecycleCommandSequence(t *testing.T) {
	dialed := ""
	runner := &sshrunnertest.Fake{
		Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Stdout: runningVM} },
		Dialer:  chAPI(t, "/dev/pts/5", &dialed),
	}
	e := testEngine(t, runner)
	ctx := t.Context()

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
		sshrunner.Quote(sshrunner.Shell(suspendScript, testBinary, "w1", "eru-w1")),
		sshrunner.Quote(sshrunner.Shell(resumeScript, testBinary, "w1", "eru-w1")),
		sshrunner.Quote(sshrunner.Shell(refreshScript, testRoot+"/w1.json", "/run/eru/workloads/w1.json", "/dev/pts/5", "4242")),
		sshrunner.Quote(sshrunner.Shell(removeScript, testBinary, "w1", testRoot+"/w1.json", "/run/eru/workloads/w1.json", "eru-w1", "1")),
	}
	if !slices.Equal(runner.Lines(), want) {
		t.Errorf("got %q, want %q", runner.Lines(), want)
	}
	for _, step := range []string{"vm hibernate --name", "vm restore --restore-mode copy", "snapshot rm"} {
		if !strings.Contains(suspendScript+resumeScript+removeScript, step) {
			t.Errorf("the lifecycle scripts do not carry %q", step)
		}
	}
}

func TestVirtualizationRemoveReportsAMissingWorkload(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Code: notExistsCode} }}
	e := testEngine(t, runner)

	if err := e.VirtualizationRemove(t.Context(), "w1", true, false); !errors.Is(err, coretypes.ErrWorkloadNotExists) {
		t.Errorf("got %v, want ErrWorkloadNotExists", err)
	}
}

func TestVirtualizationInspectParsesTheVMRecord(t *testing.T) {
	tests := []struct {
		name    string
		stdout  string
		running bool
		ip      string
	}{
		{"a running guest with its cni address", runningVM, true, "10.22.0.5"},
		{"a stopped guest", stoppedVM, false, ""},
		{"a stale record is not running", strings.Replace(runningVM, `"state":"running"`, `"state":"stopped (stale)"`, 1), false, "10.22.0.5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Stdout: tt.stdout} }}
			e := testEngine(t, runner)

			info, err := e.VirtualizationInspect(t.Context(), "w1")
			if err != nil {
				t.Fatalf("inspect: %v", err)
			}
			if info.Running != tt.running {
				t.Errorf("got running %v, want %v", info.Running, tt.running)
			}
			if info.Networks[defaultNetwork] != tt.ip {
				t.Errorf("got networks %v, want %q on %q", info.Networks, tt.ip, defaultNetwork)
			}
			if info.Image != testImage {
				t.Errorf("got image %q, want %q", info.Image, testImage)
			}
		})
	}
}

func TestVirtualizationInspectReportsAMissingWorkload(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Code: notExistsCode} }}
	e := testEngine(t, runner)

	if _, err := e.VirtualizationInspect(t.Context(), "w1"); !errors.Is(err, coretypes.ErrWorkloadNotExists) {
		t.Errorf("got %v, want ErrWorkloadNotExists", err)
	}
}

func TestVirtualizationWaitEndsWhenTheGuestStops(t *testing.T) {
	events := `{"event":"ADDED","vm":` + runningVM + "}\n" + `{"event":"MODIFIED","vm":` + stoppedVM + "}\n"
	running := &sshrunnertest.Session{Out: events}
	runner := &sshrunnertest.Fake{Started: []*sshrunnertest.Session{running}}
	e := testEngine(t, runner)

	waited, err := e.VirtualizationWait(t.Context(), "w1", "")
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if waited.Code != 0 {
		t.Errorf("got code %d, want 0", waited.Code)
	}
	want := sshrunner.Quote(sshrunner.Shell(waitScript, testBinary, "w1"))
	if len(runner.Lines()) != 1 || runner.Lines()[0] != want {
		t.Errorf("got %q, want %q", runner.Lines(), want)
	}
	if !running.Closed() {
		t.Error("the status stream must be closed once the guest stopped")
	}
	if strings.Contains(waitScript, "-n 1") {
		t.Error("cocoon answers -n 1 with the current state and exits, so the wait must follow the stream")
	}
}

func TestVirtualizationWaitReturnsAtOnceForAGuestThatAlreadyStopped(t *testing.T) {
	runner := &sshrunnertest.Fake{Started: []*sshrunnertest.Session{{Out: `{"event":"ADDED","vm":` + stoppedVM + "}\n"}}}
	e := testEngine(t, runner)

	waited, err := e.VirtualizationWait(t.Context(), "w1", "")
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if waited.Code != 0 {
		t.Errorf("got code %d, want 0", waited.Code)
	}
}

func TestVirtualizationWaitFailsWhenTheStreamEndsEarly(t *testing.T) {
	runner := &sshrunnertest.Fake{Started: []*sshrunnertest.Session{{Out: `{"event":"ADDED","vm":` + runningVM + "}\n"}}}
	e := testEngine(t, runner)

	waited, err := e.VirtualizationWait(t.Context(), "w1", "")
	if err == nil {
		t.Fatal("a stream that ends while the guest runs must not be reported as a clean exit")
	}
	if waited.Code != -1 {
		t.Errorf("got code %d, want -1", waited.Code)
	}
}

func TestVirtualizationUpdateResourceWaitsOnCocoon(t *testing.T) {
	runner := &sshrunnertest.Fake{}
	e := testEngine(t, runner)

	err := e.VirtualizationUpdateResource(t.Context(), "w1", resourcetypes.Resources{"cpumem": {"cpu": 2.0}})
	if !errors.Is(err, coretypes.ErrEngineNotImplemented) {
		t.Errorf("got %v, want ErrEngineNotImplemented", err)
	}
	if len(runner.Lines()) != 0 {
		t.Errorf("got %q, want no command for a realloc cocoon cannot do", runner.Lines())
	}
}

func TestVirtualizationUpdateResourceTreatsARemapAsANoOp(t *testing.T) {
	runner := &sshrunnertest.Fake{}
	e := testEngine(t, runner)

	if err := e.VirtualizationUpdateResource(t.Context(), "w1", resourcetypes.Resources{"cpumem": {"cpu": 2.0, "remap": true}}); err != nil {
		t.Fatalf("a remap must not fail a vm: %v", err)
	}
	if len(runner.Lines()) != 0 {
		t.Errorf("got %q, want no round trip for a remap", runner.Lines())
	}
}
