package process

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/workloadmeta"
)

const (
	scriptUnit       = "eru-w1.service"
	scriptRef        = "hub.io/ns/app:v1"
	scriptDescriptor = `{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:abc","size":7}`
	scriptLauncher   = "echo launched"

	systemctlShim = `#!/bin/sh
pop() {
[ -f "$1" ] || return 0
head -n 1 "$1"
if [ "$(wc -l < "$1")" -gt 1 ]; then
tail -n +2 "$1" > "$1.tmp"
mv "$1.tmp" "$1"
fi
}
if [ "$1" = show ]; then
case "$*" in
*"-p LoadState"*) printf '%s\n' "$STUB_LOADSTATE";;
*"-p SubState --value"*) pop "$STUB_SUBSTATE_FILE";;
*"-p ExecMainStatus"*) printf '%s\n' "$STUB_STATUS";;
*) printf '%s' "$STUB_SHOW";;
esac
exit 0
fi
printf 'systemctl %s\n' "$*" >> "$STUB_LOG"
for verb in $STUB_FAIL; do
if [ "$verb" = "$1" ]; then
printf 'systemctl: %s failed\n' "$1" >&2
exit 1
fi
done
exit 0
`

	mountpointShim = `#!/bin/sh
printf 'mountpoint %s\n' "$*" >> "$STUB_LOG"
exit "${STUB_MOUNTPOINT:-1}"
`

	mountShim = `#!/bin/sh
printf 'mount %s\n' "$*" >> "$STUB_LOG"
exit "${STUB_MOUNT:-0}"
`

	umountShim = `#!/bin/sh
printf 'umount %s\n' "$*" >> "$STUB_LOG"
exit 0
`

	sleepShim = `#!/bin/sh
printf 'sleep %s\n' "$*" >> "$STUB_LOG"
exit 0
`

	tarShim = `#!/bin/sh
printf 'tar %s\n' "$*" >> "$STUB_LOG"
[ "${STUB_TAR:-0}" = 0 ] || exit "$STUB_TAR"
mode=
file=
dir=.
prev=
for arg in "$@"; do
case "$arg" in
-cf) mode=c;;
-xf) mode=x;;
esac
case "$prev" in
-C) dir=$arg;;
-cf|-xf) file=$arg;;
esac
prev=$arg
done
case "$mode" in
c) printf 'archive\n' > "$file";;
x) [ -z "$STUB_TAR_MEMBER" ] || printf 'member\n' > "$dir/$STUB_TAR_MEMBER";;
esac
exit 0
`

	orasShim = `#!/bin/sh
printf 'oras %s\n' "$*" >> "$STUB_LOG"
case "$1 $2" in
"pull "*)
[ "${STUB_ORAS_PULL:-0}" = 0 ] || exit "$STUB_ORAS_PULL"
out=
prev=
for arg in "$@"; do
if [ "$prev" = "-o" ]; then out=$arg; fi
prev=$arg
done
for name in $STUB_ORAS_FILES; do
printf 'blob\n' > "$out/$name"
done
;;
"manifest fetch")
[ "${STUB_ORAS_FETCH:-0}" = 0 ] || exit "$STUB_ORAS_FETCH"
printf '%s\n' "$STUB_DESCRIPTOR"
;;
"push "*)
[ "${STUB_ORAS_PUSH:-0}" = 0 ] || exit "$STUB_ORAS_PUSH"
;;
esac
exit 0
`

	journalctlShim = `#!/bin/sh
printf 'journalctl %s\n' "$*" >> "$STUB_LOG"
printf '%s\n' "$STUB_JOURNAL"
[ "${STUB_JOURNAL_HOLD:-0}" = 1 ] || exit 0
trap 'printf "journalctl killed\n" >> "$STUB_LOG"; exit 0' TERM
/bin/sleep 5 >/dev/null 2>&1 &
wait
exit 0
`
)

