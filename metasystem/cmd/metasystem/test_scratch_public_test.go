package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"golang.org/x/sys/unix"
)

// scratchProbeCheck writes one distinct token into every per-group
// disposable location, reads it back, and reports name, path and token as
// NUL-terminated fields to a probe directory outside the run, so paths with
// spaces survive intact. An unset location fails the group: nothing falls
// back to the caller's HOME or temp.
const scratchProbeCheck = `#!/bin/sh
set -eu
id="$1"
input="$2"
report="$3"
probe="$SCRATCH_PROBE"
token="$id-$$"
mkdir -p "$report"
printf '%s\n' "$$" > "$probe/pid"
for name in HOME TMPDIR GOCACHE XDG_CACHE_HOME XDG_CONFIG_HOME STATICCHECK_CACHE; do
  eval "dir=\${$name:-}"
  if [ -z "$dir" ]; then
    printf '%s\000\000\000' "$name" >> "$probe/env"
    exit 7
  fi
  mkdir -p "$dir"
  printf '%s-%s\n' "$token" "$name" > "$dir/scratch-token-$name"
  printf '%s\000%s\000%s\000' "$name" "$dir" "$(cat "$dir/scratch-token-$name")" >> "$probe/env"
done
if [ -n "${SCRATCH_READY:-}" ]; then
  printf x > "$SCRATCH_READY"
  read _ < "$SCRATCH_RELEASE"
  if printf late > "$HOME/scratch-late"; then printf 'late=ok\n' >> "$probe/late"; else printf 'late=failed\n' >> "$probe/late"; fi
fi
if grep -q '^red' "$input"; then
  printf '<testsuite><testcase classname="portable" name="%s"><failure message="red input"/></testcase></testsuite>\n' "$id" > "$report/result.xml"
  printf 'scratch probe red %s\n' "$token"
  exit 9
fi
printf '<testsuite><testcase classname="portable" name="%s"/></testsuite>\n' "$id" > "$report/result.xml"
`

var scratchProbeLocations = []string{"HOME", "TMPDIR", "GOCACHE", "XDG_CACHE_HOME", "XDG_CONFIG_HOME", "STATICCHECK_CACHE"}

type scratchPublicFixture struct {
	*portableProofFixture
	probe, control, ready, release string
	args                           []string
	// captured holds every custodian group seen in a record, each handed to
	// the fixture reaper when first seen, independent of later records.
	captured map[int64]bool
	// workload is the exact probe process seen at live readiness; a
	// per-fixture cleanup stops it by identity even if its leader is gone.
	workload *identity.Ref
	peerLock os.FileInfo
}

// command shadows the shared fixture's unbounded call: every CLI call of
// this fixture runs in its own reaped process group within the fixture
// exit bound. A timeout fails the phase; output is read only after join.
func (f *scratchPublicFixture) command(args ...string) (int, string) {
	f.t.Helper()
	bound, err := testenv.FixtureExitWaitBound()
	if err != nil {
		f.t.Fatal(err)
	}
	command := f.proofCommand.command(f.commandEnvironment(), f.engine, args...)
	command.Dir = f.root
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	pid := 0
	testenv.ReapFixtureProcessGroups(f.t, []testenv.FixtureProcessGroup{{Verb: "scratch fixture " + strings.Join(args[:min(2, len(args))], " "),
		Resolve: func() (int, bool, error) { return pid, pid != 0, nil }}})
	if err := command.Start(); err != nil {
		f.t.Fatal(err)
	}
	pid = command.Process.Pid
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	var waitErr error
	select {
	case waitErr = <-done:
	case <-time.After(bound):
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		select {
		case <-done:
			f.t.Fatalf("metasystem %s exceeded %s; output=%s", strings.Join(args, " "), bound, output.String())
		case <-time.After(bound):
			f.t.Fatalf("metasystem %s exceeded %s and was not joined after SIGKILL", strings.Join(args, " "), bound)
		}
	}
	if waitErr == nil {
		return 0, output.String()
	}
	var exit *exec.ExitError
	if !errors.As(waitErr, &exit) {
		f.t.Fatalf("run metasystem %s: %v: %s", strings.Join(args, " "), waitErr, output.String())
	}
	return exit.ExitCode(), output.String()
}

