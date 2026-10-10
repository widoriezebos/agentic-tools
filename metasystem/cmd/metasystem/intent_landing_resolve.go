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
		object: "landing", action: "resolve", audience: "both", summary: "take a merge conflict as the seat, or return it with its paths",
		usage:    []string{"metasystem landing resolve [--as-seat]"},
		flags:    []intentFlag{{name: "as-seat", usage: "take the conflict on the batch tree without aborting"}},
		details:  []string{"Runs only in the registered landing checkout, before anyone edits the conflict. With --as-seat, records resolving and its paths without aborting. Without it, aborts and returns the goal with its paths by class, including the stopped resolution job id.", "Run metasystem work rebase G on the goal branch, which regenerates what the testing contract declares, then hand in again. A conflict with a batch member holds the goal until that member lands.", "An already resolved tree changes nothing. Obsolete resolve-begun.json records are ignored and landing status names them as stale."},
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
	// HEAD's declaration remains readable when the contract itself conflicts.
	relative, _, err := config.CommittedLookup(filepath.Join(admitted.installation, "metasystem.conf"), "testing.contract")
	if err != nil {
		return inv.render(landingLaneFailure(targets, "the testing contract declaration cannot be read", err))
	}
	prefix, err := filepath.Rel(string(admitted.layout.Checkout), admitted.installation)
	if err != nil {
		return inv.render(landingLaneFailure(targets, "the lane installation cannot be placed", err))
	}
	seams := admitted.owners.plainResolve
	seams.AsSeat = inv.input.switched("as-seat")
	seams.Proof = inv.laneBatchSeams(admitted.home, admitted.record, admitted.owners.proveSeams(admitted.installation))
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
	defer plain.SyncPolicyQuestion(admitted.installation, admitted.owners.machine, admitted.owners.now())
	out, err := plain.Resolve(admitted.home, admitted.installation, string(admitted.layout.Checkout), contract, seams)
	if err == nil && out.Outcome == "resolving" {
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: out, Summary: fmt.Sprintf("Resolving %d conflicts of %s", len(out.Conflict.Paths), out.Goal), next: inv.publicArgv("landing", "status")})
	}
	if err == nil && out.Outcome == "abandoned" {
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: out, Summary: out.Reason, next: inv.publicArgv("landing", "status")})
	}
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
