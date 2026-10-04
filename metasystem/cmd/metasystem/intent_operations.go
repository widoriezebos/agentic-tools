package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// Administrative operations name one explicit human choice and hand it to
// the engine verb that owns it, as its own process in the installation, so
// the owner's caller classification, human proof, digests and refusal rules
// apply unchanged. The public result reports the owner's outcome and output;
// it never promotes a named person to a proven one.

// engineVerb runs one owner verb of the selected installation's engine as
// its own process, through the explicit internal entry with --json, and
// reads its answer from the envelope verb names; the error is an answer
// that could not be read, the intentResult a refusal before anything ran.
func (inv *intentInvocation) engineVerb(verb string, args ...string) (verbresult.Result, *intentResult, error) {
	binary, err := inv.delivery().executable()
	if err != nil {
		return verbresult.Result{}, &intentResult{Outcome: intentFailed, code: 1, Summary: "the running engine's own path could not be read, so nothing was done",
			retry: "try again", Details: []string{"engine path: " + err.Error()}}, nil
	}
	envelope := inv.delivery().ownerEnvelope
	if envelope == nil {
		envelope = runIntentOwnerEnvelope
	}
	result, readErr := envelope(intentProcess{argv: append(append([]string{binary, "internal"}, args...), "--json"), dir: inv.layout.InstallationRoot.Path()}, verb)
	return result, nil, readErr
}

// ownerEnvelopeResult is the public outcome of one owner verb's envelope:
// its data as the owner's record when it confirmed, its summary and code
// when it did not, and a plain failure when its answer could not be read.
func ownerEnvelopeResult(owner verbresult.Result, readErr error, targets []intentTarget, done string) intentResult {
	data := map[string]any{"exitCode": owner.Exit}
	if len(owner.Data) > 0 {
		var parsed any
		if json.Unmarshal(owner.Data, &parsed) == nil {
			data["owner"] = parsed
		}
	}
	if readErr != nil {
		return intentResult{Outcome: intentFailed, Targets: targets, Data: data, code: 1,
			Summary: "the command's answer could not be read, so what it did is unknown", retry: "once the cause is fixed",
			Details: []string{readErr.Error()}}
	}
	if owner.Outcome == verbresult.Confirmed {
		return intentResult{Outcome: intentConfirmed, Targets: targets, Data: data, Summary: done}
	}
	// The owner's own command, when it named one, replaces the retry.
	result := intentResult{Outcome: intentRefused, Targets: targets, Data: data, code: max(owner.Exit, 1), Summary: owner.Summary,
		Details: refusalCodeDetails(owner.Code), retry: "once the cause above is fixed"}
	if owner.Next != nil && len(owner.Next.Argv) > 0 {
		result.next, result.nextReason = owner.Next.Argv, owner.Next.Reason
	}
	return result
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
	summary := fmt.Sprintf("the command stopped (exit %d) without saying why; --json shows its output", ran.code)
	if reason := ownerRejectionReason(parsed); reason != "" {
		summary = reason
	} else if len(problem) > 0 {
		summary = problem[0]
	} else if lines := nonEmptyLines(output); parsed == nil && len(lines) > 0 {
		summary = lines[len(lines)-1]
	}
	if ran.err != nil {
		summary = "the command could not run: " + ran.err.Error()
	}
	// The summary is not repeated under itself.
	problem = slices.DeleteFunc(problem, func(line string) bool { return line == summary })
	return intentResult{Outcome: intentRefused, Targets: targets, Data: data, code: max(ran.code, 1), Summary: summary, text: problem,
		retry: ownerRetry(problem)}
}

// ownerViewed gives an owner verb's act its page: ✓ what it did, then the
// owner's own lines unless they were its structured output, which --json
// carries. A refusal keeps the legacy shape.
func ownerViewed(result intentResult) intentResult {
	data, _ := result.Data.(map[string]any)
	_, lines := data["owner"].([]string)
	summary, text := result.Summary, result.text
	result.view = func(page *textui.Page) {
		page.Done(sentence(summary))
		if !lines {
			return
		}
		section := page.Section("", "")
		for _, line := range text {
			section.Text(strings.TrimSpace(line))
		}
	}
	return result
}

