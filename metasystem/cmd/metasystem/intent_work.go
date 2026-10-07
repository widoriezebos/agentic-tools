package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/applaunch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/audit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/designgate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/protocol"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes/external"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
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

var intentReadEachRound = regexp.MustCompile(`(?m)^Read each round: yes[\t \r]*$`)

// intentWorkOwners are the owners the work commands call. Tests give each
// invocation its own runner, Git, wait, test runner and settings readers.
type intentWorkOwners struct {
	engineStamp string
	designGate  designGateOwners
	adapter     func(string) (adapter.Adapter, error)
	units       func(layout stateroot.Layout) *launch.UnitRunner
	git         func(dir string, args ...string) ([]byte, error)
	wait        func(args []string, print func(metarun.WaitResult, bool), stdout, stderr io.Writer) int
	// testRun is the testing runner, reached with the argv its former child
	// carried; it returns the structured result it prints and its exit.
	testRun  func(dir string, argv []string, stderr io.Writer) ([]byte, int, error)
	settings func(confPath string) (launch.Settings, error)
	config   func(key, confPath string) (value, source string, code int, err error)

	resolveModel func(confPath, runtime, model string) (string, error)
	inspectRead  func(root, goalID, commit string) (branch.BranchReadResult, error)
	// jobWatch and runWatch are the job and tracked-run waiters work wait
	// --exit-code blocks in, with their own pinned exit codes.
	jobWatch command
	runWatch command
	// waitClock replaces the default wait owner's clock (see
	// runWaitCommandOnClock); nil keeps the kernel clock.
	waitClock func(*metarun.WaitOptions)
}

// intentConfPath is the selected installation's configuration file.

// unitReadFindingsClass is the class of the temporary store a unit's read
// writes its findings into (Part B R1): owned by the unit's named inputs,
// released with its unit (Round D3 N4).
const unitReadFindingsClass = diskstore.UnitReadFindingsClass

func intentConfPath(layout stateroot.Layout) string {
	return layout.InstallationRoot.Path("metasystem.conf")
}

func unitLaunchSettings(layout stateroot.Layout, serving func(string) (string, string), lookupEnv func(string) (string, bool)) (launch.Settings, error) {
	installation, _ := serving(layout.InstallationRoot.Path())
	return launch.ResolveSettings(filepath.Join(installation, "metasystem.conf"), lookupEnv)
}

func (inv *intentInvocation) work() intentWorkOwners {
	owners := inv.owners.work
	if owners.adapter == nil {
		owners.adapter = adapter.Detect
	}
	if owners.inspectRead == nil {
		owners.inspectRead = branch.InspectBranchRead
	}
	if owners.units == nil {
		// The serving installation supplies launch settings; launch and unit
		// records stay the user's own stores.
		owners.units = func(layout stateroot.Layout) *launch.UnitRunner {
			manager := launchManager()
			if layout.InstallationRoot != "" {
				manager.Settings, manager.SettingsError = unitLaunchSettings(layout, delegationToolInstallation, launchLookupEnv)
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
		clock := owners.waitClock
		owners.wait = func(args []string, print func(metarun.WaitResult, bool), stdout, stderr io.Writer) int {
			return runWaitCommandOnClock(args, nil, waitCallerPID(), print, clock, stdout, stderr)
		}
	}
	if owners.testRun == nil {
		// The testing runner runs in this process (design 6.2): this process
		// is the caller its proof admission classifies, the parent the former
		// child classified, and its result comes back on a buffer as the
		// child's standard output did.
		owners.testRun = func(_ string, argv []string, stderr io.Writer) ([]byte, int, error) {
			if len(argv) < 3 || argv[0] != "internal" || argv[1] != "test" || argv[2] != "run" {
				return nil, 1, fmt.Errorf("the testing runner takes internal test run, not %q", argv)
			}
			var stdout bytes.Buffer
			code := runTestRunWith(testRunInvocation{callerPID: int64(os.Getpid()), stdout: &stdout, stderr: stderr, name: "internal test run"}, argv[3:])
			return stdout.Bytes(), code, nil
		}
	}
	if owners.jobWatch == nil {
		owners.jobWatch = runJobWatchVerb
	}
	if owners.runWatch == nil {
		owners.runWatch = runRunWatch
	}
	if owners.settings == nil {
		owners.settings = func(confPath string) (launch.Settings, error) {
			return launch.ResolveSettings(confPath, launchLookupEnv)
		}
	}
	if owners.config == nil {
		owners.config = configSettingWithDefault
	}
	if owners.resolveModel == nil {
		owners.resolveModel = func(confPath, runtime, model string) (string, error) {
			model, _, err := config.ResolveModelAlias(confPath, runtime, model)
			return model, err
		}
	}
	return owners
}

// configSettingWithDefault resolves one key with its source. A key with a
// compiled-in default that no source holds (the one table, defaults.go, the
// disk-lifetime settings included) answers that default, source "default".
func configSettingWithDefault(key, confPath string) (string, string, int, error) {
	params := config.GetParams{Key: key, ConfPath: confPath}
	value, code, err := config.Get(params)
	if err != nil || code != 0 {
		return "", "", code, err
	}
	source, err := config.KeyOrigin(params)
	if err != nil || !config.RuntimeSelectionKey(key) {
		return value, source, 0, err
	}
	raw := params
	raw.KeepAuto = true
	if configured, _, rawErr := config.Get(raw); rawErr == nil && configured == config.AutoRuntime {
		choice, choiceErr := config.ResolveAutoRuntime(confPath, nil)
		if choiceErr != nil {
			return "", "", 1, choiceErr
		}
		source += "; " + choice.Describe()
	}
	return value, source, 0, nil
}

var intentBriefFlag = intentFlag{name: "brief", value: "FILE", usage: "the brief, relative to the directory the command runs in"}

func intentWorkCommands() []intentCommand {
	return []intentCommand{
		{
			object: "work", action: "brief", laidOut: true, audience: "agent", summary: "write a brief scaffold from a goal and its accepted design",
			usage: []string{"metasystem work brief G --out FILE"},
			details: []string{
				"Carries the goal's intent and done criteria, its approval and box, its branch worktree, and the accepted design's",
				"units, constraints, return and acceptance sections. A decision neither record holds is written as a MISSING DECISION",
				"line, and work build refuses the brief until each is filled. An existing different FILE is never overwritten.",
			},
			flags:    []intentFlag{intentTargetFlag, {name: "out", value: "FILE", usage: "where to write the brief"}},
			maxArgs:  1,
			examples: []string{"metasystem work brief verbs-match-intent --out /tmp/work-brief.md"},
			run:      runIntentBrief,
		},
		{
			object: "work", action: "build", laidOut: true, primary: true, audience: "agent", summary: "build and test a goal's work, ready for independent review",
			usage: []string{
				"metasystem work build G [--work NAME] --brief FILE --check COMMAND...",
				"metasystem work build run:RUN",
			},
			details: []string{
				"Needs an approved goal. A ready goal nobody holds is claimed for this session first, under the claim's own rules; a goal",
				"another session holds is never taken. The goal's own worktree is prepared or reused; this checkout is left as it is.",
				"A work name is the caller's name for one part of the goal; without --work the first build is main, and a goal with one",
				"work item continues it. The same goal, work and request reach the same attempt again; a different request is refused",
				"and is sent as a correction with work revise.",
				"--check ends the options: every later word is the proof command's argument vector, run without a shell.",
				"The size is the work's row in the brief's or the accepted design's units table; without a row give --lines N.",
				"The first read may use the tool calls the brief names (Maximum reader tool calls: N), --read-tool-calls N, or else",
				"the configured intent.review.tool-calls allowance (48 unless metasystem.conf says otherwise).",
				"The build ends awaiting review, green or red, with the first read's verdict as preliminary feedback. Nothing is approved,",
				"certified or landed: work review G examines the result independently.",
				"work build run:RUN continues that unit run; a bare id that names a unit run and no goal does the same.",
			},
			flags: []intentFlag{
				{name: "work", value: "NAME", usage: "the goal's named work (default: main, or the goal's only work)"},
				{name: "last", usage: "marks this unit as the goal's last; for a goal whose design has no Units table"},
				intentLineageFlag,
				intentBriefFlag,
				{name: "lines", value: "N", usage: "the unit's changed-line estimate, when no units table has its row"},
				{name: "read-tool-calls", value: "N", usage: "the independent read's tool-call budget, when the brief does not name it"},
				{name: "model", value: "MODEL", advanced: true, usage: "the build model for this unit instead of launch.build.model"},
				{name: "effort", value: "EFFORT", advanced: true, usage: "the build effort for this unit instead of launch.build.effort"},
				{name: "plan", value: "FILE", advanced: true, hidden: true, usage: "an existing unit plan (the unit run plan format)"},
				{name: "check", value: "COMMAND...", rest: true, usage: "the proof command, run in the goal worktree's copy of this folder; it ends the options"},
			},
			maxArgs: 2,
			accepts: []string{refGoal, refRun},
			examples: []string{
				"metasystem work build verbs-match-intent --brief work-brief.md --check go test -count=1 -run 'TestIntent' ./cmd/metasystem/",
				"metasystem work build verbs-match-intent --work discovery --brief discovery.md --check go test ./cmd/metasystem/",
				"metasystem work build run:20260925T101500Z-abc123",
			},
			run: runIntentBuild,
		},
		{
			object: "work", action: "wait", laidOut: true, audience: "agent", summary: "wait for a goal's work, its landing, a person's act, a job or a read",
			usage: []string{"metasystem work wait G [--work NAME] [--timeout DURATION]",
				"metasystem work wait G --for landing|human-act [--verb V] [--since TIP] [--timeout DURATION]",
				"metasystem work wait REF [--timeout DURATION]",
				"metasystem work wait j2:J --exit-code [--caller-pid PID]",
				"metasystem work wait --run ID --exit-code",
				"metasystem work wait --path PATH --until present|absent [--timeout DURATION]",
				"metasystem work wait --list [--session S]"},
			details: []string{
				"work wait G continues the goal's running work: the one named with --work, or the only work item that is running.",
				"With nothing running it says so; when the goal is queued to land it offers work wait G --for landing.",
				"--for landing waits until the goal lands; --for human-act waits for a person's act on it, only the act --verb names when given.",
				"--since TIP waits for events after that goal-ledger revision (default: the ledger as it is now).",
				"REF is a reference as a result printed it: j1:ID or j2:ID (a job), run:ID (a unit run), read:REF (a diagnostic read)",
				"or wait:ID, which resumes that recorded wait instead of starting a new one. A bare id works when it names exactly one record.",
				"A wait that reaches its timeout reports the work as still in progress and prints the exact command that continues the wait.",
				"--path waits for a file to be present or absent; a relative PATH is from the current directory.",
				"--exit-code blocks until the dispatch job, or the tracked run --run names, is terminal and exits with its pinned code.",
				"--list shows this checkout's recorded waits that a later work wait wait:ID resumes, checked against --session when given.",
			},
			flags: []intentFlag{
				{name: "work", value: "NAME", usage: "the goal's named work to wait for"},
				{name: "timeout", value: "DURATION", usage: "how long this invocation waits (for example 20s or 10m)"},
				{name: "for", aliases: []string{"event"}, value: "EVENT", usage: "landing or human-act: wait for that goal event instead of work"},
				{name: "since", aliases: []string{"after"}, value: "TIP", usage: "with --for: the goal-ledger revision to wait after (default: the current one)"},
				{name: "verb", value: "VERB", usage: "with --for human-act: the person's act to wait for"},
				{name: "path", value: "PATH", usage: "a file to wait for, with --until"},
				{name: "until", value: "STATE", usage: "with --path: present or absent"},
				{name: "question", value: "ID", advanced: true, usage: "answer waits: the question"},
				{name: "chain", value: "ROOT", advanced: true, usage: "landing waits: the delegate chain root"},
				{name: "exit-code", usage: "with j2:J or --run ID: block until terminal and exit with the pinned code"},
				{name: "run", value: "ID", usage: "with --exit-code: the tracked run (a suite, cohort or custom run) to wait for"},
				{name: "caller-pid", value: "PID", advanced: true, usage: "with --exit-code: the caller whose exit ends the wait (default: the parent process)"},
				{name: "list", usage: "list the recorded waits a later work wait wait:ID resumes"},
				{name: "session", value: "S", advanced: true, usage: "with --list: the runtime session that must hold the checkout"},
			},
			maxArgs: 1,
			accepts: []string{refGoal, refJ1, refJ2, refRun, refRead, refWait},
			examples: []string{"metasystem work wait verbs-match-intent", "metasystem work wait verbs-match-intent --for landing", "metasystem work wait verbs-match-intent --for human-act --verb approve",
				"metasystem work wait j2:20260925-job-1 --timeout 10m", "metasystem work wait j2:impl-01 --exit-code", "metasystem work wait --list"},
			run: runIntentWorkWait,
		},
		{
			object: "test", action: "run", laidOut: true, audience: "both", summary: "run the risk-selected tests for this checkout",
			usage: []string{"metasystem test run [--goal G] [--authority H] [--mode auto|standard|deep]"},
			details: []string{
				"Runs the risk-selected tests for this checkout and reports the result, naming its proof attempt; test wait proof:ID reads that attempt's recorded end.",
				"test plan previews the selected tests and their reasons without running them; test status says whether retained proof already covers an exact tree.",
			},
			flags: []intentFlag{
				{name: "goal", value: "G", usage: "the accepted goal owning the delivery"},
				{name: "authority", value: "H", advanced: true, usage: "the claimed goal authorizing the proof reservation"},
				{name: "mode", value: "MODE", usage: "auto (default), standard or deep"},
				testVerboseFlag,
			},
			maxArgs:  0,
			examples: []string{"metasystem test run", "metasystem test run --goal verbs-match-intent --mode standard"},
			run:      runIntentTest,
		},
		{
			object: "test", action: "declare-moves", laidOut: true, audience: "agent", summary: "record that this change moves or removes Stop test assertions on purpose",
			usage: []string{"metasystem test declare-moves G --reason TEXT [--base COMMIT]"},
			details: []string{
				"The static gate refuses a change that removes or changes an assertion deciding whether work must stop, unless the change carries a declaration.",
				"This writes that declaration under docs/stop-decision-moves for goal G, which a person must first allow: metasystem goal allow G stop-test-changes --reason TEXT.",
				"Commit the file with the change; a declaration binds exactly the moves it names, so a later change needs its own.",
			},
			flags: []intentFlag{reasonFlag("why", "why this change moves a Stop decision"),
				{name: "base", value: "COMMIT", advanced: true, usage: "the base the moves are measured from (default: the merge base with origin/main)"}},
			maxArgs: 1, examples: []string{"metasystem test declare-moves verbs-match-intent --reason 'the Stop checks moved into Go'"},
			run: runIntentDeclareStopMoves,
		},
		{
			object: "test", action: "wait", laidOut: true, audience: "agent", summary: "wait for a proof attempt's recorded end",
			usage:    []string{"metasystem test wait proof:ID [--timeout DURATION]"},
			flags:    []intentFlag{{name: "timeout", value: "DURATION", usage: "how long this invocation waits (for example 20s or 10m)"}},
			maxArgs:  1,
			accepts:  []string{refProof},
			examples: []string{"metasystem test wait proof:20260925T101500Z-1a2b --timeout 10m"},
			run:      runIntentTestWait,
		},
		{
			object: "settings", action: "show", laidOut: true, audience: "both", summary: "the launch settings, or one setting with its source",
			usage: []string{"metasystem settings show [KEY]"},
			details: []string{"Without KEY: the launch settings. With KEY: that launch setting or any metasystem.conf key.",
				"Read only. settings set changes one for this seat; proof.full and proof.cheap belong to metasystem.conf. settings check validates them all."},
			maxArgs:  1,
			examples: []string{"metasystem settings show", "metasystem settings show launch.read.model"},
			run:      runIntentSettings,
		},
		{
			object: "settings", action: "keys", laidOut: true, audience: "both", summary: "every configured key of the selected installation, with its source",
			usage:    []string{"metasystem settings keys [--matching PREFIX]"},
			flags:    []intentFlag{{name: "matching", value: "PREFIX", usage: "only keys starting with PREFIX"}},
			maxArgs:  0,
			examples: []string{"metasystem settings keys", "metasystem settings keys --matching launch."},
			run:      runIntentSettingsKeys,
		},
		{
			object: "settings", action: "set", laidOut: true, audience: "both", summary: "set one configuration key for this checkout's seat",
			usage: []string{"metasystem settings set KEY VALUE"},
			details: []string{"Writes seat settings into metasystem.conf.local, the seat's own layer over metasystem.conf.",
				"proof.full and proof.cheap are committed-only repository declarations; declare them in metasystem.conf through a goal and land it on main. settings set refuses them.",
				"KEY must be declared; settings check reports undeclared keys already in the local file.",
				"A key already holding the value is left as it is."},
			maxArgs:  2,
			examples: []string{"metasystem settings set role.default.model.claude claude-opus-5-5"},
			run:      runIntentSettingsSet,
		},
		{
			object: "settings", action: "check", laidOut: true, audience: "both", summary: "validate every setting and the testing contract, changing nothing",
			usage: []string{"metasystem settings check"},
			details: []string{"Validates metasystem.conf, then the testing contract it names and the contract's declared tools; no test runs.",
				"A test run resolves each group's native tests when it runs; this check does not take the host's proof lease."},
			maxArgs:  0,
			examples: []string{"metasystem settings check"},
			run:      runIntentSettingsCheck,
		},
		{
			object: "settings", action: "coordinator", audience: "both", summary: "whether this checkout is its ledger's coordinator; declare or withdraw it",
			usage: []string{"metasystem settings coordinator [--declare|--withdraw --by NAME]"},
			details: []string{"--declare and --withdraw are a person's act at an agent-free terminal, and declaring needs this machine quiet",
				"(no claimed goal, job, run or mission here)."},
			flags: []intentFlag{
				{name: "declare", usage: "declare this checkout"},
				{name: "withdraw", usage: "withdraw the declaration"},
				{name: "by", value: "NAME", usage: "the person deciding"},
			},
			maxArgs:  0,
			examples: []string{"metasystem settings coordinator", "metasystem settings coordinator --declare --by Wido"},
			run:      runIntentSettingsCoordinator,
		},
	}
}

// runIntentTestWait waits for one test run's recorded end.
func runIntentTestWait(inv *intentInvocation) int {
	if len(inv.input.args) != 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "test wait needs the test run to wait on; nothing was done",
			Decision: "nothing to wait on yet; metasystem test run prints the reference to wait on"})
	}
	ref, problem := inv.resolveWorkRef(inv.input.args[0], inv.command.accepts)
	if problem != nil {
		return inv.render(*problem)
	}
	return runIntentWaitObserved(inv, "proof", ref.id)
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
			Summary: notAnInstallation(path, err),
			next:    append(inv.typedArgvLess("repo"), "--repo", "REPOSITORY"), nextReason: "REPOSITORY is a checkout MetaSystem is set up in; or run it inside one"}
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
	runner.ExaminationRoot = inv.layout.InstallationRoot.Path()
	runner.ExaminationRead = dispatchcore.CollectExamination
	runner.InheritedFindings = func(id, unit string) ([]readsubject.Finding, error) {
		if id == "" {
			return nil, nil
		}
		if inv.stateRoot == "" {
			if problem := inv.selectRoot(); problem != nil {
				return nil, fmt.Errorf("the inherited goal evidence cannot be located: %s", problem.Summary)
			}
		}
		projection, _, problem := inv.projection()
		if problem != nil {
			return nil, fmt.Errorf("the inherited goal evidence cannot be read: %s", problem.Summary)
		}
		file, _ := goalRecord(projection, id)
		if file == nil {
			return nil, fmt.Errorf("goal %s is unavailable", id)
		}
		var findings []readsubject.Finding
		for _, obligation := range file.ReviewObligations {
			if obligation.TargetUnit == unit {
				findings = append(findings, obligation.OriginalEvidence)
			}
		}
		return findings, nil
	}
	runner.BeforeModelLaunch = inv.unitLaunchAuthority
	runner.PlanProof = inv.unitProof
	judge := landingFlakeJudge(inv.layout.InstallationRoot.Path(), func(root string, args ...string) (string, error) {
		data, err := inv.work().git(root, args...)
		return string(data), err
	}, inv.layout.InstallationRel)
	runner.KnownFlake = func(record launch.Record) (bool, error) {
		state, err := runner.Manager.Store.StateDir(record.ID)
		if err != nil {
			return false, err
		}
		data, err := os.ReadFile(filepath.Join(state, "exec.log"))
		if err != nil {
			return false, err
		}
		failed := plain.FailedChecks(data)
		if len(failed) == 0 {
			return false, nil
		}
		root, err := inv.work().git(record.WorkingDirectory, "rev-parse", "--show-toplevel")
		if err != nil {
			return false, err
		}
		// A judge error (a unit new on main, a unit id that is not a path) means
		// "not a known flake", as at landing (plain/replay.go): the base comparison decides.
		judgements, err := judge(strings.TrimSpace(string(root)), "HEAD", failed)
		if err != nil {
			return false, nil
		}
		known := true
		for _, unit := range failed {
			known = known && judgements[unit.Unit].Known && !judgements[unit.Unit].Affected
		}
		return known, nil
	}
	runner.RecordMain = func(record launch.Record, commit, tree string) error {
		state, err := runner.Manager.Store.StateDir(record.ID)
		if err != nil {
			return err
		}
		log := filepath.Join(state, "exec.log")
		data, err := os.ReadFile(log)
		if err != nil {
			return err
		}
		return inv.landing().landingIncidentRecorder(inv.layout.InstallationRoot.Path())([]plain.Result{{Result: plain.Red, Commit: commit, Tree: tree, Attempt: record.ID, Log: log, Failed: plain.FailedChecks(data)}})
	}
	if runner.ReviewPolicy == nil {
		runner.ReviewPolicy = func() (string, error) {
			params, err := inv.policyParams("review.stop")
			if err != nil {
				repair, _ := inv.reviewPolicyRepair(err)
				return "", fmt.Errorf("review.stop cannot be read; %s: %w", shellCommand(repair), err)
			}
			policy, err := config.ResolvePolicy(params)
			if err != nil {
				repair, _ := inv.reviewPolicyRepair(err)
				return "", fmt.Errorf("review.stop cannot be read; %s: %w", shellCommand(repair), err)
			}
			return policy.Value, err
		}
	}
	return runner
}

