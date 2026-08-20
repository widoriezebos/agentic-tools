package steward

// The steward's verdict ladder (plans/idle-watchdog-design.md): given
// one tick's snapshot of workers, open work, and progress high-water
// marks, decide what this tick does. Death is a proof, not an
// absence; unknown dominates dead; a live worker is never displaced —
// visibility is the response until proven death.

import "fmt"

// Verdict is what this tick concluded about the repository.
type Verdict string

const (
	VerdictNoWork      Verdict = "no-work"
	VerdictHealthy     Verdict = "healthy"
	VerdictStalledIdle Verdict = "stalled-idle" // live worker, no progress
	VerdictStalledDead Verdict = "stalled-dead" // proven death with open work
	VerdictUnknown     Verdict = "unknown"      // liveness not provable
	VerdictDegraded    Verdict = "degraded"     // ledger/journal unreadable
)

// Action is what the tick does about it.
type Action string

const (
	ActNone   Action = "none"
	ActNotify Action = "notify"
	ActRevive Action = "revive" // always notification-gated downstream
)

// OpenWork is the ledger's answer, degraded-honest.
type OpenWork string

const (
	WorkNone     OpenWork = "none"     // valid Goal-free, or validly empty claims and journal
	WorkOwned    OpenWork = "owned"    // this machine's claim, legacy Current, or an owned journal entry
	WorkDegraded OpenWork = "degraded" // missing, malformed, mixed, unreadable
)

// Workers summarizes the census over enrolled sessions, delegate
// jobs, live gates, mission runners, and monitored runs.
type Workers struct {
	// Live counts identities proven alive by the clock-step-immune
	// process identity. A live gate or runner counts here.
	Live int
	// Untracked counts live processes nobody can account for; they
	// prevent a death proof.
	Untracked int
	// Unprovable counts records whose identity cannot prove death
	// (weaker delegate identities, unreadable stores).
	Unprovable int
	// CensusComplete is true only when a full scan succeeded; an
	// empty enrollment store without a completed scan proves nothing.
	CensusComplete bool
}

// Snapshot is one tick's input, assembled by the environment readers;
// the ladder itself touches nothing outside this struct.
type Snapshot struct {
	Work    OpenWork
	Workers Workers
	// TicksSinceProgress counts ticks since either high-water mark
	// (HEAD object id, claim-History opid set) last advanced.
	TicksSinceProgress int
	StaleTicks         int // noise threshold for a live worker
	// DryRevivals counts consecutive revivals without a high-water
	// advance; nothing the steward produces resets it.
	DryRevivals int
	MaxRevivals int
	// ActiveContinuation is an open continuation job record not yet
	// reaped; it suppresses any further dispatch.
	ActiveContinuation bool
}

// Decision is the tick's outcome: the verdict, the action, and the
// reason every notification and receipt carries.
type Decision struct {
	Verdict Verdict
	Action  Action
	Reason  string
}

// Decide runs the ladder.
func Decide(s Snapshot) Decision {
	if s.Work == WorkDegraded {
		return Decision{VerdictDegraded, ActNotify,
			"the goal ledger or journal cannot be read; refusing to guess"}
	}
	if s.Work == WorkNone {
		return Decision{VerdictNoWork, ActNone, "no open work on this machine"}
	}

	w := s.Workers
	if w.Live > 0 {
		if s.TicksSinceProgress < s.StaleTicks {
			return Decision{VerdictHealthy, ActNone,
				"a live worker with recent progress"}
		}
		return Decision{VerdictStalledIdle, ActNotify, fmt.Sprintf(
			"a live worker with no progress for %d ticks; a live holder is never displaced — operator attention requested",
			s.TicksSinceProgress)}
	}

	if !w.CensusComplete || w.Untracked > 0 || w.Unprovable > 0 {
		return Decision{VerdictUnknown, ActNotify, fmt.Sprintf(
			"death not provable (census complete: %v, untracked: %d, unprovable: %d); notifying, never spawning on doubt",
			w.CensusComplete, w.Untracked, w.Unprovable)}
	}

	// Proven death with open work. Staleness is irrelevant: dead
	// seconds after a fresh commit is still dead.
	if s.ActiveContinuation {
		return Decision{VerdictStalledDead, ActNotify,
			"worker provably dead, but a continuation is already open and unreaped"}
	}
	if s.DryRevivals >= s.MaxRevivals {
		return Decision{VerdictStalledDead, ActNotify, fmt.Sprintf(
			"worker provably dead, but %d revivals produced no progress; refusing to spawn again — operator needed",
			s.DryRevivals)}
	}
	return Decision{VerdictStalledDead, ActRevive,
		"worker provably dead with open work; reviving after delivered notification"}
}
