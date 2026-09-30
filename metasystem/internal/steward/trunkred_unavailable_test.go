package steward

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func unavailableHealthEntry(now time.Time) goal.TrunkRedEntry {
	entry := healthTrunkRedEntry("tr-section-deep-unavailable", "", now.Add(-time.Hour))
	entry.Status, entry.NotRunReason, entry.Holds = "unavailable", "standing cadence authority is unavailable", []string{}
	entry.Sightings[0].Attempt, entry.Sightings[0].Batch = goal.CadenceUnavailableAttempt, ""
	return entry
}

// A cadence that could not run is its own state: health says the check is
// unavailable with its cause and one command, never that main is red.
func TestTrunkRedHealthReportsAnUnavailableCadenceAsUnavailableNotRed(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 30, 16, 30, 0, 0, time.UTC)
	cadence := healthCadence(now.Add(-10*time.Minute), "unavailable")
	cadence.RunID, cadence.AttemptID = goal.CadenceUnavailableAttempt, goal.CadenceUnavailableAttempt
	bed := newRoleTrunkRedBed(t, now, []goal.TrunkRedEntry{unavailableHealthEntry(now)}, cadence)
	role := bed.trunkRedWithoutBatch()
	if role.Status != HealthUnknown || !strings.Contains(role.Reason, "could not run") || !strings.Contains(role.Reason, "goal standing-validation is not open") ||
		role.Remedy != "metasystem goal show standing-validation" || strings.Contains(role.Reason, "non-green") {
		t.Fatalf("unavailable cadence role=%+v", role)
	}
}

func TestTrunkRedHealthDoesNotCountAnUnavailableEntryAsAnOpenRed(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 30, 16, 30, 0, 0, time.UTC)
	bed := newRoleTrunkRedBed(t, now, []goal.TrunkRedEntry{unavailableHealthEntry(now)}, healthCadence(now.Add(-10*time.Minute), "passed"))
	role := bed.trunkRedWithoutBatch()
	if role.Status != HealthAlive || !strings.Contains(role.Reason, "no open trunk red") || strings.Contains(role.Reason, "without an owner") {
		t.Fatalf("unavailable entry role=%+v", role)
	}
}
