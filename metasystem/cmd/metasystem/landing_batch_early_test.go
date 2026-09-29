package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// quietEarlySeams are early acts that find nothing: a bed about something
// else runs them without effect.
func quietEarlySeams() batch.EarlySeams {
	return batch.EarlySeams{
		Cheap:  func(batch.Record) (batch.EarlyResult, error) { return batch.EarlyResult{}, nil },
		Prove:  func(batch.Record) (batch.EarlyResult, error) { return batch.EarlyResult{}, nil },
		Budget: func(batch.Record) (bool, string) { return false, "quiet" },
	}
}

// earlyRecord is a waiting batch: goal-a and goal-b joined, goal-c
// withdrawn, on base "base" with the recorded tip "tip-ab".
func earlyRecord() batch.Record {
	claim := func(revision uint64) batch.Claim {
		return batch.Claim{Machine: "landing", Lineage: "owner", Epoch: 1, Revision: revision, AccountingRevision: revision + 1}
	}
	record := batch.Record{Schema: 1, BatchID: "01j5x00000000000000000ea01", State: batch.StateOpen, TipTree: "tip-ab",
		Units: []batch.Unit{{GoalID: "goal-a", State: batch.UnitJoined, Claim: claim(2)}, {GoalID: "goal-b", State: batch.UnitJoined, Claim: claim(4)},
			{GoalID: "goal-c", State: batch.UnitWithdrawn, Claim: claim(6)}}}
	record.BaseTree = "base"
	return record
}

// TestBatchEarlyCheapPhaseRunsTheJoinsChecksOnTheRecordedTip (R27, U10b-3):
// the production cheap phase is the join's own admission run, once, on the
// tip the joins recorded, charged to the head member; a fresh group gets an
// episode nothing retains; a red returns its groups and attempt, and any
// other failure is an error.
func TestBatchEarlyCheapPhaseRunsTheJoinsChecksOnTheRecordedTip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var calls []string
	var episode batch.JoinAdmission
	outcome := func(result batch.JoinAdmission, proof proofrun.TestResult, err error) batchAdmissionRun {
		return func(gotRoot, batchID, baseTree, goalID string, claim batch.Claim, tree, label string,
			fresh func(batch.JoinAdmission, int64) (batch.JoinAdmission, error)) (batch.JoinAdmission, proofrun.TestResult, bool, error) {
			calls = append(calls, gotRoot+"|"+batchID+"|"+baseTree+"|"+goalID+"|"+tree+"|"+label)
			if claim.Revision != 4 || claim.AccountingRevision != 5 {
				t.Fatalf("charged to the wrong claim: %+v", claim)
			}
			var freshErr error
			episode, freshErr = fresh(batch.JoinAdmission{Tree: tree, Status: "pending"}, 60_000)
			if freshErr != nil {
				t.Fatal(freshErr)
			}
			return result, proof, true, err
		}
	}
	result, err := earlyCheapPhase(root, earlyRecord(), outcome(batch.JoinAdmission{Status: "verified", AttemptID: "cheap-1"}, proofrun.TestResult{}, nil))
	if err != nil || result.Attempt != "cheap-1" || len(result.Failing) != 0 ||
		!slices.Equal(calls, []string{root + "|01j5x00000000000000000ea01|base|goal-b|tip-ab|early-cheap"}) ||
		len(episode.FreshEpisode) != 64 || episode.FreshExpiresAt == "" {
		t.Fatalf("green: result %+v err %v calls %v episode %+v", result, err, calls, episode)
	}

	red := proofrun.TestResult{AttemptID: "cheap-2", Groups: []proofrun.GroupResult{{ID: "go-unit", Status: "failed", LogPath: "/logs/unit.log"},
		{ID: "go-lint", Status: "passed"}}}
	result, err = earlyCheapPhase(root, earlyRecord(), outcome(batch.JoinAdmission{}, red, &batch.JoinAdmissionRed{Reason: "BATCH_JOIN_ADMISSION_RED: go-unit"}))
	if err != nil || result.Attempt != "cheap-2" || len(result.Failing) != 1 || result.Failing[0].ID != "go-unit" || result.Failing[0].LogPath != "/logs/unit.log" {
		t.Fatalf("red: result %+v err %v", result, err)
	}

	if _, err = earlyCheapPhase(root, earlyRecord(), outcome(batch.JoinAdmission{}, proofrun.TestResult{}, errors.New("BATCH_JOIN_TEST_DROPPED: x"))); err == nil {
		t.Fatal("a dropped run was not an error")
	}
}