// joinFacts joins a headline's facts with the page's separator.
func joinFacts(page *textui.Page, parts ...string) string {
	if page.Env().ASCII {
		return strings.Join(parts, ", ")
	}
	return strings.Join(parts, " · ")
}

// sentence is a text as a headline: its first letter upper case.
func sentence(text string) string {
	if text == "" {
		return text
	}
	return strings.ToUpper(text[:1]) + text[1:]
}

// ownerRetry is line 2 of an owner verb's refusal: nothing when the owner's
// own lines already name the command that resolves it, else this command
// again once its cause is fixed.
func ownerRetry(lines []string) string {
	for _, line := range lines {
		if strings.Contains(line, "metasystem ") {
			return ""
		}
	}
	return "once the cause above is fixed"
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
			Summary: fmt.Sprintf("%s are separate decisions; give one; nothing was done", strings.Join(given, " and ")),
			next:    inv.retryWith(stripDashes(given[1:])), nextReason: "keeps " + given[0] + "; one decision at a time"}
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
		var extra []string
		for _, flag := range missing {
			value := strings.ToUpper(strings.TrimPrefix(flag, "--"))
			if flag == "--by" {
				value = inv.knownPerson()
			}
			extra = append(extra, flag, value)
		}
		return &intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("this choice needs %s; nothing was done", strings.Join(missing, ", ")),
			next: inv.retryWith(stripDashes(missing), extra...), nextReason: "with your values"}
	}
	return nil
}

// refuseOthers refuses options that belong to other choices.
func (inv *intentInvocation) refuseOthers(allowed []string, names ...string) *intentResult {
	for _, name := range names {
		if inv.input.has(name) && !containsIntentValue(allowed, name) {
			return &intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--%s does not belong to this choice; nothing was done", name),
				next: inv.retryWith([]string{name}), nextReason: "without --" + name}
		}
	}
	return nil
}

// stripDashes is option names less their leading dashes.
func stripDashes(flags []string) []string {
	names := make([]string, 0, len(flags))
	for _, flag := range flags {
		names = append(names, strings.TrimLeft(flag, "-"))
	}
	return names
}

var (
	intentSHA256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	intentTreePattern   = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
)

var goalSyncOptions = []string{"recover", "refresh", "publish", "goal", "upgrade", "accept-remote-history", "source-digest", "amendments", "identity", "sync-mode", "by"}

