package missionrunner

import (
	"fmt"
	"os"
	"testing"
)

// TestMain compresses the fixture cap scale for the whole package: the
// engine's grace, verify, gate, and backoff ceilings bound real facts
// with early exit, so the suite pays them only on negative paths — at
// scale 50 a five-second grace is 250ms and the 289-test suite fits the
// gauntlet's fast-test law instead of costing twenty-four minutes. An
// explicitly exported scale (CI, a triage session) is respected.
func TestMain(m *testing.M) {
	// Compression is SAFE but not DEFAULT — measured 2026-08-30 after
	// every wedge closed (the pins are gone; scale 50 runs the package
	// green repeatedly): compressed 897s vs default 890s. The waits no
	// longer dominate — real subprocess and git work does — so the flip
	// buys nothing and costs timing fidelity. Export a scale to compress
	// for triage; the numbers above are the reason the default stands.
	if err := prepareMissionBedTemplates(); err != nil {
		fmt.Fprintf(os.Stderr, "prepare mission-bed templates: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	cleanMissionBedTemplates()
	os.Exit(code)
}
