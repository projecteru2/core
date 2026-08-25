package containerd

import (
	"testing"

	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/typeurl/v2"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func TestUpdateKeepsTheLimitsItDoesNotOwn(t *testing.T) {
	pids := int64(128)
	stored := &oci.Spec{Linux: &specs.Linux{Resources: &specs.LinuxResources{
		Pids:    &specs.LinuxPids{Limit: &pids},
		Devices: []specs.LinuxDeviceCgroup{{Allow: true, Type: "c", Access: "rwm"}},
		CPU:     &specs.LinuxCPU{Cpus: "0"},
	}}}
	record := containers.Container{Spec: mustMarshal(t, stored)}

	quota := int64(50000)
	limits := &specs.LinuxResources{CPU: &specs.LinuxCPU{Quota: &quota}}
	if err := withSpecResources(limits)(t.Context(), nil, &record); err != nil {
		t.Fatalf("update: %v", err)
	}

	updated := &oci.Spec{}
	if err := typeurl.UnmarshalTo(record.Spec, updated); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if updated.Linux.Resources.Pids == nil || *updated.Linux.Resources.Pids.Limit != 128 {
		t.Errorf("got %+v, want the pids limit an update never carries", updated.Linux.Resources.Pids)
	}
	if len(updated.Linux.Resources.Devices) != 1 {
		t.Errorf("got %+v, want the device rules to survive", updated.Linux.Resources.Devices)
	}
	if updated.Linux.Resources.CPU.Quota == nil || *updated.Linux.Resources.CPU.Quota != 50000 {
		t.Errorf("got %+v, want the new quota", updated.Linux.Resources.CPU)
	}
	if updated.Linux.Resources.CPU.Cpus != "" {
		t.Errorf("got %q, want the cpuset replaced, not merged", updated.Linux.Resources.CPU.Cpus)
	}
}

func mustMarshal(t *testing.T, spec *oci.Spec) typeurl.Any {
	t.Helper()
	any, err := typeurl.MarshalAny(spec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return any
}