func (f *scratchPublicFixture) requireCommand(args ...string) string {
	f.t.Helper()
	status, output := f.command(args...)
	if status != 0 {
		f.t.Fatalf("metasystem %s exited %d: %s", strings.Join(args, " "), status, output)
	}
	return output
}

func (f *scratchPublicFixture) requireReusableRun(args ...string) string {
	f.t.Helper()
	status, output := f.command(args...)
	if status != proofrun.ExitReusableSuccess {
		f.t.Fatalf("metasystem %s exited %d, want reusable success %d: %s", strings.Join(args, " "), status, proofrun.ExitReusableSuccess, output)
	}
	return output
}

// captureWorkload records the live probe process by kernel identity and
// registers its exact stop for fixture cleanup.
func (f *scratchPublicFixture) captureWorkload() {
	f.t.Helper()
	exact, state, err := (identity.KernelProber{}).Probe(int64(f.workloadPID()))
	if err != nil || state != identity.Alive {
		f.t.Fatalf("workload not alive at readiness: state=%s err=%v", state, err)
	}
	ref := exact.Ref()
	f.workload = &ref
	f.t.Cleanup(func() { _ = identity.SignalExact(identity.KernelProber{}, ref, syscall.SIGKILL) })
}

// requireWorkloadReleased: the captured workload is dead or its pid now
// names a different process.
func (f *scratchPublicFixture) requireWorkloadReleased() {
	f.t.Helper()
	if f.workload == nil {
		f.t.Fatal("workload identity was never captured")
	}
	exact, state, err := (identity.KernelProber{}).Probe(f.workload.Pid)
	if err != nil || state == identity.Alive && identity.SameIdentity(exact, *f.workload) && !exact.Zombie {
		f.t.Fatalf("workload %+v still executable: state=%s err=%v", *f.workload, state, err)
	}
}

