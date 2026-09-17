package branch_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

func readUnit(t *testing.T, f *branchFixture, unit, commit string) {
	t.Helper()
	digest, err := branch.UnitDigest(f.root, commit)
	if err != nil {
		t.Fatal(err)
	}
	record := fmt.Sprintf("metasystem/records/misc/goal-a-%s-read.md", unit)
	write(t, f.root, record, "Read commit "+commit+" with unit digest "+digest+" and found it clean.\n")
	if _, _, err := branch.CommitRead(branch.CommitReadRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Units: strings.Split(unit, "+"),
		OpID: "read-" + unit, CheckClaim: claimAllowed, ReaderRecord: record,
		GateRunID: "fast-" + unit, GateTree: unitTree(t, f, commit),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestStatusLandReadyPrefixAndParkSafety(t *testing.T) {
	f := newBranchFixture(t)
	u1 := commitUnit(t, f, "u1", "metasystem/one.go", "one")
	readUnit(t, f, "u1", u1)
	_ = commitUnit(t, f, "u2", "metasystem/two.go", "two")
	u3 := commitUnit(t, f, "u3", "metasystem/three.go", "three")
	readUnit(t, f, "u3", u3)
	tip := git(t, f.root, "rev-parse", "refs/heads/goal/goal-a")
	status, err := branch.InspectStatus(f.root, f.base, tip, "goal-a")
	if err != nil {
		t.Fatal(err)
	}
	if status.Prefix != 1 || len(status.Units) != 3 || status.Units[0].ReadState != "read clean" ||
		status.Units[1].ReadState != "built" || status.Units[2].ReadState != "read clean" {
		t.Fatalf("status = %+v", status)
	}
	if _, err := branch.Push(pushRequest(f, "status-push")); err != nil {
		t.Fatal(err)
	}
	remote := func() (string, string, bool, error) { return f.base, tip, true, nil }
	parked, err := branch.CheckParkBranch(f.root, "goal-a", "continue", remote)
	if err != nil || !parked.Branch || !strings.Contains(parked.Summary, "u3") ||
		!strings.Contains(parked.Summary, u3) || !strings.Contains(parked.Summary, "read clean") {
		t.Fatalf("park state = %+v err=%v", parked, err)
	}
	commitUnit(t, f, "u4", "metasystem/four.go", "four")
	_, err = branch.CheckParkBranch(f.root, "goal-a", "continue", remote)
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != branch.ParkUnpushedCode {
		t.Fatalf("unpushed park = %v", err)
	}

	without := newBranchFixture(t)
	remoteCalls := 0
	unreadable := func() (string, string, bool, error) {
		remoteCalls++
		return "", "", false, errors.New("remote unavailable")
	}
	if state, err := branch.CheckParkBranch(without.root, "goal-a", "ordinary next step", unreadable); err != nil || state.Branch || remoteCalls != 0 {
		t.Fatalf("branchless park = %+v err=%v", state, err)
	}
	narrated := git(t, without.root, "commit-tree", without.base+"^{tree}", "-p", without.base, "-m", "unit\n\nGoal-Unit: goal-a/u1")
	if shouldSweep, err := branch.ShouldSweep(without.root, "goal-a", "ordinary next step"); err != nil || shouldSweep {
		t.Fatalf("ordinary branchless goal should sweep=%v err=%v", shouldSweep, err)
	}
	if shouldSweep, err := branch.ShouldSweep(without.root, "goal-a", "resume commit "+narrated); err != nil || !shouldSweep {
		t.Fatalf("narrated unit goal should sweep=%v err=%v", shouldSweep, err)
	}
	_, err = branch.CheckParkBranch(without.root, "goal-a", "resume commit "+narrated, unreadable)
	if !errors.As(err, &refusal) || refusal.Code != branch.ParkUnpushedCode {
		t.Fatalf("missing narrated branch = %v", err)
	}
	if remoteCalls != 0 {
		t.Fatalf("branchless park read remote %d times", remoteCalls)
	}
}

func TestStatusAndReadBindWholeBuildList(t *testing.T) {
	f := newBranchFixture(t)
	stage(t, f, "metasystem/multi.go", "multi")
	commit, err := branch.CommitStaged(branch.CommitRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Units: []string{"5", "6", "7a", "7b"},
		OpID: "multi-status", Kind: branch.Unit, CheckClaim: claimAllowed,
	})
	if err != nil {
		t.Fatal(err)
	}
	readUnit(t, f, "5+6+7a+7b", commit)
	tip := git(t, f.root, "rev-parse", "refs/heads/goal/goal-a")
	status, err := branch.InspectStatus(f.root, f.base, tip, "goal-a")
	if err != nil || status.Prefix != 1 || len(status.Units) != 1 || strings.Join(status.Units[0].Units, "+") != "5+6+7a+7b" || status.Units[0].ReadState != "read clean" {
		t.Fatalf("multi-unit status=%+v err=%v", status, err)
	}
}
