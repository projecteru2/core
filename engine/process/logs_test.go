package process

import (
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"

	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestJournalFlags(t *testing.T) {
	tests := []struct {
		name string
		opts *enginetypes.VirtualizationLogStreamOptions
		want []string
	}{
		{"defaults", &enginetypes.VirtualizationLogStreamOptions{ID: "w1"}, []string{"-o", "cat"}},
		{
			"a tail",
			&enginetypes.VirtualizationLogStreamOptions{ID: "w1", Tail: "10"},
			[]string{"-o", "cat", "-n", "10"},
		},
		{
			"unix seconds pass through as an absolute stamp",
			&enginetypes.VirtualizationLogStreamOptions{ID: "w1", Since: "1700000000"},
			[]string{"-o", "cat", "--since", "@1700000000"},
		},
		{
			"an RFC3339 window becomes absolute stamps",
			&enginetypes.VirtualizationLogStreamOptions{ID: "w1", Since: "2023-11-14T22:13:20Z", Until: "2023-11-14T23:13:20Z"},
			[]string{"-o", "cat", "--since", "@1700000000", "--until", "@1700003600"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := journalFlags(tt.opts)
			if err != nil {
				t.Fatalf("flags: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJournalFlagsRejectAnUnreadableTimestamp(t *testing.T) {
	_, err := journalFlags(&enginetypes.VirtualizationLogStreamOptions{ID: "w1", Since: "yesterday"})
	if !errors.Is(err, coretypes.ErrInvaildWorkloadOps) {
		t.Errorf("got %v, want ErrInvaildWorkloadOps", err)
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
	want := quote([]string{"journalctl", "-u", "eru-w1.service", "-o", "cat"})
	if len(runner.lines) != 1 || runner.lines[0] != want {
		t.Errorf("got %q, want %q", runner.lines, want)
	}
	body, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(body) != "hello\n" {
		t.Errorf("got %q, want %q", body, "hello\n")
	}
}

func TestVirtualizationLogsFollowStopsWithTheUnit(t *testing.T) {
	running := &fakeSession{stdout: "line\n"}
	runner := &fakeRunner{started: running}
	e := testEngine(t, runner)

	stdout, _, err := e.VirtualizationLogs(t.Context(), &enginetypes.VirtualizationLogStreamOptions{ID: "w1", Follow: true})
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	want := quote(shell(followScript, "eru-w1.service", "-f", "-o", "cat"))
	if len(runner.lines) != 1 || runner.lines[0] != want {
		t.Fatalf("got %q, want %q", runner.lines, want)
	}
	if !strings.Contains(followScript, "kill") {
		t.Error("the follow script must kill journalctl once the unit leaves running")
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
