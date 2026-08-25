package containerd

import (
	"context"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/core/runtime/restart"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/typeurl/v2"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestContainerLabelsCarryTheRestartPolicy(t *testing.T) {
	labels, err := containerLabels(&enginetypes.VirtualizationCreateOptions{
		Name:    "app_web_abc123",
		Restart: "on-failure:3",
		Labels:  map[string]string{"ERU": "1"},
	}, &ocispec.ImageConfig{StopSignal: "SIGQUIT"})
	if err != nil {
		t.Fatalf("labels: %v", err)
	}

	if labels[client.StopSignalLabel] != "SIGQUIT" {
		t.Errorf("got %q, want the image's stop signal", labels[client.StopSignalLabel])
	}
	if labels["ERU"] != "1" {
		t.Error("core's own labels must survive")
	}
	if labels[restart.PolicyLabel] != "on-failure:3" {
		t.Errorf("got %q, want on-failure:3", labels[restart.PolicyLabel])
	}
	if labels[restart.LogURILabel] != logShimURI {
		t.Errorf("got %q, want %q", labels[restart.LogURILabel], logShimURI)
	}
}

func TestContainerLabelsLeaveAnUnmanagedWorkloadAlone(t *testing.T) {
	for _, policy := range []string{"", "no"} {
		labels, err := containerLabels(&enginetypes.VirtualizationCreateOptions{Name: "app_web_abc123", Restart: policy}, &ocispec.ImageConfig{})
		if err != nil {
			t.Fatalf("labels: %v", err)
		}
		if _, ok := labels[restart.PolicyLabel]; ok {
			t.Errorf("restart %q must not put the workload under the restart plugin", policy)
		}
	}
}

func TestContainerLabelsRejectAnUnknownPolicy(t *testing.T) {
	if _, err := containerLabels(&enginetypes.VirtualizationCreateOptions{Restart: "sometimes"}, &ocispec.ImageConfig{}); err == nil {
		t.Error("an unsupported restart policy must be refused")
	}
}

func TestStopSignalFallsBackToSigterm(t *testing.T) {
	if got := stopSignal(map[string]string{}); got != syscall.SIGTERM {
		t.Errorf("got %v, want SIGTERM", got)
	}
	if got := stopSignal(map[string]string{client.StopSignalLabel: "SIGQUIT"}); got != syscall.SIGQUIT {
		t.Errorf("got %v, want SIGQUIT", got)
	}
	if got := stopSignal(map[string]string{client.StopSignalLabel: "SIGNOPE"}); got != syscall.SIGTERM {
		t.Errorf("got %v, want SIGTERM for an unparsable label", got)
	}
}

func TestStopContractMapsCoresThreeCases(t *testing.T) {
	e := testEngine(t, &sshrunnertest.Fake{})
	image := syscall.SIGQUIT

	tests := []struct {
		name   string
		asked  time.Duration
		grace  time.Duration
		signal syscall.Signal
	}{
		{"a plain stop takes the engine's default", -1, defaultStopTimeout, image},
		{"a forced stop kills at once", 0, 0, syscall.SIGKILL},
		{"an explicit grace period is honoured", 5 * time.Second, 5 * time.Second, image},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grace := e.gracePeriod(tt.asked)
			if grace != tt.grace {
				t.Errorf("got %v, want %v", grace, tt.grace)
			}
			if got := killSignal(image, grace); got != tt.signal {
				t.Errorf("got %v, want %v", got, tt.signal)
			}
		})
	}
}

func TestGracePeriodPrefersTheConfiguredTimeout(t *testing.T) {
	e := testEngine(t, &sshrunnertest.Fake{})
	e.config.Containerd.StopTimeout = 30 * time.Second

	if got := e.gracePeriod(-1); got != 30*time.Second {
		t.Errorf("got %v, want the configured 30s", got)
	}
}

