package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// attorneyIntentOwners are the general grant's seams in the public verbs;
// zero values are production.
type attorneyIntentOwners struct {
	// entries reads the accepted ledger's power-of-attorney entries.
	entries func(root string) ([]goal.PowerOfAttorneyEntry, error)
	// holder reads the checkout's lease holder at the grant.
	holder func(root string) (lease.CurrentHolderView, error)
	// admit is the grant's admission for actor selection (M6); nil is
	// humanauthority.AtAttorney as wired at startup.
	admit func(root string, pid int64, now time.Time) (humanauthority.HelmGrant, bool)
	// exclusive takes the grant lock for a local revoke.
	exclusive func(root string, wait time.Duration) (bool, error)
}

func (o attorneyIntentOwners) withDefaults() attorneyIntentOwners {
	if o.entries == nil {
		o.entries = acceptedAttorneyEntries
	}
	if o.holder == nil {
		o.holder = lease.CurrentHolder
	}
	if o.admit == nil {
		o.admit = func(root string, pid int64, now time.Time) (humanauthority.HelmGrant, bool) {
			if humanauthority.AtAttorney == nil {
				return humanauthority.HelmGrant{}, false
			}
			return humanauthority.AtAttorney(root, pid, now)
		}
	}
	if o.exclusive == nil {
		o.exclusive = func(root string, wait time.Duration) (bool, error) {
			return processGrantLock.exclusive(root, wait, time.Sleep)
		}
	}
	return o
}

// revokeLockWait bounds how long a local revoke waits for acts admitted
// under a grant to finish before it publishes anyway.
const revokeLockWait = 30 * time.Second

// isGeneralActs reports whether --acts names the general grant at all.
func isGeneralActs(acts string) bool {
	for _, act := range strings.Split(acts, ",") {
		if strings.TrimSpace(act) == goal.GeneralAct {
			return true
		}
	}
	return false
}

// runIntentGrantEverything records a general power of attorney: the main
// session holding this checkout's lease acts for the person until the end.
func runIntentGrantEverything(inv *intentInvocation) int {
	retry := inv.typedArgv()
	if strings.TrimSpace(inv.input.text("acts")) != goal.GeneralAct {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--acts everything takes no other act beside it; nothing was done",
			next: append(withoutOption(retry, "acts"), "--acts", goal.GeneralAct), nextReason: "everything already covers every act"})
	}
	if inv.input.has("tiers") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--acts everything covers every tier and takes no --tiers; nothing was done",
			next: withoutOption(retry, "tiers")})
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	now, err := inv.owners.commandNow(inv.stateRoot)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "the clock can't be read, so nothing was done",
			next: retry, nextReason: "try again", Details: []string{err.Error()}})
	}
	zone := inv.owners.helm.withDefaults().zone
	end, err := parseGrantEnd(now, zone, inv.input.text("for"), inv.input.text("until"))
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: err.Error() + ", so nothing was done",
			next: append(withoutOption(withoutOption(retry, "for"), "until"), "--for", "24h"), nextReason: grantEndOthers})
	}
	actor, proof, problem := inv.actingAs("grant", "", actorHuman)
	if problem != nil {
		return inv.render(*problem)
	}
	if proof == nil || proof.Helm != nil || !proof.EnrolledTerminalFor(inv.stateRoot) {
		cause := error(errors.New(humanauthority.OutcomeTerminalMissing))
		if proof != nil {
			_ = humanauthority.RecordAttorneyRefusal(inv.stateRoot, *proof, "grant add", "a grant is added only by the person's own proof", now)
			if walk := proof.WalkRefusal(); walk != nil {
				cause = walk
			}
		}
		refusal := inv.personRefusal("", cause, "")
		refusal.code = 2
		refusal.Details = append(refusal.Details, "a general grant is only ever the person's own act; the helm and other grants never stand in for it")
		return inv.render(*refusal)
	}
	owners := inv.owners.attorney.withDefaults()
	checkout, err := canonicalCheckout(inv.stateRoot)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "this checkout's path can't be resolved, so nothing was done",
			next: retry, nextReason: "try again", Details: []string{err.Error()}})
	}
	holder, err := owners.holder(checkout)
	if err != nil || holder.MainId == "" || holder.OwnerLineage == "" {
		refusal := intentResult{Outcome: intentRefused, code: 2, Summary: "no session holds this checkout's lease, so there is no one to grant to; nothing was done",
			Decision: "start the agent session in this checkout, then run " + shellCommand(retry) + " again"}
		if err != nil {
			refusal.Details = []string{err.Error()}
		}
		return inv.render(refusal)
	}
	machine, err := inv.owners.dependencies.machine(inv.stateRoot)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "this machine's name can't be read, so nothing was done",
			Decision: "metasystem machine list shows this machine's name; then run " + shellCommand(retry) + " again", Details: []string{err.Error()}})
	}
	grant := goal.GeneralGrant{Machine: machine, Checkout: checkout, Lineage: holder.OwnerLineage, Until: end}
	args := append([]string{"--root", inv.stateRoot}, actor...)
	report := &ownerReport{}
	dependencies := inv.owners.dependencies
	dependencies.report = report
	code := runGoalGrantGeneralWithInputs(args, grant, inv.owners.prove, inv.owners.commandNow, dependencies)
	local := end.In(zone).Format("15:04 MST (2006-01-02)")
	by := strings.TrimPrefix(actorValue(actor, "--by"), "human:")
	result := ownerResult(report, code, intentResult{
		Summary: fmt.Sprintf("granted %s: the main session of %s on %s acts for %s until %s", report.entry, checkout, machine, by, local),
		Data:    map[string]any{"grant": report.entry, "acts": []string{goal.GeneralAct}, "for": machine, "checkout": checkout, "lineage": holder.OwnerLineage, "until": end.UTC().Format(time.RFC3339)}})
	if report.entry != "" {
		result.Targets = []intentTarget{{Kind: "grant", ID: report.entry}}
	}
	if result.Outcome == intentConfirmed {
		result.text = append(result.text, "metasystem grant revoke "+report.entry+" ends it")
	}
	return inv.render(result)
}

