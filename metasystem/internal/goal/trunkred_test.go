package goal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
)

func TestTrunkRedBranchReferenceRoundTripsAndValidates(t *testing.T) {
	t.Parallel()
	branch := TrunkRedBranch{Name: "fix/red", Commit: "abc123", State: TrunkRedBranchOpen}
	data, err := json.Marshal(branch)
	if err != nil || string(data) != `{"name":"fix/red","commit":"abc123","state":"open"}` {
		t.Fatalf("marshal branch: %s, %v", data, err)
	}
	var decoded TrunkRedBranch
	if err := json.Unmarshal(data, &decoded); err != nil || decoded != branch || decoded.Validate() != nil {
		t.Fatalf("round trip: %+v, %v", decoded, err)
	}
	for _, invalid := range []TrunkRedBranch{{Commit: "abc"}, {State: TrunkRedBranchOpen}, {Name: "fix", Commit: "abc"}, {Name: "fix", State: "unknown"}} {
		if invalid.Validate() == nil {
			t.Fatalf("accepted invalid branch %+v", invalid)
		}
	}
	if err := (TrunkRedBranch{}).Validate(); err != nil {
		t.Fatalf("empty branch: %v", err)
	}
}
func TestTrunkRedVerdictSplitsByOwnerMachine(t *testing.T) {
	t.Parallel()
	closed := &TrunkRedClosure{At: "closed"}
	tree := &TreeGoals{Live: map[string]*GoalFile{}, TrunkRed: []TrunkRedEntry{
		{ID: "mine", Owner: TrunkRedOwner{Machine: "m1"}},
		{ID: "theirs", Owner: TrunkRedOwner{Machine: "m2"}},
		{ID: "closed", Owner: TrunkRedOwner{Machine: "m1"}, Closed: closed},
	}}
	verdict, err := Next(Projection{Root: t.TempDir(), Tree: tree}, "m1")
	if err != nil || len(verdict.TrunkRedOwned) != 1 || verdict.TrunkRedOwned[0].ID != "mine" || len(verdict.TrunkRedElsewhere) != 1 || verdict.TrunkRedElsewhere[0].ID != "theirs" {
		t.Fatalf("split: %+v, %v", verdict, err)
	}
}
func TestTrunkRedDoesNotChangeSelection(t *testing.T) {
	t.Parallel()
	frontier := NextVerdict{Ready: []string{"ready"}}
	want := SelectNext(frontier)
	frontier.TrunkRedOwned = []TrunkRedEntry{{ID: "red"}}
	if got := SelectNext(frontier); got != want {
		t.Fatalf("selection changed from %+v to %+v", want, got)
	}
}

func testTrunkRedEntry(id, identity, opened string) TrunkRedEntry {
	opid := Opid("01J5X0000000000000000000R0", "mac-a", "lin-1")
	return TrunkRedEntry{
		ID: id, Identity: identity, Group: "fast", Status: "failed", Failures: []TrunkRedFailure{},
		Sightings: []TrunkRedSighting{{Attempt: "attempt-1", Batch: "batch-1", BaseCommit: "base-1", SeenAt: opened, Opid: opid}},
		Owner:     TrunkRedOwner{Machine: "mac-a", Since: opened, How: "joiner"}, Holds: []string{"batch-1"}, Opened: opened,
	}
}

