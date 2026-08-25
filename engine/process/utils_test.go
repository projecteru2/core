package process

import (
	"testing"
)

func TestCgroupPathNestsSlicesOnDashes(t *testing.T) {
	got := cgroupPath(sliceName("my-pod"), unitName("abc"))
	want := "/sys/fs/cgroup/eru.slice/eru-my.slice/eru-my-pod.slice/eru-abc.service"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRestartPolicy(t *testing.T) {
	tests := []struct {
		name    string
		restart string
		want    string
	}{
		{"empty", "", ""},
		{"always", "always", "always"},
		{"unless-stopped folds into always", "unless-stopped", "always"},
		{"on-failure drops its retry count", "on-failure:3", "on-failure"},
		{"no", "no", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := restartPolicy(tt.restart); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSystemdEnvQuotesTheValue(t *testing.T) {
	tests := []struct {
		name  string
		entry string
		want  string
	}{
		{"plain", "FOO=bar", `FOO="bar"`},
		{"value with spaces", "OPTS=-a -b", `OPTS="-a -b"`},
		{"value with a quote", `MSG=say "hi"`, `MSG="say \"hi\""`},
		{"a literal percent is doubled for systemd", "FMT=100%", `FMT="100%%"`},
		{"no assignment", "FOO", "FOO"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := systemdEnv(tt.entry); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLastEnvValueWins(t *testing.T) {
	env := []string{"ERU_POD=client", "APP_NAME=web", "ERU_POD=prod"}

	if got := lastEnvValue(env, "ERU_POD"); got != "prod" {
		t.Errorf("got %q, want %q: core appends its own value after the client's", got, "prod")
	}
}

func TestValidPodname(t *testing.T) {
	tests := []struct {
		name    string
		podname string
		want    bool
	}{
		{"letters, digits and separators", "eru-test_1.0", true},
		{"empty", "", false},
		{"a slash would open a path", "eru/test", false},
		{"a space would split the unit name", "eru test", false},
		{"an at sign makes it a template unit", "eru@test", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validPodname(tt.podname); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseShow(t *testing.T) {
	shown := parseShow("LoadState=loaded\nActiveState=active\nUser=\n")
	if shown["ActiveState"] != "active" {
		t.Errorf("got %q, want %q", shown["ActiveState"], "active")
	}
	if user, ok := shown["User"]; !ok || user != "" {
		t.Errorf("got %q %v, want an empty User", user, ok)
	}
}
