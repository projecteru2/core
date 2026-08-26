package sshrunner

import (
	"context"
	"io"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"

	coretypes "github.com/projecteru2/core/types"
)

func TestNodeInfoReadsTheNodeItself(t *testing.T) {
	runner := &stubRunner{stdout: "machine-1\n8\n16384\n1048576\n"}

	info, err := NodeInfo(t.Context(), runner, "/var/lib/eru")
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if info.ID != "machine-1" || info.NCPU != 8 {
		t.Errorf("got %+v, want the node's own identity", info)
	}
	if info.MemTotal != 16384*kiB || info.StorageTotal != 1048576*kiB {
		t.Errorf("got %d/%d, want kibibytes scaled to bytes", info.MemTotal, info.StorageTotal)
	}
	if runner.line != Quote(Shell(infoScript, "/var/lib/eru")) {
		t.Errorf("got %q, want the info script rooted at the engine's directory", runner.line)
	}
}

func TestNodeInfoRefusesAnUnreadableNode(t *testing.T) {
	runner := &stubRunner{stdout: "machine-1\n"}

	if _, err := NodeInfo(t.Context(), runner, "/var/lib/eru"); !errors.Is(err, coretypes.ErrInvaildNodeEndpoint) {
		t.Errorf("got %v, want ErrInvaildNodeEndpoint", err)
	}
}

func TestInfoScriptCreatesTheRunDirAndKeepsItsFailuresVisible(t *testing.T) {
	if !strings.HasPrefix(infoScript, "set -e\nmkdir -p") {
		t.Error("the info script must create the run dir under set -e")
	}
	if strings.Contains(infoScript, "2>/dev/null") {
		t.Error("the info script must not swallow the errors that make a node look empty")
	}
}

func TestNodeInfoRefusesAFieldThatIsNotANumber(t *testing.T) {
	runner := &stubRunner{stdout: "machine-1\n8\n\n1048576\n"}

	if _, err := NodeInfo(t.Context(), runner, "/var/lib/eru"); !errors.Is(err, coretypes.ErrInvaildNodeEndpoint) {
		t.Errorf("got %v, want ErrInvaildNodeEndpoint", err)
	}
}

func TestNodeInfoReportsANonZeroExit(t *testing.T) {
	runner := &stubRunner{code: 1, stderr: "df: not found"}

	if _, err := NodeInfo(t.Context(), runner, "/var/lib/eru"); err == nil {
		t.Error("a failed script must not be parsed as node info")
	}
}

type stubRunner struct {
	line   string
	stdout string
	stderr string
	code   int
}

func (r *stubRunner) Run(_ context.Context, line string, _ io.Reader) (*Result, error) {
	r.line = line
	return &Result{Stdout: r.stdout, Stderr: r.stderr, Code: r.code}, nil
}

func (r *stubRunner) Start(context.Context, string, *StartOptions) (Session, error) {
	return nil, os.ErrInvalid
}

func (r *stubRunner) Files(context.Context) (Files, error) {
	return nil, os.ErrInvalid
}

func (r *stubRunner) Dial(context.Context, string, string) (net.Conn, error) {
	return nil, os.ErrInvalid
}

func (r *stubRunner) Close() error {
	return nil
}
