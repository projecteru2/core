package sshrunner

import (
	"context"
	"io"
	"net"
	"os"
)

// Runner drives one node over a single SSH connection.
type Runner interface {
	Run(ctx context.Context, line string, stdin io.Reader) (*Result, error)
	Start(ctx context.Context, line string, opts *StartOptions) (Session, error)
	Files(ctx context.Context) (Files, error)
	Dial(ctx context.Context, network, addr string) (net.Conn, error)
	Ping(ctx context.Context) error
	Close() error
}

// Session is a command running on the node with its streams attached.
type Session interface {
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser
	Stderr() io.ReadCloser
	Resize(height, width uint) error
	Wait() (int, error)
	Close() error
}

// Files is sftp access to the node's filesystem.
type Files interface {
	Open(path string) (io.ReadCloser, error)
	Create(path string) (io.WriteCloser, error)
	MkdirAll(path string) error
	Stat(path string) (*FileInfo, error)
	Chown(path string, uid, gid int) error
	Chmod(path string, mode os.FileMode) error
	Close() error
}

// StartOptions selects the stream shape of a long-running command.
type StartOptions struct {
	Stdin bool
	TTY   bool
}

// Result is what a finished command left behind.
type Result struct {
	Stdout string
	Stderr string
	Code   int
}

// FileInfo is the ownership and mode of a node-side file.
type FileInfo struct {
	UID  int
	GID  int
	Mode os.FileMode
}

// Reader streams a running command's stdout and closes the session with it.
func Reader(sess Session) io.ReadCloser {
	return &sessionReader{Reader: sess.Stdout(), sess: sess}
}

type sessionReader struct {
	io.Reader
	sess Session
}

func (r *sessionReader) Close() error {
	return r.sess.Close()
}
