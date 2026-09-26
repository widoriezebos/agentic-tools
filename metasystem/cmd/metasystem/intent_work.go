package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// The work commands prepare and advance an agent's unit of work: brief
// writes a brief scaffold from the goal and its accepted design, build turns
// a brief and a check command into the existing unit plan and advances it
// through build, proof and independent read with the unit runner, fold unit
// sends a follow-up round to that run, and wait waits on a job, a goal event
// or a unit run. test and settings reach the selected installation's test
// runner and settings. The unit runner keeps every run, its retry identity,
// its retained inputs and its judgement state; these commands never approve,
// certify or land what they built, and a read's verdict is preliminary
// feedback for the author.

// intentMissingDecision marks a decision a generated brief could not take
// from the ledger or the design. build refuses a brief that still has one.
const intentMissingDecision = "MISSING DECISION:"

// intentReaderToolCalls is the brief line build reads the independent
// read's tool-call budget from; it is the review template's own wording.
var intentReaderToolCalls = regexp.MustCompile(`(?m)^Maximum reader tool calls:\s*([0-9]+)\s*$`)

// intentWorkOwners are the owners the work commands call. Tests give each
// invocation its own runner, Git, wait, subprocess and settings readers.
type intentWorkOwners struct {
	units      func(layout stateroot.Layout) *launch.UnitRunner
	git        func(dir string, args ...string) ([]byte, error)
	wait       func(args []string, print func(metarun.WaitResult, bool)) int
	subprocess func(dir string, argv []string, stderr io.Writer) ([]byte, int, error)
	settings   func(confPath string) (launch.Settings, error)
	config     func(key, confPath string) (value, source string, code int, err error)
}

// intentConfPath is the selected installation's configuration file.
func intentConfPath(layout stateroot.Layout) string {
	return filepath.Join(layout.InstallationRoot, "metasystem.conf")
}

func (inv *intentInvocation) work() intentWorkOwners {
	owners := inv.owners.work
	if owners.units == nil {
		// The selected installation supplies the templates and the launch
		// settings; launch and unit records stay the user's own stores.
		owners.units = func(layout stateroot.Layout) *launch.UnitRunner {
			manager := launchManager()
			if layout.InstallationRoot != "" {
				manager.TemplateDirectory = filepath.Join(layout.InstallationRoot, "scripts", "agents", "templates")
				manager.Settings, manager.SettingsError = launch.ResolveSettings(intentConfPath(layout), launchLookupEnv)
			}
			return &launch.UnitRunner{Manager: manager, Git: launch.OSGitRunner{}}
		}
	}
	if owners.git == nil {
		owners.git = func(dir string, args ...string) ([]byte, error) {
			command := exec.Command("git", append([]string{"-C", dir}, args...)...)
			var stderr bytes.Buffer
			command.Stderr = &stderr
			output, err := command.Output()
			if err != nil {
				return output, fmt.Errorf("git %s: %v: %s", args[0], err, strings.TrimSpace(stderr.String()))
			}
			return output, nil
		}
	}
	if owners.wait == nil {
		owners.wait = func(args []string, print func(metarun.WaitResult, bool)) int {
			return runWaitCommand(args, nil, waitCallerPID(), print)
		}
	}
	if owners.subprocess == nil {
		owners.subprocess = func(dir string, argv []string, stderr io.Writer) ([]byte, int, error) {
			executable, err := os.Executable()
			if err != nil {
				return nil, 1, err
			}
			command := exec.Command(executable, argv...)
			command.Dir, command.Stdin, command.Stderr = dir, os.Stdin, stderr
			output, err := command.Output()
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				return output, exit.ExitCode(), nil
			}
			if err != nil {
				return output, 1, err
			}
			return output, 0, nil
		}
	}
	if owners.settings == nil {
		owners.settings = func(confPath string) (launch.Settings, error) {
			return launch.ResolveSettings(confPath, launchLookupEnv)
		}
	}
	if owners.config == nil {
		owners.config = func(key, confPath string) (string, string, int, error) {
			params := config.GetParams{Key: key, ConfPath: confPath}
			value, code, err := config.Get(params)
			if err != nil || code != 0 {
				return "", "", code, err
			}
			source, err := config.KeyOrigin(params)
			return value, source, 0, err
		}
	}
	return owners
}

var intentBriefFlag = intentFlag{name: "brief", value: "FILE", usage: "the brief, relative to the directory the command runs in"}

func intentWorkCommands() []intentCommand {
	return []intentCommand{
		{
			name: "brief", audience: "agent", summary: "write a brief scaffold from a goal and its accepted design",
			usage: []string{"metasystem brief G --out FILE"},
			details: []string{
				"Carries the goal's intent and done criteria, its approval and box, its branch worktree, and the accepted design's",
				"units, constraints, return and acceptance sections. A decision neither record holds is written as a MISSING DECISION",
				"line, and build refuses the brief until each is filled. An existing different FILE is never overwritten.",
			},
			flags:    []intentFlag{intentTargetFlag, {name: "out", value: "FILE", usage: "where to write the brief"}},
			maxArgs:  1,
			examples: []string{"metasystem brief verbs-match-intent --out /tmp/work-brief.md"},
			run:      runIntentBrief,
		},
		{
			name: "build", audience: "agent", summary: "build one unit: build, proof and independent read, then await judgement",
			usage: []string{
				"metasystem build G UNIT --brief FILE [--lines N] [--read-tool-calls N] --check COMMAND...",
				"metasystem build --resume RUN",
				"metasystem build [G UNIT] --plan FILE",
			},
			details: []string{
				"Needs an approved goal and the goal/G branch checked out in a worktree; the command to prepare one is printed when it is missing.",
				"--check ends the options: every later word is the proof command's argument vector, run without a shell.",
				"The unit's size is its row in the brief's or the accepted design's units table; without a row give --lines N.",
				"The read may use the tool calls the brief names (Maximum reader tool calls: N) or --read-tool-calls N; it has",
				"as many rounds as the goal's approved box allows.",
				"The same goal, unit and request reach the same run again, with the inputs and base it started with; a different",
				"request is refused with that run's id. A follow-up is fold unit RUN.",
				"The run ends awaiting judgement, green or red, with the read's verdict as preliminary feedback. Nothing is approved,",
				"certified or landed.",
			},
			flags: []intentFlag{
				intentBriefFlag,
				{name: "lines", value: "N", usage: "the unit's changed-line estimate, when no units table has its row"},
				{name: "read-tool-calls", value: "N", usage: "the independent read's tool-call budget, when the brief does not name it"},
				{name: "resume", value: "RUN", usage: "continue this unit run"},
				{name: "model", value: "MODEL", advanced: true, usage: "the build model for this unit instead of launch.build.model"},
				{name: "effort", value: "EFFORT", advanced: true, usage: "the build effort for this unit instead of launch.build.effort"},
				{name: "plan", value: "FILE", advanced: true, usage: "an existing unit plan (the unit run plan format)"},
				{name: "check", value: "COMMAND...", rest: true, usage: "the proof command; it ends the options"},
			},
			maxArgs: 2,
			examples: []string{
				"metasystem build verbs-match-intent work-intents --brief work-brief.md --check go test -count=1 -run 'TestIntent' ./cmd/metasystem/",
				"metasystem build --resume 20260925T101500Z-abc123",
			},
			run: runIntentBuild,
		},
		{
			name: "wait", audience: "agent", summary: "wait for a job, a goal event or a unit run",
			usage: []string{"metasystem wait job J [--timeout DURATION]", "metasystem wait goal G [--event landing|human-act] [--after TIP] [--timeout DURATION]", "metasystem wait unit RUN [--timeout DURATION]"},
			details: []string{
				"A wait that reaches its timeout reports the work as still in progress and prints the exact command that continues the wait.",
				"wait goal waits for the goal's landing after the accepted ledger as it is now, unless --event and --after say otherwise.",
				"wait unit continues the unit run's recorded steps; it starts no new build.",
				"The flag forms (metasystem wait --job J, wait register, wait end) keep their existing behavior.",
			},
			flags: []intentFlag{
				{name: "timeout", value: "DURATION", usage: "how long this invocation waits (for example 10m)"},
				{name: "event", value: "EVENT", advanced: true, usage: "goal waits: landing (default) or human-act"},
				{name: "after", value: "TIP", advanced: true, usage: "goal waits: the accepted ledger commit to wait after (default: the current one)"},
				{name: "verb", value: "VERB", advanced: true, usage: "human-act waits: the act to wait for"},
				{name: "question", value: "ID", advanced: true, usage: "answer waits: the question"},
				{name: "chain", value: "ROOT", advanced: true, usage: "landing waits: the delegate chain root"},
			},
			maxArgs:  2,
			examples: []string{"metasystem wait job 20260925-job-1 --timeout 10m", "metasystem wait goal verbs-match-intent", "metasystem wait unit 20260925T101500Z-abc123"},
			run:      runIntentWait,
		},
		{
			name: "test", audience: "both", summary: "run the risk-selected tests for this checkout",
			usage: []string{"metasystem test [--goal G] [--authority H] [--mode auto|standard|deep]"},
			details: []string{
				"Runs the selected installation's test runner (metasystem test run --root INSTALLATION) and reports its result.",
				"metasystem test plan, test verify and test report keep their own grammar.",
			},
			flags: []intentFlag{
				{name: "goal", value: "G", usage: "the accepted goal owning the delivery"},
				{name: "authority", value: "H", advanced: true, usage: "the claimed goal authorizing the proof reservation"},
				{name: "mode", value: "MODE", usage: "auto (default), standard or deep"},
			},
			maxArgs:  0,
			examples: []string{"metasystem test", "metasystem test --goal verbs-match-intent --mode standard"},
			run:      runIntentTest,
		},
		{
			name: "settings", audience: "both", summary: "show the selected installation's settings and where each value comes from",
			usage: []string{"metasystem settings [KEY]"},
			details: []string{"Without KEY: the launch settings. With KEY: that launch setting or any metasystem.conf key.",
				"Read only. Settings are changed in metasystem.conf or the environment, not by this command."},
			maxArgs:  1,
			examples: []string{"metasystem settings", "metasystem settings launch.read.model", "metasystem settings context.ceiling.tokens --repo ../other"},
			run:      runIntentSettings,
		},
	}
}

