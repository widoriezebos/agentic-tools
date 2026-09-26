package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stoptransition"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
)

// The process and question commands: start, stop, restart and read this
// checkout's machinery, enroll a terminal, ask and answer questions, and read
// the fleet and its health. Each target kind keeps its own owner: the checkout
// stop transition, the quiet session stop, one job's cancellation, the
// mission answer transition and the authenticated channel. Nothing here
// decides authority; the owners do, and their refusals are rendered.

var (
	intentUIListenFlag     = intentFlag{name: "listen", value: "ADDRESS", usage: "ui: loopback IP and port (default: configured address)"}
	intentUIWaitFlag       = intentFlag{name: "wait-seconds", value: "N", usage: "ui: seconds to wait for shutdown (default: 15; zero checks immediately)"}
	intentInstallationFlag = intentFlag{name: "installation", value: "DIR", advanced: true,
		usage: "the metasystem installation of this checkout, when it is not the one found from --repo"}
	intentTemporaryArmFlag = intentFlag{name: "temporary-human-word", value: "WORD", advanced: true,
		usage: "recorded relayed words presented as the human's; arms TEMPORARILY"}
)

func processIntentCommands() []intentCommand {
	return []intentCommand{
		{
			name: "start", group: "operations", audience: "both", summary: "start assigned work or MetaSystem services",
			usage: []string{"metasystem start session", "metasystem start mission M"},
			administrationUsage: []string{"metasystem start [checkout]", "metasystem start ui [--listen ADDRESS]",
				"metasystem start machine NAME [--destination PATH] [--resume ID]"},
			details: []string{
				"start (or start checkout) is done by a person at their enrolled terminal. New work may start again, and the helpers that watch and continue work are started.",
				"A terminal that stays enrolled keeps its authority when the engine is rebuilt; no new enrollment is needed.",
				"start session prepares the current agent session to work.",
				"start mission M starts that autonomous mission.",
				"start ui starts the browser interface. start machine NAME clones, builds, configures, enrolls and supervises one new",
				"machine of this fleet on this host (a person's act; --resume ID continues an interrupted launch).",
			},
			flags: []intentFlag{intentUIListenFlag, intentInstallationFlag, intentTemporaryArmFlag, intentReviewByFlag, {name: "lineage", value: "LINEAGE", advanced: true, hidden: true, usage: "start session: the session's owner lineage"},
				{name: "destination", value: "PATH", advanced: true, usage: "start machine: where the new machine's clone lands"},
				{name: "resume", value: "ID", advanced: true, usage: "start machine: continue this interrupted launch"}},
			maxArgs:  2,
			examples: []string{"metasystem start session", "metasystem start", "metasystem start ui", "metasystem start machine m1f"},
			run:      runIntentStart,
		},
		{
			name: "stop", group: "operations", audience: "both", summary: "stop selected work, a session, or MetaSystem",
			usage: []string{"metasystem stop job J", "metasystem stop session --by NAME", "metasystem stop review REF",
				"metasystem stop design G [--out FILE] [--attempt N]"},
			administrationUsage: []string{"metasystem stop [checkout]", "metasystem stop ui [--wait-seconds N]"},
			details: []string{
				"stop (or stop checkout) is done by a person at their enrolled terminal. Every job and helper of this checkout stops, and no new work starts until start.",
				"If something keeps running, stop says what, and nothing is reported as stopped that is not.",
				"stop job J cancels exactly the selected job J; nothing else stops.",
				"stop session authorizes one quiet stop of the announced main session; the checkout keeps running.",
			},
			flags: []intentFlag{intentUIWaitFlag, intentInstallationFlag, {name: "by", value: "NAME", usage: "stop session: the attending person"},
				{name: "out", value: "FILE", usage: "stop design: the design document, when the goal has several"},
				{name: "attempt", value: "N", usage: "stop design: this attempt instead of the newest"}},
			maxArgs:  2,
			examples: []string{"metasystem stop job design-r2-4f1c", "metasystem stop", "metasystem stop session --by Wido"},
			run:      runIntentStop,
			legacy:   legacyProcessCall,
		},
		{
			name: "restart", group: "administration", audience: "both", summary: "restart MetaSystem for this checkout, or its interface",
			usage: []string{"metasystem restart checkout", "metasystem restart ui [--listen ADDRESS] [--wait-seconds N]"},
			details: []string{
				"restart checkout starts again only after everything stopped; if either half fails, it says where it got to and what to run next.",
				"restart ui restarts the browser interface with the executable on disk; an agent may do this within its authorization.",
				"restart checkout requires enrolled-terminal human authority. Starting the interface through an agent does not authenticate a person for its human actions.",
			},
			flags:    []intentFlag{intentUIListenFlag, intentUIWaitFlag, intentInstallationFlag, intentTemporaryArmFlag, intentReviewByFlag},
			maxArgs:  1,
			examples: []string{"metasystem restart checkout", "metasystem restart ui"},
			run:      runIntentRestart,
		},
		{
			name: "status", group: "work", primary: true, audience: "both", summary: "what is running: a goal's work, this checkout, or one job",
			usage:               []string{"metasystem status G [--work NAME]", "metasystem status goal G [--work NAME]", "metasystem status job J", "metasystem status work [--all]", "metasystem status run RUN", "metasystem status mission M"},
			administrationUsage: []string{"metasystem status [checkout]", "metasystem status ui", "metasystem status --machines [--refresh]"},
			details: []string{
				"Unknown and stale readings are reported as such; a read failure is never shown as stopped or healthy.",
				"status G lists every named work item of the goal with its stage and the command that continues it; show G is the goal's record.",
				"Only work visible to the current user is shown; a job reference is matched exactly, never by prefix.",
			},
			flags: []intentFlag{intentInstallationFlag, {name: "work", value: "NAME", usage: "with G: only this named work"},
				{name: "machines", usage: "every machine's presence, this one first"},
				{name: "all", usage: "with status work: ended jobs too"},
				{name: "refresh", aliases: []string{"fetch"}, usage: "with --machines: fetch presence now instead of the last copy"}},
			maxArgs:  2,
			examples: []string{"metasystem status verbs-match-intent", "metasystem status", "metasystem status job design-r2-4f1c"},
			run:      runIntentStatus,
			legacy:   legacyProcessCall,
		},
		{
			name: "enroll", group: "administration", audience: "human", summary: "authenticate your terminal for human decisions in MetaSystem",
			usage:    []string{"metasystem enroll --name NAME"},
			details:  []string{"Run it at an agent-free terminal. The local enrollment and its fleet publication are reported separately."},
			flags:    []intentFlag{{name: "name", aliases: []string{"by"}, value: "NAME", usage: "your name"}, intentLineageFlag},
			maxArgs:  0,
			examples: []string{"metasystem enroll --name Wido"},
			run:      runIntentEnroll,
		},
		{
			name: "ask", group: "questions", primary: true, audience: "agent", summary: "ask the person a question through the channel",
			usage: []string{"metasystem ask G --question TEXT --option TEXT...", "metasystem ask G --question TEXT --option 'LABEL: CONSEQUENCE'... [--recommend LABEL]",
				"metasystem ask --retry Q", "metasystem ask --withdraw Q --reason TEXT"},
			details: []string{
				"The person answers in the channel thread; the answer is authenticated there, never by a local command.",
				"--kind selects an authority question (stop, budget-above-norm, carry); stop and budget-above-norm take --budget BOX.",
				"When delivery failed, ask --retry Q delivers exactly that stored question once more;",
				"it never asks a new question and touches no other. ask --withdraw Q --reason TEXT withdraws it the same way.",
				"show question Q shows its state and how it is answered; wait question Q waits for its answer.",
			},
			flags: []intentFlag{
				intentTargetFlag,
				{name: "question", value: "TEXT", usage: "the question, the first line the person reads"},
				fileFlag("question", "read the question from FILE"),
				{name: "option", value: "TEXT", repeat: true, usage: "one answer, as 'label: consequence' (repeatable)"},
				{name: "option-file", value: "FILE", repeat: true, advanced: true, usage: "read one answer from each FILE; answers keep the order given (repeatable)"},
				{name: "recommend", value: "LABEL", usage: "the option you recommend"},
				{name: "fact", value: "TEXT", repeat: true, advanced: true, usage: "a fact the person needs (repeatable)"},
				{name: "fact-file", value: "FILE", repeat: true, advanced: true, usage: "read one fact from each FILE; facts keep the order given (repeatable)"},
				{name: "kind", value: "KIND", advanced: true, usage: "an authority question: stop, budget-above-norm or carry"},
				{name: "wants", value: "TOKEN", advanced: true, usage: "the exact answer token (carry questions)"},
				{name: "budget", value: "BOX", advanced: true, usage: "the proposed compact box for stop and budget-above-norm"},
				{name: "retry", value: "Q", usage: "deliver this existing, undelivered question again; no new question is asked"},
				{name: "withdraw", value: "Q", usage: "withdraw this question, with --reason"},
				reasonFlag("because", "with --withdraw: why the question is withdrawn"),
			},
			maxArgs:  1,
			examples: []string{"metasystem ask verbs-match-intent --question 'Land slice 2 now?' --option 'yes: land it' --option 'no: wait for review' --recommend yes"},
			run:      runIntentAsk,
		},
		{
			name: "answer", group: "questions", audience: "human", summary: "answer a question, or see where a channel question is answered",
			usage: []string{"metasystem answer Q [TEXT]", "metasystem answer M/Q TEXT", "metasystem answer Q --answer-file FILE"},
			details: []string{
				"Q is a channel question or a mission's question; when both share the id, channel:Q or M/Q names one.",
				"TEXT answers a mission question: the mission records it (or changes nothing), then resumes when its requirements are satisfied.",
				"Repeating the same answer completes an interrupted resume; a different answer to an answered question is refused.",
				"A channel question is answered in its channel thread, which authenticates the person; answer Q shows where and how.",
				"A mission answer applies the answer and advances the mission, or changes nothing.",
				"--answer-file reads the answer from FILE, exactly, less its final line end; the same answer may also be given inline.",
				"A channel question is answered in its channel thread; answer question Q prints how, and records nothing.",
			},
			flags:    []intentFlag{{name: "answer-file", value: "FILE", advanced: true, usage: "read the mission answer from FILE"}},
			maxArgs:  4,
			examples: []string{"metasystem answer host-failure 'retry: the host is back'", "metasystem answer demo/host-failure --answer-file answer.md", "metasystem answer q-20260925-1"},
			run:      runIntentAnswer,
		},
		{
			name: "check", group: "operations", primary: true, audience: "both", summary: "diagnose problems with this checkout, changing nothing",
			usage:               []string{"metasystem check goals"},
			administrationUsage: []string{"metasystem check", "metasystem check settings"},
			details: []string{"Checks this checkout once and repairs nothing. Each problem names the command that fixes it, where there is one.",
				"check goals lists the goal files that differ from their published base: the edits repair goals --accept-edits would publish.",
				"check settings validates every setting of the selected installation's configuration and changes nothing."},
			flags:    []intentFlag{intentInstallationFlag},
			maxArgs:  1,
			examples: []string{"metasystem check goals", "metasystem check", "metasystem check --json", "metasystem check settings"},
			run:      runIntentCheck,
		},
	}
}

