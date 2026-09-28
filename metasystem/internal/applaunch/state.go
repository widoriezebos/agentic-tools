package applaunch

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// State is the run's liveness, read from the record's refs alone. Readiness
// is a separate answer, because an application can be alive and not
// answering, and saying one for the other is how a review ends up looking at
// something that is not there.
type State string

const (
	// Running: the supervisor is alive and the application is recorded.
	Running State = "running"
	// Starting: the supervisor is alive and has not yet recorded readiness.
	Starting State = "starting"
	// ChildEnded: the application ended and processes of the supervisor's
	// own group are still alive, so the supervisor stays the owner.
	ChildEnded State = "child-ended"
	// Finished: the run ended. This is a record, not a process.
	Finished State = "ended"
	// Stopped: no record at all.
	Stopped State = "stopped"
	// Orphaned: the supervisor is gone with an application recorded. The
	// application may still run, and stop ends it by its recorded ref.
	Orphaned State = "orphaned"
	// Stale: the supervisor is gone without having written ended and without
	// an application recorded — the one instant between the spawn and the
	// child's write. The record is kept and said, never removed by a reader.
	Stale State = "stale"
	// Uninspectable: an identity that could not be proved either way.
	Uninspectable State = "uninspectable"
	// Unreadable: a record that will not parse.
	Unreadable State = "unreadable"
)

// Readiness is the second answer, asked of the probe for the http and tcp
// forms and read from the run's own startup observation for log and none.
type Readiness string

const (
	Answering    Readiness = "answering"
	NotAnswering Readiness = "not-answering"
	ObservedOnce Readiness = "observed-at-startup"
	NotYet       Readiness = "not-yet-ready"
	NoProbe      Readiness = "no-probe"
)

// Status is one run's two answers and the record they were read from.
type Status struct {
	Key       string
	State     State
	Readiness Readiness
	// Since is when the answering probe last changed, as far as this record
	// knows: the run's readiness instant, or its start.
	Since   string
	Record  *Record
	Members []Member
	Problem string
}

// Member is one live process of a recorded group, named by its proven
// identity. It exists so that status can list what an inspection finds
// without ever signalling a bare number.
type Member struct {
	Pid  int64
	Ref  string
	Self bool
}

// Prober is the identity reader; a test supplies its own.
type Prober = identity.Prober

// GroupReader enumerates the live members of one process group. It is a seam
// only so that a test can hold a group it did not create.
type GroupReader func(pgid int64) ([]Member, error)

// KernelGroup reads the process table and keeps the members of one group
// whose identity can be proved.
func KernelGroup(pgid int64) ([]Member, error) {
	if pgid < 1 {
		return nil, nil
	}
	pids, err := identity.AllPids()
	if err != nil {
		return nil, err
	}
	prober := identity.KernelProber{}
	var members []Member
	for _, pid := range pids {
		group, err := unix.Getpgid(int(pid))
		if err != nil || int64(group) != pgid {
			continue
		}
		exact, state, err := prober.Probe(pid)
		if err != nil || state != identity.Alive || !exact.ArgvKnown {
			// A zombie keeps its group signalable but is finished work: a
			// KILL can do no more to it, and counting it would hold a stop
			// open for its parent's reaping debt.
			continue
		}
		encoded, err := identity.EncodeRef(exact.Ref())
		if err != nil {
			continue
		}
		members = append(members, Member{Pid: pid, Ref: encoded})
	}
	sort.Slice(members, func(i, j int) bool { return members[i].Pid < members[j].Pid })
	return members, nil
}

// ReadOptions are the readers one status answer needs.
type ReadOptions struct {
	Prober Prober
	Group  GroupReader
	// Probe asks the contract's readiness probe once. A nil probe means the
	// caller wants liveness only.
	Probe func(contract Contract, address string) error
	Now   func() time.Time
}

func (o ReadOptions) prober() Prober {
	if o.Prober != nil {
		return o.Prober
	}
	return identity.KernelProber{}
}

func (o ReadOptions) group() GroupReader {
	if o.Group != nil {
		return o.Group
	}
	return KernelGroup
}

