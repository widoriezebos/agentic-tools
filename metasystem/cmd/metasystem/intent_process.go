package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/contract"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/evidencetable"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostsetup"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/covenant"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hookswitch"
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
	lineage := intentFlag{name: "lineage", value: "LINEAGE", advanced: true, hidden: true, usage: "the session's owner lineage"}
	return []intentCommand{
		{
			object: "system", action: "start", audience: "human", summary: "start MetaSystem for this checkout",
			usage: []string{"metasystem system start", "metasystem system start --if-down"},
			details: []string{
				"Done by a person at their enrolled terminal. New work may start again, and the helpers that watch and continue work are started.",
				"A terminal that stays enrolled keeps its authority when the engine is rebuilt; no new enrollment is needed.",
				"--if-down is the scheduler's recovery: it starts only helpers that are down and carries no session or lease authority.",
			},
			flags:    []intentFlag{intentInstallationFlag, intentTemporaryArmFlag, intentReviewByFlag, {name: "if-down", advanced: true, usage: "recover only helpers that are down (the scheduler's recovery path)"}},
			maxArgs:  0,
			examples: []string{"metasystem system start"},
			run:      runIntentSystemStart,
		},
		{
			object: "system", action: "stop", audience: "human", summary: "stop MetaSystem for this checkout",
			usage: []string{"metasystem system stop"},
			details: []string{
				"Done by a person at their enrolled terminal. Every job and helper of this checkout stops, and no new work starts until system start.",
				"If something keeps running, stop says what, and nothing is reported as stopped that is not.",
			},
			flags:    []intentFlag{intentInstallationFlag},
			maxArgs:  0,
			examples: []string{"metasystem system stop"},
			run:      runIntentSystemStop,
		},
		{
			object: "system", action: "restart", audience: "human", summary: "stop and start MetaSystem for this checkout",
			usage: []string{"metasystem system restart"},
			details: []string{
				"Starts again only after everything stopped; if either half fails, it says where it got to and what to run next.",
				"Requires enrolled-terminal human authority.",
			},
			flags:    []intentFlag{intentInstallationFlag, intentTemporaryArmFlag, intentReviewByFlag},
			maxArgs:  0,
			examples: []string{"metasystem system restart"},
			run:      runIntentSystemRestart,
		},
		{
			object: "system", action: "status", audience: "both", summary: "whether MetaSystem runs for this checkout, and its watchdog's view",
			usage: []string{"metasystem system status", "metasystem system status --steward"},
			details: []string{"Unknown and stale readings are reported as such; a read failure is never shown as stopped or healthy.",
				"--steward prints the idle watchdog's view: evidence age, live intents and pending notifications."},
			flags:    []intentFlag{intentInstallationFlag, {name: "steward", usage: "the idle watchdog's view instead"}},
			maxArgs:  0,
			examples: []string{"metasystem system status", "metasystem system status --steward"},
			run:      runIntentSystemStatus,
		},
		{
			object: "system", action: "check", primary: true, audience: "both", summary: "diagnose problems with this checkout, changing nothing",
			usage:    []string{"metasystem system check"},
			details:  []string{"Checks this checkout once: its machinery, what system setup would change, its skills, and its app covenant (shape and evidence) when it has one; it repairs nothing.", "Each problem names the command that fixes it, where there is one."},
			flags:    []intentFlag{intentInstallationFlag},
			maxArgs:  0,
			examples: []string{"metasystem system check", "metasystem system check --json"},
			run:      runIntentDoctor,
		},
		{
			object: "system", action: "enroll", audience: "human", summary: "authenticate your terminal for human decisions in MetaSystem",
			usage:    []string{"metasystem system enroll --name NAME"},
			details:  []string{"Run it at an agent-free terminal. The local enrollment and its fleet publication are reported separately."},
			flags:    []intentFlag{{name: "name", aliases: []string{"by"}, value: "NAME", usage: "your name"}, intentLineageFlag},
			maxArgs:  0,
			examples: []string{"metasystem system enroll --name Wido"},
			run:      runIntentEnroll,
		},
		{
			object: "system", action: "setup", audience: "both", summary: "set this checkout up to work with its engine: runtimes, hooks, commit fence and testing-contract merges",
			usage: []string{"metasystem system setup [--runtimes CSV|none] [--copy-skills]"},
			details: []string{
				"Checks that the engine answers the hook entry, then registers the agent runtimes (instruction pointers, skills, profiles and lifecycle hooks), writes every registered runtime's hooks to run the engine directly, enrolls the git pre-commit fence and registers the testing contract's git merge driver.",
				"The runtimes are the ones metasystem.runtimes enables unless --runtimes names them. Nothing is written when the engine is missing or older than the hook entry; the refusal names the build. A repeat with everything in place changes nothing; metasystem system check reports what setup would change.",
			},
			flags: []intentFlag{intentInstallationFlag,
				{name: "runtimes", value: "CSV", usage: "the runtimes to register, or none (default: those metasystem.runtimes enables)"},
				{name: "copy-skills", usage: "copy skill trees instead of linking them"}},
			maxArgs:  0,
			examples: []string{"metasystem system setup", "metasystem system setup --runtimes claude,codex"},
			run:      runIntentSystemSetup,
		},
		adoptIntentCommand(),
		{
			object: "session", action: "start", audience: "agent", summary: "prepare the current agent session to work",
			usage:    []string{"metasystem session start"},
			details:  []string{"Announces this session, restamps its stop capability and starts the helpers the session needs."},
			flags:    []intentFlag{intentInstallationFlag, lineage},
			maxArgs:  0,
			examples: []string{"metasystem session start"},
			run:      runIntentSessionStart,
		},
		{
			object: "session", action: "stop", audience: "human", summary: "stop this session's work quietly",
			usage:    []string{"metasystem session stop --by NAME"},
			details:  []string{"A person authorizes one quiet stop of the announced main session at the enrolled terminal; the checkout keeps running."},
			flags:    []intentFlag{{name: "by", value: "NAME", usage: "the attending person"}},
			maxArgs:  0,
			examples: []string{"metasystem session stop --by Wido"},
			run:      func(inv *intentInvocation) int { return inv.stopSession() },
		},
		{
			object: "mission", action: "start", audience: "both", summary: "start an autonomous mission",
			usage: []string{"metasystem mission start M [--wait]"}, maxArgs: 1, examples: []string{"metasystem mission start demo"},
			details: []string{"The mission's signed contract passes its launch checks first. Without --wait the mission runs on its own and this returns once its first turn starts."},
			flags:   []intentFlag{missionWaitFlag},
			run:     func(inv *intentInvocation) int { return runIntentMissionNamed(inv, "start") },
		},
		{
			object: "mission", action: "status", audience: "both", summary: "a mission's runner status",
			usage: []string{"metasystem mission status M"}, maxArgs: 1, examples: []string{"metasystem mission status demo"},
			run: func(inv *intentInvocation) int { return runIntentMissionNamed(inv, "status") },
		},
		{
			object: "mission", action: "resume", audience: "human", summary: "resume a parked or interrupted mission",
			usage: []string{"metasystem mission resume M [--wait]"}, maxArgs: 1, examples: []string{"metasystem mission resume demo"},
			flags: []intentFlag{missionWaitFlag},
			run:   func(inv *intentInvocation) int { return runIntentMissionNamed(inv, "resume") },
		},
		{
			object: "mission", action: "seal", audience: "human", summary: "check a mission contract and seal it so a person can sign it",
			usage: []string{"metasystem mission seal M"},
			details: []string{
				"M is the mission id (plans/mission-M.contract.md) or the contract file.",
				"Checks the authored contract and prints any sizing warnings, then records its baseline and priced exposure in the file.",
				"Add the approval line it names, commit it, and run metasystem mission start M. A sealed contract is left as it is.",
			},
			maxArgs: 1, examples: []string{"metasystem mission seal demo"},
			run: runIntentMissionSeal,
		},
		{
			object: "mission", action: "repair", audience: "human", summary: "record a person's resolution of one mission workspace problem",
			usage: []string{"metasystem mission repair M --problem N --confirm-restored TREE --by NAME --reason TEXT",
				"metasystem mission repair M --problem N --accept-workspace --waive CLAIM... --by NAME --reason TEXT"},
			details: []string{
				"--confirm-restored says the files already match that recorded safe tree (restore them first; nothing is restored by this command);",
				"--accept-workspace accepts the observed workspace with each waived attribution claim named. Every problem must be resolved before the mission resumes.",
			},
			flags: []intentFlag{
				{name: "problem", value: "N", usage: "the recorded problem's number"},
				{name: "confirm-restored", value: "TREE", usage: "the recorded safe tree the files already match"},
				{name: "accept-workspace", usage: "accept the observed workspace"},
				{name: "waive", value: "CLAIM", repeat: true, usage: "with --accept-workspace: an attribution claim waived (repeatable)"},
				{name: "by", value: "NAME", usage: "the person deciding"},
				reasonFlag("why", "why"),
			},
			maxArgs:  1,
			examples: []string{"metasystem mission repair demo --problem 2 --confirm-restored 3f2a9c1e0d4b5a6978695a4b3c2d1e0f98765432 --by Wido --reason 'restored from the snapshot'"},
			run: func(inv *intentInvocation) int {
				if len(inv.input.args) != 1 {
					return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "mission repair needs the mission: metasystem mission repair M ...; nothing was done"})
				}
				return runIntentRepairMission(inv, inv.input.args[0])
			},
		},
		{
			object: "ui", action: "start", audience: "both", summary: "start the browser interface",
			usage: []string{"metasystem ui start [--listen ADDRESS]"}, flags: []intentFlag{intentUIListenFlag, intentInstallationFlag}, maxArgs: 0,
			examples: []string{"metasystem ui start"}, run: func(inv *intentInvocation) int { return inv.uiTarget("start") },
		},
		{
			object: "ui", action: "stop", audience: "both", summary: "stop the browser interface",
			usage: []string{"metasystem ui stop [--wait-seconds N]"}, flags: []intentFlag{intentUIWaitFlag, intentInstallationFlag}, maxArgs: 0,
			examples: []string{"metasystem ui stop"}, run: func(inv *intentInvocation) int { return inv.uiTarget("stop") },
		},
		{
			object: "ui", action: "restart", audience: "both", summary: "restart the browser interface with the executable on disk",
			usage:    []string{"metasystem ui restart [--listen ADDRESS] [--wait-seconds N]"},
			details:  []string{"An agent may do this within its authorization. Starting the interface through an agent does not authenticate a person for its human actions."},
			flags:    []intentFlag{intentUIListenFlag, intentUIWaitFlag, intentInstallationFlag},
			maxArgs:  0,
			examples: []string{"metasystem ui restart"}, run: func(inv *intentInvocation) int { return inv.runUIVerb("restart") },
		},
		{
			object: "ui", action: "status", audience: "both", summary: "whether the browser interface runs, and where",
			usage: []string{"metasystem ui status"}, flags: []intentFlag{intentInstallationFlag}, maxArgs: 0,
			examples: []string{"metasystem ui status"}, run: func(inv *intentInvocation) int { return inv.uiTarget("status") },
		},
		{
			object: "machine", action: "list", audience: "both", summary: "every machine's presence, this one first",
			usage:    []string{"metasystem machine list [--refresh]"},
			flags:    []intentFlag{{name: "refresh", aliases: []string{"fetch"}, usage: "fetch presence now instead of the last copy"}},
			maxArgs:  0,
			examples: []string{"metasystem machine list", "metasystem machine list --refresh"},
			run:      runIntentFleet,
		},
		{
			object: "machine", action: "start", audience: "human", summary: "clone, build, configure, enroll and supervise one new machine of this fleet",
			usage:   []string{"metasystem machine start NAME [--destination PATH] [--resume ID]"},
			details: []string{"A person's act on this host; --resume ID continues an interrupted launch."},
			flags: []intentFlag{intentTemporaryArmFlag, intentReviewByFlag,
				{name: "destination", value: "PATH", advanced: true, usage: "where the new machine's clone lands"},
				{name: "resume", value: "ID", advanced: true, usage: "continue this interrupted launch"}},
			maxArgs:  1,
			examples: []string{"metasystem machine start m1f"},
			run: func(inv *intentInvocation) int {
				if len(inv.input.args) != 1 {
					return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "machine start needs the machine's name: metasystem machine start NAME; nothing was done"})
				}
				return runIntentStartMachine(inv, inv.input.args[0])
			},
		},
		{
			object: "work", action: "status", primary: true, audience: "both", summary: "running work, or one goal's work, job, run or read",
			usage: []string{"metasystem work status [--all]", "metasystem work status G [--work NAME]", "metasystem work status REF",
				"metasystem work status [G | j1:ID] --history [--since RFC3339]"},
			details: []string{
				"Without a target: this user's running launches and this repository's running dispatch jobs, each with its reference; --all adds ended ones.",
				"REF is a goal, or a reference as a result printed it: j1:ID (a launch), j2:ID (a dispatch job), run:ID (a unit run) or read:REF (a diagnostic read).",
				"A bare id is searched in each of those stores; several matches refuse and list each reference. A reference is matched exactly, never by prefix.",
				"--history reports how launches ended and why any was refused: every launch, one goal's, or one launch; --since keeps activity at or after that instant.",
			},
			flags: []intentFlag{{name: "all", usage: "without a target: ended jobs too"}, {name: "work", value: "NAME", usage: "with G: only this named work"},
				{name: "history", usage: "how launches ended and why any was refused"}, {name: "since", value: "RFC3339", usage: "with --history: activity at or after this instant"}},
			maxArgs:  1,
			accepts:  []string{refGoal, refJ1, refJ2, refRun, refRead},
			examples: []string{"metasystem work status", "metasystem work status verbs-match-intent", "metasystem work status j2:design-r2-4f1c", "metasystem work status verbs-match-intent --history"},
			run:      runIntentWorkStatus,
		},
		{
			object: "work", action: "stop", audience: "both", summary: "stop one running job or diagnostic read, or every running job of a goal",
			usage: []string{"metasystem work stop REF", "metasystem work stop G"},
			details: []string{"REF is j1:ID (a launch), j2:ID (a dispatch job) or read:REF (a diagnostic read): exactly that one stops.",
				"G is a goal: every running job of that goal stops, and no other goal's. A goal its budget stopped completes its recorded stop by itself once none of its jobs runs."},
			maxArgs:  1,
			accepts:  []string{refGoal, refJ1, refJ2, refRead},
			examples: []string{"metasystem work stop j2:design-r2-4f1c", "metasystem work stop verbs-match-intent"},
			run:      runIntentWorkStop,
		},
		{
			object: "question", action: "ask", primary: true, audience: "agent", summary: "ask the person a question through the channel",
			usage: []string{"metasystem question ask G --question TEXT --option TEXT...", "metasystem question ask G --question TEXT --option 'LABEL: CONSEQUENCE'... [--recommend LABEL]"},
			details: []string{
				"The person answers in the channel thread; the answer is authenticated there, never by a local command.",
				"--kind selects an authority question (stop, budget-above-norm, carry); stop and budget-above-norm take --budget BOX.",
				"question show Q shows its state and how it is answered; question wait Q waits for its answer.",
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
			},
			maxArgs:  1,
			examples: []string{"metasystem question ask verbs-match-intent --question 'Land slice 2 now?' --option 'yes: land it' --option 'no: wait for review' --recommend yes"},
			run:      runIntentAsk,
		},
		{
			object: "question", action: "retry", audience: "agent", summary: "deliver one stored, undelivered question once more",
			usage:    []string{"metasystem question retry Q"},
			details:  []string{"It never asks a new question and touches no other."},
			maxArgs:  1,
			examples: []string{"metasystem question retry q-20260925-1"},
			run: func(inv *intentInvocation) int {
				if len(inv.input.args) != 1 {
					return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "question retry needs the question: metasystem question retry Q; nothing was done"})
				}
				return runIntentAskRetry(inv, inv.input.args[0])
			},
		},
		{
			object: "question", action: "withdraw", audience: "agent", summary: "withdraw one question, with a reason",
			usage:    []string{"metasystem question withdraw Q --reason TEXT"},
			flags:    []intentFlag{reasonFlag("because", "why the question is withdrawn")},
			maxArgs:  1,
			examples: []string{"metasystem question withdraw q-20260925-1 --reason 'decided in the review'"},
			run: func(inv *intentInvocation) int {
				if len(inv.input.args) != 1 {
					return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "question withdraw needs the question: metasystem question withdraw Q --reason TEXT; nothing was done"})
				}
				return runIntentAskWithdraw(inv, inv.input.args[0])
			},
		},
		{
			object: "question", action: "answer", audience: "human", summary: "answer a question, or see where a channel question is answered",
			usage: []string{"metasystem question answer Q [TEXT]", "metasystem question answer M/Q TEXT", "metasystem question answer Q --answer-file FILE"},
			details: []string{
				"Q is a channel question or a mission's question; when both share the id, channel:Q or M/Q names one.",
				"TEXT answers a mission question: the mission records it (or changes nothing), then resumes when its requirements are satisfied.",
				"Repeating the same answer completes an interrupted resume; a different answer to an answered question is refused.",
				"A channel question is answered in its channel thread, which authenticates the person; question answer Q shows where and how.",
				"--answer-file reads the answer from FILE, exactly, less its final line end; the same answer may also be given inline.",
			},
			flags:    []intentFlag{{name: "answer-file", value: "FILE", advanced: true, usage: "read the mission answer from FILE"}},
			maxArgs:  2,
			examples: []string{"metasystem question answer host-failure 'retry: the host is back'", "metasystem question answer demo/host-failure --answer-file answer.md", "metasystem question answer q-20260925-1"},
			run:      runIntentAnswerQuestion,
		},
		{
			object: "question", action: "show", audience: "both", summary: "one question, from the channel or a mission, and how it is answered",
			usage:    []string{"metasystem question show Q"},
			maxArgs:  1,
			examples: []string{"metasystem question show q-20260925-1", "metasystem question show demo/host-failure"},
			run:      func(inv *intentInvocation) int { return runIntentShowQuestion(inv, inv.input.args) },
		},
		{
			object: "question", action: "list", audience: "both", summary: "the channel questions still open",
			usage:    []string{"metasystem question list"},
			maxArgs:  0,
			examples: []string{"metasystem question list"},
			run:      runIntentQuestionList,
		},
		{
			object: "question", action: "wait", audience: "agent", summary: "wait for a question's answer",
			usage:    []string{"metasystem question wait Q [--timeout DURATION]"},
			flags:    []intentFlag{{name: "timeout", value: "DURATION", usage: "how long this invocation waits (for example 20s or 10m)"}},
			maxArgs:  1,
			examples: []string{"metasystem question wait channel:q-20260925-1 --timeout 10m"},
			run: func(inv *intentInvocation) int {
				if len(inv.input.args) != 1 {
					return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "question wait needs the question: metasystem question wait Q; nothing was done"})
				}
				return inv.render(inv.waitQuestion(inv.input.args[0]))
			},
		},
	}
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
				Summary:  notAnInstallation(path, err),
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

