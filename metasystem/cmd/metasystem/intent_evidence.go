package main

// The evidence object (design engine-owns-disk-lifetimes Part B 3.12, U6c;
// Wido 2026-09-28, option B and export before delete): what MetaSystem
// keeps as durable evidence outside the checkout. show reads; export
// copies items into verified archives; dispose is a person's removal from
// a previewed plan, exported first when asked. Past the bound machinery
// only reports (Round B2-3); removing an item is only a person's act.

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/evidence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
	"golang.org/x/sys/unix"
)

func evidenceIntentCommands() []intentCommand {
	return []intentCommand{
		{
			object: "evidence", action: "show", audience: "both", summary: "this checkout's durable evidence against its bound, or where one path's evidence went, changing nothing",
			usage: []string{"metasystem evidence show [--all]", "metasystem evidence show PATH|ITEM"},
			details: []string{
				"With no word: this checkout's evidence segment against evidence.segment-cap-gib, how many items are past the age floor and which an exclusion holds (an open goal, a receipt no retro covered, a citation by path), and, when the segment is over its bound, by how much with the ready command pair. Machinery never removes evidence. Judged on the accepted goal ledger as it stands; nothing is fetched.",
				"Entries of the root outside every segment are listed as not managed: remove them by hand if unneeded; dispose never takes them. A removal that was cut short is reported; metasystem evidence dispose settles it.",
				"--all names every evidence root of this host with its owner. With a path or an item name: what that path is now — a live file, or a file of a removed item answered from its tombstone (size, sha256, date, rule, receipt, and the archive when it was exported).",
				"Output is a short summary; --verbose prints every item; --json carries everything.",
			},
			flags:    []intentFlag{{name: "all", usage: "every evidence root of this host with its owner"}, intentVerboseFlag},
			maxArgs:  1,
			examples: []string{"metasystem evidence show", "metasystem evidence show --all", "metasystem evidence show /Users/me/metasystem-evidence/project/agents/107e72c67539/stopverb-design1/jobs/stopverb-design1.log"},
			run:      runIntentEvidenceShow,
			laidOut:  true,
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
			laidOut:  true,
		},
		{
			object: "evidence", action: "dispose", audience: "human", summary: "a person's removal of evidence items from a previewed plan, exported first when asked",
			usage: []string{
				"metasystem evidence dispose ITEM|PATH...|--over-bound [--export DIR] --preview",
				"metasystem evidence dispose [--plan ID] [--override] [--reason TEXT]",
			},
			details: []string{
				"--preview writes one plan and changes nothing else (beyond settling a removal cut short, as every form does first): each item with its step, file count and bytes, and held or clear by every exclusion, judged on the accepted goal ledger as it stands (nothing is fetched); the plan says how old that ledger is. Agents may preview.",
				"Executing a plan (--plan ID, or this terminal session's newest preview younger than a day) is a person's act, proven at the terminal a person enrolled with metasystem system enroll --name NAME: it observes the ledger afresh, re-judges every item, skips an item changed since the preview, and removes the rest with an inventory, a tombstone and a receipt. A held item is skipped unless --override, and every override is recorded. With --export DIR each item is exported and verified first and removed only when its export verified.",
				"Only an item of a segment is removed; an entry that is not managed is refused (remove it by hand if unneeded). Every form first settles a removal that was cut short: it is never continued, it is rolled back unless its receipt was written (else only its set-aside copy is removed), and the item is previewed again.",
				"--over-bound selects, per segment over its bound, its items past the age floor, oldest first, until the segment would be under its cap, and says what it cannot select.",
			},
			flags: []intentFlag{
				{name: "preview", usage: "plan only: write the plan and change nothing else"},
				{name: "plan", value: "ID", usage: "the preview to execute (default: the newest)"},
				{name: "over-bound", usage: "every item the bound says a person may remove"},
				{name: "export", value: "DIR", usage: "export each item to DIR and remove it only when the export verified"},
				{name: "override", usage: "take held items anyway; each override is recorded"},
				{name: "reason", value: "TEXT", usage: "why, for the receipt"},
				intentVerboseFlag,
			},
			maxArgs:  -1,
			examples: []string{"metasystem evidence dispose --over-bound --export /Volumes/Backup/metasystem-exports --preview", "metasystem evidence dispose --plan 01K2Z7Q3M8XW1V0P9D4J6S5R2T"},
			run:      runIntentEvidenceDispose,
			laidOut:  true,
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
		return inv.render(intentResult{Outcome: intentConfirmed, Summary: answer.Line, Data: answer, view: func(page *textui.Page) {
			page.Headline(shortPaths(page.Env(), answer.Line))
		}})
	}
	if inv.input.switched("all") {
		var lines []string
		var data []map[string]any
		open := env.OpenDisposals()
		lines = append(lines, open...)
		roots := env.Roots()
		var items []string
		for _, root := range roots {
			bytes, _, _ := diskstore.Measure(ctx, root.Path)
			line := fmt.Sprintf("%s: %s, %s", root.Path, root.Owner, textui.GiB(bytes))
			if len(root.NotManaged) > 0 {
				line += fmt.Sprintf("; %d entries outside every segment, %s", len(root.NotManaged), evidence.NotManagedLine)
			}
			lines = append(lines, line)
			if inv.input.switched("verbose") {
				for _, path := range root.NotManaged {
					lines = append(lines, "  "+path+": "+evidence.NotManagedLine)
				}
			}
			data = append(data, map[string]any{"root": root.Path, "owner": root.Owner, "bytes": bytes, "notManaged": root.NotManaged})
		}
		if inv.input.switched("verbose") {
			// Every item the inventory enumerates: the only things
			// evidence dispose accepts (Round B2-2, R1).
			targets, err := env.Enumerate(ctx)
			if err != nil {
				lines = append(lines, "the items cannot be listed: "+err.Error())
			}
			for _, target := range targets {
				lines = append(lines, fmt.Sprintf("  %s %s, %s", target.Item.Kind, target.Item.Path, textui.GiB(target.Item.Bytes)))
				items = append(items, fmt.Sprintf("%s %s, %s", target.Item.Kind, target.Item.Path, textui.Bytes(target.Item.Bytes)))
			}
			if err != nil {
				items = append(items, "the items cannot be listed: "+err.Error())
			}
		}
		summary := fmt.Sprintf("%d evidence root(s) on this host", len(data))
		return inv.render(intentResult{Outcome: intentConfirmed, Summary: summary, text: lines, Data: data, view: func(page *textui.Page) {
			pageEnv := page.Env()
			page.Headline(textui.Count(len(data), "evidence root", "evidence roots") + " on this computer")
			if len(open) > 0 {
				section := page.Section("Cut short", "")
				for _, line := range open {
					section.Text(shortPaths(pageEnv, line))
				}
			}
			table := page.Section("", "").Table(textui.Column{}, textui.Column{Right: true}, textui.Column{Flex: true, Wrap: true})
			for index, root := range roots {
				bytes, _ := data[index]["bytes"].(int64)
				owner := root.Owner
				if len(root.NotManaged) > 0 {
					owner += fmt.Sprintf("; %s outside every segment: %s", textui.Count(len(root.NotManaged), "entry", "entries"), evidence.NotManagedLine)
				}
				table.Row(textui.Plain(pageEnv.Path(root.Path)), textui.Plain(textui.Bytes(bytes)), textui.Dim(shortPaths(pageEnv, owner)))
			}
			if page.Verbose() {
				section := page.Section("Items", "what evidence dispose accepts")
				for _, item := range items {
					section.Text(shortPaths(pageEnv, item))
				}
			}
		}})
	}
	view := env.Show(ctx)
	lines := view.Lines(inv.input.switched("verbose"))
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: lines[0], text: lines[1:], Data: view, view: func(page *textui.Page) {
		evidenceShowView(page, view, lines)
	}})
}