// runIntentGoalSync previews the goal files against their published base,
// or carries out the one explicit mode it is given.
func runIntentGoalSync(inv *intentInvocation) int {
	choice, problem := inv.exclusiveChoice("recover", "refresh", "publish", "upgrade", "accept-remote-history")
	if problem == nil {
		switch choice {
		case "", "recover", "refresh":
			problem = inv.refuseOthers([]string{choice}, goalSyncOptions...)
		case "publish":
			if problem = inv.refuseOthers([]string{"publish", "goal", "by"}, goalSyncOptions...); problem == nil {
				problem = inv.requireInputs("goal", "by")
			}
		case "accept-remote-history":
			if problem = inv.refuseOthers([]string{choice, "by"}, goalSyncOptions...); problem == nil {
				problem = inv.requireInputs("by")
			}
		case "upgrade":
			if problem = inv.refuseOthers([]string{"upgrade", "source-digest", "amendments", "identity", "sync-mode", "by"}, goalSyncOptions...); problem == nil {
				problem = inv.requireInputs("by")
			}
			if mode := inv.input.text("sync-mode"); problem == nil && mode != "" && mode != "remote" && mode != "local" {
				problem = &intentResult{Outcome: intentRefused, code: 2, Summary: "--sync-mode is remote or local; nothing was done",
					next: inv.retryWith([]string{"sync-mode"}, "--sync-mode", "remote"), nextReason: "or local"}
			}
			if digest := inv.input.text("source-digest"); problem == nil && digest != "" && !intentSHA256Pattern.MatchString(digest) {
				problem = &intentResult{Outcome: intentRefused, code: 2, Summary: "--source-digest is not a file checksum (64 lowercase hex characters); nothing was done",
					next: inv.retryWith([]string{"source-digest"}), nextReason: "shows the file's checksum to review"}
			}
		}
	}
	if problem != nil {
		return inv.render(*problem)
	}
	if choice == "" {
		return runIntentGoalSyncPreview(inv)
	}
	if choice == "upgrade" {
		// Upgrading reads the legacy ledger, which a synced installation no
		// longer requires; only the installation is resolved.
		if problem := inv.resolveLayout(); problem != nil {
			return inv.render(*problem)
		}
		root, _ := inv.owners.resolver.RootForInstallation(inv.layout.InstallationRoot)
		inv.stateRoot = root.Path()
	} else if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	targets := []intentTarget{{Kind: "installation", ID: inv.layout.InstallationRoot.Path()}}
	scope := map[string]any{"scope": "installation", "stateRoot": inv.stateRoot}
	switch choice {
	case "recover":
		reports, err := recoverGoalJournal(inv.layout.InstallationRoot, inv.stateRoot, inv.owners.commandNow, inv.owners.dependencies)
		if err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: scope,
				Summary: "the goal changes left unfinished here could not be finished: " + err.Error(), next: inv.publicArgv("system", "check"), nextReason: "diagnose what stops recovery"})
		}
		entries, lines := []map[string]string{}, []string{}
		for _, report := range reports {
			entries = append(entries, map[string]string{"opid": report.Opid, "action": string(report.Action), "detail": report.Detail})
			lines = append(lines, fmt.Sprintf("%s: %s - %s", report.Opid, report.Action, report.Detail))
		}
		scope["entries"] = entries
		if len(reports) == 0 {
			return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: scope,
				Summary: "no goal change was left unfinished in this whole installation; nothing was recovered",
				view: func(page *textui.Page) {
					page.Headline("No goal change was left unfinished in this installation", "nothing to recover")
				}})
		}
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: scope, Details: lines,
			Summary: fmt.Sprintf("finished %d unfinished goal change(s) across this whole installation; running ones were left alone", len(reports)),
			view: func(page *textui.Page) {
				page.Done("Finished " + textui.Count(len(reports), "unfinished goal change", "unfinished goal changes") + " across this installation")
				page.Section("", "").Text("Changes still running were left alone.")
			}})
	case "publish":
		goals := inv.input.values["goal"]
		args := []string{"--root", inv.stateRoot, "--by", inv.input.text("by")}
		for _, id := range goals {
			args = append(args, "--id", id)
		}
		ran := inv.goalOwnerCall(inv.ownerCalls().goalReconcile, args...)
		scope["goals"] = goals
		return inv.render(ownerViewed(ownerVerbResult(ran, targets, fmt.Sprintf("the reviewed edits of %s were reconciled against their base and republished", strings.Join(goals, ", ")), scope)))
	case "refresh":
		ran := inv.goalOwnerCall(inv.ownerCalls().goalReconcile, "--root", inv.stateRoot, "--refresh-only")
		return inv.render(ownerViewed(ownerVerbResult(ran, targets, "the published view's interrupted refresh was completed; no edit was read as new authority", scope)))
	case "accept-remote-history":
		caller, by := ownercall.CurrentProcess(), inv.input.text("by")
		ran := ownerCall(func(stdout, stderr io.Writer) int {
			return inv.ownerCalls().goalRepair(caller, stdout, stderr, inv.stateRoot, by)
		})
		return inv.render(ownerViewed(ownerVerbResult(ran, targets, "the fetched remote history of the same ledger was accepted locally; nothing was pushed", scope)))
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
			Summary: fmt.Sprintf("there is no old goals file to upgrade at %s; nothing was done", source),
			next:    inv.publicArgv("goal", "sync"), nextReason: "shows whether the goals are already synced", Details: []string{err.Error()}})
	}
	current := goal.SourceDigestOf(data)
	scope["sourceDigest"], scope["source"] = current, source
	if !inv.input.has("source-digest") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: targets, Data: scope,
			Summary: fmt.Sprintf("review %s first: the upgrade runs only on the file you reviewed; nothing was done", source),
			next:    inv.retryWith([]string{"source-digest"}, "--source-digest", current), nextReason: "once you have reviewed it; its checksum now",
			Details: []string{"--amendments FILE adds or amends goals in the same upgrade; see metasystem help goal sync"}})
	}
	if digest := inv.input.text("source-digest"); digest != current {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: scope,
			Summary: "the goals file changed after your review; nothing was done",
			next:    inv.retryWith([]string{"source-digest"}, "--source-digest", current), nextReason: "once you have reviewed it again",
			Details: []string{fmt.Sprintf("reviewed checksum %s, current %s", digest, current)}})
	}
	args := []string{"--root", inv.stateRoot, "--source-digest", current, "--by", inv.input.text("by")}
	if inv.input.has("amendments") {
		args = append(args, "--manifest", inv.flagPath("amendments"))
	}
	for _, pair := range [][2]string{{"identity", "--identity"}, {"sync-mode", "--sync-mode"}} {
		if inv.input.has(pair[0]) {
			args = append(args, pair[1], inv.input.text(pair[0]))
		}
	}
	ran := inv.goalOwnerCall(inv.ownerCalls().goalMigrate, args...)
	return inv.render(ownerViewed(ownerVerbResult(ran, targets, "the legacy goals file was upgraded to the synced ledger", scope)))
}

