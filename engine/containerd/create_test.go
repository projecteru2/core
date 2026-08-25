package containerd

import (
	"testing"

	"github.com/containerd/containerd/v2/core/runtime/restart"

	enginetypes "github.com/projecteru2/core/engine/types"
)

func TestContainerLabelsCarryTheRestartPolicy(t *testing.T) {
	labels, err := containerLabels(&enginetypes.VirtualizationCreateOptions{
		Name:    "app_web_abc123",
		Restart: "on-failure:3",
		Labels:  map[string]string{"ERU": "1"},
	})
	if err != nil {
		t.Fatalf("labels: %v", err)
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
		labels, err := containerLabels(&enginetypes.VirtualizationCreateOptions{Name: "app_web_abc123", Restart: policy})
		if err != nil {
			t.Fatalf("labels: %v", err)
		}
		if _, ok := labels[restart.PolicyLabel]; ok {
			t.Errorf("restart %q must not put the workload under the restart plugin", policy)
		}
	}
}

func TestContainerLabelsRejectAnUnknownPolicy(t *testing.T) {
	if _, err := containerLabels(&enginetypes.VirtualizationCreateOptions{Restart: "sometimes"}); err == nil {
		t.Error("an unsupported restart policy must be refused")
	}
}

func TestResolveDevicesReadsHexDeviceNumbers(t *testing.T) {
	devices, err := resolveDevices([]blockDevice{{Path: "/dev/sda"}, {Path: "/dev/nvme0n1"}}, "8 0\n103 1\n")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if devices[0].Major != 8 || devices[0].Minor != 0 {
		t.Errorf("got %d:%d, want 8:0", devices[0].Major, devices[0].Minor)
	}
	if devices[1].Major != 259 || devices[1].Minor != 1 {
		t.Errorf("got %d:%d, want 259:1", devices[1].Major, devices[1].Minor)
	}
}

func TestResolveDevicesRefusesAShortAnswer(t *testing.T) {
	if _, err := resolveDevices([]blockDevice{{Path: "/dev/sda"}}, "8\n"); err == nil {
		t.Error("a truncated answer must not silently become device 0:0")
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