func (inv *intentInvocation) checkoutTarget(scope processScope) []intentTarget {
	return []intentTarget{{Kind: "checkout", ID: scope.Checkout}}
}

// processReportResult renders a transition report: its lines, and its own
// exit code, which stays nonzero for an incomplete or unknown reading.
func processReportResult(targets []intentTarget, summary string, report stoptransition.Report) intentResult {
	return intentResult{Outcome: intentConfirmed, Targets: targets, Summary: summary, text: report.Lines, code: report.ExitCode,
		Data: map[string]any{"lines": nonNilLines(report.Lines), "exitCode": report.ExitCode}}
}

// processUnchangedResult is a repeated start or stop whose effect already
// held (R-129-ui): success, nothing written, and the owner's last line says
// what already holds.
func processUnchangedResult(targets []intentTarget, report stoptransition.Report) intentResult {
	summary := ""
	if len(report.Lines) > 0 {
		summary = report.Lines[len(report.Lines)-1]
	}
	return intentResult{Outcome: intentUnchanged, Targets: targets, Summary: summary, text: report.Lines,
		Data: map[string]any{"lines": nonNilLines(report.Lines), "exitCode": report.ExitCode, "since": report.Since}}
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

// runIntentSystemStart arms this checkout's machinery at a person's word,
// or with --if-down recovers only helpers that are down.
func runIntentSystemStart(inv *intentInvocation) int {
	if inv.input.switched("if-down") {
		for _, other := range []string{"temporary-human-word", "review-by"} {
			if inv.input.has(other) {
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("system start --if-down carries no person's word; --%s is not taken; nothing was done", other)})
			}
		}
		scope, _, problem := inv.selectProcessScope()
		if problem != nil {
			return inv.render(*problem)
		}
		return runUpWith([]string{"--metasystem-root", scope.Installation, "--repo", scope.Checkout, "--recover-only", "--if-down"}, inv.owners.processes.process.repositoryTop)
	}
	scope, scale, problem := inv.selectProcessScope()
	if problem != nil {
		return inv.render(*problem)
	}
	before, _ := processFence(scope)
	report, refusal := inv.owners.processes.process.arm(scope, scale, inv.input.text("temporary-human-word"), inv.input.text("review-by"))
	if refusal != nil {
		return inv.render(inv.startRefusal(scope, scale, before, refusal, report))
	}
	if report.Unchanged {
		return inv.render(processUnchangedResult(inv.checkoutTarget(scope), report))
	}
	return inv.render(processReportResult(inv.checkoutTarget(scope), "started "+scope.Checkout, report))
}

