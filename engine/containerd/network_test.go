package containerd

import (
	"slices"
	"testing"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	coretypes "github.com/projecteru2/core/types"
)

const testConfList = `{
  "cniVersion": "1.0.0",
  "name": "eru-cni",
  "plugins": [
    {"type": "bridge", "ipam": {"type": "host-local", "ranges": [[{"subnet": "10.10.0.0/16"}]]}}
  ]
}
{"cniVersion": "1.0.0", "name": "mgmt", "type": "macvlan", "ipam": {"type": "host-local", "subnet": "192.168.1.0/24"}}
`

func TestNetworkListReadsTheCNIConfDir(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Stdout: testConfList} }}
	e := testEngine(t, runner)

	networks, err := e.NetworkList(t.Context(), nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := sshrunner.Quote(sshrunner.Shell(listNetworkScript, cniConfDir))
	if len(runner.Lines()) != 1 || runner.Lines()[0] != want {
		t.Fatalf("got %q, want %q", runner.Lines(), want)
	}
	if len(networks) != 2 {
		t.Fatalf("got %d networks, want 2", len(networks))
	}
	if networks[0].Name != "eru-cni" || !slices.Equal(networks[0].Subnets, []string{"10.10.0.0/16"}) {
		t.Errorf("got %+v, want the conflist's ranges", networks[0])
	}
	if networks[1].Name != "mgmt" || !slices.Equal(networks[1].Subnets, []string{"192.168.1.0/24"}) {
		t.Errorf("got %+v, want the plain conf's subnet", networks[1])
	}
}

func TestNetworkListNarrowsToTheAskedNetworks(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Stdout: testConfList} }}

	networks, err := testEngine(t, runner).NetworkList(t.Context(), []string{"mgmt"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(networks) != 1 || networks[0].Name != "mgmt" {
		t.Errorf("got %+v, want only mgmt", networks)
	}
}

func TestNetworkConnectIsNotACNIOperation(t *testing.T) {
	e := testEngine(t, &sshrunnertest.Fake{})

	if _, err := e.NetworkConnect(t.Context(), "eru-cni", "w1", "", ""); !errors.Is(err, coretypes.ErrEngineNotImplemented) {
		t.Errorf("got %v, want ErrEngineNotImplemented", err)
	}
	if err := e.NetworkDisconnect(t.Context(), "eru-cni", "w1", true); !errors.Is(err, coretypes.ErrEngineNotImplemented) {
		t.Errorf("got %v, want ErrEngineNotImplemented", err)
	}
}
