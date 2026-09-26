package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// claimFixtureOwners are the work bed's owners with this test process
// announced as the checkout's lease holder (the existing proof fixture
// seam, in the synthetic root only) and the real goal-branch claim check,
// given only the machine name and the holder's root.
func claimFixtureOwners(t *testing.T, bed *workBed) intentOwners {
	t.Helper()
	announceProofFixtureHolder(t, bed.root())
	owners := bed.workOwners()
	endpoint := owners.dependencies.endpoint
	owners.connection.endpoint = func(root string) (goal.Endpoint, error) {
		// The bed's ledger fixture is keyed by its own spelling of the root.
		if sameCanonicalPath(root, bed.root()) {
			root = bed.root()
		}
		return endpoint(root)
	}
	owners.connection.claimCheck = func(root, id string, endpoint goal.Endpoint) func() error {
		return goalBranchClaimCheckWith(root, id, endpoint, func(string, string) (string, error) { return "mac-cli", nil }, func(string) string { return bed.root() })
	}
	return owners
}

func claimFixtureBuild(t *testing.T, bed *workBed, owners intentOwners) intentResult {
	t.Helper()
	brief := filepath.Join(bed.root(), "claim-brief.md")
	os.WriteFile(brief, []byte("Build it.\n\nMaximum reader tool calls: 5\n"), 0o644)
	var stdout, stderr bytes.Buffer
	runIntentIn(mustIntentCommand(t, "build"), append([]string{bed.id, "--work", "main", "--brief", brief, "--lines", "5", "--json", "--check"}, workArgv...),
		&stdout, &stderr, bed.root(), owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("build printed no JSON: %v %q %q", err, stdout.String(), stderr.String())
	}
	return result
}

func claimHistory(file *goal.GoalFile) int {
	count := 0
	for _, line := range file.History {
		if line.Verb == "claim" {
			count++
		}
	}
	return count
}

// TestIntentBuildAutoClaimPositive: with this process the authenticated
// lease holder (epoch 1), build claims the approved, unheld goal through the
// real claim owner as mac-cli+m1, binds the stop capability to the holder's
// epoch, builds, and a repeat claims nothing more.
func TestIntentBuildAutoClaimPositive(t *testing.T) {
	t.Parallel()
	bed := newWorkBedWith(t, func(file *goal.GoalFile) {
		workApprovedBox(file)
		file.State, file.Claimed, file.StopCapability, file.StopFence = goal.StateApproved, nil, nil, nil
	})
	bed.lineage = "m1"
	owners := claimFixtureOwners(t, bed)
	classified, err := classifyVerbCallerWith(bed.root(), int64(os.Getppid()), fakeTop(bed.root()))
	if err != nil || classified.Class != lease.ClassMain || !classified.Holder || classified.ClaimEpoch == nil || *classified.ClaimEpoch != 1 {
		t.Fatalf("the fixture caller is not the authenticated holder: %+v %v", classified, err)
	}
	before := claimHistory(bed.goalFile(bedGoal))
	result := claimFixtureBuild(t, bed, owners)
	file := bed.goalFile(bedGoal)
	if result.Outcome != intentConfirmed || file.State != goal.StateClaimed || file.Claimed == nil ||
		file.Claimed.Machine != "mac-cli" || file.Claimed.Lineage != "m1" || claimHistory(file) != before+1 {
		t.Fatalf("auto-claim build: %+v claimed=%+v history=%d", result, file.Claimed, claimHistory(file))
	}
	if file.StopCapability == nil || file.StopCapability.ClaimEpoch != *classified.ClaimEpoch {
		t.Fatalf("the stop capability does not carry the holder's epoch: %+v", file.StopCapability)
	}
	claimFixtureBuild(t, bed, owners)
	if again := bed.goalFile(bedGoal); claimHistory(again) != before+1 {
		t.Fatalf("a repeated build claimed again: %d", claimHistory(again))
	}
}

// TestIntentBuildForeignClaimRefused: with the same authenticated holder, a
// goal another machine holds is never taken; the real claim check refuses
// with the holder mismatch, the ledger is unchanged and nothing launches.
func TestIntentBuildForeignClaimRefused(t *testing.T) {
	t.Parallel()
	bed := newWorkBedWith(t, func(file *goal.GoalFile) {
		workApprovedBox(file)
		if file.Claimed != nil {
			file.Claimed.Machine, file.Claimed.Lineage = "mac-other", "o1"
		}
	})
	bed.lineage = "m1"
	owners := claimFixtureOwners(t, bed)
	before := bed.goalFile(bedGoal)
	result := claimFixtureBuild(t, bed, owners)
	after := bed.goalFile(bedGoal)
	if result.Outcome != intentRefused || !strings.Contains(result.Summary, "GOAL_BRANCH_NOT_HOLDER") ||
		!strings.Contains(result.Summary, "mac-cli+m1") || after.Claimed == nil || after.Claimed.Machine != "mac-other" ||
		claimHistory(after) != claimHistory(before) || len(bed.starter.launched()) != 0 {
		t.Fatalf("foreign claim: %+v claimed=%+v launches=%v", result, after.Claimed, bed.starter.launched())
	}
}