// intentYieldsToLegacy keeps an existing call on its own handler where a
// public name is also a family or a top-level call: a registered family verb
// after the name, or the flag-only and subcommand forms of wait.
func intentYieldsToLegacy(args []string, registered []family) bool {
	if len(args) < 2 {
		return args[0] == "wait"
	}
	for _, fam := range registered {
		if fam.name != args[0] {
			continue
		}
		for _, v := range fam.verbs {
			if v.name == args[1] {
				return true
			}
		}
	}
	if args[0] == "wait" {
		return !slices.Contains([]string{"job", "goal", "unit", "-h", "--help", "-help"}, args[1])
	}
	return false
}

// writeIntentHelpWithFamily is a public command's help; where the name is
// also a family, that family's own help follows, so both forms are documented.
func writeIntentHelpWithFamily(w io.Writer, command intentCommand, registered []family) {
	writeIntentCommandHelp(w, command)
	for _, fam := range registered {
		if fam.name == command.name {
			fmt.Fprintln(w)
			writeFamilyHelp(w, fam)
		}
	}
}

// resolveLayout selects the installation from --repo or the current
// directory, without requiring a goal ledger.
func (inv *intentInvocation) resolveLayout() *intentResult {
	if inv.layout.InstallationRoot != "" {
		return nil
	}
	path := inv.cwd
	if inv.input.has("repo") {
		path = inv.callerPath(inv.input.text("repo"))
	}
	layout, err := inv.owners.resolver.ResolveLayout(path)
	if err != nil {
		return &intentResult{Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("%s is not inside one metasystem installation: %v", shellCommand([]string{path}), err),
			Decision: "run this inside the repository, or name it with --repo PATH"}
	}
	inv.layout = layout
	return nil
}

// unitRunner is the unit runner for the selected installation. A command
// that names only a run id still works outside a repository, with the
// launch manager's own settings.
func (inv *intentInvocation) unitRunner() *launch.UnitRunner {
	_ = inv.resolveLayout()
	runner := inv.work().units(inv.layout)
	runner.BeforeModelLaunch = inv.unitLaunchAuthority
	return runner
}

// unitLaunchAuthority is asked before every build or read launch of a unit
// run this command advances, new or continued: the goal must be claimed by
// this session and the checkout lease held, resolved through the selected
// installation's own configuration.
func (inv *intentInvocation) unitLaunchAuthority(record launch.UnitRunRecord, _ launch.StartSpec) error {
	conn := inv.connection()
	endpoint, err := conn.endpoint(inv.layout.InstallationRoot)
	if err != nil {
		return err
	}
	return branch.CheckHolder(conn.claimCheck(inv.layout.InstallationRoot, record.Goal, endpoint))
}

// build

func runIntentBuild(inv *intentInvocation) int {
	switch {
	case inv.input.has("resume"):
		for _, conflicting := range []string{"brief", "check", "lines", "plan", "model", "effort", "read-tool-calls"} {
			if inv.input.has(conflicting) {
				return inv.render(intentResult{Outcome: intentRefused, code: 2,
					Summary: fmt.Sprintf("--resume continues a recorded run and takes no --%s; nothing was done", conflicting), Decision: "give --resume RUN alone"})
			}
		}
		if len(inv.input.args) > 0 {
			return inv.render(intentResult{Outcome: intentRefused, code: 2,
				Summary: "--resume takes the run, not a goal and unit; nothing was done", Decision: "give --resume RUN alone"})
		}
		run := inv.input.text("resume")
		runner := inv.unitRunner()
		result, err := runner.Continue(launch.UnitRequest{Resume: run})
		return inv.render(inv.unitOutcome(runner, result, err, []intentTarget{{Kind: "unit", ID: run}}, inv.publicArgv("build", "--resume", run)))
	case inv.input.has("plan"):
		return runIntentBuildPlan(inv)
	}
	return runIntentBuildUnit(inv)
}

func runIntentBuildPlan(inv *intentInvocation) int {
	for _, conflicting := range []string{"brief", "check", "lines", "model", "effort", "read-tool-calls"} {
		if inv.input.has(conflicting) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("--plan already holds the brief, size, read and proof; --%s is not taken with it; nothing was done", conflicting), Decision: "give the plan alone, or build from --brief and --check without --plan"})
		}
	}
	path := inv.callerPath(inv.input.text("plan"))
	plan, err := launch.ReadUnitPlan(path)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, Summary: err.Error() + "; nothing was built", code: 1})
	}
	if named := inv.input.args; len(named) > 0 && (named[0] != plan.Goal || len(named) > 1 && named[1] != plan.Unit) {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("the plan is unit %s of goal %s, not %s; nothing was built", plan.Unit, plan.Goal, strings.Join(named, " ")),
			Decision: "name the plan's goal and unit, or none"})
	}
	runner := inv.unitRunner()
	result, err := runner.AdvanceNamed(path)
	targets := []intentTarget{{Kind: "goal", ID: plan.Goal}, {Kind: "unit", ID: plan.Unit}}
	return inv.render(inv.unitOutcome(runner, result, err, targets, append([]string{"metasystem", "build"}, inv.raw...)))
}

