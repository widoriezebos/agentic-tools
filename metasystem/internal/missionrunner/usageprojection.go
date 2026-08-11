package missionrunner

import (
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/events"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// aggregateUsageForProjection refreshes the mission usage aggregate so the
// fence projection that follows reads current totals — every terminal round
// reaches the fence ledger at each conclude, park, and failure exit
// (plans/patience-orphan-usage.md O4). Best-effort by design: a failure is
// witnessed as an aggregation-failed flight-recorder event and the older
// aggregate stands until the next successful call; it never fails a park, a
// conclusion, or an exit. Aggregation takes its own mission-fence lock,
// disjoint from the state lock, and a content-equal pass writes nothing.
func aggregateUsageForProjection(root, missionID, site string) {
	if err := mission.AggregateUsage(root, missionID); err != nil {
		message := clipSummary(err.Error())
		events.EmitOnce(root, "runner", "aggregation-failed", message,
			int64(os.Getpid()), 0, 1, map[string]string{
				"missionId": missionID, "site": site, "error": message, "level": "warning",
			})
	}
}
