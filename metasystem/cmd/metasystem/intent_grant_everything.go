package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
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
		cause := humanauthority.Refused(humanauthority.OutcomeTerminalMissing, nil)
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
	for _, path := range append(paths, inv.stateRoot, inv.layout.InstallationRoot.Path()) {
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
	// The ledger is read where grant list reads it: at the state root. Its
	// files are read relative to the root given, so a template checkout's
	// repository top (status's path) finds no root record (F2).
	if inv.stateRoot != "" {
		first = inv.stateRoot
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

// handoffPartnerActor carries the grant's person to owners that take --by
// when the caller left it out.
func (inv *intentInvocation) handoffPartnerActor(args []string) ([]string, *intentResult) {
	grant, problem := inv.partnerActor("")
	if problem != nil || grant == nil || inv.input.has("by") {
		return args, problem
	}
	for _, flag := range inv.command.flags {
		if flag.name == "by" {
			inv.input.values["by"] = []string{grant.Helm.By}
			return append(args, "--by", grant.Helm.By), nil
		}
	}
	return args, nil
}

// partnerActor is the partner's actor selection: the partner's session
// lineage has no ordinary agent route, whatever checkout an act targets.
// The grant remains the only source of the person's authority.
func (inv *intentInvocation) partnerActor(target string) (*humanauthority.Proof, *intentResult) {
	lineage := inv.input.text("lineage")
	if owner := inv.owners.dependencies.ownerLineage; owner != nil && owner() == "project-partner" {
		lineage = owner()
	}
	guarded, drivesWork := inv.partnerForm()
	if lineage != "project-partner" || !guarded {
		return nil, nil
	}
	for _, name := range []string{"root", "installation", "repo"} {
		if value, given, _ := takeIntentFlag(inv.raw, name, true); given && value == "" {
			return nil, &intentResult{Outcome: intentRefused, code: 1,
				Summary: "an empty --" + name + " names no checkout; nothing was done"}
		}
	}
	if drivesWork {
		return nil, &intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(target),
			Summary: "the project partner never drives a build; a seat does; nothing was done"}
	}
	if inv.command.name == "work wait" && len(inv.input.args) == 1 && !inv.input.has("for") && !inv.input.has("path") && !inv.input.switched("list") && !inv.input.switched("exit-code") {
		if strings.HasPrefix(inv.input.args[0], "run:") {
			refused := inv.waitUnit("", 0, inv.targets(inv.input.args[0]), nil)
			return nil, &refused
		}
		if !strings.Contains(inv.input.args[0], ":") {
			work, _ := inv.goalWork(inv.input.args[0])
			work, _ = inv.namedWorkOnly(inv.input.args[0], work)
			for _, one := range work {
				if one.Running() || inv.input.has("work") {
					refused := inv.waitUnit("", 0, workTargets(inv.input.args[0], one), nil)
					return nil, &refused
				}
			}
		}
	}
	homeRefusal := &intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(target),
		Summary: inv.command.name + " is the person's act through the partner; the partner acts only from its own checkout in this step (requested checkout: " + inv.stateRoot + "); nothing was done"}
	home, err := inv.owners.resolver.ResolveLayout(inv.cwd)
	if err != nil {
		return nil, homeRefusal
	}
	homeRoot := home.GitRoot
	if home.Template {
		homeRoot = home.InstallationRoot.Path()
	}
	homeCheckout, homeErr := canonicalCheckout(homeRoot)
	checkout, checkoutErr := canonicalCheckout(inv.stateRoot)
	if homeErr != nil || checkoutErr != nil || homeCheckout != checkout {
		return nil, homeRefusal
	}
	ledger := inv.owners.dependencies.authorityFacts.ledgerIdentity
	if ledger == nil {
		ledger = goal.ExistingLedgerIdentity
	}
	state := brain.Read(inv.stateRoot, ledger(inv.stateRoot))
	if state.State != brain.Declared || state.Record.Role != brain.Partner {
		return nil, homeRefusal
	}
	owners := inv.owners.attorney.withDefaults()
	now, clockErr := inv.owners.commandNow(inv.stateRoot)
	entries, readErr := owners.entries(inv.stateRoot)
	var entry goal.PowerOfAttorneyEntry
	for _, candidate := range entries {
		if candidate.General() && candidate.Checkout == checkout && candidate.For == state.Record.Machine && candidate.Lineage == lineage {
			entry = candidate
		}
	}
	reason := "the grant does not admit this session"
	if readErr != nil || clockErr != nil {
		reason += " (the grant or clock cannot be read)"
	} else if live, why := entry.LiveAt(now); entry.ID != "" && !live {
		if entry.Revoked != "" {
			at, _ := time.Parse(time.RFC3339, entry.Revoked)
			why = "revoked " + at.In(inv.owners.helm.withDefaults().zone).Format("15:04") + " by " + strings.TrimPrefix(entry.RevokedBy, "human:")
		}
		reason += " (" + why + ")"
	} else if strings.TrimSpace(inv.input.text("impact")) == "" {
		reason = "--impact is missing"
	} else if grant, admitted := owners.admit(inv.stateRoot, int64(os.Getppid()), now); admitted && grant.Grant != "" && grant.By != "" {
		if typed := strings.TrimPrefix(inv.input.text("by"), "human:"); typed == "" || typed == grant.By {
			if proof, err := humanauthority.HelmProof(inv.stateRoot, grant, now); err == nil {
				inv.owners.dependencies.partnerGrantAdmitted = true
				return &proof, nil
			}
		} else {
			reason = "the named person differs from the grant's person"
		}
	}
	logGrant := entry.ID
	if logGrant == "" {
		logGrant = "none"
	}
	if err := humanauthority.RecordAttorneyRefusal(inv.stateRoot, humanauthority.Proof{Helm: &humanauthority.HelmGrant{Grant: logGrant, By: strings.TrimPrefix(entry.By, "human:")}}, inv.command.name, reason, now, inv.input.text("impact")); err != nil {
		reason += "; the refusal could not be logged: " + err.Error()
	}
	result := &intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(target),
		Summary: inv.command.name + " is the person's act through the partner; " + reason + "; nothing was done",
		next:    []string{"metasystem", "partner", "start"}, nextReason: "at the person's own terminal"}
	if reason == "--impact is missing" {
		result.next = inv.retryWith([]string{"impact"}, "--impact", "WHY; undo: HOW")
		result.nextReason = "state the impact first"
	}
	return nil, result
}