// evidenceShowView lays out this checkout's segment: its items against the
// cap as the headline, then where it lives, the age floor, how the items
// stand and the ledger they were judged on.
func evidenceShowView(page *textui.Page, view evidence.SegmentView, lines []string) {
	env := page.Env()
	if view.Root == "" {
		page.Headline("This checkout keeps no evidence yet")
		page.Section("", "").Text(shortPaths(env, view.Unknown))
		return
	}
	position := view.Position
	var held, removable int
	for _, item := range view.Items {
		if len(item.Held) > 0 {
			held++
		} else if item.Removable {
			removable++
		}
	}
	page.Headline("This checkout keeps "+textui.Count(len(view.Items), "evidence item", "evidence items"),
		textui.Bytes(position.TotalBytes)+" of its "+textui.Bytes(position.CapBytes)+" cap")
	page.Facts(
		textui.KV{Key: "root", Value: []textui.Span{textui.Plain(env.Path(view.Root)), textui.Dim("  segment " + position.Segment)}},
		textui.KV{Key: "age floor", Value: []textui.Span{textui.Plain(textui.Count(int(view.AgeFloor.Hours()/24), "day", "days"))}},
		textui.KV{Key: "items", Value: []textui.Span{textui.Plain(fmt.Sprintf("%d past the age floor and clear · %d held by an exclusion", removable, held))}},
		textui.KV{Key: "ledger", Value: []textui.Span{textui.Plain(shortPaths(env, view.Ledger))}},
	)
	// The lines after the counts: the unknowns, pending removals, the
	// bound's position and the entries outside every segment, as the owner
	// words them (every item, with --verbose).
	if len(lines) > 2 {
		section := page.Section("", "")
		for _, line := range lines[2:] {
			section.Text(shortPaths(env, diskstore.CountedNouns(strings.TrimSpace(line))))
		}
	}
}

