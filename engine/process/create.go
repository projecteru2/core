package process

import (
	"cmp"
	"context"
	"encoding/json"
	"path/filepath"
	"strconv"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	podEnvKey = "ERU_POD"
	rootUser  = "root"

	createScript = `set -e
dir=$1; ref=$2; launcher=$3; record=$4; overlay=$5; metadata=$6
mkdir -p "$dir/lower"
oras pull "$ref" -o "$dir/lower"
if [ "$overlay" = 1 ]; then mkdir -p "$dir/upper" "$dir/work" "$dir/merged"; fi
printf '%s\n' "$launcher" > "$dir/run.sh"
mkdir -p "$(dirname "$record")"
printf '%s\n' "$metadata" > "$record.tmp"
mv "$record.tmp" "$record"
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

	ID := newID()
	dir := workloadDir(e.root, ID)
	u := &unit{
		ID:       ID,
		Podname:  envValue(opts.Env, podEnvKey),
		User:     opts.User,
		Working:  opts.WorkingDir,
		TasksMax: rArgs.TasksMax,
		Opts:     opts,
		Resource: resource,
	}
	if rArgs.Raw {
		u.Working = cmp.Or(opts.WorkingDir, filepath.Join(dir, "lower"))
	} else {
		u.Root = filepath.Join(dir, "merged")
	}
	if opts.Privileged {
		u.User = rootUser
	}

	record, err := json.Marshal(newMeta(ctx, u, e.ep.Nodename, e.host))
	if err != nil {
		return nil, err
	}
	overlay := strconv.Itoa(utils.Bool2Int(!rArgs.Raw))
	argv := shell(createScript, dir, opts.Image, quote(u.argv()), metaPath(ID), overlay, string(record))
	if _, err = e.run(ctx, argv...); err != nil {
		return nil, err
	}
	return &enginetypes.VirtualizationCreated{ID: ID, Name: opts.Name, Labels: opts.Labels}, nil
}
