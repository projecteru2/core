package containerd

import (
	"context"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/docker/go-units"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	corecluster "github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	minMemory       = 4 * units.MiB
	defaultCPUShare = 1024
	hostNetwork     = "host"
	readOnlyMode    = "ro"
	rootUser        = "root"
	bindType        = "bind"
	bindOption      = "rbind"

	// hookBinary runs CNI in the node's netns; core has none.
	hookBinary  = "/usr/local/bin/eru-agent"
	hookCommand = "oci-hook"
	// namespaceAnnotation is how the hook learns its containerd namespace: an OCI
	// hook is handed the runtime state, never the container's labels.
	namespaceAnnotation = "eru.namespace"

	nofileLimit = 65535
)

var (
	privilegedCaps = []string{
		"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_DAC_READ_SEARCH", "CAP_FOWNER", "CAP_FSETID",
		"CAP_KILL", "CAP_SETGID", "CAP_SETUID", "CAP_SETPCAP", "CAP_LINUX_IMMUTABLE",
		"CAP_NET_BIND_SERVICE", "CAP_NET_BROADCAST", "CAP_NET_ADMIN", "CAP_NET_RAW",
		"CAP_IPC_LOCK", "CAP_IPC_OWNER", "CAP_SYS_MODULE", "CAP_SYS_RAWIO", "CAP_SYS_CHROOT",
		"CAP_SYS_PTRACE", "CAP_SYS_PACCT", "CAP_SYS_ADMIN", "CAP_SYS_BOOT", "CAP_SYS_NICE",
		"CAP_SYS_RESOURCE", "CAP_SYS_TIME", "CAP_SYS_TTY_CONFIG", "CAP_MKNOD", "CAP_LEASE",
		"CAP_AUDIT_WRITE", "CAP_AUDIT_CONTROL", "CAP_SETFCAP", "CAP_MAC_OVERRIDE", "CAP_MAC_ADMIN",
		"CAP_SYSLOG", "CAP_WAKE_ALARM", "CAP_BLOCK_SUSPEND", "CAP_AUDIT_READ",
	}

	throttleSetters = [...]func(*specs.LinuxBlockIO, specs.LinuxThrottleDevice){
		func(b *specs.LinuxBlockIO, d specs.LinuxThrottleDevice) {
			b.ThrottleReadIOPSDevice = append(b.ThrottleReadIOPSDevice, d)
		},
		func(b *specs.LinuxBlockIO, d specs.LinuxThrottleDevice) {
			b.ThrottleWriteIOPSDevice = append(b.ThrottleWriteIOPSDevice, d)
		},
		func(b *specs.LinuxBlockIO, d specs.LinuxThrottleDevice) {
			b.ThrottleReadBpsDevice = append(b.ThrottleReadBpsDevice, d)
		},
		func(b *specs.LinuxBlockIO, d specs.LinuxThrottleDevice) {
			b.ThrottleWriteBpsDevice = append(b.ThrottleWriteBpsDevice, d)
		},
	}
)

// blockDevice is a node-resolved block device the IOPS knobs address.
type blockDevice struct {
	Path  string
	Major int64
	Minor int64
	Rates []string
}

// withImageConfig applies what the image declares. containerd's own oci.WithImageConfig cannot
// be used: it resolves users by temp-mounting the rootfs on the client, and core is not the node.
func withImageConfig(config *ocispec.ImageConfig) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, container *containers.Container, spec *specs.Spec) error {
		if err := oci.WithEnv(config.Env)(ctx, client, container, spec); err != nil {
			return err
		}
		if args := slices.Concat(config.Entrypoint, config.Cmd); len(args) > 0 {
			spec.Process.Args = args
		}
		if config.WorkingDir != "" {
			spec.Process.Cwd = config.WorkingDir
		}
		uid, gid, numeric := numericUser(config.User)
		switch {
		case numeric:
			spec.Process.User = specs.User{UID: uid, GID: gid}
		case config.User != "":
			log.WithFunc("engine.containerd.withImageConfig").
				Warnf(ctx, "image user %q is not numeric and only the node can resolve it, the workload keeps the spec default", config.User)
		}
		return nil
	}
}