// evidenceTargets are the items the words name, or with --over-bound the
// removal set and its still-over lines.
func evidenceTargets(inv *intentInvocation, env evidence.Env, fetch bool) ([]evidence.Target, []string, *intentResult) {
	ctx := context.Background()
	if inv.input.switched("over-bound") {
		if len(inv.input.args) > 0 {
			return nil, nil, &intentResult{Outcome: intentRefused, code: 2,
				Summary: "--over-bound picks the items itself, so it takes no named items; nothing was done",
				next:    withoutArgs(inv.typedArgv(), inv.input.args), nextReason: "or name the items without --over-bound"}
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
			if errors.Is(err, evidence.ErrNotEvidence) {
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
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: refusal,
			next: append(withoutOption(inv.typedArgv(), "to"), "--to", "DIR"), nextReason: "DIR is where the copies go"})
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
	return inv.render(intentResult{Outcome: outcome, Summary: summary, text: lines, Data: outcomes, view: func(page *textui.Page) {
		env := page.Env()
		headline := fmt.Sprintf("Exported %d of %s to %s", done-already, textui.Count(len(outcomes), "item", "items"), env.Path(dir))
		facts := []string{}
		if already > 0 {
			facts = append(facts, fmt.Sprintf("%d already exported and verified", already))
		}
		if done < len(outcomes) {
			facts = append(facts, fmt.Sprintf("%d not exported", len(outcomes)-done))
		}
		if outcome == intentConfirmed {
			page.Done(strings.Join(append([]string{headline}, facts...), " · "))
		} else {
			page.Headline(headline, facts...)
		}
		evidenceOutcomeSection(page, lines)
	}})
}

// evidenceOutcomeSection lists what an act did not do (and, with --verbose,
// what it did), each with what settles it.
func evidenceOutcomeSection(page *textui.Page, lines []string) {
	if len(lines) == 0 {
		return
	}
	section := page.Section("", "")
	for _, line := range lines {
		section.Text(shortPaths(page.Env(), strings.TrimSpace(line)))
	}
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
	// Every form first settles a removal that was cut short (Round B2-3,
	// rule 2): rolled back, never continued; the lines lead the output.
	settleEnv, problem := evidenceEnv(inv, top, "")
	if problem != nil {
		return inv.render(*problem)
	}
	var settled []string
	for _, line := range settleEnv.SettleOpen(context.Background()) {
		settled = append(settled, "  settled first: "+line)
	}
	render := func(result intentResult) int {
		result.text = append(append([]string(nil), settled...), result.text...)
		if view := result.view; view != nil && len(settled) > 0 {
			result.view = func(page *textui.Page) {
				view(page)
				section := page.Section("Settled first", "a removal that was cut short")
				for _, line := range settled {
					section.Text(shortPaths(page.Env(), strings.TrimPrefix(strings.TrimSpace(line), "settled first: ")))
				}
			}
		}
		return inv.render(result)
	}
	owners := inv.owners.disk.withDefaults()
	if inv.input.switched("preview") {
		env, problem := evidenceEnv(inv, top, "")
		if problem != nil {
			return render(*problem)
		}
		exportDir := ""
		if inv.input.has("export") {
			var refusal string
			if exportDir, refusal = env.ExportDirFor(inv.input.text("export"), registeredStorePaths(top)); refusal != "" {
				return render(intentResult{Outcome: intentRefused, code: 2, Summary: refusal,
					next: append(withoutOption(inv.typedArgv(), "export"), "--export", "DIR"), nextReason: "with another directory"})
			}
		}
		targets, stillOver, problem := evidenceTargets(inv, env, false)
		if problem != nil {
			return render(*problem)
		}
		plan, err := env.Preview(context.Background(), targets, stillOver, exportDir)
		if err != nil {
			return render(intentResult{Outcome: intentFailed, code: 1, Summary: "the disposal plan couldn't be saved, so nothing was planned",
				next: inv.sameCommand(), nextReason: "tries again", Details: []string{"the plan could not be written: " + err.Error()}})
		}
		return render(evidencePlanResult(inv, plan))
	}
	if len(inv.input.args) > 0 || inv.input.switched("over-bound") || inv.input.has("export") {
		return render(intentResult{Outcome: intentRefused, code: 2,
			Summary:    "a disposal removes only what a preview planned, so nothing was removed",
			next:       inv.publicArgv(append(append([]string{"evidence", "dispose"}, inv.raw...), "--preview")...),
			nextReason: "plans it; then metasystem evidence dispose carries the plan out"})
	}
	by, err := owners.person(top)
	if err != nil {
		refused := inv.personRefusal("", err, "")
		refused.code = 3
		refused.Summary = "only a person may carry out an evidence disposal, and " + refused.Summary
		refused.Details = append(refused.Details, "metasystem evidence dispose ... --preview shows what it would do; any shell may preview")
		return render(*refused)
	}
	env, problem := evidenceEnv(inv, top, by)
	if problem != nil {
		return render(*problem)
	}
	id := inv.input.text("plan")
	if id == "" {
		id = evidence.NewestDisposePlan(env.HomeStateRoot, env.Session, env.Now)
	}
	if id == "" {
		return render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "this terminal hasn't previewed an evidence disposal in the last day, so nothing was removed",
			next:    inv.publicArgv("evidence", "dispose", "--over-bound", "--preview"), nextReason: "plans one; or name an earlier plan with --plan"})
	}
	plan, err := evidence.ReadDisposePlan(env.HomeStateRoot, id)
	if err != nil {
		return render(intentResult{Outcome: intentRefused, code: 2, Summary: "disposal plan " + id + " can't be read, so nothing was removed",
			next: inv.publicArgv("evidence", "dispose", "--over-bound", "--preview"), nextReason: "plans a new one", Details: []string{err.Error()}})
	}
	outcomes := env.Execute(context.Background(), plan, evidence.ExecuteOptions{Override: inv.input.switched("override"), Reason: inv.input.text("reason")})
	done, freed, lines := evidenceOutcomeLines(outcomes, inv.input.switched("verbose"))
	already := 0
	for _, outcome := range outcomes {
		if outcome.Already {
			already++
		}
	}
	summary := fmt.Sprintf("plan %s: %d of %d item(s) disposed, %s freed", plan.ID, done-already, len(outcomes), textui.GiB(freed))
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
	return render(intentResult{Outcome: result, Summary: summary, text: lines, Data: outcomes, view: func(page *textui.Page) {
		headline := fmt.Sprintf("Plan %s: disposed %d of %s, %s freed", plan.ID, done-already, textui.Count(len(outcomes), "item", "items"), textui.Bytes(freed))
		var facts []string
		if already > 0 {
			facts = append(facts, fmt.Sprintf("%d already disposed", already))
		}
		if done < len(outcomes) {
			facts = append(facts, fmt.Sprintf("%d not disposed", len(outcomes)-done))
		}
		if result == intentConfirmed {
			page.Done(strings.Join(append([]string{headline}, facts...), " · "))
		} else {
			page.Headline(headline, facts...)
		}
		evidenceOutcomeSection(page, lines)
	}})
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
			line := fmt.Sprintf("  %s %s: %s, %d files, %s", item.State, item.Step, item.Path, item.Files, textui.GiB(item.Bytes))
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
	summary := fmt.Sprintf("preview: nothing was changed; plan %s: %d clear (%d files, %s), %d held, %d declined", plan.ID, clear, files, textui.GiB(bytes), held, declined)
	if plan.Export != "" {
		summary += "; each exported to " + plan.Export + " first"
	}
	shown := append([]string(nil), lines...)
	lines = append(lines, "  a person executes it: metasystem evidence dispose --plan "+plan.ID)
	return intentResult{Outcome: intentConfirmed, Summary: summary, text: lines, Data: plan, view: func(page *textui.Page) {
		env := page.Env()
		page.Headline("Preview: nothing was changed", "plan "+plan.ID)
		facts := []textui.KV{
			{Key: "clear", Value: []textui.Span{textui.Plain(fmt.Sprintf("%s, %s, %s", textui.Count(clear, "item", "items"), textui.Count(files, "file", "files"), textui.Bytes(bytes)))}},
			{Key: "held", Value: []textui.Span{textui.Plain(textui.Count(held, "item", "items"))}},
			{Key: "declined", Value: []textui.Span{textui.Plain(textui.Count(declined, "item", "items"))}},
		}
		if plan.Export != "" {
			facts = append(facts, textui.KV{Key: "export", Value: []textui.Span{textui.Plain("each item to " + env.Path(plan.Export) + " first")}})
		}
		page.Facts(facts...)
		evidenceOutcomeSection(page, shown)
		page.Hint(textui.Hint{Argv: []string{"metasystem", "evidence", "dispose", "--plan", plan.ID}, Reason: "a person carries it out"})
	}}
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