func TestMetaScriptReportsTheRecordAndTheMountState(t *testing.T) {
	tests := []struct {
		name       string
		meta       string
		mountpoint string
		wantCode   int
		wantStdout string
	}{
		{"a mounted overlay", overlayMeta, "0", 0, "1\n" + overlayMeta + "\n"},
		{"an unmounted overlay", overlayMeta, "1", 0, "0\n" + overlayMeta + "\n"},
		{"a raw workload", rawMeta, "1", 0, "0\n" + rawMeta + "\n"},
		{"a workload the node lost", "", "1", workloadmeta.NotExistsCode, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := newScriptNode(t)
			node.env["STUB_MOUNTPOINT"] = tt.mountpoint
			if tt.meta != "" {
				node.write(t, filepath.Join(node.dir, "meta.json"), tt.meta+"\n")
			}

			got := node.run(t, metaScript, node.dir)

			if got.code != tt.wantCode {
				t.Errorf("got exit %d, want %d", got.code, tt.wantCode)
			}
			if got.stdout != tt.wantStdout {
				t.Errorf("got %q, want %q", got.stdout, tt.wantStdout)
			}
		})
	}
}

func TestStartScriptPreparesTheOverlayBeforeTheUnit(t *testing.T) {
	tests := []struct {
		name       string
		loadState  string
		subState   string
		mountpoint string
		work       bool
		fail       string
		wantCode   int
		wantCalls  []string
		wantRun    bool
	}{
		{
			name:     "a running unit is left alone",
			subState: subStateRunning,
		},
		{
			name:       "a stale unit is stopped and its overlay mounted",
			loadState:  "loaded",
			subState:   "dead",
			mountpoint: "1",
			work:       true,
			wantCalls: []string{
				"systemctl stop " + scriptUnit,
				"mountpoint -q {dir}/merged",
				"mount -t overlay overlay -o lowerdir={dir}/lower,upperdir={dir}/upper,workdir={dir}/work {dir}/merged",
			},
			wantRun: true,
		},
		{
			name:      "a raw workload has no overlay to mount",
			loadState: "not-found",
			subState:  "dead",
			wantRun:   true,
		},
		{
			name:       "an overlay that is already mounted is kept",
			loadState:  "not-found",
			subState:   "dead",
			mountpoint: "0",
			work:       true,
			wantCalls:  []string{"mountpoint -q {dir}/merged"},
			wantRun:    true,
		},
		{
			name:      "a unit that will not stop fails the start",
			loadState: "loaded",
			subState:  "dead",
			fail:      "stop",
			wantCode:  1,
			wantCalls: []string{"systemctl stop " + scriptUnit},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := newScriptNode(t)
			node.env["STUB_LOADSTATE"] = tt.loadState
			node.env["STUB_MOUNTPOINT"] = tt.mountpoint
			node.env["STUB_FAIL"] = tt.fail
			node.states(t, tt.subState)
			node.write(t, filepath.Join(node.dir, "meta.json"), overlayMeta)
			node.write(t, filepath.Join(node.dir, "run.sh"), "echo started\n")
			if tt.work {
				node.mkdir(t, filepath.Join(node.dir, "work"))
			}

			got := node.run(t, startScript, node.dir, scriptUnit, node.record)

			if got.code != tt.wantCode {
				t.Fatalf("got exit %d, want %d: %s", got.code, tt.wantCode, got.stderr)
			}
			node.assertCalls(t, tt.wantCalls...)
			if !tt.wantRun {
				if got.stdout != "" {
					t.Errorf("got %q, want the unit left unstarted", got.stdout)
				}
				return
			}
			if got.stdout != "started\n" {
				t.Errorf("got %q, want the launcher exec'd", got.stdout)
			}
			if body := node.read(t, node.record); body != overlayMeta {
				t.Errorf("got %q, want the record published for eru-agent", body)
			}
		})
	}
}

