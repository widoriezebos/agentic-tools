// Command metasystem is the metasystem's one binary. Operator verbs such as
// up and health route directly; internal families group the narrower
// decisions that plumbing invokes. Compatibility wrappers keep historical
// names only long enough to exec into these verbs. File naming is one file
// per routed surface; cross-family helpers live in helpers.go and nowhere
// else.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// A verb takes its own arguments (after the family and verb words)
// and returns a process exit code. Verbs print their own output and
// errors; main only routes.
type verb struct {
	name    string
	summary string
	run     func(args []string) int
}

type family struct {
	name    string
	summary string
	verbs   []verb
}

func families() []family {
	return []family{
		{
			name:    "app",
			summary: "the application this project builds, under its launch contract",
			verbs: []verb{
				{"serve", "own one run of the application for its life (internal)", runAppServe},
			},
		},
		{
			name:    "ui",
			summary: "the checkout's browser interface",
			verbs: []verb{
				{"serve", "serve the interface in the foreground (internal)", runUIServe},
				{"tools", "serve the interface's read tools to the Project Partner over stdio (internal)", runUITools},
			},
		},
		{
			name:    "testing",
			summary: "semantic maintenance of the testing contract",
			verbs: []verb{
				{"merge-driver", "run the testing contract Git merge driver, or print its setup", runTestingMergeDriver},
			},
		},
		{
			name:    "test",
			summary: "risk-selected common application testing with retained proof and reuse",
			verbs: []verb{
				{"plan", "compute the candidate's risk-selected groups without running tests", runTestPlan},
				{"run", "admit and execute the recomputed selected test plan", runTestRun},
				{"verify", "verify sufficient retained proof without launching tests or builds", runTestVerify},
				{"worker-capabilities", "report the installed testing worker protocol (internal)", runTestWorkerCapabilities},
				{"worker", "execute one admitted selected plan (internal)", runTestWorker},
			},
		},
		{
			name:    "brain",
			summary: "the fleet brain seat: designation, boot context, and checkout-local fences",
			verbs: []verb{
				{"boot", "compose the declared brain's bounded standing context", runBrainBoot},
				{"boot-inputs", "read optional brain boot inputs in the bounded child (internal)", runBrainBootInputs},
			},
		},
		{
			name:    "proof-run",
			summary: "priced validation runs with structural progress and a sibling watchdog",
			verbs: []verb{
				{"banner", "print the suite witness state, duration class, heartbeat, and log paths", runProofRunBanner},
				{"launch", "launch a suite in its own process group with a sibling watchdog", runProofRunLaunch},
				{"worker-authorized", "authenticate a suite worker against its live parent proof", runProofRunWorkerAuthorized},
				{"watchdog", "watch suite output growth and enforce the section ceiling (internal)", runProofRunWatchdog},
				{"custody-exec", "hold a resource-active command until its custodian binds exact identity (internal)", runProofRunCustodyExec},
				{"preserve", "copy bounded watchdog evidence (internal)", runProofRunPreserve},
			},
		},
		{
			name:    "config",
			summary: "configuration and identity helpers",
			verbs: []verb{
				{"get", "resolve a config key with flag/env/local/mode/conf/default precedence", runConfigGet},
				{"validate", "validate the whole metasystem.conf domain", runConfigValidate},
			},
		},
		{
			name:    "validate",
			summary: "whole-artifact validators the assert scripts exec into",
			verbs: []verb{
				{"session-isolation", "copy adapter local config into a second-session worktree and audit isolation", runValidateSessionIsolation},
			},
		},
		{
			name:    "landing",
			summary: "classify and record the two bars for a prospective landing",
			verbs: []verb{
				{"batch", "join, status, withdraw, owner, tick, or wait for a guarded landing batch", runLandingBatch},
				{"observe", "emit a provenance verdict for the prospective project tree", runLandingObserve},
				{"workspace", "print the delivery workspace projection of a whole-project tree", runLandingWorkspace},
				{"test-receipt", "run tests against one exact candidate tree and record their result", runLandingTestReceipt},
			},
		},
		{
			name:    "adapter",
			summary: "shared runtime-adapter plumbing: permissions, patches, snapshots",
			verbs: []verb{
				{"claude-tool-gate", "decide one Claude tool call against the context budget", runAdapterClaudeToolGate},
				{"claude-session-signal", "record the Claude session-established signal", runAdapterClaudeSessionSignal},
			},
		},
		{
			name:    "behavior-surface",
			summary: "versioned byte projections shared by witness, landing, adoption, and weight laws",
			verbs:   []verb{},
		},
		{
			name:    "launch",
			summary: "owned external agent processes with durable state and exact cancellation",
			verbs: []verb{
				{"supervise", "own one launch child through its terminal state (internal)", runLaunchSupervise},
			},
		},
		{
			name:    "hooks",
			summary: "the Stop hook's deadline worker: wait for it, or stop it past its deadline",
			verbs:   []verb{},
		},
		{
			name:    "util",
			summary: "small utilities for shell callers",
			verbs: []verb{
				{"hold", "stay alive carrying --tag until SIGTERM, then write the stopped file", runUtilHold},
			},
		},
		{
			name:    "json",
			summary: "JSON field access for shell callers",
			verbs:   []verb{},
		},
		{
			name:    "goal",
			summary: "the goal ledger: the thread of intent that survives every turn (D67)",
			verbs: []verb{
				{"list", "print a bounded ledger summary (--json for the records without history, --json --history for the full records, --done to list archived goals)", runGoalList},
				{"show", "print one goal without ledger history (--history for the full record)", runGoalShow},
				{"branch", "inspect, commit, land, verify, and sweep goal branches", runGoalBranch},
				{"open", "declare a goal; Current when none exists, queued otherwise", runGoalOpen},
				{"carry", "human-only: name the live successor carrying an abandoned goal, or carry a landing past one named refusal or testing group", runGoalCarry},
				{"done", "conclude the Current goal; requires --then or --and-none", runGoalDone},
				{"claim", "claim a goal (or its whole arc with --arc) for this machine", runGoalClaim},
				{"approve", "human-only (or a seat --under a power of attorney): approve a goal for execution (its box goes through goal budget), or run the grandfather sweep", runGoalApprove},
				{"next", "select ordered claimable work for one machine (read-only)", runGoalNext},
				{"reconcile", "adopt, restore, or authority-replay bytes the verbs did not write", runGoalReconcile},
				{"migrate", "the cutover: one commit turns the legacy ledger into the multi-machine tree (human act, reviewed bytes)", runGoalMigrate},
				{"fetch", "the read-side advance: validate the canonical tip and move the accepted ref", runGoalFetch},
			},
		},
		{
			name:    "seat",
			summary: "the fleet's seats: who is present, what each is running",
			verbs: []verb{
				{"launch", "clone, build, configure, enroll and supervise one new machine of this fleet on this host", runSeatLaunch},
			},
		},
		{
			name:    "steward",
			summary: "the idle watchdog: open delegated work is never silently idle (D121)",
			verbs: []verb{
				{"tick", "one scheduled observation: decide, age the evidence, report the action", runStewardTick},
				{"run", "the runner's body: tick until disarmed (spawned by arm; callable by any external ticker)", runStewardRun},
				{"arm", "explicit human enrollment and runner start (long form of metasystem system start)", runStewardArm},
			},
		},
		{
			name:    "run",
			summary: "tracked long-running work: launch, watch, conclude (the monitor facility)",
			verbs: []verb{
				{"launch", "reserve, spawn the wrapped command detached, print the watch line", runRunLaunch},
				{"wrap", "the setsid leader: bind, run the workload, write the exit sidecar (internal)", runRunWrap},
			},
		},
		{
			name:    "mission",
			summary: "the mission domain's detached loop",
			verbs: []verb{
				{"run-loop", "the detached mission loop (internal; spawned by start/resume)", runMissionRunnerRunLoop},
			},
		},
		{
			name:    "supervise",
			summary: "the supervision lifecycle (docs/design/supervision-lifecycle.md)",
			verbs: []verb{
				{"owner", "run the owner loop for a checkout (internal; launched by up)", runSuperviseOwnerLoop},
				{"component", "run a supervised component (internal; launched by the owner)", runSuperviseComponent},
				{"launch-detached", "start a command in its own session with logged output", runSuperviseLaunchDetached},
			},
		},
	}
}

