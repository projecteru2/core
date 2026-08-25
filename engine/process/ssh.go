package process

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	coretypes "github.com/projecteru2/core/types"
)

const (
	ptyTerm   = "xterm"
	ptyHeight = 40
	ptyWidth  = 80
)

// sshRunner keeps one connection per node and redials when it drops.
type sshRunner struct {
	addr   string
	config *ssh.ClientConfig

	mu     sync.Mutex
	client *ssh.Client
}

func newSSHRunner(addr string, config *ssh.ClientConfig) *sshRunner {
	return &sshRunner{addr: addr, config: config}
}

func (r *sshRunner) Run(ctx context.Context, line string, stdin io.Reader) (*result, error) {
	sess, err := r.newSession(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = sess.Close()
	}()
	stop := closeOnDone(ctx, sess)
	defer stop()

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	sess.Stdin, sess.Stdout, sess.Stderr = stdin, stdout, stderr
	code, err := exitStatus(sess.Run(line))
	if err != nil {
		return nil, err
	}
	return &result{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}, nil
}

func (r *sshRunner) Start(ctx context.Context, line string, opts *startOptions) (_ session, err error) {
	sess, err := r.newSession(ctx)
	if err != nil {
		return nil, err
	}
	running := &sshSession{sess: sess, stop: closeOnDone(ctx, sess)}
	defer func() {
		if err != nil {
			_ = running.Close()
		}
	}()

	if opts.TTY {
		if err = sess.RequestPty(ptyTerm, ptyHeight, ptyWidth, ssh.TerminalModes{}); err != nil {
			return nil, err
		}
	}
	if opts.Stdin {
		if running.stdin, err = sess.StdinPipe(); err != nil {
			return nil, err
		}
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		return nil, err
	}
	running.stdout, running.stderr = io.NopCloser(stdout), io.NopCloser(stderr)
	if err = sess.Start(line); err != nil {
		return nil, err
	}
	return running, nil
}

func (r *sshRunner) Files(ctx context.Context) (files, error) {
	client, err := r.connect(ctx, false)
	if err != nil {
		return nil, err
	}
	remote, err := sftp.NewClient(client)
	if err != nil {
		if client, err = r.connect(ctx, true); err != nil {
			return nil, err
		}
		if remote, err = sftp.NewClient(client); err != nil {
			return nil, err
		}
	}
	return &sftpFiles{client: remote}, nil
}

func (r *sshRunner) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.client == nil {
		return nil
	}
	err := r.client.Close()
	r.client = nil
	return err
}

func (r *sshRunner) newSession(ctx context.Context) (*ssh.Session, error) {
	client, err := r.connect(ctx, false)
	if err != nil {
		return nil, err
	}
	sess, err := client.NewSession()
	if err == nil {
		return sess, nil
	}
	if client, err = r.connect(ctx, true); err != nil {
		return nil, err
	}
	return client.NewSession()
}

func (r *sshRunner) connect(ctx context.Context, renew bool) (*ssh.Client, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.client != nil {
		if !renew {
			return r.client, nil
		}
		_ = r.client.Close()
		r.client = nil
	}
	conn, err := (&net.Dialer{Timeout: r.config.Timeout}).DialContext(ctx, "tcp", r.addr)
	if err != nil {
		return nil, err
	}
	handshaked, chans, reqs, err := ssh.NewClientConn(conn, r.addr, r.config)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	r.client = ssh.NewClient(handshaked, chans, reqs)
	return r.client, nil
}

type sshSession struct {
	sess   *ssh.Session
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	stop   func()
}

func (s *sshSession) Stdin() io.WriteCloser { return s.stdin }

func (s *sshSession) Stdout() io.ReadCloser { return s.stdout }

func (s *sshSession) Stderr() io.ReadCloser { return s.stderr }

func (s *sshSession) Resize(height, width uint) error {
	return s.sess.WindowChange(int(height), int(width)) //nolint:gosec // terminal geometry never reaches the int limit
}

func (s *sshSession) Wait() (int, error) {
	return exitStatus(s.sess.Wait())
}

func (s *sshSession) Close() error {
	s.stop()
	return s.sess.Close()
}

type sftpFiles struct {
	client *sftp.Client
}

func (f *sftpFiles) Open(path string) (io.ReadCloser, error) {
	file, err := f.client.Open(path)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (f *sftpFiles) Create(path string) (io.WriteCloser, error) {
	file, err := f.client.Create(path)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (f *sftpFiles) MkdirAll(path string) error {
	return f.client.MkdirAll(path)
}

func (f *sftpFiles) Stat(path string) (*fileInfo, error) {
	stat, err := f.client.Stat(path)
	if err != nil {
		return nil, err
	}
	info := &fileInfo{Mode: stat.Mode()}
	if attrs, ok := stat.Sys().(*sftp.FileStat); ok {
		info.UID, info.GID = int(attrs.UID), int(attrs.GID)
	}
	return info, nil
}

func (f *sftpFiles) Chown(path string, uid, gid int) error {
	return f.client.Chown(path, uid, gid)
}

func (f *sftpFiles) Chmod(path string, mode os.FileMode) error {
	return f.client.Chmod(path, mode)
}

func (f *sftpFiles) Close() error {
	return f.client.Close()
}

func newClientConfig(cfg coretypes.SSHConfig, user string, timeout time.Duration) (*ssh.ClientConfig, error) {
	key, err := os.ReadFile(cfg.PrivateKey) //nolint:gosec // the key path comes from the operator's own config
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}
	hostKey := ssh.InsecureIgnoreHostKey() //nolint:gosec // an empty known_hosts is the operator's documented opt-out
	if cfg.KnownHosts != "" {
		if hostKey, err = knownhosts.New(cfg.KnownHosts); err != nil {
			return nil, err
		}
	}
	return &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKey,
		Timeout:         timeout,
	}, nil
}

func closeOnDone(ctx context.Context, sess *ssh.Session) func() {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = sess.Close()
		case <-done:
		}
	}()
	return sync.OnceFunc(func() { close(done) })
}

func exitStatus(err error) (int, error) {
	var exitErr *ssh.ExitError
	switch {
	case err == nil:
		return 0, nil
	case errors.As(err, &exitErr):
		return exitErr.ExitStatus(), nil
	default:
		return -1, err
	}
}
