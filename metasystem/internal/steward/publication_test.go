package steward

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
)

func TestReviewObligationCandidateGitAdapter(t *testing.T) {
	t.Parallel()
	for _, reference := range []string{"held", "missing"} {
		t.Run(reference, func(t *testing.T) {
			t.Parallel()
			f := testgoal.NewPublicationFixture(t)
			candidate := f.Candidate(reference)
			writeValidationRun(t, f.Root, "cadence-red", run.StatusRed, "red.log", 1)
			path := run.RecordPath(f.Root, "cadence-red")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var record run.Record
			if err := json.Unmarshal(data, &record); err != nil {
				t.Fatal(err)
			}
			record.GoalId = "held"
			data, err = json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0644); err != nil {
				t.Fatal(err)
			}
			err = linkCadenceFailure(f.Root, validationWindowObservation{RunID: "cadence-red", AttemptID: "attempt-red"}, f.Now)
			tip := f.Git("rev-parse", goal.LocalLedgerBranch)
			if reference == "missing" {
				f.AssertRefusal(err)
				if tip != candidate {
					t.Fatalf("refused obligation published %s", tip)
				}
			} else if err != nil || tip == candidate {
				t.Fatalf("valid obligation: tip=%s error=%v", tip, err)
			}
		})
	}
}

func TestSeatRevivalCandidateGitAdapter(t *testing.T) {
	t.Parallel()
	for _, reference := range []string{"held", "missing"} {
		t.Run(reference, func(t *testing.T) {
			t.Parallel()
			f := testgoal.NewPublicationFixture(t)
			data, err := os.ReadFile(filepath.Join(f.Root, "plans/goals/held.md"))
			if err != nil {
				t.Fatal(err)
			}
			file, problems := goal.ParseFile(data)
			if len(problems) > 0 {
				t.Fatal(problems)
			}
			file.State = goal.StateApproved
			file.Claimed = nil
			file.Revision++
			file.Tier = 1
			file.Budget = &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 2, ReviewRoundLimit: 0}
			approval := goal.HistoryLine{At: "2026-10-14T12:01:00Z", Opid: goal.Opid("01J5X000000000000000000006", "fixture", "session"), Verb: "approve", Actor: "human:Wido", Targets: []string{"held"}, Keep: -1}
			file.History = append(file.History, approval)
			file.Approved = &goal.ApprovalRecord{By: approval.Actor, At: approval.At, Revision: file.Revision, EpisodeRevision: file.Revision, Opid: approval.Opid, Authority: goal.ApprovalAuthorityProven, Digest: goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget)}
			f.Write("plans/goals/held.md", string(goal.RenderFile(file)))
			f.Base = f.Commit()
			f.Git("update-ref", goal.LocalLedgerBranch, f.Base)
			f.Git("update-ref", goal.AcceptedRef, f.Base)
			if err := goal.ValidateCommit(f.Root, f.Base); err != nil {
				t.Fatal(err)
			}
			candidate := f.Candidate(reference)
			err = claimSeatIdleGoal(f.Root, Intent{Goal: "held", SeatActor: &SeatActor{Machine: "fixture", Lineage: "session"}, SeatClaimEpoch: 1})
			tip := f.Git("rev-parse", goal.LocalLedgerBranch)
			if reference == "missing" {
				f.AssertRefusal(err)
				if tip != candidate {
					t.Fatalf("refused revival published %s", tip)
				}
			} else if err != nil || tip == candidate {
				t.Fatalf("valid revival: tip=%s error=%v", tip, err)
			}
		})
	}
}
