package containerd

import (
	"slices"
	"strconv"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/docker/go-units"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestResourceSpecSharesACpuBoundFraction(t *testing.T) {
	limits := resourceSpec(&engine.VirtualizationResource{
		CPU:      map[string]int64{"3": 50, "1": 100},
		Quota:    1.5,
		Memory:   512 * units.MiB,
		NUMANode: "0",
	}, &RawArgs{PidsMax: 128}, nil)

	if limits.CPU.Cpus != "1,3" {
		t.Errorf("got %q, want %q: the cpuset is sorted", limits.CPU.Cpus, "1,3")
	}
	if limits.CPU.Mems != "0" {
		t.Errorf("got %q, want %q", limits.CPU.Mems, "0")
	}
	if limits.CPU.Quota != nil {
		t.Error("a bound cpuset runs without a quota")
	}
	if *limits.CPU.Shares != defaultCPUShare/2 {
		t.Errorf("got %d, want %d", *limits.CPU.Shares, defaultCPUShare/2)
	}
	if *limits.Memory.Limit != 512*units.MiB || *limits.Memory.Swap != 512*units.MiB {
		t.Errorf("got %d/%d, want the limit on both", *limits.Memory.Limit, *limits.Memory.Swap)
	}
	if *limits.Memory.Reservation != 256*units.MiB {
		t.Errorf("got %d, want %d: half the limit", *limits.Memory.Reservation, 256*units.MiB)
	}
	if *limits.Pids.Limit != 128 {
		t.Errorf("got %d, want 128", *limits.Pids.Limit)
	}
}

func TestResourceSpecKeepsTheQuotaThroughARemap(t *testing.T) {
	cpus := map[string]int64{}
	for i := range 8 {
		cpus[strconv.Itoa(i)] = 100
	}
	limits := resourceSpec(&engine.VirtualizationResource{CPU: cpus, Quota: 0.5, Remap: true}, &RawArgs{}, nil)

	if limits.CPU.Cpus != "0,1,2,3,4,5,6,7" {
		t.Errorf("got %q, want every cpu", limits.CPU.Cpus)
	}
	if limits.CPU.Quota == nil || *limits.CPU.Quota != 50000 {
		t.Fatalf("got %v, want 50000: a remap shares the cpus, the quota is the only throttle left", limits.CPU.Quota)
	}
	if *limits.CPU.Shares != defaultCPUShare {
		t.Errorf("got %d, want %d", *limits.CPU.Shares, defaultCPUShare)
	}
}

func TestResourceSpecKeepsAnUnboundQuota(t *testing.T) {
	limits := resourceSpec(&engine.VirtualizationResource{Quota: 2}, &RawArgs{}, nil)

	if limits.CPU.Quota == nil || *limits.CPU.Quota != 200000 {
		t.Errorf("got %v, want 200000", limits.CPU.Quota)
	}
	if *limits.CPU.Shares != defaultCPUShare {
		t.Errorf("got %d, want %d", *limits.CPU.Shares, defaultCPUShare)
	}
	if limits.Memory != nil {
		t.Error("an unset memory limit must stay unset")
	}
}

func TestResourceSpecReservationNeverFallsUnderTheMinimum(t *testing.T) {
	limits := resourceSpec(&engine.VirtualizationResource{Memory: 5 * units.MiB}, &RawArgs{}, nil)

	if *limits.Memory.Reservation != minMemory {
		t.Errorf("got %d, want %d", *limits.Memory.Reservation, int64(minMemory))
	}
}

func TestThrottlesAddressDevicesByNumber(t *testing.T) {
	limits := resourceSpec(&engine.VirtualizationResource{}, &RawArgs{}, []blockDevice{
		{Path: "/dev/sda", Major: 8, Minor: 0, Rates: []string{"100", "0", "1M", ""}},
	})

	if len(limits.BlockIO.ThrottleReadIOPSDevice) != 1 || limits.BlockIO.ThrottleReadIOPSDevice[0].Rate != 100 {
		t.Fatalf("got %+v, want one read IOPS device", limits.BlockIO.ThrottleReadIOPSDevice)
	}
	if limits.BlockIO.ThrottleReadIOPSDevice[0].Major != 8 {
		t.Errorf("got major %d, want 8", limits.BlockIO.ThrottleReadIOPSDevice[0].Major)
	}
	if len(limits.BlockIO.ThrottleWriteIOPSDevice) != 0 {
		t.Error("a zero rate sets no knob")
	}
	if len(limits.BlockIO.ThrottleReadBpsDevice) != 1 || limits.BlockIO.ThrottleReadBpsDevice[0].Rate != units.MiB {
		t.Errorf("got %+v, want a 1MiB read bandwidth", limits.BlockIO.ThrottleReadBpsDevice)
	}
}

