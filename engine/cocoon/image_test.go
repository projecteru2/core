package cocoon

import (
	"slices"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	coretypes "github.com/projecteru2/core/types"
)

const partsManifest = `{"schemaVersion":2,"artifactType":"application/vnd.cocoonstack.os-image.v1+json",` +
	`"layers":[{"mediaType":"application/vnd.cocoonstack.disk.qcow2.part","digest":"sha256:a","size":1}]}`

func TestImagePullHandsAVMImageToCocoon(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(line string) *sshrunner.Result {
		if strings.HasPrefix(line, "'oras'") {
			return &sshrunner.Result{Stdout: `{"schemaVersion":2,"layers":[{"mediaType":"application/vnd.oci.image.layer.v1.tar+gzip"}]}`}
		}
		return &sshrunner.Result{}
	}}
	e := testEngine(t, runner)

	if _, err := e.ImagePull(t.Context(), testImage, false); err != nil {
		t.Fatalf("pull: %v", err)
	}
	want := []string{
		sshrunner.Quote(sshrunner.Shell(orasProbe)),
		sshrunner.Quote([]string{"oras", "manifest", "fetch", testImage}),
		sshrunner.Quote([]string{testBinary, "image", "pull", testImage}),
	}
	if !slices.Equal(runner.Lines(), want) {
		t.Errorf("got %q, want %q", runner.Lines(), want)
	}
}

func TestImagePullImportsAPartsArtifact(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(line string) *sshrunner.Result {
		if strings.HasPrefix(line, "'oras'") {
			return &sshrunner.Result{Stdout: partsManifest}
		}
		return &sshrunner.Result{}
	}}
	e := testEngine(t, runner)

	if _, err := e.ImagePull(t.Context(), "ghcr.io/cocoonstack/windows/win11:25h2", false); err != nil {
		t.Fatalf("pull: %v", err)
	}
	want := sshrunner.Quote(sshrunner.Shell(importScript, testBinary, "ghcr.io/cocoonstack/windows/win11:25h2"))
	if len(runner.Lines()) != 3 || runner.Lines()[2] != want {
		t.Errorf("got %q, want the probe, the manifest read then %q", runner.Lines(), want)
	}
	for _, step := range []string{"oras pull", "image import", `"$tmp"/*.part`} {
		if !strings.Contains(importScript, step) {
			t.Errorf("the import script does not carry %q", step)
		}
	}
}

func TestImagePullSkipsOrasForACloudImageURL(t *testing.T) {
	runner := &sshrunnertest.Fake{}
	e := testEngine(t, runner)

	url := "https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-amd64.img"
	if _, err := e.ImagePull(t.Context(), url, false); err != nil {
		t.Fatalf("pull: %v", err)
	}
	want := sshrunner.Quote([]string{testBinary, "image", "pull", url})
	if len(runner.Lines()) != 1 || runner.Lines()[0] != want {
		t.Errorf("got %q, want %q", runner.Lines(), want)
	}
}

func TestImagePullSkipsOrasOnANodeWithoutIt(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(line string) *sshrunner.Result {
		if strings.Contains(line, "command -v oras") {
			return &sshrunner.Result{Code: 1}
		}
		return &sshrunner.Result{}
	}}
	e := testEngine(t, runner)

	if _, err := e.ImagePull(t.Context(), testImage, false); err != nil {
		t.Fatalf("pull: %v", err)
	}
	want := []string{sshrunner.Quote(sshrunner.Shell(orasProbe)), sshrunner.Quote([]string{testBinary, "image", "pull", testImage})}
	if !slices.Equal(runner.Lines(), want) {
		t.Errorf("got %q, want the probe then cocoon's own pull", runner.Lines())
	}

	digest, err := e.ImageRemoteDigest(t.Context(), testImage)
	if err != nil || digest != "" {
		t.Errorf("got %q %v, want no digest and no error without oras", digest, err)
	}
	if lines := runner.Lines(); len(lines) != 3 || lines[2] != want[0] {
		t.Errorf("got %q, want a node that answered no probed again", lines)
	}
}