// runIntentRepairMission applies a person's typed resolution of one recorded
// workspace problem through the mission runner.
func runIntentRepairMission(inv *intentInvocation, mission string) int {
	choice, problem := inv.exclusiveChoice("confirm-restored", "accept-workspace")
	if problem == nil && choice == "" {
		problem = &intentResult{Outcome: intentRefused, code: 2,
			Summary: "mission repair needs your decision: files restored, or the workspace accepted; nothing was done",
			next:    inv.retryWith(nil, "--confirm-restored", "TREE"), nextReason: "or --accept-workspace --waive CLAIM; metasystem help mission repair explains both"}
	}
	if problem == nil {
		problem = inv.requireInputs("problem", "by", "reason")
	}
	if problem == nil && choice == "confirm-restored" {
		if !intentTreePattern.MatchString(inv.input.text("confirm-restored")) {
			problem = &intentResult{Outcome: intentRefused, code: 2, Summary: "--confirm-restored takes the recorded safe tree's id (40 to 64 hex characters); nothing was done",
				next: inv.publicArgv("mission", "status", mission), nextReason: "names the recorded safe tree"}
		} else if inv.input.has("waive") {
			problem = &intentResult{Outcome: intentRefused, code: 2, Summary: "--waive belongs to --accept-workspace; nothing was done",
				next: inv.retryWith([]string{"waive"}), nextReason: "without --waive"}
		}
	}
	if problem == nil && choice == "accept-workspace" && len(inv.input.values["waive"]) == 0 {
		problem = &intentResult{Outcome: intentRefused, code: 2, Summary: "--accept-workspace waives only the claims you name, and none was; nothing was done",
			next: inv.retryWith(nil, "--waive", "CLAIM"), nextReason: "one --waive per claim; metasystem mission status names them"}
	}
	if problem == nil {
		if value := inv.input.text("problem"); !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(value) {
			problem = &intentResult{Outcome: intentRefused, code: 2, Summary: "--problem takes the recorded problem's number; nothing was done",
				next: inv.publicArgv("mission", "status", mission), nextReason: "lists the recorded problems with their numbers"}
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
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Summary: "this installation's records cannot be found, so nothing was done",
			next: []string{"metasystem", "system", "check"}, nextReason: "names what is wrong here", Details: []string{"state root: " + err.Error()}})
	}
	taint, _ := strconv.ParseInt(inv.input.text("problem"), 10, 64)
	request := missionResolveRequest{root: root.Path(), installation: inv.layout.InstallationRoot, mission: mission, taint: taint, variant: "restore", tree: inv.input.text("confirm-restored"),
		by: inv.input.text("by"), reason: inv.input.text("reason")}
	done := fmt.Sprintf("mission %s: problem %s is resolved as confirmed restored; files were not changed by this command", mission, inv.input.text("problem"))
	if choice != "confirm-restored" {
		request.variant, request.tree, request.waived = "adopt-disputed-tree", "", inv.input.values["waive"]
		done = fmt.Sprintf("mission %s: the observed workspace is accepted for problem %s with the named claims waived", mission, inv.input.text("problem"))
	}
	// The runner's human-reserved gate classifies this process, the parent
	// the former child classified (design 6.2).
	caller := ownercall.CurrentProcess()
	ran := ownerCall(func(stdout, stderr io.Writer) int {
		return inv.ownerCalls().missionResolveTaint(caller, stdout, stderr, request)
	})
	result := ownerVerbResult(ran, []intentTarget{{Kind: "mission", ID: mission}}, done, nil)
	if result.Outcome == intentConfirmed {
		for _, line := range nonEmptyLines(string(ran.stdout)) {
			if strings.Contains(line, missionrunner.TaintAlreadyResolved) {
				// The same resolution again (R-129-ui): nothing was recorded.
				result.Outcome, result.Summary = intentUnchanged, fmt.Sprintf("mission %s: problem %s %s", mission, inv.input.text("problem"), line[strings.Index(line, missionrunner.TaintAlreadyResolved):])
			}
		}
		result.next, result.nextReason = inv.publicArgv("mission", "status", mission), "every recorded problem must be resolved before the mission resumes"
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
		problem = &intentResult{Outcome: intentRefused, code: 2, Summary: "--by names who declares or withdraws; showing takes none; nothing was done",
			next: inv.retryWith([]string{"by"}), nextReason: "shows the coordinator"}
	}
	if problem != nil {
		return inv.render(*problem)
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	targets := []intentTarget{{Kind: "coordinator", ID: inv.stateRoot}}
	if choice != "" {
		// The human gate classifies this process, the parent the owner's
		// child used to classify (design 6.2, VOA-02-R2).
		caller, by := ownercall.CurrentProcess(), inv.input.text("by")
		ran := ownerCall(func(stdout, stderr io.Writer) int {
			return inv.ownerCalls().brain(choice, caller, stdout, stderr, inv.stateRoot, by)
		})
		result := ownerVerbResult(ran, targets, map[string]string{"declare": "this checkout is declared its ledger's coordinator", "withdraw": "this checkout's coordinator declaration is withdrawn"}[choice], nil)
		if owner, _ := result.Data.(map[string]any)["owner"].(map[string]any); result.Outcome == intentConfirmed && owner["unchanged"] == true {
			result.Outcome = intentUnchanged
			if summary, _ := owner["summary"].(string); summary != "" {
				result.Summary = summary
			}
		}
		return inv.render(ownerViewed(result))
	}
	state := brain.Read(inv.stateRoot, goal.ExistingLedgerIdentity(inv.stateRoot))
	data := map[string]any{"state": state.State}
	if state.Record != nil {
		data["record"] = state.Record
	}
	switch state.State {
	case brain.Declared:
		record := state.Record
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: data,
			Summary: fmt.Sprintf("this checkout is the coordinator of ledger %s, declared by %s at %s", record.Ledger, record.DeclaredBy, record.DeclaredAt),
			view: func(page *textui.Page) {
				declared := record.DeclaredAt
				if at, err := time.Parse(time.RFC3339, record.DeclaredAt); err == nil {
					declared = page.Env().Time(at)
				}
				page.Headline("This checkout is its ledger's coordinator", "declared by "+record.DeclaredBy, declared)
				page.Facts(textui.KV{Key: "ledger", Value: []textui.Span{textui.Plain(record.Ledger)}})
			}})
	case brain.Undeclared:
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: data,
			Summary: "no coordinator is declared for this checkout", next: inv.publicArgv("settings", "coordinator", "--declare", "--by", inv.knownPerson()),
			nextReason: "a person at an agent-free terminal declares it",
			view:       func(page *textui.Page) { page.Headline("No coordinator is declared for this checkout") }})
	}
	data["reason"] = state.Reason
	return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: data,
		Summary: "the coordinator declaration is unreadable: " + state.Reason, next: inv.publicArgv("system", "check"), nextReason: "diagnose the declaration"})
}

