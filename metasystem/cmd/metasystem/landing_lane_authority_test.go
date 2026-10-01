package main

// The landing lane's stable claim identity (lane design r10 K7): a join
// hands its goal, at the source seat's word, to {lane machine, landing-lane,
// custody epoch of the host record}; no agent, owner or lease holder of the
// lane checkout takes part.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
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

const laneAuthorityBatch = "01j5x00000000000000000kd01"

// join joins standing-validation from the seat to the lane through the
// production join and the production handover (reading this bed's host
// home); the stubs stand in only for reading the seat's build.
func (bed laneAuthorityBed) join(t *testing.T) (batch.Record, error) {
	t.Helper()
	dependencies := batchowner.ProductionBatchJoinDependencies()
	dependencies.Binding = func(string, string, time.Time) (dispatchcore.GoalBinding, error) {
		return dispatchcore.GoalBinding{Revision: 3, Machine: "mac-cli", Lineage: "m1",
			File: &goal.GoalFile{Claimed: &goal.ClaimRecord{AccountingRevision: 2}}, Capability: goal.StopCapability{ClaimEpoch: 1}}, nil
	}
	dependencies.Chain = func(string, string, string, uint64) (batch.CertifiedChain, error) {
		return batch.CertifiedChain{ID: "chain-a", Patch: bed.patch}, nil
	}
	dependencies.Base = func(string) (string, error) { return bed.base, nil }
	dependencies.Mint = func() (string, error) { return laneAuthorityBatch, nil }
	dependencies.Author, dependencies.ReleaseSet, dependencies.CostForecast = nil, nil, nil
	dependencies.ProtectedTests = func(string, string, string) error { return nil }
	dependencies.Plan = func(string, string, string) (testpolicy.Plan, error) {
		return testpolicy.Plan{RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard}, nil
	}
	dependencies.AdmissionRun = func(_ string, _ string, unit batch.Unit) (batch.JoinAdmission, error) {
		return batch.JoinAdmission{Tree: unit.Admission.Tree, Status: "verified", AttemptID: "join-admission"}, nil
	}
	// The production handover, reading this bed's host home.
	dependencies.Handover = batchowner.LaneForwardHandover(func() (string, error) { return bed.home, nil }, &batchowner.LaneCalls)
	return batchowner.ExecuteBatchJoin(batchowner.BatchJoinRequest{SeatRoot: bed.seat, LandingRoot: bed.lane,
		GoalID: "standing-validation", ChainID: "chain-a", At: laneAuthorityNow}, dependencies)
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
	record, err := bed.join(t)
	if err != nil {
		t.Fatalf("join with no agent running: %v", err)
	}
	if record.BatchID != laneAuthorityBatch || len(record.Units) != 1 || record.Units[0].State != batch.UnitJoined {
		t.Fatalf("queued batch = %+v", record)
	}
	file := bed.ledger(t)
	claim := lane.ClaimIdentity{Machine: file.Claimed.Machine, Lineage: file.Claimed.Lineage, Epoch: uint64(file.StopCapability.ClaimEpoch)}
	if claim.Machine != "lane-host" || claim.Lineage != lane.ClaimLineage || claim.Epoch != bed.record.CustodyEpoch {
		t.Fatalf("custody went to %+v, want lane-host+%s at custody epoch %d", claim, lane.ClaimLineage, bed.record.CustodyEpoch)
	}
	if handed := file.Claimed.HandedOver; handed.Batch != laneAuthorityBatch || handed.FromMachine != "mac-cli" || handed.FromLineage != "m1" {
		t.Fatalf("handed over = %+v, want the seat's claim recorded for batch %s", handed, laneAuthorityBatch)
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

// The lane's new claim identity activates only when the old owner holds
// nothing (lane design r10 §5, Astra R9-01): while the ledger shows any
// goal claimed by the old owner lineage landing-m1l, landing set refuses,
// names each one and the command that settles it, and registers nothing;
// an unreadable ledger registers nothing either.
func TestLandingSetRefusedWhileTheOldOwnerHoldsClaims(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	bed.oldClaims = []string{"goal-a", "goal-b"}
	code, stdout, stderr := bed.run(t, "landing", "set", bed.landingA, "--json")
	var result intentResult
	if err := json.Unmarshal([]byte(stdout+stderr), &result); err != nil {
		t.Fatal(err)
	}
	if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "goal-a") || !strings.Contains(result.Summary, "goal-b") ||
		result.Next == nil || !slices.Equal(result.Next.Argv[:4], []string{"metasystem", "goal", "release", "goal-a"}) {
		t.Fatalf("set with old-owner claims = %d %+v", code, result)
	}
	if _, ok, _ := lane.Read(bed.home); ok {
		t.Fatal("the lane was registered while the old owner held claims")
	}
	bed.oldClaims, bed.oldClaimsErr = nil, errors.New("the ledger can't be fetched")
	if code, _, _ := bed.run(t, "landing", "set", bed.landingA); code == 0 {
		t.Fatal("the lane was registered on a ledger that could not be read")
	}
	if _, ok, _ := lane.Read(bed.home); ok {
		t.Fatal("the lane was registered on a ledger that could not be read")
	}
	bed.oldClaimsErr = nil
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("set once the old owner holds nothing = %d %s", code, stderr)
	}
}