var (
	intentModelPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:\[\]-]*$`)
	intentEffortOptions = []string{"minimal", "low", "medium", "high", "xhigh", "max"}
)

func runIntentBuildUnit(inv *intentInvocation) int {
	if len(inv.input.args) != 2 || !inv.input.has("brief") || !inv.input.has("check") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary:  "build needs the goal, the unit name, --brief FILE and --check COMMAND...; nothing was done",
			Decision: "metasystem build G UNIT --brief FILE --check COMMAND... (see metasystem help build)"})
	}
	if model := inv.input.text("model"); inv.input.has("model") && !intentModelPattern.MatchString(model) {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--model %s is not a model name; nothing was done", shellCommand([]string{model}))})
	}
	if effort := inv.input.text("effort"); inv.input.has("effort") && !slices.Contains(intentEffortOptions, effort) {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("--effort must be one of %s, not %s; nothing was done", strings.Join(intentEffortOptions, ", "), shellCommand([]string{effort}))})
	}
	id, unit := inv.input.args[0], inv.input.args[1]
	targets := []intentTarget{{Kind: "goal", ID: id}, {Kind: "unit", ID: unit}}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return inv.render(*problem)
	}
	file, where := goalRecord(projection, id)
	if file == nil {
		return unknownGoal(inv, id)
	}
	if where != "live" {
		return inv.render(intentResult{Outcome: intentRefused, Targets: targets, code: 1,
			Summary: fmt.Sprintf("goal %s is %s; nothing was built", id, where)})
	}
	if file.Budget == nil || file.Budget.ReviewRoundLimit <= 0 {
		return inv.render(intentResult{Outcome: intentRefused, Targets: targets, code: 1,
			Summary:  fmt.Sprintf("goal %s has no approved box, so its review-round limit is unknown; nothing was built", id),
			Decision: "a person approves the goal with its box: metasystem approve " + id})
	}
	designs, problem := inv.acceptedDesignPaths(id)
	if problem != nil {
		problem.Targets = targets
		return inv.render(*problem)
	}
	runner := inv.unitRunner()
	request, problem := inv.unitRequest(runner, id, unit, designs, int(file.Budget.ReviewRoundLimit))
	if problem != nil {
		problem.Targets = targets
		return inv.render(*problem)
	}
	result, err := runner.AdvancePrepared(request.worktree, id, unit, request.bytes, request.options, request.prepare)
	outcome := inv.unitOutcome(runner, result, err, targets, append([]string{"metasystem", "build"}, inv.raw...))
	if data, ok := outcome.Data.(map[string]any); ok {
		data["inputs"] = request.directory
	}
	return inv.render(outcome)
}

// unitRequest is what one public build asks for: the caller's inputs,
// validated and rendered as the bytes that identify the request, and the
// preparation the unit runner calls, under the unit's lock, only when the
// unit has no run yet.
type unitRequest struct {
	worktree, directory string
	bytes               []byte
	options             launch.UnitOptions
	prepare             func(directory string) (string, error)
}

type unitRequestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func fileIdentity(path string) (unitRequestFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return unitRequestFile{}, err
	}
	return unitRequestFile{Path: path, SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}, nil
}

// unitRequest checks everything the build needs before the unit runner is
// asked for anything, and writes nothing.
func (inv *intentInvocation) unitRequest(runner *launch.UnitRunner, id, unit string, designs []string, rounds int) (unitRequest, *intentResult) {
	briefPath := inv.callerPath(inv.input.text("brief"))
	brief, err := os.ReadFile(briefPath)
	if err != nil {
		return unitRequest{}, &intentResult{Outcome: intentRefused, code: 1, Summary: "cannot read the brief: " + err.Error() + "; nothing was built"}
	}
	if missing := missingDecisionLines(brief); len(missing) > 0 {
		return unitRequest{}, &intentResult{Outcome: intentRefused, code: 1, text: missing,
			Summary:  fmt.Sprintf("the brief %s still has %d missing decision(s); nothing was built", shellCommand([]string{briefPath}), len(missing)),
			Decision: "fill each MISSING DECISION line in the brief"}
	}
	toolCalls, problem := inv.readToolCalls(brief)
	if problem != nil {
		return unitRequest{}, problem
	}
	check := inv.input.values["check"]
	worktree, problem := inv.prepareGoalWorktree(id)
	if problem != nil {
		return unitRequest{}, problem
	}
	sizeSource, lines, problem := inv.unitSize(unit, briefPath, designs)
	if problem != nil {
		return unitRequest{}, problem
	}
	readModel, problem := unitReadModel(runner)
	if problem != nil {
		return unitRequest{}, problem
	}
	templates := runner.Manager.TemplateDirectory
	if templates == "" {
		templates = filepath.Join(inv.layout.InstallationRoot, "scripts", "agents", "templates")
	}
	template, err := os.ReadFile(filepath.Join(templates, "review-brief.md"))
	if err != nil {
		return unitRequest{}, &intentResult{Outcome: intentFailed, code: 1, Summary: "cannot read the review brief template: " + err.Error()}
	}
	directory, err := runner.NamedInputDirectory(worktree, id, unit)
	if err != nil {
		return unitRequest{}, &intentResult{Outcome: intentFailed, code: 1, Summary: "cannot name the unit's input directory: " + err.Error()}
	}
	identity := struct {
		Brief         unitRequestFile   `json:"brief"`
		Designs       []unitRequestFile `json:"designs"`
		Check         []string          `json:"check"`
		Lines         string            `json:"lines,omitempty"`
		ReadToolCalls int               `json:"readToolCalls"`
		Model         string            `json:"model,omitempty"`
		Effort        string            `json:"effort,omitempty"`
	}{Check: check, Lines: inv.input.text("lines"), ReadToolCalls: toolCalls, Model: inv.input.text("model"), Effort: inv.input.text("effort"), Designs: []unitRequestFile{}}
	if identity.Brief, err = fileIdentity(briefPath); err != nil {
		return unitRequest{}, &intentResult{Outcome: intentRefused, code: 1, Summary: "cannot read the brief: " + err.Error()}
	}
	for _, design := range designs {
		entry, err := fileIdentity(design)
		if err != nil {
			return unitRequest{}, &intentResult{Outcome: intentRefused, code: 1, Summary: "cannot read the accepted design: " + err.Error() + "; nothing was built"}
		}
		identity.Designs = append(identity.Designs, entry)
	}
	encoded, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return unitRequest{}, &intentResult{Outcome: intentFailed, code: 1, Summary: err.Error()}
	}
	git := inv.work().git
	prepare := func(directory string) (string, error) {
		head, err := git(worktree, "rev-parse", "HEAD")
		if err != nil {
			return "", fmt.Errorf("cannot read the goal branch's commit: %w", err)
		}
		base := strings.TrimSpace(string(head))
		buildBrief, readBrief := filepath.Join(directory, "build-brief.md"), filepath.Join(directory, "read-brief.md")
		// The reader writes its findings where its sandbox allows writes and
		// outside the product diff: a private temporary directory, created
		// once for the unit and kept in the plan. The launch owner copies the
		// file into each read launch; the unit runner recreates a cleaned
		// directory before a later read.
		findingsDirectory, err := os.MkdirTemp("", "metasystem-unit-read-")
		if err != nil {
			return "", fmt.Errorf("cannot create the read's findings directory: %w", err)
		}
		findings, planPath := filepath.Join(findingsDirectory, "read-findings.md"), filepath.Join(directory, "plan.json")
		unitsPage := buildBrief
		if strings.HasPrefix(sizeSource, "design:") {
			unitsPage = strings.TrimPrefix(sizeSource, "design:")
		}
		binding := unitBinding{goal: id, unit: unit, worktree: worktree, base: base, brief: briefPath, designs: designs, check: check,
			estimate: sizeSource == "lines", unitsPage: unitsPage, lines: lines, findings: findings, rounds: rounds, toolCalls: toolCalls}
		plan := launch.UnitPlan{Unit: unit, Goal: id, Worktree: worktree, Base: base,
			Build: launch.UnitBuildPlan{Brief: buildBrief, Inputs: append([]string{}, designs...), Outputs: []string{}, UnitsPage: unitsPage, Units: []string{unit}},
			Read:  launch.UnitReadPlan{Brief: readBrief, Inputs: append([]string{}, designs...), Outputs: []string{findings}, Model: readModel},
			Proof: []launch.ProofCommand{{Name: "check", Dir: worktree, Argv: append([]string{}, check...), Env: []string{}}}}
		encodedPlan, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return "", err
		}
		for _, file := range []struct{ path, text string }{
			{buildBrief, binding.buildBrief(brief)},
			{readBrief, binding.readBrief(template, buildBrief)},
			{planPath, string(encodedPlan) + "\n"},
		} {
			if _, err := atomicfile.WriteText(file.path, file.text, directory); err != nil {
				return "", err
			}
		}
		if _, err := launch.ReadUnitPlan(planPath); err != nil {
			return "", fmt.Errorf("the generated unit plan is invalid: %w", err)
		}
		pack := &launch.Manager{TemplateDirectory: templates}
		if _, err := pack.CheckPack(launch.StartSpec{Kind: "read", Brief: readBrief, WorkingDirectory: worktree}); err != nil {
			return "", fmt.Errorf("the generated read brief does not fill the review template: %w", err)
		}
		return planPath, nil
	}
	return unitRequest{worktree: worktree, directory: directory, bytes: append(encoded, '\n'), prepare: prepare,
		options: launch.UnitOptions{BuildModel: inv.input.text("model"), BuildEffort: inv.input.text("effort"), MaxRounds: rounds}}, nil
}

// readToolCalls is the independent read's tool-call budget: --read-tool-calls,
// or the brief's "Maximum reader tool calls: N" line. Neither is a missing
// decision; no number is assumed.
func (inv *intentInvocation) readToolCalls(brief []byte) (int, *intentResult) {
	if inv.input.has("read-tool-calls") {
		value, err := strconv.Atoi(inv.input.text("read-tool-calls"))
		if err != nil || value <= 0 {
			return 0, &intentResult{Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("--read-tool-calls must be a positive whole number, not %s; nothing was done", shellCommand([]string{inv.input.text("read-tool-calls")}))}
		}
		if match := intentReaderToolCalls.FindSubmatch(brief); match != nil && string(match[1]) != strconv.Itoa(value) {
			return 0, &intentResult{Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("the brief allows the read %s tool calls but --read-tool-calls says %d; nothing was built", match[1], value)}
		}
		return value, nil
	}
	if match := intentReaderToolCalls.FindSubmatch(brief); match != nil {
		if value, err := strconv.Atoi(string(match[1])); err == nil && value > 0 {
			return value, nil
		}
	}
	return 0, &intentResult{Outcome: intentRefused, code: 1,
		Summary:  "the independent read's tool-call budget is not decided; nothing was built",
		Decision: "a line 'Maximum reader tool calls: N' in the brief, or --read-tool-calls N"}
}

func missingDecisionLines(brief []byte) []string {
	var missing []string
	for index, line := range strings.Split(string(brief), "\n") {
		if strings.Contains(line, intentMissingDecision) {
			missing = append(missing, fmt.Sprintf("line %d: %s", index+1, strings.TrimSpace(line)))
		}
	}
	return missing
}

// callerPath binds a file argument to the directory the command runs in.
func (inv *intentInvocation) callerPath(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(inv.cwd, path)
}

// goalWorktree finds the worktree that has goal/G checked out. Without one
// it prints the Git command that prepares it; the current checkout and its
// uncommitted files are left where they are.
func (inv *intentInvocation) goalWorktree(id string) (string, *intentResult) {
	git := inv.work().git
	output, err := git(inv.layout.GitRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return "", &intentResult{Outcome: intentFailed, code: 1, Summary: "cannot list the repository's worktrees: " + err.Error()}
	}
	want := "branch refs/heads/goal/" + id
	for _, block := range strings.Split(string(output), "\n\n") {
		var path string
		found := false
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "worktree ") {
				path = strings.TrimPrefix(line, "worktree ")
			}
			if line == want {
				found = true
			}
		}
		if found && path != "" {
			if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
				return path, nil
			}
		}
	}
	target := filepath.Join(filepath.Dir(inv.layout.GitRoot), filepath.Base(inv.layout.GitRoot)+"-"+id)
	argv := []string{"git", "-C", inv.layout.GitRoot, "worktree", "add", "-b", "goal/" + id, target}
	if _, err := git(inv.layout.GitRoot, "rev-parse", "--verify", "-q", "refs/heads/goal/"+id); err == nil {
		argv = []string{"git", "-C", inv.layout.GitRoot, "worktree", "add", target, "goal/" + id}
	}
	return "", &intentResult{Outcome: intentRefused, code: 1,
		Summary: fmt.Sprintf("no worktree has the goal/%s branch checked out; nothing was built", id),
		next:    argv, nextReason: "check the goal branch out in its own worktree; this checkout and its uncommitted files stay as they are"}
}

// acceptedDesignPaths are the goal's accepted design records, as absolute
// paths in a stable order. A project that cannot be read is refused, never
// taken as a goal without a design.
func (inv *intentInvocation) acceptedDesignPaths(id string) ([]string, *intentResult) {
	designs, problem := inv.linkedDesigns(id)
	if problem != "" {
		return nil, &intentResult{Outcome: intentFailed, code: 1,
			Summary: "cannot read the project's design records, so the goal's design is unknown: " + problem + "; nothing was done"}
	}
	paths := []string{}
	for _, design := range designs {
		if design.Status != "accepted" {
			continue
		}
		// The project reader names a record inside the checkout relative to
		// it, and any other record by its absolute path.
		path := filepath.FromSlash(design.Path)
		if !filepath.IsAbs(path) {
			path = filepath.Join(inv.layout.GitRoot, path)
		}
		// One record can be reached through two homes spelled differently.
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			path = resolved
		}
		if !slices.Contains(paths, path) {
			paths = append(paths, path)
		}
	}
	slices.Sort(paths)
	return paths, nil
}

// unitSize finds the unit's row in the brief's or an accepted design's
// units table, the row build admission reads. It returns where the size
// comes from: "brief", "design:PATH", or "lines" for the caller's estimate.
func (inv *intentInvocation) unitSize(unit, brief string, designs []string) (string, int64, *intentResult) {
	var given int64
	if inv.input.has("lines") {
		value, err := strconv.ParseInt(inv.input.text("lines"), 10, 64)
		if err != nil || value <= 0 {
			return "", 0, &intentResult{Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("--lines must be a positive whole number of changed lines, not %s; nothing was done", shellCommand([]string{inv.input.text("lines")}))}
		}
		given = value
	}
	for index, page := range append([]string{brief}, designs...) {
		lines, err := launch.DeclaredUnitLines(page, unit)
		if err != nil {
			message := err.Error()
			if strings.Contains(message, "missing=units-table") || strings.Contains(message, "missing=row") || strings.Contains(message, "missing=size-column") {
				continue
			}
			return "", 0, &intentResult{Outcome: intentRefused, code: 1, Summary: fmt.Sprintf("the units table in %s: %v; nothing was built", page, err)}
		}
		if given != 0 && given != lines {
			return "", 0, &intentResult{Outcome: intentRefused, code: 2,
				Summary:  fmt.Sprintf("unit %s is declared at %d lines in %s but --lines says %d; nothing was built", unit, lines, page, given),
				Decision: "drop --lines to use the declared row, or correct the row"}
		}
		if index == 0 {
			return "brief", lines, nil
		}
		return "design:" + page, lines, nil
	}
	if given == 0 {
		return "", 0, &intentResult{Outcome: intentRefused, code: 1,
			Summary:  fmt.Sprintf("LAUNCH_BUILD_UNSIZED unit=%s: neither the brief nor an accepted design of the goal has a units row for it; nothing was built", unit),
			Decision: fmt.Sprintf("an honest estimate of unit %s's changed lines, given as --lines N, or a units table row in the brief", unit)}
	}
	return "lines", given, nil
}

func unitReadModel(runner *launch.UnitRunner) (string, *intentResult) {
	if runner.Manager == nil {
		return "", &intentResult{Outcome: intentFailed, code: 1, Summary: "unit launch manager is unavailable"}
	}
	if runner.Manager.SettingsError != nil {
		return "", &intentResult{Outcome: intentFailed, code: 1, Summary: "cannot read the launch settings: " + runner.Manager.SettingsError.Error()}
	}
	settings := runner.Manager.Settings
	if len(settings.Values) == 0 {
		settings = launch.DefaultSettings()
	}
	for _, value := range settings.Values {
		if value.Key == launch.ReadModelKey && strings.TrimSpace(value.Value) != "" {
			return value.Value, nil
		}
	}
	return "", &intentResult{Outcome: intentRefused, code: 1, Summary: "no independent read model is configured (" + launch.ReadModelKey + ")"}
}

type unitBinding struct {
	goal, unit, worktree, base, brief, unitsPage, findings string
	designs, check                                         []string
	estimate                                               bool
	lines                                                  int64
	rounds, toolCalls                                      int
}

// buildBrief is the caller's brief under the binding the run is held to.
// A caller's estimate comes first as the units table, so admission reads it
// before any table in the caller's text.
func (b unitBinding) buildBrief(brief []byte) string {
	var text strings.Builder
	fmt.Fprintf(&text, "# Unit binding\n\nUnit `%s` of goal `%s`, built by the unit runner. The caller's brief follows this binding.\n\n", b.unit, b.goal)
	if b.estimate {
		fmt.Fprintf(&text, "| Unit | Lines |\n| --- | ---: |\n| %s | %d |\n\n", b.unit, b.lines)
	}
	fmt.Fprintf(&text, "- Branch: goal/%s, checked out in %s\n- Base commit: %s\n", b.goal, b.worktree, b.base)
	if len(b.designs) == 0 {
		text.WriteString("- Design: no accepted design names this goal; the brief is the specification\n")
	}
	for _, design := range b.designs {
		fmt.Fprintf(&text, "- Design: %s, the accepted specification this unit builds within\n", design)
	}
	if b.estimate {
		fmt.Fprintf(&text, "- Size: %d changed lines, the caller's estimate\n", b.lines)
	} else {
		fmt.Fprintf(&text, "- Size: the %s row of the units table in %s (%d changed lines)\n", b.unit, b.unitsPage, b.lines)
	}
	argv, _ := json.Marshal(b.check)
	fmt.Fprintf(&text, "- Proof after the build, run without a shell as this argument vector: %s\n", argv)
	fmt.Fprintf(&text, "- Rounds: at most %d, the goal's approved review-round limit\n", b.rounds)
	fmt.Fprintf(&text, "- Caller's brief: %s\n\nLeave the change in the worktree, uncommitted. The run ends awaiting judgement; it is not approved or landed by the build.\n\n---\n\n", b.brief)
	text.Write(brief)
	return text.String()
}

var reviewPlaceholder = regexp.MustCompile(`<[^<>]+>`)

// readBrief fills the review template for this unit's independent read with
// the goal's approved round limit and the decided tool-call budget. A
// placeholder this command does not know stays in place, and the pack check
// that follows refuses it.
func (b unitBinding) readBrief(template []byte, buildBrief string) string {
	text := string(template)
	if cut := strings.Index(text, "## Scoped confirmation read after a fold"); cut >= 0 {
		text = text[:cut] + "## Scoped confirmation read after a fold\n\nNot this read. A follow-up round of this unit is read again by the unit runner with this brief.\n"
	}
	proof := shellCommand(b.check)
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		switch {
		case strings.HasPrefix(line, "1. `<file>"):
			lines[index] = "1. The unit's diff against base " + b.base + " — every change the build brief " + buildBrief + " requires is present, correct and tested, and nothing outside it changed."
		case strings.HasPrefix(line, "2. `<fixture file>"):
			lines[index] = "2. The proof command " + proof + " — its tests exercise the changed behavior rather than only the happy path."
		case strings.Contains(line, "VERDICT:"):
			lines[index] = strings.ReplaceAll(line, "<N>", "N")
		}
	}
	return reviewPlaceholder.ReplaceAllStringFunc(strings.Join(lines, "\n"), func(token string) string {
		inner := strings.Join(strings.Fields(strings.Trim(token, "<>")), " ")
		switch {
		case inner == "chain name":
			return "unit " + b.unit + " of goal " + b.goal
		case strings.HasPrefix(inner, "N focused rounds"):
			return fmt.Sprintf("%d focused rounds, the goal's approved review-round limit; the unit runner refuses a round past it", b.rounds)
		case strings.HasPrefix(inner, "who and what this review defends against"):
			return "trusted operators and agents working this goal; accidental defects, regressions, crashes and contract drift in the unit's diff are in scope, hostile inputs are not"
		case strings.HasPrefix(inner, "the files, behaviors, or contracts under review"):
			return "the diff of unit " + b.unit + " on goal/" + b.goal + " against " + b.base + ", judged against " + buildBrief + "; everything outside that diff is out"
		case inner == "absolute path to the reader's private copy":
			return b.worktree
		case strings.HasPrefix(inner, "commit SHA, or base and candidate identifying the exact diff"):
			return "base " + b.base + " and the candidate diff handed to this read"
		case inner == "N":
			return strconv.Itoa(b.toolCalls)
		case inner == "absolute findings file path":
			return b.findings
		}
		return token
	})
}

