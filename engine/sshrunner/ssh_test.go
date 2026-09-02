package sshrunner

import (
	"context"
	"io"
	"net"
	"syscall"
	"testing"
	"testing/synctest"

	"github.com/cockroachdb/errors"
	"golang.org/x/crypto/ssh"
)

var refusedChannel = &ssh.OpenChannelError{Reason: ssh.ConnectionFailed, Message: "open failed"}

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

func TestRetryRefused(t *testing.T) {
	tests := []struct {
		name     string
		dials    []error
		want     error
		attempts int
	}{
		{"an accepted forward is dialled once", []error{nil}, nil, 1},
		{"a forward the node accepts on the second try succeeds", []error{refusedChannel, nil}, nil, 2},
		{"a forward the node keeps refusing gives up", []error{refusedChannel, refusedChannel, refusedChannel, refusedChannel, refusedChannel}, refusedChannel, 5},
		{"a refusal burst longer than one interval is absorbed", []error{refusedChannel, refusedChannel, refusedChannel, nil}, nil, 4},
		{"a dead transport is not a refusal", []error{io.EOF}, io.EOF, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				attempts := 0
				_, err := retryRefused(t.Context(), func() (net.Conn, error) {
					attempts++
					return nil, tt.dials[attempts-1]
				})
				if !errors.Is(err, tt.want) {
					t.Errorf("got %v, want %v", err, tt.want)
				}
				if attempts != tt.attempts {
					t.Errorf("got %d dials, want %d", attempts, tt.attempts)
				}
			})
		})
	}
}

func TestRetryRefusedStopsOnADoneContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	attempts := 0
	_, err := retryRefused(ctx, func() (net.Conn, error) {
		attempts++
		return nil, refusedChannel
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("got %v, want a cancelled context", err)
	}
	if attempts != 1 {
		t.Errorf("got %d dials, want 1", attempts)
	}
}

func TestBoundedGivesUpOnADoneContextAndClosesTheLateResult(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		release := make(chan struct{})
		late := &closeRecorder{}
		result := make(chan error, 1)
		go func() {
			_, err := bounded(ctx, nil, func(*ssh.Client) (io.Closer, error) {
				<-release
				return late, nil
			})
			result <- err
		}()
		synctest.Wait()
		cancel()
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want a cancelled context", err)
		}
		close(release)
		synctest.Wait()
		if !late.closed {
			t.Error("a session opened after the caller left must be closed")
		}
	})
}

func TestIsTransportErrorCountsAContextDeadline(t *testing.T) {
	if !isTransportError(context.DeadlineExceeded) {
		t.Skip("a context deadline no longer looks like a dead link")
	}
}

func TestBoundedReturnsTheResultOfAnOpenThatFinishes(t *testing.T) {
	want := &closeRecorder{}
	got, err := bounded(t.Context(), nil, func(*ssh.Client) (io.Closer, error) { return want, nil })
	if err != nil || got != want {
		t.Fatalf("got %v, %v; want the opened value", got, err)
	}
	if want.closed {
		t.Error("a result handed to the caller must stay open")
	}
}

func TestConnectKeepsAClientAnotherCallerRenewed(t *testing.T) {
	runner := newSSHRunner("127.0.0.1:1", &ssh.ClientConfig{})
	current := &ssh.Client{}
	runner.client = current

	got, err := runner.connect(t.Context(), &ssh.Client{})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if got != current {
		t.Error("a stale client that is no longer current must not trigger a redial")
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

type closeRecorder struct{ closed bool }

func (c *closeRecorder) Close() error {
	c.closed = true
	return nil
}