// withProcess applies the workload's own command, user and working directory over the image's.
func withProcess(opts *enginetypes.VirtualizationCreateOptions, entrypoint []string) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, spec *specs.Spec) error {
		if len(opts.Cmd) > 0 {
			spec.Process.Args = slices.Concat(entrypoint, opts.Cmd)
		}
		if opts.WorkingDir != "" {
			spec.Process.Cwd = opts.WorkingDir
		}
		spec.Process.Terminal = opts.Stdin
		user := opts.User
		if opts.Privileged {
			user = "0"
		}
		return applyUser(spec, user)
	}
}

// withResources maps eru's knobs onto the cgroup v2 fields runc writes.
func withResources(resource *engine.VirtualizationResource, rArgs *RawArgs, devices []blockDevice) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, spec *specs.Spec) error {
		if spec.Linux == nil {
			spec.Linux = &specs.Linux{}
		}
		spec.Linux.Resources = resourceSpec(resource, rArgs, devices)
		spec.Process.Rlimits = rlimits(rArgs.Ulimits)
		return nil
	}
}

// withMounts binds the workload's volumes and the node's resolver files into the container.
func withMounts(opts *enginetypes.VirtualizationCreateOptions, resource *engine.VirtualizationResource, dir string) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, spec *specs.Spec) error {
		spec.Mounts = slices.Concat(spec.Mounts, volumeMounts(resource.Volumes, opts.Env), resolverMounts(opts, dir))
		return nil
	}
}

// withNetwork drops the network namespace for a host-network workload — its absence is what
// the agent and CNI read as host networking; a CNI workload keeps the private one.
func withNetwork(opts *enginetypes.VirtualizationCreateOptions) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, container *containers.Container, spec *specs.Spec) error {
		if spec.Linux == nil {
			spec.Linux = &specs.Linux{}
		}
		if _, host := opts.Networks[hostNetwork]; host {
			return oci.WithHostNamespace(specs.NetworkNamespace)(ctx, client, container, spec)
		}
		if len(opts.Sysctl) > 0 {
			maps.Copy(ensureSysctl(spec), opts.Sysctl)
		}
		return nil
	}
}

// withHooks hands the netns to eru-agent, which is where CNI can run. Both hooks carry the
// same argv; the hook tells an attach from a detach by the runtime state on its stdin.
func withHooks(networks map[string]string, namespace, socket string) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, spec *specs.Spec) error {
		if spec.Annotations == nil {
			spec.Annotations = map[string]string{}
		}
		spec.Annotations[namespaceAnnotation] = namespace
		if _, host := networks[hostNetwork]; host || len(networks) == 0 {
			return nil
		}
		hook := specs.Hook{Path: hookBinary, Args: hookArgs(networks, socket)}
		spec.Hooks = &specs.Hooks{CreateRuntime: []specs.Hook{hook}, Poststop: []specs.Hook{hook}}
		return nil
	}
}

// withPrivileged grants every capability and unmasks the kernel paths runc hides by default.
func withPrivileged(privileged bool) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, spec *specs.Spec) error {
		if !privileged {
			return nil
		}
		spec.Process.Capabilities = &specs.LinuxCapabilities{
			Bounding:  privilegedCaps,
			Effective: privilegedCaps,
			Permitted: privilegedCaps,
		}
		if spec.Linux != nil {
			spec.Linux.ReadonlyPaths = nil
			spec.Linux.MaskedPaths = nil
			spec.Linux.Resources = allowAllDevices(spec.Linux.Resources)
		}
		return nil
	}
}

// withCapabilities applies the raw args on top of the image's default capability set.
func withCapabilities(rArgs *RawArgs) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, spec *specs.Spec) error {
		if len(rArgs.CapAdd) == 0 && len(rArgs.CapDrop) == 0 {
			return nil
		}
		if spec.Process.Capabilities == nil {
			spec.Process.Capabilities = &specs.LinuxCapabilities{}
		}
		caps := spec.Process.Capabilities
		for _, set := range []*[]string{&caps.Bounding, &caps.Effective, &caps.Permitted} {
			*set = capabilitySet(*set, rArgs.CapAdd, rArgs.CapDrop)
		}
		return nil
	}
}