// unitOutcome renders a unit runner result. Awaiting judgement is the end
// of a build, green or red; the read's verdict is reported as the reader
// wrote it, and only VERDICT: land is a clean read. A capped wait is work
// still in progress with the command that continues it.
func (inv *intentInvocation) unitOutcome(runner *launch.UnitRunner, result launch.UnitResult, err error, targets []intentTarget, again []string) intentResult {
	record := result.Record
	if err != nil {
		message := err.Error()
		switch {
		case strings.HasPrefix(message, "UNIT_RUN_BUSY"):
			return intentResult{Outcome: intentInProgress, Targets: targets, code: 3, Summary: message,
				next: again, nextReason: "another call is advancing this run; the same command continues it"}
		case strings.HasPrefix(message, "UNIT_NAMED_INPUT_CHANGED"):
			return intentResult{Outcome: intentRefused, Targets: targets, code: 1, Summary: message + "; nothing was launched",
				Decision: "send the change to that run as a follow-up (metasystem fold unit RUN --brief FILE), or build it under another unit name"}
		case record.ID != "":
			return intentResult{Outcome: intentFailed, Targets: append(targets, intentTarget{Kind: "unit", ID: record.ID}), code: 1, Summary: message,
				Data: unitData(record, runner.Manager), next: inv.publicArgv("wait", "unit", record.ID), nextReason: "the run is recorded; continue it once the cause is fixed"}
		}
		return intentResult{Outcome: intentRefused, Targets: targets, code: 1, Summary: message}
	}
	targets = append(targets, intentTarget{Kind: "unit", ID: record.ID})
	data := unitData(record, runner.Manager)
	if result.Capped {
		return intentResult{Outcome: intentInProgress, Targets: targets, code: 3, Data: data,
			Summary: fmt.Sprintf("run %s round %d is still running step %s (launch %s)", record.ID, result.Round, result.Step, result.Launch),
			next:    inv.publicArgv("wait", "unit", record.ID), nextReason: "continue waiting for this run"}
	}
	round := record.Rounds[len(record.Rounds)-1]
	line := unitJudgementLine(record, runner.Manager)
	if round.Outcome == "read-compacted" {
		return intentResult{Outcome: intentFailed, Targets: targets, code: unitExitReadCompacted, Data: data, text: []string{line},
			Summary:  fmt.Sprintf("run %s round %d: the read was compacted and its verdict does not count", record.ID, round.Number),
			Decision: "judge the round; a follow-up round is metasystem fold unit " + record.ID + " --brief FILE",
			next:     inv.publicArgv("review", "unit", record.ID), nextReason: "the proof passed: commit this result and request its committed review"}
	}
	verdict := "no read"
	if verdicts, _ := data["readVerdicts"].([]string); len(verdicts) > 0 {
		verdict = strings.Join(verdicts, "; ")
	}
	text := []string{line}
	if clean, _ := data["readClean"].(bool); !clean && round.Outcome == "green" {
		text = append(text, "The build, proof and read processes finished, but the read did not return VERDICT: land; this is not a clean read.")
	}
	text = append(text, "The read is preliminary feedback for the author. Nothing is approved, certified or landed. A follow-up round: metasystem fold unit "+record.ID+" --brief FILE")
	judged := intentResult{Outcome: intentConfirmed, Targets: targets, Data: data, text: text,
		Summary: fmt.Sprintf("unit %s of %s: run %s round %d awaits judgement (%s; read verdict: %s)", record.Unit, record.Goal, record.ID, round.Number, round.Outcome, verdict)}
	if launch.UnitReviewReadyOutcomes[round.Outcome] {
		judged.next, judged.nextReason = inv.publicArgv("review", "unit", record.ID), "the proof passed: commit this result, publish it and request its committed review"
	}
	return judged
}

