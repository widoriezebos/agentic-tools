package launch

// The presence observation, the wall clock, and the liveness of a launch.
//
// All three are the concrete sides of seams the sequencer and the
// reconciliation are written against, and all three are here rather than in
// the caller so that the verb and the server observe presence and judge a
// dead launch in exactly one way.

import (
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
)

// SeatPresence reads the fleet's presence through the seat transport, into a
// namespace of this launch's own.
//
// A namespace of its own is the same rule every other reader follows: two
// concurrent fetches into one shared ref collide on that ref's lock. It is
// also why nothing here touches the tick's copy or the interface's — a verb
// that fetched into either would move a ref its owner is reading.
type SeatPresence struct {
	Transport seat.Git
}

// Look fetches once and answers the record published under that machine.
//
// The record travels rather than a yes: what the caller is confirming is that
// THIS launch's machine published, and only the record's own repository
// identity and generation say that. A fetch that failed is not a machine that
// is silent: the error travels, and the caller tries again on its next tick.
func (p SeatPresence) Look(namespace, machine string) (seat.Record, bool, error) {
	if err := p.Transport.Fetch(namespace); err != nil {
		return seat.Record{}, false, err
	}
	copied, err := p.Transport.Read(namespace)
	if err != nil {
		return seat.Record{}, false, err
	}
	published, found := copied.Records[machine]
	return published, found, nil
}

// Taken is every nickname the fleet's presence already names, read through
// one fetch of this reader's own namespace.
//
// It fetches rather than reading whatever copy is on disk, and it surfaces
// both failures rather than answering with an empty fleet. A launch judges a
// nickname against this answer, and an empty answer from a fetch that failed
// would read as a free name — which is the one mistake this check exists to
// prevent.
func (p SeatPresence) Taken(namespace string) ([]string, error) {
	defer func() { _ = p.Forget(namespace) }()
	if err := p.Transport.Fetch(namespace); err != nil {
		return nil, refuse(CodeFleetUnreadable,
			"this seat could not bring in the fleet's presence, so it cannot say which nicknames are taken: %v", err)
	}
	copied, err := p.Transport.Read(namespace)
	if err != nil {
		return nil, refuse(CodeFleetUnreadable,
			"this seat could not read the fleet's presence, so it cannot say which nicknames are taken: %v", err)
	}
	names := make([]string, 0, len(copied.Records)+len(copied.Malformed))
	for name := range copied.Records {
		names = append(names, name)
	}
	// A record this seat could not parse is still a machine that published
	// under that name, so the name is spoken for.
	for name := range copied.Malformed {
		names = append(names, name)
	}
	return names, nil
}

// Forget deletes the namespace this launch fetched into.
func (p SeatPresence) Forget(namespace string) error {
	return p.Transport.DeleteNamespace(namespace)
}

// NewNamespace is one launch's own presence fetch namespace.
func NewNamespace() (string, error) {
	id, err := goal.NewOperationULID()
	if err != nil {
		return "", err
	}
	return seat.FetchNamespacePrefix + "/" + id, nil
}

// Wall is the clock every run but a test's uses.
type Wall struct{}

func (Wall) Now() time.Time                            { return time.Now().UTC() }
func (Wall) After(wait time.Duration) <-chan time.Time { return time.After(wait) }

// Live reports whether the process a record names is still the one that
// started that launch.
//
// The pair decides it. A pid alone is reused, so a record naming a pid some
// other program now carries would read as a launch still running; the
// kernel's own start time for that pid is what tells the two apart.
func Live(process Process) bool {
	if process.PID <= 0 {
		return false
	}
	born, alive := identity.ProcessBirth(int64(process.PID))
	if !alive {
		return false
	}
	if process.StartedAt == 0 {
		return true
	}
	return born.Unix() == process.StartedAt
}

// Identify is this process's own identity, written into the record it starts.
func Identify(pid int) Process {
	born, alive := identity.ProcessBirth(int64(pid))
	if !alive {
		return Process{PID: pid}
	}
	return Process{PID: pid, StartedAt: born.Unix()}
}