// legacyProcessCall keeps the old top-level stop, status and arm-style calls
// on their own parsers: no target kind and none of the public-only options.
func legacyProcessCall(args []string) bool {
	for index := 0; index < len(args); index++ {
		switch arg := args[index]; {
		case slices.Contains([]string{"checkout", "job", "session", "run", "design", "goal", "--json", "-json", "--help", "-help", "-h", "--work", "-work",
			"--machines", "-machines", "--refresh", "-refresh", "--fetch", "-fetch"}, arg):
			return false
		case slices.Contains([]string{"--repo", "-repo", "--installation", "-installation"}, arg):
			index++
		case !strings.HasPrefix(arg, "-"):
			// A goal named after the verb is the public status G.
			return false
		}
	}
	return true
}

// processIntentOwners are the owners the process and question commands call.
type processIntentOwners struct {
	process        processOwners
	up             func(up.Options) up.Result
	health         func(repo, installation string, now time.Time) steward.HealthVerdict
	healthNow      func(root string) (time.Time, error)
	fleet          func(root string, fetch bool, now time.Time) (seat.Report, error)
	launches       func() *launch.Manager
	cancelDispatch func(checkout, job string) (map[string]any, int, error)
	sessionStop    func(stateRoot, by string) (goal.SessionStop, string, int)
	enroll         goalTerminalEnroller
	ask            func(root string, in channelAskInput) (channel.Question, []string, int, error)
	question       func(root, id string) (channel.Question, error)
	mission        func(root, mission string) (*missionrunner.Engine, error)
	channelLink    func(root string) (channel.Provider, channel.DestinationConfig)
	ui             func(verb string, roots lifecycle.Roots, options uiIntentOptions) (uiLifecycleResult, error)
	executable     func() (string, error)
}

func defaultProcessIntentOwners() processIntentOwners {
	return processIntentOwners{
		process: defaultProcessOwners(),
		up:      up.Run,
		health: func(repo, installation string, now time.Time) steward.HealthVerdict {
			return steward.PreviewHealthAt(repo, installation, now, nil)
		},
		healthNow: func(root string) (time.Time, error) {
			now, ok, err := stewardFixtureNow(root)
			if err != nil || ok {
				return now, err
			}
			return time.Now().UTC(), nil
		},
		fleet:          seatFleetReport,
		launches:       newLaunchManager,
		cancelDispatch: cancelDispatchJob,
		sessionStop:    authorizeSessionStop,
		enroll:         humanauthority.Enroll,
		ask:            askChannelQuestion,
		question:       channel.ReadQuestion,
		mission:        missionRunnerCommandEngine,
		channelLink: func(root string) (channel.Provider, channel.DestinationConfig) {
			link, _ := phase.Load(root, false)
			return link.Provider, link.Destination
		},
		ui:         uiLifecycleFor,
		executable: os.Executable,
	}
}

// cancelDispatchJob cancels one dispatch job through its delegate owner and
// returns that owner's typed JSON outcome.
func cancelDispatchJob(checkout, job string) (map[string]any, int, error) {
	var stdout, stderr bytes.Buffer
	code := runDelegateIn([]string{"--cancel", job}, checkout, &stdout, &stderr)
	var outcome map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &outcome); err != nil {
		return nil, code, fmt.Errorf("the delegate owner returned no typed outcome: %v; %s", err, strings.TrimSpace(stderr.String()))
	}
	return outcome, code, nil
}

// selectProcessScope resolves the repository once for a process owner: its
// Git top, its installation and the engine that installation carries. A
// missing or unreadable installation is refused.
// selectInstallation finds the checkout and the installation a process or
// interface command acts on. A named --installation is resolved first, so a
// caller at the repository top can name an installation nested below it; it
// must belong to the repository selected by --repo or the current directory.
// Without it, the layout found from that path decides.
func (inv *intentInvocation) selectInstallation() (stateroot.Layout, string, bool, *intentResult) {
	path := inv.cwd
	if inv.input.has("repo") {
		path = inv.input.text("repo")
		if !filepath.IsAbs(path) {
			path = filepath.Join(inv.cwd, path)
		}
	}
	if !inv.input.has("installation") {
		layout, err := inv.owners.resolver.ResolveLayout(path)
		if err != nil {
			return stateroot.Layout{}, "", false, &intentResult{Outcome: intentRefused, code: 2,
				Summary:  fmt.Sprintf("%s is not inside one metasystem installation: %v", shellCommand([]string{path}), err),
				Decision: "run this inside the repository, name it with --repo PATH, or name its installation with --installation DIR"}
		}
		return layout, layout.InstallationRoot, false, nil
	}
	named := inv.input.text("installation")
	if !filepath.IsAbs(named) {
		named = filepath.Join(inv.cwd, named)
	}
	installation, err := canonicalPath(named)
	if err != nil {
		return stateroot.Layout{}, "", false, &intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--installation %s: %v", shellCommand([]string{named}), err)}
	}
	owned, ownedErr := inv.owners.resolver.ResolveLayout(installation)
	if ownedErr != nil || !sameCanonicalPath(owned.InstallationRoot, installation) {
		return stateroot.Layout{}, "", false, &intentResult{Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("%s is not a metasystem installation; nothing was done", shellCommand([]string{installation})),
			Decision: "name the directory that holds the installation's metasystem.conf with --installation DIR"}
	}
	selected, topErr := inv.owners.processes.process.repositoryTop(path)
	if topErr != nil || !sameCanonicalPath(owned.GitRoot, selected) {
		checkout := path
		if topErr == nil {
			checkout = selected
		}
		return stateroot.Layout{}, "", false, &intentResult{Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("the installation %s does not belong to the checkout %s; nothing was done", shellCommand([]string{installation}), shellCommand([]string{checkout})),
			Decision: "name an installation inside this checkout, or select its checkout with --repo PATH"}
	}
	return owned, installation, true, nil
}

// selectProcessScope resolves the repository once for a process owner: its
// Git top, its installation and the engine that installation carries. A
// missing, foreign or unreadable installation is refused.
func (inv *intentInvocation) selectProcessScope() (processScope, int, *intentResult) {
	layout, installation, explicit, problem := inv.selectInstallation()
	if problem != nil {
		return processScope{}, 0, problem
	}
	inv.layout = layout
	var err error
	binary := filepath.Join(installation, "bin", "metasystem")
	if !regularFile(binary) {
		return processScope{}, 0, &intentResult{Outcome: intentRefused, code: 1,
			Summary:  fmt.Sprintf("the installation %s carries no engine at bin/metasystem", shellCommand([]string{installation})),
			Decision: "build and install this checkout's engine, or name its installation with --installation DIR"}
	}
	if inv.stateRoot, err = inv.owners.resolver.RootForInstallation(installation); err != nil {
		return processScope{}, 0, &intentResult{Outcome: intentRefused, code: 1, Summary: "the installation's state root cannot be resolved: " + err.Error()}
	}
	scale := upWaitScale()
	if scale < 1 {
		return processScope{}, 0, &intentResult{Outcome: intentRefused, code: 2, Summary: "METASYSTEM_FIXTURE_CAP_SCALE_MILLI must be a positive integer"}
	}
	// Process records (the stop fence, the steward's runner and health) live
	// under the state root: the installation of a template checkout, the
	// application repository of an adopted one.
	scope := processScope{Checkout: layout.GitRoot, Installation: installation, InstallationExplicit: explicit, Root: inv.stateRoot, Binary: binary}
	return scope, scale, nil
}

// kindAndTarget reads VERB [KIND] [TARGET] where a bare verb means this checkout.
func (inv *intentInvocation) kindAndTarget(kinds ...string) (string, string, *intentResult) {
	args := inv.input.args
	if len(args) == 0 {
		return "checkout", "", nil
	}
	kind := args[0]
	if !slices.Contains(kinds, kind) {
		return "", "", &intentResult{Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("does not know the target %s; it takes %s. Nothing was done", shellCommand([]string{kind}), strings.Join(inv.command.allUsage(), " | ")),
			Decision: "name one of the targets above"}
	}
	target := ""
	if len(args) > 1 {
		target = args[1]
	}
	needsID := kind == "job" || kind == "run"
	switch {
	case needsID && target == "":
		return "", "", &intentResult{Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("%s %s needs the %s id; nothing was done", inv.command.name, kind, kind),
			Decision: fmt.Sprintf("name it: metasystem %s %s ID", inv.command.name, kind)}
	case !needsID && target != "":
		return "", "", &intentResult{Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("%s %s takes no id; unexpected %s. Nothing was done", inv.command.name, kind, shellCommand([]string{target}))}
	}
	return kind, target, nil
}

