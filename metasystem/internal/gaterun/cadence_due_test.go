package gaterun

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// TestCadenceDueByClock (A-a): the keeper's wake reads whether validation is
// due from local state alone, by the cadence's own rules: no cadence status
// yet, a forced window that has run its interval, or the validation weight
// over its threshold. An identity change needs a fetch and a revalidation,
// which is validate's to judge, not the keeper's.
func TestCadenceDueByClock(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 9, 30, 6, 0, 0, 0, time.UTC)
	latest := &goal.CadenceStatus{ForcedWindowStart: start.Format(time.RFC3339)}
	for _, test := range []struct {
		name      string
		latest    *goal.CadenceStatus
		now       time.Time
		weightDue bool
		due       bool
	}{
		{"never validated", nil, start, false, true},
		{"inside the window", latest, start.Add(CadenceForcedInterval - time.Second), false, false},
		{"window ran out", latest, start.Add(CadenceForcedInterval), false, true},
		{"weight over threshold", latest, start.Add(time.Hour), true, true},
	} {
		due, err := CadenceDueByClock(test.now, test.latest, test.weightDue)
		if err != nil || due != test.due {
			t.Fatalf("%s: due=%t err=%v, want %t", test.name, due, err, test.due)
		}
	}
	if _, err := CadenceDueByClock(start, &goal.CadenceStatus{ForcedWindowStart: "not a time"}, false); err == nil {
		t.Fatal("an unreadable forced window read as a verdict")
	}
}