func TestTrunkRedRegisterParsesRendersAndJoinsEveryReader(t *testing.T) {
	t.Parallel()
	early := testTrunkRedEntry("tr-fast-a", "tr-fast-a", "2026-09-17T01:00:00Z")
	late := testTrunkRedEntry("tr-fast-b", "tr-fast-b", "2026-09-17T02:00:00Z")
	rendered := RenderTrunkRed([]TrunkRedEntry{late, early})
	entries, problems := ParseTrunkRed(rendered)
	if len(problems) != 0 || len(entries) != 2 || entries[0].ID != early.ID || entries[1].ID != late.ID || rendered[len(rendered)-1] != '\n' {
		t.Fatalf("round trip = entries:%+v problems:%v rendered:%s", entries, problems, rendered)
	}
	files := vTree(vRoot(), nil, nil)
	files[trunkRedPath] = rendered
	tree, treeProblems := ParseTreeFiles(files)
	if len(treeProblems) != 0 || len(tree.TrunkRed) != 2 {
		t.Fatalf("tree reader = %+v problems=%v", tree, treeProblems)
	}

	root := soloLedgerRepo(t)
	published := publishTrunkRed(t, root, []TrunkRedEntry{late, early})
	mustGit(t, root, "reset", "--hard", published.Tip)
	snapshot, err := CaptureSnapshot(root)
	if err != nil || string(snapshot.Files[trunkRedPath]) != string(rendered) {
		t.Fatalf("snapshot = %+v err=%v", snapshot, err)
	}

	invalid := early
	invalid.Owner.By = "a human on a machine-owned entry"
	if _, problems := ParseTrunkRed(RenderTrunkRed([]TrunkRedEntry{invalid})); len(problems) == 0 || !strings.Contains(string(problems[0]), trunkRedPath) {
		t.Fatalf("invalid owner was accepted: %v", problems)
	}
	invalid = early
	invalid.FixBranch = TrunkRedBranch{Name: "fix/red", State: TrunkRedBranchOpen}
	if _, problems := ParseTrunkRed(RenderTrunkRed([]TrunkRedEntry{invalid})); len(problems) == 0 {
		t.Fatal("a named branch without a commit was accepted")
	}
	duplicate := early
	duplicate.ID = early.ID + "-2"
	if _, problems := ParseTrunkRed(RenderTrunkRed([]TrunkRedEntry{early, duplicate})); len(problems) == 0 {
		t.Fatal("two open entries with one identity were accepted")
	}
}

func trunkRedRecordFixture(identity, batch, attempt, base, seen string) TrunkRedRecordArgs {
	return TrunkRedRecordArgs{Batch: batch, Attempt: attempt, BaseCommit: base, BaseTree: "tree-" + base, SeenAt: seen,
		OwnerMachine: "mac-a", Groups: []TrunkRedRecordGroup{{Identity: identity, Group: "fast", Status: "failed", Failures: []TrunkRedFailure{}}}}
}

