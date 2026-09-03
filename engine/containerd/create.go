package containerd

import (
	"context"
	"encoding/json"
	"maps"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/runtime/restart"
	"github.com/containerd/containerd/v2/pkg/identifiers"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/platforms"
	"github.com/docker/go-units"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
)

const (
	// containerd flattens the query into argv pairs, so log-shim arrives as the agent's subcommand
	logShimURI = "binary://" + hookBinary + "?log-shim"

	hostNameMax   = 64
	snapshotMount = "rootfs"

	stdinLabel = "eru.stdin"

	userLookupScript = snapshotMountScript + `cat "$dir/etc/passwd"
printf '%s\n' ---
cat "$dir/etc/group"
`
	mountMark    = "mount:"
	deviceMark   = "device:"
	deviceBase   = 16
	deviceFields = 3

	prepareScript = `set -e
dir=$1; resolv=$2; hosts=$3; shift 3
if [ -n "$resolv$hosts" ]; then mkdir -p "$dir"; fi
if [ -n "$resolv" ]; then printf '%s' "$resolv" > "$dir/resolv.conf"; fi
if [ -n "$hosts" ]; then printf '%s' "$hosts" > "$dir/hosts"; fi
for entry in "$@"; do
case "$entry" in
mount:*) mkdir -p "${entry#mount:}";;
device:*) stat -L -c '%f %t %T' "${entry#device:}" 2>/dev/null || echo "0 0 0";;
esac
done
`
)

var logShimURL, _ = url.Parse(logShimURI)

// RawArgs carries containerd-specific workload options through core untouched.
type RawArgs struct {
	CapAdd  []string        `json:"cap_add"`
	CapDrop []string        `json:"cap_drop"`
	Devices []string        `json:"devices"` // host[:container[:perms]]
	Ulimits []*units.Ulimit `json:"ulimits"`
	Runtime string          `json:"runtime"`
	PidsMax int64           `json:"pids_max"` // cgroup v2 pids.max
}

type containerUpdater interface {
	Update(ctx context.Context, opts ...client.UpdateContainerOpts) error
}

func (e *Engine) VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (*enginetypes.VirtualizationCreated, error) {
	logger := log.WithFunc("engine.containerd.VirtualizationCreate")
	resource := &engine.VirtualizationResource{}
	if err := resource.Decode(opts.EngineParams); err != nil {
		logger.Errorf(ctx, err, "failed to parse engine args %+v", opts.EngineParams)
		return nil, coretypes.ErrInvalidEngineArgs
	}
	if resource.Memory > 0 && resource.Memory < minMemory || resource.Memory < 0 {
		return nil, coretypes.ErrInvaildMemory
	}
	rArgs := &RawArgs{}
	if len(opts.RawArgs) > 0 {
		if err := json.Unmarshal(opts.RawArgs, rArgs); err != nil {
			return nil, err
		}
	}

	// the container id is the workload name: containerd carries no name eru-agent could read
	ID := opts.Name
	if err := identifiers.Validate(ID); err != nil {
		return nil, errors.Wrapf(coretypes.ErrInvalidWorkloadName, "containerd cannot name %q", ID)
	}
	if len(ID) > hostNameMax {
		return nil, errors.Wrapf(coretypes.ErrInvalidWorkloadName, "%q is longer than a hostname may be (%d)", ID, hostNameMax)
	}
	dir := workloadDir(ID)
	mounts := volumeMounts(resource.Volumes, opts.Env)
	throttled, devices, err := e.prepareNode(ctx, opts, mounts, resource, rArgs, dir)
	if err != nil {
		return nil, err
	}
	ref := normalizeRef(opts.Image)
	image, err := e.client.GetImage(ctx, ref)
	if err != nil {
		return nil, err
	}
	imageConfig, err := e.imageConfig(ctx, image)
	if err != nil {
		return nil, err
	}
	labels, err := containerLabels(opts, imageConfig)
	if err != nil {
		return nil, err
	}

	created, err := e.client.NewContainer(ctx, ID, slices.Concat([]client.NewContainerOpts{
		client.WithImage(image),
		client.WithImageName(ref),
		client.WithNewSnapshot(ID, image),
		client.WithContainerLabels(labels),
		client.WithNewSpec(
			oci.WithDefaultSpecForPlatform(platforms.Format(e.platform)),
			oci.WithHostname(ID),
			withImageConfig(imageConfig),
			oci.WithEnv(opts.Env),
			withProcess(opts, imageConfig.Entrypoint),
			withCapabilities(rArgs),
			withResources(resource, rArgs, throttled),
			withPrivileged(opts.Privileged),
			withDevices(devices),
			withMounts(opts, mounts, dir),
			withNetwork(opts),
			withHooks(opts.Networks, e.namespace, nonDefaultSocket(e.socket)),
		),
	}, runtimeOpts(rArgs))...)
	if err != nil {
		e.discard(ctx, dir)
		return nil, err
	}
	if err = e.applyImageUser(ctx, created, ID, runAsUser(opts, imageConfig)); err != nil {
		e.discardWorkload(ctx, created, dir)
		return nil, err
	}
	return &enginetypes.VirtualizationCreated{ID: created.ID(), Name: opts.Name, Labels: opts.Labels}, nil
}

