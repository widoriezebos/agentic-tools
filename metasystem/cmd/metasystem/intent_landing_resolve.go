package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func landingResolveCommand() intentCommand {
	return laneCommand(intentCommand{
		object: "landing", action: "resolve", audience: "both", summary: "regenerate declared outputs in a conflicted merge, or return its source conflicts",
		usage:    []string{"metasystem landing resolve"},
		details:  []string{"Runs only in the registered landing checkout, before anyone edits the conflict. Main's generated files are rebuilt using the testing contract's argv, without a shell, and staged for the merge commit.", "A source conflict with main returns the goal for work rebase; a conflict with a batch member holds it until that member lands. A failed regeneration aborts and is classified: a lost process retries once, and a command that also fails before the merge holds for a question. Regeneration records never count as proof; commit the merge and run landing prove.", "An already resolved tree changes nothing. landing status shows the running command and its log size; landing stop ends it."},
		maxArgs:  0,
		examples: []string{"metasystem landing resolve"},
	}, runIntentLandingResolve)
}

func runIntentLandingResolve(inv *intentInvocation, admitted laneAdmitted) int {
	targets := laneTargets(admitted.record.Root)
	if !inv.insideLaneCheckout(admitted.record.Root) {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: "landing resolve runs in the registered landing checkout; nothing was changed", next: []string{"cd", admitted.record.Root}, nextReason: "then run metasystem landing resolve"})
	}
	if refused := inv.lanePaused(admitted, "resolved"); refused != nil {
		return inv.render(*refused)
	}
	// Main's declaration remains readable even when the testing contract is
	// itself a source conflict. Such a conflict must be returned, not edited.
	relative, _, err := config.CommittedLookup(filepath.Join(admitted.installation, "metasystem.conf"), "testing.contract")
	if err != nil {
		return inv.render(landingLaneFailure(targets, "the testing contract declaration cannot be read", err))
	}
	prefix, err := filepath.Rel(string(admitted.layout.Checkout), admitted.installation)
	if err != nil {
		return inv.render(landingLaneFailure(targets, "the lane installation cannot be placed", err))
	}
	seams := admitted.owners.plainResolve
	git := seams.Git
	if git == nil {
		git = plain.Git
	}
	data, err := git(string(admitted.layout.Checkout), "show", "HEAD:"+filepath.ToSlash(filepath.Join(prefix, relative)))
	if err != nil {
		return inv.render(landingLaneFailure(targets, "main's testing contract cannot be read", err))
	}
	contract, err := testpolicy.Decode([]byte(data))
	if err != nil {
		return inv.render(landingLaneFailure(targets, "main's testing contract is invalid", err))
	}
	out, err := plain.Resolve(admitted.home, admitted.installation, string(admitted.layout.Checkout), contract, seams)
	if out.Held && out.Goal == "" {
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: out, Summary: "the lane tree has no unresolved paths; nothing was changed"})
	}
	if out.Entry != nil && out.Entry.State == plain.StateWaiting {
		result := intentResult{Outcome: intentUnchanged, Targets: targets, Data: out,
			Summary: fmt.Sprintf("goal %s stays waiting: %s", out.Goal, out.Reason), next: inv.publicArgv("landing", "status")}
		if err != nil {
			result.Outcome, result.code, result.Details = intentFailed, 1, []string{err.Error()}
		}
		return inv.render(result)
	}
	if err != nil {
		if out.Entry != nil && out.Entry.State == plain.StateReturned {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: out,
				Summary: fmt.Sprintf("returned %s: %s", out.Goal, out.Entry.Reason), Details: append(inv.writeReturnedCard(out.Goal), err.Error()),
				next: inv.publicArgv("landing", "status"), nextReason: "shows the remaining hand-ins"})
		}
		result := landingLaneFailure(targets, "the lane merge could not be resolved: "+oneLine(err.Error()), err)
		result.Data = out
		return inv.render(result)
	}
	if out.Entry != nil {
		summary := fmt.Sprintf("returned %s: %s", out.Goal, out.Entry.Reason)
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: out, Summary: summary, Details: inv.writeReturnedCard(out.Goal), next: inv.publicArgv("landing", "status"), nextReason: "shows the remaining hand-ins"})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: out, Summary: "regenerated and staged the generated conflicts for " + out.Goal + "; commit the merge, then run landing prove", Details: []string{strings.Join([]string{"log", out.Log}, ": ")}})
}
