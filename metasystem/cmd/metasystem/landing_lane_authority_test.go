package main

// The landing lane's stable claim identity (lane design r10 K7): a join
// hands its goal, at the source seat's word, to {lane machine, landing-lane,
// custody epoch of the host record}; no agent, owner or lease holder of the
// lane checkout takes part.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

var laneAuthorityNow = time.Date(2026, 8, 30, 9, 30, 0, 0, time.UTC)

// laneAuthorityBed is a seat whose ledger holds standing-validation claimed
// by mac-cli+m1, and a registered nested landing checkout named lane-host
// on a host home of its own, registered twice so its custody epoch is 2.
type laneAuthorityBed struct {
	home, seat, lane string
	record           lane.Record
	// base is the lane's main tree; patch adds one unit to it.
	base  string
	patch []byte
}

func newLaneAuthorityBed(t *testing.T) laneAuthorityBed {
	t.Helper()
	bed := laneAuthorityBed{seat: syncedClaimedGoalFixture(t)}
	amendSyncedGoalFixture(t, bed.seat, "the seat's claim carries its stop capability", func(file *goal.GoalFile) {
		file.StopCapability = &goal.StopCapability{Generation: 1, Revision: file.Claimed.Revision, Machine: "mac-cli", ClaimEpoch: 1}
	})
	base := t.TempDir()
	bed.home, bed.lane = filepath.Join(base, "home"), filepath.Join(base, "lane")
	other := filepath.Join(base, "earlier-lane")
	registerLane(t, bed.home, other, "Wido", laneAuthorityNow)
	registerLane(t, bed.home, bed.lane, "Wido", laneAuthorityNow)
	goalSyncMutationGit(t, bed.lane, "config", "metasystem.goal.machine", "lane-host")
	record, ok, err := lane.Read(bed.home)
	if err != nil || !ok || record.CustodyEpoch != 2 || filepath.Base(record.Install) != "metasystem" {
		t.Fatalf("lane record = %+v %t %v; want a nested lane at custody epoch 2", record, ok, err)
	}
	bed.record = record
	git := func(args ...string) string {
		return strings.TrimSpace(goalSyncMutationGit(t, bed.lane, append([]string{"-c", "user.name=lane", "-c", "user.email=lane@example.invalid", "-c", "commit.gpgsign=false"}, args...)...))
	}
	git("add", "-A")
	git("commit", "-q", "-m", "the lane's main")
	unit := filepath.Join(bed.lane, "units", "a.txt")
	if err := os.MkdirAll(filepath.Dir(unit), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unit, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "units/a.txt")
	bed.patch = []byte(strings.TrimRight(goalSyncMutationGit(t, bed.lane, "diff", "--cached", "--binary"), "\n") + "\n")
	git("rm", "-q", "--cached", "units/a.txt")
	if err := os.Remove(unit); err != nil {
		t.Fatal(err)
	}
	bed.base = git("rev-parse", "HEAD^{tree}")
	return bed
}

func (bed laneAuthorityBed) ledger(t *testing.T) *goal.GoalFile {
	t.Helper()
	data := goalSyncMutationGit(t, bed.seat, "show", goal.AcceptedRef+":plans/goals/standing-validation.md")
	file, problems := goal.ParseFile([]byte(data))
	if len(problems) != 0 {
		t.Fatalf("ledger entry: %v", problems)
	}
	return file
}