// unitData is the run's current round as recorded, with each counting
// read's verdict and its retained findings copies from the launch records,
// and the retained plan and round directory a later review can freeze.
func unitData(record launch.UnitRunRecord, manager *launch.Manager) map[string]any {
	data := map[string]any{"run": record.ID, "unit": record.Unit, "goal": record.Goal, "state": record.State, "worktree": record.Worktree,
		"base": record.Base, "plan": record.Plan, "maxRounds": record.MaxRounds, "buildModel": record.BuildModel, "buildEffort": record.BuildEffort}
	if len(record.Rounds) == 0 {
		return data
	}
	round := record.Rounds[len(record.Rounds)-1]
	data["round"], data["outcome"], data["steps"], data["directory"] = round.Number, round.Outcome, round.Steps, round.Directory
	verdicts, findings := []string{}, []string{}
	clean := false
	for index, step := range round.Steps {
		if !strings.HasPrefix(step.Name, "read") || step.State != launch.StepPassed || readStepHasCountingRerun(round.Steps, index) {
			continue
		}
		verdict := chooseUnitValue(step.Verdict, "none")
		verdicts = append(verdicts, verdict)
		if manager != nil && step.LaunchID != "" {
			if launchRecord, err := manager.Store.Read(step.LaunchID); err == nil {
				for _, output := range launchRecord.Outputs {
					findings = append(findings, output.Path)
				}
			}
		}
	}
	if len(verdicts) > 0 {
		clean = true
		for _, verdict := range verdicts {
			clean = clean && verdict == "land"
		}
	}
	data["readVerdicts"], data["readFindings"], data["readClean"] = verdicts, findings, clean
	return data
}

