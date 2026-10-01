// Command metasystem is the metasystem's one binary. Operator verbs such as
// up and health route directly; internal families group the narrower
// decisions that plumbing invokes. Compatibility wrappers keep historical
// names only long enough to exec into these verbs. File naming is one file
// per routed surface; cross-family helpers live in helpers.go and nowhere
// else.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// entry is one process entrypoint of a family: its words, what it does, the
// handler, and the launcher that starts it. Every internal verb is an
// entrypoint (design 3.2): a program other than a person or agent starts it
// as a process of its own. launcher is the file that starts it and evidence
// is the text in that file that names it (the argv words, or the command
// line it writes), which TestEveryInternalVerbHasALauncherThatStartsIt
// checks.
type verb struct {
	name     string
	summary  string
	run      command
	launcher string
	evidence string
	// required are the options the entrypoint refuses to run without.
	required []string
}

type family struct {
	name    string
	summary string
	verbs   []verb
}

// topLevelEntry is an entrypoint whose argv is one word before its own
// arguments.
type topLevelEntry struct {
	name, usage        string
	launcher, evidence string
	required           []string
	run                func(args []string, stdout, stderr io.Writer, repositoryTop func(string) (string, error)) int
}

// topLevelEntries are the one-word entrypoints.
func topLevelEntries() []topLevelEntry {
	entries := registeredTopLevelEntries()
	for i := range entries {
		run := entries[i].run
		entries[i].run = func(args []string, stdout, stderr io.Writer, repositoryTop func(string) (string, error)) int {
			return helpAware(func(args []string, stdout, stderr io.Writer) int { return run(args, stdout, stderr, repositoryTop) })(args, stdout, stderr)
		}
	}
	return entries
}

func registeredTopLevelEntries() []topLevelEntry {
	return []topLevelEntry{
		{name: "up", usage: "up [--repo <checkout>] [--pid <pid> --start-time <epoch>] | up --print-scheduler-entry [--repo <checkout>]",
			launcher: "internal/testrun/rearm.go", evidence: `"up", "--repo"`,
			run: func(args []string, stdout, stderr io.Writer, repositoryTop func(string) (string, error)) int {
				return runUpWith(args, repositoryTop, stdout, stderr)
			}},
		{name: "hook", usage: "hook <runtime> <start|stop|end|receipt|tool>",
			launcher: "internal/hooks/runtime_hook_worker.go", evidence: `"internal", "hook"`,
			run: func(args []string, stdout, stderr io.Writer, _ func(string) (string, error)) int {
				return runHookEntry(args, stdout, stderr)
			}},
		{name: "pre-commit", usage: "pre-commit --root <installation>",
			launcher: "internal/ledgerfence/fence.go", evidence: "internal pre-commit", required: []string{"root"},
			run: func(args []string, stdout, stderr io.Writer, _ func(string) (string, error)) int {
				return runPreCommitEntry(args, stdout, stderr)
			}},
		{name: "pre-push", usage: "pre-push --home <home> <remote> <url>",
			launcher: "internal/landing/lane/hook.go", evidence: "internal pre-push", required: []string{"home"},
			run: func(args []string, stdout, stderr io.Writer, _ func(string) (string, error)) int {
				return runPrePushEntry(args, stdout, stderr)
			}},
		{name: runtimes.SupervisorEntry, usage: runtimes.SupervisorEntry + " <runtime> <verb> --root <installation> [flags]",
			launcher: "internal/delegation/owners.go", evidence: "runtimes.SupervisorArgs",
			run: func(args []string, stdout, stderr io.Writer, _ func(string) (string, error)) int {
				return runDelegateSupervisor(args, stdout, stderr)
			}},
		{name: "delegate", usage: "delegate --revive <intent> | delegate __run-member --root <installation> -- <command...> | delegate __<callback> ...",
			launcher: "cmd/metasystem/steward_verbs.go", evidence: `"internal", "delegate", "--revive"`,
			run: func(args []string, stdout, stderr io.Writer, _ func(string) (string, error)) int {
				return runDelegate(args, stdout, stderr)
			}},
		{name: "__complete", usage: "__complete -- WORD...",
			launcher: "cmd/metasystem/completion.go", evidence: "__complete",
			run: func(args []string, stdout, stderr io.Writer, _ func(string) (string, error)) int {
				return runCompleteEntry(args, stdout, stderr)
			}},
	}
}

