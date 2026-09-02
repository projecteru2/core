package cocoon

import (
	"archive/tar"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	coretypes "github.com/projecteru2/core/types"
)

func TestVirtualizationCopyToStreamsOneTarEntry(t *testing.T) {
	session := &sshrunnertest.Session{}
	runner := &sshrunnertest.Fake{Started: []*sshrunnertest.Session{session}, Respond: runningRecord}
	e := testEngine(t, runner)

	if err := e.VirtualizationCopyTo(t.Context(), "w1", "/etc/app/app.conf", []byte("key=value\n"), 1000, 1000, 0o640); err != nil {
		t.Fatalf("copy to: %v", err)
	}
	want := sshrunner.Quote([]string{testBinary, "vm", "exec", "-i", "w1", "--", "tar", "-x", "-P", "-f", "-"})
	if len(runner.Lines()) != 2 || runner.Lines()[1] != want {
		t.Fatalf("got %q, want the state check then %q", runner.Lines(), want)
	}
	archive := tar.NewReader(strings.NewReader(session.In()))
	header, err := archive.Next()
	if err != nil {
		t.Fatalf("read the entry: %v", err)
	}
	if header.Name != "/etc/app/app.conf" || header.Uid != 1000 || header.Gid != 1000 || header.Mode != 0o640 {
		t.Errorf("got header %+v, want the target with its ownership and mode", header)
	}
	body, _ := io.ReadAll(archive)
	if string(body) != "key=value\n" {
		t.Errorf("got %q, want the content", body)
	}
	if !session.Closed() {
		t.Error("the session must be closed once the tar is written")
	}
}

func TestVirtualizationCopyToReportsTheGuestFailure(t *testing.T) {
	runner := &sshrunnertest.Fake{Started: []*sshrunnertest.Session{{Code: 2, Err: "tar: cannot open"}}, Respond: runningRecord}
	e := testEngine(t, runner)

	err := e.VirtualizationCopyTo(t.Context(), "w1", "/etc/app.conf", []byte("x"), 0, 0, 0o644)
	if err == nil || !strings.Contains(err.Error(), "tar: cannot open") {
		t.Errorf("got %v, want the guest's stderr", err)
	}
}

func TestVirtualizationCopyToRefusesAGuestThatHasNotBooted(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result {
		return &sshrunner.Result{Stdout: storedRecord + "\n" + stoppedVM}
	}}
	e := testEngine(t, runner)

	err := e.VirtualizationCopyTo(t.Context(), "w1", "/etc/app.conf", []byte("x"), 0, 0, 0o644)
	if !errors.Is(err, coretypes.ErrInvaildWorkloadOps) {
		t.Errorf("got %v, want ErrInvaildWorkloadOps", err)
	}
	if len(runner.Lines()) != 1 {
		t.Errorf("got %q, want the state check and no exec", runner.Lines())
	}
}

func TestVirtualizationCopyFromReadsTheTarEntry(t *testing.T) {
	buf := &bytes.Buffer{}
	archive := tar.NewWriter(buf)
	if err := archive.WriteHeader(&tar.Header{Name: "/etc/app.conf", Size: 5, Mode: 0o600, Uid: 7, Gid: 8}); err != nil {
		t.Fatalf("header: %v", err)
	}
	if _, err := archive.Write([]byte("hello")); err != nil {
		t.Fatalf("body: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	runner := &sshrunnertest.Fake{Started: []*sshrunnertest.Session{{Out: buf.String()}}}
	e := testEngine(t, runner)

	content, uid, gid, mode, err := e.VirtualizationCopyFrom(t.Context(), "w1", "/etc/app.conf")
	if err != nil {
		t.Fatalf("copy from: %v", err)
	}
	want := sshrunner.Quote([]string{testBinary, "vm", "exec", "w1", "--", "tar", "-c", "-P", "-f", "-", "/etc/app.conf"})
	if runner.Lines()[0] != want {
		t.Errorf("got %q, want %q", runner.Lines()[0], want)
	}
	if string(content) != "hello" || uid != 7 || gid != 8 || mode != 0o600 {
		t.Errorf("got %q %d %d %o, want the entry's content, ownership and mode", content, uid, gid, mode)
	}
}

func TestVirtualizationCopyFromReportsAMissingFile(t *testing.T) {
	runner := &sshrunnertest.Fake{Started: []*sshrunnertest.Session{{Code: 2}}}
	e := testEngine(t, runner)

	if _, _, _, _, err := e.VirtualizationCopyFrom(t.Context(), "w1", "/missing"); !errors.Is(err, coretypes.ErrWorkloadNotExists) {
		t.Errorf("got %v, want ErrWorkloadNotExists", err)
	}
}
