package sshrunner

import (
	"sync"

	"github.com/cockroachdb/errors"
)

var errExecNotFound = errors.New("exec not found")

type Execs struct {
	mu      sync.Mutex
	running map[string]Session
}

func NewExecs() *Execs {
	return &Execs{running: map[string]Session{}}
}

func (e *Execs) Add(execID string, sess Session) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.running[execID] = sess
}

func (e *Execs) Resize(execID string, height, width uint) error {
	e.mu.Lock()
	running, ok := e.running[execID]
	e.mu.Unlock()
	if !ok {
		return errors.Wrap(errExecNotFound, execID)
	}
	return running.Resize(height, width)
}

func (e *Execs) ExitCode(execID string) (int, error) {
	e.mu.Lock()
	running, ok := e.running[execID]
	delete(e.running, execID)
	e.mu.Unlock()
	if !ok {
		return -1, errors.Wrap(errExecNotFound, execID)
	}
	defer func() {
		_ = running.Close()
	}()
	return running.Wait()
}