// families are the internal entrypoints grouped by their first word.
func families() []family {
	registered := registeredFamilies()
	for i := range registered {
		for j := range registered[i].verbs {
			registered[i].verbs[j].run = helpAware(registered[i].verbs[j].run)
		}
	}
	return registered
}

func registeredFamilies() []family {
	return []family{
		{
			name: "app", summary: "the application's run supervisor",
			verbs: []verb{
				{name: "serve", summary: "own one run of the application for its life", run: runAppServe, launcher: "internal/applaunch/launch.go", evidence: "\"app\", \"serve\""},
			},
		},
		{
			name: "ui", summary: "the browser interface's servers",
			verbs: []verb{
				{name: "serve", summary: "serve the interface in the foreground with a ready descriptor", run: runUIServe, launcher: "internal/ui/lifecycle/launch.go", evidence: "\"ui\", \"serve\""},
				{name: "tools", summary: "serve the interface's read tools to the Project Partner over stdio", run: runUITools, launcher: "internal/ui/partner/runtime.go", evidence: "\"ui\", \"tools\""},
			},
		},
		{
			name: "testing", summary: "the testing contract's git merge driver",
			verbs: []verb{
				{name: "merge-driver", summary: "merge the testing contract as git's merge driver", run: runTestingMergeDriver, launcher: "internal/testpolicy/contractgit/register.go", evidence: "testing merge-driver"},
			},
		},
		{
			name: "test", summary: "proof runs on another engine or as landing prove's child",
			verbs: []verb{
				{name: "plan", summary: "compute a candidate's risk-selected groups for a pinned or candidate engine", run: func(args []string, stdout, stderr io.Writer) int {
					return runTestPlanAs("internal test plan", args, stdout, stderr)
				}, launcher: "cmd/metasystem/test_protection.go", evidence: "\"test\", \"plan\"", required: []string{"root"}},
				{name: "run", summary: "run one proof as landing prove's own child", run: runTestRun, launcher: "internal/landing/kernel/prove.go", evidence: "\"internal\", \"test\", \"run\"", required: []string{"root"}},
				{name: "verify", summary: "verify retained proof on the base engine a carried landing builds", run: runTestVerify, launcher: "cmd/metasystem/landing_path.go", evidence: "\"test\", \"verify\"", required: []string{"root"}},
				{name: "worker-capabilities", summary: "report the testing worker protocol of a pinned engine", run: runTestWorkerCapabilities, launcher: "internal/testrun/worker.go", evidence: "\"test\", \"worker-capabilities\""},
				{name: "worker", summary: "execute one admitted selected plan on a pinned engine", run: runTestWorker, launcher: "cmd/metasystem/test_protection.go", evidence: "\"test\", \"worker\"", required: []string{"packet", "packet-sha256", "result"}},
			},
		},
		{
			name: "brain", summary: "the brain boot's bounded input child",
			verbs: []verb{
				{name: "boot-inputs", summary: "read the brain boot inputs in a child the boot can kill at its deadline", run: runBrainBootInputs, launcher: "cmd/metasystem/brain_boot.go", evidence: "\"brain\", \"boot-inputs\"", required: []string{"root", "repo", "dir"}},
			},
		},
		{
			name: "proof-run", summary: "suite runs, their watchdog and custody",
			verbs: []verb{
				{name: "banner", summary: "print the suite witness state for the development gate", run: runProofRunBanner, launcher: "cmd/devgate/gate.go", evidence: "\"proof-run\", \"banner\"", required: []string{"suite", "root", "progress", "log"}},
				{name: "launch", summary: "launch a suite in its own process group with a sibling watchdog", run: runProofRunLaunch, launcher: "cmd/devgate/gate.go", evidence: "\"proof-run\", \"launch\"", required: []string{"root", "conf"}},
				{name: "worker-authorized", summary: "authenticate a suite worker against its live parent proof", run: runProofRunWorkerAuthorized, launcher: "cmd/devgate/gate.go", evidence: "\"proof-run\", \"worker-authorized\"", required: []string{"root"}},
				{name: "watchdog", summary: "watch suite output growth and enforce the section ceiling", run: runProofRunWatchdog, launcher: "internal/proofrun/launcher.go", evidence: "\"proof-run\", \"watchdog\""},
				{name: "custody-exec", summary: "hold a resource-active command until its custodian binds its identity", run: runProofRunCustodyExec, launcher: "internal/proofrun/resource_custody.go", evidence: "\"proof-run\", \"custody-exec\""},
				{name: "preserve", summary: "copy bounded watchdog evidence in a child the watchdog can kill", run: runProofRunPreserve, launcher: "internal/proofrun/watchdog.go", evidence: "\"proof-run\", \"preserve\""},
			},
		},
		{
			name: "config", summary: "a new machine's configuration steps",
			verbs: []verb{
				{name: "validate", summary: "validate a new machine's configuration on its own engine", run: runConfigValidate, launcher: "internal/seat/launch/sequence.go", evidence: "\"config\", \"validate\""},
			},
		},
		{
			name: "validate", summary: "a new machine's isolation step",
			verbs: []verb{
				{name: "session-isolation", summary: "isolate a new machine's local configuration on its own engine", run: runValidateSessionIsolation, launcher: "internal/seat/launch/sequence.go", evidence: "\"validate\", \"session-isolation\"", required: []string{"source-root", "destination-root", "manifest", "harness-root"}},
			},
		},
		{
			name: "landing", summary: "a carried landing's base-engine judgment",
			verbs: []verb{
				{name: "observe", summary: "judge a carried landing on the base engine it builds", run: runLandingObserve, launcher: "cmd/metasystem/landing_path.go", evidence: "\"landing\", \"observe\""},
				{name: "workspace", summary: "project a carried landing's workspace on the base engine it builds", run: runLandingWorkspace, launcher: "cmd/metasystem/landing_path.go", evidence: "\"landing\", \"workspace\"", required: []string{"root", "tree"}},
			},
		},
		{
			name: "adapter", summary: "runtime hooks the agent runtimes start",
			verbs: []verb{
				{name: "claude-tool-gate", summary: "decide one Claude tool call on the checkout's local engine", run: runAdapterClaudeToolGate, launcher: "internal/hooks/runtime_hook.go", evidence: "\"adapter\", \"claude-tool-gate\"", required: []string{"root"}},
				{name: "claude-session-signal", summary: "record a delegate Claude session's start from its settings hook", run: runAdapterClaudeSessionSignal, launcher: "internal/adapter/claude.go", evidence: "adapter claude-session-signal"},
			},
		},
		{
			name: "launch", summary: "the launch supervisor",
			verbs: []verb{
				{name: "supervise", summary: "own one launched agent process through its end, in a session of its own", run: runLaunchSupervise, launcher: "internal/launch/process.go", evidence: "\"launch\", \"supervise\"", required: []string{"id"}},
			},
		},
		{
			name: "util", summary: "the fake runtime's stand-in child",
			verbs: []verb{
				{name: "hold", summary: "stand in for a runtime child of the fake runtime until signalled", run: runUtilHold, launcher: "internal/adapter/supervisor/fake.go", evidence: "\"util\", \"hold\"", required: []string{"tag"}},
			},
		},
		{
			name: "goal", summary: "a new machine's ledger steps",
			verbs: []verb{
				{name: "next", summary: "select a new machine's claimable work on its own engine", run: runGoalNext, launcher: "internal/seat/launch/sequence.go", evidence: "\"goal\", \"next\""},
				{name: "fetch", summary: "advance a new machine's accepted ledger on its own engine", run: runGoalFetch, launcher: "internal/seat/launch/sequence.go", evidence: "\"goal\", \"fetch\""},
			},
		},
		{
			name: "seat", summary: "a new machine's launch",
			verbs: []verb{
				{name: "launch", summary: "clone, build, configure and supervise a new machine, outliving the request", run: runSeatLaunch, launcher: "cmd/metasystem/ui_launch.go", evidence: "\"seat\", \"launch\""},
			},
		},
		{
			name: "steward", summary: "the steward's runner and a new machine's enrollment",
			verbs: []verb{
				{name: "run", summary: "the steward's resident runner", run: runStewardRun, launcher: "internal/steward/runner.go", evidence: "\"steward\", \"run\"", required: []string{"repo"}},
				{name: "arm", summary: "enroll a new machine's steward on its own engine", run: runStewardArm, launcher: "internal/seat/launch/sequence.go", evidence: "\"steward\", \"arm\"", required: []string{"repo"}},
			},
		},
		{
			name: "run", summary: "the cadence run's detached leader",
			verbs: []verb{
				{name: "wrap", summary: "the detached leader of landing validate's cadence run", run: runRunWrap, launcher: "internal/cadence/tick.go", evidence: "\"run\", \"wrap\""},
			},
		},
		{
			name: "mission", summary: "a mission's detached loop",
			verbs: []verb{
				{name: "run-loop", summary: "a started mission's detached loop", run: runMissionRunnerRunLoop, launcher: "internal/missionrunner/launch.go", evidence: "\"mission\", \"run-loop\""},
			},
		},
		{
			name: "supervise", summary: "the supervision owner and its components",
			verbs: []verb{
				{name: "owner", summary: "the supervision owner loop in a session of its own", run: runSuperviseOwnerLoop, launcher: "internal/supervise/arming.go", evidence: "\"supervise\", \"owner\"", required: []string{"repo", "tag"}},
				{name: "component", summary: "one supervised component the owner starts", run: runSuperviseComponent, launcher: "cmd/metasystem/supervise_owner.go", evidence: "\"supervise\", \"component\"", required: []string{"component", "tag", "heartbeat"}},
			},
		},
	}
}

