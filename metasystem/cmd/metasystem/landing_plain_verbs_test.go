package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// plainVerbBed is a computer whose registered landing lane is a real
// nested checkout (Git toplevel != installation) of a bare origin, with a
// stand-in proof command. Its lane home is a run-scoped registry home, so a
// detached engine child finds the same lane.
type plainVerbBed struct {
	*laneVerbBed
	registry, checkout, installation, origin, main string
	// exited is the FIFO a detached engine child holds until it exits.
	exited string
	owners intentOwners
}

func newPlainVerbBed(t *testing.T) *plainVerbBed {
	t.Helper()
	bed := &plainVerbBed{laneVerbBed: newLaneVerbBed(t)}
	base := filepath.Dir(bed.home)
	bed.registry = filepath.Join(base, "registry")
	bed.home = filepath.Join(bed.registry, ".metasystem")
	if err := os.MkdirAll(bed.home, 0o755); err != nil {
		t.Fatal(err)
	}
	bed.registry, bed.home = realpath.Resolve(bed.registry), realpath.Resolve(bed.home)
	bed.checkout = filepath.Join(realpath.Resolve(base), "lane")
	bed.installation = filepath.Join(bed.checkout, "metasystem")
	if err := os.MkdirAll(bed.installation, 0o755); err != nil {
		t.Fatal(err)
	}
	bed.git(t, base, "init", "--quiet", "-b", "main", bed.checkout)
	for path, text := range map[string]string{
		filepath.Join(bed.installation, "metasystem.conf"): "metasystem.template=true\n",
		filepath.Join(bed.checkout, ".gitignore"):          "metasystem/artifacts/\nmetasystem/metasystem.conf.local\n",
	} {
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bed.origin = filepath.Join(base, "origin.git")
	bed.git(t, base, "init", "--quiet", "--bare", "-b", "main", bed.origin)
	bed.git(t, bed.checkout, "add", "-A")
	bed.git(t, bed.checkout, "commit", "--quiet", "-m", "main")
	bed.git(t, bed.checkout, "remote", "add", "origin", bed.origin)
	bed.git(t, bed.checkout, "push", "--quiet", "origin", "main")
	bed.git(t, bed.checkout, "fetch", "--quiet", "origin")
	bed.main = bed.git(t, bed.checkout, "rev-parse", "HEAD")
	registerLane(t, bed.home, bed.checkout, "Wido", laneTestNow)
	bed.owners = bed.laneVerbBed.owners()
	bed.owners.landing.returnClaim = func(string, string) error { return nil }
	bed.owners.resolver = stateroot.NewResolver(stateroot.RepositoryTop, os.Executable)
	return bed
}

func (bed *plainVerbBed) git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=lane", "-c", "user.email=lane@example.invalid", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// script writes a stand-in proof command that exits with code.
func (bed *plainVerbBed) script(t *testing.T, name string, code int) string {
	t.Helper()
	path := filepath.Join(filepath.Dir(bed.checkout), name)
	body := "#!/bin/sh\necho \"proving $LANDING_TREE at $LANDING_COMMIT\"\nexit " + string(rune('0'+code)) + "\n"
	if err := testexec.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func (bed *plainVerbBed) setCommand(t *testing.T, command string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(bed.installation, "metasystem.conf.local"), []byte("landing.prove.command="+command+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// seat pushes goal/G from a seat clone and hands it in, as work land does.
func (bed *plainVerbBed) seat(t *testing.T, goal string) string {
	t.Helper()
	dir := filepath.Join(filepath.Dir(bed.checkout), "seat-"+goal)
	bed.git(t, filepath.Dir(bed.checkout), "clone", "--quiet", bed.origin, dir)
	bed.git(t, dir, "checkout", "--quiet", "-b", "goal/"+goal)
	if err := os.WriteFile(filepath.Join(dir, goal+".txt"), []byte(goal+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.git(t, dir, "add", "-A")
	bed.git(t, dir, "commit", "--quiet", "-m", goal)
	bed.git(t, dir, "push", "--quiet", "origin", "goal/"+goal)
	sha := bed.git(t, dir, "rev-parse", "HEAD")
	if _, _, err := plain.HandIn(bed.installation, plain.Line{Goal: goal, Branch: "goal/" + goal, SHA: sha, Seat: "seat-" + goal, At: "2026-10-01T20:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	return sha
}

// merge is the landing agent stand-in: latest main, then each queued sha.
func (bed *plainVerbBed) merge(t *testing.T, shas ...string) string {
	t.Helper()
	bed.git(t, bed.checkout, "fetch", "--quiet", "origin")
	bed.git(t, bed.checkout, "checkout", "--quiet", "--detach", "origin/main")
	for _, sha := range shas {
		bed.git(t, bed.checkout, "merge", "--quiet", "--no-ff", "--no-edit", sha)
	}
	return bed.git(t, bed.checkout, "rev-parse", "HEAD")
}

func (bed *plainVerbBed) run(t *testing.T, words ...string) (int, string) {
	t.Helper()
	command, ok := findIntentAction(words[0], words[1])
	if !ok {
		t.Fatalf("no public command %s %s", words[0], words[1])
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, words[2:], &stdout, &stderr, bed.cwd, bed.owners)
	return code, stdout.String() + stderr.String()
}

func (bed *plainVerbBed) status(t *testing.T) map[string]any {
	t.Helper()
	code, text := bed.run(t, "landing", "status", "--json")
	var result struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(text), &result); err != nil || code != 0 {
		t.Fatalf("status --json = %d %v\n%s", code, err, text)
	}
	return result.Data
}

func queueStates(data map[string]any) map[string]string {
	states := map[string]string{}
	queue, _ := data["queue"].([]any)
	for _, raw := range queue {
		entry, _ := raw.(map[string]any)
		goal, _ := entry["goal"].(string)
		state, _ := entry["state"].(string)
		states[goal] = state
	}
	return states
}

// detachedEngine starts the proof as the production does, detached through
// gaterun.LaunchDetached, with this test binary standing in for the engine.
func (bed *plainVerbBed) detachedEngine(t *testing.T) plain.ProveSeams {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bed.exited = filepath.Join(filepath.Dir(bed.checkout), "exited.fifo")
	if err := syscall.Mkfifo(bed.exited, 0o600); err != nil {
		t.Fatal(err)
	}
	return plain.ProveSeams{Executable: func() (string, error) { return executable, nil },
		Launch: func(argv []string, dir, log string) (int64, error) {
			return gaterun.LaunchDetached(gaterun.DetachedLaunch{Argv: argv, Dir: dir, Log: log,
				Env: []string{"GO_WANT_BATCH_E2E_COMMAND=1", "METASYSTEM_SUPERVISION_REGISTRY_HOME=" + bed.registry, engineChildExited + "=" + bed.exited}})
		}}
}

// Plain lane steps 2 to 5 end to end over real git: two seats hand in, the
// agent stand-in merges both on latest main, landing prove starts the
// configured command over HEAD detached and returns at once; the proof
// outlives the call and records green for HEAD's tree; landing push
// fast-forwards main to HEAD and both lines then read landed, derived from
// main. landing status --json shows the queue, the last proof and the last
// push.
func TestPlainLaneVerbsLandTwoSeats(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
	bed.setCommand(t, bed.script(t, "prove-green.sh", 0))
	bed.owners.landing.plainProve = bed.detachedEngine(t)
	shaA, shaB := bed.seat(t, "goal-a"), bed.seat(t, "goal-b")
	data := bed.status(t)
	if got := queueStates(data); got["goal-a"] != plain.StateWaiting || got["goal-b"] != plain.StateWaiting {
		t.Fatalf("status shows the waiting queue: %v", got)
	}
	for _, key := range []string{"batch", "next", "running_proof", "last_proof", "last_push"} {
		if value, present := data[key]; !present || value != nil {
			t.Fatalf("status --json %s = %v, present %v; want null", key, value, present)
		}
	}
	head := bed.merge(t, shaA, shaB)
	tree := bed.git(t, bed.checkout, "rev-parse", "HEAD^{tree}")
	if code, text := bed.run(t, "landing", "prove"); code != 0 || !strings.Contains(text, "proving") {
		t.Fatalf("prove = %d\n%s", code, text)
	}
	// The verb returned; the detached engine child proves on and records
	// the result before it exits.
	waitForEngineChildExit(t, bed.exited)
	data = bed.status(t)
	if proof, _ := data["last_proof"].(map[string]any); proof == nil || proof["tree"] != tree || proof["result"] != plain.Green || proof["commit"] != head {
		t.Fatalf("last_proof = %v, running_proof = %v", data["last_proof"], data["running_proof"])
	}
	code, text := bed.run(t, "landing", "push")
	if code != 0 || !strings.Contains(text, "pushed "+shortLandingID(head)) {
		t.Fatalf("push = %d\n%s", code, text)
	}
	if got := bed.git(t, bed.checkout, "ls-remote", "origin", "refs/heads/main"); !strings.HasPrefix(got, head) {
		t.Fatalf("main is not HEAD: %s", got)
	}
	// Main moved by fast-forward: the old main is an ancestor of the new.
	bed.git(t, bed.checkout, "merge-base", "--is-ancestor", bed.main, head)
	data = bed.status(t)
	if got := queueStates(data); got["goal-a"] != plain.StateLanded || got["goal-b"] != plain.StateLanded {
		t.Fatalf("both lines landed: %v", got)
	}
	if push, _ := data["last_push"].(map[string]any); push == nil || push["commit"] != head {
		t.Fatalf("last_push = %v", data["last_push"])
	}
	if code, text := bed.run(t, "landing", "push"); code != 0 || !strings.Contains(text, "main already is "+shortLandingID(head)) {
		t.Fatalf("repeat push = %d\n%s", code, text)
	}
}

// landing push refuses a red tree, an unproven one, another tree's green
// and a HEAD that does not contain origin's main; main does not move.
func TestPlainLanePushRefusesRedUnprovenOtherTreeAndNonFastForward(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
	refused := func(label, want string) {
		t.Helper()
		code, text := bed.run(t, "landing", "push")
		if code != 1 || !strings.Contains(text, want) || len(strings.Split(strings.TrimRight(text, "\n"), "\n")) != 2 {
			t.Fatalf("%s: push = %d\n%s", label, code, text)
		}
		if got := bed.git(t, bed.checkout, "ls-remote", "origin", "refs/heads/main"); !strings.HasPrefix(got, bed.main) {
			t.Fatalf("%s: a refused push moved main: %s", label, got)
		}
	}
	bed.merge(t, bed.seat(t, "goal-a"))
	refused("unproven", "never proven")
	bed.setCommand(t, bed.script(t, "prove-red.sh", 1))
	if code, text := bed.run(t, "landing", "prove", "--wait"); code != 1 || !strings.Contains(text, "red") {
		t.Fatalf("a red prove = %d\n%s", code, text)
	}
	refused("red", "not green")
	if err := os.WriteFile(filepath.Join(bed.installation, "fix.txt"), []byte("fix\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.git(t, bed.checkout, "add", "-A")
	bed.git(t, bed.checkout, "commit", "--quiet", "-m", "fix red tree")
	bed.setCommand(t, bed.script(t, "prove-green.sh", 0))
	if code, text := bed.run(t, "landing", "prove", "--wait"); code != 0 {
		t.Fatalf("a green prove = %d\n%s", code, text)
	}
	if err := os.WriteFile(filepath.Join(bed.installation, "fix.txt"), []byte("another fix\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.git(t, bed.checkout, "add", "-A")
	bed.git(t, bed.checkout, "commit", "--quiet", "-m", "another tree")
	refused("other tree", "never proven")
	if code, text := bed.run(t, "landing", "prove", "--wait"); code != 0 {
		t.Fatalf("a green prove = %d\n%s", code, text)
	}
	other := bed.seat(t, "goal-b")
	bed.git(t, filepath.Dir(bed.checkout), "--git-dir", bed.origin, "update-ref", "refs/heads/main", other)
	bed.main = other
	refused("not a fast-forward", "does not contain origin's main")
}

// A red proof refuses the push; landing return gives the goal back with
// its reason, which the seat's queue line then shows; a repeat return is
// success and a return of nothing waiting is refused.
func TestPlainLaneRedIsReturnedToItsSeat(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
	bed.setCommand(t, bed.script(t, "prove-red.sh", 1))
	bed.merge(t, bed.seat(t, "goal-a"))
	if code, text := bed.run(t, "landing", "prove", "--wait"); code != 1 || !strings.Contains(text, "red") {
		t.Fatalf("a red prove = %d\n%s", code, text)
	}
	if code, text := bed.run(t, "landing", "return", "goal-a", "--reason", "app-standard fails since it joined"); code != 0 || !strings.Contains(text, "goal-a") {
		t.Fatalf("return = %d\n%s", code, text)
	}
	latest, ok, err := plain.Latest(bed.installation, "goal-a")
	if err != nil || !ok || latest.State != plain.StateReturned || latest.Reason != "app-standard fails since it joined" {
		t.Fatalf("the seat's line: %+v %v", latest, err)
	}
	if got := queueStates(bed.status(t)); got["goal-a"] != plain.StateReturned {
		t.Fatalf("status shows the return: %v", got)
	}
	if code, text := bed.run(t, "landing", "return", "goal-a", "--reason", "again"); code != 0 || !strings.Contains(text, "already returned") {
		t.Fatalf("a repeat return = %d\n%s", code, text)
	}
	if code, text := bed.run(t, "landing", "return", "goal-z", "--reason", "x"); code != 1 || !strings.Contains(text, "goal-z") {
		t.Fatalf("a return of nothing waiting = %d\n%s", code, text)
	}
}

// landing prove without a proof command refuses in two lines naming the
// setting; without --wait it starts the proof detached and returns; a
// repeat while that tree's proof runs starts nothing; another tree is
// refused while it runs; status shows the running proof; a paused lane
// refuses prove and push.
func TestPlainLaneProveStartsDetachedOnce(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
	code, text := bed.run(t, "landing", "prove")
	if lines := strings.Split(strings.TrimRight(text, "\n"), "\n"); code != 1 || len(lines) != 2 || !strings.Contains(lines[1], "metasystem settings set landing.prove.command") {
		t.Fatalf("no proof command = %d\n%s", code, text)
	}
	bed.setCommand(t, "true")
	var launched [][]string
	bed.owners.landing.plainProve = plain.ProveSeams{
		Executable: func() (string, error) { return "/engine/metasystem", nil },
		Launch: func(argv []string, dir, _ string) (int64, error) {
			if dir != bed.checkout {
				t.Errorf("the proof runs in %q", dir)
			}
			launched = append(launched, argv)
			return int64(os.Getpid()), nil
		}}
	if code, text := bed.run(t, "landing", "prove"); code != 0 || !strings.Contains(text, "proving") || len(launched) != 1 ||
		!slices.Equal(launched[0][:4], []string{"/engine/metasystem", "landing", "prove", "--wait"}) {
		t.Fatalf("prove = %d %v\n%s", code, launched, text)
	}
	if code, text := bed.run(t, "landing", "prove"); code != 0 || !strings.Contains(text, "already proving") || len(launched) != 1 {
		t.Fatalf("a repeat while it runs = %d %v\n%s", code, launched, text)
	}
	if running, _ := bed.status(t)["running_proof"].(map[string]any); running == nil || running["tree"] == nil || running["since"] == nil {
		t.Fatalf("running_proof = %v", running)
	}
	if code, text := bed.run(t, "landing", "status"); code != 0 || !strings.HasPrefix(text, "The landing lane is proving tree ") {
		t.Fatalf("status while the proof runs must say so in its first line = %d\n%s", code, text)
	}
	if err := os.WriteFile(filepath.Join(bed.installation, "other.txt"), []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.git(t, bed.checkout, "add", "-A")
	bed.git(t, bed.checkout, "commit", "--quiet", "-m", "other")
	if code, text := bed.run(t, "landing", "prove"); code != 1 || !strings.Contains(text, "is being proven") || len(launched) != 1 {
		t.Fatalf("another tree while one runs = %d\n%s", code, text)
	}
	if _, err := lane.SetPause(bed.home, "Wido", laneTestNow); err != nil {
		t.Fatal(err)
	}
	for _, verb := range []string{"prove", "push"} {
		if code, text := bed.run(t, "landing", verb); code != 1 || !strings.Contains(text, "stopped") {
			t.Fatalf("a paused %s = %d\n%s", verb, code, text)
		}
	}
}

func init() {
	registerIdempotency("landing resolve", idemStateful, "the tree is already resolved: success, nothing written", witnessLandingResolveRepeat)
	registerIdempotency("landing prove", idemStateful, "the same tree's proof runs: success, nothing started", witnessLandingProveRepeat)
	registerIdempotency("landing push", idemStateful, "main already is the proven HEAD: success, nothing pushed", witnessLandingPushRepeat)
	registerIdempotency("landing return", idemStateful, "the goal is already returned: success, nothing written", witnessLandingReturnRepeat)
}

// witnessLandingProveRepeat: the first landing prove starts the tree's
// proof in the background; the repeat while it runs is success, starts no
// second proof and leaves the host home and the lane's records as they
// were. The stand-in proof process is this test's own, so it runs.
func witnessLandingProveRepeat(t *testing.T) {
	bed := newPlainVerbBed(t)
	bed.setCommand(t, "true")
	launches := 0
	bed.owners.landing.plainProve = plain.ProveSeams{Executable: func() (string, error) { return "/fixture/metasystem", nil },
		Launch: func([]string, string, string) (int64, error) { launches++; return int64(os.Getpid()), nil }}
	if code, text := bed.run(t, "landing", "prove"); code != 0 || !strings.Contains(text, "proving ") {
		t.Fatalf("first landing prove = %d\n%s", code, text)
	}
	home, records := idemTreeDigest(t, bed.home), idemTreeDigest(t, plain.Dir(bed.installation))
	if code, text := bed.run(t, "landing", "prove"); code != 0 || !strings.Contains(text, "already proving") {
		t.Fatalf("repeated landing prove = %d\n%s", code, text)
	}
	idemSameTree(t, "a repeated landing prove (home)", home, idemTreeDigest(t, bed.home))
	idemSameTree(t, "a repeated landing prove (records)", records, idemTreeDigest(t, plain.Dir(bed.installation)))
	if launches != 1 {
		t.Fatalf("a repeated landing prove started %d proofs; want 1", launches)
	}
}

// witnessLandingPushRepeat pushes the lane checkout's proven HEAD twice:
// the second is success, pushes nothing and leaves the records as they
// were.
func witnessLandingPushRepeat(t *testing.T) {
	bed := newPlainVerbBed(t)
	head := bed.merge(t, bed.seat(t, "goal-a"))
	bed.setCommand(t, "true")
	if code, text := bed.run(t, "landing", "prove", "--wait"); code != 0 {
		t.Fatalf("prove = %d\n%s", code, text)
	}
	if code, text := bed.run(t, "landing", "push"); code != 0 || !strings.Contains(text, "pushed "+shortLandingID(head)+" to main (from "+shortLandingID(bed.main)+")") {
		t.Fatalf("first push = %d\n%s", code, text)
	}
	home, records := idemTreeDigest(t, bed.home), idemTreeDigest(t, plain.Dir(bed.installation))
	if code, text := bed.run(t, "landing", "push"); code != 0 || !strings.Contains(text, "main already is "+shortLandingID(head)) {
		t.Fatalf("repeated push = %d\n%s", code, text)
	}
	idemSameTree(t, "a repeated landing push (home)", home, idemTreeDigest(t, bed.home))
	idemSameTree(t, "a repeated landing push (records)", records, idemTreeDigest(t, plain.Dir(bed.installation)))
}

// witnessLandingReturnRepeat returns a waiting goal twice: the second is
// success and leaves the lane's records as they were.
func witnessLandingReturnRepeat(t *testing.T) {
	bed := newPlainVerbBed(t)
	bed.seat(t, "goal-a")
	if code, text := bed.run(t, "landing", "return", "goal-a", "--reason", "red"); code != 0 {
		t.Fatalf("first return = %d\n%s", code, text)
	}
	records := idemTreeDigest(t, plain.Dir(bed.installation))
	if code, text := bed.run(t, "landing", "return", "goal-a", "--reason", "red"); code != 0 || !strings.Contains(text, "already returned") {
		t.Fatalf("repeated return = %d\n%s", code, text)
	}
	idemSameTree(t, "a repeated landing return", records, idemTreeDigest(t, plain.Dir(bed.installation)))
}

// F-1 at the verb: landing prove proves HEAD's committed tree in a worktree
// of its own, so uncommitted and untracked changes in the lane checkout
// (a steward's record, a stray file) neither refuse it nor reach the
// command: it starts, and --wait proves it green.
func TestPlainLaneProveIgnoresTheLaneCheckoutsChanges(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
	bed.setCommand(t, "test ! -e fix.txt")
	launched := 0
	bed.owners.landing.plainProve = plain.ProveSeams{Executable: func() (string, error) { return "/engine/metasystem", nil },
		Launch: func([]string, string, string) (int64, error) {
			launched++
			return int64(os.Getpid()), nil
		}}
	if err := os.WriteFile(filepath.Join(bed.installation, "fix.txt"), []byte("not committed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bed.installation, "metasystem.conf"), []byte("metasystem.template=true\n# steward\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, text := bed.run(t, "landing", "prove"); code != 0 || launched != 1 {
		t.Fatalf("landing prove = %d, launched %d\n%s", code, launched, text)
	}
	if err := os.Remove(filepath.Join(plain.Dir(bed.installation), "running.json")); err != nil {
		t.Fatal(err)
	}
	if code, text := bed.run(t, "landing", "prove", "--wait"); code != 0 || !strings.Contains(text, "green") {
		t.Fatalf("landing prove --wait = %d\n%s", code, text)
	}
}

// waitForEngineChildExit opens the exit FIFO, which lets a child blocked
// opening it run, and reads it to its end: the child's exit, the only
// writer.
func waitForEngineChildExit(t *testing.T, fifo string) {
	t.Helper()
	reader, err := os.Open(fifo)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if _, err := io.Copy(io.Discard, reader); err != nil {
		t.Fatal(err)
	}
}

// landing prove of a tree already proven green reports it at once, names
// landing push as the next step and starts nothing in the background, so
// the landing agent pushes in the same turn instead of ending it to wait
// for a wake its own instant result already consumed.
func TestPlainLaneProveReportsAKnownGreenAtOnce(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
	bed.setCommand(t, "true")
	if code, text := bed.run(t, "landing", "prove", "--wait"); code != 0 || !strings.Contains(text, "proven green") {
		t.Fatalf("prove --wait = %d\n%s", code, text)
	}
	var launched [][]string
	bed.owners.landing.plainProve = plain.ProveSeams{
		Executable: func() (string, error) { return "/engine/metasystem", nil },
		Launch: func(argv []string, _, _ string) (int64, error) {
			launched = append(launched, argv)
			return int64(os.Getpid()), nil
		}}
	code, text := bed.run(t, "landing", "prove")
	if code != 0 || !strings.Contains(text, "already proven green") || !strings.Contains(text, "metasystem landing push") || len(launched) != 0 {
		t.Fatalf("prove of a proven tree = %d %v\n%s", code, launched, text)
	}
}
