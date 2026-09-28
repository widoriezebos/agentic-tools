package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatchproc"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

func TestGoalRevisionAdmissionCommandMarksThenEnforcesWithExplicitDispatchContext(t *testing.T) {
	now := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	repository := newProofAdmissionRepositoryFixture(t, now, false)
	root, reads := repository.root, repository.reads()
	commandNow := repository.commandNow(now)
	repository.amend(t, "standing-validation", func(file *goal.GoalFile) {
		file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1}
		file.Risk = nil
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	want := "RISK_UNANSWERED goal=standing-validation tier=3 next: goal edit --risk"
	verdict, err := goalRevisionAdmissionVerdict(root, 5, "implementer", reads, commandNow)
	if err != nil || !verdict.Refused() || verdict.PolicyRefusal != want {
		t.Fatalf("unanswered-risk admission did not refuse: %+v err=%v", verdict, err)
	}
	repository.amend(t, "standing-validation", func(file *goal.GoalFile) {
		file.Risk = &goal.RiskRecord{Severity: 3, Novelty: 3, Exposure: 1, Accumulation: 1, Basis: "The fixture answers every risk question."}
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	if verdict, err := goalRevisionAdmissionVerdict(root, 5, "implementer", reads, commandNow); err != nil || verdict.Refused() || verdict.PolicyNotice != "" {
		t.Fatalf("answered-risk admission was not admitted: %+v err=%v", verdict, err)
	}
}

// goalRevisionAdmissionVerdict runs the dispatch admission owner the internal
// job goal-revision-admission printed, for standing-validation revision 2, a
// fresh dispatch and MECHANICAL reach, at the command clock.
func goalRevisionAdmissionVerdict(root string, proposedCap uint64, role string, reads dispatchcore.ProofAdmissionReads, commandNow func(string) (time.Time, error)) (dispatchcore.GoalRevisionAdmission, error) {
	now, err := commandNow(root)
	if err != nil {
		return dispatchcore.GoalRevisionAdmission{}, err
	}
	return dispatchcore.EvaluateGoalRevisionAdmissionForDispatchWithReads(root, "standing-validation", 2, proposedCap, now, role, "fresh", reads, dispatchcore.HazardMechanical)
}

// An abandoned breach-stopped goal cannot enter command budget admission.
func TestGoalRevisionAdmissionCommandRefusesAbandonedBreachStoppedGoalBeforeBudget(t *testing.T) {
	now := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	repository := newProofAdmissionRepositoryFixture(t, now, false)
	root, reads := repository.root, repository.reads()
	commandNow := repository.commandNow(now)
	file := repository.goalFile(t, "standing-validation")
	closedAt := "2026-08-30T08:07:00Z"
	abandonedAt := "2026-08-30T08:08:00Z"
	stopID := "stop-standing-validation-r2-f1"
	file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1, FenceEpoch: 1}
	file.StopFence = &goal.StopFence{StopID: stopID, Revision: 2, Epoch: 1, CapabilityGeneration: 2, ClosedAt: closedAt, Reason: goal.StopReasonElapsedLimit}
	file.History = append(file.History, goal.HistoryLine{
		At: closedAt, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAC", "mac-cli", "m1"),
		Verb: "breach-stop", Actor: "mac-cli+m1", Targets: []string{file.Id}, Keep: -1,
	})
	displaced := "mac-cli+m1@" + file.Claimed.At
	abandonOpid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAD", "mac-cli", "m1")
	reason := "The stopped work will not resume."
	file.State = goal.StateAbandoned
	file.Claimed = nil
	file.Revision = 5
	file.Abandoned = &goal.AbandonRecord{By: "human:Wido", At: abandonedAt, Revision: 5, Opid: abandonOpid, Displaced: displaced, StopID: stopID, Because: reason}
	file.History = append(file.History, goal.HistoryLine{
		At: abandonedAt, Opid: abandonOpid, Verb: "abandon", Actor: "human:Wido",
		Targets: []string{file.Id}, Displaced: displaced, StopID: stopID, Keep: -1, Reason: reason,
	})
	opid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAE", "mac-cli", "m1")
	parent, err := repository.Capture(opid)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := repository.Build(opid, parent, []goal.Change{
		{Path: "plans/goals/standing-validation.md", Delete: true},
		{Path: "records/goals/standing-validation.md", Content: goal.RenderFile(file)},
	}, "abandoned breach-stopped admission fixture")
	if err != nil {
		t.Fatal(err)
	}
	if commit == parent {
		t.Fatal("archive did not create a distinct accepted tree")
	}
	if outcome, err := repository.Publish(parent, commit); err != nil || outcome != goal.CASLanded {
		t.Fatalf("publish archived goal: outcome=%v err=%v", outcome, err)
	}
	if err := repository.AcceptedCAS(parent, commit); err != nil {
		t.Fatal(err)
	}
	endpoint, err := reads.ResolveEndpoint(root)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		t.Fatalf("accepted abandoned fixture is invalid: %v", err)
	}
	archived := projection.Tree.Abandoned[file.Id]
	if projection.Tree.Live[file.Id] != nil || archived == nil || archived.State != goal.StateAbandoned || archived.Claimed != nil || archived.StopCapability == nil || archived.StopFence == nil || archived.StopFence.StopID != stopID || archived.Abandoned == nil || archived.Abandoned.StopID != stopID {
		t.Fatalf("accepted ledger does not contain the abandoned stopped goal: %+v", archived)
	}
	verdict, err := goalRevisionAdmissionVerdict(root, 1, "implementer", reads, commandNow)
	if err == nil || !strings.Contains(err.Error(), "not a claimed accepted goal") || strings.Contains(err.Error(), "BUDGET_") || verdict.Refused() {
		t.Fatalf("abandoned goal reached budget admission: %+v err=%v", verdict, err)
	}
}

func TestGoalRevisionAdmissionCommandRefusesExhaustedCodeCriticClass(t *testing.T) {
	now := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	repository := newProofAdmissionRepositoryFixture(t, now, false)
	root, reads := repository.root, repository.reads()
	commandNow := repository.commandNow(now)
	repository.amend(t, "standing-validation", func(file *goal.GoalFile) {
		file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1}
		file.Budget.AttemptLimit = 20
		file.Budget.ReservedJobMinutesLimit = 1000
		file.Budget.ActiveJobLimit = 10
		file.Budget.ReviewRoundLimit = 2
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, job := range []string{"code-one", "code-two"} {
		writeTemp(t, jobs, job+".json", map[string]any{
			"jobId": job, "operationId": job, "role": "code-critic", "parentJob": nil,
			"goalId": "standing-validation", "goalRevision": 2, "capMin": 1, "status": "completed",
			"reviewChainCounted": true,
		})
	}
	verdict, err := goalRevisionAdmissionVerdict(root, 1, "code-critic", reads, commandNow)
	refusal := strings.Join(dispatchcore.FormatGoalRevisionAdmission(verdict), "\n")
	if err != nil || !verdict.Refused() || !strings.Contains(refusal, "codeCritiques=2/2") {
		t.Fatalf("admission did not refuse the exhausted code-critic class: err=%v lines=%q", err, refusal)
	}
}

func TestGoalRevisionAdmissionCommandJSONCarriesBudgetExtensionOffer(t *testing.T) {
	now := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	repository, _ := proofAdmissionExtensionFixture(t)
	root, reads := repository.root, repository.reads()
	config := []byte("metasystem.runtimes=fake\nmetasystem.governance.correlation-policy=A\nmetasystem.budget.tier-3=8h/1/1200m/1/3\n")
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), config, 0o644); err != nil {
		t.Fatal(err)
	}
	receipt := fmt.Sprintf("%d|%s|RECEIPT|type=implement|outcome=shipped|goal=standing-validation|note=fixture\n",
		now.Add(-time.Hour).Unix(), now.Add(-time.Hour).Format(time.RFC3339))
	repository.seed(map[string][]byte{"metasystem/metasystem.conf": config, "metasystem/memory/receipts.log": []byte(receipt)})
	if got := string(repository.rawFile(t, "metasystem/memory/receipts.log")); got != receipt {
		t.Fatalf("accepted receipt = %q, want %q", got, receipt)
	}
	inputs := repository.extendBudgetInputs(t)
	beforeTip, _, err := repository.Accepted()
	if err != nil {
		t.Fatal(err)
	}
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, jobs, "spent.json", map[string]any{
		"jobId": "spent", "operationId": "spent", "goalId": "standing-validation", "goalRevision": 2,
		"capMin": 1, "status": "completed", "startedAt": "2026-08-30T08:20:00Z", "endedAt": "2026-08-30T08:21:00Z",
		"pid": 4242,
	})
	verdict, err := goalRevisionAdmissionVerdict(root, 1, "implementer", reads, repository.commandNow(now))
	if err != nil || !verdict.Refused() {
		t.Fatalf("offer admission was not a refusal: %+v err=%v", verdict, err)
	}
	encoded, err := json.Marshal(verdict)
	var decoded dispatchcore.GoalRevisionAdmission
	if err != nil || json.Unmarshal(encoded, &decoded) != nil || decoded.Extension == nil || decoded.Extension.EvidenceKind != "landing" {
		t.Fatalf("the verdict's JSON lost the offer: %+v err=%v output=%s", decoded, err, encoded)
	}

	parent, state, err := (identity.KernelProber{}).Probe(int64(os.Getppid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe budget extension caller: state=%s err=%v", state, err)
	}
	if _, err := lease.AnnounceWithPair(root, "budget-extension-main", parent.Pid, parent.StartedAt.Unix(),
		parent.StartTicks, parent.BootID, "mac-cli", "fake", "m1"); err != nil {
		t.Fatal(err)
	}
	holder, err := lease.ClassifyVerb(root, parent.Pid)
	if err != nil || holder.Class != lease.ClassMain || !holder.Holder || holder.Announcement == nil || holder.Announcement.OwnerLineage != "m1" {
		t.Fatalf("budget extension caller is not the m1 MAIN holder: holder=%+v err=%v", holder, err)
	}
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "m1")
	extendArgs := []string{"--root", root, "--id", "standing-validation", "--revision", "2", "--proposed-cap", "1",
		"--role", "implementer", "--dispatch-mode", "fresh", "--destructive-reach", "MECHANICAL"}
	extended, extendCode := captureStdout(t, func() int {
		return runGoalExtendBudgetWithInputs(extendArgs, repository.commandNow(now), inputs, reads)
	})
	if extendCode != 0 || !strings.Contains(extended, `"outcome":"confirmed"`) {
		t.Fatalf("extend-budget command did not replay and apply the offer: code=%d output=%s", extendCode, extended)
	}
	tip, _, err := repository.Accepted()
	if err != nil || tip == beforeTip {
		t.Fatalf("accepted tip did not advance: before=%s after=%s err=%v", beforeTip, tip, err)
	}
	record := string(repository.rawFile(t, "metasystem/plans/goals/standing-validation.md"))
	if !strings.Contains(record, "- BudgetExtension: ") || !strings.Contains(record, "attemptLimit=1->2") {
		t.Fatalf("extend-budget command did not persist its marker: %s", record)
	}
	if strings.Count(record, "- BudgetExtension: ") != 1 {
		t.Fatalf("extend-budget command wrote multiple markers: %s", record)
	}

	writeTemp(t, jobs, "spent-again.json", map[string]any{
		"jobId": "spent-again", "operationId": "spent-again", "goalId": "standing-validation", "goalRevision": 2,
		"capMin": 1, "status": "completed", "startedAt": "2026-08-30T08:30:00Z", "endedAt": "2026-08-30T08:31:00Z",
	})
	second, secondCode := captureStderr(t, func() int {
		return runGoalExtendBudgetWithInputs(extendArgs, repository.commandNow(now), inputs, reads)
	})
	if secondCode != 1 || !strings.Contains(second, "extended once at 2026-08-30T09:00:00Z") {
		t.Fatalf("second extend-budget command did not name its marker: code=%d output=%s", secondCode, second)
	}
	secondTip, _, err := repository.Accepted()
	if err != nil || secondTip != tip || string(repository.rawFile(t, "metasystem/plans/goals/standing-validation.md")) != record {
		t.Fatalf("once-only refusal changed the accepted goal: before=%s after=%s err=%v", tip, secondTip, err)
	}
}

