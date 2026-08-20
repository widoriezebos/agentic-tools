package goal

import (
	"strings"
	"testing"
)

func TestRepairAcceptsARewoundRemoteUnderAHuman(t *testing.T) {
	origin, a, _ := twoClones(t)
	publishGoal(t, a, "op-first", "goal-first", nil)
	if _, err := FetchAdvance(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	firstTip := mustGit(t, a, "rev-parse", AcceptedRef)

	// A second publish advances further; branch surgery then rewinds
	// the canonical branch back to the FIRST ledger tip (still this
	// ledger, older state).
	publishGoal(t, a, "op-second", "goal-second", []*GoalFile{vGoal("goal-first", StateQueued)})
	if _, err := FetchAdvance(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	mustGit(t, origin, "update-ref", "refs/heads/main", firstTip)

	// The ordinary advance refuses; the repair without a human
	// refuses; the attributed repair accepts.
	if _, err := FetchAdvance(endpointFor(a)); err == nil || !strings.Contains(err.Error(), "rewound") {
		t.Fatalf("the rewind must refuse first: %v", err)
	}
	if _, err := RepairAcceptRemote(endpointFor(a), ""); err == nil || !strings.Contains(err.Error(), "--by") {
		t.Fatalf("the repair names its human: %v", err)
	}
	res, err := RepairAcceptRemote(endpointFor(a), "wido")
	if err != nil || !res.Advanced {
		t.Fatalf("the attributed repair accepts the remote: %+v %v", res, err)
	}
	// The LOCAL postcondition: the ref equals the target.
	if got := mustGit(t, a, "rev-parse", AcceptedRef); got != firstTip {
		t.Fatalf("the accepted ref equals the repaired target: %s vs %s", got, firstTip)
	}
	// The journal carries the act under the human's name.
	entries, err := Entries(a)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if e.Intent.Verb == "repair-accept-remote" && e.Intent.Args["by"] == "wido" &&
			e.Phase == PhaseTerminal && e.Outcome == OutcomeConfirmed {
			found = true
		}
	}
	if !found {
		t.Fatalf("the repair is journaled under its human: %+v", entries)
	}
	// Recovery resolved: the ordinary advance works again.
	if _, err := FetchAdvance(endpointFor(a)); err != nil {
		t.Fatalf("the ordinary advance resumes after the repair: %v", err)
	}
}

func TestRepairStillRefusesAForeignLedger(t *testing.T) {
	_, a, _ := twoClones(t)
	publishGoal(t, a, "op-mine", "goal-mine", nil)
	if _, err := FetchAdvance(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	foreignOrigin, c, _ := twoClones(t)
	foreignRoot := vRoot()
	foreignRoot.Identity = "01J5XEEEEEEEEEEEEEEEEEEEEE"
	files := vTree(foreignRoot, []*GoalFile{vGoal("theirs", StateQueued)}, nil)
	var changes []Change
	for p, content := range files {
		changes = append(changes, Change{Path: p, Content: content})
	}
	if res, err := Publish(endpointFor(c), PublishRequest{
		Opid: "op-theirs", Machine: "mac-c", Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open theirs",
		Mutate: func(tip string) ([]Change, error) { return changes, nil },
	}); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("foreign publish: %+v %v", res, err)
	}
	mustGit(t, a, "remote", "set-url", "origin", foreignOrigin)
	// The repair waives descent, never identity.
	if _, err := RepairAcceptRemote(endpointFor(a), "wido"); err == nil ||
		!strings.Contains(err.Error(), "foreign ledger") {
		t.Fatalf("the repair must still refuse a foreign ledger: %v", err)
	}
}

func TestDescendantRevertAcceptsWithThePrefixDiagnosis(t *testing.T) {
	_, a, b := twoClones(t)

	// Version 1 of the goal file, then version 2 with one more
	// History line (an edit's lawful footprint).
	v1 := vGoal("evolving", StateQueued)
	v2 := vGoal("evolving", StateQueued)
	v2.Revision = 2
	v2.History = append(append([]HistoryLine{}, v1.History...), HistoryLine{
		At: "2026-08-20T12:00:00Z", Opid: "01J5X0000000000000000000C0-mac-a-1a2b3c4d",
		Verb: "edit", Actor: "mac-a+lin-1", Targets: []string{"evolving"}, Keep: -1,
	})
	rootFiles := vTree(vRoot(), nil, nil)

	publish := func(opid string, goalBytes []byte, withRoot bool) {
		changes := []Change{{Path: goalsPrefix + "evolving.md", Content: goalBytes}}
		if withRoot {
			changes = append(changes, Change{Path: goalsPrefix + "backlog.md", Content: rootFiles[goalsPrefix+"backlog.md"]})
		}
		res, err := Publish(endpointFor(a), PublishRequest{
			Opid: opid, Machine: "mac-a", Lineage: "l1",
			Intent: testIntentFor("edit"), Message: "goal step " + opid,
			Mutate: func(tip string) ([]Change, error) { return changes, nil },
		})
		if err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("publish %s: %+v %v", opid, res, err)
		}
	}
	publish("op-v1", RenderFile(v1), true)
	publish("op-v2", RenderFile(v2), false)
	if _, err := FetchAdvance(endpointFor(a)); err != nil {
		t.Fatal(err)
	}
	// The DESCENDANT revert comes from the OTHER clone: the
	// diagnosis is a read-side event, and the publisher's own
	// confirmation already advances its accepted ref past it.
	resB, err := Publish(endpointFor(b), PublishRequest{
		Opid: "op-revert", Machine: "mac-b", Lineage: "l1",
		Intent: testIntentFor("edit"), Message: "goal step op-revert",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{{Path: goalsPrefix + "evolving.md", Content: RenderFile(v1)}}, nil
		},
	})
	if err != nil || resB.Outcome != OutcomeConfirmed {
		t.Fatalf("B's revert publish: %+v %v", resB, err)
	}

	res, err := FetchAdvance(endpointFor(a))
	if err != nil || !res.Advanced {
		t.Fatalf("a descendant revert restoring an older valid state is ACCEPTED (R8-11): %+v %v", res, err)
	}
	if !strings.Contains(res.Detail, "evolving.md") || !strings.Contains(res.Detail, "prefix") {
		t.Fatalf("the prefix diagnosis is REPORTED with the acceptance: %s", res.Detail)
	}
}
