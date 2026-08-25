package sshrunner

import (
	"testing"
)

func TestQuoteEscapesEverySingleQuote(t *testing.T) {
	got := Quote([]string{"printf", "%s\n", "it's; rm -rf /"})
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
		{"a name", "process://node1.example.com", "", "node1.example.com", "node1.example.com:22", false},
		{"ipv6 with a port", "process://[fd00::1]:2222", "", "fd00::1", "[fd00::1]:2222", false},
		{"ipv6 without a port", "process://[fd00::1]", "", "fd00::1", "[fd00::1]:22", false},
		{"bare ipv6", "process://fd00::1", "", "fd00::1", "[fd00::1]:22", false},
		{"wrong scheme", "tcp://10.0.0.1", "", "", "", true},
		{"empty host", "process://", "", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, host, addr, err := ParseEndpoint(tt.endpoint, "process://")
			if (err != nil) != tt.wantErr {
				t.Fatalf("got error %v, wantErr %v", err, tt.wantErr)
			}
			if user != tt.user || host != tt.host || addr != tt.addr {
				t.Errorf("got %q %q %q, want %q %q %q", user, host, addr, tt.user, tt.host, tt.addr)
			}
		})
	}
}
