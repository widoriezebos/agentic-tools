package steward

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

var situationNow = claimQuotaClock

// situationQuotaBed is the claim-quota ledger with this machine's claim on
// held-h, three ready priority-1 goals ranked c, a, b and one unranked goal,
// and a saved health record whose claim-quota role reads alive.
func situationQuotaBed(t *testing.T) string {
	t.Helper()
	ready := func(id string, sequence uint64) *goal.GoalFile {
		file := claimQuotaReady(id, 1, "m1e")
		file.Sequence = sequence
		return file
	}
	root := claimQuotaTickBed(t, nil, claimQuotaHeld("held-h"), ready("ready-c", 1), ready("ready-a", 2), ready("ready-b", 3),
		approvedStewardGoal("aaa-unranked", "Wait unranked", "Build it.", "2026-08-23T00:00:00Z"))
	if _, err := observePartialHealth(root, situationNow.Add(-time.Hour), []RoleVerdict{roleAlive(RoleClaimQuota, "saved by a tick")}); err != nil {
		t.Fatal(err)
	}
	return root
}

// situationLaneHome is a host home with the lane registered on a checkout.
func situationLaneHome(t *testing.T) string {
	t.Helper()
	home, checkout := t.TempDir(), t.TempDir()
	data, err := json.Marshal(lane.Record{Root: checkout, Install: filepath.Join(checkout, "metasystem"), CustodyEpoch: 1, RegisteredBy: "Wido"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(lane.HostDir(home), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lane.RecordPath(home), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return home
}

// countedProject is the production projection counted per call.
func countedProject(calls *int) func(goal.Endpoint, bool, time.Time) (goal.Projection, error) {
	return func(endpoint goal.Endpoint, fetch bool, now time.Time) (goal.Projection, error) {
		*calls++
		return goal.Project(endpoint, fetch, now)
	}
}

func TestTheSituationReadsTheLedgerAndTheQuotaLive(t *testing.T) {
	t.Parallel()
	root := situationQuotaBed(t)
	states := map[string]string{}
	projections := 0
	src := SituationSources{Now: func() time.Time { return situationNow }, LaneHome: func() (string, error) { return t.TempDir(), nil },
		Project: countedProject(&projections), ClaimLaneState: claimQuotaLane(t, root, states),
		Waiting: func(string) ([]string, error) { return []string{"a durable wait"}, nil }}
	release := ClaimQuotaBlock{Held: "held-h", Ready: []string{"ready-c", "ready-a", "ready-b"}, Command: "metasystem goal release held-h --reason TEXT"}

	s := ReadSituation(root, src)
	if !slices.Equal(s.Next.Claimed, []string{"held-h"}) || s.Ledger.Tip == "" || !s.At.Equal(situationNow) || s.Machine != "m1e" {
		t.Fatalf("the ledger part: claimed=%v tip=%q at=%v machine=%q unavailable=%v", s.Next.Claimed, s.Ledger.Tip, s.At, s.Machine, s.Unavailable)
	}
	if want := []string{"ready-c", "ready-a", "ready-b"}; len(s.Order) < 3 || !slices.Equal(s.Order[:3], want) || slices.Index(s.Order, "aaa-unranked") < 3 {
		t.Fatalf("order = %v, want %v before any unranked goal", s.Order, want)
	}
	if !reflect.DeepEqual(s.Quota, []ClaimQuotaBlock{release}) || s.Unavailable["quota"] != "" {
		t.Fatalf("quota = %+v (unavailable %q), want one block %+v", s.Quota, s.Unavailable["quota"], release)
	}
	if !slices.Equal(s.Waiting, []string{"a durable wait"}) || projections != 1 {
		t.Fatalf("waiting=%v projections=%d, want the wait and one projection", s.Waiting, projections)
	}

	states["held-h"] = "waiting"
	if s = ReadSituation(root, src); len(s.Quota) != 0 || projections != 2 {
		t.Fatalf("a waiting hand-in frees the claim: quota=%+v projections=%d", s.Quota, projections)
	}
	states["held-h"] = "returned"
	land := release
	land.Command = "metasystem work land held-h"
	if s = ReadSituation(root, src); !reflect.DeepEqual(s.Quota, []ClaimQuotaBlock{land}) || projections != 3 {
		t.Fatalf("a returned hand-in blocks again: quota=%+v projections=%d, want %+v", s.Quota, projections, land)
	}

	src.ClaimLaneState = nil
	if s = ReadSituation(root, src); s.Unavailable["quota"] != "no lane reader" || len(s.Quota) != 0 {
		t.Fatalf("no lane reader: quota=%+v unavailable=%q, want none and no saved verdict", s.Quota, s.Unavailable["quota"])
	}
}

func TestASituationPartFailsAlone(t *testing.T) {
	t.Parallel()
	root := situationQuotaBed(t)
	laneHome := situationLaneHome(t)
	src := func() SituationSources {
		return SituationSources{Now: func() time.Time { return situationNow }, LaneHome: func() (string, error) { return laneHome, nil },
			LaneStatus:     func(string, time.Time) plain.Status { return plain.Status{Problems: []string{"the lane read"}} },
			HostWork:       func() (int, int, error) { return 0, 0, nil },
			ClaimLaneState: claimQuotaLane(t, root, nil), Waiting: func(string) ([]string, error) { return []string{"a durable wait"}, nil }}
	}
	read := func(s Situation) (lane, ledger, quota, waiting bool) {
		return slices.Equal(s.Lane.Problems, []string{"the lane read"}), slices.Equal(s.Next.Claimed, []string{"held-h"}), len(s.Quota) == 1, len(s.Waiting) == 1
	}

	whole := src()
	if lane, ledger, quota, waiting := read(ReadSituation(root, whole)); !lane || !ledger || !quota || !waiting {
		t.Fatalf("the whole read: lane=%v ledger=%v quota=%v waiting=%v", lane, ledger, quota, waiting)
	}

	noLane := src()
	noLane.LaneHome = func() (string, error) { return "", errors.New("no host home") }
	s := ReadSituation(root, noLane)
	if lane, ledger, quota, waiting := read(s); lane || !ledger || !quota || !waiting || s.Unavailable["lane"] == "" {
		t.Fatalf("a failed lane home: lane=%v ledger=%v quota=%v waiting=%v unavailable=%v", lane, ledger, quota, waiting, s.Unavailable)
	}

	noMachine := src()
	noMachine.Machine = func(string) (string, error) { return "", errors.New("no nickname") }
	s = ReadSituation(root, noMachine)
	if lane, ledger, quota, _ := read(s); !lane || ledger || quota || s.Unavailable["ledger"] == "" || s.Unavailable["quota"] == "" || s.Unavailable["lane"] != "" {
		t.Fatalf("a failed machine read: lane=%v ledger=%v quota=%v unavailable=%v", lane, ledger, quota, s.Unavailable)
	}

	noWaiting := src()
	noWaiting.Waiting = func(string) ([]string, error) { return nil, errors.New("the waits cannot be read") }
	s = ReadSituation(root, noWaiting)
	if lane, ledger, quota, _ := read(s); !lane || !ledger || !quota || s.Unavailable["waiting"] != "the waits cannot be read" {
		t.Fatalf("a failed waiting read: lane=%v ledger=%v quota=%v unavailable=%v", lane, ledger, quota, s.Unavailable)
	}
}
