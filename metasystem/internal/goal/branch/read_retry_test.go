package branch_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
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

// retryProcessTable is the whole process table a retry's death check reads:
// the round's recorded process, in its group while live, and nothing else.
type retryProcessTable struct {
	process identity.Exact
	pgid    int64
	live    bool
}

func (f *retryProcessTable) ReadStart(pid int64) (identity.Exact, identity.Liveness, error) {
	if !f.live || pid != f.process.Pid {
		return identity.Exact{}, identity.Dead, nil
	}
	return f.process, identity.Alive, nil
}

func (f *retryProcessTable) ReadArgv(pid int64) ([]string, bool) {
	if !f.live || pid != f.process.Pid {
		return nil, false
	}
	return []string{"owned", "critic-tag"}, true
}

func (f *retryProcessTable) Pids() ([]int64, error) {
	if !f.live {
		return nil, nil
	}
	return []int64{f.process.Pid}, nil
}

func (f *retryProcessTable) Group(pid int64) (int64, error) {
	if !f.live || pid != f.process.Pid {
		return 0, unix.ESRCH
	}
	return f.pgid, nil
}

func (f *retryProcessTable) Session(pid int64) (int64, error) { return f.Group(pid) }

func (f *retryProcessTable) Parent(int64) (int64, bool) { return 0, false }

// TestIntentReviewRetryProvesRecordedProcessDead: a failed round that
// recorded its process, and no group-death time, is retried once the custody
// owner's dependencies show that process and its group dead, and is refused
// while they show it alive.
func TestIntentReviewRetryProvesRecordedProcessDead(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, false)
	unit := r.unit
	r.expectStart()
	r.expectGateAndBrief()
	r.expectStart()
	r.expectStart()
	input := filepath.Join(t.TempDir(), "accepted-design.md")
	os.WriteFile(input, []byte("Accepted design.\n"), 0o644)
	primary := identity.Exact{Pid: 4242, StartedAt: time.UnixMicro(100_000_001)}
	if runtime.GOOS == "linux" {
		primary = identity.Exact{Pid: 4242, StartedAt: time.Unix(100, 0), StartTicks: 7001, BootID: "boot-a"}
	}
	table := &retryProcessTable{process: primary, pgid: 4242, live: true}
	matches := func(argv []string, tag string) bool { return len(argv) == 2 && argv[0] == "owned" && argv[1] == tag }
	followUps := 0
	request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: unit,
		GoalID: "goal-a", UnitCommit: unit, Repository: r, BriefPath: input,
		CheckClaim: claimAllowed, Gate: func(string) (string, error) { return "green", nil },
		NewID: func(string) (string, error) { return "retry-gate", nil },
		Delegate: func(brief, goalID, commit, runtime, model string) (string, error) {
			writeReadJobWithSubject(t, r.root, "critic", unit, "running", false, r.readSubject())
			return "critic", nil
		},
		FollowUp: func(rootJob, brief string) (string, error) {
			followUps++
			return "critic-r2", nil
		},
		CustodyDeath: dispatch.CustodyDeathDependencies{Reader: table, Processes: table, MatchesTag: matches,
			TaggedScan: func(tag string) census.TaggedProcessCensus {
				return census.ScanTaggedProcesses(tag, census.TaggedScanDependencies{Processes: table, Reader: table,
					Signal: func(int64) error { return nil }, MatchesTag: matches})
			}},
	}
	if _, err := branch.RunBranchRead(request); err != nil {
		t.Fatal(err)
	}
	record := map[string]any{"jobId": "critic", "role": "code-critic", "round": 1, "status": "failed",
		"error": "protocol_error", "runnerClosed": false, "reviews": "commit:" + unit, "goalId": "goal-a",
		"findingRegister": []any{}, "instanceTag": "critic-tag", "pgid": 4242}
	ref := primary.Ref()
	record["pid"], record["pidStartedAt"] = ref.Pid, ref.StartedAtSec
	if ref.StartTicks > 0 {
		record["pidStartTicks"], record["bootId"] = ref.StartTicks, ref.BootID
	} else {
		record["pidStartedAtExactMicro"] = ref.StartedAtUnixMicro
	}
	writeJSONFixture(t, r.root, "artifacts/agents/jobs/critic.json", record)
	retried := request
	retried.BriefPath, retried.Retry = "", 1
	if _, err := branch.RunBranchRead(retried); err == nil || !strings.Contains(err.Error(), "may still be running") || followUps != 0 {
		t.Fatalf("a round whose recorded process is alive was retried: %v %d", err, followUps)
	}
	table.live = false
	if result, err := branch.RunBranchRead(retried); err != nil || result.State != "dispatched" || result.Retry != "critic-r2" || followUps != 1 {
		t.Fatalf("a round whose recorded process and group are dead: %+v %v %d", result, err, followUps)
	}
}

