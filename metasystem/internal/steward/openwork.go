package steward

// The open-work reader over the legacy single-file ledger — the
// format this repository runs today. The new per-goal format's
// reader joins when the backlog cutover lands; both feed the same
// OpenWork verdict. Degraded-honest: unreadable or unparseable
// state never means "no work".

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// ReadOpenWork answers whether delegated work is open on THIS
// machine, routing on the checkout's world: a converted checkout
// judges from the synced ledger by this machine's enrolled
// nickname, and a legacy checkout keeps the single-file reading
// byte-identical. The verdict feeds the dead-man's decision, so
// absence of a readable ledger is degraded-honest, never no-work.
func ReadOpenWork(repoRoot string) (OpenWork, string, error) {
	resolvedRoot, err := goal.ResolveStateRoot(repoRoot)
	if err != nil {
		return WorkDegraded, fmt.Sprintf("goal state root is uncertain: %v", err), nil
	}
	repoRoot = resolvedRoot
	if goal.NewWorld(repoRoot) {
		return convertedOpenWork(repoRoot)
	}
	return LegacyOpenWork(repoRoot)
}

// convertedOpenWork consumes the same fresh, liveness-joined predicate as the
// turn verdict. A stale claim or job record never makes backlog look active.
func convertedOpenWork(repoRoot string) (OpenWork, string, error) {
	if attention, present, err := loadLedgerAttentionState(repoRoot); err != nil {
		return WorkDegraded, fmt.Sprintf("ledger-attention state unreadable: %v", err), nil
	} else if present && attention.LastOutcome == "failed" {
		return WorkDegraded, fmt.Sprintf("fresh canonical ledger read failed: %s", attention.LastFailure), nil
	}
	work, err := goal.ReadClaimableBudgetedWork(repoRoot, time.Now())
	if err != nil {
		return WorkDegraded, fmt.Sprintf("fresh canonical ledger unreadable: %v", err), nil
	}
	return classifySharedBacklog(work)
}

func classifySharedBacklog(work goal.ClaimableBudgetedWork) (OpenWork, string, error) {
	if work.HasInFlight() {
		return WorkInFlight, fmt.Sprintf("live backlog activity: %s", strings.Join(work.InFlight, ", ")), nil
	}
	if len(work.Claimable) > 0 {
		return WorkClaimable, fmt.Sprintf("claimable shared goals await a live claim or job: %s", strings.Join(work.Claimable, ", ")), nil
	}
	if len(work.Claimed) > 0 {
		// A claim-only stale world remains owned work for the existing worker
		// census and revival ladder. It is not in flight: only the joined
		// branch above may make that claim. A separate ready queue takes the
		// claimable branch first, so this stale record cannot suppress it.
		return WorkOwned, fmt.Sprintf("claimed goals have no directly joined live process: %s", strings.Join(work.Claimed, ", ")), nil
	}
	if work.GoalFree {
		return WorkNone, "goal-free declared", nil
	}
	if work.Queued > 0 {
		return WorkNone, fmt.Sprintf("%d queued goals are visible but not claimable budgeted work", work.Queued), nil
	}
	return WorkNone, "no claim held here and no declaration", nil
}

// LegacyOpenWork reads plans/goals.md and answers whether delegated
// work is open on this machine. Pre-cutover the ledger is
// single-machine by standing rule, so a Current goal is this
// machine's work.
func LegacyOpenWork(repoRoot string) (OpenWork, string, error) {
	path := goal.LedgerPath(repoRoot)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// The converged design is degraded-honest: only a valid
		// explicit declaration is no-work. An absent ledger might be
		// a half-deleted checkout mid-incident — exactly when a
		// wrong "nothing to do" would be most costly.
		return WorkDegraded, "no goal ledger exists; refusing to conclude no-work from absence", nil
	}
	if err != nil {
		return WorkDegraded, fmt.Sprintf("goal ledger unreadable: %v", err), nil
	}
	ledger, problems := goal.Parse(data)
	if len(problems) > 0 {
		return WorkDegraded, fmt.Sprintf("goal ledger has %d parse problems; refusing to guess", len(problems)), nil
	}
	_ = ledger
	work, err := goal.ReadClaimableBudgetedWork(repoRoot, time.Now())
	if err != nil {
		return WorkDegraded, fmt.Sprintf("legacy backlog judgment failed: %v", err), nil
	}
	return classifySharedBacklog(work)
}
