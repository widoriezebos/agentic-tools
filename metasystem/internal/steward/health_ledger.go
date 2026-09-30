package steward

import (
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// healthLedger reads the goal ledger once for one health evaluation. The
// claimed-goal budget, stop-capability, delivery and trunk-red roles each
// projected it on their own, about half a second apiece on a seat's Stop.
type healthLedger struct {
	root string
	now  time.Time
	once sync.Once

	newWorld      bool
	endpoint      goal.Endpoint
	endpointErr   error
	projection    goal.Projection
	projectionErr error
}

var (
	healthLedgerNewWorld = goal.NewWorld
	healthLedgerResolve  = goal.ResolveEndpoint
	healthLedgerProject  = goal.Project
)

func newHealthLedger(root string, now time.Time) *healthLedger {
	return &healthLedger{root: root, now: now}
}

func (l *healthLedger) read() *healthLedger {
	l.once.Do(func() {
		if l.newWorld = healthLedgerNewWorld(l.root); !l.newWorld {
			return
		}
		if l.endpoint, l.endpointErr = healthLedgerResolve(l.root); l.endpointErr != nil {
			return
		}
		l.projection, l.projectionErr = healthLedgerProject(l.endpoint, false, l.now)
	})
	return l
}
