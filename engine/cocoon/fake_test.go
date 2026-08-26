package cocoon

import (
	"context"
	"strings"
	"testing"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

const (
	testBinary = "/usr/local/bin/cocoon"
	testRoot   = "/var/lib/eru/cocoon"
	testRunDir = "/var/lib/cocoon/run"
	testVMID   = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	testImage  = "ghcr.io/cocoonstack/cocoon/ubuntu:24.04"
	testUser   = "eru"
	testIDLen  = 32
	testPty    = "/dev/pts/3"

	storedRecord = `{"id":"w1","kind":"vm","name":"app_web_xyz","user":"` + testUser + `","nodename":"node1"}`

	linuxVM = `{"id":"` + testVMID + `","hypervisor":"cloud-hypervisor","state":"created","first_booted":false,` +
		`"config":{"cpu":2,"memory":1073741824,"image":"` + testImage + `","network":"eru-cni"},` +
		`"network_configs":[{"tap":"tap01ARZ3ND-0","network":{"ip":"10.22.0.5","gateway":"10.22.0.1","prefix":16}}]}`
	windowsVM = `{"id":"` + testVMID + `","hypervisor":"cloud-hypervisor","state":"created","first_booted":false,` +
		`"config":{"cpu":2,"memory":4294967296,"image":"win11","windows":true},` +
		`"network_configs":[{"tap":"tap01ARZ3ND-0","network":{"ip":"10.22.0.5","gateway":"10.22.0.1","prefix":16}}]}`
	runningVM = `{"id":"` + testVMID + `","hypervisor":"cloud-hypervisor","state":"running","first_booted":true,"pid":4242,"config":{"image":"` + testImage + `"},` +
		`"network_configs":[{"tap":"tap01ARZ3ND-0","network":{"ip":"10.22.0.5","gateway":"10.22.0.1","prefix":16}}]}`
	ptyVM = `{"id":"` + testVMID + `","hypervisor":"cloud-hypervisor","state":"running","first_booted":true,"pid":4242,` +
		`"console_path":"` + testPty + `","config":{"image":"` + testImage + `"}}`
	stoppedVM       = `{"id":"` + testVMID + `","state":"stopped","first_booted":true,"config":{"image":"` + testImage + `"}}`
	bootedWindowsVM = `{"id":"` + testVMID + `","hypervisor":"cloud-hypervisor","state":"running","first_booted":true,"pid":4242,` +
		`"config":{"image":"win11","windows":true},` +
		`"network_configs":[{"tap":"tap01ARZ3ND-0","network":{"ip":"10.22.0.5","gateway":"10.22.0.1","prefix":16}}]}`
)

func testEngine(t *testing.T, runner *sshrunnertest.Fake) *Engine {
	t.Helper()
	return &Engine{
		cocoon: coretypes.CocoonConfig{Binary: testBinary, Root: testRoot, RunDir: testRunDir, CgroupParent: defaultCgroupParent},
		ep:     enginetypes.NewParams("node1", Prefix+"10.0.0.1"),
		runner: runner,
		execs:  sshrunner.NewExecs(),
	}
}

func runningRecord(string) *sshrunner.Result {
	return &sshrunner.Result{Stdout: storedRecord + "\n" + runningVM}
}

func createdVM(line string) *sshrunner.Result {
	if strings.Contains(line, "'create'") {
		return &sshrunner.Result{Stdout: linuxVM}
	}
	return &sshrunner.Result{}
}

func createdVMThenCanceled(cancel context.CancelFunc) func(string) *sshrunner.Result {
	return func(line string) *sshrunner.Result {
		if strings.Contains(line, "'create'") {
			cancel()
		}
		return createdVM(line)
	}
}
