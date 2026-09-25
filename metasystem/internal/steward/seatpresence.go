package steward

// The seat-presence component: one machine's presence, fetched and published
// once per tick. It owns both its fetch and its publish, so a presence
// problem is reported as a presence problem and never as a ledger-attention
// failure, and it runs after the ledger-attention completion record and
// before the evidence-store reads whose failure ends the tick degraded, so a
// torn evidence store on this machine does not make it read dead to its
// peers.

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
)

// SeatPresenceReport is what one pass of the component did, for the tick's
// own report and for the tests.
type SeatPresenceReport struct {
	// Outcome is published, skipped or failed.
	Outcome string `json:"outcome"`
	Reason  string `json:"reason,omitempty"`
	Detail  string `json:"detail,omitempty"`
	Rung    int    `json:"rung,omitempty"`
	// Notified names every standing change this pass queued.
	Notified []string `json:"notified,omitempty"`
}

// seatPresenceWindow reads the reader's own stale window.
func seatPresenceWindow(repoRoot string) time.Duration {
	minutes, err := config.SeatPresenceStaleMinutes(filepath.Join(repoRoot, "metasystem.conf"))
	if err != nil || minutes == 0 {
		minutes = config.DefaultSeatPresenceStaleMinutes
	}
	return time.Duration(minutes) * time.Minute
}

func seatPresenceNamespace(repoRoot string) string {
	pinned, err := config.SeatPresenceNamespace(filepath.Join(repoRoot, "metasystem.conf"))
	if err != nil {
		return ""
	}
	return pinned
}

// RunSeatPresence fetches the fleet's presence, publishes this machine's own
// record, notices every peer's change of standing, and records what it did.
//
// The two failure classes are kept apart, as section 3 requires. A transport
// failure is this component's OUTCOME and the tick continues. A failure to
// write the component's own durable evidence — its publication state, its
// standings — is returned as the error, and the tick's rule for every
// component applies to it: the tick ends rather than pretending the
// observation was recorded.
func RunSeatPresence(repoRoot string, runner *seat.RunnerContext, generation int, now time.Time) (SeatPresenceReport, error) {
	report := SeatPresenceReport{}
	// The manual verb calls this same component with no runner context. That
	// is settled FIRST and before anything that can write publication state,
	// because a command process must not be able to move the health verdict
	// of the machine whose runner is the only lawful publisher.
	manual := runner == nil
	if manual {
		report = seatPresenceSkip(report, seat.SkipManualTick)
	}
	transport, err := seat.NewGit(repoRoot)
	if err != nil {
		detail := "the goal endpoint is unreadable: " + err.Error()
		if manual {
			report.Detail = detail
			return report, nil
		}
		return seatPresenceFailed(repoRoot, report, detail, now)
	}

	// The conflict check is only as good as the copy it reads, so the two
	// transport acts are told apart from the standings they feed: a fetch or
	// a read that failed leaves the previously fetched refs, which still
	// carry standings, but it does NOT license a publish. Publishing on a
	// stale or empty copy would overwrite a newer record from another
	// checkout publishing under this nickname — the one case the conflict
	// refusal exists to catch.
	var problems []string
	current := true
	if err := transport.Fetch(seat.TickNamespace); err != nil {
		problems = append(problems, "fetch failed: "+err.Error())
		current = false
	}
	fleetCopy, readErr := transport.Read(seat.TickNamespace)
	if readErr != nil {
		problems = append(problems, "read failed: "+readErr.Error())
		fleetCopy = seat.Copy{Records: map[string]seat.Record{}, Malformed: map[string]string{}}
		current = false
	}

	machine, enrolled := seat.Machine(repoRoot)
	var mine *seat.Record
	switch {
	case manual:
		// Already settled above; a manual tick reads and notices, and
		// publishes nothing.
	case !enrolled:
		report = seatPresenceSkip(report, seat.SkipNoNickname)
	case generation < 1:
		report = seatPresenceSkip(report, seat.SkipUnarmed)
	case !current:
		report.Outcome = seat.OutcomeFailed
		report.Detail = "presence was not published because this machine could not read the fleet's current presence"
	default:
		report, mine = publishSeatPresence(repoRoot, transport, fleetCopy, machine, *runner, now, report)
	}
	if mine != nil {
		// This machine knows its own record before any reader fetches it, so
		// the standings it derives report itself from what it just published
		// rather than from the copy it read a moment earlier.
		if fleetCopy.Records == nil {
			fleetCopy.Records = map[string]seat.Record{}
		}
		fleetCopy.Records[machine] = *mine
		delete(fleetCopy.Malformed, machine)
	}
	if len(problems) > 0 {
		report.Detail = strings.TrimSpace(report.Detail + " " + strings.Join(problems, "; "))
	}

	// A manual tick writes no publication state and changes no verdict.
	if !manual {
		state, _, stateErr := seat.LoadPublicationState(repoRoot)
		if stateErr != nil {
			// A torn publication state is not a reason to stop publishing:
			// it is rewritten here and health says unknown until it is.
			state = seat.PublicationState{}
		}
		switch report.Outcome {
		case seat.OutcomePublished:
			state = seat.RecordPublished(state, machine, seat.Rung(report.Rung), seatTickSeconds(runner), report.Detail, now)
		case seat.OutcomeSkipped:
			state = seat.RecordSkipped(state, report.Reason, now)
		default:
			state = seat.RecordFailed(state, report.Detail, now)
		}
		if err := seat.SavePublicationState(repoRoot, state); err != nil {
			return report, fmt.Errorf("write seat-presence publication state: %w", err)
		}
	}

	notified, standingsDetail, err := noticeSeatStandings(repoRoot, machine, fleetCopy, now)
	report.Notified = notified
	if standingsDetail != "" {
		report.Detail = strings.TrimSpace(report.Detail + " " + standingsDetail)
	}
	if err != nil {
		return report, fmt.Errorf("record seat-presence standings: %w", err)
	}
	return report, nil
}

