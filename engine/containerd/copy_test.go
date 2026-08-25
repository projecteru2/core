package containerd

import (
	"testing"

	"github.com/projecteru2/core/engine/sshrunner"
)

func TestMissingPathSeparatesTarsWarningsFromItsFailures(t *testing.T) {
	tests := []struct {
		name string
		res  *sshrunner.Result
		want bool
	}{
		{"a warning about a changed file is not a missing path", &sshrunner.Result{Code: 1, Stderr: "tar: file changed as we read it"}, false},
		{"a fatal error is", &sshrunner.Result{Code: 2, Stderr: "tar: /nope: Cannot stat"}, true},
		{"so is the message, whatever the code", &sshrunner.Result{Code: 1, Stderr: "tar: /nope: No such file or directory"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := missingPath(tt.res); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
