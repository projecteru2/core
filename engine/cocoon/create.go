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

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	podEnvKey   = "ERU_POD"
	osWindows   = "windows"
	formatJSON  = "json"
	volumeParts = 4

	discardTimeout = 30 * time.Second

	recordScript = `set -e
durable=$1; record=$2; body=$3
mkdir -p "$(dirname "$durable")" "$(dirname "$record")"
printf '%s\n' "$body" > "$durable"
cp -f "$durable" "$record.tmp"
mv "$record.tmp" "$record"
`
)

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
	network, err := requestedNetwork(opts.Networks)
	if err != nil {
		return nil, err
	}
	ID := newID()
	argv, err := createArgv(e.cocoon.Binary, ID, opts, resource, rArgs.OS == osWindows, network)
	if err != nil {
		return nil, err
	}

	res, err := e.run(ctx, argv...)
	if err != nil {
		return nil, err
	}
	vm, err := parseVM(res.Stdout)
	if err == nil {
		err = e.record(ctx, ID, opts, vm)
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
	_, err = e.run(ctx, sshrunner.Shell(recordScript, durablePath(e.cocoon.Root, ID), metaPath(ID), string(body))...)
	return err
}

// discard removes a VM whose eru record never landed; core only knows the ones that did.
func (e *Engine) discard(ctx context.Context, ID string) {
	ctx, cancel := context.WithTimeout(utils.NewInheritCtx(ctx), discardTimeout)
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

// dataDisks turns the storage plugin's `src:dst:mode:size` volumes into cocoon data disks.
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

// requestedNetwork picks the conflist a deploy names; cocoon's IPAM assigns the address.
func requestedNetwork(networks map[string]string) (string, error) {
	if len(networks) > 1 {
		return "", errors.Wrapf(coretypes.ErrInvalidEngineArgs, "a vm takes one network, got %v", slices.Sorted(maps.Keys(networks)))
	}
	for name, ip := range networks {
		if ip != "" {
			return "", errors.Wrapf(coretypes.ErrInvalidEngineArgs, "cocoon assigns addresses through CNI, %s=%s cannot be fixed", name, ip)
		}
		return name, nil
	}
	return "", nil
}
