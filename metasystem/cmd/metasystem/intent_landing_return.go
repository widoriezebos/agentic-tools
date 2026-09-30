package main

// landing return MEMBER --disposition (lane design r10 K8): one member
// leaves its batch with a typed disposition whose evidence the batch holds.
//
// Two paths, by who asks:
//   - the landing agent (red, conflict, seam-too-large): a kernel verb. The
//     engine identity is checked first (K5), the caller must descend from
//     the landing agent's launch (K7), and a pause holds it (K2);
//   - a person at an enrolled terminal (person): human-authorized cleanup.
//     It is admitted while the lane is paused and runs on whatever engine
//     the person's seat has, because a person is never denied a verb; the
//     engine identity guards what the agent does. A proven person may also
//     name an evidence disposition, which then goes the kernel's way.

import (
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func landingReturnCommand() intentCommand {
	return intentCommand{
		object: "landing", action: "return", audience: "both", summary: "give one member of a landing batch back to its seat, with the evidence its disposition needs",
		usage: []string{"metasystem landing return MEMBER --disposition red|conflict|seam-too-large|person [--reason TEXT] [--batch ID]"},
		details: []string{"red needs a proof of this batch that failed on the member; conflict and seam-too-large need what landing begin recorded when the member did not compose; a proof that could not run is never a member's failure.",
			"person is a person's word at an enrolled terminal: it needs no other evidence, and a stopped lane still takes it.",
			"The member goes back to its seat, or is released when its seat is gone, and is read back from the ledger before the return is confirmed; when it is not, the same command continues it."},
		flags: []intentFlag{
			{name: "disposition", value: "KIND", usage: "red, conflict, seam-too-large or person"},
			reasonFlag("because", "why, recorded with the return"),
			{name: "batch", value: "ID", advanced: true, usage: "the batch the member leaves (default: the one it waits in)"},
			{name: "by", value: "NAME", usage: "who acts (default: the enrolled person)"},
		},
		maxArgs:  1,
		examples: []string{"metasystem landing return verbs-match-intent --disposition person --reason 'the design changes first'"},
		run:      runIntentLandingReturn,
	}
}

func runIntentLandingReturn(inv *intentInvocation) int {
	disposition := strings.TrimSpace(inv.input.text("disposition"))
	if disposition == batch.DispositionPerson {
		return inv.personReturn()
	}
	kernel, problem := inv.admitLaneKernel()
	if problem != nil {
		return inv.render(*problem)
	}
	targets := laneTargets(kernel.record.Root)
	member, usage := inv.returnMember(targets, disposition)
	if usage != nil {
		return inv.render(*usage)
	}
	person, personErr := kernel.owners.person(inv.proveAt())
	if personErr != nil {
		person = ""
		if err := kernel.owners.agent(kernel.record.Root); err != nil {
			return inv.render(intentResult{Outcome: intentRefused, code: 3, Targets: targets,
				Summary: "only the landing agent, or a person at an enrolled terminal, returns a member with evidence; nothing was returned",
				next:    inv.publicArgv("landing", "return", member, "--disposition", batch.DispositionPerson), nextReason: "a person gives it back at their word",
				Details: []string{"refused because: " + err.Error(), "the person's check: " + refusalCause(personErr)}})
		}
	}
	return inv.landingReturn(kernel.owners, kernel.home, kernel.record, kernel.installation, member, disposition, person)
}

// personReturn is a person's return: proven at an enrolled terminal, no
// engine identity asked for.
func (inv *intentInvocation) personReturn() int {
	owners, home, record, problem := inv.laneContext(true)
	if problem != nil {
		return inv.render(*problem)
	}
	targets := laneTargets(record.Root)
	member, usage := inv.returnMember(targets, batch.DispositionPerson)
	if usage != nil {
		return inv.render(*usage)
	}
	person, err := owners.person(inv.proveAt())
	if err != nil {
		refused := inv.personRefusal("", err, inv.input.text("by"))
		refused.Targets, refused.code = targets, 3
		refused.Summary = "only a person gives a member back at their word, and " + strings.TrimSuffix(refused.Summary, ", so nothing was done") + "; nothing was returned"
		return inv.render(*refused)
	}
	if named := strings.TrimSpace(inv.input.text("by")); named != "" {
		person = named
	}
	installation, err := owners.installation(record.Root)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets,
			Summary: "the landing lane's installation can't be found, so nothing was returned",
			next:    inv.publicArgv("landing", "status", "--verbose"), nextReason: "shows the lane's checkout",
			Details: []string{err.Error()}})
	}
	return inv.landingReturn(owners, home, record, installation, member, batch.DispositionPerson, person)
}