func TestWithImageConfigTakesTheImagesEntrypointAndUser(t *testing.T) {
	spec := newTestSpec()
	spec.Process.Env = []string{"PATH=/usr/bin"}

	config := &ocispec.ImageConfig{
		Env:        []string{"PATH=/opt/bin", "MODE=prod"},
		Entrypoint: []string{"/docker-entrypoint.sh"},
		Cmd:        []string{"nginx", "-g", "daemon off;"},
		WorkingDir: "/srv",
		User:       "101:101",
	}
	if err := withImageConfig(config)(t.Context(), nil, nil, spec); err != nil {
		t.Fatalf("spec: %v", err)
	}

	want := []string{"/docker-entrypoint.sh", "nginx", "-g", "daemon off;"}
	if !slices.Equal(spec.Process.Args, want) {
		t.Errorf("got %q, want %q", spec.Process.Args, want)
	}
	if !slices.Equal(spec.Process.Env, []string{"PATH=/opt/bin", "MODE=prod"}) {
		t.Errorf("got %q, want the image env replacing the default PATH", spec.Process.Env)
	}
	if spec.Process.Cwd != "/srv" {
		t.Errorf("got %q, want /srv", spec.Process.Cwd)
	}
	if spec.Process.User.UID != 101 || spec.Process.User.GID != 101 {
		t.Errorf("got %d:%d, want 101:101", spec.Process.User.UID, spec.Process.User.GID)
	}
}

func TestWithImageConfigLeavesTheSpecAloneWhenTheImageDeclaresNothing(t *testing.T) {
	spec := newTestSpec()
	spec.Process.Args = []string{"/sbin/init"}
	spec.Process.Cwd = "/"

	if err := withImageConfig(&ocispec.ImageConfig{})(t.Context(), nil, nil, spec); err != nil {
		t.Fatalf("spec: %v", err)
	}
	if !slices.Equal(spec.Process.Args, []string{"/sbin/init"}) {
		t.Errorf("got %q, want the default spec's args", spec.Process.Args)
	}
	if spec.Process.Cwd != "/" || spec.Process.User.UID != 0 {
		t.Errorf("got %q %d, want the default spec untouched", spec.Process.Cwd, spec.Process.User.UID)
	}
}

func TestWithImageConfigKeepsTheDefaultUserForANamedOne(t *testing.T) {
	spec := newTestSpec()

	// only the node can read the image's /etc/passwd, so a named user cannot be applied
	if err := withImageConfig(&ocispec.ImageConfig{User: "nginx"})(t.Context(), nil, nil, spec); err != nil {
		t.Fatalf("spec: %v", err)
	}
	if spec.Process.User.UID != 0 {
		t.Errorf("got %d, want the spec default", spec.Process.User.UID)
	}
}

func TestNumericUser(t *testing.T) {
	tests := []struct {
		name string
		user string
		uid  uint32
		gid  uint32
		ok   bool
	}{
		{"uid and gid", "1000:1001", 1000, 1001, true},
		{"a lone uid takes the root group", "1000", 1000, 0, true},
		{"root by id", "0", 0, 0, true},
		{"root by name, the cli's default", "root", 0, 0, true},
		{"a name", "app", 0, 0, false},
		{"a named group", "1000:staff", 0, 0, false},
		{"empty", "", 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uid, gid, ok := numericUser(tt.user)
			if uid != tt.uid || gid != tt.gid || ok != tt.ok {
				t.Errorf("got %d %d %v, want %d %d %v", uid, gid, ok, tt.uid, tt.gid, tt.ok)
			}
		})
	}
}

