package dispatch

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func candidateProofAdmission(request proofrun.AdmissionRequest) proofrun.AdmissionRequest {
	request.CandidateGoalID = request.GoalID
	request.CandidateRevision = request.AccountingRevision
	request.CandidateBudgetEpoch = request.BudgetEpoch
	if request.Identity.CommandClass == "testing" && request.CandidateTree == "" {
		request.CandidateTree = strings.Repeat("b", 40)
	}
	return dispatchFixtureProofAdmission(proofrun.WithTestHostLoadSampler(request, "0"))
}

func dispatchFixtureProofAdmission(request proofrun.AdmissionRequest) proofrun.AdmissionRequest {
	return proofrun.WithTestHostAdmissionDirectory(request, filepath.Join(request.ControlRoot, "artifacts", "agents", "host-admission"))
}

func requireProofReservationNotAdmissionRefused(t *testing.T, decision proofrun.LaunchResult) {
	t.Helper()
	if decision.Disposition == proofrun.DispositionAdmissionRefused {
		t.Fatalf("proof reservation fixture was admission-refused: %+v", decision)
	}
}

func shippedReceiptLine(at time.Time, goalID, kind, outcome string) string {
	return fmt.Sprintf("%d|%s|RECEIPT|type=%s|outcome=%s|goal=%s|note=fixture", at.Unix(), at.UTC().Format(time.RFC3339), kind, outcome, goalID)
}

func TestGoalRevisionAdmissionOffersOneLandingEarnedExtension(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	bed := newBudgetReceiptBed(t, 1, 10000, 10)
	root := bed.root
	bed.setReceipt(t, shippedReceiptLine(now.Add(-30*time.Minute), "bounded", "implement", "shipped")+"\n", 1)
	writeBudgetJob(t, root, "settled", "reserve-settled", 3, 1, "completed", budgetJobLife{
		startedAt: "2026-08-28T09:40:00Z", endedAt: "2026-08-28T09:41:00Z", pid: 4242,
	})

	verdict, err := bed.revisionAdmission("bounded", 3, 1, now)
	if err != nil || verdict.Extension == nil || verdict.Extension.EvidenceKind != "landing" {
		t.Fatalf("attempt exhaustion did not offer the landing-earned extension: %+v %v", verdict, err)
	}
	if verdict.Extension.From.AttemptLimit != 1 || verdict.Extension.To.AttemptLimit != 11 ||
		verdict.Extension.From.ReservedJobMinutesLimit != 10000 || verdict.Extension.To.ReservedJobMinutesLimit != 11200 {
		t.Fatalf("offer did not add the tier-three box: %+v", verdict.Extension)
	}
	lines := FormatGoalRevisionAdmission(verdict)
	if len(lines) != 1 || !strings.Contains(lines[0], "; extension available: landing ") || !strings.HasSuffix(lines[0], " at 2026-08-28T09:30:00Z") {
		t.Fatalf("refusal did not carry the offer: %v", lines)
	}
	binding, err := bed.binding("bounded", now)
	if err != nil {
		t.Fatal(err)
	}
	seat, err := bed.admission(binding.Lineage, now)
	if err != nil || !seat.Refused() {
		t.Fatalf("seat walk did not keep its ordinary refusal: %+v %v", seat, err)
	}
}