func (inv *intentInvocation) checkoutTarget(scope processScope) []intentTarget {
	return []intentTarget{{Kind: "checkout", ID: scope.Checkout}}
}

// processReportResult renders a transition report: its lines, and its own
// exit code, which stays nonzero for an incomplete or unknown reading.
func processReportResult(targets []intentTarget, summary string, report stoptransition.Report) intentResult {
	return intentResult{Outcome: intentConfirmed, Targets: targets, Summary: summary, text: report.Lines, code: report.ExitCode,
		Data: map[string]any{"lines": nonNilLines(report.Lines), "exitCode": report.ExitCode}}
}

func nonNilLines(lines []string) []string {
	if lines == nil {
		return []string{}
	}
	return lines
}

// processRefusalResult keeps the owner's sentence and its own next line; a
// human act the caller cannot perform from here is named, never run.
func processRefusalResult(targets []intentTarget, prefix string, refusal *processRefusal, report stoptransition.Report) intentResult {
	sentence := strings.TrimSuffix(strings.TrimSpace(refusal.sentence), ".")
	if refusal.plain != "" && sentence == "" {
		sentence = refusal.plain
	}
	result := intentResult{Outcome: intentRefused, Targets: targets, Summary: prefix + sentence + "; nothing was changed by this step",
		text: report.Lines, code: max(refusal.code, 1),
		Data: map[string]any{"lines": nonNilLines(report.Lines), "remedy": refusal.second}}
	if refusal.second != "" {
		result.Decision = refusal.second
	}
	return result
}

func runIntentStart(inv *intentInvocation) int {
	if problem := inv.checkUIOptionsTarget(); problem != nil {
		return inv.render(*problem)
	}
	if args := inv.input.args; len(args) == 2 && args[0] == "mission" {
		return runIntentMissionTarget(inv, "start", args[1])
	} else if len(args) == 1 && args[0] == "ui" {
		return inv.uiTarget("start")
	} else if len(args) == 2 && args[0] == "machine" {
		return runIntentStartMachine(inv, args[1])
	}
	for _, only := range []string{"destination", "resume"} {
		if inv.input.has(only) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--%s belongs to start machine NAME; nothing was done", only)})
		}
	}
	kind, _, problem := inv.kindAndTarget("checkout", "session")
	if problem != nil {
		return inv.render(*problem)
	}
	scope, scale, problem := inv.selectProcessScope()
	if problem != nil {
		return inv.render(*problem)
	}
	owners := inv.owners.processes
	if kind == "session" {
		if inv.input.has("temporary-human-word") || inv.input.has("review-by") {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "start session is an agent's start and takes no human word; nothing was done",
				Decision: "a person starts the checkout with metasystem start"})
		}
		binary, err := owners.executable()
		if err == nil {
			binary, err = canonicalPath(binary)
		}
		if err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "cannot resolve the running engine: " + err.Error()})
		}
		result := owners.up(up.Options{
			Root: scope.Installation, MetasystemRoot: scope.Installation, Scope: scope.Checkout, Binary: binary,
			OwnerLineage: inv.input.text("lineage"), WaitScaleMilli: scale, CallerPid: int64(os.Getppid()),
			RestampStopCapability: restampStopCapabilityForUp,
		})
		outcome := intentConfirmed
		if result.ExitCode() != 0 {
			outcome = intentRefused
		}
		return inv.render(intentResult{Outcome: outcome, Targets: []intentTarget{{Kind: "session", ID: scope.Checkout}}, code: result.ExitCode(),
			Summary: "session start: " + result.Outcome, text: result.Lines(), Decision: result.Remedy,
			Data: map[string]any{"outcome": result.Outcome, "lines": nonNilLines(result.Lines()), "remedy": result.Remedy}})
	}
	before, _ := processFence(scope)
	report, refusal := owners.process.arm(scope, scale, inv.input.text("temporary-human-word"), inv.input.text("review-by"))
	if refusal != nil {
		return inv.render(inv.startRefusal(scope, scale, before, refusal, report))
	}
	return inv.render(processReportResult(inv.checkoutTarget(scope), "started "+scope.Checkout, report))
}

func runIntentStop(inv *intentInvocation) int {
	if problem := inv.checkUIOptionsTarget(); problem != nil {
		return inv.render(*problem)
	}
	if args := inv.input.args; len(args) == 1 && args[0] == "ui" {
		return inv.uiTarget("stop")
	} else if len(args) == 2 && args[0] == "design" {
		return runIntentStopDesign(inv, args[1])
	} else if len(args) == 2 && args[0] == "review" {
		return runIntentReviewRef(inv, "stop", args[1])
	} else if inv.input.has("out") || inv.input.has("attempt") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--out and --attempt belong to stop design G; nothing was done"})
	}
	kind, target, problem := inv.kindAndTarget("checkout", "job", "session")
	if problem != nil {
		return inv.render(*problem)
	}
	if kind != "session" && inv.input.has("by") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--by names the person authorizing stop session; nothing was done"})
	}
	switch kind {
	case "job":
		return inv.stopJob(target)
	case "session":
		return inv.stopSession()
	}
	scope, scale, problem := inv.selectProcessScope()
	if problem != nil {
		return inv.render(*problem)
	}
	report, refusal := inv.owners.processes.process.stop(scope, scale)
	if refusal != nil {
		return inv.render(processRefusalResult(inv.checkoutTarget(scope), "stop refused: ", refusal, report))
	}
	if report.ExitCode != 0 {
		_, fence := processFence(scope)
		return inv.render(intentResult{Outcome: intentPartial, code: report.ExitCode, Targets: inv.checkoutTarget(scope), text: report.Lines,
			Summary: "stop did not finish: some processes are still running (listed below); no new work starts (" + fence + ")",
			next:    inv.publicArgv(append([]string{"stop"}, inv.forward("installation")...)...), nextReason: "stop again; end any process that survives a second stop yourself",
			Data: map[string]any{"lines": nonNilLines(report.Lines), "exitCode": report.ExitCode, "fence": fence}})
	}
	return inv.render(processReportResult(inv.checkoutTarget(scope), "stopped "+scope.Checkout, report))
}

func (inv *intentInvocation) stopSession() int {
	by := strings.TrimSpace(inv.input.text("by"))
	if by == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: []intentTarget{{Kind: "session", ID: ""}},
			Summary: "stop session needs the attending person: --by NAME; nothing was done",
			next:    inv.publicArgv("stop", "session", "--by", "NAME"), nextReason: "at the enrolled terminal, with your name"})
	}
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	marker, refusal, code := inv.owners.processes.sessionStop(inv.stateRoot, by)
	if code != 0 {
		return inv.render(intentResult{Outcome: intentRefused, code: code, Targets: []intentTarget{{Kind: "session", ID: ""}}, Summary: refusal,
			Decision: "a person authorizes this at the enrolled terminal; the checkout keeps running either way"})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: []intentTarget{{Kind: "session", ID: marker.SessionId}},
		Summary: fmt.Sprintf("session stop authorized once for %s at holder %s epoch %d by %s; the checkout keeps running", marker.SessionId, marker.HolderMainId, marker.ClaimEpoch, marker.By),
		Data:    map[string]any{"sessionStop": marker}})
}

// selectLayoutRoot resolves the repository's state root for owners that read
// repository state without needing the engine binary or the synced ledger.
func (inv *intentInvocation) selectLayoutRoot() *intentResult {
	path := inv.cwd
	if inv.input.has("repo") {
		path = inv.input.text("repo")
		if !filepath.IsAbs(path) {
			path = filepath.Join(inv.cwd, path)
		}
	}
	layout, err := inv.owners.resolver.ResolveLayout(path)
	if err == nil {
		inv.layout = layout
		inv.stateRoot, err = inv.owners.resolver.RootForInstallation(layout.InstallationRoot)
	}
	if err != nil {
		return &intentResult{Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("%s is not inside one metasystem installation: %v", shellCommand([]string{path}), err),
			Decision: "run this inside the repository, or name it with --repo PATH"}
	}
	return nil
}

var dispatchJobIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// intentJob is the one retained record a job id names.
type intentJob struct {
	id         string // the owner's own id, never a qualified reference
	kind       string // launch or dispatch
	launch     launch.Record
	dispatch   map[string]any
	recordPath string
}

// resolveJob finds the exact record J names: a launch record of the current
// user, or a dispatch job of this repository. Two matches, or none, refuse.
// Public job references: j1:ID names a launch of this user and j2:ID a
// dispatch job of this repository; a raw ID is searched in both. The prefix
// is decoded only here, and owners always receive the raw ID.
const (
	launchJobPrefix   = "j1:"
	dispatchJobPrefix = "j2:"
)

func jobReference(job intentJob) string {
	if job.kind == "launch" {
		return launchJobPrefix + job.id
	}
	return dispatchJobPrefix + job.id
}

