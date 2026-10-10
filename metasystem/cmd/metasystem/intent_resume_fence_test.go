package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestGoalResumeClaimedFenceAllowsDone(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, true, func(file *goal.GoalFile) {
		// A person's reservation retains its originating claim authority.
		file.Claimed.By = "human:Wido"
		file.Claimed.EpisodeAt = file.Claimed.At
		file.Claimed.EpisodeRevision = file.Claimed.Revision
		for i := range file.History {
			if file.History[i].Verb == "claim" {
				file.History[i].Actor = "human:Wido"
				file.History[i].AuthorityOutcome = goal.AuthorityOutcomeHumanAuthorityProven
			}
		}
	})
	before := bed.goalFile(bedGoal)
	other := bed.goalFile(bedGoal)
	other.Id, other.StopFence = "another-working-claim", nil
	other.Claimed.By = ""
	other.StopCapability.FenceEpoch = 0
	for i := range other.History {
		other.History[i].Targets = []string{other.Id}
	}
	bed.addGoal(other)
	code, result := bed.runJSON(bed.owners(), "goal", "done", bedGoal, "--reason", "shipped and verified", "--lineage", "m1")
	if code == 0 || !strings.Contains(result.Summary, "only goal resume may clear its launch fence") {
		t.Fatalf("done before resume = %d %+v", code, result)
	}
	code, stdout, stderr := bed.run(bed.owners(), "goal", "resume", bedGoal, "--fixture-human-authority", "--lineage", "m1")
	if code != 0 {
		t.Fatalf("resume = %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	t.Logf("claimed with fence: %s", strings.TrimSpace(stdout))
	after := bed.goalFile(bedGoal)
	last := after.History[len(after.History)-1]
	if after.StopFence != nil || after.State != goal.StateClaimed || !reflect.DeepEqual(after.Budget, before.Budget) ||
		last.Verb != "resume" || last.Actor != "human:Wido" || !strings.Contains(last.Reason, before.StopFence.StopID) || !strings.Contains(last.Reason, before.StopFence.Reason) {
		t.Fatalf("resume did not clear and record the fence under the standing box: after=%+v last=%+v", after, last)
	}
	code, result = bed.runJSON(bed.owners(), "goal", "done", bedGoal, "--reason", "shipped and verified", "--lineage", "m1")
	if code != 0 || result.Outcome != intentConfirmed || bed.goalFile(bedGoal).State != goal.StateDone {
		t.Fatalf("done after resume = %d %+v", code, result)
	}
}

func TestGoalResumeClaimedWithoutFenceIsUnchanged(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	before := goal.RenderFile(bed.goalFile(bedGoal))
	code, stdout, stderr := bed.run(bed.owners(), "goal", "resume", bedGoal)
	if code != 0 || !strings.Contains(stdout, bedGoal+" is running under its standing box; there is nothing to resume") || stderr != "" || bed.repo.publications != 0 {
		t.Fatalf("unfenced resume = %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	t.Logf("claimed without fence: %s", strings.TrimSpace(stdout))
	if string(goal.RenderFile(bed.goalFile(bedGoal))) != string(before) {
		t.Fatal("nothing to resume changed the goal")
	}
	bed.expectBindings(0)
}

func TestGoalResumeStoppedFenceKeepsLandingAndOwner(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, true, func(file *goal.GoalFile) {
		file.Landing = &goal.LandingRecord{At: file.History[len(file.History)-1].At, Opid: file.History[len(file.History)-1].Opid}
	})
	before := bed.goalFile(bedGoal)
	code, stdout, stderr := bed.run(bed.owners(), "goal", "resume", bedGoal, "--fixture-human-authority", "--lineage", "m1")
	if code != 0 {
		t.Fatalf("stopped resume = %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	t.Logf("stopped with fence: %s", strings.TrimSpace(stdout))
	after := bed.goalFile(bedGoal)
	if after.StopFence != nil || after.State != goal.StateClaimed || after.Claimed == nil ||
		after.Claimed.Machine != before.Claimed.Machine || after.Claimed.Lineage != before.Claimed.Lineage ||
		after.Claimed.Revision != after.Revision || after.StopCapability.FenceEpoch != 0 ||
		!reflect.DeepEqual(after.Landing, before.Landing) || !reflect.DeepEqual(after.Budget, before.Budget) {
		t.Fatalf("stopped resume lost its standing execution or landing: before=%+v after=%+v", before, after)
	}
}

// The recorded headers bind claim epoch 9 to fence epoch 1. The accepted
// snapshot can precede those fences while the published ledger contains them.
func TestGoalResumeRecordedFencesRefreshAcceptedLedger(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"fleet-provider-and-session-recovery", "process-changes-cover-declarations-and-interventions"} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			bed := newIntentBed(t, false, nil)
			data, err := os.ReadFile(filepath.Join("testdata", "resume-fences", id+".md"))
			if err != nil {
				t.Fatal(err)
			}
			file, problems := goal.ParseFile(data)
			if len(problems) != 0 {
				t.Fatalf("recorded header: %v", problems)
			}
			if !file.IsFencedClaim() || file.StopCapability.ClaimEpoch != 9 || file.StopFence.Epoch != 1 {
				t.Fatalf("recorded fence binding lost: %+v", file)
			}
			fence := *file.StopFence
			stale, problems := goal.ParseFile(data)
			if len(problems) != 0 {
				t.Fatal(problems)
			}
			stale.StopFence, stale.StopCapability.FenceEpoch = nil, 0
			stale.Revision--
			stale.History = stale.History[:len(stale.History)-1]
			bed.addGoal(stale)
			oldTip := bed.repo.accepted
			fresh := obligationFilesCopy(bed.repo.commit(oldTip).files)
			fresh["plans/goals/"+id+".md"] = data
			freshTip := "0000000000000000000000000000000000000002"
			bed.repo.commits[freshTip] = obligationCommit{parent: oldTip, files: fresh, at: time.Date(2026, 10, 10, 15, 10, 0, 0, time.UTC)}
			bed.repo.canonical, bed.repo.serial = freshTip, 2
			capability := file.StopCapability
			if err := goal.WriteStopBatch(bed.root(), goal.StopBatch{
				StopID: fence.StopID, GoalID: id, GoalRevision: fence.Revision,
				FenceEpoch: fence.Epoch, CapabilityGeneration: fence.CapabilityGeneration,
				Machine: capability.Machine, ClaimEpoch: capability.ClaimEpoch,
				Reason: fence.Reason, State: goal.StopBatchComplete,
				OpenedAt: fence.ClosedAt, UpdatedAt: fence.ClosedAt, CompletedAt: fence.ClosedAt, Pass: 1,
			}); err != nil {
				t.Fatal(err)
			}
			owners := bed.owners()
			owners.commandNow = func(string) (time.Time, error) { return time.Date(2026, 10, 10, 16, 0, 0, 0, time.UTC), nil }
			owners.binding = func(root, target string, now time.Time) (dispatchcore.GoalBinding, error) {
				endpoint, err := owners.dependencies.endpoint(root)
				if err != nil {
					return dispatchcore.GoalBinding{}, err
				}
				projection, err := goal.Project(endpoint, false, now)
				if err != nil {
					return dispatchcore.GoalBinding{}, err
				}
				current := projection.Tree.Live[target]
				return dispatchcore.GoalBinding{GoalID: target, Revision: current.Claimed.Revision, Fence: current.StopFence, Capability: *current.StopCapability, File: current}, nil
			}
			code, stdout, stderr := bed.run(owners, "goal", "resume", id, "--fixture-human-authority", "--lineage", "m1")
			if code != 0 {
				t.Fatalf("resume = %d stdout=%q stderr=%q", code, stdout, stderr)
			}
			after := bed.goalFile(id)
			event := after.History[len(after.History)-1]
			if after.StopFence != nil || after.State != goal.StateClaimed || after.StopCapability.FenceEpoch != 0 ||
				!reflect.DeepEqual(after.Budget, file.Budget) || after.Claimed.Machine != file.Claimed.Machine || after.Claimed.Lineage != file.Claimed.Lineage ||
				event.Verb != "resume" || event.Actor != "human:Wido" || !strings.Contains(event.Reason, fence.StopID) || !strings.Contains(event.Reason, fence.Reason) {
				t.Fatalf("resume used the stale ledger or lost the recorded binding: stdout=%q fence=%+v claim=%+v event=%+v", stdout, after.StopFence, after.Claimed, event)
			}
			t.Logf("recorded fenced goal: %s", strings.TrimSpace(stdout))
		})
	}
}

func TestGoalResumeRefreshFailurePreservesFence(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, true, nil)
	before := goal.RenderFile(bed.goalFile(bedGoal))
	bed.repo.captureErr = errors.New("ledger transport unavailable")
	code, result := bed.runJSON(bed.owners(), "goal", "resume", bedGoal, "--fixture-human-authority", "--lineage", "m1")
	if code == 0 || result.Outcome == intentUnchanged || !strings.Contains(result.Summary, "ledger transport unavailable") || bed.repo.publications != 0 {
		t.Fatalf("failed refresh = %d %+v", code, result)
	}
	if string(goal.RenderFile(bed.goalFile(bedGoal))) != string(before) {
		t.Fatal("failed refresh changed the fence")
	}
	bed.expectBindings(0)
}