// fold unit

// fold unit

// runIntentFoldUnit sends the follow-up brief to the run through the unit
// runner's follow-up, which reuses the run's plan, proof and read and
// refuses a round past the run's approved limit.
func runIntentFoldUnit(inv *intentInvocation, run string) int {
	if len(inv.input.args) != 2 || !inv.input.has("brief") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "fold unit needs the run and --brief FILE; nothing was done", Decision: "metasystem fold unit RUN --brief FILE"})
	}
	brief := inv.callerPath(inv.input.text("brief"))
	runner := inv.unitRunner()
	result, err := runner.Continue(launch.UnitRequest{Resume: run, FollowUp: brief})
	return inv.render(inv.unitOutcome(runner, result, err, []intentTarget{{Kind: "unit", ID: run}}, inv.publicArgv("wait", "unit", run)))
}

// wait

func runIntentWait(inv *intentInvocation) int {
	if len(inv.input.args) != 2 || !slices.Contains([]string{"job", "goal", "unit"}, inv.input.args[0]) {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "wait takes a kind and its id: wait job J, wait goal G or wait unit RUN; nothing was done", Decision: "name what to wait for"})
	}
	kind, id := inv.input.args[0], inv.input.args[1]
	if kind != "goal" {
		for _, selector := range []string{"event", "after", "verb", "question", "chain"} {
			if inv.input.has(selector) {
				return inv.render(intentResult{Outcome: intentRefused, code: 2,
					Summary: fmt.Sprintf("--%s selects a goal event; wait %s takes only --timeout; nothing was done", selector, kind)})
			}
		}
	}
	var timeout time.Duration
	if inv.input.has("timeout") {
		value, err := time.ParseDuration(inv.input.text("timeout"))
		if err != nil || value <= 0 || value > 24*time.Hour {
			return inv.render(intentResult{Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("--timeout must be a positive duration of at most 24h, such as 10m, not %s; nothing was done", shellCommand([]string{inv.input.text("timeout")}))})
		}
		timeout = value
	}
	targets := []intentTarget{{Kind: kind, ID: id}}
	if kind == "unit" {
		runner := inv.unitRunner()
		if timeout > 0 && runner.Manager != nil {
			settings := runner.Manager.Settings
			if len(settings.Values) == 0 {
				settings = launch.DefaultSettings()
			}
			settings.WaitCapSeconds = int64(timeout / time.Second)
			runner.Manager.Settings = settings
		}
		result, err := runner.Continue(launch.UnitRequest{Resume: id})
		return inv.render(inv.unitOutcome(runner, result, err, targets, inv.publicArgv("wait", "unit", id)))
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	args := []string{"--root", inv.layout.InstallationRoot, "--" + kind, id}
	if kind == "goal" {
		selector := metarun.WaitSelector{Kind: "goal", TargetID: id, GoalID: id, Event: inv.input.text("event"), After: inv.input.text("after"),
			Verb: inv.input.text("verb"), Question: inv.input.text("question"), Chain: inv.input.text("chain")}
		if selector.Event == "" {
			selector.Event = "landing"
		}
		if selector.After == "" {
			projection, _, problem := inv.projection()
			if problem != nil {
				return inv.render(*problem)
			}
			if projection.Tip == "" {
				return inv.render(intentResult{Outcome: intentRefused, Targets: targets, code: 1,
					Summary:  "the accepted goal ledger has no tip to wait after; nothing was registered",
					Decision: "the accepted ledger commit to wait after, given as --after TIP"})
			}
			selector.After = projection.Tip
		}
		if err := metarun.ValidateWaitSelector(selector); err != nil {
			return inv.render(intentResult{Outcome: intentRefused, Targets: targets, code: metarun.ExitInvalidWait, Summary: err.Error() + "; nothing was registered"})
		}
		args = append(args, "--event", selector.Event, "--after", selector.After)
		for _, pair := range [][2]string{{"verb", selector.Verb}, {"question", selector.Question}, {"chain", selector.Chain}} {
			if pair[1] != "" {
				args = append(args, "--"+pair[0], pair[1])
			}
		}
	}
	if timeout > 0 {
		args = append(args, "--timeout", timeout.String())
	}
	var waited *metarun.WaitResult
	code := inv.work().wait(args, func(result metarun.WaitResult, _ bool) { waited = &result })
	if waited == nil {
		return inv.render(intentResult{Outcome: intentFailed, Targets: targets, code: max(code, 1),
			Summary: "the wait owner stopped before a result; its message is on standard error"})
	}
	outcome := intentResult{Targets: targets, code: waited.ExitCode, Data: waited,
		Summary: fmt.Sprintf("%s %s: %s", kind, id, strings.TrimSpace(strings.Join([]string{waited.SourceOutcome, waited.Reason}, " ")))}
	resume := []string{"metasystem", "wait", "--resume", waited.WaitID, "--root", inv.layout.InstallationRoot}
	switch waited.ExitCode {
	case metarun.ExitGreen, metarun.ExitRed, metarun.ExitEndedUnknown, metarun.ExitLaunchFailed:
		outcome.Outcome = intentConfirmed
	case metarun.ExitWaitDeadline, metarun.ExitInterrupted:
		outcome.Outcome = intentInProgress
		if waited.WaitID != "" {
			outcome.next, outcome.nextReason = resume, "the work continues; this continues the same wait"
		}
	case metarun.ExitNoRecord, metarun.ExitInvalidWait, metarun.ExitWaiterBusy:
		outcome.Outcome = intentRefused
	default:
		outcome.Outcome = intentFailed
	}
	return inv.render(outcome)
}

