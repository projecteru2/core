package containerd

import (
	"context"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
)

var relayHold = make(chan struct{})

func TestRelayFifosParksASessionOnEveryNodeFifo(t *testing.T) {
	runner := &sshrunnertest.Fake{Started: []*sshrunnertest.Session{parked(), {}, {}}}
	e := testEngine(t, runner)

	creator, err := e.relayFifos(t.Context(), "app_web_abc123")
	if err != nil {
		t.Fatalf("relay: %v", err)
	}

	dir := workloadRoot + "/app_web_abc123/" + fifoDirName
	want := []string{
		sshrunner.Quote(sshrunner.Shell(fifoMakeScript, dir, dir+"/stdin", dir+"/stdout", dir+"/stderr")),
		sshrunner.Quote(sshrunner.Shell(fifoWriteScript, dir+"/stdin")),
		sshrunner.Quote([]string{"cat", dir + "/stdout"}),
		sshrunner.Quote([]string{"cat", dir + "/stderr"}),
	}
	if !slices.Equal(runner.Lines(), want) {
		t.Fatalf("got %q, want %q", runner.Lines(), want)
	}
	if !strings.Contains(want[1], `'exec cat > "$1"'`) {
		t.Errorf("got %q, want the redirect inside a quoted script: handed to cat as argv it becomes a filename", want[1])
	}
	if opts := runner.Options(); len(opts) != 3 || !opts[0].Stdin || opts[1].Stdin || opts[2].Stdin ||
		opts[0].TTY || opts[1].TTY || opts[2].TTY {
		t.Errorf("got %+v, want stdin on the writer alone and a pipe everywhere: only a pipe carries the end of the input", opts)
	}
	if _, ok := e.attaches["app_web_abc123"]; !ok {
		t.Error("the relays are the attach, and must be registered as one")
	}

	stdio, err := creator("app_web_abc123")
	if err != nil {
		t.Fatalf("creator: %v", err)
	}
	config := stdio.Config()
	if config.Terminal || config.Stdin != dir+"/stdin" || config.Stdout != dir+"/stdout" || config.Stderr != dir+"/stderr" {
		t.Errorf("got %+v, want the task created on the node's own fifos", config)
	}
}

func TestRelayFifosClosesWhatItOpenedWhenASessionIsRefused(t *testing.T) {
	opened := []*sshrunnertest.Session{{}, {}}
	e := testEngine(t, &sshrunnertest.Fake{Started: opened, StartErr: errors.New("no more sessions")})

	if _, err := e.relayFifos(t.Context(), "app_web_abc123"); err == nil {
		t.Fatal("a relay core could not open must not report a started workload")
	}

	for i, sess := range opened {
		if !sess.Closed() {
			t.Errorf("relay %d leaked its session", i)
		}
	}
	if _, ok := e.attaches["app_web_abc123"]; ok {
		t.Error("a failed relay leaves no attach behind for the next one to find")
	}
}

func TestAttachHandsBackTheRelayedFifos(t *testing.T) {
	stdin := parked()
	e := testEngine(t, &sshrunnertest.Fake{Started: []*sshrunnertest.Session{
		stdin, {Out: "hello\n"}, {Out: "oops\n"},
	}})

	if _, err := e.relayFifos(t.Context(), "app_web_abc123"); err != nil {
		t.Fatalf("relay: %v", err)
	}
	stdout, stderr, in, err := e.VirtualizationAttach(t.Context(), "app_web_abc123", true, true)
	if err != nil {
		t.Fatalf("attach: %v", err)
	}

	if body, readErr := io.ReadAll(stdout); readErr != nil || string(body) != "hello\n" {
		t.Errorf("got %q %v, want what the stdout fifo relay reads", body, readErr)
	}
	if body, readErr := io.ReadAll(stderr); readErr != nil || string(body) != "oops\n" {
		t.Errorf("got %q %v, want what the stderr fifo relay reads", body, readErr)
	}
	if _, writeErr := in.Write([]byte("ping\n")); writeErr != nil {
		t.Fatalf("stdin: %v", writeErr)
	}
	if stdin.In() != "ping\n" {
		t.Errorf("got %q, want the bytes to reach the fifo writer's stdin", stdin.In())
	}
	if err = stdout.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if stdin.Closed() {
		t.Error("the caller's stream close must not kill the relays the workload runs on")
	}
}

func TestAttachWithoutAStartedWorkloadIsRefused(t *testing.T) {
	e := testEngine(t, &sshrunnertest.Fake{})

	_, _, _, err := e.VirtualizationAttach(t.Context(), "app_web_abc123", true, true)

	if !errors.Is(err, errAttachNotFound) {
		t.Errorf("got %v, want errAttachNotFound", err)
	}
}

func TestReleaseAttachEndsEveryRelay(t *testing.T) {
	opened := []*sshrunnertest.Session{parked(), {}, {}}
	e := testEngine(t, &sshrunnertest.Fake{Started: opened})

	if _, err := e.relayFifos(t.Context(), "app_web_abc123"); err != nil {
		t.Fatalf("relay: %v", err)
	}
	e.releaseAttach("app_web_abc123")

	for i, sess := range opened {
		if !sess.Closed() {
			t.Errorf("relay %d outlived the workload", i)
		}
	}
	if _, ok := e.attaches["app_web_abc123"]; ok {
		t.Error("a consumed exit leaves no attach behind")
	}
}

