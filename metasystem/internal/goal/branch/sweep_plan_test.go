package branch

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// planRig answers the plan's reads and fails on any mutating git verb, fetch
// or push: an observation must change nothing (DL2-13).
type planRig struct {
	t          *testing.T
	remoteTips map[string]string
	localTip   string
	status     string
	present    map[string]bool
	unlanded   bool
	calls      []string
}

var planMutatingVerbs = map[string]bool{"update-ref": true, "worktree remove": true, "switch": true, "fetch": true, "push": true, "gc": true}

func (r *planRig) RemoteTip(_, remote, _ string) (string, bool, error) {
	tip, ok := r.remoteTips[remote]
	return tip, ok, nil
}
func (r *planRig) Fetch(string, string, string, string) error {
	r.t.Errorf("the plan fetched")
	return errors.New("fetch in a plan")
}
func (r *planRig) Push(string, string, string, string, string) (CASOutcome, error) {
	r.t.Errorf("the plan pushed")
	return CASUnknown, errors.New("push in a plan")
}

func (r *planRig) git(repo string, args ...string) ([]byte, error) {
	line := strings.Join(args, " ")
	r.calls = append(r.calls, line)
	for verb := range planMutatingVerbs {
		if strings.HasPrefix(line, verb) {
			r.t.Errorf("the plan ran a mutating git command: %s", line)
		}
	}
	switch {
	case strings.HasPrefix(line, "worktree list"):
		return []byte("worktree /repo\nbranch refs/heads/main\n\nworktree /repo-goal-a\nbranch refs/heads/goal/goal-a\n"), nil
	case strings.HasPrefix(line, "--no-optional-locks status"):
		return []byte(r.status), nil
	case strings.HasPrefix(line, "status"):
		r.t.Errorf("the plan ran a status that may refresh the index: %s", line)
		return nil, nil
	case strings.HasPrefix(line, "merge-base"):
		return []byte(sweepPolicyBase + "\n"), nil
	case strings.HasPrefix(line, "log"):
		return nil, nil
	}
	return nil, nil
}

func (r *planRig) dependencies() sweepDependencies {
	return sweepDependencies{
		localBranchTip: func(string, string) (string, bool, error) { return r.localTip, r.localTip != "", nil },
		gitOutput:      r.git,
		validateRange: func(_, _, tip, _ string) ([]Commit, error) {
			if r.unlanded {
				return []Commit{{ID: tip, Kind: "unit", Unit: "u1"}}, nil
			}
			return nil, nil
		},
		ancestor:      func(string, string, string) (bool, error) { return false, nil },
		clearRef:      func(string, string) error { r.t.Errorf("the plan cleared a ref"); return nil },
		objectPresent: func(_ string, sha string) bool { return r.present[sha] },
	}
}

func planRequest(transport PushTransport) SweepRequest {
	return SweepRequest{Repo: "/repo", Remote: "origin", EndpointTip: sweepPolicyEndpoint, GoalID: sweepPolicyGoal, PushTransport: transport}
}

func TestSweepPlanObservesWithoutMutating(t *testing.T) {
	t.Parallel()
	t.Run("clean and landed", func(t *testing.T) {
		rig := &planRig{t: t, remoteTips: map[string]string{"origin": sweepPolicyOrigin}, localTip: sweepPolicyLocal,
			present: map[string]bool{sweepPolicyOrigin: true}}
		plan, err := sweepPlanWithDependencies(planRequest(rig), rig.dependencies())
		if err != nil || plan.Refusal != nil {
			t.Fatalf("plan = %+v, %v", plan, err)
		}
		if len(plan.Worktrees) != 1 || plan.Worktrees[0] != "/repo-goal-a" || plan.LocalTip != sweepPolicyLocal || plan.RemoteTips["origin"] != sweepPolicyOrigin {
			t.Fatalf("plan = %+v", plan)
		}
		if len(plan.Unverified) != 0 || plan.Nothing() {
			t.Fatalf("plan = %+v", plan)
		}
	})
	t.Run("dirty worktree", func(t *testing.T) {
		rig := &planRig{t: t, remoteTips: map[string]string{}, localTip: sweepPolicyLocal, status: " M file.go\n"}
		plan, err := sweepPlanWithDependencies(planRequest(rig), rig.dependencies())
		if err != nil || !IsOperationRefusal(plan.Refusal, StaleCode) {
			t.Fatalf("a dirty worktree plan = %+v, %v; want the StaleCode refusal", plan, err)
		}
	})
	t.Run("unlanded commits", func(t *testing.T) {
		rig := &planRig{t: t, remoteTips: map[string]string{}, localTip: sweepPolicyLocal, unlanded: true}
		plan, err := sweepPlanWithDependencies(planRequest(rig), rig.dependencies())
		if err != nil || !IsOperationRefusal(plan.Refusal, SweepUnlandedCode) {
			t.Fatalf("an unlanded plan = %+v, %v; want the unlanded refusal", plan, err)
		}
	})
	t.Run("remote tip not fetched", func(t *testing.T) {
		rig := &planRig{t: t, remoteTips: map[string]string{"origin": sweepPolicyUnknown}, localTip: ""}
		plan, err := sweepPlanWithDependencies(planRequest(rig), rig.dependencies())
		if err != nil || plan.Refusal != nil || len(plan.Unverified) != 1 || !strings.Contains(plan.Unverified[0], sweepPolicyUnknown) {
			t.Fatalf("a remote tip absent locally = %+v, %v; want it unverified, never fetched", plan, err)
		}
	})
	t.Run("nothing", func(t *testing.T) {
		rig := &planRig{t: t, remoteTips: map[string]string{}}
		deps := rig.dependencies()
		deps.gitOutput = func(repo string, args ...string) ([]byte, error) {
			if strings.HasPrefix(strings.Join(args, " "), "worktree list") {
				return []byte("worktree /repo\nbranch refs/heads/main\n"), nil
			}
			return rig.git(repo, args...)
		}
		plan, err := sweepPlanWithDependencies(planRequest(rig), deps)
		if err != nil || !plan.Nothing() {
			t.Fatalf("an absent goal branch = %+v, %v; want nothing to sweep", plan, err)
		}
	})
}

// Sweep under a context runs its git through it: a stalled git call ends at
// the deadline and the sweep returns, never waiting on the stub (DL2-17).
func TestSweepReturnsAtTheContextWithAStalledGit(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rig := &planRig{t: t, remoteTips: map[string]string{}, localTip: sweepPolicyLocal}
	deps := rig.dependencies()
	stalled := 0
	deps.gitOutput = func(repo string, args ...string) ([]byte, error) {
		stalled++
		cancel() // the budget ends while git is stalled
		<-ctx.Done()
		return nil, ctx.Err()
	}
	request := planRequest(rig)
	request.Context = ctx
	request.CheckClaim = func() error { return nil }
	if _, err := sweepWithDependencies(request, deps); !errors.Is(err, context.Canceled) {
		t.Fatalf("a stalled sweep returned %v, want the context's end", err)
	}
	if stalled != 1 {
		t.Fatalf("the sweep went on to %d git calls after the deadline", stalled)
	}
	// A cancelled context stops the plan before any read.
	plan, err := sweepPlanWithDependencies(request, rig.dependencies())
	if !errors.Is(err, context.Canceled) || plan.GoalID != "" {
		t.Fatalf("a plan under an expired context = %+v, %v", plan, err)
	}
}
