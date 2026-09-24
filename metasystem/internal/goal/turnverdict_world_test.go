package goal

import (
	"strings"
	"testing"
)

// The end-of-turn verdict on a converted checkout: a claim-only world retains
// the existing next-step block-once contract. Liveness governs whether a
// claim may suppress separate claimable backlog, which is covered by the idle
// invariant fixtures.
func TestTurnVerdictConvertedClaimHasTheFloor(t *testing.T) {
	t.Parallel()
	fixture, _, _ := fakeServingFixture(t, "bed-m1", map[string]*GoalFile{
		"ship-it": {
			Id: "ship-it", State: "claimed", Intent: "Ship the whole thing", Origin: "main",
			NextStep: "Land it in pieces.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
			Claimed: &ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
		},
	})
	store := *fixture
	options := TurnVerdictOptions{SeatActor: Actor{Machine: "bed-m1", Lineage: "coordinator"}}
	first, err := store.TurnVerdict(ScanResult{}, "world-session", "", "main-1", options)
	if err != nil {
		t.Fatal(err)
	}
	if !first.ShouldBlock || !strings.Contains(first.Display, "Land it in pieces") {
		t.Fatalf("the claimed goal must block once with its next step: %+v", first)
	}
	second, err := store.TurnVerdict(ScanResult{}, "world-session", "", "main-1", options)
	if err != nil {
		t.Fatal(err)
	}
	if second.ShouldBlock || !strings.Contains(second.Display, "ship-it") {
		t.Fatalf("the repeat surfaces as display, not a block: %+v", second)
	}
	t.Run("bound source rejects a different state root", func(t *testing.T) {
		mismatched := store
		mismatched.Root = t.TempDir()
		verdict, err := mismatched.TurnVerdict(ScanResult{}, "wrong-root", "", "main-1")
		if err != nil {
			t.Fatal(err)
		}
		if verdict.Class != "infrastructure" || verdict.Component != "state-root" || verdict.ShouldBlock || !strings.Contains(verdict.Display, "does not match resolved state root") {
			t.Fatalf("mismatched bound source escaped state-root validation: %+v", verdict)
		}
	})
}

// A converted queue without valid structured budgets is visible in backlog
// order but is not the claimable backlog this invariant blocks on.
func TestTurnVerdictConvertedBudgetlessQueueIsQuiet(t *testing.T) {
	t.Parallel()
	fixture, _, _ := fakeServingFixture(t, "bed-m1", map[string]*GoalFile{
		"older": {
			Id: "older", State: "queued", Intent: "First in line", Origin: "main",
			NextStep: "Work this first.", OpenedAt: "2026-08-20T00:00:00Z", Revision: 1,
		},
		"newer": {
			Id: "newer", State: "queued", Intent: "Second", Origin: "main",
			NextStep: "Work this later.", OpenedAt: "2026-08-22T00:00:00Z", Revision: 1,
		},
	})
	store := *fixture
	v, err := store.TurnVerdict(ScanResult{}, "queue-session", "", "main-1")
	if err != nil {
		t.Fatal(err)
	}
	if v.ShouldBlock || !strings.Contains(v.Display, "newer") {
		t.Fatalf("the ordered budgetless head must stay visible without blocking: %+v", v)
	}
}

// A fresh goal-free declaration on the root record is the all-clear.
func TestTurnVerdictConvertedFreshFreeIsAllClear(t *testing.T) {
	t.Parallel()
	fixture, client, _ := fakeServingFixture(t, "bed-m1", nil)
	store := *fixture
	scan, err := ScanDigest(store.Root)
	if err != nil {
		t.Fatal(err)
	}
	rewriteBedRoot(t, client, scan)
	v, err := store.TurnVerdict(ScanResult{}, "free-session", "", "main-1")
	if err != nil {
		t.Fatal(err)
	}
	if v.ShouldBlock || !strings.Contains(v.Display, "NOTHING LEFT TO WORK ON") {
		t.Fatalf("a fresh declaration is the all-clear: %+v", v)
	}
}

// rewriteBedRoot advances the accepted tree with a fresh Goal-free root.
func rewriteBedRoot(t *testing.T, client *fakeGoalRepository, digest string) {
	t.Helper()
	parent, present, err := client.Accepted()
	if err != nil || !present {
		t.Fatalf("read accepted root: tip=%q present=%t err=%v", parent, present, err)
	}
	files, err := client.Files(parent, goalsPrefix+"backlog.md")
	if err != nil {
		t.Fatal(err)
	}
	record, problems := ParseRoot(files[goalsPrefix+"backlog.md"])
	if len(problems) != 0 {
		t.Fatalf("parse accepted root: %v", problems)
	}
	record.Revision++
	record.Free = &FreeRecord{Declared: "2026-08-23T00:00:00Z", Origin: "human", Digest: digest}
	const opid = "01ARZ3NDEKTSV4RRFFQ69G5FAZ-human-00000003"
	record.History = append(record.History, HistoryLine{
		At: "2026-08-23T00:00:00Z", Opid: opid,
		Verb: "declare-free", Actor: "human:wido", Keep: -1,
	})
	commit, err := client.Build(opid, parent, []Change{{Path: goalsPrefix + "backlog.md", Content: RenderRoot(record)}}, "root rewrite")
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := client.Publish(parent, commit); err != nil || outcome != CASLanded {
		t.Fatalf("publish root rewrite: outcome=%v err=%v", outcome, err)
	}
	if err := client.AcceptedCAS(parent, commit); err != nil {
		t.Fatal(err)
	}
	accepted, present, err := client.Accepted()
	if err != nil || !present || accepted != commit || accepted == parent {
		t.Fatalf("root rewrite did not advance the accepted tip: before=%q after=%q commit=%q present=%t err=%v", parent, accepted, commit, present, err)
	}
	acceptedFiles, err := client.Files(accepted, goalsPrefix+"backlog.md")
	if err != nil {
		t.Fatal(err)
	}
	revised, problems := ParseRoot(acceptedFiles[goalsPrefix+"backlog.md"])
	if len(problems) != 0 || revised.Free == nil || revised.Free.Digest != digest || len(revised.History) != len(record.History) || revised.History[len(revised.History)-1].Verb != "declare-free" {
		t.Fatalf("accepted root lost its Goal-free declaration or history: root=%+v problems=%v", revised, problems)
	}
}
