package dispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
)

// These tests are the Go port of the dispatch-owned halves of the goal CLI
// shell bed's structured-budget and fenced-set-budget scenarios: job
// goal-admission, job breach-stop and job stop-batch-reconcile against a
// ledger the real goal owners wrote (open, approve, claim, set-budget), with
// the accepted ledger in memory and the clock in the test's hand.

type gcliBudgetBed struct {
	t          *testing.T
	root       string
	repository *testgoal.Repository
	reads      goalAdmissionReads
	proof      *humanauthority.Proof
	sequence   int
}

func newGCLIBudgetBed(t *testing.T, at time.Time) *gcliBudgetBed {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"plans/goals/backlog.md": goal.RenderRoot(&goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1,
	})}
	bed := &gcliBudgetBed{t: t, root: root, repository: testgoal.New(files, at, "0000000000000000000000000000000000000001")}
	bed.reads = goalAdmissionReads{
		NewWorld: func(got string) bool { return got == root },
		ResolveEndpoint: func(got string) (goal.Endpoint, error) {
			if got != root {
				return goal.Endpoint{}, fmt.Errorf("undeclared goal endpoint root %q", got)
			}
			return bed.endpoint(), nil
		},
		ResolveMachine: func(got string) (string, error) {
			if got != root {
				return "", fmt.Errorf("undeclared goal machine root %q", got)
			}
			return "fixture-machine", nil
		},
	}
	authorization, err := fixtureauth.New(root)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := humanauthority.FixtureGoalProof(root, authorization.GoalHumanAuthority(), at)
	if err != nil {
		t.Fatal(err)
	}
	bed.proof = &proof
	return bed
}

func (bed *gcliBudgetBed) endpoint() goal.Endpoint {
	return goal.Endpoint{Root: bed.root, Remote: "local", Branch: goal.LocalLedgerBranch, Repository: bed.repository}
}

// request is the seat's (fixture-machine+fixture-lineage) act at, or the
// person's when human is set.
func (bed *gcliBudgetBed) request(at time.Time, human bool) goal.VerbRequest {
	bed.sequence++
	request := goal.VerbRequest{Endpoint: bed.endpoint(), Actor: goal.Actor{Machine: "fixture-machine", Lineage: "fixture-lineage"},
		Ulid: fmt.Sprintf("01J5X000000000000000GC%04d", bed.sequence), Now: at, ClaimEpoch: 1}
	if human {
		request.Actor.Human = "Wido"
	}
	return request
}

func (bed *gcliBudgetBed) confirm(what string, result goal.PublishResult, err error) {
	bed.t.Helper()
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		bed.t.Fatalf("%s: %+v %v", what, result, err)
	}
}

// budget is the command surface's canonical tuple (goal.NewBudget), as the
// long-form limit flags produce it.
func (bed *gcliBudgetBed) budget(elapsed string, attempts, minutes, active, rounds int64) goal.Budget {
	bed.t.Helper()
	budget, err := goal.NewBudget(elapsed, attempts, minutes, active, rounds)
	if err != nil {
		bed.t.Fatal(err)
	}
	return budget
}

// claimApproved opens a person's tier-3 goal, approves it under budget and
// claims it, all at at: the shell bed's open, approve_fixture_goal and claim.
func (bed *gcliBudgetBed) claimApproved(id, next string, budget goal.Budget, at time.Time) {
	bed.t.Helper()
	risk := goal.RiskRecord{Severity: 3, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "fixture risk"}
	result, err := goal.OpenRisked(bed.request(at, true), id, "Fixture goal "+id+".", goal.OriginHuman, next, nil, nil, risk, 3, "", nil, bed.proof)
	bed.confirm("open "+id, result, err)
	result, err = goal.Approve(bed.request(at, true), []string{id}, &budget, bed.proof)
	bed.confirm("approve "+id, result, err)
	result, err = goal.Claim(bed.request(at, false), id)
	bed.confirm("claim "+id, result, err)
}

func (bed *gcliBudgetBed) accepted(id string) string {
	bed.t.Helper()
	tip, _, err := bed.repository.Accepted()
	if err != nil {
		bed.t.Fatal(err)
	}
	files, err := bed.repository.Files(tip, "plans/goals/"+id+".md")
	if err != nil {
		bed.t.Fatal(err)
	}
	return string(files["plans/goals/"+id+".md"])
}

