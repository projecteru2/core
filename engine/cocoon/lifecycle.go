package cocoon

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	resourcetypes "github.com/projecteru2/core/resource/types"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	notExistsCode = 64
	eventDeleted  = "DELETED"
	// guestIface is the adapter name the cocoonstack Windows images give their virtio NIC.
	guestIface = "Ethernet"

	addressTimeout = 5 * time.Minute

	// startScript prints the record before and after the boot: first_booted is read before, the pid after.
	startScript = `set -e
bin=$1; vm=$2; durable=$3
test -f "$durable" || exit 64
"$bin" vm inspect "$vm"
"$bin" vm start "$vm" >/dev/null
"$bin" vm inspect "$vm"
`

	// addressScript retries until cocoon-agent answers, which takes a Windows guest a minute.
	addressScript = `bin=$1; vm=$2; shift 2
tries=0
until "$bin" vm exec "$vm" -- netsh interface ip set address ` + guestIface + ` static "$@"; do
tries=$((tries+1))
[ "$tries" -lt 90 ] || exit 1
sleep 2
done
`

	removeScript = `bin=$1; vm=$2; durable=$3; record=$4; snap=$5; force=$6
test -f "$durable" || exit 64
set --
if [ "$force" = 1 ]; then set -- --force; fi
if ! out=$("$bin" vm rm "$@" "$vm" 2>&1) && "$bin" vm inspect "$vm" >/dev/null 2>&1; then
printf '%s\n' "$out" >&2
exit 1
fi
"$bin" snapshot rm "$snap" >/dev/null 2>&1 || true
rm -f "$durable" "$record"
`

	suspendScript = `bin=$1; vm=$2; snap=$3
"$bin" snapshot rm "$snap" >/dev/null 2>&1 || true
exec "$bin" vm hibernate --name "$snap" "$vm"
`

	// resumeScript restores by copy, so the snapshot can go once the guest runs again.
	resumeScript = `set -e
bin=$1; vm=$2; snap=$3
"$bin" vm restore --restore-mode copy "$vm" "$snap" >/dev/null
"$bin" snapshot rm "$snap" >/dev/null
"$bin" vm inspect "$vm"
`

	stopScript = `bin=$1; vm=$2; durable=$3; shift 3
test -f "$durable" || exit 64
exec "$bin" vm stop "$@" "$vm"
`

	inspectScript = `bin=$1; vm=$2; durable=$3
test -f "$durable" || exit 64
cat "$durable"
exec "$bin" vm inspect "$vm"
`

	// waitScript checks the vm first: the event stream stays silent for a vm that is not there.
	waitScript = `bin=$1; vm=$2
"$bin" vm inspect "$vm" >/dev/null || exit 1
exec "$bin" vm status --event --format json "$vm"
`
)

func (e *Engine) VirtualizationStart(ctx context.Context, ID string) error {
	res, err := e.runRecorded(ctx, sshrunner.Shell(startScript, e.cocoon.Binary, ID, durablePath(e.cocoon.Root, ID)), ID)
	if err != nil {
		return err
	}
	before, after, err := parseVMs(res.Stdout)
	if err != nil {
		return err
	}
	if err = e.refreshRecord(ctx, ID, after); err != nil {
		return err
	}
	if addr := before.address(); before.Config.Windows && !before.FirstBooted && addr != nil {
		go e.programAddress(ctx, ID, addr)
	}
	return nil
}

func (e *Engine) VirtualizationStop(ctx context.Context, ID string, gracefulTimeout time.Duration) error {
	flags := []string{}
	switch {
	case gracefulTimeout == 0:
		flags = append(flags, "--force")
	case gracefulTimeout > 0:
		flags = append(flags, "--timeout", strconv.FormatInt(max(int64(gracefulTimeout.Seconds()), 1), 10))
	}
	argv := sshrunner.Shell(stopScript, slices.Concat([]string{e.cocoon.Binary, ID, durablePath(e.cocoon.Root, ID)}, flags)...)
	_, err := e.runRecorded(ctx, argv, ID)
	return err
}

func (e *Engine) VirtualizationRemove(ctx context.Context, ID string, _, force bool) error {
	argv := sshrunner.Shell(removeScript, e.cocoon.Binary, ID, durablePath(e.cocoon.Root, ID), metaPath(ID), snapshotName(ID), strconv.Itoa(utils.Bool2Int(force)))
	_, err := e.runRecorded(ctx, argv, ID)
	return err
}

