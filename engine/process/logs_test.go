package process

import (
	"io"
	"slices"
	"testing"

	"github.com/cockroachdb/errors"

	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestJournalArgv(t *testing.T) {
	tests := []struct {
		name string
		opts *enginetypes.VirtualizationLogStreamOptions
		want []string
	}{
		{
			"defaults",
			&enginetypes.VirtualizationLogStreamOptions{ID: "w1"},
			[]string{"journalctl", "-u", "eru-w1.service", "-o", "cat"},
		},
		{
			"follow with a tail",
			&enginetypes.VirtualizationLogStreamOptions{ID: "w1", Follow: true, Tail: "10"},
			[]string{"journalctl", "-u", "eru-w1.service", "-o", "cat", "-f", "-n", "10"},
		},
		{
			"a time window",
			&enginetypes.VirtualizationLogStreamOptions{ID: "w1", Since: "-1h", Until: "now"},
			[]string{"journalctl", "-u", "eru-w1.service", "-o", "cat", "--since", "-1h", "--until", "now"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := journalArgv(unitName(tt.opts.ID), tt.opts); !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVirtualizationLogsBuffersWhenNotFollowing(t *testing.T) {
	runner := &fakeRunner{respond: func(string) *result { return &result{Stdout: "hello\n"} }}
	e := testEngine(t, runner)

	stdout, stderr, err := e.VirtualizationLogs(t.Context(), &enginetypes.VirtualizationLogStreamOptions{ID: "w1"})
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	if stderr != nil {
		t.Error("journald merges the streams, so stderr must stay nil")
	}
	body, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(body) != "hello\n" {
		t.Errorf("got %q, want %q", body, "hello\n")
	}
}

func TestVirtualizationLogsFollowClosesTheSession(t *testing.T) {
	running := &fakeSession{stdout: "line\n"}
	runner := &fakeRunner{started: running}
	e := testEngine(t, runner)

	stdout, _, err := e.VirtualizationLogs(t.Context(), &enginetypes.VirtualizationLogStreamOptions{ID: "w1", Follow: true})
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	if err = stdout.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !running.closed {
		t.Error("closing the log stream must close the ssh session")
	}
}

func TestVirtualizationAttachRefusesStdin(t *testing.T) {
	e := testEngine(t, &fakeRunner{})

	_, _, _, err := e.VirtualizationAttach(t.Context(), "w1", true, true)
	if !errors.Is(err, coretypes.ErrEngineNotImplemented) {
		t.Errorf("got %v, want ErrEngineNotImplemented", err)
	}
}
