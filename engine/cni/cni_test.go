package cni

import (
	"slices"
	"testing"
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

func TestParseReadsConflistsAndPlainConfs(t *testing.T) {
	networks, err := Parse(testConfList, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
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

func TestParseNarrowsToTheAskedNetworks(t *testing.T) {
	networks, err := Parse(testConfList, []string{"mgmt"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(networks) != 1 || networks[0].Name != "mgmt" {
		t.Errorf("got %+v, want only mgmt", networks)
	}
}

func TestParseRejectsABrokenConf(t *testing.T) {
	if _, err := Parse(`{"name": "eru-cni"`, nil); err == nil {
		t.Error("a truncated conf must not parse")
	}
}
