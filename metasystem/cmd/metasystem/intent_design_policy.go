package main

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"math/big"
)

func (inv *intentInvocation) designEffectPolicy(plan designReviewPlan, chain dispatchcore.DesignCritiqueChain, kind string) *intentResult {
	policy, err := inv.unitRunner().ReviewPolicy()
	if err == nil && policy != "auto" && policy != "person" {
		cap, _ := new(big.Int).SetString(policy, 10)
		if cap.Cmp(big.NewInt(3)) < 0 {
			if err := dispatchcore.LimitDesignCorrections(inv.layout.InstallationRoot.Path(), chain.Root, int(cap.Int64())); err != nil {
				return inv.unknownDesignExamination(plan, chain, err)
			}
		}
	}
	if err == nil && (kind == "" || policy != "person" || inv.directPersonProof("design "+kind) == nil) {
		return nil
	}
	result := &intentResult{Targets: plan.targets, Outcome: intentInProgress, Summary: "the review policy holds the prepared design " + kind, next: inv.sameCommand(), nextReason: "its holder runs this prepared act; release that hold to resume automatically"}
	if err != nil {
		result.Details = []string{err.Error()}
		result.nextReason = "repair the review.stop setting, then run the same command to resume"
		return result
	}
	if plan.goalID != "" {
		_, err = channel.Ask(channel.AskRequest{RepoRoot: inv.layout.InstallationRoot.Path(), Goal: plan.goalID, Kind: "other", Facts: []string{"The review policy holds the prepared design " + kind + "."}, Recommendation: "The holder runs the prepared act.", UnitStop: &channel.UnitStopQuestion{Loop: "design-round", Subject: chain.Root, Attempt: int(chain.NewestRound), Finding: kind, Needs: shellCommand(inv.sameCommand()), AcceptableActs: []string{"design-" + kind, "design-ruling"}}, Now: inv.unitStopNow()})
		if err != nil {
			result.Details = []string{err.Error()}
		}
	}
	return result
}
func (inv *intentInvocation) recordDesignEffect(plan designReviewPlan, chain dispatchcore.DesignCritiqueChain, kind, operation string) error {
	if plan.goalID == "" {
		return nil
	}
	return channel.RecordUnitStopAct(inv.layout.InstallationRoot.Path(), channel.UnitStopAct{ID: operation + ":" + kind, Goal: plan.goalID, Loop: "design-round", Subject: chain.Root, Attempt: int(chain.NewestRound), Findings: []string{kind}, Kind: "design-" + kind, Reason: "the prepared design act succeeded", At: inv.unitStopNow()})
}
