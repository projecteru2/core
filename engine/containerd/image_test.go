package containerd

import (
	"testing"

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