func TestRelayFifosReplacesTheRelaysOfAnEarlierStart(t *testing.T) {
	first := []*sshrunnertest.Session{parked(), {}, {}}
	runner := &sshrunnertest.Fake{Started: slices.Concat(first, []*sshrunnertest.Session{parked(), {}, {}})}
	e := testEngine(t, runner)

	if _, err := e.relayFifos(t.Context(), "app_web_abc123"); err != nil {
		t.Fatalf("relay: %v", err)
	}
	if _, err := e.relayFifos(t.Context(), "app_web_abc123"); err != nil {
		t.Fatalf("restart: %v", err)
	}

	for i, sess := range first {
		if !sess.Closed() {
			t.Errorf("relay %d of the first start holds a session slot on a fifo that is gone", i)
		}
	}
}

func TestTheRelaysOutliveTheDeployRequestThatStartedThem(t *testing.T) {
	stdin := parked()
	runner := &sshrunnertest.Fake{Started: []*sshrunnertest.Session{stdin, {}, {}}}
	e := testEngine(t, runner)

	ctx, cancel := context.WithCancel(t.Context())
	if _, err := e.relayFifos(ctx, "app_web_abc123"); err != nil {
		t.Fatalf("relay: %v", err)
	}
	cancel()

	held := runner.Contexts()
	if len(held) != relayStreams {
		t.Fatalf("got %d relays, want %d", len(held), relayStreams)
	}
	for i, sessCtx := range held {
		if sessCtx.Err() != nil {
			t.Errorf("relay %d dies with the deploy request, and the workload outlives that", i)
		}
	}

	_, _, in, err := e.VirtualizationAttach(t.Context(), "app_web_abc123", true, true)
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if _, err = in.Write([]byte("ping\n")); err != nil {
		t.Fatalf("stdin: %v", err)
	}
	if stdin.In() != "ping\n" {
		t.Errorf("got %q, want a write after the request ended to still reach the fifo", stdin.In())
	}
}

func TestARelayDeathIsSurfacedWithTheReasonTheNodeGave(t *testing.T) {
	dying := &sshrunnertest.Session{Code: 1, Err: "sh: cannot create /var/lib/eru/containerd/w1/fifo/stdin: Permission denied\n"}
	relay, _ := watchedAttach()

	relay.watch(t.Context(), "app_web_abc123", stdinStream, dying)

	select {
	case err := <-relay.died:
		if !strings.Contains(err.Error(), "Permission denied") {
			t.Errorf("got %v, want the node's own account of the failure", err)
		}
		if !strings.Contains(err.Error(), "stdin") {
			t.Errorf("got %v, want the stream named", err)
		}
	default:
		t.Fatal("a relay that dies under the workload must not die silently")
	}
}

func TestARelayThatEndsCleanlyIsNotReported(t *testing.T) {
	relay, closed := watchedAttach()

	relay.watch(t.Context(), "app_web_abc123", "stdout", &sshrunnertest.Session{})

	if len(relay.died) != 0 {
		t.Error("a relay ends with the task it serves, and that is not a failure")
	}
	if *closed {
		t.Error("only the stdin relay ending is the end of the workload's input")
	}
}

func TestARelayTheEngineReleasedIsNotReported(t *testing.T) {
	relay, closed := watchedAttach()
	relay.close()

	relay.watch(t.Context(), "app_web_abc123", stdinStream, &sshrunnertest.Session{Code: 255, Err: "killed\n"})

	if len(relay.died) != 0 {
		t.Error("tearing an attach down is not a relay failure")
	}
	if *closed {
		t.Error("a workload core is done with needs no stdin closer")
	}
}

func TestTheStdinRelayEndingClosesTheTasksInput(t *testing.T) {
	relay, closed := watchedAttach()

	relay.watch(t.Context(), "app_web_abc123", stdinStream, &sshrunnertest.Session{})

	if !*closed {
		t.Error("the shim holds the fifo open itself, so only CloseIO is the workload's stdin EOF")
	}
	if len(relay.died) != 0 {
		t.Error("core closing its write side is how a lambda ends, not a failure")
	}
}

func TestAStdinRelayThatDiedDoesNotCloseTheTasksInput(t *testing.T) {
	relay, closed := watchedAttach()

	relay.watch(t.Context(), "app_web_abc123", stdinStream, &sshrunnertest.Session{Code: 1, Err: "cannot create fifo\n"})

	if *closed {
		t.Error("a relay that died never carried the input, so its end is not an EOF")
	}
	if len(relay.died) != 1 {
		t.Error("a relay that died must still be surfaced")
	}
}

func TestStartRefusesAWorkloadWhoseRelayAlreadyDied(t *testing.T) {
	e := testEngine(t, &sshrunnertest.Fake{Started: []*sshrunnertest.Session{parked(), {}, {}}})

	if _, err := e.relayFifos(t.Context(), "app_web_abc123"); err != nil {
		t.Fatalf("relay: %v", err)
	}
	relay := e.attaches["app_web_abc123"]
	relay.watch(t.Context(), "app_web_abc123", stdinStream, &sshrunnertest.Session{Code: 1, Err: "cannot create fifo\n"})

	if err := e.relayFailure("app_web_abc123"); err == nil {
		t.Error("a workload whose stdin relay is gone would hang forever, so the start must fail")
	}
	if err := e.relayFailure("app_web_other"); err != nil {
		t.Errorf("got %v, want nothing for a workload with no relays", err)
	}
}

func TestVirtualizationResizeIsANoOpOnAPipe(t *testing.T) {
	e := testEngine(t, &sshrunnertest.Fake{})

	if err := e.VirtualizationResize(t.Context(), "app_web_abc123", 24, 80); err != nil {
		t.Errorf("got %v, want a workload with no terminal to accept the resize it has nothing to do with", err)
	}
}

func parked() *sshrunnertest.Session {
	return &sshrunnertest.Session{Hold: relayHold}
}

func watchedAttach() (*attach, *bool) {
	closed := new(bool)
	return &attach{
		died:       make(chan error, relayStreams),
		closeStdin: func(context.Context) error { *closed = true; return nil },
	}, closed
}
