package process

import (
	"testing"
)

func TestQuoteEscapesEverySingleQuote(t *testing.T) {
	got := quote([]string{"printf", "%s\n", "it's; rm -rf /"})
	want := `'printf' '%s` + "\n" + `' 'it'\''s; rm -rf /'`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		user     string
		host     string
		addr     string
		wantErr  bool
	}{
		{"host only", "process://10.0.0.1", "", "10.0.0.1", "10.0.0.1:22", false},
		{"user and port", "process://eru@10.0.0.1:2222", "eru", "10.0.0.1", "10.0.0.1:2222", false},
		{"wrong scheme", "tcp://10.0.0.1", "", "", "", true},
		{"empty host", "process://", "", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, host, addr, err := parseEndpoint(tt.endpoint)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got error %v, wantErr %v", err, tt.wantErr)
			}
			if user != tt.user || host != tt.host || addr != tt.addr {
				t.Errorf("got %q %q %q, want %q %q %q", user, host, addr, tt.user, tt.host, tt.addr)
			}
		})
	}
}

func TestCgroupPathNestsSlicesOnDashes(t *testing.T) {
	got := cgroupPath(sliceName("my-pod"), unitName("abc"))
	want := "/sys/fs/cgroup/eru.slice/eru-my.slice/eru-my-pod.slice/eru-abc.service"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, tag := splitRef(tt.ref)
			if name != tt.want || tag != tt.tag {
				t.Errorf("got %q %q, want %q %q", name, tag, tt.want, tt.tag)
			}
		})
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

func TestParseShow(t *testing.T) {
	shown := parseShow("LoadState=loaded\nActiveState=active\nUser=\n")
	if shown["ActiveState"] != "active" {
		t.Errorf("got %q, want %q", shown["ActiveState"], "active")
	}
	if user, ok := shown["User"]; !ok || user != "" {
		t.Errorf("got %q %v, want an empty User", user, ok)
	}
}