func TestStopScriptStopsTheUnitAndDropsTheOverlay(t *testing.T) {
	tests := []struct {
		name       string
		loadState  string
		mountpoint string
		force      string
		fail       string
		wantCode   int
		wantCalls  []string
	}{
		{
			name:       "a forced stop kills first",
			loadState:  "loaded",
			mountpoint: "1",
			force:      "1",
			wantCalls: []string{
				"systemctl kill -s SIGKILL " + scriptUnit,
				"systemctl stop " + scriptUnit,
				"mountpoint -q {dir}/merged",
			},
		},
		{
			name:       "a graceful stop never kills",
			loadState:  "loaded",
			mountpoint: "1",
			force:      "0",
			wantCalls: []string{
				"systemctl stop " + scriptUnit,
				"mountpoint -q {dir}/merged",
			},
		},
		{
			name:       "an unloaded unit is only unmounted",
			loadState:  "not-found",
			mountpoint: "0",
			force:      "1",
			wantCalls: []string{
				"mountpoint -q {dir}/merged",
				"umount -l {dir}/merged",
			},
		},
		{
			name:       "a kill on a unit systemd already dropped is tolerated",
			loadState:  "loaded",
			mountpoint: "1",
			force:      "1",
			fail:       "kill",
			wantCalls: []string{
				"systemctl kill -s SIGKILL " + scriptUnit,
				"systemctl stop " + scriptUnit,
				"mountpoint -q {dir}/merged",
			},
		},
		{
			name:       "a failed stop is reported",
			loadState:  "loaded",
			mountpoint: "1",
			force:      "1",
			fail:       "stop",
			wantCode:   1,
			wantCalls: []string{
				"systemctl kill -s SIGKILL " + scriptUnit,
				"systemctl stop " + scriptUnit,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := newScriptNode(t)
			node.env["STUB_LOADSTATE"] = tt.loadState
			node.env["STUB_MOUNTPOINT"] = tt.mountpoint
			node.env["STUB_FAIL"] = tt.fail

			got := node.run(t, stopScript, scriptUnit, node.dir, tt.force)

			if got.code != tt.wantCode {
				t.Fatalf("got exit %d, want %d: %s", got.code, tt.wantCode, got.stderr)
			}
			node.assertCalls(t, tt.wantCalls...)
		})
	}
}

func TestRemoveScriptUnmountsTheOverlayItDrops(t *testing.T) {
	node := newScriptNode(t)
	node.env["STUB_LOADSTATE"] = "loaded"
	node.env["STUB_MOUNTPOINT"] = "0"
	node.states(t, "dead")
	node.write(t, node.record, overlayMeta)

	got := node.run(t, removeScript, scriptUnit, node.dir, node.record, "0")

	if got.code != 0 {
		t.Fatalf("got exit %d, want a stopped workload removed: %s", got.code, got.stderr)
	}
	node.assertCalls(t,
		"systemctl reset-failed "+scriptUnit,
		"mountpoint -q {dir}/merged",
		"umount -l {dir}/merged",
	)
	for _, path := range []string{node.dir, node.record} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s survived the remove: %v", path, err)
		}
	}
}

func TestInspectScriptShowsTheUnitOfALiveWorkload(t *testing.T) {
	tests := []struct {
		name       string
		keepDir    bool
		wantCode   int
		wantStdout string
	}{
		{"a workload the node still has", true, 0, showOutput},
		{"a workload the node lost", false, workloadmeta.NotExistsCode, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := newScriptNode(t)
			node.env["STUB_SHOW"] = showOutput
			if !tt.keepDir {
				node.remove(t, node.dir)
			}

			got := node.run(t, inspectScript, node.dir, scriptUnit)

			if got.code != tt.wantCode {
				t.Errorf("got exit %d, want %d", got.code, tt.wantCode)
			}
			if got.stdout != tt.wantStdout {
				t.Errorf("got %q, want %q", got.stdout, tt.wantStdout)
			}
		})
	}
}