// runIntentGoalSyncPreview lists the goal files that differ from their
// published base, changing nothing.
func runIntentGoalSyncPreview(inv *intentInvocation) int {
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	targets := []intentTarget{{Kind: "installation", ID: inv.layout.InstallationRoot.Path()}}
	base, err := goal.BaseTip(inv.stateRoot)
	var deltas []goal.SnapshotDelta
	if err == nil {
		var snapshot *goal.Snapshot
		if snapshot, err = goal.CaptureSnapshot(inv.stateRoot); err == nil {
			deltas, err = goal.DiffAgainstBase(inv.stateRoot, base, snapshot)
		}
	}
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the goal files could not be compared with their published base",
			next: []string{"metasystem", "system", "check"}, nextReason: "names what is wrong here", Details: []string{err.Error()}})
	}
	lines := make([]string, 0, len(deltas))
	var edited []string
	for _, delta := range deltas {
		lines = append(lines, fmt.Sprintf("  %s %s", delta.Kind, delta.Path))
		if id := goalFileID(delta.Path); id != "" && !slices.Contains(edited, id) {
			edited = append(edited, id)
		}
	}
	data := map[string]any{"base": base, "edits": deltas, "goals": edited}
	if len(deltas) == 0 {
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: data, Summary: "the goal files match their published base; there are no hand edits",
			view: func(page *textui.Page) {
				page.Headline("The goal files match their published base", "no hand edits", "base "+textui.SHA(base))
			}})
	}
	publish := []string{"goal", "sync", "--publish"}
	for _, id := range edited {
		publish = append(publish, "--goal", id)
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: data, text: lines,
		Summary: fmt.Sprintf("%d goal file(s) differ from their published base %s; nothing was changed", len(deltas), shortSHA(base)),
		next:    inv.publicArgv(append(publish, "--by", inv.knownPerson())...), nextReason: "a person publishes these reviewed edits, naming each goal",
		view: func(page *textui.Page) {
			page.Headline(textui.Count(len(deltas), "goal file differs", "goal files differ")+" from the published base", "base "+textui.SHA(base), "nothing was changed")
			table := page.Section("", "").Table(textui.Column{}, textui.Column{})
			for _, delta := range deltas {
				table.Row(textui.Plain(string(delta.Kind)), textui.Plain(delta.Path))
			}
		}})
}

