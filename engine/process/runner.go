package process

import (
	"context"
	"io"
	"os"
)

// runner executes commands and moves files on one node.
type runner interface {
	Run(ctx context.Context, line string, stdin io.Reader) (*result, error)
	Start(ctx context.Context, line string, opts *startOptions) (session, error)
	Files(ctx context.Context) (files, error)
	Close() error
}

// session is one command still running on the node.
type session interface {
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser
	Stderr() io.ReadCloser
	Resize(height, width uint) error
	Wait() (int, error)
	Close() error
}

// files is the node's filesystem.
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
