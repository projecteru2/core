package cocoon

import (
	"strings"
	"testing"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

const testSnapshotJSON = `{"id":"S1","name":"` + testSnap + `","cpu":4}`

func TestRawEngineRendersEachSnapshotOp(t *testing.T) {
	params := []byte(`{"name":"` + testSnap + `"}`)
	tests := []struct {
		name     string
		op       string
		params   []byte
		stdout   string
		wantArgv []string
		wantData string
	}{
		{
			name:     "save",
			op:       opSnapshotSave,
			params:   params,
			stdout:   testSnapshotJSON,
			wantArgv: sshrunner.Shell(saveScript, testBinary, testSnap, "w1"),
			wantData: testSnapshotJSON,
		},
		{
			name:     "list",
			op:       opSnapshotList,
			stdout:   "[" + testSnapshotJSON + "]",
			wantArgv: []string{testBinary, "snapshot", "list", "--format", "json"},
			wantData: "[" + testSnapshotJSON + "]",
		},
		{
			name:     "an empty list in prose",
			op:       opSnapshotList,
			stdout:   "No snapshots found.\n",
			wantArgv: []string{testBinary, "snapshot", "list", "--format", "json"},
			wantData: "[]",
		},
		{
			name:     "inspect",
			op:       opSnapshotInspect,
			params:   params,
			stdout:   testSnapshotJSON,
			wantArgv: []string{testBinary, "snapshot", "inspect", testSnap},
			wantData: testSnapshotJSON,
		},
		{
			name:     "remove",
			op:       opSnapshotRemove,
			params:   params,
			stdout:   "deleted: S1\n",
			wantArgv: []string{testBinary, "snapshot", "rm", testSnap},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Stdout: tt.stdout} }}
			e := testEngine(t, runner)

			got, err := e.RawEngine(t.Context(), &enginetypes.RawEngineOptions{ID: "w1", Op: tt.op, Params: tt.params})
			if err != nil {
				t.Fatalf("raw engine: %v", err)
			}
			if want := sshrunner.Quote(tt.wantArgv); len(runner.Lines()) != 1 || runner.Lines()[0] != want {
				t.Errorf("got %q, want %q", runner.Lines(), want)
			}
			if got.ID != "w1" || string(got.Data) != tt.wantData {
				t.Errorf("got %s %q, want w1 %q", got.ID, got.Data, tt.wantData)
			}
		})
	}
}

func TestRawEngineRefusesABadSnapshotNameBeforeTheNode(t *testing.T) {
	tests := []struct {
		name   string
		params string
	}{
		{"no params", ""},
		{"params that are not json", "save"},
		{"no name", `{}`},
		{"a leading dash", `{"name":"-rf"}`},
		{"a shell metacharacter", `{"name":"a;b"}`},
		{"a space", `{"name":"a b"}`},
		{"past cocoon's 63 chars", `{"name":"` + strings.Repeat("a", 64) + `"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &sshrunnertest.Fake{}
			e := testEngine(t, runner)

			for _, op := range []string{opSnapshotSave, opSnapshotInspect, opSnapshotRemove} {
				if _, err := e.RawEngine(t.Context(), &enginetypes.RawEngineOptions{ID: "w1", Op: op, Params: []byte(tt.params)}); !errors.Is(err, coretypes.ErrInvalidEngineArgs) {
					t.Errorf("%s: got %v, want ErrInvalidEngineArgs", op, err)
				}
			}
			if lines := runner.Lines(); len(lines) != 0 {
				t.Errorf("got %q, want no round trip", lines)
			}
		})
	}
}

func TestRawEngineRefusesAnUnknownOp(t *testing.T) {
	runner := &sshrunnertest.Fake{}
	e := testEngine(t, runner)

	if _, err := e.RawEngine(t.Context(), &enginetypes.RawEngineOptions{ID: "w1", Op: "snapshot.export"}); !errors.Is(err, coretypes.ErrEngineNotImplemented) {
		t.Errorf("got %v, want ErrEngineNotImplemented", err)
	}
	if lines := runner.Lines(); len(lines) != 0 {
		t.Errorf("got %q, want no round trip", lines)
	}
}

func TestRawEngineReportsACocoonFailure(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Code: 1, Stderr: "snapshot name taken"} }}
	e := testEngine(t, runner)

	_, err := e.RawEngine(t.Context(), &enginetypes.RawEngineOptions{ID: "w1", Op: opSnapshotSave, Params: []byte(`{"name":"` + testSnap + `"}`)})
	if err == nil || !strings.Contains(err.Error(), "snapshot name taken") {
		t.Errorf("got %v, want cocoon's failure", err)
	}
}
