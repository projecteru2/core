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
	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/engine/workloadmeta"
	"github.com/projecteru2/core/log"
	resourcetypes "github.com/projecteru2/core/resource/types"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	runningCode = 65

	// systemctl exits 1 on a transient unit that is already unloaded, so ask before acting on one.
	loadedFunc = `loaded() { [ "$(systemctl show "$1" -p LoadState --value 2>/dev/null)" = loaded ]; }
`

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
	startScript = "set -e\n" + loadedFunc + `dir=$1; unit=$2; record=$3
if [ "$(systemctl show "$unit" -p SubState --value)" = ` + subStateRunning + ` ]; then exit 0; fi
if loaded "$unit"; then systemctl stop "$unit"; fi
if [ -d "$dir/work" ] && ! mountpoint -q "$dir/merged"; then
mount -t overlay overlay -o "lowerdir=$dir/lower,upperdir=$dir/upper,workdir=$dir/work" "$dir/merged"
fi
mkdir -p "$(dirname "$record")"
cp -f "$dir/meta.json" "$record"
exec sh "$dir/run.sh"
`

	stopScript = "set -e\n" + loadedFunc + `unit=$1; dir=$2; force=$3
if loaded "$unit"; then
if [ "$force" = 1 ]; then systemctl kill -s SIGKILL "$unit" 2>/dev/null || true; fi
systemctl stop "$unit"
fi
if mountpoint -q "$dir/merged"; then umount -l "$dir/merged"; fi
`

	removeScript = "set -e\n" + loadedFunc + fmt.Sprintf(`unit=$1; dir=$2; record=$3; force=$4
test -d "$dir" || exit %d
if loaded "$unit"; then
if [ "$force" = 1 ]; then
systemctl stop "$unit"
elif [ "$(systemctl show "$unit" -p SubState --value)" = %s ]; then
exit %d
fi
systemctl reset-failed "$unit" 2>/dev/null || true
fi
if mountpoint -q "$dir/merged"; then umount -l "$dir/merged"; fi
rm -rf "$dir" "$record"
`, workloadmeta.NotExistsCode, subStateRunning, runningCode)

	inspectScript = fmt.Sprintf(`set -e
dir=$1; unit=$2
test -d "$dir" || exit %d
systemctl show "$unit" -p %s
`, workloadmeta.NotExistsCode, showProperties)

	updateScript = "set -e\n" + loadedFunc + `unit=$1; dir=$2; shift 2
test -d "$dir" || exit 0
printf '%s\n' "$@" > "$dir/` + propsFile + `.tmp"
if loaded "$unit"; then systemctl set-property --runtime "$unit" "$@"; fi
mv "$dir/` + propsFile + `.tmp" "$dir/` + propsFile + `"
`
)

func (e *Engine) VirtualizationStart(ctx context.Context, ID string) error {
	_, err := e.run(ctx, sshrunner.Shell(startScript, workloadDir(e.root, ID), unitName(ID), workloadmeta.Path(ID))...)
	return err
}

func (e *Engine) VirtualizationStop(ctx context.Context, ID string, gracefulTimeout time.Duration) error {
	force := strconv.Itoa(utils.Bool2Int(gracefulTimeout == 0))
	_, err := e.run(ctx, sshrunner.Shell(stopScript, unitName(ID), workloadDir(e.root, ID), force)...)
	return err
}

func (e *Engine) VirtualizationRemove(ctx context.Context, ID string, _, force bool) error {
	argv := sshrunner.Shell(removeScript, unitName(ID), workloadDir(e.root, ID), workloadmeta.Path(ID), strconv.Itoa(utils.Bool2Int(force)))
	res, err := e.call(ctx, argv...)
	if err != nil {
		return err
	}
	e.records.Delete(ID)
	switch res.Code {
	case workloadmeta.NotExistsCode:
		return coretypes.ErrWorkloadNotExists
	case runningCode:
		return errors.Wrapf(coretypes.ErrInvaildWorkloadOps, "workload %s is running, stop it first or force the removal", ID)
	}
	return sshrunner.ExitError(argv, res)
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
	argv := sshrunner.Shell(inspectScript, workloadDir(e.root, ID), unitName(ID))
	res, err := e.call(ctx, argv...)
	if err != nil {
		return nil, err
	}
	if res.Code == workloadmeta.NotExistsCode {
		e.records.Delete(ID)
		return nil, errors.Wrapf(coretypes.ErrWorkloadNotExists, "no workload directory for %s", ID)
	}
	if err = sshrunner.ExitError(argv, res); err != nil {
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
	res, err := sshrunner.Stream(ctx, e.runner, sshrunner.Shell(waitScript, unitName(ID))...)
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
	_, err := e.run(ctx, sshrunner.Shell(updateScript, slices.Concat([]string{unitName(ID), workloadDir(e.root, ID)}, updateProperties(resource))...)...)
	return err
}
