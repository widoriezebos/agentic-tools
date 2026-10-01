package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// plainVerbBed is a registered nested lane checkout with an origin and a
// stand-in proof command.
type plainVerbBed struct {
	*kernelBed
	origin string
	main   string
	owners intentOwners
	seats  int
}

func newPlainVerbBed(t *testing.T) *plainVerbBed {
	t.Helper()
	bed := &plainVerbBed{kernelBed: newKernelBed(t)}
	bed.main = bed.landMain(t)
	bed.origin = filepath.Join(filepath.Dir(bed.checkout), "origin.git")
	bed.owners = bed.kernelBed.owners()
	return bed
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

func (bed *kernelBed) setCommand(t *testing.T, command string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(bed.installation, "metasystem.conf.local"), []byte("landing.prove.command="+command+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// seat pushes goal/G from a seat clone and hands it in, as work land does.
func (bed *plainVerbBed) seat(t *testing.T, goal string) string {
	t.Helper()
	bed.seats++
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

// merge is the landing agent stand-in: latest main, then each branch.
func (bed *plainVerbBed) merge(t *testing.T, goals ...string) string {
	t.Helper()
	bed.git(t, bed.checkout, "fetch", "--quiet", "origin")
	bed.git(t, bed.checkout, "checkout", "--quiet", "--detach", "origin/main")
	for _, goal := range goals {
		bed.git(t, bed.checkout, "merge", "--quiet", "--no-ff", "--no-edit", "origin/goal/"+goal)
	}
	return bed.git(t, bed.checkout, "rev-parse", "HEAD")
}

func (bed *plainVerbBed) run(t *testing.T, words ...string) (int, string) {
	t.Helper()
	return bed.runWith(t, bed.owners, words...)
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

// Plain lane steps 3, 4 and 6 end to end over real git: two seats hand in,
// the agent stand-in merges both, landing prove runs the configured command
// over HEAD and records green, landing push fast-forwards main to HEAD and
// both lines then read landed; landing status --json shows the queue, the
// last proof and the last push.
func TestPlainLaneVerbsLandTwoSeats(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
	code, text := bed.run(t, "landing", "prove", "--wait")
	if code == 0 || !strings.Contains(text, "settings set landing.prove.command") {
		t.Fatalf("no proof command = %d\n%s", code, text)
	}
	bed.setCommand(t, bed.script(t, "prove-green.sh", 0))
	bed.seat(t, "goal-a")
	bed.seat(t, "goal-b")
	if got := queueStates(bed.status(t)); got["goal-a"] != plain.StateWaiting || got["goal-b"] != plain.StateWaiting {
		t.Fatalf("status shows the waiting queue: %v", got)
	}
	head := bed.merge(t, "goal-a", "goal-b")
	tree := bed.git(t, bed.checkout, "rev-parse", "HEAD^{tree}")
	if code, text := bed.run(t, "landing", "prove", "--wait"); code != 0 || !strings.Contains(text, "green") {
		t.Fatalf("prove = %d\n%s", code, text)
	}
	data := bed.status(t)
	if proof, _ := data["last_proof"].(map[string]any); proof == nil || proof["tree"] != tree || proof["result"] != plain.Green || proof["commit"] != head {
		t.Fatalf("last_proof = %v", data["last_proof"])
	}
	code, text = bed.run(t, "landing", "push")
	if code != 0 || !strings.Contains(text, "pushed "+shortLandingID(head)) {
		t.Fatalf("push = %d\n%s", code, text)
	}
	if got := bed.git(t, bed.checkout, "ls-remote", "origin", "refs/heads/main"); !strings.HasPrefix(got, head) {
		t.Fatalf("main is not HEAD: %s", got)
	}
	data = bed.status(t)
	if got := queueStates(data); got["goal-a"] != plain.StateLanded || got["goal-b"] != plain.StateLanded {
		t.Fatalf("both lines landed: %v", got)
	}
	if push, _ := data["last_push"].(map[string]any); push == nil || push["commit"] != head || push["old"] != bed.main {
		t.Fatalf("last_push = %v", data["last_push"])
	}
	// Idempotent: nothing new to push is success.
	if code, text := bed.run(t, "landing", "push"); code != 0 || !strings.Contains(text, "main already is "+shortLandingID(head)) {
		t.Fatalf("repeat push = %d\n%s", code, text)
	}
}

// A red proof refuses the push; landing return gives the goal back with
// its reason, which the seat's queue line then shows; a repeat return is
// success.
func TestPlainLaneRedIsReturnedToItsSeat(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
	bed.setCommand(t, bed.script(t, "prove-red.sh", 1))
	bed.seat(t, "goal-a")
	bed.merge(t, "goal-a")
	if code, text := bed.run(t, "landing", "prove", "--wait"); code != 1 || !strings.Contains(text, "red") {
		t.Fatalf("a red prove = %d\n%s", code, text)
	}
	if code, text := bed.run(t, "landing", "push"); code != 1 || !strings.Contains(text, "not green") {
		t.Fatalf("a push of a red tree = %d\n%s", code, text)
	}
	if code, text := bed.run(t, "landing", "return", "goal-a", "--reason", "app-standard fails since it joined"); code != 0 || !strings.Contains(text, "goal-a") {
		t.Fatalf("return = %d\n%s", code, text)
	}
	latest, ok, err := plain.Latest(bed.installation, "goal-a")
	if err != nil || !ok || latest.State != plain.StateReturned || latest.Reason != "app-standard fails since it joined" {
		t.Fatalf("the seat's line: %+v %v", latest, err)
	}
	if code, text := bed.run(t, "landing", "return", "goal-a", "--reason", "again"); code != 0 || !strings.Contains(text, "already returned") {
		t.Fatalf("a repeat return = %d\n%s", code, text)
	}
	if code, text := bed.run(t, "landing", "return", "goal-z", "--reason", "x"); code != 1 || !strings.Contains(text, "goal-z") {
		t.Fatalf("a return of nothing waiting = %d\n%s", code, text)
	}
}

// landing prove without --wait starts the proof detached and returns; a
// repeat while that tree's proof runs starts nothing; status shows the
// running proof; a paused lane refuses.
func TestPlainLaneProveStartsDetachedOnce(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
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
	if running, _ := bed.status(t)["running_proof"].(map[string]any); running == nil || running["state"] != "running" {
		t.Fatalf("running_proof = %v", running)
	}
	if _, err := lane.SetPause(bed.home, "Wido", laneTestNow); err != nil {
		t.Fatal(err)
	}
	if code, text := bed.run(t, "landing", "push"); code != 1 || !strings.Contains(text, "stopped") {
		t.Fatalf("a paused push = %d\n%s", code, text)
	}
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

func init() {
	registerIdempotency("landing return", idemStateful, "the goal is already returned: success, nothing written", witnessLandingReturnRepeat)
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