func TestWithProcessTakesTheWorkloadOverTheImage(t *testing.T) {
	spec := newTestSpec()

	err := withProcess(&enginetypes.VirtualizationCreateOptions{
		Cmd:        []string{"/bin/app", "-v"},
		WorkingDir: "/srv",
		User:       "1000:1001",
	}, nil)(t.Context(), nil, nil, spec)
	if err != nil {
		t.Fatalf("spec: %v", err)
	}

	if !slices.Equal(spec.Process.Args, []string{"/bin/app", "-v"}) {
		t.Errorf("got %q, want the workload command", spec.Process.Args)
	}
	if spec.Process.Cwd != "/srv" {
		t.Errorf("got %q, want /srv", spec.Process.Cwd)
	}
	if spec.Process.User.UID != 1000 || spec.Process.User.GID != 1001 {
		t.Errorf("got %d:%d, want 1000:1001", spec.Process.User.UID, spec.Process.User.GID)
	}
}

func TestWithProcessKeepsTheImageEntrypoint(t *testing.T) {
	tests := []struct {
		name       string
		entrypoint []string
		cmd        []string
		want       []string
	}{
		{
			"a deploy command runs under the image's entrypoint",
			[]string{"/docker-entrypoint.sh"},
			[]string{"nginx", "-g", "daemon off;"},
			[]string{"/docker-entrypoint.sh", "nginx", "-g", "daemon off;"},
		},
		{
			"without an entrypoint the deploy command stands alone",
			nil,
			[]string{"/bin/app", "-v"},
			[]string{"/bin/app", "-v"},
		},
		{
			"without a deploy command the image's own args stand",
			[]string{"/docker-entrypoint.sh"},
			nil,
			[]string{"/docker-entrypoint.sh", "nginx"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := newTestSpec()
			config := &ocispec.ImageConfig{Entrypoint: tt.entrypoint, Cmd: []string{"nginx"}}
			if err := withImageConfig(config)(t.Context(), nil, nil, spec); err != nil {
				t.Fatalf("image config: %v", err)
			}
			opts := &enginetypes.VirtualizationCreateOptions{Cmd: tt.cmd}
			if err := withProcess(opts, config.Entrypoint)(t.Context(), nil, nil, spec); err != nil {
				t.Fatalf("spec: %v", err)
			}
			if !slices.Equal(spec.Process.Args, tt.want) {
				t.Errorf("got %q, want %q", spec.Process.Args, tt.want)
			}
		})
	}
}

func TestWithProcessTakesRootByName(t *testing.T) {
	spec := newTestSpec()

	if err := withProcess(&enginetypes.VirtualizationCreateOptions{User: rootUser}, nil)(t.Context(), nil, nil, spec); err != nil {
		t.Fatalf("spec: %v", err)
	}
	if spec.Process.User.UID != 0 || spec.Process.User.GID != 0 {
		t.Errorf("got %d:%d, want 0:0", spec.Process.User.UID, spec.Process.User.GID)
	}
}

func TestWithProcessRejectsANamedUser(t *testing.T) {
	err := withProcess(&enginetypes.VirtualizationCreateOptions{User: "app"}, nil)(t.Context(), nil, nil, newTestSpec())

	if !errors.Is(err, coretypes.ErrInvalidEngineArgs) {
		t.Errorf("got %v, want ErrInvalidEngineArgs", err)
	}
}

func TestWithProcessRunsPrivilegedAsRoot(t *testing.T) {
	spec := newTestSpec()

	if err := withProcess(&enginetypes.VirtualizationCreateOptions{User: "app", Privileged: true}, nil)(t.Context(), nil, nil, spec); err != nil {
		t.Fatalf("spec: %v", err)
	}
	if spec.Process.User.UID != 0 {
		t.Errorf("got %d, want 0", spec.Process.User.UID)
	}
}

func TestWithNetworkDropsTheNamespaceForHostNetworking(t *testing.T) {
	spec := newTestSpec()
	spec.Linux.Namespaces = []specs.LinuxNamespace{{Type: specs.NetworkNamespace}, {Type: specs.PIDNamespace}}

	opts := &enginetypes.VirtualizationCreateOptions{Networks: map[string]string{hostNetwork: ""}}
	if err := withNetwork(opts)(t.Context(), nil, nil, spec); err != nil {
		t.Fatalf("spec: %v", err)
	}
	if slices.ContainsFunc(spec.Linux.Namespaces, func(ns specs.LinuxNamespace) bool { return ns.Type == specs.NetworkNamespace }) {
		t.Error("a host-network workload keeps the node's netns")
	}
}