func newScratchPublicFixture(t *testing.T, input string, blocking bool) *scratchPublicFixture {
	t.Helper()
	fixture := &scratchPublicFixture{probe: t.TempDir(), captured: map[int64]bool{}}
	if blocking {
		fifos := t.TempDir()
		fixture.ready, fixture.release = filepath.Join(fifos, "ready"), filepath.Join(fifos, "release")
		for _, fifo := range []string{fixture.ready, fixture.release} {
			if err := unix.Mkfifo(fifo, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	// The probe script, its environment and a surface owning it belong to
	// the trusted base; the candidate changes only the app input.
	fixture.portableProofFixture = newPortableProofFixtureWithSetup(t, "", func(base *portableProofFixture) {
		base.contract.Surfaces[0].Paths = append(base.contract.Surfaces[0].Paths, "scripts/check.sh")
		env := base.contract.Groups[0].Env
		env["SCRATCH_PROBE"] = fixture.probe
		if blocking {
			env["SCRATCH_READY"], env["SCRATCH_RELEASE"] = fixture.ready, fixture.release
		}
		base.writeContract()
		base.write("scripts/check.sh", scratchProbeCheck, 0o755)
	})
	fixture.write("app/a.txt", input+" (scratch candidate)\n", 0o644)
	tree := fixture.commit("scratch cleanup candidate")
	control, err := canonicalProofRoot(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	fixture.control = control
	fixture.args = []string{"--root", fixture.root, "--goal", "portable", "--tree", tree, "--mode", "auto", "--purpose", "delivery"}
	fixture.requireCommand(append([]string{"test", "plan"}, fixture.args...)...)
	return fixture
}

func (f *scratchPublicFixture) runArgs(result string) []string {
	return append(append([]string{"test", "run"}, f.args...), "--result", result)
}

// observed returns the per-location paths and tokens the workload reported,
// requiring each to be distinct and inside one recorded run root.
func (f *scratchPublicFixture) observed() map[string]string {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.probe, "env"))
	if err != nil {
		f.t.Fatalf("workload never reported its locations: %v", err)
	}
	fields := strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
	if len(fields)%3 != 0 {
		f.t.Fatalf("workload report is not NUL-terminated triples: %q", data)
	}
	paths := map[string]string{}
	for index := 0; index < len(fields); index += 3 {
		name, path, token := fields[index], fields[index+1], fields[index+2]
		if path == "" || !strings.HasSuffix(token, "-"+name) {
			f.t.Fatalf("workload location %s unset or unwritten: path=%q token=%q", name, path, token)
		}
		paths[name] = path
	}
	seen, run := map[string]string{}, ""
	for _, name := range scratchProbeLocations {
		path := paths[name]
		if other, ok := seen[path]; ok || path == "" {
			f.t.Fatalf("location %s=%q is missing or shared with %s", name, path, other)
		}
		seen[path] = name
		owner := f.scratchRunOf(path)
		if owner == "" || run != "" && owner != run {
			f.t.Fatalf("location %s=%q is not inside this run's scratch root (run %q)", name, path, run)
		}
		run = owner
	}
	return paths
}

func (f *scratchPublicFixture) scratchRunOf(path string) string {
	for _, store := range []string{proofrun.ScratchStore(f.root), proofrun.ScratchStore(f.control)} {
		if rest, ok := strings.CutPrefix(path, store+string(os.PathSeparator)); ok {
			run, _, _ := strings.Cut(rest, string(os.PathSeparator))
			return run
		}
	}
	return ""
}

// scratchRecordView reads the record's JSON: current records keep every
// custodian in `custodians`; the legacy singular group may be zero.
type scratchRecordView struct {
	Run        string `json:"run"`
	Root       string `json:"root"`
	Attempt    string `json:"attempt"`
	Custodians []struct {
		Ref   string `json:"ref"`
		Group int64  `json:"group"`
	} `json:"custodians"`
	LegacyGroup int64             `json:"custodianGroup"`
	Worktrees   []json.RawMessage `json:"worktrees"`
}

func (record scratchRecordView) groups() []int64 {
	var groups []int64
	for _, custodian := range record.Custodians {
		if custodian.Group != 0 {
			groups = append(groups, custodian.Group)
		}
	}
	if len(groups) == 0 && record.LegacyGroup != 0 {
		groups = append(groups, record.LegacyGroup)
	}
	return groups
}

func (f *scratchPublicFixture) records() map[string]scratchRecordView {
	f.t.Helper()
	records := map[string]scratchRecordView{}
	matches, _ := filepath.Glob(filepath.Join(proofrun.ScratchStore(f.control), "*.json"))
	for _, match := range matches {
		var record scratchRecordView
		data, err := os.ReadFile(match)
		if err != nil || json.Unmarshal(data, &record) != nil {
			f.t.Fatalf("scratch record %s: %v %s", match, err, data)
		}
		records[record.Run] = record
		for _, group := range record.groups() {
			if !f.captured[group] {
				f.captured[group] = true
				testenv.ReapFixtureProcessGroups(f.t, []testenv.FixtureProcessGroup{{Verb: fmt.Sprintf("scratch custodian group %d", group),
					Resolve: func() (int, bool, error) { return int(group), true, nil }}})
			}
		}
	}
	return records
}

// requireGroupsDrained: no captured custodian group has a live member.
func (f *scratchPublicFixture) requireGroupsDrained() {
	f.t.Helper()
	for group := range f.captured {
		if err := unix.Kill(-int(group), 0); !errors.Is(err, unix.ESRCH) {
			f.t.Errorf("custodian group %d still has members: %v", group, err)
		}
	}
}

func (f *scratchPublicFixture) workloadPID() int {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.probe, "pid"))
	pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || parseErr != nil || pid <= 0 {
		f.t.Fatalf("workload pid: %v %v %q", err, parseErr, data)
	}
	return pid
}

