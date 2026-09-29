package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// admitAsLaneOwner admits a launch as the lane checkout's lease holder does.
func admitAsLaneOwner(t *testing.T, r *proofAdmissionRepository, request proofLaunchAdmission, class string) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
	t.Helper()
	request.BeforePublish = func(reservation *proofrun.AdmissionRequest) {
		*reservation = privateProofAdmissionRequest(proofrun.WithTestHostLoadSampler(*reservation, "0"))
	}
	reads := r.reads()
	classify := func(string, int64) (lease.ClassifyResult, error) {
		return lease.ClassifyResult{Class: class, Holder: class == lease.ClassMain}, nil
	}
	return admitProofLaunchWithReadsAndClassifier(candidateProofLaunchAdmission(request), func() dispatchcore.ProofAdmissionReads { return reads }, classify)
}

// TestLaneProofChargesTheLaneNotAGoal (U11b): a proof launched with the
// lane's accounting identity instead of a goal is admitted with the lane as
// the attempt's owner, and no goal's budget moves; a lane identity that does
// not resolve refuses before anything is reserved, and a lane proof never
// names a goal too.
func TestLaneProofChargesTheLaneNotAGoal(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	repository := newProofAdmissionRepositoryFixture(t, now, false)
	root := repository.root
	file := repository.goalFile(t, "standing-validation")
	before := dispatchcore.ProjectBudget(root, file, now)
	const account = "lane:0123456789ab"
	request := proofLaunchAdmission{ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"), LaneID: account,
		CapMin: "1", ScopeClass: "full", CommandClass: "testing", Now: now,
		laneAccount: func(string) (string, error) { return account, nil },
		laneOwner:   func(string, int64) error { return nil }}
	attempt, decision, joined, err := admitAsLaneOwner(t, repository, request, lease.ClassMain)
	if err != nil || joined || decision.Disposition != proofrun.DispositionExecuted || attempt.GoalID != account || attempt.AccountedGoal() != account {
		t.Fatalf("lane proof: attempt=%+v decision=%+v joined=%v err=%v", attempt, decision, joined, err)
	}
	after := dispatchcore.ProjectBudget(root, repository.goalFile(t, "standing-validation"), now)
	if after.Attempts != before.Attempts || after.ReservedJobMinutes != before.ReservedJobMinutes {
		t.Fatalf("a lane proof moved a goal budget: before=%+v after=%+v", before, after)
	}
	retained, err := proofrun.ReadAttempts(root)
	if err != nil || len(retained) != 1 || retained[0].GoalID != account {
		t.Fatalf("retained attempts=%+v err=%v", retained, err)
	}

	unresolved := request
	unresolved.laneAccount = func(string) (string, error) {
		return "", errors.New("LANE_ACCOUNT_UNRESOLVED: no landing lane is registered on this host")
	}
	if _, _, _, err := admitAsLaneOwner(t, repository, unresolved, lease.ClassMain); err == nil || !strings.Contains(err.Error(), "LANE_ACCOUNT_UNRESOLVED") {
		t.Fatalf("unresolved lane: %v", err)
	}
	other := request
	other.laneAccount = func(string) (string, error) { return "lane:ffffffffffff", nil }
	if _, _, _, err := admitAsLaneOwner(t, repository, other, lease.ClassMain); err == nil || !strings.Contains(err.Error(), "LANE_ACCOUNT_UNRESOLVED") {
		t.Fatalf("another lane's identity: %v", err)
	}
	both := request
	both.GoalID = "standing-validation"
	if _, _, _, err := admitAsLaneOwner(t, repository, both, lease.ClassMain); err == nil || !strings.Contains(err.Error(), "--goal") {
		t.Fatalf("lane and goal together: %v", err)
	}
	// A seat coordinator holding a checkout that happens to be the lane is not
	// its owner process (N-1).
	seat := request
	seat.laneOwner = func(string, int64) error {
		return errors.New("the lane checkout is held by lineage m1e-coordinator, not its landing owner")
	}
	if _, _, _, err := admitAsLaneOwner(t, repository, seat, lease.ClassMain); err == nil || !strings.Contains(err.Error(), "owner process") {
		t.Fatalf("a seat coordinator charged the lane: %v", err)
	}
}

// TestTrustedPolicyEngineForwardsTheLane (U11b F-3): the pinned trusted-base
// engine plans a lane-charged run on the lane, never on some goal; an engine
// that predates --lane is refused with the fix named, never asked with a goal.
func TestTrustedPolicyEngineForwardsTheLane(t *testing.T) {
	t.Parallel()
	argsFile := filepath.Join(t.TempDir(), "args")
	engine := filepath.Join(t.TempDir(), "policy-engine")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + argsFile + "'\nprintf '%s\\n' '{\"schemaVersion\":1}'\n"
	if err := testexec.WriteFile(engine, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	request := testingSelectionRequest{Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery, LaneID: "lane:0123456789ab"}
	if _, err := planWithTrustedPolicyEngine(engine, request, t.TempDir(), "candidate"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Fields(string(data))
	if strings.Join(args[:3], " ") != "internal test plan" || !slices.Contains(args, "--lane") || !slices.Contains(args, "lane:0123456789ab") || slices.Contains(args, "--goal") {
		t.Fatalf("policy child argv=%v", args)
	}
	old := filepath.Join(t.TempDir(), "old-engine")
	if err := testexec.WriteFile(old, []byte("#!/bin/sh\necho 'flag provided but not defined: -lane' >&2\nexit 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := planWithTrustedPolicyEngine(old, request, t.TempDir(), "candidate"); err == nil || !strings.Contains(err.Error(), "LANE_ENGINE_TOO_OLD") ||
		!strings.Contains(err.Error(), "metasystem landing restart") {
		t.Fatalf("an engine without --lane: %v", err)
	}
}
