package steward

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestTrunkRedRoleVerdicts(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	t.Run("none is alive without cadence", func(t *testing.T) {
		bed := newRoleTrunkRedBed(t, now, nil, nil)
		role := bed.trunkRedWithoutBatch()
		if role.Status != HealthAlive || role.Reason != "no open trunk red" {
			t.Fatalf("no entries: %+v", role)
		}
	})
	t.Run("owned is alive with age", func(t *testing.T) {
		t.Parallel()
		bed := newRoleTrunkRedBed(t, now, []goal.TrunkRedEntry{healthTrunkRedEntry("owned", "bed-m1", now.Add(-2*time.Hour))}, healthCadence(now, "passed"))
		role := bed.trunkRedWithoutBatch()
		if role.Status != HealthAlive || role.Reason != "1 open, all owned; oldest 2h0m0s" || role.Remedy != "" {
			t.Fatalf("owned entry: %+v", role)
		}
	})
	t.Run("empty owner is dead", func(t *testing.T) {
		bed := newRoleTrunkRedBed(t, now, []goal.TrunkRedEntry{healthTrunkRedEntry("unowned", "", now.Add(-time.Hour))}, healthCadence(now, "passed"))
		role := bed.trunkRedWithoutBatch()
		if role.Status != HealthDead || !strings.Contains(role.Reason, "unowned") || !strings.Contains(role.Remedy, "metasystem incident claim unowned --goal") {
			t.Fatalf("unowned entry: %+v", role)
		}
	})
	t.Run("unowned flake and hang entries are tracked, not reds on main", func(t *testing.T) {
		pending := healthTrunkRedEntry("pending", "", now.Add(-time.Hour))
		pending.Class = goal.TrunkRedClassPendingFlake
		hang := healthTrunkRedEntry("hang", "", now.Add(-time.Hour))
		hang.Class = goal.TrunkRedClassHang
		bed := newRoleTrunkRedBed(t, now, []goal.TrunkRedEntry{hang, pending}, healthCadence(now, "passed"))
		role := bed.trunkRedWithoutBatch()
		if role.Status != HealthAlive || role.Reason != "no open trunk red; 2 flake or hang entries tracked separately (metasystem incident list)" {
			t.Fatalf("tracked defects: %+v", role)
		}
	})
	if !KnownHealthRole(RoleTrunkRed) {
		t.Fatal("trunk-red is absent from the published health role order")
	}
}

func TestTrunkRedIgnoresAnOverdueCadence(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name, groupStatus string
		window            time.Time
	}{
		{name: "missing", window: now},
		{name: "overdue", groupStatus: "passed", window: now.Add(-6*time.Hour - time.Minute)},
		{name: "non-green", groupStatus: "failed", window: now},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var status *goal.CadenceStatus
			if test.groupStatus != "" {
				status = healthCadence(test.window, test.groupStatus)
			}
			bed := newRoleTrunkRedBed(t, now, nil, status)
			role := bed.trunkRedWithoutBatch()
			if role.Status != HealthAlive || role.Reason != "no open trunk red" {
				t.Fatalf("cadence health=%+v", role)
			}
			bed = newRoleTrunkRedBed(t, now, []goal.TrunkRedEntry{healthTrunkRedEntry("owned", "bed-m1", now)}, status)
			if role := bed.trunkRedWithoutBatch(); role.Status != HealthAlive || role.Remedy != "" {
				t.Fatalf("an owned trunk red stays alive independently of cadence: %+v", role)
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
