package plain

import (
	"errors"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// An unreadable trunk-red policy holds a hand-in only while an incident is open.
func TestTrunkDecisionUnreadablePolicyHoldsOnlyWithOpenIncident(t *testing.T) {
	t.Parallel()
	broken := errors.New(`landing.trunk-red must be auto or person, not "broken"`)
	entry := Entry{Goal: "g"}
	if err := TrunkDecision(entry, nil, PolicyValue{}, broken, lane.Record{}); err != nil {
		t.Fatalf("no open incident, broken policy held the hand-in: %v", err)
	}
	open := []goal.TrunkRedEntry{{Identity: "red-1"}}
	if err := TrunkDecision(entry, open, PolicyValue{}, broken, lane.Record{}); err == nil || !strings.Contains(err.Error(), "cannot be read") {
		t.Fatalf("open incident with a broken policy did not hold: %v", err)
	}
}
