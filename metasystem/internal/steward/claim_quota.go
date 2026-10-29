package steward

import (
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// RoleClaimQuota reports ready priority-1 work the one-claim rule holds back.
const RoleClaimQuota HealthRole = "claim-quota"

// checkClaimQuotaWith asks the claim's own quota query about every ready
// priority-1 goal for this machine. Without the lane reader (every preview)
// it shows the verdict the steward tick recorded.
func checkClaimQuotaWith(repoRoot, runRoot string, ledger *healthLedger) RoleVerdict {
	if ledger.laneState == nil {
		if record, err := loadHealthRecord(HealthRecordPath(repoRoot)); err == nil {
			for _, role := range record.Verdict.Roles {
				if role.Role == RoleClaimQuota {
					return role
				}
			}
		}
		return roleAlive(RoleClaimQuota, "the steward tick observes the claim quota")
	}
	unreadable := func(err error) RoleVerdict {
		return roleUnknown(RoleClaimQuota, "the claim quota cannot be read: "+err.Error(), "", RemedyFact{Cause: CauseUnreadable})
	}
	if !ledger.read().newWorld {
		return roleAlive(RoleClaimQuota, "the bootstrap ledger has no claims")
	}
	if ledger.endpointErr != nil {
		return unreadable(ledger.endpointErr)
	}
	if ledger.projectionErr != nil {
		return unreadable(ledger.projectionErr)
	}
	machine, err := goal.ResolveMachine(repoRoot)
	if err != nil {
		return unreadable(err)
	}
	next, err := goal.Next(ledger.projection, machine)
	if err != nil {
		return unreadable(err)
	}
	tree := ledger.projection.Tree
	laneState := func(id, main string) (string, error) { return ledger.laneState(repoRoot, id, main) }
	var parts, pairs []string
	var facts []RemedyFact
	for _, id := range next.Ready {
		if tree.Live[id].Priority != 1 {
			continue
		}
		block := goal.ClaimQuotaBlocker(tree, id, machine, ledger.projection.Tip, laneState)
		if block.Held == "" {
			continue
		}
		facts = append(facts, RemedyFact{Cause: CauseClaimQuota, Goal: block.Held, Ready: id, Command: block.Command})
		parts = append(parts, id+" (priority 1) is ready but "+block.Held+" holds this machine's one claim; "+block.Held+" "+
			claimActivity(runRoot, block.Held)+"; next: "+block.Command+", then metasystem goal claim "+id)
		pairs = append(pairs, id+">"+block.Held)
	}
	if len(facts) == 0 {
		return roleAlive(RoleClaimQuota, "no ready priority-1 goal waits behind this machine's claim")
	}
	sort.Strings(pairs)
	verdict := roleDead(RoleClaimQuota, strings.Join(parts, "; "), "", facts...)
	verdict.Cause = strings.Join(pairs, " ")
	return verdict
}

// claimActivity says whether the holding claim is working or waiting.
func claimActivity(runRoot, held string) string {
	jobs, err := goal.NonTerminalGoalJobs(runRoot, held)
	if err != nil {
		return "its jobs can't be read"
	}
	if len(jobs) > 0 {
		return "has a running job"
	}
	// Questions are run state, kept under the installation beside the jobs.
	for _, question := range channel.GoalOpenQuestions(runRoot) {
		if question.Goal == held {
			return "waits on an open question"
		}
	}
	return "has no job and no open question"
}