// runIntentSessionStart prepares the current agent session through up.
func runIntentSessionStart(inv *intentInvocation) int {
	scope, scale, problem := inv.selectProcessScope()
	if problem != nil {
		return inv.render(*problem)
	}
	owners := inv.owners.processes
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

// runIntentSystemStop stops this checkout's machinery at a person's word.
func runIntentSystemStop(inv *intentInvocation) int {
	scope, scale, problem := inv.selectProcessScope()
	if problem != nil {
		return inv.render(*problem)
	}
	report, refusal := inv.owners.processes.process.stop(scope, scale)
	if refusal != nil {
		return inv.render(processRefusalResult(inv.checkoutTarget(scope), "stop refused: ", refusal, report))
	}
	if report.Unchanged {
		return inv.render(processUnchangedResult(inv.checkoutTarget(scope), report))
	}
	if report.ExitCode != 0 {
		_, fence := processFence(scope)
		return inv.render(intentResult{Outcome: intentPartial, code: report.ExitCode, Targets: inv.checkoutTarget(scope), text: report.Lines,
			Summary: "stop did not finish: some processes are still running (listed below); no new work starts (" + fence + ")",
			next:    inv.publicArgv(append([]string{"system", "stop"}, inv.forward("installation")...)...), nextReason: "stop again; end any process that survives a second stop yourself",
			Data: map[string]any{"lines": nonNilLines(report.Lines), "exitCode": report.ExitCode, "fence": fence}})
	}
	return inv.render(processReportResult(inv.checkoutTarget(scope), "stopped "+scope.Checkout, report))
}

func (inv *intentInvocation) stopSession() int {
	by := strings.TrimSpace(inv.input.text("by"))
	if by == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: []intentTarget{{Kind: "session", ID: ""}},
			Summary: "stop session needs the attending person: --by NAME; nothing was done",
			next:    inv.publicArgv("session", "stop", "--by", "NAME"), nextReason: "at the enrolled terminal, with your name"})
	}
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	marker, refusal, code := inv.owners.processes.sessionStop(inv.stateRoot, by)
	if code != 0 {
		return inv.render(intentResult{Outcome: intentRefused, code: code, Targets: []intentTarget{{Kind: "session", ID: ""}}, Summary: refusal,
			Decision: "a person authorizes this at the enrolled terminal; the checkout keeps running either way"})
	}
	if refusal != "" {
		// The same person's authorization already holds (R-129-ui).
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: []intentTarget{{Kind: "session", ID: marker.SessionId}},
			Summary: refusal, Data: map[string]any{"sessionStop": marker}})
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
			Summary:  notAnInstallation(path, err),
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

