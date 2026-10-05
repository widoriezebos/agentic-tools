package branch_test

import (
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// reviewedAmendBed uses Git because replayed commits and retained refs are the behavior under test.
func reviewedAmendBed(t *testing.T) (*branchFixture, string, string, string) {
	t.Helper()
	f := newBranchFixture(t)
	var second, previous string
	for _, unit := range []string{"u1", "u2"} {
		commit := commitUnit(t, f, unit, "metasystem/"+unit+".go", "one\n")
		record := "metasystem/records/misc/" + unit + "-read.md"
		write(t, f.root, record, "Reviewed commit "+commit+".\nChange "+mustUnitDigest(t, f.root, commit)+".\nFound it clean.\n")
		read, _, err := branch.CommitRead(branch.CommitReadRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
			GoalID: "goal-a", Unit: unit, OpID: "read-" + unit, ReaderRecord: record, CheckClaim: claimAllowed,
			GateRunID: "first-gate", GateTree: unitTree(t, f, commit)})
		if err != nil {
			t.Fatal(err)
		}
		second, previous = commit, read
	}
	stage(t, f, "metasystem/u1.go", "corrected\n")
	tip, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", Unit: "u1", OpID: "correct-u1", Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed})
	if err != nil {
		t.Fatal(err)
	}
	return f, previous, second, tip
}

func TestAmendCarriesLaterReviewGitAdapter(t *testing.T) {
	t.Parallel()
	f, previous, second, amended := reviewedAmendBed(t)
	gates := 0
	req := branch.CarryRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", CheckClaim: claimAllowed,
		Gate: func(dir string) (string, error) {
			gates++
			if git(t, dir, "show", "HEAD:metasystem/u2.go") != "one" {
				t.Fatal("gate did not run on the replayed unit")
			}
			return "checks passed", nil
		}}
	carried, err := branch.CarryReviews(req)
	if err != nil || !slices.Equal(carried.Carried, []string{"u2"}) || !slices.Equal(carried.NeedsReview, []string{"u1"}) || carried.NewTip == amended || gates != 1 {
		t.Fatalf("carry = %+v, gates = %d, error = %v", carried, gates, err)
	}
	commits, err := branch.ValidateRange(f.root, f.base, carried.NewTip, "goal-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 4 || commits[1].ID == second || commits[3].Kind != branch.Read {
		t.Fatalf("replayed branch = %+v", commits)
	}
	info, err := branch.KindOf(f.root, carried.NewTip, "goal-a")
	if err != nil || info.CommitID != commits[1].ID {
		t.Fatalf("carried review = %+v, error = %v", info, err)
	}
	att, err := branch.ValidateAttestationAt(f.root, carried.NewTip, f.base, "goal-a", "u2", commits[1].ID)
	if err != nil || att.Carry == nil || att.Carry.FromCommit != second || att.Carry.ToCommit != commits[1].ID ||
		len(att.TestsChanged) != 0 {
		t.Fatalf("attestation = %+v, error = %v", att, err)
	}
	status, err := branch.InspectStatus(f.root, f.base, carried.NewTip, "goal-a")
	if err != nil || status.Prefix != 0 || len(status.Units) != 2 || status.Units[0].ReadState == "read clean" || status.Units[1].ReadState != "read clean" {
		t.Fatalf("hand-in status = %+v, error = %v", status, err)
	}
	again, err := branch.CarryReviews(req)
	if err != nil || len(again.Carried) != 0 || !slices.Equal(again.NeedsReview, []string{"u1"}) || again.NewTip != carried.NewTip || gates != 1 {
		t.Fatalf("repeated carry = %+v, gates = %d, error = %v", again, gates, err)
	}
	if git(t, f.root, "rev-parse", "refs/metasystem/goals/before/goal-a/"+previous) != previous {
		t.Fatal("the reviewed commits are no longer retained")
	}
}

func TestAmendChangedLaterUnitNeedsReviewGitAdapter(t *testing.T) {
	t.Parallel()
	f, _, _, _ := reviewedAmendBed(t)
	stage(t, f, "metasystem/u2.go", "different\n")
	tip, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", Unit: "u2", OpID: "correct-u2", Kind: branch.Unit, Amend: true, CheckClaim: claimAllowed})
	if err != nil {
		t.Fatal(err)
	}
	carried, err := branch.CarryReviews(branch.CarryRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", CheckClaim: claimAllowed, Gate: func(string) (string, error) {
			t.Fatal("changed units cannot carry a review")
			return "", nil
		}})
	if err != nil || len(carried.Carried) != 0 || !slices.Equal(carried.NeedsReview, []string{"u1", "u2"}) || carried.NewTip != tip {
		t.Fatalf("changed carry = %+v, error = %v", carried, err)
	}
}

func TestAmendKeepsReviewedTipGitAdapter(t *testing.T) {
	t.Parallel()
	f, previous, _, tip := reviewedAmendBed(t)
	if got := git(t, f.root, "rev-parse", "refs/metasystem/goals/before/goal-a/"+previous); got != previous {
		t.Fatalf("kept tip = %s, want %s", got, previous)
	}
	if got := git(t, f.root, "rev-parse", "HEAD"); got != tip {
		t.Fatalf("installed tip = %s, want %s", got, tip)
	}
}