func main() {
	// While the seat is at the helm, a person proof refused in the seat's
	// primary checkout is the holder's act (helm_admits.go).
	wireHelmAdmission()
	// A live general power of attorney makes the act of the main session
	// holding the checkout's lease the granting person's (attorney_admits.go).
	wireAttorneyAdmission()
	// Their notices wait for the command's outcome (admission_notice.go).
	processAdmissionNotices.arm()
	os.Exit(dispatch(os.Args[1:]))
}

func dispatch(args []string) int {
	return dispatchWithFamilies(args, os.Stdout, os.Stderr, families())
}

// dispatchWithFamilies releases the process's scratch on the way to the exit
// code, never in main (R2): a normal end leaves nothing, and a root a child
// still holds, or that a goroutine still uses, stays for the sweeper's
// process proof.
func dispatchWithFamilies(args []string, stdout, stderr io.Writer, registered []family) int {
	defer func() {
		_ = diskstore.ReleaseProcessScratch(context.Background(), diskstore.WriterDrain{Now: time.Now, Sleep: time.Sleep})
	}()
	return dispatchWithFamiliesAndRepositoryTop(args, stdout, stderr, registered, stateroot.RepositoryTop)
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
	// The top-level page takes the flags every command takes (F1): bare
	// metasystem --repo PATH is the page, as it is from any path.
	if strings.HasPrefix(args[0], "-") {
		if rest, ok := stripHelpGlobalFlags(args); ok && (len(rest) == 0 || len(rest) == 1 && isHelpWord(rest[0])) {
			writeIntentRootHelp(stdout)
			return 0
		}
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
	// steward, up, ...) keep their argv; a family word with a verb it lacks,
	// or none, is answered by the family's own internal help.
	if isTopLevelEntry(args[0]) || familyNamed(registered, args[0]) {
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
	// An object's page takes the flags every command takes (F1).
	if len(args) > 1 && strings.HasPrefix(args[1], "-") {
		if rest, ok := stripHelpGlobalFlags(args[1:]); ok && (len(rest) == 0 || len(rest) == 1 && isHelpWord(rest[0])) {
			writeIntentObjectHelp(stdout, object)
			return 0
		}
	}
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
			return command.passthrough(args[2:], stdout, stderr)
		}
		return runIntent(command, args[2:], stdout, stderr, defaultIntentOwners())
	}
	if familyHasVerb(registered, object, args[1]) {
		return dispatchInternal(args, stdout, stderr, registered, repositoryTop)
	}
	writeUnknownIntentAction(stderr, object, args[1], args[2:])
	return 2
}

