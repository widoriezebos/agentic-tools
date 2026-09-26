package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

// Administrative operations name one explicit human choice and hand it to
// the engine verb that owns it, as its own process in the installation, so
// the owner's caller classification, human proof, digests and refusal rules
// apply unchanged. The public result reports the owner's outcome and output;
// it never promotes a named person to a proven one.

// engineVerb runs one owner verb of the selected installation's engine.
func (inv *intentInvocation) engineVerb(args ...string) (intentProcessResult, *intentResult) {
	binary, err := inv.delivery().executable()
	if err != nil {
		return intentProcessResult{}, &intentResult{Outcome: intentFailed, code: 1, Summary: "the engine executable is unavailable: " + err.Error() + "; nothing was done"}
	}
	return inv.delivery().process(intentProcess{argv: append([]string{binary}, args...), dir: inv.layout.InstallationRoot}), nil
}

// ownerVerbResult is the public outcome of one owner verb run: its structured
// output when it printed one, its refusal text otherwise.
func ownerVerbResult(ran intentProcessResult, targets []intentTarget, done string, data map[string]any) intentResult {
	if data == nil {
		data = map[string]any{}
	}
	output := strings.TrimSpace(string(ran.stdout))
	var parsed any
	if json.Unmarshal([]byte(output), &parsed) == nil {
		data["owner"] = parsed
	} else if output != "" {
		data["owner"] = nonEmptyLines(output)
	}
	data["exitCode"] = ran.code
	if ran.code == 0 && ran.err == nil {
		return intentResult{Outcome: intentConfirmed, Targets: targets, Data: data, Summary: done, text: nonEmptyLines(output)}
	}
	problem := nonEmptyLines(string(ran.stderr))
	summary := fmt.Sprintf("the owner exited %d without a reason; its output is under owner", ran.code)
	if reason := ownerRejectionReason(parsed); reason != "" {
		summary = reason
	} else if len(problem) > 0 {
		summary = problem[0]
	} else if lines := nonEmptyLines(output); parsed == nil && len(lines) > 0 {
		summary = lines[len(lines)-1]
	}
	if ran.err != nil {
		summary = "the owner could not run: " + ran.err.Error()
	}
	return intentResult{Outcome: intentRefused, Targets: targets, Data: data, code: max(ran.code, 1), Summary: summary, text: problem}
}

// ownerRejectionReason is the reason an owner's own JSON result gives for
// its refusal, read from the fields owners use for it, top level first and
// then a nested refusal object.
func ownerRejectionReason(parsed any) string {
	object, ok := parsed.(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range []string{"reason", "refusal", "why", "detail", "message", "error", "summary"} {
		switch value := object[key].(type) {
		case string:
			if text := strings.TrimSpace(value); text != "" {
				return text
			}
		case map[string]any:
			if reason := ownerRejectionReason(value); reason != "" {
				return reason
			}
		}
	}
	return ""
}

// exclusiveChoice returns the one choice given among names, or a refusal
// when several are.
func (inv *intentInvocation) exclusiveChoice(names ...string) (string, *intentResult) {
	var given []string
	for _, name := range names {
		if inv.input.has(name) {
			given = append(given, "--"+name)
		}
	}
	if len(given) > 1 {
		return "", &intentResult{Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("%s are separate decisions; give one; nothing was done", strings.Join(given, " and "))}
	}
	if len(given) == 1 {
		return strings.TrimPrefix(given[0], "--"), nil
	}
	return "", nil
}

func (inv *intentInvocation) requireInputs(names ...string) *intentResult {
	var missing []string
	for _, name := range names {
		if strings.TrimSpace(inv.input.text(name)) == "" {
			missing = append(missing, "--"+name)
		}
	}
	if len(missing) > 0 {
		return &intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("this choice needs %s; nothing was done", strings.Join(missing, ", "))}
	}
	return nil
}

// refuseOthers refuses options that belong to other choices.
func (inv *intentInvocation) refuseOthers(allowed []string, names ...string) *intentResult {
	for _, name := range names {
		if inv.input.has(name) && !containsIntentValue(allowed, name) {
			return &intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--%s does not belong to this choice; nothing was done", name)}
		}
	}
	return nil
}

var (
	intentSHA256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	intentTreePattern   = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
)

var repairGoalsOptions = []string{"accept-edits", "refresh", "upgrade", "accept-remote-history", "source-digest", "amendments", "identity", "sync-mode", "by"}

