package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// goalBudgetResumeFixture gives command tests a physical checkout and an
// immutable accepted tree without requiring a version-control process.
type goalBudgetResumeFixture struct {
	*obligationCommandFixture
	initialBinding *dispatchcore.GoalBinding
	bindingCalls   int
}

func newGoalBudgetResumeFixture(t *testing.T, stopped bool, amend func(*goal.GoalFile)) *goalBudgetResumeFixture {
	t.Helper()
	base := newObligationCommandFixture(t)
	file, _ := base.acceptedGoal()
	if stopped {
		closedAt := "2026-09-01T09:00:00Z"
		stopID := "stop-standing-validation-r2-f1"
		file.Revision++
		file.StopCapability = &goal.StopCapability{
			Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1, FenceEpoch: 1,
		}
		file.StopFence = &goal.StopFence{
			StopID: stopID, Revision: 2, Epoch: 1, CapabilityGeneration: 2,
			ClosedAt: closedAt, Reason: goal.StopReasonElapsedLimit,
		}
		file.History = append(file.History, goal.HistoryLine{
			At: closedAt, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAC", "mac-cli", "m1"),
			Verb: "breach-stop", Actor: "mac-cli+m1", Targets: []string{"standing-validation"}, Keep: -1,
		})
	}
	if amend != nil {
		amend(file)
	}
	path := "plans/goals/standing-validation.md"
	rendered := goal.RenderFile(file)
	if err := os.WriteFile(filepath.Join(base.root(), filepath.FromSlash(path)), rendered, 0o644); err != nil {
		t.Fatal(err)
	}
	initial := obligationFilesCopy(base.repo.commit(base.repo.accepted).files)
	initial[path] = append([]byte(nil), rendered...)
	id := fmt.Sprintf("%040x", 1)
	base.repo = &obligationRepository{
		t: t, commits: map[string]obligationCommit{id: {files: initial, at: time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)}},
		canonical: id, accepted: id, serial: 1,
	}
	fixture := &goalBudgetResumeFixture{obligationCommandFixture: base}
	if file.State == goal.StateClaimed && file.Claimed != nil && file.StopCapability != nil {
		fixture.initialBinding = &dispatchcore.GoalBinding{
			GoalID: file.Id, Revision: file.Claimed.Revision, Tier: file.Tier,
			Machine: file.Claimed.Machine, Lineage: file.Claimed.Lineage,
			Capability: *file.StopCapability, Fence: file.StopFence, File: file,
		}
	}
	if file.StopFence != nil {
		fence := file.StopFence
		capability := file.StopCapability
		batch := goal.StopBatch{
			StopID: fence.StopID, GoalID: file.Id, GoalRevision: fence.Revision,
			FenceEpoch: fence.Epoch, CapabilityGeneration: fence.CapabilityGeneration,
			Machine: capability.Machine, ClaimEpoch: capability.ClaimEpoch,
			Reason: fence.Reason, State: goal.StopBatchComplete,
			OpenedAt: fence.ClosedAt, UpdatedAt: fence.ClosedAt, CompletedAt: fence.ClosedAt, Pass: 1,
		}
		if err := goal.WriteStopBatch(base.root(), batch); err != nil {
			t.Fatal(err)
		}
	}
	return fixture
}

func (f *goalBudgetResumeFixture) binding(root, id string, now time.Time) (dispatchcore.GoalBinding, error) {
	f.t.Helper()
	f.facts.checkRoot(root)
	if id != "standing-validation" || !now.Equal(time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)) {
		f.t.Fatalf("binding input id=%q now=%s", id, now)
	}
	f.bindingCalls++
	if f.initialBinding == nil {
		f.t.Fatal("command requested a binding for an unclaimed goal")
	}
	return *f.initialBinding, nil
}

func (f *goalBudgetResumeFixture) expectBindings(count int) {
	f.t.Helper()
	if f.bindingCalls != count {
		f.t.Fatalf("binding calls = %d, want %d", f.bindingCalls, count)
	}
}

func (f *goalBudgetResumeFixture) runBudget(args []string, prove goalAuthorityProver) int {
	return runGoalBudgetWithInputs(args, prove, f.commandNow, f.dependencies(), f.binding)
}

func (f *goalBudgetResumeFixture) runResume(args []string, prove goalAuthorityProver) int {
	return runGoalResumeWithInputs(args, prove, f.commandNow, f.dependencies(), f.binding)
}

func (f *goalBudgetResumeFixture) project() *goal.GoalFile {
	f.t.Helper()
	endpoint, err := f.dependencies().endpoint(f.root())
	if err != nil {
		f.t.Fatal(err)
	}
	projection, err := goal.Project(endpoint, false, time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC))
	if err != nil {
		f.t.Fatal(err)
	}
	file := projection.Tree.Live["standing-validation"]
	if file == nil {
		f.t.Fatal("accepted goal is missing")
	}
	return file
}

func TestGoalResumeMissingInjectedEndpointRefusesBeforeBinding(t *testing.T) {
	fixture := newGoalBudgetResumeFixture(t, true, nil)
	root := fixture.root()
	writeFixtureEnrollment(t, root, "Wido")
	dependencies := fixture.dependencies()
	dependencies.endpoint = nil
	args := append(completeResumeArgs(root), "--fixture-human-authority")
	code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
		return runGoalResumeWithInputs(args, fixedFixtureGoalAuthority, fixture.commandNow, dependencies, fixture.binding)
	})
	if code != 1 || stdout != "" || !strings.Contains(stderr, "goal resume: goal endpoint reader is missing.") {
		t.Fatalf("missing endpoint did not refuse cleanly: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	fixture.expectBindings(0)
	if fixture.repo.publications != 0 {
		t.Fatalf("missing endpoint published %d times", fixture.repo.publications)
	}
}