// requireClean inspects world state: the reported locations, every store
// entry except the kept ones, Git worktree registrations and the workload.
func (f *scratchPublicFixture) requireClean(paths map[string]string, keep ...string) {
	f.t.Helper()
	run := f.scratchRunOf(paths["HOME"])
	for _, owned := range []string{run + ".json", run + ".record-lock", run} {
		if _, err := os.Lstat(filepath.Join(proofrun.ScratchStore(f.control), owned)); !os.IsNotExist(err) {
			f.t.Errorf("completed run's %s remains: %v", owned, err)
		}
	}
	for name, path := range paths {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			f.t.Errorf("%s location %s outlived the run: %v", name, path, err)
		}
	}
	entries, err := os.ReadDir(proofrun.ScratchStore(f.control))
	if err != nil && !os.IsNotExist(err) {
		f.t.Fatal(err)
	}
	for _, entry := range entries {
		if !scratchKept(entry.Name(), keep) {
			f.t.Errorf("scratch store retains %s", entry.Name())
		}
	}
	if admin, err := os.ReadDir(filepath.Join(f.root, ".git", "worktrees")); err == nil && len(admin) != 0 {
		f.t.Errorf("Git worktree admin entries remain: %v", admin)
	}
	if list := f.git("worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
		f.t.Errorf("registered worktrees remain:\n%s", list)
	}
	if err := unix.Kill(f.workloadPID(), 0); !errors.Is(err, unix.ESRCH) {
		f.t.Errorf("workload %d still alive: %v", f.workloadPID(), err)
	}
}

func scratchKept(name string, keep []string) bool {
	for _, kept := range keep {
		if name == kept || name == kept+".json" || name == kept+".record-lock" {
			return true
		}
	}
	return false
}

func readScratchResult(t *testing.T, path string) proofrun.TestResult {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var result proofrun.TestResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if err := proofrun.ValidateTestResult(result); err != nil {
		t.Fatalf("result %s: %v", path, err)
	}
	return result
}

// requireRetainedLog: the group's evidence persists outside the scratch root.
func (f *scratchPublicFixture) requireRetainedLog(result proofrun.TestResult, want string) {
	f.t.Helper()
	if len(result.Groups) != 1 || result.Groups[0].LogPath == "" || f.scratchRunOf(result.Groups[0].LogPath) != "" {
		f.t.Fatalf("group log is missing or inside scratch: %+v", result.Groups)
	}
	log, err := os.ReadFile(result.Groups[0].LogPath)
	if err != nil || !strings.Contains(string(log), want) {
		f.t.Fatalf("retained log %s lacks %q: %v %q", result.Groups[0].LogPath, want, err, log)
	}
}

func TestTestRunScratchPassReuseAndVerifyAfterCleanup(t *testing.T) {
	t.Parallel()
	f := newScratchPublicFixture(t, "green a", false)
	first := filepath.Join(t.TempDir(), "first.json")
	f.requireCommand(f.runArgs(first)...)
	paths := f.observed()
	f.requireClean(paths)
	result := readScratchResult(t, first)
	if len(result.Groups) != 1 || result.Groups[0].Status != "passed" {
		t.Fatalf("first run groups: %+v", result.Groups)
	}
	reported, _ := os.ReadFile(filepath.Join(f.probe, "env"))

	// The exact request reuses the attempt under a new, also removed, root.
	repeat := filepath.Join(t.TempDir(), "repeat.json")
	f.requireReusableRun(f.runArgs(repeat)...)
	if again := readScratchResult(t, repeat); again.AttemptID != result.AttemptID {
		t.Fatalf("reuse attempt %s differs from %s", again.AttemptID, result.AttemptID)
	}
	if after, _ := os.ReadFile(filepath.Join(f.probe, "env")); !bytes.Equal(after, reported) {
		t.Fatalf("reusable success launched the workload again:\n%s", after)
	}
	f.requireClean(paths)
	f.requireCommand(append([]string{"test", "verify"}, f.args...)...)
}