// goalFileID is the goal a ledger file path belongs to, or empty.
func goalFileID(path string) string {
	if !strings.HasSuffix(path, ".md") || strings.HasSuffix(path, "/backlog.md") {
		return ""
	}
	return strings.TrimSuffix(filepath.Base(path), ".md")
}

// The record verbs: design show and list, decision list and show.
func runIntentDesignShow(inv *intentInvocation) int {
	return runIntentShowRecords(inv, "design", inv.input.args)
}

func runIntentDesignList(inv *intentInvocation) int {
	return runIntentShowRecords(inv, "designs", inv.input.args)
}

func runIntentDecisionList(inv *intentInvocation) int {
	return runIntentShowRecords(inv, "decisions", inv.input.args)
}

func runIntentDecisionShow(inv *intentInvocation) int {
	return runIntentShowRecords(inv, "record", inv.input.args)
}

// runIntentShowRecords shows the project's own records through the project
// reader: designs, decisions, one record, or the designs of one goal.
func runIntentShowRecords(inv *intentInvocation, kind string, args []string) int {
	if inv.input.has("history") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--history belongs to a goal's record; nothing was done",
			next: inv.retryWith([]string{"history"}), nextReason: "without --history"})
	}
	goalID := inv.input.text("goal")
	if kind == "design" && len(args) == 1 {
		if goalID != "" && goalID != args[0] {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("design show names one goal, not %s and --goal %s; nothing was done", args[0], goalID),
				next: inv.publicArgv("design", "show", args[0]), nextReason: "or the other goal"})
		}
		goalID, args = args[0], nil
	}
	switch {
	case kind == "record" && len(args) != 1:
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "decision show takes one record id; nothing was done",
			next: inv.publicArgv("decision", "list"), nextReason: "lists the decision records with their ids"})
	case kind == "design" && goalID == "":
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "design show needs the goal whose designs it shows; nothing was done",
			next: inv.publicArgv("design", "list"), nextReason: "lists the design records with their goals"})
	case kind == "design" && (inv.input.has("attempt") || inv.input.has("out")):
		if problem := inv.selectRoot(); problem != nil {
			return inv.render(*problem)
		}
		return inv.render(inv.showDesignAttempts(goalID))
	case kind != "design" && (inv.input.has("attempt") || inv.input.has("out")):
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--attempt and --out belong to design show; nothing was done",
			next: inv.retryWith([]string{"attempt", "out"}), nextReason: "without them"})
	case (kind == "designs" || kind == "decisions") && len(args) != 0:
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: inv.command.name + " takes no further words; nothing was done",
			next: inv.publicArgv(inv.command.words()...), nextReason: "without them"})
	}
	if problem := inv.resolveLayout(); problem != nil {
		return inv.render(*problem)
	}
	stateRoot, err := inv.owners.resolver.RootForInstallation(inv.layout.InstallationRoot)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "this installation's records cannot be found, so nothing was read",
			next: []string{"metasystem", "system", "check"}, nextReason: "names what is wrong here", Details: []string{"state root: " + err.Error()}})
	}
	read, err := project.Read(project.Roots{Checkout: inv.layout.GitRoot, Installation: inv.layout.InstallationRoot, StateRoot: stateRoot})
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the project's records cannot be read: " + err.Error(), next: inv.publicArgv("system", "check"), nextReason: "diagnose the record homes"})
	}
	if kind == "record" {
		record := read.Record(args[0])
		if record == nil {
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: []intentTarget{{Kind: "record", ID: args[0]}},
				Summary: fmt.Sprintf("the project has no record %s; nothing was read", shellCommand(args[:1])), next: inv.publicArgv("decision", "list"), nextReason: "lists the decision records with their ids"})
		}
		referencedBy := read.ReferencedBy(record.ID)
		shown := *record
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: []intentTarget{{Kind: "record", ID: record.ID}},
			Summary: fmt.Sprintf("%s %s (%s): %s", record.Kind, record.ID, record.Status, record.Title),
			text:    []string{"path: " + record.Path, "goals: " + strings.Join(record.Goals, ", ")},
			Data:    map[string]any{"record": recordView(*record), "referencedBy": referencedBy},
			view: func(page *textui.Page) {
				page.Headline(sentence(string(shown.Kind))+" "+shown.ID+" is "+string(shown.Status), shown.Title)
				rows := []textui.KV{{Key: "path", Value: []textui.Span{textui.Plain(page.Env().Path(shown.Path))}}}
				if len(shown.Goals) > 0 {
					rows = append(rows, textui.KV{Key: "goals", Value: []textui.Span{textui.Plain(strings.Join(shown.Goals, ", "))}})
				}
				if len(referencedBy) > 0 {
					rows = append(rows, textui.KV{Key: "referenced by", Value: []textui.Span{textui.Plain(textui.Count(len(referencedBy), "record", "records"))}})
				}
				page.Facts(rows...)
			}})
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
	result := intentResult{Outcome: intentConfirmed, Data: map[string]any{"kind": recordKind, "goal": goalID, "records": views}, text: lines, Summary: summary,
		view: recordListView(records, string(recordKind), goalID)}
	if kind == "design" && len(records) == 0 {
		// A goal the ledger does not know is named as unknown, not as a
		// goal without designs.
		if problem := inv.selectRoot(); problem == nil {
			if projection, _, problem := inv.projection(); problem == nil {
				if file, _ := goalRecord(projection, goalID); file == nil {
					return unknownGoal(inv, goalID)
				}
			}
		}
		result.Summary = fmt.Sprintf("goal %s has no design record", goalID)
	}
	return inv.render(result)
}

// recordListView is a list of project records: the count, then one row per
// record, its id and status, its path and its title.
func recordListView(records []project.Record, kind, goalID string) func(*textui.Page) {
	return func(page *textui.Page) {
		switch {
		case len(records) == 0 && goalID != "":
			page.Headline("Goal " + goalID + " has no " + kind + " record")
			return
		case len(records) == 0:
			page.Headline("No " + kind + " records")
			return
		}
		count := textui.Count(len(records), kind+" record", kind+" records")
		if goalID != "" {
			count += " for goal " + goalID
		}
		page.Headline(sentence(count))
		table := page.Section("", "").Table(textui.Column{}, textui.Column{}, textui.Column{Flex: true, Wrap: true})
		for _, record := range records {
			table.Row(textui.Plain(record.ID), textui.Dim(string(record.Status)), textui.Plain(record.Title))
		}
		if page.Verbose() {
			paths := page.Section("Files", "")
			for _, record := range records {
				paths.KV(record.ID, textui.Plain(page.Env().Path(record.Path)))
			}
		}
	}
}

func recordView(record project.Record) map[string]any {
	return map[string]any{"kind": record.Kind, "id": record.ID, "status": record.Status, "title": record.Title, "path": record.Path, "goals": record.Goals}
}