// admission is job goal-admission --stop-lineage fixture-lineage: its lines
// and its exit code (0 admitted, 9 refused, 10 refused with a live stop).
func (bed *gcliBudgetBed) admission(at time.Time) (string, int) {
	bed.t.Helper()
	verdict, err := evaluateGoalAdmissionWithReads(bed.root, "fixture-lineage", at, bed.reads)
	if err != nil {
		bed.t.Fatal(err)
	}
	lines := strings.Join(FormatGoalAdmission(verdict), "\n")
	if !verdict.Refused() {
		return lines, 0
	}
	for _, refusal := range verdict.Refusals {
		if refusal.LiveStopReason != "" {
			return lines, 10
		}
	}
	return lines, 9
}

func gcliBudgetLine(record, prefix string) string {
	for _, line := range strings.Split(record, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}

// structured-budget, dispatch half: a claim under the complete tuple is
// admitted while within all four limits and refused at its elapsed breach
// boundary naming the exact limit, with a breach-stop request (exit 10).
func TestGoalCLIBudgetStructuredAdmission(t *testing.T) {
	t.Parallel()
	claimAt := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	bed := newGCLIBudgetBed(t, claimAt)
	bed.claimApproved("budget-check", "Appetite: 4h is inert human prose, not a budget.",
		bed.budget("8h", 2, 120, 1, 3), claimAt)
	if claim := gcliBudgetLine(bed.accepted("budget-check"), "- Claimed: "); !strings.Contains(claim, " revision=3") {
		t.Fatalf("the claim did not bind revision 3: %q", claim)
	}

	if lines, code := bed.admission(time.Date(2026, 8, 20, 5, 1, 0, 0, time.UTC)); code != 0 {
		t.Fatalf("the structured claim was refused while within all four limits: code=%d %s", code, lines)
	}
	lines, code := bed.admission(time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC))
	if code != 10 {
		t.Fatalf("the claim at its structured breach boundary did not request breach-stop (code=%d): %s", code, lines)
	}
	if !strings.Contains(lines, "BUDGET_REFUSED: goal budget-check revision=3 admission closed: elapsedLimit") {
		t.Fatalf("the structured refusal did not name its exact limit: %s", lines)
	}
}

// fenced-set-budget: an elapsed breach closes the launch fence (job
// breach-stop), the empty stop batch reconciles COMPLETE, and one person's
// set-budget on the stopped goal installs the replacement tuple, lifts the
// completed fence naming the stop on its history line, and reopens admission.
func TestGoalCLIBudgetFencedSetBudget(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
	bed := newGCLIBudgetBed(t, start)
	bed.claimApproved("fenced-one-step", "Run the larger budget.",
		bed.budget("1m", 2, 120, 1, 3), start)
	revision, _, err := resolveGoalRevisionWithReads(bed.root, "fenced-one-step", bed.reads)
	if err != nil {
		t.Fatal(err)
	}

	stopAt := start.Add(2 * time.Minute)
	batch, err := ensureBreachStopWithReads(bed.root, "fenced-one-step", revision, stopAt, bed.reads, "")
	if err != nil {
		t.Fatal(err)
	}
	reconciled, err := ReconcileStopBatch(bed.root, batch.StopID, stopAt)
	if err != nil || reconciled.State != goal.StopBatchComplete {
		t.Fatalf("fenced set-budget fixture did not complete stop batch %s: %+v %v", batch.StopID, reconciled, err)
	}
	if gcliBudgetLine(bed.accepted("fenced-one-step"), "- StopFence:") == "" {
		t.Fatalf("breach-stop did not close the launch fence:\n%s", bed.accepted("fenced-one-step"))
	}

	raised := bed.budget("8h", 2, 120, 1, 3)
	result, err := goal.SetBudgetApproved(bed.request(start.Add(3*time.Minute), true), "fenced-one-step", raised, bed.proof)
	bed.confirm("set-budget fenced-one-step", result, err)
	record := bed.accepted("fenced-one-step")
	if line := gcliBudgetLine(record, "- StopFence:"); line != "" {
		t.Fatalf("one-step set-budget left the completed launch fence in place: %q", line)
	}
	if !regexp.MustCompile(` set-budget .* resumed=` + regexp.QuoteMeta(batch.StopID)).MatchString(record) {
		t.Fatalf("one-step set-budget history did not name the lifted stop %s:\n%s", batch.StopID, record)
	}
	if line := gcliBudgetLine(record, "- Budget: "); line != "- Budget: elapsedLimit=1d attemptLimit=2 reservedJobMinutesLimit=120 activeJobLimit=1 reviewRoundLimit=3" {
		t.Fatalf("one-step set-budget did not install the replacement tuple: %q", line)
	}
	if lines, code := bed.admission(start.Add(3 * time.Minute)); code != 0 {
		t.Fatalf("one-step set-budget did not reopen admission: code=%d %s", code, lines)
	}
}