func (inv *intentInvocation) unitProof(plan launch.UnitPlan) ([]launch.ProofCommand, error) {
	bound, err := inv.work().adapter(plan.Worktree)
	if err != nil || bound == nil {
		return nil, nil
	}
	closure, err := bound.Closure(plan.Worktree, plan.Base, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("cannot plan the round's tests: %w", err)
	}
	directory := closure.Root
	if directory == "" {
		directory = plan.Worktree
	}
	proof := []launch.ProofCommand{}
	for _, step := range bound.TestSteps(closure) {
		proof = append(proof, launch.ProofCommand{Name: step.Name, Dir: directory, Argv: append([]string{}, step.Args...), Env: []string{}})
	}
	return append(proof, plan.Proof...), nil
}

// unitLaunchAuthority is asked before every build or read launch of a unit
// run this command advances, new or continued: the goal must be claimed by
// this session and the checkout lease held, resolved through the selected
// installation's own configuration.
func (inv *intentInvocation) unitLaunchAuthority(record launch.UnitRunRecord, _ launch.StartSpec) error {
	conn := inv.connection()
	endpoint, err := conn.endpoint(inv.layout.InstallationRoot.Path())
	if err != nil {
		return err
	}
	return branch.CheckHolder(conn.claimCheck(inv.layout.InstallationRoot.Path(), record.Goal, endpoint))
}

// build

func runIntentBuild(inv *intentInvocation) int {
	if problem := inv.buildEngineAdmission(); problem != nil {
		return inv.render(*problem)
	}
	if inv.input.has("plan") {
		return runIntentBuildPlan(inv)
	}
	if len(inv.input.args) == 1 && !inv.input.has("brief") && !inv.input.has("check") {
		// One word without a request continues a unit run it names.
		ref, problem := inv.resolveWorkRef(inv.input.args[0], inv.command.accepts)
		if problem != nil {
			return inv.render(*problem)
		}
		if ref.kind == refRun {
			for _, conflicting := range []string{"lines", "model", "effort", "read-tool-calls", "work"} {
				if inv.input.has(conflicting) {
					return inv.render(intentResult{Outcome: intentRefused, code: 2,
						Summary: fmt.Sprintf("building %s continues a recorded run, which takes no --%s; nothing was done", ref.qualified(), conflicting),
						next:    inv.publicArgv("work", "build", ref.qualified()), nextReason: "continues the run"})
				}
			}
			runner := inv.unitRunner()
			result, err := runner.Continue(launch.UnitRequest{Resume: ref.id})
			return inv.render(inv.unitOutcome(runner, result, err, []intentTarget{{Kind: "run", ID: ref.qualified()}}, inv.publicArgv("work", "build", ref.qualified())))
		}
	}
	if len(inv.input.args) > 0 {
		if kind, _ := splitReference(inv.input.args[0]); kind != "" {
			if kind != refRun {
				return inv.render(*inv.refusedKind(inv.input.args[0], kind))
			}
			return inv.render(intentResult{Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("building %s continues a recorded run, which takes no brief, checks or work name; nothing was done", inv.input.args[0]),
				next:    inv.publicArgv("work", "build", inv.input.args[0]), nextReason: "continues the run"})
		}
	}
	return runIntentBuildUnit(inv)
}

