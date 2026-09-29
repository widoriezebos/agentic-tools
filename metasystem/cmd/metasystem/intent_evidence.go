package main

// The evidence object (design engine-owns-disk-lifetimes Part B 3.12, U6c;
// Wido 2026-09-28, option B and export before delete): what MetaSystem
// keeps as durable evidence outside the checkout. show reads; export
// copies items into verified archives; dispose is a person's removal from
// a previewed plan, exported first when asked. Past the bound machinery
// only compacts; removing an item is only a person's act.

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/evidence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"golang.org/x/sys/unix"
)

func evidenceIntentCommands() []intentCommand {
	return []intentCommand{
		{
			object: "evidence", action: "show", audience: "both", summary: "this checkout's durable evidence against its bound, or where one path's evidence went, changing nothing",
			usage: []string{"metasystem evidence show [--all]", "metasystem evidence show PATH|ITEM"},
			details: []string{
				"With no word: this checkout's evidence segment against evidence.segment-cap-gib, how many items are live or compacted, which are eligible and which an exclusion holds (an open goal, a receipt no retro covered, a citation by path), what the next pass would compact, and, when the segment stays over its bound after compaction, the ready command pair. Judged on the accepted goal ledger as it stands; nothing is fetched.",
				"--all names every evidence root of this host with its owner. With a path or an item name: what that path is now — a live file, or a file of a compacted or removed item answered from its tombstone (size, sha256, date, rule, receipt, and the archive when it was exported).",
				"Output is a short summary; --verbose prints every item; --json carries everything.",
			},
			flags:    []intentFlag{{name: "all", usage: "every evidence root of this host with its owner"}, intentVerboseFlag},
			maxArgs:  1,
			examples: []string{"metasystem evidence show", "metasystem evidence show --all", "metasystem evidence show /Users/me/metasystem-evidence/project/agents/107e72c67539/stopverb-design1/jobs/stopverb-design1.log"},
			run:      runIntentEvidenceShow,
		},
		{
			object: "evidence", action: "export", audience: "both", summary: "copy evidence items into verified, self-contained archives outside every evidence root",
			usage: []string{"metasystem evidence export ITEM|PATH... [--to DIR]", "metasystem evidence export --over-bound [--to DIR]"},
			details: []string{
				"One compressed archive per item, named by the item's content digest, with a manifest of every file, every blob its recipes name (inside the archive) and every recipe line; the archive is read back and verified, then made durable, before the item counts as exported. The same content again is 'already exported'; changed content is a new archive beside the old.",
				"DIR is --to, else evidence.export-dir; one of them is needed. A directory inside an evidence root, a checkout or a registered store is declined with where to put it instead. --over-bound selects what evidence dispose --over-bound would remove.",
			},
			flags:    []intentFlag{{name: "to", value: "DIR", usage: "where the archives go (default: evidence.export-dir)"}, {name: "over-bound", usage: "every item the bound says a person may remove"}, intentVerboseFlag},
			maxArgs:  -1,
			examples: []string{"metasystem evidence export stopverb-design1 --to /Volumes/Backup/metasystem-exports", "metasystem evidence export --over-bound"},
			run:      runIntentEvidenceExport,
		},
		{
			object: "evidence", action: "dispose", audience: "human", summary: "a person's removal of evidence items from a previewed plan, exported first when asked",
			usage: []string{
				"metasystem evidence dispose ITEM|PATH...|--over-bound [--export DIR] [--compact] --preview",
				"metasystem evidence dispose [--plan ID] [--override] [--reason TEXT]",
			},
			details: []string{
				"--preview writes one plan and changes nothing else: each item with its step, file count and bytes, and held or clear by every exclusion, judged on the accepted goal ledger as it stands (nothing is fetched); the plan says how old that ledger is. Agents may preview.",
				"Executing a plan (--plan ID, or the newest preview) is a person's act, proven at the terminal a person enrolled with metasystem system enroll --name NAME: it observes the ledger afresh, re-judges every item, skips an item changed since the preview, and removes the rest with an inventory, a tombstone and a receipt. A held item is skipped unless --override, and every override is recorded. With --export DIR each item is exported and verified first and removed only when its export verified. --compact compacts instead of removing, and is declined for an item with no compact form.",
				"--over-bound selects, per segment over its bound, its compacted items and its events archives past the age floor, oldest first, until the segment would be under its cap, and says what it cannot select.",
			},
			flags: []intentFlag{
				{name: "preview", usage: "plan only: write the plan and change nothing else"},
				{name: "plan", value: "ID", usage: "the preview to execute (default: the newest)"},
				{name: "over-bound", usage: "every item the bound says a person may remove"},
				{name: "export", value: "DIR", usage: "export each item to DIR and remove it only when the export verified"},
				{name: "compact", usage: "compact instead of removing"},
				{name: "remove", advanced: true, usage: "remove (the default step, spelled explicitly)"},
				{name: "override", usage: "take held items anyway; each override is recorded"},
				{name: "reason", value: "TEXT", usage: "why, for the receipt"},
				intentVerboseFlag,
			},
			maxArgs:  -1,
			examples: []string{"metasystem evidence dispose --over-bound --export /Volumes/Backup/metasystem-exports --preview", "metasystem evidence dispose --plan 01K2Z7Q3M8XW1V0P9D4J6S5R2T"},
			run:      runIntentEvidenceDispose,
		},
	}
}