// resolveJob finds the exact record a job reference names: a launch record
// of the current user, or a dispatch job of this repository. Two matches,
// or none, refuse; an ambiguity offers both references for the same verb.
func (inv *intentInvocation) resolveJob(ref, verb string) (intentJob, *intentResult) {
	targets := []intentTarget{{Kind: "job", ID: ref}}
	id, searchLaunch, searchDispatch := ref, true, true
	if raw, found := strings.CutPrefix(ref, launchJobPrefix); found {
		id, searchDispatch = raw, false
	} else if raw, found := strings.CutPrefix(ref, dispatchJobPrefix); found {
		id, searchLaunch = raw, false
	}
	var found []intentJob
	if searchLaunch && inv.owners.processes.launches != nil {
		manager := inv.owners.processes.launches()
		record, err := manager.Store.Read(id)
		switch {
		case err == nil:
			found = append(found, intentJob{id: id, kind: "launch", launch: record})
		case errors.Is(err, fs.ErrNotExist), strings.Contains(err.Error(), "invalid launch id"):
		default:
			return intentJob{}, &intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: fmt.Sprintf("the launch record %s is unreadable: %v", id, err)}
		}
	}
	repository := "no repository was resolved"
	if searchDispatch && dispatchJobIDPattern.MatchString(id) {
		if problem := inv.selectLayoutRoot(); problem == nil {
			repository = inv.stateRoot
			path := filepath.Join(inv.stateRoot, "artifacts", "agents", "jobs", id+".json")
			_, statErr := os.Stat(path)
			object, readErr := dispatchcore.ReadRecordObject(path)
			switch {
			case errors.Is(statErr, fs.ErrNotExist):
			case readErr == nil:
				found = append(found, intentJob{id: id, kind: "dispatch", dispatch: object, recordPath: path})
			default:
				return intentJob{}, &intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: fmt.Sprintf("the dispatch job record %s is unreadable: %v", path, readErr)}
			}
		} else {
			repository = "no repository at " + shellCommand([]string{inv.cwd}) + " (" + problem.Summary + ")"
		}
	}
	switch len(found) {
	case 0:
		return intentJob{}, &intentResult{Outcome: intentRefused, code: 1, Targets: targets,
			Summary: fmt.Sprintf("no job %s: no launch record of this user and no dispatch job in %s; nothing was done", shellCommand([]string{ref}), repository),
			next:    inv.publicArgv("status", "work", "--all"), nextReason: "lists this user's launches and this repository's dispatch jobs with their references"}
	case 2:
		lines, choices := []string{}, []map[string]any{}
		for _, job := range found {
			lines = append(lines, fmt.Sprintf("  %s: %s", shellCommand(inv.publicArgv(verb, "job", jobReference(job))), jobPurpose(job)))
			choices = append(choices, map[string]any{"reference": jobReference(job), "purpose": jobPurpose(job), "argv": inv.publicArgv(verb, "job", jobReference(job))})
		}
		return intentJob{}, &intentResult{Outcome: intentRefused, code: 1, Targets: targets, text: lines,
			Summary:  fmt.Sprintf("%s names both a launch of this user and a dispatch job of this repository; nothing was done", shellCommand([]string{ref})),
			Decision: "name the one meant by its reference, " + jobReference(found[0]) + " or " + jobReference(found[1]),
			Data:     map[string]any{"candidates": []string{jobReference(found[0]), jobReference(found[1])}, "choices": choices}}
	}
	return found[0], nil
}

// jobPurpose describes a job for a person choosing it.
func jobPurpose(job intentJob) string {
	if job.kind == "launch" {
		parts := []string{job.launch.Kind + " launch", string(job.launch.State)}
		if job.launch.Goal != "" {
			parts = append(parts, "goal "+job.launch.Goal)
		}
		if job.launch.WorkingDirectory != "" {
			parts = append(parts, "in "+job.launch.WorkingDirectory)
		}
		return strings.Join(parts, ", ")
	}
	role, _ := job.dispatch["role"].(string)
	status, _ := job.dispatch["status"].(string)
	parts := []string{cmpOr(role, "dispatch") + " job", cmpOr(status, "status unknown")}
	if goalID, _ := job.dispatch["goalId"].(string); goalID != "" {
		parts = append(parts, "goal "+goalID)
	}
	return strings.Join(parts, ", ")
}

// runIntentStatusWork lists the jobs a person can wait for or stop: this
// user's launches and the selected repository's dispatch jobs, running only
// unless --all includes ended ones, each with its public reference.
func runIntentStatusWork(inv *intentInvocation) int {
	all := inv.input.switched("all")
	var jobs []intentJob
	var records []launch.Record
	if inv.owners.processes.launches != nil {
		listed, err := inv.owners.processes.launches().List()
		if err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "this user's launches cannot be listed: " + err.Error()})
		}
		records = listed
	}
	for _, record := range records {
		if all || !record.State.Terminal() {
			jobs = append(jobs, intentJob{id: record.ID, kind: "launch", launch: record})
		}
	}
	scope := "this user's launches"
	if problem := inv.selectLayoutRoot(); problem == nil {
		scope += " and the dispatch jobs of " + inv.stateRoot
		paths, _ := filepath.Glob(filepath.Join(inv.stateRoot, "artifacts", "agents", "jobs", "*.json"))
		slices.Sort(paths)
		for _, path := range paths {
			object, readErr := dispatchcore.ReadRecordObject(path)
			if readErr != nil {
				continue
			}
			status, _ := object["status"].(string)
			if all || !dispatchcore.TerminalStatus(status) {
				jobs = append(jobs, intentJob{id: strings.TrimSuffix(filepath.Base(path), ".json"), kind: "dispatch", dispatch: object, recordPath: path})
			}
		}
	} else {
		scope += " (no repository here, so no dispatch jobs)"
	}
	views, lines := []map[string]any{}, []string{}
	for _, job := range jobs {
		ref := jobReference(job)
		ended := (job.kind == "launch" && job.launch.State.Terminal()) || (job.kind == "dispatch" && dispatchcore.TerminalStatus(fmt.Sprint(job.dispatch["status"])))
		view := map[string]any{"reference": ref, "kind": job.kind, "purpose": jobPurpose(job), "status": inv.publicArgv("status", "job", ref)}
		line := fmt.Sprintf("  %s: %s", ref, jobPurpose(job))
		if !ended {
			view["wait"], view["stop"] = inv.publicArgv("wait", "job", ref), inv.publicArgv("stop", "job", ref)
			line += "; " + shellCommand(inv.publicArgv("wait", "job", ref)) + " or " + shellCommand(inv.publicArgv("stop", "job", ref))
		}
		views = append(views, view)
		lines = append(lines, line)
	}
	word := "running"
	if all {
		word = "known"
	}
	result := intentResult{Outcome: intentConfirmed, text: lines, Data: map[string]any{"scope": scope, "all": all, "jobs": views},
		Summary: fmt.Sprintf("%d %s job(s) among %s", len(jobs), word, scope)}
	if !all {
		result.next, result.nextReason = inv.publicArgv("status", "work", "--all"), "also lists ended jobs"
	}
	return inv.render(result)
}

func (inv *intentInvocation) stopJob(ref string) int {
	job, problem := inv.resolveJob(ref, "stop")
	if problem != nil {
		return inv.render(*problem)
	}
	id := job.id
	targets := []intentTarget{{Kind: "job", ID: jobReference(job)}}
	if job.kind == "launch" {
		if job.launch.State.Terminal() {
			return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Summary: fmt.Sprintf("launch %s already ended: %s", id, job.launch.State),
				text: []string{launchReport(job.launch)}, Data: map[string]any{"kind": "launch", "record": job.launch}})
		}
		record, err := inv.owners.processes.launches().Cancel(id)
		if err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: fmt.Sprintf("launch %s cancel: %v", id, err),
				Data: map[string]any{"kind": "launch", "record": record}})
		}
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Summary: fmt.Sprintf("launch %s cancelled: %s", id, record.State),
			text: []string{launchReport(record)}, Data: map[string]any{"kind": "launch", "record": record}})
	}
	outcome, code, err := inv.owners.processes.cancelDispatch(inv.layout.GitRoot, id)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: max(code, 1), Targets: targets, Summary: fmt.Sprintf("dispatch job %s cancel: %v", id, err)})
	}
	result := intentResult{Targets: targets, code: code, Data: map[string]any{"kind": "dispatch", "owner": outcome}}
	label, _ := outcome["outcome"].(string)
	detail, _ := outcome["detail"].(string)
	if code == 0 && label == "CANCELLED" {
		result.Outcome, result.Summary = intentConfirmed, fmt.Sprintf("dispatch job %s cancelled", id)
	} else {
		result.Outcome, result.Summary = intentRefused, strings.TrimSpace(fmt.Sprintf("dispatch job %s not cancelled: %s %s", id, label, detail))
		result.code = max(code, 1)
	}
	return inv.render(result)
}