// helpGlobalFlags are the flags every public command takes that change
// nothing a help page shows; the value says whether the flag takes one.
var helpGlobalFlags = map[string]bool{"repo": true, "root": true, "json": false, "verbose": false}

// stripHelpGlobalFlags removes the help-neutral global flags from args. It
// fails when one of them is missing its value, so the caller refuses instead
// of showing a page for a half-typed command line.
func stripHelpGlobalFlags(args []string) ([]string, bool) {
	var rest []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if !strings.HasPrefix(arg, "-") || isHelpWord(arg) {
			rest = append(rest, arg)
			continue
		}
		name, _, joined := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		takesValue, known := helpGlobalFlags[name]
		switch {
		case !known:
			rest = append(rest, arg)
		case takesValue && !joined:
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return nil, false
			}
			index++
		}
	}
	return rest, true
}

// looksLikeFlagOrPath reports whether a word in an object's or action's
// place is a flag or a path: no command is spelled like one, so a "did you
// mean" for it would only guess, and a guess may change state (F1).
func looksLikeFlagOrPath(word string) bool {
	return strings.HasPrefix(word, "-") || strings.ContainsRune(word, '/') || strings.HasPrefix(word, ".") || strings.HasPrefix(word, "~")
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
	for _, entry := range topLevelEntries() {
		if entry.name == args[0] {
			return entry.run(args[1:], stdout, stderr, repositoryTop)
		}
	}
	for _, fam := range registered {
		if fam.name != args[0] {
			continue
		}
		if len(args) >= 2 {
			for _, v := range fam.verbs {
				if v.name == args[1] {
					return v.run(args[2:], stdout, stderr)
				}
			}
			if isHelpWord(args[1]) {
				fmt.Fprintf(stderr, "metasystem internal %s: %s takes no further word; nothing was done\n", fam.name, args[1])
			} else {
				fmt.Fprintf(stderr, "metasystem internal %s: %q is not one of its entrypoints; nothing was done\n", fam.name, args[1])
			}
		} else {
			fmt.Fprintf(stderr, "internal %s needs one of its entrypoints named; nothing was done\n", fam.name)
		}
		writeFamilyHelp(stderr, fam)
		return 2
	}
	fmt.Fprintf(stderr, "no internal entrypoint is named %q; nothing was done\n", args[0])
	fmt.Fprintln(stderr, "run: metasystem help  (the public commands; metasystem internal lists the entrypoints)")
	return 2
}