func TestParseDeviceStatsReadsHexModeAndNumbers(t *testing.T) {
	stats, err := parseDeviceStats("61b0 8 0\n21b6 103 1\n", 2)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if stats[0].Major != 8 || stats[0].Minor != 0 || deviceType(stats[0].Mode) != "b" {
		t.Errorf("got %+v, want block device 8:0", stats[0])
	}
	if stats[1].Major != 259 || stats[1].Minor != 1 || deviceType(stats[1].Mode) != "c" {
		t.Errorf("got %+v, want char device 259:1", stats[1])
	}
	if stats[0].Perm() != 0o660 {
		t.Errorf("got %o, want 0660", stats[0].Perm())
	}
}

func TestParseDeviceStatsRefusesAShortAnswer(t *testing.T) {
	if _, err := parseDeviceStats("8 0\n", 1); err == nil {
		t.Error("a truncated answer must not silently become device 0:0")
	}
}

func TestDeviceTypeRejectsWhatIsNoDeviceNode(t *testing.T) {
	if got := deviceType(0o100644); got != "" {
		t.Errorf("got %q, want no type for a regular file", got)
	}
}

func TestRequestedDevicesTakeTheirTargetAndAccess(t *testing.T) {
	nodes, marks := requestedDevices([]string{"/dev/fuse", "/dev/sda:/dev/xvda:r", ""})

	if len(nodes) != 2 || len(marks) != 2 {
		t.Fatalf("got %d nodes and %d marks, want 2 of each", len(nodes), len(marks))
	}
	if nodes[0].Target != "/dev/fuse" || nodes[0].Access != defaultDeviceAccess {
		t.Errorf("got %+v, want the path as its own target", nodes[0])
	}
	if nodes[1].Target != "/dev/xvda" || nodes[1].Access != "r" {
		t.Errorf("got %+v, want the requested target and access", nodes[1])
	}
	if marks[1] != deviceMark+"/dev/sda" {
		t.Errorf("got %q, want the host path stat'ed", marks[1])
	}
}

func TestResolverFilesRenderWhatCoreWasAsked(t *testing.T) {
	resolv, hosts := resolverFiles(&enginetypes.VirtualizationCreateOptions{
		DNS:   []string{"10.0.0.1", "10.0.0.2"},
		Hosts: []string{"db:10.0.0.9"},
	}, "app_web_abc123")

	if resolv != "nameserver 10.0.0.1\nnameserver 10.0.0.2\n" {
		t.Errorf("got %q, want both resolvers", resolv)
	}
	if want := "10.0.0.9\tdb\n"; hosts[len(hosts)-len(want):] != want {
		t.Errorf("got %q, want it to end with %q", hosts, want)
	}
}

func TestResolverFilesAlwaysResolveTheWorkloadsOwnHostname(t *testing.T) {
	resolv, hosts := resolverFiles(&enginetypes.VirtualizationCreateOptions{}, "app_web_abc123")

	if resolv != "" {
		t.Errorf("got %q, want the node's own resolver", resolv)
	}
	if !strings.Contains(hosts, "127.0.1.1\tapp_web_abc123\n") {
		t.Errorf("got %q, want the hostname to resolve", hosts)
	}
	if !strings.Contains(hosts, "127.0.0.1\tlocalhost\n") {
		t.Errorf("got %q, want the localhost preamble", hosts)
	}
}

