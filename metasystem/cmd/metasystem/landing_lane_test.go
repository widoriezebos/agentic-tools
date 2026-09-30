package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

var laneTestNow = time.Date(2026, 9, 29, 18, 0, 0, 0, time.UTC)

// laneBed is a host with a lane home, two nested landing checkouts and two
// seats whose settings the test writes.
type laneBed struct {
	home, landingA, landingB, seatA, seatB string
	seams                                  batchowner.LandingLaneSeams
}

func newLaneBed(t *testing.T) *laneBed {
	t.Helper()
	base := t.TempDir()
	bed := &laneBed{home: filepath.Join(base, "home"), landingA: filepath.Join(base, "landing-a"), landingB: filepath.Join(base, "landing-b"),
		seatA: filepath.Join(base, "seat-a"), seatB: filepath.Join(base, "seat-b")}
	for _, dir := range []string{bed.home, bed.landingA, bed.landingB, bed.seatA, bed.seatB} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, seat := range []string{bed.seatA, bed.seatB} {
		if err := os.WriteFile(filepath.Join(seat, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bed.home, bed.landingA, bed.landingB = realpath.Resolve(bed.home), realpath.Resolve(bed.landingA), realpath.Resolve(bed.landingB)
	landingCheckout(t, bed.landingA)
	landingCheckout(t, bed.landingB)
	bed.seams = batchowner.LandingLaneSeams{Home: func() (string, error) { return bed.home, nil },
		Validate: func(root, _ string, _ time.Time) (string, error) { return realpath.Resolve(root), nil }}
	return bed
}

func (bed *laneBed) setRoot(t *testing.T, seat, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(seat, "metasystem.conf.local"), []byte("landing.batch-root="+root+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A person registers the lane; a seat naming it and a seat naming nothing
// both land through it.
func TestBatchRootServesThePersonsLane(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	registerLane(t, bed.home, bed.landingA, "Wido", laneTestNow)
	bed.setRoot(t, bed.seatA, bed.landingA)
	for _, seat := range []string{bed.seatA, bed.seatB} {
		root, configured, err := bed.seams.BatchRoot(seat, laneTestNow)
		if err != nil || !configured || root != bed.landingA {
			t.Fatalf("seat %s = %q %v %v; want the person's lane %s", seat, root, configured, err, bed.landingA)
		}
	}
	record, ok, err := lane.Read(bed.home)
	if err != nil || !ok || record.Root != bed.landingA || record.RegisteredBy != "Wido" || record.Install != filepath.Join(bed.landingA, "metasystem") {
		t.Fatalf("record = %+v %v %v", record, ok, err)
	}
}

// While a person unsets the lane, a seat's join is refused in plain words
// naming the one command that finishes the unset; a reader still sees the
// lane.
func TestJoinIsRefusedWhileTheLaneIsUnset(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	registerLane(t, bed.home, bed.landingA, "Wido", laneTestNow)
	seams := lane.UnsetSeams{
		Settle: func(lane.Layout) (lane.Settlement, error) {
			return lane.Settlement{Live: []string{"a proof runs"}}, nil
		},
		Records:   func(lane.Layout) ([]batch.Record, error) { return nil, nil },
		Reconcile: func(lane.Layout, batch.Record) ([]lane.Unresolved, error) { return nil, nil },
		Return:    func(lane.Layout, batch.Record, string) ([]lane.Unresolved, error) { return nil, nil },
		Confirm:   func(lane.Layout, []batch.Record) ([]lane.Unresolved, error) { return nil, nil },
	}
	if report, err := lane.Unset(bed.home, "Wido", laneTestNow, false, seams); err != nil || report.Stopped != lane.StepSettled {
		t.Fatalf("unset = %+v %v", report, err)
	}
	inv := &intentInvocation{layout: stateroot.Layout{InstallationRoot: bed.seatA}, owners: intentOwners{delivery: &intentDeliveryOwners{
		batchRoot: bed.seams.BatchRoot, now: func() time.Time { return laneTestNow }}}}
	_, _, refused := inv.landingBatchRoot(nil)
	if refused == nil || refused.Outcome != intentRefused || strings.Contains(refused.Summary, lane.CodeUnsetting) || !strings.Contains(refused.Summary, "being unset") ||
		strings.Join(refused.next, " ") != "metasystem landing unset" || !strings.Contains(strings.Join(refused.Details, " "), lane.CodeUnsetting) {
		t.Fatalf("a join while the lane is unset = %+v; want the situation on line 1, metasystem landing unset on line 2, %s only in the details", refused, lane.CodeUnsetting)
	}
	if root, configured, err := bed.seams.Resolve(bed.seatA, laneTestNow); err != nil || !configured || root != bed.landingA {
		t.Fatalf("a reader during the unset = %q %v %v; want the lane", root, configured, err)
	}
}

// Neither the seat nor the host names a lane: today's answer, not configured.
func TestBatchRootWithoutAnyLaneIsNotConfigured(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	if root, configured, err := bed.seams.BatchRoot(bed.seatA, laneTestNow); err != nil || configured || root != "" {
		t.Fatalf("no lane = %q %v %v", root, configured, err)
	}
	if _, ok, _ := lane.Read(bed.home); ok {
		t.Fatalf("a seat without a setting registered a lane")
	}
}

// A seat naming another checkout is refused by work land in plain words,
// naming both paths and the one fix, and nothing is joined.
func TestWorkLandRefusesASeatWhoseRootIsNotTheHostLane(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	registerLane(t, bed.home, bed.landingA, "seat-a", laneTestNow)
	bed.setRoot(t, bed.seatB, bed.landingB)
	inv := &intentInvocation{layout: stateroot.Layout{InstallationRoot: bed.seatB}, owners: intentOwners{delivery: &intentDeliveryOwners{
		batchRoot: bed.seams.BatchRoot, now: func() time.Time { return laneTestNow }}}}
	_, _, refused := inv.landingBatchRoot(nil)
	if refused == nil || refused.Outcome != intentRefused {
		t.Fatalf("seat B landed through another lane: %+v", refused)
	}
	for _, want := range []string{bed.landingA, bed.landingB, "registered by seat-a"} {
		if !strings.Contains(refused.Summary, want) {
			t.Errorf("summary %q lacks %q", refused.Summary, want)
		}
	}
	if strings.Contains(refused.Summary, lane.CodeMismatch) || !strings.Contains(strings.Join(refused.Details, " "), lane.CodeMismatch) {
		t.Errorf("the code %s belongs in the details, not line 1: %+v", lane.CodeMismatch, refused)
	}
	for _, want := range []string{"metasystem settings set landing.batch-root " + bed.landingA, "metasystem landing set " + bed.landingB} {
		if !strings.Contains(refused.Decision, want) {
			t.Errorf("decision %q lacks %q", refused.Decision, want)
		}
	}
}

// The owner serves only the host's lane: the checkout a person registered
// serves it; another checkout naming itself is refused and does not run; a
// checkout with no setting serves the lane when it is the lane.
func TestLandingOwnerServesOnlyTheHostLane(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	home := bed.seams.Home
	registerLane(t, bed.home, bed.landingA, "Wido", laneTestNow)
	for _, landing := range []string{bed.landingA, bed.landingB} {
		if err := os.WriteFile(filepath.Join(landing, "metasystem.conf"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bed.setRoot(t, bed.landingA, bed.landingA)
	root, paused, err := batchowner.LandingOwnerLaneRoot(home, bed.landingA, bed.landingA, laneTestNow)
	if err != nil || root != bed.landingA || paused {
		t.Fatalf("lane A's own owner = %q %v %v", root, paused, err)
	}
	bed.setRoot(t, bed.landingB, bed.landingB)
	_, _, err = batchowner.LandingOwnerLaneRoot(home, bed.landingB, bed.landingB, laneTestNow)
	var refusal *lane.Refusal
	if !errors.As(err, &refusal) || refusal.Code != lane.CodeMismatch {
		t.Fatalf("lane B's owner = %v; want %s so a second owner never runs", err, lane.CodeMismatch)
	}
	if err := os.Remove(filepath.Join(bed.landingB, "metasystem.conf.local")); err != nil {
		t.Fatal(err)
	}
	if root, _, err = batchowner.LandingOwnerLaneRoot(home, bed.landingB, bed.landingB, laneTestNow); err != nil || root != bed.landingA {
		t.Fatalf("unset owner B = %q %v; want the host's lane A, which is not B, so B does not run", root, err)
	}
	if _, err := lane.SetPause(bed.home, "Wido", laneTestNow); err != nil {
		t.Fatal(err)
	}
	if _, paused, _ = batchowner.LandingOwnerLaneRoot(home, bed.landingA, bed.landingA, laneTestNow); !paused {
		t.Fatalf("a person's pause is not seen by the owner")
	}
}

// With no home for the lane, a seat keeps its own setting, as before U12:
// there is no host state, so nothing is registered, gated or kept.
func TestBatchRootWithoutALaneHomeKeepsTheSeatSetting(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	if err := os.WriteFile(filepath.Join(bed.landingA, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	bed.setRoot(t, bed.landingA, bed.landingA)
	noHome := func() (string, error) { return "", errors.New("no home") }
	root, paused, err := batchowner.LandingOwnerLaneRoot(noHome, bed.landingA, bed.landingA, laneTestNow)
	if err != nil || paused || realpath.Resolve(root) != bed.landingA {
		t.Fatalf("owner without a lane home = %q %v %v", root, paused, err)
	}
	if batchowner.LandingLaneProving(noHome) != nil || batchowner.LandingLaneKeeper(noHome) != nil {
		t.Fatalf("a host without a lane home gates proofs or keeps an owner")
	}
}

// helm's report reads the lane through the lane resolver: a seat with no
// landing.batch-root of its own sees the batches of the host's lane that
// carry its work.
func TestHelmHeldBatchesReadsTheHostLaneForAnUnsetSeat(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	if err := os.MkdirAll(filepath.Join(bed.seatB, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	registerLane(t, bed.home, bed.landingA, "Wido", laneTestNow)
	batches := filepath.Join(bed.landingA, "artifacts", "agents", "landing-batches")
	if err := os.MkdirAll(batches, 0o755); err != nil {
		t.Fatal(err)
	}
	record := `{"batchId":"b1","state":"proving","units":[{"seatRoot":"` + bed.seatB + `"}]}`
	if err := os.WriteFile(filepath.Join(batches, "b1.json"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	seat, err := helm.Locate(bed.seatB)
	if err != nil {
		t.Fatal(err)
	}
	held, err := helmHeldBatches(bed.seatB, seat, func(installation string, now time.Time) (string, bool, error) {
		return bed.seams.Resolve(installation, now)
	})
	if err != nil || len(held) != 1 || held[0] != "b1 (proving)" {
		t.Fatalf("unset seat's held batches = %v %v; want b1 from the host lane", held, err)
	}
}

// The one launcher of a batch's proof and diagnostic children asks each
// child to hold the host's proving flock for its life; only a spare launch
// (the early proof) goes without it.
func TestBatchProofLauncherAsksTheChildToHoldTheProvingLock(t *testing.T) {
	t.Parallel()
	args := []string{"internal", "test", "run", "--root", "/r"}
	held := batchowner.BatchProofCommand("/bin/metasystem", args, false)
	if !slices.Contains(held.Args, batchowner.HoldHostProvingFlag) {
		t.Fatalf("a proof child launched without the proving lock: %v", held.Args)
	}
	spare := batchowner.BatchProofCommand("/bin/metasystem", args, true)
	if slices.Contains(spare.Args, batchowner.HoldHostProvingFlag) {
		t.Fatalf("a spare launch holds the proving lock: %v", spare.Args)
	}
	if len(args) != 5 {
		t.Fatalf("the launcher changed its caller's arguments: %v", args)
	}
}

// internal test run with the flag holds the host's proving flock until it
// ends, waiting while another proof holds it; the flag never reaches the run.
func TestTestRunHoldsTheProvingLockForItsLife(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	homeOf := func() (string, error) { return home, nil }
	rest, release, err := batchowner.HoldHostProvingFor(homeOf, []string{"internal", "test", "run", batchowner.HoldHostProvingFlag, "--root", "/r"})
	if err != nil || slices.Contains(rest, batchowner.HoldHostProvingFlag) || len(rest) != 5 {
		t.Fatalf("hold = %v %v", rest, err)
	}
	if holder, busy, _ := lane.ProbeProving(home); !busy || holder != fmt.Sprintf("pid %d", os.Getpid()) {
		t.Fatalf("while the run lives: busy=%v holder=%q", busy, holder)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	if _, busy, _ := lane.ProbeProving(home); busy {
		t.Fatalf("the lock outlived the run")
	}
	rest, release, err = batchowner.HoldHostProvingFor(homeOf, []string{"internal", "test", "run"})
	if err != nil || len(rest) != 3 || release() != nil {
		t.Fatalf("without the flag = %v %v", rest, err)
	}
	if _, busy, _ := lane.ProbeProving(home); busy {
		t.Fatalf("a run without the flag took the lock")
	}
}

// F-4: starting the owner of a landing checkout that is gone is refused
// before anything is created or launched (ensureBatchOwner's first check;
// the test never reaches a launch).
func TestEnsureBatchOwnerRefusesAGoneLane(t *testing.T) {
	t.Parallel()
	gone := filepath.Join(t.TempDir(), "gone-landing")
	err := batchowner.LandingCheckoutPresent(gone)
	var refusal *lane.Refusal
	if !errors.As(err, &refusal) || refusal.Code != lane.CodeGone || !strings.Contains(refusal.Fix, "metasystem landing set PATH") {
		t.Fatalf("ensure a gone lane = %v", err)
	}
	if _, statErr := os.Stat(gone); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("the gone lane was recreated: %v", statErr)
	}
}

// Only a person's landing set registers a lane (design r10 §1): a seat whose
// landing.batch-root names a checkout, on a computer with no registered
// lane, writes no host record and lands its own work; the lane's would-be
// owner there does not run either.
func TestSeatSettingNeverRegisters(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	bed.setRoot(t, bed.seatA, bed.landingA)
	root, configured, err := bed.seams.BatchRoot(bed.seatA, laneTestNow)
	if err != nil || configured || root != "" {
		t.Fatalf("seat naming a lane on a computer with none = %q %v %v; want no lane, so the seat lands itself", root, configured, err)
	}
	if _, ok, _ := lane.Read(bed.home); ok {
		t.Fatalf("a seat's landing.batch-root registered the host's lane")
	}
	if err := os.WriteFile(filepath.Join(bed.landingA, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	bed.setRoot(t, bed.landingA, bed.landingA)
	root, _, err = batchowner.LandingOwnerLaneRoot(bed.seams.Home, bed.landingA, bed.landingA, laneTestNow)
	if err != nil || root != "" {
		t.Fatalf("a landing checkout naming itself = %q %v; want no lane: it registers nothing and its owner serves none", root, err)
	}
	if _, ok, _ := lane.Read(bed.home); ok {
		t.Fatalf("the owner of a checkout naming itself registered the host's lane")
	}
}

// At cutover every seat meets an older engine's lane record: work land
// says so in plain words on line 1, names the one command on line 2, and
// keeps the code for --verbose and --json.
func TestWorkLandNamesLandingSetForAnOlderLaneRecord(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	if err := os.MkdirAll(lane.HostDir(bed.home), 0o700); err != nil {
		t.Fatal(err)
	}
	old := `{"root":"` + bed.landingA + `","registeredBy":"m1e","at":"2026-09-29T18:00:00Z"}`
	if err := os.WriteFile(lane.RecordPath(bed.home), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	inv := &intentInvocation{layout: stateroot.Layout{InstallationRoot: bed.seatA}, owners: intentOwners{delivery: &intentDeliveryOwners{
		batchRoot: bed.seams.BatchRoot, now: func() time.Time { return laneTestNow }}}}
	_, _, refused := inv.landingBatchRoot(nil)
	if refused == nil || strings.Contains(refused.Summary, lane.CodeRecordIncomplete) || !strings.Contains(refused.Summary, "older engine") ||
		strings.Join(refused.next, " ") != "metasystem landing set "+bed.landingA || !strings.Contains(strings.Join(refused.Details, " "), lane.CodeRecordIncomplete) {
		t.Fatalf("work land on an older lane record = %+v", refused)
	}
}