func TestWaitScriptPollsUntilTheUnitLeavesRunning(t *testing.T) {
	tests := []struct {
		name       string
		states     []string
		status     string
		wantSleeps int
	}{
		{"a running unit is polled", []string{subStateRunning, subStateRunning, "exited"}, "3", 2},
		{"a dead unit answers at once", []string{"dead"}, "0", 0},
		{"a failed unit answers at once", []string{"failed"}, "1", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := newScriptNode(t)
			node.states(t, tt.states...)
			node.env["STUB_STATUS"] = tt.status

			got := node.run(t, waitScript, scriptUnit)

			if got.code != 0 {
				t.Fatalf("got exit %d, want the wait to end cleanly: %s", got.code, got.stderr)
			}
			if got.stdout != tt.status+"\n" {
				t.Errorf("got %q, want ExecMainStatus %q", got.stdout, tt.status)
			}
			if sleeps := len(node.calls(t)); sleeps != tt.wantSleeps {
				t.Errorf("got %d polls, want %d", sleeps, tt.wantSleeps)
			}
		})
	}
}

func TestListScriptPrintsTheImageCache(t *testing.T) {
	tests := []struct {
		name       string
		entries    []string
		wantStdout string
	}{
		{"a cache with images", []string{"a%2Fb", "c%2Fd"}, "a%2Fb\nc%2Fd\n"},
		{"a node that never pulled", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := newScriptNode(t)
			cache := filepath.Join(node.root, imageCache)
			for _, entry := range tt.entries {
				node.mkdir(t, filepath.Join(cache, entry))
			}

			got := node.run(t, listScript, cache)

			if got.code != 0 {
				t.Fatalf("got exit %d, want a missing cache reported as empty: %s", got.code, got.stderr)
			}
			if got.stdout != tt.wantStdout {
				t.Errorf("got %q, want %q", got.stdout, tt.wantStdout)
			}
		})
	}
}

func TestPullScriptUnpacksTheBundleAndRecordsTheDigest(t *testing.T) {
	node := newScriptNode(t)
	node.env["STUB_ORAS_FILES"] = "bundle.tar"
	node.env["STUB_DESCRIPTOR"] = scriptDescriptor
	node.env["STUB_TAR_MEMBER"] = "server"
	cache := filepath.Join(node.root, imageCache, "app")
	node.write(t, filepath.Join(cache, "stale"), "old")

	got := node.run(t, pullScript, scriptRef, cache, "--plain-http")

	if got.code != 0 {
		t.Fatalf("got exit %d, want the pull to land: %s", got.code, got.stderr)
	}
	node.assertCalls(t,
		"oras pull "+scriptRef+" -o "+cache+" --plain-http",
		"tar -C "+cache+" -xf "+cache+"/bundle.tar",
		"oras manifest fetch --descriptor "+scriptRef+" --plain-http",
	)
	if body := node.read(t, filepath.Join(cache, digestFile)); body != scriptDescriptor+"\n" {
		t.Errorf("got %q, want the descriptor cached", body)
	}
	for _, path := range []string{filepath.Join(cache, "bundle.tar"), filepath.Join(cache, "stale")} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s survived the pull: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(cache, "server")); err != nil {
		t.Errorf("the unpacked bundle is missing: %v", err)
	}
}

func TestPullScriptLeavesAnUnpackedArtifactAlone(t *testing.T) {
	node := newScriptNode(t)
	node.env["STUB_ORAS_FILES"] = "server"
	node.env["STUB_DESCRIPTOR"] = scriptDescriptor
	cache := filepath.Join(node.root, imageCache, "app")

	got := node.run(t, pullScript, scriptRef, cache)

	if got.code != 0 {
		t.Fatalf("got exit %d, want the pull to land: %s", got.code, got.stderr)
	}
	node.assertCalls(t,
		"oras pull "+scriptRef+" -o "+cache,
		"oras manifest fetch --descriptor "+scriptRef,
	)
}