func (e *Engine) VirtualizationSuspend(ctx context.Context, ID string) error {
	_, err := e.run(ctx, sshrunner.Shell(suspendScript, e.cocoon.Binary, ID, snapshotName(ID))...)
	return err
}

func (e *Engine) VirtualizationResume(ctx context.Context, ID string) error {
	res, err := e.run(ctx, sshrunner.Shell(resumeScript, e.cocoon.Binary, ID, snapshotName(ID))...)
	if err != nil {
		return err
	}
	vm, err := parseVM(res.Stdout)
	if err != nil {
		return err
	}
	return e.refreshRecord(ctx, ID, vm)
}

func (e *Engine) VirtualizationInspect(ctx context.Context, ID string) (*enginetypes.VirtualizationInfo, error) {
	res, err := e.runRecorded(ctx, sshrunner.Shell(inspectScript, e.cocoon.Binary, ID, durablePath(e.cocoon.Root, ID)), ID)
	if err != nil {
		return nil, err
	}
	record, vm, err := parseInspect(res.Stdout)
	if err != nil {
		return nil, err
	}
	return &enginetypes.VirtualizationInfo{
		ID:       ID,
		User:     record.User,
		Image:    vm.Config.Image,
		Running:  vm.running(),
		Networks: vm.networks(),
	}, nil
}

func (e *Engine) VirtualizationResize(context.Context, string, uint, uint) error {
	return coretypes.ErrEngineNotImplemented
}

// VirtualizationWait follows the status stream until the guest is no longer running; a VM has no exit code.
func (e *Engine) VirtualizationWait(ctx context.Context, ID, _ string) (*enginetypes.VirtualizationWaitResult, error) {
	running, err := e.runner.Start(ctx, sshrunner.Quote(sshrunner.Shell(waitScript, e.cocoon.Binary, ID)), &sshrunner.StartOptions{})
	if err != nil {
		return &enginetypes.VirtualizationWaitResult{Message: err.Error(), Code: -1}, err
	}
	defer func() {
		_ = running.Close()
	}()
	decoder := json.NewDecoder(running.Stdout())
	for {
		event := &vmEvent{}
		if err = decoder.Decode(event); err != nil {
			err = errors.Wrapf(err, "the status stream of %s ended", ID)
			return &enginetypes.VirtualizationWaitResult{Message: err.Error(), Code: -1}, err
		}
		if event.Event == eventDeleted || !event.VM.running() {
			return &enginetypes.VirtualizationWaitResult{}, nil
		}
	}
}

func (e *Engine) VirtualizationUpdateResource(ctx context.Context, ID string, engineParams resourcetypes.Resources) error {
	resource := &engine.VirtualizationResource{}
	if err := resource.Decode(engineParams); err != nil {
		log.WithFunc("engine.cocoon.VirtualizationUpdateResource").WithField("ID", ID).
			Errorf(ctx, err, "failed to parse engine args %+v", engineParams)
		return err
	}
	if resource.Remap {
		return nil
	}
	return errors.Wrap(coretypes.ErrEngineNotImplemented, "cpu and memory hot-plug wait on cocoon (projecteru2/core#661)")
}

func (e *Engine) programAddress(ctx context.Context, ID string, addr *guestAddress) {
	ctx, cancel := context.WithTimeout(utils.NewInheritCtx(ctx), addressTimeout)
	defer cancel()
	if _, err := e.run(ctx, sshrunner.Shell(addressScript, e.cocoon.Binary, ID, addr.IP, addr.mask(), addr.Gateway)...); err != nil {
		log.WithFunc("engine.cocoon.programAddress").Warnf(ctx, "vm %s did not take the address %s: %v", ID, addr.IP, err)
	}
}

// runRecorded runs a script that exits 64 when the workload has no record on the node.
func (e *Engine) runRecorded(ctx context.Context, argv []string, ID string) (*sshrunner.Result, error) {
	res, err := e.call(ctx, argv...)
	if err != nil {
		return nil, err
	}
	if res.Code == notExistsCode {
		return nil, errors.Wrapf(coretypes.ErrWorkloadNotExists, "no record for %s", ID)
	}
	return res, sshrunner.ExitError(argv, res)
}
