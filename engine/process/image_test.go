package process

import (
	"slices"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	coretypes "github.com/projecteru2/core/types"
)

func TestImageBuildFromExistCapturesTheBundle(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(line string) *sshrunner.Result {
		if strings.Contains(line, "meta.json") {
			return &sshrunner.Result{Stdout: "1\n" + overlayMeta}
		}
		return &sshrunner.Result{Stdout: `{"digest":"sha256:abc"}`}
	}}
	e := testEngine(t, runner)

	digest, err := e.ImageBuildFromExist(t.Context(), "w1", []string{"hub.io/ns/app:v2"}, "")
	if err != nil {
		t.Fatalf("exist: %v", err)
	}
	if digest != "sha256:abc" {
		t.Errorf("got digest %q, want %q", digest, "sha256:abc")
	}
	for _, want := range []string{
		`tar -C "$dir/merged"`,
		"--disable-path-validation",
		bundleMedia,
		"trap cleanup EXIT",
	} {
		if !strings.Contains(runner.Lines()[1], want) {
			t.Errorf("exist command does not carry %q", want)
		}
	}
}

func TestImageBuildFromExistRefusesARawWorkload(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Stdout: "0\n" + rawMeta} }}
	e := testEngine(t, runner)

	_, err := e.ImageBuildFromExist(t.Context(), "w1", []string{"hub.io/ns/app:v2"}, "")
	if !errors.Is(err, coretypes.ErrEngineNotImplemented) {
		t.Errorf("got %v, want ErrEngineNotImplemented", err)
	}
}

func TestRegistryFlags(t *testing.T) {
	e := testEngine(t, &sshrunnertest.Fake{})
	e.config.Registry = coretypes.RegistryConfig{
		Auths:     map[string]coretypes.AuthConfig{"hub.io": {Username: "u", Password: "p"}},
		PlainHTTP: []string{"local:5000"},
	}

	tests := []struct {
		name string
		ref  string
		want []string
	}{
		{"a registry with credentials", "hub.io/ns/app:v1", []string{"--username", "u", "--password", "p"}},
		{"a plain-http registry", "local:5000/ns/app:v1", []string{"--plain-http"}},
		{"an unknown registry", "quay.io/ns/app:v1", []string{}},
		{"a bare name normalizes to docker.io", "app:v1", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := e.registryFlags(tt.ref); !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestImagePullUnpacksTheBundleLayer(t *testing.T) {
	runner := &sshrunnertest.Fake{}
	e := testEngine(t, runner)

	if _, err := e.ImagePull(t.Context(), "hub.io/ns/app:v1", false); err != nil {
		t.Fatalf("pull: %v", err)
	}
	for _, want := range []string{`rm -rf "$dir"`, "oras pull", `unpack "$dir"`, `tar -C "$1" -xf "$archive"`} {
		if !strings.Contains(runner.Lines()[0], want) {
			t.Errorf("pull command does not carry %q", want)
		}
	}
}
