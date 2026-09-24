package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestBatchCostPrefixGenericBudgetRefusalUsesTypedReturn(t *testing.T) {
	t.Parallel()
	root, tree := batchPrefixReceiptTestRoot(t)
	fake := filepath.Join(root, "budget-refusal-engine")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' 'metasystem test run: BUDGET_REFUSED reservedJobMinutesLimit used=1000 limit=1000' >&2\nexit %d\n", proofrun.ExitAdmissionRefused)
	if err := testexec.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	dependencies := batchTestExecutionDependencies(t, root, tree, fake)
	record := batch.Record{Units: []batch.Unit{{GoalID: "goal-a", Claim: batch.Claim{Revision: 7, AccountingRevision: 5}}}}
	_, err := executeBatchPrefixReceiptWithDependencies(root, "batch", record, "goal-a", tree, batch.PrefixDecision{Groups: []string{"same"}}, dependencies)
	var budget *batch.PrefixBudgetRefusal
	if !errors.As(err, &budget) || !strings.Contains(budget.Error(), "BUDGET_REFUSED") {
		t.Fatalf("generic engine budget refusal was not typed for member return: %T %v", err, err)
	}
}

func TestBatchCostLandingReadyElapsedAuthorityMatchesDispatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	claimed := now.Add(-2 * time.Hour).Format(time.RFC3339)
	claim := batch.Claim{Machine: "source", Lineage: "lineage", Epoch: 1, Revision: 2, AccountingRevision: 2}
	unit := batch.Unit{GoalID: "goal-ready", State: batch.UnitJoined, Claim: claim}
	record := batch.Record{BatchID: "01j5x00000000000000000ba98", Seal: map[string]batch.Claim{"goal-ready": claim}}
	file := &goal.GoalFile{Id: "goal-ready", State: goal.StateClaimed, Revision: 2,
		Budget: &goal.Budget{ElapsedLimit: "1h", AttemptLimit: 2, ReservedJobMinutesLimit: 20, ActiveJobLimit: 1},
		Claimed: &goal.ClaimRecord{At: claimed, Revision: 2, AccountingRevision: 2,
			HandedOver: goal.HandedOver{FromMachine: claim.Machine, FromLineage: claim.Lineage, FromEpoch: claim.Epoch, Batch: record.BatchID}},
		History: []goal.HistoryLine{{At: now.Add(-3 * time.Hour).Format(time.RFC3339), Verb: "open"}, {At: claimed, Verb: "claim"}},
		Landing: &goal.LandingRecord{At: now.Add(-time.Minute).Format(time.RFC3339)},
	}
	projection := goal.Projection{Tree: &goal.TreeGoals{Live: map[string]*goal.GoalFile{unit.GoalID: file}}}
	if err := authorizeBatchMemberInProjection(root, now, record, unit, projection); err != nil {
		t.Fatalf("landing-ready claim lost elapsed suspension: %v", err)
	}
	file.Landing = nil
	var budget *batch.PrefixBudgetRefusal
	if err := authorizeBatchMemberInProjection(root, now, record, unit, projection); !errors.As(err, &budget) {
		t.Fatalf("ordinary elapsed claim was not refused: %v", err)
	}
	file.Landing = &goal.LandingRecord{At: now.Add(-time.Minute).Format(time.RFC3339)}
	file.StopFence = &goal.StopFence{StopID: "stop-goal-ready-r2"}
	var fenced *batch.PrefixFencedRefusal
	if err := authorizeBatchMemberInProjection(root, now, record, unit, projection); !errors.As(err, &fenced) {
		t.Fatalf("landing-ready elapsed suspension escaped a live fence: %v", err)
	}
}