func TestPullScriptRecordsNoDigestWhenTheRegistryRefuses(t *testing.T) {
	node := newScriptNode(t)
	node.env["STUB_ORAS_PULL"] = "1"
	cache := filepath.Join(node.root, imageCache, "app")

	got := node.run(t, pullScript, scriptRef, cache)

	if got.code == 0 {
		t.Fatal("got exit 0, want a failed pull reported")
	}
	if _, err := os.Stat(filepath.Join(cache, digestFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a failed pull must leave no digest: %v", err)
	}
}

func TestCreateScriptPullsTheBundleAndPublishesTheRecord(t *testing.T) {
	node := newScriptNode(t)
	node.env["STUB_ORAS_FILES"] = "bundle.tar"
	node.env["STUB_TAR_MEMBER"] = "server"
	bind := filepath.Join(node.root, "data")

	got := node.create(t, "1", filepath.Join(node.root, "cache"), bind, "--plain-http")

	if got.code != 0 {
		t.Fatalf("got exit %d, want the workload created: %s", got.code, got.stderr)
	}
	node.assertCalls(t,
		"oras pull "+scriptRef+" -o {dir}/lower --plain-http",
		"tar -C {dir}/lower -xf {dir}/lower/bundle.tar",
	)
	for _, path := range []string{"lower/server", "upper", "work", "merged"} {
		if _, err := os.Stat(filepath.Join(node.dir, path)); err != nil {
			t.Errorf("%s is missing: %v", path, err)
		}
	}
	if _, err := os.Stat(bind); err != nil {
		t.Errorf("the bind source was not created: %v", err)
	}
	for path, want := range map[string]string{
		filepath.Join(node.dir, "run.sh"):    scriptLauncher + "\n",
		filepath.Join(node.dir, propsFile):   "CPUQuota=200%\n",
		filepath.Join(node.dir, "meta.json"): overlayMeta + "\n",
		node.record:                          overlayMeta + "\n",
	} {
		if body := node.read(t, path); body != want {
			t.Errorf("got %q in %s, want %q", body, path, want)
		}
	}
}

func TestCreateScriptSeedsTheLowerDirFromTheImageCache(t *testing.T) {
	node := newScriptNode(t)
	cache := filepath.Join(node.root, "cache")
	node.write(t, filepath.Join(cache, "server"), "binary")
	node.write(t, filepath.Join(cache, digestFile), scriptDescriptor)

	got := node.create(t, "1", cache, "")

	if got.code != 0 {
		t.Fatalf("got exit %d, want the cached bundle copied: %s", got.code, got.stderr)
	}
	node.assertCalls(t)
	if body := node.read(t, filepath.Join(node.dir, "lower", "server")); body != "binary" {
		t.Errorf("got %q, want the cached bundle in place", body)
	}
	if _, err := os.Stat(filepath.Join(node.dir, "lower", digestFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the digest of the cache must not reach the workload: %v", err)
	}
}

func TestCreateScriptGivesARawWorkloadNoOverlay(t *testing.T) {
	node := newScriptNode(t)
	node.env["STUB_ORAS_FILES"] = "server"

	got := node.create(t, "0", filepath.Join(node.root, "cache"), "")

	if got.code != 0 {
		t.Fatalf("got exit %d, want the workload created: %s", got.code, got.stderr)
	}
	for _, path := range []string{"upper", "work", "merged"} {
		if _, err := os.Stat(filepath.Join(node.dir, path)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("a raw workload must not get %s: %v", path, err)
		}
	}
}

func TestCreateScriptDropsTheWorkloadDirWhenThePullFails(t *testing.T) {
	node := newScriptNode(t)
	node.env["STUB_ORAS_PULL"] = "1"

	got := node.create(t, "1", filepath.Join(node.root, "cache"), "")

	if got.code == 0 {
		t.Fatal("got exit 0, want a failed pull reported")
	}
	for _, path := range []string{node.dir, node.record} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s survived a failed create: %v", path, err)
		}
	}
}

