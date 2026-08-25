package sshrunner

import (
	"io"
	"net"
	"syscall"
	"testing"

	"github.com/cockroachdb/errors"
	"golang.org/x/crypto/ssh"
)

func TestIsTransportError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"a refused channel is sshd's session limit, not a dead link", &ssh.OpenChannelError{Reason: ssh.Prohibited}, false},
		{"end of file", io.EOF, true},
		{"a closed connection", net.ErrClosed, true},
		{"a broken pipe", syscall.EPIPE, true},
		{"a reset connection", syscall.ECONNRESET, true},
		{"a dial timeout", &net.OpError{Err: errors.New("timeout")}, true},
		{"an unrelated failure", errors.New("ssh: unexpected packet"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransportError(tt.err); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSSHRunnerBoundsConcurrentSessions(t *testing.T) {
	runner := newSSHRunner("127.0.0.1:22", &ssh.ClientConfig{})

	if !runner.sessions.TryAcquire(maxSessions) {
		t.Fatalf("a fresh runner must allow %d sessions", maxSessions)
	}
	if runner.sessions.TryAcquire(1) {
		t.Errorf("session %d must queue instead of opening", maxSessions+1)
	}
	runner.sessions.Release(maxSessions)
}
