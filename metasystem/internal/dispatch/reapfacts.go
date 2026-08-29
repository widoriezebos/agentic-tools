package dispatch

import (
	"time"
)

// The record-only facts one reap decision needs. Everything here is a function
// of the record and the clock: process liveness, group wind-down, and the
// terminal CAS stay with the caller, which owns the processes. Budget expiry
// deliberately reuses the supervision reaper's verdict, so the standing reaper
// and a waiting dispatcher can never disagree about whether one record's
// budget has run out.
type ReapFacts struct {
	// Status is the record's current status, echoed for the caller's dispatch.
	Status string `json:"status"`
	// SetupAbandoned marks a pending-setup record old enough that its creating
	// dispatcher must have died between create and setup.
	SetupAbandoned bool `json:"setupAbandoned"`
	// HandshakeWaiting marks a job still inside its handshake window: no
	// session yet, a handshake budget, and the deadline (plus grace) not yet
	// passed. A waiting job is deferred, not reaped.
	HandshakeWaiting bool `json:"handshakeWaiting"`
	// ReconciliationDue marks a fingerprinted reservation whose bounded launch
	// window has ended without publishing a primary process identity. The next
	// reaper action is nonce-wide reconciliation, not a death verdict.
	ReconciliationDue bool `json:"reconciliationDue"`
	// BudgetExpired marks a job past its absolute cap (capDeadline, else
	// startedAt plus capMin minutes).
	BudgetExpired bool `json:"budgetExpired"`
}

// AbandonedSetupGrace: a pending-setup record older than this has been
// abandoned — its creating dispatcher finishes setup in seconds, so ten
// minutes of pending-setup means that dispatcher died between create and
// setup. Generous so a slow live dispatcher is never raced. Exported because
// it is THE setup grace: the standing reaper's verdict and the mission
// runner's drain clock must measure the same window.
const AbandonedSetupGrace = 10 * time.Minute

// HandshakeBackstopGraceSec is the slack past a recorded handshake deadline
// before a backstop may act — the same number dispatch.sh passes as
// handshake_backstop_grace_sec, named here so every Go caller computes
// handshake expiry from one constant.
const HandshakeBackstopGraceSec int64 = 2

// ComputeReapFacts derives the record-only reap facts for one job record.
// graceSec is the slack past a handshake deadline before the backstop may act.
func ComputeReapFacts(recordPath string, graceSec int64, now time.Time) (ReapFacts, error) {
	record, err := readPlainObject(recordPath)
	if err != nil {
		return ReapFacts{}, err
	}
	return ComputeReapFactsForRecord(record, graceSec, now), nil
}

// ComputeReapFactsForRecord derives reap facts from an already-read record so
// every reaper uses the same launch-window and reconciliation boundary.
func ComputeReapFactsForRecord(record map[string]any, graceSec int64, now time.Time) ReapFacts {
	facts := ReapFacts{Status: asString(record["status"])}
	identitylessReservation := fingerprintedIdentitylessReservation(record)
	handoffRequested := record["reconciliationHandoff"] != nil

	if facts.Status == "pending-setup" {
		// A fingerprinted reservation's deadline wakes reconciliation; it
		// never proves that its creator or an unrecorded child is gone.
		if asString(record["reservationDeadlinePurpose"]) == "wake-only" {
			facts.ReconciliationDue = identitylessReservation &&
				(handoffRequested || reservationWakeReached(record, now))
			return facts
		}
		if created, ok := record["createdAt"].(string); ok {
			if at, err := parseRecordTime(created); err == nil {
				facts.SetupAbandoned = now.Sub(at) > AbandonedSetupGrace
			}
		}
		return facts
	}
	if facts.Status != "pending" && facts.Status != "running" {
		return facts
	}

	// A job is inside its handshake while it has no session, whether the
	// record still says pending or an adapter already moved it to running.
	// The window ends at the deadline stamped at launch — the number the
	// waiting dispatcher works from — or, for a record without one, at the
	// handshake budget measured from startedAt.
	session := asString(record["sessionId"])
	budget, hasBudget := numInt(record["sessionEstablishedTimeoutSec"])
	inHandshake := facts.Status == "pending" || (facts.Status == "running" && session == "")
	handshakeWindowKnown := false
	if inHandshake && hasBudget && budget >= 1 {
		if deadline, ok := numInt(record["handshakeDeadline"]); ok && deadline >= 1 {
			handshakeWindowKnown = true
			facts.HandshakeWaiting = now.Unix() < deadline+graceSec
		} else if started, ok := record["startedAt"].(string); ok {
			if at, err := parseRecordTime(started); err == nil {
				handshakeWindowKnown = true
				age := int64(now.Sub(at) / time.Second)
				facts.HandshakeWaiting = age < budget+graceSec
			}
		}
	}
	if identitylessReservation && asString(record["phase"]) != "cancelling" {
		facts.ReconciliationDue = handoffRequested ||
			(handshakeWindowKnown && !facts.HandshakeWaiting) ||
			(!handshakeWindowKnown && reservationWakeReached(record, now))
	}

	facts.BudgetExpired = CapExpired(record, now)
	return facts
}

func fingerprintedIdentitylessReservation(record map[string]any) bool {
	status := asString(record["status"])
	if status != "pending-setup" && status != "pending" {
		return false
	}
	if asString(record["fingerprint"]) == "" || asString(record["instanceTag"]) == "" {
		return false
	}
	pid, hasPID := numInt(record["pid"])
	return !hasPID || pid < 1
}

func reservationWakeReached(record map[string]any, now time.Time) bool {
	deadline, ok := record["reservationDeadline"].(string)
	if !ok || deadline == "" {
		return false
	}
	at, err := parseRecordTime(deadline)
	return err == nil && !now.Before(at)
}

// CapExpired reports whether a job is past its absolute budget: the explicit
// capDeadline when the record carries one, else startedAt plus capMin minutes.
// Exported because it is THE budget verdict: the dispatch-side reap and the
// supervision reaper must reach the same conclusion from the same record.
func CapExpired(record map[string]any, now time.Time) bool {
	if deadline, ok := record["capDeadline"].(string); ok && deadline != "" {
		if t, err := time.Parse("2006-01-02T15:04:05Z", deadline); err == nil {
			return !now.Before(t)
		}
	}
	capMin, ok := numInt(record["capMin"])
	started, hasStarted := record["startedAt"].(string)
	if ok && capMin >= 1 && hasStarted {
		if t, err := time.Parse("2006-01-02T15:04:05Z", started); err == nil {
			return now.Sub(t) >= time.Duration(capMin)*time.Minute
		}
	}
	return false
}
