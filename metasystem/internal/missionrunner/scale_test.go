package missionrunner

import (
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
	// Compression stays OPT-IN: at scale 50 the suite exposed an
	// order-dependent hang (a compressed test leaks state that wedges
	// TestWallMechanicalRecoveryRestoresTheComposedTree even after env
	// restore) plus second-granular taint identity and compounding
	// nested-start windows — each named in its pinned test. The
	// conversion arc (timing-tests-synthetic-clock) owns making
	// compression the default; until then export the scale explicitly
	// to run the audit mode.
	os.Exit(m.Run())
}
