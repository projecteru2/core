package cocoon

import (
	"context"
	"encoding/json"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/engine/workloadmeta"
	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	osWindows   = "windows"
	formatJSON  = "json"
	volumeParts = 4

	discardTimeout = 30 * time.Second

	// reseedScript rewrites a clone's static NICs by MAC and its hostname; the guest still carries the source's.
	reseedScript = `bin=$1; vm=$2; shift 2
guest='rm -f /etc/systemd/network/10-*.network
hostnamectl set-hostname "$1" 2>/dev/null || hostname "$1"
shift
while [ $# -ge 3 ]; do
f="/etc/systemd/network/10-$(printf %s "$1" | tr -d :).network"
printf "[Match]\nMACAddress=%s\n\n[Network]\nAddress=%s\n" "$1" "$2" > "$f"
if [ -n "$3" ]; then printf "Gateway=%s\n" "$3" >> "$f"; fi
shift 3
done
systemctl restart systemd-networkd'
tries=0
until "$bin" vm exec "$vm" -- sh -c "$guest" sh "$@"; do
tries=$((tries+1))
[ "$tries" -lt 30 ] || exit 1
sleep 1
done
`

	publishRecord = `mkdir -p "$(dirname "$record")"
cp -f "$durable" "$record.tmp"
mv "$record.tmp" "$record"
`

	recordScript = `set -e
durable=$1; record=$2; body=$3
mkdir -p "$(dirname "$durable")"
printf '%s\n' "$body" > "$durable.tmp"
mv "$durable.tmp" "$durable"
` + publishRecord
)

var coreEnvKeys = []string{cluster.EnvAppName, cluster.EnvPod, cluster.EnvNodeName, cluster.EnvWorkloadSeq}

// RawArgs carries vm-specific workload options through core untouched.
type RawArgs struct {
	OS string `json:"os"` // "windows" boots a Windows guest
}

func (e *Engine) VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (*enginetypes.VirtualizationCreated, error) {
	logger := log.WithFunc("engine.cocoon.VirtualizationCreate")
	resource := &engine.VirtualizationResource{}
	if err := resource.Decode(opts.EngineParams); err != nil {
		logger.Errorf(ctx, err, "failed to parse engine args %+v", opts.EngineParams)
		return nil, coretypes.ErrInvalidEngineArgs
	}
	rArgs := &RawArgs{}
	if len(opts.RawArgs) > 0 {
		if err := json.Unmarshal(opts.RawArgs, rArgs); err != nil {
			return nil, err
		}
	}
	network, err := requestedNetwork(ctx, opts.Networks)
	if err != nil {
		return nil, err
	}
	ID := utils.RandomID()
	var argv []string
	if snapshot, ok := strings.CutPrefix(opts.Image, snapshotScheme); ok {
		logger.Debugf(ctx, "vm %s takes its cpu, memory and storage from snapshot %s", opts.Name, snapshot)
		argv, err = cloneArgv(e.cocoon.Binary, ID, snapshot, resource.Volumes, rArgs.OS == osWindows, network)
	} else {
		argv, err = createArgv(e.cocoon.Binary, ID, opts, resource, rArgs.OS == osWindows, network)
	}
	if err != nil {
		return nil, err
	}
	if unapplied := unappliedOptions(opts); len(unapplied) > 0 {
		logger.Warnf(ctx, "cocoon does not apply %s to vm %s", strings.Join(unapplied, ", "), opts.Name)
	}

	res, err := e.run(ctx, argv...)
	if err != nil {
		return nil, err
	}
	vm, err := parseVM(res.Stdout)
	if err == nil {
		err = e.record(ctx, ID, opts, vm)
	}
	if err == nil && strings.HasPrefix(opts.Image, snapshotScheme) {
		_, err = e.run(ctx, reseedArgv(e.cocoon.Binary, ID, opts.Name, vm)...)
	}
	if err != nil {
		e.discard(ctx, ID)
		return nil, err
	}
	return &enginetypes.VirtualizationCreated{ID: ID, Name: opts.Name, Labels: opts.Labels}, nil
}

// record writes the meta file, durably under the root and on tmpfs for eru-agent.
func (e *Engine) record(ctx context.Context, ID string, opts *enginetypes.VirtualizationCreateOptions, vm *vmRecord) error {
	body, err := json.Marshal(newMeta(ctx, ID, opts, vm, e.ep.Nodename, e.cocoon))
	if err != nil {
		return err
	}
	_, err = e.run(ctx, sshrunner.Shell(recordScript, durablePath(e.cocoon.Root, ID), workloadmeta.Path(ID), string(body))...)
	return err
}