func TestTrunkRedMutationBoundariesRejectInvalidAuthorityAndShape(t *testing.T) {
	t.Parallel()
	root := soloLedgerRepo(t)
	request := trunkRedVerbReq(root, "01J5X0000000000000000000Q1", "mac-a")
	tip := mustGit(t, root, "rev-parse", AcceptedRef)
	for name, args := range map[string]TrunkRedRecordArgs{
		"empty batch":  trunkRedRecordFixture("tr-fast-empty-batch", "", "attempt", "base", request.stamp()),
		"empty groups": {Batch: "batch", Attempt: "attempt", BaseCommit: "base", BaseTree: "tree", SeenAt: request.stamp()},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := RecordTrunkRed(request, args); err == nil {
				t.Fatalf("%s record was accepted", name)
			}
			if got := mustGit(t, root, "rev-parse", AcceptedRef); got != tip {
				t.Fatalf("%s record moved accepted tip from %s to %s", name, tip, got)
			}
		})
	}

	identity := "tr-fast-brainfence"
	result, err := RecordTrunkRed(request, trunkRedRecordFixture(identity, "batch", "attempt", "base", request.stamp()))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("record fixture: %+v %v", result, err)
	}
	record := brain.Record{Schema: brain.Schema, Ledger: ExistingLedgerIdentity(root), Machine: "brain", DeclaredBy: "Wido", DeclaredAt: "2026-09-17T00:00:00Z"}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(brain.Path(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brain.Path(root), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	before := mustGit(t, root, "rev-parse", AcceptedRef)
	own := trunkRedVerbReq(root, "01J5X0000000000000000000Q2", "mac-a")
	if _, err := OwnTrunkRed(own, TrunkRedOwnArgs{Entry: identity, Goal: "solo-goal"}); err == nil || !strings.Contains(err.Error(), "act trunk-red own is fenced") {
		t.Fatalf("brain-owned trunk-red mutation error=%v", err)
	}
	if after := mustGit(t, root, "rev-parse", AcceptedRef); after != before {
		t.Fatalf("brain fence moved accepted tip from %s to %s", before, after)
	}
}

func TestTrunkRedRecordOwnClearAndCloseTransactions(t *testing.T) {
	t.Parallel()
	root := soloLedgerRepo(t)
	identity := "tr-fast-0123456789ab"
	first := trunkRedVerbReq(root, "01J5X0000000000000000000R1", "mac-a")
	first.Now = time.Date(2026, 9, 17, 1, 0, 0, 0, time.UTC)
	result, err := RecordTrunkRed(first, trunkRedRecordFixture(identity, "batch-1", "attempt-1", "base-1", first.stamp()))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("first record: %+v %v", result, err)
	}
	replay, err := RecordTrunkRed(first, trunkRedRecordFixture(identity, "batch-1", "attempt-1", "base-1", first.stamp()))
	if err != nil || replay.Outcome != OutcomeConfirmed || replay.Detail != "idempotent" {
		t.Fatalf("record replay: %+v %v", replay, err)
	}
	second := trunkRedVerbReq(root, "01J5X0000000000000000000R2", "mac-a")
	second.Now = first.Now.Add(time.Hour)
	secondArgs := trunkRedRecordFixture(identity, "batch-2", "attempt-2", "base-2", second.stamp())
	secondArgs.Groups[0].Status = "not-run"
	secondArgs.Groups[0].NotRunReason = "runner unavailable"
	result, err = RecordTrunkRed(second, secondArgs)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("second sighting: %+v %v", result, err)
	}
	projected, err := ProjectAt(root, result.Tip)
	entry := projected.Tree.TrunkRed[0]
	if err != nil || entry.ID != identity || len(entry.Sightings) != 2 || len(entry.Holds) != 2 || entry.Status != "not-run" || entry.Owner.How != "joiner" {
		t.Fatalf("merged entry: %+v %v", entry, err)
	}

	clear := trunkRedVerbReq(root, "01J5X0000000000000000000R3", "mac-a")
	clear.Now = second.Now.Add(time.Hour)
	result, err = ClearTrunkRed(clear, TrunkRedClearArgs{Entry: identity, Attempt: "green-1", BaseCommit: "base-3", BaseTree: "tree-3", Group: "fast", ExpectedEntry: entry})
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("clear: %+v %v", result, err)
	}
	third := trunkRedVerbReq(root, "01J5X0000000000000000000R4", "mac-a")
	third.Now = clear.Now.Add(time.Hour)
	result, err = RecordTrunkRed(third, trunkRedRecordFixture(identity, "batch-3", "attempt-3", "base-4", third.stamp()))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("record after close: %+v %v", result, err)
	}
	projected, _ = ProjectAt(root, result.Tip)
	if len(projected.Tree.TrunkRed) != 2 || projected.Tree.TrunkRed[1].ID != identity+"-2" || projected.Tree.TrunkRed[0].Closed == nil {
		t.Fatalf("closed identity suffix: %+v", projected.Tree.TrunkRed)
	}

	ownedID := identity + "-2"
	own := trunkRedVerbReq(root, "01J5X0000000000000000000R5", "mac-a")
	own.Now = third.Now.Add(time.Hour)
	result, err = OwnTrunkRed(own, TrunkRedOwnArgs{Entry: ownedID, Goal: "solo-goal", Branch: "fix/red", BranchCommit: "abc123"})
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("take entry: %+v %v", result, err)
	}
	other := trunkRedVerbReq(root, "01J5X0000000000000000000R6", "mac-b")
	result, err = OwnTrunkRed(other, TrunkRedOwnArgs{Entry: ownedID, Goal: "solo-goal"})
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "TRUNK_RED_OWNED_ELSEWHERE") {
		t.Fatalf("other machine take: %+v %v", result, err)
	}
	human := trunkRedVerbReq(root, "01J5X0000000000000000000R7", "mac-b")
	human.Actor.Human = "Wido"
	human.Now = own.Now.Add(time.Hour)
	result, err = OwnTrunkRed(human, TrunkRedOwnArgs{Entry: ownedID, Goal: "solo-goal", To: "mac-c", By: "Wido"})
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("human reassignment: %+v %v", result, err)
	}
	closeRequest := trunkRedVerbReq(root, "01J5X0000000000000000000R8", "mac-b")
	closeRequest.Actor.Human = "Wido"
	closeRequest.Now = human.Now.Add(time.Hour)
	result, err = CloseTrunkRed(closeRequest, TrunkRedCloseArgs{Entry: ownedID, By: "Wido", Why: "the failure was external"})
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("human close: %+v %v", result, err)
	}
	projected, _ = ProjectAt(root, result.Tip)
	closed := projected.Tree.TrunkRed[1]
	if closed.Owner.Machine != "mac-c" || closed.Owner.How != "hand" || closed.Owner.By != "Wido" || closed.Closed == nil || closed.Closed.Why != "the failure was external" || len(closed.Holds) != 0 {
		t.Fatalf("human-owned closed entry: %+v", closed)
	}
}