func runIntentStatus(inv *intentInvocation) int {
	if args := inv.input.args; len(args) == 2 && args[0] == "goal" {
		return runIntentStatusGoal(inv, args[1])
	}
	if args := inv.input.args; len(args) == 2 && args[0] == "mission" {
		return runIntentMissionTarget(inv, "status", args[1])
	} else if len(args) == 1 && args[0] == "ui" {
		return inv.uiTarget("status")
	} else if inv.input.switched("machines") {
		if len(args) > 0 || inv.input.has("work") || inv.input.has("installation") {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "status --machines takes only --refresh; nothing was done"})
		}
		return runIntentFleet(inv)
	} else if inv.input.has("refresh") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--refresh belongs to status --machines; nothing was done"})
	}
	if args := inv.input.args; len(args) == 1 && args[0] == "work" {
		return runIntentStatusWork(inv)
	} else if inv.input.switched("all") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--all belongs to status work; nothing was done"})
	}
	if args := inv.input.args; len(args) == 1 && !slices.Contains([]string{"checkout", "job", "run"}, args[0]) {
		return runIntentStatusGoal(inv, args[0])
	}
	if inv.input.has("work") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--work names a goal's work: status G --work NAME; nothing was done"})
	}
	kind, target, problem := inv.kindAndTarget("checkout", "job", "run")
	if problem != nil {
		return inv.render(*problem)
	}
	switch kind {
	case "job":
		job, problem := inv.resolveJob(target, "status")
		if problem != nil {
			return inv.render(*problem)
		}
		target = job.id
		targets := []intentTarget{{Kind: "job", ID: jobReference(job)}}
		if job.kind == "launch" {
			record, err := inv.owners.processes.launches().Status(target)
			if err != nil {
				return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: fmt.Sprintf("launch %s status: %v", target, err)})
			}
			return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Summary: fmt.Sprintf("launch %s: %s", target, record.State),
				text: []string{launchReport(record)}, Data: map[string]any{"kind": "launch", "record": record}})
		}
		status, _ := job.dispatch["status"].(string)
		if status == "" {
			status = "unknown (the record names no status)"
		}
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Summary: fmt.Sprintf("dispatch job %s: %s", target, status),
			Data: map[string]any{"kind": "dispatch", "record": job.dispatch, "recordPath": job.recordPath}})
	case "run":
		targets := []intentTarget{{Kind: "unit", ID: target}}
		runner := &launch.UnitRunner{Manager: inv.owners.processes.launches()}
		record, err := runner.Status(target)
		if errors.Is(err, fs.ErrNotExist) {
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: fmt.Sprintf("no unit run %s of this user; nothing was read", shellCommand([]string{target}))})
		}
		if err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: fmt.Sprintf("unit run %s: %v", target, err)})
		}
		lines := []string{}
		for _, round := range record.Rounds {
			lines = append(lines, fmt.Sprintf("round %d: %s", round.Number, round.Outcome))
		}
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, text: lines,
			Summary: fmt.Sprintf("unit run %s (%s, goal %s): %s", record.ID, record.Unit, record.Goal, record.State), Data: map[string]any{"record": record}})
	}
	scope, scale, problem := inv.selectProcessScope()
	if problem != nil {
		return inv.render(*problem)
	}
	report, err := inv.owners.processes.process.status(scope, scale)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: inv.checkoutTarget(scope), Summary: "status is unknown: " + err.Error()})
	}
	return inv.render(processReportResult(inv.checkoutTarget(scope), "status of "+scope.Checkout, report))
}

func runIntentRestart(inv *intentInvocation) int {
	if problem := inv.checkUIOptionsTarget(); problem != nil {
		return inv.render(*problem)
	}
	if len(inv.input.args) == 0 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "restart needs its target: metasystem restart checkout, or metasystem restart ui; nothing was done",
			Decision: "name the target"})
	}
	kind, _, problem := inv.kindAndTarget("checkout", "ui")
	if problem != nil {
		return inv.render(*problem)
	}
	if kind == "ui" {
		return inv.runUIVerb("restart")
	}
	scope, scale, problem := inv.selectProcessScope()
	if problem != nil {
		return inv.render(*problem)
	}
	targets := inv.checkoutTarget(scope)
	owners := inv.owners.processes.process
	stopped, refusal := owners.stop(scope, scale)
	if refusal != nil {
		result := processRefusalResult(targets, "restart refused at its stop, nothing was stopped or started: ", refusal, stopped)
		return inv.render(result)
	}
	if stopped.ExitCode != 0 {
		return inv.render(intentResult{Outcome: intentPartial, code: stopped.ExitCode, Targets: targets, text: stopped.Lines,
			Summary:  "restart stopped at its stop: the stop did not complete, so nothing was started",
			Decision: "resolve the survivors listed above, then run metasystem restart checkout again",
			Data:     map[string]any{"reached": "stopping", "stop": map[string]any{"lines": nonNilLines(stopped.Lines), "exitCode": stopped.ExitCode}}})
	}
	armed, refusal := owners.arm(scope, scale, inv.input.text("temporary-human-word"), inv.input.text("review-by"))
	lines := append(append([]string(nil), stopped.Lines...), armed.Lines...)
	if refusal != nil {
		// The arm sequence may refuse after it reopened the fence; the fence
		// record says which state the checkout is in now.
		_, fence := processFence(scope)
		result := intentResult{Outcome: intentPartial, code: max(refusal.code, 1), Targets: targets, text: lines,
			Summary: "restart stopped the checkout, but its start refused: " + strings.TrimSuffix(strings.TrimSpace(refusal.sentence), ".") + "; the stop fence is now " + fence,
			next:    inv.publicArgv(append([]string{"start"}, inv.forward("installation")...)...), nextReason: "start it once the refusal above is resolved",
			Data: map[string]any{"reached": "stopped", "fence": fence, "stop": map[string]any{"lines": nonNilLines(stopped.Lines), "exitCode": stopped.ExitCode},
				"start": map[string]any{"lines": nonNilLines(armed.Lines), "remedy": refusal.second}}}
		return inv.render(result)
	}
	return inv.render(intentResult{Outcome: intentConfirmed, code: armed.ExitCode, Targets: targets, text: lines, Summary: "restarted " + scope.Checkout,
		Data: map[string]any{"reached": "started", "stop": map[string]any{"lines": nonNilLines(stopped.Lines), "exitCode": stopped.ExitCode},
			"start": map[string]any{"lines": nonNilLines(armed.Lines), "exitCode": armed.ExitCode}}})
}

func runIntentEnroll(inv *intentInvocation) int {
	name := strings.TrimSpace(inv.input.text("name"))
	if name == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "enroll needs your name: --name NAME; nothing was done",
			next: inv.publicArgv("enroll", "--name", "NAME"), nextReason: "at an agent-free terminal, with your name"})
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	report := &ownerReport{}
	dependencies := inv.owners.dependencies
	dependencies.report = report
	args := append([]string{"--root", inv.stateRoot, "--by", name}, inv.forward("lineage")...)
	code := runGoalEnrollTerminalWithDependencies(args, inv.owners.processes.enroll, inv.owners.commandNow, dependencies)
	targets := []intentTarget{{Kind: "terminal", ID: name}}
	switch {
	case report.refusal != nil && report.value != nil:
		return inv.render(intentResult{Outcome: intentPartial, code: max(report.refusal.code, 1), Targets: targets, Summary: report.refusal.sentence,
			Decision: strings.TrimSpace(report.refusal.remedy.words), Data: map[string]any{"enrollment": report.value, "fleetPublished": false}})
	case report.refusal != nil:
		result := ownerResult(report, code, intentResult{})
		result.Targets = targets
		return inv.render(result)
	case code == 0 && report.result != nil:
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Summary: "this terminal is enrolled for " + name + " and the fleet cutoff is published",
			Data: map[string]any{"enrollment": report.value, "fleetPublished": true, "owner": ownerPublication(*report.result)}})
	}
	return inv.render(ownerResult(report, code, intentResult{}))
}

func runIntentAsk(inv *intentInvocation) int {
	if choice, problem := inv.exclusiveChoice("retry", "withdraw"); problem != nil {
		return inv.render(*problem)
	} else if choice != "" {
		for _, other := range []string{"question", "option", "option-file", "recommend", "fact", "fact-file", "kind", "wants", "budget", "id"} {
			if inv.input.has(other) {
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--%s acts on an existing question and takes no --%s; nothing was done", choice, other)})
			}
		}
		if len(inv.input.args) > 0 {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--%s names the question itself: ask --%s Q; nothing was done", choice, choice)})
		}
		if choice == "retry" {
			if inv.input.has("reason") {
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--reason explains a withdrawal; a retry takes none; nothing was done"})
			}
			return runIntentAskRetry(inv, inv.input.text("retry"))
		}
		return runIntentAskWithdraw(inv, inv.input.text("withdraw"))
	}
	if inv.input.has("reason") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--reason belongs to ask --withdraw Q; nothing was asked"})
	}
	id, problem := inv.singleTarget()
	if problem != nil {
		return inv.render(*problem)
	}
	if id == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "ask needs the goal the question is about: " + inv.command.usage[0] + "; nothing was done", Decision: "name the goal"})
	}
	question := strings.TrimSpace(inv.input.text("question"))
	if question == "" || len(inv.input.values["option"]) == 0 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
			Summary: "ask needs --question TEXT and at least one --option 'LABEL: CONSEQUENCE'; nothing was asked"})
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	kind := inv.input.text("kind")
	if kind == "" {
		// An ordinary question; the other kinds carry authority.
		kind = "other"
	}
	in := channelAskInput{Goal: id, Kind: kind, Facts: append([]string{question}, inv.input.values["fact"]...),
		Options: inv.input.values["option"], Recommendation: inv.input.text("recommend"), Wants: inv.input.text("wants")}
	if inv.input.has("budget") {
		if kind != "stop" && kind != "budget-above-norm" {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
				Summary: "--budget proposes a box for --kind stop or --kind budget-above-norm only; nothing was asked"})
		}
		reviewRoundMax, err := config.ReviewRoundMax(filepath.Join(inv.stateRoot, "metasystem.conf"))
		if err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: err.Error()})
		}
		budget, err := goalbudget.ParseBox(inv.input.text("budget"), nil, reviewRoundMax)
		if err != nil {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
				Summary: fmt.Sprintf("--budget %s: %v; nothing was asked", shellCommand([]string{inv.input.text("budget")}), err), Decision: "give the complete compact box, for example 1d/10/720m/1/3"})
		}
		if kind == "stop" {
			in.Wants = goal.ResumeApprovalToken(id, budget)
		} else {
			in.Budget = &budget
		}
	} else if kind == "stop" || kind == "budget-above-norm" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
			Summary: fmt.Sprintf("a %s question proposes a box: add --budget BOX; nothing was asked", kind)})
	}
	if kind == "carry" && !goal.ValidCarryToken(in.Wants) {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
			Summary: "a carry question needs --wants exactly 'carry workspace=<sha40> goal=<id> past=<name>'; nothing was asked"})
	}
	q, warnings, code, err := inv.owners.processes.ask(inv.stateRoot, in)
	if err != nil && q.ID == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: max(code, 1), Targets: inv.targets(id), Summary: err.Error() + "; nothing was asked", text: warnings})
	}
	targets := []intentTarget{{Kind: "goal", ID: id}, {Kind: "question", ID: q.ID}}
	delivery, pending := "posted to the channel", false
	switch {
	case q.Thread == nil && q.Undelivered > 0:
		delivery, pending = fmt.Sprintf("not delivered yet: posting to the channel failed %d time(s)", q.Undelivered), true
	case q.Thread == nil:
		delivery, pending = "not sent: no channel is configured for this repository", true
	}
	data := map[string]any{"question": q, "delivery": delivery, "replyInstructions": channel.ReplyInstructions(q)}
	lines := append(warnings, "delivery: "+delivery)
	wait := inv.publicArgv("wait", "question", "channel:"+q.ID)
	poll := inv.publicArgv("ask", "--retry", q.ID)
	if err != nil {
		return inv.render(inv.askAfterFailure(q, err, code, targets, warnings))
	}
	if pending {
		result := intentResult{Outcome: intentInProgress, code: 1, Targets: targets, text: lines, Data: data,
			Summary: "question " + q.ID + " is recorded but " + delivery}
		if q.Undelivered > 0 {
			result.next, result.nextReason = poll, "delivers exactly this stored question once more"
		} else {
			result.Outcome = intentPartial
			result.Decision = "configure a channel for this repository; the question waits until one delivers it"
		}
		return inv.render(result)
	}
	lines = append(lines, "the person answers in the channel thread: "+channel.ReplyInstructions(q))
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Summary: "asked " + q.ID + " and " + delivery, text: lines,
		next: wait, nextReason: "wait for the authenticated answer", Data: data})
}

