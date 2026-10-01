package main

// landing return MEMBER --reason TEXT (simple lane): one member leaves its
// batch and goes back to its seat through the existing settlement.
//
// Two askers, by who is proven:
//   - a person at an enrolled terminal: the person's word, admitted while
//     the lane is stopped;
//   - otherwise the landing agent, which decided the member broke the
//     batch or does not merge: its reason is all the return needs. The
//     caller must descend from the landing agent's launch, and a stopped
//     lane holds it.

import (
	"errors"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func landingReturnCommand() intentCommand {
	return intentCommand{
		object: "landing", action: "return", audience: "both", summary: "give one member of the landing lane back to its seat, with the reason",
		usage: []string{"metasystem landing return MEMBER --reason TEXT [--batch ID]"},
		details: []string{"The landing agent returns the member it found broke the batch or does not merge; its reason is recorded with the return.",
			"A person at an enrolled terminal returns one at their word, also while the lane is stopped.",
			"The member goes back to its seat, or is released when its seat is gone, and is read back from the ledger before the return is confirmed; when it is not, the same command continues it."},
		flags: []intentFlag{
			reasonFlag("because", "why, recorded with the return"),
			{name: "batch", value: "ID", advanced: true, usage: "the batch the member leaves (default: the one it waits in)"},
			{name: "by", value: "NAME", usage: "who acts (default: the enrolled person)"},
		},
		maxArgs:  1,
		examples: []string{"metasystem landing return verbs-match-intent --reason 'app-standard fails since it joined'"},
		run:      runIntentLandingReturn,
	}
}

func runIntentLandingReturn(inv *intentInvocation) int {
	owners, home, record, problem := inv.laneContext(true)
	var incomplete *lane.Refusal
	if problem != nil {
		// An older engine's lane record still names its checkout: a person
		// returns its members through it, each under the authority that
		// holds it.
		older, ok, err := lane.Read(home)
		if !ok || !errors.As(err, &incomplete) || incomplete.Code != lane.CodeRecordIncomplete || older.Root == "" {
			return inv.render(*problem)
		}
		record = older
	}
	targets := laneTargets(record.Root)
	if len(inv.input.args) != 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: targets,
			Summary: "landing return names one member and the reason it goes back; nothing was returned",
			next:    inv.publicArgv("landing", "return", "MEMBER", "--reason", "TEXT"), nextReason: "returns MEMBER"})
	}
	member := inv.input.args[0]
	disposition := batch.DispositionPerson
	person, personErr := owners.person(inv.proveAt())
	if personErr != nil {
		if problem != nil {
			refused := inv.personRefusal("", personErr, inv.input.text("by"))
			refused.Targets, refused.code = targets, 3
			refused.Summary = "only a person returns the members of an older lane, and " + strings.TrimSuffix(refused.Summary, ", so nothing was done") + "; nothing was returned"
			return inv.render(*refused)
		}
		if err := owners.agentCaller(record.Root); err != nil {
			return inv.render(intentResult{Outcome: intentRefused, code: 3, Targets: targets,
				Summary: "only the landing agent, or a person at an enrolled terminal, returns a member; nothing was returned",
				next:    inv.publicArgv("landing", "return", member, "--reason", "TEXT"), nextReason: "a person at an enrolled terminal returns it",
				Details: []string{"refused because: " + err.Error(), "the person's check: " + refusalCause(personErr)}})
		}
		person, disposition = "", batch.DispositionRed
	} else if named := strings.TrimPrefix(strings.TrimSpace(inv.input.text("by")), "human:"); named != "" && named != person {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets,
			Summary: "this terminal is enrolled for " + person + ", not " + named + ", so nothing was returned",
			next:    withoutOption(inv.typedArgv(), "by"), nextReason: "the enrolled name is filled in"})
	}
	installation, err := owners.installation(record.Root)
	if layout, layoutErr := record.Layout(); layoutErr == nil {
		installation, err = string(layout.Install), nil
	}
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets,
			Summary: "the landing lane's installation can't be found, so nothing was returned",
			next:    inv.publicArgv("landing", "status", "--verbose"), nextReason: "shows the lane's checkout",
			Details: []string{err.Error()}})
	}
	return inv.landingReturn(owners, home, record, installation, member, disposition, person)
}