// proveAt is where a person's proof is read: this seat's installation, else
// the folder the command runs in.
func (inv *intentInvocation) proveAt() string {
	if inv.resolveLayout() == nil {
		return inv.layout.InstallationRoot
	}
	return inv.cwd
}

// returnMember reads the one member and a disposition the lane knows.
func (inv *intentInvocation) returnMember(targets []intentTarget, disposition string) (string, *intentResult) {
	if len(inv.input.args) != 1 || !slices.Contains(batch.Dispositions, disposition) {
		return "", &intentResult{Outcome: intentRefused, code: 2, Targets: targets,
			Summary: "landing return names one member and how it leaves: red, conflict, seam-too-large or person; nothing was returned",
			next:    inv.publicArgv("landing", "return", "MEMBER", "--disposition", batch.DispositionPerson), nextReason: "a person gives MEMBER back at their word"}
	}
	return inv.input.args[0], nil
}

func (inv *intentInvocation) landingReturn(owners laneVerbOwners, home string, record lane.Record, installation, member, disposition, person string) int {
	targets := append(laneTargets(record.Root), intentTarget{Kind: "goal", ID: member})
	actor := person
	if actor == "" {
		actor = lane.AccountID(record.Root)
	}
	report, err := owners.returnMember(batchowner.MemberReturn{Home: home, Checkout: record.Root, Install: installation,
		BatchID: strings.TrimSpace(inv.input.text("batch")), Member: member, Disposition: disposition, Person: person,
		Reason: strings.TrimSpace(inv.input.text("reason")), Actor: actor, Now: owners.now(), Calls: &batchowner.BatchOwnerCalls})
	var returnRefusal *batch.ReturnRefusal
	var laneRefusal *lane.Refusal
	switch {
	case errors.As(err, &returnRefusal):
		result := intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: report, Summary: returnRefusal.Message,
			next: inv.publicArgv("landing", "status", "--verbose"), nextReason: "shows the batch's proofs and members",
			Details: []string{"refused because: " + returnRefusal.Code}}
		if returnRefusal.Code == batch.CodeReturnEvidenceMissing && disposition != batch.DispositionPerson {
			result.next, result.nextReason = inv.publicArgv("landing", "return", member, "--disposition", batch.DispositionPerson), "a person gives it back at their word"
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
		return inv.render(intentResult{Outcome: intentPartial, code: 1, Targets: targets, Data: report,
			Summary: "asked to return " + member + " from batch " + report.Batch + " as " + disposition + ", not confirmed yet: " + oneLine(report.Unresolved),
			retry:   "continues the return", Details: []string{"evidence: " + report.Evidence}})
	}
	summary := "returned " + member + " from batch " + report.Batch + " as " + disposition + " (" + report.Evidence + ")"
	outcome := intentConfirmed
	if report.Repeat {
		outcome, summary = intentUnchanged, member+" is already returned from batch "+report.Batch+" as "+disposition+" ("+report.Evidence+")"
	}
	return inv.render(intentResult{Outcome: outcome, Targets: targets, Data: report, Summary: summary,
		view: landingDone(summary, record.Root)})
}

// laneAgentCaller proves this process descends from the landing agent's
// launch that holds the lane checkout (K7).
func laneAgentCaller(checkout string) error {
	return proveLaneOwnerCaller(checkout, int64(os.Getpid()))
}