func main() {
	os.Exit(dispatch(os.Args[1:]))
}

func dispatch(args []string) int {
	return dispatchWithFamilies(args, os.Stdout, os.Stderr, families())
}

func dispatchWithFamilies(args []string, stdout, stderr io.Writer, registered []family) int {
	return dispatchWithFamiliesAndRepositoryTop(args, stdout, stderr, registered, stateroot.RepositoryTop)
}

func dispatchWithRepositoryTop(args []string, repositoryTop func(string) (string, error)) int {
	return dispatchWithFamiliesAndRepositoryTop(args, os.Stdout, os.Stderr, families(), repositoryTop)
}

// dispatchWithFamiliesAndRepositoryTop routes one invocation: the public
// (object, action) pairs first, then the hidden entries, then the explicit
// internal form. A (word, verb) pair that is neither a public action nor an
// entry falls through to its family, transitionally, so machinery callers
// keep working until their port; nothing else is routed, and an unknown word
// is refused before any effect.
func dispatchWithFamiliesAndRepositoryTop(args []string, stdout, stderr io.Writer, registered []family, repositoryTop func(string) (string, error)) int {
	if len(args) == 0 {
		writeIntentRootHelp(stdout)
		return 0
	}
	if args[0] == "help" {
		return runIntentHelp(args[1:], stdout, stderr)
	}
	if args[0] == "--help" || args[0] == "-h" {
		if len(args) != 1 {
			fmt.Fprintln(stderr, "usage: metasystem help [OBJECT [ACTION]]")
			return 2
		}
		writeIntentRootHelp(stdout)
		return 0
	}
	if args[0] == "internal" {
		if len(args) == 1 || len(args) == 2 && (args[1] == "--help" || args[1] == "-h") {
			writeInternalUsage(stdout, registered)
			return 0
		}
		return dispatchInternal(args[1:], stdout, stderr, registered, repositoryTop)
	}
	if args[0] == "status" {
		command, _ := findIntentCommand("status")
		if len(args) == 2 && isHelpWord(args[1]) {
			writeIntentHelp(stdout, command)
			return 0
		}
		return runIntent(command, args[1:], stdout, stderr, defaultIntentOwners())
	}
	if isIntentObject(args[0]) {
		return dispatchObject(args, stdout, stderr, registered, repositoryTop)
	}
	// Process entrypoints whose first word is not an object (supervise,
	// steward, up, ...) and the transitional families keep their argv.
	if args[0] == runtimes.SupervisorEntry {
		return dispatchInternal(args, stdout, stderr, registered, repositoryTop)
	}
	if len(args) >= 2 && !isHelpWord(args[1]) && (args[0] == "up" || familyHasVerb(registered, args[0], args[1])) {
		return dispatchInternal(args, stdout, stderr, registered, repositoryTop)
	}
	if args[0] == "up" {
		return dispatchInternal(args, stdout, stderr, registered, repositoryTop)
	}
	writeUnknownIntentCommand(stderr, args[0], args[1:])
	return 2
}