// Read is the one reader every verb uses. It removes nothing: a stale record
// is said and kept, which is where this owner departs from the interface's
// lifecycle, whose reader takes a stale record away under its lock. Here the
// record carries the last check and the data word until the evidence of the
// run has been copied, and only stop, reset or the next start may remove it.
func Read(stateRoot, key string, contract Contract, o ReadOptions) (Status, error) {
	status := Status{Key: key}
	record, err := ReadRecord(stateRoot, key)
	if errors.Is(err, os.ErrNotExist) {
		status.State, status.Readiness = Stopped, NoProbe
		return status, nil
	}
	if err != nil {
		status.State, status.Readiness, status.Problem = Unreadable, NoProbe, err.Error()
		return status, nil
	}
	status.Record = record
	status.Since = record.StartedAt
	if record.ReadyAt != "" {
		status.Since = record.ReadyAt
	}
	if record.Ended != nil {
		status.State, status.Readiness = Finished, NoProbe
		return status, nil
	}
	supervisor, refErr := record.SupervisorRef()
	if refErr != nil {
		status.State, status.Readiness, status.Problem = Unreadable, NoProbe, refErr.Error()
		return status, nil
	}
	prober := o.prober()
	switch identity.AliveRef(prober, supervisor) {
	case identity.Unknown:
		status.State, status.Readiness = Uninspectable, NoProbe
		return status, nil
	case identity.Dead:
		status.Members, _ = o.group()(record.Group)
		if record.Child == "" {
			status.State, status.Readiness = Stale, NoProbe
			status.Problem = fmt.Sprintf("supervisor gone, group %d, child not recorded", record.Group)
			return status, nil
		}
		status.State, status.Readiness = Orphaned, NoProbe
		status.Problem = "orphaned: the application may still run"
		if child, _, err := record.ChildRef(); err == nil {
			switch identity.AliveRef(prober, child) {
			case identity.Dead:
				status.Problem = "orphaned: the supervisor is gone and the application is dead"
			case identity.Unknown:
				status.Problem = "orphaned: the supervisor is gone and the application is uninspectable"
			}
		}
		return status, nil
	}
	// The supervisor is alive, so the record is current. What the child is
	// doing decides between starting, running and child-ended.
	child, recorded, err := record.ChildRef()
	if !recorded || err != nil {
		status.State, status.Readiness = Starting, NotYet
		return status, nil
	}
	switch identity.AliveRef(prober, child) {
	case identity.Unknown:
		status.State, status.Readiness = Uninspectable, NoProbe
		return status, nil
	case identity.Dead:
		status.Members, _ = o.group()(record.Group)
		if len(livingBesides(status.Members, supervisor.Pid)) > 0 {
			status.State, status.Readiness = ChildEnded, NoProbe
			status.Problem = "child ended, descendants alive; the supervisor stays the owner"
			return status, nil
		}
		status.State, status.Readiness = ChildEnded, NoProbe
		status.Problem = "child ended; the supervisor is writing the ended record"
		return status, nil
	}
	if record.ReadyAt == "" {
		status.State, status.Readiness = Starting, NotYet
		return status, nil
	}
	status.State = Running
	status.Readiness = readinessOf(contract, record, o)
	return status, nil
}

func readinessOf(contract Contract, record *Record, o ReadOptions) Readiness {
	if !contract.Probed() {
		// A log line cannot unmatch and a form with nothing to probe cannot
		// go dark: both are observations of this run's start, and they stay
		// true for the run's life.
		if record.ReadyAt != "" {
			return ObservedOnce
		}
		return NotYet
	}
	if o.Probe == nil {
		return NoProbe
	}
	if o.Probe(contract, record.Address) == nil {
		return Answering
	}
	return NotAnswering
}

func livingBesides(members []Member, pid int64) []Member {
	var others []Member
	for _, member := range members {
		if member.Pid != pid {
			others = append(others, member)
		}
	}
	return others
}

// Lines renders one status as a person reads it: liveness and readiness
// said separately, then the facts of the run.
func (s Status) Lines() []string {
	lines := []string{"state: " + string(s.State)}
	if s.Problem != "" {
		lines = append(lines, s.Problem)
	}
	switch s.Readiness {
	case Answering:
		lines = append(lines, "readiness: answering")
	case NotAnswering:
		since := s.Since
		if since == "" {
			since = "an unrecorded time"
		}
		lines = append(lines, "readiness: not answering since "+since)
	case ObservedOnce:
		lines = append(lines, "readiness: ready, observed at startup")
	case NotYet:
		lines = append(lines, "readiness: not yet ready")
	}
	if s.Record == nil {
		return lines
	}
	record := s.Record
	if record.Address != "" {
		lines = append(lines, "address: "+record.Address)
	}
	if record.Ref != "" {
		lines = append(lines, "ref: "+record.Ref)
	}
	if record.Commit != "" {
		lines = append(lines, "commit: "+record.Commit)
	}
	if record.Goal != "" {
		lines = append(lines, "goal: "+record.Goal)
	}
	if record.StartedAt != "" {
		lines = append(lines, "since: "+record.StartedAt)
	}
	lines = append(lines, record.DataSentence())
	if record.Log != "" {
		lines = append(lines, "log: "+record.Log)
	}
	for _, tool := range record.Tools {
		lines = append(lines, "tool "+tool.Line())
	}
	if record.Ended != nil {
		lines = append(lines, "ended: "+record.Ended.At+" ("+record.Ended.ExitStatus+")")
	}
	if record.Check != nil {
		lines = append(lines, "check "+record.Check.Group+": "+record.Check.Verdict+" at "+record.Check.At)
	} else {
		lines = append(lines, "check: none recorded for this run")
	}
	for _, member := range s.Members {
		lines = append(lines, "group member "+strconv.FormatInt(member.Pid, 10)+" "+member.Ref)
	}
	return lines
}
