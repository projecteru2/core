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

func TestGuestAddressMask(t *testing.T) {
	if got := (&guestAddress{Prefix: 20}).mask(); got != "255.255.240.0" {
		t.Errorf("got %q, want %q", got, "255.255.240.0")
	}
}

func TestSplitRefIgnoresARegistryPort(t *testing.T) {
	name, tag := splitRef("hub.io:5000/ns/app:v1")
	if name != "hub.io:5000/ns/app" || tag != "v1" {
		t.Errorf("got %q %q, want %q %q", name, tag, "hub.io:5000/ns/app", "v1")
	}
}