// topLevelEntryNamed returns the one-word entrypoint called word.
func topLevelEntryNamed(word string) (topLevelEntry, bool) {
	for _, entry := range topLevelEntries() {
		if entry.name == word {
			return entry, true
		}
	}
	return topLevelEntry{}, false
}

// isTopLevelEntry reports whether word is a one-word entrypoint.
func isTopLevelEntry(word string) bool {
	for _, entry := range topLevelEntries() {
		if entry.name == word {
			return true
		}
	}
	return false
}

// familyNamed reports whether word names an internal family.
func familyNamed(registered []family, word string) bool {
	for _, fam := range registered {
		if fam.name == word {
			return true
		}
	}
	return false
}

// writeUnknownIntentCommand refuses a first word no object answers to and
// names the current command that was probably meant.
func writeUnknownIntentCommand(w io.Writer, name string, rest []string) {
	entry, top := topLevelEntryNamed(name)
	switch {
	case top:
		fmt.Fprintf(w, "metasystem: %q is not an object people or agents use but a process entrypoint that %s starts; nothing was done\n", name, entry.launcher)
	case familyNamed(families(), name):
		fmt.Fprintf(w, "metasystem: %q is not an object people or agents use but a family of process entrypoints; metasystem internal %s lists them and what starts each; nothing was done\n", name, name)
	default:
		fmt.Fprintf(w, "metasystem: unknown object %q; nothing was done\n", name)
	}
	if looksLikeFlagOrPath(name) {
		// No suggestion: a flag or path is not a misspelt object.
	} else if near := suggestIntent(name, rest); len(near) > 0 {
		fmt.Fprintf(w, "did you mean: %s\n", strings.Join(near, " | "))
	}
	fmt.Fprintln(w, "metasystem lists the objects; metasystem OBJECT lists its actions")
}

