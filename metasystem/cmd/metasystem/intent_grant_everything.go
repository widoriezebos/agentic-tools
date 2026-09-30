package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
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
	if strings.TrimSpace(inv.input.text("acts")) != goal.GeneralAct {
		return inv.refuse("", "everything stands alone in --acts; nothing was done", grantEndExamples)
	}
	if inv.input.has("tiers") {
		return inv.refuse("", "everything covers every tier: drop --tiers; nothing was done", grantEndExamples)
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	now, err := inv.owners.commandNow(inv.stateRoot)
	if err != nil {
		return inv.refuse("", "the clock is unreadable: "+err.Error()+"; nothing was done", "")
	}
	zone := inv.owners.helm.withDefaults().zone
	end, err := parseGrantEnd(now, zone, inv.input.text("for"), inv.input.text("until"))
	if err != nil {
		return inv.refuse("", err.Error()+"; nothing was done", grantEndExamples)
	}
	actor, proof, problem := inv.actingAs("grant", "", actorHuman)
	if problem != nil {
		return inv.render(*problem)
	}
	if proof == nil || proof.Helm != nil || !proof.EnrolledTerminalFor(inv.stateRoot) {
		if proof != nil {
			_ = humanauthority.RecordAttorneyRefusal(inv.stateRoot, *proof, "grant add", "a grant is added only by the person's own proof", now)
		}
		return inv.refuse("", "a general power of attorney is the person's own act at the enrolled terminal, never at the helm or under a grant; nothing was done",
			humanauthority.PersonActRemedy("metasystem grant add --acts everything --for 24h"))
	}
	owners := inv.owners.attorney.withDefaults()
	checkout, err := canonicalCheckout(inv.stateRoot)
	if err != nil {
		return inv.refuse("", "cannot name this checkout: "+err.Error()+"; nothing was done", "")
	}
	holder, err := owners.holder(checkout)
	if err != nil || holder.MainId == "" || holder.OwnerLineage == "" {
		reason := "no session holds this checkout's lease"
		if err != nil {
			reason += " (" + err.Error() + ")"
		}
		return inv.refuse("", reason+"; the grant is for the session that holds it; nothing was done", "start the seat's session first, then grant")
	}
	machine, err := inv.owners.dependencies.machine(inv.stateRoot)
	if err != nil {
		return inv.refuse("", "cannot name this machine: "+err.Error()+"; nothing was done", "")
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
		dependencies.complain("a general power of attorney is the person's own act at the enrolled terminal, never at the helm or under a grant: " + humanauthority.PersonActRemedy("metasystem grant add --acts everything --for 24h"))
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
			dependencies.complain("the general grant confirmed but could not record its authority proof:", err)
			return 1
		}
	}
	return dependencies.publishGrant(res, opid)
}

// generalGrantLine is one general grant as grant list shows it.
func generalGrantLine(entry goal.PowerOfAttorneyEntry, now time.Time, zone *time.Location) (string, bool) {
	until, _ := time.Parse(time.RFC3339, entry.Until)
	live, why := entry.LiveAt(now)
	state := "closed: " + why
	if live {
		state = shortDuration(until.Sub(now)) + " left"
	}
	return fmt.Sprintf("  %s  everything  by %s  for the main session of %s on %s  until %s  %s", entry.ID, strings.TrimPrefix(entry.By, "human:"),
		entry.Checkout, entry.For, until.In(zone).Format("15:04 MST (2006-01-02)"), state), live
}

// shortDuration is 23h12m, 5d3h or 42m.
func shortDuration(d time.Duration) string {
	d = d.Truncate(time.Minute)
	switch {
	case d >= 48*time.Hour:
		return fmt.Sprintf("%dd%dh", int(d/(24*time.Hour)), int(d%(24*time.Hour)/time.Hour))
	case d >= time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d/time.Hour), int(d%time.Hour/time.Minute))
	default:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
}

// attorneyStatusLine is the first line of status while a general grant for
// this checkout is live; empty otherwise, or when nothing can be read.
func (inv *intentInvocation) attorneyStatusLine(paths ...string) string {
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
		return ""
	}
	// The ledger is read where grant list reads it: at the state root. Its
	// files are read relative to the root given, so a template checkout's
	// repository top (status's path) finds no root record (F2).
	if inv.stateRoot != "" {
		first = inv.stateRoot
	}
	entries, err := owners.entries(first)
	if err != nil {
		return ""
	}
	now, err := inv.owners.commandNow(first)
	if err != nil {
		return ""
	}
	zone := inv.owners.helm.withDefaults().zone
	for _, entry := range entries {
		if !entry.General() {
			continue
		}
		if live, _ := entry.LiveAt(now); !live {
			continue
		}
		for _, candidate := range candidates {
			if entry.Checkout == filepath.Clean(candidate) {
				until, _ := time.Parse(time.RFC3339, entry.Until)
				return fmt.Sprintf("POWER OF ATTORNEY: %s acts for %s until %s (grant %s) — metasystem grant revoke %s ends it",
					entry.For, strings.TrimPrefix(entry.By, "human:"), until.In(zone).Format("15:04 MST (2006-01-02)"), entry.ID, entry.ID)
			}
		}
	}
	return ""
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
