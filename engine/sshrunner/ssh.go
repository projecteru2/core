package sshrunner

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/sync/semaphore"

	coretypes "github.com/projecteru2/core/types"
)

const (
	ptyTerm   = "xterm"
	ptyHeight = 40
	ptyWidth  = 80
	// sshd's default MaxSessions is 10; queue past that instead of being refused.
	maxSessions = 8
	// streams live on connections of their own, opened as they fill up and closed once idle (projecteru2/core#670)
	maxStreamClients = 4
	streamIdle       = 30 * time.Second

	openRetries       = 4
	openRetryInterval = 100 * time.Millisecond

	keepaliveRequest = "keepalive@openssh.com"
)

type sshOp[T any] func(*ssh.Client) (T, error)

var _ Runner = (*sshRunner)(nil)

// sshRunner keeps one connection per node and redials when the transport drops.
type sshRunner struct {
	addr     string
	config   *ssh.ClientConfig
	sessions *semaphore.Weighted
	streams  *streamPool

	mu     sync.Mutex
	client *ssh.Client
}

// New builds a runner that dials addr on demand and keeps the connection.
func New(addr string, config *ssh.ClientConfig) Runner {
	return newSSHRunner(addr, config)
}

func newSSHRunner(addr string, config *ssh.ClientConfig) *sshRunner {
	r := &sshRunner{addr: addr, config: config, sessions: semaphore.NewWeighted(maxSessions)}
	r.streams = newStreamPool(func(ctx context.Context) (streamConn, error) { return r.dial(ctx) })
	return r
}

func (r *sshRunner) Run(ctx context.Context, line string, stdin io.Reader) (*Result, error) {
	if err := r.sessions.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	defer r.sessions.Release(1)

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
	return &Result{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}, nil
}

func (r *sshRunner) Start(ctx context.Context, line string, opts *StartOptions) (_ Session, err error) {
	stream, sess, err := r.openStream(ctx)
	if err != nil {
		return nil, err
	}
	running := &sshSession{sess: sess, release: sync.OnceFunc(func() { r.streams.release(stream) })}
	running.stop = closeOnDone(ctx, running)
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

func (r *sshRunner) Files(ctx context.Context) (Files, error) {
	if err := r.sessions.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	release := sync.OnceFunc(func() { r.sessions.Release(1) })

	remote, err := retry(ctx, r, func(client *ssh.Client) (*sftp.Client, error) { return sftp.NewClient(client) })
	if err != nil {
		release()
		return nil, err
	}
	files := &sftpFiles{client: remote, release: release}
	files.stop = closeOnDone(ctx, files)
	return files, nil
}

func (r *sshRunner) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	// a forward is not a session, so MaxSessions does not bound it
	return retry(ctx, r, func(client *ssh.Client) (net.Conn, error) { return client.Dial(network, addr) })
}

func (r *sshRunner) Ping(ctx context.Context) error {
	// a global request on the connection answers even when every session is held
	_, err := retry(ctx, r, func(client *ssh.Client) (struct{}, error) {
		_, _, err := client.SendRequest(keepaliveRequest, true, nil)
		return struct{}{}, err
	})
	return err
}

func (r *sshRunner) Close() error {
	r.streams.close()
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
	return retry(ctx, r, (*ssh.Client).NewSession)
}

// connect hands out the current client, and redials only when the caller's stale client is still the current one.
func (r *sshRunner) connect(ctx context.Context, stale *ssh.Client) (*ssh.Client, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.client != nil {
		if r.client != stale {
			return r.client, nil
		}
		_ = r.client.Close()
		r.client = nil
	}
	client, err := r.dial(ctx)
	if err != nil {
		return nil, err
	}
	r.client = client
	return r.client, nil
}

func (r *sshRunner) dial(ctx context.Context) (*ssh.Client, error) {
	conn, err := (&net.Dialer{Timeout: r.config.Timeout}).DialContext(ctx, "tcp", r.addr)
	if err != nil {
		return nil, err
	}
	handshaked, chans, reqs, err := ssh.NewClientConn(conn, r.addr, r.config)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return ssh.NewClient(handshaked, chans, reqs), nil
}