func TestExistScriptCapturesTheFrozenOverlay(t *testing.T) {
	node := newScriptNode(t)
	node.env["STUB_MOUNTPOINT"] = "1"
	node.env["STUB_DESCRIPTOR"] = scriptDescriptor
	layer := filepath.Join(node.dir, existArchive)

	got := node.run(t, existScript, scriptUnit, node.dir, scriptRef, layer, "--plain-http")

	if got.code != 0 {
		t.Fatalf("got exit %d, want the bundle pushed: %s", got.code, got.stderr)
	}
	if got.stdout != scriptDescriptor+"\n" {
		t.Errorf("got %q, want the descriptor of the pushed artifact", got.stdout)
	}
	node.assertCalls(t,
		"mountpoint -q {dir}/merged",
		"mount -t overlay overlay -o lowerdir={dir}/lower,upperdir={dir}/upper,workdir={dir}/work {dir}/merged",
		"systemctl freeze "+scriptUnit,
		"tar -C {dir}/merged -cf "+layer+" .",
		"systemctl thaw "+scriptUnit,
		"oras push --disable-path-validation --artifact-type "+bundleMedia+" "+scriptRef+" "+layer+":"+bundleMedia+" --plain-http",
		"oras manifest fetch --descriptor "+scriptRef+" --plain-http",
		"systemctl thaw "+scriptUnit,
		"umount -l {dir}/merged",
	)
	if _, err := os.Stat(layer); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the layer archive survived the push: %v", err)
	}
}

func TestExistScriptKeepsAnOverlayItDidNotMount(t *testing.T) {
	node := newScriptNode(t)
	node.env["STUB_MOUNTPOINT"] = "0"
	node.env["STUB_DESCRIPTOR"] = scriptDescriptor
	layer := filepath.Join(node.dir, existArchive)

	got := node.run(t, existScript, scriptUnit, node.dir, scriptRef, layer)

	if got.code != 0 {
		t.Fatalf("got exit %d, want the bundle pushed: %s", got.code, got.stderr)
	}
	for _, call := range node.calls(t) {
		if strings.HasPrefix(call, "mount ") || strings.HasPrefix(call, "umount ") {
			t.Errorf("got %q, want the running workload's own mount left alone", call)
		}
	}
}

func TestExistScriptThawsTheUnitWhenTheCaptureFails(t *testing.T) {
	node := newScriptNode(t)
	node.env["STUB_MOUNTPOINT"] = "1"
	node.env["STUB_TAR"] = "2"
	layer := filepath.Join(node.dir, existArchive)

	got := node.run(t, existScript, scriptUnit, node.dir, scriptRef, layer)

	if got.code != 2 {
		t.Fatalf("got exit %d, want the tar failure reported", got.code)
	}
	node.assertCalls(t,
		"mountpoint -q {dir}/merged",
		"mount -t overlay overlay -o lowerdir={dir}/lower,upperdir={dir}/upper,workdir={dir}/work {dir}/merged",
		"systemctl freeze "+scriptUnit,
		"tar -C {dir}/merged -cf "+layer+" .",
		"systemctl thaw "+scriptUnit,
		"umount -l {dir}/merged",
	)
}