// runIntentRepairGoals is repair goals and its explicit human choices.
func runIntentRepairGoals(inv *intentInvocation) int {
	choice, problem := inv.exclusiveChoice("accept-edits", "refresh", "upgrade", "accept-remote-history")
	if problem == nil {
		switch choice {
		case "":
			problem = inv.refuseOthers(nil, repairGoalsOptions...)
		case "accept-edits", "accept-remote-history":
			if problem = inv.refuseOthers([]string{choice, "by"}, repairGoalsOptions...); problem == nil {
				problem = inv.requireInputs("by")
			}
		case "refresh":
			problem = inv.refuseOthers([]string{"refresh"}, repairGoalsOptions...)
		case "upgrade":
			if problem = inv.refuseOthers([]string{"upgrade", "source-digest", "amendments", "identity", "sync-mode", "by"}, repairGoalsOptions...); problem == nil {
				problem = inv.requireInputs("by")
			}
			if mode := inv.input.text("sync-mode"); problem == nil && mode != "" && mode != "remote" && mode != "local" {
				problem = &intentResult{Outcome: intentRefused, code: 2, Summary: "--sync-mode is remote or local; nothing was done"}
			}
			if digest := inv.input.text("source-digest"); problem == nil && digest != "" && !intentSHA256Pattern.MatchString(digest) {
				problem = &intentResult{Outcome: intentRefused, code: 2, Summary: "--source-digest is the reviewed file's 64-character lowercase SHA-256; nothing was done"}
			}
		}
	}
	if problem != nil {
		return inv.render(*problem)
	}
	if choice == "upgrade" {
		// Upgrading reads the legacy ledger, which a synced installation no
		// longer requires; only the installation is resolved.
		if problem := inv.resolveLayout(); problem != nil {
			return inv.render(*problem)
		}
		inv.stateRoot, _ = inv.owners.resolver.RootForInstallation(inv.layout.InstallationRoot)
	} else if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	targets := []intentTarget{{Kind: "installation", ID: inv.layout.InstallationRoot}}
	scope := map[string]any{"scope": "installation", "stateRoot": inv.stateRoot}
	switch choice {
	case "":
		reports, err := recoverGoalJournal(inv.stateRoot, inv.owners.commandNow, inv.owners.dependencies)
		if err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: scope,
				Summary: "the goal journal of this installation was not recovered: " + err.Error(), next: inv.publicArgv("check"), nextReason: "diagnose what stops recovery"})
		}
		entries, lines := []map[string]string{}, []string{}
		for _, report := range reports {
			entries = append(entries, map[string]string{"opid": report.Opid, "action": string(report.Action), "detail": report.Detail})
			lines = append(lines, fmt.Sprintf("%s: %s - %s", report.Opid, report.Action, report.Detail))
		}
		scope["entries"] = entries
		if len(reports) == 0 {
			return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: scope,
				Summary: "the goal journal of this whole installation is clean; nothing was recovered"})
		}
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: scope, text: lines,
			Summary: fmt.Sprintf("recovered %d stranded journal entr(ies) across this whole installation; live owners were left alone", len(reports))})
	case "accept-edits":
		ran, problem := inv.engineVerb("goal", "reconcile", "--root", inv.stateRoot, "--by", inv.input.text("by"))
		if problem != nil {
			return inv.render(*problem)
		}
		return inv.render(ownerVerbResult(ran, targets, "the reviewed local goal edits were reconciled against their base and republished", scope))
	case "refresh":
		ran, problem := inv.engineVerb("goal", "reconcile", "--root", inv.stateRoot, "--refresh-only")
		if problem != nil {
			return inv.render(*problem)
		}
		return inv.render(ownerVerbResult(ran, targets, "the published view's interrupted refresh was completed; no edit was read as new authority", scope))
	case "accept-remote-history":
		ran, problem := inv.engineVerb("goal", "repair", "--accept-remote", "--by", inv.input.text("by"), "--root", inv.stateRoot)
		if problem != nil {
			return inv.render(*problem)
		}
		return inv.render(ownerVerbResult(ran, targets, "the fetched remote history of the same ledger was accepted locally; nothing was pushed", scope))
	}
	return runIntentRepairUpgrade(inv, targets, scope)
}

