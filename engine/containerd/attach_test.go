package containerd

import (
	"context"
	"io"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	cerrdefs "github.com/containerd/errdefs"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
)

func TestInteractiveStartRunsCtrStartOverTheSession(t *testing.T) {
	running := &sshrunnertest.Session{Out: "hello\n", Err: "oops\n"}
	runner := &sshrunnertest.Fake{Started: running}
	e := testEngine(t, runner)

	if err := e.startInteractive(t.Context(), &fakeTaskContainer{id: "app_web_abc123"}); err != nil {
		t.Fatalf("start: %v", err)
	}

	want := sshrunner.Quote([]string{
		ctrBinary, "--address", defaultSocket, "--namespace", defaultNamespace, "tasks", "start", "app_web_abc123",
	})
	if len(runner.Lines()) != 1 || runner.Lines()[0] != want {
		t.Fatalf("got %q, want %q", runner.Lines(), want)
	}
	if opts := runner.Options(); len(opts) != 1 || !opts[0].Stdin || opts[0].TTY {
		t.Errorf("got %+v, want stdin on a plain pipe: only a pipe carries the end of the input", opts)
	}
	if _, ok := e.attaches["app_web_abc123"]; !ok {
		t.Error("the start session is the attach, and must be registered as one")
	}
}

func TestAttachHandsBackTheStartSessionsStreams(t *testing.T) {
	hold := make(chan struct{})
	t.Cleanup(func() { close(hold) })
	running := &sshrunnertest.Session{Out: "hello\n", Err: "oops\n", Hold: hold}
	e := testEngine(t, &sshrunnertest.Fake{Started: running})

	if err := e.startInteractive(t.Context(), &fakeTaskContainer{id: "app_web_abc123"}); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, stderr, stdin, err := e.VirtualizationAttach(t.Context(), "app_web_abc123", true, true)
	if err != nil {
		t.Fatalf("attach: %v", err)
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
	if err = stdout.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if running.Closed() {
		t.Error("the caller's stream close must not kill the session the workload runs under")
	}
}

func TestAttachWithoutAStartedWorkloadIsRefused(t *testing.T) {
	e := testEngine(t, &sshrunnertest.Fake{})

	_, _, _, err := e.VirtualizationAttach(t.Context(), "app_web_abc123", true, true)

	if !errors.Is(err, errAttachNotFound) {
		t.Errorf("got %v, want errAttachNotFound", err)
	}
}

func TestVirtualizationResizeGoesToTheStartedPty(t *testing.T) {
	running := &sshrunnertest.Session{}
	e := testEngine(t, &sshrunnertest.Fake{Started: running})

	if err := e.startInteractive(t.Context(), &fakeTaskContainer{id: "app_web_abc123"}); err != nil {
		t.Fatalf("start: %v", err)
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

func TestVirtualizationWaitTakesTheStartSessionsExit(t *testing.T) {
	running := &sshrunnertest.Session{Code: 3}
	e := testEngine(t, &sshrunnertest.Fake{Started: running})

	if err := e.startInteractive(t.Context(), &fakeTaskContainer{id: "app_web_abc123"}); err != nil {
		t.Fatalf("start: %v", err)
	}
	result, err := e.VirtualizationWait(t.Context(), "app_web_abc123", "")
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if result.Code != 3 {
		t.Errorf("got %d, want 3: ctr exits with the task's status", result.Code)
	}
	if _, ok := e.attaches["app_web_abc123"]; ok {
		t.Error("the watch must be consumed once waited on")
	}
	if !running.Closed() {
		t.Error("a finished workload must release its session")
	}
}

func TestInteractiveStartWaitsForTheTaskCtrMakes(t *testing.T) {
	hold := make(chan struct{})
	t.Cleanup(func() { close(hold) })
	e := testEngine(t, &sshrunnertest.Fake{Started: &sshrunnertest.Session{Hold: hold}})
	found := &fakeTaskContainer{id: "app_web_abc123", appearsAfter: 1}

	if err := e.startInteractive(t.Context(), found); err != nil {
		t.Fatalf("start: %v", err)
	}

	if found.polls != 2 {
		t.Errorf("got %d polls, want the start to wait for the task ctr creates", found.polls)
	}
	if _, ok := e.attaches["app_web_abc123"]; !ok {
		t.Error("the attach is registered once the task is there")
	}
}

func TestInteractiveStartFailsWhenCtrExitsBeforeItsTask(t *testing.T) {
	running := &sshrunnertest.Session{Code: 1}
	e := testEngine(t, &sshrunnertest.Fake{Started: running})
	found := &fakeTaskContainer{id: "app_web_abc123", never: true}

	err := e.startInteractive(t.Context(), found)

	if err == nil {
		t.Fatal("a workload whose task ctr never made must not report a started workload")
	}
	if _, ok := e.attaches["app_web_abc123"]; ok {
		t.Error("a failed start leaves no attach behind for the next one to find")
	}
}

type fakeTaskContainer struct {
	id           string
	appearsAfter int
	never        bool
	polls        int
}

func (f *fakeTaskContainer) ID() string {
	return f.id
}

func (f *fakeTaskContainer) Task(context.Context, cio.Attach) (client.Task, error) {
	f.polls++
	if f.never || f.polls <= f.appearsAfter {
		return nil, cerrdefs.ErrNotFound
	}
	return nil, nil
}
