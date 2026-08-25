package process

import (
	"slices"
	"testing"
	"time"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
)

func TestUnitArgvRendersABoundWorkload(t *testing.T) {
	u := &unit{
		ID:          "abcdef",
		Podname:     "prod",
		User:        "app",
		Root:        testRoot + "/abcdef/merged",
		Working:     "/home/app",
		TasksMax:    512,
		StopTimeout: 10 * time.Second,
		Opts: &enginetypes.VirtualizationCreateOptions{
			Name:    "app_web_xyz",
			Env:     []string{"FOO=bar baz"},
			Cmd:     []string{"/bin/server", "--port", "8080"},
			Restart: "on-failure:3",
		},
		Resource: &engine.VirtualizationResource{
			Quota:       1.5,
			CPU:         map[string]int64{"0": 100, "1": 50},
			NUMANode:    "0",
			Memory:      1 << 30,
			IOPSOptions: map[string]string{"/dev/sda": "100:200:1M:2M"},
		},
	}

	want := []string{
		"systemd-run",
		"--unit=eru-abcdef.service",
		"--slice=eru-prod.slice",
		"-p", "Description=app/web",
		"-p", "RemainAfterExit=yes",
		"-p", "SyslogIdentifier=eru",
		"-p", "User=app",
		"-p", "WorkingDirectory=/home/app",
		"-p", "RootDirectory=" + testRoot + "/abcdef/merged",
		"-p", `Environment=FOO="bar baz"`,
		"-p", "AllowedCPUs=0 1",
		"-p", "AllowedMemoryNodes=0",
		"-p", "CPUWeight=50",
		"-p", "MemoryMax=1073741824",
		"-p", "MemoryHigh=536870912",
		"-p", "TasksMax=512",
		"-p", "IOReadIOPSMax=/dev/sda 100",
		"-p", "IOWriteIOPSMax=/dev/sda 200",
		"-p", "IOReadBandwidthMax=/dev/sda 1048576",
		"-p", "IOWriteBandwidthMax=/dev/sda 2097152",
		"-p", "Restart=on-failure",
		"-p", "TimeoutStopSec=10",
		"--", "/bin/server", "--port", "8080",
	}
	if got := u.argv(); !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestUnitArgvRendersARawWorkload(t *testing.T) {
	u := &unit{
		ID:      "abcdef",
		Podname: "prod",
		Working: testRoot + "/abcdef/lower",
		Opts: &enginetypes.VirtualizationCreateOptions{
			Name: "app_web_xyz",
			Cmd:  []string{"./server"},
		},
		Resource: &engine.VirtualizationResource{Quota: 2, Volumes: []string{"/data/app:/data:rw"}},
	}

	want := []string{
		"systemd-run",
		"--unit=eru-abcdef.service",
		"--slice=eru-prod.slice",
		"-p", "Description=app/web",
		"-p", "RemainAfterExit=yes",
		"-p", "SyslogIdentifier=eru",
		"-p", "WorkingDirectory=" + testRoot + "/abcdef/lower",
		"-p", "CPUQuota=200%",
		"-p", "BindPaths=/data/app:/data",
		"--", "./server",
	}
	if got := u.argv(); !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCPUWeight(t *testing.T) {
	tests := []struct {
		name  string
		quota float64
		remap bool
		want  int
	}{
		{"whole core keeps the default weight", 2, false, 100},
		{"fractional share scales the weight", 1.25, false, 25},
		{"remapped workloads keep the default weight", 1.25, true, 100},
		{"a tiny share never falls below one", 1.001, false, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cpuWeight(tt.quota, tt.remap); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBindPaths(t *testing.T) {
	tests := []struct {
		name    string
		volumes []string
		env     []string
		want    []string
	}{
		{"a two-part binding is read-write", []string{"/src:/dst"}, nil, []string{"BindPaths=/src:/dst"}},
		{"an explicit ro binding is read-only", []string{"/src:/dst:ro"}, nil, []string{"BindReadOnlyPaths=/src:/dst"}},
		{"a size field is ignored", []string{"/src:/dst:rw:1024"}, nil, []string{"BindPaths=/src:/dst"}},
		{
			"the source is expanded from the environment",
			[]string{"/vol/$APP_NAME:/dst"},
			[]string{"APP_NAME=web"},
			[]string{"BindPaths=/vol/web:/dst"},
		},
		{"a binding with no host path is skipped", []string{":/dst:rw"}, nil, []string{}},
		{"a one-part binding is skipped", []string{"/dst"}, nil, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bindPaths(tt.volumes, tt.env); !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestThrottlesSkipZeroRates(t *testing.T) {
	want := []string{"IOWriteBandwidthMax=/dev/sdb 4096"}
	if got := throttles(map[string]string{"/dev/sdb": "0:0:0:4096"}); !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
