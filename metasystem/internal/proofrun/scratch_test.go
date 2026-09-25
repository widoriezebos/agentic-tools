package proofrun

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// scratchProbe reports every pid Dead unless named; a named Alive pid reads
// as this test process's exact start, so a ref built by fakeRef matches it.
type scratchProbe struct {
	self   identity.Exact
	states map[int64]identity.Liveness
}

func (p scratchProbe) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	state, ok := p.states[pid]
	if !ok {
		return identity.Exact{}, identity.Dead, nil
	}
	if state != identity.Alive {
		return identity.Exact{}, state, nil
	}
	exact := p.self
	exact.Pid = pid
	return exact, identity.Alive, nil
}

func newScratchProbe(t *testing.T) scratchProbe {
	t.Helper()
	self, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("self probe: %v", err)
	}
	return scratchProbe{self: self, states: map[int64]identity.Liveness{}}
}

func (p scratchProbe) ref(t *testing.T, pid int64) (identity.Ref, string) {
	t.Helper()
	exact := p.self
	exact.Pid = pid
	ref := exact.Ref()
	encoded, err := identity.EncodeRef(ref)
	if err != nil {
		t.Fatal(err)
	}
	return ref, encoded
}

func readScratchRecord(t *testing.T, control, run string) ScratchRecord {
	t.Helper()
	var record ScratchRecord
	encoded, err := os.ReadFile(filepath.Join(ScratchStore(control), run+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &record); err != nil {
		t.Fatal(err)
	}
	return record
}

func writeScratchRecord(t *testing.T, control string, record ScratchRecord) {
	t.Helper()
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ScratchStore(control), record.Run+".json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

// crashedScratch is a run whose launcher died: its descriptor is gone (the
// kernel closed it) and its recorded launcher reads Dead.
func crashedScratch(t *testing.T, control string, probe scratchProbe, edit func(*ScratchRecord)) ScratchRecord {
	t.Helper()
	run, err := CreateScratchRun(control)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(run.Dir("groups"), "bytes"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = run.writer.Close()
	record := readScratchRecord(t, control, run.ID())
	_, record.Launcher = probe.ref(t, 900001)
	if edit != nil {
		edit(&record)
	}
	writeScratchRecord(t, control, record)
	return record
}

func scratchOutcome(outcomes []ReconcileOutcome, run string) ReconcileOutcome {
	for _, outcome := range outcomes {
		if outcome.AttemptID == "scratch:"+run {
			return outcome
		}
	}
	return ReconcileOutcome{}
}

func TestScratchNormalCleanupRemovesRootAndRecord(t *testing.T) {
	t.Parallel()
	control := t.TempDir()
	run, err := CreateScratchRun(control)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range append([]string{".writer-lock", ".metasystem-scratch", "go.mod"}, ScratchSubdirectories...) {
		if _, err := os.Lstat(filepath.Join(run.Root(), name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(run.Dir("gocache"), "entry"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run.Cleanup(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(run.Root()); !os.IsNotExist(err) {
		t.Fatalf("root survived: %v", err)
	}
	if entries, _ := os.ReadDir(ScratchStore(control)); len(entries) != 0 {
		t.Fatalf("store not empty: %v", entries)
	}
}

// A2: the recovery predicate reads StatusDone records, Unknown liveness and
// a missing process record set as pending; all Dead with the lock free
// removes.
func TestScratchRecoveryPredicateKeepsEveryUnprovenWriterPending(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	control, attempt, record := reconcileFixture(t, now)
	record.Status = StatusDone
	if err := writeRecord(record); err != nil {
		t.Fatal(err)
	}
	probe := newScratchProbe(t)
	done := crashedScratch(t, control, probe, func(r *ScratchRecord) { r.Attempt = attempt.AttemptID })
	members := map[int64][]int64{record.SuiteProcess.Pgid: {4242}}
	options := ScratchOptions{Prober: probe, GroupMembers: func(pgid int64) ([]int64, error) { return members[pgid], nil }}
	if outcome := scratchOutcome(ReconcileScratch(control, options), done.Run); outcome.Action != ReconcileScratchPending ||
		!strings.Contains(outcome.Reason, "live members") {
		t.Fatalf("StatusDone record with a live suite member = %+v", outcome)
	}

	unknown := scratchProbe{self: probe.self, states: map[int64]identity.Liveness{900001: identity.Unknown}}
	if outcome := scratchOutcome(ReconcileScratch(control, ScratchOptions{Prober: unknown, GroupMembers: options.GroupMembers}), done.Run); outcome.Action != ReconcileScratchPending {
		t.Fatalf("Unknown launcher = %+v", outcome)
	}

	missing := crashedScratch(t, control, probe, func(r *ScratchRecord) { r.Attempt = "01MISSINGATTEMPT0000000000" })
	delete(members, record.SuiteProcess.Pgid)
	outcomes := ReconcileScratch(control, options)
	if outcome := scratchOutcome(outcomes, missing.Run); outcome.Action != ReconcileScratchPending {
		t.Fatalf("missing attempt = %+v", outcome)
	}
	if outcome := scratchOutcome(outcomes, done.Run); outcome.Action != ReconcileScratchRemoved {
		t.Fatalf("all dead, lock free = %+v", outcome)
	}
	if _, err := os.Lstat(done.Root); !os.IsNotExist(err) {
		t.Fatalf("recovered root survived: %v", err)
	}
}

// A3 custodian cases: launcher Dead but custodian alive stays; custodian
// Dead with an empty group removes.
func TestScratchRecoveryWaitsForTheRecordedCustodian(t *testing.T) {
	t.Parallel()
	control := t.TempDir()
	probe := newScratchProbe(t)
	probe.states[900002] = identity.Alive
	record := crashedScratch(t, control, probe, func(r *ScratchRecord) {
		_, ref := probe.ref(t, 900002)
		r.Custodians = []ScratchCustodian{{Ref: ref, Group: 900002}}
	})
	empty := func(int64) ([]int64, error) { return nil, nil }
	if outcome := scratchOutcome(ReconcileScratch(control, ScratchOptions{Prober: probe, GroupMembers: empty}), record.Run); outcome.Action != ReconcileScratchPending ||
		!strings.Contains(outcome.Reason, "custodian") {
		t.Fatalf("live custodian = %+v", outcome)
	}
	delete(probe.states, 900002)
	if outcome := scratchOutcome(ReconcileScratch(control, ScratchOptions{Prober: probe, GroupMembers: empty}), record.Run); outcome.Action != ReconcileScratchRemoved {
		t.Fatalf("dead custodian = %+v", outcome)
	}
}

// A3 lock cases and ROOT correction 1: the launcher closes only its own
// copy; a child holding the inherited description keeps the lock, so normal
// cleanup reports cleanup.incomplete with the record retained, recovery
// reports writer-lock-held, and only the child's exit permits removal.
func TestScratchInheritedWriterLockOutlivesTheLaunchersCopy(t *testing.T) {
	t.Parallel()
	control := t.TempDir()
	run, err := CreateScratchRun(control)
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command("/bin/sh", "-c", "echo ready; read line || true")
	child.ExtraFiles = []*os.File{run.Writer()}
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || line != "ready\n" {
		t.Fatalf("child readiness = %q, %v", line, err)
	}
	err = run.Cleanup(nil)
	if err == nil || !strings.Contains(err.Error(), ScratchIncomplete) || !strings.Contains(err.Error(), "writer-lock-held") {
		t.Fatalf("cleanup with a live inheritor = %v", err)
	}
	if _, err := os.Stat(filepath.Join(ScratchStore(control), run.ID()+".json")); err != nil {
		t.Fatalf("record not retained: %v", err)
	}
	probe := newScratchProbe(t)
	record := readScratchRecord(t, control, run.ID())
	_, record.Launcher = probe.ref(t, 900001)
	writeScratchRecord(t, control, record)
	options := ScratchOptions{Prober: probe, GroupMembers: func(int64) ([]int64, error) { return nil, nil }}
	if outcome := scratchOutcome(ReconcileScratch(control, options), run.ID()); outcome.Action != ReconcileScratchPending || outcome.Reason != "writer-lock-held" {
		t.Fatalf("recovery with a live inheritor = %+v", outcome)
	}
	_ = stdin.Close()
	if err := child.Wait(); err != nil {
		t.Fatal(err)
	}
	if outcome := scratchOutcome(ReconcileScratch(control, options), run.ID()); outcome.Action != ReconcileScratchRemoved {
		t.Fatalf("recovery after the child exited = %+v", outcome)
	}
}

// A8 and the kill-path sentinels: only recorded roots are considered; a live
// peer stays, a symlink or foreign marker refuses, and an unrecorded
// same-prefix directory is never touched.
func TestScratchRecoveryRootShapes(t *testing.T) {
	t.Parallel()
	control := t.TempDir()
	probe := newScratchProbe(t)
	empty := func(int64) ([]int64, error) { return nil, nil }

	absent := crashedScratch(t, control, probe, nil)
	if err := os.RemoveAll(absent.Root); err != nil {
		t.Fatal(err)
	}
	emptyUnmarked := crashedScratch(t, control, probe, nil)
	if err := os.RemoveAll(emptyUnmarked.Root); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(emptyUnmarked.Root, 0o700); err != nil {
		t.Fatal(err)
	}
	unmarked := crashedScratch(t, control, probe, nil)
	if err := os.Remove(filepath.Join(unmarked.Root, scratchMarkerName)); err != nil {
		t.Fatal(err)
	}
	symlinked := crashedScratch(t, control, probe, nil)
	elsewhere := t.TempDir()
	if err := os.RemoveAll(symlinked.Root); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, symlinked.Root); err != nil {
		t.Fatal(err)
	}
	lockless := crashedScratch(t, control, probe, nil)
	if err := os.Remove(filepath.Join(lockless.Root, scratchLockName)); err != nil {
		t.Fatal(err)
	}
	foreign := crashedScratch(t, control, probe, nil)
	if err := os.WriteFile(filepath.Join(foreign.Root, scratchMarkerName), []byte(`{"schema":"metasystem.scratch.v1","run":"other","root":"/x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	peer, err := CreateScratchRun(control)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Cleanup(nil)
	peerRecord := readScratchRecord(t, control, peer.ID())
	probe.states[900003] = identity.Alive
	_, peerRecord.Launcher = probe.ref(t, 900003)
	writeScratchRecord(t, control, peerRecord)
	sentinel := filepath.Join(ScratchStore(control), absent.Run[:10]+"UNRECORDED")
	if err := os.Mkdir(sentinel, 0o700); err != nil {
		t.Fatal(err)
	}

	outcomes := ReconcileScratch(control, ScratchOptions{Prober: probe, GroupMembers: empty})
	want := map[string]string{absent.Run: ReconcileScratchRemoved, emptyUnmarked.Run: ReconcileScratchRemoved,
		unmarked.Run: ReconcileScratchRefused, symlinked.Run: ReconcileScratchRefused, lockless.Run: ReconcileScratchRefused,
		foreign.Run: ReconcileScratchRefused, peer.ID(): ReconcileScratchPending}
	for run, action := range want {
		if outcome := scratchOutcome(outcomes, run); outcome.Action != action {
			t.Errorf("%s = %+v, want %s", run, outcome, action)
		}
	}
	if len(outcomes) != len(want) {
		t.Errorf("outcomes = %+v", outcomes)
	}
	for _, kept := range []string{sentinel, elsewhere, peer.Root(), filepath.Join(unmarked.Root, "groups", "bytes")} {
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("%s touched: %v", kept, err)
		}
	}
	if _, err := os.Lstat(emptyUnmarked.Root); !os.IsNotExist(err) {
		t.Errorf("empty unmarked root survived: %v", err)
	}
	if outcomes := ReconcileScratch(control, ScratchOptions{Prober: probe, GroupMembers: empty, Self: peer.ID()}); scratchOutcome(outcomes, peer.ID()).Action != "" {
		t.Errorf("the caller's own record was reconciled")
	}
}

// Recovery removes recorded worktrees before the root and keeps root and
// record when a registration stays unresolved.
func TestScratchRecoveryKeepsRootWhileAWorktreeIsUnresolved(t *testing.T) {
	t.Parallel()
	control := t.TempDir()
	probe := newScratchProbe(t)
	tuple := ScratchWorktree{Group: "g", Parent: "/p", Top: "/p/worktree-x", Control: "/c", Common: "/c/.git", State: ScratchWorktreeReserved}
	record := crashedScratch(t, control, probe, func(r *ScratchRecord) { r.Worktrees = []ScratchWorktree{tuple} })
	answer := gittree.WorktreeRefused
	var seen []gittree.WorktreeTuple
	options := ScratchOptions{Prober: probe, GroupMembers: func(int64) ([]int64, error) { return nil, nil },
		RemoveWorktree: func(tuple gittree.WorktreeTuple, _ gittree.Workspace) (string, error) {
			seen = append(seen, tuple)
			return answer, nil
		}}
	if outcome := scratchOutcome(ReconcileScratch(control, options), record.Run); outcome.Action != ReconcileScratchRefused {
		t.Fatalf("refused worktree = %+v", outcome)
	}
	if _, err := os.Stat(record.Root); err != nil {
		t.Fatalf("root removed after an unresolved registration: %v", err)
	}
	answer = gittree.WorktreeRemoved
	if outcome := scratchOutcome(ReconcileScratch(control, options), record.Run); outcome.Action != ReconcileScratchRemoved {
		t.Fatalf("resolved worktree = %+v", outcome)
	}
	if len(seen) != 2 || seen[0] != (gittree.WorktreeTuple{Parent: "/p", Top: "/p/worktree-x", Control: "/c", Common: "/c/.git"}) {
		t.Fatalf("recorded tuples = %+v", seen)
	}
}

// ROOT correction 2: the resource custodian inherits the run's writer lock
// and holds it through its drain even after the workload closed its own
// copy; it is recorded before the workload runs, and only its exit lets the
// launcher's cleanup take the lock.
func TestScratchCustodianHoldsTheWriterAfterTheWorkloadClosesIt(t *testing.T) {
	t.Parallel()
	engine := buildResourceCustodyEngine(t)
	control := t.TempDir()
	run, err := CreateScratchRun(control)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ready, release := filepath.Join(dir, "ready"), filepath.Join(dir, "release")
	for _, fifo := range []string{ready, release} {
		if err := syscall.Mkfifo(fifo, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command("/bin/sh", "-c", `exec 3>&-; echo closed > "$1"; read line < "$2"`, "workload", ready, release)
	ctx := WithScratchRun(WithResourceCustodyExecutable(context.Background(), engine), run)
	done := make(chan error, 1)
	go func() { done <- RunResourceCommand(ctx, command, nil) }()
	readyFile, err := os.Open(ready)
	if err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(readyFile).ReadString('\n')
	readyFile.Close()
	if err != nil || line != "closed\n" {
		t.Fatalf("workload readiness = %q, %v", line, err)
	}
	if record := readScratchRecord(t, control, run.ID()); len(record.Custodians) != 1 || record.Custodians[0].Group <= 0 {
		t.Fatalf("custodian not recorded before the workload ran: %+v", record)
	}
	if err := run.Cleanup(nil); err == nil || !strings.Contains(err.Error(), "writer-lock-held") {
		t.Fatalf("cleanup while the custodian holds the writer = %v", err)
	}
	releaseFile, err := os.OpenFile(release, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = releaseFile.WriteString("go\n")
	releaseFile.Close()
	if err := <-done; err != nil {
		t.Fatalf("resource command: %v", err)
	}
	// A second command's custodian is recorded beside the first.
	if err := RunResourceCommand(ctx, exec.Command("/bin/sh", "-c", "exit 0"), nil); err != nil {
		t.Fatal(err)
	}
	if record := readScratchRecord(t, control, run.ID()); len(record.Custodians) != 2 {
		t.Fatalf("custodians = %+v, want both", record.Custodians)
	}
	if err := run.Cleanup(nil); err != nil {
		t.Fatalf("cleanup after the custodian drained: %v", err)
	}
	if _, err := os.Lstat(run.Root()); !os.IsNotExist(err) {
		t.Fatalf("root survived: %v", err)
	}
}

// scratchGitStub answers the read-only plumbing PlanDetachedWorktreeIn asks.
func scratchGitStub(t *testing.T) gittree.Workspace {
	t.Helper()
	control, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	common := filepath.Join(control, ".git")
	if err := os.MkdirAll(filepath.Join(common, "worktrees"), 0o700); err != nil {
		t.Fatal(err)
	}
	return gittree.Workspace{Dir: control, RawSource: func(request gittree.RawRequest) gittree.RawResult {
		args := strings.Join(request.Args, " ")
		switch {
		case strings.HasSuffix(args, "rev-parse --show-toplevel"):
			return gittree.RawResult{Stdout: []byte(control + "\n")}
		case strings.HasSuffix(args, "rev-parse --show-prefix"):
			return gittree.RawResult{Stdout: []byte("\n")}
		case strings.HasSuffix(args, "rev-parse --git-common-dir"):
			return gittree.RawResult{Stdout: []byte(common + "\n")}
		}
		t.Errorf("unexpected git %s", args)
		return gittree.RawResult{ExitCode: 1}
	}}
}

// dupCloseOnExec duplicates file like syscall.Dup but marks the copy
// close-on-exec under ForkLock, so parallel tests' children never inherit it.
func dupCloseOnExec(file *os.File) (int, error) {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	fd, err := syscall.Dup(int(file.Fd()))
	if err == nil {
		syscall.CloseOnExec(fd)
	}
	return fd, err
}

// borrowScratch reopens run as its worker would, through a duplicate of the
// launcher's description (what ExtraFiles hands a child).
func borrowScratch(t *testing.T, control string, run *ScratchRun, attempt string) *ScratchRun {
	t.Helper()
	fd, err := dupCloseOnExec(run.Writer())
	if err != nil {
		t.Fatal(err)
	}
	borrowed, err := OpenScratchRun(control, attempt, *run.Locator(fd))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = borrowed.writer.Close() })
	return borrowed
}

// Launcher and worker handles mutate one durable record concurrently; no
// append is lost, and the launcher's cleanup reloads the record after the
// writers drained and removes the worktree only the worker recorded.
func TestScratchRecordMutationsSerializeAcrossHandlesAndCleanupReloads(t *testing.T) {
	t.Parallel()
	control := t.TempDir()
	run, err := CreateScratchRun(control)
	if err != nil {
		t.Fatal(err)
	}
	if err := run.RecordAttempt("proof-attempt-a"); err != nil {
		t.Fatal(err)
	}
	worker := borrowScratch(t, control, run, "proof-attempt-a")
	probe := newScratchProbe(t)
	const each = 20
	start := make(chan struct{})
	errs := make(chan error, 2*each)
	for index, handle := range []*ScratchRun{run, worker} {
		go func(handle *ScratchRun, base int64) {
			<-start
			for step := int64(0); step < each; step++ {
				ref, _ := probe.ref(t, base+step)
				errs <- handle.RecordCustodian(ref, base+step)
			}
		}(handle, int64(910000+index*1000))
	}
	close(start)
	for range 2 * each {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if record := readScratchRecord(t, control, run.ID()); len(record.Custodians) != 2*each || record.Attempt != "proof-attempt-a" {
		t.Fatalf("durable record lost a mutation: %d custodians, attempt %q", len(record.Custodians), record.Attempt)
	}
	plan, err := worker.PlanWorktree(scratchGitStub(t), "groups")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(plan.Parent, run.Dir("groups")+string(os.PathSeparator)) {
		t.Fatalf("worker worktree %s is outside the run root", plan.Parent)
	}
	if stale, _ := filepath.Glob(filepath.Join(ScratchStore(control), "*.tmp-*")); len(stale) != 0 {
		t.Fatalf("temporary record files left: %v", stale)
	}
	var removed []string
	if err := run.Cleanup(nil); err == nil || !strings.Contains(err.Error(), "writer-lock-held") {
		t.Fatalf("cleanup while the worker holds its inherited writer = %v", err)
	}
	_ = worker.writer.Close() // the worker exits
	err = run.Cleanup(func(tuple gittree.WorktreeTuple, _ gittree.Workspace) (string, error) {
		removed = append(removed, tuple.Top)
		return gittree.WorktreeRemoved, os.RemoveAll(tuple.Parent)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0] != plan.Top {
		t.Fatalf("cleanup removed %v, want the worker's %s", removed, plan.Top)
	}
	if entries, _ := os.ReadDir(ScratchStore(control)); len(entries) != 0 {
		t.Fatalf("store not empty: %v", entries)
	}
}

// The worker accepts only the launcher's own inherited description of this
// run's writer lock, for the authenticated attempt, in this store.
func TestOpenScratchRunAuthenticatesTheInheritedWriter(t *testing.T) {
	t.Parallel()
	control := t.TempDir()
	run, err := CreateScratchRun(control)
	if err != nil {
		t.Fatal(err)
	}
	defer run.Cleanup(nil)
	if err := run.RecordAttempt("proof-attempt-a"); err != nil {
		t.Fatal(err)
	}
	shared, err := dupCloseOnExec(run.Writer())
	if err != nil {
		t.Fatal(err)
	}
	// The accepted borrowed run takes ownership of shared through its writer;
	// closing the number here too would let that writer's finalizer later
	// close whatever descriptor reused it.
	var borrowed *ScratchRun
	defer func() {
		if borrowed != nil {
			_ = borrowed.writer.Close()
		} else {
			_ = syscall.Close(shared)
		}
	}()
	independent, err := os.OpenFile(filepath.Join(run.Root(), ".writer-lock"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer independent.Close()
	other, err := os.Open(filepath.Join(run.Root(), "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	locator := *run.Locator(shared)
	cases := map[string]func() (string, ScratchLocator){
		"independent description": func() (string, ScratchLocator) {
			l := locator
			l.FD = int(independent.Fd())
			return "proof-attempt-a", l
		},
		"another file":           func() (string, ScratchLocator) { l := locator; l.FD = int(other.Fd()); return "proof-attempt-a", l },
		"another attempt":        func() (string, ScratchLocator) { return "proof-attempt-b", locator },
		"root outside the store": func() (string, ScratchLocator) { l := locator; l.Root = t.TempDir(); return "proof-attempt-a", l },
		"unknown run":            func() (string, ScratchLocator) { l := locator; l.Run = "scratch-x"; return "proof-attempt-a", l },
	}
	for name, build := range cases {
		attempt, bad := build()
		if borrowed, err := OpenScratchRun(control, attempt, bad); err == nil || borrowed != nil {
			t.Errorf("%s was accepted", name)
		}
	}
	borrowed, err = OpenScratchRun(control, "proof-attempt-a", locator)
	if err != nil {
		t.Fatal(err)
	}
	if err := borrowed.Cleanup(nil); err == nil {
		t.Fatal("a borrowed run removed its launcher's root")
	}
	if _, err := os.Stat(run.Root()); err != nil {
		t.Fatalf("root after the borrowed cleanup attempt: %v", err)
	}
}

const scratchWorkerChildEnv = "GO_WANT_SCRATCH_WORKER_CHILD"

// A real worker process receives the writer through ExtraFiles, authenticates
// it from its packet locator, and appends to the durable record while the
// launcher appends too; the launcher's cleanup sees the worker's tuple.
func TestScratchWorkerProcessAppendsThroughTheInheritedWriter(t *testing.T) {
	t.Parallel()
	if spec := os.Getenv(scratchWorkerChildEnv); spec != "" {
		scratchWorkerChild(t, spec)
		return
	}
	control := t.TempDir()
	run, err := CreateScratchRun(control)
	if err != nil {
		t.Fatal(err)
	}
	if err := run.RecordAttempt("proof-attempt-a"); err != nil {
		t.Fatal(err)
	}
	// One unrelated inherited file first: the locator must name fd 4, not 3.
	unrelated, err := os.Open(filepath.Join(run.Root(), "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	defer unrelated.Close()
	locator, _ := json.Marshal(run.Locator(ScratchWriterFD([]*os.File{unrelated})))
	child := exec.Command(os.Args[0], "-test.run=^TestScratchWorkerProcessAppendsThroughTheInheritedWriter$", "-test.count=1")
	child.Env = append(os.Environ(), scratchWorkerChildEnv+"="+control+"\n"+string(locator))
	child.ExtraFiles = []*os.File{unrelated, run.Writer()}
	output, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	child.Stderr = child.Stdout
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(output)
	if line, err := reader.ReadString('\n'); err != nil || line != "opened\n" {
		t.Fatalf("worker readiness = %q, %v", line, err)
	}
	probe := newScratchProbe(t)
	for step := int64(0); step < 20; step++ {
		ref, _ := probe.ref(t, 920000+step)
		if err := run.RecordCustodian(ref, 920000+step); err != nil {
			t.Fatal(err)
		}
	}
	rest, _ := io.ReadAll(reader)
	if err := child.Wait(); err != nil {
		t.Fatalf("worker: %v\n%s", err, rest)
	}
	record := readScratchRecord(t, control, run.ID())
	if len(record.Custodians) != 40 || len(record.Worktrees) != 1 || record.Worktrees[0].State != ScratchWorktreeReserved {
		t.Fatalf("durable record after both processes: %d custodians, worktrees %+v\n%s", len(record.Custodians), record.Worktrees, rest)
	}
	var removed []string
	if err := run.Cleanup(func(tuple gittree.WorktreeTuple, _ gittree.Workspace) (string, error) {
		removed = append(removed, tuple.Top)
		return gittree.WorktreeRemoved, os.RemoveAll(tuple.Parent)
	}); err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0] != record.Worktrees[0].Top {
		t.Fatalf("cleanup removed %v", removed)
	}
}

func scratchWorkerChild(t *testing.T, spec string) {
	control, encoded, _ := strings.Cut(spec, "\n")
	var locator ScratchLocator
	if err := json.Unmarshal([]byte(encoded), &locator); err != nil || locator.FD != 4 {
		t.Fatalf("locator %s: %v", encoded, err)
	}
	worker, err := OpenScratchRun(control, "proof-attempt-a", locator)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("opened")
	probe := newScratchProbe(t)
	for step := int64(0); step < 20; step++ {
		ref, _ := probe.ref(t, 930000+step)
		if err := worker.RecordCustodian(ref, 930000+step); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := worker.PlanWorktree(scratchGitStub(t), "groups"); err != nil {
		t.Fatal(err)
	}
}

// Identity getters run concurrently with whole-record mutations from the
// same handle; under -race this proves the names are read without a race
// and never change.
func TestScratchRunGettersRaceWithMutations(t *testing.T) {
	t.Parallel()
	control := t.TempDir()
	run, err := CreateScratchRun(control)
	if err != nil {
		t.Fatal(err)
	}
	defer run.Cleanup(nil)
	id, root := run.ID(), run.Root()
	start := make(chan struct{})
	done := make(chan error, 6)
	for range 4 {
		go func() {
			<-start
			for range 200 {
				locator := run.Locator(3)
				if run.ID() != id || run.Root() != root || run.Dir("groups") != filepath.Join(root, "groups") ||
					locator.Run != id || locator.Root != root || run.Writer() == nil || run.Materialization().HooksPath != filepath.Join(root, "no-hooks") {
					done <- fmt.Errorf("identity changed under mutation")
					return
				}
			}
			done <- nil
		}()
	}
	for worker := range 2 {
		go func() {
			<-start
			for step := range 50 {
				if err := run.RecordAttempt(fmt.Sprintf("proof-attempt-%d-%d", worker, step)); err != nil {
					done <- err
					return
				}
			}
			done <- nil
		}()
	}
	close(start)
	for range 6 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}

// C2: a store symlinked outside the control root is refused before any byte
// is written or removed there; a control-root alias still works.
func TestScratchStoreMustResolveInsideTheControlRoot(t *testing.T) {
	t.Parallel()
	control := t.TempDir()
	external := t.TempDir()
	sentinel := filepath.Join(external, "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	probe := newScratchProbe(t)
	elsewhere := t.TempDir()
	foreign := crashedScratch(t, elsewhere, probe, nil)
	for _, name := range []string{foreign.Run + ".json", foreign.Run} {
		if err := os.Rename(filepath.Join(ScratchStore(elsewhere), name), filepath.Join(external, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(ScratchStore(control)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, ScratchStore(control)); err != nil {
		t.Fatal(err)
	}
	if run, err := CreateScratchRun(control); err == nil || run != nil {
		t.Fatal("a scratch run was created through an external store symlink")
	}
	outcomes := ReconcileScratch(control, ScratchOptions{Prober: probe, GroupMembers: func(int64) ([]int64, error) { return nil, nil }})
	if len(outcomes) != 1 || outcomes[0].Action != ReconcileScratchRefused {
		t.Fatalf("reconcile through an external store = %+v", outcomes)
	}
	entries, _ := os.ReadDir(external)
	if len(entries) != 3 {
		t.Fatalf("external store changed: %v", entries)
	}
	for _, kept := range []string{sentinel, filepath.Join(external, foreign.Run+".json"), filepath.Join(external, foreign.Run, "groups", "bytes")} {
		if _, err := os.Stat(kept); err != nil {
			t.Fatalf("%s: %v", kept, err)
		}
	}
	real := t.TempDir()
	alias := filepath.Join(t.TempDir(), "control-alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	run, err := CreateScratchRun(alias)
	if err != nil {
		t.Fatalf("control-root alias refused: %v", err)
	}
	if err := run.Cleanup(nil); err != nil {
		t.Fatal(err)
	}
}

// C4: a failed sidecar deletion keeps the record discoverable, and the next
// reconcile pass retries; a record lock left after its record went is
// recovered by its exact run name. Nothing is lost or swept by prefix.
func TestScratchRecordDeletionFailuresAreRetried(t *testing.T) {
	t.Parallel()
	control := t.TempDir()
	probe := newScratchProbe(t)
	record := crashedScratch(t, control, probe, nil)
	store, err := filepath.EvalSymlinks(ScratchStore(control))
	if err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(store, record.Run+".json")
	temporary := recordPath + ".tmp-123"
	if err := os.WriteFile(temporary, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(store, "scratch-unrelated.record-lock")
	if err := os.WriteFile(unrelated, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	failing := func(target string) func(string) error {
		return func(path string) error {
			if path == target {
				return fmt.Errorf("injected removal failure")
			}
			return os.Remove(path)
		}
	}
	options := ScratchOptions{Prober: probe, GroupMembers: func(int64) ([]int64, error) { return nil, nil }, RemoveFile: failing(temporary)}
	if outcome := scratchOutcome(ReconcileScratch(control, options), record.Run); outcome.Action != ReconcileScratchPending {
		t.Fatalf("failed temporary deletion = %+v", outcome)
	}
	if _, err := os.Stat(recordPath); err != nil {
		t.Fatalf("record lost after a failed sidecar deletion: %v", err)
	}
	options.RemoveFile = failing(scratchRecordLockPath(recordPath))
	if outcome := scratchOutcome(ReconcileScratch(control, options), record.Run); outcome.Action != ReconcileScratchPending {
		t.Fatalf("failed lock deletion = %+v", outcome)
	}
	if _, err := os.Stat(scratchRecordLockPath(recordPath)); err != nil {
		t.Fatalf("record lock: %v", err)
	}
	options.RemoveFile = nil
	if outcome := scratchOutcome(ReconcileScratch(control, options), record.Run); outcome.Action != ReconcileScratchRemoved {
		t.Fatalf("retry = %+v", outcome)
	}
	entries, _ := os.ReadDir(store)
	if len(entries) != 1 || entries[0].Name() != filepath.Base(unrelated) {
		t.Fatalf("store after retry = %v", entries)
	}
}

// When another cleanup removed the record before this pass took
// the record lock, a failed sidecar deletion is reported pending, and the
// next pass retries it.
func TestScratchMissingRecordBranchReportsSidecarFailures(t *testing.T) {
	t.Parallel()
	store := t.TempDir()
	run := "scratch-mugzzzzz-0123456789abcdef"
	recordPath := filepath.Join(store, run+".json")
	temporary := recordPath + ".tmp-7"
	if err := os.WriteFile(temporary, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	failing := ScratchOptions{RemoveFile: func(path string) error {
		if path == temporary {
			return fmt.Errorf("injected removal failure")
		}
		return os.Remove(path)
	}}
	if action, reason := reconcileScratchOne(t.TempDir(), store, run, failing); action != ReconcileScratchPending || !strings.Contains(reason, "injected") {
		t.Fatalf("missing record with a failed sidecar deletion = %s: %s", action, reason)
	}
	if _, err := os.Stat(temporary); err != nil {
		t.Fatalf("sidecar: %v", err)
	}
	if action, reason := reconcileScratchOne(t.TempDir(), store, run, ScratchOptions{}); action != ReconcileScratchRemoved {
		t.Fatalf("retry = %s: %s", action, reason)
	}
	if entries, _ := os.ReadDir(store); len(entries) != 0 {
		t.Fatalf("store after retry = %v", entries)
	}
}
