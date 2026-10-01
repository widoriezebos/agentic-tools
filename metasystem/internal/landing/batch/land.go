package batch

import (
	"fmt"
	"strings"
)

// LandingProgress is the durable boundary between local construction, the
// single push, and trailer-based recovery.
type LandingProgress struct {
	Base          string            `json:"base"`
	Commits       map[string]string `json:"commits,omitempty"`
	BuildCommits  map[string]string `json:"buildCommits,omitempty"`
	BranchTip     string            `json:"branchTip,omitempty"` // legacy current-tip projection
	CandidateTip  string            `json:"candidateTip,omitempty"`
	ReceiptTip    string            `json:"receiptTip,omitempty"`
	HeldChecked   bool              `json:"heldChecked,omitempty"`
	PushComplete  bool              `json:"pushComplete,omitempty"`
	PushedTip     string            `json:"pushedTip,omitempty"`
	CleanupDone   bool              `json:"cleanupDone,omitempty"`
	RefusedOrigin string            `json:"refusedOrigin,omitempty"`
	RefusedBase   string            `json:"refusedBase,omitempty"`
	PushRejection *PushRejection    `json:"pushRejection,omitempty"`
}

func (progress LandingProgress) candidateTip() string {
	if progress.CandidateTip != "" {
		return progress.CandidateTip
	}
	return progress.BranchTip
}

func (progress LandingProgress) publishedTip() string {
	if progress.ReceiptTip != "" {
		return progress.ReceiptTip
	}
	return progress.candidateTip()
}

// PushRejection is the durable held state for a remote refusal that was not
// caused by a stale lease. The green proof remains valid while the endpoint is
// unchanged, so later ticks have no work to repeat.
type PushRejection struct {
	Text      string `json:"text"`
	At        string `json:"at"`
	OriginTip string `json:"originTip"`
}

// MissingLeaseBaseError reports an impossible endpoint transaction: recovery
// cannot compare a commit lease with a tree or an inferred fallback.
type MissingLeaseBaseError struct{}

// FlakeAllowanceRefusal is the recheck's refusal to publish a composed proof
// whose known flake is no longer carried: every member was already returned.
type FlakeAllowanceRefusal struct{ Code, Reason string }

// A moved base is returned, never republished (lane runtime design r10,
// K3): a lease the endpoint refuses because main moved abandons the
// candidate and reopens the batch on the new main.

// BaseMove is what moved main under a batch: the changed paths, the
// installation prefix they are relative to, and the batch whose landing
// moved it (empty when no batch of this store did).
type BaseMove struct {
	Changed          []string
	Prefix, LandedBy string
}

// MovedBase is the moved-base decision: reopen, or rebase and keep the proof.
type MovedBase struct {
	Reopen bool
}

// movedBaseConflictLine is the return of a member whose changes do not apply
// on the moved base: BATCH_JOIN_CONFLICT's files and the next command.
func movedBaseConflictLine(conflict *assemblyConflict, landedBy string) string {
	files := strings.Join(conflict.Paths, ", ")
	if files == "" {
		files = conflict.Error()
	}
	with := "what landed on main"
	if landedBy != "" {
		with = "what landed in batch " + landedBy
	}
	return fmt.Sprintf("CONFLICT with %s (files %s). Rebase goal/%s on main, then metasystem work land %s.", with, files, conflict.GoalID, conflict.GoalID)
}