// jobReference is the qualified public reference of a job.
func jobReference(job intentJob) string {
	if job.kind == "launch" {
		return launchJobPrefix + job.id
	}
	return dispatchJobPrefix + job.id
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
	jobs, scope, problem := inv.listJobs(all)
	if problem != nil {
		return inv.render(*problem)
	}
	views, lines := []map[string]any{}, []string{}
	for _, job := range jobs {
		ref := jobReference(job)
		ended := jobEnded(job)
		view := map[string]any{"reference": ref, "kind": job.kind, "purpose": jobPurpose(job), "status": inv.publicArgv("work", "status", ref)}
		line := fmt.Sprintf("  %s: %s", ref, jobPurpose(job))
		if !ended {
			view["wait"], view["stop"] = inv.publicArgv("work", "wait", ref), inv.publicArgv("work", "stop", ref)
			line += "; " + shellCommand(inv.publicArgv("work", "wait", ref)) + " or " + shellCommand(inv.publicArgv("work", "stop", ref))
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
		result.next, result.nextReason = inv.publicArgv("work", "status", "--all"), "also lists ended jobs"
	}
	return inv.render(result)
}

// jobEnded reports whether a launch or dispatch job has ended.
func jobEnded(job intentJob) bool {
	return (job.kind == "launch" && job.launch.State.Terminal()) || (job.kind == "dispatch" && dispatchcore.TerminalStatus(fmt.Sprint(job.dispatch["status"])))
}

// jobGoal is the goal a launch or dispatch job works for.
func jobGoal(job intentJob) string {
	if job.kind == "launch" {
		return job.launch.Goal
	}
	goalID, _ := job.dispatch["goalId"].(string)
	return goalID
}

// listJobs is this user's launches and the selected repository's dispatch
// jobs, running only unless all includes ended ones, and the scope read.
func (inv *intentInvocation) listJobs(all bool) ([]intentJob, string, *intentResult) {
	var jobs []intentJob
	var records []launch.Record
	if inv.owners.processes.launches != nil {
		listed, err := inv.owners.processes.launches().List()
		if err != nil {
			return nil, "", &intentResult{Outcome: intentFailed, code: 1, Summary: "this user's launches cannot be listed: " + err.Error()}
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
	return jobs, scope, nil
}

func (inv *intentInvocation) stopJob(ref string) int {
	job, problem := inv.resolveJob(ref, "stop")
	if problem != nil {
		return inv.render(*problem)
	}
	return inv.render(inv.stopResolvedJob(job))
}

// stopResolvedJob stops one launch or dispatch job; one already ended is
// already stopped (R-129-ui).
func (inv *intentInvocation) stopResolvedJob(job intentJob) intentResult {
	id := job.id
	targets := []intentTarget{{Kind: "job", ID: jobReference(job)}}
	if job.kind == "launch" {
		if job.launch.State.Terminal() {
			return intentResult{Outcome: intentUnchanged, Targets: targets, Summary: fmt.Sprintf("launch %s already ended: %s", id, job.launch.State),
				text: []string{launchReport(job.launch)}, Data: map[string]any{"kind": "launch", "record": job.launch}}
		}
		record, err := inv.owners.processes.launches().Cancel(id)
		if err != nil {
			return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: fmt.Sprintf("launch %s cancel: %v", id, err),
				Data: map[string]any{"kind": "launch", "record": record}}
		}
		return intentResult{Outcome: intentConfirmed, Targets: targets, Summary: fmt.Sprintf("launch %s cancelled: %s", id, record.State),
			text: []string{launchReport(record)}, Data: map[string]any{"kind": "launch", "record": record}}
	}
	if status, _ := job.dispatch["status"].(string); dispatchcore.TerminalStatus(status) {
		// A job that already ended is already stopped: the repeat is success
		// and never reaches the cancellation owner (R-129-ui).
		summary := fmt.Sprintf("%s is already stopped: %s", jobReference(job), status)
		if ended, _ := job.dispatch["endedAt"].(string); ended != "" {
			summary += " (at " + ended + ")"
		}
		return intentResult{Outcome: intentUnchanged, Targets: targets, Summary: summary, Data: map[string]any{"kind": "dispatch", "status": status}}
	}
	outcome, code, err := inv.owners.processes.cancelDispatch(inv.layout.GitRoot, id)
	if err != nil {
		return intentResult{Outcome: intentFailed, code: max(code, 1), Targets: targets, Summary: fmt.Sprintf("dispatch job %s cancel: %v", id, err)}
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
	return result
}

// runIntentCheckoutStatus is the overview of this checkout: its
// machinery, running work, open questions and attention items.
func runIntentCheckoutStatus(inv *intentInvocation) int {
	scope, scale, problem := inv.selectProcessScope()
	if problem != nil {
		return inv.render(inv.withHelm(*problem, inv.helmPath()))
	}
	report, err := inv.owners.processes.process.status(scope, scale)
	if err != nil {
		return inv.render(inv.withHelm(intentResult{Outcome: intentFailed, code: 1, Targets: inv.checkoutTarget(scope), Summary: "status is unknown: " + err.Error()}, scope.Checkout))
	}
	return inv.render(inv.withHelm(processReportResult(inv.checkoutTarget(scope), "status of "+scope.Checkout, report), scope.Checkout))
}

// runIntentSystemStatus is the checkout's machinery status, or with
// --steward the idle watchdog's own view.
func runIntentSystemStatus(inv *intentInvocation) int {
	if !inv.input.switched("steward") {
		return runIntentCheckoutStatus(inv)
	}
	scope, _, problem := inv.selectProcessScope()
	if problem != nil {
		return inv.render(*problem)
	}
	args := []string{"--repo", scope.Checkout}
	if inv.input.switched("json") {
		args = append(args, "--json")
	}
	return runStewardStatus(args)
}

// runIntentWorkStatus lists running work, or reads the one goal, job, run
// or read a reference names.
func runIntentWorkStatus(inv *intentInvocation) int {
	if inv.input.switched("history") {
		return runIntentWorkHistory(inv)
	}
	if inv.input.has("since") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--since belongs to work status --history; nothing was done"})
	}
	if len(inv.input.args) == 0 {
		if inv.input.has("work") {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--work names a goal's work: work status G --work NAME; nothing was done"})
		}
		return runIntentStatusWork(inv)
	}
	if inv.input.switched("all") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--all belongs to the list of running work (work status without a target); nothing was done"})
	}
	ref, problem := inv.resolveWorkRef(inv.input.args[0], inv.command.accepts)
	if problem != nil {
		return inv.render(*problem)
	}
	if ref.kind != refGoal && inv.input.has("work") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--work names a goal's work: work status G --work NAME; nothing was done"})
	}
	switch ref.kind {
	case refGoal:
		return runIntentStatusGoal(inv, ref.id)
	case refRead:
		return runIntentReviewRef(inv, "show", ref.id)
	case refRun:
		targets := []intentTarget{{Kind: "run", ID: ref.qualified()}}
		runner := &launch.UnitRunner{Manager: inv.owners.processes.launches()}
		record, err := runner.Status(ref.id)
		if errors.Is(err, fs.ErrNotExist) {
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: fmt.Sprintf("no unit run %s of this user; nothing was read", shellCommand([]string{ref.qualified()}))})
		}
		if err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: fmt.Sprintf("unit run %s: %v", ref.qualified(), err)})
		}
		lines := []string{}
		for _, round := range record.Rounds {
			lines = append(lines, fmt.Sprintf("round %d: %s", round.Number, round.Outcome))
		}
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, text: lines,
			Summary: fmt.Sprintf("unit run %s (%s, goal %s): %s", unitRunPrefix+record.ID, record.Unit, record.Goal, record.State), Data: map[string]any{"record": record}})
	}
	job := ref.job
	targets := []intentTarget{{Kind: "job", ID: jobReference(job)}}
	if job.kind == "launch" {
		record, err := inv.owners.processes.launches().Status(job.id)
		if err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: fmt.Sprintf("launch %s status: %v", jobReference(job), err)})
		}
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Summary: fmt.Sprintf("launch %s: %s", jobReference(job), record.State),
			text: []string{launchReport(record)}, Data: map[string]any{"kind": "launch", "record": record}})
	}
	status, _ := job.dispatch["status"].(string)
	if status == "" {
		status = "unknown (the record names no status)"
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Summary: fmt.Sprintf("dispatch job %s: %s", jobReference(job), status),
		Data: map[string]any{"kind": "dispatch", "record": job.dispatch, "recordPath": job.recordPath}})
}