// The old owner's claims are read from the ledger: a goal claimed under
// landing-m1l is named, one the lane's claim identity holds is not.
func TestOldOwnerClaimsAreReadFromTheLedger(t *testing.T) {
	t.Parallel()
	seat := syncedClaimedGoalFixture(t)
	if held, err := laneOldOwnerClaims(seat, laneAuthorityNow); err != nil || len(held) != 0 {
		t.Fatalf("a seat's claim read as the old owner's: %v %v", held, err)
	}
	amendSyncedGoalFixture(t, seat, "the old owner holds it", func(file *goal.GoalFile) {
		file.Claimed.Machine, file.Claimed.Lineage = "landing", lane.OldOwnerLineage
		file.Claimed.HandedOver = goal.HandedOver{FromMachine: "mac-cli", FromLineage: "m1", FromEpoch: 1, Batch: "01j5x00000000000000000kd09"}
		file.StopCapability = &goal.StopCapability{Generation: 1, Revision: file.Claimed.Revision, Machine: "landing", ClaimEpoch: 1}
	})
	if held, err := laneOldOwnerClaims(seat, laneAuthorityNow); err != nil || !slices.Equal(held, []string{"standing-validation"}) {
		t.Fatalf("old-owner claims = %v %v", held, err)
	}
	amendSyncedGoalFixture(t, seat, "the lane's claim identity holds it", func(file *goal.GoalFile) {
		file.Claimed.Lineage = lane.ClaimLineage
	})
	if held, err := laneOldOwnerClaims(seat, laneAuthorityNow); err != nil || len(held) != 0 {
		t.Fatalf("the lane's own claim read as the old owner's: %v %v", held, err)
	}
	amendSyncedGoalFixture(t, seat, "a claim the lane holds for no batch", func(file *goal.GoalFile) {
		file.Claimed.HandedOver = goal.HandedOver{}
	})
	if held, err := laneHeldGoals(seat, laneAuthorityNow); err != nil || !slices.Equal(held, []string{"standing-validation"}) {
		t.Fatalf("unset's hint does not list the goal the lane's claim identity holds: %v %v", held, err)
	}
}

