package delegation_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

const (
	budgetMachine = "budget-machine"
	budgetLineage = "budget-fixture"
)

// goalWorld publishes a migrated goal world in the bed's repository: the
// root record and the given goal files committed and accepted on the local
// ledger, with the bed's machine enrolled.
func (b *bed) goalWorld(files ...*goal.GoalFile) {
	b.t.Helper()
	b.writeFile("plans/goals/backlog.md", string(goal.RenderRoot(&goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1,
	})))
	for _, file := range files {
		b.writeFile(filepath.Join("plans", "goals", file.Id+".md"), string(goal.RenderFile(file)))
	}
	b.acceptGoals()
	b.git("config", "metasystem.goal.machine", budgetMachine)
	b.git("config", "goal.sync-remote", "local")
}

// acceptGoals commits the working goal files and moves both ledger refs to
// the commit.
func (b *bed) acceptGoals() {
	b.t.Helper()
	b.git("add", "-A", "plans")
	b.git("commit", "-qm", "goal ledger")
	tip := b.git("rev-parse", "HEAD")
	b.git("update-ref", goal.LocalLedgerBranch, tip)
	b.git("update-ref", goal.AcceptedRef, tip)
}

// alignClock puts the lifecycle clock at a fixed instant behind every wall
// time the real record owner stamps creation and termination with, so the
// lifecycle's ownership proof lands before them as it does in production
// however long the host takes, and re-arms supervision at that time.
func (b *bed) alignClock() time.Time {
	b.t.Helper()
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	b.doubles.Clock.Current = now
	b.armSupervision()
	return now
}