// runIntentWorkStop stops exactly the job or diagnostic read a reference
// names.
func runIntentWorkStop(inv *intentInvocation) int {
	if len(inv.input.args) != 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "work stop needs the reference of the work to stop: metasystem work stop REF; nothing was done",
			next: inv.publicArgv("work", "status"), nextReason: "lists running work with its references"})
	}
	ref, problem := inv.resolveWorkRef(inv.input.args[0], inv.command.accepts)
	if problem != nil {
		return inv.render(*problem)
	}
	if ref.kind == refRead {
		return runIntentReviewRef(inv, "stop", ref.id)
	}
	if ref.kind == refGoal {
		return runIntentWorkStopGoal(inv, ref.id)
	}
	return inv.stopJob(ref.qualified())
}

// runIntentWorkStopGoal stops every running job of goal G, and no other
// goal's. A goal its budget stopped keeps its recorded stop: once none of its
// jobs runs, the stop completes by itself (the steward's next pass advances
// it), and this act advances it at once as well. A repeat with nothing
// running stops nothing and is success (R-129-ui).
func runIntentWorkStopGoal(inv *intentInvocation, id string) int {
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	projection, now, problem := inv.projection()
	if problem != nil {
		return inv.render(*problem)
	}
	targets := []intentTarget{{Kind: "goal", ID: id}}
	file, _ := goalRecord(projection, id)
	if file == nil {
		// Neither a goal nor a record of any kind work stop takes.
		return inv.render(*inv.noReference(id, inv.command.accepts))
	}
	jobs, _, problem := inv.listJobs(false)
	if problem != nil {
		return inv.render(*problem)
	}
	var stopped, failed []string
	var lines []string
	for _, job := range jobs {
		if jobGoal(job) != id || jobEnded(job) {
			continue
		}
		result := inv.stopResolvedJob(job)
		lines = append(lines, jobReference(job)+": "+result.Summary)
		if result.Outcome == intentConfirmed || result.Outcome == intentUnchanged {
			stopped = append(stopped, jobReference(job))
		} else {
			failed = append(failed, jobReference(job))
		}
	}
	data := map[string]any{"stopped": nonNilLines(stopped), "failed": nonNilLines(failed)}
	stopLine := ""
	if file.StopFence != nil {
		// The recorded stop's bookkeeping, as the steward's pass does it.
		stopID := file.StopFence.StopID
		if batch, err := dispatchcore.ReconcileStopBatch(inv.stateRoot, stopID, now); err == nil {
			data["stop"], data["stopState"] = stopID, string(batch.State)
			if batch.State == goal.StopBatchComplete {
				stopLine = "its budget stop " + stopID + " is complete; metasystem goal resume " + id + " lifts it"
			} else {
				stopLine = "its budget stop " + stopID + " completes by itself once none of its jobs runs"
			}
			lines = append(lines, stopLine)
		}
	}
	switch {
	case len(failed) > 0:
		return inv.render(intentResult{Outcome: intentPartial, code: 1, Targets: targets, Data: data, text: lines,
			Summary: fmt.Sprintf("goal %s: %d job(s) stopped, %d not stopped", id, len(stopped), len(failed)),
			next:    inv.publicArgv("work", "status", id), nextReason: "shows each of the goal's jobs and its state"})
	case len(stopped) == 0:
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: data, text: lines,
			Summary: fmt.Sprintf("no job of goal %s is running; nothing was stopped", id)})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: data, text: lines,
		Summary: fmt.Sprintf("goal %s: %d running job(s) stopped", id, len(stopped))})
}

// runIntentSystemRestart stops this checkout's machinery and, only once
// everything stopped, starts it again.
func runIntentSystemRestart(inv *intentInvocation) int {
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
			Decision: "resolve the survivors listed above, then run metasystem system restart again",
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
			next:    inv.publicArgv(append([]string{"system", "start"}, inv.forward("installation")...)...), nextReason: "start it once the refusal above is resolved",
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
			next: inv.publicArgv("system", "enroll", "--name", "NAME"), nextReason: "at an agent-free terminal, with your name"})
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
	case code == 0 && report.result != nil && report.result.Unchanged:
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Summary: report.result.Detail,
			Data: map[string]any{"enrollment": report.value, "fleetPublished": true, "owner": ownerPublication(*report.result)}})
	case code == 0 && report.result != nil:
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Summary: "this terminal is enrolled for " + name + " and the fleet cutoff is published",
			Data: map[string]any{"enrollment": report.value, "fleetPublished": true, "owner": ownerPublication(*report.result)}})
	}
	return inv.render(ownerResult(report, code, intentResult{}))
}

func runIntentAsk(inv *intentInvocation) int {
	id, problem := inv.singleTarget()
	if problem != nil {
		return inv.render(*problem)
	}
	if id == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "question ask needs the goal the question is about: " + inv.command.usage[0] + "; nothing was done", Decision: "name the goal"})
	}
	question := strings.TrimSpace(inv.input.text("question"))
	if question == "" || len(inv.input.values["option"]) == 0 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
			Summary: "question ask needs --question TEXT and at least one --option 'LABEL: CONSEQUENCE'; nothing was asked"})
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
	wait := inv.publicArgv("question", "wait", "channel:"+q.ID)
	poll := inv.publicArgv("question", "retry", q.ID)
	if errors.Is(err, errQuestionAlreadyOpen) {
		// The same question already stands open (R-129-ui): success, and
		// nothing was written, posted or published again.
		repeat := intentResult{Outcome: intentUnchanged, Targets: targets, text: lines, Data: data,
			Summary: "question " + q.ID + " is already open for goal " + id + " with this text and these options (" + delivery + "); nothing was asked again",
			next:    wait, nextReason: "wait for the authenticated answer"}
		if pending && q.Undelivered > 0 {
			repeat.next, repeat.nextReason = poll, "delivers exactly this stored question once more"
		}
		return inv.render(repeat)
	}
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
	resume := inv.publicArgv("mission", "resume", missionID)
	switch {
	case code == 0:
		return (intentResult{Outcome: intentConfirmed, Targets: targets, Summary: "answered " + askID + " of mission " + missionID, text: lines, Data: data})
	case effects.ResetRecorded && !effects.AskAnswered:
		// The reset line is authoritative; answering again is lawful and
		// completes the answer.
		return (intentResult{Outcome: intentPartial, code: code, Targets: targets, text: lines, Data: data,
			Summary: "the reset for " + askID + " is recorded, but the question could not be marked answered",
			next:    inv.publicArgv("question", "answer", missionID+"/"+askID, answer), nextReason: "answer again; a second reset is harmless"})
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
	code := verdict.ExitCode()
	covenantData := map[string]any{"present": false}
	if path, problem := checkCovenantShape(scope); path != "" {
		covenantData = map[string]any{"present": true, "path": path, "valid": problem == nil}
		if problem != nil {
			covenantData["reason"] = problem.Error()
			lines = append(lines, "covenant invalid: "+problem.Error()+"; the app's covenant must parse before any mission trusts it")
			code = max(code, 1)
		} else {
			lines = append(lines, "covenant shape valid: "+path+"; adequacy not established: shape says the rows parse, never that the proofs guard the intent")
			evidenceLines, evidence, traceable := checkCovenantEvidence(filepath.Dir(path))
			lines = append(lines, evidenceLines...)
			covenantData["evidence"] = evidence
			covenantData["traceable"] = traceable
			if !traceable {
				code = max(code, 1)
			}
		}
	}
	if layout, err := inv.owners.resolver.ResolveLayout(scope.Checkout); err == nil {
		if drift := setupDrift(layout); len(drift) > 0 {
			lines = append(lines, drift...)
			code = max(code, 1)
		}
	}
	adapters, refused := adapterReport(scope.Installation)
	lines = append(lines, adapters...)
	if refused > 0 {
		code = max(code, 1)
	}
	// Skills are where users extend the metasystem; their frontmatter and
	// naming rules are part of this checkout's health.
	skillsData := map[string]any{"valid": true}
	var skillLines strings.Builder
	if err := validate.SkillInventory(scope.Installation, &skillLines); err != nil {
		skillsData = map[string]any{"valid": false, "reason": err.Error()}
		lines = append(lines, "skills invalid: "+err.Error()+"; fix that skill's SKILL.md (its name and description frontmatter)")
		code = max(code, 1)
	}
	result := intentResult{Outcome: intentConfirmed, code: code, Targets: inv.checkoutTarget(scope),
		Summary: verdict.LineWithoutRemedies(), text: lines, Data: additiveData(steward.NewHookHealthPreview(verdict), map[string]any{"publicRemedies": remedies, "covenant": covenantData, "adapters": adapters, "skills": skillsData})}
	if first != nil {
		result.next, result.nextReason = first, "the first public remedy check found"
	}
	return inv.render(inv.withHelm(result, scope.Checkout))
}

