package containerd

import (
	"io"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/containerd/containerd/v2/client"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
)

func TestAttachRunsCtrAttachOverTheSession(t *testing.T) {
	running := &sshrunnertest.Session{Out: "hello\n", Err: "oops\n"}
	runner := &sshrunnertest.Fake{Started: running}
	e := testEngine(t, runner)

	stdout, stderr, stdin, err := e.startAttach(t.Context(), "app_web_abc123", true, nil)
	if err != nil {
		t.Fatalf("attach: %v", err)
	}

	want := sshrunner.Quote([]string{
		ctrBinary, "--address", defaultSocket, "--namespace", defaultNamespace, "tasks", "attach", "app_web_abc123",
	})
	if len(runner.Lines()) != 1 || runner.Lines()[0] != want {
		t.Fatalf("got %q, want %q", runner.Lines(), want)
	}
	opts := runner.Options()
	if len(opts) != 1 || !opts[0].Stdin || !opts[0].TTY {
		t.Errorf("got %+v, want stdin and a pty", opts)
	}

	if body, readErr := io.ReadAll(stdout); readErr != nil || string(body) != "hello\n" {
		t.Errorf("got %q %v, want the session's stdout", body, readErr)
	}
	if body, readErr := io.ReadAll(stderr); readErr != nil || string(body) != "oops\n" {
		t.Errorf("got %q %v, want the session's stderr", body, readErr)
	}
	if _, writeErr := stdin.Write([]byte("ping\n")); writeErr != nil {
		t.Fatalf("stdin: %v", writeErr)
	}
	if running.In() != "ping\n" {
		t.Errorf("got %q, want the bytes to reach the session's stdin", running.In())
	}
}

func TestAttachWithoutATerminalAsksForNoPty(t *testing.T) {
	runner := &sshrunnertest.Fake{Started: &sshrunnertest.Session{}}
	e := testEngine(t, runner)

	if _, _, _, err := e.startAttach(t.Context(), "app_web_abc123", false, nil); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if opts := runner.Options(); len(opts) != 1 || opts[0].TTY {
		t.Errorf("got %+v, want no pty: ctr only takes a console when the spec has one", opts)
	}
}

func TestVirtualizationResizeGoesToTheAttachedPty(t *testing.T) {
	running := &sshrunnertest.Session{}
	e := testEngine(t, &sshrunnertest.Fake{Started: running})

	if _, _, _, err := e.startAttach(t.Context(), "app_web_abc123", true, nil); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if err := e.VirtualizationResize(t.Context(), "app_web_abc123", 24, 80); err != nil {
		t.Fatalf("resize: %v", err)
	}
	if height, width := running.Resized(); height != 24 || width != 80 {
		t.Errorf("got %dx%d, want 24x80", height, width)
	}
}

func TestVirtualizationResizeWithoutAnAttachIsRefused(t *testing.T) {
	e := testEngine(t, &sshrunnertest.Fake{})

	if err := e.VirtualizationResize(t.Context(), "app_web_abc123", 24, 80); !errors.Is(err, errAttachNotFound) {
		t.Errorf("got %v, want errAttachNotFound", err)
	}
}

func TestVirtualizationWaitTakesTheExitTheAttachRegistered(t *testing.T) {
	exited := make(chan client.ExitStatus, 1)
	exited <- *client.NewExitStatus(3, time.Now(), nil)
	running := &sshrunnertest.Session{}
	e := testEngine(t, &sshrunnertest.Fake{Started: running})

	if _, _, _, err := e.startAttach(t.Context(), "app_web_abc123", true, exited); err != nil {
		t.Fatalf("attach: %v", err)
	}
	result, err := e.VirtualizationWait(t.Context(), "app_web_abc123", "")
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if result.Code != 3 {
		t.Errorf("got %d, want 3: ctr deletes the task, so the watch is the only exit left", result.Code)
	}
	if _, ok := e.attaches["app_web_abc123"]; ok {
		t.Error("the watch must be consumed once waited on")
	}
}

func TestAttachClosesTheSessionWithTheStream(t *testing.T) {
	running := &sshrunnertest.Session{}
	e := testEngine(t, &sshrunnertest.Fake{Started: running})

	stdout, _, _, err := e.startAttach(t.Context(), "app_web_abc123", true, nil)
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	if err = stdout.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !running.Closed() {
		t.Error("closing the attach stream must close the ssh session")
	}
}