func TestTrunkRedRecoveryRebuildsRecordOwnAndClear(t *testing.T) {
	t.Parallel()
	root := soloLedgerRepo(t)
	identity := "tr-fast-recovery0001"
	record := trunkRedVerbReq(root, "01J5X0000000000000000000S1", "mac-a")
	record.Now = time.Date(2026, 9, 17, 8, 0, 0, 0, time.UTC)
	recordRequest := trunkRedRecordRequest(record, trunkRedRecordFixture(identity, "batch-r", "attempt-r", "base-r", record.stamp()))
	createDeadTrunkRedEntry(t, root, recordRequest)
	if reports, err := Recover(record.Endpoint); err != nil || len(reports) == 0 {
		t.Fatalf("recover record: %+v %v", reports, err)
	}

	own := trunkRedVerbReq(root, "01J5X0000000000000000000S2", "mac-a")
	ownRequest := trunkRedOwnRequest(own, TrunkRedOwnArgs{Entry: identity, Goal: "solo-goal"})
	createDeadTrunkRedEntry(t, root, ownRequest)
	if reports, err := Recover(own.Endpoint); err != nil || len(reports) == 0 {
		t.Fatalf("recover own: %+v %v", reports, err)
	}
	ownedProjection, err := Project(own.Endpoint, false, time.Now())
	if err != nil || len(ownedProjection.Tree.TrunkRed) != 1 {
		t.Fatalf("project owned entry: %+v %v", ownedProjection, err)
	}

	clear := trunkRedVerbReq(root, "01J5X0000000000000000000S3", "mac-a")
	clearRequest := trunkRedClearRequest(clear, TrunkRedClearArgs{Entry: identity, Attempt: "green-r", BaseCommit: "base-green", BaseTree: "tree-green", Group: "fast",
		ExpectedEntry: ownedProjection.Tree.TrunkRed[0]})
	createDeadTrunkRedEntry(t, root, clearRequest)
	if reports, err := Recover(clear.Endpoint); err != nil || len(reports) == 0 {
		t.Fatalf("recover clear: %+v %v", reports, err)
	}
	projection, err := Project(clear.Endpoint, false, time.Now())
	if err != nil || len(projection.Tree.TrunkRed) != 1 || projection.Tree.TrunkRed[0].Closed == nil || projection.Tree.TrunkRed[0].Owner.How != "taken" {
		t.Fatalf("recovered tree: %+v %v", projection.Tree.TrunkRed, err)
	}
}

func TestTrunkRedRecoveryLegacyClearClosesOpenEntry(t *testing.T) {
	t.Parallel()
	root, request, _, at := legacyTrunkRedClearFixture(t)
	createDeadTrunkRedEntry(t, root, request)

	reports, err := Recover(requestEndpoint(root))
	if err != nil {
		t.Fatal(err)
	}
	report := trunkRedRecoveryReport(t, reports, request.Opid)
	if report.Action != ActionComplete || !strings.Contains(report.Detail, "confirmed") {
		t.Fatalf("legacy clear recovery report: %+v", report)
	}
	entry := projectedTrunkRedEntry(t, root, at)
	if entry.Closed == nil || entry.Closed.Attempt != request.Intent.Args["attempt"] ||
		entry.Closed.BaseCommit != request.Intent.Args["baseCommit"] || entry.Closed.How != "green" ||
		entry.Closed.Opid != request.Opid || len(entry.Holds) != 0 || entry.FixBranch.State != TrunkRedBranchMerged {
		t.Fatalf("legacy clear did not close the entry from its stored proof: %+v", entry)
	}
}

