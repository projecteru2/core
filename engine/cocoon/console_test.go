package cocoon

import (
	"testing"

	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
)

func TestConsoleIsThePtyOfADirectBootGuest(t *testing.T) {
	dialed := ""
	e := testEngine(t, &sshrunnertest.Fake{Dialer: chAPI(t, "/dev/pts/3", &dialed)})

	console, err := e.console(t.Context(), &vmRecord{ID: testVMID, Hypervisor: "cloud-hypervisor"})
	if err != nil {
		t.Fatalf("console: %v", err)
	}
	if console != "/dev/pts/3" {
		t.Errorf("got %q, want the pty", console)
	}
	if want := testRunDir + "/cloudhypervisor/" + testVMID + "/api.sock"; dialed != want {
		t.Errorf("dialed %q, want %q", dialed, want)
	}
}

func TestConsoleFallsBackToTheSerialSocket(t *testing.T) {
	dialed := ""
	e := testEngine(t, &sshrunnertest.Fake{Dialer: chAPI(t, "", &dialed)})

	console, err := e.console(t.Context(), &vmRecord{ID: testVMID, Hypervisor: "cloud-hypervisor"})
	if err != nil {
		t.Fatalf("console: %v", err)
	}
	if want := testRunDir + "/cloudhypervisor/" + testVMID + "/console.sock"; console != want {
		t.Errorf("got %q, want %q", console, want)
	}
}

func TestConsoleOfAFirecrackerGuestIsTheSocketWithoutAQuery(t *testing.T) {
	e := testEngine(t, &sshrunnertest.Fake{})

	console, err := e.console(t.Context(), &vmRecord{ID: testVMID, Hypervisor: "firecracker"})
	if err != nil {
		t.Fatalf("console: %v", err)
	}
	if want := testRunDir + "/firecracker/" + testVMID + "/console.sock"; console != want {
		t.Errorf("got %q, want %q", console, want)
	}
}