// actorValue is the value after flag in a forwarded argument list.
func actorValue(args []string, flag string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}

// runGoalGrantGeneralWithInputs is the general grant's owner call: the
// person's proof at the enrolled terminal again, then goal.GrantGeneral.
func runGoalGrantGeneralWithInputs(args []string, grant goal.GeneralGrant, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	f, ok := dependencies.parseSyncFlags("grant", args)
	if !ok {
		return 2
	}
	if !converted(f.root) || f.by == "" {
		dependencies.complain("a general grant needs a synced backlog plus --by")
		return 2
	}
	classification, err := classifyGoalAuthorityFirstWithFacts("grant", f, dependencies.authorityFacts)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	proof, err := proveGoalHumanAuthorityAt("grant", f, prove, commandNow)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	if proof.Helm != nil || !proof.EnrolledTerminalFor(f.root) {
		dependencies.complain("a general grant is added only at your own enrolled terminal, so nothing was done")
		return 1
	}
	req, err := syncReqClassifiedWithTerminalGradeAtWithDependencies(f.root, f.by, f.lineage, &proof, classification, false, commandNow, dependencies)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	res, err := goal.GrantGeneral(req, &proof, grant)
	dependencies.landed(res)
	if err != nil {
		return dependencies.publish(res, err)
	}
	opid := goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage)
	if res.Outcome == goal.OutcomeConfirmed {
		if err := recordGoalApprovalProof(f.root, opid, "goal grant", proof); err != nil {
			dependencies.complain("the grant is recorded, but the record of who granted it could not be written:", err)
			return 1
		}
	}
	return dependencies.publishGrant(res, opid)
}

// attorneyStatusLine is the first line of status while a general grant for
// this checkout is live; empty otherwise, or when nothing can be read.
func (inv *intentInvocation) attorneyStatusLine(paths ...string) string {
	entry, live := inv.liveGeneralGrant(paths...)
	if !live {
		return ""
	}
	return attorneyLine(entry, inv.owners.helm.withDefaults().zone)
}

// attorneyLine is a live general grant as --json's status carries it.
func attorneyLine(entry goal.PowerOfAttorneyEntry, zone *time.Location) string {
	until, _ := time.Parse(time.RFC3339, entry.Until)
	return fmt.Sprintf("POWER OF ATTORNEY: %s acts for %s until %s (grant %s) — metasystem grant revoke %s ends it",
		entry.For, strings.TrimPrefix(entry.By, "human:"), until.In(zone).Format("15:04 MST (2006-01-02)"), entry.ID, entry.ID)
}

// grantAttention is a live general grant in the banner: informational,
// since it widens what the seat may do rather than asking anything.
func grantAttention(entry goal.PowerOfAttorneyEntry, env textui.Env) textui.Attention {
	until, _ := time.Parse(time.RFC3339, entry.Until)
	return textui.Attention{State: textui.Live, Text: fmt.Sprintf("%s's grant is live: everything, %s", strings.TrimPrefix(entry.By, "human:"), env.Until(until)),
		Detail: []string{"for the main session of " + entry.For + " · grant " + entry.ID}}
}

// liveGeneralGrant is the live general grant for this checkout, when one
// can be read.
func (inv *intentInvocation) liveGeneralGrant(paths ...string) (goal.PowerOfAttorneyEntry, bool) {
	owners := inv.owners.attorney.withDefaults()
	var candidates []string
	first := ""
	for _, path := range append(paths, inv.stateRoot, inv.layout.InstallationRoot) {
		if path == "" {
			continue
		}
		if canonical, err := canonicalCheckout(path); err == nil {
			candidates = append(candidates, canonical)
			if first == "" {
				first = path
			}
		}
	}
	if len(candidates) == 0 || inv.owners.commandNow == nil {
		return goal.PowerOfAttorneyEntry{}, false
	}
	entries, err := owners.entries(first)
	if err != nil {
		return goal.PowerOfAttorneyEntry{}, false
	}
	now, err := inv.owners.commandNow(first)
	if err != nil {
		return goal.PowerOfAttorneyEntry{}, false
	}
	for _, entry := range entries {
		if !entry.General() {
			continue
		}
		if live, _ := entry.LiveAt(now); !live {
			continue
		}
		for _, candidate := range candidates {
			if entry.Checkout == filepath.Clean(candidate) {
				return entry, true
			}
		}
	}
	return goal.PowerOfAttorneyEntry{}, false
}

// attorneyActor is M6: before an agent's lineage shortcut, a live grant
// that admits this caller makes the act the granting person's.
func (inv *intentInvocation) attorneyActor() (string, bool) {
	now, err := inv.owners.commandNow(inv.stateRoot)
	if err != nil {
		return "", false
	}
	grant, ok := inv.owners.attorney.withDefaults().admit(inv.stateRoot, int64(os.Getppid()), now)
	if !ok || grant.Grant == "" || strings.TrimSpace(grant.By) == "" {
		return "", false
	}
	return grant.By, true
}