// runIntentRepairUpgrade converts the legacy goals file under its reviewed
// digest. Without --source-digest nothing runs: the current digest is shown
// for the person to review against.
func runIntentRepairUpgrade(inv *intentInvocation, targets []intentTarget, scope map[string]any) int {
	source := filepath.Join(inv.stateRoot, "plans", "goals.md")
	data, err := os.ReadFile(source)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: scope,
			Summary: fmt.Sprintf("there is no legacy goals file to upgrade at %s: %v; nothing was done", source, err)})
	}
	current := goal.SourceDigestOf(data)
	scope["sourceDigest"], scope["source"] = current, source
	if !inv.input.has("source-digest") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: targets, Data: scope,
			Summary:  fmt.Sprintf("the upgrade runs only on reviewed bytes; %s now has SHA-256 %s; nothing was done", source, current),
			Decision: "review that file, then repeat with --source-digest " + current + " (and --amendments FILE when goals must be added or amended; see metasystem help repair)"})
	}
	if digest := inv.input.text("source-digest"); digest != current {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: scope,
			Summary: fmt.Sprintf("the reviewed digest %s is not the file's current digest %s; the file changed after review; nothing was done", digest, current)})
	}
	args := []string{"goal", "migrate", "--root", inv.stateRoot, "--source-digest", current, "--by", inv.input.text("by")}
	if inv.input.has("amendments") {
		args = append(args, "--manifest", inv.flagPath("amendments"))
	}
	for _, pair := range [][2]string{{"identity", "--identity"}, {"sync-mode", "--sync-mode"}} {
		if inv.input.has(pair[0]) {
			args = append(args, pair[1], inv.input.text(pair[0]))
		}
	}
	ran, problem := inv.engineVerb(args...)
	if problem != nil {
		return inv.render(*problem)
	}
	return inv.render(ownerVerbResult(ran, targets, "the legacy goals file was upgraded to the synced ledger", scope))
}

// runIntentRepairWaits reports this checkout's durable wait continuations,
// checked against the checkout holder's session when one is named.
func runIntentRepairWaits(inv *intentInvocation) int {
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	result := inv.recoverWaits()
	return inv.render(result)
}

// runIntentRepairMission applies a person's typed resolution of one recorded
// workspace problem through the mission runner.
func runIntentRepairMission(inv *intentInvocation, mission string) int {
	choice, problem := inv.exclusiveChoice("confirm-restored", "accept-workspace")
	if problem == nil && choice == "" {
		problem = &intentResult{Outcome: intentRefused, code: 2,
			Summary:  "repair mission needs the person's decision: --confirm-restored TREE or --accept-workspace --waive CLAIM...; nothing was done",
			Decision: "see metasystem help repair"}
	}
	if problem == nil {
		problem = inv.requireInputs("problem", "by", "reason")
	}
	if problem == nil && choice == "confirm-restored" {
		if !intentTreePattern.MatchString(inv.input.text("confirm-restored")) {
			problem = &intentResult{Outcome: intentRefused, code: 2, Summary: "--confirm-restored is the recorded safe tree's 40 to 64 lowercase hex id; nothing was done"}
		} else if inv.input.has("waive") {
			problem = &intentResult{Outcome: intentRefused, code: 2, Summary: "--waive belongs to --accept-workspace; nothing was done"}
		}
	}
	if problem == nil && choice == "accept-workspace" && len(inv.input.values["waive"]) == 0 {
		problem = &intentResult{Outcome: intentRefused, code: 2, Summary: "--accept-workspace names each attribution claim it waives with --waive CLAIM; none is waived automatically; nothing was done"}
	}
	if problem == nil {
		if value := inv.input.text("problem"); !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(value) {
			problem = &intentResult{Outcome: intentRefused, code: 2, Summary: "--problem is the recorded problem's positive number; nothing was done"}
		}
	}
	if problem != nil {
		return inv.render(*problem)
	}
	if problem := inv.resolveLayout(); problem != nil {
		return inv.render(*problem)
	}
	root, err := inv.owners.resolver.RootForInstallation(inv.layout.InstallationRoot)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Summary: "the installation's state root is unavailable: " + err.Error()})
	}
	args := []string{"mission", "resolve-taint", "--root", root, "--mission", mission, "--taint", inv.input.text("problem")}
	done := fmt.Sprintf("mission %s: problem %s is resolved as confirmed restored; files were not changed by this command", mission, inv.input.text("problem"))
	if choice == "confirm-restored" {
		args = append(args, "--restore", inv.input.text("confirm-restored"))
	} else {
		args = append(args, "--adopt")
		for _, claim := range inv.input.values["waive"] {
			args = append(args, "--waives", claim)
		}
		done = fmt.Sprintf("mission %s: the observed workspace is accepted for problem %s with the named claims waived", mission, inv.input.text("problem"))
	}
	args = append(args, "--by", inv.input.text("by"), "--reason", inv.input.text("reason"))
	ran, problem := inv.engineVerb(args...)
	if problem != nil {
		return inv.render(*problem)
	}
	result := ownerVerbResult(ran, []intentTarget{{Kind: "mission", ID: mission}}, done, nil)
	if result.Outcome == intentConfirmed {
		result.next, result.nextReason = inv.publicArgv("status", "mission", mission), "every recorded problem must be resolved before the mission resumes"
	}
	return inv.render(result)
}