func TestANamedImageUserIsReadOffTheWorkloadsOwnSnapshot(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result {
		return &sshrunner.Result{Stdout: "root:x:0:0::/root:/bin/sh\nmemcache:x:11211:11211::/home/memcache:/bin/sh\n" +
			"---\nmemcache:x:11211:\nadm:x:27:memcache\n"}
	}}
	container := &fakeContainer{spec: &oci.Spec{Process: &specs.Process{}}}

	if err := testEngine(t, runner).applyImageUser(t.Context(), container, "app_web_abc123", "memcache"); err != nil {
		t.Fatalf("user: %v", err)
	}

	user := container.spec.Process.User
	if user.UID != 11211 || user.GID != 11211 {
		t.Errorf("got %d:%d, want 11211:11211", user.UID, user.GID)
	}
	if !slices.Equal(user.AdditionalGids, []uint32{27}) {
		t.Errorf("got %v, want the groups the image lists memcache in", user.AdditionalGids)
	}
	want := "'app_web_abc123' '" + workloadRoot + "/app_web_abc123/" + snapshotMount + "'"
	if line := runner.Lines()[0]; !strings.HasSuffix(line, want) {
		t.Errorf("got %q, want the lookup to mount the snapshot the container owns", line)
	}
}

func TestADefaultDeployStillRunsAsTheImagesUser(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result {
		return &sshrunner.Result{Stdout: "memcache:x:11211:11211::/home/memcache:/bin/sh\n---\n"}
	}}
	container := &fakeContainer{spec: &oci.Spec{Process: &specs.Process{}}}
	opts := &enginetypes.VirtualizationCreateOptions{User: cliDefaultUser}

	user := runAsUser(opts, &ocispec.ImageConfig{User: "memcache"})
	if err := testEngine(t, runner).applyImageUser(t.Context(), container, "app_web_abc123", user); err != nil {
		t.Fatalf("user: %v", err)
	}

	if container.spec.Process.User.UID != 11211 {
		t.Errorf("got %d, want 11211: the cli's default --user root is not a request to run as root", container.spec.Process.User.UID)
	}
}

func TestANumericImageUserNeedsNoSnapshot(t *testing.T) {
	for _, user := range []string{"", "root", "11211", "11211:11211"} {
		t.Run(user, func(t *testing.T) {
			runner := &sshrunnertest.Fake{}
			container := &fakeContainer{}

			if err := testEngine(t, runner).applyImageUser(t.Context(), container, "app_web_abc123", user); err != nil {
				t.Fatalf("user: %v", err)
			}
			if lines := runner.Lines(); len(lines) != 0 {
				t.Errorf("got %q, want the create's own spec to carry a numeric user", lines)
			}
			if container.updates != 0 {
				t.Error("a numeric user costs no second round trip")
			}
		})
	}
}

func TestAnImageUserTheSnapshotDoesNotHaveFailsTheCreate(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result {
		return &sshrunner.Result{Stdout: "root:x:0:0::/root:/bin/sh\n---\n"}
	}}
	container := &fakeContainer{}

	err := testEngine(t, runner).applyImageUser(t.Context(), container, "app_web_abc123", "memcache")

	if !errors.Is(err, coretypes.ErrInvalidEngineArgs) {
		t.Errorf("got %v, want ErrInvalidEngineArgs", err)
	}
	if container.updates != 0 {
		t.Error("a workload must not silently keep root when the image named a user")
	}
}

func TestTheUserLookupScriptFailsLoudly(t *testing.T) {
	if !strings.Contains(userLookupScript, "mounts=$(") {
		t.Error("a command substitution inside eval hides the mount's exit status")
	}
	for _, read := range []string{"\ncat \"$dir/etc/passwd\"\n", "\ncat \"$dir/etc/group\"\n"} {
		if !strings.Contains(userLookupScript, read) {
			t.Errorf("%q must fail the script, not hand back an empty passwd", read)
		}
	}
}

type fakeContainer struct {
	spec    *oci.Spec
	updates int
}

func (f *fakeContainer) Update(ctx context.Context, opts ...client.UpdateContainerOpts) error {
	f.updates++
	stored, err := typeurl.MarshalAny(f.spec)
	if err != nil {
		return err
	}
	record := containers.Container{Spec: stored}
	for _, opt := range opts {
		if err = opt(ctx, nil, &record); err != nil {
			return err
		}
	}
	return typeurl.UnmarshalTo(record.Spec, f.spec)
}