func runIntentAnswer(inv *intentInvocation) int {
	args := inv.input.args
	if len(args) >= 1 && args[0] != "mission" && args[0] != "question" {
		return runIntentAnswerQuestion(inv)
	}
	if len(args) == 0 || (args[0] != "mission" && args[0] != "question") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "answer takes " + strings.Join(inv.command.allUsage(), " | ") + "; nothing was answered",
			Decision: "name what the question belongs to"})
	}
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	if args[0] == "question" {
		if len(args) < 2 {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "answer question needs the question id; nothing was answered"})
		}
		id := args[1]
		targets := []intentTarget{{Kind: "question", ID: id}}
		q, err := inv.owners.processes.question(inv.stateRoot, id)
		if err != nil {
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: fmt.Sprintf("no channel question %s: %v", shellCommand([]string{id}), err)})
		}
		if q.Answer != nil {
			return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Summary: "question " + id + " was already answered through its channel", Data: map[string]any{"question": q}})
		}
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets,
			Summary:  "question " + id + " is answered in its channel thread, which authenticates you; nothing was recorded here",
			Decision: channel.ReplyInstructions(q),
			Data:     map[string]any{"question": q, "replyInstructions": channel.ReplyInstructions(q)}})
	}
	if path := inv.input.text("answer-file"); path != "" && len(args) >= 3 {
		text, problem := inv.readTextFile("answer-file", path)
		if problem != nil {
			return inv.render(*problem)
		}
		if len(args) == 4 && args[3] != text {
			return inv.render(intentResult{Outcome: intentRefused, code: 2,
				Summary: "the inline answer and --answer-file differ; nothing was answered", Decision: "give the answer once"})
		}
		args = append(args[:3:3], text)
	}
	if len(args) != 4 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "answer mission needs M, Q and the answer as one quoted TEXT or --answer-file FILE: metasystem answer mission M Q TEXT; nothing was answered"})
	}
	return inv.render(inv.answerMission(args[1], args[2], args[3]))
}

// answerMission records a person's answer through the mission owner, which
// applies it or changes nothing.
func (inv *intentInvocation) answerMission(missionID, askID, answer string) intentResult {
	targets := []intentTarget{{Kind: "mission", ID: missionID}, {Kind: "question", ID: askID}}
	if !missionIDRe.MatchString(missionID) || !missionIDRe.MatchString(askID) || strings.TrimSpace(answer) == "" || strings.ContainsRune(answer, 0) {
		return (intentResult{Outcome: intentRefused, code: 2, Targets: targets, Summary: "a mission and question id are lowercase words with dashes, and the answer is non-empty text; nothing was answered"})
	}
	engine, err := inv.owners.processes.mission(inv.stateRoot, missionID)
	if err != nil {
		return (intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "mission answer: " + err.Error()})
	}
	var output, errs bytes.Buffer
	engine.Output, engine.Errors = &output, &errs
	askPath := filepath.Join(inv.stateRoot, "artifacts", "agents", "missions", missionID, "asks", askID+".json")
	answeredBefore := missionAskAnswered(askPath)
	code := engine.Answer(askID, answer)
	lines := intentOwnerLines(output.String(), errs.String())
	answered := missionAskAnswered(askPath) && !answeredBefore
	effects := engine.LastAnswer
	data := map[string]any{"mission": missionID, "ask": askID, "exitCode": code, "askAnswered": answered, "effects": effects, "lines": nonNilLines(lines)}
	resume := inv.publicArgv("resume", "mission", missionID)
	switch {
	case code == 0:
		return (intentResult{Outcome: intentConfirmed, Targets: targets, Summary: "answered " + askID + " of mission " + missionID, text: lines, Data: data})
	case effects.ResetRecorded && !effects.AskAnswered:
		// The reset line is authoritative; answering again is lawful and
		// completes the answer.
		return (intentResult{Outcome: intentPartial, code: code, Targets: targets, text: lines, Data: data,
			Summary: "the reset for " + askID + " is recorded, but the question could not be marked answered",
			next:    inv.publicArgv("answer", "mission", missionID, askID, answer), nextReason: "answer again; a second reset is harmless"})
	case effects.AskAnswered || answered:
		result := intentResult{Outcome: intentPartial, code: code, Targets: targets, text: lines, Data: data,
			Summary: "the answer to " + askID + " is recorded but the mission did not continue yet"}
		if effects.ResetRecorded {
			result.next, result.nextReason = resume, "resuming the mission applies the recorded answer"
		}
		return (result)
	case code == 3:
		return (intentResult{Outcome: intentRefused, code: code, Targets: targets, text: lines, Data: data,
			Summary: "the mission did not take the answer; nothing changed"})
	}
	return (intentResult{Outcome: intentFailed, code: code, Targets: targets, text: lines, Data: data, Summary: "the mission's state could not be read; nothing changed"})
}

func intentOwnerLines(streams ...string) []string {
	var lines []string
	for _, stream := range streams {
		for _, line := range strings.Split(strings.TrimSpace(stream), "\n") {
			if strings.TrimSpace(line) != "" {
				lines = append(lines, line)
			}
		}
	}
	return lines
}

func missionAskAnswered(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var ask map[string]any
	return json.Unmarshal(data, &ask) == nil && ask["answeredAt"] != nil
}

func runIntentFleet(inv *intentInvocation) int {
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	report, err := inv.owners.processes.fleet(inv.layout.GitRoot, inv.input.switched("refresh"), seatFleetNow())
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "fleet: " + err.Error()})
	}
	encoded, err := report.JSON()
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "fleet: " + err.Error()})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: "the fleet", text: intentOwnerLines(report.Text()), Data: json.RawMessage(encoded)})
}

func runIntentDoctor(inv *intentInvocation) int {
	scope, _, problem := inv.selectProcessScope()
	if problem != nil {
		return inv.render(*problem)
	}
	owners := inv.owners.processes
	now, err := owners.healthNow(scope.Installation)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 2, Summary: "doctor: the fixture clock is unreadable: " + err.Error()})
	}
	verdict := owners.health(scope.Root, scope.Installation, now)
	stopped, _, _ := stopfence.Closed(scope.Root)
	lines, remedies := []string{}, []map[string]any{}
	var first []string
	for _, role := range verdict.Roles {
		if role.Status == steward.HealthAlive {
			continue
		}
		line := fmt.Sprintf("%s %s: %s", role.Role, role.Status, role.Reason)
		public, instruction := publicHealthRemedy(role, stopped)
		switch {
		case len(public) > 0:
			line += "; remedy: " + shellCommand(public)
			if first == nil {
				first = public
			}
		case instruction != "":
			line += "; " + instruction
		case role.NoAutomaticRemedy:
			line += "; no command repairs this"
		}
		lines = append(lines, line)
		remedies = append(remedies, map[string]any{"role": role.Role, "public": public, "instruction": instruction, "facts": role.RemedyFacts, "ownerRemedy": role.Remedy})
	}
	result := intentResult{Outcome: intentConfirmed, code: verdict.ExitCode(), Targets: inv.checkoutTarget(scope),
		Summary: verdict.Line(), text: lines, Data: additiveData(steward.NewHookHealthPreview(verdict), map[string]any{"publicRemedies": remedies})}
	if first != nil {
		result.next, result.nextReason = first, "the first public remedy check found"
	}
	return inv.render(result)
}