// runIntentWorkHistory reports how launches ended and why any was refused:
// every launch, one goal's, or one launch (j1:ID), through the launch
// report owner.
func runIntentWorkHistory(inv *intentInvocation) int {
	for _, other := range []string{"all", "work"} {
		if inv.input.has(other) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("work status --history takes a goal or j1:ID and --since, not --%s; nothing was done", other)})
		}
	}
	var args []string
	if len(inv.input.args) == 1 {
		switch kind, id := splitReference(inv.input.args[0]); kind {
		case refJ1:
			if inv.input.has("since") {
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--since narrows a goal's or every launch's history, not one launch's; nothing was done"})
			}
			args = []string{"--id", id}
		case "":
			args = []string{"--goal", inv.input.args[0]}
		default:
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("work status --history reads launches: a goal or j1:ID, not %s; nothing was done", inv.input.args[0])})
		}
	}
	if inv.input.has("since") {
		args = append(args, "--since", inv.input.text("since"))
	}
	if inv.input.switched("json") {
		args = append(args, "--json")
	}
	return runLaunchReport(args)
}

// checkCovenantShape checks the app covenant's shape when the checkout has
// one, at its installation or its repository root: the path read, and the
// reason it does not parse. Shape is all it proves, never adequacy.
func checkCovenantShape(scope processScope) (string, error) {
	for _, dir := range []string{scope.Installation, scope.Checkout} {
		if dir == "" {
			continue
		}
		path := filepath.Join(dir, covenant.Filename)
		if _, err := os.Lstat(path); err != nil {
			continue
		}
		_, err := covenant.Load(path)
		return path, err
	}
	return "", nil
}

// setupDrift is what system setup would change in this checkout, each item
// ending in the act that repairs it: runtime registrations that differ from
// the ones metasystem.runtimes enables, hooks that do not run the engine,
// and a testing contract that does not merge through the engine's driver.
func setupDrift(layout stateroot.Layout) []string {
	repair := "; metasystem system setup repairs it"
	var drift []string
	selected := hookswitch.ConfiguredRuntimes(layout.InstallationRoot)
	check := func(options hostsetup.Options) error {
		_, err := hostsetup.SetupWithResolver(options, func(string) (stateroot.Layout, error) { return layout, nil })
		return err
	}
	registration := hostsetup.Options{RepositoryPath: layout.RepositoryRoot, Runtimes: selected, Check: true}
	if err := check(registration); err != nil {
		registration.CopySkills = true
		if copyErr := check(registration); copyErr != nil {
			drift = append(drift, "runtime registrations differ: "+err.Error()+repair)
		}
	}
	if registered := hookswitch.RegisteredRuntimes(layout.RepositoryRoot); len(registered) > 0 {
		if err := check(hostsetup.Options{RepositoryPath: layout.RepositoryRoot, Runtimes: registered, HooksOnly: true, Check: true}); err != nil {
			drift = append(drift, "hooks do not run the engine: "+err.Error()+repair)
		}
	}
	engine, err := hooks.DirectEngine(layout.InstallationRoot, hookswitch.Git)
	if err != nil {
		return append(drift, "the engine the hooks would run cannot be found: "+err.Error())
	}
	if _, gitErr := hookswitch.Git("-C", layout.RepositoryRoot, "rev-parse", "--git-dir"); gitErr == nil {
		missing, err := contractgit.Registration(layout.RepositoryRoot, hookswitch.TestingContract(layout), engine, hookswitch.Git)
		if err != nil {
			missing = []string{err.Error()}
		}
		for _, item := range missing {
			drift = append(drift, "testing-contract merges: "+item+repair)
		}
	}
	return drift
}

// checkCovenantEvidence runs the covenant's traceability gate at root, the
// directory holding covenant.json: every requirement backed by a row of
// docs/covenant-evidence.md whose declared dependencies are present. The
// statuses the rows record are claims on file, not re-verified here.
func checkCovenantEvidence(root string) ([]string, *evidencetable.Report, bool) {
	cov, err := covenant.Load(filepath.Join(root, covenant.Filename))
	if err != nil {
		return []string{"covenant evidence not judged: " + err.Error()}, nil, false
	}
	rootFD, err := evidencetable.OpenRoot(root)
	if err != nil {
		return []string{"covenant evidence not judged: " + err.Error()}, nil, false
	}
	defer unix.Close(rootFD)
	table, err := evidencetable.LoadTable(rootFD, root)
	if err != nil {
		return []string{"covenant evidence not judged: " + err.Error() + "; the table is docs/covenant-evidence.md"}, nil, false
	}
	report := evidencetable.Judge(cov, table, rootFD)
	var lines []string
	for _, refusal := range report.Refusals {
		lines = append(lines, fmt.Sprintf("covenant evidence refused %s: %s", refusal.Kind, refusal.Detail))
	}
	for _, pair := range report.Pairs {
		line := fmt.Sprintf("requirement %s (proof %s): %s", pair.ID, pair.Proof, pair.Verdict)
		if pair.Assessment != "" {
			line += fmt.Sprintf(" [%s: %s]", pair.Status, pair.Assessment)
		}
		lines = append(lines, line)
	}
	for _, orphan := range report.Orphans {
		line := fmt.Sprintf("orphan row %s %q (proof %s, %s)", orphan.CriterionID, orphan.Criterion, orphan.Proof, orphan.Status)
		if len(orphan.Notes) > 0 {
			line += " — " + strings.Join(orphan.Notes, "; ")
		}
		lines = append(lines, line)
	}
	for _, note := range report.Notes {
		lines = append(lines, "note: "+note)
	}
	if report.Outcome != "traceable" {
		return append(lines, fmt.Sprintf("covenant evidence refused: %s (%d refusal(s))", report.App, len(report.Refusals))), report, false
	}
	return append(lines, fmt.Sprintf("covenant evidence traceable: %s (%d requirement(s), wired %d, floating %d); recorded statuses are claims on file, not re-verified here",
		report.App, len(report.Pairs), report.Counts.DerivedWired, report.Counts.DerivedFloating)), report, true
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
		steward.RoleCensusFreshness, steward.RoleHookFreshness, steward.RoleSessionMain:
		if stopped {
			return []string{"metasystem", "system", "start"}, ""
		}
		return []string{"metasystem", "session", "start"}, ""
	case steward.RoleLedgerAttention:
		return []string{"metasystem", "goal", "list"}, ""
	case steward.RoleNonterminalJobs:
		return nil, "metasystem work stop j2:JOB records a job whose process is gone as ended; metasystem status lists the work"
	case steward.RoleRetroDebt:
		return nil, "run the retro and record its receipt"
	case steward.RoleTrunkRed:
		if strings.Contains(role.Reason, "cadence") {
			// The landing owner records the deep validation cadence on its
			// own runs.
			return []string{"metasystem", "system", "start"}, ""
		}
		return []string{"metasystem", "incident", "list"}, ""
	case steward.RoleCapabilitySnapshots:
		return nil, "the next delegated job for each runtime named probes it and records a fresh snapshot; nothing needs doing now"
	case steward.RoleSpendFence:
		return nil, "a person raises the spend ceiling in metasystem.conf"
	case steward.RoleProofAttempts:
		return []string{"metasystem", "test", "run"}, ""
	}
	return nil, reasonRemedy(role.Reason)
}

// reasonRemedy is the instruction for a role whose reason is its own
// remedy: the command it names, or the change it names.
func reasonRemedy(reason string) string {
	if strings.Contains(reason, "run ") {
		return "run the command the reason above names"
	}
	return "a person changes what the reason above names; no metasystem command does it"
}

