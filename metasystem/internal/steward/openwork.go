package steward

// The open-work reader over the legacy single-file ledger — the
// format this repository runs today. The new per-goal format's
// reader joins when the backlog cutover lands; both feed the same
// OpenWork verdict. Degraded-honest: unreadable or unparseable
// state never means "no work".

import (
	"fmt"
	"os"
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
	if goal.NewWorld(repoRoot) {
		return convertedOpenWork(repoRoot)
	}
	return LegacyOpenWork(repoRoot)
}

// convertedOpenWork reads the synced projection: work is OWNED here
// exactly when this machine's nickname holds a claim; the root
// record's Goal-free declaration is the explicit no-work; queued
// unclaimed goals stay visible, not revivable.
func convertedOpenWork(repoRoot string) (OpenWork, string, error) {
	machine, err := goal.ResolveMachine(repoRoot)
	if err != nil {
		return WorkDegraded, fmt.Sprintf("no machine identity for the open-work judgment: %v", err), nil
	}
	endpoint, err := goal.ResolveEndpoint(repoRoot)
	if err != nil {
		return WorkDegraded, fmt.Sprintf("synced ledger endpoint unavailable: %v", err), nil
	}
	proj, err := goal.Project(endpoint, false, time.Now())
	if err != nil {
		return WorkDegraded, fmt.Sprintf("synced ledger unreadable: %v", err), nil
	}
	queued := 0
	for id, f := range proj.Tree.Live {
		if f.Claimed != nil && f.Claimed.Machine == machine {
			return WorkOwned, fmt.Sprintf("claimed goal: %s", id), nil
		}
		if f.State == "queued" {
			queued++
		}
	}
	if proj.Tree.Root != nil && proj.Tree.Root.Free != nil {
		return WorkNone, "goal-free declared", nil
	}
	if queued > 0 {
		return WorkNone, fmt.Sprintf("%d queued goals await a claim; none is owned here", queued), nil
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
	if ledger.Current != nil {
		return WorkOwned, fmt.Sprintf("current goal: %s", ledger.Current.Id), nil
	}
	if ledger.Free != nil {
		return WorkNone, "goal-free declared", nil
	}
	if len(ledger.Queued) > 0 {
		// Queued-but-unclaimed work is visible, not revivable: no
		// worker owns it, so the steward reports rather than spawns.
		return WorkNone, fmt.Sprintf("%d queued goals await a claim; none is owned", len(ledger.Queued)), nil
	}
	return WorkNone, "no current goal and no declaration", nil
}