func seatTickSeconds(runner *seat.RunnerContext) int {
	if runner == nil || runner.TickSeconds < 1 {
		return 1
	}
	return runner.TickSeconds
}

func seatPresenceSkip(report SeatPresenceReport, reason string) SeatPresenceReport {
	report.Outcome, report.Reason = seat.OutcomeSkipped, reason
	return report
}

func seatPresenceFailed(repoRoot string, report SeatPresenceReport, detail string, now time.Time) (SeatPresenceReport, error) {
	report.Outcome, report.Detail = seat.OutcomeFailed, detail
	state, _, _ := seat.LoadPublicationState(repoRoot)
	state = seat.RecordFailed(state, detail, now)
	if err := seat.SavePublicationState(repoRoot, state); err != nil {
		return report, fmt.Errorf("write seat-presence publication state: %w", err)
	}
	return report, nil
}

// publishSeatPresence composes this machine's record and climbs the ladder.
func publishSeatPresence(repoRoot string, transport seat.Git, fleetCopy seat.Copy, machine string,
	runner seat.RunnerContext, now time.Time, report SeatPresenceReport) (SeatPresenceReport, *seat.Record) {
	record, detail, err := seat.Compose(machine, runner, seat.ReadJobs(repoRoot), SeatBox(repoRoot, now), now)
	if err != nil {
		report.Outcome, report.Detail = seat.OutcomeFailed, err.Error()
		return report, nil
	}
	report.Detail = detail
	if published, seen := fleetCopy.Records[machine]; seen {
		if conflict := seat.Conflict(&published, record); conflict != nil {
			report.Outcome, report.Reason = seat.OutcomeSkipped, conflict.Error()
			return report, nil
		}
	}
	state, _, _ := seat.LoadPublicationState(repoRoot)
	branchTips, tipErr := transport.Tips(seat.TickNamespace, seat.BranchNamespace)
	if tipErr != nil {
		branchTips = map[string]string{}
	}
	result, err := seat.Publish(transport, seat.PublishRequest{
		Record:    record,
		Start:     seat.Rung(state.Rung),
		Pinned:    seatPresenceNamespace(repoRoot),
		BranchTip: branchTips[machine],
	})
	if err != nil {
		report.Outcome = seat.OutcomeFailed
		report.Detail = strings.TrimSpace(report.Detail + " " + err.Error())
		if result.Rung > 0 {
			report.Rung = int(result.Rung)
		}
		return report, nil
	}
	report.Outcome, report.Rung = seat.OutcomePublished, int(result.Rung)
	if result.Fell {
		report.Detail = strings.TrimSpace(report.Detail + " " + fmt.Sprintf("fell to rung %d (%s): %s",
			result.Rung, result.Rung.Description(), strings.Join(result.Refusals, "; ")))
	}
	return report, &record
}

// noticeSeatStandings compares every machine's standing with the standing
// recorded at the previous tick and queues one notification per change,
// deduplicated by (machine, standing, since). Order matters and it is ledger
// attention's order: every notification is queued first, then the new
// standings are persisted with their frozen since, so a crash between the
// two repeats a notification and never loses one.
// A standings file that cannot be read is NOT a baseline: the silent first
// read belongs to an absent file alone. Treating a torn one as absent would
// swallow every transition it holds the history for and then overwrite the
// only evidence of what went wrong, so an unreadable file is reported and
// left exactly where it is.
func noticeSeatStandings(repoRoot, machine string, fleetCopy seat.Copy, now time.Time) ([]string, string, error) {
	previous, existed, err := seat.LoadStandings(repoRoot)
	if err != nil {
		return nil, "the seat-fleet standings are unreadable and were left untouched: " + err.Error(), nil
	}
	claims, unavailable := seat.Claims(repoRoot)
	standings := seat.Fleet(seat.FleetInput{
		This: machine, Copy: fleetCopy, Claims: claims, ClaimsUnavailable: unavailable,
		Previous: previous.Machines, Now: now, Window: seatPresenceWindow(repoRoot),
	})
	var notified []string
	for _, notification := range seat.Transitions(previous.Machines, standings, !existed) {
		if err := QueueNotification(repoRoot, PendingNotification{Nonce: notification.Nonce, Message: notification.Message}); err != nil {
			return notified, "", err
		}
		notified = append(notified, notification.Nonce)
	}
	return notified, "", seat.SaveStandings(repoRoot, seat.NextStandings(standings, now))
}

// checkSeatPresence is the health role: one thing decided, alive or dead by
// the age of the last successful publish. The consecutive-failure counting
// and the escalation belong to the evaluator, which owns them for every role.
func checkSeatPresence(repoRoot string, now time.Time) RoleVerdict {
	state, readable, err := seat.LoadPublicationState(repoRoot)
	verdict := seat.Health(state, readable, err, now, seatPresenceWindow(repoRoot))
	switch verdict.Status {
	case seat.StatusDead:
		return roleDead(RoleSeatPresence, verdict.Reason, verdict.Remedy)
	case seat.StatusUnknown:
		return roleUnknown(RoleSeatPresence, verdict.Reason, verdict.Remedy)
	default:
		return roleAlive(RoleSeatPresence, verdict.Reason)
	}
}