func TestTrunkRedRecoveryLegacyClearIgnoresChangedOwner(t *testing.T) {
	t.Parallel()
	root, request, entry, at := legacyTrunkRedClearFixture(t)
	createDeadTrunkRedEntry(t, root, request)

	reassign := trunkRedVerbReq(root, "01J5X0000000000000000000V2", "mac-b")
	reassign.Actor.Human = "Wido"
	reassign.Now = at.Add(time.Hour)
	result, err := OwnTrunkRed(reassign, TrunkRedOwnArgs{Entry: entry.ID, Goal: "solo-goal", To: "mac-b", By: "Wido"})
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("reassign entry: %+v %v", result, err)
	}
	if reports, err := Recover(requestEndpoint(root)); err != nil {
		t.Fatal(err)
	} else if report := trunkRedRecoveryReport(t, reports, request.Opid); report.Action != ActionComplete || !strings.Contains(report.Detail, "confirmed") {
		t.Fatalf("legacy clear recovery report: %+v", report)
	}
	recovered := projectedTrunkRedEntry(t, root, at.Add(2*time.Hour))
	if recovered.Owner.Machine != "mac-b" || recovered.Closed == nil || recovered.Closed.Opid != request.Opid {
		t.Fatalf("legacy clear did not retain the changed owner and close the entry: %+v", recovered)
	}
}

func TestTrunkRedRecoveryLegacyClearRejectsClosedEntry(t *testing.T) {
	t.Parallel()
	root, request, entry, at := legacyTrunkRedClearFixture(t)
	createDeadTrunkRedEntry(t, root, request)

	other := trunkRedVerbReq(root, "01J5X0000000000000000000V3", "mac-b")
	other.Now = at.Add(time.Hour)
	result, err := ClearTrunkRed(other, TrunkRedClearArgs{Entry: entry.ID, Attempt: "other-attempt", BaseCommit: "other-base",
		BaseTree: "other-tree", Group: entry.Group, ExpectedEntry: entry})
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("other clear: %+v %v", result, err)
	}
	closedBefore := projectedTrunkRedEntry(t, root, at.Add(2*time.Hour)).Closed
	if reports, err := Recover(requestEndpoint(root)); err != nil {
		t.Fatal(err)
	} else if report := trunkRedRecoveryReport(t, reports, request.Opid); report.Action != ActionComplete || !strings.Contains(report.Detail, "rejected") {
		t.Fatalf("legacy clear recovery report: %+v", report)
	}
	journal, err := ReadEntry(root, request.Opid)
	if err != nil || journal.Outcome != OutcomeRejected || !strings.Contains(journal.Evidence, "TRUNK_RED_CLOSED") {
		t.Fatalf("legacy clear journal: %+v %v", journal, err)
	}
	closedAfter := projectedTrunkRedEntry(t, root, at.Add(2*time.Hour)).Closed
	if closedAfter == nil || closedBefore == nil || *closedAfter != *closedBefore || closedAfter.Opid != other.opid() {
		t.Fatalf("rejected legacy clear changed the existing closure: before=%+v after=%+v", closedBefore, closedAfter)
	}
}

func TestTrunkRedRecoveryPresentEmptyBindingIsRejected(t *testing.T) {
	t.Parallel()
	root, request, _, at := legacyTrunkRedClearFixture(t)
	request.Intent.Args["expectedEntry"] = ""
	createDeadTrunkRedEntry(t, root, request)
	assertUnreadableTrunkRedBindingRejected(t, root, request, at)
}

func TestTrunkRedRecoveryPresentMalformedBindingIsRejected(t *testing.T) {
	t.Parallel()
	root, request, _, at := legacyTrunkRedClearFixture(t)
	request.Intent.Args["expectedEntry"] = "{not-json"
	createDeadTrunkRedEntry(t, root, request)
	assertUnreadableTrunkRedBindingRejected(t, root, request, at)
}

func TestTrunkRedRecoveryLegacyClearPreservesStoredIntent(t *testing.T) {
	t.Parallel()
	root, stored, _, _ := legacyTrunkRedClearFixture(t)
	createDeadTrunkRedEntry(t, root, stored)
	entry, err := ReadEntry(root, stored.Opid)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, err := requestForEntry(requestEndpoint(root), entry)
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt.Opid != stored.Opid {
		t.Fatalf("rebuilt opid = %q, want %q", rebuilt.Opid, stored.Opid)
	}
	if len(rebuilt.Intent.Args) != 6 {
		t.Fatalf("rebuilt legacy arguments = %#v", rebuilt.Intent.Args)
	}
	for key, value := range stored.Intent.Args {
		if rebuilt.Intent.Args[key] != value {
			t.Fatalf("rebuilt legacy argument %q = %q, want %q", key, rebuilt.Intent.Args[key], value)
		}
	}
	if _, present := rebuilt.Intent.Args["expectedEntry"]; present {
		t.Fatalf("rebuilt legacy clear invented an entry binding: %#v", rebuilt.Intent.Args)
	}
}