func TestOrasProbeIsAskedOnlyOnceOnceItAnswered(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Stdout: `{"digest":"sha256:abc"}`} }}
	e := testEngine(t, runner)

	for range 2 {
		if _, err := e.ImageRemoteDigest(t.Context(), testImage); err != nil {
			t.Fatalf("digest: %v", err)
		}
	}
	if lines := runner.Lines(); len(lines) != 3 {
		t.Errorf("got %q, want one probe and two manifest reads", lines)
	}
}

func TestImageListFiltersByName(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result {
		return &sshrunner.Result{Stdout: `[{"id":"sha256:a","name":"` + testImage + `","type":"oci"},{"id":"sha256:b","name":"win11","type":"cloudimg"}]`}
	}}
	e := testEngine(t, runner)

	images, err := e.ImageList(t.Context(), "ghcr.io/cocoonstack")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(images) != 1 || images[0].ID != "sha256:a" || !slices.Equal(images[0].Tags, []string{testImage}) {
		t.Errorf("got %+v, want the one image under the prefix", images)
	}
}

func TestImageListOnANodeWithNoImages(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result {
		return &sshrunner.Result{Stdout: "No images found.\n"}
	}}
	e := testEngine(t, runner)

	images, err := e.ImageList(t.Context(), "ghcr.io/cocoonstack")
	if err != nil {
		t.Fatalf("cocoon answers an empty store in prose, not json: %v", err)
	}
	if len(images) != 0 {
		t.Errorf("got %+v, want no images", images)
	}
}

func TestImageLocalDigests(t *testing.T) {
	tests := []struct {
		name  string
		image string
		res   *sshrunner.Result
		want  []string
	}{
		{"a stored image", testImage, &sshrunner.Result{Stdout: `{"id":"sha256:abc","name":"` + testImage + `"}`}, []string{"ghcr.io/cocoonstack/cocoon/ubuntu@sha256:abc"}},
		{"a missing image", testImage, &sshrunner.Result{Code: 1}, nil},
		{"a cloud image is keyed by its url", "https://x/y.img", &sshrunner.Result{Stdout: `{"id":"sha256:abc","name":"https://x/y.img"}`}, []string{"https://x/y.img"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := testEngine(t, &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return tt.res }})

			got, err := e.ImageLocalDigests(t.Context(), tt.image)
			if err != nil {
				t.Fatalf("digests: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestImageRemoteDigest(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result { return &sshrunner.Result{Stdout: `{"digest":"sha256:abc"}`} }}
	e := testEngine(t, runner)

	digest, err := e.ImageRemoteDigest(t.Context(), testImage)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if digest != "ghcr.io/cocoonstack/cocoon/ubuntu@sha256:abc" {
		t.Errorf("got %q, want the name at the manifest digest", digest)
	}
	url := "https://x/y.img"
	if digest, err = e.ImageRemoteDigest(t.Context(), url); err != nil || digest != url {
		t.Errorf("got %q %v, want the url itself", digest, err)
	}
	if len(runner.Lines()) != 2 {
		t.Errorf("got %q, want the probe and one oras call, none for the url", runner.Lines())
	}
}

func TestImageBuildFromExistNeedsARegistryPush(t *testing.T) {
	runner := &sshrunnertest.Fake{}
	e := testEngine(t, runner)

	if _, err := e.ImageBuildFromExist(t.Context(), "w1", []string{"hub.io/ns/app:v2"}, ""); !errors.Is(err, coretypes.ErrEngineNotImplemented) {
		t.Errorf("got %v, want ErrEngineNotImplemented", err)
	}
	if len(runner.Lines()) != 0 {
		t.Errorf("got %q, want no snapshot for a build core cannot push", runner.Lines())
	}
	if _, err := e.ImagePush(t.Context(), "hub.io/ns/app:v2"); !errors.Is(err, coretypes.ErrEngineNotImplemented) {
		t.Errorf("got %v, want ErrEngineNotImplemented", err)
	}
}
