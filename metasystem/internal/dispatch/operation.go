package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
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
		return "", fmt.Errorf("default operation identity requires a role and lowercase SHA-256 brief digest")
	}
	wire := "delegate-operation-v2\x00" + goalID + "\x00" + strconv.FormatUint(goalRevision, 10) + "\x00" + string(mode) + "\x00" + role + "\x00" + briefDigest + "\x00" + parentJob
	sum := sha256.Sum256([]byte(wire))
	return role + "-" + hex.EncodeToString(sum[:12]), nil
}
