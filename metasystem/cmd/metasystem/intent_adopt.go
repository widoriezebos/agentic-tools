package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adopt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/cachedomain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// system adopt: install MetaSystem into a fresh application repository. The
// adoption owner (internal/adopt) decides what ships and what is refused; this
// file supplies the engine build, the goal genesis and the rendering.

func adoptIntentCommand() intentCommand {
	adoptable := strings.Join(runtimes.Adoptable(), ",")
	return intentCommand{
		object: "system", action: "adopt", audience: "human", summary: "install MetaSystem into a fresh application repository",
		usage: []string{"metasystem system adopt TARGET [--runtimes " + adoptable + "|none] [--enable SKILL] [--copy-skills]"},
		details: []string{
			"Run it in the template checkout (or name it with --repo). Exports its committed payload into TARGET (created when missing), writes metasystem.conf for the selected runtimes, registers skills and profiles, installs the shipped enforcement, seeds the goal ledger and records the template commit.",
			"--runtimes defaults to " + runtimes.AdoptionDefault() + "; the adoptable runtimes are " + strings.Join(runtimes.Adoptable(), ", ") + ", and none registers no runtime. --enable moves an optional skill into skills/; --copy-skills copies skill trees instead of linking them.",
			"Adopting a target that is already this installation at the same commit changes nothing. A target with other instruction assets, an older installation, or differing payload files is refused with the way forward (docs/metasystem-reconciliation.md); nothing is ever overwritten.",
			"Only file-shaped instruction assets are detectable: check by hand that no agent-directed prose, prompt directories, or agent-encoding hooks or CI exist before calling a repository fresh.",
		},
		flags: []intentFlag{
			{name: "runtimes", value: "LIST", usage: "comma-separated runtimes to register (" + adoptable + "), or none"},
			{name: "enable", value: "SKILL", repeat: true, usage: "move an optional skill into skills/"},
			{name: "copy-skills", usage: "copy skill trees instead of linking them"},
		},
		maxArgs:  1,
		examples: []string{"metasystem system adopt ../my-app", "metasystem system adopt ../my-app --runtimes claude,codex"},
		run:      runIntentSystemAdopt,
	}
}

func runIntentSystemAdopt(inv *intentInvocation) int {
	if len(inv.input.args) != 1 || strings.TrimSpace(inv.input.args[0]) == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: adopt.CodeUsage,
			Summary: "name the repository to adopt into", next: []string{"metasystem", "system", "adopt", "TARGET"}, nextReason: "TARGET is the application's directory"})
	}
	target := inv.input.args[0]
	if !filepath.IsAbs(target) {
		target = filepath.Join(inv.cwd, target)
	}
	source, err := adoptionSource(inv)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: adopt.CodeUsage, Summary: err.Error(),
			next: []string{"metasystem", "system", "adopt", inv.input.args[0], "--repo", "TEMPLATE"}, nextReason: "run it in the template checkout, or name the template with --repo"})
	}
	runtimeSelection := inv.input.text("runtimes")
	if inv.input.has("runtimes") && runtimeSelection == "" {
		runtimeSelection = " "
	}
	adoptOwner := inv.owners.adopt
	if adoptOwner == nil {
		adoptOwner = adopt.Adopt
	}
	result, err := adoptOwner(adopt.Options{
		Source: source, Target: target, Runtimes: runtimeSelection,
		Enable: inv.input.values["enable"], CopySkills: inv.input.switched("copy-skills"),
		Stdout: inv.stdout, Stderr: inv.stderr,
		Deps: adopt.Deps{
			Build:       func(source string) error { return buildAdoptionEngine(source, inv.stderr) },
			EngineStamp: supervise.BuildStamp,
			Genesis:     adoptGenesis,
		},
	})
	targets := []intentTarget{{Kind: "repository", ID: target}}
	if err != nil {
		refusal, ok := err.(*adopt.Refusal)
		if !ok {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "adoption failed: " + err.Error(),
				next: []string{"metasystem", "system", "check", "--repo", target}, nextReason: "see what the target is missing"})
		}
		result := intentResult{Outcome: intentRefused, code: refusal.Code, Targets: targets,
			Summary: refusal.Message, text: refusal.Detail, next: refusal.Argv, nextReason: refusal.Remedy, Decision: refusal.Remedy}
		if refusal.Again && len(refusal.Argv) == 0 {
			// Line 2 is this adoption again, once the cause is gone.
			result.Decision, result.retry = "", refusal.Remedy
		}
		return inv.render(result)
	}
	targets = []intentTarget{{Kind: "repository", ID: result.Target}}
	if result.Already {
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets,
			Summary: "the target is already this template's installation at " + result.SHA + "; nothing to do",
			Data:    map[string]any{"sha": result.SHA, "target": result.Target},
			view: func(page *textui.Page) {
				page.Done(joinFacts(page, "The target is already this template's installation", "at "+textui.SHA(result.SHA), "nothing to do"))
			}})
	}
	lines := append([]string{}, result.Notes...)
	lines = append(lines, adoptionRegistrations(result.Target, result.Installed)...)
	quoted := shellCommand([]string{result.Target})
	lines = append(lines,
		"finish the adoption:",
		"  1. Replace testing.json with a reviewed application test contract; until then metasystem settings check reports it incomplete.",
		"  2. Fill docs/project-rules.md with verified project facts (commands, invariants, budgets, reserved decisions).",
		"  3. Optionally set models, tiers and the evidence root in metasystem.conf.local; the defaults above apply until you do.",
		"  4. Commit those reviewed bytes from your enrolled terminal (metasystem system enroll --name NAME), then upgrade the committed goal ledger with metasystem goal sync --upgrade --by NAME before opening the first goal.",
		"  5. Prove and land the first change with metasystem test run, metasystem work review and metasystem work land.",
		"  Or take the guided path for these steps plus covenant v1 and the first goals: the inception interview (skills/inception/SKILL.md), on the coordinator seat, with the person present.",
		"  Then metasystem settings check --repo "+quoted+" and metasystem system check --repo "+quoted+" must both pass.")
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets,
		Summary: "adopted at template SHA " + result.SHA + " into " + result.Target, text: lines,
		Data: map[string]any{"sha": result.SHA, "target": result.Target, "notes": result.Notes, "installed": result.Installed},
		next: []string{"metasystem", "settings", "check", "--repo", result.Target}, nextReason: "after filling the facts above",
		view: adoptedView(result, inv.input.args[0], inv.knownPerson())})
}

