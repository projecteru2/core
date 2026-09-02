package sshrunner

import (
	"context"
	"io"
	"net"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

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
			_, err := bounded(ctx, func() (io.Closer, error) {
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
	got, err := bounded(t.Context(), func() (io.Closer, error) { return want, nil })
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

func TestStreamPoolOpensAConnectionPerEightSessions(t *testing.T) {
	dials := 0
	pool := newStreamPool(func(context.Context) (streamConn, error) { dials++; return &fakeConn{}, nil })
	held := []*streamClient{}
	for range maxSessions + 1 {
		c, err := pool.acquire(t.Context())
		if err != nil {
			t.Fatalf("acquire: %v", err)
		}
		held = append(held, c)
	}
	if dials != 2 {
		t.Fatalf("got %d dials, want 2 for %d sessions", dials, maxSessions+1)
	}
	if held[0] != held[maxSessions-1] || held[0] == held[maxSessions] {
		t.Error("the first eight sessions share one connection and the ninth opens another")
	}
}

func TestStreamPoolClosesAConnectionLeftIdle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		conn := &fakeConn{}
		pool := newStreamPool(func(context.Context) (streamConn, error) { return conn, nil })
		c, err := pool.acquire(t.Context())
		if err != nil {
			t.Fatalf("acquire: %v", err)
		}
		pool.release(c)
		time.Sleep(streamIdle / 2)
		if _, err := pool.acquire(t.Context()); err != nil {
			t.Fatalf("acquire: %v", err)
		}
		time.Sleep(streamIdle)
		synctest.Wait()
		if conn.closed {
			t.Fatal("a connection taken again before its idle time must stay open")
		}
		pool.release(c)
		time.Sleep(streamIdle + time.Second)
		synctest.Wait()
		if !conn.closed || len(pool.conns) != 0 {
			t.Error("an idle connection must be closed and forgotten")
		}
	})
}

func TestStreamPoolWaitsWhenEveryConnectionIsFull(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		pool := newStreamPool(func(context.Context) (streamConn, error) { return &fakeConn{}, nil })
		held := []*streamClient{}
		for range maxStreamClients * maxSessions {
			c, err := pool.acquire(t.Context())
			if err != nil {
				t.Fatalf("acquire: %v", err)
			}
			held = append(held, c)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := pool.acquire(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want the caller to wait on its context once every connection is full", err)
		}
		pool.release(held[0])
		if _, err := pool.acquire(t.Context()); err != nil {
			t.Fatalf("a released slot must be handed out again: %v", err)
		}
	})
}

func TestOpenStreamDropsADeadConnection(t *testing.T) {
	dead, live := &fakeConn{err: io.EOF}, &fakeConn{}
	conns := []streamConn{dead, live}
	r := newSSHRunner("127.0.0.1:22", &ssh.ClientConfig{})
	r.streams = newStreamPool(func(context.Context) (streamConn, error) { c := conns[0]; conns = conns[1:]; return c, nil })

	stream, _, err := r.openStream(t.Context())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if stream.conn != live || !dead.closed {
		t.Error("a connection whose session open fails on the transport must be closed and replaced")
	}
}

func TestCloseOnDoneClosesWhenTheContextEnds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		c := &closeRecorder{}
		stop := closeOnDone(ctx, c)
		cancel()
		synctest.Wait()
		if !c.closed {
			t.Fatal("the closer must run when the context ends")
		}
		stop()
	})
}

type fakeConn struct {
	err    error
	closed bool
}

func (c *fakeConn) NewSession() (*ssh.Session, error) {
	return nil, c.err
}

func (c *fakeConn) Close() error {
	c.closed = true
	return nil
}

type closeRecorder struct{ closed bool }

func (c *closeRecorder) Close() error {
	c.closed = true
	return nil
}
