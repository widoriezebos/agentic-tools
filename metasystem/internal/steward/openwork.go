package steward

// The open-work reader over the legacy single-file ledger — the
// format this repository runs today. The new per-goal format's
// reader joins when the backlog cutover lands; both feed the same
// OpenWork verdict. Degraded-honest: unreadable or unparseable
// state never means "no work".

import (
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// LegacyOpenWork reads plans/goals.md and answers whether delegated
// work is open on this machine. Pre-cutover the ledger is
// single-machine by standing rule, so a Current goal is this
// machine's work.
func LegacyOpenWork(repoRoot string) (OpenWork, string, error) {
	path := goal.LedgerPath(repoRoot)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// No ledger at all: nothing was ever delegated here.
		return WorkNone, "no goal ledger exists", nil
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
