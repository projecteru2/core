package sshrunner

import (
	"io"
	"testing"

	"github.com/cockroachdb/errors"
)

func TestExecsReleaseAnExecWithItsExitCode(t *testing.T) {
	execs := NewExecs()
	running := &stubSession{code: 7}
	execs.Add("e1", running)

	if err := execs.Resize("e1", 24, 80); err != nil {
		t.Fatalf("resize: %v", err)
	}
	if running.height != 24 || running.width != 80 {
		t.Errorf("got %dx%d, want 24x80", running.height, running.width)
	}

	code, err := execs.ExitCode("e1")
	if err != nil {
		t.Fatalf("exit code: %v", err)
	}
	if code != 7 {
		t.Errorf("got %d, want 7", code)
	}
	if !running.closed {
		t.Error("a finished exec must release its session")
	}
}

func TestExecsRejectAnUnknownExec(t *testing.T) {
	execs := NewExecs()

	if _, err := execs.ExitCode("missing"); !errors.Is(err, errExecNotFound) {
		t.Errorf("got %v, want errExecNotFound", err)
	}
	if err := execs.Resize("missing", 24, 80); !errors.Is(err, errExecNotFound) {
		t.Errorf("got %v, want errExecNotFound", err)
	}
}

func TestExecsForgetAnExecOnceItHasEnded(t *testing.T) {
	execs := NewExecs()
	execs.Add("e1", &stubSession{})

	if _, err := execs.ExitCode("e1"); err != nil {
		t.Fatalf("exit code: %v", err)
	}
	if _, err := execs.ExitCode("e1"); !errors.Is(err, errExecNotFound) {
		t.Errorf("got %v, want errExecNotFound", err)
	}
}

type stubSession struct {
	code   int
	height uint
	width  uint
	closed bool
}

func (s *stubSession) Stdin() io.WriteCloser { return nil }

func (s *stubSession) Stdout() io.ReadCloser { return nil }

func (s *stubSession) Stderr() io.ReadCloser { return nil }

func (s *stubSession) Resize(height, width uint) error {
	s.height, s.width = height, width
	return nil
}

func (s *stubSession) Wait() (int, error) { return s.code, nil }

func (s *stubSession) Close() error {
	s.closed = true
	return nil
}
