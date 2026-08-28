package census

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

// TaggedProcess is one stable process identity whose argv carries the
// reservation tag in a shipped position. PGID is observed in the same census
// so adoption never guesses a group from the process id.
type TaggedProcess struct {
	PID      int64
	PGID     int64
	Identity identity.Exact
	Universe ProcessUniverse
}

// ProcessUniverse says whether an enumerated process could have been launched
// by this dispatcher. Injected process tables are complete dispatcher
// universes by construction, so the zero value is signalable.
type ProcessUniverse uint8

const (
	ProcessUniverseSignalable ProcessUniverse = iota
	ProcessUniverseForeign
	ProcessUniverseExcludedByAge
)

func (u ProcessUniverse) String() string {
	switch u {
	case ProcessUniverseForeign:
		return "foreign"
	case ProcessUniverseExcludedByAge:
		return "excluded-by-age"
	default:
		return "signalable"
	}
}

// ReservationStartTimeSlack allows for wall-clock steps because process start
// time and reservation creation time come from that clock. A backward step
// larger than this slack during the exact creation-to-fork window can exclude
// a real child. That residue is named and bounded here, and is accepted instead
// of letting old unreadable processes make absence permanently unprovable.
const ReservationStartTimeSlack = 60 * time.Second

// IndeterminateProcess preserves a process observation whose tag membership
// could not be classified. A signalable unreadable process prevents an
// absence proof. A foreign or pre-reservation unreadable process stays in the
// result for diagnosis without blocking that proof.
type IndeterminateProcess struct {
	PID      int64
	PGID     int64
	Reason   string
	Universe ProcessUniverse
}

// TaggedProcessCensus is the adoption scanner's complete result. A process
// table failure is scan-wide; per-process probe failures remain individually
// named in Indeterminate.
type TaggedProcessCensus struct {
	Tagged           []TaggedProcess
	Indeterminate    []IndeterminateProcess
	EnumerationError string
}

// Complete reports whether this result may prove tag absence.
func (r TaggedProcessCensus) Complete() bool {
	return r.EnumerationError == "" && r.UnknownWithinUniverse() == 0
}

// UnknownWithinUniverse counts only observations that could belong to this
// reservation generation. Excluded observations remain in the result for
// diagnosis, but they cannot prevent a complete absence proof.
func (r TaggedProcessCensus) UnknownWithinUniverse() int {
	unknown := 0
	for _, process := range r.Indeterminate {
		if process.Universe == ProcessUniverseSignalable {
			unknown++
		}
	}
	return unknown
}

// ForeignCount reports observations excluded by the dispatcher's permission
// domain while retaining them as evidence.
func (r TaggedProcessCensus) ForeignCount() int {
	return r.countUniverse(ProcessUniverseForeign)
}

// ExcludedByAgeCount reports same-user observations that started too early to
// carry a tag minted by this reservation generation.
func (r TaggedProcessCensus) ExcludedByAgeCount() int {
	return r.countUniverse(ProcessUniverseExcludedByAge)
}

func (r TaggedProcessCensus) countUniverse(universe ProcessUniverse) int {
	count := 0
	for _, process := range r.Indeterminate {
		if process.Universe == universe {
			count++
		}
	}
	return count
}

// TaggedProcessScanner is the seam reservation reconciliation consumes.
type TaggedProcessScanner interface {
	ScanTag(tag string, reservationCreatedAt time.Time) TaggedProcessCensus
}

// KernelTaggedProcessScanner binds the result shape to the live process table.
type KernelTaggedProcessScanner struct {
	MatchesTag func(argv []string, tag string) bool
}

func (s KernelTaggedProcessScanner) ScanTag(tag string, reservationCreatedAt time.Time) TaggedProcessCensus {
	if s.MatchesTag == nil {
		return TaggedProcessCensus{EnumerationError: "tag-position matcher is unavailable"}
	}
	return ScanTaggedProcesses(tag, TaggedScanDependencies{
		MatchesTag: s.MatchesTag, ReservationCreatedAt: reservationCreatedAt,
	})
}

// TaggedScanDependencies keeps process-table access injectable while the
// result and its completeness law remain owned here.
type TaggedScanDependencies struct {
	PIDs       func() ([]int64, error)
	Signal     func(pid int64) error
	PGID       func(pid int64) (int64, error)
	Reader     identity.VerificationReader
	MatchesTag func(argv []string, tag string) bool
	// A zero time preserves the fixture and non-adoption scanner behavior:
	// without a reservation generation, age cannot narrow the universe.
	ReservationCreatedAt time.Time
}

