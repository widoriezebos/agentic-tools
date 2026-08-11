package dispatch

import (
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
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
	facts := ReapFacts{Status: asString(record["status"])}

	if facts.Status == "pending-setup" {
		if created, ok := record["createdAt"].(string); ok {
			if at, err := parseRecordTime(created); err == nil {
				facts.SetupAbandoned = now.Sub(at) > AbandonedSetupGrace
			}
		}
		return facts, nil
	}
	if facts.Status != "pending" && facts.Status != "running" {
		return facts, nil
	}

	// A job is inside its handshake while it has no session, whether the
	// record still says pending or an adapter already moved it to running.
	// The window ends at the deadline stamped at launch — the number the
	// waiting dispatcher works from — or, for a record without one, at the
	// handshake budget measured from startedAt.
	session := asString(record["sessionId"])
	budget, hasBudget := numInt(record["sessionEstablishedTimeoutSec"])
	inHandshake := facts.Status == "pending" || (facts.Status == "running" && session == "")
	if inHandshake && hasBudget && budget >= 1 {
		if deadline, ok := numInt(record["handshakeDeadline"]); ok && deadline >= 1 {
			facts.HandshakeWaiting = now.Unix() < deadline+graceSec
		} else if started, ok := record["startedAt"].(string); ok {
			if at, err := parseRecordTime(started); err == nil {
				age := int64(now.Sub(at) / time.Second)
				facts.HandshakeWaiting = age < budget+graceSec
			}
		}
	}

	facts.BudgetExpired = supervise.CapExpired(record, now)
	return facts, nil
}
