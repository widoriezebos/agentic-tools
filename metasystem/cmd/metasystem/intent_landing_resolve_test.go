package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type resolveVerbFixture struct {
	root, install, home string
	owners              intentOwners
	contract            string
}

func newResolveVerbFixture(t *testing.T) *resolveVerbFixture {
	t.Helper()
	b := &resolveVerbFixture{root: realpath.Resolve(t.TempDir()), home: realpath.Resolve(t.TempDir())}
	b.install = filepath.Join(b.root, "metasystem")
	for _, dir := range []string{b.install, filepath.Join(b.root, ".git"), lane.HostDir(b.home)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("testing.contract=testing.json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	contract, err := testpolicy.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	contract.Generated = []testpolicy.Generated{{Paths: []string{"out/**"}, Command: []string{"fixture-build"}}}
	data, err = json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	b.contract = string(data)
	record, err := json.Marshal(lane.Record{Root: b.root, Install: b.install, CustodyEpoch: 1, RegisteredBy: "Wido"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lane.RecordPath(b.home), record, 0o600); err != nil {
		t.Fatal(err)
	}
	b.owners = (&laneVerbBed{home: b.home}).owners()
	return b
}

func (b *resolveVerbFixture) run(t *testing.T, cwd string, words ...string) (int, string) {
	t.Helper()
	command, ok := findIntentAction("landing", words[0])
	if !ok {
		t.Fatalf("landing %s is not discoverable", words[0])
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, words[1:], &stdout, &stderr, cwd, b.owners)
	return code, stdout.String() + stderr.String()
}

func witnessLandingResolveRepeat(t *testing.T) {
	b := newResolveVerbFixture(t)
	if _, _, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "goal-sha"}); err != nil {
		t.Fatal(err)
	}
	b.owners.landing.plainResolve.Git = func(dir string, args ...string) (string, error) {
		if dir != b.root {
			t.Fatalf("Git cwd=%s", dir)
		}
		switch strings.Join(args, " ") {
		case "show HEAD:metasystem/testing.json":
			return b.contract, nil
		case "diff --name-only --diff-filter=U -z":
			return "", nil
		default:
			t.Fatalf("hold queried %v", args)
			return "", nil
		}
	}
	home, records := idemTreeDigest(t, b.home), idemTreeDigest(t, plain.Dir(b.install))
	var first string
	for run := range 2 {
		code, text := b.run(t, b.root, "resolve", "--json")
		var result struct {
			Outcome string
			Data    plain.ResolveOutcome
		}
		if err := json.Unmarshal([]byte(text), &result); err != nil || code != 0 || result.Outcome != intentUnchanged || !result.Data.Held {
			t.Fatalf("landing resolve run %d = %d %s", run+1, code, text)
		}
		if run == 0 {
			first = text
		} else if text != first {
			t.Fatalf("repeated landing resolve changed its answer: first=%s repeated=%s", first, text)
		}
		idemSameTree(t, "landing resolve on a resolved tree (home)", home, idemTreeDigest(t, b.home))
		idemSameTree(t, "landing resolve on a resolved tree (records)", records, idemTreeDigest(t, plain.Dir(b.install)))
	}
}

func TestLandingResolveVerbUsesMainsContractAndReturnsSourceDetails(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	if _, _, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "goal-sha"}); err != nil {
		t.Fatal(err)
	}
	// The checkout's conflicted contract cannot be decoded; main's blob can.
	if err := os.WriteFile(filepath.Join(b.install, "testing.json"), []byte("<<<<<<< conflict"), 0o644); err != nil {
		t.Fatal(err)
	}
	var writes [][]string
	b.sourceConflictGit(t, func() { writes = append(writes, []string{"merge", "--abort"}) })
	code, text := b.run(t, b.root, "resolve", "--json")
	var result struct {
		Outcome string
		Details []string
		Data    plain.ResolveOutcome
	}
	if code != 0 || json.Unmarshal([]byte(text), &result) != nil {
		t.Fatalf("resolve exit=%d output=%s", code, text)
	}
	if result.Outcome != intentConfirmed || len(result.Details) != 0 {
		t.Fatalf("source return outcome=%s details=%v", result.Outcome, result.Details)
	}
	if result.Data.Conflict == nil || result.Data.Conflict.Main != "main-sha" || len(result.Data.Conflict.Paths) != 1 || result.Data.Conflict.Paths[0].Resolution != "keep both, main's lines then the goal's" || result.Data.Entry == nil || result.Data.Entry.State != plain.StateReturned || !reflect.DeepEqual(writes, [][]string{{"merge", "--abort"}}) {
		t.Fatalf("returned conflict=%+v writes=%v", result.Data, writes)
	}
}

func (b *resolveVerbFixture) sourceConflictGit(t *testing.T, abort func()) {
	t.Helper()
	b.owners.landing.plainResolve = plain.ResolveSeams{Git: func(dir string, args ...string) (string, error) {
		if dir != b.root {
			t.Fatalf("Git cwd=%s", dir)
		}
		switch strings.Join(args, " ") {
		case "show HEAD:metasystem/testing.json":
			return b.contract, nil
		case "diff --name-only --diff-filter=U -z":
			return "metasystem/src/list.go\x00", nil
		case "rev-parse --verify HEAD^{commit}":
			return "main-sha", nil
		case "rev-parse --verify MERGE_HEAD^{commit}":
			return "goal-sha", nil
		case "diff --name-only -z AUTO_MERGE --", "ls-files --others --exclude-standard -z":
			return "", nil
		case "ls-files --unmerged -z -- metasystem/src/list.go":
			return "100644 base 1\tsrc\x00100644 main 2\tsrc\x00100644 goal 3\tsrc\x00", nil
		case "merge --abort":
			abort()
			return "", nil
		}
		if args[0] == "diff" {
			return "@@ -2,0 +3 @@\n+insert\n", nil
		}
		t.Fatalf("unexpected Git call %v", args)
		return "", nil
	}}
}

func TestLandingResolveVerbFailsWhenReturnCannotBeAppended(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	if _, _, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "goal-sha"}); err != nil {
		t.Fatal(err)
	}
	aborted := false
	b.sourceConflictGit(t, func() {
		aborted = true
		// Keep the hand-in readable while refusing the return's append.
		if err := os.Chmod(filepath.Join(plain.Dir(b.install), "queue.jsonl"), 0o444); err != nil {
			t.Fatal(err)
		}
	})
	code, text := b.run(t, b.root, "resolve", "--json")
	var result struct {
		Outcome string
		Details []string
		Data    plain.ResolveOutcome
	}
	if json.Unmarshal([]byte(text), &result) != nil || code != 1 || result.Outcome != intentFailed || !aborted || result.Data.Entry == nil || result.Data.Entry.State != plain.StateWaiting || len(result.Details) == 0 {
		t.Fatalf("return append failure=%d %s aborted=%v", code, text, aborted)
	}
	entry, ok, err := plain.Latest(b.install, "goal")
	if err != nil || !ok || entry.State != plain.StateWaiting {
		t.Fatalf("unrecorded return changed queue: %+v ok=%v err=%v", entry, ok, err)
	}
}

func TestLandingResolveVerbReportsFailedRegenerationAsReturnedWithoutRetry(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	if _, _, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "goal-sha"}); err != nil {
		t.Fatal(err)
	}
	aborted := false
	b.owners.landing.plainResolve = plain.ResolveSeams{
		Git: func(_ string, args ...string) (string, error) {
			switch strings.Join(args, " ") {
			case "show HEAD:metasystem/testing.json":
				return b.contract, nil
			case "diff --name-only --diff-filter=U -z", "ls-files -z", "ls-tree -r --name-only -z HEAD -- metasystem/out/result":
				return "metasystem/out/result\x00", nil
			case "rev-parse --verify HEAD^{commit}":
				return "main-sha", nil
			case "rev-parse --verify MERGE_HEAD^{commit}":
				return "goal-sha", nil
			case "diff --name-only -z AUTO_MERGE --", "ls-files --others --exclude-standard -z",
				"restore --source=HEAD --staged --worktree -- metasystem/out/result",
				"restore --source=AUTO_MERGE --worktree -- metasystem/out/result":
				return "", nil
			case "merge --abort":
				aborted = true
				return "", nil
			default:
				t.Fatalf("unexpected Git call %v", args)
				return "", nil
			}
		},
		Run: func(_ []string, _ string, log *os.File, _ func(int64) error) error {
			if _, err := log.WriteString("fixture regeneration failed\n"); err != nil {
				t.Fatal(err)
			}
			return exec.Command("/usr/bin/false").Run()
		},
	}
	code, text := b.run(t, b.root, "resolve")
	entry, ok, err := plain.Latest(b.install, "goal")
	if err != nil || !ok || entry.State != plain.StateReturned || !aborted {
		t.Fatalf("queue=%+v ok=%v err=%v aborted=%v", entry, ok, err, aborted)
	}
	if code != 1 || !strings.Contains(oneSpaced(text), "returned goal: regeneration exited 1; log:") ||
		!strings.Contains(oneSpaced(text), oneSpaced(entry.Reason)) || strings.Contains(text, "tries again") || strings.Contains(text, "could not be resolved") {
		t.Fatalf("failed regeneration answer=%d %s", code, text)
	}
}

func TestLandingResolveVerbHoldsAndRefusesWrongCheckoutOrPausedLane(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	b.owners.landing.plainResolve.Git = func(_ string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "show HEAD:metasystem/testing.json":
			return b.contract, nil
		case "diff --name-only --diff-filter=U -z":
			return "", nil
		default:
			t.Fatalf("hold queried %v", args)
			return "", nil
		}
	}
	if code, text := b.run(t, b.root, "resolve"); code != 0 || !strings.Contains(text, "no unresolved paths") {
		t.Fatalf("hold=%d %s", code, text)
	}
	b.owners.landing.plainResolve.Git = func(_ string, args ...string) (string, error) { t.Fatalf("refusal queried %v", args); return "", nil }
	if code, text := b.run(t, t.TempDir(), "resolve"); code == 0 || !strings.Contains(oneSpaced(text), "runs in the registered landing checkout") {
		t.Fatalf("wrong checkout=%d %s", code, text)
	}
	if _, err := lane.SetPause(b.home, "Wido", laneTestNow); err != nil {
		t.Fatal(err)
	}
	if code, text := b.run(t, b.root, "resolve"); code == 0 || !strings.Contains(text, "stopped") {
		t.Fatalf("paused=%d %s", code, text)
	}
}

func TestLandingStopEndsRegenerationAfterPausingLane(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	stopped := 0
	b.owners.landing.stopRegeneration = func(install string) error {
		stopped++
		if install != b.install {
			t.Fatalf("stop install=%s", install)
		}
		if _, paused := lane.ReadPause(b.home); !paused {
			t.Fatal("regeneration stopped before the lane paused")
		}
		return nil
	}
	for call := 1; call <= 2; call++ {
		if code, text := b.run(t, b.root, "stop", "--by", "Wido"); code != 0 || stopped != call {
			t.Fatalf("stop=%d calls=%d want=%d %s", code, stopped, call, text)
		}
	}
	b.owners.landing.stopRegeneration = func(string) error { return fmt.Errorf("cannot signal command") }
	if code, text := b.run(t, b.root, "stop", "--by", "Wido"); code == 0 || !strings.Contains(oneSpaced(text), "the running regeneration could not be stopped") {
		t.Fatalf("stop failure=%d %s", code, text)
	}
	if _, paused := lane.ReadPause(b.home); !paused {
		t.Fatal("stop failure resumed the lane")
	}
}
