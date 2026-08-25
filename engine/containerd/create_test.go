package containerd

import (
	"syscall"
	"testing"
	"time"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/runtime/restart"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	enginetypes "github.com/projecteru2/core/engine/types"
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
	})

	if resolv != "nameserver 10.0.0.1\nnameserver 10.0.0.2\n" {
		t.Errorf("got %q, want both resolvers", resolv)
	}
	if want := "10.0.0.9\tdb\n"; hosts[len(hosts)-len(want):] != want {
		t.Errorf("got %q, want it to end with %q", hosts, want)
	}
}

func TestResolverFilesStayEmptyWithoutAnOverride(t *testing.T) {
	if resolv, hosts := resolverFiles(&enginetypes.VirtualizationCreateOptions{}); resolv != "" || hosts != "" {
		t.Errorf("got %q %q, want the node's own files", resolv, hosts)
	}
}
