package containerd

import (
	"context"
	"testing"

	"github.com/containerd/containerd/v2/core/images"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	coretypes "github.com/projecteru2/core/types"
)

func TestRegistryAuthFindsTheHubUnderAnyOfItsNames(t *testing.T) {
	hub := coretypes.AuthConfig{Username: "eru", Password: "secret"}

	tests := []struct {
		name  string
		keyed string
		asked string
	}{
		{"the resolver asks by the registry host", "docker.io", "registry-1.docker.io"},
		{"buildkit asks by the config file key", "docker.io", "https://index.docker.io/v1/"},
		{"the operator keyed it the long way", "https://index.docker.io/v1/", "registry-1.docker.io"},
		{"and the short way round", "registry-1.docker.io", "docker.io"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auths := map[string]coretypes.AuthConfig{tt.keyed: hub}
			auth, ok := registryAuth(auths, tt.asked)
			if !ok || auth.Username != "eru" {
				t.Errorf("got %+v %v, want the hub credentials", auth, ok)
			}
		})
	}
}

func TestRegistryAuthKeepsAPrivateRegistryExact(t *testing.T) {
	auths := map[string]coretypes.AuthConfig{"hub.io:5000": {Username: "eru"}}

	if _, ok := registryAuth(auths, "other.io"); ok {
		t.Error("a private registry must not fall back to another host's credentials")
	}
	if auth, ok := registryAuth(auths, "hub.io:5000"); !ok || auth.Username != "eru" {
		t.Errorf("got %+v %v, want an exact match", auth, ok)
	}
}

func TestGCRefsRootEveryBlobTheManifestOwns(t *testing.T) {
	manifest := ocispec.Manifest{
		Config: ocispec.Descriptor{Digest: "sha256:cfg"},
		Layers: []ocispec.Descriptor{{Digest: "sha256:l0"}, {Digest: "sha256:l1"}},
	}

	refs := gcRefs(manifest)

	if refs["containerd.io/gc.ref.content.config"] != "sha256:cfg" {
		t.Errorf("got %q, want the config rooted", refs["containerd.io/gc.ref.content.config"])
	}
	for i, want := range []string{"sha256:l0", "sha256:l1"} {
		key := "containerd.io/gc.ref.content.l." + string(rune('0'+i))
		if refs[key] != want {
			t.Errorf("got %q for %s, want %q", refs[key], key, want)
		}
	}
}

func TestCreateImageReferenceUpdatesAnExistingTarget(t *testing.T) {
	ref := "docker.io/projecteru2/core:latest"
	oldTarget := ocispec.Descriptor{Digest: digest.FromString("old")}
	newTarget := ocispec.Descriptor{Digest: digest.FromString("new")}
	store := &existingImageStore{image: images.Image{Name: ref, Target: oldTarget}}

	if err := createImageReference(t.Context(), store, images.Image{Name: ref, Target: newTarget}); err != nil {
		t.Fatalf("create image reference: %v", err)
	}
	if store.image.Target.Digest != newTarget.Digest {
		t.Fatalf("got target %s, want %s", store.image.Target.Digest, newTarget.Digest)
	}
	if len(store.updatedFields) != 1 || store.updatedFields[0] != "target" {
		t.Fatalf("got updated fields %v, want target", store.updatedFields)
	}
}

type existingImageStore struct {
	image         images.Image
	updatedFields []string
}

func (s *existingImageStore) Get(context.Context, string) (images.Image, error) {
	return s.image, nil
}

func (s *existingImageStore) List(context.Context, ...string) ([]images.Image, error) {
	return []images.Image{s.image}, nil
}

func (s *existingImageStore) Create(context.Context, images.Image) (images.Image, error) {
	return images.Image{}, cerrdefs.ErrAlreadyExists
}

func (s *existingImageStore) Update(_ context.Context, image images.Image, fields ...string) (images.Image, error) {
	s.image = image
	s.updatedFields = fields
	return image, nil
}

func (s *existingImageStore) Delete(context.Context, string, ...images.DeleteOpt) error {
	return nil
}