// withDevices exposes host devices as <host>[:<container>[:<perms>]].
func withDevices(devices []string) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, spec *specs.Spec) error {
		for _, device := range devices {
			parts := strings.Split(device, ":")
			path := parts[0]
			if path == "" {
				continue
			}
			target := path
			if len(parts) > 1 && parts[1] != "" {
				target = parts[1]
			}
			access := "rwm"
			if len(parts) > 2 && parts[2] != "" {
				access = parts[2]
			}
			spec.Linux.Devices = append(spec.Linux.Devices, specs.LinuxDevice{Path: target, Type: "c"})
			spec.Linux.Resources = allowDevice(spec.Linux.Resources, access)
		}
		return nil
	}
}

func resourceSpec(resource *engine.VirtualizationResource, rArgs *RawArgs, devices []blockDevice) *specs.LinuxResources {
	limits := &specs.LinuxResources{CPU: &specs.LinuxCPU{}}
	shares := uint64(defaultCPUShare)
	period := uint64(corecluster.CPUPeriodBase)
	limits.CPU.Period = &period

	if resource.Quota > 0 {
		quota := int64(resource.Quota * float64(corecluster.CPUPeriodBase))
		limits.CPU.Quota = &quota
	}
	if len(resource.CPU) > 0 {
		limits.CPU.Cpus = strings.Join(slices.Sorted(maps.Keys(resource.CPU)), ",")
		limits.CPU.Mems = resource.NUMANode
		if !resource.Remap {
			limits.CPU.Quota = nil
			if _, fraction := math.Modf(resource.Quota); fraction > 0 {
				shares = uint64(math.Round(defaultCPUShare * fraction))
			}
		}
	}
	limits.CPU.Shares = &shares

	if resource.Memory > 0 {
		reservation := max(resource.Memory/2, minMemory)
		limits.Memory = &specs.LinuxMemory{
			Limit:       &resource.Memory,
			Swap:        &resource.Memory,
			Reservation: &reservation,
		}
	}
	if rArgs.PidsMax > 0 {
		limits.Pids = &specs.LinuxPids{Limit: &rArgs.PidsMax}
	}
	if blockIO := throttles(devices); blockIO != nil {
		limits.BlockIO = blockIO
	}
	return limits
}

func throttles(devices []blockDevice) *specs.LinuxBlockIO {
	blockIO := &specs.LinuxBlockIO{}
	empty := true
	for _, device := range devices {
		for i, setter := range throttleSetters {
			if i >= len(device.Rates) {
				break
			}
			rate := parseRate(device.Rates[i])
			if rate == 0 {
				continue
			}
			throttle := specs.LinuxThrottleDevice{Rate: rate}
			throttle.Major, throttle.Minor = device.Major, device.Minor
			setter(blockIO, throttle)
			empty = false
		}
	}
	if empty {
		return nil
	}
	return blockIO
}

func volumeMounts(volumes, env []string) []specs.Mount {
	mounts := []specs.Mount{}
	for _, volume := range volumes {
		parts := strings.Split(expandEnv(volume, env), ":")
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			continue
		}
		mode := "rw"
		if len(parts) > 2 && parts[2] == readOnlyMode {
			mode = readOnlyMode
		}
		mounts = append(mounts, specs.Mount{
			Type:        bindType,
			Source:      parts[0],
			Destination: parts[1],
			Options:     []string{bindOption, mode},
		})
	}
	return mounts
}

// resolverMounts give the container a resolver: containerd writes neither file itself.
func resolverMounts(opts *enginetypes.VirtualizationCreateOptions, dir string) []specs.Mount {
	resolv, hosts := "/etc/resolv.conf", "/etc/hosts"
	if len(opts.DNS) > 0 {
		resolv = filepath.Join(dir, "resolv.conf")
	}
	if len(opts.Hosts) > 0 {
		hosts = filepath.Join(dir, "hosts")
	}
	return []specs.Mount{
		{Type: bindType, Source: resolv, Destination: "/etc/resolv.conf", Options: []string{bindOption, readOnlyMode}},
		{Type: bindType, Source: hosts, Destination: "/etc/hosts", Options: []string{bindOption, readOnlyMode}},
	}
}

