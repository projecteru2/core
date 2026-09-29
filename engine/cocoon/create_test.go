package cocoon

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/engine/sshrunner"
	"github.com/projecteru2/core/engine/sshrunner/sshrunnertest"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/engine/workloadmeta"
	resourcetypes "github.com/projecteru2/core/resource/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestVirtualizationCreateRendersTheVMAndRecordsIt(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: createdVM}
	e := testEngine(t, runner)

	created, err := e.VirtualizationCreate(t.Context(), &enginetypes.VirtualizationCreateOptions{
		Name:         "app_web_xyz",
		Image:        testImage,
		User:         testUser,
		Env:          []string{"ERU_POD=vms"},
		Networks:     map[string]string{"eru-cni": ""},
		EngineParams: resourcetypes.Resources{"cpumem": {"cpu": 1.5, "memory": 1 << 30}, "storage": {"storage": 20 << 30, "volumes": []string{"/data:/data:rw:1073741824"}}},
		Labels:       map[string]string{"ERU_META": `{"Publish":["80"],"HealthCheck":{"TCPPorts":["80"]}}`},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(created.ID) != testIDLen {
		t.Fatalf("got id %q, want a %d-hex id", created.ID, testIDLen)
	}
	lines := runner.Lines()
	if len(lines) != 2 {
		t.Fatalf("got %d commands, want the create and the record", len(lines))
	}
	want := sshrunner.Quote([]string{
		testBinary, "vm", "create", "--output", "json",
		"--cpu", "2", "--memory", "1073741824", "--storage", "21474836480",
		"--data-disk", "size=1073741824,mount=/data",
		"--network", "eru-cni", "--user", "eru",
		"--name", created.ID, testImage,
	})
	if lines[0] != want {
		t.Errorf("got %q, want %q", lines[0], want)
	}
	for _, field := range []string{
		durablePath(testRoot, created.ID),
		workloadmeta.Path(created.ID),
		`"kind":"vm"`,
		`"user":"` + testUser + `"`,
		`"podname":"vms"`,
		`"publish":["80"]`,
		`"healthcheck":{"tcp_ports":["80"]}`,
		`"networks":{"eru-cni":"10.22.0.5"}`,
		`"cgroup":"/sys/fs/cgroup/cocoon.slice/vm-` + testVMID + `.scope"`,
		`"iface":"tap01ARZ3ND-0"`,
		`"log":{"console_socket":"/var/lib/cocoon/run/cloudhypervisor/` + testVMID + `/console.sock"}`,
	} {
		if !strings.Contains(lines[1], field) {
			t.Errorf("the record command does not carry %s", field)
		}
	}
}

func TestVirtualizationCreateDiscardsAVMWhoseRecordFailed(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(line string) *sshrunner.Result {
		switch {
		case strings.Contains(line, "'create'"):
			return &sshrunner.Result{Stdout: linuxVM}
		case strings.Contains(line, "'rm'"):
			return &sshrunner.Result{}
		}
		return &sshrunner.Result{Code: 1, Stderr: "read-only file system"}
	}}
	e := testEngine(t, runner)

	if _, err := e.VirtualizationCreate(t.Context(), &enginetypes.VirtualizationCreateOptions{Name: "app_web_xyz", Image: testImage}); err == nil {
		t.Fatal("a failed record must fail the create")
	}
	lines := runner.Lines()
	if len(lines) != 3 || !strings.Contains(lines[2], "'rm' '--force'") {
		t.Errorf("got %q, want a forced rm after the failed record", lines)
	}
}

func TestVirtualizationCreateDiscardsAVMWhoseDeadlineExpired(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	runner := &sshrunnertest.Fake{Respond: createdVMThenCanceled(cancel)}
	e := testEngine(t, runner)

	if _, err := e.VirtualizationCreate(ctx, &enginetypes.VirtualizationCreateOptions{Name: "app_web_xyz", Image: testImage}); err == nil {
		t.Fatal("a record core could not write must fail the create")
	}
	lines := runner.Lines()
	if len(lines) != 2 || !strings.Contains(lines[1], "'rm' '--force'") {
		t.Errorf("got %q, want the create then a forced rm the dead context cannot stop", lines)
	}
}