func TestGoalExtendBudgetRefusesSeamsThatAreNotExtendable(t *testing.T) {
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "m1")
	now := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	baseArgs := func(root string) []string {
		return []string{"--root", root, "--id", "standing-validation", "--revision", "2", "--proposed-cap", "1",
			"--role", "implementer", "--dispatch-mode", "fresh", "--destructive-reach", "MECHANICAL"}
	}
	t.Run("zero proposed cap", func(t *testing.T) {
		repository := newProofAdmissionRepositoryFixture(t, now, false)
		root := repository.root
		args := baseArgs(root)
		for index := range args {
			if args[index] == "--proposed-cap" {
				args[index+1] = "0"
			}
		}
		output, code := captureStderr(t, func() int {
			return runGoalExtendBudgetWithInputs(args, repository.commandNow(now), repository.extendBudgetInputs(t), repository.reads())
		})
		if code != 2 || !strings.Contains(output, "positive --proposed-cap") {
			t.Fatalf("zero-cap extension refusal: code=%d output=%s", code, output)
		}
	})
	t.Run("admitted", func(t *testing.T) {
		repository := newProofAdmissionRepositoryFixture(t, now, false)
		root := repository.root
		repository.amend(t, "standing-validation", func(file *goal.GoalFile) {
			file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1}
		})
		inputs := repository.extendBudgetInputs(t)
		output, code := captureStderr(t, func() int {
			return runGoalExtendBudgetWithInputs(baseArgs(root), repository.commandNow(now), inputs, repository.reads())
		})
		if code != 1 || !strings.Contains(output, "is admitted; there is no budget refusal to extend") {
			t.Fatalf("admitted seam refusal: code=%d output=%s", code, output)
		}
	})
	t.Run("active job", func(t *testing.T) {
		repository := newProofAdmissionRepositoryFixture(t, now, false)
		root := repository.root
		repository.amend(t, "standing-validation", func(file *goal.GoalFile) {
			file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1}
			file.Budget.AttemptLimit = 20
			file.Budget.ReservedJobMinutesLimit = 10000
			file.Budget.ActiveJobLimit = 1
			file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
		})
		jobs := filepath.Join(root, "artifacts", "agents", "jobs")
		if err := os.MkdirAll(jobs, 0o755); err != nil {
			t.Fatal(err)
		}
		writeTemp(t, jobs, "active.json", map[string]any{
			"jobId": "active", "operationId": "active", "goalId": "standing-validation", "goalRevision": 2,
			"capMin": 1, "status": "running",
		})
		inputs := repository.extendBudgetInputs(t)
		output, code := captureStderr(t, func() int {
			return runGoalExtendBudgetWithInputs(baseArgs(root), repository.commandNow(now), inputs, repository.reads())
		})
		if code != 1 || !strings.Contains(output, "activeJobLimit") || !strings.Contains(output, "no consumption-earned budget extension offer") {
			t.Fatalf("active-job seam refusal: code=%d output=%s", code, output)
		}
	})
	t.Run("live stop", func(t *testing.T) {
		repository := newProofAdmissionRepositoryFixture(t, now, false)
		root := repository.root
		repository.amend(t, "standing-validation", func(file *goal.GoalFile) {
			file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1}
			file.Budget.ElapsedLimit = "1h"
			file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
		})
		inputs := repository.extendBudgetInputs(t)
		output, code := captureStderr(t, func() int {
			return runGoalExtendBudgetWithInputs(baseArgs(root), repository.commandNow(now.Add(time.Hour)), inputs, repository.reads())
		})
		if code != 1 || !strings.Contains(output, "names a live stop, not an extendable exhaustion") {
			t.Fatalf("live-stop seam refusal: code=%d output=%s", code, output)
		}
	})
}