func hookArgs(networks map[string]string, socket string) []string {
	args := []string{filepath.Base(hookBinary), hookCommand}
	for _, name := range slices.Sorted(maps.Keys(networks)) {
		args = append(args, "--network", name)
	}
	if socket != "" {
		args = append(args, "--socket", socket)
	}
	return args
}

// applyUser takes uid[:gid]; only the node can turn a name into an id.
func applyUser(spec *specs.Spec, user string) error {
	if user == "" {
		return nil
	}
	uid, gid, ok := numericUser(user)
	if !ok {
		return errors.Wrapf(coretypes.ErrInvalidEngineArgs, "user %q must be numeric on a containerd node", user)
	}
	spec.Process.User = specs.User{UID: uid, GID: gid}
	return nil
}

// numericUser parses uid[:gid]; an unnamed group is root, since the passwd entry that would
// carry the user's own group lives in the image.
func numericUser(user string) (uid, gid uint32, ok bool) {
	switch user {
	case "":
		return 0, 0, false
	case rootUser:
		return 0, 0, true
	}
	name, group, _ := strings.Cut(user, ":")
	parsedUID, err := strconv.ParseUint(name, 10, 32)
	if err != nil {
		return 0, 0, false
	}
	parsedGID := uint64(0)
	if group != "" {
		if parsedGID, err = strconv.ParseUint(group, 10, 32); err != nil {
			return 0, 0, false
		}
	}
	return uint32(parsedUID), uint32(parsedGID), true
}

func capabilitySet(current, add, drop []string) []string {
	set := slices.Clone(current)
	for _, name := range add {
		if capability := normalizeCapability(name); !slices.Contains(set, capability) {
			set = append(set, capability)
		}
	}
	for _, name := range drop {
		capability := normalizeCapability(name)
		set = slices.DeleteFunc(set, func(held string) bool { return held == capability })
	}
	return set
}

func normalizeCapability(name string) string {
	name = strings.ToUpper(name)
	if strings.HasPrefix(name, "CAP_") {
		return name
	}
	return "CAP_" + name
}

func rlimits(ulimits []*units.Ulimit) []specs.POSIXRlimit {
	if len(ulimits) == 0 {
		return []specs.POSIXRlimit{{Type: "RLIMIT_NOFILE", Soft: nofileLimit, Hard: nofileLimit}}
	}
	limits := make([]specs.POSIXRlimit, 0, len(ulimits))
	for _, ulimit := range ulimits {
		limits = append(limits, specs.POSIXRlimit{
			Type: "RLIMIT_" + strings.ToUpper(ulimit.Name),
			Soft: uint64(ulimit.Soft), //nolint:gosec // a negative rlimit is rejected by units.ParseUlimit
			Hard: uint64(ulimit.Hard), //nolint:gosec // a negative rlimit is rejected by units.ParseUlimit
		})
	}
	return limits
}

func ensureSysctl(spec *specs.Spec) map[string]string {
	if spec.Linux.Sysctl == nil {
		spec.Linux.Sysctl = map[string]string{}
	}
	return spec.Linux.Sysctl
}

func allowAllDevices(limits *specs.LinuxResources) *specs.LinuxResources {
	return allowDevice(limits, "rwm")
}

func allowDevice(limits *specs.LinuxResources, access string) *specs.LinuxResources {
	if limits == nil {
		limits = &specs.LinuxResources{}
	}
	limits.Devices = append(limits.Devices, specs.LinuxDeviceCgroup{Allow: true, Access: access})
	return limits
}

func expandEnv(value string, env []string) string {
	lookup := make(map[string]string, len(env))
	for _, entry := range env {
		if key, held, ok := strings.Cut(entry, "="); ok {
			lookup[key] = held
		}
	}
	return os.Expand(value, func(key string) string { return lookup[key] })
}

func parseRate(rate string) uint64 {
	parsed, err := utils.ParseRAMInHuman(rate)
	if err != nil || parsed < 0 {
		return 0
	}
	return uint64(parsed)
}
