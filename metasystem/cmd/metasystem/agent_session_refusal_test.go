package main

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// An agent session that forgot to name itself is told to name its session.
// The walk to the enrolled terminal can stop at the first shell before it
// reaches the agent (a headless session has no terminal), and "enroll this
// terminal" would send the agent to a person's act it cannot perform.
func TestAgentSessionWithoutLineageIsToldToNameItsSession(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, makeQueued)
	owners := bed.owners()
	owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, humanauthority.Refused(humanauthority.OutcomeTerminalMissing, nil)
	}
	owners.agent.caller = func(*intentInvocation, string) string { return lease.ClassMain }
	code, _, stderr := bed.run(owners, "goal", "claim", bedGoal)
	lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	if code == 0 || len(lines) != 2 ||
		lines[0] != "✗ an agent session ran this without naming its session, so nothing was done" ||
		lines[1] != "  → metasystem goal claim "+bedGoal+" --lineage LINEAGE  as the session that holds the work" {
		t.Fatalf("goal claim = %d\n%s", code, stderr)
	}
	// A person's shell keeps the person's remedy.
	owners.agent.caller = func(*intentInvocation, string) string { return lease.ClassHuman }
	if code, _, stderr = bed.run(owners, "goal", "claim", bedGoal); code == 0 || strings.Contains(stderr, "--lineage LINEAGE  as the session") {
		t.Fatalf("goal claim from a person's shell = %d\n%s", code, stderr)
	}
}
