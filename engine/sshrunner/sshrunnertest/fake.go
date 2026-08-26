// Package sshrunnertest is a scripted runner every engine's tests drive a node with.
package sshrunnertest

import (
	"context"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/projecteru2/core/engine/sshrunner"
	coretypes "github.com/projecteru2/core/types"
)

var _ sshrunner.Runner = (*Fake)(nil)

// Fake records the command lines it is given and answers them from Respond.
type Fake struct {
	Respond func(line string) *sshrunner.Result
	// Started hands one session to each started command in order.
	Started []*Session
	// StartErr refuses every command started past the ones Started scripts.
	StartErr error

	mu    sync.Mutex
	lines []string
	opts  []*sshrunner.StartOptions
	ctxs  []context.Context
}

// Lines returns the command lines the engine has sent so far.
func (f *Fake) Lines() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.lines...)
}

// Options returns the stream shape the engine asked each started command for.
func (f *Fake) Options() []*sshrunner.StartOptions {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*sshrunner.StartOptions(nil), f.opts...)
}

// Contexts returns the context each started command was bound to, which is its lifetime.
func (f *Fake) Contexts() []context.Context {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]context.Context(nil), f.ctxs...)
}

func (f *Fake) Run(ctx context.Context, line string, _ io.Reader) (*sshrunner.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.record(line)
	if f.Respond != nil {
		return f.Respond(line), nil
	}
	return &sshrunner.Result{}, nil
}

func (f *Fake) Start(ctx context.Context, line string, opts *sshrunner.StartOptions) (sshrunner.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.record(line)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opts = append(f.opts, opts)
	f.ctxs = append(f.ctxs, ctx)
	if len(f.opts) > len(f.Started) {
		if f.StartErr != nil {
			return nil, f.StartErr
		}
		return &Session{}, nil
	}
	return f.Started[len(f.opts)-1], nil
}

func (f *Fake) Files(context.Context) (sshrunner.Files, error) {
	return nil, coretypes.ErrEngineNotImplemented
}

func (f *Fake) Dial(context.Context, string, string) (net.Conn, error) {
	return nil, coretypes.ErrEngineNotImplemented
}

func (f *Fake) Close() error {
	return nil
}

func (f *Fake) record(line string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lines = append(f.lines, line)
}

// Session is one scripted command with its exit code and output.
type Session struct {
	Code int
	Out  string
	Err  string
	Hold <-chan struct{}

	mu     sync.Mutex
	closed bool
	in     strings.Builder
	height uint
	width  uint
}

// Closed reports whether the engine has released the session.
func (s *Session) Closed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// In returns what the engine has written to the session's stdin.
func (s *Session) In() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.in.String()
}

// Resized returns the terminal geometry the engine last asked for.
func (s *Session) Resized() (height, width uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.height, s.width
}

func (s *Session) Stdin() io.WriteCloser {
	return &sessionStdin{sess: s}
}

func (s *Session) Stdout() io.ReadCloser {
	return io.NopCloser(strings.NewReader(s.Out))
}

func (s *Session) Stderr() io.ReadCloser {
	return io.NopCloser(strings.NewReader(s.Err))
}

func (s *Session) Resize(height, width uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.height, s.width = height, width
	return nil
}

func (s *Session) Wait() (int, error) {
	if s.Hold != nil {
		<-s.Hold
	}
	return s.Code, nil
}

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

type sessionStdin struct {
	sess *Session
}

func (w *sessionStdin) Write(p []byte) (int, error) {
	w.sess.mu.Lock()
	defer w.sess.mu.Unlock()
	return w.sess.in.Write(p)
}

func (w *sessionStdin) Close() error {
	return nil
}
