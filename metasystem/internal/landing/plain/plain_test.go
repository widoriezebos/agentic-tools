package plain

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
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
	done                    []string
	doneErr                 error
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

func (b *bed) seams() PushSeams {
	return PushSeams{Now: func() time.Time { return bedNow }, Done: func(entry Entry, main string) (string, error) {
		if b.doneErr != nil {
			return "", b.doneErr
		}
		b.done = append(b.done, entry.Goal)
		return "done", nil
	}}
}

func (b *bed) originMain() string {
	b.t.Helper()
	return b.git(b.root, "--git-dir", b.origin, "rev-parse", "refs/heads/main")
}

func states(t *testing.T, install string) map[string]string {
	t.Helper()
	entries, err := Entries(install)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, entry := range entries {
		out[entry.Goal] = entry.State
	}
	return out
}

// Two seats hand in; the agent stand-in merges both on main, proves the
// result green with the project's command and pushes: main moves by
// fast-forward to the proven HEAD, both goals are done, and both lines are
// landed, done first.
func TestTwoSeatsHandInAndLandByOneGreenPush(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	before := b.originMain()
	shaA := b.seat("seat-a", "goal-a")
	shaB := b.seat("seat-b", "goal-b")
	b.handIn("m1e", "goal-a", shaA)
	b.handIn("ui", "goal-b", shaB)
	if got := states(t, b.install); got["goal-a"] != StateWaiting || got["goal-b"] != StateWaiting {
		t.Fatalf("hand-ins wait: %v", got)
	}
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
	outcome, err := Push(b.install, b.checkout, b.seams())
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Changed || outcome.Old != before || outcome.Commit != head || b.originMain() != head {
		t.Fatalf("main fast-forwarded to HEAD: %+v origin=%s", outcome, b.originMain())
	}
	slices.Sort(b.done)
	if strings.Join(b.done, ",") != "goal-a,goal-b" || len(outcome.Settled) != 2 {
		t.Fatalf("both goals done: %v settled=%+v", b.done, outcome.Settled)
	}
	if got := states(t, b.install); got["goal-a"] != StateLanded || got["goal-b"] != StateLanded {
		t.Fatalf("both lines landed: %v", got)
	}
	if push, ok, err := LastPush(b.install); err != nil || !ok || push.Commit != head || push.Old != before {
		t.Fatalf("last push: %+v %v %v", push, ok, err)
	}
	// Idempotent: the same push again changes nothing and settles nothing.
	b.done = nil
	again, err := Push(b.install, b.checkout, b.seams())
	if err != nil || again.Changed || len(again.Settled) != 0 || len(b.done) != 0 {
		t.Fatalf("a repeat push: %+v %v done=%v", again, err, b.done)
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
		_, err := Push(b.install, b.checkout, b.seams())
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
	if got := states(t, b.install); got["goal-a"] != StateWaiting {
		t.Fatalf("nothing settled: %v", got)
	}
}

// F1: a push that crashed before its settlement is finished by the next
// push, which settles though it has nothing new to push; a done that fails
// leaves the line waiting, and it is settled once done succeeds.
func TestNextPushFinishesASettlementACrashLeft(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	sha := b.seat("seat-a", "goal-a")
	b.handIn("m1e", "goal-a", sha)
	head := b.merge("goal-a")
	b.prove(b.greenScript)
	// The crash: main got HEAD, nothing was settled.
	b.git(b.checkout, "push", "--quiet", "origin", "HEAD:refs/heads/main")
	b.doneErr = errors.New("the ledger can't be reached")
	if _, err := Push(b.install, b.checkout, b.seams()); err == nil || !strings.Contains(err.Error(), "the ledger can't be reached") {
		t.Fatalf("a failed done is the push's error: %v", err)
	}
	if got := states(t, b.install); got["goal-a"] != StateWaiting {
		t.Fatalf("no landed line before done: %v", got)
	}
	b.doneErr = nil
	outcome, err := Push(b.install, b.checkout, b.seams())
	if err != nil || outcome.Changed || outcome.Commit != head || len(outcome.Settled) != 1 || strings.Join(b.done, ",") != "goal-a" {
		t.Fatalf("the next push settles: %+v %v done=%v", outcome, err, b.done)
	}
	if got := states(t, b.install); got["goal-a"] != StateLanded {
		t.Fatalf("landed: %v", got)
	}
}

// A refused push still settles what main already holds.
func TestARefusedPushStillSettlesWhatMainHolds(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	sha := b.seat("seat-a", "goal-a")
	b.handIn("m1e", "goal-a", sha)
	b.merge("goal-a")
	b.git(b.checkout, "push", "--quiet", "origin", "HEAD:refs/heads/main")
	b.write(filepath.Join(b.checkout, "next.txt"), "next\n")
	b.git(b.checkout, "add", "next.txt")
	b.git(b.checkout, "commit", "--quiet", "-m", "next")
	_, err := Push(b.install, b.checkout, b.seams())
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.Code != CodeUnproven {
		t.Fatalf("unproven: %v", err)
	}
	if got := states(t, b.install); got["goal-a"] != StateLanded {
		t.Fatalf("settled though refused: %v", got)
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
	b.write(filepath.Join(b.root, "block"), "")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	seams := ProveSeams{Executable: func() (string, error) { return executable, nil },
		Launch: func(argv []string, dir, log string) (int64, error) {
			return gaterun.LaunchDetached(gaterun.DetachedLaunch{Argv: argv, Dir: dir, Log: log,
				Env: []string{proveChildEnv + "=" + b.install, proveChildCommand + "=" + b.greenScript}})
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
	b.write(b.blockRelease, "")
	deadline := time.Now().Add(2 * time.Minute)
	for {
		result, ok, err := LastResult(b.install)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			if result.Result != Green || result.Tree != running.Tree || result.Attempt != running.Attempt {
				t.Fatalf("result: %+v", result)
			}
			break
		}
		if time.Now().After(deadline) {
			log, _ := os.ReadFile(running.Log)
			t.Fatalf("the detached proof never ended: %s", log)
		}
		time.Sleep(20 * time.Millisecond)
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