func TestBatchCostPortableJoinRefusesOverBudgetBeforeHandoverAndStatusShowsSnapshot(t *testing.T) {
	batchE2EProcessEnvironment.Lock()
	t.Cleanup(batchE2EProcessEnvironment.Unlock)
	portable := newPortableProofFixture(t)
	portable.contract.Groups = append(portable.contract.Groups, portable.group("app-b", "b"), portable.group("app-c", "c"))
	portable.contract.SchemaVersion = testpolicy.ExecutionContractSchemaVersion
	for index := range portable.contract.Groups {
		portable.contract.Groups[index].Phase, portable.contract.Groups[index].EnvironmentMode = "acceptance", "inherit"
	}
	portable.contract.Groups[2].Phase = "admission"
	portable.contract.Surfaces = []testpolicy.Surface{
		{ID: "app-a", Paths: []string{"app/a.txt"}, Standard: []string{"app-a"}, Critical: []string{"app-a-observed"}},
		{ID: "app-b", Paths: []string{"app/b.txt"}, Standard: []string{"app-b"}, Critical: []string{"app-b-observed"}},
		{ID: "app-c", Paths: []string{"app/c.txt"}, Standard: []string{"app-c"}, Critical: []string{"app-c-observed"}},
		{ID: "control", Paths: []string{"testing.json", "records/**", "plans/goals/**", "memory/**", "metasystem/**", "scripts/**", "metasystem.conf"}, Standard: []string{"app-a"}},
	}
	portable.contract.Cadence = []string{"app-a", "app-b", "app-c"}
	portable.write("app/b.txt", "green b\n", 0o644)
	portable.write("app/c.txt", "green c\n", 0o644)
	portable.writeContract()
	portable.writeBytes("plans/goals/backlog.md", goal.RenderRoot(&goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncRemote, Revision: 1,
	}), 0o644)
	if err := os.Remove(filepath.Join(portable.root, "plans", "goals", "portable.md")); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"goal-a", "goal-b", "goal-c"} {
		path := filepath.Join(portable.root, "plans", "goals", id+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file, problems := goal.ParseFile(data)
		if len(problems) != 0 {
			t.Fatalf("parse seed goal %s: %v", id, problems)
		}
		lineage := "lineage-" + id
		file.Claimed.Machine, file.Claimed.Lineage = id, lineage
		file.StopCapability.Machine = id
		file.History[1].Actor = id + "+" + lineage
		file.History[1].Opid = goal.Opid(strings.ToUpper(strings.Split(string(file.History[1].Opid), "-")[0]), id, lineage)
		if id == "goal-b" {
			file.Budget.ReservedJobMinutesLimit = 1
			file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
		}
		portable.writeBytes("plans/goals/"+id+".md", goal.RenderFile(file), 0o644)
	}
	config, err := os.ReadFile(filepath.Join(portable.root, "metasystem.conf")) // Synthetic fixture configuration.
	if err != nil {
		t.Fatal(err)
	}
	portable.write("metasystem.conf", string(config)+"goal.human.portable=Portable Fixture <portable@example.invalid>\n", 0o644)
	portable.write("scripts/receipt.sh", "#!/bin/sh\nset -eu\nmkdir -p memory\nprintf '%s\\n' \"$*\" >> memory/receipts.log\ngit add memory/receipts.log\n", 0o755)
	buildScript := fmt.Sprintf(portableCandidateBuild, strconv.Quote(portable.buildCounter), strconv.Quote(portable.engine))
	portable.write("scripts/agents/go-build.sh", buildScript, 0o755)
	portable.write("scripts/agents/commit.sh", batchE2ECommitScript, 0o755)
	portable.write("scripts/agents/pre-commit-guard.sh", "#!/bin/sh\nexit 0\n", 0o755)
	portable.commit("seed budget-limited portable batch application")
	baseCommit := portable.git("rev-parse", "HEAD")
	origin := filepath.Join(t.TempDir(), "origin.git")
	if output, err := exec.Command("git", "init", "-q", "-b", "main", "--bare", origin).CombinedOutput(); err != nil {
		t.Fatalf("create portable origin: %v: %s", err, output)
	}
	portable.git("remote", "add", "origin", origin)
	portable.git("push", "-q", "origin", "main")
	portable.git("update-ref", goal.AcceptedRef, baseCommit)
	landing := filepath.Join(t.TempDir(), "landing")
	bed := &batchE2EFixture{t: t, origin: origin, landing: landing, seats: map[string]string{}, now: time.Now().UTC()}
	bed.clone(landing, "landing")
	engine := &batchE2EEngine{path: portable.engine}
	bed.enrollPolicyEngine(baseCommit, engine)
	bed.holdLanding()
	for _, member := range []struct{ id, input string }{{"goal-a", "a"}, {"goal-b", "b"}, {"goal-c", "c"}} {
		seat := filepath.Join(t.TempDir(), member.id)
		bed.clone(seat, member.id)
		bed.seats[member.id] = seat
		(&batchE2EFixture{t: t, landing: seat, now: bed.now}).enrollPolicyEngine(baseCommit, engine)
		bed.announce(seat, "lineage-"+member.id)
		portableGoalBranch(t, bed, seat, member.id, member.input)
	}
	portable.root = landing
	originalPlanner, originalAdmissionExecutable := batchTreePlanExecutable, batchJoinAdmissionExecutable
	batchTreePlanExecutable = func() (string, error) { return portable.engine, nil }
	batchJoinAdmissionExecutable = func() (string, error) { return portable.engine, nil }
	t.Cleanup(func() {
		batchTreePlanExecutable, batchJoinAdmissionExecutable = originalPlanner, originalAdmissionExecutable
	})
	for key, value := range map[string]string{
		"GO_WANT_BATCH_E2E_COMMAND": "1", "METASYSTEM_OWNER_LINEAGE": landingOwnerLineage,
		"METASYSTEM_PROOF_ADMISSION_TEST_DIR":     portable.admissionDir,
		"METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT": landing,
	} {
		t.Setenv(key, value)
	}
	batchID := bed.join("goal-a")
	builds, native := portable.counts()
	if native["b"] != 0 || native["c"] != 0 {
		t.Fatalf("B or C ran before its join: %v", native)
	}
	code, _, stderr := captureCommandOutput(t, true, true, func() int {
		return runLandingBatch([]string{"join", "--root", bed.seats["goal-b"], "--goal", "goal-b", "--last"})
	})
	if code == 0 || !strings.Contains(stderr, "BATCH_COST_HEADROOM_REFUSED") {
		t.Fatalf("budget-limited join exit=%d stderr=%s", code, stderr)
	}
	record := bed.load(batchID)
	if record.ClosedReason != "budget-cost" || len(record.Units) != 1 || record.Units[0].GoalID != "goal-a" {
		t.Fatalf("join refusal moved ownership or failed to close existing admission: %+v", record)
	}
	seatGoal, err := os.ReadFile(filepath.Join(bed.seats["goal-b"], "plans", "goals", "goal-b.md"))
	if err != nil {
		t.Fatal(err)
	}
	seatFile, problems := goal.ParseFile(seatGoal)
	if len(problems) != 0 || seatFile.State != goal.StateClaimed || seatFile.Claimed == nil || seatFile.Claimed.Machine != "goal-b" {
		t.Fatalf("refused member lost seat claim: %v, %+v", problems, seatFile)
	}
	if afterBuilds, afterNative := portable.counts(); afterBuilds != builds || afterNative["b"] != 0 {
		t.Fatalf("forecast/refusal launched B native work: builds %d→%d native=%v", builds, afterBuilds, afterNative)
	}
	baseTree, err := fetchLandingBaseTree(landing)
	if err != nil {
		t.Fatal(err)
	}
	if err := productionBatchProofDependencies.seal(landing, batchID, landingOwnerLineage, baseTree, time.Now().UTC()); err != nil {
		t.Fatalf("production seal after budget closure: %v", err)
	}
	record = bed.load(batchID)
	if record.CostForecast == nil || record.CostForecast.Currency != "snapshot-not-revalidated" || record.CostForecast.ObservedAt == "" {
		t.Fatalf("sealed batch has no labeled historical cost forecast: %+v", record.CostForecast)
	}
	code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
		return runLandingBatch([]string{"status", "--root", bed.seats["goal-a"], "--batch", batchID})
	})
	var status batchStatusOutput
	if code != 0 || json.Unmarshal([]byte(stdout), &status) != nil || len(status.Batches) != 1 ||
		status.Batches[0].CostForecast == nil || status.Batches[0].CostForecast.Currency != "snapshot-not-revalidated" ||
		status.Batches[0].CostForecast.ObservedAt == "" {
		t.Fatalf("public status lost stored historical forecast: exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if len(status.Batches[0].LiveHeadroom) != 1 {
		t.Fatalf("public status omitted separate live headroom: %+v", status.Batches[0])
	}
	priorAttemptsLeft := status.Batches[0].LiveHeadroom[0].AttemptsLeft
	// The forecast fits C before its handover. A second, valid reservation
	// spends C's unchanged accounting revision immediately after handover;
	// only the final shared admission may decide whether native work starts.
	originalDependencies := batchJoinDependenciesForCommand
	var raceForecast batch.CostForecast
	batchJoinDependenciesForCommand = func() batchJoinDependencies {
		dependencies := originalDependencies()
		costForecast := dependencies.costForecast
		dependencies.costForecast = func(root string, record batch.Record, unit batch.Unit, at time.Time,
			plan func(string, string, string) (testpolicy.Plan, error), assemble func(string, string, []batch.Unit) ([]string, error)) (batch.Unit, batch.CostForecast, error) {
			prepared, forecast, err := costForecast(root, record, unit, at, plan, assemble)
			if unit.GoalID == "goal-c" {
				raceForecast = forecast
			}
			return prepared, forecast, err
		}
		handover := dependencies.handover
		dependencies.handover = func(request batchJoinRequest, id string, claim batch.Claim) error {
			if err := handover(request, id, claim); err != nil || request.GoalID != "goal-c" {
				return err
			}
			return reserveCostFixtureSpend(landing, "goal-c", claim, 1000)
		}
		return dependencies
	}
	t.Cleanup(func() { batchJoinDependenciesForCommand = originalDependencies })
	code, _, stderr = captureCommandOutput(t, true, true, func() int {
		return runLandingBatch([]string{"join", "--root", bed.seats["goal-c"], "--goal", "goal-c", "--last"})
	})
	if code == 0 || !strings.Contains(stderr, "BUDGET_REFUSED") {
		_, afterNative := portable.counts()
		t.Fatalf("same-revision spending race join exit=%d stderr=%s native=%v forecast=%+v", code, stderr, afterNative, raceForecast)
	}
	records, err := batch.NewStore(landing, nil).Records()
	if err != nil {
		t.Fatal(err)
	}
	returned := false
	for _, candidate := range records {
		if candidate.BatchID == batchID || len(candidate.Units) == 0 || candidate.Units[0].GoalID != "goal-c" {
			continue
		}
		returned = candidate.Units[0].State == batch.UnitReturnPending && candidate.Units[0].Outcome == batch.UnitWithdrawnBudget && candidate.State == batch.StateDissolved
	}
	if !returned {
		t.Fatalf("post-handover budget refusal did not durably return C: %+v", records)
	}
	if afterBuilds, afterNative := portable.counts(); afterBuilds != builds || afterNative["c"] != 0 {
		t.Fatalf("C native command crossed final budget admission: candidate builds %d→%d native=%v", builds, afterBuilds, afterNative)
	}
	// A stored forecast is historical even when live cap and retained budget
	// evidence change under the same immutable batch binding.
	if err := reserveCostFixtureSpend(landing, "goal-a", record.Units[0].Claim, 1); err != nil {
		t.Fatal(err)
	}
	configuration, err := os.ReadFile(filepath.Join(landing, "metasystem.conf")) // Synthetic fixture configuration.
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(landing, "metasystem.conf"), append(configuration, []byte("cap.min.proof.main.proof=1\nproof.admission.top-level-max=1\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = captureCommandOutput(t, true, true, func() int {
		return runLandingBatch([]string{"status", "--root", bed.seats["goal-a"], "--batch", batchID})
	})
	if code != 0 || json.Unmarshal([]byte(stdout), &status) != nil || len(status.Batches) != 1 ||
		status.Batches[0].CostForecast == nil || status.Batches[0].CostForecast.ObservedAt != record.CostForecast.ObservedAt ||
		status.Batches[0].CostForecast.Currency != "snapshot-not-revalidated" || status.Batches[0].CostSnapshotStale ||
		len(status.Batches[0].LiveHeadroom) != 1 || status.Batches[0].LiveHeadroom[0].AttemptsLeft >= priorAttemptsLeft {
		t.Fatalf("status relabeled mutable forecast as current after budget/cap move: exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

func reserveCostFixtureSpend(root, goalID string, claim batch.Claim, minutes uint64) error {
	canonical, err := canonicalProofRoot(root)
	if err != nil {
		return err
	}
	root = canonical
	goalLock, err := goalrevision.Acquire(root, goalID, claim.Revision, "cost-forecast-fixture")
	if err != nil {
		return err
	}
	defer goalLock.Release()
	proofLock, err := proofrun.AcquireMutation(root)
	if err != nil {
		return err
	}
	defer proofLock.Release()
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		return err
	}
	identity, err := proofrun.BuildProofIdentity(root, filepath.Join(root, "metasystem.conf"), "fixture", "cost-spend", []string{"spend"}, 1)
	if err != nil {
		return err
	}
	request := proofrun.AdmissionRequest{ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"),
		GoalID: goalID, GoalRevision: claim.Revision, AccountingRevision: claim.AccountingRevision,
		CandidateGoalID: goalID, CandidateRevision: claim.AccountingRevision, ReservedMinutes: minutes,
		Identity: identity, Launcher: launcher, Now: time.Now().UTC()}
	_, decision, err := proofrun.ReserveLocked(privateProofAdmissionRequest(proofrun.WithTestHostLoadSampler(request, "0")))
	if err != nil {
		return err
	}
	if decision.Disposition != proofrun.DispositionExecuted {
		return fmt.Errorf("cost fixture reservation was not admitted: %+v", decision)
	}
	data, err := os.ReadFile(filepath.Join(root, "plans", "goals", goalID+".md"))
	if err != nil {
		return err
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		return fmt.Errorf("cost fixture goal parse: %v", problems)
	}
	projection := dispatchcore.ProjectBudget(root, file, time.Now().UTC())
	if projection.Status != dispatchcore.BudgetKnown {
		return fmt.Errorf("cost fixture reservation did not project: %+v", projection.Unknown)
	}
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return err
	}
	live, err := goal.Project(endpoint, true, time.Now().UTC())
	if err != nil {
		return err
	}
	if live.Tree == nil || live.Tree.Live[goalID] == nil {
		return fmt.Errorf("cost fixture accepted ledger lost %s", goalID)
	}
	accepted := dispatchcore.ProjectBudget(root, live.Tree.Live[goalID], time.Now().UTC())
	if accepted.Status != dispatchcore.BudgetKnown {
		return fmt.Errorf("cost fixture accepted projection unknown: %+v (accepted rev=%d accounting=%d, local rev=%d accounting=%d)",
			accepted.Unknown, live.Tree.Live[goalID].Claimed.Revision, live.Tree.Live[goalID].Claimed.AccountingRevision,
			file.Claimed.Revision, file.Claimed.AccountingRevision)
	}
	return nil
}

// Reserve through the real attempt owner using an identity from an earlier
// native command/JUnit result. No result or observation record is fabricated.
func reserveCostNewerGroupObservation(t *testing.T, root, groupID string) proofrun.Attempt {
	t.Helper()
	controlRoot, err := canonicalProofRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	attempts, err := proofrun.ReadAttempts(controlRoot)
	if err != nil {
		t.Fatal(err)
	}
	var source proofrun.Attempt
	identity := ""
	for _, attempt := range attempts {
		if attempt.Terminal == nil || attempt.Terminal.Result != proofrun.TerminalSuccess || attempt.TestResult == nil {
			continue
		}
		for _, group := range attempt.TestResult.Groups {
			if group.ID == groupID && group.Status == "passed" && group.NativeLaunched && group.ExecutionIdentity != "" {
				source, identity = attempt, group.ExecutionIdentity
			}
		}
	}
	if identity == "" {
		t.Fatalf("no retained native command/JUnit success for %s", groupID)
	}
	goalLock, err := goalrevision.Acquire(controlRoot, source.GoalID, source.GoalRevision, "cost-newer-observation-fixture")
	if err != nil {
		t.Fatal(err)
	}
	defer goalLock.Release()
	proofLock, err := proofrun.AcquireMutation(controlRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer proofLock.Release()
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	request := proofrun.AdmissionRequest{ControlRoot: controlRoot, ExecutionRoot: source.ExecutionRoot,
		ConfPath: filepath.Join(controlRoot, "metasystem.conf"), GoalID: source.GoalID,
		GoalRevision: source.GoalRevision, AccountingRevision: source.AccountingRevision,
		BudgetEpoch: source.BudgetEpoch, CandidateGoalID: source.CandidateGoalID,
		CandidateRevision: source.CandidateRevision, CandidateBudgetEpoch: source.CandidateBudgetEpoch,
		CandidateTree: source.CandidateTree, ReservedMinutes: 1, Identity: source.ProofIdentity,
		Launcher: launcher, Now: time.Now().UTC(), ComponentIdentities: map[string]string{groupID: identity},
		SharedComponents: true, ForceAttempt: true, ForceGroups: true}
	live, decision, err := proofrun.ReserveLocked(privateProofAdmissionRequest(proofrun.WithTestHostLoadSampler(request, "0")))
	if err != nil || decision.Disposition != proofrun.DispositionExecuted || live.TestOwned[groupID] != identity {
		t.Fatalf("newer retained producer reservation: attempt=%+v decision=%+v err=%v", live, decision, err)
	}
	return live
}

func TestBatchCostStatusLabelsStoredSnapshotHistoricalAndStale(t *testing.T) {
	t.Parallel()
	record := batch.Record{Schema: 1, BatchID: "01j5x00000000000000000ba01", State: batch.StateSealed,
		TipTree: "tip", Units: []batch.Unit{{GoalID: "goal-a", Chain: "a", State: batch.UnitJoined}},
	}
	record.BaseTree, record.PrefixTrees = "base", []string{"tip"}
	forecast := batch.CostForecast{SchemaVersion: 1, ObservedAt: time.Unix(5, 0).UTC().Format(time.RFC3339Nano),
		Currency: "snapshot-not-revalidated", Binding: batch.CostBinding(record, record.Units, record.PrefixTrees)}
	record.CostForecast = &forecast
	settings := config.BatchLanding{Root: t.TempDir(), MaxWait: time.Minute}
	view := batchRecordStatus(record, settings)
	if view.CostForecast == nil || view.CostForecast.Currency != "snapshot-not-revalidated" || view.CostSnapshotStale {
		t.Fatalf("exact stored snapshot was presented incorrectly: %+v", view)
	}
	record.BaseTree = "moved"
	view = batchRecordStatus(record, settings)
	if view.CostForecast == nil || !view.CostSnapshotStale {
		t.Fatalf("moved base was presented as current: %+v", view)
	}
}

func TestBatchCostPortableEvidenceForecastIsReadOnlyAndWarmsFromRealCommand(t *testing.T) {
	batchE2EProcessEnvironment.Lock()
	t.Cleanup(batchE2EProcessEnvironment.Unlock)
	fixture := newPortableProofFixture(t)
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "portable-lineage")
	t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", fixture.admissionDir)
	t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", fixture.root)
	tree := fixture.git("rev-parse", "HEAD^{tree}")
	selection := costSelection{ID: "tip:portable", Kind: "tip", Tree: tree, GoalID: "portable", Requirements: []string{"app-a"}}
	cold, err := forecastTestingSelection(fixture.root, selection, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(cold.Groups) != 1 || cold.Groups[0].Status != "unknown" || cold.Groups[0].IdentityKnown {
		t.Fatalf("cold forecast invented known evidence: %+v", cold)
	}
	if builds, native := fixture.counts(); builds != 0 || native["a"] != 0 {
		t.Fatalf("cold forecast launched native work: builds=%d native=%v", builds, native)
	}
	result := filepath.Join(t.TempDir(), "tip.json")
	fixture.requireCommand("test", "run", "--root", fixture.root, "--goal", "portable", "--tree", tree,
		"--mode", "auto", "--purpose", "delivery", "--batch-prefix", "--batch-requirements", batchRequirementsArgument([]string{"app-a"}), "--result", result)
	builds, native := fixture.counts()
	warm, err := forecastTestingSelection(fixture.root, selection, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(warm.Groups) != 1 || warm.Groups[0].Status != "reusable" || !warm.Groups[0].IdentityKnown {
		t.Fatalf("warm forecast missed retained command/JUnit proof: %+v", warm)
	}
	if afterBuilds, afterNative := fixture.counts(); afterBuilds != builds || afterNative["a"] != native["a"] {
		t.Fatalf("warm forecast launched native work: before=%d/%v after=%d/%v", builds, native, afterBuilds, afterNative)
	}
	goalData, err := os.ReadFile(filepath.Join(fixture.root, "plans", "goals", "portable.md"))
	if err != nil {
		t.Fatal(err)
	}
	goalFile, problems := goal.ParseFile(goalData)
	if len(problems) != 0 || goalFile.Claimed == nil || goalFile.StopCapability == nil {
		t.Fatalf("portable goal claim is incomplete: %+v %v", goalFile, problems)
	}
	claim := batch.Claim{Machine: goalFile.Claimed.Machine, Lineage: goalFile.Claimed.Lineage,
		Epoch: uint64(goalFile.StopCapability.ClaimEpoch), Revision: goalFile.Claimed.Revision,
		AccountingRevision: goalFile.Claimed.AccountingRevision}
	const statusBatchID = "01j5x00000000000000000ba47"
	record := batch.Record{Schema: 1, BatchID: statusBatchID, State: batch.StateSealed,
		BaseTree: tree, TipTree: tree, PrefixTrees: []string{tree}, SelectedGroups: []string{"app-a"},
		Units: []batch.Unit{{GoalID: "portable", Chain: "portable-cost", Claim: claim, State: batch.UnitJoined,
			ChangedPaths: []string{"app/a.txt"}, SelectedGroups: []string{"app-a"}}},
		Seal: map[string]batch.Claim{"portable": claim}}
	priorPlanner := batchTreePlanExecutable
	batchTreePlanExecutable = func() (string, error) { return fixture.engine, nil }
	t.Cleanup(func() { batchTreePlanExecutable = priorPlanner })
	storedForecast, err := forecastBatchCost(fixture.root, record, nil, time.Now().UTC())
	if err != nil || len(storedForecast.Groups) != 1 || storedForecast.Groups[0].Status != "reusable" {
		t.Fatalf("real warm batch forecast is incomplete: %+v err=%v", storedForecast, err)
	}
	record.CostForecast = &storedForecast
	if err := batch.NewStore(fixture.root, nil).Create(record); err != nil {
		t.Fatal(err)
	}
	seat := filepath.Join(t.TempDir(), "seat")
	if output, err := exec.Command("git", "clone", "-q", fixture.root, seat).CombinedOutput(); err != nil {
		t.Fatalf("clone separate status seat: %v: %s", err, output)
	}
	readStatus := func(stage string) batchStatusView {
		t.Helper()
		code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
			return runLandingBatch([]string{"status", "--root", seat, "--landing-root", fixture.root, "--batch", statusBatchID})
		})
		var output batchStatusOutput
		if code != 0 || json.Unmarshal([]byte(stdout), &output) != nil || len(output.Batches) != 1 {
			t.Fatalf("%s public status failed: exit=%d stdout=%s stderr=%s", stage, code, stdout, stderr)
		}
		view := output.Batches[0]
		if view.CostForecast == nil || view.CostForecast.ObservedAt != storedForecast.ObservedAt ||
			view.CostForecast.Currency != "snapshot-not-revalidated" || view.CostSnapshotStale ||
			len(view.LiveHeadroom) != 1 || view.LiveHeadroom[0].GoalID != "portable" {
			t.Fatalf("%s public status lost historical snapshot or separate live headroom: %+v", stage, view)
		}
		if afterBuilds, afterNative := fixture.counts(); afterBuilds != builds || afterNative["a"] != native["a"] {
			t.Fatalf("%s status launched build/native work: before=%d/%v after=%d/%v", stage, builds, native, afterBuilds, afterNative)
		}
		return view
	}
	beforeStatus := readStatus("older-green")
	liveAttempt := reserveCostNewerGroupObservation(t, fixture.root, "app-a")
	live, err := forecastTestingSelection(fixture.root, selection, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(live.Groups) != 1 || live.Groups[0].Status != "live-wait" || live.Groups[0].Reason != "newer-live-producer" {
		t.Fatalf("older green survived a newer live producer: %+v", live)
	}
	if afterBuilds, afterNative := fixture.counts(); afterBuilds != builds || afterNative["a"] != native["a"] {
		t.Fatalf("live forecast launched native work: before=%d/%v after=%d/%v", builds, native, afterBuilds, afterNative)
	}
	liveStatus := readStatus("newer-live")
	if liveStatus.LiveHeadroom[0].AttemptsLeft >= beforeStatus.LiveHeadroom[0].AttemptsLeft {
		t.Fatalf("newer live attempt did not change separate headroom: before=%+v live=%+v", beforeStatus.LiveHeadroom, liveStatus.LiveHeadroom)
	}
	if _, err := proofrun.FinalizeAttempt(liveAttempt.ControlRoot, liveAttempt.AttemptID, proofrun.TerminalFailed, 1,
		"fixture newer red observation", nil, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	red, err := forecastTestingSelection(fixture.root, selection, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(red.Groups) != 1 || red.Groups[0].Status != "missing" || red.Groups[0].Reason != "newer-red-observation" {
		t.Fatalf("older green survived a newer red producer: %+v", red)
	}
	if afterBuilds, afterNative := fixture.counts(); afterBuilds != builds || afterNative["a"] != native["a"] {
		t.Fatalf("red forecast launched native work: before=%d/%v after=%d/%v", builds, native, afterBuilds, afterNative)
	}
	redStatus := readStatus("newer-red")
	if redStatus.LiveHeadroom[0].AttemptsLeft != liveStatus.LiveHeadroom[0].AttemptsLeft {
		t.Fatalf("finalizing newer red attempt changed reservation headroom: live=%+v red=%+v", liveStatus.LiveHeadroom, redStatus.LiveHeadroom)
	}
}

func TestBatchCostFreshForecastMatchesRetainedVerificationAtExpiry(t *testing.T) {
	batchE2EProcessEnvironment.Lock()
	t.Cleanup(batchE2EProcessEnvironment.Unlock)
	now := time.Now().UTC().Truncate(time.Second)
	t.Setenv(goalNowEnvironment, now.Format(time.RFC3339Nano))
	processTable := filepath.Join(t.TempDir(), "processes.json")
	if err := os.WriteFile(processTable, []byte("[]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", processTable)
	fixture := newPortableProofFixture(t)
	fixture.writeEpisodeContract()
	tree := fixture.commit("declare forecast freshness observation")
	// Detached prefix commits bind freshness. Use the authorized fixture instant
	// so the run, forecast, and verifier materialize the same candidate commit.
	t.Setenv("GIT_AUTHOR_DATE", now.Format(time.RFC3339Nano))
	t.Setenv("GIT_COMMITTER_DATE", now.Format(time.RFC3339Nano))
	detached, err := (gittree.Workspace{Dir: fixture.root}).NewDetachedWorktree(tree)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := detached.Close(); err != nil {
			t.Error(err)
		}
	})
	executionRoot := batch.ModuleRoot(detached.Workspace().Dir)
	episode := strings.Repeat("6", 64)
	expiresAt := now.Add(time.Hour)
	expires := expiresAt.Format(time.RFC3339Nano)
	base := []string{"--root", executionRoot, "--control-root", fixture.root, "--goal", "portable", "--tree", tree,
		"--mode", "auto", "--purpose", "delivery", "--batch-prefix", "--batch-requirements", batchRequirementsArgument([]string{"app-a"}),
		"--fresh-episode", episode, "--fresh-expires-at", expires}
	fixture.requireCommand(append([]string{"test", "run"}, base...)...)
	selection := costSelection{ID: "tip:fresh", Kind: "tip", Tree: tree, GoalID: "portable", Requirements: []string{"app-a"},
		FreshEpisode: episode, FreshExpiresAt: expires}
	verification := testingSelectionRequest{Root: executionRoot, ControlRoot: fixture.root, GoalID: "portable", Tree: tree,
		Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery, BatchPrefixReceipt: true,
		BatchRequirements: []string{"app-a"}, FreshEpisode: episode, FreshExpiresAt: expires}
	for _, boundary := range []struct {
		name     string
		at       time.Time
		reusable bool
	}{
		{name: "expiry-minus-one-nanosecond", at: expiresAt.Add(-time.Nanosecond), reusable: true},
		{name: "exact-expiry", at: expiresAt},
		{name: "expiry-plus-one-nanosecond", at: expiresAt.Add(time.Nanosecond)},
	} {
		t.Run(boundary.name, func(t *testing.T) {
			t.Setenv(goalNowEnvironment, boundary.at.Format(time.RFC3339Nano))
			forecast, forecastErr := forecastTestingSelection(fixture.root, selection, 2)
			verified, verifyErr := verifyRetainedTesting(verification)
			if boundary.reusable {
				if forecastErr != nil || len(forecast.Groups) != 1 || forecast.Groups[0].GroupID != "app-a" ||
					forecast.Groups[0].Status != "reusable" || forecast.Groups[0].Reason != "" ||
					verifyErr != nil || !verified.Delivery.Sufficient {
					t.Fatalf("freshness boundary %s forecast=%+v forecastErr=%v verified=%+v verifyErr=%v",
						boundary.at, forecast, forecastErr, verified.Delivery, verifyErr)
				}
				return
			}
			if forecastErr != nil {
				t.Fatalf("expired forecast returned an unrelated error at %s: %v", boundary.at, forecastErr)
			}
			if len(forecast.Groups) != 1 || forecast.Groups[0].GroupID != "app-a" ||
				forecast.Groups[0].Status != "missing" || forecast.Groups[0].Reason != "missing-proof" {
				t.Fatalf("expired forecast did not report app-a freshness expiry at %s: %+v", boundary.at, forecast)
			}
			const expired = "freshness episode has expired; renew the proof decision"
			if verifyErr == nil || verifyErr.Error() != expired {
				t.Fatalf("retained verification returned the wrong expiry error at %s: %v", boundary.at, verifyErr)
			}
		})
	}
}

func TestBatchCostTipFirstSharesKnownIdentityButNotUnknownOrFresh(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("cap.min.proof.main.proof=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	units := []batch.Unit{{GoalID: "goal-a", Chain: "a", State: batch.UnitJoined},
		{GoalID: "goal-b", Chain: "b", State: batch.UnitJoined}, {GoalID: "goal-c", Chain: "c", State: batch.UnitJoined}}
	record := batch.Record{TipTree: "tree-c", Units: units}
	// Newer prefixes introduce B and C. A and B native identities are the
	// same across later trees, while unknown metadata and fresh episodes stay
	// distinct requirements.
	record.BaseTree, record.PrefixTrees = "base", []string{"tree-a", "tree-b", "tree-c"}
	decision := func(_ string, units []batch.Unit, _ string) (batch.PrefixDecision, error) {
		ids := []string{"a"}
		if len(units) >= 2 {
			ids = append(ids, "b", "u", "fresh")
		}
		if len(units) == 3 {
			ids = append(ids, "c")
		}
		return batch.PrefixDecision{Groups: ids}, nil
	}
	run := func(_ string, selection costSelection, cap uint64) (costSelectionEvidence, error) {
		out := costSelectionEvidence{Request: batch.CostForecastRequest{ID: selection.ID, Kind: selection.Kind,
			Tree: selection.Tree, ChargeGoal: selection.GoalID, SelectedGroups: slices.Clone(selection.Requirements)}}
		for _, id := range selection.Requirements {
			row := batch.CostForecastGroup{RequestID: selection.ID, Tree: selection.Tree, ChargeGoal: selection.GoalID,
				GroupID: id, ExecutionIdentity: "same-" + id, IdentityKnown: true, Status: "missing",
				DeclaredAllowanceMS: int64(cap) * 60000}
			if id == "u" {
				row.IdentityKnown, row.ExecutionIdentity, row.Status = false, "", "unknown"
			}
			if id == "fresh" {
				row.Reason = "fresh-episode-not-created"
			}
			if id == "b" {
				row.ExclusiveResources = []string{"shared-device"}
			}
			if id == "c" {
				row.ExclusiveResources = []string{"shared-device"}
			}
			out.Groups = append(out.Groups, row)
		}
		return out, nil
	}
	budget := func(_ string, _ batch.Unit, _ *batch.Unit, _ time.Time) (batchCostBudgetProjection, error) {
		return batchCostBudgetProjection{Budget: dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown, Limits: goal.Budget{
			ElapsedLimit: "4h", AttemptLimit: 8, ReservedJobMinutesLimit: 1000, ActiveJobLimit: 2}}}, nil
	}
	forecast, err := forecastBatchCostWith(root, record, nil, time.Now(), decision, run, budget)
	if err != nil {
		t.Fatal(err)
	}
	if forecast.Requests[0].ChargeGoal != "goal-c" || !forecast.Requests[0].NeedsAttempt ||
		forecast.Requests[1].ChargeGoal != "goal-a" || forecast.Requests[1].NeedsAttempt ||
		forecast.Requests[2].ChargeGoal != "goal-b" || !forecast.Requests[2].NeedsAttempt {
		t.Fatalf("tip-first per-owner demand: %+v", forecast.Requests)
	}
	if forecast.KnownMissing != 5 || forecast.UnknownMissing != 2 || forecast.DeclaredReservationMin != 4 ||
		!slices.Equal(forecast.ExclusiveResourceConflicts, []string{"shared-device"}) {
		t.Fatalf("identity/unknown/fresh/conflict accounting: %+v", forecast)
	}
	if forecast.Budgets[0].AttemptDemand != 0 || forecast.Budgets[1].AttemptDemand != 1 || forecast.Budgets[2].AttemptDemand != 1 {
		t.Fatalf("per-owner attempt demand: %+v", forecast.Budgets)
	}
}

func TestBatchCostReplacementFreshEpisodeChangesBindingAndNativeDemand(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("cap.min.proof.main.proof=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	units := []batch.Unit{{GoalID: "goal-a", Chain: "a", State: batch.UnitJoined},
		{GoalID: "goal-b", Chain: "b", State: batch.UnitJoined}, {GoalID: "goal-c", Chain: "c", State: batch.UnitJoined}}
	record := batch.Record{BaseTree: "base", TipTree: "tree-c", PrefixTrees: []string{"tree-a", "tree-b", "tree-c"}, Units: units,
		Seal: map[string]batch.Claim{"goal-a": {}, "goal-b": {}, "goal-c": {}}, PrefixEpisodes: map[string]batch.PrefixEpisode{}}
	decision := func(_ string, _ []batch.Unit, _ string) (batch.PrefixDecision, error) {
		return batch.PrefixDecision{Groups: []string{"shared"}, FreshRequired: true, FreshMaxAgeMS: 60000}, nil
	}
	for index, unit := range units {
		planned, err := decision(root, units[:index+1], record.PrefixTrees[index])
		if err != nil {
			t.Fatal(err)
		}
		id, err := batch.PrefixDecisionID(record.BaseTree, record.PrefixTrees[index], units[:index+1], record.Seal, planned)
		if err != nil {
			t.Fatal(err)
		}
		record.PrefixEpisodes[unit.GoalID] = batch.PrefixEpisode{DecisionID: id, Token: strings.Repeat(string(rune('a'+index)), 64),
			ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)}
	}
	run := func(_ string, selection costSelection, cap uint64) (costSelectionEvidence, error) {
		return costSelectionEvidence{Request: batch.CostForecastRequest{ID: selection.ID, Kind: selection.Kind,
			Tree: selection.Tree, ChargeGoal: selection.GoalID}, Groups: []batch.CostForecastGroup{{
			RequestID: selection.ID, Tree: selection.Tree, ChargeGoal: selection.GoalID,
			GroupID: "shared", ExecutionIdentity: "same-identity", IdentityKnown: true, Status: "missing",
			FreshEpisode: selection.FreshEpisode, DeclaredAllowanceMS: int64(cap) * 60000,
		}}}, nil
	}
	budget := func(_ string, _ batch.Unit, _ *batch.Unit, _ time.Time) (batchCostBudgetProjection, error) {
		return batchCostBudgetProjection{Budget: dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown,
			Limits: goal.Budget{ElapsedLimit: "4h", AttemptLimit: 8, ReservedJobMinutesLimit: 1000, ActiveJobLimit: 2}}}, nil
	}
	first, err := forecastBatchCostWith(root, record, nil, time.Now(), decision, run, budget)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Requests) != 3 || first.KnownMissing != 3 {
		t.Fatalf("different episodes coalesced native demand: %+v", first)
	}
	for _, row := range first.Groups {
		if row.CoveredBy != "" || row.FreshEpisode == "" {
			t.Fatalf("fresh work reused a different episode: %+v", row)
		}
	}
	replaced := record.PrefixEpisodes["goal-b"]
	replaced.Token = strings.Repeat("d", 64)
	record.PrefixEpisodes["goal-b"] = replaced
	second, err := forecastBatchCostWith(root, record, nil, time.Now(), decision, run, budget)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(first.Binding, second.Binding) || second.Binding.PrefixEpisodes[1].Token != replaced.Token ||
		second.KnownMissing != 3 || second.Requests[2].NeedsAttempt != true {
		t.Fatalf("replacement episode kept old forecast or coalesced work: first=%+v second=%+v", first, second)
	}
}

