package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
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
	bed.seams = batchowner.LandingLaneSeams{Home: func() (string, error) { return bed.home, nil }}
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

// work land asks lane.Resolve whether a lane is registered, and nothing
// more (simple lane, unit C): a lane a person is unsetting or has stopped
// is still the lane, so work land refuses beside it until the unset ends.
// The resolution is the production one over a real home.
func TestWorkLandAsksOnlyWhetherALaneIsRegistered(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	registerLane(t, bed.home, bed.landingA, "Wido", laneTestNow)
	seams := lane.UnsetSeams{
		Settle: func(lane.Layout) (lane.Settlement, error) {
			return lane.Settlement{Live: []string{"the landing agent runs"}}, nil
		},
	}
	if report, err := lane.Unset(bed.home, "Wido", laneTestNow, false, seams); err != nil || report.Stopped != lane.StepSettled {
		t.Fatalf("unset = %+v %v", report, err)
	}
	if _, paused := lane.ReadPause(bed.home); !paused {
		t.Fatal("the unset's fence did not stop the lane")
	}
	production := batchowner.ProductionLandingLaneSeams()
	production.Home = func() (string, error) { return bed.home, nil }
	inv := &intentInvocation{layout: stateroot.Layout{InstallationRoot: stateroottest.Installation(t, bed.seatA)}, owners: intentOwners{delivery: &intentDeliveryOwners{
		laneRoot: production.BatchRoot, now: func() time.Time { return laneTestNow }}}}
	refused := inv.laneRegistered(nil)
	if refused == nil || refused.Summary != "this computer has a landing lane, which lands only a goal's branch; nothing was landed" {
		t.Fatalf("work land on a registered lane = %+v; want the lane's refusal and no other question asked", refused)
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
	inv := &intentInvocation{layout: stateroot.Layout{InstallationRoot: stateroottest.Installation(t, bed.seatB)}, owners: intentOwners{delivery: &intentDeliveryOwners{
		laneRoot: bed.seams.BatchRoot, now: func() time.Time { return laneTestNow }}}}
	refused := inv.laneRegistered(nil)
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

// With no home for the lane, a seat keeps its own setting, as before U12:
// there is no host state, so nothing is registered, gated or kept.
func TestBatchRootWithoutALaneHomeKeepsTheSeatSetting(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	if err := os.WriteFile(filepath.Join(bed.landingA, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	bed.setRoot(t, bed.seatA, bed.landingA)
	noHome := func() (string, error) { return "", errors.New("no home") }
	seams := batchowner.LandingLaneSeams{Home: noHome}
	root, configured, err := seams.BatchRoot(bed.seatA, laneTestNow)
	if err != nil || !configured || realpath.Resolve(root) != bed.landingA {
		t.Fatalf("seat without a lane home = %q %v %v; want its own setting", root, configured, err)
	}
}

// Only a person's landing set registers a lane (design r10 §1): a seat whose
// landing.batch-root names a checkout, on a computer with no registered
// lane, writes no host record and lands its own work.
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
	inv := &intentInvocation{layout: stateroot.Layout{InstallationRoot: stateroottest.Installation(t, bed.seatA)}, owners: intentOwners{delivery: &intentDeliveryOwners{
		laneRoot: bed.seams.BatchRoot, now: func() time.Time { return laneTestNow }}}}
	refused := inv.laneRegistered(nil)
	if refused == nil || strings.Contains(refused.Summary, lane.CodeRecordIncomplete) || !strings.Contains(refused.Summary, "older engine") ||
		strings.Join(refused.next, " ") != "metasystem landing set "+bed.landingA || !strings.Contains(strings.Join(refused.Details, " "), lane.CodeRecordIncomplete) {
		t.Fatalf("work land on an older lane record = %+v", refused)
	}
}
