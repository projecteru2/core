package process

import (
	"context"
	"io"
	"os"
)

type runner interface {
	Run(ctx context.Context, line string, stdin io.Reader) (*result, error)
	Start(ctx context.Context, line string, opts *startOptions) (session, error)
	Files(ctx context.Context) (files, error)
	Close() error
}

type session interface {
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser
	Stderr() io.ReadCloser
	Resize(height, width uint) error
	Wait() (int, error)
	Close() error
}

type files interface {
	Open(path string) (io.ReadCloser, error)
	Create(path string) (io.WriteCloser, error)
	MkdirAll(path string) error
	Stat(path string) (*fileInfo, error)
	Chown(path string, uid, gid int) error
	Chmod(path string, mode os.FileMode) error
	Close() error
}

type startOptions struct {
	Stdin bool
	TTY   bool
}

type result struct {
	Stdout string
	Stderr string
	Code   int
}

type fileInfo struct {
	UID  int
	GID  int
	Mode os.FileMode
}