func TestBatchCostEarlierSharedIdentityChargesFirstPrefixOwner(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("cap.min.proof.main.proof=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	units := []batch.Unit{{GoalID: "goal-a", State: batch.UnitJoined}, {GoalID: "goal-b", State: batch.UnitJoined}, {GoalID: "goal-c", State: batch.UnitJoined}}
	record := batch.Record{BaseTree: "base", TipTree: "tip", PrefixTrees: []string{"a", "b", "tip"}, Units: units}
	decision := func(_ string, selected []batch.Unit, _ string) (batch.PrefixDecision, error) {
		if len(selected) == 3 {
			return batch.PrefixDecision{Groups: []string{"tip-only"}}, nil
		}
		return batch.PrefixDecision{Groups: []string{"shared"}}, nil
	}
	run := func(_ string, selection costSelection, cap uint64) (costSelectionEvidence, error) {
		return costSelectionEvidence{Request: batch.CostForecastRequest{ID: selection.ID, Kind: selection.Kind,
			Tree: selection.Tree, ChargeGoal: selection.GoalID}, Groups: []batch.CostForecastGroup{{
			RequestID: selection.ID, Tree: selection.Tree, ChargeGoal: selection.GoalID,
			GroupID: selection.Requirements[0], ExecutionIdentity: "identity-" + selection.Requirements[0],
			IdentityKnown: true, Status: "missing", DeclaredAllowanceMS: int64(cap) * 60000,
		}}}, nil
	}
	budget := func(_ string, unit batch.Unit, _ *batch.Unit, _ time.Time) (batchCostBudgetProjection, error) {
		attempts := uint64(1)
		if unit.GoalID == "goal-b" {
			attempts = 0
		}
		return batchCostBudgetProjection{Budget: dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown,
			Limits: goal.Budget{ElapsedLimit: "4h", AttemptLimit: attempts, ReservedJobMinutesLimit: 20, ActiveJobLimit: 1}}}, nil
	}
	forecast, err := forecastBatchCostWith(root, record, nil, time.Now(), decision, run, budget)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal([]string{forecast.Requests[0].ChargeGoal, forecast.Requests[1].ChargeGoal, forecast.Requests[2].ChargeGoal},
		[]string{"goal-c", "goal-a", "goal-b"}) || forecast.Budgets[0].AttemptDemand != 1 ||
		forecast.Budgets[1].AttemptDemand != 0 || !forecast.Budgets[1].Fits || forecast.Budgets[2].AttemptDemand != 1 {
		t.Fatalf("tip then original-order prefix charge disagrees with receipt compositor: requests=%+v budgets=%+v", forecast.Requests, forecast.Budgets)
	}
}