func TestTestRunScratchFailureKeepsEvidenceNotScratch(t *testing.T) {
	t.Parallel()
	f := newScratchPublicFixture(t, "red a", false)
	resultPath := filepath.Join(t.TempDir(), "red.json")
	status, output := f.command(f.runArgs(resultPath)...)
	if status == 0 || status == proofrun.ExitReusableSuccess {
		t.Fatalf("red run exited %d: %s", status, output)
	}
	paths := f.observed()
	f.requireClean(paths)
	result := readScratchResult(t, resultPath)
	if result.Groups[0].Status != "failed" || result.Groups[0].NativeExitStatus == nil || *result.Groups[0].NativeExitStatus != 9 {
		t.Fatalf("red group: %+v", result.Groups)
	}
	f.requireRetainedLog(result, "scratch probe red")
	stored, err := proofrun.ReadAttempt(f.control, result.AttemptID)
	if err != nil || stored.AttemptID != result.AttemptID || stored.CandidateTree != result.CandidateTree ||
		stored.Terminal == nil || stored.Terminal.Result != proofrun.TerminalFailed || len(proofrun.CommittedDeliveryReceipt(stored)) != 0 {
		t.Fatalf("failed run lacks its retained terminal-failed attempt: %+v err=%v", stored, err)
	}
	if stored.TestResult != nil && (stored.TestResult.AttemptID != result.AttemptID || stored.TestResult.Groups[0].Status != "failed") {
		t.Fatalf("retained ledger result differs from the CLI result: %+v", stored.TestResult)
	}
}

// blockedScratchRun is one real `test run` whose workload waits on a FIFO.
// Its output is read only after the launcher was joined.
type blockedScratchRun struct {
	f              *scratchPublicFixture
	command        *exec.Cmd
	done           chan error
	output         bytes.Buffer
	ready, release *os.File
	bound          time.Duration
	exited         error
}

