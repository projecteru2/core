package containerd

import (
	"slices"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestInfoNamesTheEngine(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result {
		return &sshrunner.Result{Stdout: "machine-1\n8\n16384\n1048576\n"}
	}}

	info, err := testEngine(t, runner).Info(t.Context())
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if info.Type != Type || info.ID != "machine-1" || info.NCPU != 8 {
		t.Errorf("got %+v, want the node's own identity", info)
	}
}

func TestNodePlatformMapsUnameOntoAnArchitecture(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Stdout: "aarch64\n"} }}

	platform, err := nodePlatform(t.Context(), runner)
	if err != nil {
		t.Fatalf("platform: %v", err)
	}
	if platform.OS != "linux" || platform.Architecture != "arm64" {
		t.Errorf("got %+v, want linux/arm64", platform)
	}
}

func TestNodePlatformRefusesAnUnknownMachine(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Stdout: "riscv64\n"} }}

	if _, err := nodePlatform(t.Context(), runner); !errors.Is(err, coretypes.ErrInvaildNodeEndpoint) {
		t.Errorf("got %v, want ErrInvaildNodeEndpoint", err)
	}
}

func TestWorkloadNetworksComeFromTheHooksLabels(t *testing.T) {
	networks := workloadNetworks(map[string]string{
		networkLabelPrefix + "eru-cni": "10.10.0.5",
		"ERU":                          "1",
	}, "10.0.0.1")

	if len(networks) != 1 || networks["eru-cni"] != "10.10.0.5" {
		t.Errorf("got %+v, want the CNI address the hook recorded", networks)
	}
}

func TestWorkloadNetworksFallBackToTheNode(t *testing.T) {
	networks := workloadNetworks(map[string]string{"ERU": "1"}, "10.0.0.1")

	if networks[hostNetwork] != "10.0.0.1" {
		t.Errorf("got %+v, want the node's own address", networks)
	}
}

func TestExecArgvCarriesTheEnvironmentIntoTheCommand(t *testing.T) {
	e := testEngine(t, &sshrunnertest.Fake{})

	argv := e.execArgv("w1", "e1", &enginetypes.ExecConfig{
		Cmd:        []string{"sh", "-c", "echo hi"},
		Env:        []string{"A=1"},
		User:       "app",
		WorkingDir: "/srv",
		Tty:        true,
	})

	want := []string{
		ctrBinary, "--address", defaultSocket, "--namespace", defaultNamespace, "tasks", "exec", "--exec-id", "e1",
		"--tty", "--user", "app", "--cwd", "/srv", "w1", "env", "A=1", "sh", "-c", "echo hi",
	}
	if !slices.Equal(argv, want) {
		t.Errorf("got %q, want %q", argv, want)
	}
}

func TestNormalizeRefExpandsAShortName(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want string
	}{
		{"a short name and tag", "nginx:alpine", "docker.io/library/nginx:alpine"},
		{"a short name alone", "nginx", "docker.io/library/nginx:latest"},
		{"a namespaced short name", "projecteru2/core:v1", "docker.io/projecteru2/core:v1"},
		{"a fully qualified ref", "hub.io/ns/app:v1", "hub.io/ns/app:v1"},
		{"a ref behind a registry port", "hub.io:5000/ns/app:v1", "hub.io:5000/ns/app:v1"},
		{"a digest ref", "nginx@sha256:" + strings.Repeat("a", 64), "docker.io/library/nginx@sha256:" + strings.Repeat("a", 64)},
		{"an unparsable filter passes through", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeRef(tt.ref); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
