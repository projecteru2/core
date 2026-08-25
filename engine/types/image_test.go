package types

import (
	"testing"
)

func TestSplitRefIgnoresARegistryPort(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want string
		tag  string
	}{
		{"tagged", "hub.io/ns/app:v1", "hub.io/ns/app", "v1"},
		{"tagged behind a port", "hub.io:5000/ns/app:v1", "hub.io:5000/ns/app", "v1"},
		{"untagged behind a port", "hub.io:5000/ns/app", "hub.io:5000/ns/app", ""},
		{"a fully qualified ref", "docker.io/library/nginx:alpine", "docker.io/library/nginx", "alpine"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, tag := SplitRef(tt.ref)
			if name != tt.want || tag != tt.tag {
				t.Errorf("got %q %q, want %q %q", name, tag, tt.want, tt.tag)
			}
		})
	}
}
