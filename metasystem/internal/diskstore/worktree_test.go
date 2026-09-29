package diskstore

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// linkedBed is a real repository with a checkout registry: goal and session
// worktrees are git's own linked worktrees, which no stub can prove.
type linkedBed struct {
	t        *testing.T
	repo     string
	control  string
	registry Registry
}

func newLinkedBed(t *testing.T) *linkedBed {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := realDir(t)
	bed := &linkedBed{t: t, repo: filepath.Join(root, "repo")}
	bed.control = filepath.Join(bed.repo, "metasystem")
	bed.registry = CheckoutRegistry(bed.control)
	if err := os.MkdirAll(bed.repo, 0o700); err != nil {
		t.Fatal(err)
	}
	bed.must(bed.repo, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(bed.repo, ".gitignore"), []byte("metasystem/artifacts/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bed.must(bed.repo, "add", ".gitignore")
	bed.must(bed.repo, "commit", "-q", "-m", "base")
	return bed
}

func (b *linkedBed) must(dir string, args ...string) string {
	b.t.Helper()
	out, err := realWorkspaceGit(context.Background(), dir, args...)
	if err != nil {
		b.t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// goalWorktree registers and makes goal/<id>'s worktree beside the repo,
// the way work build does: reserved, made by git, accepted.
func (b *linkedBed) goalWorktree(id string) Record {
	b.t.Helper()
	path := filepath.Join(filepath.Dir(b.repo), "repo-"+id)
	record, err := ReserveLinkedWorktree(b.registry, path, GoalWorktreeClass, Owner{Kind: OwnerGoal, Ref: id}, b.control, "", testNow, rand.Reader)
	if err != nil {
		b.t.Fatal(err)
	}
	b.must(b.repo, "worktree", "add", "-q", "-b", "goal/"+id, path, "HEAD")
	accepted, err := AcceptLinkedWorktree(b.registry, record.ID)
	if err != nil {
		b.t.Fatal(err)
	}
	return accepted
}

// sweep runs one checkout pass with the goal-worktree proof and census.
func (b *linkedBed) sweep(proof GoalWorktreeProof, census CensusReader) Report {
	b.t.Helper()
	report, err := RunPass(context.Background(), PassOptions{Kind: "checkout", Name: b.control, Registry: b.registry, Mode: ModeApply,
		Now: testNow, Clock: func() time.Time { return testNow },
		Classes: []Class{RegisteredStores{Registry: b.registry, Proofs: map[OwnerKind]OwnerProof{
			OwnerGoal: ClassProofs{Owner: OwnerGoal, ByClass: map[string]OwnerProof{GoalWorktreeClass: proof}}}}},
		CensusReader: &census})
	if err != nil {
		b.t.Fatal(err)
	}
	return report
}

// goalSweep is goal done's sweep as far as the worktree goes: git removes
// the worktree and the branch.
func (b *linkedBed) goalSweep(ctx context.Context, goalID string) error {
	path := filepath.Join(filepath.Dir(b.repo), "repo-"+goalID)
	if _, err := realWorkspaceGit(ctx, b.repo, "worktree", "remove", path); err != nil {
		return err
	}
	_, err := realWorkspaceGit(ctx, b.repo, "branch", "-D", "goal/"+goalID)
	return err
}

// censusOf is a fake use census over the given processes, every one
// readable.
func censusOf(processes ...CensusProcess) CensusReader {
	byPid := map[int64]CensusProcess{}
	var pids []int64
	for _, process := range processes {
		byPid[process.Pid], pids = process, append(pids, process.Pid)
	}
	return CensusReader{
		Pids:       func() ([]int64, error) { return pids, nil },
		ProcessUID: func(pid int64) (uint32, bool) { _, ok := byPid[pid]; return 501, ok },
		Use: func(pid int64) (identity.ProcessUse, error) {
			process := byPid[pid]
			return identity.ProcessUse{Cwd: process.Cwd, Executable: process.Executable, Files: process.Files}, nil
		},
		Command: func(pid int64) string { return "fixture" },
	}
}

func keptLine(report Report, path, reason string) bool {
	for _, line := range append(append([]Line(nil), report.Kept...), report.Pending...) {
		if line.Path == path && strings.Contains(line.Reason, reason) {
			return true
		}
	}
	return false
}

// Registration writes nothing inside the tree (DL2-10): the worktree's
// status, the checkout's status and the tree identity are unchanged.
func TestGoalWorktreeRegistrationLeavesTheTreeClean(t *testing.T) {
	t.Parallel()
	bed := newLinkedBed(t)
	tree := bed.must(bed.repo, "rev-parse", "HEAD^{tree}")
	record := bed.goalWorktree("g")
	if record.State != StateAccepted || record.Identity.Gitdir == "" || record.Layout != LayoutCopy {
		t.Fatalf("record = %+v", record)
	}
	for _, dir := range []string{record.Path, bed.repo} {
		if status := bed.must(dir, "status", "--porcelain=v1", "--untracked-files=all"); status != "" {
			t.Fatalf("registration dirtied %s: %q", dir, status)
		}
	}
	if again := bed.must(record.Path, "rev-parse", "HEAD^{tree}"); again != tree {
		t.Fatalf("the candidate tree changed: %s, was %s", again, tree)
	}
	// A repeat is the same record (R-129).
	if repeat, err := ReserveLinkedWorktree(bed.registry, record.Path, GoalWorktreeClass, record.Owner, bed.control, "", testNow, rand.Reader); err != nil || repeat.ID != record.ID {
		t.Fatalf("repeat = %+v, %v", repeat, err)
	}
}

// A concluded goal's worktree is kept while anything keeps it (U5e): an open
// goal, uncommitted work, unlanded commits, a live process whose cwd is
// inside, one with only an open file inside, an incomplete census; a clean,
// landed, unused worktree goes with its tips archived first.
func TestSweeperReleasesAConcludedGoalsWorktreeOnlyWhenNothingKeepsIt(t *testing.T) {
	t.Parallel()
	bed := newLinkedBed(t)
	record := bed.goalWorktree("g")
	tip := bed.must(record.Path, "rev-parse", "HEAD")
	ended, refusal := false, ""
	proof := GoalWorktreeProof{GitRoot: bed.repo, Git: realWorkspaceGit, Now: testNow,
		Ended: func(Owner) (bool, bool, string) { return ended, true, "the ledger at abc" },
		Plan:  func(context.Context, string) (string, error) { return refusal, nil },
		Sweep: bed.goalSweep}
	clear := censusOf()

	if report := bed.sweep(proof, clear); !keptLine(report, record.Path, "is open") {
		t.Fatalf("an open goal's worktree is kept: %+v", report)
	}
	ended = true
	dirty := filepath.Join(record.Path, "draft.txt")
	if err := os.WriteFile(dirty, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if report := bed.sweep(proof, clear); !keptLine(report, record.Path, "draft.txt") {
		t.Fatalf("uncommitted work keeps the worktree: %+v", report)
	}
	if err := os.Remove(dirty); err != nil {
		t.Fatal(err)
	}
	refusal = "goal/g at local tip x has unlanded commits: y"
	if report := bed.sweep(proof, clear); !keptLine(report, record.Path, "unlanded commits") {
		t.Fatalf("unlanded commits keep the worktree: %+v", report)
	}
	refusal = ""
	inside := bed.sweep(proof, censusOf(CensusProcess{Pid: 4242, Cwd: filepath.Join(record.Path, "sub")}))
	if !keptLine(inside, record.Path, "pid 4242") {
		t.Fatalf("a process whose cwd is inside keeps the worktree: %+v", inside)
	}
	open := bed.sweep(proof, censusOf(CensusProcess{Pid: 4343, Cwd: bed.repo, Files: []string{filepath.Join(record.Path, ".gitignore")}}))
	if !keptLine(open, record.Path, "pid 4343") {
		t.Fatalf("a process with an open file inside keeps the worktree: %+v", open)
	}
	gap := clear
	gap.Pids = func() ([]int64, error) { return []int64{5151}, nil }
	gap.ProcessUID = func(int64) (uint32, bool) { return 501, true }
	gap.Use = func(int64) (identity.ProcessUse, error) {
		return identity.ProcessUse{}, errors.New("descriptors unreadable")
	}
	if report := bed.sweep(proof, gap); len(report.Pending) != 1 || !strings.Contains(report.Pending[0].Command, "--release "+record.ID) {
		t.Fatalf("an incomplete census keeps the worktree and names --release: %+v", report)
	}
	if loaded, _ := bed.registry.Load(record.ID); loaded.State != StateAccepted {
		t.Fatalf("a kept worktree stays accepted: %+v", loaded)
	}

	report := bed.sweep(proof, clear)
	if len(report.Actions) != 1 {
		t.Fatalf("a concluded, clean, landed, unused worktree goes: %+v", report)
	}
	if _, err := os.Stat(record.Path); !os.IsNotExist(err) {
		t.Fatalf("the worktree survived: %v", err)
	}
	if archived := bed.must(bed.repo, "rev-parse", "refs/archive/goal-g/goal/g"); archived != tip {
		t.Fatalf("the tip is archived before removal: %s, want %s", archived, tip)
	}
	if loaded, _ := bed.registry.Load(record.ID); loaded.State != StateReleased {
		t.Fatalf("record = %+v", loaded)
	}
	if again := bed.sweep(proof, clear); len(again.Actions) != 0 {
		t.Fatalf("a repeat releases nothing: %+v", again)
	}
}

// An engine verb inside the worktree and the sweeper never both proceed
// (DL3B-01): with the entrant's shared hold the sweeper's item is pending;
// once the release has begun, an entrant finds the store gone.
func TestAPausedEntrantAndTheSweeperNeverBothProceed(t *testing.T) {
	t.Parallel()
	bed := newLinkedBed(t)
	record := bed.goalWorktree("g")
	proof := GoalWorktreeProof{GitRoot: bed.repo, Git: realWorkspaceGit, Now: testNow,
		Ended: func(Owner) (bool, bool, string) { return true, true, "the ledger at abc" },
		Plan:  func(context.Context, string) (string, error) { return "", nil },
		Sweep: bed.goalSweep}
	entrant, err := bed.registry.Enter(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if report := bed.sweep(proof, censusOf()); len(report.Actions) != 0 || !keptLine(report, record.Path, "record lock is held") {
		t.Fatalf("an entrant's hold makes the item pending: %+v", report)
	}
	if err := entrant.Leave(); err != nil {
		t.Fatal(err)
	}
	// A release cut short after it wrote releasing: an entrant that
	// arrives now reads the store gone and refuses; the sweeper finishes.
	critical, err := bed.registry.TryCritical(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	releasing := critical.Record()
	releasing.State = StateReleasing
	if err := critical.Write(releasing); err != nil {
		t.Fatal(err)
	}
	_ = critical.Release()
	if _, err := bed.registry.Enter(record.ID); !errors.Is(err, ErrStoreGone) {
		t.Fatalf("an entrant during a release = %v, want the store gone", err)
	}
	if report := bed.sweep(proof, censusOf()); len(report.Actions) != 1 {
		t.Fatalf("the next pass finishes the release: %+v", report)
	}
	if _, err := bed.registry.Enter(record.ID); !errors.Is(err, ErrStoreGone) {
		t.Fatalf("an entrant after the release = %v, want the store gone", err)
	}
}

// goal done's own sweep enters the same critical section (3.2 "Goal"): a
// held record lock runs no removal; a sweep that refuses leaves the record
// accepted for the next attempt; a completed one releases it.
func TestGoalDoneSweepRunsInsideTheCriticalSection(t *testing.T) {
	t.Parallel()
	bed := newLinkedBed(t)
	record := bed.goalWorktree("g")
	ran := 0
	request := LinkedRelease{GitRoot: bed.repo, Git: realWorkspaceGit, Census: &UseCensus{Taken: true}, Now: testNow, By: "goal done",
		Remove: func(context.Context) error { ran++; return errors.New("goal/g at local tip x has unlanded commits: y") }}
	entrant, err := bed.registry.Enter(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := ReleaseLinkedWorktrees(context.Background(), bed.registry, []string{record.ID}, request); err != nil || !outcome.Pending || ran != 0 {
		t.Fatalf("a held lock = %+v, %v, removals %d", outcome, err, ran)
	}
	_ = entrant.Leave()
	if _, err := ReleaseLinkedWorktrees(context.Background(), bed.registry, []string{record.ID}, request); err == nil || ran != 1 {
		t.Fatalf("a refused sweep is reported: %v, removals %d", err, ran)
	}
	if loaded, _ := bed.registry.Load(record.ID); loaded.State != StateAccepted {
		t.Fatalf("a refused sweep leaves the record accepted: %+v", loaded)
	}
	request.Remove = func(ctx context.Context) error { return bed.goalSweep(ctx, "g") }
	if outcome, err := ReleaseLinkedWorktrees(context.Background(), bed.registry, []string{record.ID}, request); err != nil || !outcome.Done {
		t.Fatalf("a completed sweep = %+v, %v", outcome, err)
	}
	if loaded, _ := bed.registry.Load(record.ID); loaded.State != StateReleased || loaded.ReleasedBy != "goal done" {
		t.Fatalf("record = %+v", loaded)
	}
}

// sessionBed makes a second session's worktree the way session isolate
// does: reserved with its bootstrap, made by git, its identity recorded.
func (b *linkedBed) sessionWorktree(name string) Record {
	b.t.Helper()
	path := filepath.Join(filepath.Dir(b.repo), name)
	record, err := ReserveLinkedWorktree(b.registry, path, SessionWorktreeClass, Owner{Kind: OwnerSession, Ref: name}, b.control, BootstrapRef(1, 1), testNow, rand.Reader)
	if err != nil {
		b.t.Fatal(err)
	}
	b.must(b.repo, "worktree", "add", "-q", "-b", "session/"+name, path, "HEAD")
	identified, err := b.registry.IdentifyLinkedWorktree(record.ID)
	if err != nil {
		b.t.Fatal(err)
	}
	return identified
}

func (b *linkedBed) sessionSweep(proof SessionWorktreeProof, census CensusReader, now time.Time) Report {
	b.t.Helper()
	proof.Now = now
	report, err := RunPass(context.Background(), PassOptions{Kind: "checkout", Name: b.control, Registry: b.registry, Mode: ModeApply,
		Now: now, Clock: func() time.Time { return now },
		Classes: []Class{RegisteredStores{Registry: b.registry, Proofs: map[OwnerKind]OwnerProof{
			OwnerSession: ClassProofs{Owner: OwnerSession, ByClass: map[string]OwnerProof{SessionWorktreeClass: proof}}}}},
		CensusReader: &census})
	if err != nil {
		b.t.Fatal(err)
	}
	return report
}

// The session state machine (3.2 "Seat", U5e): a bootstrap that exits
// before a main announces itself leaves the record reserved and reported
// after the grace, never released; an announced live main keeps it; a dead
// main with a surviving delegate in the worktree keeps it; both ended, clean
// and unused, it goes with its branch, every tip archived first.
func TestSessionWorktreeStateMachine(t *testing.T) {
	t.Parallel()
	bed := newLinkedBed(t)
	record := bed.sessionWorktree("side")
	if record.State != StateReserved || record.Identity.Gitdir == "" || record.Bootstrap == "" {
		t.Fatalf("record = %+v", record)
	}
	tip := bed.must(record.Path, "rev-parse", "HEAD")
	main := MainNone
	proof := SessionWorktreeProof{GitRoot: bed.repo, Git: realWorkspaceGit, Grace: 24 * time.Hour,
		Main:          func(string) (MainLiveness, string) { return main, "fixture" },
		BootstrapDead: func(string) (bool, bool) { return true, true }}
	if report := bed.sessionSweep(proof, censusOf(), testNow.Add(time.Hour)); !keptLine(report, record.Path, "is starting") {
		t.Fatalf("within the grace the session is starting: %+v", report)
	}
	later := testNow.Add(25 * time.Hour)
	if report := bed.sessionSweep(proof, censusOf(), later); !keptLine(report, record.Path, "never released") || len(report.Actions) != 0 {
		t.Fatalf("a bootstrap that died with no main is reported, not released: %+v", report)
	}
	main = MainAlive
	if report := bed.sessionSweep(proof, censusOf(), later); !keptLine(report, record.Path, "main is alive") {
		t.Fatalf("a live main keeps the worktree: %+v", report)
	}
	main = MainDead
	delegate := censusOf(CensusProcess{Pid: 777, Cwd: record.Path})
	if report := bed.sessionSweep(proof, delegate, later); !keptLine(report, record.Path, "pid 777") {
		t.Fatalf("a surviving delegate in the worktree keeps it: %+v", report)
	}
	report := bed.sessionSweep(proof, censusOf(), later)
	if len(report.Actions) != 1 {
		t.Fatalf("both ended, clean and unused, it goes: %+v", report)
	}
	if _, err := os.Stat(record.Path); !os.IsNotExist(err) {
		t.Fatalf("the worktree survived: %v", err)
	}
	if _, err := realWorkspaceGit(context.Background(), bed.repo, "rev-parse", "--verify", "-q", "refs/heads/session/side"); err == nil {
		t.Fatal("the session branch survived")
	}
	if archived := bed.must(bed.repo, "rev-parse", "refs/archive/session-side/session/side"); archived != tip {
		t.Fatalf("the tip is archived first: %s, want %s", archived, tip)
	}
	if loaded, _ := bed.registry.Load(record.ID); loaded.State != StateReleased || !strings.Contains(strings.Join(loaded.Notes, " "), "accepted by the sweeper") {
		t.Fatalf("the announced main's acceptance is recorded in the release: %+v", loaded)
	}
}

// A failed creation leaves no reservation that pends for ever, and never
// removes what exists.
func TestAnAbandonedReservationBecomesHistoryOnlyWhenNothingExists(t *testing.T) {
	t.Parallel()
	bed := newLinkedBed(t)
	path := filepath.Join(filepath.Dir(bed.repo), "repo-x")
	record, err := ReserveLinkedWorktree(bed.registry, path, GoalWorktreeClass, Owner{Kind: OwnerGoal, Ref: "x"}, bed.control, "", testNow, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := AbandonLinkedWorktree(bed.registry, record.ID, "git failed"); err == nil {
		t.Fatal("a reservation whose path exists was abandoned")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := AbandonLinkedWorktree(bed.registry, record.ID, "git failed"); err != nil {
		t.Fatal(err)
	}
	if loaded, _ := bed.registry.Load(record.ID); loaded.State != StateReleased {
		t.Fatalf("record = %+v", loaded)
	}
}

// A person's --release of a store with its own release sequence (DL3B-12):
// the person's word stands in for the processes the census could not read,
// and nothing else: a readable holder still keeps it; the release runs the
// store's own sequence (tips archived first) and records the person.
func TestAPersonReleasesAGoalWorktreeThatOnlyAnIncompleteCensusKept(t *testing.T) {
	t.Parallel()
	bed := newLinkedBed(t)
	record := bed.goalWorktree("g")
	proof := ClassProofs{Owner: OwnerGoal, ByClass: map[string]OwnerProof{GoalWorktreeClass: GoalWorktreeProof{GitRoot: bed.repo, Git: realWorkspaceGit, Now: testNow,
		Ended: func(Owner) (bool, bool, string) { return true, true, "the ledger at abc" },
		Plan:  func(context.Context, string) (string, error) { return "", nil },
		Sweep: bed.goalSweep}}}
	held := &UseCensus{Taken: true, Processes: []CensusProcess{{Pid: 5151, UID: 501, Command: "vim", Cwd: record.Path}},
		Unreadable: []CensusGap{{Pid: 7, Reason: "unreadable"}}}
	if verdict, err := ReleaseByPerson(context.Background(), bed.registry, record.ID, proof, held, "Wido"); err != nil || verdict.Decision == Release || !strings.Contains(verdict.Reason, "pid 5151") {
		t.Fatalf("a readable holder = %+v, %v", verdict, err)
	}
	gap := &UseCensus{Taken: true, Unreadable: []CensusGap{{Pid: 7, Reason: "unreadable"}}}
	verdict, err := ReleaseByPerson(context.Background(), bed.registry, record.ID, proof, gap, "Wido")
	if err != nil || verdict.Decision != Release {
		t.Fatalf("an incomplete census alone = %+v, %v", verdict, err)
	}
	if _, err := os.Stat(record.Path); !os.IsNotExist(err) {
		t.Fatalf("the worktree survived: %v", err)
	}
	if loaded, _ := bed.registry.Load(record.ID); loaded.State != StateReleased || loaded.ReleasedBy != "person Wido" {
		t.Fatalf("record = %+v", loaded)
	}
	if verdict, err := ReleaseByPerson(context.Background(), bed.registry, record.ID, proof, gap, "Wido"); err != nil || verdict.Reason != "already released" {
		t.Fatalf("a repeat = %+v, %v", verdict, err)
	}
}
