package systemd

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
)

func (s *SystemdSSH) VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (created *enginetypes.VirtualizationCreated, err error) {
	return
}

func (s *SystemdSSH) VirtualizationCopyTo(ctx context.Context, ID, path string, content io.Reader, AllowOverwriteDirWithFile, CopyUIDGID bool) (err error) {
	return
}

func (s *SystemdSSH) VirtualizationStart(ctx context.Context, ID string) (err error) { return }

func (s *SystemdSSH) VirtualizationStop(ctx context.Context, ID string) (err error) { return }

func (s *SystemdSSH) VirtualizationRemove(ctx context.Context, ID string, volumes, force bool) (err error) {
	return
}

func (s *SystemdSSH) VirtualizationInspect(ctx context.Context, ID string) (info *enginetypes.VirtualizationInfo, err error) {
	return
}

func (s *SystemdSSH) VirtualizationLogs(ctx context.Context, ID string, follow, stdout, stderr bool) (reader io.ReadCloser, err error) {
	return
}

func (s *SystemdSSH) VirtualizationAttach(ctx context.Context, ID string, stream, stdin bool) (reader io.ReadCloser, writer io.WriteCloser, err error) {
	return
}

func (s *SystemdSSH) VirtualizationResize(ctx context.Context, ID string, height, width uint) (err error) {
	return
}

func (s *SystemdSSH) VirtualizationWait(ctx context.Context, ID, state string) (res *enginetypes.VirtualizationWaitResult, err error) {
	return
}

func (s *SystemdSSH) VirtualizationUpdateResource(ctx context.Context, ID string, opts *enginetypes.VirtualizationResource) (err error) {
	return
}

func (s *SystemdSSH) VirtualizationCopyFrom(ctx context.Context, ID, path string) (reader io.ReadCloser, filename string, err error) {
	return
}
