package steward

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

// U6b follow-up 3: while the resident runner's standing is not its own (its
// arming process is still its parent), the breach-stop pass is deferred: it
// scans nothing, runs no stop and writes no FAILED report. Readiness is a
// seam, so the transient is driven without a clock.
func TestDeferredBreachStopPassScansNothingAndReportsNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	scans, stops := 0, 0
	scanner := func(string, time.Time) ([]dispatch.StopRoute, error) {
		scans++
		return []dispatch.StopRoute{{GoalID: "bounded-goal", Revision: 2}}, nil
	}
	// The stop the arming ancestor's classification would meet: refused.
	stop := func(string, uint64) (string, error) {
		stops++
		return "breach-stop requires the authenticated lease holder or enrolled steward custodian", errors.New("exit status 1")
	}
	ready := false
	cfg := TickConfig{Now: now, BreachStop: stop, BreachStopReady: func() bool { return ready }}
	if reports := custodialBreachStops(root, cfg, scanner); len(reports) != 0 || scans != 0 || stops != 0 {
		t.Fatalf("a deferred pass acted: reports=%+v scans=%d stops=%d", reports, scans, stops)
	}
	ready = true
	reports := custodialBreachStops(root, cfg, scanner)
	if len(reports) != 1 || reports[0].State != "FAILED" || scans != 1 || stops != 1 {
		t.Fatalf("a ready pass did not run the stop exactly once: reports=%+v scans=%d stops=%d", reports, scans, stops)
	}
	// nil readiness is the external tick: always ready.
	cfg.BreachStopReady = nil
	if reports := custodialBreachStops(root, cfg, scanner); len(reports) != 1 || scans != 2 {
		t.Fatalf("the external tick's pass was deferred: reports=%+v scans=%d", reports, scans)
	}
}

// U6b follow-up 2: a breach is healed by the armed steward's own tick, so no
// steward remedy may tell a person that a manual tick completes it; the
// remedy names the public actions a person has meanwhile.
func TestBreachRemediesNameThePublicActionsNotAManualTick(t *testing.T) {
	t.Parallel()
	remedy := remedyFor(RoleClaimedGoalBudget, RemedyFact{Cause: CauseBreachStopOpen, Goal: "bounded-goal", Stop: "stop"}).Plain
	for _, want := range []string{"goal bounded-goal's budget stop stop completes by itself on the steward's next pass", "nothing needs doing"} {
		if !strings.Contains(remedy, want) {
			t.Fatalf("breach remedy %q does not name %q", remedy, want)
		}
	}
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "steward tick --repo") {
			t.Fatalf("%s still names a manual steward tick as a remedy", source)
		}
	}
}
