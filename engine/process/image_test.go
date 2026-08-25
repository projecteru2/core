package process

import (
	"strings"
	"testing"

	"github.com/cockroachdb/errors"

	coretypes "github.com/projecteru2/core/types"
)

func TestImageBuildFromExistCapturesTheBundle(t *testing.T) {
	runner := &fakeRunner{respond: func(line string) *result {
		if strings.Contains(line, "meta.json") {
			return &result{Stdout: "1\n" + overlayMeta}
		}
		return &result{Stdout: `{"digest":"sha256:abc"}`}
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
		if !strings.Contains(runner.lines[1], want) {
			t.Errorf("exist command does not carry %q", want)
		}
	}
}

func TestImageBuildFromExistRefusesARawWorkload(t *testing.T) {
	runner := &fakeRunner{respond: func(string) *result { return &result{Stdout: "0\n" + rawMeta} }}
	e := testEngine(t, runner)

	_, err := e.ImageBuildFromExist(t.Context(), "w1", []string{"hub.io/ns/app:v2"}, "")
	if !errors.Is(err, coretypes.ErrEngineNotImplemented) {
		t.Errorf("got %v, want ErrEngineNotImplemented", err)
	}
}

func TestImagePullUnpacksTheBundleLayer(t *testing.T) {
	runner := &fakeRunner{}
	e := testEngine(t, runner)

	if _, err := e.ImagePull(t.Context(), "hub.io/ns/app:v1", false); err != nil {
		t.Fatalf("pull: %v", err)
	}
	for _, want := range []string{"oras pull", `unpack "$dir"`, `tar -C "$1" -xf "$archive"`} {
		if !strings.Contains(runner.lines[0], want) {
			t.Errorf("pull command does not carry %q", want)
		}
	}
}