// adoptedView is a fresh adoption: what was installed where, the notes, the
// registrations by directory, and the steps that finish it. Commands name the
// target as the person typed it, which works where they typed it.
func adoptedView(result adopt.Result, typed, person string) func(*textui.Page) {
	return func(page *textui.Page) {
		env := page.Env()
		page.Done("Adopted at template " + textui.SHA(result.SHA))
		page.Facts(textui.KV{Key: "into", Value: []textui.Span{textui.Plain(env.Path(result.Target))}})
		if len(result.Notes) > 0 {
			notes := page.Section("Notes", "")
			for _, note := range result.Notes {
				notes.Text(note)
			}
		}
		counts, order := adoptionRegistrationCounts(result.Target, result.Installed)
		if len(order) > 0 {
			registered := page.Section("Registered", "")
			for _, dir := range order {
				registered.KV(dir, textui.Plain(textui.Count(counts[dir], "path", "paths")))
			}
		}
		quoted := shellCommand([]string{typed})
		steps := page.Section("Finish the adoption", "")
		for _, step := range []string{
			"1. Replace testing.json with a reviewed application test contract; until then metasystem settings check reports it incomplete.",
			"2. Fill docs/project-rules.md with verified project facts (commands, invariants, budgets, reserved decisions).",
			"3. Optionally set models, tiers and the evidence root in metasystem.conf.local; the defaults above apply until you do.",
			"4. Commit those reviewed bytes from your enrolled terminal (metasystem system enroll --name " + person + "), then upgrade the committed goal ledger with metasystem goal sync --upgrade --by " + person + " before opening the first goal.",
			"5. Prove and land the first change with metasystem test run, metasystem work review and metasystem work land.",
			"Or take the guided path for these steps plus covenant v1 and the first goals: the inception interview (skills/inception/SKILL.md), on the coordinator seat, with the person present.",
			"Then metasystem settings check --repo " + quoted + " and metasystem system check --repo " + quoted + " must both pass.",
		} {
			steps.Text(step)
		}
		page.Hint(textui.Hint{Argv: []string{"metasystem", "settings", "check", "--repo", typed}, Reason: "after filling the facts above"})
	}
}

// adoptionSource is the template installation: the one containing --repo, or
// the current directory, as for every public action.
func adoptionSource(inv *intentInvocation) (string, error) {
	path := inv.cwd
	if inv.input.has("repo") {
		path = inv.input.text("repo")
		if !filepath.IsAbs(path) {
			path = filepath.Join(inv.cwd, path)
		}
	}
	layout, err := inv.owners.resolver.ResolveLayout(path)
	if err != nil {
		return "", fmt.Errorf("%s is not inside a metasystem template checkout: %v", shellCommand([]string{path}), err)
	}
	return layout.InstallationRoot.Path(), nil
}

// buildAdoptionEngine runs the bootstrap build in the template: the engine is
// always rebuilt from source, never copied on trust.
func buildAdoptionEngine(source string, progress io.Writer) error {
	command := exec.Command("go", "run", "-trimpath", "./cmd/devgate", "build")
	command.Dir = source
	environment, err := cachedomain.Carry(os.Environ(), "")
	if err != nil {
		return err
	}
	command.Env = environment
	command.Stdout, command.Stderr = progress, progress
	return command.Run()
}

// adoptGenesis writes the target's first accepted goal baseline through the
// goal owner's one genesis path. The caller is classified against the target,
// as every goal action is, and genesis admits the adoption shape (a goal-free
// ledger on a checkout whose history carries none) and nothing more. The ledger
// fence is enrolled after authorization, as for every goal mutation.
func adoptGenesis(target string) error {
	caller, err := goalCaller(target, 0, "reconcile")
	if err != nil {
		return err
	}
	if err := ensureGuardEnrolled(target); err != nil {
		return err
	}
	_, err = (&goal.Store{Root: target}).Reconcile(caller)
	return err
}

// adoptionRegistrations summarizes the registration paths runtime setup wrote,
// one line per directory; the JSON result keeps every path.
func adoptionRegistrations(target string, installed []string) []string {
	counts, order := adoptionRegistrationCounts(target, installed)
	lines := make([]string, 0, len(order))
	for _, dir := range order {
		lines = append(lines, fmt.Sprintf("registered: %d path(s) under %s", counts[dir], dir))
	}
	return lines
}

// adoptionRegistrationCounts counts the registration paths per directory,
// in the order runtime setup wrote them.
func adoptionRegistrationCounts(target string, installed []string) (map[string]int, []string) {
	counts := map[string]int{}
	var order []string
	for _, path := range installed {
		dir := filepath.Dir(path)
		if rel, err := filepath.Rel(target, dir); err == nil && filepath.IsLocal(rel) {
			dir = rel
		}
		if counts[dir] == 0 {
			order = append(order, dir)
		}
		counts[dir]++
	}
	return counts, order
}
