package utils

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	minimalConfig = `etcd:
    machines:
        - "http://127.0.0.1:2379"
`

	fullConfig = `bind: ":5001"
statsd: "127.0.0.1:8125"
profile: ":12346"
global_timeout: 300s

auth:
    username: admin
    password: password
etcd:
    machines:
        - "http://127.0.0.1:2379"
    lock_prefix: "core/_lock"
git:
    scm_type: "github"
registry:
    hub: "hub.docker.com"
    namespace: "projecteru2"
build:
    node_filter:
        labels:
            eru.build: "1"
scheduler:
    sharebase: 50
`
)

func TestLoadConfigAppliesDefaults(t *testing.T) {
	config, err := LoadConfig(writeConfig(t, minimalConfig))
	assert.NoError(t, err)

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"duration default", config.LockTimeout, 30 * time.Second},
		{"duration default on a nested struct", config.GRPCConfig.ServiceHeartbeatInterval, 15 * time.Second},
		{"string default", config.Etcd.Prefix, "/eru"},
		{"numeric string default", config.Bind, "5001"},
		{"string default holding a colon", config.ProbeTarget, "8.8.8.8:80"},
		{"int default", config.Scheduler.ShareBase, 100},
		{"negative int default", config.Scheduler.MaxShare, -1},
		{"nested struct default", config.Containerd.Socket, "/run/containerd/containerd.sock"},
		{"twice-nested struct default", config.Process.StopTimeout, 10 * time.Second},
		{"default in a section the file omits", config.Redis.Addr, "localhost:6379"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.got)
		})
	}
}

func TestLoadConfigLetsTheFileOverrideDefaults(t *testing.T) {
	config, err := LoadConfig(writeConfig(t, fullConfig))
	assert.NoError(t, err)

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"duration", config.GlobalTimeout, 300 * time.Second},
		{"string", config.Bind, ":5001"},
		{"int", config.Scheduler.ShareBase, 50},
		{"nested string", config.Etcd.LockPrefix, "core/_lock"},
		{"nested struct field", config.Registry.Hub, "hub.docker.com"},
		{"map inside a twice-nested struct", config.Build.NodeFilter.Labels, map[string]string{"eru.build": "1"}},
		{"slice", config.Etcd.Machines, []string{"http://127.0.0.1:2379"}},
		{"untouched default alongside overrides", config.Virt.APIVersion, "v1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.got)
		})
	}
}

func TestLoadConfigRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"missing required field", "bind: \":5001\"\n"},
		{"required field set to its zero value", minimalConfig + "scheduler:\n    maxshare: 0\n"},
		{"file is not a mapping", "test\n"},
		{"malformed yaml", "bind: \":5001\"\n  broken\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadConfig(writeConfig(t, tt.body))
			assert.Error(t, err)
		})
	}
}

func TestLoadConfigNamesTheMissingRequiredField(t *testing.T) {
	_, err := LoadConfig(writeConfig(t, "bind: \":5001\"\n"))
	assert.ErrorContains(t, err, "Machines is required, but blank")
}

func TestLoadConfigRejectsAMissingFile(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "absent.yaml"))
	assert.Error(t, err)
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "core.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