// prepareNode asks the node for what containerd's API cannot answer about a create.
func (e *Engine) prepareNode(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions, mounts []specs.Mount, resource *engine.VirtualizationResource, rArgs *RawArgs, dir string) ([]blockDevice, []nodeDevice, error) {
	paths := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		paths = append(paths, mountMark+mount.Source)
	}
	throttled, throttleMarks := throttleDevices(resource.IOPSOptions)
	devices, deviceMarks := requestedDevices(rArgs.Devices)
	paths = slices.Concat(paths, throttleMarks, deviceMarks)

	resolv, hosts := resolverFiles(opts, filepath.Base(dir))
	if len(paths) == 0 && resolv == "" && hosts == "" {
		return nil, nil, nil
	}
	res, err := e.run(ctx, sshrunner.Shell(prepareScript, slices.Concat([]string{dir, resolv, hosts}, paths)...)...)
	if err != nil {
		return nil, nil, err
	}
	stats, err := parseDeviceStats(res.Stdout, len(throttled)+len(devices))
	if err != nil {
		return nil, nil, err
	}
	for i := range throttled {
		throttled[i].Major, throttled[i].Minor = stats[i].Major, stats[i].Minor
	}
	for i := range devices {
		stat := stats[len(throttled)+i]
		if devices[i].Type = deviceType(stat.Mode); devices[i].Type == "" {
			return nil, nil, errors.Wrapf(coretypes.ErrInvalidEngineArgs, "%s is no device node", devices[i].Path)
		}
		devices[i].Major, devices[i].Minor, devices[i].Mode = stat.Major, stat.Minor, stat.Perm()
	}
	return throttled, devices, nil
}

func (e *Engine) applyImageUser(ctx context.Context, container containerUpdater, ID, user string) error {
	if _, _, numeric := numericUser(user); user == "" || numeric {
		return nil
	}
	argv := sshrunner.Shell(userLookupScript, ctrBinary, e.socket, e.namespace, ID, filepath.Join(workloadDir(ID), snapshotMount))
	res, err := e.run(ctx, argv...)
	if err != nil {
		return err
	}
	resolved, err := lookupUser(res.Stdout, user)
	if err != nil {
		return errors.Wrapf(err, "image user %q", user)
	}
	return container.Update(ctx, withSpecUser(*resolved))
}

func (e *Engine) resolveThrottles(ctx context.Context, options map[string]string) ([]blockDevice, error) {
	devices, marks := throttleDevices(options)
	if len(devices) == 0 {
		return nil, nil
	}
	res, err := e.run(ctx, sshrunner.Shell(prepareScript, slices.Concat([]string{"", "", ""}, marks)...)...)
	if err != nil {
		return nil, err
	}
	stats, err := parseDeviceStats(res.Stdout, len(devices))
	if err != nil {
		return nil, err
	}
	for i := range devices {
		devices[i].Major, devices[i].Minor = stats[i].Major, stats[i].Minor
	}
	return devices, nil
}

// discard drops the node state a failed create left behind.
func (e *Engine) discard(ctx context.Context, dir string) {
	if _, err := e.run(ctx, "rm", "-rf", dir); err != nil {
		log.WithFunc("engine.containerd.discard").Errorf(ctx, err, "clean %s", dir)
	}
}

