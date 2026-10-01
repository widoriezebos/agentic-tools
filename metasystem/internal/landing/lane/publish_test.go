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

// publishBed is a real lane: a bare file origin, a nested lane checkout
// (Git toplevel != installation) registered by landing set, the lane
// engine enrolled, and a seat clone of the same origin.
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
// as always.
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

// Publish pushes the exact tuple with the lease and reads main back; no
// token and no hook stand in the way.
func TestPublishPushesTheTuple(t *testing.T) {
	t.Parallel()
	bed := newPublishBed(t)
	old := bed.main(t)
	next := bed.commit(t, bed.checkout, old, "next.txt")
	if err := Publish(bed.home, bed.tuple(t, old, next), OpPublish, AuthorityAgent); err != nil || bed.main(t) != next {
		t.Fatalf("Publish of the exact tuple = %v, main %s", err, bed.main(t))
	}
	published, err := ReadPublications(bed.home)
	if err != nil || len(published) != 1 || published[0].New != next {
		t.Fatalf("publications = %+v %v; want the one", published, err)
	}
}

// The pre-push hook an earlier engine installed in the lane checkout goes
// at landing set, and with the lane at unset; a hook of the checkout's own
// stays.
func TestLandingSetAndUnsetRemoveTheEarlierHook(t *testing.T) {
	t.Parallel()
	bed := newPublishBed(t)
	hook := filepath.Join(bed.git(t, bed.checkout, "rev-parse", "--path-format=absolute", "--git-path", "hooks"), "pre-push")
	layout, err := NewLayout(bed.checkout)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, hook, "#!/bin/sh\n"+hookMarker+": only a landing publication pushes from this checkout.\nexit 1\n")
	if _, _, err := Register(bed.home, layout, "Wido", laneNow); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(hook); err == nil {
		t.Fatalf("landing set kept the earlier lane hook")
	}
	writeFile(t, hook, "#!/bin/sh\n"+hookMarker+"\nexit 1\n")
	if report, err := Unset(bed.home, "Wido", laneNow, false, emptyUnsetSeams()); err != nil || !report.Unregistered {
		t.Fatalf("unset = %+v %v", report, err)
	}
	if _, err := os.Stat(hook); err == nil {
		t.Fatalf("the earlier lane hook outlived the lane")
	}
	writeFile(t, hook, "#!/bin/sh\nexit 0\n")
	register(t, bed.home, bed.checkout)
	if data, err := os.ReadFile(hook); err != nil || string(data) != "#!/bin/sh\nexit 0\n" {
		t.Fatalf("landing set touched the checkout's own hook: %q %v", data, err)
	}
}