func TestVirtualizationCreateKeepsTheConflistOfAnInheritedNetwork(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: createdVM}
	e := testEngine(t, runner)

	if _, err := e.VirtualizationCreate(t.Context(), &enginetypes.VirtualizationCreateOptions{
		Name:     "app_web_xyz",
		Image:    testImage,
		Networks: map[string]string{"eru-cni": "10.22.0.9"},
	}); err != nil {
		t.Fatalf("an inherited network must not fail the replace: %v", err)
	}
	if lines := runner.Lines(); !strings.Contains(lines[0], "'--network' 'eru-cni'") || strings.Contains(lines[0], "10.22.0.9") {
		t.Errorf("got %q, want the conflist name without the old address", lines[0])
	}
}

func TestVirtualizationCreateClonesASnapshotImage(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: clonedFrom}
	e := testEngine(t, runner)

	created, err := e.VirtualizationCreate(t.Context(), &enginetypes.VirtualizationCreateOptions{
		Name:         "app_web_xyz",
		Image:        snapshotScheme + testSnap,
		User:         testUser,
		Networks:     map[string]string{"eru-cni": ""},
		EngineParams: resourcetypes.Resources{"cpumem": {"cpu": 1.5, "memory": 1 << 30}, "storage": {"storage": 20 << 30, "volumes": []string{"/data:/data:rw:1073741824"}}},
	})
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	lines := runner.Lines()
	if len(lines) != 3 {
		t.Fatalf("got %d commands, want the clone, the record and the reseed", len(lines))
	}
	if reseed := sshrunner.Quote(reseedArgv(testBinary, created.ID, "app_web_xyz", mustParseVM(t, clonedVM))); lines[2] != reseed {
		t.Errorf("got %q, want the reseed %q", lines[2], reseed)
	}
	for _, arg := range []string{"'app_web_xyz'", "'02:00:00:00:00:07'", "'10.22.0.7/16'", "'10.22.0.1'"} {
		if !strings.Contains(lines[2], arg) {
			t.Errorf("the reseed does not carry %s", arg)
		}
	}
	want := sshrunner.Quote([]string{
		testBinary, "vm", "clone", "--output", "json", "--name", created.ID,
		"--network", "eru-cni", "--data-disk", "size=1073741824,mount=/data", testSnap,
	})
	if lines[0] != want {
		t.Errorf("got %q, want %q", lines[0], want)
	}
	for _, field := range []string{
		durablePath(testRoot, created.ID),
		`"user":"` + testUser + `"`,
		`"networks":{"eru-cni":"10.22.0.7"}`,
		`"cgroup":"/sys/fs/cgroup/cocoon.slice/vm-` + testVMID + `.scope"`,
		`"iface":"tap01ARZ3ND-0"`,
	} {
		if !strings.Contains(lines[1], field) {
			t.Errorf("the record command does not carry %s", field)
		}
	}
}

func TestVirtualizationCreateDiscardsACloneWhoseRecordFailed(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(line string) *sshrunner.Result {
		switch {
		case strings.Contains(line, "'clone'"):
			return &sshrunner.Result{Stdout: clonedVM}
		case strings.Contains(line, "'rm'"):
			return &sshrunner.Result{}
		}
		return &sshrunner.Result{Code: 1, Stderr: "read-only file system"}
	}}
	e := testEngine(t, runner)

	if _, err := e.VirtualizationCreate(t.Context(), &enginetypes.VirtualizationCreateOptions{Name: "app_web_xyz", Image: snapshotScheme + testSnap}); err == nil {
		t.Fatal("a failed record must fail the clone")
	}
	lines := runner.Lines()
	if len(lines) != 3 || !strings.Contains(lines[2], "'rm' '--force'") {
		t.Errorf("got %q, want a forced rm after the failed record", lines)
	}
}

func TestVirtualizationCreateDiscardsACloneThatWouldNotReseed(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(line string) *sshrunner.Result {
		switch {
		case strings.Contains(line, "'clone'"):
			return &sshrunner.Result{Stdout: clonedVM}
		case strings.Contains(line, "systemd-networkd"):
			return &sshrunner.Result{Code: 1, Stderr: "agent did not answer"}
		}
		return &sshrunner.Result{}
	}}
	e := testEngine(t, runner)

	if _, err := e.VirtualizationCreate(t.Context(), &enginetypes.VirtualizationCreateOptions{Name: "app_web_xyz", Image: snapshotScheme + testSnap}); err == nil {
		t.Fatal("a clone whose network was not rewritten must fail")
	}
	lines := runner.Lines()
	if len(lines) != 4 || !strings.Contains(lines[3], "'rm' '--force'") {
		t.Errorf("got %q, want a forced rm after the failed reseed", lines)
	}
}