func TestBatchCostBudgetDoesNotBorrowAnotherGoalsHeadroom(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("cap.min.proof.main.proof=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	units := []batch.Unit{{GoalID: "goal-a", State: batch.UnitJoined}, {GoalID: "goal-b", State: batch.UnitJoined}}
	record := batch.Record{TipTree: "tip", Units: units}
	record.BaseTree, record.PrefixTrees = "base", []string{"prefix", "tip"}
	decision := func(_ string, _ []batch.Unit, _ string) (batch.PrefixDecision, error) {
		return batch.PrefixDecision{Groups: []string{"two-unknowns"}}, nil
	}
	run := func(_ string, selection costSelection, cap uint64) (costSelectionEvidence, error) {
		return costSelectionEvidence{Request: batch.CostForecastRequest{ID: selection.ID, Kind: selection.Kind,
			Tree: selection.Tree, ChargeGoal: selection.GoalID}, Groups: []batch.CostForecastGroup{
			{RequestID: selection.ID, Tree: selection.Tree, ChargeGoal: selection.GoalID, GroupID: "u1", Status: "unknown", DeclaredAllowanceMS: int64(cap) * 60000},
			{RequestID: selection.ID, Tree: selection.Tree, ChargeGoal: selection.GoalID, GroupID: "u2", Status: "unknown", DeclaredAllowanceMS: int64(cap) * 60000},
		}}, nil
	}
	budget := func(_ string, unit batch.Unit, _ *batch.Unit, _ time.Time) (batchCostBudgetProjection, error) {
		limits := goal.Budget{ElapsedLimit: "4h", AttemptLimit: 8, ReservedJobMinutesLimit: 1000, ActiveJobLimit: 2}
		if unit.GoalID == "goal-b" {
			limits.AttemptLimit = 0
		}
		return batchCostBudgetProjection{Budget: dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown, Limits: limits}}, nil
	}
	forecast, err := forecastBatchCostWith(root, record, nil, time.Now(), decision, run, budget)
	if err != nil {
		t.Fatal(err)
	}
	if forecast.Requests[0].ReservationMinutes != 2 || forecast.UnknownMissing != 4 ||
		forecast.Budgets[0].Fits != true || forecast.Budgets[1].Fits || forecast.Budgets[1].Reason != "attempt-headroom" {
		t.Fatalf("per-request cap and isolated budget: %+v", forecast)
	}
	unknownBudget := func(_ string, unit batch.Unit, _ *batch.Unit, _ time.Time) (batchCostBudgetProjection, error) {
		if unit.GoalID == "goal-b" {
			return batchCostBudgetProjection{Budget: dispatchcore.BudgetProjection{Status: dispatchcore.BudgetUnknown,
				Unknown: &dispatchcore.BudgetUnknownEvidence{Code: dispatchcore.BudgetUnknown,
					Record: "artifacts/agents/jobs/goal-b-unreadable.json", Reason: "retained job accounting is unreadable"}}}, nil
		}
		return batchCostBudgetProjection{Budget: dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown,
			Limits: goal.Budget{ElapsedLimit: "4h", AttemptLimit: 100, ReservedJobMinutesLimit: 10000, ActiveJobLimit: 10}}}, nil
	}
	unknown, err := forecastBatchCostWith(root, record, nil, time.Now(), decision, run, unknownBudget)
	if err != nil {
		t.Fatal(err)
	}
	if unknown.Budgets[0].Fits != true || unknown.Budgets[0].AttemptDemand != 1 ||
		unknown.Budgets[1].Fits || unknown.Budgets[1].Status != string(dispatchcore.BudgetUnknown) ||
		unknown.Budgets[1].AttemptDemand != 1 || !strings.Contains(unknown.Budgets[1].Reason, "goal-b-unreadable.json") ||
		forecastCostRefusal(unknown) == nil {
		t.Fatalf("unknown charged-goal budget borrowed another member's ample headroom: %+v", unknown.Budgets)
	}
}

