package cocoon

import (
	"testing"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	coretypes "github.com/projecteru2/core/types"
)

func TestInfoReadsTheNodeCapacity(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result {
		return &sshrunner.Result{Stdout: "machine1\n8\n16777216\n104857600\n"}
	}}
	e := testEngine(t, runner)

	info, err := e.Info(t.Context())
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if info.ID != "machine1" || info.NCPU != 8 || info.MemTotal != 16777216*1024 || info.StorageTotal != 104857600*1024 {
		t.Errorf("got %+v, want the machine id with cpu, memory and storage in bytes", info)
	}
	if info.Type != Type {
		t.Errorf("got %q, want the cocoon engine type", info.Type)
	}
}

func TestInfoRefusesANodeItCouldNotMeasure(t *testing.T) {
	tests := []struct {
		name string
		res  *sshrunner.Result
	}{
		{"the script failed", &sshrunner.Result{Code: 1, Stderr: "df: /var/lib/cocoon/run: No such file or directory"}},
		{"a line is missing", &sshrunner.Result{Stdout: "machine1\n8\n16777216\n"}},
		{"a field is not a number", &sshrunner.Result{Stdout: "machine1\n8\n\n104857600\n"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := testEngine(t, &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return tt.res }})

			if _, err := e.Info(t.Context()); err == nil {
				t.Error("a node core could not measure must not register with zero capacity")
			}
		})
	}
}

func TestInfoReportsAnEndpointError(t *testing.T) {
	e := testEngine(t, &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Stdout: "\n"} }})

	if _, err := e.Info(t.Context()); !errors.Is(err, coretypes.ErrInvaildNodeEndpoint) {
		t.Errorf("got %v, want ErrInvaildNodeEndpoint", err)
	}
}