func TestVirtualizationCreateReportsAMissingSnapshot(t *testing.T) {
	runner := &sshrunnertest.Fake{Respond: func(string) *sshrunner.Result {
		return &sshrunner.Result{Code: 1, Stderr: "inspect snapshot " + testSnap + ": not found"}
	}}
	e := testEngine(t, runner)

	_, err := e.VirtualizationCreate(t.Context(), &enginetypes.VirtualizationCreateOptions{Name: "app_web_xyz", Image: snapshotScheme + testSnap})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("got %v, want cocoon's lookup failure", err)
	}
	if lines := runner.Lines(); len(lines) != 1 {
		t.Errorf("got %q, want no rm for a vm cocoon never made", lines)
	}
}

func TestVirtualizationCreateRefusesACloneBeforeTheNode(t *testing.T) {
	tests := []struct {
		name  string
		image string
		raw   string
	}{
		{"a windows guest", snapshotScheme + testSnap, `{"os":"windows"}`},
		{"an empty snapshot name", snapshotScheme, ""},
		{"a name cocoon would refuse", snapshotScheme + "-rf", ""},
		{"a name with a space", snapshotScheme + "a b", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &sshrunnertest.Fake{Respond: clonedFrom}
			e := testEngine(t, runner)

			_, err := e.VirtualizationCreate(t.Context(), &enginetypes.VirtualizationCreateOptions{Name: "app_web_xyz", Image: tt.image, RawArgs: []byte(tt.raw)})
			if !errors.Is(err, coretypes.ErrInvalidEngineArgs) {
				t.Errorf("got %v, want ErrInvalidEngineArgs", err)
			}
			if lines := runner.Lines(); len(lines) != 0 {
				t.Errorf("got %q, want no round trip", lines)
			}
		})
	}
}

func TestCreateArgvForAWindowsGuest(t *testing.T) {
	opts := &enginetypes.VirtualizationCreateOptions{Image: "win11", User: "eru"}
	resource := &engine.VirtualizationResource{Quota: 4, Memory: 4 << 30, Volumes: []string{"/scratch:/scratch:rw:2147483648"}}

	got, err := createArgv(testBinary, "abc", opts, resource, true, "")
	if err != nil {
		t.Fatalf("argv: %v", err)
	}
	want := []string{
		testBinary, "vm", "create", "--output", "json",
		"--cpu", "4", "--memory", "4294967296",
		"--data-disk", "size=2147483648,fstype=none",
		"--windows",
		"--name", "abc", "win11",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCreateArgvLeavesUnsetResourcesToCocoon(t *testing.T) {
	got, err := createArgv(testBinary, "abc", &enginetypes.VirtualizationCreateOptions{Image: testImage}, &engine.VirtualizationResource{}, false, "")
	if err != nil {
		t.Fatalf("argv: %v", err)
	}
	want := []string{testBinary, "vm", "create", "--output", "json", "--name", "abc", testImage}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDataDisksNeedAMountAndASize(t *testing.T) {
	if _, err := dataDisks([]string{"/host/dir:/data:rw"}, false); !errors.Is(err, coretypes.ErrInvalidVolumeBind) {
		t.Errorf("got %v, want ErrInvalidVolumeBind", err)
	}
}

func TestRequestedNetwork(t *testing.T) {
	tests := []struct {
		name     string
		networks map[string]string
		want     string
		wantErr  bool
	}{
		{"none means the default conflist", nil, "", false},
		{"one name", map[string]string{"mgmt": ""}, "mgmt", false},
		{"an inherited address keeps only its conflist", map[string]string{"mgmt": "10.0.0.2"}, "mgmt", false},
		{"two networks are refused", map[string]string{"a": "", "b": ""}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := requestedNetwork(t.Context(), tt.networks)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got error %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUnappliedOptions(t *testing.T) {
	coreEnv := []string{"APP_NAME=app", "ERU_POD=vms", "ERU_NODE_NAME=n1", "ERU_WORKLOAD_SEQ=0"}
	tests := []struct {
		name string
		opts *enginetypes.VirtualizationCreateOptions
		want []string
	}{
		{"core env alone", &enginetypes.VirtualizationCreateOptions{Env: coreEnv}, nil},
		{"deploy env", &enginetypes.VirtualizationCreateOptions{Env: append(coreEnv, "TOKEN=x")}, []string{"env"}},
		{
			"every unapplied field",
			&enginetypes.VirtualizationCreateOptions{DNS: []string{"1.1.1.1"}, Hosts: []string{"h:1.2.3.4"}, Cmd: []string{"run"}, WorkingDir: "/srv"},
			[]string{"dns", "extra_hosts", "entrypoint commands", "entrypoint dir"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := unappliedOptions(tt.opts); !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