// startBlockedScratchRun starts the real CLI and waits, within the fixture
// exit bound, for the workload's readiness byte.
func (f *scratchPublicFixture) startBlockedScratchRun(resultPath string) *blockedScratchRun {
	t := f.t
	t.Helper()
	bound, err := testenv.FixtureExitWaitBound()
	if err != nil {
		t.Fatal(err)
	}
	run := &blockedScratchRun{f: f, bound: bound, done: make(chan error, 1)}
	// A nonblocking descriptor makes the FIFO pollable (os.OpenFile's is
	// not on darwin), so the readiness read honors a deadline.
	readyFD, err := unix.Open(f.ready, unix.O_RDWR|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	run.ready = os.NewFile(uintptr(readyFD), f.ready)
	t.Cleanup(func() { run.ready.Close() })
	if run.release, err = os.OpenFile(f.release, os.O_RDWR, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { run.release.Close() })
	run.command = f.proofCommand.command(f.commandEnvironment(), f.engine, f.runArgs(resultPath)...)
	run.command.Dir = f.root
	run.command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	run.command.Stdout, run.command.Stderr = &run.output, &run.output
	pid := 0
	testenv.ReapFixtureProcessGroups(t, []testenv.FixtureProcessGroup{{Verb: "scratch cleanup launcher", Resolve: func() (int, bool, error) { return pid, pid != 0, nil }}})
	if err := run.command.Start(); err != nil {
		t.Fatal(err)
	}
	// The read ends at the deadline, and an early
	// launcher exit wakes it explicitly, so the reader is always joined.
	if err := run.ready.SetReadDeadline(time.Now().Add(bound)); err != nil {
		t.Fatalf("readiness FIFO has no deadline: %v", err)
	}
	pid = run.command.Process.Pid
	go func() { run.done <- run.command.Wait() }()
	readiness := make(chan error, 1)
	go func() {
		one := make([]byte, 1)
		_, err := io.ReadFull(run.ready, one)
		readiness <- err
	}()
	select {
	case err := <-readiness:
		if err != nil {
			run.fail("no readiness within %s: %v", bound, err)
		}
	case err := <-run.done:
		run.done <- err
		_ = run.ready.SetReadDeadline(time.Now())
		<-readiness
		run.fail("run exited before readiness: %v", err)
	}
	f.captureWorkload()
	f.records() // capture every custodian group recorded so far
	return run
}

// join waits for the launcher within the bound, then its output is safe.
func (r *blockedScratchRun) join() bool {
	select {
	case r.exited = <-r.done:
		return true
	case <-time.After(r.bound):
		return false
	}
}

// fail stops the launcher group, releases the FIFOs, joins, and only then
// reports with the launcher's output; recorded groups are reaped at cleanup.
func (r *blockedScratchRun) fail(format string, args ...any) {
	r.f.t.Helper()
	_ = syscall.Kill(-r.command.Process.Pid, syscall.SIGKILL)
	r.ready.Close()
	r.release.Close()
	output := "(launcher not joined)"
	if r.join() {
		output = r.output.String()
	}
	r.f.records()
	r.f.t.Fatalf(format+"; output=%s", append(args, output)...)
}

// liveScratchWorld proves the blocked workload's tokens exist in its
// recorded root and seeds, beside it, a same-prefix directory with a marker
// and a same-prefix symlink to a foreign directory; exact-ownership cleanup
// must keep both untouched.
func (f *scratchPublicFixture) liveScratchWorld(peerRoot string, paths map[string]string) (string, []string) {
	f.t.Helper()
	for name, path := range paths {
		data, err := os.ReadFile(filepath.Join(path, "scratch-token-"+name))
		if err != nil || !strings.HasSuffix(strings.TrimSpace(string(data)), "-"+name) {
			f.t.Fatalf("live token %s under %s: %v %q", name, path, err, data)
		}
	}
	run := f.scratchRunOf(paths["HOME"])
	record, ok := f.records()[run]
	if !ok || record.Attempt == "" || len(record.groups()) == 0 {
		f.t.Fatalf("live run %s record incomplete: %+v", run, record)
	}
	if _, err := os.Stat(peerRoot); err != nil {
		f.t.Fatalf("live peer root removed by the run's reconciliation: %v", err)
	}
	store := proofrun.ScratchStore(f.control)
	directory, link := run+"-unrelated", run+"-foreign"
	if err := os.Mkdir(filepath.Join(store, directory), 0o700); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store, directory, "marker"), []byte("same prefix, not this run"), 0o600); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.probe, "foreign-marker"), []byte("foreign"), 0o600); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Symlink(f.probe, filepath.Join(store, link)); err != nil {
		f.t.Fatal(err)
	}
	f.t.Logf("SCRATCH_LIVE run=%s attempt=%s custodianGroups=%v worktrees=%d", run, record.Attempt, record.groups(), len(record.Worktrees))
	return run, []string{directory, link}
}

func (f *scratchPublicFixture) seedPeerAndSentinel() (*proofrun.ScratchRun, string) {
	f.t.Helper()
	peer, err := proofrun.CreateScratchRun(f.control)
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { _ = peer.Cleanup(nil) })
	// A real record mutation gives the live peer its record lock.
	if err := peer.RecordAttempt("scratch-peer-attempt"); err != nil {
		f.t.Fatal(err)
	}
	lock, err := os.Lstat(filepath.Join(proofrun.ScratchStore(f.control), peer.ID()+".record-lock"))
	if err != nil || !lock.Mode().IsRegular() {
		f.t.Fatalf("live peer record lock: %v %v", lock, err)
	}
	f.peerLock = lock
	if err := os.WriteFile(filepath.Join(peer.Dir("groups"), "peer-token"), []byte("peer"), 0o600); err != nil {
		f.t.Fatal(err)
	}
	sentinel := filepath.Join(proofrun.ScratchStore(f.control), "unrelated-sentinel")
	if err := os.WriteFile(sentinel, []byte("not a run"), 0o600); err != nil {
		f.t.Fatal(err)
	}
	return peer, filepath.Base(sentinel)
}

