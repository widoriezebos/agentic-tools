package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

func neverProjectionDeadline(time.Duration) <-chan time.Time { return make(chan time.Time) }

func checkedProjectionDeadline(t *testing.T) func(time.Duration) <-chan time.Time {
	t.Helper()
	var calls atomic.Int32
	t.Cleanup(func() {
		if calls.Load() == 0 {
			t.Error("fresh ledger read bypassed its fixture deadline")
		}
	})
	return func(wait time.Duration) <-chan time.Time {
		calls.Add(1)
		if wait != 4*time.Second && wait != 3*time.Second {
			t.Errorf("fresh ledger deadline=%s", wait)
		}
		return make(chan time.Time)
	}
}

func TestFrozenCorpusPhaseOwnsItsInjectedDeadline(t *testing.T) {
	t.Parallel()
	event := make(chan time.Time)
	calls := 0
	phases := newFrozenCorpusPhases(t, func(wait time.Duration) <-chan time.Time {
		calls++
		if wait != 5*time.Minute {
			t.Errorf("phase budget=%s; want 5m", wait)
		}
		return event
	})
	ctx, cancel := phases.begin("fixture").context()
	defer cancel()
	if calls != 1 || ctx.Err() != nil {
		t.Fatalf("injected deadline calls=%d context=%v", calls, ctx.Err())
	}
	if _, present := ctx.Deadline(); present {
		t.Fatal("phase inherited a wall-clock deadline")
	}
	close(event)
	<-ctx.Done()
	if context.Cause(ctx) != context.DeadlineExceeded {
		t.Fatalf("phase expiry cause=%v", context.Cause(ctx))
	}
	event = make(chan time.Time)
	next, stop := phases.begin("next fixture").context()
	stop()
	if context.Cause(next) != context.Canceled || calls != 2 {
		t.Fatalf("phase cancellation cause=%v calls=%d", context.Cause(next), calls)
	}
}

func TestPublicLandingGateUsesInjectedDeadline(t *testing.T) {
	t.Parallel()
	b, owners, _ := gatedDeliveryBed(t, waitingToLandBed)
	deadlines := 0
	b.laneInputs = func(inputs *intentOwners) {
		inputs.dependencies.projectionDeadline = func(wait time.Duration) <-chan time.Time {
			deadlines++
			if wait != 4*time.Second {
				t.Errorf("landing gate deadline=%s; want 4s", wait)
			}
			return make(chan time.Time)
		}
	}
	code, result := b.do("work", "land", bedGoal)
	expectGateRefusal(t, "injected landing gate", code, result, "waits for a person")
	if deadlines != 1 || len(owners.pushes) != 0 {
		t.Fatalf("landing gate deadlines=%d pushes=%d; want one deadline and no push", deadlines, len(owners.pushes))
	}
}

// Real Git must receive both the projection deadline and the subprocess fetch deadline.
func TestPublicLandCandidateGitAdapterUsesInjectedDeadline(t *testing.T) {
	t.Parallel()
	f := newWholeOwnerLanding(t)
	var outer, process atomic.Int32
	deps := goalBranchLandPrepDependencies{CandidateOnly: true, ProjectionDeadline: func(wait time.Duration) <-chan time.Time {
		switch wait {
		case 4 * time.Second:
			outer.Add(1)
		case 3 * time.Second:
			process.Add(1)
		default:
			t.Errorf("land candidate deadline=%s", wait)
		}
		return make(chan time.Time)
	}}
	deps.Prepare = func(request branch.LandRequest) (branch.LandResult, error) {
		if outer.Load() != 2 || process.Load() != 2 {
			t.Errorf("land preparation admitted with deadlines=%d/%d; want the holder read and fresh projection", outer.Load(), process.Load())
		}
		return branch.PrepareLanding(request)
	}
	result, code, err := goalBranchLandPrepRun([]string{"--root", f.goalRoot, "--goal", "standing-validation", "--last"}, deps)
	if err != nil || code != 0 || result.Result.Candidate == "" || outer.Load() < 2 || process.Load() != outer.Load() {
		t.Fatalf("public land candidate code=%d result=%+v err=%v deadlines=%d/%d; want candidate with all claim reads on injected deadlines", code, result, err, outer.Load(), process.Load())
	}
}