func isHelpWord(word string) bool { return word == "--help" || word == "-h" || word == "-help" }

// dispatchObject routes a first word that is an object: its action list, a
// public action, a hidden entry, or a family verb of the same name that no
// public action or entry takes.
func dispatchObject(args []string, stdout, stderr io.Writer, registered []family, repositoryTop func(string) (string, error)) int {
	object := args[0]
	if len(args) == 1 || isHelpWord(args[1]) {
		if len(args) > 2 {
			fmt.Fprintf(stderr, "usage: metasystem %s ACTION [TARGET...] [OPTIONS]\n", object)
			return 2
		}
		writeIntentObjectHelp(stdout, object)
		return 0
	}
	if command, ok := findIntentAction(object, args[1]); ok {
		if command.hidden {
			return dispatchInternal(args, stdout, stderr, registered, repositoryTop)
		}
		if len(args) == 3 && isHelpWord(args[2]) {
			writeIntentHelp(stdout, command)
			return 0
		}
		if command.passthrough != nil {
			return command.passthrough(args[2:])
		}
		return runIntent(command, args[2:], stdout, stderr, defaultIntentOwners())
	}
	if familyHasVerb(registered, object, args[1]) {
		return dispatchInternal(args, stdout, stderr, registered, repositoryTop)
	}
	writeUnknownIntentAction(stderr, object, args[1], args[2:])
	return 2
}

