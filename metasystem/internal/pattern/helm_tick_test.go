package pattern

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

type quietCensus struct{}

func (quietCensus) Workers(string) (steward.Workers, error) { return steward.Workers{}, nil }

// Under the helm the steward's tick decides nothing (HM-7), but the pattern
// pass still reports (D2): the real pass, called by the real helm tick.
func TestHelmTickReports(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	at := bedStart.Add(10 * time.Minute)
	b.cycle(at)
	b.takeHelm(b.top)
	result, err := steward.RunTick(b.repo, steward.TickConfig{Now: at.Add(cadence), Patterns: b.pass.Run}, quietCensus{})
	if err != nil {
		t.Fatalf("the helm tick failed: %v", err)
	}
	if result.Decision.Verdict != steward.VerdictHelm {
		t.Fatalf("the tick was not a helm tick: %+v", result.Decision)
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
	if churn := openOf(b.churn()); len(churn) != 1 || len(b.sent) != 1 {
		t.Fatalf("the helm tick did not report the churn: %+v %q", churn, b.sent)
	}
}

// cadence is the default steward cadence.
const cadence = 600 * time.Second
