package cocoon

import (
	"testing"
)

func TestScopePathKeysOnTheCocoonID(t *testing.T) {
	got := scopePath("cocoon.slice", testVMID)
	want := "/sys/fs/cgroup/cocoon.slice/vm-" + testVMID + ".scope"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRunDirIsNamedAfterTheBackend(t *testing.T) {
	tests := []struct {
		name       string
		hypervisor string
		want       string
	}{
		{"cloud hypervisor", "cloud-hypervisor", "/var/lib/cocoon/run/cloudhypervisor/" + testVMID},
		{"firecracker", "firecracker", "/var/lib/cocoon/run/firecracker/" + testVMID},
		{"unset means cloud hypervisor", "", "/var/lib/cocoon/run/cloudhypervisor/" + testVMID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := &vmRecord{ID: testVMID, Hypervisor: tt.hypervisor}
			if got := vm.runDir(testRunDir); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConsoleIsTheOneCocoonResolvedAtBoot(t *testing.T) {
	tests := []struct {
		name       string
		hypervisor string
		reported   string
		want       string
	}{
		{"the pty of a direct-boot guest", "cloud-hypervisor", testPty, testPty},
		{"a cocoon that reports none", "cloud-hypervisor", "", testRunDir + "/cloudhypervisor/" + testVMID + "/console.sock"},
		{"firecracker", "firecracker", "", testRunDir + "/firecracker/" + testVMID + "/console.sock"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := &vmRecord{ID: testVMID, Hypervisor: tt.hypervisor, ConsolePath: tt.reported}
			if got := vm.console(testRunDir); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNetworksAreKeyedByTheConflistCocoonReports(t *testing.T) {
	tests := []struct {
		name    string
		network string
		want    string
	}{
		{"the conflist cocoon picked", "eru-cni", "eru-cni"},
		{"a guest cocoon names no conflist for", "", defaultNetwork},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := &vmRecord{
				Config: vmConfig{Network: tt.network},
				NICs:   []nic{{TAP: "tap0", Network: &guestAddress{IP: "10.22.0.5", Gateway: "10.22.0.1", Prefix: 16}}},
			}
			if got := vm.networks(); got[tt.want] != "10.22.0.5" {
				t.Errorf("got %v, want the address under %q", got, tt.want)
			}
		})
	}
}

func TestGuestAddressMask(t *testing.T) {
	if got := (&guestAddress{Prefix: 20}).mask(); got != "255.255.240.0" {
		t.Errorf("got %q, want %q", got, "255.255.240.0")
	}
}