func TestTrunkRedClearZeroExpectedEntryRemainsBound(t *testing.T) {
	t.Parallel()
	root, _, entry, at := legacyTrunkRedClearFixture(t)
	clear := trunkRedVerbReq(root, "01J5X0000000000000000000V5", "mac-a")
	clear.Now = at.Add(time.Hour)
	result, err := ClearTrunkRed(clear, TrunkRedClearArgs{Entry: entry.ID, Attempt: "green-zero", BaseCommit: "base-zero",
		BaseTree: "tree-zero", Group: entry.Group})
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "TRUNK_RED_CHANGED") {
		t.Fatalf("zero expected entry clear: %+v %v", result, err)
	}
	if projected := projectedTrunkRedEntry(t, root, at.Add(2*time.Hour)); projected.Closed != nil {
		t.Fatalf("zero expected entry clear closed the entry: %+v", projected)
	}
}

func TestTrunkRedLegacyClearRequestKeepsUnknownAndAlreadyApplied(t *testing.T) {
	t.Parallel()
	t.Run("unknown entry", func(t *testing.T) {
		root, request, _, _ := legacyTrunkRedClearFixture(t)
		request.Intent.Args["entry"] = "tr-fast-missing"
		request.Intent.Targets = []string{"tr-fast-missing"}
		rebuilt, err := requestForEntry(requestEndpoint(root), Entry{Opid: request.Opid, Machine: request.Machine,
			Lineage: request.Lineage, Intent: request.Intent})
		if err != nil {
			t.Fatal(err)
		}
		tip := mustGit(t, root, "rev-parse", LocalLedgerBranch)
		if _, err := rebuilt.Mutate(tip); err == nil || !strings.Contains(err.Error(), "TRUNK_RED_UNKNOWN") {
			t.Fatalf("legacy clear of unknown entry: %v", err)
		}
	})
	t.Run("same operation already closed", func(t *testing.T) {
		root, request, entry, at := legacyTrunkRedClearFixture(t)
		entry.Closed = &TrunkRedClosure{At: at.Add(time.Hour).Format(time.RFC3339), Attempt: request.Intent.Args["attempt"],
			BaseCommit: request.Intent.Args["baseCommit"], How: "green", Opid: request.Opid}
		entry.Holds = []string{}
		publishTrunkRedWithRequest(t, root, entry, "01J5X0000000000000000000V4")
		rebuilt, err := requestForEntry(requestEndpoint(root), Entry{Opid: request.Opid, Machine: request.Machine,
			Lineage: request.Lineage, Intent: request.Intent})
		if err != nil {
			t.Fatal(err)
		}
		tip := mustGit(t, root, "rev-parse", LocalLedgerBranch)
		_, err = rebuilt.Mutate(tip)
		var already AlreadyApplied
		if !errors.As(err, &already) {
			t.Fatalf("legacy clear did not recognize its closure: %v", err)
		}
	})
}

func legacyTrunkRedClearFixture(t *testing.T) (string, PublishRequest, TrunkRedEntry, time.Time) {
	t.Helper()
	root := soloLedgerRepo(t)
	at := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	entry := testTrunkRedEntry("tr-fast-legacy-clear", "tr-fast-legacy-clear", at.Format(time.RFC3339))
	entry.FixGoal = "solo-goal"
	entry.FixBranch = TrunkRedBranch{Name: "fix/legacy-clear", Commit: "abc123", State: TrunkRedBranchOpen}
	publishTrunkRed(t, root, []TrunkRedEntry{entry})
	clear := trunkRedVerbReq(root, "01J5X0000000000000000000V1", "mac-a")
	clear.Now = at.Add(time.Hour)
	request := trunkRedClearRequest(clear, TrunkRedClearArgs{Entry: entry.ID, Attempt: "green-legacy", BaseCommit: "base-green",
		BaseTree: "tree-green", Group: entry.Group, BranchMerged: true, ExpectedEntry: entry})
	delete(request.Intent.Args, "expectedEntry")
	return root, request, entry, at
}

