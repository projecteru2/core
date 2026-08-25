package process

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	resourcetypes "github.com/projecteru2/core/resource/types"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	notExistsCode  = 64
	showProperties = "LoadState,ActiveState,SubState,ExecMainPID,ExecMainStatus,MemoryCurrent,CPUUsageNSec,User"

	startScript = `set -e
dir=$1
if [ -d "$dir/work" ] && ! mountpoint -q "$dir/merged"; then
mount -t overlay overlay -o "lowerdir=$dir/lower,upperdir=$dir/upper,workdir=$dir/work" "$dir/merged"
fi
exec sh "$dir/run.sh"
`
	stopScript = `set -e
unit=$1; dir=$2; force=$3
if [ "$force" = 1 ]; then systemctl kill --signal=SIGKILL "$unit" 2>/dev/null || true; fi
systemctl stop "$unit"
if mountpoint -q "$dir/merged"; then umount "$dir/merged"; fi
`
	waitScript = `unit=$1
while :; do
case "$(systemctl show "$unit" -p ActiveState --value)" in inactive|failed) break;; esac
sleep 1
done
systemctl show "$unit" -p ExecMainStatus --value
`
)

var removeScript = fmt.Sprintf(`set -e
unit=$1; dir=$2; record=$3; force=$4
test -d "$dir" || exit %d
if [ "$force" = 1 ]; then systemctl stop "$unit" 2>/dev/null || true; fi
systemctl reset-failed "$unit" 2>/dev/null || true
if mountpoint -q "$dir/merged"; then umount "$dir/merged"; fi
rm -rf "$dir" "$record"
`, notExistsCode)

func (e *Engine) VirtualizationStart(ctx context.Context, ID string) error {
	_, err := e.run(ctx, shell(startScript, workloadDir(e.root, ID))...)
	return err
}

func (e *Engine) VirtualizationStop(ctx context.Context, ID string, gracefulTimeout time.Duration) error {
	force := strconv.Itoa(utils.Bool2Int(gracefulTimeout == 0))
	_, err := e.run(ctx, shell(stopScript, unitName(ID), workloadDir(e.root, ID), force)...)
	return err
}

func (e *Engine) VirtualizationRemove(ctx context.Context, ID string, _, force bool) error {
	argv := shell(removeScript, unitName(ID), workloadDir(e.root, ID), metaPath(ID), strconv.Itoa(utils.Bool2Int(force)))
	res, err := e.call(ctx, argv...)
	if err != nil {
		return err
	}
	if res.Code == notExistsCode {
		return coretypes.ErrWorkloadNotExists
	}
	return exitError(argv, res)
}

func (e *Engine) VirtualizationSuspend(ctx context.Context, ID string) error {
	_, err := e.run(ctx, "systemctl", "freeze", unitName(ID))
	return err
}

func (e *Engine) VirtualizationResume(ctx context.Context, ID string) error {
	_, err := e.run(ctx, "systemctl", "thaw", unitName(ID))
	return err
}

func (e *Engine) VirtualizationInspect(ctx context.Context, ID string) (*enginetypes.VirtualizationInfo, error) {
	res, err := e.run(ctx, "systemctl", "show", unitName(ID), "-p", showProperties)
	if err != nil {
		return nil, err
	}
	shown := parseShow(res.Stdout)
	if shown["LoadState"] == "not-found" {
		return nil, coretypes.ErrWorkloadNotExists
	}
	return &enginetypes.VirtualizationInfo{
		ID:       ID,
		User:     shown["User"],
		Running:  shown["ActiveState"] == "active",
		Networks: map[string]string{hostNetwork: e.host},
	}, nil
}

func (e *Engine) VirtualizationResize(context.Context, string, uint, uint) error {
	return coretypes.ErrEngineNotImplemented
}

func (e *Engine) VirtualizationWait(ctx context.Context, ID, _ string) (*enginetypes.VirtualizationWaitResult, error) {
	res, err := e.run(ctx, shell(waitScript, unitName(ID))...)
	if err != nil {
		return &enginetypes.VirtualizationWaitResult{Message: err.Error(), Code: -1}, err
	}
	code, _ := strconv.ParseInt(strings.TrimSpace(res.Stdout), 10, 64)
	return &enginetypes.VirtualizationWaitResult{Code: code}, nil
}

func (e *Engine) VirtualizationUpdateResource(ctx context.Context, ID string, engineParams resourcetypes.Resources) error {
	resource := &engine.VirtualizationResource{}
	if err := resource.Decode(engineParams); err != nil {
		log.WithFunc("engine.process.VirtualizationUpdateResource").WithField("ID", ID).
			Errorf(ctx, err, "failed to parse engine args %+v", engineParams)
		return err
	}
	props := properties(resource, 0)
	if len(props) == 0 {
		return nil
	}
	_, err := e.run(ctx, slices.Concat([]string{"systemctl", "set-property", unitName(ID)}, props)...)
	return err
}
