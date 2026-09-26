package branch_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// TestIntentReviewRetryAuthority: a retry of a failed examination is refused
// while the examination is live, admitted once the round is terminal
// without a return, starts exactly one round in the same critic chain, and
// is rejoined on every repeat, even after the retry itself failed; the
// failed retry offers a retry of its own round, and a completed round with
// a return is never retried.
func TestIntentReviewRetryAuthority(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, false)
	unit := r.unit
	r.expectStart()
	r.expectGateAndBrief()
	for range 7 {
		r.expectStart()
	}
	input := filepath.Join(t.TempDir(), "accepted-design.md")
	os.WriteFile(input, []byte("Accepted design.\n"), 0o644)
	followUps := []string{}
	request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: unit,
		GoalID: "goal-a", UnitCommit: unit, Repository: r, BriefPath: input,
		CheckClaim: claimAllowed, Gate: func(string) (string, error) { return "green", nil },
		NewID: func(string) (string, error) { return "retry-gate", nil },
		Delegate: func(brief, goalID, commit, runtime, model string) (string, error) {
			writeReadJobWithSubject(t, r.root, "critic", unit, "running", false, r.readSubject())
			return "critic", nil
		},
		FollowUp: func(rootJob, brief string) (string, error) {
			if rootJob != "critic" || brief == "" {
				t.Fatalf("follow-up of %q with brief %q", rootJob, brief)
			}
			round := len(followUps) + 2
			job := "critic-r" + string(rune('0'+round))
			followUps = append(followUps, job)
			writeJSONFixture(t, r.root, "artifacts/agents/jobs/"+job+".json", map[string]any{"jobId": job, "role": "code-critic",
				"round": round, "status": "running", "parentJob": "critic", "dispatchMode": "follow-up", "reviews": "commit:" + unit})
			return job, nil
		},
	}
	if _, err := branch.RunBranchRead(request); err != nil {
		t.Fatal(err)
	}
	retry := func(round int64) (branch.BranchReadResult, error) {
		retried := request
		retried.BriefPath, retried.Retry = "", round
		return branch.RunBranchRead(retried)
	}
	if _, err := retry(1); err == nil || !strings.Contains(err.Error(), "still running") || len(followUps) != 0 {
		t.Fatalf("a live examination was retried: %v %v", err, followUps)
	}
	writeJSONFixture(t, r.root, "artifacts/agents/jobs/critic.json", map[string]any{"jobId": "critic", "role": "code-critic", "round": 1,
		"status": "failed", "error": "process-lost", "reviews": "commit:" + unit, "goalId": "goal-a", "findingRegister": []any{}})
	if result, err := retry(1); err != nil || result.State != "dispatched" || result.Retry != "critic-r2" || len(followUps) != 1 {
		t.Fatalf("a proved-stopped failed examination: %+v %v %v", result, err, followUps)
	}
	if result, err := retry(1); err != nil || result.State != "retry-joined" || result.Retry != "critic-r2" || len(followUps) != 1 {
		t.Fatalf("a repeated retry rejoins: %+v %v", result, err)
	}
	// The retry itself fails: retry 1 still rejoins it, retry 2 is its own.
	writeJSONFixture(t, r.root, "artifacts/agents/jobs/critic-r2.json", map[string]any{"jobId": "critic-r2", "role": "code-critic",
		"round": 2, "status": "failed", "error": "process-lost", "parentJob": "critic", "reviews": "commit:" + unit})
	if result, err := retry(1); err != nil || result.Retry != "critic-r2" || len(followUps) != 1 {
		t.Fatalf("retry 1 after its retry failed: %+v %v", result, err)
	}
	if result, err := retry(2); err != nil || result.Retry != "critic-r3" || len(followUps) != 2 {
		t.Fatalf("retry of the failed retry: %+v %v", result, err)
	}
	// A round with a return is decided, never retried; an older round that
	// is not the newest is refused.
	writeJSONFixture(t, r.root, "artifacts/agents/jobs/critic-r3.json", map[string]any{"jobId": "critic-r3", "role": "code-critic",
		"round": 3, "status": "failed", "parentJob": "critic", "reviews": "commit:" + unit})
	writeJSONFixture(t, r.root, "artifacts/agents/critic/rounds/3/return.json", map[string]any{"jobId": "critic-r3", "round": 3, "findings": []any{}})
	if _, err := retry(3); err == nil || !strings.Contains(err.Error(), "wrote a return") || len(followUps) != 2 {
		t.Fatalf("a round with a return was retried: %v", err)
	}
	if _, err := retry(5); err == nil || !strings.Contains(err.Error(), "not the newest") {
		t.Fatalf("a retry of a round that does not exist: %v", err)
	}
}