// openStream takes a slot on a stream connection and opens the session there; a dead connection is dropped and the open retried once.
func (r *sshRunner) openStream(ctx context.Context) (*streamClient, *ssh.Session, error) {
	for attempt := 0; ; attempt++ {
		stream, err := r.streams.acquire(ctx)
		if err != nil {
			return nil, nil, err
		}
		sess, err := bounded(ctx, stream.conn.NewSession)
		if err == nil {
			return stream, sess, nil
		}
		if ctx.Err() != nil || !isTransportError(err) || attempt == 1 {
			r.streams.release(stream)
			return nil, nil, err
		}
		r.streams.evict(stream)
	}
}

type sshSession struct {
	sess    *ssh.Session
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	stop    func()
	release func()
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
	s.release()
	return s.sess.Close()
}

type sftpFiles struct {
	client  *sftp.Client
	stop    func()
	release func()
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

func (f *sftpFiles) Stat(path string) (*FileInfo, error) {
	stat, err := f.client.Stat(path)
	if err != nil {
		return nil, err
	}
	attrs := stat.Sys().(*sftp.FileStat)
	return &FileInfo{Mode: stat.Mode(), UID: int(attrs.UID), GID: int(attrs.GID)}, nil
}

func (f *sftpFiles) Chown(path string, uid, gid int) error {
	return f.client.Chown(path, uid, gid)
}

func (f *sftpFiles) Chmod(path string, mode os.FileMode) error {
	return f.client.Chmod(path, mode)
}

func (f *sftpFiles) Close() error {
	f.stop()
	f.release()
	return f.client.Close()
}

// NewClientConfig builds the ssh client config core uses for every node it drives over SSH.
func NewClientConfig(cfg coretypes.SSHConfig, user string, timeout time.Duration) (*ssh.ClientConfig, error) {
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

// closeOnDone closes the wrapper when ctx ends, so the slot goes back with the transport.
func closeOnDone(ctx context.Context, c io.Closer) func() {
	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	return func() { stop() }
}

// retry backs off on a refused channel open, and redials a transport that died underneath the call.
func retry[T any](ctx context.Context, r *sshRunner, f sshOp[T]) (T, error) {
	return retryRefused(ctx, func() (T, error) { return openOnce(ctx, r, f) })
}

func openOnce[T any](ctx context.Context, r *sshRunner, f sshOp[T]) (T, error) {
	var zero T
	client, err := r.connect(ctx, nil)
	if err != nil {
		return zero, err
	}
	v, err := bounded(ctx, func() (T, error) { return f(client) })
	if err == nil || ctx.Err() != nil || !isTransportError(err) {
		return v, err
	}
	if client, err = r.connect(ctx, client); err != nil {
		return zero, err
	}
	return bounded(ctx, func() (T, error) { return f(client) })
}

func bounded[T any](ctx context.Context, open func() (T, error)) (T, error) {
	type opened struct {
		v   T
		err error
	}
	done := make(chan opened, 1)
	go func() {
		v, err := open()
		done <- opened{v, err}
	}()
	select {
	case res := <-done:
		return res.v, res.err
	case <-ctx.Done():
		go func() {
			if res := <-done; res.err == nil {
				if c, ok := any(res.v).(io.Closer); ok {
					_ = c.Close()
				}
			}
		}()
		var zero T
		return zero, ctx.Err()
	}
}

func retryRefused[T any](ctx context.Context, open func() (T, error)) (T, error) {
	interval := openRetryInterval
	for attempt := 0; ; attempt++ {
		res, err := open()
		if err == nil || !isChannelRefused(err) || attempt == openRetries {
			return res, err
		}
		select {
		case <-ctx.Done():
			return res, ctx.Err()
		case <-time.After(interval):
		}
		interval *= 2
	}
}

// isTransportError separates a dead connection from sshd refusing one more channel.
func isTransportError(err error) bool {
	if isChannelRefused(err) {
		return false
	}
	var netErr net.Error
	return errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) ||
		errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) || errors.As(err, &netErr)
}

func isChannelRefused(err error) bool {
	var openErr *ssh.OpenChannelError
	return errors.As(err, &openErr)
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
