package pattern

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

type quietCensus struct{}

func (quietCensus) Workers(string) (steward.Workers, error) { return steward.Workers{}, nil }

// Under the helm the steward's tick decides nothing (HM-7), but the pattern
// pass still reports (D2): the real pass, called by the real helm tick, sees
// the landing seat at the helm and marks the open episode held.
func TestHelmTickReports(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	id := batchID("n")
	b.writeBatch(id, refused(bedStart, 5), b.seat)
	at := bedStart.Add(10 * time.Minute)
	b.cycle(at)
	if open := b.open(); len(open) != 1 || open[0].Standing != "" {
		t.Fatalf("no plain open episode before the helm: %+v", b.episodes())
	}
	b.takeHelm(b.top)
	result, err := steward.RunTick(b.repo, steward.TickConfig{Now: at.Add(cadence), Patterns: b.pass.Run}, quietCensus{})
	if err != nil {
		t.Fatalf("the helm tick failed: %v", err)
	}
	if result.Decision.Verdict != steward.VerdictHelm {
		t.Fatalf("the tick was not a helm tick: %+v", result.Decision)
	}
	open := b.open()
	if len(open) != 1 || open[0].Standing != steward.StandingHeld {
		t.Fatalf("the helm tick did not run the pattern pass: %+v", b.episodes())
	}
	// The report the person at the helm needs: main churned while they held
	// the lane, and the helm tick reports it.
	bare := b.origin()
	var commits []trunkCommit
	for write := 0; write < 6; write++ {
		commits = append(commits, trunkCommit{At: at.Add(cadence + time.Duration(write)*time.Minute), Opid: opid(write, "landing", LaneLineage), Path: "metasystem/plans/goals/trunk-red.json"})
	}
	b.push(bare, commits)
	if _, err := steward.RunTick(b.repo, steward.TickConfig{Now: at.Add(2 * cadence), Patterns: b.pass.Run}, quietCensus{}); err != nil {
		t.Fatalf("the second helm tick failed: %v", err)
	}
	if churn := openOf(b.churn()); len(churn) != 1 || len(b.sent) != 2 {
		t.Fatalf("the helm tick did not report the churn: %+v %q", churn, b.sent)
	}
}