func familyHasVerb(registered []family, name, verb string) bool {
	for _, fam := range registered {
		if fam.name != name {
			continue
		}
		for _, v := range fam.verbs {
			if v.name == verb {
				return true
			}
		}
	}
	return false
}

// dispatchInternal routes the engine families and the internal top-level
// calls with their own parsers, output and exit codes.
func dispatchInternal(args []string, stdout, stderr io.Writer, registered []family, repositoryTop func(string) (string, error)) int {
	if len(args) == 2 && (args[1] == "--help" || args[1] == "-h") {
		for _, fam := range registered {
			if fam.name == args[0] {
				writeFamilyHelp(stdout, fam)
				return 0
			}
		}
	}
	if args[0] == "up" {
		return runUpWith(args[1:], repositoryTop)
	}
	if args[0] == "hook" {
		return runHookEntry(args[1:])
	}
	if args[0] == "pre-commit" {
		return runPreCommitEntry(args[1:], stdout, stderr)
	}
	if args[0] == runtimes.SupervisorEntry {
		return runDelegateSupervisor(args[1:])
	}
	if args[0] == "wait" {
		return runWait(args[1:])
	}
	if args[0] == "delegate" {
		return runDelegate(args[1:])
	}
	for _, fam := range registered {
		if fam.name != args[0] {
			continue
		}
		if len(args) < 2 {
			fmt.Fprintf(stderr, "metasystem %s: a verb is required\n", fam.name)
			writeFamilyHelp(stderr, fam)
			return 2
		}
		for _, v := range fam.verbs {
			if v.name == args[1] {
				return v.run(args[2:])
			}
		}
		fmt.Fprintf(stderr, "metasystem %s: unknown verb %q\n", fam.name, args[1])
		writeFamilyHelp(stderr, fam)
		return 2
	}
	writeUnknownIntentCommand(stderr, args[0], args[1:])
	return 2
}

// writeUnknownIntentCommand refuses a first word no object answers to and
// names the current command that was probably meant.
func writeUnknownIntentCommand(w io.Writer, name string, rest []string) {
	fmt.Fprintf(w, "metasystem: unknown object %q; nothing was done\n", name)
	if near := suggestIntent(name, rest); len(near) > 0 {
		fmt.Fprintf(w, "did you mean: %s\n", strings.Join(near, " | "))
	}
	fmt.Fprintln(w, "metasystem lists the objects; metasystem OBJECT lists its actions")
}