// buildEngineAdmission refuses to start work when main changed machinery
// after this engine was built. Without source or a comparable stamp, or
// when Git cannot establish the changes, admission stays open.
func (inv *intentInvocation) buildEngineAdmission() *intentResult {
	stamp := inv.work().engineStamp
	if stamp == "" {
		stamp = supervise.BuildStamp
	}
	if stamp == "dev" || strings.HasPrefix(stamp, "dev-") || strings.HasPrefix(stamp, "witness-") {
		return nil
	}
	if inv.resolveLayout() != nil {
		return nil
	}
	root := inv.layout.InstallationRoot.Path()
	if source, err := os.Stat(filepath.Join(root, "cmd", "metasystem")); err != nil || !source.IsDir() {
		return nil
	}
	git := inv.work().git
	output, err := git(root, "log", "--format=", "--name-only", stamp+"..origin/main", "--", "internal", "cmd")
	if err != nil {
		return nil
	}
	paths := 0
	for _, path := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(path) != "" {
			paths++
		}
	}
	if paths == 0 {
		return nil
	}
	tip, err := git(root, "rev-parse", "origin/main")
	if err != nil {
		return nil
	}
	return &intentResult{Outcome: intentRefused, code: 1,
		Summary: "main changed the build machinery after this engine was built; nothing was started",
		next:    []string{"go", "run", "./cmd/devgate", "build"}, nextReason: "rebuild in " + root + ", then repeat this command",
		Details: []string{fmt.Sprintf("BUILD_ENGINE_STALE engine=%s main=%s paths=%d", stamp, strings.TrimSpace(string(tip)), paths)}}
}

func runIntentBuildPlan(inv *intentInvocation) int {
	for _, conflicting := range []string{"brief", "check", "lines", "model", "effort", "read-tool-calls"} {
		if inv.input.has(conflicting) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("--plan already holds the brief, size, review and checks, so --%s doesn't go with it; nothing was done", conflicting),
				next:    inv.typedArgvLess(conflicting), nextReason: "builds from the plan alone"})
		}
	}
	path := inv.callerPath(inv.input.text("plan"))
	plan, err := launch.ReadUnitPlan(path)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, Summary: err.Error() + "; nothing was built", code: 1,
			next: inv.sameCommand(), nextReason: "once --plan names a readable plan"})
	}
	if named := inv.input.args; len(named) > 0 && (named[0] != plan.Goal || len(named) > 1 && named[1] != plan.Unit) {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("the plan is work %s of goal %s, not %s; nothing was built", plan.Unit, plan.Goal, strings.Join(named, " ")),
			next:    inv.publicArgv("work", "build", "--plan", path), nextReason: "builds what the plan names"})
	}
	runner := inv.unitRunner()
	result, err := runner.AdvanceNamed(path)
	targets := []intentTarget{{Kind: "goal", ID: plan.Goal}, {Kind: "unit", ID: plan.Unit}}
	return inv.render(inv.unitOutcome(runner, result, err, targets, inv.sameCommand()))
}

