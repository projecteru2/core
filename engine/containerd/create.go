package containerd

import (
	"context"
	"encoding/json"
	"maps"
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

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
)

const (
	// logShimURI runs the agent's log-shim, which writes the task's output to journald.
	// containerd flattens the query into argv pairs, so the mode arrives as a subcommand.
	logShimURI = "binary://" + hookBinary + "?log-shim"

	mountMark  = "mount:"
	deviceMark = "device:"
	deviceBase = 16

	prepareScript = `set -e
dir=$1; resolv=$2; hosts=$3; shift 3
if [ -n "$resolv$hosts" ]; then mkdir -p "$dir"; fi
if [ -n "$resolv" ]; then printf '%s' "$resolv" > "$dir/resolv.conf"; fi
if [ -n "$hosts" ]; then printf '%s' "$hosts" > "$dir/hosts"; fi
for entry in "$@"; do
case "$entry" in
mount:*) mkdir -p "${entry#mount:}";;
device:*) stat -L -c '%t %T' "${entry#device:}" 2>/dev/null || echo "0 0";;
esac
done
`
)

// RawArgs carries containerd-specific workload options through core untouched.
type RawArgs struct {
	CapAdd  []string        `json:"cap_add"`
	CapDrop []string        `json:"cap_drop"`
	Devices []string        `json:"devices"` // host[:container[:perms]]
	Ulimits []*units.Ulimit `json:"ulimits"`
	Runtime string          `json:"runtime"`
	PidsMax int64           `json:"pids_max"` // cgroup v2 pids.max
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

	// the container id is the workload name: eru-agent reads appname, entrypoint and
	// ident straight off it, and containerd carries no name of its own
	ID := opts.Name
	if err := identifiers.Validate(ID); err != nil {
		return nil, errors.Wrapf(coretypes.ErrInvalidWorkloadName, "containerd cannot name %q", ID)
	}
	dir := workloadDir(ID)
	devices, err := e.prepareNode(ctx, opts, resource, dir)
	if err != nil {
		return nil, err
	}
	image, err := e.client.GetImage(ctx, opts.Image)
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
		client.WithImageName(opts.Image),
		client.WithNewSnapshot(ID, image),
		client.WithContainerLabels(labels),
		client.WithNewSpec(
			oci.WithDefaultSpecForPlatform(platforms.Format(e.platform)),
			withImageConfig(imageConfig),
			oci.WithEnv(opts.Env),
			withProcess(opts),
			withCapabilities(rArgs),
			withResources(resource, rArgs, devices),
			withPrivileged(opts.Privileged),
			withDevices(rArgs.Devices),
			withMounts(opts, resource, dir),
			withNetwork(opts),
			withHooks(opts.Networks, e.namespace, nonDefaultSocket(e.socket)),
		),
	}, runtimeOpts(rArgs))...)
	if err != nil {
		e.discard(ctx, dir)
		return nil, err
	}
	return &enginetypes.VirtualizationCreated{ID: created.ID(), Name: opts.Name, Labels: opts.Labels}, nil
}

// prepareNode creates the bind sources, writes the resolver files and resolves the
// block devices the IOPS knobs address; only the node can answer any of it.
func (e *Engine) prepareNode(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions, resource *engine.VirtualizationResource, dir string) ([]blockDevice, error) {
	paths := []string{}
	for _, mount := range volumeMounts(resource.Volumes, opts.Env) {
		paths = append(paths, mountMark+mount.Source)
	}
	devices := make([]blockDevice, 0, len(resource.IOPSOptions))
	for _, path := range slices.Sorted(maps.Keys(resource.IOPSOptions)) {
		devices = append(devices, blockDevice{Path: path, Rates: strings.Split(resource.IOPSOptions[path], ":")})
		paths = append(paths, deviceMark+path)
	}

	resolv, hosts := resolverFiles(opts)
	if len(paths) == 0 && resolv == "" && hosts == "" {
		return nil, nil
	}
	res, err := e.run(ctx, sshrunner.Shell(prepareScript, slices.Concat([]string{dir, resolv, hosts}, paths)...)...)
	if err != nil {
		return nil, err
	}
	return resolveDevices(devices, res.Stdout)
}

// discard drops the node state a failed create left behind.
func (e *Engine) discard(ctx context.Context, dir string) {
	if _, err := e.run(ctx, "rm", "-rf", dir); err != nil {
		log.WithFunc("engine.containerd.discard").Errorf(ctx, err, "clean %s", dir)
	}
}

func containerLabels(opts *enginetypes.VirtualizationCreateOptions, config *ocispec.ImageConfig) (map[string]string, error) {
	labels := maps.Clone(opts.Labels)
	if labels == nil {
		labels = map[string]string{}
	}
	if config.StopSignal != "" {
		labels[client.StopSignalLabel] = config.StopSignal
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

func resolverFiles(opts *enginetypes.VirtualizationCreateOptions) (resolv, hosts string) {
	for _, server := range opts.DNS {
		resolv += "nameserver " + server + "\n"
	}
	if len(opts.Hosts) > 0 {
		hosts = "127.0.0.1\tlocalhost\n::1\tlocalhost ip6-localhost ip6-loopback\n"
		for _, entry := range opts.Hosts {
			if name, addr, ok := strings.Cut(entry, ":"); ok {
				hosts += addr + "\t" + name + "\n"
			}
		}
	}
	return resolv, hosts
}

// resolveDevices zips the node's `stat` output onto the devices it was asked about.
func resolveDevices(devices []blockDevice, out string) ([]blockDevice, error) {
	if len(devices) == 0 {
		return nil, nil
	}
	lines := strings.Fields(strings.TrimSpace(out))
	if len(lines) < 2*len(devices) {
		return nil, errors.Newf("node reported %q for %d block devices", out, len(devices))
	}
	for i := range devices {
		major, err := strconv.ParseInt(lines[2*i], deviceBase, 64)
		if err != nil {
			return nil, err
		}
		minor, err := strconv.ParseInt(lines[2*i+1], deviceBase, 64)
		if err != nil {
			return nil, err
		}
		devices[i].Major, devices[i].Minor = major, minor
	}
	return devices, nil
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
