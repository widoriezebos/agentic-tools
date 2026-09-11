package gaterun

import (
	"os"
	"path/filepath"
	"testing"
)

// A landing's weight is scaled by the owning goal's highest risk answer;
// the goal's answers never choose per-landing depth, they bring the deep
// cadence run sooner.
func TestWeightAddScalesByGoalRisk(t *testing.T) {
	numstat := []byte("40\t2\tmetasystem/internal/gaterun/weight.go\n")
	unscaled := t.TempDir()
	scaled := t.TempDir()
	for _, root := range []string{unscaled, scaled} {
		if err := os.MkdirAll(filepath.Join(root, "artifacts", "agents"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	base, _, err := WeightAdd(unscaled, "abc1234", numstat, "metasystem", 0)
	if err != nil {
		t.Fatal(err)
	}
	if base.Accumulated <= 0 {
		t.Fatalf("an engine source landing weighs nothing: %+v", base)
	}
	tripled, _, err := WeightAddScaled(scaled, "abc1234", numstat, "metasystem", 0, 3)
	if err != nil {
		t.Fatal(err)
	}
	if tripled.Accumulated != 3*base.Accumulated || tripled.Landings != 1 {
		t.Fatalf("scale 3 did not triple the landing's weight: base=%d scaled=%d landings=%d", base.Accumulated, tripled.Accumulated, tripled.Landings)
	}
	below, _, err := WeightAddScaled(t.TempDir(), "abc1234", numstat, "metasystem", 0, 0)
	if err == nil && below.Accumulated != base.Accumulated {
		t.Fatalf("a scale below 1 did not count as 1: %d vs %d", below.Accumulated, base.Accumulated)
	}
}
