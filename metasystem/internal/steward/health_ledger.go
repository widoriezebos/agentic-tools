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

	readWorld    func(string) bool
	readEndpoint func(string) (goal.Endpoint, error)
	project      func(goal.Endpoint, bool, time.Time) (goal.Projection, error)

	newWorld      bool
	endpoint      goal.Endpoint
	endpointErr   error
	projection    goal.Projection
	projectionErr error
}

func newHealthLedger(root string, now time.Time) *healthLedger {
	return &healthLedger{root: root, now: now, readWorld: goal.NewWorld, readEndpoint: goal.ResolveEndpoint, project: goal.Project}
}

func (l *healthLedger) read() *healthLedger {
	l.once.Do(func() {
		if l.newWorld = l.readWorld(l.root); !l.newWorld {
			return
		}
		if l.endpoint, l.endpointErr = l.readEndpoint(l.root); l.endpointErr != nil {
			return
		}
		l.projection, l.projectionErr = l.project(l.endpoint, false, l.now)
	})
	return l
}