func TestBatchCostUnknownJoinKeepsFirstRecordAbsentAndClosesOnlyExistingAdmission(t *testing.T) {
	t.Parallel()
	for _, existing := range []bool{false, true} {
		name := "first-member"
		if existing {
			name = "prospective-member"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			request, dependencies, admissionCalls := prepublicationJoinBed(t)
			request.GoalID = "goal-b"
			store := batch.NewStore(request.LandingRoot, nil)
			if existing {
				if err := store.Create(batch.Record{Schema: 1, BatchID: "01j5x00000000000000000ba99", State: batch.StateOpen,
					BaseTree: "base-tree", TipTree: "existing-tree", PrefixTrees: []string{"existing-tree"},
					Units: []batch.Unit{{GoalID: "goal-a", Chain: "chain-a", State: batch.UnitJoined,
						Claim: batch.Claim{Machine: "seat-a", Lineage: "lineage-a", Epoch: 1, Revision: 2, AccountingRevision: 1}}}}); err != nil {
					t.Fatal(err)
				}
			}
			handedOver, ensured := false, 0
			dependencies.handover = func(batchJoinRequest, string, batch.Claim) error { handedOver = true; return nil }
			dependencies.ensure = func(string) error { ensured++; return nil }
			dependencies.publishAdmission = func(batch.Store, string, batch.Unit, string, time.Time,
				func(string, string, string) (testpolicy.Plan, error), func() error, batch.JoinAdmissionRun) error {
				return fmt.Errorf("unknown budget reached publication")
			}
			dependencies.costForecast = func(_ string, record batch.Record, incoming batch.Unit, _ time.Time,
				_ func(string, string, string) (testpolicy.Plan, error), _ func(string, string, []batch.Unit) ([]string, error)) (batch.Unit, batch.CostForecast, error) {
				incoming.SelectedGroups = []string{"c"}
				units := append(slices.Clone(record.Units), incoming)
				prefixes := append(slices.Clone(record.PrefixTrees), "candidate-tree")
				budgets := []batch.CostForecastBudget{}
				if existing {
					budgets = append(budgets, batch.CostForecastBudget{GoalID: "goal-a", Status: string(dispatchcore.BudgetKnown), Fits: true,
						AttemptsLeft: 100, ReservedMinutesLeft: 10000})
				}
				budgets = append(budgets, batch.CostForecastBudget{GoalID: incoming.GoalID, Status: string(dispatchcore.BudgetUnknown),
					Reason: "artifacts/agents/jobs/goal-b-unreadable.json: retained job accounting is unreadable"})
				return incoming, batch.CostForecast{SchemaVersion: 1, Currency: "snapshot-not-revalidated",
					Binding: batch.CostBinding(record, units, prefixes), Budgets: budgets}, nil
			}
			_, err := executeBatchJoin(request, dependencies)
			if err == nil || !strings.Contains(err.Error(), "BATCH_COST_HEADROOM_REFUSED") || handedOver || *admissionCalls != 0 {
				t.Fatalf("unknown budget crossed source ownership or native admission: err=%v handover=%t admission=%d", err, handedOver, *admissionCalls)
			}
			records, err := store.Records()
			if err != nil {
				t.Fatal(err)
			}
			if !existing {
				if len(records) != 0 || ensured != 0 {
					t.Fatalf("unknown first member created an empty batch: records=%+v ensured=%d", records, ensured)
				}
				return
			}
			if len(records) != 1 || len(records[0].Units) != 1 || records[0].Units[0].GoalID != "goal-a" ||
				records[0].ClosedReason != "budget-cost" || ensured != 1 {
				t.Fatalf("unknown prospective member moved custody or missed existing closure: records=%+v ensured=%d", records, ensured)
			}
		})
	}
}