// TestBatchEarlyProofIsLaunchedAsTheTipProofReservingNoHeadroom (R27,
// U10b-3): the early proof is one delivery attempt on the recorded tip, the
// head member's claim revisions and the members' union plan, launched by the
// tip proof's own launcher; its argv is the tip proof's without
// --require-diagnostic-headroom and without --hold-host-proving (the early
// proof runs on spare capacity and never takes the host's proving flock,
// U12), and a batch proof carrying an early retry decision names it.
func TestBatchEarlyProofIsLaunchedAsTheTipProofReservingNoHeadroom(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	plans := map[string][]string{"goal-a": {"g1", "g2"}, "goal-b": {"g2", "g3"}}
	plan := func(_, goalID, tree string, mode testpolicy.Mode) (testpolicy.Plan, error) {
		if tree != "tip-ab" {
			t.Fatalf("planned %s on %s", goalID, tree)
		}
		return testpolicy.Plan{RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard, SelectedGroups: plans[goalID]}, nil
	}
	var launched batchProofLaunch
	launch := func(request batchProofLaunch) (proofrun.TestResult, error) {
		launched = request
		return proofrun.TestResult{AttemptID: "early-1", Groups: []proofrun.GroupResult{{ID: "g1", Status: "passed"}, {ID: "g3", Status: "failed", LogPath: "/logs/g3.log"}}},
			errors.New("exit status 1")
	}
	result, err := earlyProof(root, earlyRecord(), plan, launch)
	want := batchProofLaunch{Root: batch.ModuleRoot(root), BatchID: "01j5x00000000000000000ea01", GoalID: "goal-b", Tree: "tip-ab",
		ResultPath: filepath.Join(batch.ModuleRoot(root), "artifacts", "agents", "proof-runs", "batch", "01j5x00000000000000000ea01-early.json"),
		Mode:       testpolicy.ModeStandard, Groups: []string{"g1", "g2", "g3"}, GoalRevision: 4, AccountingRevision: 5, Early: true}
	if err != nil || result.Attempt != "early-1" || len(result.Failing) != 1 || result.Failing[0].ID != "g3" || !equalLaunch(launched, want) {
		t.Fatalf("early proof: result %+v err %v launched %+v", result, err, launched)
	}

	argv := func(request batchProofLaunch) []string {
		t.Helper()
		dir := t.TempDir()
		recorded := filepath.Join(dir, "argv")
		stub := filepath.Join(dir, "engine")
		script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + recorded + "\nwhile [ $# -gt 0 ]; do [ \"$1\" = --result ] && printf '{}' > \"$2\"; shift; done\n"
		if err := testexec.WriteFile(stub, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		request.ResultPath = filepath.Join(dir, "result.json")
		if _, err := launchBatchTipProofWithDependencies(request, batchExecutionDependencies{
			executable: func() (string, error) { return stub, nil },
			checkout:   func(string, string) (string, func() error, error) { return dir, func() error { return nil }, nil },
			topLevel:   func(root string) (string, error) { return root, nil },
			readGit:    func(string, ...string) (string, error) { return "", nil },
		}); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(recorded)
		if err != nil {
			t.Fatal(err)
		}
		// The run's own directories differ per launch; the rest is the argv.
		fields := strings.Fields(string(data))
		for index := range fields {
			if index > 0 && (fields[index-1] == "--root" || fields[index-1] == "--result") {
				fields[index] = "<own>"
			}
		}
		return fields
	}
	tip := batchProofLaunch{Root: root, BatchID: "b", GoalID: "goal-b", Tree: "tip-ab", Mode: testpolicy.ModeStandard, GoalRevision: 4, AccountingRevision: 5}
	early := tip
	early.Early = true
	retried := tip
	retried.RetryDecision = "/decisions/early-retry.json"
	tipArgv, earlyArgv, retriedArgv := argv(tip), argv(early), argv(retried)
	// The early proof launches through the one launcher on spare capacity
	// (U12): it never takes the host's proving flock, so speculative work
	// never delays a real proof; the tip proof and its retry hold it.
	if !slices.Contains(tipArgv, "--require-diagnostic-headroom") || slices.Contains(earlyArgv, "--require-diagnostic-headroom") ||
		!slices.Contains(tipArgv, holdHostProvingFlag) || !slices.Contains(retriedArgv, holdHostProvingFlag) || slices.Contains(earlyArgv, holdHostProvingFlag) ||
		!slices.Equal(slices.DeleteFunc(slices.Clone(tipArgv), func(arg string) bool {
			return arg == "--require-diagnostic-headroom" || arg == holdHostProvingFlag
		}), earlyArgv) ||
		!slices.Contains(retriedArgv, "--retry-decision") || !slices.Contains(retriedArgv, "/decisions/early-retry.json") || slices.Contains(tipArgv, "--retry-decision") {
		t.Fatalf("argv:\ntip     %q\nearly   %q\nretried %q", tipArgv, earlyArgv, retriedArgv)
	}
}

func equalLaunch(got, want batchProofLaunch) bool {
	return got.Root == want.Root && got.BatchID == want.BatchID && got.GoalID == want.GoalID && got.Tree == want.Tree &&
		got.ResultPath == want.ResultPath && got.Mode == want.Mode && slices.Equal(got.Groups, want.Groups) &&
		got.GoalRevision == want.GoalRevision && got.AccountingRevision == want.AccountingRevision && got.Early == want.Early &&
		got.Token == "" && got.CandidateTip == "" && got.RetryDecision == "" && got.FreshEpisode == ""
}

// TestBatchEarlyBudgetKeepsTheBatchProofsHeadroom (U3-01): an early proof is
// admitted only when the head member's budget keeps, after its one attempt,
// the two attempts and the reserved minutes the batch proof's diagnostic
// headroom needs; two attempts left, no early proof.
func TestBatchEarlyBudgetKeepsTheBatchProofsHeadroom(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	budget := func(attempts, minutes uint64, status dispatchcore.BudgetProjectionStatus) func(string, batch.Unit, *batch.Unit, time.Time) (batchCostBudgetProjection, error) {
		return func(_ string, unit batch.Unit, incoming *batch.Unit, _ time.Time) (batchCostBudgetProjection, error) {
			if unit.GoalID != "goal-b" || incoming != nil {
				t.Fatalf("projected %s (incoming %v), want the head goal-b", unit.GoalID, incoming)
			}
			limits, err := goal.NewBudget("100h", 10, 1000, 1, 2)
			if err != nil {
				t.Fatal(err)
			}
			return batchCostBudgetProjection{Budget: dispatchcore.BudgetProjection{Status: status, Limits: limits, Attempts: 10 - attempts, ReservedJobMinutes: 1000 - minutes}}, nil
		}
	}
	cap := func(string) (uint64, error) { return 60, nil }
	for _, row := range []struct {
		name              string
		attempts, minutes uint64
		status            dispatchcore.BudgetProjectionStatus
		ok                bool
		why               string
	}{
		{"three attempts and three caps left", 3, 180, dispatchcore.BudgetKnown, true, ""},
		{"two attempts left", 2, 900, dispatchcore.BudgetKnown, false, "no early work: goal-b has 2 attempts and 900 reserved minutes left, kept for the batch proof"},
		{"two caps of minutes left", 5, 179, dispatchcore.BudgetKnown, false, "no early work: goal-b has 5 attempts and 179 reserved minutes left, kept for the batch proof"},
		{"unknown budget", 5, 900, dispatchcore.BudgetUnknown, false, "no early work: goal-b's budget is unknown"},
	} {
		ok, why := earlyBudget("/lane", earlyRecord(), at, budget(row.attempts, row.minutes, row.status), cap)
		if ok != row.ok || why != row.why {
			t.Errorf("%s: ok %t why %q, want %t %q", row.name, ok, why, row.ok, row.why)
		}
	}
}

// TestBatchProofRetriesWhatTheEarlyProofOfItsTreeFailed (U3-03, through the
// real shared-component admission and retained verifier): the early proof of
// goal-a and goal-b fails a group nobody was named for; the awaited goal-y
// never joins, so the batch proof's candidate is the early tree. Without a
// decision the admission refuses the batch proof (retry required for the
// early producer); with the decision the lane writes, the batch proof
// re-executes the group, and its source is the batch proof's own attempt,
// never the early red.
func TestBatchProofRetriesWhatTheEarlyProofOfItsTreeFailed(t *testing.T) {
	t.Parallel()
	fixture := newPortableFileProof(t)
	fixture.put("scripts/check.sh", portableFlakeCheck, 0o755)
	tree, files := fixture.snapshot()
	plan := fixture.plan(fixture.loadedContract(files), "app/a.txt")
	run := fixture.request(tree, files, plan)
	run.CandidateEngineBuildIdentity = fixture.engineIdentity(tree, run.Environment)
	if red, _ := fixture.execute(run, false); portableGroups(red)["app-a"].Status != "failed" {
		t.Fatalf("the early proof did not fail: %+v", red)
	}
	earlyAttempt := retainedAttemptFor(t, fixture.root, "")
	// The batch record kept nothing of the early red (it ended after the
	// start, or the owner restarted): the lookup is the retained store's.
	record := batch.Record{Schema: 1, BatchID: "01j5x00000000000000000ea01", State: batch.StateSealed, TipTree: tree,
		StartReason: "goal-y on m1c left the pipeline without joining; nothing else within reach"}
	head := batch.Unit{GoalID: "portable"}

	ids, _ := fixture.prepare(run)
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	identity := proofrun.BuildProofIdentityForContext(proofrun.ExecutionContext{ManifestDigest: strings.Repeat("a", 64), Configuration: strings.Repeat("b", 64),
		Platform: runtime.GOOS + "/" + runtime.GOARCH, Toolchain: strings.Repeat("c", 64)}, "selected", "testing", nil, 2)
	admission := privateProofAdmissionRequest(proofrun.WithTestHostLoadSampler(proofrun.AdmissionRequest{ControlRoot: fixture.root, ExecutionRoot: fixture.root,
		GoalID: "portable", GoalRevision: 2, AccountingRevision: 2, CandidateGoalID: "portable", CandidateRevision: 2, CandidateTree: tree,
		ReservedMinutes: 2, Identity: identity, Launcher: launcher, Now: time.Now().UTC(), ComponentIdentities: ids, SharedComponents: true, ForceAttempt: true}, "0"))
	if decision, decided, err := proofrun.NoChildDecisionLocked(admission); err != nil || !decided ||
		decision.Disposition != proofrun.DispositionRetryRequired || decision.PriorAttempt != earlyAttempt {
		t.Fatalf("without a decision: %+v decided %t err %v", decision, decided, err)
	}

	path, err := tipRetryDecision(fixture.root, record, head, proofrun.ReadAttempts)
	if err != nil || path == "" {
		t.Fatalf("no retry decision for the early tree: %q %v", path, err)
	}
	var written proofrun.RetryDecision
	if data, readErr := os.ReadFile(path); readErr != nil || json.Unmarshal(data, &written) != nil || written.PriorAttempt != earlyAttempt {
		t.Fatalf("decision %s: %+v %v", path, written, readErr)
	}
	fixture.retry = path
	retried, _ := fixture.execute(run, true)
	group := portableGroups(retried)["app-a"]
	if group.Status != "passed" || !group.NativeLaunched || fixture.counts()["a"] != 2 {
		t.Fatalf("the batch proof did not re-execute the early red: %+v counts %v", group, fixture.counts())
	}
	batchAttempt := retainedAttemptFor(t, fixture.root, earlyAttempt)
	verified, err := fixture.verify(run, files, retried, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	proof := batch.Proof{Status: "green", Tree: tree, AttemptID: batchAttempt, SelectedGroups: []string{"app-a"}, Executions: []string{"app-a"}}
	if sources, err := batch.ResolveSources(proof, batchSourcesFromVerification(verified)); err != nil || sources["app-a"].Attempt != batchAttempt {
		t.Fatalf("sources %+v err %v, want app-a from the batch proof %s, never the early %s", sources, err, batchAttempt, earlyAttempt)
	}

	// A member joined after the early proof: the batch proof's tree is not the
	// early tree, and nothing is retried.
	record.TipTree = strings.Repeat("f", 40)
	if path, err := tipRetryDecision(fixture.root, record, head, proofrun.ReadAttempts); err != nil || path != "" {
		t.Fatalf("a grown batch carried a retry decision: %q %v", path, err)
	}
}

// TestBatchEarlyProofPassesAreReusedByIdentityAlone (R27, R10, the honest
// caveat, through the real retained verifier): a group the early proof
// passed is reused by the batch proof of a grown tree whose change leaves
// that group's inputs alone, at the identical execution identity, from the
// early attempt; a change to its inputs moves the identity and nothing is
// reused.
func TestBatchEarlyProofPassesAreReusedByIdentityAlone(t *testing.T) {
	t.Parallel()
	fixture := newPortableFileProof(t)
	partial, partialFiles := fixture.snapshot()
	plan := fixture.plan(fixture.loadedContract(partialFiles), "app/a.txt")
	run := fixture.request(partial, partialFiles, plan)
	run.CandidateEngineBuildIdentity = fixture.engineIdentity(partial, run.Environment)
	early, earlyIDs := fixture.execute(run, true)
	earlyAttempt := retainedAttemptFor(t, fixture.root, "")

	// The late member changes a document only.
	fixture.put("docs/late.md", "the late member's change\n", 0o644)
	grown, grownFiles := fixture.snapshot()
	grownRun := fixture.request(grown, grownFiles, plan)
	grownRun.CandidateEngineBuildIdentity = fixture.engineIdentity(grown, grownRun.Environment)
	grownIDs, _ := fixture.prepare(grownRun)
	if grownIDs["app-a"] != earlyIDs["app-a"] {
		t.Fatalf("an untouched group's identity moved across trees: %s then %s", earlyIDs["app-a"], grownIDs["app-a"])
	}
	verified, err := fixture.verify(grownRun, grownFiles, early, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if sources := batchSourcesFromVerification(verified); sources["app-a"] != earlyAttempt || fixture.counts()["a"] != 1 {
		t.Fatalf("app-a at identity %s was not reused from the early attempt %s: %v counts %v", grownIDs["app-a"], earlyAttempt, sources, fixture.counts())
	}

	// A late member that changes the group's own input moves its identity.
	fixture.put("app/a.txt", "the late member's a\n", 0o644)
	moved, movedFiles := fixture.snapshot()
	movedRun := fixture.request(moved, movedFiles, plan)
	movedRun.CandidateEngineBuildIdentity = fixture.engineIdentity(moved, movedRun.Environment)
	movedIDs, _ := fixture.prepare(movedRun)
	verified, _ = fixture.verify(movedRun, movedFiles, early, time.Now().UTC())
	if movedIDs["app-a"] == earlyIDs["app-a"] || batchSourcesFromVerification(verified)["app-a"] != "" {
		t.Fatalf("a changed input was reused: identity %s (early %s)", movedIDs["app-a"], earlyIDs["app-a"])
	}
}
