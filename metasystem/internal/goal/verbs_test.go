package goal

import (
	"strings"
	"testing"
	"time"
)

func verbReq(root, ulid, machine string) VerbRequest {
	return VerbRequest{
		Endpoint: endpointFor(root),
		Actor:    Actor{Machine: machine, Lineage: "lin-1"},
		Ulid:     ulid,
		Now:      time.Date(2026, 8, 20, 22, 0, 0, 0, time.UTC),
	}
}

// seedLedger publishes a root record so the verbs act on a lawful
// tree (migration owns real bootstrap; fixtures seed directly).
func seedLedger(t *testing.T, root string) {
	t.Helper()
	files := vTree(vRoot(), nil, nil)
	res, err := Publish(endpointFor(root), PublishRequest{
		Opid: "op-seed-" + root[len(root)-6:], Machine: "mac-seed", Lineage: "l1",
		Intent: testIntentFor("migrate"), Message: "seed root record",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{{Path: goalsPrefix + "backlog.md", Content: files[goalsPrefix+"backlog.md"]}}, nil
		},
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("seed: %+v %v", res, err)
	}
}

func TestOpenClaimDoneLifecycle(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)

	res, err := Open(verbReq(a, "01J5X0000000000000000000D0", "mac-a"), "build-it", "Build the thing.", "main", "Start with the walls.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	res, err = Claim(verbReq(a, "01J5X0000000000000000000D1", "mac-a"), "build-it")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	res, err = Done(verbReq(a, "01J5X0000000000000000000D2", "mac-a"), "build-it", "Built and verified.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("done: %+v %v", res, err)
	}

	// The archive carries the record with the full history; the live
	// set is empty; every touched write bumped Revision exactly once.
	t2, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if len(t2.Live) != 0 {
		t.Fatalf("done moves the file out of the live set: %v", sortedGoalIds(t2.Live))
	}
	archived, ok := t2.Done["build-it"]
	if !ok {
		t.Fatal("done lands in the archive")
	}
	if archived.Conclude != "Built and verified." || archived.Revision != 3 || len(archived.History) != 3 {
		t.Fatalf("the archived record carries the whole lawful history: rev=%d hist=%d conclude=%q",
			archived.Revision, len(archived.History), archived.Conclude)
	}
	verbs := []string{archived.History[0].Verb, archived.History[1].Verb, archived.History[2].Verb}
	if strings.Join(verbs, ",") != "open,claim,done" {
		t.Fatalf("history names the verbs in order: %v", verbs)
	}
}

func TestSameGoalClaimRaceOneWinnerNamed(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X0000000000000000000D3", "mac-a"), "contested", "One goal, two machines.", "main", "Race."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}

	// A claims first; B's claim rebuilds on the new tip, reads the
	// standing claim, and classifies the loss naming the winner.
	resA, err := Claim(verbReq(a, "01J5X0000000000000000000D4", "mac-a"), "contested")
	if err != nil || resA.Outcome != OutcomeConfirmed {
		t.Fatalf("A claims: %+v %v", resA, err)
	}
	resB, err := Claim(verbReq(b, "01J5X0000000000000000000D5", "mac-b"), "contested")
	if err != nil || resB.Outcome != OutcomeLost {
		t.Fatalf("B loses the claim race: %+v %v", resB, err)
	}
	if !strings.Contains(resB.Detail, "mac-a") {
		t.Fatalf("the loser names the winner's operation: %s", resB.Detail)
	}
}

