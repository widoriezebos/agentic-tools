// Package turn owns the vocabulary of a mission turn: the words the
// orchestrator, the runner, and the turn-prompt validator must all agree on.
// It holds no behavior and imports nothing — a shared vocabulary that lives
// in one of its consumers makes that consumer a dependency of everyone else
// who speaks it (the engine was imported wholesale for one table).
package turn

// The ask-reason vocabulary. An ask carries a reason class saying why a
// mission stopped for a human; two audiences read it, and they accept
// different sets.

// orchestratorAskReasons are the reason classes an ORCHESTRATOR may raise an
// ask with. Anything else is rejected and surfaces as a host-failure ask
// instead.
var orchestratorAskReasons = map[string]bool{
	"reserved-decision": true,
	"red-test":          true,
	"merge-conflict":    true,
	"host-failure":      true,
}

// runnerAskReasons are the reason classes only the RUNNER raises: a batched
// fence refusal, the budget park, and the drain-deadline park.
var runnerAskReasons = map[string]bool{
	"fence":          true,
	"stop-loss":      true,
	"drain-stalled":  true,
	"wall-violation": true,
}

// OrchestratorMayRaise reports whether an orchestrator may raise an ask with
// this reason class.
func OrchestratorMayRaise(reason string) bool { return orchestratorAskReasons[reason] }

// PromptMayCarry reports whether an open ask with this reason class may
// appear in a turn prompt: everything an orchestrator may raise, plus the
// runner's own. The turn-prompt validator must accept these, or the first
// runner-raised ask poisons every later prompt into refusal — the
// deterministic park that ended cohort bm-2s-20260810t195923z-80785.
func PromptMayCarry(reason string) bool {
	return orchestratorAskReasons[reason] || runnerAskReasons[reason]
}

// OrchestratorAskReasons lists the orchestrator-raisable classes, sorted, for
// the messages that must enumerate them.
func OrchestratorAskReasons() []string { return sortedKeys(orchestratorAskReasons) }

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
