package main

import (
	"fmt"
	"strconv"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
)

// Unknown evidence holds both the round and the register. Collection can be
// recovered; the one reserved execution never turns into another allowance.
func (inv *intentInvocation) unknownDesignExamination(plan designReviewPlan, chain dispatchcore.DesignCritiqueChain, cause error) *intentResult {
	ruling := inv.publicArgv("design", "review", plan.design, "--ruling", "TEXT", "--reason", "TEXT", "--by", "NAME")
	scope := inv.publicArgv("design", "review", plan.design, "--scope", "FILE", "--reason", "TEXT", "--by", "NAME")
	root, err := inv.jobRecord(chain.Root)
	stop := loopstop.Stop{Loop: "design-round", Subject: chain.Root, Scope: plan.design, Tree: plan.subject,
		Attempt: int(recordInt(root, "criticRoundsConsumed")), Budget: int(inv.designRoundLimit(chain.Root)),
		Decision: "stop", Class: "unknown design evidence", Handoff: "stopped unreadable-design-evidence",
		Evidence: cause.Error(), At: inv.unitStopNow().UTC().Format("2006-01-02T15:04:05Z"),
		Required: []string{shellCommand(ruling), "or", shellCommand(scope)}}
	if err == nil {
		err = dispatchcore.RecordDesignEvidenceStop(inv.layout.InstallationRoot.Path(), chain.Root, stop)
	}
	result := &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: "the design evidence is unknown; the design round and register stay stopped",
		Data: stop, Details: []string{cause.Error()}, text: []string{shellCommand(ruling), shellCommand(scope)},
		next: ruling, nextReason: "a person rules on this candidate despite unknown advisory evidence; collection can also recover when the retained evidence is restored"}
	if err != nil {
		result.Details = append(result.Details, fmt.Sprintf("the stop could not be retained: %v", err))
	}
	if status := recordText(chain.Newest, "status"); recordText(root, "unknownExaminationRetryFrom") == "" && (status == "completed" || status == "failed") {
		result.next = inv.publicArgv("design", "review", plan.design, "--retry", strconv.FormatInt(chain.NewestRound, 10))
		result.nextReason = "first recover retained collection; otherwise reserve exactly one fresh examination of this same candidate"
	}
	return result
}