// requirePreserved checks the live peer, the plain sentinel, and the exact
// bytes, types and link target of the same-prefix entries.
func (f *scratchPublicFixture) requirePreserved(peer *proofrun.ScratchRun, sentinel string, prefixed []string) {
	f.t.Helper()
	store := proofrun.ScratchStore(f.control)
	if data, err := os.ReadFile(filepath.Join(peer.Dir("groups"), "peer-token")); err != nil || string(data) != "peer" {
		f.t.Fatalf("live peer data lost: %v %q", err, data)
	}
	if _, err := os.Stat(filepath.Join(store, peer.ID()+".json")); err != nil {
		f.t.Fatalf("live peer record removed: %v", err)
	}
	if lock, err := os.Lstat(filepath.Join(store, peer.ID()+".record-lock")); err != nil || !lock.Mode().IsRegular() || !os.SameFile(lock, f.peerLock) {
		f.t.Fatalf("live peer record lock replaced or removed: %v %v", lock, err)
	}
	if data, err := os.ReadFile(filepath.Join(store, sentinel)); err != nil || string(data) != "not a run" {
		f.t.Fatalf("unrelated sentinel changed: %v %q", err, data)
	}
	if info, err := os.Lstat(filepath.Join(store, prefixed[0])); err != nil || !info.IsDir() {
		f.t.Fatalf("same-prefix directory changed: %v %v", info, err)
	}
	if data, err := os.ReadFile(filepath.Join(store, prefixed[0], "marker")); err != nil || string(data) != "same prefix, not this run" {
		f.t.Fatalf("same-prefix marker changed: %v %q", err, data)
	}
	if info, err := os.Lstat(filepath.Join(store, prefixed[1])); err != nil || info.Mode()&os.ModeSymlink == 0 {
		f.t.Fatalf("foreign symlink changed: %v %v", info, err)
	}
	if target, err := os.Readlink(filepath.Join(store, prefixed[1])); err != nil || target != f.probe {
		f.t.Fatalf("foreign symlink target = %q, %v", target, err)
	}
	if data, err := os.ReadFile(filepath.Join(f.probe, "foreign-marker")); err != nil || string(data) != "foreign" {
		f.t.Fatalf("foreign target marker changed: %v %q", err, data)
	}
}

