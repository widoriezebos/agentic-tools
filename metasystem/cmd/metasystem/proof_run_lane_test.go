package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
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
		laneAccount: func(string) (string, error) { return account, nil }}
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
	if _, _, _, err := admitAsLaneOwner(t, repository, request, lease.ClassUntrusted); err == nil || !strings.Contains(err.Error(), "lane's owner") {
		t.Fatalf("an untrusted caller charged the lane: %v", err)
	}
}
