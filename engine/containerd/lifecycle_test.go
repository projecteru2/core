package containerd

import (
	"context"
	"maps"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/containerd/containerd/api/services/tasks/v1"
	tasktypes "github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/core/runtime/restart"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs/pkg/errgrpc"
	"github.com/containerd/typeurl/v2"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	coretypes "github.com/projecteru2/core/types"
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

func TestStopTreatsAVanishedTaskAsStopped(t *testing.T) {
	gone := errgrpc.ToNative(status.Errorf(codes.NotFound, "task w1 not found"))
	if err := nilIfGone(gone); err != nil {
		t.Errorf("got %v, want a task the restart plugin reaped first counted as stopped", err)
	}
	if err := nilIfGone(coretypes.ErrMockError); !errors.Is(err, coretypes.ErrMockError) {
		t.Errorf("got %v, want a real failure preserved", err)
	}
}

func TestRemoveKeepsRestartStateWhenRunningWorkloadIsRejected(t *testing.T) {
	store := &trackingContainerStore{record: containers.Container{
		ID: "w1",
		Labels: map[string]string{
			restart.PolicyLabel: "always",
			restart.StatusLabel: string(client.Running),
		},
	}}
	runtimeClient, err := client.New("", client.WithServices(
		client.WithContainerStore(store),
		client.WithTaskClient(&runningTaskClient{}),
	))
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	e := newEngine(&Engine{client: runtimeClient})

	err = e.VirtualizationRemove(t.Context(), "w1", true, false)
	if !errors.Is(err, coretypes.ErrInvaildWorkloadOps) {
		t.Fatalf("got %v, want a running workload rejected", err)
	}
	if got := store.record.Labels[restart.StatusLabel]; got != string(client.Running) {
		t.Errorf("got restart status %q, want running", got)
	}
}

func TestUpdateSpeaksOneWordForAVanishedTask(t *testing.T) {
	gone := errgrpc.ToNative(status.Errorf(codes.NotFound, "task w1 not found"))
	if err := notExistsIfGone(gone); !errors.Is(err, coretypes.ErrWorkloadNotExists) {
		t.Errorf("got %v, want a task that vanished mid-update reported as not exists", err)
	}
	if err := notExistsIfGone(coretypes.ErrMockError); !errors.Is(err, coretypes.ErrMockError) {
		t.Errorf("got %v, want a real failure preserved", err)
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

type trackingContainerStore struct {
	containers.Store
	record containers.Container
}

func (s *trackingContainerStore) Get(context.Context, string) (containers.Container, error) {
	return s.record, nil
}

func (s *trackingContainerStore) Update(_ context.Context, record containers.Container, _ ...string) (containers.Container, error) {
	maps.Copy(s.record.Labels, record.Labels)
	return s.record, nil
}

type runningTaskClient struct {
	tasks.TasksClient
}

func (c *runningTaskClient) Get(context.Context, *tasks.GetRequest, ...grpc.CallOption) (*tasks.GetResponse, error) {
	return &tasks.GetResponse{Process: &tasktypes.Process{ID: "w1", Status: tasktypes.Status_RUNNING}}, nil
}
