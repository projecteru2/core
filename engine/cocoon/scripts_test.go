package cocoon

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
	scriptVM      = "w1"
	scriptSnap    = snapshotPrefix + scriptVM
	scriptConsole = "/var/lib/cocoon/run/cloudhypervisor/" + scriptVM + "/console.sock"
	scriptDurable = `{"id":"w1","kind":"vm","log":{"console_socket":"/old/console.sock"},"netns_pid":0}`
	scriptEvents  = `{"event":"UPDATED","vm":{"state":"running"}}
{"event":"DELETED","vm":{"state":"stopped"}}`

	cocoonShim = `#!/bin/sh
printf 'cocoon %s\n' "$*" >> "$STUB_LOG"
pop() {
[ -f "$1" ] || return 0
head -n 1 "$1"
if [ "$(wc -l < "$1")" -gt 1 ]; then
tail -n +2 "$1" > "$1.tmp"
mv "$1.tmp" "$1"
fi
}
case "$1 $2" in
"vm inspect")
[ "${STUB_INSPECT:-0}" = 0 ] || exit "$STUB_INSPECT"
pop "$STUB_VM_FILE"
;;
"vm start") exit "${STUB_START:-0}";;
"vm stop") exit "${STUB_STOP:-0}";;
"vm rm")
printf '%s' "$STUB_RM_STDERR" >&2
exit "${STUB_RM:-0}"
;;
"vm hibernate") exit "${STUB_HIBERNATE:-0}";;
"vm restore") exit "${STUB_RESTORE:-0}";;
"vm exec")
code=$(pop "$STUB_EXEC_FILE")
exit "${code:-0}"
;;
"vm status") printf '%s\n' "$STUB_EVENTS";;
"snapshot rm") exit "${STUB_SNAPSHOT:-0}";;
"image inspect") exit "${STUB_IMAGE:-1}";;
"image import") exit "${STUB_IMPORT:-0}";;
esac
exit 0
`

	orasShim = `#!/bin/sh
printf 'oras %s\n' "$*" >> "$STUB_LOG"
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
exit 0
`

	sedShim = `#!/bin/sh
PATH=/usr/bin:/bin
export PATH
printf 'sed %s\n' "$*" >> "$STUB_LOG"
[ "$1" = "-i" ] || exec sed "$@"
shift
n=$#
i=1
while [ "$i" -lt "$n" ]; do
set -- "$@" "$1"
shift
i=$((i+1))
done
file=$1
shift
[ -f "$file" ] || exit 1
sed "$@" "$file" > "$file.edited" || exit 1
mv "$file.edited" "$file"
exit 0
`

	sleepShim = `#!/bin/sh
printf 'sleep %s\n' "$*" >> "$STUB_LOG"
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

func TestRecordScriptWritesBothCopiesOfTheRecord(t *testing.T) {
	node := newScriptNode(t)

	got := node.run(t, recordScript, node.durable, node.record, storedRecord)

	if got.code != 0 {
		t.Fatalf("got exit %d, want the record written: %s", got.code, got.stderr)
	}
	for _, path := range []string{node.durable, node.record} {
		if body := node.read(t, path); body != storedRecord+"\n" {
			t.Errorf("got %q in %s, want %q", body, path, storedRecord+"\n")
		}
		if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s.tmp survived the rename: %v", path, err)
		}
	}
}

func TestRecordScriptPublishesNothingWhenTheDurableCopyFails(t *testing.T) {
	node := newScriptNode(t)
	node.write(t, filepath.Join(node.root, "blocked"), "")

	got := node.run(t, recordScript, filepath.Join(node.root, "blocked", "w1.json"), node.record, storedRecord)

	if got.code == 0 {
		t.Fatal("got exit 0, want an unwritable root reported")
	}
	if _, err := os.Stat(node.record); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("eru-agent must not see a record core could not store: %v", err)
	}
}

func TestStartScriptInspectsTheVMAroundTheBoot(t *testing.T) {
	tests := []struct {
		name        string
		keepDurable bool
		start       string
		wantCode    int
		wantCalls   []string
		wantStdout  string
	}{
		{
			name:        "a booted vm reports its record twice",
			keepDurable: true,
			wantCalls: []string{
				"cocoon vm inspect " + scriptVM,
				"cocoon vm start " + scriptVM,
				"cocoon vm inspect " + scriptVM,
			},
			wantStdout: linuxVM + "\n" + runningVM + "\n",
		},
		{
			name:      "a vm the node lost",
			wantCode:  workloadmeta.NotExistsCode,
			wantCalls: nil,
		},
		{
			name:        "a vm that will not boot",
			keepDurable: true,
			start:       "3",
			wantCode:    3,
			wantCalls: []string{
				"cocoon vm inspect " + scriptVM,
				"cocoon vm start " + scriptVM,
			},
			wantStdout: linuxVM + "\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := newScriptNode(t)
			node.env["STUB_START"] = tt.start
			node.records(t, linuxVM, runningVM)
			if tt.keepDurable {
				node.write(t, node.durable, scriptDurable)
			}

			got := node.run(t, startScript, node.binary, scriptVM, node.durable)

			if got.code != tt.wantCode {
				t.Fatalf("got exit %d, want %d: %s", got.code, tt.wantCode, got.stderr)
			}
			if got.stdout != tt.wantStdout {
				t.Errorf("got %q, want %q", got.stdout, tt.wantStdout)
			}
			node.assertCalls(t, tt.wantCalls...)
		})
	}
}

func TestRefreshScriptRewritesTheConsoleAndThePID(t *testing.T) {
	node := newScriptNode(t)
	node.write(t, node.durable, scriptDurable)

	got := node.run(t, refreshScript, node.durable, node.record, scriptConsole, "4242")

	if got.code != 0 {
		t.Fatalf("got exit %d, want the record refreshed: %s", got.code, got.stderr)
	}
	want := `{"id":"w1","kind":"vm","log":{"console_socket":"` + scriptConsole + `"},"netns_pid":4242}`
	if body := node.read(t, node.durable); body != want {
		t.Errorf("got %q, want %q", body, want)
	}
	if body := node.read(t, node.record); body != want {
		t.Errorf("got %q, want the refreshed record published", body)
	}
}

func TestRefreshScriptPublishesNothingForAVMTheNodeLost(t *testing.T) {
	node := newScriptNode(t)

	got := node.run(t, refreshScript, node.durable, node.record, scriptConsole, "4242")

	if got.code == 0 {
		t.Fatal("got exit 0, want a missing durable record reported")
	}
	if _, err := os.Stat(node.record); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("eru-agent must not see a record of a vm that is gone: %v", err)
	}
}

func TestAddressScriptRetriesUntilTheGuestAgentAnswers(t *testing.T) {
	tests := []struct {
		name      string
		codes     []string
		wantCode  int
		wantExecs int
	}{
		{"a guest that answers at once", []string{"0"}, 0, 1},
		{"a guest that boots slowly", []string{"1", "1", "0"}, 0, 3},
		{"a guest that never answers", []string{"1"}, 1, 90},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := newScriptNode(t)
			node.write(t, node.env["STUB_EXEC_FILE"], strings.Join(tt.codes, "\n")+"\n")

			got := node.run(t, addressScript, node.binary, scriptVM, "10.22.0.5", "255.255.0.0", "10.22.0.1")

			if got.code != tt.wantCode {
				t.Fatalf("got exit %d, want %d: %s", got.code, tt.wantCode, got.stderr)
			}
			execs := 0
			want := "cocoon vm exec " + scriptVM + " -- netsh interface ip set address " + guestIface + " static 10.22.0.5 255.255.0.0 10.22.0.1"
			for _, call := range node.calls(t) {
				if call == want {
					execs++
				}
			}
			if execs != tt.wantExecs {
				t.Errorf("got %d attempts, want %d", execs, tt.wantExecs)
			}
		})
	}
}

func TestRemoveScriptDropsTheVMAndBothRecords(t *testing.T) {
	tests := []struct {
		name        string
		keepDurable bool
		force       string
		rm          string
		inspect     string
		wantCode    int
		wantCalls   []string
		wantGone    bool
	}{
		{
			name:        "a forced remove",
			keepDurable: true,
			force:       "1",
			wantCalls: []string{
				"cocoon vm rm --force " + scriptVM,
				"cocoon snapshot rm " + scriptSnap,
			},
			wantGone: true,
		},
		{
			name:        "a graceful remove",
			keepDurable: true,
			force:       "0",
			wantCalls: []string{
				"cocoon vm rm " + scriptVM,
				"cocoon snapshot rm " + scriptSnap,
			},
			wantGone: true,
		},
		{
			name:        "a vm cocoon had already dropped",
			keepDurable: true,
			force:       "0",
			rm:          "1",
			inspect:     "1",
			wantCalls: []string{
				"cocoon vm rm " + scriptVM,
				"cocoon vm inspect " + scriptVM,
				"cocoon snapshot rm " + scriptSnap,
			},
			wantGone: true,
		},
		{
			name:        "a vm that refused to go",
			keepDurable: true,
			force:       "0",
			rm:          "1",
			wantCode:    1,
			wantCalls: []string{
				"cocoon vm rm " + scriptVM,
				"cocoon vm inspect " + scriptVM,
			},
		},
		{
			name:     "a vm the node lost",
			force:    "1",
			wantCode: workloadmeta.NotExistsCode,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := newScriptNode(t)
			node.env["STUB_RM"] = tt.rm
			node.env["STUB_INSPECT"] = tt.inspect
			node.env["STUB_RM_STDERR"] = "vm is running"
			node.records(t, runningVM)
			node.write(t, node.record, storedRecord)
			if tt.keepDurable {
				node.write(t, node.durable, scriptDurable)
			}

			got := node.run(t, removeScript, node.binary, scriptVM, node.durable, node.record, scriptSnap, tt.force)

			if got.code != tt.wantCode {
				t.Fatalf("got exit %d, want %d: %s", got.code, tt.wantCode, got.stderr)
			}
			node.assertCalls(t, tt.wantCalls...)
			if tt.wantCode == 1 && !strings.Contains(got.stderr, "vm is running") {
				t.Errorf("got %q, want the cocoon failure reported", got.stderr)
			}
			for _, path := range []string{node.durable, node.record} {
				_, err := os.Stat(path)
				if tt.wantGone && !errors.Is(err, os.ErrNotExist) {
					t.Errorf("%s survived the remove: %v", path, err)
				}
				if !tt.wantGone && tt.keepDurable && err != nil {
					t.Errorf("a refused remove must keep %s: %v", path, err)
				}
			}
		})
	}
}

func TestSuspendScriptReplacesTheSnapshotItHibernatesInto(t *testing.T) {
	tests := []struct {
		name      string
		hibernate string
		wantCode  int
	}{
		{"a guest that hibernates", "", 0},
		{"a guest that cannot hibernate", "4", 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := newScriptNode(t)
			node.env["STUB_HIBERNATE"] = tt.hibernate
			node.env["STUB_SNAPSHOT"] = "1"

			got := node.run(t, suspendScript, node.binary, scriptVM, scriptSnap)

			if got.code != tt.wantCode {
				t.Fatalf("got exit %d, want %d: %s", got.code, tt.wantCode, got.stderr)
			}
			node.assertCalls(t,
				"cocoon snapshot rm "+scriptSnap,
				"cocoon vm hibernate --name "+scriptSnap+" "+scriptVM,
			)
		})
	}
}

func TestResumeScriptRestoresByCopyAndDropsTheSnapshot(t *testing.T) {
	node := newScriptNode(t)
	node.records(t, runningVM)

	got := node.run(t, resumeScript, node.binary, scriptVM, scriptSnap)

	if got.code != 0 {
		t.Fatalf("got exit %d, want the guest resumed: %s", got.code, got.stderr)
	}
	if got.stdout != runningVM+"\n" {
		t.Errorf("got %q, want the record of the resumed vm", got.stdout)
	}
	node.assertCalls(t,
		"cocoon vm restore --restore-mode copy "+scriptVM+" "+scriptSnap,
		"cocoon snapshot rm "+scriptSnap,
		"cocoon vm inspect "+scriptVM,
	)
}

func TestResumeScriptKeepsTheSnapshotWhenTheRestoreFails(t *testing.T) {
	node := newScriptNode(t)
	node.env["STUB_RESTORE"] = "5"
	node.records(t, runningVM)

	got := node.run(t, resumeScript, node.binary, scriptVM, scriptSnap)

	if got.code != 5 {
		t.Fatalf("got exit %d, want the restore failure reported", got.code)
	}
	node.assertCalls(t, "cocoon vm restore --restore-mode copy "+scriptVM+" "+scriptSnap)
}

func TestStopScriptPassesTheStopFlagsToCocoon(t *testing.T) {
	tests := []struct {
		name        string
		keepDurable bool
		stop        string
		flags       []string
		wantCode    int
		wantCalls   []string
	}{
		{
			name:        "a graceful stop",
			keepDurable: true,
			flags:       []string{"--timeout", "30"},
			wantCalls:   []string{"cocoon vm stop --timeout 30 " + scriptVM},
		},
		{
			name:        "a forced stop",
			keepDurable: true,
			flags:       []string{"--force"},
			wantCalls:   []string{"cocoon vm stop --force " + scriptVM},
		},
		{
			name:        "a guest that will not stop",
			keepDurable: true,
			stop:        "6",
			wantCode:    6,
			wantCalls:   []string{"cocoon vm stop " + scriptVM},
		},
		{
			name:     "a vm the node lost",
			wantCode: workloadmeta.NotExistsCode,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := newScriptNode(t)
			node.env["STUB_STOP"] = tt.stop
			if tt.keepDurable {
				node.write(t, node.durable, scriptDurable)
			}

			got := node.run(t, stopScript, slices.Concat([]string{node.binary, scriptVM, node.durable}, tt.flags)...)

			if got.code != tt.wantCode {
				t.Fatalf("got exit %d, want %d: %s", got.code, tt.wantCode, got.stderr)
			}
			node.assertCalls(t, tt.wantCalls...)
		})
	}
}

func TestInspectScriptPrintsTheRecordAndTheVM(t *testing.T) {
	tests := []struct {
		name        string
		keepDurable bool
		inspect     string
		wantCode    int
		wantStdout  string
	}{
		{"a vm the node runs", true, "", 0, scriptDurable + "\n" + runningVM + "\n"},
		{"a vm cocoon no longer knows", true, "7", 7, scriptDurable + "\n"},
		{"a vm the node lost", false, "", workloadmeta.NotExistsCode, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := newScriptNode(t)
			node.env["STUB_INSPECT"] = tt.inspect
			node.records(t, runningVM)
			if tt.keepDurable {
				node.write(t, node.durable, scriptDurable+"\n")
			}

			got := node.run(t, inspectScript, node.binary, scriptVM, node.durable)

			if got.code != tt.wantCode {
				t.Fatalf("got exit %d, want %d: %s", got.code, tt.wantCode, got.stderr)
			}
			if got.stdout != tt.wantStdout {
				t.Errorf("got %q, want %q", got.stdout, tt.wantStdout)
			}
		})
	}
}

func TestWaitScriptFollowsTheStatusStreamOfALiveVM(t *testing.T) {
	node := newScriptNode(t)
	node.env["STUB_EVENTS"] = scriptEvents
	node.records(t, runningVM)

	got := node.run(t, waitScript, node.binary, scriptVM)

	if got.code != 0 {
		t.Fatalf("got exit %d, want the stream followed: %s", got.code, got.stderr)
	}
	if got.stdout != scriptEvents+"\n" {
		t.Errorf("got %q, want the status events", got.stdout)
	}
	node.assertCalls(t,
		"cocoon vm inspect "+scriptVM,
		"cocoon vm status --event --format json -n 1 "+scriptVM,
	)
}

func TestWaitScriptRefusesToWaitOnAVMCocoonDoesNotHave(t *testing.T) {
	node := newScriptNode(t)
	node.env["STUB_INSPECT"] = "1"

	got := node.run(t, waitScript, node.binary, scriptVM)

	if got.code != 1 {
		t.Fatalf("got exit %d, want 1: the event stream stays silent for a vm that is not there", got.code)
	}
	node.assertCalls(t, "cocoon vm inspect "+scriptVM)
}

func TestImportScriptReassemblesAPartsArtifactOnce(t *testing.T) {
	node := newScriptNode(t)
	node.env["STUB_ORAS_FILES"] = "disk.0.part disk.1.part"

	got := node.run(t, importScript, node.binary, testImage)

	if got.code != 0 {
		t.Fatalf("got exit %d, want the parts imported: %s", got.code, got.stderr)
	}
	calls := node.calls(t)
	if len(calls) != 3 {
		t.Fatalf("got calls %q, want an inspect, a pull and an import", calls)
	}
	tmp := strings.TrimPrefix(calls[1], "oras pull "+testImage+" -o ")
	node.assertCalls(t,
		"cocoon image inspect "+testImage,
		"oras pull "+testImage+" -o "+tmp,
		"cocoon image import "+testImage+" "+tmp+"/disk.0.part "+tmp+"/disk.1.part",
	)
	if _, err := os.Stat(tmp); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the scratch dir survived the import: %v", err)
	}
}

func TestImportScriptLeavesAnImportedArtifactAlone(t *testing.T) {
	node := newScriptNode(t)
	node.env["STUB_IMAGE"] = "0"

	got := node.run(t, importScript, node.binary, testImage)

	if got.code != 0 {
		t.Fatalf("got exit %d, want the import skipped: %s", got.code, got.stderr)
	}
	node.assertCalls(t, "cocoon image inspect "+testImage)
}

func TestImportScriptImportsNothingWhenThePullFails(t *testing.T) {
	node := newScriptNode(t)
	node.env["STUB_ORAS_PULL"] = "1"

	got := node.run(t, importScript, node.binary, testImage)

	if got.code == 0 {
		t.Fatal("got exit 0, want a failed pull reported")
	}
	for _, call := range node.calls(t) {
		if strings.HasPrefix(call, "cocoon image import") {
			t.Errorf("got %q, want no import of an artifact that never landed", call)
		}
	}
}

func TestOrasProbeAnswersForTheNodeItRunsOn(t *testing.T) {
	tests := []struct {
		name    string
		present bool
	}{
		{"a node with oras", true},
		{"a node without oras", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := newScriptNode(t)
			node.path = node.bin
			if !tt.present {
				node.remove(t, filepath.Join(node.bin, "oras"))
			}

			if got := node.run(t, orasProbe); (got.code == 0) != tt.present {
				t.Errorf("got exit %d, want oras present %v", got.code, tt.present)
			}
		})
	}
}

func TestFollowScriptEndsTheJournalWithTheGuest(t *testing.T) {
	node := newScriptNode(t)
	node.env["STUB_EVENTS"] = scriptEvents
	node.env["STUB_JOURNAL"] = "console line"
	node.env["STUB_JOURNAL_HOLD"] = "1"

	got := node.run(t, followScript, node.binary, scriptVM, "SYSLOG_IDENTIFIER=eru", "ERU_ID="+scriptVM, "-f", "-n", "10")

	if got.code != 0 {
		t.Fatalf("got exit %d, want the follow to end cleanly: %s", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "console line") {
		t.Errorf("got %q, want the journal streamed", got.stdout)
	}
	node.assertHasCalls(t,
		"journalctl SYSLOG_IDENTIFIER=eru ERU_ID="+scriptVM+" -f -n 10",
		"cocoon vm status --event --format json -n 1 "+scriptVM,
		"journalctl killed",
	)
}

type scriptRun struct {
	code   int
	stdout string
	stderr string
}

type scriptNode struct {
	root    string
	bin     string
	binary  string
	durable string
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
		durable: filepath.Join(root, "cocoon", scriptVM+metaSuffix),
		record:  filepath.Join(root, "run", scriptVM+metaSuffix),
		logPath: filepath.Join(root, "calls.log"),
	}
	node.binary = filepath.Join(node.bin, "cocoon")
	node.path = node.bin + ":/usr/bin:/bin"
	node.env = map[string]string{
		"STUB_LOG":       node.logPath,
		"STUB_VM_FILE":   filepath.Join(root, "records"),
		"STUB_EXEC_FILE": filepath.Join(root, "execs"),
	}
	for name, body := range map[string]string{
		"cocoon":     cocoonShim,
		"oras":       orasShim,
		"sed":        sedShim,
		"sleep":      sleepShim,
		"journalctl": journalctlShim,
	} {
		node.write(t, filepath.Join(node.bin, name), body)
		if err := os.Chmod(filepath.Join(node.bin, name), 0o755); err != nil {
			t.Fatalf("setup %s: %v", name, err)
		}
	}
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

func (n *scriptNode) environ() []string {
	env := make([]string, 0, len(n.env))
	for key, value := range n.env {
		env = append(env, key+"="+value)
	}
	return env
}

func (n *scriptNode) records(t *testing.T, records ...string) {
	t.Helper()
	n.write(t, n.env["STUB_VM_FILE"], strings.Join(records, "\n")+"\n")
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
	if !slices.Equal(got, want) {
		t.Errorf("got calls %q, want %q", got, want)
	}
}

func (n *scriptNode) assertHasCalls(t *testing.T, want ...string) {
	t.Helper()
	got := n.calls(t)
	for _, line := range want {
		if !slices.Contains(got, line) {
			t.Errorf("got calls %q, want one of them to be %q", got, line)
		}
	}
}

func (n *scriptNode) write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("setup %s: %v", path, err)
	}
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

func (n *scriptNode) remove(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("setup %s: %v", path, err)
	}
}
