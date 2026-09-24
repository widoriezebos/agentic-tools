package goal

import (
	"errors"
	"strings"
	"testing"
)

// repairRepository injects faults at this endpoint without changing the shared fake.
type repairRepository struct {
	Repository
	readErr error
	readTip string
	absent  bool
	casCall func(old, next string) error
	casOld  []string
}

func (r *repairRepository) Accepted() (string, bool, error) {
	if r.readErr != nil {
		return r.readTip, false, r.readErr
	}
	if r.absent {
		return r.readTip, false, nil
	}
	return r.Repository.Accepted()
}

func (r *repairRepository) AcceptedCAS(old, next string) error {
	r.casOld = append(r.casOld, old)
	if r.casCall != nil {
		return r.casCall(old, next)
	}
	return r.Repository.AcceptedCAS(old, next)
}

func repairPublishGoal(t *testing.T, e Endpoint, opid, id string, extra []*GoalFile) PublishResult {
	t.Helper()
	files := vTree(vRoot(), append([]*GoalFile{vGoal(id, StateQueued)}, extra...), nil)
	changes := make([]Change, 0, len(files))
	for path, content := range files {
		changes = append(changes, Change{Path: path, Content: content})
	}
	res, err := Publish(e, PublishRequest{
		Opid: opid, Machine: "mac-a", Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open " + id,
		Mutate: func(string) ([]Change, error) { return changes, nil },
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("publish %s: %+v %v", opid, res, err)
	}
	return res
}

func repairEntry(t *testing.T, root, by, oldTip, newTip string, outcome Outcome) {
	t.Helper()
	entries, err := Entries(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Intent.Verb == "repair-accept-remote" &&
			entry.Intent.Args["by"] == by && entry.Intent.Args["oldTip"] == oldTip &&
			entry.Intent.Args["newTip"] == newTip && entry.Phase == PhaseTerminal &&
			entry.Outcome == outcome && strings.HasPrefix(entry.Opid, "repair-read-") {
			return
		}
	}
	t.Fatalf("repair terminal with human, opid, tips, and outcome %s missing: %+v", outcome, entries)
}

func repairCaptureReleased(t *testing.T, client *fakeGoalRepository, captures, releases int) {
	t.Helper()
	if len(client.captures) != captures+1 || len(client.released) != releases+1 ||
		!strings.HasPrefix(client.released[releases], "read-") {
		t.Fatalf("repair capture was not released: captures=%v released=%v", client.captures, client.released)
	}
}

func TestRepairAcceptsARewoundRemoteUnderAHuman(t *testing.T) {
	t.Parallel()
	e, client := fakeGoalEndpoint(t)
	firstTip := repairPublishGoal(t, e, "op-first", "goal-first", nil).Tip
	if _, err := FetchAdvance(e); err != nil {
		t.Fatal(err)
	}
	laterTip := repairPublishGoal(t, e, "op-second", "goal-second", []*GoalFile{vGoal("goal-first", StateQueued)}).Tip
	if _, err := FetchAdvance(e); err != nil {
		t.Fatal(err)
	}
	// The canonical branch was rewound to an older committed snapshot of this ledger.
	client.store.mu.Lock()
	client.store.canonical = firstTip
	client.store.mu.Unlock()
	if _, err := FetchAdvance(e); err == nil || !strings.Contains(err.Error(), "rewound") {
		t.Fatalf("the rewind must refuse first: %v", err)
	}
	if _, err := RepairAcceptRemote(e, ""); err == nil || !strings.Contains(err.Error(), "--by") {
		t.Fatalf("the repair names its human: %v", err)
	}
	res, err := RepairAcceptRemote(e, "wido")
	if err != nil || !res.Advanced || res.Tip != firstTip {
		t.Fatalf("the attributed repair accepts the remote: %+v %v", res, err)
	}
	if got := acceptedTipForEndpoint(t, e); got != firstTip {
		t.Fatalf("accepted ref: %s vs %s", got, firstTip)
	}
	if client.store.canonical != firstTip {
		t.Fatalf("repair mutated the canonical tip: %s", client.store.canonical)
	}
	repairEntry(t, e.Root, "wido", laterTip, firstTip, OutcomeConfirmed)
	if _, err := FetchAdvance(e); err != nil {
		t.Fatalf("the ordinary advance resumes after the repair: %v", err)
	}
}

func TestRepairStillRefusesAForeignLedger(t *testing.T) {
	t.Parallel()
	a, b := fakeGoalEndpointPair(t)
	mine := repairPublishGoal(t, a, "op-mine", "goal-mine", nil).Tip
	if _, err := FetchAdvance(a); err != nil {
		t.Fatal(err)
	}
	if _, err := FetchAdvance(b); err != nil {
		t.Fatal(err)
	}
	// The next committed snapshot has a different root identity, despite
	// descending from the accepted commit in the shared store.
	foreignRoot := vRoot()
	foreignRoot.Identity = "01J5XEEEEEEEEEEEEEEEEEEEEE"
	foreignFiles := vTree(foreignRoot, []*GoalFile{vGoal("goal-mine", StateQueued), vGoal("theirs", StateQueued)}, nil)
	res, err := Publish(b, PublishRequest{
		Opid: "op-theirs", Machine: "mac-b", Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open theirs",
		Mutate: func(string) ([]Change, error) {
			return []Change{
				{Path: goalsPrefix + "backlog.md", Content: foreignFiles[goalsPrefix+"backlog.md"]},
				{Path: goalsPrefix + "theirs.md", Content: foreignFiles[goalsPrefix+"theirs.md"]},
			}, nil
		},
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("foreign publish: %+v %v", res, err)
	}
	if _, err := RepairAcceptRemote(a, "wido"); err == nil || !strings.Contains(err.Error(), "foreign ledger") {
		t.Fatalf("the repair must still refuse a foreign ledger: %v", err)
	}
	if got := acceptedTipForEndpoint(t, a); got != mine {
		t.Fatalf("foreign repair moved accepted to %s", got)
	}
}

func TestDescendantRevertAcceptsWithThePrefixDiagnosis(t *testing.T) {
	t.Parallel()
	a, b := fakeGoalEndpointPair(t)
	v1 := vGoal("evolving", StateQueued)
	v2 := vGoal("evolving", StateQueued)
	v2.Revision = 2
	v2.History = append(append([]HistoryLine{}, v1.History...), HistoryLine{
		At: "2026-08-20T12:00:00Z", Opid: "01J5X0000000000000000000C0-mac-a-1a2b3c4d",
		Verb: "edit", Actor: "mac-a+lin-1", Targets: []string{"evolving"}, Keep: -1,
	})
	publish := func(opid string, goalBytes []byte) {
		res, err := Publish(a, PublishRequest{
			Opid: opid, Machine: "mac-a", Lineage: "l1",
			Intent: testIntentFor("edit"), Message: "goal step " + opid,
			Mutate: func(string) ([]Change, error) {
				return []Change{{Path: goalsPrefix + "evolving.md", Content: goalBytes}}, nil
			},
		})
		if err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("publish %s: %+v %v", opid, res, err)
		}
	}
	publish("op-v1", RenderFile(v1))
	publish("op-v2", RenderFile(v2))
	if _, err := FetchAdvance(a); err != nil {
		t.Fatal(err)
	}
	// B publishes a descendant whose committed file restores V1's history.
	resB, err := Publish(b, PublishRequest{
		Opid: "op-revert", Machine: "mac-b", Lineage: "l1",
		Intent: testIntentFor("edit"), Message: "goal step op-revert",
		Mutate: func(string) ([]Change, error) {
			return []Change{{Path: goalsPrefix + "evolving.md", Content: RenderFile(v1)}}, nil
		},
	})
	if err != nil || resB.Outcome != OutcomeConfirmed {
		t.Fatalf("B's revert publish: %+v %v", resB, err)
	}
	res, err := FetchAdvance(a)
	if err != nil || !res.Advanced {
		t.Fatalf("a descendant revert restoring an older valid state is accepted: %+v %v", res, err)
	}
	if !strings.Contains(res.Detail, "evolving.md") || !strings.Contains(res.Detail, "prefix") {
		t.Fatalf("the prefix diagnosis is reported with the acceptance: %s", res.Detail)
	}
}

func TestRepairAcceptedReadStates(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		e, client := fakeGoalEndpoint(t)
		fetched := repairPublishGoal(t, e, "op-read-absent", "one", nil).Tip
		client.accepted = ""
		beforeCaptures, beforeReleases := len(client.captures), len(client.released)
		wrapped := &repairRepository{Repository: client, absent: true, readTip: "nonempty absent hint"}
		e.Repository = wrapped
		res, err := RepairAcceptRemote(e, "wido")
		if err != nil || !res.Advanced || res.Tip != fetched || len(wrapped.casOld) != 1 || wrapped.casOld[0] != "" {
			t.Fatalf("absent ref must create with empty-old CAS: %+v old=%v err=%v", res, wrapped.casOld, err)
		}
		if got := client.accepted; got != fetched || client.store.canonical != fetched {
			t.Fatalf("accepted/canonical: %s/%s wanted %s", got, client.store.canonical, fetched)
		}
		repairEntry(t, e.Root, "wido", "", fetched, OutcomeConfirmed)
		repairCaptureReleased(t, client, beforeCaptures, beforeReleases)
	})
	t.Run("unreadable", func(t *testing.T) {
		e, client := fakeGoalEndpoint(t)
		fetched := repairPublishGoal(t, e, "op-read-error", "one", nil).Tip
		accepted := client.accepted
		beforeEntries, err := Entries(e.Root)
		if err != nil {
			t.Fatal(err)
		}
		beforeCaptures, beforeReleases := len(client.captures), len(client.released)
		wrapped := &repairRepository{Repository: client, readTip: accepted, readErr: errors.New("accepted ref corrupt")}
		e.Repository = wrapped
		if _, err := RepairAcceptRemote(e, "wido"); err == nil || !strings.Contains(err.Error(), "accepted ref corrupt") {
			t.Fatalf("unreadable ref must refuse: %v", err)
		}
		afterEntries, err := Entries(e.Root)
		if err != nil {
			t.Fatal(err)
		}
		if len(afterEntries) != len(beforeEntries) || len(wrapped.casOld) != 0 ||
			client.accepted != accepted || client.store.canonical != fetched {
			t.Fatalf("unreadable ref mutated state: entries=%d/%d CAS=%v accepted=%s canonical=%s", len(beforeEntries), len(afterEntries), wrapped.casOld, client.accepted, client.store.canonical)
		}
		repairCaptureReleased(t, client, beforeCaptures, beforeReleases)
	})
}