func TestLandingAdvancementFindsTemplateReceiptBelowGitRoot(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	top := t.TempDir()
	installation := filepath.Join(top, "metasystem")
	for _, directory := range []string{filepath.Join(top, "development"), filepath.Join(installation, "memory"), filepath.Join(installation, "plans", "goals")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(top, "development", "metasystem-design.md"), []byte("# template\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	line := shippedReceiptLine(now.Add(-time.Hour), "nested-goal", "implement", "shipped")
	if err := os.WriteFile(filepath.Join(installation, "memory", "receipts.log"), []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "plans", "goals", "backlog.md"), []byte("# fixture ledger\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	receiptPath, err := budgetExtensionReceiptPathWithReads(installation, receiptAdmissionReads{
		TopLevel: func(root string) (string, error) {
			if root != installation {
				t.Fatalf("repository top root = %q, want %q", root, installation)
			}
			return top, nil
		},
	})
	if err != nil || receiptPath != "metasystem/memory/receipts.log" {
		t.Fatalf("nested receipt path = %q, %v", receiptPath, err)
	}
	data, err := os.ReadFile(filepath.Join(installation, "memory", "receipts.log"))
	if err != nil {
		t.Fatal(err)
	}
	receipt := newStrictBudgetReceipt(t, installation, top, receiptPath)
	receipt.data = append([]byte(nil), data...)
	receipt.present = true
	receipt.want = 1
	evidence, err := landingAdvancementEvidenceWithReads(installation, "nested-goal", now, receipt.reads())
	if err != nil || len(evidence) != 1 || evidence[0].kind != "landing" {
		t.Fatalf("nested template receipt was not selected: evidence=%+v err=%v", evidence, err)
	}
}

func TestGoalRevisionAdmissionRejectsMalformedProposalCoordinates(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	if _, err := EvaluateGoalRevisionAdmissionForDispatch(root, "bounded", 3, 0, now, "implementer", "fresh", HazardMechanical); err == nil || !strings.Contains(err.Error(), "positive proposed cap") {
		t.Fatalf("zero-cap proposal was not refused: %v", err)
	}
	if _, err := EvaluateGoalRevisionAdmissionForDispatch(root, "bounded", 3, 1, now, "implementer", "retry", HazardMechanical); err == nil || !strings.Contains(err.Error(), "fresh or follow-up") {
		t.Fatalf("unknown dispatch mode was not refused: %v", err)
	}
}

func TestGoalRevisionAdmissionOffersForUsedPlusProposedMinutes(t *testing.T) {
	now := time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC)
	bed := newBudgetReceiptBed(t, 10, 240, 10)
	root := bed.root
	bed.setReceipt(t, shippedReceiptLine(now.Add(-time.Hour), "bounded", "implement", "shipped")+"\n", 1)
	writeBudgetJob(t, root, "settled", "reserve-settled", 3, 120, "completed", budgetJobLife{
		startedAt: "2026-08-28T09:35:00Z", endedAt: "2026-08-28T10:25:00Z", pid: 4242,
	})
	writeBudgetJob(t, root, "running", "reserve-running", 3, 120, "running", budgetJobLife{})
	verdict, err := bed.revisionAdmission("bounded", 3, 120, now)
	if err != nil || verdict.Extension == nil || len(verdict.Refusal.Breaches) != 1 || verdict.Refusal.Breaches[0].Field != "reservedJobMinutesLimit" {
		t.Fatalf("170+120 minute refusal did not offer: %+v %v", verdict, err)
	}
}

func TestGoalRevisionAdmissionOffersAtReservedMinuteLimit(t *testing.T) {
	now := time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC)
	bed := newBudgetReceiptBed(t, 10, 240, 10)
	root := bed.root
	bed.setReceipt(t, shippedReceiptLine(now.Add(-time.Hour), "bounded", "implement", "shipped")+"\n", 1)
	writeBudgetJob(t, root, "running", "reserve-running", 3, 240, "running", budgetJobLife{})
	verdict, err := bed.revisionAdmission("bounded", 3, 1, now)
	if err != nil || verdict.Extension == nil || len(verdict.Refusal.Breaches) != 1 ||
		verdict.Refusal.Breaches[0].Field != "reservedJobMinutesLimit" {
		t.Fatalf("minute exhaustion at the limit did not offer: %+v %v", verdict, err)
	}
}

func TestGoalRevisionAdmissionWithholdsOfferForOtherBoundaries(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	t.Run("mixed", func(t *testing.T) {
		bed := newBudgetReceiptBed(t, 1, 10000, 1)
		root := bed.root
		bed.setReceipt(t, shippedReceiptLine(now.Add(-time.Minute), "bounded", "implement", "shipped")+"\n", 0)
		writeBudgetJob(t, root, "running", "reserve-running", 3, 1, "running", budgetJobLife{})
		verdict, err := bed.revisionAdmission("bounded", 3, 1, now)
		if err != nil || verdict.Extension != nil || len(verdict.Refusal.Breaches) != 2 {
			t.Fatalf("mixed breach carried an extension: %+v %v", verdict, err)
		}
	})
	t.Run("elapsed stop", func(t *testing.T) {
		bed := newBudgetReceiptBed(t, 10, 10000, 10)
		bed.setReceipt(t, shippedReceiptLine(now.Add(-time.Minute), "bounded", "implement", "shipped")+"\n", 0)
		verdict, err := bed.revisionAdmission("bounded", 3, 1, now.Add(48*time.Hour))
		if err != nil || verdict.Extension != nil || verdict.LiveStopReason == "" {
			t.Fatalf("live stop carried an extension: %+v %v", verdict, err)
		}
	})
	t.Run("review round", func(t *testing.T) {
		bed := newReviewReceiptBed(t)
		root := bed.root
		bed.setReceipt(t, shippedReceiptLine(now.Add(-time.Minute), "bounded", "implement", "shipped")+"\n", 0)
		writeCountedCriticRootAtRevision(t, root, "critic-one", "code-critic", 2)
		writeCountedCriticRootAtRevision(t, root, "critic-two", "code-critic", 2)
		verdict, err := bed.revisionAdmissionForDispatch("bounded", 2, 1, now,
			"code-critic", "fresh", HazardMechanical)
		if err != nil || verdict.Extension != nil || verdict.Refusal == nil || len(verdict.Refusal.Breaches) != 1 ||
			verdict.Refusal.Breaches[0].Field != "codeCritiques" {
			t.Fatalf("review-round refusal carried an extension: %+v %v", verdict, err)
		}
	})
}