// publicRemedyForFact is the public act for one typed cause.
func publicRemedyForFact(fact steward.RemedyFact) ([]string, string) {
	switch fact.Cause {
	case steward.CauseBudgetMissing, steward.CauseBudgetMalformed:
		return []string{"metasystem", "goal", "budget", fact.Goal, "BOX"}, ""
	case steward.CauseBudgetBreach:
		return []string{"metasystem", "goal", "budget", fact.Goal, "BOX"}, "goal " + fact.Goal + " is over its box; its stop runs by itself, and a person may give it a larger box"
	case steward.CauseBudgetUnknown:
		return nil, "a person repairs record " + fact.Record + ", which cannot be read as a budget, then runs metasystem system check"
	case steward.CauseBreachStopOpen:
		return nil, "goal " + fact.Goal + "'s budget stop " + fact.Stop + " completes by itself on the steward's next pass; nothing needs doing"
	case steward.CauseBreachStopUnresolved:
		return nil, "a person inspects budget stop " + fact.Stop + " of goal " + fact.Goal + " and its job records; new work stays fenced until it resolves"
	case steward.CauseEpochMismatch:
		return []string{"metasystem", "session", "start"}, ""
	case steward.CauseForeignLineage:
		return []string{"metasystem", "goal", "release", fact.Goal}, "the session that claimed goal " + fact.Goal + " releases it, or a person takes it over: metasystem goal claim " + fact.Goal + " --take-over --reason TEXT"
	case steward.CauseStopCapabilityMissing:
		return nil, "a person repairs goal " + fact.Goal + "'s record (" + fact.Record + "), which has no stop capability"
	}
	return nil, "a person changes what the reason above names; no metasystem command does it"
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
			next:    []string{"metasystem", "ui", "start"}, nextReason: "start it once the problem above is fixed"})
	case restart.Stop == lifecycle.Timeout:
		return inv.render(intentResult{Outcome: intentPartial, code: max(result.Code, 1), Targets: targets, text: result.Lines, Data: data,
			Summary: "the interface was asked to stop but is still running, so it was not started again",
			next:    []string{"metasystem", "ui", "restart"}, nextReason: "try again once it has stopped"})
	}
	return inv.render(intentResult{Outcome: intentRefused, code: max(result.Code, 1), Targets: targets, text: result.Lines, Data: data,
		Summary: "the interface could not be restarted; nothing was changed"})
}

type uiIntentOptions struct {
	listen      string
	listenSet   bool
	waitSeconds int64
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
		next:    inv.publicArgv(append([]string{"system", "start"}, inv.forward("installation")...)...), nextReason: "start again once the problem above is fixed"}
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
		result.Decision = "repair the channel question storage at " + shellCommand([]string{questionFile}) + ", then read the question with " + shellCommand(inv.publicArgv("question", "show", "channel:"+q.ID))
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
		result.next = inv.publicArgv("question", "retry", q.ID)
		result.nextReason = "delivers exactly this stored question once more"
	}
	return result
}

// runIntentAnswerQuestion is answer Q [TEXT]: a mission question is
// answered and resumed through the mission owners; a channel question shows
// its authenticated reply location and records nothing.
func runIntentAnswerQuestion(inv *intentInvocation) int {
	args := inv.input.args
	if len(args) == 0 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "needs the question: metasystem question answer Q [TEXT]; nothing was answered",
			next: inv.publicArgv("question", "list"), nextReason: "lists the open questions with their ids"})
	}
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
			next: inv.publicArgv("question", "show", q.publicName()), nextReason: "the question and its options"})
	}
	if q.ask["answeredAt"] != nil {
		recorded, _ := q.ask["answer"].(string)
		if recorded != text {
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: []intentTarget{{Kind: "question", ID: q.publicName()}},
				Summary: fmt.Sprintf("mission %s's question %s is already answered with a different answer; nothing was changed", q.mission, q.id)})
		}
		// The same answer again completes what an interrupted call left;
		// when the mission already runs, nothing is left and the repeat is
		// unchanged (R-129-ui).
		return inv.render(inv.resumeMission(q.mission, intentResult{Outcome: intentUnchanged, Summary: "the answer to " + q.id + " was already recorded"}))
	}
	answered := inv.answerMission(q.mission, q.id, text)
	if answered.Outcome != intentConfirmed && answered.Outcome != intentPartial {
		return inv.render(answered)
	}
	if data, _ := answered.Data.(map[string]any); answered.Outcome == intentPartial && data["askAnswered"] == false {
		// Only the reset is recorded: the same answer completes it, and the
		// mission is not resumed on a question still open.
		if effects, ok := data["effects"].(missionrunner.AnswerEffects); !ok || !effects.AskAnswered {
			return inv.render(answered)
		}
	}
	return inv.render(inv.resumeMission(q.mission, answered))
}

// resumeMission asks the mission runner to resume after an answer; the
// runner decides whether that is lawful now.
func (inv *intentInvocation) resumeMission(mission string, answered intentResult) intentResult {
	ran := inv.missionOwnerLaunch(mission, "resume")
	resumed := ownerVerbResult(ran, append(answered.Targets, intentTarget{Kind: "mission", ID: mission}), answered.Summary+"; mission "+mission+" resumed", nil)
	resumed.Data = mergeData(map[string]any{"answer": answered.Data}, resumed.Data)
	if already, running := missionAlreadyRunning(ran); running {
		resumed.Summary = answered.Summary + "; " + already
		if answered.Outcome == intentUnchanged {
			resumed.Outcome = intentUnchanged
		}
	}
	if resumed.Outcome != intentConfirmed && resumed.Outcome != intentUnchanged {
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
	var ran intentProcessResult
	if verb == "status" {
		root := inv.stateRoot
		ran = ownerCall(func(stdout, stderr io.Writer) int {
			return inv.ownerCalls().missionStatus(stdout, stderr, root, mission)
		})
	} else {
		ran = inv.missionOwnerLaunch(mission, verb)
	}
	done := map[string]string{"start": "mission " + mission + " started", "resume": "mission " + mission + " resumed", "status": "mission " + mission + " status"}[verb]
	result := ownerVerbResult(ran, []intentTarget{{Kind: "mission", ID: mission}}, done, nil)
	if already, running := missionAlreadyRunning(ran); running && verb != "status" {
		result.Outcome, result.Summary = intentUnchanged, already
	}
	if verb == "status" && ran.err == nil {
		result.Outcome, result.code = intentConfirmed, 0
		result.Summary = strings.TrimSpace(string(ran.stdout))
		if result.Summary == "" {
			result.Summary = done
		}
		// EM-08: a mission with no state here is not a status record; exit
		// 0 would tell a script the mission exists.
		if strings.Contains(result.Summary, " status=unreadable reason=missing-state") {
			result = inv.missionStatusWithoutState(mission)
		}
	}
	return inv.render(result)
}

// missionStatusWithoutState refuses the status of a mission that has no
// runner state in this repository: never started when its contract is
// there, unknown otherwise. Nothing is read or changed.
func (inv *intentInvocation) missionStatusWithoutState(mission string) intentResult {
	targets := []intentTarget{{Kind: "mission", ID: mission}}
	for _, root := range []string{inv.stateRoot, inv.layout.GitRoot} {
		if root == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, "plans", "mission-"+mission+".contract.md")); err == nil {
			result := intentResult{Targets: targets, Outcome: intentRefused, code: 1,
				Summary: "mission " + mission + " has a contract but was never started; nothing was read"}
			result.next, result.nextReason = inv.publicArgv("mission", "start", mission), "starts the mission its contract describes"
			return result
		}
	}
	return intentResult{Targets: targets, Outcome: intentRefused, code: 1,
		Summary: "no mission " + mission + " in this repository; nothing was read"}
}

// missionOwnerLaunch starts or resumes one mission through the runner in
// this process (design 6.2); this process is the caller a closed fence's
// reopening classifies.
func (inv *intentInvocation) missionOwnerLaunch(mission, mode string) intentProcessResult {
	caller, root, wait := currentProcessIdentity(), inv.stateRoot, inv.input.switched("wait")
	return ownerCall(func(stdout, stderr io.Writer) int {
		return inv.ownerCalls().missionLaunch(caller, stdout, stderr, root, mission, mode, wait)
	})
}

// missionAlreadyRunning reads a start or resume that found the mission's
// runner already live: the runner printed one line saying so and exited 0.
func missionAlreadyRunning(ran intentProcessResult) (string, bool) {
	if ran.err != nil || ran.code != 0 {
		return "", false
	}
	for _, line := range nonEmptyLines(string(ran.stdout)) {
		if strings.HasPrefix(line, missionrunner.AlreadyRunningPrefix) {
			return line, true
		}
	}
	return "", false
}

