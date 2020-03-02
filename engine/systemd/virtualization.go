package systemd

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/utils"
)

const (
	cmdFileExist      = `/usr/bin/test -f '%s'`
	cmdCopyFromStdin  = `/bin/cp -f /dev/stdin '%s'`
	cmdMkdir          = `/bin/mkdir -p %s`
	cmdRemove         = `/bin/rm -f %s`
	cmdSystemdReload  = `/bin/systemctl daemon-reload`
	cmdSystemdRestart = `/bin/systemctl restart %s`
	cmdSystemdStop    = `/bin/systemctl stop %s`
	cmdSystemdStatus  = `/bin/systemctl show %s --property SubState,ActiveState,Environment,Description --no-pager`
)

func (s *SystemdSSH) VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (created *enginetypes.VirtualizationCreated, err error) {
	ID := "SYSTEMD-" + utils.RandomString(46)

	cpuAmount, err := s.CPUInfo(ctx)
	if err != nil {
		return
	}
	buffer, err := s.newUnitBuilder(ID, opts).buildUnit().buildPreExec(cpuAmount).buildExec().buildPostExec().buffer()
	if err != nil {
		return
	}

	// cp - /usr/local/lib/systemd/system/
	if err = s.VirtualizationCopyTo(ctx, "", getUnitFilename(ID), buffer, true, true); err != nil {
		return
	}
	// systemctl daemon-reload
	_, stderr, err := s.runSingleCommand(ctx, cmdSystemdReload, nil)
	return &enginetypes.VirtualizationCreated{
		ID:   ID,
		Name: opts.Name,
	}, errors.Wrap(err, stderr.String())
}

func (s *SystemdSSH) VirtualizationCopyTo(ctx context.Context, ID, path string, content io.Reader, AllowOverwriteDirWithFile, _ bool) (err error) {
	// mkdir -p $(dirname $PATH)
	dirname, _ := filepath.Split(path)
	if _, stderr, err := s.runSingleCommand(ctx, fmt.Sprintf(cmdMkdir, dirname), nil); err != nil {
		return errors.Wrap(err, stderr.String())
	}

	// test -f $PATH && exit -1
	if !AllowOverwriteDirWithFile {
		if _, _, err = s.runSingleCommand(ctx, fmt.Sprintf(cmdFileExist, path), nil); err == nil {
			return fmt.Errorf("[VirtualizationCopyTo] file existed: %s", path)
		}
	}

	// cp /dev/stdin $PATH
	_, stderr, err := s.runSingleCommand(ctx, fmt.Sprintf(cmdCopyFromStdin, path), content)
	return errors.Wrap(err, stderr.String())
}

func (s *SystemdSSH) VirtualizationStart(ctx context.Context, ID string) (err error) {
	// systemctl restart $ID
	_, stderr, err := s.runSingleCommand(ctx, fmt.Sprintf(cmdSystemdRestart, ID), nil)
	return errors.Wrap(err, stderr.String())
}

func (s *SystemdSSH) VirtualizationStop(ctx context.Context, ID string) (err error) {
	// systemctl stop $ID
	_, stderr, err := s.runSingleCommand(ctx, fmt.Sprintf(cmdSystemdStop, ID), nil)
	return errors.Wrap(err, stderr.String())
}

func (s *SystemdSSH) VirtualizationRemove(ctx context.Context, ID string, volumes, force bool) (err error) {
	if force {
		s.VirtualizationStop(ctx, ID)
	}

	// rm -f $FILE
	if _, stderr, err := s.runSingleCommand(ctx, fmt.Sprintf(cmdRemove, getUnitFilename(ID)), nil); err != nil {
		return errors.Wrap(err, stderr.String())
	}

	// systemctl daemon-reload
	_, stderr, err := s.runSingleCommand(ctx, fmt.Sprintf(cmdSystemdReload), nil)
	return errors.Wrap(err, stderr.String())
}

func (s *SystemdSSH) VirtualizationInspect(ctx context.Context, ID string) (info *enginetypes.VirtualizationInfo, err error) {
	stdout, stderr, err := s.runSingleCommand(ctx, fmt.Sprintf(cmdSystemdStatus, ID), nil)
	if err != nil {
		return nil, errors.Wrap(err, stderr.String())
	}

	serviceStatus := newServiceStatus(stdout)

	env, err := serviceStatus.env()
	if err != nil {
		return
	}

	labels, err := serviceStatus.labels()
	if err != nil {
		return
	}

	return &enginetypes.VirtualizationInfo{
		ID:       ID,
		User:     "root",
		Running:  serviceStatus.running(),
		Env:      env,
		Labels:   labels,
		Networks: map[string]string{"host": s.hostIP},
	}, nil
}

func (s *SystemdSSH) VirtualizationLogs(ctx context.Context, ID string, follow, stdout, stderr bool) (reader io.ReadCloser, err error) {
	err = engine.NotImplementedError
	return
}

func (s *SystemdSSH) VirtualizationAttach(ctx context.Context, ID string, stream, stdin bool) (reader io.ReadCloser, writer io.WriteCloser, err error) {
	err = engine.NotImplementedError
	return
}

func (s *SystemdSSH) VirtualizationResize(ctx context.Context, ID string, height, width uint) (err error) {
	err = engine.NotImplementedError
	return
}

func (s *SystemdSSH) VirtualizationWait(ctx context.Context, ID, state string) (res *enginetypes.VirtualizationWaitResult, err error) {
	err = engine.NotImplementedError
	return
}

func (s *SystemdSSH) VirtualizationUpdateResource(ctx context.Context, ID string, opts *enginetypes.VirtualizationResource) (err error) {
	err = engine.NotImplementedError
	return
}

func (s *SystemdSSH) VirtualizationCopyFrom(ctx context.Context, ID, path string) (reader io.ReadCloser, filename string, err error) {
	err = engine.NotImplementedError
	return
}