// discard removes a VM whose eru record never landed; core only knows the ones that did.
func (e *Engine) discard(ctx context.Context, ID string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), discardTimeout)
	defer cancel()
	if _, err := e.run(ctx, e.vm("rm", "--force", ID)...); err != nil {
		log.WithFunc("engine.cocoon.discard").Errorf(ctx, err, "failed to remove the half-created vm %s", ID)
	}
}

func createArgv(binary, ID string, opts *enginetypes.VirtualizationCreateOptions, resource *engine.VirtualizationResource, windows bool, network string) ([]string, error) {
	argv := []string{binary, "vm", "create", "--output", formatJSON}
	if resource.Quota > 0 {
		argv = append(argv, "--cpu", strconv.Itoa(int(math.Ceil(resource.Quota))))
	}
	if resource.Memory > 0 {
		argv = append(argv, "--memory", strconv.FormatInt(resource.Memory, 10))
	}
	if resource.Storage > 0 {
		argv = append(argv, "--storage", strconv.FormatInt(resource.Storage, 10))
	}
	disks, err := dataDisks(resource.Volumes, windows)
	if err != nil {
		return nil, err
	}
	for _, disk := range disks {
		argv = append(argv, "--data-disk", disk)
	}
	if network != "" {
		argv = append(argv, "--network", network)
	}
	switch {
	case windows:
		argv = append(argv, "--windows")
	case opts.User != "":
		argv = append(argv, "--user", opts.User)
	}
	return append(argv, "--name", ID, opts.Image), nil
}

// cloneArgv boots the vm from a snapshot, which fixes its cpu, memory, storage and guest os.
func cloneArgv(binary, ID, snapshot string, volumes []string, windows bool, network string) ([]string, error) {
	if windows {
		return nil, errors.Wrap(coretypes.ErrInvalidEngineArgs, "a windows guest cannot be cloned from a snapshot")
	}
	if err := checkSnapshotName(snapshot); err != nil {
		return nil, err
	}
	argv := []string{binary, "vm", "clone", "--output", formatJSON, "--name", ID}
	if network != "" {
		argv = append(argv, "--network", network)
	}
	disks, err := dataDisks(volumes, false)
	if err != nil {
		return nil, err
	}
	for _, disk := range disks {
		argv = append(argv, "--data-disk", disk)
	}
	return append(argv, snapshot), nil
}

// dataDisks turns the storage plugin's `src:dst:mode:size` volumes into cocoon data disks.
func reseedArgv(binary, ID, hostname string, vm *vmRecord) []string {
	args := []string{binary, ID, hostname}
	for _, n := range vm.NICs {
		if n.MAC != "" && n.Network != nil && n.Network.IP != "" {
			args = append(args, n.MAC, n.Network.IP+"/"+strconv.Itoa(n.Network.Prefix), n.Network.Gateway)
		}
	}
	return sshrunner.Shell(reseedScript, args...)
}

func dataDisks(volumes []string, windows bool) ([]string, error) {
	disks := make([]string, 0, len(volumes))
	for _, volume := range volumes {
		parts := strings.Split(volume, ":")
		if len(parts) < volumeParts || parts[1] == "" || parts[3] == "" {
			return nil, errors.Wrapf(coretypes.ErrInvalidVolumeBind, "a vm data disk needs a mount and a size: %s", volume)
		}
		spec := "size=" + parts[3]
		if windows {
			spec += ",fstype=none"
		} else {
			spec += ",mount=" + parts[1]
		}
		disks = append(disks, spec)
	}
	return disks, nil
}

func unappliedOptions(opts *enginetypes.VirtualizationCreateOptions) []string {
	var unapplied []string
	if slices.ContainsFunc(opts.Env, isDeployEnv) {
		unapplied = append(unapplied, "env")
	}
	if len(opts.DNS) > 0 {
		unapplied = append(unapplied, "dns")
	}
	if len(opts.Hosts) > 0 {
		unapplied = append(unapplied, "extra_hosts")
	}
	if len(opts.Cmd) > 0 {
		unapplied = append(unapplied, "entrypoint commands")
	}
	if opts.WorkingDir != "" {
		unapplied = append(unapplied, "entrypoint dir")
	}
	return unapplied
}

func isDeployEnv(env string) bool {
	key, _, _ := strings.Cut(env, "=")
	return !slices.Contains(coreEnvKeys, key)
}

// requestedNetwork picks the conflist a deploy names; cocoon's IPAM assigns the address.
func requestedNetwork(ctx context.Context, networks map[string]string) (string, error) {
	if len(networks) > 1 {
		return "", errors.Wrapf(coretypes.ErrInvalidEngineArgs, "a vm takes one network, got %v", slices.Sorted(maps.Keys(networks)))
	}
	for name, ip := range networks {
		if ip != "" {
			log.WithFunc("engine.cocoon.requestedNetwork").Debugf(ctx, "cocoon assigns addresses through CNI, %s=%s is not carried over", name, ip)
		}
		return name, nil
	}
	return "", nil
}
