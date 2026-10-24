package cocoon

import (
	"context"
	"encoding/json"
	"regexp"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

const (
	snapshotScheme = "snapshot://"

	opSnapshotSave    = "snapshot.save"
	opSnapshotList    = "snapshot.list"
	opSnapshotInspect = "snapshot.inspect"
	opSnapshotRemove  = "snapshot.remove"

	noSnapshots = "No snapshots"

	saveScript = `set -e
bin=$1; name=$2; vm=$3
"$bin" snapshot save --name "$name" "$vm" >/dev/null
exec "$bin" snapshot inspect "$name"
`
)

// validSnapshotName is cocoon's own snapshot-name grammar.
var validSnapshotName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/-]{0,62}$`)

type snapshotParams struct {
	Name string `json:"name"`
}

func (e *Engine) RawEngine(ctx context.Context, opts *enginetypes.RawEngineOptions) (*enginetypes.RawEngineResult, error) {
	argv, err := e.rawArgv(opts)
	if err != nil {
		return nil, err
	}
	res, err := e.run(ctx, argv...)
	if err != nil {
		return nil, err
	}
	out := res.Stdout
	switch {
	case opts.Op == opSnapshotRemove:
		out = ""
	case opts.Op == opSnapshotList && strings.HasPrefix(strings.TrimSpace(out), noSnapshots):
		out = "[]"
	}
	return &enginetypes.RawEngineResult{ID: opts.ID, Data: []byte(out)}, nil
}

func (e *Engine) rawArgv(opts *enginetypes.RawEngineOptions) ([]string, error) {
	switch opts.Op {
	case opSnapshotList:
		return e.snapshot("list", "--format", formatJSON), nil
	case opSnapshotSave, opSnapshotInspect, opSnapshotRemove:
	default:
		return nil, errors.Wrapf(coretypes.ErrEngineNotImplemented, "cocoon has no raw op %q", opts.Op)
	}
	params := &snapshotParams{}
	if err := json.Unmarshal(opts.Params, params); err != nil {
		return nil, errors.Wrapf(coretypes.ErrInvalidEngineArgs, "%s params: %v", opts.Op, err)
	}
	if err := checkSnapshotName(params.Name); err != nil {
		return nil, err
	}
	switch opts.Op {
	case opSnapshotSave:
		return sshrunner.Shell(saveScript, e.cocoon.Binary, params.Name, opts.ID), nil
	case opSnapshotRemove:
		return e.snapshot("rm", params.Name), nil
	}
	return e.snapshot("inspect", params.Name), nil
}

// snapshotDigests reports a snapshot:// image as its own digest while the snapshot is on the node.
func (e *Engine) snapshotDigests(ctx context.Context, image, name string) ([]string, error) {
	if err := checkSnapshotName(name); err != nil {
		return nil, err
	}
	res, err := e.call(ctx, e.snapshot("inspect", name)...)
	if err != nil || res.Code != 0 {
		return nil, err
	}
	return []string{image}, nil
}

func (e *Engine) snapshot(args ...string) []string {
	return slices.Concat([]string{e.cocoon.Binary, "snapshot"}, args)
}

func checkSnapshotName(name string) error {
	if !validSnapshotName.MatchString(name) {
		return errors.Wrapf(coretypes.ErrInvalidEngineArgs, "snapshot name %q must match %s", name, validSnapshotName)
	}
	return nil
}