func requestEndpoint(root string) Endpoint {
	return Endpoint{Root: root, Remote: "local", Branch: "refs/heads/main"}
}

func projectedTrunkRedEntry(t *testing.T, root string, at time.Time) TrunkRedEntry {
	t.Helper()
	projection, err := Project(requestEndpoint(root), false, at)
	if err != nil || len(projection.Tree.TrunkRed) != 1 {
		t.Fatalf("project trunk-red entry: %+v %v", projection.Tree.TrunkRed, err)
	}
	return projection.Tree.TrunkRed[0]
}

func trunkRedRecoveryReport(t *testing.T, reports []RecoveryReport, opid string) RecoveryReport {
	t.Helper()
	for _, report := range reports {
		if report.Opid == opid {
			return report
		}
	}
	t.Fatalf("recovery report for %s not found: %+v", opid, reports)
	return RecoveryReport{}
}

func assertUnreadableTrunkRedBindingRejected(t *testing.T, root string, request PublishRequest, at time.Time) {
	t.Helper()
	want := "the stored trunk-red clear has no readable entry binding; close it by hand"
	reports, err := Recover(requestEndpoint(root))
	if err != nil {
		t.Fatal(err)
	}
	report := trunkRedRecoveryReport(t, reports, request.Opid)
	if report.Action != ActionComplete || report.Detail != "not rebuildable: "+want {
		t.Fatalf("unreadable binding recovery report: %+v", report)
	}
	journal, err := ReadEntry(root, request.Opid)
	if err != nil || journal.Outcome != OutcomeRejected || journal.Evidence != want {
		t.Fatalf("unreadable binding journal: %+v %v", journal, err)
	}
	if entry := projectedTrunkRedEntry(t, root, at.Add(2*time.Hour)); entry.Closed != nil {
		t.Fatalf("unreadable binding closed the entry: %+v", entry)
	}
}

func publishTrunkRedWithRequest(t *testing.T, root string, entry TrunkRedEntry, ulid string) {
	t.Helper()
	request := trunkRedVerbReq(root, ulid, "mac-a")
	result, err := Publish(request.Endpoint, PublishRequest{Opid: request.opid(), Machine: request.Actor.Machine, Lineage: request.Actor.Lineage,
		Intent: Intent{Verb: "trunk-red-fixture"}, Message: "publish trunk-red fixture",
		Mutate: func(string) ([]Change, error) {
			return []Change{{Path: trunkRedPath, Content: RenderTrunkRed([]TrunkRedEntry{entry})}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(root, commit) }})
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("publish trunk-red fixture: %+v %v", result, err)
	}
}

func createDeadTrunkRedEntry(t *testing.T, root string, request PublishRequest) {
	t.Helper()
	entry, err := CreateEntry(root, request.Opid, request.Machine, request.Lineage, request.Intent)
	if err != nil {
		t.Fatal(err)
	}
	entry.Owner = OwnerIdentity{Pid: 999999999, PidStartedAt: 1}
	if err := writeEntry(root, entry); err != nil {
		t.Fatal(err)
	}
}

func trunkRedVerbReq(root, ulid, machine string) VerbRequest {
	request := verbReq(root, ulid, machine)
	request.Endpoint = Endpoint{Root: root, Remote: "local", Branch: "refs/heads/main"}
	return request
}

func publishTrunkRed(t *testing.T, root string, entries []TrunkRedEntry) PublishResult {
	t.Helper()
	request := trunkRedVerbReq(root, "01J5X0000000000000000000T1", "mac-a")
	result, err := Publish(request.Endpoint, PublishRequest{Opid: request.opid(), Machine: request.Actor.Machine, Lineage: request.Actor.Lineage,
		Intent: Intent{Verb: "trunk-red-fixture"}, Message: "publish trunk-red fixture",
		Mutate: func(string) ([]Change, error) {
			return []Change{{Path: trunkRedPath, Content: RenderTrunkRed(entries)}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(root, commit) }})
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("publish trunk-red fixture: %+v %v", result, err)
	}
	return result
}
