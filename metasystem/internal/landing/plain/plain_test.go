package plain

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

var bedNow = time.Date(2026, 10, 1, 22, 0, 0, 0, time.UTC)

// bed is an origin with main, seats that push goal branches to it, and a
// nested lane checkout (Git toplevel != installation) cloned from it.
type bed struct {
	t                       *testing.T
	root, origin            string
	checkout, install       string
	greenScript, redScript  string
	stampFile, blockRelease string
	seats                   int
}

func newBed(t *testing.T) *bed {
	t.Helper()
	b := &bed{t: t, root: t.TempDir()}
	b.origin = filepath.Join(b.root, "origin.git")
	b.git(b.root, "init", "--quiet", "--bare", "-b", "main", b.origin)
	seed := filepath.Join(b.root, "seed")
	b.git(b.root, "init", "--quiet", "-b", "main", seed)
	b.write(filepath.Join(seed, ".gitignore"), "metasystem/artifacts/\n")
	b.write(filepath.Join(seed, "metasystem", "metasystem.conf"), "metasystem.template=true\n")
	b.git(seed, "add", "-A")
	b.git(seed, "commit", "--quiet", "-m", "main")
	b.git(seed, "remote", "add", "origin", b.origin)
	b.git(seed, "push", "--quiet", "origin", "main")
	b.checkout = filepath.Join(b.root, "lane")
	b.git(b.root, "clone", "--quiet", b.origin, b.checkout)
	b.install = filepath.Join(b.checkout, "metasystem")
	// The stand-in proof commands: green records the tree and commit it was
	// given and exits 0; red exits 1.
	b.stampFile = filepath.Join(b.root, "proved.txt")
	b.greenScript = filepath.Join(b.root, "prove-green.sh")
	b.redScript = filepath.Join(b.root, "prove-red.sh")
	b.blockRelease = filepath.Join(b.root, "release")
	if err := testexec.WriteFile(b.greenScript, []byte("#!/bin/sh\ni=0\nwhile [ -e \""+b.root+"/block\" ] && [ ! -e \""+b.blockRelease+"\" ] && [ $i -lt 2400 ]; do sleep 0.05; i=$((i+1)); done\necho \"$LANDING_TREE $LANDING_COMMIT $(git rev-parse HEAD^{tree})\" >> \""+b.stampFile+"\"\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(b.redScript, []byte("#!/bin/sh\necho app-standard failed\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return b
}

func (b *bed) git(dir string, args ...string) string {
	b.t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=lane", "-c", "user.email=lane@example.invalid", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
	if err != nil {
		b.t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (b *bed) write(path, text string) {
	b.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

// seat clones origin as a seat, commits the goal's work on goal/G and pushes
// the branch; it returns the branch's sha.
func (b *bed) seat(name, goal string) string {
	b.t.Helper()
	dir := filepath.Join(b.root, name)
	if _, err := os.Stat(dir); err != nil {
		b.git(b.root, "clone", "--quiet", b.origin, dir)
	}
	b.git(dir, "checkout", "--quiet", "-B", "goal/"+goal, "origin/main")
	b.seats++
	b.write(filepath.Join(dir, goal+".txt"), fmt.Sprintf("%s %d\n", goal, b.seats))
	b.git(dir, "add", "-A")
	b.git(dir, "commit", "--quiet", "-m", goal)
	b.git(dir, "push", "--quiet", "--force", "origin", "goal/"+goal)
	return b.git(dir, "rev-parse", "HEAD")
}

func (b *bed) handIn(seat, goal, sha string) (Entry, bool) {
	b.t.Helper()
	entry, added, err := HandIn(b.install, Line{Goal: goal, Branch: "goal/" + goal, SHA: sha, Seat: seat, At: bedNow.Format(time.RFC3339)})
	if err != nil {
		b.t.Fatal(err)
	}
	return entry, added
}

// merge is the landing agent stand-in: latest main, then each queued branch.
func (b *bed) merge(goals ...string) string {
	b.t.Helper()
	b.git(b.checkout, "fetch", "--quiet", "origin")
	b.git(b.checkout, "checkout", "--quiet", "--detach", "origin/main")
	for _, goal := range goals {
		b.git(b.checkout, "merge", "--quiet", "--no-ff", "--no-edit", "origin/goal/"+goal)
	}
	return b.git(b.checkout, "rev-parse", "HEAD")
}

func (b *bed) prove(script string) Result {
	b.t.Helper()
	var output bytes.Buffer
	result, err := Run(b.install, b.checkout, script, "", &output, ProveSeams{Now: func() time.Time { return bedNow }})
	if err != nil {
		b.t.Fatalf("prove: %v (%s)", err, output.String())
	}
	return result
}

func (b *bed) push() (PushOutcome, error) {
	return Push(b.install, b.checkout, bedNow)
}

func (b *bed) originMain() string {
	b.t.Helper()
	return b.git(b.root, "--git-dir", b.origin, "rev-parse", "refs/heads/main")
}

// states are the queue's states, landed derived against origin's main.
func (b *bed) states() map[string]string {
	b.t.Helper()
	entries, err := Entries(b.install)
	if err != nil {
		b.t.Fatal(err)
	}
	b.git(b.checkout, "fetch", "--quiet", "origin")
	derived, err := Landed(entries, ContainedIn(b.checkout, b.originMain()))
	if err != nil {
		b.t.Fatal(err)
	}
	out := map[string]string{}
	for _, entry := range derived {
		out[entry.Goal] = entry.State
	}
	return out
}

// Two seats hand in; the agent stand-in merges both on main, proves the
// result green with the project's command and pushes: main moves by
// fast-forward to the proven HEAD and both lines read landed, derived from
// main; nothing is settled or recorded for them.
func TestTwoSeatsHandInAndLandByOneGreenPush(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	before := b.originMain()
	shaA := b.seat("seat-a", "goal-a")
	shaB := b.seat("seat-b", "goal-b")
	b.handIn("m1e", "goal-a", shaA)
	b.handIn("ui", "goal-b", shaB)
	if got := b.states(); got["goal-a"] != StateWaiting || got["goal-b"] != StateWaiting {
		t.Fatalf("hand-ins wait: %v", got)
	}
	queue, _ := os.ReadFile(queuePath(b.install))
	head := b.merge("goal-a", "goal-b")
	result := b.prove(b.greenScript)
	tree := b.git(b.checkout, "rev-parse", "HEAD^{tree}")
	if result.Result != Green || result.Tree != tree || result.Commit != head {
		t.Fatalf("green result for HEAD's tree: %+v", result)
	}
	stamp, _ := os.ReadFile(b.stampFile)
	if got := strings.Fields(string(stamp)); len(got) != 3 || got[0] != tree || got[1] != head || got[2] != tree {
		t.Fatalf("the command gets LANDING_TREE and LANDING_COMMIT and runs at HEAD: %q", stamp)
	}
	outcome, err := b.push()
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Changed || outcome.Old != before || outcome.Commit != head || b.originMain() != head {
		t.Fatalf("main fast-forwarded to HEAD: %+v origin=%s", outcome, b.originMain())
	}
	if got := b.states(); got["goal-a"] != StateLanded || got["goal-b"] != StateLanded {
		t.Fatalf("both lines read landed: %v", got)
	}
	if after, _ := os.ReadFile(queuePath(b.install)); !bytes.Equal(after, queue) {
		t.Fatalf("a push wrote the queue: %q", after)
	}
	if push, ok, err := LastPush(b.install); err != nil || !ok || push.Commit != head || push.Old != before {
		t.Fatalf("last push: %+v %v %v", push, ok, err)
	}
	// Idempotent: the same push again changes nothing.
	again, err := b.push()
	if err != nil || again.Changed || again.Commit != head {
		t.Fatalf("a repeat push: %+v %v", again, err)
	}
}

// Push refuses a red tree, an unproven one, a green result of another tree,
// and a HEAD that does not contain origin's main; main does not move.
func TestPushRefusesRedUnprovenOtherTreeAndNonFastForward(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	main := b.originMain()
	sha := b.seat("seat-a", "goal-a")
	b.handIn("m1e", "goal-a", sha)
	b.merge("goal-a")
	refused := func(code string) {
		t.Helper()
		_, err := b.push()
		var refusal *Refusal
		if !errors.As(err, &refusal) || refusal.Code != code {
			t.Fatalf("want %s, got %v", code, err)
		}
		if b.originMain() != main {
			t.Fatal("a refused push moved main")
		}
	}
	refused(CodeUnproven)
	b.prove(b.redScript)
	refused(CodeRed)
	// A green result of this tree, then HEAD moves to another tree.
	b.prove(b.greenScript)
	b.write(filepath.Join(b.checkout, "fix.txt"), "fix\n")
	b.git(b.checkout, "add", "fix.txt")
	b.git(b.checkout, "commit", "--quiet", "-m", "fix")
	refused(CodeUnproven)
	// Main moves under the lane: HEAD no longer contains it.
	b.prove(b.greenScript)
	other := b.seat("seat-b", "goal-b")
	b.git(filepath.Join(b.root, "seat-b"), "push", "--quiet", "origin", other+":refs/heads/main")
	main = b.originMain()
	refused(CodeNotFastForward)
	if got := b.states(); got["goal-a"] != StateWaiting {
		t.Fatalf("not landed: %v", got)
	}
}

// Pending is the keeper's signal: a hand-in neither returned nor in main.
// A line main already contains (pushed, or merged by any route) is not
// pending, nor is a returned one.
func TestPendingIsWhatMainDoesNotHoldAndNotReturned(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	shaA := b.seat("seat-a", "goal-a")
	shaB := b.seat("seat-b", "goal-b")
	shaC := b.seat("seat-c", "goal-c")
	b.handIn("m1e", "goal-a", shaA)
	b.handIn("m1e", "goal-b", shaB)
	b.handIn("ui", "goal-c", shaC)
	pending := func() []string {
		t.Helper()
		entries, err := Pending(b.install, b.checkout)
		if err != nil {
			t.Fatal(err)
		}
		goals := []string{}
		for _, entry := range entries {
			goals = append(goals, entry.Goal)
		}
		return goals
	}
	if got := pending(); strings.Join(got, ",") != "goal-a,goal-b,goal-c" {
		t.Fatalf("all pending: %v", got)
	}
	b.merge("goal-a")
	b.git(b.checkout, "push", "--quiet", "origin", "HEAD:refs/heads/main")
	if _, _, err := Return(b.install, "goal-b", "red", bedNow); err != nil {
		t.Fatal(err)
	}
	if got := pending(); strings.Join(got, ",") != "goal-c" {
		t.Fatalf("landed and returned are not pending: %v", got)
	}
}

// A hand-in repeated at the same sha is one line; a new sha is a new line.
// A return is kept with its reason, a repeat return changes nothing, and
// the seat's newest hand-in reads returned.
func TestHandInRepeatIsOneLineAndReturnShowsAtTheSeat(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	sha := b.seat("seat-a", "goal-a")
	if _, added := b.handIn("m1e", "goal-a", sha); !added {
		t.Fatal("first hand-in adds")
	}
	if entry, added := b.handIn("m1e", "goal-a", sha); added || entry.State != StateWaiting {
		t.Fatalf("repeat: %+v %v", entry, added)
	}
	data, _ := os.ReadFile(queuePath(b.install))
	if strings.Count(string(data), "\n") != 1 {
		t.Fatalf("one line: %q", data)
	}
	entry, changed, err := Return(b.install, "goal-a", "app-standard fails since it joined", bedNow)
	if err != nil || !changed || entry.State != StateReturned {
		t.Fatalf("return: %+v %v %v", entry, changed, err)
	}
	if _, changed, err := Return(b.install, "goal-a", "again", bedNow); err != nil || changed {
		t.Fatalf("repeat return: %v %v", changed, err)
	}
	latest, ok, err := Latest(b.install, "goal-a")
	if err != nil || !ok || latest.State != StateReturned || latest.Reason != "app-standard fails since it joined" {
		t.Fatalf("the seat reads its return: %+v", latest)
	}
	if _, _, err := Return(b.install, "goal-z", "x", bedNow); !errors.Is(err, ErrNotWaiting) {
		t.Fatalf("nothing waiting: %v", err)
	}
	// The seat fixes and hands in again: a new waiting line.
	fixed := b.seat("seat-a", "goal-a")
	if entry, added := b.handIn("m1e", "goal-a", fixed); !added || entry.State != StateWaiting {
		t.Fatalf("a new sha hands in again: %+v", entry)
	}
	waiting, _ := Waiting(b.install)
	if len(waiting) != 1 || waiting[0].SHA != fixed {
		t.Fatalf("waiting: %+v", waiting)
	}
}

// landing prove's detached start: the proof runs in a process of its own,
// a repeat while the same tree's proof runs starts nothing, another tree is
// refused, and the result is appended when it ends.
func TestDetachedProveRunsOnceAndRecordsItsResult(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	exited := filepath.Join(b.root, "exited.fifo")
	if err := syscall.Mkfifo(exited, 0o600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	seams := ProveSeams{Executable: func() (string, error) { return executable, nil },
		Launch: func(argv []string, dir, log string) (int64, error) {
			return gaterun.LaunchDetached(gaterun.DetachedLaunch{Argv: argv, Dir: dir, Log: log,
				Env: []string{proveChildEnv + "=" + b.install, proveChildCommand + "=" + b.greenScript, proveChildExited + "=" + exited}})
		}}
	running, already, err := Start(b.install, b.checkout, seams)
	if err != nil || already || running.Pid == 0 {
		t.Fatalf("start: %+v %v %v", running, already, err)
	}
	again, already, err := Start(b.install, b.checkout, seams)
	if err != nil || !already || again.Attempt != running.Attempt {
		t.Fatalf("a repeat while it runs: %+v %v %v", again, already, err)
	}
	b.write(filepath.Join(b.checkout, "other.txt"), "other\n")
	b.git(b.checkout, "add", "other.txt")
	b.git(b.checkout, "commit", "--quiet", "-m", "other")
	var busy *Busy
	if _, _, err := Start(b.install, b.checkout, seams); !errors.As(err, &busy) {
		t.Fatalf("another tree while one runs: %v", err)
	}
	// Back to the tree being proven, so its result stands.
	b.git(b.checkout, "reset", "--quiet", "--hard", running.Commit)
	// The child holds the exit FIFO from its start until it exits: opening
	// it lets the child run, and reading it to the end waits for its exit.
	waitForChildExit(t, exited)
	result, ok, err := LastResult(b.install)
	if err != nil || !ok || result.Result != Green || result.Tree != running.Tree || result.Attempt != running.Attempt {
		log, _ := os.ReadFile(running.Log)
		t.Fatalf("result: %+v %v %v\n%s", result, ok, err, log)
	}
	if _, recorded, _, _ := ReadRunning(b.install, ProveSeams{}); recorded {
		t.Fatal("running.json outlives its proof")
	}
}

// A proof whose process died without a result holds nothing: the next
// start proves again.
func TestADiedProofHoldsNothing(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	if err := os.MkdirAll(Dir(b.install), 0o755); err != nil {
		t.Fatal(err)
	}
	tree := b.git(b.checkout, "rev-parse", "HEAD^{tree}")
	if err := writeRunning(b.install, Running{Attempt: "dead", Tree: tree, Pid: 1, Process: "not-a-ref"}); err != nil {
		t.Fatal(err)
	}
	if _, recorded, alive, _ := ReadRunning(b.install, ProveSeams{}); !recorded || alive {
		t.Fatalf("recorded and dead: %v %v", recorded, alive)
	}
	result := b.prove(b.greenScript)
	if result.Result != Green {
		t.Fatalf("proved again: %+v", result)
	}
}

// The keeper's wake: "queued" while a hand-in is neither returned nor in
// main; "proof-finished" besides when a proof ended after the agent's last
// launch and queued work remains. A landed or returned queue wakes nothing,
// whatever proof ended.
func TestWakeReasonsAreQueuedAndProofFinished(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	reasons := func(launch time.Time) string {
		t.Helper()
		got, err := WakeReasons(b.install, b.checkout, launch)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(got, ",")
	}
	if got := reasons(time.Time{}); got != "" {
		t.Fatalf("an empty queue wakes nothing: %q", got)
	}
	sha := b.seat("seat-a", "goal-a")
	b.handIn("m1e", "goal-a", sha)
	if got := reasons(time.Time{}); got != WakeQueued {
		t.Fatalf("a hand-in wakes: %q", got)
	}
	b.merge("goal-a")
	b.prove(b.greenScript) // ends at bedNow
	if got := reasons(bedNow.Add(-time.Minute)); got != WakeQueued+","+WakeProofFinished {
		t.Fatalf("a proof that ended after the launch wakes: %q", got)
	}
	if got := reasons(bedNow.Add(time.Minute)); got != WakeQueued {
		t.Fatalf("a proof that ended before the launch was seen: %q", got)
	}
	if _, err := b.push(); err != nil {
		t.Fatal(err)
	}
	if got := reasons(bedNow.Add(-time.Minute)); got != "" {
		t.Fatalf("a landed queue wakes nothing: %q", got)
	}
}

// The keeper holds while running.json names a live proof, and not when its
// process is gone.
func TestProofHoldWhileALiveProofRuns(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	alive := true
	seams := ProveSeams{Alive: func(Running) bool { return alive }}
	if reason, err := ProofHold(b.install, seams); err != nil || reason != "" {
		t.Fatalf("no proof holds nothing: %q %v", reason, err)
	}
	if err := os.MkdirAll(Dir(b.install), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeRunning(b.install, Running{Attempt: "a1", Tree: strings.Repeat("t", 40), Since: bedNow.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	if reason, err := ProofHold(b.install, seams); err != nil || !strings.Contains(reason, "a1") {
		t.Fatalf("a live proof holds: %q %v", reason, err)
	}
	alive = false
	if reason, err := ProofHold(b.install, seams); err != nil || reason != "" {
		t.Fatalf("a dead proof holds nothing: %q %v", reason, err)
	}
}

// The proof runs in a fresh detached worktree of the lane repository at the
// commit being proven, from that worktree's installation folder: machinery
// writing a tracked file of the lane checkout while it runs (the steward's
// narrator digest) does not make it red, and the worktree is gone after.
func TestProveRunsInAFreshWorktreeSoLaneWritesDoNotRedIt(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.seat("seat-a", "goal-a")
	head := b.merge("goal-a")
	tree := b.git(b.checkout, "rev-parse", "HEAD^{tree}")
	where := filepath.Join(b.root, "where.txt")
	script := "echo steward >> \"" + filepath.Join(b.install, "metasystem.conf") + "\"\n" +
		"echo \"$(pwd -P) $(git rev-parse HEAD) $(git rev-parse HEAD^{tree})\" > \"" + where + "\"\n"
	result := b.prove(script)
	if result.Result != Green || result.Tree != tree || result.Commit != head || result.Reason != "" {
		t.Fatalf("a steward write in the lane checkout redded the proof: %+v", result)
	}
	data, err := os.ReadFile(where)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(string(data))
	install, _ := filepath.EvalSymlinks(b.install)
	if len(got) != 3 || got[0] == install || filepath.Base(got[0]) != "metasystem" || got[1] != head || got[2] != tree {
		t.Fatalf("the command ran at %q, not in a worktree's installation at %s", data, Short(head))
	}
	if _, err := os.Stat(got[0]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the proof's worktree outlived the proof: %v", err)
	}
	if list := b.git(b.checkout, "worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
		t.Fatalf("the proof's worktree is still registered:\n%s", list)
	}
}

// F-1 by construction: an uncommitted fix in the lane checkout is not seen
// by the proof, which proves the committed tree; red stays red until the fix
// is committed.
func TestProveSeesTheCommittedTreeNotTheLaneCheckout(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.seat("seat-a", "goal-a")
	b.merge("goal-a")
	needsFix := "cd \"$(git rev-parse --show-toplevel)\" && test -e fix.txt\n"
	b.write(filepath.Join(b.checkout, "fix.txt"), "an uncommitted fix\n")
	if result := b.prove(needsFix); result.Result != Red {
		t.Fatalf("the proof saw the uncommitted fix: %+v", result)
	}
	b.git(b.checkout, "add", "fix.txt")
	b.git(b.checkout, "commit", "--quiet", "-m", "fix")
	if result := b.prove(needsFix); result.Result != Green {
		t.Fatalf("the committed fix: %+v", result)
	}
}

// A proof's worktree left by a crash is removed by the next prove.
func TestProveRemovesAWorktreeACrashLeft(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	left := filepath.Join(proofTrees(b.install), "crashed")
	b.git(b.checkout, "worktree", "add", "--quiet", "--detach", left, "HEAD")
	if result := b.prove("true"); result.Result != Green {
		t.Fatalf("prove: %+v", result)
	}
	if _, err := os.Stat(left); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the crashed proof's worktree is still there: %v", err)
	}
	if list := b.git(b.checkout, "worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
		t.Fatalf("worktrees:\n%s", list)
	}
}

// F-2: a new hand-in of a goal supersedes the goal's older waiting line:
// it is not pending, so the keeper does not wake for it forever, and it
// reads superseded.
func TestANewHandInSupersedesTheOlderWaitingLine(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	first := b.seat("seat-a", "goal-a")
	b.handIn("m1e", "goal-a", first)
	second := b.seat("seat-a", "goal-a")
	b.handIn("m1e", "goal-a", second)
	pending, err := Pending(b.install, b.checkout)
	if err != nil || len(pending) != 1 || pending[0].SHA != second {
		t.Fatalf("pending: %+v %v", pending, err)
	}
	entries, _ := Entries(b.install)
	if len(entries) != 2 || entries[0].State != StateSuperseded || entries[1].State != StateWaiting {
		t.Fatalf("entries: %+v", entries)
	}
	if _, _, err := Return(b.install, "goal-a", "red", bedNow); err != nil {
		t.Fatal(err)
	}
	if pending, _ := Pending(b.install, b.checkout); len(pending) != 0 {
		t.Fatalf("after the return nothing is pending: %+v", pending)
	}
}