// runIntentSettingsCoordinator shows, declares or withdraws this checkout
// as its ledger's coordinator.
func runIntentSettingsCoordinator(inv *intentInvocation) int {
	choice, problem := inv.exclusiveChoice("declare", "withdraw")
	if problem == nil && choice != "" {
		problem = inv.requireInputs("by")
	}
	if problem == nil && choice == "" && inv.input.has("by") {
		problem = &intentResult{Outcome: intentRefused, code: 2, Summary: "--by names the person declaring or withdrawing; reading takes none; nothing was done"}
	}
	if problem != nil {
		return inv.render(*problem)
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	targets := []intentTarget{{Kind: "coordinator", ID: inv.stateRoot}}
	if choice != "" {
		ran, problem := inv.engineVerb("brain", choice, "--root", inv.stateRoot, "--by", inv.input.text("by"))
		if problem != nil {
			return inv.render(*problem)
		}
		return inv.render(ownerVerbResult(ran, targets, map[string]string{"declare": "this checkout is declared its ledger's coordinator", "withdraw": "this checkout's coordinator declaration is withdrawn"}[choice], nil))
	}
	state := brain.Read(inv.stateRoot, goal.ExistingLedgerIdentity(inv.stateRoot))
	data := map[string]any{"state": state.State}
	if state.Record != nil {
		data["record"] = state.Record
	}
	switch state.State {
	case brain.Declared:
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: data,
			Summary: fmt.Sprintf("this checkout is the coordinator of ledger %s, declared by %s at %s", state.Record.Ledger, state.Record.DeclaredBy, state.Record.DeclaredAt)})
	case brain.Undeclared:
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: data,
			Summary: "no coordinator is declared for this checkout", next: inv.publicArgv("settings", "coordinator", "--declare", "--by", "NAME"),
			nextReason: "a person at an agent-free terminal declares it"})
	}
	data["reason"] = state.Reason
	return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: data,
		Summary: "the coordinator declaration is unreadable: " + state.Reason, next: inv.publicArgv("check"), nextReason: "diagnose the declaration"})
}

// runIntentCheck diagnoses without writing: the checkout's machinery, or
// with goals the hand edits that repair goals --accept-edits would publish.
func runIntentCheck(inv *intentInvocation) int {
	switch args := inv.input.args; {
	case len(args) == 0:
		return runIntentDoctor(inv)
	case len(args) == 1 && args[0] == "goals":
		return runIntentCheckGoals(inv)
	case len(args) == 1 && args[0] == "settings":
		if problem := inv.selectRoot(); problem != nil {
			return inv.render(*problem)
		}
		root := inv.layout.InstallationRoot
		ran, problem := inv.engineVerb("config", "validate", "--conf", filepath.Join(root, "metasystem.conf"), "--repo", root)
		if problem != nil {
			return inv.render(*problem)
		}
		return inv.render(ownerVerbResult(ran, nil, "the settings of "+root+" are valid", map[string]any{"installation": root}))
	}
	return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "check takes nothing, goals or settings; nothing was done", Decision: "see metasystem help check"})
}

