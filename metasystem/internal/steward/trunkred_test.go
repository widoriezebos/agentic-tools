package steward

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestTrunkRedRoleVerdicts(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	t.Run("none is not green", func(t *testing.T) {
		bed := newRoleTrunkRedBed(t, now, nil, nil)
		role := bed.trunkRedWithoutBatch()
		if role.Status != HealthDead || !strings.Contains(role.Reason, "no deep validation cadence") || !strings.Contains(role.Remedy, "gate cadence-tick") {
			t.Fatalf("no entries: %+v", role)
		}
	})
	t.Run("owned is alive with age", func(t *testing.T) {
		bed := newRoleTrunkRedBed(t, now, []goal.TrunkRedEntry{healthTrunkRedEntry("owned", "bed-m1", now.Add(-2*time.Hour))}, healthCadence(now, "passed"))
		role := bed.trunkRedWithoutBatch()
		if role.Status != HealthAlive || role.Reason != "1 open, all owned; oldest 2h0m0s" {
			t.Fatalf("owned entry: %+v", role)
		}
	})
	t.Run("empty owner is dead", func(t *testing.T) {
		bed := newRoleTrunkRedBed(t, now, []goal.TrunkRedEntry{healthTrunkRedEntry("unowned", "", now.Add(-time.Hour))}, healthCadence(now, "passed"))
		role := bed.trunkRedWithoutBatch()
		if role.Status != HealthDead || !strings.Contains(role.Reason, "unowned") || !strings.Contains(role.Remedy, "goal trunk-red own --id unowned") {
			t.Fatalf("unowned entry: %+v", role)
		}
	})
	t.Run("stale empty batch hold is dead but a fresh hold is alive", func(t *testing.T) {
		bed := newRoleTrunkRedBed(t, now, nil, healthCadence(now, "passed"))
		landing := t.TempDir()
		if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte("landing.batch-root="+landing+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		resolverCalls := 0
		resolve := func(confPath, seatRoot string, clock func() time.Time) (config.BatchLanding, error) {
			resolverCalls++
			if confPath != filepath.Join(bed.root, "metasystem.conf") || seatRoot != bed.root || !clock().Equal(now) {
				t.Fatalf("batch resolver arguments: conf=%q root=%q now=%s", confPath, seatRoot, clock())
			}
			return config.NewBatchLanding(landing, time.Minute, clock)
		}
		writeHealthBatch(t, landing, now.Add(-2*time.Minute), "held-opid")
		role := bed.trunkRed(resolve)
		if role.Status != HealthDead || !strings.Contains(role.Reason, "batch batch-held opid") || !strings.Contains(role.Remedy, "landing batch tick") {
			t.Fatalf("stale hold: %+v", role)
		}
		writeHealthBatch(t, landing, now.Add(-2*time.Minute), "second-opid")
		role = bed.trunkRed(resolve)
		if role.Status != HealthDead || !strings.Contains(role.Reason, "batch batch-held opid second-opid") {
			t.Fatalf("stale rotated hold: %+v", role)
		}
		writeHealthBatch(t, landing, now.Add(-30*time.Second), "held-opid")
		role = bed.trunkRed(resolve)
		if role.Status != HealthAlive || role.Reason != "no open trunk red" {
			t.Fatalf("fresh hold: %+v", role)
		}
		writeHealthBatch(t, landing, now.Add(-30*time.Second), "second-opid", now.Add(-2*time.Minute))
		role = bed.trunkRed(resolve)
		if role.Status != HealthAlive || role.Reason != "no open trunk red" {
			t.Fatalf("fresh re-hold after stale hold: %+v", role)
		}
		if resolverCalls != 4 {
			t.Fatalf("batch resolver calls = %d, want 4", resolverCalls)
		}
	})
	t.Run("unreadable configured batch root is unknown", func(t *testing.T) {
		bed := newRoleTrunkRedBed(t, now, nil, healthCadence(now, "passed"))
		if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte("landing.batch-root="+filepath.Join(t.TempDir(), "missing")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		resolverCalls := 0
		role := bed.trunkRed(func(confPath, seatRoot string, clock func() time.Time) (config.BatchLanding, error) {
			resolverCalls++
			if confPath != filepath.Join(bed.root, "metasystem.conf") || seatRoot != bed.root || !clock().Equal(now) {
				t.Fatalf("batch resolver arguments: conf=%q root=%q now=%s", confPath, seatRoot, clock())
			}
			return config.BatchLanding{}, errors.New("declared missing batch checkout")
		})
		if resolverCalls != 1 {
			t.Fatalf("batch resolver calls = %d, want 1", resolverCalls)
		}
		if role.Status != HealthUnknown || !strings.Contains(role.Reason, "batch root") {
			t.Fatalf("unreadable root: %+v", role)
		}
	})
	if !KnownHealthRole(RoleTrunkRed) {
		t.Fatal("trunk-red is absent from the published health role order")
	}
}

func TestTrunkRedHealthReportsOverdueAndNonGreenCadence(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name, groupStatus, reason string
		window                    time.Time
	}{
		{name: "overdue", groupStatus: "passed", reason: "overdue", window: now.Add(-6*time.Hour - time.Minute)},
		{name: "non-green", groupStatus: "failed", reason: "non-green", window: now},
	} {
		t.Run(test.name, func(t *testing.T) {
			status := healthCadence(test.window, test.groupStatus)
			bed := newRoleTrunkRedBed(t, now, nil, status)
			role := bed.trunkRedWithoutBatch()
			if role.Status != HealthDead || !strings.Contains(role.Reason, test.reason) || !strings.Contains(role.Reason, status.TrunkCommit) || !strings.Contains(role.Remedy, "gate cadence-tick") {
				t.Fatalf("cadence health=%+v", role)
			}
		})
	}
}

func healthCadence(window time.Time, groupStatus string) *goal.CadenceStatus {
	return &goal.CadenceStatus{TrunkCommit: strings.Repeat("a", 40), TrunkTree: strings.Repeat("b", 40), Trigger: goal.CadenceTriggerForcedWindow,
		RunID: "run-health", AttemptID: "attempt-health", StartedAt: window.UTC().Format(time.RFC3339), EndedAt: window.UTC().Format(time.RFC3339),
		ForcedWindowStart: window.UTC().Format(time.RFC3339), Opid: goal.Opid("01J5X00000000000000000CR01", "bed-m1", "cadence"),
		Groups: []goal.CadenceGroupStatus{{Group: "section/deep", ExecutionIdentity: strings.Repeat("c", 64), Status: groupStatus, EvidenceDigest: strings.Repeat("d", 64)}}}
}

func healthTrunkRedEntry(id, machine string, opened time.Time) goal.TrunkRedEntry {
	stamp := opened.UTC().Format(time.RFC3339)
	owner := goal.TrunkRedOwner{}
	if machine != "" {
		owner = goal.TrunkRedOwner{Machine: machine, Since: stamp, How: "joiner"}
	}
	return goal.TrunkRedEntry{ID: id, Identity: id, Group: "fast", Status: "failed", Failures: []goal.TrunkRedFailure{},
		Sightings: []goal.TrunkRedSighting{{Attempt: "attempt-1", Batch: "batch-1", BaseCommit: "base-1", SeenAt: stamp,
			Opid: goal.Opid("01J5X0000000000000000000W1", "bed-m1", "lineage")}}, Owner: owner, Holds: []string{"batch-1"}, Opened: stamp}
}

func writeHealthBatch(t *testing.T, root string, heldAt time.Time, currentOpid string, previousHoldAt ...time.Time) {
	t.Helper()
	directory := filepath.Join(root, "artifacts", "agents", "landing-batches")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	history := []map[string]any{}
	opids := []string{"held-opid"}
	if len(previousHoldAt) != 0 {
		history = append(history,
			map[string]any{"at": previousHoldAt[0].UTC().Format(time.RFC3339Nano), "verb": "trunk-red-hold", "detail": "attempt=a groups=fast opid=held-opid"},
			map[string]any{"at": previousHoldAt[0].Add(time.Second).UTC().Format(time.RFC3339Nano), "verb": "trunk-red-recorded", "detail": "entries=entry-1"})
		if currentOpid != "held-opid" {
			opids = append(opids, currentOpid)
		}
		history = append(history, map[string]any{"at": heldAt.UTC().Format(time.RFC3339Nano), "verb": "trunk-red-hold", "detail": "attempt=b groups=fast opid=" + currentOpid})
	} else {
		history = append(history, map[string]any{"at": heldAt.UTC().Format(time.RFC3339Nano), "verb": "trunk-red-hold", "detail": "attempt=a groups=fast opid=held-opid"})
	}
	if currentOpid != "held-opid" && len(previousHoldAt) == 0 {
		history = append(history, map[string]any{"at": heldAt.Add(time.Second).UTC().Format(time.RFC3339Nano), "verb": "trunk-red-record-failed", "detail": "opid=held-opid outcome=aborted new-opid=" + currentOpid})
		opids = append(opids, currentOpid)
	}
	record := map[string]any{
		"schema": 1, "batchId": "batch-held", "state": "held-trunk-red",
		"trunkRed": map[string]any{"opid": currentOpid, "opids": opids, "entries": []any{}},
		"history":  history,
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "batch-held.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