func TestFollowScriptEndsTheJournalWithTheUnit(t *testing.T) {
	node := newScriptNode(t)
	node.states(t, subStateRunning, "dead")
	node.env["STUB_JOURNAL"] = "boot line"
	node.env["STUB_JOURNAL_HOLD"] = "1"

	got := node.run(t, followScript, scriptUnit, "-f", "-n", "10")

	if got.code != 0 {
		t.Fatalf("got exit %d, want the follow to end cleanly: %s", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "boot line") {
		t.Errorf("got %q, want the journal streamed", got.stdout)
	}
	node.assertHasCalls(t, "journalctl -u "+scriptUnit+" -f -n 10", "sleep 1", "journalctl killed")
}

func TestChdirScriptRunsTheCommandInTheWorkingDir(t *testing.T) {
	node := newScriptNode(t)

	got := node.run(t, chdirScript, node.dir, "pwd")

	if got.code != 0 {
		t.Fatalf("got exit %d, want the command run: %s", got.code, got.stderr)
	}
	if strings.TrimSpace(got.stdout) != node.dir {
		t.Errorf("got %q, want the command run in %q", got.stdout, node.dir)
	}
}

func TestChdirScriptFailsOnAWorkingDirThatIsGone(t *testing.T) {
	node := newScriptNode(t)

	got := node.run(t, chdirScript, filepath.Join(node.root, "gone"), "pwd")

	if got.code == 0 {
		t.Fatal("got exit 0, want an unusable working dir reported")
	}
	if got.stdout != "" {
		t.Errorf("got %q, want the command left unrun", got.stdout)
	}
}

type scriptRun struct {
	code   int
	stdout string
	stderr string
}

type scriptNode struct {
	root    string
	bin     string
	dir     string
	record  string
	logPath string
	path    string
	env     map[string]string
}

func newScriptNode(t *testing.T) *scriptNode {
	t.Helper()
	root := t.TempDir()
	node := &scriptNode{
		root:    root,
		bin:     filepath.Join(root, "bin"),
		dir:     filepath.Join(root, "workloads", "w1"),
		record:  filepath.Join(root, "run", "w1.json"),
		logPath: filepath.Join(root, "calls.log"),
	}
	node.path = node.bin + ":/usr/bin:/bin"
	node.env = map[string]string{
		"STUB_LOG":           node.logPath,
		"STUB_SUBSTATE_FILE": filepath.Join(root, "substate"),
		"STUB_LOADSTATE":     "not-found",
	}
	node.mkdir(t, node.dir)
	for name, body := range map[string]string{
		"systemctl":  systemctlShim,
		"mountpoint": mountpointShim,
		"mount":      mountShim,
		"umount":     umountShim,
		"sleep":      sleepShim,
		"tar":        tarShim,
		"oras":       orasShim,
		"journalctl": journalctlShim,
	} {
		node.write(t, filepath.Join(node.bin, name), body)
		if err := os.Chmod(filepath.Join(node.bin, name), 0o755); err != nil {
			t.Fatalf("setup %s: %v", name, err)
		}
	}
	node.states(t, "dead")
	return node
}

func (n *scriptNode) run(t *testing.T, script string, args ...string) scriptRun {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "/bin/sh", slices.Concat([]string{"-c", script, "sh"}, args)...)
	cmd.Env = slices.Concat([]string{"PATH=" + n.path, "HOME=" + n.root, "TMPDIR=" + os.TempDir()}, n.environ())
	stdout, stderr := &strings.Builder{}, &strings.Builder{}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	got := scriptRun{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		got.code = exitErr.ExitCode()
	default:
		t.Fatalf("run: %v", err)
	}
	return got
}

func (n *scriptNode) create(t *testing.T, overlay, cache, bind string, flags ...string) scriptRun {
	t.Helper()
	args := slices.Concat([]string{
		n.dir, scriptRef, cache, scriptLauncher, n.record, overlay, overlayMeta, bind, "CPUQuota=200%",
	}, flags)
	return n.run(t, createScript, args...)
}

func (n *scriptNode) environ() []string {
	env := make([]string, 0, len(n.env))
	for key, value := range n.env {
		env = append(env, key+"="+value)
	}
	return env
}

func (n *scriptNode) states(t *testing.T, states ...string) {
	t.Helper()
	n.write(t, n.env["STUB_SUBSTATE_FILE"], strings.Join(states, "\n")+"\n")
}

func (n *scriptNode) calls(t *testing.T) []string {
	t.Helper()
	body, err := os.ReadFile(n.logPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(body), "\n"), "\n")
}

func (n *scriptNode) assertCalls(t *testing.T, want ...string) {
	t.Helper()
	got := n.calls(t)
	expanded := make([]string, 0, len(want))
	for _, line := range want {
		expanded = append(expanded, n.expand(line))
	}
	if !slices.Equal(got, expanded) {
		t.Errorf("got calls %q, want %q", got, expanded)
	}
}

func (n *scriptNode) assertHasCalls(t *testing.T, want ...string) {
	t.Helper()
	got := n.calls(t)
	for _, line := range want {
		if !slices.Contains(got, n.expand(line)) {
			t.Errorf("got calls %q, want one of them to be %q", got, n.expand(line))
		}
	}
}

func (n *scriptNode) expand(line string) string {
	return strings.ReplaceAll(line, "{dir}", n.dir)
}

func (n *scriptNode) write(t *testing.T, path, body string) {
	t.Helper()
	n.mkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("setup %s: %v", path, err)
	}
}

func (n *scriptNode) read(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

func (n *scriptNode) mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("setup %s: %v", path, err)
	}
}

func (n *scriptNode) remove(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("setup %s: %v", path, err)
	}
}
