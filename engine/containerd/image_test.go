package containerd

import (
	"testing"

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