// evidenceEnv is the environment of a person's evidence verb.
func evidenceEnv(inv *intentInvocation, top, by string) (evidence.Env, *intentResult) {
	owners := inv.owners.disk.withDefaults()
	build := owners.evidenceEnv
	if build == nil {
		build = func(ctx context.Context, top, by string) (evidence.Env, error) {
			return steward.EvidenceEnv(ctx, top, steward.DiskPass{Clock: owners.now}, by)
		}
	}
	env, err := build(context.Background(), top, by)
	if env.Session == "" {
		env.Session = terminalSession()
	}
	if err != nil {
		return env, &intentResult{Outcome: intentFailed, code: 1, Summary: "the evidence roots cannot be read: " + err.Error(),
			Decision: "metasystem settings check names what is wrong with this checkout's settings"}
	}
	return env, nil
}

func runIntentEvidenceShow(inv *intentInvocation) int {
	top, problem := inv.diskRoot()
	if problem != nil {
		return inv.render(*problem)
	}
	env, problem := evidenceEnv(inv, top, "")
	if problem != nil {
		return inv.render(*problem)
	}
	ctx := context.Background()
	if len(inv.input.args) == 1 {
		argument := inv.input.args[0]
		if !filepath.IsAbs(argument) {
			if target, err := env.Locate(ctx, argument); err == nil {
				argument = target.Item.Path
			} else if abs, absErr := filepath.Abs(argument); absErr == nil {
				argument = abs
			}
		}
		answer := evidence.Pointer(ctx, argument)
		return inv.render(intentResult{Outcome: intentConfirmed, Summary: answer.Line, Data: answer})
	}
	if inv.input.switched("all") {
		var lines []string
		var data []map[string]any
		for _, root := range env.Roots() {
			bytes, _, _ := diskstore.Measure(ctx, root.Path)
			lines = append(lines, fmt.Sprintf("%s: %s, %s", root.Path, root.Owner, evidenceBytes(bytes)))
			data = append(data, map[string]any{"root": root.Path, "owner": root.Owner, "bytes": bytes, "unsegmented": root.Unsegmented})
		}
		summary := fmt.Sprintf("%d evidence root(s) on this host", len(lines))
		return inv.render(intentResult{Outcome: intentConfirmed, Summary: summary, text: lines, Data: data})
	}
	view := env.Show(ctx)
	lines := view.Lines(inv.input.switched("verbose"))
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: lines[0], text: lines[1:], Data: view})
}

func evidenceBytes(bytes int64) string { return fmt.Sprintf("%.2f GiB", float64(bytes)/float64(1<<30)) }