func TestTestRunScratchCancellationDrainsAndDeletes(t *testing.T) {
	t.Parallel()
	f := newScratchPublicFixture(t, "green a", true)
	peer, sentinel := f.seedPeerAndSentinel()
	resultPath := filepath.Join(t.TempDir(), "cancel.json")
	blocked := f.startBlockedScratchRun(resultPath)
	paths := f.observed()
	run, prefixed := f.liveScratchWorld(peer.Root(), paths)
	attempt := f.records()[run].Attempt
	if err := proofrun.RequestCancellation(f.control, attempt, "scratch cleanup cancellation"); err != nil {
		t.Fatal(err)
	}
	if !blocked.join() {
		blocked.fail("cancelled run did not terminate within %s", blocked.bound)
	}
	output := blocked.output.String()
	if blocked.exited == nil || blocked.command.ProcessState.ExitCode() == proofrun.ExitReusableSuccess {
		t.Fatalf("cancelled run exited %v; output=%s", blocked.exited, output)
	}
	f.requireClean(paths, peer.ID(), sentinel, prefixed[0], prefixed[1])
	f.requirePreserved(peer, sentinel, prefixed)
	f.requireGroupsDrained()
	f.requireWorkloadReleased()
	// The concrete cancelled terminal without a delivery receipt is the
	// claim. A nonzero worker may leave no result (the terminal commit then
	// retains nil), so each result copy is validated only where it exists.
	stored, err := proofrun.ReadAttempt(f.control, attempt)
	if err != nil || stored.Terminal == nil || stored.Terminal.Result != proofrun.TerminalCancelled || stored.CancellationIntent == "" ||
		len(proofrun.CommittedDeliveryReceipt(stored)) != 0 {
		t.Fatalf("cancelled attempt lacks a terminal cancelled record: %+v err=%v", stored, err)
	}
	results := map[string]proofrun.TestResult{}
	if _, err := os.Stat(resultPath); err == nil {
		results["result file"] = readScratchResult(t, resultPath)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if stored.TestResult != nil {
		results["attempt ledger"] = *stored.TestResult
	}
	for label, candidate := range results {
		if candidate.AttemptID != attempt || candidate.Delivery.Sufficient || len(candidate.Groups) != 1 ||
			candidate.Groups[0].Status != "cancelled" || !candidate.Groups[0].NativeLaunched {
			t.Fatalf("%s is not an insufficient cancelled result for %s: %+v", label, attempt, candidate)
		}
	}
	t.Logf("SCRATCH_CANCEL attempt=%s retainedResults=%d", attempt, len(results))
	attempts, err := proofrun.ReadAttempts(f.control)
	if err != nil {
		t.Fatal(err)
	}
	for _, recorded := range attempts {
		if recorded.Terminal == nil {
			t.Fatalf("attempt %s still holds its reservation after cancellation", recorded.AttemptID)
		}
	}
}

func TestTestRunScratchLauncherCrashWaitsForDrainThenRecovers(t *testing.T) {
	t.Parallel()
	f := newScratchPublicFixture(t, "green a", true)
	peer, sentinel := f.seedPeerAndSentinel()
	blocked := f.startBlockedScratchRun(filepath.Join(t.TempDir(), "crash.json"))
	paths := f.observed()
	run, prefixed := f.liveScratchWorld(peer.Root(), paths)
	if err := blocked.command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if !blocked.join() {
		blocked.fail("killed launcher did not exit within %s", blocked.bound)
	}
	// The workload still holds the writer lock: recovery must wait.
	for _, outcome := range proofrun.ReconcileScratch(f.control, proofrun.ScratchOptions{}) {
		if outcome.AttemptID == "scratch:"+run && outcome.Action != proofrun.ReconcileScratchPending {
			t.Fatalf("reconciliation touched a root with a live writer: %+v", outcome)
		}
	}
	if _, ok := f.records()[run]; !ok {
		t.Fatal("record removed while its writer lived")
	}
	if _, err := blocked.release.Write([]byte("x\n")); err != nil {
		t.Fatal(err)
	}
	bound := blocked.bound
	workload := f.workloadPID()
	deadline := time.Now().Add(bound)
	removed := false
	// Recovery goes through the standing reconciliation entry, which
	// settles the orphaned attempt first and then its scratch root.
	for !removed && time.Now().Before(deadline) {
		if err := unix.Kill(workload, 0); errors.Is(err, unix.ESRCH) {
			outcomes, err := proofrun.ReconcileAttempts(f.control, proofrun.ReconcileOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for _, outcome := range outcomes {
				if outcome.AttemptID == "scratch:"+run && outcome.Action == proofrun.ReconcileScratchRefused {
					t.Fatalf("recovery refused a drained root: %+v", outcome)
				}
			}
			_, present := f.records()[run]
			removed = !present
		}
		if !removed {
			select {
			case <-t.Context().Done():
				t.Fatal(t.Context().Err())
			case <-time.After(50 * time.Millisecond):
			}
		}
	}
	if !removed {
		t.Fatalf("drained root %s was not recovered within %s", run, bound)
	}
	late, _ := os.ReadFile(filepath.Join(f.probe, "late"))
	// Custody may stop the orphan instead of letting it finish; it must
	// never find its root already deleted.
	if strings.Contains(string(late), "late=failed") {
		t.Fatalf("root vanished before its orphaned workload finished writing:\n%s", late)
	}
	t.Logf("SCRATCH_CRASH run=%s orphanFinished=%t", run, strings.Contains(string(late), "late=ok"))
	f.requireClean(paths, peer.ID(), sentinel, prefixed[0], prefixed[1])
	f.requirePreserved(peer, sentinel, prefixed)
	f.requireGroupsDrained()
	f.requireWorkloadReleased()
}