func TestGoalRevisionAdmissionNamesStandingExtensionMarker(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	bed := newBudgetReceiptBed(t, 1, 10000, 10)
	root := bed.root
	path := filepath.Join(root, "plans", "goals", "bounded.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	markerOpid := "01ARZ3NDEKTSV4RRFFQ69G5FAZ-bed-m1-00000004"
	file.Revision++
	file.Budget.AttemptLimit = 2
	file.Budget.ReservedJobMinutesLimit = 11200
	file.History = append(file.History, goal.HistoryLine{At: now.Format(time.RFC3339), Opid: markerOpid,
		Verb: "extend-budget", Actor: "bed-m1+coordinator", Targets: []string{"bounded"}, Keep: -1})
	file.BudgetExtension = &goal.BudgetExtensionRecord{At: now.Format(time.RFC3339), Opid: markerOpid,
		AttemptLimitFrom: 1, AttemptLimitTo: 2, ReservedJobMinutesFrom: 10000, ReservedJobMinutesTo: 11200,
		EvidenceKind: "landing", EvidenceID: "fixture-receipt", EvidenceAt: now.Add(-time.Hour).Format(time.RFC3339)}
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.accept(t)
	for index := 1; index <= 2; index++ {
		name := fmt.Sprintf("spent-%d", index)
		writeBudgetJob(t, root, name, name, 3, 1, "completed", budgetJobLife{
			startedAt: "2026-08-28T09:40:00Z", endedAt: "2026-08-28T09:41:00Z",
		})
	}
	verdict, err := bed.revisionAdmission("bounded", 3, 1, now)
	lines := FormatGoalRevisionAdmission(verdict)
	if err != nil || verdict.Extension != nil || len(lines) != 1 ||
		!strings.Contains(lines[0], "; extended once at 2026-08-28T10:00:00Z; a further raise is a person's set-budget") {
		t.Fatalf("standing marker was not named: verdict=%+v lines=%v err=%v", verdict, lines, err)
	}
}

func TestLandingEvidenceAppliesCorrectionsAndWindow(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	bed := newBudgetReceiptBed(t, 10, 10000, 10)
	root := bed.root
	original := shippedReceiptLine(now.Add(-time.Hour), "bounded", "implement", "shipped")
	digest := fmt.Sprintf("%x", sha1.Sum([]byte(original)))
	correction := fmt.Sprintf("%d|%s|CORRECTION|ref_epoch=%d|ref_sha1=%s|field=goal|was=bounded|now=other|reason=fixture",
		now.Unix(), now.Format(time.RFC3339), now.Add(-time.Hour).Unix(), digest)
	content := strings.Join([]string{
		original, correction,
		shippedReceiptLine(now.Add(-30*time.Minute), "bounded", "design", "shipped"),
		shippedReceiptLine(now.Add(-20*time.Minute), "bounded", "implement", "parked"),
		shippedReceiptLine(now.Add(-3*time.Hour), "bounded", "implement", "shipped"),
	}, "\n") + "\n"
	bed.setReceipt(t, content, 1)
	evidence, err := landingAdvancementEvidenceWithReads(root, "bounded", now, bed.reads.Receipt)
	if err != nil || len(evidence) != 0 {
		t.Fatalf("weak landing shapes became advancement: %+v %v", evidence, err)
	}
}

func TestLandingEvidenceCorrectionAppliesToDuplicateIdentity(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	original := shippedReceiptLine(now.Add(-time.Hour), "bounded", "implement", "shipped")
	digest := fmt.Sprintf("%x", sha1.Sum([]byte(original)))
	correction := fmt.Sprintf("%d|%s|CORRECTION|ref_epoch=%d|ref_sha1=%s|field=goal|was=bounded|now=other|reason=fixture",
		now.Unix(), now.Format(time.RFC3339), now.Add(-time.Hour).Unix(), digest)
	for name, lines := range map[string][]string{
		"correction after duplicates":  {original, original, correction},
		"duplicate after correction":   {original, correction, original},
		"correction before duplicates": {correction, original, original},
	} {
		t.Run(name, func(t *testing.T) {
			bed := newBudgetReceiptBed(t, 10, 10000, 10)
			root := bed.root
			bed.setReceipt(t, strings.Join(lines, "\n")+"\n", 1)
			evidence, err := landingAdvancementEvidenceWithReads(root, "bounded", now, bed.reads.Receipt)
			if err != nil || len(evidence) != 0 {
				t.Fatalf("correction left a duplicate receipt as advancement: %+v %v", evidence, err)
			}
		})
	}
}

func TestClosedCritiqueEvidenceRequiresClosedFoldedCompletedChain(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	writeJSON(t, filepath.Join(jobs, "work-root.json"), map[string]any{
		"jobId": "work-root", "parentJob": nil, "goalId": "bounded", independentCritiqueReferenceField: "critic-root",
	})
	critic := map[string]any{
		"jobId": "critic-root", "parentJob": nil, "goalId": "bounded", "role": "code-critic", "round": 1,
		"status": "completed", "endedAt": now.Add(-time.Hour).Format(time.RFC3339), "chainClosed": true,
		findingRegisterField: []any{}, findingRegisterRoundField: 1,
	}
	writeJSON(t, filepath.Join(jobs, "critic-root.json"), critic)
	evidence := reviewAdvancementEvidence(root, "bounded", now)
	if len(evidence) != 1 || evidence[0].id != "critic-root" {
		t.Fatalf("closed critique was not evidence: %+v", evidence)
	}
	critic["chainClosed"] = false
	writeJSON(t, filepath.Join(jobs, "critic-root.json"), critic)
	if evidence := reviewAdvancementEvidence(root, "bounded", now); len(evidence) != 0 {
		t.Fatalf("open critique pointer was evidence: %+v", evidence)
	}
}

func validDeliveryTestResult(attemptID string) proofrun.TestResult {
	zero := 0
	digest := strings.Repeat("a", 64)
	result := proofrun.TestResult{
		SchemaVersion: 1, CandidateEngineIdentityVersion: proofrun.CandidateEngineIdentitySchemaVersion,
		AttemptID: attemptID, Purpose: testpolicy.PurposeDelivery, RequestedMode: "auto", RequiredMode: "standard", ExecutedMode: "standard",
		ProjectRoot: "/project", BaseCommit: "base", CandidateTree: strings.Repeat("b", 40), ContractDigest: digest,
		BaseContractDigest: digest, PolicyEngineDigest: digest, CandidateEngineDigest: digest,
		CandidateEngineBuildIdentity: strings.Repeat("c", 40), BehaviorPolicyDigest: digest, PlanDigest: digest,
		SelectedGroups: []string{"smoke"}, RequiredGroups: []string{"smoke"}, LaunchCounts: proofrun.LaunchCounts{CountsComplete: true},
		Cost: proofrun.TestCost{DeclaredTargetMS: 1}, Groups: []proofrun.GroupResult{{ID: "smoke", Kind: "unit", InputManifest: []string{"source"},
			Status: "passed", NativeLaunched: true, CollectionComplete: true, NativeExitStatus: &zero,
			ToolIdentities: map[string]string{}, ReportDigests: map[string]string{}}},
	}
	result.RecomputeDelivery()
	return result
}

func TestProofDeliveryEvidenceRequiresSufficientRecentSuccess(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	result := validDeliveryTestResult("proof-one")
	attempt := proofrun.Attempt{AttemptID: "proof-one", GoalID: "bounded", EndedAt: now.Add(-time.Hour).Format(time.RFC3339),
		Terminal: &proofrun.AttemptTerminal{Result: proofrun.TerminalSuccess, At: now.Add(-time.Hour).Format(time.RFC3339)}, TestResult: &result}
	if evidence := proofAttemptAdvancement(attempt, "bounded", now); evidence == nil || evidence.kind != "receipt" {
		t.Fatalf("sufficient delivery success was not evidence: %+v", evidence)
	}
	insufficient := result
	insufficient.Delivery.Sufficient = false
	attempt.TestResult = &insufficient
	if evidence := proofAttemptAdvancement(attempt, "bounded", now); evidence != nil {
		t.Fatalf("terminal success with insufficient delivery was evidence: %+v", evidence)
	}
	attempt.TestResult = &result
	attempt.Terminal.At = now.Add(-30 * time.Minute).Format(time.RFC3339)
	if evidence := proofAttemptAdvancement(attempt, "bounded", now); evidence != nil {
		t.Fatalf("proof attempt with contradictory terminal time was evidence: %+v", evidence)
	}
	attempt.Terminal.At = attempt.EndedAt
	attempt.EndedAt = now.Add(-3 * time.Hour).Format(time.RFC3339)
	attempt.Terminal.At = attempt.EndedAt
	if evidence := proofAttemptAdvancement(attempt, "bounded", now); evidence != nil {
		t.Fatalf("old proof attempt was evidence: %+v", evidence)
	}
}

func TestExtensionEvidenceFollowsTheCandidate(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	result := validDeliveryTestResult("proof-candidate")
	attempt := proofrun.Attempt{SchemaVersion: proofrun.CandidateAttemptSchemaVersion, AttemptID: "proof-candidate",
		GoalID: "authority-c", CandidateGoalID: "candidate-x", EndedAt: now.Add(-time.Hour).Format(time.RFC3339Nano),
		Terminal: &proofrun.AttemptTerminal{Result: proofrun.TerminalSuccess, At: now.Add(-time.Hour).Format(time.RFC3339Nano)}, TestResult: &result}
	if evidence := proofAttemptAdvancement(attempt, "candidate-x", now); evidence == nil || evidence.id != attempt.AttemptID {
		t.Fatalf("candidate did not receive its proof advancement: %+v", evidence)
	}
	if evidence := proofAttemptAdvancement(attempt, "authority-c", now); evidence != nil {
		t.Fatalf("authority received candidate-owned proof advancement: %+v", evidence)
	}
}

func TestGoalRevisionAdmissionOffersWhenBothConsumptionMembersBreach(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	bed := newBudgetReceiptBed(t, 1, 1, 10)
	root := bed.root
	bed.setReceipt(t, shippedReceiptLine(now.Add(-30*time.Minute), "bounded", "implement", "shipped")+"\n", 1)
	writeBudgetJob(t, root, "settled", "reserve-settled", 3, 1, "completed", budgetJobLife{
		startedAt: "2026-08-28T09:40:00Z", endedAt: "2026-08-28T09:41:00Z", pid: 4242,
	})
	verdict, err := bed.revisionAdmission("bounded", 3, 1, now)
	if err != nil || verdict.Refusal == nil || len(verdict.Refusal.Breaches) != 2 || verdict.Extension == nil {
		t.Fatalf("attempts and minutes exhausted together did not offer: %+v %v", verdict, err)
	}
	for _, breach := range verdict.Refusal.Breaches {
		if breach.Field != "attemptLimit" && breach.Field != "reservedJobMinutesLimit" {
			t.Fatalf("the both-members fixture breached another member: %+v", verdict.Refusal.Breaches)
		}
	}
}

func TestGoalRevisionAdmissionWithholdsOfferWithoutEvidenceOrWhenTheRaiseWouldNotAdmit(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	t.Run("no evidence", func(t *testing.T) {
		bed := newBudgetReceiptBed(t, 1, 10000, 10)
		root := bed.root
		bed.setReceipt(t, "", 1)
		writeBudgetJob(t, root, "settled", "reserve-settled", 3, 1, "completed", budgetJobLife{
			startedAt: "2026-08-28T09:40:00Z", endedAt: "2026-08-28T09:41:00Z", pid: 4242,
		})
		verdict, err := bed.revisionAdmission("bounded", 3, 1, now)
		if err != nil || verdict.Refusal == nil || verdict.Extension != nil {
			t.Fatalf("an attempts breach without advancement carried an offer: %+v %v", verdict, err)
		}
	})
	t.Run("proposal beyond the raised box", func(t *testing.T) {
		bed := newBudgetReceiptBed(t, 10, 240, 10)
		root := bed.root
		bed.setReceipt(t, shippedReceiptLine(now.Add(-time.Hour), "bounded", "implement", "shipped")+"\n", 1)
		writeBudgetJob(t, root, "running", "reserve-running", 3, 240, "running", budgetJobLife{})
		verdict, err := bed.revisionAdmission("bounded", 3, 2000, now)
		if err != nil || verdict.Refusal == nil || verdict.Extension != nil {
			t.Fatalf("a proposal the raise cannot admit carried an offer: %+v %v", verdict, err)
		}
	})
}