// claimedGoal is a claimed goal on the bed's machine and lineage, opened
// and claimed before now.
func claimedGoal(id string, now time.Time, budget *goal.Budget) *goal.GoalFile {
	opened := now.Add(-3 * time.Hour).Format(time.RFC3339)
	claimed := now.Add(-2 * time.Hour).Format(time.RFC3339)
	file := &goal.GoalFile{
		Id: id, State: goal.StateClaimed, Intent: "Exercise dispatch admission", Origin: goal.OriginMain,
		NextStep: "Dispatch.", OpenedAt: opened, Revision: 2, Tier: 3,
		Risk:    &goal.RiskRecord{Severity: 3, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "The isolated fixture holds severity at tier 3."},
		Claimed: &goal.ClaimRecord{Machine: budgetMachine, Lineage: budgetLineage, At: claimed},
		History: []goal.HistoryLine{
			{At: opened, Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAV-budget-machine-00000000", Verb: "open", Actor: budgetMachine + "+" + budgetLineage, Targets: []string{id}, Keep: -1},
			{At: claimed, Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAW-budget-machine-00000001", Verb: "claim", Actor: budgetMachine + "+" + budgetLineage, Targets: []string{id}, Keep: -1},
		},
	}
	if budget != nil {
		file.Budget = budget
		file.Claimed.Revision = 2
		file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: budgetMachine, ClaimEpoch: 4}
	}
	return file
}

// Fixture budgetless dispatch (lines 1358-1453): a claim without a
// structured budget (a pre-law survivor) closes the goal-free dispatch's
// admission. The refusal names the exact goal record and is typed, and no
// job record, reservation or launch exists afterwards.
func TestPortP2DispatchIntegrationBudgetlessClaimRefusesBeforeReservation(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.goalWorld(claimedGoal("budgetless-survivor", b.doubles.Clock.Now(), nil))
	brief := b.brief("budgetless.md", "design", "Review the design.")
	outputs := b.designPage()
	env := b.dispatchEnv("fresh")
	env.OwnerLineage = budgetLineage
	result := b.runEnv(env, "dispatch", "--role", "design-critic", "--outputs", outputs, "--design", fixtureDesign,
		"--brief", brief, "--job-id", "budgetless-refused")
	if result.ExitCode == 0 {
		t.Fatalf("a budgetless claim admitted a dispatch: %q", result.Stdout)
	}
	const named = "BUDGET_UNKNOWN record=plans/goals/budgetless-survivor.md goal=budgetless-survivor"
	if !strings.Contains(b.stderr.String(), named) {
		t.Fatalf("the refusal did not name the exact goal record: %q", b.stderr.String())
	}
	outcome := outcomeOf(t, result)
	if outcome["outcome"] != "REFUSED-BUDGET" || !strings.Contains(outcome["detail"].(string), named) {
		t.Fatalf("outcome %v", outcome)
	}
	if _, err := os.Stat(b.recordPath("budgetless-refused")); !os.IsNotExist(err) {
		t.Fatalf("the budgetless refusal created a job record: %v", err)
	}
	if len(b.launches) != 0 || len(b.calls("adapter.Launch")) != 0 {
		t.Fatal("the budgetless refusal launched")
	}
}

// useRealGoalOwner binds delegate operations through the real goal owner
// over the bed's ledger instead of the scripted binding.
func (b *bed) useRealGoalOwner() {
	b.t.Helper()
	real, err := delegation.NewOwnerPorts(delegation.OwnerConfig{Root: b.root, Host: b.doubles.Host})
	if err != nil {
		b.t.Fatal(err)
	}
	ports := b.doubles.Ports()
	ports.Git, ports.Goal = real.Git, real.Goal
	life, err := delegation.New(delegation.Config{Root: b.root, RepoScope: b.root, Engine: "/engine/metasystem"}, ports)
	if err != nil {
		b.t.Fatal(err)
	}
	b.life = life
}

func (b *bed) budgetEnv(mode dispatch.DispatchMode) delegation.Env {
	env := b.dispatchEnv(mode)
	env.OwnerLineage = budgetLineage
	return env
}

// Fixture structured budget (lines 1455-1575 and 1611-1644): a claimed
// goal with a complete structured tuple admits its first goal-bound
// dispatch, which binds the accepted goal revision; the completed attempt
// closes the attempt boundary, so a further dispatch and a follow-up both
// refuse as REFUSED-BUDGET naming the exact boundary and leave no job
// record or child reservation behind.
func TestPortP2DispatchIntegrationStructuredBudgetAdmitsThenClosesAtTheAttemptBoundary(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.useRealGoalOwner()
	b.completeOnWait()
	now := b.alignClock()
	b.goalWorld(claimedGoal("structured-budget", now, &goal.Budget{
		ElapsedLimit: "1d", AttemptLimit: 1, ReservedJobMinutesLimit: 1200, ActiveJobLimit: 1, ReviewRoundLimit: 3,
	}))
	brief := b.brief("structured-budget.md", "verify", "Verify the thing.")
	dispatchVerifier := func(job string, wait bool) delegation.Result {
		args := []string{"dispatch", "--role", "verifier", "--brief", brief, "--permissions", "none",
			"--goal", "structured-budget", "--destructive-reach", "MECHANICAL", "--job-id", job}
		if wait {
			args = append(args, "--wait")
		}
		return b.runEnv(b.budgetEnv("fresh"), args...)
	}

	requireExit(t, dispatchVerifier("structured-budget-within", true), 0, b.stderr.String())
	within := b.record("structured-budget-within")
	if within["status"] != "completed" || within["launchMode"] != "shared-checkout" ||
		within["goalId"] != "structured-budget" || fmt.Sprint(within["goalRevision"]) != "2" {
		t.Fatalf("the within-limits dispatch did not bind the accepted structured goal revision: %v", within)
	}

	result := dispatchVerifier("structured-budget-refused", false)
	requireBudgetRefusal(t, b, result, "attemptLimit used=1 limit=1")
	if _, err := os.Stat(b.recordPath("structured-budget-refused")); !os.IsNotExist(err) {
		t.Fatalf("the structured admission refusal created a job record: %v", err)
	}

	message := b.writeFile("follow.md", "Please continue.\n")
	result = b.runEnv(b.budgetEnv("follow-up"), "follow-up", "--job", "structured-budget-within", "--message", message)
	requireBudgetRefusal(t, b, result, "attemptLimit used=1 limit=1")
	if _, err := os.Stat(b.recordPath("structured-budget-within-r2")); !os.IsNotExist(err) {
		t.Fatalf("the structured follow-up refusal created a child reservation: %v", err)
	}
	if len(b.launches) != 1 {
		t.Fatalf("a refused budget launched: %d launches", len(b.launches))
	}
}

func requireBudgetRefusal(t *testing.T, b *bed, result delegation.Result, boundary string) {
	t.Helper()
	if result.ExitCode == 0 {
		t.Fatalf("an exhausted budget admitted a launch: %q", result.Stdout)
	}
	outcome := outcomeOf(t, result)
	detail, _ := outcome["detail"].(string)
	if outcome["outcome"] != "REFUSED-BUDGET" || !strings.Contains(detail, "BUDGET_REFUSED: goal structured-budget") ||
		!strings.Contains(detail, boundary) {
		t.Fatalf("outcome %v stderr %q", outcome, b.stderr.String())
	}
}

// Fixture earned extension (lines 1577-1644): after the attempt boundary
// closes, a shipped implementation receipt at the accepted tip makes the
// revision seam offer one tier-box extension. The dispatcher hands the exact
// seam to the extension owner, re-judges, and the same dispatch proceeds.
// Once the extended box is spent again, a further launch refuses as
// REFUSED-BUDGET naming the standing marker, never asks for a second
// extension, and leaves no job record.
func TestPortP2DispatchIntegrationEarnedExtensionProceedsOnceThenNamesItsMarker(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.useRealGoalOwner()
	b.completeOnWait()
	now := b.alignClock()
	conf := b.readFile("metasystem.conf")
	b.writeFile("metasystem.conf", conf+"metasystem.budget.tier-3=8h/1/1200m/1/3\n")
	file := claimedGoal("structured-budget", now, &goal.Budget{
		ElapsedLimit: "1d", AttemptLimit: 1, ReservedJobMinutesLimit: 1200, ActiveJobLimit: 1, ReviewRoundLimit: 3,
	})
	b.goalWorld(file)
	brief := b.brief("structured-budget.md", "verify", "Verify the thing.")
	dispatchVerifier := func(job string, wait bool) delegation.Result {
		args := []string{"dispatch", "--role", "verifier", "--brief", brief, "--permissions", "none",
			"--goal", "structured-budget", "--destructive-reach", "MECHANICAL", "--job-id", job}
		if wait {
			args = append(args, "--wait")
		}
		return b.runEnv(b.budgetEnv("fresh"), args...)
	}
	requireExit(t, dispatchVerifier("structured-budget-within", true), 0, b.stderr.String())

	// Advancement evidence lands on the accepted ledger.
	receiptAt := now.Add(-30 * time.Minute)
	b.writeFile("memory/receipts.log", fmt.Sprintf("%d|%s|RECEIPT|type=implement|outcome=shipped|goal=structured-budget|built_by=fixture|note=fixture advancement\n",
		receiptAt.Unix(), receiptAt.Format(time.RFC3339)))
	b.git("add", "memory/receipts.log")
	b.acceptGoals()

	// The extension owner double applies the offered raise exactly once, as
	// the real owner publishes it: consumption members raised, marker
	// written, claim binding untouched.
	extendedAt := now.Format(time.RFC3339)
	b.doubles.Host.ExtendFunc = func(request delegation.ExtendBudgetRequest) (string, int) {
		file.Revision++
		file.Budget.AttemptLimit, file.Budget.ReservedJobMinutesLimit = 2, 2400
		opid := "01ARZ3NDEKTSV4RRFFQ69G5FAZ-budget-machine-00000004"
		file.History = append(file.History, goal.HistoryLine{At: extendedAt, Opid: opid, Verb: "extend-budget",
			Actor: budgetMachine + "+" + budgetLineage, Targets: []string{"structured-budget"}, Keep: -1})
		file.BudgetExtension = &goal.BudgetExtensionRecord{At: extendedAt, Opid: opid,
			AttemptLimitFrom: 1, AttemptLimitTo: 2, ReservedJobMinutesFrom: 1200, ReservedJobMinutesTo: 2400,
			EvidenceKind: "landing", EvidenceID: "fixture-receipt", EvidenceAt: receiptAt.Format(time.RFC3339)}
		b.writeFile("plans/goals/structured-budget.md", string(goal.RenderFile(file)))
		b.acceptGoals()
		return `{"outcome":"confirmed"}`, 0
	}
	requireExit(t, dispatchVerifier("structured-budget-extended", true), 0, b.stderr.String())
	if len(b.doubles.Host.ExtendCalled) != 1 {
		t.Fatalf("extension requests %+v, want exactly one", b.doubles.Host.ExtendCalled)
	}
	request := b.doubles.Host.ExtendCalled[0]
	if request.Root != b.root || request.GoalID != "structured-budget" || request.Revision != 2 || request.ProposedCap < 1 ||
		request.Role != "verifier" || request.DispatchMode != "fresh" || request.DestructiveReach != "MECHANICAL" ||
		request.OwnerLineage != budgetLineage {
		t.Fatalf("the dispatcher did not hand the exact seam to the extension owner: %+v", request)
	}
	if extended := b.record("structured-budget-extended"); extended["status"] != "completed" || fmt.Sprint(extended["goalRevision"]) != "2" {
		t.Fatalf("the dispatcher did not proceed on the unchanged claim after the earned extension: %v", extended)
	}

	result := dispatchVerifier("structured-budget-refused", false)
	requireBudgetRefusal(t, b, result, "extended once at "+extendedAt)
	if len(b.doubles.Host.ExtendCalled) != 1 {
		t.Fatal("a standing extension marker asked the owner for a second extension")
	}
	if _, err := os.Stat(b.recordPath("structured-budget-refused")); !os.IsNotExist(err) {
		t.Fatalf("the second exhaustion created a job record: %v", err)
	}
}