func runIntentCheckGoals(inv *intentInvocation) int {
	if inv.input.has("installation") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "check goals reads the selected repository; --installation belongs to the machinery check; nothing was done"})
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	targets := []intentTarget{{Kind: "installation", ID: inv.layout.InstallationRoot}}
	base, err := goal.BaseTip(inv.stateRoot)
	var deltas []goal.SnapshotDelta
	if err == nil {
		var snapshot *goal.Snapshot
		if snapshot, err = goal.CaptureSnapshot(inv.stateRoot); err == nil {
			deltas, err = goal.DiffAgainstBase(inv.stateRoot, base, snapshot)
		}
	}
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the goal files cannot be compared with their published base: " + err.Error()})
	}
	lines := make([]string, 0, len(deltas))
	for _, delta := range deltas {
		lines = append(lines, fmt.Sprintf("  %s %s", delta.Kind, delta.Path))
	}
	data := map[string]any{"base": base, "edits": deltas}
	if len(deltas) == 0 {
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: data, Summary: "the goal files match their published base; there are no hand edits"})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: data, text: lines,
		Summary: fmt.Sprintf("%d goal file(s) differ from their published base %s; nothing was changed", len(deltas), shortSHA(base)),
		next:    inv.publicArgv("repair", "goals", "--accept-edits", "--by", "NAME"), nextReason: "a person publishes these reviewed edits"})
}

// runIntentShowRecords shows the project's own records through the project
// reader: designs, decisions, one record, or the designs of one goal.
func runIntentShowRecords(inv *intentInvocation, kind string, args []string) int {
	if inv.input.has("history") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--history belongs to a goal's record; nothing was done"})
	}
	goalID := inv.input.text("id")
	switch {
	case kind == "record" && (len(args) != 1 || goalID != ""):
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "show record takes one record id and no --goal; nothing was done"})
	case kind == "design" && (len(args) != 0 || goalID == ""):
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "show design names its goal: show design --goal G; nothing was done"})
	case kind == "design" && (inv.input.has("attempt") || inv.input.has("out")):
		if problem := inv.selectRoot(); problem != nil {
			return inv.render(*problem)
		}
		return inv.render(inv.showDesignAttempts(goalID))
	case kind != "design" && (inv.input.has("attempt") || inv.input.has("out")):
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--attempt and --out belong to show design --goal G; nothing was done"})
	case (kind == "designs" || kind == "decisions") && len(args) != 0:
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "show " + kind + " takes no further words; nothing was done"})
	}
	if problem := inv.resolveLayout(); problem != nil {
		return inv.render(*problem)
	}
	stateRoot, err := inv.owners.resolver.RootForInstallation(inv.layout.InstallationRoot)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the installation's state root is unavailable: " + err.Error()})
	}
	read, err := project.Read(project.Roots{Checkout: inv.layout.GitRoot, Installation: inv.layout.InstallationRoot, StateRoot: stateRoot})
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the project's records cannot be read: " + err.Error(), next: inv.publicArgv("check"), nextReason: "diagnose the record homes"})
	}
	if kind == "record" {
		record := read.Record(args[0])
		if record == nil {
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: []intentTarget{{Kind: "record", ID: args[0]}},
				Summary: fmt.Sprintf("the project has no record %s; nothing was read", shellCommand(args[:1])), next: inv.publicArgv("show", "designs"), nextReason: "list the design records"})
		}
		referencedBy := read.ReferencedBy(record.ID)
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: []intentTarget{{Kind: "record", ID: record.ID}},
			Summary: fmt.Sprintf("%s %s (%s): %s", record.Kind, record.ID, record.Status, record.Title),
			text:    []string{"path: " + record.Path, "goals: " + strings.Join(record.Goals, ", ")},
			Data:    map[string]any{"record": recordView(*record), "referencedBy": referencedBy}})
	}
	recordKind := project.KindDesign
	if kind == "decisions" {
		recordKind = project.KindDecision
	}
	records := read.List(recordKind, project.ListOptions{Goal: goalID})
	views, lines := []map[string]any{}, []string{}
	for _, record := range records {
		views = append(views, recordView(record))
		lines = append(lines, fmt.Sprintf("  %s  %s  %s  %s", record.ID, record.Status, record.Path, record.Title))
	}
	summary := fmt.Sprintf("%d %s record(s)", len(records), recordKind)
	if goalID != "" {
		summary += " for goal " + goalID
	}
	result := intentResult{Outcome: intentConfirmed, Data: map[string]any{"kind": recordKind, "goal": goalID, "records": views}, text: lines, Summary: summary}
	if kind == "design" && len(records) == 0 {
		result.Summary = fmt.Sprintf("goal %s has no design record", goalID)
	}
	return inv.render(result)
}

func recordView(record project.Record) map[string]any {
	return map[string]any{"kind": record.Kind, "id": record.ID, "status": record.Status, "title": record.Title, "path": record.Path, "goals": record.Goals}
}