func TestWithNetworkKeepsNonNetSysctlsOnHostNetworking(t *testing.T) {
	spec := newTestSpec()
	spec.Linux.Namespaces = []specs.LinuxNamespace{{Type: specs.NetworkNamespace}}

	opts := &enginetypes.VirtualizationCreateOptions{
		Networks: map[string]string{hostNetwork: ""},
		Sysctl:   map[string]string{"net.core.somaxconn": "1024", "kernel.shm_rmid_forced": "1"},
	}
	if err := withNetwork(opts)(t.Context(), nil, nil, spec); err != nil {
		t.Fatalf("spec: %v", err)
	}
	if _, ok := spec.Linux.Sysctl["net.core.somaxconn"]; ok {
		t.Error("the node owns net.* on host networking")
	}
	if spec.Linux.Sysctl["kernel.shm_rmid_forced"] != "1" {
		t.Errorf("got %+v, want the namespaced sysctl kept", spec.Linux.Sysctl)
	}
}

func TestWithNetworkKeepsEverySysctlOnACNINetwork(t *testing.T) {
	spec := newTestSpec()

	opts := &enginetypes.VirtualizationCreateOptions{
		Networks: map[string]string{"eru-cni": ""},
		Sysctl:   map[string]string{"net.core.somaxconn": "1024"},
	}
	if err := withNetwork(opts)(t.Context(), nil, nil, spec); err != nil {
		t.Fatalf("spec: %v", err)
	}
	if spec.Linux.Sysctl["net.core.somaxconn"] != "1024" {
		t.Errorf("got %+v, want net.* in the workload's own netns", spec.Linux.Sysctl)
	}
}

func TestWithHooksHandTheNetnsToTheAgent(t *testing.T) {
	spec := newTestSpec()

	networks := map[string]string{"eru-cni": "10.0.0.5", "mgmt": ""}
	if err := withHooks(networks, "eru", "")(t.Context(), nil, nil, spec); err != nil {
		t.Fatalf("spec: %v", err)
	}

	want := []string{"eru-agent", "oci-hook", "--network", "eru-cni", "--network", "mgmt"}
	if len(spec.Hooks.CreateRuntime) != 1 || !slices.Equal(spec.Hooks.CreateRuntime[0].Args, want) {
		t.Errorf("got %+v, want %q", spec.Hooks.CreateRuntime, want)
	}
	if spec.Hooks.CreateRuntime[0].Path != hookBinary {
		t.Errorf("got %q, want %q", spec.Hooks.CreateRuntime[0].Path, hookBinary)
	}
	if len(spec.Hooks.Poststop) != 1 || !slices.Equal(spec.Hooks.Poststop[0].Args, want) {
		t.Errorf("got %+v, want the same argv as the attach hook", spec.Hooks.Poststop)
	}
	if spec.Annotations[namespaceAnnotation] != "eru" {
		t.Errorf("got %q, want the hook's containerd namespace", spec.Annotations[namespaceAnnotation])
	}
	if got := hookArgs(networks, "/run/eru/containerd.sock"); got[len(got)-2] != "--socket" {
		t.Errorf("got %q, want a non-default socket named for the hook", got)
	}
}

func TestNoNetworkIsHostNetworking(t *testing.T) {
	spec := newTestSpec()
	spec.Linux.Namespaces = []specs.LinuxNamespace{{Type: specs.NetworkNamespace}}

	opts := &enginetypes.VirtualizationCreateOptions{}
	if err := withNetwork(opts)(t.Context(), nil, nil, spec); err != nil {
		t.Fatalf("spec: %v", err)
	}
	if err := withHooks(opts.Networks, "eru", "")(t.Context(), nil, nil, spec); err != nil {
		t.Fatalf("hooks: %v", err)
	}

	hasNetns := slices.ContainsFunc(spec.Linux.Namespaces, func(ns specs.LinuxNamespace) bool {
		return ns.Type == specs.NetworkNamespace
	})
	if hasNetns {
		t.Error("a private netns nothing configures would have no route out, and inspect reports host")
	}
	if spec.Hooks != nil {
		t.Error("no network means no CNI hook")
	}
	if networks := workloadNetworks(map[string]string{}, "10.0.0.1"); networks[hostNetwork] != "10.0.0.1" {
		t.Errorf("got %+v, want inspect to report the node's address", networks)
	}
}

func TestWithHooksSkipHostNetworking(t *testing.T) {
	spec := newTestSpec()

	if err := withHooks(map[string]string{hostNetwork: ""}, "eru", "")(t.Context(), nil, nil, spec); err != nil {
		t.Fatalf("spec: %v", err)
	}
	if spec.Hooks != nil {
		t.Error("host networking needs no CNI hook")
	}
	if spec.Annotations[namespaceAnnotation] != "eru" {
		t.Error("the namespace annotation stands whatever the network is")
	}
}