// writeUnknownIntentAction refuses an action the object does not have.
func writeUnknownIntentAction(w io.Writer, object, action string, rest []string) {
	fmt.Fprintf(w, "metasystem %s: unknown action %q; nothing was done\n", object, action)
	if near := suggestIntentAction(object, action, rest); len(near) > 0 {
		fmt.Fprintf(w, "did you mean: %s\n", strings.Join(near, " | "))
	}
	fmt.Fprintf(w, "metasystem %s lists its actions\n", object)
}

func writeFamilyHelp(w io.Writer, fam family) {
	fmt.Fprintf(w, "usage: metasystem internal %s <verb> [flags]\n%s\n", fam.name, fam.summary)
	for _, command := range fam.verbs {
		fmt.Fprintf(w, "  %-14s %s\n", command.name, command.summary)
	}
	if len(fam.verbs) > 0 {
		fmt.Fprintf(w, "example: metasystem internal %s %s --help (show leaf flags)\n", fam.name, fam.verbs[0].name)
	}
	fmt.Fprintln(w, "Flags are specific to each verb; use the verb help before adding options such as --root or --dir.")
	if fam.name == "launch" {
		fmt.Fprintln(w, "Launch records belong to the current user under ~/.metasystem/launch; they are not selected by repository.")
		fmt.Fprintln(w, "--root is not a launch flag. Use --id to select a launch record.")
	}
}

func writeInternalUsage(w io.Writer, registered []family) {
	fmt.Fprintln(w, "MetaSystem machinery: maintainer reference for current process protocols.")
	fmt.Fprintln(w, "For application tasks, use metasystem help. These handlers keep their own authority checks.")
	fmt.Fprintln(w)
	writeUsage(w, registered)
}

func writeUsage(w io.Writer, registered []family) {
	fmt.Fprintln(w, "usage: metasystem internal <family> <verb> [flags]")
	fmt.Fprintln(w, "       metasystem internal up [--repo <checkout>] [--pid <pid> --start-time <epoch>]")
	fmt.Fprintln(w, "       metasystem internal up --print-scheduler-entry [--repo <checkout>]")
	fmt.Fprintln(w, "       metasystem internal hook <runtime> <start|stop|end|receipt|tool>  (run by the runtime settings system setup writes)")
	fmt.Fprintln(w, "       metasystem internal pre-commit --root <installation>  (run by the enrolled git pre-commit hook)")
	fmt.Fprintln(w, "       metasystem internal delegate-supervisor <runtime> <verb> --root <installation> [flags]  (launched by internal/delegation and internal/missionrunner/host.go)")
	fmt.Fprintln(w, "       metasystem internal wait (--job <id>|--run <id>|--attempt <id>|--goal <id>|--path <absolute-path> --until <present|absent>|--resume <wait-id>) [--timeout <duration>] [--json]")
	fmt.Fprintln(w, "       metasystem internal wait register --pid <pid> --label <text> [--job <id>] [--timeout <duration>] [--json]")
	fmt.Fprintln(w, "       metasystem internal wait register --human --question <text> --timeout <duration> [--json]")
	fmt.Fprintln(w, "       metasystem internal wait end --wait-id <id> [--json]")
	fmt.Fprintln(w, "       metasystem internal delegate --role <role> --brief <file> --goal <id|none-explicit> --destructive-reach <class> [--op <id>]")
	fmt.Fprintln(w, "       metasystem internal delegate --follow-up <job> --brief <file>")
	fmt.Fprintln(w, "       metasystem internal delegate --cancel <job>")
	fmt.Fprintln(w, "Process entrypoints (started by the named launcher; never typed by a person or agent):")
	for _, command := range intentCommands() {
		if command.hidden {
			fmt.Fprintf(w, "  %-28s %s\n", command.name, command.launcher)
		}
	}
	for _, fam := range registered {
		fmt.Fprintf(w, "  %-10s %s\n", fam.name, fam.summary)
		for _, v := range fam.verbs {
			fmt.Fprintf(w, "    %-14s %s\n", v.name, v.summary)
		}
	}
}