// publicHealthRemedy is the public command, or the plain instruction, for
// one unhealthy role, chosen from the role and its typed remedy facts; the
// owner's own remedy is kept only as diagnostic data.
func publicHealthRemedy(role steward.RoleVerdict, stopped bool) ([]string, string) {
	if len(role.RemedyFacts) > 0 {
		return publicRemedyForFact(role.RemedyFacts[0])
	}
	switch role.Role {
	case steward.RoleStewardRunner, steward.RoleSupervisionOwner, steward.RoleRepoWatcher, steward.RoleNarratorFreshness,
		steward.RoleCensusFreshness, steward.RoleHookFreshness:
		if stopped {
			return []string{"metasystem", "start"}, ""
		}
		return []string{"metasystem", "start", "session"}, ""
	case steward.RoleLedgerAttention:
		return []string{"metasystem", "goals"}, ""
	case steward.RoleNonterminalJobs:
		return nil, "the job reaper reconciles these on its next pass; metasystem status lists the work"
	case steward.RoleRetroDebt:
		return nil, "run the retro and record its receipt"
	case steward.RoleTrunkRed:
		return []string{"metasystem", "incidents"}, ""
	case steward.RoleSpendFence:
		return nil, "a person raises the spend ceiling in metasystem.conf"
	case steward.RoleProofAttempts:
		return []string{"metasystem", "test"}, ""
	}
	return nil, "no public command repairs this; the reason above names what a person must change"
}

// publicRemedyForFact is the public act for one typed cause.
func publicRemedyForFact(fact steward.RemedyFact) ([]string, string) {
	switch fact.Cause {
	case steward.CauseBudgetMissing, steward.CauseBudgetMalformed:
		return []string{"metasystem", "budget", fact.Goal, "BOX"}, ""
	case steward.CauseBudgetBreach:
		return []string{"metasystem", "budget", fact.Goal, "BOX"}, "goal " + fact.Goal + " is over its box; its stop runs by itself, and a person may give it a larger box"
	case steward.CauseBudgetUnknown:
		return nil, "a person repairs record " + fact.Record + ", which cannot be read as a budget, then runs metasystem check"
	case steward.CauseBreachStopOpen:
		return nil, "goal " + fact.Goal + "'s budget stop " + fact.Stop + " completes by itself on the steward's next pass; nothing needs doing"
	case steward.CauseBreachStopUnresolved:
		return nil, "a person inspects budget stop " + fact.Stop + " of goal " + fact.Goal + " and its job records; new work stays fenced until it resolves"
	case steward.CauseEpochMismatch:
		return []string{"metasystem", "start", "session"}, ""
	case steward.CauseForeignLineage:
		return []string{"metasystem", "release", fact.Goal}, "the session that claimed goal " + fact.Goal + " releases it, or a person takes it over: metasystem claim " + fact.Goal + " --take-over --reason TEXT"
	case steward.CauseStopCapabilityMissing:
		return nil, "a person repairs goal " + fact.Goal + "'s record (" + fact.Record + "), which has no stop capability"
	}
	return nil, "no public command repairs this; the reason above names what a person must change"
}

// runUIVerb runs the interface's status or restart through its lifecycle
// owner and renders the typed result.
func (inv *intentInvocation) runUIVerb(verb string) int {
	options, optionProblem := inv.uiOptions()
	if optionProblem != nil {
		return inv.render(*optionProblem)
	}
	layout, installation, _, problem := inv.selectInstallation()
	if problem != nil {
		return inv.render(*problem)
	}
	roots, err := lifecycle.ResolveRootsWith(inv.owners.processes.process.repositoryTop, inv.owners.resolver.RootForInstallation, layout.GitRoot, installation)
	targets := []intentTarget{{Kind: "ui", ID: layout.GitRoot}}
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: err.Error() + "; nothing was done"})
	}
	lifecycleResult, err := inv.owners.processes.ui(verb, roots, options)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: err.Error() + "; nothing was done"})
	}
	result := lifecycleResult.Result
	data := map[string]any{"lines": nonNilLines(result.Lines), "exitCode": result.Code}
	if verb == "status" {
		data["state"] = lifecycleResult.State
		outcome := intentConfirmed
		if lifecycleResult.State == lifecycle.Unreadable {
			outcome = intentFailed
		}
		summary := "the interface is " + string(lifecycleResult.State)
		if lifecycleResult.State == lifecycle.Running {
			summary = "the interface is running"
		}
		return inv.render(intentResult{Outcome: outcome, code: result.Code, Targets: targets, Summary: summary, text: result.Lines, Data: data})
	}
	restart := lifecycleResult.Restart
	data["stop"], data["started"] = restart.Stop, restart.Started
	switch {
	case restart.Started && restart.Start.Code == 0:
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Summary: "the interface restarted", text: result.Lines, Data: data})
	case restart.Started:
		return inv.render(intentResult{Outcome: intentPartial, code: result.Code, Targets: targets, text: result.Lines, Data: data,
			Summary: "the interface stopped but did not start again",
			next:    []string{"metasystem", "start", "ui"}, nextReason: "start it once the problem above is fixed"})
	case restart.Stop == lifecycle.Timeout:
		return inv.render(intentResult{Outcome: intentPartial, code: max(result.Code, 1), Targets: targets, text: result.Lines, Data: data,
			Summary: "the interface was asked to stop but is still running, so it was not started again",
			next:    []string{"metasystem", "restart", "ui"}, nextReason: "try again once it has stopped"})
	}
	return inv.render(intentResult{Outcome: intentRefused, code: max(result.Code, 1), Targets: targets, text: result.Lines, Data: data,
		Summary: "the interface could not be restarted; nothing was changed"})
}

type uiIntentOptions struct {
	listen      string
	listenSet   bool
	waitSeconds int64
}

func (inv *intentInvocation) checkUIOptionsTarget() *intentResult {
	if (inv.input.has("listen") || inv.input.has("wait-seconds")) && !slices.Equal(inv.input.args, []string{"ui"}) {
		return &intentResult{Outcome: intentRefused, code: 2, Summary: "--listen and --wait-seconds belong to the ui target; nothing was done"}
	}
	return nil
}

func (inv *intentInvocation) uiOptions() (uiIntentOptions, *intentResult) {
	options := uiIntentOptions{listen: inv.input.text("listen"), listenSet: inv.input.has("listen"), waitSeconds: 15}
	if inv.input.has("wait-seconds") {
		seconds, err := strconv.ParseInt(inv.input.text("wait-seconds"), 10, 64)
		if err != nil || seconds < 0 || seconds > int64((1<<63-1)/time.Second) {
			return options, &intentResult{Outcome: intentRefused, code: 2, Summary: "--wait-seconds needs a nonnegative whole number of seconds that fits a duration; nothing was done"}
		}
		options.waitSeconds = seconds
	}
	return options, nil
}

// uiLifecycleFor runs one interface lifecycle verb with the interface's own
// defaults; an error is a refusal before anything was done.
func uiLifecycleFor(verb string, roots lifecycle.Roots, options uiIntentOptions) (uiLifecycleResult, error) {
	listen, err := uiListen(verb, roots, options.listen, options.listenSet)
	if err != nil {
		return uiLifecycleResult{}, err
	}
	return uiLifecycleRun(verb, roots, listen, options.waitSeconds), nil
}

func sameCanonicalPath(left, right string) bool {
	left, leftErr := canonicalPath(left)
	right, rightErr := canonicalPath(right)
	return leftErr == nil && rightErr == nil && left == right
}

// processFence reads the checkout's stop fence as "state/phase".
func processFence(scope processScope) (stopfence.Record, string) {
	record, err := stopfence.Read(scope.Root)
	if err != nil {
		return record, "unreadable: " + err.Error()
	}
	return record, record.State + "/" + record.Phase
}

// startRefusal renders a start whose arm sequence refused: a refusal before
// the fence moved changed nothing; one after it names the fence and the
// processes running now, and the exact start to run again.
func (inv *intentInvocation) startRefusal(scope processScope, scale int, before stopfence.Record, refusal *processRefusal, report stoptransition.Report) intentResult {
	after, fence := processFence(scope)
	if after.Generation == before.Generation && after.State == before.State && after.Phase == before.Phase {
		return processRefusalResult(inv.checkoutTarget(scope), "start refused: ", refusal, report)
	}
	data := map[string]any{"fence": fence, "lines": nonNilLines(report.Lines), "remedy": refusal.second}
	lines := append([]string(nil), report.Lines...)
	if status, err := inv.owners.processes.process.status(scope, scale); err == nil {
		data["running"] = map[string]any{"lines": nonNilLines(status.Lines), "exitCode": status.ExitCode}
		lines = append(lines, "running now:")
		lines = append(lines, status.Lines...)
	} else {
		data["running"] = "unknown: " + err.Error()
	}
	return intentResult{Outcome: intentPartial, code: max(refusal.code, 1), Targets: inv.checkoutTarget(scope), text: lines, Data: data,
		Summary: "start began but did not finish: " + strings.TrimSuffix(strings.TrimSpace(refusal.sentence), ".") + "; the checkout accepts new work again (" + fence + ") but not everything is running",
		next:    inv.publicArgv(append([]string{"start"}, inv.forward("installation")...)...), nextReason: "start again once the problem above is fixed"}
}

