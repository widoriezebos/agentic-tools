package launch

// The landing kind (landing-lane-runtime-redesign §3, unit A-a): one landing
// agent per computer, started on demand by the lane's keeper, on the seat's
// launch machinery. It runs headless in the registered lane checkout with its
// brief on stdin, under its own lineage so a successor succeeds a dead
// predecessor's lease, and binds to the lane's process-creation fence as a
// seat does. The lane never starts a seat (Amendment 1): the guard is here,
// in the launcher, so no caller of Start gets past it.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

// LandingKind is the launch kind of the landing agent.
const LandingKind = "landing"

// LandingOwnerLineage is the owner lineage every landing agent runs under,
// one per computer: a successor succeeds its dead predecessor's lease and
// claims by the lease's own succession rule, and the kernel's agent-issued
// operations require descent from it (K7).
const LandingOwnerLineage = "landing-agent"

// LandingEnvironment is what a landing launch sets over the launcher's
// environment, as a seat's does: its lineage, and an empty delegate root and
// session id, so the child is a session of the lane checkout and not a
// delegate or the launcher's own session. Its PATH is the inherited one with
// the lane installation's bin directory (module is the lane's module root)
// first, so the `metasystem` its skill names is the lane's own engine.
func LandingEnvironment(module, inheritedPath string) []string {
	path := filepath.Join(module, "bin")
	if inheritedPath != "" {
		path += string(os.PathListSeparator) + inheritedPath
	}
	return []string{"METASYSTEM_OWNER_LINEAGE=" + LandingOwnerLineage, "METASYSTEM_DELEGATE_ROOT=", "METASYSTEM_SESSION_ID=", "PATH=" + path}
}

// LaneCheckout is the host's registered landing lane as the launcher's guard
// reads it: the checkout's top and its module root (the same path in a flat
// layout). Registered false is a host with no lane.
type LaneCheckout struct {
	Registered bool
	Checkout   string
	Module     string
	// Incomplete is set when an older engine wrote the lane record: the
	// guard still holds on its checkout, and no landing agent starts on it
	// until a person registers the lane again.
	Incomplete error
}

// landingNoFenceRoot refuses a landing launch that names no state root.
const landingNoFenceRoot = "this landing launch names no state root, so it has no stop fence to bind to"

func noFenceRoot(kind string) string {
	if kind == LandingKind {
		return landingNoFenceRoot
	}
	return seatNoFenceRoot
}

// fencedKind is a kind bound to its installation's process-creation fence:
// the steward's seat and the keeper's landing agent.
func fencedKind(kind string) bool { return kind == "seat" || kind == LandingKind }

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return realpath.Resolve(filepath.Clean(a)) == realpath.Resolve(filepath.Clean(b))
}

// admitOnLane is the launcher's lane guard. A seat is refused when it would
// run in, or bind to the fence of, the registered lane checkout; the landing
// kind is admitted only there (its working directory is the checkout, its
// fence the lane's), and one at a time. An unreadable lane refuses both, and
// the landing kind is refused where no lane reader is wired.
func (m *Manager) admitOnLane(spec StartSpec) error {
	if !fencedKind(spec.Kind) {
		return nil
	}
	if m.Lane == nil {
		if spec.Kind == LandingKind {
			return errors.New("the landing agent starts only on the landing lane, which this launcher cannot read")
		}
		return nil
	}
	lane, err := m.Lane()
	if err != nil {
		return fmt.Errorf("this computer's landing lane cannot be read, so no %s starts: %w", spec.Kind, err)
	}
	onLane := func(path string) bool {
		return lane.Registered && (samePath(path, lane.Checkout) || samePath(path, lane.Module))
	}
	if spec.Kind == "seat" {
		if onLane(spec.WorkingDirectory) || onLane(spec.FenceRoot) {
			return fmt.Errorf("%s is this computer's landing lane, and the landing lane never starts a seat", lane.Checkout)
		}
		return nil
	}
	if lane.Incomplete != nil {
		return fmt.Errorf("the landing agent starts only on a lane this engine registered: %w", lane.Incomplete)
	}
	if !lane.Registered || !samePath(spec.WorkingDirectory, lane.Checkout) || !onLane(spec.FenceRoot) {
		if !lane.Registered {
			return fmt.Errorf("the landing agent starts only on the landing lane, and none is registered")
		}
		return fmt.Errorf("the landing agent starts only in the lane checkout %s, not %s", lane.Checkout, spec.WorkingDirectory)
	}
	records, err := m.Store.List()
	if err != nil {
		return fmt.Errorf("the launches cannot be read, so no second landing agent can be ruled out: %w", err)
	}
	for _, record := range records {
		if record.Kind != LandingKind || record.State.Terminal() {
			continue
		}
		// A launch whose supervisor and child are both gone is reconciled
		// as ended, never kept as a phantom that blocks every successor.
		current, err := m.Status(record.ID)
		if err != nil {
			return fmt.Errorf("landing agent %s cannot be read, so no second one can be ruled out: %w", record.ID, err)
		}
		if !current.State.Terminal() {
			return fmt.Errorf("landing agent %s has not ended; one landing agent runs per computer", record.ID)
		}
	}
	return nil
}