// runIntentMissionNamed starts, resumes or reads the one named mission.
// missionWaitFlag runs a started or resumed mission in this terminal until
// it ends, instead of on its own.
var missionWaitFlag = intentFlag{name: "wait", usage: "run the mission here until it ends instead of on its own"}

// runIntentMissionSeal checks and seals one mission contract through the
// contract owner in this process. A sealed contract is unchanged (R-129).
func runIntentMissionSeal(inv *intentInvocation) int {
	if len(inv.input.args) != 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "mission seal needs the mission: metasystem mission seal M; nothing was done"})
	}
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	named := inv.input.args[0]
	path := inv.callerPath(named)
	if missionIDRe.MatchString(named) {
		if _, err := os.Stat(path); err != nil {
			path = filepath.Join(inv.layout.GitRoot, "plans", "mission-"+named+".contract.md")
		}
	}
	targets := []intentTarget{{Kind: "contract", ID: path}}
	digest, warnings, err := inv.ownerCalls().missionSeal(path)
	switch {
	case errors.Is(err, contract.ErrAlreadySealed):
		return inv.render(intentResult{Targets: targets, Outcome: intentUnchanged, Summary: "the contract is already sealed; nothing changed: " + path})
	case err != nil:
		return inv.render(intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: "the contract was not sealed: " + err.Error() + "; nothing was changed",
			nextReason: "fix the contract and run metasystem mission seal again; a contract that already carries an approval line is sealed before the line is added"})
	}
	lines := make([]string, 0, len(warnings)+1)
	for _, warning := range warnings {
		lines = append(lines, "warning: "+warning)
	}
	lines = append(lines, "sign it: add the line  Approval: name=NAME; date=YYYY-MM-DD; contract-sha256="+digest+"  and commit the contract")
	return inv.render(intentResult{Targets: targets, Outcome: intentConfirmed, text: lines,
		Summary: "mission contract sealed: " + path + " (contract-sha256=" + digest + ")",
		Data:    map[string]any{"contract": path, "contractSha256": digest, "warnings": nonNilLines(warnings)}})
}

func runIntentMissionNamed(inv *intentInvocation, verb string) int {
	if len(inv.input.args) != 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("mission %s needs the mission: metasystem mission %s M; nothing was done", verb, verb)})
	}
	return runIntentMission(inv, verb, inv.input.args[0])
}

// runIntentQuestionList lists the open channel questions of this
// repository through the channel's own question walk.
func runIntentQuestionList(inv *intentInvocation) int {
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	questions, unreadable := channel.WalkOpenQuestions(inv.stateRoot)
	lines, views := []string{}, []map[string]any{}
	for _, q := range questions {
		first := ""
		if len(q.Facts) > 0 {
			first = q.Facts[0]
		}
		lines = append(lines, fmt.Sprintf("  channel:%s  %s  goal %s  %s", q.ID, q.State, q.Goal, first))
		views = append(views, map[string]any{"reference": "channel:" + q.ID, "state": q.State, "goal": q.Goal, "question": first,
			"show": inv.publicArgv("question", "show", "channel:"+q.ID)})
	}
	for _, problem := range unreadable {
		lines = append(lines, "  unreadable: "+problem)
	}
	result := intentResult{Outcome: intentConfirmed, text: lines, Data: map[string]any{"questions": views, "unreadable": nonNilLines(unreadable)},
		Summary: fmt.Sprintf("%d open channel question(s)", len(questions))}
	if len(unreadable) > 0 {
		result.Outcome, result.code = intentPartial, 1
	}
	return inv.render(result)
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
	if ran.Unchanged && ran.Result.Code == 0 {
		summary := map[string]string{"start": "the interface already runs", "stop": "the interface is already stopped"}[verb]
		if len(ran.Result.Lines) > 0 {
			summary = ran.Result.Lines[0]
		}
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: data, text: ran.Result.Lines, Summary: summary})
	}
	if ran.Result.Code == 0 {
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: data, text: ran.Result.Lines,
			Summary: map[string]string{"start": "the interface is started", "stop": "the interface is stopped"}[verb]})
	}
	return inv.render(intentResult{Outcome: intentRefused, code: ran.Result.Code, Targets: targets, Data: data, text: ran.Result.Lines,
		Summary: "the interface did not " + verb + "; its report is below", next: inv.publicArgv("ui", "status"), nextReason: "what the interface is doing now"})
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
	if owner, _ := result.Data.(map[string]any)["owner"].(map[string]any); result.Outcome == intentConfirmed && owner["alreadyLaunched"] == true {
		result.Outcome = intentUnchanged
		result.Summary = fmt.Sprintf("machine %s is already launched (launch %v, %v at %v)", name, owner["launch"], owner["outcome"], owner["destination"])
	}
	if result.Outcome != intentConfirmed && result.Outcome != intentUnchanged {
		result.next, result.nextReason = inv.publicArgv("machine", "list"), "every machine's presence"
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

// runIntentSystemSetup switches this checkout's runtime hooks to the engine's
// direct `internal hook` command and enrolls its pre-commit fence
// (plans/designs/verbs-object-action.md 3.3, U9 activation).
func runIntentSystemSetup(inv *intentInvocation) int {
	layout, installation, _, problem := inv.selectInstallation()
	if problem != nil {
		return inv.render(*problem)
	}
	deps := hookswitch.Production()
	deps.Resolve = inv.owners.resolver.ResolveLayout
	if inv.owners.hookSwitch != nil {
		deps = inv.owners.hookSwitch(deps)
	}
	var options hookswitch.Options
	if inv.input.has("runtimes") {
		if strings.TrimSpace(inv.input.text("runtimes")) == "" {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--runtimes names runtimes, or none; nothing was done"})
		}
		options.Runtimes = strings.Split(inv.input.text("runtimes"), ",")
	}
	options.CopySkills = inv.input.switched("copy-skills")
	report, err := hookswitch.Setup(installation, options, deps)
	targets := []intentTarget{{Kind: "checkout", ID: layout.GitRoot}}
	if err != nil {
		var refusal *hookswitch.RefusalError
		if errors.As(err, &refusal) {
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: refusal.Reason, Decision: refusal.Remedy})
		}
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets,
			Summary: "the hooks were not switched to the engine: " + err.Error(), Decision: "diagnose the checkout with: metasystem system check"})
	}
	lines := []string{fmt.Sprintf("the engine at %s serves the hook entry", report.Engine)}
	changed := map[string]bool{}
	for _, path := range report.Changed {
		changed[path] = true
		lines = append(lines, "written: "+path)
	}
	if len(report.Runtimes) == 0 {
		lines = append(lines, "no agent runtime is registered in this checkout")
	} else if len(report.Changed) == 0 {
		lines = append(lines, "hooks already run the engine: "+strings.Join(report.Runtimes, ", "))
	}
	switch report.Fence {
	case hookswitch.FenceReenrolled:
		lines = append(lines, "pre-commit fence re-enrolled: the hook from before the engine guard now runs the engine ("+report.FenceHook+")")
	case hookswitch.FenceEnrolled:
		lines = append(lines, "pre-commit fence enrolled: "+report.FenceHook)
	case hookswitch.FenceNoGit:
		lines = append(lines, "no git repository: no pre-commit fence to enroll")
	default:
		lines = append(lines, "pre-commit fence already runs the engine: "+report.FenceHook)
	}
	switch report.MergeDriver {
	case contractgit.DriverRegistered:
		lines = append(lines, "the testing contract merges through the engine's merge driver")
	case contractgit.DriverUnchanged:
		lines = append(lines, "the testing contract's merge driver is already registered")
	}
	outcome, summary := intentConfirmed, "this checkout is set up: its runtimes, hooks, commit fence and testing-contract merges run the engine"
	if report.Unchanged() {
		outcome, summary = intentUnchanged, "this checkout is already set up; nothing was changed"
	}
	return inv.render(intentResult{Outcome: outcome, Targets: targets, Summary: summary, text: lines,
		Data: map[string]any{"engine": report.Engine, "runtimes": nonNilLines(report.Runtimes), "changed": nonNilLines(report.Changed), "fence": report.Fence, "fenceHook": report.FenceHook, "mergeDriver": report.MergeDriver}})
}
