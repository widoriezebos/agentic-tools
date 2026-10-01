package lane

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// prePushChild is this test binary run as the lane installation's engine by
// the lane checkout's real pre-push hook: the hook execs
// <install>/bin/metasystem internal pre-push …, which the bed links to this
// binary.
func prePushChild() (int, bool) {
	// internal pre-push --home HOME REMOTE URL
	if len(os.Args) == 7 && os.Args[1] == "internal" && os.Args[2] == "pre-push" && os.Args[3] == "--home" {
		return RunPrePush(os.Args[4], os.Args[6], os.Stdin, os.Stderr), true
	}
	return 0, false
}

// publishBed is a real lane: a bare file origin, a nested lane checkout
// (Git toplevel != installation) registered by landing set with its real
// pre-push hook, the lane engine enrolled, and a seat clone of the same
// origin.
type publishBed struct {
	home, origin, checkout, install, seat string
}

func newPublishBed(t *testing.T) *publishBed {
	t.Helper()
	base := resolved(t.TempDir())
	bed := &publishBed{home: filepath.Join(base, "home"), origin: filepath.Join(base, "origin.git"),
		checkout: filepath.Join(base, "lane"), seat: filepath.Join(base, "seat")}
	bed.install = filepath.Join(bed.checkout, "metasystem")
	seed := filepath.Join(base, "seed")
	bed.git(t, base, "init", "--quiet", "--bare", "-b", "main", bed.origin)
	bed.git(t, base, "clone", "--quiet", bed.origin, seed)
	for name, content := range map[string]string{"README.md": "seed\n", "metasystem/metasystem.conf": "metasystem.template=true\n", ".gitignore": "metasystem/bin/\nmetasystem/artifacts/\n"} {
		writeFile(t, filepath.Join(seed, name), content)
	}
	bed.git(t, seed, "add", "-A")
	bed.git(t, seed, "commit", "--quiet", "-m", "seed")
	bed.git(t, seed, "push", "--quiet", "origin", "main")
	bed.git(t, base, "clone", "--quiet", bed.origin, bed.checkout)
	bed.git(t, base, "clone", "--quiet", bed.origin, bed.seat)
	engine := filepath.Join(bed.install, "bin", "metasystem")
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(engine), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(binary, engine); err != nil {
		t.Fatal(err)
	}
	enrolled := steward.InstallIdentity{RepoIdentity: bed.install, Generation: 3, InstallPath: engine,
		InstallDigest: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("engine"))), MintedAt: "2026-10-01T08:00:00Z",
		Enrollment: steward.EnrollmentHumanTerminal}
	if err := steward.MintIdentity(steward.RepoIdentityPath(bed.install), enrolled); err != nil {
		t.Fatal(err)
	}
	register(t, bed.home, bed.checkout)
	return bed
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (bed *publishBed) gitEnv(dir string, env []string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=lane", "-c", "user.email=lane@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	command.Env = append(os.Environ(), env...)
	out, err := command.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (bed *publishBed) git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := bed.gitEnv(dir, nil, args...)
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return out
}

// commit makes a commit on top of parent in dir, off every branch, and
// returns it.
func (bed *publishBed) commit(t *testing.T, dir, parent, name string) string {
	t.Helper()
	bed.git(t, dir, "checkout", "--quiet", "--detach", parent)
	writeFile(t, filepath.Join(dir, name), name+"\n")
	bed.git(t, dir, "add", name)
	bed.git(t, dir, "commit", "--quiet", "-m", name)
	return bed.git(t, dir, "rev-parse", "HEAD")
}

func (bed *publishBed) main(t *testing.T) string {
	t.Helper()
	return bed.git(t, bed.origin, "rev-parse", "refs/heads/main")
}

func (bed *publishBed) tuple(t *testing.T, old, next string) Tuple {
	t.Helper()
	return Tuple{Repo: bed.checkout, RemoteURL: bed.origin, Ref: MainRef, Old: old, New: next,
		Tree: bed.git(t, bed.checkout, "rev-parse", next+"^{tree}"), Kind: KindLanding, Op: "batch-1", ProofAttempt: "attempt-1", Generation: 3}
}