func TestClaimRefusalsAreNamed(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X0000000000000000000D6", "mac-a"), "dep", "The blocker.", "main", "First."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open dep: %+v %v", res, err)
	}
	// A goal blocked by an open dependency refuses the claim by name.
	res, err := Open(verbReq(a, "01J5X0000000000000000000D7", "mac-a"), "eager", "Wants to run early.", "main", "Second.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open eager: %+v %v", res, err)
	}
	// Wire the edge by hand for the fixture (edit lands next slice):
	// publish an updated file carrying BlockedBy.
	t2, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	eager := t2.Live["eager"]
	eager.Blocked = []string{"dep"}
	eager.Revision++
	if resw, errw := Publish(endpointFor(a), PublishRequest{
		Opid: "op-wire-edge", Machine: "mac-a", Lineage: "l1",
		Intent: testIntentFor("edit"), Message: "wire edge",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{{Path: livePath("eager"), Content: RenderFile(eager)}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(a, commit) },
	}); errw != nil || resw.Outcome != OutcomeConfirmed {
		t.Fatalf("wire edge: %+v %v", resw, errw)
	}

	// Refusals are REJECTED results, journaled by name — not Go
	// errors: the engine's contract for a definite rejection.
	res2, err := Claim(verbReq(a, "01J5X0000000000000000000D8", "mac-a"), "eager")
	if err != nil || res2.Outcome != OutcomeRejected || !strings.Contains(res2.Detail, "blocked by dep") {
		t.Fatalf("claiming past an open blocker rejects by name: %+v %v", res2, err)
	}
	res3, err := Claim(verbReq(a, "01J5X0000000000000000000DE", "mac-a"), "ghost")
	if err != nil || res3.Outcome != OutcomeRejected || !strings.Contains(res3.Detail, "ghost") {
		t.Fatalf("claiming a goal that does not exist rejects naming it: %+v %v", res3, err)
	}
}

func TestReleaseIsOwnerOrHuman(t *testing.T) {
	_, a, b := twoClones(t)
	seedLedger(t, a)
	if res, err := Open(verbReq(a, "01J5X0000000000000000000D9", "mac-a"), "held", "Held by A.", "main", "Work."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	if res, err := Claim(verbReq(a, "01J5X0000000000000000000DA", "mac-a"), "held"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	// B's agent cannot release A's claim: a rejection by name.
	resF, err := Release(verbReq(b, "01J5X0000000000000000000DB", "mac-b"), "held")
	if err != nil || resF.Outcome != OutcomeRejected || !strings.Contains(resF.Detail, "human act") {
		t.Fatalf("a foreign release rejects as a human act: %+v %v", resF, err)
	}
	// B under a human can.
	humanReq := verbReq(b, "01J5X0000000000000000000DC", "mac-b")
	humanReq.Actor.Human = "wido"
	res, err := Release(humanReq, "held")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("a human-directed foreign release proceeds: %+v %v", res, err)
	}
	t2, err := loadTree(b, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	released := t2.Live["held"]
	if released.State != StateQueued || released.Claimed != nil {
		t.Fatalf("the release returns the goal to the queue: %+v", released)
	}
	last := released.History[len(released.History)-1]
	if last.Actor != "human:wido" {
		t.Fatalf("the history attributes the HUMAN authority: %+v", last)
	}
}

func TestOpenClearsGoalFreeInTheSameCommit(t *testing.T) {
	_, a, _ := twoClones(t)
	// Seed a root record WITH a Goal-free declaration.
	root := vRoot()
	root.Free = &FreeRecord{Declared: "2026-08-20T11:00:00Z", Origin: "main", Digest: strings.Repeat("cd", 32)}
	files := vTree(root, nil, nil)
	if res, err := Publish(endpointFor(a), PublishRequest{
		Opid: "op-seed-free", Machine: "mac-seed", Lineage: "l1",
		Intent: testIntentFor("migrate"), Message: "seed free root",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{{Path: goalsPrefix + "backlog.md", Content: files[goalsPrefix+"backlog.md"]}}, nil
		},
	}); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("seed: %+v %v", res, err)
	}

	res, err := Open(verbReq(a, "01J5X0000000000000000000DD", "mac-a"), "revival", "Work resumes.", "main", "Go.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open under Goal-free: %+v %v", res, err)
	}
	t2, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if t2.Root.Free != nil {
		t.Fatal("open clears the Goal-free declaration in the same commit")
	}
	if _, ok := t2.Live["revival"]; !ok {
		t.Fatal("the opened goal is live")
	}
}