var (
	intentModelPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:\[\]-]*$`)
	intentEffortOptions = []string{"minimal", "low", "medium", "high", "xhigh", "max"}
)

func runIntentBuildUnit(inv *intentInvocation) int {
	if len(inv.input.args) < 1 || len(inv.input.args) > 2 || !inv.input.has("brief") || !inv.input.has("check") {
		retry := inv.sameCommand()
		if len(inv.input.args) == 0 {
			retry = inv.typedArgvFor("GOAL")
		}
		if !inv.input.has("brief") {
			retry = append(retry, "--brief", "FILE")
		}
		if !inv.input.has("check") {
			retry = append(retry, "--check", "COMMAND")
		}
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "work build needs one goal, a brief and the checks to run; nothing was done",
			next:    retry, nextReason: "FILE is the brief; COMMAND is the check that must pass (metasystem work build --help)"})
	}
	if len(inv.input.args) == 2 && inv.input.has("work") && inv.input.args[1] != inv.input.text("work") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("the work is named twice, %s and --work %s; nothing was done", inv.input.args[1], inv.input.text("work")),
			next:    inv.typedArgvLess("work"), nextReason: "keeps " + inv.input.args[1]})
	}
	if model := inv.input.text("model"); inv.input.has("model") && !intentModelPattern.MatchString(model) {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--model %s is not a model name; nothing was done", shellCommand([]string{model})),
			next: inv.typedArgvLess("model"), nextReason: "the roster's model"})
	}
	if effort := inv.input.text("effort"); inv.input.has("effort") && !slices.Contains(intentEffortOptions, effort) {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("--effort must be one of %s, not %s; nothing was done", strings.Join(intentEffortOptions, ", "), shellCommand([]string{effort})),
			next:    append(inv.typedArgvLess("effort"), "--effort", "high"), nextReason: "or another of those"})
	}
	id, unit := inv.input.args[0], inv.input.text("work")
	if len(inv.input.args) == 2 {
		unit = inv.input.args[1]
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	var ambiguous []string
	if unit == "" {
		// Unnamed work is the goal's only work item, or main for its first.
		work, problem := inv.goalWork(id)
		if problem != nil {
			return inv.render(*problem)
		}
		switch len(work) {
		case 0:
			unit = "main"
		case 1:
			unit = work[0].Unit
		default:
			// Several work items: the one whose retained request is exactly
			// this request is the one being repeated; otherwise the caller
			// names it.
			for _, one := range work {
				ambiguous = append(ambiguous, one.Unit)
			}
			unit = work[0].Unit
		}
	}
	targets := []intentTarget{{Kind: "goal", ID: id}, {Kind: "work", ID: unit}}
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
			Summary:  fmt.Sprintf("goal %s is %s; nothing was built", id, where),
			Decision: "nothing to do; only an open goal is built"})
	}
	if file.Budget == nil || file.Budget.ReviewRoundLimit <= 0 {
		return inv.render(intentResult{Outcome: intentRefused, Targets: targets, code: 1,
			Summary: fmt.Sprintf("goal %s is not approved with a budget yet; nothing was built", id),
			next:    inv.publicArgv("goal", "approve", id), nextReason: "a person approves it; then repeat this command"})
	}
	if file.State != goal.StateClaimed {
		// An approved goal nobody holds is claimed through the claim owner,
		// with its readiness, quota and elapsed checks; its refusal is the
		// build's answer.
		if claimed := inv.acquireClaim(id); claimed.Outcome != intentConfirmed {
			claimed.Summary = fmt.Sprintf("build claims goal %s first, and the claim was not granted: %s; nothing was built", id, strings.TrimSpace(claimed.Summary))
			return inv.render(claimed)
		}
	}
	// The goal must be this session's before anything is reserved: a goal
	// another session holds is refused with the claim owner's own reason.
	conn := inv.connection()
	if endpoint, err := conn.endpoint(inv.layout.InstallationRoot.Path()); err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: "the goal branch can't be reached, so nothing was built",
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}.withCause(err))
	} else if err := branch.CheckHolder(conn.claimCheck(inv.layout.InstallationRoot.Path(), id, endpoint)); err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: err.Error() + "; nothing was built",
			next: inv.publicArgv("goal", "claim", id, "--take-over", "--reason", "TEXT"), nextReason: "a person takes the goal over; or the session holding it builds",
			Details: refusalCodeDetails(goal.RefusalCode(err))})
	}
	designs, problem := inv.acceptedDesignPaths(id)
	if problem != nil {
		problem.Targets = targets
		return inv.render(*problem)
	}
	gateFacts := inv.designGateFacts(string(inv.layout.InstallationRoot), id)
	gateResult := designgate.Check(gateFacts)
	person := !inv.input.has("lineage") && (inv.owners.dependencies.ownerLineage == nil || inv.owners.dependencies.ownerLineage() == "")
	if gateResult.Mode == "refuse" && gateResult.WouldRefuse {
		reason := strings.TrimPrefix(strings.SplitN(gateResult.Warning[0], ";", 2)[0], "warning: ")
		if !person {
			detail := fmt.Sprintf("BUILD_DESIGN_NOT_ACCEPTED goal=%s verdict=%s", id, gateResult.Verdict)
			if ruling := refusal.GovernedBy["BUILD_DESIGN_NOT_ACCEPTED"]; ruling != "" {
				detail += " governed-by=" + ruling
			}
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets,
				Summary: reason + "; nothing was built", Data: map[string]any{"designGate": gateResult},
				next:    inv.publicArgv("goal", "allow", id, goal.PermissionBuildWithoutDesign, "--reason", "TEXT"),
				Details: []string{detail}})
		}
		gateResult.Warning[0] = "warning: " + reason + "; it goes on at your word"
	}
	if gateResult.Warning[0] != "" {
		fmt.Fprintln(inv.stderr, gateResult.Warning[0]+"\n"+gateResult.Warning[1])
	}
	runner := inv.unitRunner()
	request, problem := inv.unitRequest(runner, id, unit, designs, int(file.Budget.ReviewRoundLimit))
	if problem != nil {
		problem.Targets = targets
		return inv.render(*problem)
	}
	if len(ambiguous) > 0 {
		var matched []string
		for _, name := range ambiguous {
			if retained, found, err := runner.RetainedRequest(request.worktree, id, name); err == nil && found && bytes.Equal(retained, request.bytes) {
				matched = append(matched, name)
			}
		}
		if len(matched) != 1 {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Data: map[string]any{"candidates": ambiguous},
				Summary: fmt.Sprintf("goal %s has %d work items (%s), and this request matches %d of them; nothing was built", id, len(ambiguous), strings.Join(ambiguous, ", "), len(matched)),
				next:    append(inv.typedArgvLess("work"), "--work", firstOr(matched, ambiguous[0])), nextReason: "or another work name"})
		}
		if unit = matched[0]; unit != ambiguous[0] {
			targets = []intentTarget{{Kind: "goal", ID: id}, {Kind: "work", ID: unit}}
			if request, problem = inv.unitRequest(runner, id, unit, designs, int(file.Budget.ReviewRoundLimit)); problem != nil {
				problem.Targets = targets
				return inv.render(*problem)
			}
		}
	}
	prepare := request.prepare
	request.prepare = func(directory string) (string, error) {
		path, err := prepare(directory)
		if err == nil {
			inv.recordDesignGate(filepath.Dir(filepath.Dir(directory)), request.worktree, unit, gateFacts, gateResult, person)
		}
		return path, err
	}
	result, err := runner.AdvancePrepared(request.worktree, id, unit, request.bytes, request.options, request.prepare)
	outcome := inv.unitOutcome(runner, result, err, targets, inv.sameCommand())
	if data, ok := outcome.Data.(map[string]any); ok {
		data["inputs"] = request.directory
		data["designGate"] = gateResult
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
		return unitRequest{}, &intentResult{Outcome: intentRefused, code: 1, Summary: fileProblem("brief", briefPath, err) + "; nothing was built",
			next: inv.sameCommand(), nextReason: "once --brief names a readable file"}
	}
	if missing := missingDecisionLines(brief); len(missing) > 0 {
		return unitRequest{}, &intentResult{Outcome: intentRefused, code: 1, text: missing,
			Summary: fmt.Sprintf("the brief %s still has %d missing decision(s), listed above; nothing was built", shellCommand([]string{briefPath}), len(missing)),
			next:    inv.sameCommand(), nextReason: "after filling each MISSING DECISION line in the brief"}
	}
	readEachRound := intentReadEachRound.Match(brief)
	var toolCalls int
	if readEachRound {
		var problem *intentResult
		toolCalls, problem = inv.readToolCalls(brief)
		if problem != nil {
			return unitRequest{}, problem
		}
	}
	check := inv.input.values["check"]
	worktree, problem := inv.prepareGoalWorktree(id)
	if problem != nil {
		return unitRequest{}, problem
	}
	checkDir := inv.worktreeFolderHere(worktree)
	sizeSource, lines, problem := inv.unitSize(unit, briefPath, designs)
	if problem != nil {
		return unitRequest{}, problem
	}
	var readModel string
	if readEachRound {
		readModel, problem = unitReadModel(runner)
		if problem != nil {
			return unitRequest{}, problem
		}
	}
	templates := runner.Manager.Templates
	if templates == nil {
		templates = protocol.Templates()
	}
	var template []byte
	if readEachRound {
		template, err = fs.ReadFile(templates, "review-brief.md")
		if err != nil {
			return unitRequest{}, &intentResult{Outcome: intentFailed, code: 1, Summary: "the review brief template can't be read, so nothing was built",
				next: inv.publicArgv("system", "check"), nextReason: "checks the installation", Details: []string{err.Error()}}
		}
	}
	directory, err := runner.NamedInputDirectory(worktree, id, unit)
	if err != nil {
		return unitRequest{}, &intentResult{Outcome: intentFailed, code: 1, Summary: "the build's input folder can't be made, so nothing was built",
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
	}
	identity := struct {
		Brief         unitRequestFile   `json:"brief"`
		Designs       []unitRequestFile `json:"designs"`
		Check         []string          `json:"check"`
		Lines         string            `json:"lines,omitempty"`
		ReadToolCalls int               `json:"readToolCalls"`
		Model         string            `json:"model,omitempty"`
		Effort        string            `json:"effort,omitempty"`
		Whole         bool              `json:"whole,omitempty"`
	}{Check: check, Lines: inv.input.text("lines"), ReadToolCalls: toolCalls, Model: inv.input.text("model"), Effort: inv.input.text("effort"), Designs: []unitRequestFile{}, Whole: inv.input.switched("last")}
	if identity.Brief, err = fileIdentity(briefPath); err != nil {
		return unitRequest{}, &intentResult{Outcome: intentRefused, code: 1, Summary: fileProblem("brief", briefPath, err) + "; nothing was built",
			next: inv.sameCommand(), nextReason: "once --brief names a readable file"}
	}
	for _, design := range designs {
		entry, err := fileIdentity(design)
		if err != nil {
			return unitRequest{}, &intentResult{Outcome: intentRefused, code: 1, Summary: fileProblem("accepted design", design, err) + "; nothing was built",
				next: inv.sameCommand(), nextReason: "once the design is readable"}
		}
		identity.Designs = append(identity.Designs, entry)
	}
	encoded, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return unitRequest{}, &intentResult{Outcome: intentFailed, code: 1, Summary: "the build request can't be written down, so nothing was built",
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
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
		// once for the unit's named inputs and kept in the plan. The launch owner copies the
		// file into each read launch; the unit runner recreates a cleaned
		// directory before a later read.
		// It outlives this command (a later round's read writes into it),
		// so it is a registered temporary store the unit's named inputs own
		// (Part B R1), never an unowned TMPDIR entry.
		var findings string
		if readEachRound {
			findingsDirectory, err := diskstore.CreateTempStore(diskstore.UnitReadFindingsName(filepath.Base(directory)), unitReadFindingsClass,
				diskstore.Owner{Kind: diskstore.OwnerUnit, Ref: filepath.Base(directory)})
			if err != nil {
				return "", fmt.Errorf("cannot create the read's findings directory: %w", err)
			}
			findings = filepath.Join(findingsDirectory, "read-findings.md")
		}
		planPath := filepath.Join(directory, "plan.json")
		unitsPage := buildBrief
		if strings.HasPrefix(sizeSource, "design:") {
			unitsPage = strings.TrimPrefix(sizeSource, "design:")
		}
		binding := unitBinding{goal: id, unit: unit, worktree: worktree, base: base, brief: briefPath, designs: designs, check: check,
			estimate: sizeSource == "lines", unitsPage: unitsPage, lines: lines, findings: findings, rounds: rounds, toolCalls: toolCalls}
		plan := launch.UnitPlan{Unit: unit, Goal: id, Worktree: worktree, Base: base, Whole: identity.Whole,
			Build: launch.UnitBuildPlan{Brief: buildBrief, Inputs: append([]string{}, designs...), Outputs: []string{}, UnitsPage: unitsPage, Units: []string{unit}},
			Proof: []launch.ProofCommand{{Name: "check", Dir: checkDir, Argv: append([]string{}, check...), Env: []string{}}}}
		if readEachRound {
			plan.Read = launch.UnitReadPlan{Brief: readBrief, Inputs: append([]string{}, designs...), Outputs: []string{findings}, Model: readModel}
			if _, err := atomicfile.WriteText(readBrief, binding.readBrief(template, buildBrief), directory); err != nil {
				return "", err
			}
		}
		encodedPlan, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return "", err
		}
		for _, file := range []struct{ path, text string }{
			{buildBrief, binding.buildBrief(brief)},
			{planPath, string(encodedPlan) + "\n"},
		} {
			if _, err := atomicfile.WriteText(file.path, file.text, directory); err != nil {
				return "", err
			}
		}
		if _, err := launch.ReadUnitPlan(planPath); err != nil {
			return "", fmt.Errorf("the generated unit plan is invalid: %w", err)
		}
		pack := &launch.Manager{Templates: templates}
		if readEachRound {
			if _, err := pack.CheckPack(launch.StartSpec{Kind: "read", Brief: readBrief, WorkingDirectory: worktree}); err != nil {
				return "", fmt.Errorf("the generated read brief does not fill the review template: %w", err)
			}
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
				Summary: fmt.Sprintf("--read-tool-calls must be a positive whole number, not %s; nothing was done", shellCommand([]string{inv.input.text("read-tool-calls")})),
				next:    append(inv.typedArgvLess("read-tool-calls"), "--read-tool-calls", "30"), nextReason: "30 is an example allowance"}
		}
		if match := intentReaderToolCalls.FindSubmatch(brief); match != nil && string(match[1]) != strconv.Itoa(value) {
			return 0, &intentResult{Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("the brief allows the review %s tool calls but --read-tool-calls says %d; nothing was built", match[1], value),
				next:    inv.typedArgvLess("read-tool-calls"), nextReason: "uses the brief's allowance"}
		}
		return value, nil
	}
	if match := intentReaderToolCalls.FindSubmatch(brief); match != nil {
		if value, err := strconv.Atoi(string(match[1])); err == nil && value > 0 {
			return value, nil
		}
	}
	value, err := config.IntentReviewToolCalls(intentConfPath(inv.layout))
	if err != nil {
		return 0, &intentResult{Outcome: intentRefused, code: 1,
			Summary: "the review's tool-call allowance can't be read from the settings; nothing was built",
			next:    inv.typedArgvWith("--read-tool-calls", "30"), nextReason: "names it here; or correct " + config.IntentReviewToolCallsKey + " in the settings",
			Details: []string{err.Error()}}
	}
	return value, nil
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
		return "", &intentResult{Outcome: intentFailed, code: 1, Summary: "the repository's worktrees can't be listed, so nothing was built",
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
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
			Summary: "the project's design records can't be read, so the goal's design is unknown; nothing was done",
			next:    inv.publicArgv("design", "list"), nextReason: "shows what is wrong with the records", Details: []string{problem}}
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
				Summary: fmt.Sprintf("--lines must be a positive whole number of changed lines, not %s; nothing was done", shellCommand([]string{inv.input.text("lines")})),
				next:    append(inv.typedArgvLess("lines"), "--lines", "N"), nextReason: "N is your estimate of the changed lines"}
		}
		given = value
	}
	for index, page := range append([]string{brief}, designs...) {
		lines, err := launch.DeclaredUnitLines(page, unit)
		if err != nil {
			if missing := launch.UnsizedMissing(err); missing == "units-table" || missing == "row" || missing == "size-column" {
				continue
			}
			summary := fmt.Sprintf("the units table in %s can't be read; nothing was built", page)
			if launch.UnsizedMissing(err) != "" {
				// The table's own fault is the cause the person fixes.
				summary = fmt.Sprintf("%s (%s); nothing was built", err.Error(), page)
			}
			return "", 0, &intentResult{Outcome: intentRefused, code: 1, Summary: summary,
				next: inv.sameCommand(), nextReason: "after correcting the table", Details: []string{err.Error()}}
		}
		if given != 0 && given != lines {
			return "", 0, &intentResult{Outcome: intentRefused, code: 2,
				Summary: fmt.Sprintf("work %s is declared at %d lines in %s, but --lines says %d; nothing was built", unit, lines, page, given),
				next:    inv.typedArgvLess("lines"), nextReason: "uses the declared size; or correct the row"}
		}
		if index == 0 {
			return "brief", lines, nil
		}
		return "design:" + page, lines, nil
	}
	if given == 0 {
		return "", 0, &intentResult{Outcome: intentRefused, code: 1,
			Summary: fmt.Sprintf("the size of work %s is unknown: no units row in the brief or an accepted design; nothing was built", unit),
			next:    inv.typedArgvWith("--lines", "N"), nextReason: "N is your honest estimate of the changed lines; or add a units row to the brief",
			Details: []string{fmt.Sprintf("LAUNCH_BUILD_UNSIZED unit=%s", unit)}}
	}
	return "lines", given, nil
}

func unitReadModel(runner *launch.UnitRunner) (string, *intentResult) {
	if runner.Manager == nil {
		return "", &intentResult{Outcome: intentFailed, code: 1, Summary: "builds can't be started from this checkout, so nothing was built",
			Decision: "run metasystem system check: it names what the setup is missing"}
	}
	if runner.Manager.SettingsError != nil {
		return "", &intentResult{Outcome: intentFailed, code: 1, Summary: "the launch settings can't be read, so nothing was built",
			Decision: "run metasystem settings check: it names the broken setting", Details: []string{runner.Manager.SettingsError.Error()}}
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
	return "", &intentResult{Outcome: intentRefused, code: 1, Summary: "no model is set for the independent review, so nothing was built",
		Decision: "run metasystem settings set " + launch.ReadModelKey + " MODEL"}
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
		plain, details := launchAccount(err)
		switch {
		case launch.IsCode(err, "UNIT_STOPPED"):
			if len(record.Rounds) > 0 && (record.Rounds[len(record.Rounds)-1].Outcome == "build-gap" || record.Rounds[len(record.Rounds)-1].Stop != nil && record.Rounds[len(record.Rounds)-1].Stop.Loop == "unit-build") {
				next, _ := inv.workContinuation(record.Goal, launch.NamedWork{Unit: record.Unit, Record: &record}, true)
				return intentResult{Outcome: intentRefused, Targets: targets, code: 1, Summary: plain, Details: details,
					Data: unitData(record, runner.Manager), next: next}
			}
			return intentResult{Outcome: intentRefused, Targets: targets, code: 1, Summary: plain, Details: details,
				Data: unitData(record, runner.Manager), next: inv.workArgv(record, "review"), nextReason: "applies the unit's recorded review decision"}
		case launch.IsCode(err, "UNIT_ROUND_DIVERGENT"):
			var divergence *launch.CodedError
			if errors.As(err, &divergence) {
				plain = divergence.Reason.Error()
			}
			goalID, unit := record.Goal, record.Unit
			return intentResult{Outcome: intentRefused, Targets: targets, code: 1, Summary: plain, Details: details,
				next:       inv.publicArgv("work", "build", goalID, "--work", "NEW", "--brief", "FILE", "--check", "..."),
				nextReason: fmt.Sprintf("take-a-step-back; or land with metasystem work review %s --work %s", goalID, unit)}
		case launch.IsCode(err, "UNIT_ROUND_CAP"):
			var cap *launch.UnitRoundCapError
			if errors.As(err, &cap) {
				return intentResult{Outcome: intentRefused, Targets: targets, code: 1, Summary: cap.Reason.Error(), Details: details,
					next: inv.publicArgv(cap.Next...), nextReason: "the counted rounds are used; take this outcome"}
			}
		case launch.IsCode(err, "UNIT_RUN_BUSY"):
			return intentResult{Outcome: intentInProgress, Targets: targets, code: 3, Summary: "another command is advancing this work right now",
				next: again, nextReason: "the same command continues it", Details: details}
		case launch.IsCode(err, "UNIT_NAMED_INPUT_CHANGED"):
			goalID, unit := targetID(targets, "goal", "GOAL"), targetID(targets, "work", "NAME")
			return intentResult{Outcome: intentRefused, Targets: targets, code: 1, Summary: plain + "; nothing was launched",
				next:       inv.publicArgv("work", "revise", goalID, "--work", unit, "--brief", "FILE"),
				nextReason: "sends the change as a correction; or build it under another work name", Details: details}
		case record.ID != "":
			return intentResult{Outcome: intentFailed, Targets: append(targets, intentTarget{Kind: "unit", ID: record.ID}), code: 1, Summary: plain,
				Data: unitData(record, runner.Manager), next: inv.workArgv(record, "wait"), nextReason: "the work is recorded; continue it once the cause is fixed",
				Details: details}
		}
		return intentResult{Outcome: intentRefused, Targets: targets, code: 1, Summary: plain, next: again, nextReason: "once that is settled", Details: details}
	}
	if err := inv.syncBuildHolds(record); err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Summary: "the build hold questions could not be reconciled", Details: []string{err.Error()}, next: inv.workArgv(record, "wait")}
	}
	targets = append(targets, intentTarget{Kind: "unit", ID: record.ID})
	data := unitData(record, runner.Manager)
	if result.Capped {
		return intentResult{Outcome: intentInProgress, Targets: targets, code: 3, Data: data,
			Summary: fmt.Sprintf("run %s round %d is still running step %s (launch %s)", record.ID, result.Round, result.Step, result.Launch),
			next:    inv.workArgv(record, "wait"), nextReason: "continue waiting for this work"}
	}
	round := record.Rounds[len(record.Rounds)-1]
	line := unitJudgementLine(record, runner.Manager)
	if round.Outcome == "build-size" || round.Outcome == "build-gap" || round.Stop != nil && round.Stop.Loop == "unit-build" {
		next := inv.workArgv(record, "review", "--reason", "TEXT", "--by", "NAME")
		if round.Outcome != "build-size" {
			next, _ = inv.workContinuation(record.Goal, launch.NamedWork{Unit: record.Unit, Record: &record}, true)
		}
		return intentResult{Outcome: intentRefused, Targets: targets, code: 1, Data: data, Summary: round.Stop.Handoff + ": " + round.Stop.Class, next: next}
	}
	if round.Outcome == "read-compacted" {
		return intentResult{Outcome: intentFailed, Targets: targets, code: unitExitReadCompacted, Data: data, text: []string{line},
			Summary:  fmt.Sprintf("run %s round %d: the read was compacted and its verdict does not count", record.ID, round.Number),
			Decision: "judge the attempt; a correction is " + shellCommand(inv.workArgv(record, "revise", "--after", fmt.Sprint(round.Number), "--brief", "FILE")),
			next:     inv.workArgv(record, "review"), nextReason: "the checks passed: an independent review examines this result"}
	}
	verdict := "no read"
	if verdicts, _ := data["readVerdicts"].([]string); len(verdicts) > 0 {
		verdict = strings.Join(verdicts, "; ")
	}
	text := []string{line}
	for _, step := range round.Steps {
		if strings.HasPrefix(step.Name, "proof:") && (data["plan"] != record.Plan || step.State == launch.StepFailed) {
			line := step.Name + ": " + string(step.State)
			if step.Reason != "" {
				line += ": " + step.Reason
			}
			text = append(text, line)
		}
	}
	if round.ReadModel == "" {
		text = append(text, "No read ran this round; work review asks the committed read.")
	} else if clean, _ := data["readClean"].(bool); !clean && round.Outcome == "green" {
		text = append(text, "The build, its checks and the read finished, but the read did not return VERDICT: land; this is not a clean read.")
	}
	text = append(text, "A clean read of the build by another model than the builder's is the unit's read; any other read is feedback and work review asks the committed critic. Nothing is approved, certified or landed. A correction: "+shellCommand(inv.workArgv(record, "revise", "--after", fmt.Sprint(round.Number), "--brief", "FILE")))
	judged := intentResult{Outcome: intentConfirmed, Targets: targets, Data: data, text: text,
		Summary: fmt.Sprintf("unit %s of %s: run %s round %d awaits judgement (%s; read verdict: %s)", record.Unit, record.Goal, record.ID, round.Number, round.Outcome, verdict)}
	if launch.UnitReviewReadyOutcomes[round.Outcome] {
		judged.next, judged.nextReason = inv.workArgv(record, "review"), "the checks passed: review records this result and completes its unit read"
	} else {
		judged.next, judged.nextReason = inv.workArgv(record, "revise", "--after", fmt.Sprint(round.Number), "--brief", "FILE"), "the attempt did not pass its checks; a correction brief starts one new attempt"
	}
	return judged
}

// unitRunnerAccount is the plain part of a unit runner's refusal: its code
// and key=value head ("UNIT_ROUND_LIMIT unit=u goal=g ...:") are dropped,
// the words after them kept. A refusal that is only a code says so plainly.
func unitRunnerAccount(message string) string {
	head, rest, found := strings.Cut(message, ": ")
	code, _, _ := strings.Cut(head, " ")
	if !unitRefusalCode.MatchString(code) {
		return message
	}
	if found && strings.TrimSpace(rest) != "" {
		return strings.TrimSpace(rest)
	}
	return "the build runner refused this request (" + strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(code, "UNIT_"), "_", " ")) + ")"
}

// withoutWords is argv less every word in drop.
func withoutWords(argv, drop []string) []string {
	var kept []string
	for _, word := range argv {
		if !slices.Contains(drop, word) {
			kept = append(kept, word)
		}
	}
	return kept
}

// targetID is the id of the first target of kind, or fallback.
func targetID(targets []intentTarget, kind, fallback string) string {
	for _, target := range targets {
		if target.Kind == kind && target.ID != "" {
			return target.ID
		}
	}
	return fallback
}

// workArgv is a public command about the run's goal and named work.
func (inv *intentInvocation) workArgv(record launch.UnitRunRecord, verb string, extra ...string) []string {
	if record.Goal == "" || record.Unit == "" {
		if verb == "revise" {
			// revise run continues the run's newest round; it names no attempt.
			extra = []string{"--brief", "FILE"}
		}
		return inv.publicArgv(append(append(workVerbWords(verb), unitRunPrefix+record.ID), extra...)...)
	}
	words := append(workVerbWords(verb), record.Goal, "--work", record.Unit)
	return inv.publicArgv(append(words, extra...)...)
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
	data["stop"], data["reads"], data["unknownRetries"] = round.Stop, round.Reads, round.UnknownRetries
	data["round"], data["outcome"], data["steps"], data["directory"] = round.Number, round.Outcome, round.Steps, round.Directory
	path := filepath.Join(round.Directory, "plan.json")
	if _, err := os.Stat(path); err == nil {
		data["plan"] = path
	}
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

// revise run

// runIntentReviseRun sends the follow-up brief to the run through the unit
// runner's follow-up, which reuses the run's plan, proof and read and
// refuses a round past the run's approved limit.
func runIntentReviseRun(inv *intentInvocation, run string) int {
	if len(inv.input.args) != 1 || !inv.input.has("brief") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "revising a run needs the correction brief; nothing was done",
			next:    inv.publicArgv("work", "revise", unitRunPrefix+run, "--brief", "FILE"), nextReason: "FILE says what to correct"})
	}
	if problem := inv.buildEngineAdmission(); problem != nil {
		return inv.render(*problem)
	}
	brief := inv.callerPath(inv.input.text("brief"))
	runner := inv.unitRunner()
	if inv.input.has("reason") || inv.input.has("by") {
		if problem := inv.selectRoot(); problem != nil {
			return inv.render(*problem)
		}
		actor, _, problem := inv.actingAs("revise", run, actorHuman)
		if problem != nil {
			return inv.render(*problem)
		}
		person := ""
		for i, v := range actor {
			if v == "--by" && i+1 < len(actor) {
				person = actor[i+1]
			}
		}
		reason := inv.input.text("reason")
		if reason == "" {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "a person's revision needs its reason", next: inv.typedArgvWith("--reason", "TEXT"), nextReason: "give the reason for this revision"})
		}
		impact := "Impact: this admits one correction despite the recorded review.\nThe unit needs another read; its automatic allowance remains.\nCancel the admitted run to undo the request."
		current, err := runner.Status(run)
		if err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the revision could not be admitted", Details: []string{err.Error()}, next: inv.sameCommand(), nextReason: "retries this admission without changing its inputs"})
		}
		if err := inv.recordUnitStopOverride(current.Goal, "work-revise", reason, impact, person); err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the revision could not be admitted", Details: []string{err.Error()}, next: inv.sameCommand(), nextReason: "retries this admission without changing its inputs"})
		}
		bytes, err := os.ReadFile(brief)
		if err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the correction brief cannot be read", Details: []string{err.Error()}, next: inv.sameCommand(), nextReason: "uses the corrected brief path"})
		}
		result, err := runner.Revise(launch.UnitRevisionRequest{Run: run, Brief: bytes, Person: person, Reason: reason, Impact: impact})
		if result.Revision.Attempt > 0 && len(result.Revision.Findings) > 0 && len(current.Rounds) > 0 {
			stop := current.Rounds[len(current.Rounds)-1].Stop
			if stop != nil {
				actErr := channel.RecordUnitStopAct(inv.layout.InstallationRoot.Path(), channel.UnitStopAct{ID: result.Record.ID + ":" + fmt.Sprint(result.Revision.Attempt), Goal: current.Goal, Loop: stop.Loop, Subject: stop.Subject, Attempt: stop.Attempt, Findings: result.Revision.Findings, Kind: "work-revise", Reason: reason, At: inv.unitStopNow()})
				if actErr != nil {
					return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the revision was admitted, but its questions need reconciliation", Details: []string{actErr.Error()}, next: inv.sameCommand(), nextReason: "reconciles the same recorded admission"})
				}
			}
		}
		return inv.render(inv.unitOutcome(runner, result.UnitResult, err, []intentTarget{{Kind: "run", ID: unitRunPrefix + run}}, inv.publicArgv("work", "wait", unitRunPrefix+run)))
	}
	result, err := runner.Continue(launch.UnitRequest{Resume: run, FollowUp: brief})
	return inv.render(inv.unitOutcome(runner, result, err, []intentTarget{{Kind: "run", ID: unitRunPrefix + run}}, inv.publicArgv("work", "wait", unitRunPrefix+run)))
}

// wait

// runIntentWorkWait waits for the one thing its target names: a goal's
// running work or goal event, a job, a unit run, a diagnostic read, a
// durable wait to resume, or a path to appear or disappear.
func runIntentWorkWait(inv *intentInvocation) int {
	args := inv.input.args
	eventSelectors := []string{"since", "verb", "question", "chain"}
	waitModes := append([]string{"work", "for", "path", "until", "timeout"}, eventSelectors...)
	if inv.input.switched("list") {
		for _, other := range append([]string{"exit-code", "caller-pid", "run"}, waitModes...) {
			if inv.input.has(other) {
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--list takes only --session, not --%s; nothing was done", other),
					next: inv.typedArgvLess(other), nextReason: "lists the waits"})
			}
		}
		if len(args) > 0 {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--list lists every wait and takes no target; nothing was done",
				next: withoutWords(inv.sameCommand(), args), nextReason: "lists the waits"})
		}
		if problem := inv.selectRoot(); problem != nil {
			return inv.render(*problem)
		}
		waits := inv.recoverWaits()
		if waits.Outcome == intentUnchanged {
			// The headline says there are none; the line under it repeated it.
			waits.text = nil
		}
		return inv.render(waits)
	}
	if inv.input.has("session") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--session only goes with --list; nothing was done",
			next: inv.typedArgvLess("session"), nextReason: "waits without it"})
	}
	if inv.input.switched("exit-code") {
		for _, other := range waitModes {
			if inv.input.has(other) {
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--exit-code takes a job or run and --caller-pid, not --%s; nothing was done", other),
					next: inv.typedArgvLess(other), nextReason: "waits for the exit code"})
			}
		}
		return runIntentWaitExitCode(inv)
	}
	for _, only := range []string{"caller-pid", "run"} {
		if inv.input.has(only) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--%s only goes with --exit-code; nothing was done", only),
				next: inv.typedArgvWith("--exit-code"), nextReason: "waits for the exit code"})
		}
	}
	if inv.input.has("path") {
		if len(args) > 0 {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--path waits for a path and takes no other target; nothing was done",
				next: withoutWords(inv.sameCommand(), args), nextReason: "waits for the path"})
		}
		for _, other := range append([]string{"work", "for"}, eventSelectors...) {
			if inv.input.has(other) {
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--path takes --until and --timeout, not --%s; nothing was done", other),
					next: inv.typedArgvLess(other), nextReason: "waits for the path"})
			}
		}
		return runIntentWaitObserved(inv, "file", inv.input.text("path"))
	}
	if inv.input.has("until") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--until only goes with --path; nothing was done",
			next: inv.typedArgvLess("until"), nextReason: "waits without it"})
	}
	if len(args) != 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "work wait needs one thing to wait for; nothing was done",
			next:    inv.typedArgvFor("GOAL"), nextReason: "a goal's work; or a j2:JOB or run:RUN reference, or --path FILE (metasystem work wait --help)"})
	}
	ref, problem := inv.resolveWorkRef(args[0], inv.command.accepts)
	if problem != nil {
		return inv.render(*problem)
	}
	if ref.kind != refGoal {
		for _, other := range append([]string{"work", "for"}, eventSelectors...) {
			if inv.input.has(other) {
				return inv.render(intentResult{Outcome: intentRefused, code: 2,
					Summary: fmt.Sprintf("--%s is for waiting on a goal; waiting on %s takes only --timeout; nothing was done", other, ref.qualified()),
					next:    inv.typedArgvLess(other), nextReason: "waits on " + ref.qualified()})
			}
		}
	}
	switch ref.kind {
	case refWait:
		return runIntentWaitResume(inv, ref.id)
	case refRead:
		return runIntentReviewRef(inv, "wait", ref.id)
	case refRun:
		timeout, problem := inv.waitTimeout()
		if problem != nil {
			return inv.render(*problem)
		}
		return inv.render(inv.waitUnit(ref.id, timeout, []intentTarget{{Kind: "run", ID: ref.qualified()}}, inv.publicArgv("work", "wait", ref.qualified())))
	case refJ1, refJ2:
		return runIntentWaitTarget(inv, "job", ref.id, &ref.job)
	}
	if !inv.input.has("for") {
		for _, selector := range eventSelectors {
			if inv.input.has(selector) {
				return inv.render(intentResult{Outcome: intentRefused, code: 2,
					Summary: fmt.Sprintf("--%s picks a goal event, which needs --for; nothing was done", selector),
					next:    inv.typedArgvWith("--for", "landing"), nextReason: "or --for human-act"})
			}
		}
		return runIntentWaitWork(inv, ref.id)
	}
	if inv.input.has("work") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "--work waits on running work and --for on a goal event; give one; nothing was done",
			next:    inv.typedArgvLess("work"), nextReason: "waits on the goal event"})
	}
	return runIntentWaitTarget(inv, "goal", ref.id, nil)
}

// runIntentWaitTarget waits for a goal event or one resolved job through
// the durable wait owner; a launch is awaited through its own owner.
func runIntentWaitTarget(inv *intentInvocation, kind, id string, job *intentJob) int {
	timeout, problem := inv.waitTimeout()
	if problem != nil {
		return inv.render(*problem)
	}
	targets := []intentTarget{{Kind: kind, ID: id}}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	if job != nil {
		if job.kind == "launch" {
			// The durable wait owner observes dispatch jobs only.
			return inv.render(inv.waitLaunch(*job, timeout))
		}
		targets = []intentTarget{{Kind: "job", ID: jobReference(*job)}}
	}
	args := []string{"--root", inv.layout.InstallationRoot.Path(), "--" + kind, id}
	if kind == "goal" {
		selector := metarun.WaitSelector{Kind: "goal", TargetID: id, GoalID: id, Event: inv.input.text("for"), After: inv.input.text("since"),
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
					Summary: "the goal ledger has no revision yet to wait after; nothing was registered",
					next:    inv.typedArgvWith("--since", "TIP"), nextReason: "TIP is the goal-ledger revision to wait after"})
			}
			selector.After = projection.Tip
		}
		if err := metarun.ValidateWaitSelector(selector); err != nil {
			return inv.render(intentResult{Outcome: intentRefused, Targets: targets, code: metarun.ExitInvalidWait, Summary: err.Error() + "; nothing was registered",
				next: inv.typedArgvLess("verb", "question", "chain"), nextReason: "waits for the event without the narrower options"})
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
	code := inv.work().wait(args, func(result metarun.WaitResult, _ bool) { waited = &result }, inv.stdout, inv.stderr)
	if waited == nil {
		return inv.render(intentResult{Outcome: intentFailed, Targets: targets, code: max(code, 1),
			Summary: "the wait stopped before a result; its reason is printed above",
			next:    inv.sameCommand(), nextReason: "waits again once that is settled"})
	}
	outcome := intentResult{Targets: targets, code: waited.ExitCode, Data: waited,
		Summary: fmt.Sprintf("%s %s: %s", kind, targets[0].ID, strings.TrimSpace(strings.Join([]string{waited.SourceOutcome, waited.Reason}, " ")))}
	resume := inv.publicArgv("work", "wait", waitRefPrefix+waited.WaitID)
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

// runIntentWaitObserved waits on a proof attempt or a filesystem path
// through the wait owner's own observers.
func runIntentWaitObserved(inv *intentInvocation, kind, ref string) int {
	timeout, problem := inv.waitTimeout()
	if problem != nil {
		return inv.render(*problem)
	}
	args := []string{}
	targets := []intentTarget{{Kind: kind, ID: ref}}
	switch kind {
	case "file":
		until := inv.input.text("until")
		if until != "present" && until != "absent" {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--path needs --until present or --until absent; nothing was done",
				next: append(inv.typedArgvLess("until"), "--until", "present"), nextReason: "or --until absent"})
		}
		path := inv.callerPath(ref)
		targets[0].ID = path
		args = append(args, "--path", path, "--until", until)
	case "proof":
		targets[0].ID = proofRefPrefix + ref
		args = append(args, "--attempt", ref)
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	args = append([]string{"--root", inv.layout.InstallationRoot.Path()}, args...)
	if timeout > 0 {
		args = append(args, "--timeout", timeout.String())
	}
	var waited *metarun.WaitResult
	code := inv.work().wait(args, func(result metarun.WaitResult, _ bool) { waited = &result }, inv.stdout, inv.stderr)
	if waited == nil {
		return inv.render(intentResult{Outcome: intentFailed, Targets: targets, code: max(code, 1), Summary: "the wait stopped before a result; its reason is printed above",
			next: inv.sameCommand(), nextReason: "waits again once that is settled"})
	}
	outcome := intentResult{Targets: targets, code: waited.ExitCode, Data: waited,
		Summary: fmt.Sprintf("%s %s: %s", kind, targets[0].ID, strings.TrimSpace(strings.Join([]string{waited.SourceOutcome, waited.Reason}, " ")))}
	switch waited.ExitCode {
	case metarun.ExitGreen, metarun.ExitRed, metarun.ExitEndedUnknown, metarun.ExitLaunchFailed:
		outcome.Outcome = intentConfirmed
	case metarun.ExitWaitDeadline, metarun.ExitInterrupted:
		outcome.Outcome = intentInProgress
		if waited.WaitID != "" {
			outcome.next, outcome.nextReason = inv.publicArgv("work", "wait", waitRefPrefix+waited.WaitID), "this continues the same wait"
		}
	case metarun.ExitNoRecord, metarun.ExitInvalidWait, metarun.ExitWaiterBusy:
		outcome.Outcome = intentRefused
	default:
		outcome.Outcome = intentFailed
	}
	return inv.render(outcome)
}

// runIntentWaitResume continues one recorded wait through the wait owner's
// own records-only resume.
func runIntentWaitResume(inv *intentInvocation, id string) int {
	timeout, problem := inv.waitTimeout()
	if problem != nil {
		return inv.render(*problem)
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	targets := []intentTarget{{Kind: "wait", ID: id}}
	if row, _, err := metarun.FindWaiterByID(inv.stateRoot, id); err == nil && row.Selector.Poll == "channel" {
		return inv.render(inv.resumeChannelWait(id, row, timeout, targets))
	}
	args := []string{"--root", inv.layout.InstallationRoot.Path(), "--resume", id}
	if timeout > 0 {
		args = append(args, "--timeout", timeout.String())
	}
	var waited *metarun.WaitResult
	code := inv.work().wait(args, func(result metarun.WaitResult, _ bool) { waited = &result }, inv.stdout, inv.stderr)
	if waited == nil {
		return inv.render(intentResult{Outcome: intentFailed, Targets: targets, code: max(code, 1), Summary: "the wait stopped before a result; its reason is printed above",
			next: inv.sameCommand(), nextReason: "waits again once that is settled"})
	}
	outcome := intentResult{Targets: targets, code: waited.ExitCode, Data: waited,
		Summary: fmt.Sprintf("wait %s: %s", id, strings.TrimSpace(strings.Join([]string{waited.SourceOutcome, waited.Reason}, " ")))}
	switch waited.ExitCode {
	case metarun.ExitGreen, metarun.ExitRed, metarun.ExitEndedUnknown, metarun.ExitLaunchFailed:
		outcome.Outcome = intentConfirmed
	case metarun.ExitWaitDeadline, metarun.ExitInterrupted:
		outcome.Outcome, outcome.next, outcome.nextReason = intentInProgress, inv.publicArgv("work", "wait", waitRefPrefix+id), "the work continues; this continues the same wait"
	case metarun.ExitNoRecord, metarun.ExitInvalidWait, metarun.ExitWaiterBusy:
		outcome.Outcome = intentRefused
	default:
		outcome.Outcome = intentFailed
	}
	return inv.render(outcome)
}

// waitTimeout is the invocation's --timeout, at most a day.
func (inv *intentInvocation) waitTimeout() (time.Duration, *intentResult) {
	if !inv.input.has("timeout") {
		return 0, nil
	}
	value, err := time.ParseDuration(inv.input.text("timeout"))
	if err != nil || value <= 0 || value > 24*time.Hour {
		return 0, &intentResult{Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("--timeout must be a positive duration of at most 24h, such as 10m, not %s; nothing was done", shellCommand([]string{inv.input.text("timeout")})),
			next:    append(inv.typedArgvLess("timeout"), "--timeout", "10m"), nextReason: "or another duration up to 24h"}
	}
	return value, nil
}

// waitUnit continues one unit run's recorded steps for at most timeout; it
// starts no new build.
func (inv *intentInvocation) waitUnit(run string, timeout time.Duration, targets []intentTarget, again []string) intentResult {
	runner := inv.unitRunner()
	if timeout > 0 && runner.Manager != nil {
		settings := runner.Manager.Settings
		if len(settings.Values) == 0 {
			settings = launch.DefaultSettings()
		}
		settings.WaitCapSeconds = int64(timeout / time.Second)
		runner.Manager.Settings = settings
	}
	result, err := runner.Continue(launch.UnitRequest{Resume: run})
	return inv.unitOutcome(runner, result, err, targets, again)
}

// test

// testVerboseFlag is test run, plan and status's --verbose: a refusal's two
// lines, then the code, cause and facts behind it.
var testVerboseFlag = intentFlag{name: "verbose", usage: "also print the details behind a refusal: its code, cause and facts"}

// runIntentTest runs the selected installation's test runner in this process
// and reports the structured result it prints; its progress goes to standard
// error unchanged.
func runIntentTest(inv *intentInvocation) int {
	if problem := inv.resolveLayout(); problem != nil {
		return inv.render(*problem)
	}
	argv := []string{"internal", "test", "run", "--json", "--root", inv.layout.InstallationRoot.Path()}
	targets := []intentTarget{}
	for _, name := range []string{"goal", "authority", "mode"} {
		if inv.input.has(name) {
			argv = append(argv, "--"+name, inv.input.text(name))
		}
	}
	if inv.input.switched("verbose") {
		argv = append(argv, "--verbose")
	}
	if inv.input.has("goal") {
		targets = append(targets, intentTarget{Kind: "goal", ID: inv.input.text("goal")})
	}
	output, code, err := inv.work().testRun(inv.layout.GitRoot, argv, inv.stderr)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, Targets: targets, code: 1, Summary: "the tests couldn't be started",
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}.withCause(err))
	}
	// The runner answers with its --json envelope; one that cannot be read
	// is a failure, never a pass.
	child, readErr := verbresult.Read(output, "internal test run", code, "")
	if readErr != nil {
		return inv.render(intentResult{Outcome: intentFailed, Targets: targets, code: 1, Summary: "the test run's result could not be read",
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{readErr.Error()}})
	}
	result := intentResult{Targets: targets, code: code}
	attempt := ""
	if len(child.Data) > 0 {
		result.Data = child.Data
		var reported struct {
			AttemptID string `json:"attemptId"`
		}
		if json.Unmarshal(child.Data, &reported) == nil && validIntentJobID(reported.AttemptID) {
			attempt = reported.AttemptID
			result.Targets = append(result.Targets, intentTarget{Kind: "proof", ID: attempt})
			result.text = append(result.text, "test run "+attempt+": "+shellCommand(inv.publicArgv("test", "wait", proofRefPrefix+attempt))+" reads how it ended")
		}
	}
	if child.Code != "" {
		result.Details = append(result.Details, child.Details...)
	}
	if attempt != "" && (code == metarun.ExitWaitDeadline || code == metarun.ExitInterrupted) {
		result.Outcome, result.Summary = intentInProgress, fmt.Sprintf("test run %s has not ended", attempt)
		result.next, result.nextReason = inv.publicArgv("test", "wait", proofRefPrefix+attempt), "waits for it to end"
		return inv.render(result)
	}
	switch code {
	case 0:
		result.Outcome, result.Summary = intentConfirmed, "the selected tests passed"
	case proofrun.ExitAdmissionRefused:
		result.Outcome, result.Summary = intentRefused, "the tests were not started now; the reason is printed above"
		result.next, result.nextReason = inv.sameCommand(), "try again once that is settled"
	case 2:
		result.Outcome, result.Summary = intentRefused, "the test runner refused these options; the reason is printed above"
		result.Decision = "correct the options; metasystem help test run lists the ones it takes"
	default:
		result.Outcome, result.Summary = intentFailed, fmt.Sprintf("the test runner exited %d; the reason is printed above", code)
		result.next, result.nextReason = inv.sameCommand(), "try again once that is settled"
	}
	return inv.render(result)
}

// settings

// runIntentSettings reads the selected installation's metasystem.conf
// through the launch settings reader and, for any other key, the
// configuration reader, each with the source of its value.
// runIntentSettingsKeys lists every configured key with its source.
func runIntentSettingsKeys(inv *intentInvocation) int {
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	conf, matching := inv.layout.InstallationRoot.Path("metasystem.conf"), inv.input.text("matching")
	ran := ownerCall(func(stdout, _ io.Writer) int { return inv.ownerCalls().configKeys(stdout, conf, matching) })
	keys := ownerVerbResult(ran, nil, "the configured keys of "+inv.layout.InstallationRoot.Path(), map[string]any{"installation": inv.layout.InstallationRoot.Path()})
	if keys.Outcome == intentConfirmed {
		shown := "the configured keys of " + inv.statusSeatName(inv.layout.GitRoot)
		keys.headline = &shown
	}
	return inv.render(keys)
}

// runIntentSettingsCheck validates the selected installation's settings.
func runIntentSettingsCheck(inv *intentInvocation) int {
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	root := inv.layout.InstallationRoot
	ran := ownerCall(func(stdout, stderr io.Writer) int {
		return inv.ownerCalls().configValidate(stdout, stderr, root.Path("metasystem.conf"), root.Path())
	})
	settings := ownerVerbResult(ran, nil, "the settings of "+root.Path()+" are valid", map[string]any{"installation": root.Path()})
	if settings.Outcome != intentConfirmed {
		return inv.render(settings)
	}
	// The text names the checkout as a person does; --json keeps the path.
	seat := inv.statusSeatName(inv.layout.GitRoot)
	render := func(result intentResult) int {
		shown := strings.ReplaceAll(result.Summary, root.Path(), seat)
		result.headline = &shown
		return inv.render(result)
	}
	// The testing contract the settings name is validated with its declared
	// tools; no test runs and no native discovery takes the host's lease.
	contractReady := inv.owners.contractReady
	if contractReady == nil {
		contractReady = testrun.ContractReady
	}
	path, groups, err := contractReady(root.Path(), false)
	if err != nil {
		return render(intentResult{Outcome: intentRefused, code: 1, Data: map[string]any{"installation": root.Path()},
			Summary: "the settings of " + root.Path() + " are valid, but the testing contract is not: " + err.Error(),
			next:    inv.sameCommand(), nextReason: "after correcting the testing contract"})
	}
	settings.Summary = "the settings of " + root.Path() + " and their testing contract are valid"
	settings.text = append(settings.text, fmt.Sprintf("testing contract %s: %s", inv.shownPath(path), textui.Count(groups, "group", "groups")))
	// The launch contract is validated here too, with the same kind of line:
	// a project that has one gets its faults named before a start, and a
	// project that has none is not a project with a problem.
	launchPath, launchContract, launchTools, launchErr := launchContractReady(root.Path())
	switch {
	case errors.Is(launchErr, errNoLaunchContract):
		settings.text = append(settings.text, "launch contract: none declared")
	case launchErr != nil:
		return render(intentResult{Outcome: intentRefused, code: 1, Data: map[string]any{"installation": root.Path()},
			Summary: "the settings and testing contract are valid, but the launch contract is not: " + launchErr.Error(),
			next:    inv.sameCommand(), nextReason: "after correcting the launch contract"})
	default:
		settings.Summary = "the settings of " + root.Path() + ", their testing contract and their launch contract are valid"
		settings.text = append(settings.text, fmt.Sprintf("launch contract %s: %s, readiness %s, %s",
			inv.shownPath(launchPath), launchContractName(launchContract), launchContract.ReadyKind(), launchContract.DataWord()))
		for _, tool := range launchTools {
			settings.text = append(settings.text, "launch tool "+tool.Line())
		}
	}
	return render(settings)
}

func runIntentSettings(inv *intentInvocation) int {
	if problem := inv.resolveLayout(); problem != nil {
		return inv.render(*problem)
	}
	if len(inv.input.args) == 1 && config.PolicyScope(inv.input.args[0]) != "" {
		return inv.runPolicyShow(inv.input.args[0])
	}
	confPath := intentConfPath(inv.layout)
	settings, err := inv.work().settings(confPath)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the launch settings in " + confPath + " can't be read",
			next: inv.publicArgv("settings", "check"), nextReason: "names what is wrong", Details: []string{err.Error()}})
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
				shown := fmt.Sprintf("there is no setting %s in %s", shellCommand([]string{key}), inv.shownPath(confPath))
				if err != nil {
					reason += ": " + err.Error()
				}
				return inv.render(intentResult{Outcome: intentRefused, code: 1, Summary: reason, headline: &shown,
					next: inv.publicArgv("settings", "show"), nextReason: "list the launch settings"})
			}
			values = []launch.Setting{{Key: key, Value: value, Source: source}}
		}
	}
	text := make([]string, 0, len(values))
	for _, value := range values {
		text = append(text, fmt.Sprintf("%s=%s (%s)", value.Key, value.Value, value.Source))
	}
	data := map[string]any{"conf": confPath, "settings": values}
	if len(inv.input.args) == 0 {
		// Every external adapter and override, and every refused one
		// (design verbs-object-action 3.5).
		adapters, _ := adapterReport(inv.layout.InstallationRoot.Path())
		text = append(text, adapters...)
		data["adapters"] = adapters
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Data: data, text: text,
		Summary: fmt.Sprintf("%d setting(s) of %s", len(values), inv.layout.InstallationRoot),
		view:    settingsView(inv.statusSeatName(inv.layout.GitRoot), len(inv.input.args) == 1, values, data["adapters"])})
}

// settingsView is settings show's page (output-style §6.14): a table of
// key, value and where the value comes from, sorted by key; a value is
// shown as stored, since it is pasted back into settings set. One named
// setting is one line: its value and its source. --verbose adds each
// source's whole account.
func settingsView(seat string, one bool, values []launch.Setting, adapters any) func(*textui.Page) {
	sorted := slices.Clone(values)
	slices.SortFunc(sorted, func(a, b launch.Setting) int { return strings.Compare(a.Key, b.Key) })
	return func(page *textui.Page) {
		defaults := 0
		for _, value := range sorted {
			if settingSource(value.Source) == "default" {
				defaults++
			}
		}
		facts := []string{}
		if set := len(sorted) - defaults; set > 0 {
			facts = append(facts, textui.Number(int64(set))+" configured")
		}
		if defaults > 0 {
			facts = append(facts, textui.Number(int64(defaults))+" "+map[bool]string{true: "default", false: "defaults"}[defaults == 1])
		}
		if one && len(sorted) == 1 {
			source := settingSource(sorted[0].Source)
			if page.Verbose() || config.CommittedOnly(sorted[0].Key) {
				source = sorted[0].Source
			}
			page.Headline(sorted[0].Key+" is "+sorted[0].Value, source)
			return
		}
		page.Headline(textui.Count(len(sorted), "setting", "settings")+" of "+seat, facts...)
		table := page.Section("", "").Table(textui.Column{}, textui.Column{}, textui.Column{Flex: true})
		for _, value := range sorted {
			source := textui.Dim(settingSource(value.Source))
			if page.Verbose() {
				source = textui.Dim(value.Source)
			}
			table.Row(textui.Plain(value.Key), textui.Plain(value.Value), source)
		}
		if lines, ok := adapters.([]string); ok && len(lines) > 0 {
			section := page.Section("Adapters", "")
			for _, line := range lines {
				section.Text(line)
			}
		}
	}
}

// settingSource is a setting's source in one word: default, local (the
// seat's metasystem.conf.local) or conf.
func settingSource(source string) string {
	word, _, _ := strings.Cut(source, " ")
	return strings.TrimSuffix(word, ";")
}

// shownPath is a path as a person reads it: repo-relative inside the
// checkout, ~/… under the home directory.
func (inv *intentInvocation) shownPath(path string) string { return inv.textEnv(inv.stdout).Path(path) }

// runIntentWaitExitCode blocks until one delegate job or tracked run is
// terminal and exits with its pinned code, through the job and run waiters.
// Their records are read under --repo, or the directory the command runs in.
func runIntentWaitExitCode(inv *intentInvocation) int {
	root := inv.cwd
	if inv.input.has("repo") {
		root = inv.textPath(inv.input.text("repo"))
	}
	if inv.input.has("run") {
		if len(inv.input.args) != 0 || inv.input.has("caller-pid") {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--run with --exit-code waits for one tracked run and takes nothing else; nothing was done",
				next: withoutWords(inv.typedArgvLess("caller-pid"), inv.input.args), nextReason: "waits for the run's exit code"})
		}
		return inv.work().runWatch([]string{"--root", root, "--id", inv.input.text("run")}, inv.stdout, inv.stderr)
	}
	if len(inv.input.args) != 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--exit-code needs one job or a tracked run; nothing was done",
			next: inv.typedArgvFor("j2:JOB"), nextReason: "or --run RUN for a tracked run"})
	}
	job := inv.input.args[0]
	if kind, id := splitReference(job); kind == refJ2 {
		job = id
	} else if kind != "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--exit-code waits for a dispatch job or a tracked run, not %s; nothing was done", job),
			next: inv.publicArgv("work", "status", "--all"), nextReason: "lists the dispatch jobs with their j2: references"})
	}
	args := []string{"--root", root, "--job", job}
	if inv.input.has("caller-pid") {
		args = append(args, "--caller-pid", inv.input.text("caller-pid"))
	}
	return inv.work().jobWatch(args, inv.stdout, inv.stderr)
}

// brief

func runIntentBrief(inv *intentInvocation) int {
	id, problem := inv.singleTarget()
	if problem != nil {
		return inv.render(*problem)
	}
	if id == "" || !inv.input.has("out") {
		retry := inv.sameCommand()
		if id == "" {
			retry = inv.typedArgvFor("GOAL")
		}
		if !inv.input.has("out") {
			retry = append(retry, "--out", "FILE")
		}
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "a brief needs the goal and the file to write it to; nothing was written",
			next: retry, nextReason: "FILE is where the brief is written"})
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
		return inv.render(intentResult{Outcome: intentRefused, Targets: targets, code: 1, Summary: fmt.Sprintf("goal %s is %s; no brief was written", id, where),
			Decision: "nothing to do; only an open goal gets a brief"})
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
			Summary: fmt.Sprintf("%s already exists with other content; nothing was written", out),
			next:    append(inv.typedArgvLess("out"), "--out", strings.TrimSuffix(out, filepath.Ext(out))+"-new"+filepath.Ext(out)), nextReason: "writes a new file; the existing one is kept"})
	}
	if _, err := atomicfile.WriteText(out, text, filepath.Dir(out)); err != nil {
		return inv.render(intentResult{Outcome: intentFailed, Targets: targets, code: 1, Summary: "the brief can't be written to " + out,
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}})
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
			return "", nil, &intentResult{Outcome: intentFailed, code: 1, Summary: fileProblem("accepted design", design, err) + "; no brief was written",
				next: inv.sameCommand(), nextReason: "once the design is readable"}
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
			mark(fmt.Sprintf("the goal is %s without an approved box; a person approves it with metasystem goal approve %s", file.State, file.Id)))
	}
	fmt.Fprintf(&text, "\n# Goal\n\n%s\n\nNext step on the ledger: %s\n", file.Intent, file.NextStep)
	text.WriteString("\n# Workspace\n\n")
	worktree, problem := inv.goalWorktree(file.Id)
	switch {
	case problem == nil:
		fmt.Fprintf(&text, "Branch goal/%s, checked out in %s. Leave the change there, uncommitted.\n", file.Id, worktree)
	case problem.Outcome == intentRefused:
		// Ordinary before the first build: build prepares the workspace.
		fmt.Fprintf(&text, "Branch goal/%s. Its workspace does not exist yet; metasystem work build prepares it. Leave the change there, uncommitted.\n", file.Id)
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
	if allowance, err := config.IntentReviewToolCalls(intentConfPath(inv.layout)); err == nil {
		text.WriteString(fmt.Sprintf("\nThe independent read's tool-call budget (the configured allowance; change it here if this work needs another):\nMaximum reader tool calls: %d\n", allowance))
	} else {
		text.WriteString("\nThe independent read's tool-call budget:\n" + mark("the read's tool-call budget, written as the line 'Maximum reader tool calls: N'") + "\n")
	}
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

// waitLaunch waits for one launch through the launch manager, which proves a
// lost supervisor dead before calling the launch ended; a wait that ends
// first continues with the same launch reference.
func (inv *intentInvocation) waitLaunch(job intentJob, timeout time.Duration) intentResult {
	ref := jobReference(job)
	targets := []intentTarget{{Kind: "job", ID: ref}}
	record, ended, err := inv.owners.processes.launches().Wait(job.id, timeout)
	if err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: fmt.Sprintf("the wait for launch %s stopped: %v", job.id, err),
			next: inv.sameCommand(), nextReason: "waits again"}
	}
	data := map[string]any{"kind": "launch", "record": record}
	if !ended {
		again := inv.publicArgv("work", "wait", ref)
		if timeout > 0 {
			again = append(again, "--timeout", timeout.String())
		}
		return intentResult{Outcome: intentInProgress, code: metarun.ExitWaitDeadline, Targets: targets, Data: data, text: []string{launchReport(record)},
			Summary: fmt.Sprintf("launch %s is still %s", job.id, record.State), next: again, nextReason: "the same wait continues this launch"}
	}
	return intentResult{Outcome: intentConfirmed, Targets: targets, Data: data, text: []string{launchReport(record)},
		Summary: fmt.Sprintf("launch %s ended: %s", job.id, record.State)}
}

// adapterReport is one line per external adapter, override and refused
// adapter executable of an installation, and the number refused.
func adapterReport(root string) ([]string, int) {
	reg, err := external.Load(root)
	if err != nil {
		return []string{"adapter registry unreadable: " + err.Error()}, 1
	}
	var out strings.Builder
	reg.Report(&out)
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, len(reg.Refusals)
}

// launchContractName is what a contract calls its application, or the words
// for one that named itself nothing.
func launchContractName(contract applaunch.Contract) string {
	if contract.Name == "" {
		return "one unnamed application"
	}
	return contract.Name
}

// runIntentSettingsSet writes one key into the installation's local
// configuration through the configuration owner (validate.SetConfKeys, the
// one the one adoption tailoring uses), creating the local
// file when the seat has none; the shipped metasystem.conf is never touched.
func runIntentSettingsSet(inv *intentInvocation) int {
	if len(inv.input.args) != 2 || strings.TrimSpace(inv.input.args[0]) == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "settings set needs one key and its value; nothing was done",
			next:    inv.publicArgv("settings", "set", "KEY", "VALUE"), nextReason: "metasystem settings keys lists the keys"})
	}
	key, value := strings.TrimSpace(inv.input.args[0]), inv.input.args[1]
	if config.CommittedOnly(key) {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: []intentTarget{{Kind: "setting", ID: key}},
			Summary:  key + " is committed-only, so settings set cannot change it",
			Decision: settingsDeclarationRemedy(key)})
	}
	if strings.ContainsAny(key, "= \t\n") || strings.ContainsAny(value, "\n\r") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: "a setting's key has no spaces or '=' and its value is one line; nothing was done",
			next:    inv.publicArgv("settings", "set", "KEY", "VALUE"), nextReason: "a key without spaces or '=', and a one-line value"})
	}
	if problem := config.SettingKeyProblem(key); problem != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: problem.Error(),
			next: inv.publicArgv("settings", "keys"), nextReason: "lists the settings"})
	}
	if problem := config.SettingValueProblem(key, value); problem != nil {
		retryValue := "VALUE"
		if config.PolicyScope(key) != "" {
			retryValue = "auto"
		}
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: []intentTarget{{Kind: "setting", ID: key}},
			Summary: problem.Error() + "; nothing was done",
			next:    inv.publicArgv("settings", "set", key, retryValue), nextReason: "a person at an enrolled terminal sets a value from the grammar named"})
	}
	if problem := inv.resolveLayout(); problem != nil {
		return inv.render(*problem)
	}
	if config.PolicyScope(key) != "" && authoritySettings[key] {
		return inv.runPolicySet(key, value)
	}
	if authoritySettings[key] {
		if problem := inv.directPersonProof("settings set " + key); problem != nil {
			return inv.render(*problem)
		}
	}
	local := intentConfPath(inv.layout) + ".local"
	targets := []intentTarget{{Kind: "setting", ID: key}}
	before, err := os.ReadFile(local)
	existed := err == nil
	if err != nil && !os.IsNotExist(err) {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: local + " can't be read, so nothing was set",
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}})
	}
	if !existed {
		if err := os.WriteFile(local, nil, 0o600); err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: local + " can't be created, so nothing was set",
				next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}})
		}
	}
	if err := validate.SetConfKeys(local, []validate.ConfSetting{{Key: key, Value: value}}); err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: local + " can't be written, so nothing was set",
			next: inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}})
	}
	data := map[string]any{"key": key, "value": value, "file": local}
	if after, err := os.ReadFile(local); existed && err == nil && bytes.Equal(before, after) {
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: data,
			Summary: key + " already holds that value in " + local + "; nothing was changed",
			view:    settingSetView(key + " already holds " + value + " in " + inv.shownPath(local) + "; nothing was changed")})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: data,
		Summary: key + "=" + value + " is set in " + local,
		view:    settingSetView(key + " is set to " + value + " in " + inv.shownPath(local))})
}

// settingSetView is settings set's page: what the key holds now, and where.
func settingSetView(done string) func(*textui.Page) {
	return func(page *textui.Page) { page.Done(done) }
}

// runIntentDeclareStopMoves writes the Stop decision move declaration for
// one goal through the stop-surface owner in this process. The same moves
// and reason again change nothing (R-129).
func runIntentDeclareStopMoves(inv *intentInvocation) int {
	if len(inv.input.args) != 1 {
		retry := inv.typedArgvFor("GOAL")
		if !inv.input.has("reason") {
			retry = append(retry, "--reason", "TEXT")
		}
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "declare-moves needs the one goal whose change moves Stop decisions; nothing was done",
			next: retry, nextReason: "names the goal"})
	}
	id, reason := inv.input.args[0], inv.input.text("reason")
	if strings.TrimSpace(reason) == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
			Summary: "declaring the moves needs the reason the change moves a Stop decision; nothing was done",
			next:    inv.typedArgvWith("--reason", "TEXT"), nextReason: "TEXT says why"})
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	declare := inv.owners.stopMovesDeclare
	if declare == nil {
		declare = audit.DeclareStopDecisionSurface
	}
	root := inv.layout.InstallationRoot
	before := stopMovesSnapshot(root.Path())
	path, err := declare(root.Path(), audit.StopSurfaceOptions{Base: inv.input.text("base"), GoalRecord: goal.StopSurfaceGoalReader}, id, reason)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(id), Summary: err.Error() + "; nothing was declared",
			next: inv.publicArgv("goal", "allow", id, "stop-test-changes", "--reason", "TEXT"), nextReason: "a person allows goal " + id + " to move Stop assertions, then this declares the moves"})
	}
	result := intentResult{Outcome: intentConfirmed, Targets: inv.targets(id), Summary: "declared the Stop decision moves in " + path + "; commit it with the change",
		Data: map[string]any{"declaration": path}}
	if before == stopMovesSnapshot(root.Path()) {
		result.Outcome, result.Summary = intentUnchanged, "the declaration "+path+" already records these moves; nothing changed"
	}
	return inv.render(result)
}

// stopMovesSnapshot is the declarations directory's names and bytes, so a
// repeat that rewrote nothing is seen as such.
func stopMovesSnapshot(root string) string {
	directory := filepath.Join(root, "docs", "stop-decision-moves")
	entries, err := os.ReadDir(directory)
	if err != nil {
		return ""
	}
	var snapshot strings.Builder
	for _, entry := range entries {
		data, _ := os.ReadFile(filepath.Join(directory, entry.Name()))
		snapshot.WriteString(entry.Name() + "\x00" + string(data) + "\x00")
	}
	return snapshot.String()
}

// resumeChannelWait continues a durable wait on a channel answer through the
// channel wait owner in this process, which polls the provider as it waits;
// this process is the waiting caller its registration names.
func (inv *intentInvocation) resumeChannelWait(id string, row metarun.Waiter, timeout time.Duration, targets []intentTarget) intentResult {
	args := []string{"--root", inv.stateRoot, "--resume", id}
	if timeout > 0 {
		args = append(args, "--timeout", fmt.Sprint(max(int(timeout.Minutes()), 1)))
	}
	caller, lineage := ownercall.CurrentProcess(), ""
	if inv.owners.dependencies.ownerLineage != nil {
		lineage = inv.owners.dependencies.ownerLineage()
	}
	ran := ownerCall(func(stdout, stderr io.Writer) int {
		return inv.ownerCalls().channelWait(caller, lineage, stdout, stderr, args)
	})
	result := ownerVerbResult(ran, targets, "the channel question "+row.Selector.Question+" is answered", nil)
	if result.Outcome != intentConfirmed {
		result.Outcome = intentInProgress
		result.next, result.nextReason = inv.publicArgv("work", "wait", waitRefPrefix+id), "the same wait continues; delivery or the answer is still pending"
	}
	return result
}

// authoritySettings decide caller classification or person-controlled
// decisions. metasystem.runtimes=fake enables fixture caller classification
// and an environment-supplied clock. Only
// the person's own proof at the enrolled terminal sets them, never the helm
// and never a power of attorney.
var authoritySettings = map[string]bool{"metasystem.runtimes": true, "landing.batch": true, "landing.proof": true, "landing.on-red": true, "landing.trunk-red": true, "seat.driver": true, "review.stop": true, "goal.raise": true, "question.route": true}

// directPersonProof refuses unless this shell is the person at the enrolled
// terminal, proven by the walk itself.
func (inv *intentInvocation) directPersonProof(act string) *intentResult {
	// refused says, in plain words, that only the person at the enrolled
	// terminal sets this, why this shell is not that, and the one command
	// that resolves it (the enrollment, the name filled in, or the same
	// command in a terminal the person opened).
	refused := func(reason string, err error) *intentResult {
		remedy := humanauthority.RemedyFor(inv.stateRoot, err, inv.personName(""), inv.typedArgv())
		if err != nil && remedy.Reason != "" {
			reason = remedy.Reason
		}
		result := &intentResult{Outcome: intentRefused, code: 1,
			Summary: fmt.Sprintf("only you set %s, at your enrolled terminal, and %s; nothing was done", act, reason),
			next:    remedy.Argv, nextReason: remedy.Then}
		if len(result.next) == 0 {
			result.next, result.nextReason = inv.typedArgv(), "in the terminal you enrolled, never under the helm or a grant"
		}
		if err != nil {
			result.Details = []string{"refused because: " + refusalCause(err)}
		}
		return result
	}
	if inv.stateRoot == "" {
		root, err := inv.owners.resolver.RootForInstallation(inv.layout.InstallationRoot)
		if err != nil {
			return refused("this installation's state can't be found", err)
		}
		inv.stateRoot = root.Path()
	}
	if inv.owners.prove == nil || inv.owners.commandNow == nil {
		return refused("who is at this terminal can't be checked here", nil)
	}
	now, err := inv.owners.commandNow(inv.stateRoot)
	if err != nil {
		return refused("the clock can't be read", err)
	}
	proof, err := inv.owners.prove(inv.stateRoot, int64(os.Getppid()), nil, "", "", now)
	if err != nil {
		return refused(humanauthority.PlainReason(err), err)
	}
	if proof.Helm != nil || !proof.EnrolledTerminalFor(inv.stateRoot) {
		_ = humanauthority.RecordAttorneyRefusal(inv.stateRoot, proof, act, "set only by the person's own proof", now)
		return refused("this shell acts under the helm or a grant", nil)
	}
	return nil
}