// landing unset's hint for a gone lane names goal release G for each goal
// the ledger still shows the lane holding: a person's release of a claim
// the lane's claim identity holds is a foreign release a person may make,
// and it gives the goal back.
func TestAPersonReleasesAGoalTheLaneHolds(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, func(file *goal.GoalFile) {
		file.Claimed.HandedOver = goal.HandedOver{FromMachine: file.Claimed.Machine, FromLineage: file.Claimed.Lineage, FromEpoch: 1, Batch: laneAuthorityBatch}
		file.Claimed.Machine, file.Claimed.Lineage = "lane-host", lane.ClaimLineage
		if file.StopCapability != nil {
			file.StopCapability.Machine = "lane-host"
		}
	})
	held := bed.goalFile(bedGoal)
	if held.Claimed == nil || held.Claimed.Lineage != lane.ClaimLineage {
		t.Fatalf("fixture goal is not held by the lane: %+v", held.Claimed)
	}
	// The person's terminal proves itself at the stopping act, as at any
	// terminal (rule H1); the hint's command guides to naming the person
	// when the terminal is not the enrolled one.
	reader := goalSyncTerminalReader(t, bed.root(), "ttys:fixture_lane_release")
	bed.facts.reader = &reader
	owners := bed.owners()
	owners.prove = unprovable
	code, result := bed.runJSON(owners, "goal", "release", bedGoal, "--reason", "the landing lane was unset")
	if code == 0 || result.Next == nil || !strings.HasSuffix(shellCommand(result.Next.Argv), "--by NAME") {
		t.Fatalf("the hint's release at an unenrolled terminal must guide to --by: %d %+v", code, result)
	}
	code, result = bed.runJSON(owners, "goal", "release", bedGoal, "--reason", "the landing lane was unset", "--by", "Wido")
	released := bed.goalFile(bedGoal)
	if code != 0 || result.Outcome != intentConfirmed || released.State == goal.StateClaimed {
		t.Fatalf("a person's release of the lane's claim = %d %+v; goal %+v", code, result, released.Claimed)
	}
}

// A lane registered again takes a new custody epoch; landing begin renews
// every claim the lane holds for the batch to it before it records the
// series (K7), through the real ledger, and a second renewal writes
// nothing.
func TestBeginRenewsTheLanesClaimsToItsCustodyEpoch(t *testing.T) {
	t.Parallel()
	bed := newLaneAuthorityBed(t)
	if _, err := bed.join(t); err != nil {
		t.Fatal(err)
	}
	registerLane(t, bed.home, filepath.Join(filepath.Dir(bed.lane), "interim-lane"), "Wido", laneAuthorityNow)
	layout, err := lane.NewLayout(bed.lane)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := lane.Register(bed.home, layout, "Wido", laneAuthorityNow); err != nil {
		t.Fatal(err)
	}
	record, _, err := lane.Read(bed.home)
	if err != nil || record.CustodyEpoch != 4 {
		t.Fatalf("re-registered lane = %+v %v; want custody epoch 4", record, err)
	}
	// The lane's goal acts run in its own installation, whose machine is
	// the lane's; this bed's one ledger stands in for it.
	goalSyncMutationGit(t, bed.seat, "config", "metasystem.goal.machine", "lane-host")
	tree := func() (string, error) {
		return strings.TrimSpace(goalSyncMutationGit(t, bed.seat, "rev-parse", goal.AcceptedRef+"^{tree}")), nil
	}
	if renewed, err := batchowner.RenewLaneClaims(bed.home, bed.lane, bed.seat, laneAuthorityBatch, tree, &batchowner.LaneCalls); err != nil || !slices.Equal(renewed, []string{"standing-validation"}) {
		t.Fatalf("renewal = %q %v", renewed, err)
	}
	file := bed.ledger(t)
	if file.StopCapability.ClaimEpoch != 4 || file.Claimed.Lineage != lane.ClaimLineage || file.Claimed.HandedOver.Batch != laneAuthorityBatch || file.Claimed.HandedOver.FromMachine != "mac-cli" {
		t.Fatalf("renewed claim = %+v %+v", file.Claimed, file.StopCapability)
	}
	before := goalSyncMutationGit(t, bed.seat, "rev-parse", goal.AcceptedRef)
	if renewed, err := batchowner.RenewLaneClaims(bed.home, bed.lane, bed.seat, laneAuthorityBatch, tree, &batchowner.LaneCalls); err != nil || len(renewed) != 0 {
		t.Fatalf("second renewal = %q %v", renewed, err)
	}
	if after := goalSyncMutationGit(t, bed.seat, "rev-parse", goal.AcceptedRef); after != before {
		t.Fatalf("a second renewal wrote the ledger")
	}
}