func (e *Engine) discardWorkload(ctx context.Context, container client.Container, dir string) {
	if err := container.Delete(ctx, client.WithSnapshotCleanup); err != nil {
		log.WithFunc("engine.containerd.discardWorkload").Errorf(ctx, err, "remove workload %s", container.ID())
	}
	e.discard(ctx, dir)
}

func containerLabels(opts *enginetypes.VirtualizationCreateOptions, config *ocispec.ImageConfig) (map[string]string, error) {
	labels := maps.Clone(opts.Labels)
	if labels == nil {
		labels = map[string]string{}
	}
	if config.StopSignal != "" {
		labels[client.StopSignalLabel] = config.StopSignal
	}
	if opts.Stdin {
		labels[stdinLabel] = "1"
	}

	policy, _, _ := strings.Cut(opts.Restart, ":")
	if policy == "" || policy == "no" {
		return labels, nil
	}
	parsed, err := restart.NewPolicy(opts.Restart)
	if err != nil {
		return nil, errors.Wrapf(coretypes.ErrInvalidEngineArgs, "restart policy %q", opts.Restart)
	}
	labels[restart.PolicyLabel] = parsed.String()
	labels[restart.LogURILabel] = logShimURI
	return labels, nil
}

func runtimeOpts(rArgs *RawArgs) []client.NewContainerOpts {
	if rArgs.Runtime == "" {
		return nil
	}
	return []client.NewContainerOpts{client.WithRuntime(rArgs.Runtime, nil)}
}

func resolverFiles(opts *enginetypes.VirtualizationCreateOptions, ID string) (resolv, hosts string) {
	for _, server := range opts.DNS {
		resolv += "nameserver " + server + "\n"
	}
	hosts = "127.0.0.1\tlocalhost\n::1\tlocalhost ip6-localhost ip6-loopback\n127.0.1.1\t" + ID + "\n"
	for _, entry := range opts.Hosts {
		if name, addr, ok := strings.Cut(entry, ":"); ok {
			hosts += addr + "\t" + name + "\n"
		}
	}
	return resolv, hosts
}

// parseDeviceStats zips the node's `stat` output onto the devices it was asked about.
func parseDeviceStats(out string, want int) ([]deviceStat, error) {
	if want == 0 {
		return nil, nil
	}
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) < deviceFields*want {
		return nil, errors.Newf("node reported %q for %d devices", out, want)
	}
	stats := make([]deviceStat, want)
	for i := range stats {
		numbers := [deviceFields]int64{}
		for j := range numbers {
			parsed, err := strconv.ParseInt(fields[deviceFields*i+j], deviceBase, 64)
			if err != nil {
				return nil, err
			}
			numbers[j] = parsed
		}
		stats[i] = deviceStat{Mode: numbers[0], Major: numbers[1], Minor: numbers[2]}
	}
	return stats, nil
}

func requestedDevices(devices []string) (nodes []nodeDevice, marks []string) {
	nodes = make([]nodeDevice, 0, len(devices))
	marks = make([]string, 0, len(devices))
	for _, device := range devices {
		parts := strings.Split(device, ":")
		if parts[0] == "" {
			continue
		}
		node := nodeDevice{Path: parts[0], Target: parts[0], Access: defaultDeviceAccess}
		if len(parts) > 1 && parts[1] != "" {
			node.Target = parts[1]
		}
		if len(parts) > 2 && parts[2] != "" {
			node.Access = parts[2]
		}
		nodes = append(nodes, node)
		marks = append(marks, deviceMark+node.Path)
	}
	return nodes, marks
}

func throttleDevices(options map[string]string) (devices []blockDevice, marks []string) {
	devices = make([]blockDevice, 0, len(options))
	marks = make([]string, 0, len(options))
	for _, path := range slices.Sorted(maps.Keys(options)) {
		devices = append(devices, blockDevice{Path: path, Rates: strings.Split(options[path], ":")})
		marks = append(marks, deviceMark+path)
	}
	return devices, marks
}

func workloadDir(ID string) string {
	return filepath.Join(workloadRoot, ID)
}

// nonDefaultSocket keeps the hook's argv to what the agent already defaults to.
func nonDefaultSocket(socket string) string {
	if socket == defaultSocket {
		return ""
	}
	return socket
}