func (inv *intentInvocation) partnerCaller() bool {
	return inv.input.text("lineage") == "project-partner" || inv.owners.dependencies.ownerLineage != nil && inv.owners.dependencies.ownerLineage() == "project-partner"
}

// partnerMutation selects acts before their owners run, including those
// whose owner chooses its actor outside actingAs. Read forms need no grant.
func (inv *intentInvocation) partnerMutation() bool {
	guarded, _ := inv.partnerForm()
	return guarded
}

// partnerForm keeps observation, guarded acts and forbidden work together.
func (inv *intentInvocation) partnerForm() (guarded, drivesWork bool) {
	owner := inv.owners.dependencies.ownerLineage
	if inv.input.text("lineage") != "project-partner" && (owner == nil || owner() != "project-partner") {
		return false, false
	}
	observedWait := inv.input.has("path") || inv.input.switched("list") || inv.input.text("for") == "landing" || inv.input.text("for") == "human-act"
	if inv.command.name == "work wait" && len(inv.input.args) == 1 {
		ref := inv.input.args[0]
		observedWait = observedWait || strings.HasPrefix(ref, "j1:") || strings.HasPrefix(ref, "j2:")
		if id, resume := strings.CutPrefix(ref, "wait:"); resume && inv.selectRoot() == nil {
			row, _, err := metarun.FindWaiterByID(inv.stateRoot, id)
			s := row.Selector
			observedWait = err == nil && (s.Kind == "path" || s.Kind == "job" || s.Kind == "goal" && (s.Event == "landing" || s.Event == "human-act"))
		}
	}
	// Only these forms may bypass actor selection. A new command is guarded
	// until its observation or session-bookkeeping form is listed here.
	forms := []struct {
		names      string
		read       bool
		drivesWork bool
	}{
		{"status, goal list, goal show, grant list, decision list, decision show, design show, design list", true, false},
		{"work status, test plan, test list, test status, question show, question list, agent inbox, incident list", true, false},
		{"helm status, mission status, system status, system completion, landing status, alert list, machine list", true, false},
		{"disk show, evidence show, app status, app log, ui status, settings show, settings keys, receipt status, experiment status", true, false},
		{"design check, system check, settings check, experiment check", true, false},
		{"goal notes", !inv.input.has("add") && !inv.input.has("add-file") && !inv.input.has("close"), false},
		{"goal budget", !inv.input.has("budget") && len(inv.input.args) <= 1, false},
		{"goal sync", !inv.input.has("publish") && !inv.input.has("recover") && !inv.input.has("refresh") && !inv.input.has("upgrade") && !inv.input.has("accept-remote-history"), false},
		{"test baseline", inv.input.switched("check"), false},
		{"settings coordinator", !inv.input.has("declare") && !inv.input.has("withdraw"), false},
		{"session start, session stop, session status, session wait, session handoff, question wait", true, false},
		{"work wait", observedWait, false},
		{"work build, work revise, work review, work land, work finish", false, true},
	}
	for _, form := range forms {
		for _, name := range strings.Split(form.names, ", ") {
			if inv.command.name == name {
				return !form.read, form.drivesWork
			}
		}
	}
	return true, false
}
