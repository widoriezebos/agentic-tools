package plain

import (
	"os/exec"
	"testing"
	"time"
)

func TestBatchNextStepUsesTheCurrentSubjectAndMembership(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"merge", "prove", "push", "pushed", "red", "cancelled", "closed", "old commit", "old tree", "old members", "old gate", "old gate members"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newKeeperClockBed(t)
			b.wake(t)
			git := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				if args[0] == "cat-file" || args[0] == "merge-base" {
					return "", nil
				}
				return git(dir, args...)
			}
			gate := Result{BatchID: b.batch.ID, BatchMembers: b.batch.Members, Commit: "candidate-commit", Tree: "candidate", Result: Skipped, Static: Green, Attempt: "gate", At: bedNow.Format(time.RFC3339)}
			candidate := gate
			candidate.Attempt, candidate.Result = "candidate", Green
			want := true
			switch name {
			case "merge":
				git := b.seams.Git
				b.seams.Git = func(dir string, args ...string) (string, error) {
					if args[0] == "merge-base" {
						return "", exec.Command("/usr/bin/false").Run()
					}
					return git(dir, args...)
				}
			case "old commit":
				candidate.Commit, candidate.Result = "obsolete", Red
			case "old tree":
				candidate.Tree, candidate.Result = "obsolete", Red
			case "old members":
				candidate.BatchMembers = []GoalSHA{{Goal: "g", SHA: "obsolete"}}
				candidate.Result = Red
			case "cancelled":
				candidate.Result = "cancelled"
			case "old gate":
				gate.Commit, want = "obsolete", false
			case "old gate members":
				gate.BatchMembers, want = nil, false
			case "red":
				candidate.Result, want = Red, false
			case "pushed":
				writePushRecords(t, b.install, Pushed{Tree: candidate.Tree})
				want = false
			case "closed":
				b.batch.State, want = BatchClosed, false
				if err := writeBatch(b.install, b.batch, b.seams); err != nil {
					t.Fatal(err)
				}
			}
			if name != "merge" {
				if err := appendLine(gatesPath(b.install), gate, b.seams); err != nil {
					t.Fatal(err)
				}
			}
			if name != "prove" && name != "merge" && name != "old gate" && name != "old gate members" {
				if err := appendLine(resultsPath(b.install), candidate, b.seams); err != nil {
					t.Fatal(err)
				}
			}
			if next, err := batchNextStep(b.install, b.checkout, b.seams); err != nil || next != want {
				t.Fatalf("next=%v want=%v err=%v", next, want, err)
			}
		})
	}
}

func TestKeeperClockIgnoresObsoleteCandidateResults(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"cancelled", "commit", "tree", "members"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newKeeperClockBed(t)
			b.wake(t)
			gate := Result{BatchID: b.batch.ID, BatchMembers: b.batch.Members, Commit: "candidate-commit", Tree: "candidate", Result: Skipped, Static: Green, Attempt: "gate", At: bedNow.Format(time.RFC3339)}
			old := gate
			old.Result, old.Attempt = Red, "old"
			switch name {
			case "cancelled":
				old.Result = "cancelled"
			case "commit":
				old.Commit = "old"
			case "tree":
				old.Tree = "old"
			case "members":
				old.BatchMembers = nil
			}
			if err := appendLine(resultsPath(b.install), old, b.seams); err != nil {
				t.Fatal(err)
			}
			if err := appendLine(gatesPath(b.install), gate, b.seams); err != nil {
				t.Fatal(err)
			}
			b.wake(t)
			if b.agents != 2 || b.trunks != 0 {
				t.Fatalf("obsolete candidate stole the proof wake: agents=%d trunks=%d", b.agents, b.trunks)
			}
			b.wake(t)
			if b.trunks != 1 {
				t.Fatal("obsolete candidate disabled the bounded fallback")
			}
		})
	}
}
