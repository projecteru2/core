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
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	minMemory       = 4 * units.MiB
	defaultCPUShare = 1024
	hostNetwork     = "host"
	netSysctlPrefix = "net."
	readOnlyMode    = "ro"
	rootUser        = "root"
	bindType        = "bind"
	bindOption      = "rbind"

	defaultDeviceAccess = "rwm"
	anyDeviceType       = "a"
	modeTypeMask        = 0xF000
	modeChar            = 0x2000
	modeBlock           = 0x6000
	modePermMask        = 0o777

	passwdUID    = 2
	passwdGID    = 3
	groupGID     = 2
	groupMembers = 3

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

type nodeDevice struct {
	Path   string
	Target string
	Access string
	Type   string
	Major  int64
	Minor  int64
	Mode   os.FileMode
}

type deviceStat struct {
	Mode  int64
	Major int64
	Minor int64
}

func (d deviceStat) Perm() os.FileMode {
	return os.FileMode(d.Mode & modePermMask)
}

// withImageConfig applies what the image declares, with the user already resolved.
func withImageConfig(config *ocispec.ImageConfig, user *specs.User) oci.SpecOpts {
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
		if user != nil {
			spec.Process.User = *user
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

// withNetwork drops the network namespace, which is how CNI and the agent read host networking.
func withNetwork(opts *enginetypes.VirtualizationCreateOptions) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, container *containers.Container, spec *specs.Spec) error {
		if spec.Linux == nil {
			spec.Linux = &specs.Linux{}
		}
		sysctl := opts.Sysctl
		if hostNetworking(opts.Networks) {
			sysctl = withoutNetSysctls(sysctl)
			if err := oci.WithHostNamespace(specs.NetworkNamespace)(ctx, client, container, spec); err != nil {
				return err
			}
		}
		if len(sysctl) > 0 {
			maps.Copy(ensureSysctl(spec), sysctl)
		}
		return nil
	}
}

// withHooks hands the netns to eru-agent; the hook reads attach or detach off the runtime state.
func withHooks(networks map[string]string, namespace, socket string) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, spec *specs.Spec) error {
		if spec.Annotations == nil {
			spec.Annotations = map[string]string{}
		}
		spec.Annotations[namespaceAnnotation] = namespace
		if hostNetworking(networks) {
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
func withDevices(devices []nodeDevice) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, spec *specs.Spec) error {
		for _, device := range devices {
			mode := device.Mode
			spec.Linux.Devices = append(spec.Linux.Devices, specs.LinuxDevice{
				Path:     device.Target,
				Type:     device.Type,
				Major:    device.Major,
				Minor:    device.Minor,
				FileMode: &mode,
			})
			spec.Linux.Resources = allowDevice(spec.Linux.Resources, specs.LinuxDeviceCgroup{
				Allow:  true,
				Type:   device.Type,
				Major:  &device.Major,
				Minor:  &device.Minor,
				Access: device.Access,
			})
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

// resolverMounts gives the container a resolver: containerd writes neither file itself.
func resolverMounts(opts *enginetypes.VirtualizationCreateOptions, dir string) []specs.Mount {
	resolv, hosts := "/etc/resolv.conf", filepath.Join(dir, "hosts")
	if len(opts.DNS) > 0 {
		resolv = filepath.Join(dir, "resolv.conf")
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

// numericUser parses uid[:gid]; an unnamed group is root, the image owns the passwd entry.
func lookupUser(out, user string) (*specs.User, error) {
	passwd, group, _ := strings.Cut(out, "\n---\n")
	entry, ok := passwdEntry(passwd, user)
	if !ok {
		return nil, errors.Wrap(coretypes.ErrInvalidEngineArgs, "no passwd entry")
	}
	uid, err := strconv.ParseUint(entry[passwdUID], 10, 32)
	if err != nil {
		return nil, errors.Wrap(coretypes.ErrInvalidEngineArgs, "unreadable uid")
	}
	gid, err := strconv.ParseUint(entry[passwdGID], 10, 32)
	if err != nil {
		return nil, errors.Wrap(coretypes.ErrInvalidEngineArgs, "unreadable gid")
	}
	return &specs.User{
		UID:            uint32(uid),
		GID:            uint32(gid),
		AdditionalGids: additionalGids(group, user, uint32(gid)),
	}, nil
}

func passwdEntry(passwd, user string) ([]string, bool) {
	for line := range strings.Lines(passwd) {
		fields := strings.Split(strings.TrimRight(line, "\n"), ":")
		if len(fields) > passwdGID && fields[0] == user {
			return fields, true
		}
	}
	return nil, false
}

func additionalGids(group, user string, primary uint32) []uint32 {
	gids := []uint32{}
	for line := range strings.Lines(group) {
		fields := strings.Split(strings.TrimRight(line, "\n"), ":")
		if len(fields) <= groupMembers || !slices.Contains(strings.Split(fields[groupMembers], ","), user) {
			continue
		}
		gid, err := strconv.ParseUint(fields[groupGID], 10, 32)
		if err != nil || uint32(gid) == primary {
			continue
		}
		gids = append(gids, uint32(gid))
	}
	if len(gids) == 0 {
		return nil
	}
	return gids
}

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

func hostNetworking(networks map[string]string) bool {
	_, host := networks[hostNetwork]
	return host || len(networks) == 0
}

func withoutNetSysctls(sysctl map[string]string) map[string]string {
	kept := make(map[string]string, len(sysctl))
	for key, value := range sysctl {
		if !strings.HasPrefix(key, netSysctlPrefix) {
			kept[key] = value
		}
	}
	return kept
}

func ensureSysctl(spec *specs.Spec) map[string]string {
	if spec.Linux.Sysctl == nil {
		spec.Linux.Sysctl = map[string]string{}
	}
	return spec.Linux.Sysctl
}

func allowAllDevices(limits *specs.LinuxResources) *specs.LinuxResources {
	return allowDevice(limits, specs.LinuxDeviceCgroup{Allow: true, Type: anyDeviceType, Access: defaultDeviceAccess})
}

func allowDevice(limits *specs.LinuxResources, rule specs.LinuxDeviceCgroup) *specs.LinuxResources {
	if limits == nil {
		limits = &specs.LinuxResources{}
	}
	limits.Devices = append(limits.Devices, rule)
	return limits
}

func deviceType(mode int64) string {
	switch mode & modeTypeMask {
	case modeBlock:
		return "b"
	case modeChar:
		return "c"
	default:
		return ""
	}
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
