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
	// laneState reads a held goal's hand-in on the landing lane; nil reads
	// no lane, and the claim-quota role then shows the tick's verdict.
	laneState func(root, goalID, main string) (string, error)
	// stateHome is the host state home whose machine disk report the disk
	// role reads; empty is the registry's home.
	stateHome string
	progress  ProgressItems

	newWorld      bool
	endpoint      goal.Endpoint
	endpointErr   error
	projection    goal.Projection
	projectionErr error
}

func newHealthLedger(root string, now time.Time) *healthLedger {
	return &healthLedger{root: root, now: now, readWorld: goal.NewWorld, readEndpoint: goal.ResolveEndpoint, project: goal.Project}
}

func (l *healthLedger) item() {
	if l.progress != nil {
		l.progress.Item()
	}
}

func (l *healthLedger) read() *healthLedger {
	l.once.Do(func() {
		l.newWorld = l.readWorld(l.root)
		l.item()
		if !l.newWorld {
			return
		}
		l.endpoint, l.endpointErr = l.readEndpoint(l.root)
		l.item()
		if l.endpointErr != nil {
			return
		}
		if l.progress != nil {
			l.progress.Wait("external", 0)
		}
		l.projection, l.projectionErr = l.project(l.endpoint, false, l.now)
		if l.progress != nil {
			l.progress.Working()
		}
		l.item()
	})
	return l
}
