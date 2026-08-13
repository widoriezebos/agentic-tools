package identity

import (
	"strings"
)

// The kernel custodian discipline, shared by every process that judges a
// recorded job custodian: the supervision reaper's sweep and the mission
// runner's drain reap must reach the same verdict from the same record, so
// the proof lives here once.

// Custodian proves a job's recorded custodian three-way against the live
// process table: it is the SAME custodian only if the pid is alive at its
// recorded start AND its command still carries the job's tag — a recycled
// pid, or a process no longer bearing the tag, is a stranger and reads as
// dead-to-us. The fixture identity file
// (METASYSTEM_FAKE_PROCESS_IDENTITY_FILE) supplies the start time and command
// when it carries an entry for the pid — the same one-source override the
// census uses — but kernel death always vetoes it. Unknown (unreadable) never
// authorizes anything. The fixture is fenced at ARMING: supervise
// blocking-reserved-cap refuses to arm a fleet when the env var is set in a
// checkout whose metasystem.runtimes is not fake (review lease-census-7), so
// a leaked fixture cannot ride an armed component into production verdicts.
func Custodian(pid, start int64, tag string) Liveness {
	exact, state, err := (KernelProber{}).Probe(pid)
	if err == nil && state == Dead {
		return Dead
	}
	if entry, ok := FixtureEntryFor(pid); ok {
		if entry.StartedAt != start {
			return Dead
		}
		if tag != "" && !strings.Contains(entry.Command, tag) {
			return Dead
		}
		return Alive
	}
	if err != nil || state == Unknown {
		return Unknown
	}
	if exact.StartedAt.Unix() != start {
		return Dead
	}
	if tag != "" {
		// The B1 verdict-table row 5 (go-production-grade, human-approved
		// 2026-08-12): a tag is expected but the argv was UNREADABLE —
		// that is absence of evidence, not evidence of a stranger, so
		// the verdict is Unknown, which authorizes nothing. Before this
		// correction the empty join failed the tag check and read Dead,
		// authorizing process-lost on an unproven death — the exact
		// verdict the standing reaper's contract forbids.
		if !exact.ArgvKnown {
			return Unknown
		}
		if !strings.Contains(strings.Join(exact.Argv, " "), tag) {
			return Dead
		}
	}
	return Alive
}