// retiredIntentAction is an action that was removed, with why and the one
// command that does its work now: someone who learned it is told so, not
// offered a guess by spelling.
type retiredIntentAction struct{ why, next string }

var retiredIntentActions = map[string]retiredIntentAction{
	"landing restart": {why: "the lane has no owner to restart now", next: "metasystem landing start"},
}

// writeUnknownIntentAction refuses an action the object does not have.
func writeUnknownIntentAction(w io.Writer, object, action string, rest []string) {
	if retired, ok := retiredIntentActions[object+" "+action]; ok {
		fmt.Fprintf(w, "metasystem %s %s was removed; %s; nothing was done\nrun: %s\n", object, action, retired.why, retired.next)
		return
	}
	fmt.Fprintf(w, "metasystem %s: unknown action %q; nothing was done\n", object, action)
	if looksLikeFlagOrPath(action) {
		// No suggestion: a flag or path is not a misspelt action, and the
		// nearest action by spelling may be one that changes state.
	} else if near := suggestIntentAction(object, action, rest); len(near) > 0 {
		fmt.Fprintf(w, "did you mean: %s\n", strings.Join(near, " | "))
	}
	fmt.Fprintf(w, "metasystem %s lists its actions\n", object)
}

func writeFamilyHelp(w io.Writer, fam family) {
	fmt.Fprintf(w, "metasystem internal %s: %s; process entrypoints, each started by the program named, not commands for people or agents\n", fam.name, fam.summary)
	for _, v := range fam.verbs {
		fmt.Fprintf(w, "  %-24s %s (started by %s)\n", v.name, v.summary, v.launcher)
	}
	if len(fam.verbs) > 0 {
		fmt.Fprintf(w, "metasystem internal %s %s --help shows one entrypoint's options\n", fam.name, fam.verbs[0].name)
	}
	fmt.Fprintln(w, "metasystem help lists what people and agents run")
}

func writeInternalUsage(w io.Writer, registered []family) {
	fmt.Fprintln(w, "MetaSystem machinery: maintainer reference for current process protocols.")
	fmt.Fprintln(w, "For application tasks, use metasystem help. These handlers keep their own authority checks.")
	fmt.Fprintln(w)
	writeUsage(w, registered)
}

func writeUsage(w io.Writer, registered []family) {
	fmt.Fprintln(w, "Process entrypoints (started by the named launcher; never typed by a person or agent):")
	for _, entry := range topLevelEntries() {
		fmt.Fprintf(w, "  %-36s %s\n", entry.name, entry.launcher)
	}
	for _, fam := range registered {
		for _, v := range fam.verbs {
			fmt.Fprintf(w, "  %-36s %s\n", fam.name+" "+v.name, v.launcher)
		}
	}
}
