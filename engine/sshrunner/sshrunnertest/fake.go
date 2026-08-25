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
	Started *Session

	mu    sync.Mutex
	lines []string
}

// Lines returns the command lines the engine has sent so far.
func (f *Fake) Lines() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.lines...)
}

func (f *Fake) Run(_ context.Context, line string, _ io.Reader) (*sshrunner.Result, error) {
	f.record(line)
	if f.Respond != nil {
		return f.Respond(line), nil
	}
	return &sshrunner.Result{}, nil
}

func (f *Fake) Start(_ context.Context, line string, _ *sshrunner.StartOptions) (sshrunner.Session, error) {
	f.record(line)
	return f.Started, nil
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

	mu     sync.Mutex
	closed bool
}

// Closed reports whether the engine has released the session.
func (s *Session) Closed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *Session) Stdin() io.WriteCloser {
	return nil
}

func (s *Session) Stdout() io.ReadCloser {
	return io.NopCloser(strings.NewReader(s.Out))
}

func (s *Session) Stderr() io.ReadCloser {
	return io.NopCloser(strings.NewReader(""))
}

func (s *Session) Resize(uint, uint) error {
	return nil
}

func (s *Session) Wait() (int, error) {
	return s.Code, nil
}

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}
