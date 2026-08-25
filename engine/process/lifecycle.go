package process

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	resourcetypes "github.com/projecteru2/core/resource/types"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	notExistsCode = 64
	runningCode   = 65
	notLoadedCode = 5

	showProperties  = "LoadState,ActiveState,SubState,ExecMainPID,ExecMainStatus,MemoryCurrent,CPUUsageNSec,User"
	subStateRunning = "running"

	waitScript = `unit=$1
while :; do
case "$(systemctl show "$unit" -p SubState --value)" in exited|failed|dead) break;; esac
sleep 1
done
systemctl show "$unit" -p ExecMainStatus --value
`
)

var (
	// unloaded swallows systemctl's exit 5, which means the transient unit is no longer loaded.
	unloaded = fmt.Sprintf("unloaded() { \"$@\" || [ $? = %d ]; }\n", notLoadedCode)

	startScript = "set -e\n" + unloaded + `dir=$1; unit=$2; record=$3
if [ "$(systemctl show "$unit" -p SubState --value)" = ` + subStateRunning + ` ]; then exit 0; fi
unloaded systemctl stop "$unit"
if [ -d "$dir/work" ] && ! mountpoint -q "$dir/merged"; then
mount -t overlay overlay -o "lowerdir=$dir/lower,upperdir=$dir/upper,workdir=$dir/work" "$dir/merged"
fi
mkdir -p "$(dirname "$record")"
cp -f "$dir/meta.json" "$record"
exec sh "$dir/run.sh"
`

	stopScript = "set -e\n" + unloaded + `unit=$1; dir=$2; force=$3
if [ "$force" = 1 ]; then systemctl kill -s SIGKILL "$unit" 2>/dev/null || true; fi
unloaded systemctl stop "$unit"
if mountpoint -q "$dir/merged"; then umount -l "$dir/merged"; fi
`

	removeScript = "set -e\n" + unloaded + fmt.Sprintf(`unit=$1; dir=$2; record=$3; force=$4
test -d "$dir" || exit %d
if [ "$force" = 1 ]; then
unloaded systemctl stop "$unit"
elif [ "$(systemctl show "$unit" -p SubState --value)" = %s ]; then
exit %d
fi
unloaded systemctl reset-failed "$unit"
if mountpoint -q "$dir/merged"; then umount -l "$dir/merged"; fi
rm -rf "$dir" "$record"
`, notExistsCode, subStateRunning, runningCode)

	inspectScript = fmt.Sprintf(`set -e
dir=$1; unit=$2
test -d "$dir" || exit %d
systemctl show "$unit" -p %s
`, notExistsCode, showProperties)
)

func (e *Engine) VirtualizationStart(ctx context.Context, ID string) error {
	_, err := e.run(ctx, shell(startScript, workloadDir(e.root, ID), unitName(ID), metaPath(ID))...)
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
	switch res.Code {
	case notExistsCode:
		return coretypes.ErrWorkloadNotExists
	case runningCode:
		return errors.Wrapf(coretypes.ErrInvaildWorkloadOps, "workload %s is running, stop it first or force the removal", ID)
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
	argv := shell(inspectScript, workloadDir(e.root, ID), unitName(ID))
	res, err := e.call(ctx, argv...)
	if err != nil {
		return nil, err
	}
	if res.Code == notExistsCode {
		return nil, errors.Wrapf(coretypes.ErrWorkloadNotExists, "no workload directory for %s", ID)
	}
	if err = exitError(argv, res); err != nil {
		return nil, err
	}
	shown := parseShow(res.Stdout)
	return &enginetypes.VirtualizationInfo{
		ID:       ID,
		User:     shown["User"],
		Running:  shown["SubState"] == subStateRunning,
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
	status := strings.TrimSpace(res.Stdout)
	code, err := strconv.ParseInt(status, 10, 64)
	if err != nil {
		err = errors.Wrapf(err, "workload %s reported ExecMainStatus %q", ID, status)
		return &enginetypes.VirtualizationWaitResult{Message: err.Error(), Code: -1}, err
	}
	return &enginetypes.VirtualizationWaitResult{Code: code}, nil
}

func (e *Engine) VirtualizationUpdateResource(ctx context.Context, ID string, engineParams resourcetypes.Resources) error {
	resource := &engine.VirtualizationResource{}
	if err := resource.Decode(engineParams); err != nil {
		log.WithFunc("engine.process.VirtualizationUpdateResource").WithField("ID", ID).
			Errorf(ctx, err, "failed to parse engine args %+v", engineParams)
		return err
	}
	argv := slices.Concat([]string{"systemctl", "set-property", "--runtime", unitName(ID)}, updateProperties(resource))
	res, err := e.call(ctx, argv...)
	if err != nil {
		return err
	}
	if res.Code == notLoadedCode {
		return nil
	}
	return exitError(argv, res)
}