// A join works while no agent runs: the goal is handed to the lane's claim
// identity at the source seat's word, the member is queued, and nothing is
// started for it.
func TestJoinWithNoAgentRunning(t *testing.T) {
	t.Parallel()
	bed := newLaneAuthorityBed(t)
	if _, err := lease.CurrentHolder(bed.lane); !errors.Is(err, lease.ErrLeaseAbsent) {
		t.Fatalf("the lane checkout has a lease holder (%v); the bed must run no agent", err)
	}
	const batchID = "01j5x00000000000000000kd01"
	dependencies := batchowner.ProductionBatchJoinDependencies()
	dependencies.Binding = func(string, string, time.Time) (dispatchcore.GoalBinding, error) {
		return dispatchcore.GoalBinding{Revision: 3, Machine: "mac-cli", Lineage: "m1",
			File: &goal.GoalFile{Claimed: &goal.ClaimRecord{AccountingRevision: 2}}, Capability: goal.StopCapability{ClaimEpoch: 1}}, nil
	}
	dependencies.Chain = func(string, string, string, uint64) (batch.CertifiedChain, error) {
		return batch.CertifiedChain{ID: "chain-a", Patch: bed.patch}, nil
	}
	dependencies.Base = func(string) (string, error) { return bed.base, nil }
	dependencies.Mint = func() (string, error) { return batchID, nil }
	dependencies.Author, dependencies.ReleaseSet, dependencies.CostForecast = nil, nil, nil
	dependencies.ProtectedTests = func(string, string, string) error { return nil }
	dependencies.Plan = func(string, string, string) (testpolicy.Plan, error) {
		return testpolicy.Plan{RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard}, nil
	}
	dependencies.AdmissionRun = func(_ string, _ string, unit batch.Unit) (batch.JoinAdmission, error) {
		return batch.JoinAdmission{Tree: unit.Admission.Tree, Status: "verified", AttemptID: "join-admission"}, nil
	}
	// The production handover, reading this bed's host home.
	dependencies.Handover = batchowner.LaneForwardHandover(func() (string, error) { return bed.home, nil }, &batchowner.BatchOwnerCalls)
	record, err := batchowner.ExecuteBatchJoin(batchowner.BatchJoinRequest{SeatRoot: bed.seat, LandingRoot: bed.lane,
		GoalID: "standing-validation", ChainID: "chain-a", At: laneAuthorityNow}, dependencies)
	if err != nil {
		t.Fatalf("join with no agent running: %v", err)
	}
	if record.BatchID != batchID || len(record.Units) != 1 || record.Units[0].State != batch.UnitJoined {
		t.Fatalf("queued batch = %+v", record)
	}
	file := bed.ledger(t)
	claim := lane.ClaimIdentity{Machine: file.Claimed.Machine, Lineage: file.Claimed.Lineage, Epoch: uint64(file.StopCapability.ClaimEpoch)}
	if claim.Machine != "lane-host" || claim.Lineage != lane.ClaimLineage || claim.Epoch != bed.record.CustodyEpoch {
		t.Fatalf("custody went to %+v, want lane-host+%s at custody epoch %d", claim, lane.ClaimLineage, bed.record.CustodyEpoch)
	}
	if handed := file.Claimed.HandedOver; handed.Batch != batchID || handed.FromMachine != "mac-cli" || handed.FromLineage != "m1" {
		t.Fatalf("handed over = %+v, want the seat's claim recorded for batch %s", handed, batchID)
	}
	if _, err := lease.CurrentHolder(bed.lane); !errors.Is(err, lease.ErrLeaseAbsent) {
		t.Fatalf("the join started something that holds the lane checkout: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bed.lane, "artifacts", "agents", "supervision")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the join asked the lane's supervision for an owner: %v", err)
	}
}

// A handover names the lane only as the host record registers it: another
// epoch or another machine is not the lane, and nothing moves.
func TestLaneHandoverRefusesAStaleLaneIdentity(t *testing.T) {
	t.Parallel()
	bed := newLaneAuthorityBed(t)
	if _, err := laneClaimTargetLiveness(bed.home, "lane-host", int64(bed.record.CustodyEpoch)); err != nil {
		t.Fatalf("the registered lane is not live: %v", err)
	}
	for _, target := range []struct {
		machine string
		epoch   int64
	}{{"lane-host", 1}, {"other-host", 2}, {"lane-host", 0}} {
		if live, err := laneClaimTargetLiveness(bed.home, target.machine, target.epoch); err == nil || live.String() == "alive" {
			t.Fatalf("target %+v read %s (%v); want refused", target, live, err)
		}
	}
	if _, err := laneClaimTargetLiveness("", "lane-host", 2); err == nil || !strings.Contains(err.Error(), "no host lane record") {
		t.Fatalf("no home named: %v", err)
	}
}
