package systemd

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
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
	cmdCopyToStdout   = `/bin/cp -f '%s' /dev/stdout`
)

// VirtualizationCreate creates systemd service
func (s *SSHClient) VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (created *enginetypes.VirtualizationCreated, err error) {
	ID := "SYSTEMD-" + strings.ToLower(utils.RandomString(46))

	cpuAmount, err := s.cpuInfo(ctx)
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

// VirtualizationCopyTo send bytes to file system
func (s *SSHClient) VirtualizationCopyTo(ctx context.Context, ID, target string, content io.Reader, AllowOverwriteDirWithFile, _ bool) (err error) {
	// mkdir -p $(dirname $PATH)
	dirname, _ := filepath.Split(target)
	if _, stderr, err := s.runSingleCommand(ctx, fmt.Sprintf(cmdMkdir, dirname), nil); err != nil {
		return errors.Wrap(err, stderr.String())
	}

	// test -f $PATH && exit -1
	if !AllowOverwriteDirWithFile {
		if _, _, err = s.runSingleCommand(ctx, fmt.Sprintf(cmdFileExist, target), nil); err == nil {
			return fmt.Errorf("[VirtualizationCopyTo] file existed: %s", target)
		}
	}

	// cp /dev/stdin $PATH
	_, stderr, err := s.runSingleCommand(ctx, fmt.Sprintf(cmdCopyFromStdin, target), content)
	return errors.Wrap(err, stderr.String())
}

// VirtualizationStart starts a systemd service
func (s *SSHClient) VirtualizationStart(ctx context.Context, ID string) (err error) {
	// systemctl restart $ID
	_, stderr, err := s.runSingleCommand(ctx, fmt.Sprintf(cmdSystemdRestart, ID), nil)
	return errors.Wrap(err, stderr.String())
}

// VirtualizationStop stops a systemd service
func (s *SSHClient) VirtualizationStop(ctx context.Context, ID string) (err error) {
	// systemctl stop $ID
	_, stderr, err := s.runSingleCommand(ctx, fmt.Sprintf(cmdSystemdStop, ID), nil)
	return errors.Wrap(err, stderr.String())
}

// VirtualizationRemove removes a systemd service
func (s *SSHClient) VirtualizationRemove(ctx context.Context, ID string, volumes, force bool) (err error) {
	if force {
		_ = s.VirtualizationStop(ctx, ID)
	}

	// rm -f $FILE
	if _, stderr, err := s.runSingleCommand(ctx, fmt.Sprintf(cmdRemove, getUnitFilename(ID)), nil); err != nil {
		return errors.Wrap(err, stderr.String())
	}

	// systemctl daemon-reload
	_, stderr, err := s.runSingleCommand(ctx, cmdSystemdReload, nil)
	return errors.Wrap(err, stderr.String())
}

// VirtualizationInspect inspects a service
func (s *SSHClient) VirtualizationInspect(ctx context.Context, ID string) (info *enginetypes.VirtualizationInfo, err error) {
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

// VirtualizationLogs fetches service logs
func (s *SSHClient) VirtualizationLogs(ctx context.Context, opts *enginetypes.VirtualizationLogStreamOptions) (reader io.ReadCloser, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// VirtualizationAttach attaches a service's stdio
func (s *SSHClient) VirtualizationAttach(ctx context.Context, ID string, stream, stdin bool) (reader io.ReadCloser, writer io.WriteCloser, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// VirtualizationResize resizes a terminal window
func (s *SSHClient) VirtualizationResize(ctx context.Context, ID string, height, width uint) (err error) {
	err = types.ErrEngineNotImplemented
	return
}

// VirtualizationWait waits for service finishing
func (s *SSHClient) VirtualizationWait(ctx context.Context, ID, state string) (res *enginetypes.VirtualizationWaitResult, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// VirtualizationUpdateResource updates service resource limits
func (s *SSHClient) VirtualizationUpdateResource(ctx context.Context, ID string, opts *enginetypes.VirtualizationResource) (err error) {
	err = types.ErrEngineNotImplemented
	return
}

// VirtualizationCopyFrom copy files from one service to another
func (s *SSHClient) VirtualizationCopyFrom(ctx context.Context, ID, source string) (reader io.ReadCloser, filename string, err error) {
	stdout, stderr, err := s.runSingleCommand(ctx, fmt.Sprintf(cmdCopyToStdout, source), nil)
	return ioutil.NopCloser(stdout), path.Base(source), errors.Wrap(err, stderr.String())
}
