package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"errors"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
)

// DefaultOperationID binds an implicit retry to the complete v2 operation
// identity. Follow-up identities include their direct parent, so each new
// round gets a distinct repository-wide operation id while an exact retry of
// that round derives the same id.
func DefaultOperationID(goalID string, goalRevision uint64, mode DispatchMode, role, briefDigest, parentJob string) (string, error) {
	if goalID == "" {
		if goalRevision != 0 {
			return "", fmt.Errorf("default operation identity has a revision without a goal")
		}
		goalID = "none-explicit"
	} else if goalRevision == 0 {
		return "", fmt.Errorf("default operation identity requires a positive goal revision")
	}
	if mode != DispatchModeFresh && mode != DispatchModeFollowUp {
		return "", fmt.Errorf("default operation identity requires fresh or follow-up mode")
	}
	if mode == DispatchModeFresh && parentJob != "" {
		return "", fmt.Errorf("fresh default operation identity cannot name a parent job")
	}
	if mode == DispatchModeFollowUp && !validJobID.MatchString(parentJob) {
		return "", fmt.Errorf("follow-up default operation identity requires a valid parent job")
	}
	if role == "" || !incarnationRe.MatchString(briefDigest) {
		return "", errors.New("an operation name needs a role and the brief's lowercase SHA-256 checksum")
	}
	wire := "delegate-operation-v2\x00" + goalID + "\x00" + strconv.FormatUint(goalRevision, 10) + "\x00" + string(mode) + "\x00" + role + "\x00" + briefDigest + "\x00" + parentJob
	sum := sha256.Sum256([]byte(wire))
	return role + "-" + hex.EncodeToString(sum[:12]), nil
}

// OperationAttempt is attempt n of the operation named base: base itself
// for the first, and for a repeat after a start refused at setup a fresh
// name in the same grammar, so the refused start's record stays as it is
// (R-129: a repeat of the request is a new attempt, not a replay).
func OperationAttempt(base string, attempt int) string {
	at := strings.LastIndex(base, "-")
	if attempt <= 1 || at < 0 {
		return base
	}
	sum := sha256.Sum256([]byte("delegate-operation-attempt-v1\x00" + base + "\x00" + strconv.Itoa(attempt)))
	return base[:at] + "-" + hex.EncodeToString(sum[:12])
}

// NeverLaunched says whether a job record is a start refused at setup, which
// never started an agent (the setup-refusal-release rule).
func NeverLaunched(record map[string]any) bool {
	return !goalbudget.ReservationConsumesBudget(TerminalStatus(asString(record["status"])), asString(record["phase"]), asString(record["refusalClass"]))
}