// evidenceTargets are the items the words name, or with --over-bound the
// removal set and its still-over lines.
func evidenceTargets(inv *intentInvocation, env evidence.Env, fetch bool) ([]evidence.Target, []string, *intentResult) {
	ctx := context.Background()
	if inv.input.switched("over-bound") {
		if len(inv.input.args) > 0 {
			return nil, nil, &intentResult{Outcome: intentRefused, code: 2, Summary: "--over-bound selects the items itself; name items or pass --over-bound, not both; nothing was done"}
		}
		exclusions := env.Exclusions(fetch)
		targets, stillOver := env.OverBound(ctx, func(segment evidence.Segment, item evidence.Item) evidence.Judgement {
			return exclusions.Judge(ctx, segment, item)
		})
		return targets, stillOver, nil
	}
	if len(inv.input.args) == 0 {
		return nil, nil, &intentResult{Outcome: intentRefused, code: 2, Summary: "name the items (a name in this checkout's segment, or a path) or pass --over-bound; nothing was done",
			next: inv.publicArgv("evidence", "show"), nextReason: "see this checkout's items"}
	}
	var targets []evidence.Target
	for _, argument := range inv.input.args {
		if !filepath.IsAbs(argument) {
			if abs, err := filepath.Abs(argument); err == nil && strings.Contains(argument, string(filepath.Separator)) {
				argument = abs
			}
		}
		resolved, err := env.Resolve(ctx, argument)
		if err != nil {
			summary := err.Error() + "; nothing was done"
			if strings.Contains(err.Error(), evidence.ErrNotEvidence.Error()) {
				summary += "; the engine's own leftovers go by metasystem disk clean --strays"
			}
			return nil, nil, &intentResult{Outcome: intentRefused, code: 2, Summary: summary, next: inv.publicArgv("evidence", "show", "--all"), nextReason: "list the evidence roots"}
		}
		targets = append(targets, resolved...)
	}
	return targets, nil, nil
}

func evidenceOutcomeLines(outcomes []evidence.Outcome, verbose bool) (done int, freed int64, lines []string) {
	var notDone []string
	for _, outcome := range outcomes {
		if outcome.Done {
			done++
			freed += outcome.Freed
			if verbose {
				lines = append(lines, "  "+outcome.Line)
			}
			continue
		}
		notDone = append(notDone, "  "+outcome.Line)
	}
	return done, freed, append(lines, notDone...)
}

func runIntentEvidenceExport(inv *intentInvocation) int {
	top, problem := inv.diskRoot()
	if problem != nil {
		return inv.render(*problem)
	}
	env, problem := evidenceEnv(inv, top, "")
	if problem != nil {
		return inv.render(*problem)
	}
	dir, refusal := env.ExportDirFor(inv.input.text("to"), registeredStorePaths(top))
	if refusal != "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: refusal})
	}
	targets, stillOver, problem := evidenceTargets(inv, env, false)
	if problem != nil {
		return inv.render(*problem)
	}
	outcomes := env.ExportTargets(context.Background(), targets, dir)
	done, _, lines := evidenceOutcomeLines(outcomes, inv.input.switched("verbose"))
	lines = append(lines, stillOver...)
	already := 0
	for _, outcome := range outcomes {
		if outcome.Already {
			already++
		}
	}
	summary := fmt.Sprintf("exported %d of %d item(s) to %s", done-already, len(outcomes), dir)
	if already > 0 {
		summary += fmt.Sprintf("; %d already exported and verified", already)
	}
	if done < len(outcomes) {
		summary += fmt.Sprintf("; %d not exported (below, with why)", len(outcomes)-done)
	}
	outcome := intentConfirmed
	if done == already {
		outcome = intentUnchanged
	}
	return inv.render(intentResult{Outcome: outcome, Summary: summary, text: lines, Data: outcomes})
}

// registeredStorePaths are this checkout's registered stores: an export
// inside one would be removed with it.
func registeredStorePaths(top string) []string {
	records, _ := diskstore.CheckoutRegistry(top).Inventory()
	var paths []string
	for _, record := range records {
		if record.State != diskstore.StateReleased {
			paths = append(paths, record.Path)
		}
	}
	return paths
}