func TestRepairAcceptedCASLossLeavesAbandonedJournal(t *testing.T) {
	e, client := fakeGoalEndpoint(t)
	fetched := repairPublishGoal(t, e, "op-cas-first", "one", nil).Tip
	oldTip := repairPublishGoal(t, e, "op-cas-second", "two", []*GoalFile{vGoal("one", StateQueued)}).Tip
	competitor, err := client.Build("competitor", oldTip, []Change{{Path: goalsPrefix + "backlog.md", Content: RenderRoot(vRoot())}}, "competitor")
	if err != nil {
		t.Fatal(err)
	}
	// Canonical is rewound; another local actor advances accepted after
	// repair reads oldTip but before its one CAS attempt.
	client.store.mu.Lock()
	client.store.canonical = fetched
	client.store.mu.Unlock()
	beforeCaptures, beforeReleases := len(client.captures), len(client.released)
	wrapped := &repairRepository{Repository: client}
	wrapped.casCall = func(old, next string) error {
		if err := client.AcceptedCAS(old, competitor); err != nil {
			return err
		}
		return client.AcceptedCAS(old, next)
	}
	e.Repository = wrapped
	if _, err := RepairAcceptRemote(e, "wido"); err == nil || !strings.Contains(err.Error(), "stale accepted compare") {
		t.Fatalf("CAS loss must refuse: %v", err)
	}
	if len(wrapped.casOld) != 1 || wrapped.casOld[0] != oldTip || client.accepted != competitor || client.store.canonical != fetched {
		t.Fatalf("CAS loss overwrote accepted or canonical: calls=%v accepted=%s canonical=%s", wrapped.casOld, client.accepted, client.store.canonical)
	}
	repairEntry(t, e.Root, "wido", oldTip, fetched, OutcomeAbandoned)
	repairCaptureReleased(t, client, beforeCaptures, beforeReleases)
}

