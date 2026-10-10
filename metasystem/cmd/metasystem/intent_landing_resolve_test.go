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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
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
	// The fixture declares its lane policy without consulting host registrations.
	b.owners.policies = config.PolicyReaders{
		Registry: func(string) (config.PolicyRegistry, error) { return config.PolicyRegistry{Lane: b.root}, nil },
		ConfPath: func(string) (string, error) { return filepath.Join(b.install, "metasystem.conf"), nil },
		Helm:     func(string) helm.State { return helm.State{} },
	}
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

// A source conflict returns its own cause together with the conflict details.
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
	if result.Data.Conflict == nil || result.Data.Conflict.Main != "main-sha" || len(result.Data.Conflict.Paths) != 1 || result.Data.Conflict.Paths[0].Resolution != "keep both, main's lines then the goal's" || result.Data.Entry == nil || result.Data.Entry.State != plain.StateReturned || result.Data.Entry.Cause == nil || result.Data.Entry.Cause.Kind != "own" || result.Data.Entry.Cause.Goal != "goal" || result.Data.Entry.Cause.SHA != "goal-sha" || !reflect.DeepEqual(writes, [][]string{{"merge", "--abort"}}) {
		t.Fatalf("returned conflict=%+v writes=%v", result.Data, writes)
	}
}

func TestLandingResolveAsSeatRecordsConflictWithoutAborting(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	if _, _, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "goal-sha"}); err != nil {
		t.Fatal(err)
	}
	b.sourceConflictGit(t, func() { t.Fatal("taking the conflict aborted before trying") })
	for range 2 {
		code, output := b.run(t, b.root, "resolve", "--as-seat", "--json")
		var result struct {
			Outcome string
			Data    plain.ResolveOutcome
		}
		if code != 0 || json.Unmarshal([]byte(output), &result) != nil || result.Data.Outcome != "resolving" || result.Data.Entry != nil {
			t.Fatalf("exit=%d output=%s", code, output)
		}
	}
	fix, err := plain.ReadFix(b.install)
	if err != nil || fix == nil || fix.State != "resolving" || len(fix.Paths) != 1 || fix.Paths[0].Path != "metasystem/src/list.go" {
		t.Fatalf("fix=%+v err=%v", fix, err)
	}
	entry, _, err := plain.Latest(b.install, "goal")
	if err != nil || entry.State != plain.StateWaiting {
		t.Fatalf("queue=%+v %v", entry, err)
	}
	git := b.owners.landing.plainResolve.Git
	missing := exec.Command("/bin/sh", "-c", "exit 128").Run()
	b.owners.landing.plainResolve.Git = func(dir string, args ...string) (string, error) {
		if strings.Join(args, " ") == "rev-parse --verify MERGE_HEAD^{commit}" {
			return "", missing
		}
		return git(dir, args...)
	}
	code, output := b.run(t, b.root, "resolve", "--as-seat", "--json")
	var abandoned struct {
		Outcome, Summary string
		Data             plain.ResolveOutcome
	}
	if code != 0 || json.Unmarshal([]byte(output), &abandoned) != nil || abandoned.Outcome != intentUnchanged || abandoned.Data.Outcome != "abandoned" || !strings.Contains(abandoned.Summary, "resolution was abandoned") {
		t.Fatalf("missing merge exit=%d output=%s", code, output)
	}
	if active, err := plain.ReadFix(b.install); err != nil || active != nil {
		t.Fatalf("closed resolution remains active: %+v %v", active, err)
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
		case "rev-parse --verify refs/remotes/origin/main^{commit}", "rev-parse --verify HEAD^{commit}":
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

func TestLandingResolveVerbReturnsGeneratedPathsWithoutRunningCommands(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	if _, _, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "goal-sha"}); err != nil {
		t.Fatal(err)
	}
	aborted := false
	b.sourceConflictGit(t, func() { aborted = true })
	git := b.owners.landing.plainResolve.Git
	b.owners.landing.plainResolve.Git = func(dir string, args ...string) (string, error) {
		if strings.Join(args, " ") == "diff --name-only --diff-filter=U -z" {
			return "metasystem/out/result\x00", nil
		}
		return git(dir, args...)
	}
	b.owners.landing.plainResolve.Run = func([]string, string, *os.File, func(int64) error) error {
		t.Fatal("generator ran in the lane")
		return nil
	}
	begun := filepath.Join(plain.Dir(b.install), "resolve-begun.json")
	if err := os.WriteFile(begun, []byte("obsolete"), 0600); err != nil {
		t.Fatal(err)
	}
	code, text := b.run(t, b.root, "resolve")
	entry, ok, err := plain.Latest(b.install, "goal")
	if code != 0 || err != nil || !ok || entry.State != plain.StateReturned || !aborted || entry.Cause.Kind != "own" || entry.Conflict == nil || !strings.Contains(oneSpaced(text), "metasystem/out/result (generated)") || !strings.Contains(oneSpaced(text), "work rebase goal") {
		t.Fatalf("return=%d %s queue=%+v err=%v", code, text, entry, err)
	}
	if _, err := os.Stat(filepath.Join(plain.Dir(b.install), "regenerate.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("return wrote regeneration: %v", err)
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

// prepareBatch gives proof fixtures the same recorded selection that the
// keeper prepares before an agent assembles goals. Proof and replay assertions
// remain about the selected goals, not an unrecorded candidate.
func (b *resolveVerbFixture) prepareBatch(t *testing.T) {
	t.Helper()
	selected, err := plain.ReadBatch(b.install)
	if err != nil {
		t.Fatal(err)
	}
	if selected != nil {
		return
	}
	record, present, err := lane.Read(b.home)
	if err != nil || !present {
		t.Fatalf("fixture registration: %v %v", present, err)
	}
	if _, err := plain.SelectBatch(b.install, b.root, record, b.owners.landing.plainProve); err != nil {
		t.Fatalf("prepare fixture selection: %v", err)
	}
}