func TestBatchCostElapsedFollowsLandingReadyAdmission(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("cap.min.proof.main.proof=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unit := batch.Unit{GoalID: "goal-ready", State: batch.UnitJoined}
	record := batch.Record{BaseTree: "base", TipTree: "tip", PrefixTrees: []string{"tip"}, Units: []batch.Unit{unit}}
	decision := func(_ string, _ []batch.Unit, _ string) (batch.PrefixDecision, error) {
		return batch.PrefixDecision{Groups: []string{"native"}}, nil
	}
	run := func(_ string, selection costSelection, cap uint64) (costSelectionEvidence, error) {
		return costSelectionEvidence{Request: batch.CostForecastRequest{ID: selection.ID, Kind: selection.Kind,
			Tree: selection.Tree, ChargeGoal: selection.GoalID}, Groups: []batch.CostForecastGroup{{
			RequestID: selection.ID, Tree: selection.Tree, ChargeGoal: selection.GoalID,
			GroupID: "native", Status: "unknown", DeclaredAllowanceMS: int64(cap) * 60000,
		}}}, nil
	}
	projection := dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown,
		Limits:  goal.Budget{ElapsedLimit: "1h", AttemptLimit: 2, ReservedJobMinutesLimit: 20, ActiveJobLimit: 1},
		Elapsed: 2 * time.Hour, ElapsedState: dispatchcore.ElapsedBreach}
	for _, test := range []struct {
		name         string
		landingReady bool
		fits         bool
	}{
		{name: "ordinary claim elapsed", fits: false},
		{name: "landing-ready elapsed suspended", landingReady: true, fits: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			budget := func(_ string, _ batch.Unit, _ *batch.Unit, _ time.Time) (batchCostBudgetProjection, error) {
				return batchCostBudgetProjection{Budget: projection, LandingClaim: test.landingReady}, nil
			}
			forecast, err := forecastBatchCostWith(root, record, nil, time.Now(), decision, run, budget)
			if err != nil {
				t.Fatal(err)
			}
			if len(forecast.Budgets) != 1 || forecast.Budgets[0].Fits != test.fits ||
				(test.fits && forecast.Budgets[0].AttemptDemand != 1) {
				t.Fatalf("forecast disagreed with landing-ready admission: %+v", forecast.Budgets)
			}
		})
	}
}