func TestCommandTaggedProcessScannerUsesAuthorizedCompleteFixtureTable(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	processes := filepath.Join(root, "processes.json")
	if err := os.WriteFile(processes, []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", processes)
	result := (dispatchproc.TaggedProcessScanner{Root: root}).ScanTag("reservation-tag", time.Now())
	if !result.Complete() || result.EnumerationError != "" || len(result.Tagged) != 0 {
		t.Fatalf("complete empty fixture table = %+v", result)
	}
}

// writeTemp writes a JSON file and returns its path.
func writeTemp(t *testing.T, dir, name string, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// captureStdout runs fn with stdout redirected and returns what it printed.
func captureStdout(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	code, stdout, _ := captureCommandOutput(t, true, false, fn)
	return stdout, code
}

func TestCommandTaggedProcessScannerHonorsEmptyConfiguredUniverse(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	processes := writeTemp(t, t.TempDir(), "processes.json", []any{})
	identities := writeTemp(t, t.TempDir(), "identities.json", map[string]any{})
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", processes)
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", identities)

	result := (dispatchproc.TaggedProcessScanner{Root: root}).ScanTag("metasystem-job-empty-nonce", time.Time{})
	if !result.Complete() || result.EnumerationError != "" || len(result.Tagged) != 0 {
		t.Fatalf("empty configured process universe was not a complete absence proof: %+v", result)
	}
}

func TestDispatchCritiqueAdvanceVerbsPath(t *testing.T) {
	repo := t.TempDir()
	agents := filepath.Join(repo, "artifacts", "agents")
	jobs := filepath.Join(agents, "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	facts := map[string]any{
		"local": true, "recoverable": true,
		"proofBoundaryCrossed": false, "authorityBoundaryCrossed": false,
		"secretsBoundaryCrossed": false, "irreversibleDataBoundaryCrossed": false,
		"externalSideEffectBoundaryCrossed": false,
	}
	for round := 1; round <= 3; round++ {
		job := "critic"
		parent := any(nil)
		if round > 1 {
			job = "critic-r" + strconv.Itoa(round)
			if round == 2 {
				parent = "critic"
			} else {
				parent = "critic-r2"
			}
		}
		record := map[string]any{
			"jobId": job, "role": "code-critic", "round": round,
			"parentJob": parent, "status": "completed",
		}
		if round == 1 {
			record["findingRegister"] = []any{}
			record["findingRegisterRound"] = 0
			record["reviewRoundLimit"] = 3
			record["criticRoundsConsumed"] = 0
			record["demotions"] = []any{}
			record["critiqueExhaustions"] = []any{}
		}
		writeTemp(t, jobs, job+".json", record)
		findings := []any{}
		rigor := []any{}
		if round == 1 {
			findings = []any{map[string]any{
				"id": "S-1", "severity": "high", "material": true,
				"claim": "severe finding", "evidence": "direct evidence",
			}}
			rigor = []any{map[string]any{
				"findingId": "S-1", "rigorClass": "severe", "facts": facts,
				"artifact":         "metasystem/test.go",
				"reopeningTrigger": "reopen if the defect recurs",
			}}
		}
		roundDir := filepath.Join(agents, "critic", "rounds", strconv.Itoa(round))
		if err := os.MkdirAll(roundDir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeTemp(t, roundDir, "return.json", map[string]any{
			"schemaVersion": 3, "jobId": job, "round": round,
			"findings": findings, "rigor": rigor,
		})
		if outcome, err := dispatchcore.CritiqueRegisterAdvance(repo, "critic", job); err != nil || outcome != "advanced" {
			t.Fatalf("register round %d: outcome=%q err=%v", round, outcome, err)
		}
	}
	if ids, err := dispatchcore.CritiqueOpenFindingIDs(repo, "critic"); err != nil || strings.Join(ids, ",") != "S-1" {
		t.Fatalf("open finding identifiers after the folds = %v, %v", ids, err)
	}
}

func TestDispatchCritiqueRegisterCloseKeepsRegisterlessCompatibility(t *testing.T) {
	repo := t.TempDir()
	jobs := filepath.Join(repo, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, jobs, "legacy-critic.json", map[string]any{
		"jobId": "legacy-critic", "role": "code-critic", "round": 1,
		"parentJob": nil, "status": "completed",
	})
	if outcome, err := dispatchcore.CritiqueRegisterClose(repo, "legacy-critic"); err != nil || outcome != "closed" {
		t.Fatalf("register-less close = outcome %q err %v", outcome, err)
	}
}