func runIntentEvidenceDispose(inv *intentInvocation) int {
	top, problem := inv.diskRoot()
	if problem != nil {
		return inv.render(*problem)
	}
	owners := inv.owners.disk.withDefaults()
	if inv.input.switched("compact") && inv.input.switched("remove") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--compact and --remove are two steps; choose one; nothing was done"})
	}
	if inv.input.switched("preview") {
		env, problem := evidenceEnv(inv, top, "")
		if problem != nil {
			return inv.render(*problem)
		}
		exportDir := ""
		if inv.input.has("export") {
			var refusal string
			if exportDir, refusal = env.ExportDirFor(inv.input.text("export"), registeredStorePaths(top)); refusal != "" {
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: refusal})
			}
		}
		targets, stillOver, problem := evidenceTargets(inv, env, false)
		if problem != nil {
			return inv.render(*problem)
		}
		plan, err := env.Preview(context.Background(), targets, stillOver, inv.input.switched("compact"), exportDir)
		if err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the plan could not be written: " + err.Error()})
		}
		return inv.render(evidencePlanResult(inv, plan))
	}
	if len(inv.input.args) > 0 || inv.input.switched("over-bound") || inv.input.has("export") || inv.input.switched("compact") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "a disposal runs from a previewed plan: add --preview to plan these items, then run metasystem evidence dispose --plan ID; nothing was done",
			next:    inv.publicArgv(append(append([]string{"evidence", "dispose"}, inv.raw...), "--preview")...), nextReason: "plan it first"})
	}
	by, err := owners.person(top)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 3,
			Summary:  "executing an evidence disposal is a person's act, and this shell was not proven to be one; nothing was done",
			Decision: humanauthority.PersonActRemedy("metasystem evidence dispose --plan ID") + "; metasystem evidence dispose ... --preview shows what it would do"})
	}
	env, problem := evidenceEnv(inv, top, by)
	if problem != nil {
		return inv.render(*problem)
	}
	id := inv.input.text("plan")
	if id == "" {
		id = evidence.NewestDisposePlan(env.HomeStateRoot, env.Session)
	}
	if id == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "this terminal session has previewed no evidence disposal; preview one here with --preview, or name a plan with --plan ID; nothing was done",
			next:    inv.publicArgv("evidence", "dispose", "--over-bound", "--preview"), nextReason: "plan one first"})
	}
	plan, err := evidence.ReadDisposePlan(env.HomeStateRoot, id)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: err.Error()})
	}
	outcomes := env.Execute(context.Background(), plan, evidence.ExecuteOptions{Override: inv.input.switched("override"), Reason: inv.input.text("reason")})
	done, freed, lines := evidenceOutcomeLines(outcomes, inv.input.switched("verbose"))
	already := 0
	for _, outcome := range outcomes {
		if outcome.Already {
			already++
		}
	}
	summary := fmt.Sprintf("plan %s: %d of %d item(s) disposed, %s freed", plan.ID, done-already, len(outcomes), evidenceBytes(freed))
	if already > 0 {
		summary += fmt.Sprintf("; %d already disposed", already)
	}
	if done < len(outcomes) {
		summary += fmt.Sprintf("; %d not (below, with what settles each)", len(outcomes)-done)
	}
	result := intentConfirmed
	if done == already {
		result = intentUnchanged
	}
	return inv.render(intentResult{Outcome: result, Summary: summary, text: lines, Data: outcomes})
}

func evidencePlanResult(inv *intentInvocation, plan evidence.DisposePlan) intentResult {
	var clear, held, declined, files int
	var bytes int64
	var lines []string
	for _, item := range plan.Items {
		switch item.State {
		case "clear":
			clear++
			files += item.Files
			bytes += item.Bytes
		case "held":
			held++
		default:
			declined++
		}
		if inv.input.switched("verbose") || item.State != "clear" {
			line := fmt.Sprintf("  %s %s: %s, %d files, %s", item.State, item.Step, item.Path, item.Files, evidenceBytes(item.Bytes))
			if len(item.Held) > 0 {
				line += "; held: " + strings.Join(item.Held, "; ") + " (--override takes it anyway)"
			}
			if item.Decline != "" {
				line += "; " + item.Decline
			}
			lines = append(lines, line)
		}
	}
	lines = append(lines, "  "+plan.Ledger)
	for _, line := range plan.StillOver {
		lines = append(lines, "  "+line)
	}
	summary := fmt.Sprintf("preview: nothing was changed; plan %s: %d clear (%d files, %s), %d held, %d declined", plan.ID, clear, files, evidenceBytes(bytes), held, declined)
	if plan.Export != "" {
		summary += "; each exported to " + plan.Export + " first"
	}
	lines = append(lines, "  a person executes it: metasystem evidence dispose --plan "+plan.ID)
	return intentResult{Outcome: intentConfirmed, Summary: summary, text: lines, Data: plan}
}

// terminalSession is the invoking terminal's session id: a preview and its
// execution from one shell share it.
func terminalSession() string {
	sid, err := unix.Getsid(0)
	if err != nil {
		return ""
	}
	return strconv.Itoa(sid)
}