// mintFor mints tuple's token the way Publish does, under the gate, and
// returns its nonce.
func (bed *publishBed) mintFor(t *testing.T, tuple Tuple) string {
	t.Helper()
	var nonce string
	if err := Gate(bed.home, OpPublish, AuthorityAgent, func(Record) error {
		minted, err := mint(bed.home, tuple, OpPublish, AuthorityAgent)
		nonce = minted
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return nonce
}

// push runs git push from the lane checkout, the token's nonce (when one)
// in its environment.
func (bed *publishBed) push(nonce, remote string, refspecs ...string) (string, error) {
	var env []string
	if nonce != "" {
		env = []string{TokenEnv + "=" + nonce}
	}
	return bed.gitEnv(bed.checkout, env, append([]string{"push", "--porcelain", remote}, refspecs...)...)
}

// The design's K3 witness: the lane checkout's pre-push hook, installed by
// landing set and run by git itself, admits exactly the one ref update a
// publication token was minted for, and nothing else on main: no push to
// main without a token, no other commit or remote, no second ref beside
// main, no push while the
// lane is paused, and no second use of a spent token. Publish mints and
// pushes the exact tuple with the lease, and main is its new commit.
func TestHookAdmitsOnlyTheTuple(t *testing.T) {
	t.Parallel()
	bed := newPublishBed(t)
	old := bed.main(t)
	next := bed.commit(t, bed.checkout, old, "next.txt")
	other := bed.commit(t, bed.checkout, old, "other.txt")
	refused := func(what, out string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s was pushed:\n%s", what, out)
		}
		if !strings.Contains(out, "only a landing publication pushes from the landing checkout") && !strings.Contains(out, "stopped") {
			t.Fatalf("%s was refused without the hook's two lines:\n%s", what, out)
		}
		if main := bed.main(t); main != old {
			t.Fatalf("%s moved main to %s", what, main)
		}
	}

	out, err := bed.push("", bed.origin, next+":"+MainRef)
	refused("a push without a token", out, err)

	tuple := bed.tuple(t, old, next)
	nonce := bed.mintFor(t, tuple)
	out, err = bed.push(nonce, bed.origin, other+":"+MainRef)
	refused("another commit under the token", out, err)
	out, err = bed.push(nonce, bed.origin, next+":"+MainRef, other+":refs/heads/extra")
	refused("a second ref under the token", out, err)
	mirror := filepath.Join(filepath.Dir(bed.origin), "mirror.git")
	bed.git(t, filepath.Dir(bed.origin), "clone", "--quiet", "--bare", bed.origin, mirror)
	out, err = bed.push(nonce, mirror, next+":"+MainRef)
	if err == nil || !strings.Contains(out, "only a landing publication pushes") {
		t.Fatalf("a push to another remote under the token was admitted:\n%s", out)
	}
	if _, err := SetPause(bed.home, "Wido", laneNow); err != nil {
		t.Fatal(err)
	}
	out, err = bed.push(nonce, bed.origin, next+":"+MainRef)
	refused("the exact tuple while the lane is paused", out, err)
	if _, err := ClearPause(bed.home); err != nil {
		t.Fatal(err)
	}

	if out, err := bed.push(nonce, bed.origin, next+":"+MainRef); err != nil || bed.main(t) != next {
		t.Fatalf("the exact tuple was refused: %v\n%s", err, out)
	}
	third := bed.commit(t, bed.checkout, next, "third.txt")
	out, err = bed.push(nonce, bed.origin, third+":"+MainRef)
	if err == nil || bed.main(t) != next {
		t.Fatalf("a spent token pushed again:\n%s", out)
	}

	// The production path: Publish mints, pushes with the lease and reads
	// main back, and leaves no token behind.
	if err := Publish(bed.home, bed.tuple(t, next, third), OpPublish, AuthorityAgent); err != nil || bed.main(t) != third {
		t.Fatalf("Publish of the exact tuple = %v, main %s", err, bed.main(t))
	}
	if left, _ := os.ReadDir(tokenDir(bed.home)); len(left) != 0 {
		t.Fatalf("tokens left behind: %v", left)
	}
}

// A moved base is returned, never republished: Publish on a stale expected
// commit pushes nothing and names the main it found.
func TestPublishReturnsAMovedBase(t *testing.T) {
	t.Parallel()
	bed := newPublishBed(t)
	old := bed.main(t)
	moved := bed.commit(t, bed.seat, old, "seat.txt")
	bed.git(t, bed.seat, "push", "--quiet", "origin", moved+":"+MainRef)
	next := bed.commit(t, bed.checkout, old, "next.txt")
	err := Publish(bed.home, bed.tuple(t, old, next), OpPublish, AuthorityAgent)
	var refused *PublishError
	if !errors.As(err, &refused) || refused.Code != CodeBaseMoved || refused.Current != moved || !IsBaseMoved(err) {
		t.Fatalf("publish on a moved base = %v; want %s naming %s", err, CodeBaseMoved, moved)
	}
	if main := bed.main(t); main != moved {
		t.Fatalf("a moved base was republished: main %s", main)
	}
}

func ledgerRequest(opid, target string, beforePush func(int) error) goal.PublishRequest {
	return goal.PublishRequest{Opid: opid, Machine: "landing", Lineage: "landing-lane",
		Intent: goal.Intent{Verb: "open", Targets: []string{target}}, Message: "goal open " + target,
		Mutate: func(string) ([]goal.Change, error) {
			return []goal.Change{{Path: "plans/goals/" + target + ".md", Content: []byte("# " + target + "\nState: queued\n")}}, nil
		},
		BeforePush: beforePush}
}

// seatAdvance moves main from the seat, as another seat's own landing would.
func (bed *publishBed) seatAdvance(t *testing.T, name string) string {
	t.Helper()
	bed.git(t, bed.seat, "fetch", "--quiet", "origin")
	moved := bed.commit(t, bed.seat, "origin/main", name)
	bed.git(t, bed.seat, "push", "--quiet", "origin", moved+":"+MainRef)
	return moved
}

// The design's K3 witness for the ledger: the lane's own goal transaction,
// through the lane adapter around PublishCAS, publishes through the same
// boundary as a landing (the hook admits it by its token), and a lease it
// loses comes back out of the transaction as LANE_BASE_MOVED after one
// attempt, instead of the retry loop rebuilding on the moved main.
func TestLaneAdapterSurfacesBaseMoved(t *testing.T) {
	t.Parallel()
	bed := newPublishBed(t)
	endpoint := LedgerEndpoint(bed.home, goal.Endpoint{Root: bed.install, Remote: "origin", Branch: MainRef}, OpPublish, AuthorityAgent)

	landed, err := goal.Publish(endpoint, ledgerRequest("op-lane-ok", "renewed", nil))
	if err != nil || landed.Outcome != goal.OutcomeConfirmed || bed.main(t) != landed.Commit {
		t.Fatalf("the lane's ledger write through the boundary = %+v %v; main %s", landed, err, bed.main(t))
	}
	// The fixture's ledger has no root record (pre-migration), so the
	// accepted ref the first write advanced would refuse the next capture.
	bed.git(t, bed.checkout, "update-ref", "-d", goal.AcceptedRef)

	attempts := 0
	var moved string
	result, err := goal.Publish(endpoint, ledgerRequest("op-lane-moved", "returned", func(attempt int) error {
		attempts = attempt
		if attempt == 1 {
			moved = bed.seatAdvance(t, "seat-landing.txt")
		}
		return nil
	}))
	if !IsBaseMoved(err) || goal.RefusalCode(err) != CodeBaseMoved || result.Code != CodeBaseMoved {
		t.Fatalf("a lost lease through the lane adapter = %+v %v; want %s", result, err, CodeBaseMoved)
	}
	if attempts != 1 {
		t.Fatalf("the transaction retried %d times on a moved base; want one attempt", attempts)
	}
	if main := bed.main(t); main != moved {
		t.Fatalf("main is %s, want the seat's %s untouched", main, moved)
	}
	// No-lane mode is first-class: once a person unsets the lane, the same
	// adapter publishes as the checkout's own write, retrying a lost lease
	// as always and minting nothing.
	if report, err := Unset(bed.home, "Wido", laneNow, false, emptyUnsetSeams()); err != nil || !report.Unregistered {
		t.Fatalf("unset = %+v %v", report, err)
	}
	bed.git(t, bed.checkout, "update-ref", "-d", goal.AcceptedRef)
	attempts = 0
	unlaned, err := goal.Publish(endpoint, ledgerRequest("op-no-lane", "after-unset", func(attempt int) error {
		attempts = attempt
		if attempt == 1 {
			bed.seatAdvance(t, "seat-after-unset.txt")
		}
		return nil
	}))
	if err != nil || unlaned.Outcome != goal.OutcomeConfirmed || attempts < 2 || bed.main(t) != unlaned.Commit {
		t.Fatalf("a write with no lane registered = %+v %v after %d attempts; want it retried and confirmed as before", unlaned, err, attempts)
	}
	entry, err := goal.ReadEntry(bed.install, "op-lane-moved")
	if err != nil || entry.Phase != goal.PhaseTerminal || entry.Outcome != goal.OutcomeAbandoned {
		t.Fatalf("the refused write's journal entry = %+v %v; want terminal abandoned", entry, err)
	}
}

// Seats' own goal writes are unchanged with a lane registered: a seat's
// endpoint takes no adapter, its lease loss is retried within the deadline
// as always, it mints no token, and the seat checkout has no lane hook.
func TestSeatGoalWritesUnchanged(t *testing.T) {
	t.Parallel()
	bed := newPublishBed(t)
	attempts := 0
	result, err := goal.Publish(goal.Endpoint{Root: bed.seat, Remote: "origin", Branch: MainRef}, ledgerRequest("op-seat", "seat-goal", func(attempt int) error {
		attempts = attempt
		if attempt == 1 {
			// Another publisher lands first, from the lane itself.
			if err := Publish(bed.home, bed.tuple(t, bed.main(t), bed.commit(t, bed.checkout, bed.main(t), "lane.txt")), OpPublish, AuthorityAgent); err != nil {
				return err
			}
		}
		return nil
	}))
	if err != nil || result.Outcome != goal.OutcomeConfirmed || attempts < 2 || bed.main(t) != result.Commit {
		t.Fatalf("a seat's goal write after a lost lease = %+v %v after %d attempts; want it retried and confirmed", result, err, attempts)
	}
	if left, _ := os.ReadDir(tokenDir(bed.home)); len(left) != 0 {
		t.Fatalf("a seat's goal write minted lane tokens: %v", left)
	}
	hooks := bed.git(t, bed.seat, "rev-parse", "--path-format=absolute", "--git-path", "hooks")
	if _, err := os.Stat(filepath.Join(hooks, "pre-push")); err == nil {
		t.Fatalf("the seat checkout has a pre-push hook")
	}
}

// landing set installs the hook in the lane checkout only and never
// replaces a hook of the checkout's own; unset takes it away with the lane,
// after which the checkout pushes as any other.
func TestLaneHookComesAndGoesWithTheLane(t *testing.T) {
	t.Parallel()
	bed := newPublishBed(t)
	hook := filepath.Join(bed.git(t, bed.checkout, "rev-parse", "--path-format=absolute", "--git-path", "hooks"), "pre-push")
	if data, err := os.ReadFile(hook); err != nil || !strings.Contains(string(data), hookMarker) || !strings.Contains(string(data), bed.install) {
		t.Fatalf("the lane checkout's hook = %q %v", data, err)
	}
	if report, err := Unset(bed.home, "Wido", laneNow, false, emptyUnsetSeams()); err != nil || !report.Unregistered {
		t.Fatalf("unset = %+v %v", report, err)
	}
	if _, err := os.Stat(hook); err == nil {
		t.Fatalf("the hook outlived the lane")
	}
	next := bed.commit(t, bed.checkout, bed.main(t), "after-unset.txt")
	if out, err := bed.push("", "origin", next+":"+MainRef); err != nil {
		t.Fatalf("the former lane checkout could not push after unset: %v\n%s", err, out)
	}

	writeFile(t, hook, "#!/bin/sh\nexit 0\n")
	layout, err := NewLayout(bed.checkout)
	if err != nil {
		t.Fatal(err)
	}
	var refusal *Refusal
	if _, _, err := Register(bed.home, layout, "Wido", laneNow); !errors.As(err, &refusal) || refusal.Code != CodeHookOccupied {
		t.Fatalf("landing set over the checkout's own hook = %v; want %s", err, CodeHookOccupied)
	}
	if _, ok, _ := Read(bed.home); ok {
		t.Fatalf("a refused hook registered the lane")
	}
}

// F-1: a push of the lane's repository made from another checkout's
// directory (git --git-dir=<lane>/.git, the hook's cwd is the seat) is
// judged as the lane's: without a token it moves nothing.
func TestHookJudgesTheLaneRepositoryFromAnyDirectory(t *testing.T) {
	t.Parallel()
	bed := newPublishBed(t)
	old := bed.main(t)
	next := bed.commit(t, bed.checkout, old, "elsewhere.txt")
	out, err := bed.gitEnv(bed.seat, nil, "--git-dir="+filepath.Join(bed.checkout, ".git"), "push", "--porcelain", bed.origin, next+":"+MainRef)
	if err == nil || bed.main(t) != old {
		t.Fatalf("a token-less push of the lane repository from the seat's directory moved main:\n%s", out)
	}
}

// F-2: the hook guards main only (K3): a token-less push from the lane
// checkout that does not touch refs/heads/main (landing unset's goal
// branch sweep deletes branches this way) passes; one that also touches
// main is refused whole.
func TestHookAdmitsUpdatesThatLeaveMainAlone(t *testing.T) {
	t.Parallel()
	bed := newPublishBed(t)
	old := bed.main(t)
	bed.git(t, bed.seat, "push", "--quiet", "origin", old+":refs/heads/goal/ship-widget")
	if out, err := bed.push("", bed.origin, ":refs/heads/goal/ship-widget"); err != nil {
		t.Fatalf("the goal branch sweep's delete was refused: %v\n%s", err, out)
	}
	next := bed.commit(t, bed.checkout, old, "mixed.txt")
	if out, err := bed.push("", bed.origin, next+":refs/heads/goal/other", next+":"+MainRef); err == nil || bed.main(t) != old {
		t.Fatalf("a token-less push touching main among other refs was admitted:\n%s", out)
	}
}

// F-3: a gate refusal of the lane's ledger write (here: the lane is
// paused) ends the goal transaction at once with its reason, never retried
// to the deadline.
func TestLaneLedgerWriteRefusedByTheGateStopsWithItsReason(t *testing.T) {
	t.Parallel()
	bed := newPublishBed(t)
	endpoint := LedgerEndpoint(bed.home, goal.Endpoint{Root: bed.install, Remote: "origin", Branch: MainRef}, OpPublish, AuthorityAgent)
	if _, err := SetPause(bed.home, "Wido", laneNow); err != nil {
		t.Fatal(err)
	}
	attempts := 0
	result, err := goal.Publish(endpoint, ledgerRequest("op-paused", "paused-write", func(attempt int) error { attempts = attempt; return nil }))
	if attempts != 1 || goal.RefusalCode(err) != CodePaused || result.Outcome != goal.OutcomeAbandoned || !strings.Contains(result.Detail, "stopped") {
		t.Fatalf("a paused lane's ledger write = %+v %v after %d attempts; want it stopped at once with the pause", result, err, attempts)
	}
}

// F-4: a token is tied to the process that minted it and to the enrolled
// engine number: a token whose process ended, or minted for another
// generation, admits nothing, and the next mint removes dead tokens.
func TestTokensDieWithTheirProcessAndGeneration(t *testing.T) {
	t.Parallel()
	bed := newPublishBed(t)
	old := bed.main(t)
	next := bed.commit(t, bed.checkout, old, "token.txt")
	stale := bed.tuple(t, old, next)
	stale.Generation = 2
	nonce := bed.mintFor(t, stale)
	if out, err := bed.push(nonce, bed.origin, next+":"+MainRef); err == nil || bed.main(t) != old {
		t.Fatalf("a token of another engine number was admitted:\n%s", out)
	}
	dead := exec.Command("true")
	if err := dead.Run(); err != nil {
		t.Fatal(err)
	}
	orphanOf := func() string {
		nonce := bed.mintFor(t, bed.tuple(t, old, next))
		var minted token
		if _, err := readJSON(tokenPath(bed.home, nonce), &minted); err != nil {
			t.Fatal(err)
		}
		minted.Pid = dead.Process.Pid
		if err := writeJSON(bed.home, tokenPath(bed.home, nonce), minted); err != nil {
			t.Fatal(err)
		}
		return nonce
	}
	orphan := orphanOf()
	if out, err := bed.push(orphan, bed.origin, next+":"+MainRef); err == nil || bed.main(t) != old {
		t.Fatalf("a token whose process ended was admitted:\n%s", out)
	}
	orphan = orphanOf()
	bed.mintFor(t, bed.tuple(t, old, next))
	if _, err := os.Stat(tokenPath(bed.home, orphan)); err == nil {
		t.Fatalf("the next mint left a dead process's token")
	}
}
