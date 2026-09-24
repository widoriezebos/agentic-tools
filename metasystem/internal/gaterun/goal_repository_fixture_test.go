package gaterun

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
)

type goalRepositoryFixture struct {
	root       string
	endpoint   goal.Endpoint
	repository *testgoal.Repository
	proof      *humanauthority.Proof
}

func newGoalRepositoryFixture(t *testing.T, now time.Time) *goalRepositoryFixture {
	t.Helper()
	bed := &goalRepositoryFixture{root: t.TempDir()}
	if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte("metasystem.runtimes=fake\nmetasystem.budget.elapsed-grace-percent=25\nmetasystem.governance.correlation-policy=C\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(bed.root, "plans", "goals", "backlog.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, goal.RenderRoot(&goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1}), 0o644); err != nil {
		t.Fatal(err)
	}
	authorization, err := fixtureauth.New(bed.root)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := humanauthority.FixtureGoalProof(bed.root, authorization.GoalHumanAuthority(), now)
	if err != nil {
		t.Fatal(err)
	}
	bed.proof = &proof
	return bed
}

func (bed *goalRepositoryFixture) seed(t *testing.T, file *goal.GoalFile, now time.Time) {
	t.Helper()
	rootFile, err := os.ReadFile(filepath.Join(bed.root, "plans", "goals", "backlog.md"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"plans/goals/backlog.md": rootFile}
	if file != nil {
		path := "plans/goals/" + file.Id + ".md"
		data := goal.RenderFile(file)
		if err := os.WriteFile(filepath.Join(bed.root, filepath.FromSlash(path)), data, 0o644); err != nil {
			t.Fatal(err)
		}
		files[path] = data
	}
	bed.repository = testgoal.New(files, now, "0000000000000000000000000000000000000001")
	bed.endpoint = goal.Endpoint{Root: bed.root, Remote: goal.SyncLocal, Branch: goal.LocalLedgerBranch, Repository: bed.repository}
}

func (bed *goalRepositoryFixture) acceptedFile(t *testing.T, id string) *goal.GoalFile {
	t.Helper()
	tip, present, err := bed.repository.Accepted()
	if err != nil || !present {
		t.Fatalf("accepted tip: %q present=%t err=%v", tip, present, err)
	}
	path := "plans/goals/" + id + ".md"
	files, err := bed.repository.Files(tip, path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(files[path])
	if file == nil || len(problems) != 0 {
		t.Fatalf("accepted goal parse: %+v %v", file, problems)
	}
	return file
}

func (bed *goalRepositoryFixture) binding(root, id string, now time.Time) (dispatch.GoalBinding, error) {
	if root != bed.root {
		return dispatch.GoalBinding{}, fmt.Errorf("undeclared goal root %q", root)
	}
	projection, err := goal.Project(bed.endpoint, false, now)
	if err != nil {
		return dispatch.GoalBinding{}, err
	}
	file := projection.Tree.Live[id]
	if file == nil || file.State != goal.StateClaimed || file.Claimed == nil {
		return dispatch.GoalBinding{}, fmt.Errorf("goal %s is not a claimed accepted goal", id)
	}
	if file.StopCapability == nil {
		return dispatch.GoalBinding{}, fmt.Errorf("goal %s has no breach-stop authority", id)
	}
	tier := file.Tier
	if tier < 1 || tier > 3 {
		if projection.Tree.Root == nil || projection.Tree.Root.TierLaw != "" {
			return dispatch.GoalBinding{}, fmt.Errorf("goal %s has no tier", id)
		}
		tier = 3
	}
	gateWidth := "area"
	if file.Risk != nil {
		gateWidth = file.Risk.GateWidth()
	}
	return dispatch.GoalBinding{GoalID: id, Revision: file.Claimed.Revision, Tier: tier, GateWidth: gateWidth, Machine: file.Claimed.Machine, Lineage: file.Claimed.Lineage, Capability: *file.StopCapability, Fence: file.StopFence, File: file}, nil
}

func (bed *goalRepositoryFixture) discharge(id string, revision uint64, runID string, now time.Time) (WeightDischargeResult, error) {
	return weightDischargeAtWith(bed.root, id, revision, runID, now, weightDischargeReads{ResolveGoalBinding: bed.binding})
}

func (bed *goalRepositoryFixture) admittedAttempt(request run.GovernedAdmissionRequest, now time.Time) (run.GovernedAdmissionResult, error) {
	binding, err := bed.binding(bed.root, request.GoalID, now)
	if err != nil {
		return run.GovernedAdmissionResult{}, err
	}
	if binding.File.Obligation == nil || binding.File.Obligation.Revision != request.ObligationRevision {
		return run.GovernedAdmissionResult{}, fmt.Errorf("fixture has no accepted obligation revision %d", request.ObligationRevision)
	}
	projection := dispatch.ProjectBudget(bed.root, binding.File, now)
	if projection.Status != dispatch.BudgetKnown {
		return run.GovernedAdmissionResult{}, fmt.Errorf("fixture budget projection: %+v", projection.Unknown)
	}
	state, err := loadWeight(bed.root, now)
	if err != nil {
		return run.GovernedAdmissionResult{}, err
	}
	obligation := binding.File.Obligation
	generation := state.Generation
	observation := dispatch.ObserveGovernedAssumptions(bed.root, obligation.Assumptions, projection.ActiveJobs+1, 0, now)
	return run.GovernedAdmissionResult{Attempt: run.GovernedAttempt{
		GoalRevision: binding.Revision, ObligationRevision: obligation.Revision,
		WeightGeneration: &generation, BudgetEpoch: projection.WeightEpoch,
		Recurrence: obligation.Assumptions.Recurrence, ExecutionCostMinutes: (obligation.Assumptions.TimingEnvelopeSeconds + 59) / 60,
		AttemptOrdinal: projection.Attempts + 1, ReservedBefore: projection.ReservedJobMinutes,
		Budget: projection.Limits, BudgetStartedAt: projection.StartedAt.UTC().Format(time.RFC3339),
		CorrelationPolicy: obligation.ReviewPolicy, ExpectedAssumptions: obligation.Assumptions,
		AdmissionDecision: obligation.Decide(goal.EffectAuthorizeSpend), Observation: &observation, Breaker: run.BreakerClosed,
	}}, nil
}

func (bed *goalRepositoryFixture) terminalObservation(record *run.Record, ended time.Time) run.AssumptionObservation {
	started, err := time.Parse(time.RFC3339, record.StartedAt)
	if err != nil || record.Governed == nil || ended.Before(started) {
		return run.AssumptionObservation{ObservedAt: ended.UTC().Format(time.RFC3339), AssumptionState: run.AssumptionUnavailable}
	}
	return dispatch.ObserveGovernedAssumptions(bed.root, record.Governed.ExpectedAssumptions, 1,
		uint64(ended.Sub(started)/time.Second), ended)
}

func (bed *goalRepositoryFixture) publishFile(t *testing.T, file *goal.GoalFile, opid string) {
	t.Helper()
	parent, err := bed.repository.Capture(opid)
	if err != nil {
		t.Fatal(err)
	}
	next, err := bed.repository.Build(opid, parent, []goal.Change{{Path: "plans/goals/" + file.Id + ".md", Content: goal.RenderFile(file)}}, "fixture accepted goal edit")
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := bed.repository.Publish(parent, next)
	if err != nil || outcome != goal.CASLanded {
		t.Fatalf("fixture publish: %v %v", outcome, err)
	}
	if err := bed.repository.AcceptedCAS(parent, next); err != nil {
		t.Fatal(err)
	}
	if err := bed.repository.Release(opid); err != nil {
		t.Fatal(err)
	}
}