// ScanTaggedProcesses classifies each process's universe before applying the
// ordered identity sandwich to candidates. Probe failures become explicit
// indeterminate entries; only definitive exits disappear from the live result.
func ScanTaggedProcesses(tag string, dependencies TaggedScanDependencies) TaggedProcessCensus {
	if dependencies.PIDs == nil {
		dependencies.PIDs = identity.AllPids
	}
	if dependencies.Signal == nil {
		dependencies.Signal = func(pid int64) error {
			return unix.Kill(int(pid), 0)
		}
	}
	if dependencies.PGID == nil {
		dependencies.PGID = func(pid int64) (int64, error) {
			group, err := unix.Getpgid(int(pid))
			return int64(group), err
		}
	}
	if dependencies.Reader == nil {
		dependencies.Reader = identity.KernelProber{}
	}
	pids, err := dependencies.PIDs()
	if err != nil {
		return TaggedProcessCensus{EnumerationError: err.Error()}
	}
	if dependencies.MatchesTag == nil {
		return TaggedProcessCensus{EnumerationError: "tag-position matcher is unavailable"}
	}
	sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })
	result := TaggedProcessCensus{}
	for _, pid := range pids {
		universe := processPermissionUniverse(pid, dependencies.Signal)
		if universe == ProcessUniverseForeign {
			result.Indeterminate = append(result.Indeterminate, IndeterminateProcess{
				PID: pid, Reason: "signal-zero-permission-denied", Universe: universe,
			})
			continue
		}
		verification := identity.VerifyProcess(dependencies.Reader, pid, func(argv []string) bool {
			return dependencies.MatchesTag(argv, tag)
		})
		switch verification.Outcome {
		case identity.VerificationVerified:
			group, groupErr := dependencies.PGID(pid)
			if groupErr != nil {
				if errors.Is(groupErr, unix.ESRCH) {
					continue
				}
				result.Indeterminate = append(result.Indeterminate, IndeterminateProcess{
					PID: pid, Reason: "process-group-unreadable: " + groupErr.Error(),
					Universe: processUniverse(universe, verification.Identity.StartedAt, dependencies.ReservationCreatedAt),
				})
				continue
			}
			confirmed, state, confirmErr := dependencies.Reader.ReadStart(pid)
			if state == identity.Dead && confirmErr == nil {
				continue
			}
			if confirmErr != nil || state != identity.Alive {
				result.Indeterminate = append(result.Indeterminate, IndeterminateProcess{
					PID: pid, PGID: group, Reason: "identity-after-group-unreadable",
					Universe: processUniverse(universe, verification.Identity.StartedAt, dependencies.ReservationCreatedAt),
				})
				continue
			}
			comparison := identity.Compare(confirmed, verification.Identity.Ref())
			if comparison.Mode == identity.CompareInvalid || !comparison.Matches {
				result.Indeterminate = append(result.Indeterminate, IndeterminateProcess{
					PID: pid, PGID: group, Reason: "identity-changed-during-group-read",
					Universe: processUniverse(universe, confirmed.StartedAt, dependencies.ReservationCreatedAt),
				})
				continue
			}
			result.Tagged = append(result.Tagged, TaggedProcess{
				PID: pid, PGID: group, Identity: confirmed, Universe: universe,
			})
		case identity.VerificationIndeterminate:
			result.Indeterminate = append(result.Indeterminate, IndeterminateProcess{
				PID: pid, Reason: fmt.Sprintf("identity-%s", verification.Outcome),
				Universe: processUniverse(universe, verification.Identity.StartedAt, dependencies.ReservationCreatedAt),
			})
		case identity.VerificationDead, identity.VerificationNotOurs:
			// A definitive exit or readable non-match contributes no survivor.
		}
	}
	return result
}

func processPermissionUniverse(pid int64, signal func(int64) error) ProcessUniverse {
	// The dispatcher can only have launched processes it can signal. A
	// permission denial therefore proves that a process belongs to a foreign
	// permission domain: its unreadable identity cannot match and cannot block
	// absence. Every other signal result stays inside the candidate universe;
	// the identity sandwich decides whether it is alive, dead, or unreadable.
	if errors.Is(signal(pid), unix.EPERM) {
		return ProcessUniverseForeign
	}
	return ProcessUniverseSignalable
}

func processUniverse(permissionUniverse ProcessUniverse, startedAt, reservationCreatedAt time.Time) ProcessUniverse {
	if permissionUniverse != ProcessUniverseSignalable || startedAt.IsZero() || reservationCreatedAt.IsZero() {
		return permissionUniverse
	}
	if startedAt.Before(reservationCreatedAt.Add(-ReservationStartTimeSlack)) {
		// The sixty-second slack deliberately bounds the backward-clock residue.
		// We prefer that finite risk in the creation-to-fork window to letting
		// every old unreadable same-user process block absence forever.
		return ProcessUniverseExcludedByAge
	}
	return permissionUniverse
}