// test

// runIntentTest runs the selected installation's test runner as its own
// process and reports the structured result it prints; its progress goes to
// standard error unchanged.
func runIntentTest(inv *intentInvocation) int {
	if problem := inv.resolveLayout(); problem != nil {
		return inv.render(*problem)
	}
	argv := []string{"test", "run", "--json", "--root", inv.layout.InstallationRoot}
	targets := []intentTarget{}
	for _, name := range []string{"goal", "authority", "mode"} {
		if inv.input.has(name) {
			argv = append(argv, "--"+name, inv.input.text(name))
		}
	}
	if inv.input.has("goal") {
		targets = append(targets, intentTarget{Kind: "goal", ID: inv.input.text("goal")})
	}
	output, code, err := inv.work().subprocess(inv.layout.GitRoot, argv, inv.stderr)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, Targets: targets, code: 1, Summary: "cannot run the test runner: " + err.Error()})
	}
	result := intentResult{Targets: targets, code: code}
	if trimmed := bytes.TrimSpace(output); json.Valid(trimmed) && len(trimmed) > 0 {
		result.Data = json.RawMessage(trimmed)
	} else if len(trimmed) > 0 {
		result.text = []string{string(trimmed)}
	}
	switch code {
	case 0:
		result.Outcome, result.Summary = intentConfirmed, "the selected tests passed"
	case proofrun.ExitAdmissionRefused:
		result.Outcome, result.Summary = intentRefused, "the test runner refused admission; its reason is on standard error"
	case 2:
		result.Outcome, result.Summary = intentRefused, "the test runner refused its arguments; its reason is on standard error"
	default:
		result.Outcome, result.Summary = intentFailed, fmt.Sprintf("the test runner exited %d; its result is in data and its reason on standard error", code)
	}
	return inv.render(result)
}

// settings

// runIntentSettings reads the selected installation's metasystem.conf
// through the launch settings reader and, for any other key, the
// configuration reader, each with the source of its value.
func runIntentSettings(inv *intentInvocation) int {
	if problem := inv.resolveLayout(); problem != nil {
		return inv.render(*problem)
	}
	confPath := intentConfPath(inv.layout)
	settings, err := inv.work().settings(confPath)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "cannot read the launch settings of " + confPath + ": " + err.Error()})
	}
	values := settings.Values
	if len(inv.input.args) == 1 {
		key := inv.input.args[0]
		index := slices.IndexFunc(values, func(value launch.Setting) bool { return value.Key == key })
		if index >= 0 {
			values = values[index : index+1]
		} else {
			value, source, code, err := inv.work().config(key, confPath)
			if err != nil || code != 0 {
				reason := fmt.Sprintf("there is no setting %s in %s", shellCommand([]string{key}), confPath)
				if err != nil {
					reason += ": " + err.Error()
				}
				return inv.render(intentResult{Outcome: intentRefused, code: 1, Summary: reason,
					next: inv.publicArgv("settings"), nextReason: "list the launch settings"})
			}
			values = []launch.Setting{{Key: key, Value: value, Source: source}}
		}
	}
	text := make([]string, 0, len(values))
	for _, value := range values {
		text = append(text, fmt.Sprintf("%s=%s (%s)", value.Key, value.Value, value.Source))
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Data: map[string]any{"conf": confPath, "settings": values}, text: text,
		Summary: fmt.Sprintf("%d setting(s) of %s", len(values), inv.layout.InstallationRoot)})
}

// brief

func runIntentBrief(inv *intentInvocation) int {
	id, problem := inv.singleTarget()
	if problem != nil {
		return inv.render(*problem)
	}
	if id == "" || !inv.input.has("out") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "brief needs the goal and --out FILE; nothing was written", Decision: "metasystem brief G --out FILE"})
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return inv.render(*problem)
	}
	file, where := goalRecord(projection, id)
	if file == nil {
		return unknownGoal(inv, id)
	}
	targets := inv.targets(id)
	if where != "live" {
		return inv.render(intentResult{Outcome: intentRefused, Targets: targets, code: 1, Summary: fmt.Sprintf("goal %s is %s; no brief was written", id, where)})
	}
	designs, problem := inv.acceptedDesignPaths(id)
	if problem != nil {
		problem.Targets = targets
		return inv.render(*problem)
	}
	text, missing, problem := inv.briefScaffold(file, designs)
	if problem != nil {
		problem.Targets = targets
		return inv.render(*problem)
	}
	out := inv.callerPath(inv.input.text("out"))
	data := map[string]any{"path": out, "designs": designs, "missingDecisions": missing}
	if existing, err := os.ReadFile(out); err == nil {
		if string(existing) == text {
			return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: data, text: missing,
				Summary: fmt.Sprintf("%s already holds this brief", out)})
		}
		return inv.render(intentResult{Outcome: intentRefused, Targets: targets, code: 1,
			Summary:  fmt.Sprintf("%s already exists with other content; nothing was written", out),
			Decision: "name a new --out FILE; the existing file is kept"})
	}
	if _, err := atomicfile.WriteText(out, text, filepath.Dir(out)); err != nil {
		return inv.render(intentResult{Outcome: intentFailed, Targets: targets, code: 1, Summary: "cannot write the brief: " + err.Error()})
	}
	summary := fmt.Sprintf("wrote the brief for %s to %s", id, out)
	if len(missing) > 0 {
		summary += fmt.Sprintf("; %d decision(s) are missing and marked", len(missing))
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: data, text: missing, Summary: summary})
}