// proveAt is where a person's proof is read: this seat's installation, else
// the folder the command runs in.
func (inv *intentInvocation) proveAt() string {
	if inv.resolveLayout() == nil {
		return inv.layout.InstallationRoot
	}
	return inv.cwd
}

func (inv *intentInvocation) landingReturn(owners laneVerbOwners, home string, record lane.Record, installation, member, disposition, person string) int {
	targets := append(laneTargets(record.Root), intentTarget{Kind: "goal", ID: member})
	actor := person
	if actor == "" {
		actor = lane.AccountID(record.Root)
	}
	report, err := owners.returnMember(batchowner.MemberReturn{Home: home, Checkout: record.Root, Install: installation,
		BatchID: strings.TrimSpace(inv.input.text("batch")), Member: member, Disposition: disposition, Person: person,
		Reason: strings.TrimSpace(inv.input.text("reason")), Actor: actor, Now: owners.now(), Calls: &batchowner.LaneCalls})
	var returnRefusal *batch.ReturnRefusal
	var laneRefusal *lane.Refusal
	switch {
	case errors.As(err, &returnRefusal):
		result := intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: report, Summary: returnRefusal.Message,
			next: inv.publicArgv("landing", "status", "--verbose"), nextReason: "shows the batch's proofs and members",
			Details: []string{"refused because: " + returnRefusal.Code}}
		if returnRefusal.Code == batch.CodeReturnEvidenceMissing {
			result.next, result.nextReason = inv.publicArgv("landing", "return", member, "--reason", "TEXT"), "names why it goes back"
			result.Details = append(result.Details, "a return names why the member goes back: metasystem landing return MEMBER --reason TEXT")
		}
		return inv.render(result)
	case errors.As(err, &laneRefusal):
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: report, Summary: laneRefusal.Message,
			next: laneRefusal.Argv, nextReason: laneRefusal.Fix, Details: []string{"refused because: " + laneRefusal.Code}})
	case err != nil:
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: report,
			Summary: "the return of " + member + " could not be made: " + oneLine(err.Error()), retry: "tries again", Details: []string{err.Error()}})
	}
	if !report.Confirmed {
		result := intentResult{Outcome: intentPartial, code: 1, Targets: targets, Data: report,
			Summary: "asked to return " + member + " from batch " + report.Batch + ", not confirmed yet: " + oneLine(report.Unresolved),
			retry:   "continues the return", Details: []string{"evidence: " + report.Evidence}}
		// An unresolved return that names a person's command (its line 2,
		// "run: ...") offers that command instead of a retry that cannot
		// settle it, such as the release of a goal the lane does not hold.
		if _, command, found := strings.Cut(report.Unresolved, "\nrun: "); found {
			result.retry, result.next, result.nextReason = "", strings.Fields(command), "settles it at a person's word"
		}
		return inv.render(result)
	}
	summary := "returned " + member + " from batch " + report.Batch + " to its seat (" + report.Evidence + ")"
	outcome := intentConfirmed
	if report.Repeat {
		outcome, summary = intentUnchanged, member+" is already returned from batch "+report.Batch+" ("+report.Evidence+")"
	}
	return inv.render(intentResult{Outcome: outcome, Targets: targets, Data: report, Summary: summary,
		view: landingDone(summary, record.Root)})
}

// laneAgentCaller proves this process descends from the landing agent's
// launch that holds the lane checkout (K7).
func laneAgentCaller(checkout string) error {
	return proveLaneOwnerCaller(checkout, int64(os.Getpid()))
}