func TestBranchReadCompletedUnknownInputsRetryOnceAndRejoinPending(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"missing-class", "prose-disagreement", "pending-replay"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			r := newReadFactRepository(t, false)
			r.expectStart()
			r.expectGateAndBrief()
			checks := 3
			if failure == "pending-replay" {
				checks++
			}
			for range checks {
				r.expectStart()
			}
			input := filepath.Join(t.TempDir(), "brief.md")
			if err := os.WriteFile(input, []byte("Examine retained stop inputs.\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			writeUnknown := func(job string, round int) {
				t.Helper()
				record := map[string]any{"jobId": job, "role": "code-critic", "round": round, "status": "completed", "reviews": "commit:" + r.unit, "goalId": "goal-a", "engineBuild": "executing-build", "requestedModel": "resolved-model", "findingRegister": []any{}}
				if round > 1 {
					record["parentJob"] = "critic"
				}
				writeJSONFixture(t, r.root, "artifacts/agents/jobs/"+job+".json", record)
				f := map[string]any{"id": "F1", "severity": "high", "material": true, "claim": "The reader misses retained evidence.", "evidence": "Observed the missing path.", "where": "metasystem/test.go", "change": "Read retained evidence."}
				if failure == "prose-disagreement" {
					f["class"] = "missing-reader"
				}
				dir := "artifacts/agents/critic/rounds/" + fmt.Sprint(round)
				writeJSONFixture(t, r.root, dir+"/subject.json", r.readSubject())
				writeJSONFixture(t, r.root, dir+"/return.json", map[string]any{"jobId": job, "round": round, "reviewedTree": r.readSubject().Tree, "verdictMaterialCount": 1, "findings": []any{f}})
				prose := "VERDICT: REVISE material=1\n"
				if failure == "prose-disagreement" {
					prose = "VERDICT: LAND\n"
				}
				if err := os.WriteFile(filepath.Join(r.root, dir, "return.md"), []byte(prose), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			attempts, executions := 0, 0
			request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: r.unit, GoalID: "goal-a", UnitCommit: r.unit, Repository: r, BriefPath: input, CheckClaim: claimAllowed, Gate: func(string) (string, error) { return "green", nil }, NewID: func(string) (string, error) { return "retry-gate", nil }, Delegate: func(string, string, string, string, string) (string, error) {
				writeUnknown("critic", 1)
				_, err := dispatch.CollectExamination(r.root, "critic")
				want := "incomplete stop evidence"
				if failure == "prose-disagreement" {
					want = "prose verdict disagrees"
				}
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("fixture does not expose %s: %v", failure, err)
				}
				return "critic", nil
			}, FollowUp: func(root, brief string) (string, error) {
				attempts++
				data, err := os.ReadFile(filepath.Join(r.root, "artifacts/agents/jobs/critic.json"))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(data), `"unknownExaminationRetryFrom":"critic"`) && !strings.Contains(string(data), `"unknownExaminationRetryFrom": "critic"`) {
					t.Fatal("fresh examination started before recording retry")
				}
				if failure == "pending-replay" && attempts == 1 {
					return "", errors.New("follow-up ended before reserving a child")
				}
				executions++
				writeUnknown("critic-r2", 2)
				return "critic-r2", nil
			}}
			if _, err := branch.RunBranchRead(request); err != nil {
				t.Fatal(err)
			}
			retry := func(round int64) (branch.BranchReadResult, error) {
				next := request
				next.BriefPath = ""
				next.Retry = round
				return branch.RunBranchRead(next)
			}
			if failure == "pending-replay" {
				if _, err := retry(1); err == nil {
					t.Fatal("prelaunch failure was hidden")
				}
			}
			if got, err := retry(1); err != nil || got.State != "dispatched" || got.Retry != "critic-r2" || executions != 1 {
				t.Fatalf("one fresh examination: %+v,%v executions=%d", got, err, executions)
			}
			if got, err := retry(1); err != nil || got.State != "retry-joined" || executions != 1 {
				t.Fatalf("retry replay: %+v,%v executions=%d", got, err, executions)
			}
			if _, err := retry(2); err == nil || !strings.Contains(err.Error(), "already been reserved") || executions != 1 {
				t.Fatalf("third examination admitted: %v executions=%d", err, executions)
			}
		})
	}
}