// designSection is one headed section of a design record, carried into a
// brief by the kind of decision its heading names.
type designSection struct {
	path, heading, text string
}

var (
	designConstraintHeading = regexp.MustCompile(`(?i)non-goal|constraint|scope|limit|out of scope`)
	designReturnHeading     = regexp.MustCompile(`(?i)\breturn`)
	designAcceptHeading     = regexp.MustCompile(`(?i)accept|proof|obligation|criteria|done`)
)

// designSections splits a design record at every heading and sorts
// the ones whose headings name constraints, the expected return or
// acceptance. The record's own text is carried; nothing is inferred.
func designSections(path string, data []byte) (constraints, returns, acceptance []designSection) {
	var current *designSection
	flush := func() {
		if current == nil || strings.TrimSpace(current.text) == "" {
			return
		}
		switch {
		case designAcceptHeading.MatchString(current.heading):
			acceptance = append(acceptance, *current)
		case designReturnHeading.MatchString(current.heading):
			returns = append(returns, *current)
		case designConstraintHeading.MatchString(current.heading):
			constraints = append(constraints, *current)
		}
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "#") {
			flush()
			current = &designSection{path: path, heading: strings.TrimSpace(strings.TrimLeft(line, "#"))}
			continue
		}
		if current != nil {
			current.text += line + "\n"
		}
	}
	flush()
	return constraints, returns, acceptance
}

// briefScaffold writes what the ledger and the accepted design records
// hold, and a MISSING DECISION line only for a decision neither holds.
func (inv *intentInvocation) briefScaffold(file *goal.GoalFile, designs []string) (string, []string, *intentResult) {
	var missing []string
	mark := func(decision string) string {
		missing = append(missing, decision)
		return intentMissingDecision + " " + decision
	}
	var constraints, returns, acceptance []designSection
	var units []launch.UnitSize
	unitsFrom := ""
	for _, design := range designs {
		data, err := os.ReadFile(design)
		if err != nil {
			return "", nil, &intentResult{Outcome: intentFailed, code: 1, Summary: "cannot read the accepted design: " + err.Error() + "; no brief was written"}
		}
		c, r, a := designSections(design, data)
		constraints, returns, acceptance = append(constraints, c...), append(returns, r...), append(acceptance, a...)
		if rows, err := launch.DeclaredUnits(design); err == nil && len(rows) > 0 && len(units) == 0 {
			units, unitsFrom = rows, design
		}
	}
	section := func(text *strings.Builder, sections []designSection) {
		for _, one := range sections {
			fmt.Fprintf(text, "From %s, \"%s\":\n\n%s\n", one.path, one.heading, strings.TrimSpace(one.text))
			text.WriteString("\n")
		}
	}
	var text strings.Builder
	fmt.Fprintf(&text, "# Brief: %s\n\n", file.Id)
	switch {
	case (file.State == goal.StateApproved || file.State == goal.StateClaimed) && file.Budget != nil:
		fmt.Fprintf(&text, "Goal state: %s, tier %d, approved box %s; the read has at most %d rounds.\n", file.State, file.Tier, goalbudget.FormatBox(*file.Budget), file.Budget.ReviewRoundLimit)
	default:
		fmt.Fprintf(&text, "Goal state: %s, tier %d.\n%s\n", file.State, file.Tier,
			mark(fmt.Sprintf("the goal is %s without an approved box; a person approves it with metasystem approve %s", file.State, file.Id)))
	}
	fmt.Fprintf(&text, "\n# Goal\n\n%s\n\nNext step on the ledger: %s\n", file.Intent, file.NextStep)
	text.WriteString("\n# Workspace\n\n")
	worktree, problem := inv.goalWorktree(file.Id)
	switch {
	case problem == nil:
		fmt.Fprintf(&text, "Branch goal/%s, checked out in %s. Leave the change there, uncommitted.\n", file.Id, worktree)
	case len(problem.next) > 0:
		fmt.Fprintf(&text, "Branch goal/%s. %s\n", file.Id, mark("no worktree has goal/"+file.Id+" checked out; prepare it with: "+shellCommand(problem.next)))
	default:
		fmt.Fprintf(&text, "Branch goal/%s. %s\n", file.Id, mark("the goal worktree could not be found: "+problem.Summary))
	}
	text.WriteString("\n# Inputs\n\n")
	if len(designs) == 0 {
		text.WriteString(mark("no accepted design names this goal; name the specification this brief builds") + "\n")
	}
	for _, design := range designs {
		fmt.Fprintf(&text, "- Design: %s (accepted; the specification this brief builds)\n", design)
	}
	text.WriteString("\n# Units\n\n")
	if len(units) == 0 {
		text.WriteString(mark("the units and each unit's changed-line estimate (a | Unit | Lines | table)") + "\n")
	} else {
		fmt.Fprintf(&text, "From %s:\n\n| Unit | Lines |\n| --- | ---: |\n", unitsFrom)
		for _, unit := range units {
			fmt.Fprintf(&text, "| %s | %d |\n", unit.Name, unit.Lines)
		}
	}
	text.WriteString("\n# Constraints\n\n")
	switch {
	case len(constraints) > 0:
		section(&text, constraints)
	case len(designs) > 0:
		text.WriteString("The accepted design above is the specification; build within its scope and limits.\n")
	default:
		text.WriteString(mark("non-goals, what must not be touched, and time and token budgets") + "\n")
	}
	text.WriteString("\nThe independent read's tool-call budget:\n" + mark("the read's tool-call budget, written as the line 'Maximum reader tool calls: N'") + "\n")
	text.WriteString("\n# Expected Return\n\n")
	if len(returns) > 0 {
		section(&text, returns)
	} else {
		text.WriteString("The change, uncommitted in the goal worktree. The unit runner records the diff, runs the proof and the independent read,\nand keeps the run awaiting judgement.\n")
	}
	text.WriteString("\n# Acceptance Criteria\n\n")
	done := ""
	if cut := strings.Index(file.Intent, "DONE:"); cut >= 0 {
		done = strings.TrimSpace(file.Intent[cut:])
	}
	if done != "" {
		fmt.Fprintf(&text, "The goal's own criteria: %s\n\n", done)
	}
	section(&text, acceptance)
	if done == "" && len(acceptance) == 0 {
		text.WriteString(mark("observable, machine-checkable criteria; neither the goal nor an accepted design states them") + "\n")
	}
	text.WriteString("\n# Gap Rule\n\nstop and report a gap; never fill it silently.\n")
	return text.String(), missing, nil
}