func TestVolumeMountsExpandTheWorkloadEnvironment(t *testing.T) {
	mounts := volumeMounts([]string{"/data/$APP_NAME:/data", "/etc/conf:/etc/conf:ro", "bad"}, []string{"APP_NAME=web"})

	if len(mounts) != 2 {
		t.Fatalf("got %d mounts, want 2", len(mounts))
	}
	if mounts[0].Source != "/data/web" || mounts[0].Destination != "/data" {
		t.Errorf("got %q -> %q, want the expanded source", mounts[0].Source, mounts[0].Destination)
	}
	if !slices.Contains(mounts[1].Options, readOnlyMode) {
		t.Errorf("got %q, want a read-only mount", mounts[1].Options)
	}
}

func TestResolverMountsPreferTheWorkloadsOwnFiles(t *testing.T) {
	mounts := resolverMounts(&enginetypes.VirtualizationCreateOptions{DNS: []string{"10.0.0.1"}}, "/var/lib/eru/containerd/w1")

	if mounts[0].Source != "/var/lib/eru/containerd/w1/resolv.conf" {
		t.Errorf("got %q, want the workload's resolv.conf", mounts[0].Source)
	}
	if mounts[1].Source != "/etc/hosts" {
		t.Errorf("got %q, want the node's hosts file", mounts[1].Source)
	}
}

func TestWithDevicesEmitsRealNodesAndNarrowRules(t *testing.T) {
	spec := newTestSpec()

	err := withDevices([]nodeDevice{{
		Path: "/dev/sda", Target: "/dev/xvda", Access: "r", Type: "b", Major: 8, Minor: 0, Mode: 0o660,
	}})(t.Context(), nil, nil, spec)
	if err != nil {
		t.Fatalf("spec: %v", err)
	}

	if len(spec.Linux.Devices) != 1 {
		t.Fatalf("got %+v, want one device", spec.Linux.Devices)
	}
	device := spec.Linux.Devices[0]
	if device.Path != "/dev/xvda" || device.Type != "b" || device.Major != 8 || device.Minor != 0 {
		t.Errorf("got %+v, want the resolved block node at its target", device)
	}
	if device.FileMode == nil || *device.FileMode != 0o660 {
		t.Errorf("got %v, want the node's own mode", device.FileMode)
	}

	rules := spec.Linux.Resources.Devices
	if len(rules) != 1 {
		t.Fatalf("got %+v, want one cgroup rule", rules)
	}
	if rules[0].Type != "b" || rules[0].Major == nil || *rules[0].Major != 8 || rules[0].Access != "r" {
		t.Errorf("got %+v, want a rule naming just that device", rules[0])
	}
}

func TestWithPrivilegedAllowsEveryDeviceExplicitly(t *testing.T) {
	spec := newTestSpec()

	if err := withPrivileged(true)(t.Context(), nil, nil, spec); err != nil {
		t.Fatalf("spec: %v", err)
	}
	rules := spec.Linux.Resources.Devices
	if len(rules) != 1 || rules[0].Type != anyDeviceType || !rules[0].Allow {
		t.Errorf("got %+v, want an explicit allow-all rule", rules)
	}
}

func TestCapabilitySetAddsAndDrops(t *testing.T) {
	got := capabilitySet([]string{"CAP_CHOWN", "CAP_KILL"}, []string{"sys_admin"}, []string{"CAP_KILL"})

	want := []string{"CAP_CHOWN", "CAP_SYS_ADMIN"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRlimitsDefaultToNofile(t *testing.T) {
	if got := rlimits(nil); len(got) != 1 || got[0].Type != "RLIMIT_NOFILE" || got[0].Hard != nofileLimit {
		t.Errorf("got %+v, want the default nofile limit", got)
	}
	got := rlimits([]*units.Ulimit{{Name: "nproc", Soft: 10, Hard: 20}})
	if len(got) != 1 || got[0].Type != "RLIMIT_NPROC" || got[0].Soft != 10 {
		t.Errorf("got %+v, want RLIMIT_NPROC", got)
	}
}

func newTestSpec() *specs.Spec {
	return &specs.Spec{Process: &specs.Process{}, Linux: &specs.Linux{}}
}