// askAfterFailure reports a question the channel owner recorded before a
// later step failed. The saved record, not the owner's in-memory copy, says
// what a later poll or answer will see: a post whose thread was not saved is
// posted again by the next poll, and an answer in that thread cannot be
// matched to the question.
func (inv *intentInvocation) askAfterFailure(q channel.Question, failure error, code int, targets []intentTarget, warnings []string) intentResult {
	questionFile := filepath.Join(inv.stateRoot, "artifacts", "agents", "channel", "questions", q.ID+".json")
	posted := q.Thread != nil
	data := map[string]any{"questionId": q.ID, "posted": posted, "questionFile": questionFile, "error": failure.Error()}
	result := intentResult{Outcome: intentPartial, code: max(code, 1), Targets: targets, text: warnings, Data: data}
	saved, readErr := inv.owners.processes.question(inv.stateRoot, q.ID)
	if readErr != nil {
		data["savedQuestion"] = nil
		result.Summary = "question " + q.ID + " may be recorded, but a later step failed (" + failure.Error() + ") and its saved record cannot be read: " + readErr.Error()
		result.Decision = "repair the channel question storage at " + shellCommand([]string{questionFile}) + ", then read the question with " + shellCommand(inv.publicArgv("show", "question", "channel:"+q.ID))
		return result
	}
	data["savedQuestion"], data["savedThread"] = saved, saved.Thread
	switch {
	case posted && saved.Thread == nil:
		result.Summary = "question " + q.ID + " was posted to the channel, but saving its thread failed: " + failure.Error() +
			". The saved question has no thread, so the next channel poll would post it a second time"
		result.Decision = "repair the channel question storage at " + shellCommand([]string{questionFile}) +
			" before the next channel poll; the person may already see the posted message, but an answer there cannot be matched to this question until its thread is saved"
	case saved.Thread != nil:
		result.Summary = "question " + q.ID + " is posted and saved, but a later step failed: " + failure.Error()
		result.Decision = "the question stands and is answered in its channel thread; the goal ledger may not show it as asked"
	default:
		result.Summary = "question " + q.ID + " is saved but not delivered, and a later step failed: " + failure.Error()
		result.next = inv.publicArgv("ask", "--retry", q.ID)
		result.nextReason = "delivers exactly this stored question once more"
	}
	return result
}

// runIntentAnswerQuestion is answer Q [TEXT]: a mission question is
// answered and resumed through the mission owners; a channel question shows
// its authenticated reply location and records nothing.
func runIntentAnswerQuestion(inv *intentInvocation) int {
	args := inv.input.args
	if len(args) > 2 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "answer takes Q and at most one quoted TEXT; nothing was answered"})
	}
	text := ""
	if len(args) == 2 {
		text = args[1]
	}
	if path := inv.input.text("answer-file"); path != "" {
		fromFile, problem := inv.readTextFile("answer-file", path)
		if problem != nil {
			return inv.render(*problem)
		}
		if text != "" && text != fromFile {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "the inline answer and --answer-file differ; nothing was answered", Decision: "give the answer once"})
		}
		text = fromFile
	}
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	q, problem := inv.resolveQuestion(args[0])
	if problem != nil {
		return inv.render(*problem)
	}
	if q.kind == "channel" {
		view := inv.questionView(q)
		if q.channel.Answer != nil || q.channel.State == "closed" {
			view.Outcome = intentUnchanged
			return inv.render(view)
		}
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: view.Targets, Data: view.Data,
			Summary:  "channel question " + q.id + " is answered in its channel thread, which authenticates you; text given here is never proof, and nothing was recorded",
			Decision: channel.ReplyInstructions(q.channel)})
	}
	if strings.TrimSpace(text) == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "a mission question is answered with TEXT (or --answer-file FILE); nothing was answered",
			next: inv.publicArgv("show", "question", q.publicName()), nextReason: "the question and its options"})
	}
	if q.ask["answeredAt"] != nil {
		recorded, _ := q.ask["answer"].(string)
		if recorded != text {
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: []intentTarget{{Kind: "question", ID: q.publicName()}},
				Summary: fmt.Sprintf("mission %s's question %s is already answered with a different answer; nothing was changed", q.mission, q.id)})
		}
		// The same answer again completes what an interrupted call left.
		return inv.render(inv.resumeMission(q.mission, intentResult{Outcome: intentConfirmed, Summary: "the answer to " + q.id + " was already recorded"}))
	}
	answered := inv.answerMission(q.mission, q.id, text)
	if answered.Outcome != intentConfirmed && answered.Outcome != intentPartial {
		return inv.render(answered)
	}
	return inv.render(inv.resumeMission(q.mission, answered))
}

// resumeMission asks the mission runner to resume after an answer; the
// runner decides whether that is lawful now.
func (inv *intentInvocation) resumeMission(mission string, answered intentResult) intentResult {
	ran, problem := inv.engineVerb("mission", "resume", "--root", inv.stateRoot, "--mission", mission)
	if problem != nil {
		problem.Summary = answered.Summary + "; " + problem.Summary
		problem.Outcome = intentPartial
		return *problem
	}
	resumed := ownerVerbResult(ran, append(answered.Targets, intentTarget{Kind: "mission", ID: mission}), answered.Summary+"; mission "+mission+" resumed", nil)
	resumed.Data = mergeData(map[string]any{"answer": answered.Data}, resumed.Data)
	if resumed.Outcome != intentConfirmed {
		resumed.Outcome = intentPartial
		resumed.Summary = answered.Summary + ", but the mission did not resume: " + resumed.Summary
		resumed.next, resumed.nextReason = inv.sameCommand(), "the same answer completes the resume once the named cause is resolved; it is never recorded twice"
	}
	return resumed
}

// runIntentMission starts, resumes or reads one autonomous mission through
// the mission runner.
func runIntentMission(inv *intentInvocation, verb, mission string) int {
	if !missionIDRe.MatchString(mission) {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "a mission id is a lowercase word with dashes; nothing was done"})
	}
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	ran, problem := inv.engineVerb("mission", verb, "--root", inv.stateRoot, "--mission", mission)
	if problem != nil {
		return inv.render(*problem)
	}
	done := map[string]string{"start": "mission " + mission + " started", "resume": "mission " + mission + " resumed", "status": "mission " + mission + " status"}[verb]
	result := ownerVerbResult(ran, []intentTarget{{Kind: "mission", ID: mission}}, done, nil)
	if verb == "status" && ran.err == nil {
		result.Outcome, result.code = intentConfirmed, 0
		result.Summary = strings.TrimSpace(string(ran.stdout))
		if result.Summary == "" {
			result.Summary = done
		}
	}
	return inv.render(result)
}

// runIntentMissionTarget refuses the options that belong to other targets
// before the mission runner is asked.
func runIntentMissionTarget(inv *intentInvocation, verb, mission string) int {
	for _, other := range []string{"installation", "temporary-human-word", "review-by", "lineage", "work"} {
		if inv.input.has(other) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("%s mission M takes no --%s; nothing was done", verb, other)})
		}
	}
	return runIntentMission(inv, verb, mission)
}

// uiTarget is start ui, stop ui and status ui: the interface's own
// lifecycle verbs, refusing options that belong to the checkout.
func (inv *intentInvocation) uiTarget(verb string) int {
	options, optionProblem := inv.uiOptions()
	if optionProblem != nil {
		return inv.render(*optionProblem)
	}
	for _, other := range []string{"temporary-human-word", "review-by", "lineage", "by", "work", "machines", "refresh"} {
		if inv.input.has(other) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("%s ui takes no --%s; nothing was done", verb, other)})
		}
	}
	if verb == "status" {
		return inv.runUIVerb("status")
	}
	layout, installation, _, problem := inv.selectInstallation()
	if problem != nil {
		return inv.render(*problem)
	}
	roots, err := lifecycle.ResolveRootsWith(inv.owners.processes.process.repositoryTop, inv.owners.resolver.RootForInstallation, layout.GitRoot, installation)
	targets := []intentTarget{{Kind: "ui", ID: layout.GitRoot}}
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: err.Error() + "; nothing was done"})
	}
	ran, err := inv.owners.processes.ui(verb, roots, options)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: err.Error() + "; nothing was done"})
	}
	data := map[string]any{"lines": nonNilLines(ran.Result.Lines), "exitCode": ran.Result.Code}
	if ran.Result.Code == 0 {
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: data, text: ran.Result.Lines,
			Summary: map[string]string{"start": "the interface is started", "stop": "the interface is stopped"}[verb]})
	}
	return inv.render(intentResult{Outcome: intentRefused, code: ran.Result.Code, Targets: targets, Data: data, text: ran.Result.Lines,
		Summary: "the interface did not " + verb + "; its report is below", next: inv.publicArgv("status", "ui"), nextReason: "what the interface is doing now"})
}

// runIntentStartMachine adds one machine to the fleet through the seat
// launch owner: clone, build, configure, enroll and supervise.
func runIntentStartMachine(inv *intentInvocation, name string) int {
	for _, other := range []string{"lineage", "installation", "by"} {
		if inv.input.has(other) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("start machine takes no --%s; nothing was done", other)})
		}
	}
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	args := []string{"seat", "launch", "--machine", name, "--from", inv.layout.GitRoot, "--json"}
	for _, pair := range [][2]string{{"temporary-human-word", "--temporary-human-word"}, {"review-by", "--review-by"}, {"destination", "--destination"}, {"resume", "--resume"}} {
		if inv.input.has(pair[0]) {
			args = append(args, pair[1], inv.input.text(pair[0]))
		}
	}
	ran, problem := inv.engineVerb(args...)
	if problem != nil {
		return inv.render(*problem)
	}
	result := ownerVerbResult(ran, []intentTarget{{Kind: "machine", ID: name}}, "machine "+name+" is launched and supervised", nil)
	if result.Outcome != intentConfirmed {
		result.next, result.nextReason = inv.publicArgv("status", "--machines"), "every machine's presence"
	}
	return inv.render(result)
}

// additiveData is an existing structured view with additional fields beside
// its own, so schema-1 consumers keep reading the fields they know.
func additiveData(view any, extra map[string]any) any {
	encoded, err := json.Marshal(view)
	var object map[string]any
	if err != nil || json.Unmarshal(encoded, &object) != nil {
		return view
	}
	for key, value := range extra {
		if _, taken := object[key]; !taken {
			object[key] = value
		}
	}
	return object
}
