package process

import (
	"cmp"
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	podEnvKey = "ERU_POD"
	rootUser  = "root"

	createScript = "set -e\n" + unpackFunc + `dir=$1; ref=$2; cache=$3; launcher=$4; record=$5; overlay=$6; metadata=$7; binds=$8; shift 8
trap 'rm -rf "$dir"' EXIT
mkdir -p "$dir/lower"
if [ -d "$cache" ]; then
cp -a "$cache/." "$dir/lower/"
rm -f "$dir/lower/` + digestFile + `"
else
oras pull "$ref" -o "$dir/lower" "$@"
unpack "$dir/lower"
fi
if [ "$overlay" = 1 ]; then mkdir -p "$dir/upper" "$dir/work" "$dir/merged"; fi
printf '%s\n' "$binds" | while IFS= read -r source; do
if [ -n "$source" ]; then mkdir -p "$source"; fi
done
printf '%s\n' "$launcher" > "$dir/run.sh"
printf '%s\n' "$metadata" > "$dir/meta.json"
mkdir -p "$(dirname "$record")"
cp -f "$dir/meta.json" "$record.tmp"
mv "$record.tmp" "$record"
trap - EXIT
`
)

// RawArgs carries process-specific workload options through core untouched.
type RawArgs struct {
	Raw      bool `json:"raw"`       // run on the host filesystem, without an overlay root
	TasksMax int  `json:"tasks_max"` // cgroup v2 pids.max
}

func (e *Engine) VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (*enginetypes.VirtualizationCreated, error) {
	logger := log.WithFunc("engine.process.VirtualizationCreate")
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

	podname := lastEnvValue(opts.Env, podEnvKey)
	if !validPodname(podname) {
		return nil, errors.Wrapf(coretypes.ErrInvalidEngineArgs, "pod %q cannot name a systemd slice", podname)
	}

	ID := newID()
	dir := workloadDir(e.root, ID)
	u := &unit{
		ID:          ID,
		Podname:     podname,
		User:        opts.User,
		Bundle:      filepath.Join(dir, lowerDir),
		Working:     opts.WorkingDir,
		TasksMax:    rArgs.TasksMax,
		StopTimeout: e.stopTimeout,
		Opts:        opts,
		Resource:    resource,
	}
	if rArgs.Raw {
		u.Working = cmp.Or(opts.WorkingDir, u.Bundle)
	} else {
		u.Root = filepath.Join(dir, mergedDir)
	}
	if opts.Privileged {
		u.User = rootUser
	}

	record, err := json.Marshal(newMeta(ctx, u, e.ep.Nodename, e.host))
	if err != nil {
		return nil, err
	}
	overlay := strconv.Itoa(utils.Bool2Int(!rArgs.Raw))
	argv := shell(createScript, slices.Concat([]string{
		dir, opts.Image, imageDir(e.root, opts.Image), quote(u.argv()), metaPath(ID), overlay, string(record),
		strings.Join(bindSources(resource.Volumes, opts.Env), "\n"),
	}, e.registryFlags(opts.Image))...)
	if _, err = e.run(ctx, argv...); err != nil {
		return nil, err
	}
	return &enginetypes.VirtualizationCreated{ID: ID, Name: opts.Name, Labels: opts.Labels}, nil
}