func TestRepairGitAdapterMovesOnlyAcceptedRef(t *testing.T) {
	t.Parallel()
	origin, a, _ := twoClones(t)
	firstTip := publishGoal(t, a, "op-adapter-first", "first", nil).Tip
	oldTip := publishGoal(t, a, "op-adapter-second", "second", []*GoalFile{vGoal("first", StateQueued)}).Tip
	mainBefore := mustGit(t, a, "rev-parse", "refs/heads/main")
	mustGit(t, origin, "update-ref", "refs/heads/main", firstTip)
	canonicalBefore := mustGit(t, origin, "rev-parse", "refs/heads/main")
	res, err := RepairAcceptRemote(endpointFor(a), "wido")
	if err != nil || !res.Advanced || res.Tip != firstTip {
		t.Fatalf("Git adapter repair: %+v %v", res, err)
	}
	if got := mustGit(t, a, "rev-parse", AcceptedRef); got != firstTip {
		t.Fatalf("accepted ref: %s vs %s (old %s)", got, firstTip, oldTip)
	}
	if got := mustGit(t, origin, "rev-parse", "refs/heads/main"); got != canonicalBefore {
		t.Fatalf("canonical branch moved: %s vs %s", got, canonicalBefore)
	}
	if got := mustGit(t, a, "rev-parse", "refs/heads/main"); got != mainBefore {
		t.Fatalf("local main moved: %s vs %s", got, mainBefore)
	}
}
