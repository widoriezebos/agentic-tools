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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostsetup"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/covenant"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hookswitch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	processidentity "github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stoptransition"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
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
				"This user's running launches of the checkout, in it or in one of its registered worktrees, are cancelled as work stop cancels them, a line each.",
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
			object: "system", action: "completion", audience: "human", summary: "print the shell script that makes Tab complete metasystem commands",
			usage: []string{"metasystem system completion zsh|bash"},
			details: []string{
				"Tab then completes objects, actions, options, choices, file names and the open goals of the checkout the command acts on.",
				"Each Tab asks the binary as you typed it, so every checkout answers with its own commands and goals; the script reads nothing and embeds no table.",
				"zsh: add eval \"$(PATH/bin/metasystem system completion zsh)\" to ~/.zshrc after compinit. bash: the same with bash, in ~/.bashrc.",
			},
			maxArgs:  1,
			examples: []string{"metasystem system completion zsh", "metasystem system completion bash"},
			run:      runIntentSystemCompletion,
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
					return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "mission repair needs the mission's name, so nothing was done",
						next: inv.retryWith(nil, "MISSION"), nextReason: "with the mission's name"})
				}
				return runIntentRepairMission(inv, inv.input.args[0])
			},
		},
		{
			object: "ui", action: "start", audience: "both", summary: "start the browser interface",
			usage:   []string{"metasystem ui start [--listen ADDRESS]"},
			details: []string{"When this seat runs no interface, names the machine of this computer that does."},
			flags:   []intentFlag{intentUIListenFlag, intentInstallationFlag}, maxArgs: 0,
			examples: []string{"metasystem ui start"}, run: func(inv *intentInvocation) int { return inv.uiTarget("start") },
		},
		{
			object: "ui", action: "stop", audience: "both", summary: "stop the browser interface",
			usage:   []string{"metasystem ui stop [--wait-seconds N]"},
			details: []string{"When this seat runs no interface and exactly one other machine of this computer does, stop stops that one and says which; when several do, it lists each with its --repo command and stops none."},
			flags:   []intentFlag{intentUIWaitFlag, intentInstallationFlag}, maxArgs: 0,
			examples: []string{"metasystem ui stop"}, run: func(inv *intentInvocation) int { return inv.uiTarget("stop") },
		},
		{
			object: "ui", action: "restart", audience: "both", summary: "restart the browser interface with the checkout's own engine",
			usage:    []string{"metasystem ui restart [--listen ADDRESS] [--wait-seconds N]"},
			details:  []string{"An agent may do this within its authorization. Starting the interface through an agent does not authenticate a person for its human actions."},
			flags:    []intentFlag{intentUIListenFlag, intentUIWaitFlag, intentInstallationFlag},
			maxArgs:  0,
			examples: []string{"metasystem ui restart"}, run: func(inv *intentInvocation) int { return inv.runUIVerb("restart") },
		},
		{
			object: "ui", action: "status", audience: "both", summary: "whether the browser interface runs, and where",
			usage:   []string{"metasystem ui status"},
			details: []string{"When this seat runs no interface, names the machine of this computer that does."},
			flags:   []intentFlag{intentInstallationFlag}, maxArgs: 0,
			examples: []string{"metasystem ui status"}, run: func(inv *intentInvocation) int { return inv.uiTarget("status") },
		},
		{
			object: "machine", action: "list", audience: "both", summary: "every machine's presence, this one first, and what MetaSystem runs on this computer",
			usage: []string{"metasystem machine list [--refresh] [--verbose]"},
			details: []string{
				"The first line counts this computer's machines, running and stopped, its running jobs and the machines on other computers; one line per machine of the fleet follows.",
				"This computer's machines are the fleet's: of the checkouts the host registry of armed checkouts records and this checkout, those with a machine nickname that are armed now or appear in the fleet's presence; and the landing lane's checkout. The registry's other registrations (test beds, scratch clones) are not machines; --verbose counts them. A registry that cannot be read is reported, never read as no machines.",
				"--verbose adds each machine of this computer with its checkout, its helpers with pid and start in local time, its running jobs and launches and the landing lane's agent; each machine on another computer with its last report; and the processes that are not MetaSystem's, which nothing touches.",
				"Each checkout is read exactly as metasystem system status reads it.",
			},
			flags: []intentFlag{{name: "refresh", aliases: []string{"fetch"}, usage: "fetch presence now instead of the last copy"},
				intentVerboseFlag},
			maxArgs:  0,
			examples: []string{"metasystem machine list", "metasystem machine list --verbose", "metasystem machine list --refresh"},
			run:      runIntentMachineList,
		},
		{
			object: "machine", action: "stop", audience: "human", summary: "stop MetaSystem on one machine of this computer, or on every one",
			usage: []string{"metasystem machine stop NAME", "metasystem machine stop --all"},
			details: []string{
				"A person's act at their enrolled terminal, proved as metasystem system stop proves it. Each machine stops through system stop itself, the landing lane's checkout included; that stop cancels the checkout's running dispatch jobs as work stop does.",
				"This user's running launches of a stopped checkout, in it or in one of its registered worktrees, are cancelled as work stop cancels them; with --all, every running launch of this user is.",
				"NAME is a machine's nickname, its checkout's directory name or its checkout's path. A machine on another computer is stopped on that computer: metasystem system stop --repo PATH.",
				"A machine already stopped is success. Processes that are not MetaSystem's are never touched. Summary by default; --verbose prints each machine's stop.",
			},
			flags:    []intentFlag{{name: "all", usage: "every machine of this computer"}, intentVerboseFlag},
			maxArgs:  1,
			examples: []string{"metasystem machine stop --all", "metasystem machine stop m1e"},
			run:      runIntentMachineStop,
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
			run:      runIntentStartMachine,
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
			usage: []string{"metasystem question ask G --question TEXT --option TEXT...", "metasystem question ask G --question TEXT --option 'LABEL: CONSEQUENCE'... [--recommend LABEL]",
				"metasystem question ask --about lane|machine --question TEXT --option TEXT..."},
			details: []string{
				"The person answers in the channel thread; the answer is authenticated there, never by a local command.",
				"--about lane or --about machine asks about the landing lane or this machine when no goal is involved; such a question is an ordinary question and carries no authority kind.",
				"--kind selects an authority question (stop, budget-above-norm, carry); stop and budget-above-norm take --budget BOX.",
				"question show Q shows its state and how it is answered; question wait Q waits for its answer.",
				"It reaches a person, never another agent; to ask the agent on another seat of this host, or whoever works on a goal, run metasystem agent ask.",
			},
			flags: []intentFlag{
				intentTargetFlag,
				{name: "question", value: "TEXT", usage: "the question, the first line the person reads"},
				fileFlag("question", "read the question from FILE"),
				{name: "about", value: "lane|machine", usage: "with no goal: what the question is about"},
				{name: "option", value: "TEXT", repeat: true, usage: "one answer, as 'label: consequence' (repeatable)"},
				{name: "option-file", value: "FILE", repeat: true, advanced: true, usage: "read one answer from each FILE; answers keep the order given (repeatable)"},
				{name: "recommend", value: "LABEL", usage: "the option you recommend"},
				{name: "fact", value: "TEXT", repeat: true, advanced: true, usage: "a fact the person needs (repeatable)"},
				{name: "fact-file", value: "FILE", repeat: true, advanced: true, usage: "read one fact from each FILE; facts keep the order given (repeatable)"},
				{name: "kind", value: "KIND", advanced: true, usage: "an authority question: stop, budget-above-norm or carry"},
				{name: "wants", value: "TOKEN", advanced: true, usage: "the exact answer token (carry questions)"},
				{name: "budget", value: "BOX", advanced: true, usage: "the proposed compact box for stop and budget-above-norm"},
			},
			maxArgs: 1,
			examples: []string{"metasystem question ask verbs-match-intent --question 'Land slice 2 now?' --option 'yes: land it' --option 'no: wait for review' --recommend yes",
				"metasystem question ask --about lane --question 'Return the conflicting branch?' --fact 'merge conflict in internal/goal' --option 'return: its seat fixes it'"},
			run: runIntentAsk,
		},
		{
			object: "question", action: "retry", audience: "agent", summary: "deliver one stored, undelivered question once more",
			usage:    []string{"metasystem question retry Q"},
			details:  []string{"It never asks a new question and touches no other."},
			maxArgs:  1,
			examples: []string{"metasystem question retry q-20260925-1"},
			run:      runIntentQuestionRetry,
		},
		{
			object: "question", action: "withdraw", audience: "agent", summary: "withdraw one question, with a reason",
			usage:    []string{"metasystem question withdraw Q --reason TEXT"},
			flags:    []intentFlag{reasonFlag("because", "why the question is withdrawn")},
			maxArgs:  1,
			examples: []string{"metasystem question withdraw q-20260925-1 --reason 'decided in the review'"},
			run:      runIntentQuestionWithdraw,
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
			run:      runIntentQuestionShow,
		},
		{
			object: "question", action: "list", audience: "both", summary: "the channel questions still open, or those answered or withdrawn",
			usage: []string{"metasystem question list", "metasystem question list --answered [--since 7d] [--verbose]"},
			details: []string{
				"--answered reads the questions that ended, answered or withdrawn, grouped by the refusal that made an agent ask (its first fact), most frequent first, with counts and the median time to answer.",
			},
			flags: []intentFlag{{name: "answered", usage: "the questions answered or withdrawn, grouped by refusal"},
				{name: "since", value: "AGE", usage: "with --answered: questions asked within this age (7d, 36h or 90m)"}, intentVerboseFlag},
			maxArgs:  0,
			examples: []string{"metasystem question list", "metasystem question list --answered --since 7d"},
			run:      runIntentQuestionList,
		},
		{
			object: "question", action: "wait", audience: "agent", summary: "wait for a question's answer",
			usage:    []string{"metasystem question wait Q [--timeout DURATION]"},
			flags:    []intentFlag{{name: "timeout", value: "DURATION", usage: "how long this invocation waits (for example 20s or 10m)"}},
			maxArgs:  1,
			examples: []string{"metasystem question wait channel:q-20260925-1 --timeout 10m"},
			run:      runIntentQuestionWait,
		},
	}
}

// processIntentOwners are the owners the process and question commands call.
type processIntentOwners struct {
	process        processOwners
	adoptionReads  *claimAdoptionReads
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
	mission        func(root string, installation stateroot.Installation, mission string) (*missionrunner.Engine, error)
	channelLink    func(root string) (channel.Provider, channel.DestinationConfig)
	ui             func(verb string, roots lifecycle.Roots, options uiIntentOptions) (uiLifecycleResult, error)
	executable     func() (string, error)
}

func defaultProcessIntentOwners() processIntentOwners {
	return processIntentOwners{
		process: defaultProcessOwners(),
		up:      up.Run,
		health: func(repo, installation string, now time.Time) steward.HealthVerdict {
			return steward.PreviewInstalledHealth(repo, installation, now, nil)
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
			return stateroot.Layout{}, "", false, inv.notARepository(path, err)
		}
		return layout, layout.InstallationRoot.Path(), false, nil
	}
	named := inv.input.text("installation")
	if !filepath.IsAbs(named) {
		named = filepath.Join(inv.cwd, named)
	}
	installation, err := canonicalPath(named)
	if err != nil {
		return stateroot.Layout{}, "", false, &intentResult{Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("the installation %s cannot be found, so nothing was done", shellCommand([]string{named})),
			next:    inv.retryWith([]string{"installation"}), nextReason: "the repository's own installation is found without --installation",
			Details: []string{"--installation: " + err.Error()}}
	}
	owned, ownedErr := inv.owners.resolver.ResolveLayout(installation)
	if ownedErr != nil || !sameCanonicalPath(owned.InstallationRoot.Path(), installation) {
		return stateroot.Layout{}, "", false, &intentResult{Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("%s is not a metasystem installation; nothing was done", shellCommand([]string{installation})),
			next:    inv.retryWith([]string{"installation"}), nextReason: "the repository's own installation is found without --installation",
			Details: []string{"--installation names the directory that holds the installation's metasystem.conf"}}
	}
	selected, topErr := inv.owners.processes.process.repositoryTop(path)
	if topErr != nil || !sameCanonicalPath(owned.GitRoot, selected) {
		checkout := path
		if topErr == nil {
			checkout = selected
		}
		return stateroot.Layout{}, "", false, &intentResult{Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("the installation %s is not part of %s; nothing was done", shellCommand([]string{installation}), shellCommand([]string{checkout})),
			next:    inv.retryWith([]string{"installation"}), nextReason: "the repository's own installation is found without --installation",
			Details: []string{"--installation names an installation inside the selected checkout; --repo PATH selects another checkout"}}
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
			Summary: noEngineRefusal,
			next:    strings.Fields(hookswitch.BuildCommand), nextReason: "in " + installation + ", builds the engine; then repeat this command",
			Details: []string{"no engine in " + filepath.Join(installation, "bin")}}
	}
	installationRoot, err := stateroot.ParseInstallation(installation)
	var stateRoot stateroot.State
	if err == nil {
		stateRoot, err = inv.owners.resolver.RootForInstallation(installationRoot)
	}
	if err != nil {
		return processScope{}, 0, &intentResult{Outcome: intentRefused, code: 1, Summary: "this installation's records cannot be found, so nothing was done",
			next: []string{"metasystem", "system", "check"}, nextReason: "names what is wrong here", Details: []string{"state root: " + err.Error()}}
	}
	inv.stateRoot = stateRoot.Path()
	scale := upWaitScale()
	if scale < 1 {
		return processScope{}, 0, &intentResult{Outcome: intentRefused, code: 2, Summary: "a test setting in the environment scales waits by a number that is not positive, so nothing was done",
			next: append([]string{"env", "-u", "METASYSTEM_FIXTURE_CAP_SCALE_MILLI"}, inv.typedArgv()...), nextReason: "without the test setting",
			Details: []string{"METASYSTEM_FIXTURE_CAP_SCALE_MILLI must be a positive integer"}}
	}
	// The stop fence, the steward's runner, the supervision and the families a
	// stop finds are run state under the installation. The state root holds
	// the project's state: the installation itself in a template checkout,
	// the application repository in an adopted one.
	scope := processScope{Checkout: layout.GitRoot, Installation: installationRoot, InstallationExplicit: explicit, Root: stateRoot, Binary: binary}
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

// processRefusalResult keeps the owner's sentence and the command that
// resolves it; a human act the caller cannot perform from here is named,
// never run. The transition's step lines are details.
func processRefusalResult(targets []intentTarget, refusal *processRefusal, report stoptransition.Report) intentResult {
	sentence := strings.TrimSuffix(strings.TrimSpace(refusal.sentence), ".")
	if refusal.plain != "" && sentence == "" {
		sentence = refusal.plain
	}
	next, reason, decision := refusal.next, refusal.nextReason, ""
	if command, found := strings.CutPrefix(refusal.second, "run: "); next == nil && found && !strings.ContainsAny(command, ";,") {
		next = shellWords(command)
	} else if next == nil {
		decision = refusal.second
	}
	return intentResult{Outcome: intentRefused, Targets: targets, Summary: sentence + ", so nothing was changed",
		next: next, nextReason: reason, Decision: decision, code: max(refusal.code, 1),
		Details: append(append([]string(nil), refusal.details...), report.Lines...),
		Data:    map[string]any{"lines": nonNilLines(report.Lines), "remedy": refusal.second}}
}

// runIntentSystemStart arms this checkout's machinery at a person's word,
// or with --if-down recovers only helpers that are down.
func runIntentSystemStart(inv *intentInvocation) int {
	if inv.input.switched("if-down") {
		for _, other := range []string{"temporary-human-word", "review-by"} {
			if inv.input.has(other) {
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--if-down restarts only what is down and takes no --%s; nothing was done", other),
					next: withoutOption(inv.typedArgv(), other)})
			}
		}
		scope, _, problem := inv.selectProcessScope()
		if problem != nil {
			return inv.render(*problem)
		}
		return runUpWith([]string{"--metasystem-root", scope.Installation.Path(), "--repo", scope.Checkout, "--recover-only", "--if-down"}, inv.owners.processes.process.repositoryTop, inv.stdout, inv.stderr)
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
		result := processUnchangedResult(inv.checkoutTarget(scope), report)
		result.view = inv.processDoneView(scope.Checkout, "already runs for", report, report.Lines)
		return inv.render(result)
	}
	result := processReportResult(inv.checkoutTarget(scope), "started "+scope.Checkout, report)
	result.view = inv.processDoneView(scope.Checkout, "runs for", report, report.Lines)
	return inv.render(result)
}

// runIntentSessionStart prepares the current agent session through up.
func runIntentSessionStart(inv *intentInvocation) int {
	scope, _, problem := inv.selectProcessScope()
	if problem != nil {
		return inv.render(*problem)
	}
	result, problem := inv.prepareClaimSession(inv.claimLineage())
	if problem != nil {
		return inv.render(*problem)
	}
	outcome := intentConfirmed
	remedy := result.Remedy
	if result.Adoption != nil && result.Adoption.Status == "pending" {
		outcome = intentPartial
		remedy = result.Adoption.Remedy
	}
	if result.ExitCode() != 0 {
		outcome = intentRefused
	}
	return inv.render(intentResult{Outcome: outcome, Targets: []intentTarget{{Kind: "session", ID: scope.Checkout}}, code: result.ExitCode(),
		Summary: sessionPreparationSummary(result), text: result.Lines(), Decision: remedy,
		Data: map[string]any{"outcome": result.Outcome, "adoption": result.Adoption, "lines": nonNilLines(result.Lines()), "remedy": remedy}})
}

// prepareClaimSession leaves enrollment, announcement, adoption and supervision
// with up and carries the original caller into that lifecycle owner.
func (inv *intentInvocation) prepareClaimSession(lineage string) (up.Result, *intentResult) {
	scope, scale, problem := inv.selectProcessScope()
	if problem != nil {
		return up.Result{}, problem
	}
	callerPid, err := inv.owners.dependencies.authorityFacts.caller.ClassifiablePid(processidentity.KernelProber{})
	if err != nil {
		return up.Result{}, &intentResult{Outcome: intentRefused, code: 1, Summary: "the calling process could not be authenticated; the session was not started", Details: []string{err.Error()}, next: inv.publicArgv("session", "start")}
	}
	owners := inv.owners.processes
	if owners.executable == nil {
		owners.executable = os.Executable
	}
	if owners.up == nil {
		owners.up = up.Run
	}
	binary, err := owners.executable()
	if err == nil {
		binary, err = canonicalPath(binary)
	}
	if err != nil {
		return up.Result{}, &intentResult{Outcome: intentFailed, code: 1, Summary: "the running engine's own path could not be read, so the session was not started",
			retry: "try again", Details: []string{"engine path: " + err.Error()}}
	}
	reads := owners.adoptionReads
	if reads == nil {
		reads = &claimAdoptionReads{dependencies: inv.owners.dependencies, clock: inv.owners.commandNow}
	}
	result := owners.up(up.Options{
		Root: scope.Installation.Path(), MetasystemRoot: scope.Installation.Path(), Scope: scope.Checkout, Binary: binary,
		OwnerLineage: lineage, WaitScaleMilli: scale, CallerPid: callerPid,
		RestampStopCapability: reads.RestampFresh,
	})
	return result, nil
}

func sessionPreparationSummary(result up.Result) string {
	if result.Adoption != nil && result.Adoption.Status == "pending" {
		return "session preparation is partial: adoption pending; " + result.Adoption.Cause
	}
	return "session start: " + result.Outcome
}

// runIntentSystemStop stops this checkout's machinery at a person's word.
func runIntentSystemStop(inv *intentInvocation) int {
	return inv.render(systemStopResult(inv))
}

// systemStopResult is system stop's whole act and result for the checkout
// inv selects; machine stop runs it once per machine of this computer.
func systemStopResult(inv *intentInvocation) intentResult {
	scope, scale, problem := inv.selectProcessScope()
	if problem != nil {
		return *problem
	}
	report, refusal := inv.owners.processes.process.stop(scope, scale)
	if refusal != nil {
		return processRefusalResult(inv.checkoutTarget(scope), refusal, report)
	}
	if report.Unchanged {
		result := processUnchangedResult(inv.checkoutTarget(scope), report)
		result.view = inv.processDoneView(scope.Checkout, "was already stopped for", report, report.Lines)
		return result
	}
	// The checkout is stopped: this user's launches of it end with it.
	cancel := inv.cancelCheckoutLaunches(scope.Checkout)
	report.Lines = append(report.Lines, cancel.lines...)
	var result intentResult
	if report.ExitCode != 0 {
		_, fence := processFence(scope)
		result = intentResult{Outcome: intentPartial, code: report.ExitCode, Targets: inv.checkoutTarget(scope), text: report.Lines,
			Summary: "stop did not finish: some processes are still running (listed below); no new work starts (" + fence + ")",
			next:    inv.publicArgv(append([]string{"system", "stop"}, inv.forward("installation")...)...), nextReason: "stop again; end any process that survives a second stop yourself",
			Data: map[string]any{"lines": nonNilLines(report.Lines), "exitCode": report.ExitCode, "fence": fence}}
	} else {
		result = processReportResult(inv.checkoutTarget(scope), "stopped "+scope.Checkout, report)
		if cancel.failed > 0 {
			// A launch that could not be cancelled still runs; work stop
			// cancels it by its reference, a repeated system stop would not.
			result.Outcome, result.code = intentPartial, 1
			result.next, result.nextReason = inv.publicArgv("work", "stop", cancel.firstFailed), "cancel the launch that may still be this checkout's"
		} else {
			result.view = inv.processDoneView(scope.Checkout, "is stopped for", report, report.Lines)
		}
	}
	data := result.Data.(map[string]any)
	data["launchesCancelled"], data["launchesNotCancelled"], data["launchLines"] = cancel.cancelled, cancel.failed, nonNilLines(cancel.lines)
	return result
}

// processDoneView is a start, stop or restart that held: one line saying
// what MetaSystem now does for the checkout, by the name a person knows it
// by, and since when when it already did. --verbose adds the checkout and
// each step as its owner wrote it. A reading that is incomplete says so.
func (inv *intentInvocation) processDoneView(checkout, state string, report stoptransition.Report, steps []string) func(*textui.Page) {
	name := inv.statusSeatName(checkout)
	return func(page *textui.Page) {
		env := page.Env()
		headline := "MetaSystem " + state + " " + name
		if since, err := time.Parse(time.RFC3339, report.Since); report.Unchanged && err == nil {
			headline += " " + env.Since(since)
		}
		if report.ExitCode != 0 {
			page.Mark(textui.Alert, headline+", but its reading is incomplete")
		} else {
			page.Done(headline)
		}
		if report.ExitCode != 0 || page.Verbose() {
			page.Facts(textui.KV{Key: "checkout", Value: []textui.Span{textui.Plain(env.Path(checkout))}})
			section := page.Section("Steps", "")
			for _, step := range steps {
				section.Text(step)
			}
		}
	}
}

// checkoutLaunchCancel is what cancelCheckoutLaunches did: one line per
// launch, how many were cancelled and how many were not.
type checkoutLaunchCancel struct {
	lines             []string
	cancelled, failed int
	// firstFailed is the reference of the first launch not cancelled.
	firstFailed string
}

// cancelCheckoutLaunches cancels this user's running launches of checkout
// as work stop cancels them (stopResolvedJob), one line per launch. A
// launch is the checkout's where placeLaunches puts it among this
// computer's machines, so machine list, machine stop and system stop agree:
// in the checkout or below it, or in one of its registered worktrees.
func (inv *intentInvocation) cancelCheckoutLaunches(checkout string) checkoutLaunchCancel {
	var out checkoutLaunchCancel
	if inv.owners.processes.launches == nil {
		return out
	}
	records, err := inv.owners.processes.launches().List()
	if err != nil {
		out.failed++
		out.lines = append(out.lines, "this user's launches cannot be listed, so none of them was cancelled: "+err.Error())
		out.firstFailed = "ID"
		return out
	}
	running := records[:0:0]
	for _, record := range records {
		if !record.State.Terminal() {
			running = append(running, record)
		}
	}
	if len(running) == 0 {
		return out
	}
	reading := inv.discoverHostMachines(nil)
	var mine *hostMachine
	for _, machine := range reading.Machines {
		if machine.Checkout == checkout {
			mine = machine
		}
	}
	if mine == nil {
		mine = &hostMachine{Checkout: checkout}
		reading.Machines = append(reading.Machines, mine)
	}
	inv.placeLaunchRecords(&reading, running)
	if reading.LaunchProblem != "" {
		out.lines = append(out.lines, reading.LaunchProblem)
	}
	if mine.worktreesUnread {
		// A launch no checkout holds may be this checkout's build in a
		// worktree Git could not list: not cancelled on a guess, never
		// passed over as stopped.
		for _, record := range reading.launchesElsewhere {
			reference := jobReference(intentJob{id: record.ID, kind: "launch", launch: record})
			out.lines = append(out.lines, fmt.Sprintf("launch %s in %s: not cancelled; it may be this checkout's, whose worktrees cannot be listed", reference, record.WorkingDirectory))
			out.failed++
			if out.firstFailed == "" {
				out.firstFailed = reference
			}
		}
	}
	for _, record := range mine.launchRecords {
		job := intentJob{id: record.ID, kind: "launch", launch: record}
		result := inv.stopResolvedJob(job)
		out.lines = append(out.lines, "launch "+jobReference(job)+": "+result.Summary)
		switch result.Outcome {
		case intentConfirmed:
			out.cancelled++
		case intentUnchanged:
		default:
			out.failed++
			if out.firstFailed == "" {
				out.firstFailed = jobReference(job)
			}
		}
	}
	return out
}

func (inv *intentInvocation) stopSession() int {
	by := strings.TrimSpace(inv.input.text("by"))
	if by == "" {
		_ = inv.selectLayoutRoot() // the enrolled person's name, when there is one
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: []intentTarget{{Kind: "session", ID: ""}},
			Summary: "session stop needs your name, so nothing was done",
			next:    inv.publicArgv("session", "stop", "--by", inv.knownPerson()), nextReason: "at your enrolled terminal"})
	}
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	marker, refusal, code := inv.owners.processes.sessionStop(inv.stateRoot, by)
	if code != 0 {
		result := intentResult{Outcome: intentRefused, code: code, Targets: []intentTarget{{Kind: "session", ID: ""}}, Summary: refusal,
			retry: "once the cause above is fixed; the checkout keeps running either way"}
		if code == 3 {
			// Only a person stops a session quietly: the same command at the
			// person's own enrolled terminal.
			result.retry = "at your enrolled terminal, in a shell you opened yourself"
		}
		return inv.render(result)
	}
	if refusal != "" {
		// The same person's authorization already holds (R-129-ui).
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: []intentTarget{{Kind: "session", ID: marker.SessionId}},
			Summary: refusal, Data: map[string]any{"sessionStop": marker}})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: []intentTarget{{Kind: "session", ID: marker.SessionId}},
		Summary: fmt.Sprintf("the running session may now stop once without asking (%s's word); the checkout keeps running", marker.By),
		Details: []string{fmt.Sprintf("session %s, holder %s, lease epoch %d", marker.SessionId, marker.HolderMainId, marker.ClaimEpoch)},
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
		var root stateroot.State
		root, err = inv.owners.resolver.RootForInstallation(layout.InstallationRoot)
		inv.stateRoot = root.Path()
	}
	if err != nil {
		return inv.notARepository(path, err)
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
		Summary: fmt.Sprintf("%d %s job(s) among %s", len(jobs), word, scope), view: inv.jobListView(jobs, all)}
	if !all {
		result.next, result.nextReason = inv.publicArgv("work", "status", "--all"), "also lists ended jobs"
	}
	return inv.render(result)
}

// jobListView is work status without a target (output-style §6.9): how
// many jobs are listed and where they come from, then one row each with its
// reference, goal, kind, state and start. Launches are the user's on this
// host, whichever checkout started them; only the dispatch jobs are this
// checkout's own, and the headline says both. A place with no repository
// says it lists launches only.
func (inv *intentInvocation) jobListView(jobs []intentJob, all bool) func(*textui.Page) {
	place := "outside a repository"
	if inv.stateRoot != "" {
		place = "among this host's launches and " + inv.statusSeatName(inv.layout.GitRoot) + "'s jobs"
	}
	return func(page *textui.Page) {
		env := page.Env()
		word := "running"
		if all {
			word = "known"
		}
		switch len(jobs) {
		case 0:
			page.Headline("No jobs " + word + " " + place)
		default:
			page.Headline(textui.Count(len(jobs), "job", "jobs") + " " + word + " " + place)
		}
		if inv.stateRoot == "" {
			page.Facts(textui.KV{Key: "note", Value: []textui.Span{textui.Plain("no repository here, so only your launches are listed")}})
		}
		if len(jobs) == 0 {
			return
		}
		table := page.Section("", "").Table(textui.Column{Title: "job"}, textui.Column{Title: "goal"}, textui.Column{Title: "kind"},
			textui.Column{Title: "state"}, textui.Column{Title: "since"})
		for _, job := range jobs {
			kind, state, started := job.launch.Kind+" launch", string(job.launch.State), job.launch.StartedAt
			if job.kind == "dispatch" {
				role, _ := job.dispatch["role"].(string)
				status, _ := job.dispatch["status"].(string)
				kind, state = cmpOr(role, "dispatch")+" job", cmpOr(status, "unknown")
				started, _ = job.dispatch["startedAt"].(string)
			}
			since := ""
			if at, err := time.Parse(time.RFC3339, started); err == nil {
				since = env.Time(at)
			}
			mark := textui.Running
			if jobEnded(job) {
				mark = textui.Stopped
			}
			table.Row(textui.Marked(mark, jobReference(job)), textui.Plain(cmpOr(jobGoal(job), "–")), textui.Plain(kind), textui.Plain(state), textui.Plain(since))
		}
	}
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
			return nil, "", &intentResult{Outcome: intentFailed, code: 1, Summary: "your launches could not be read, so nothing was listed",
				retry: "try again", Details: []string{"launches: " + err.Error()}}
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
				text: []string{launchReport(job.launch)}, Data: map[string]any{"kind": "launch", "record": job.launch},
				view: jobStopView(jobReference(job)+" had already ended: "+string(job.launch.State), "", launchReport(job.launch))}
		}
		record, err := inv.owners.processes.launches().Cancel(id)
		if err != nil {
			return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: fmt.Sprintf("launch %s could not be cancelled", jobReference(job)),
				retry: "try again", Details: []string{"cancel: " + err.Error()}, Data: map[string]any{"kind": "launch", "record": record}}
		}
		return intentResult{Outcome: intentConfirmed, Targets: targets, Summary: fmt.Sprintf("launch %s cancelled: %s", id, record.State),
			text: []string{launchReport(record)}, Data: map[string]any{"kind": "launch", "record": record},
			view: jobStopView("Stopped "+jobReference(job)+": "+string(record.State), "", launchReport(record))}
	}
	if status, _ := job.dispatch["status"].(string); dispatchcore.TerminalStatus(status) {
		// A job that already ended is already stopped: the repeat is success
		// and never reaches the cancellation owner (R-129-ui).
		summary := fmt.Sprintf("%s is already stopped: %s", jobReference(job), status)
		ended, _ := job.dispatch["endedAt"].(string)
		if ended != "" {
			summary += " (at " + ended + ")"
		}
		return intentResult{Outcome: intentUnchanged, Targets: targets, Summary: summary, Data: map[string]any{"kind": "dispatch", "status": status},
			view: jobStopView(jobReference(job)+" had already stopped: "+status, ended, "")}
	}
	outcome, code, err := inv.owners.processes.cancelDispatch(inv.layout.GitRoot, id)
	if err != nil {
		return intentResult{Outcome: intentFailed, code: max(code, 1), Targets: targets, Summary: fmt.Sprintf("job %s could not be cancelled", jobReference(job)),
			retry: "try again", Details: []string{"cancel: " + err.Error()}}
	}
	result := intentResult{Targets: targets, code: code, Data: map[string]any{"kind": "dispatch", "owner": outcome}}
	label, _ := outcome["outcome"].(string)
	detail, _ := outcome["detail"].(string)
	if code == 0 && label == "CANCELLED" {
		result.Outcome, result.Summary = intentConfirmed, fmt.Sprintf("dispatch job %s cancelled", id)
		result.view = jobStopView("Stopped "+jobReference(job), "", "")
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
		return inv.render(inv.withHelm(intentResult{Outcome: intentFailed, code: 1, Targets: inv.checkoutTarget(scope), Summary: "this checkout's status could not be read",
			retry: "try again", Details: []string{"status: " + err.Error()}}, scope.Checkout))
	}
	result := processReportResult(inv.checkoutTarget(scope), "status of "+scope.Checkout, report)
	reading := inv.readStatusBoard()
	result.Data.(map[string]any)["board"] = reading.view
	result.view = inv.statusView(scope.Checkout, report, reading)
	return inv.render(inv.withHelm(result, scope.Checkout))
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
	return runStewardStatus(args, inv.stdout, inv.stderr)
}

// runIntentWorkStatus lists running work, or reads the one goal, job, run
// or read a reference names.
func runIntentWorkStatus(inv *intentInvocation) int {
	if inv.input.switched("history") {
		return runIntentWorkHistory(inv)
	}
	if inv.input.has("since") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--since narrows the history, so it needs --history; nothing was done",
			next: inv.retryWith(nil, "--history"), nextReason: "the history since then"})
	}
	if len(inv.input.args) == 0 {
		if inv.input.has("work") {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--work names one work of a goal, so it needs the goal; nothing was done",
				next: inv.retryWith(nil, "GOAL"), nextReason: "with the goal's id"})
		}
		return runIntentStatusWork(inv)
	}
	if inv.input.switched("all") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--all lists all work, so it takes no goal or job; nothing was done",
			next: inv.publicArgv("work", "status", "--all"), nextReason: "all work; or drop --all to read the one you named"})
	}
	ref, problem := inv.resolveWorkRef(inv.input.args[0], inv.command.accepts)
	if problem != nil {
		return inv.render(*problem)
	}
	if ref.kind != refGoal && inv.input.has("work") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--work names one work of a goal, not of a job or run; nothing was done",
			next: inv.retryWith([]string{"work"}), nextReason: "without --work"})
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
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: fmt.Sprintf("you have no unit run %s; nothing was read", shellCommand([]string{ref.qualified()})),
				next: inv.publicArgv("work", "status"), nextReason: "lists the running work"})
		}
		if err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: fmt.Sprintf("unit run %s could not be read", ref.qualified()),
				retry: "try again", Details: []string{"unit run: " + err.Error()}})
		}
		lines := []string{}
		for _, round := range record.Rounds {
			line := fmt.Sprintf("round %d: %s", round.Number, round.Outcome)
			if round.Cause != "" {
				line += " cause=" + round.Cause
			}
			lines = append(lines, line)
		}
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, text: lines,
			Summary: fmt.Sprintf("unit run %s (%s, goal %s): %s", unitRunPrefix+record.ID, record.Unit, record.Goal, record.State), Data: map[string]any{"record": record}})
	}
	job := ref.job
	targets := []intentTarget{{Kind: "job", ID: jobReference(job)}}
	if job.kind == "launch" {
		record, err := inv.owners.processes.launches().Status(job.id)
		if err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: fmt.Sprintf("launch %s could not be read", jobReference(job)),
				retry: "try again", Details: []string{"launch status: " + err.Error()}})
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
	view := func(page *textui.Page) {
		if len(stopped) == 0 {
			page.Done("No job of goal " + id + " is running; nothing to stop")
		} else {
			page.Done("Stopped " + textui.Count(len(stopped), "job", "jobs") + " of goal " + id)
			section := page.Section("", "")
			for _, ref := range stopped {
				section.Item(textui.Stopped, ref)
			}
		}
		if stopLine != "" {
			page.Facts(textui.KV{Key: "budget stop", Value: []textui.Span{textui.Plain(stopLine)}})
		}
	}
	switch {
	case len(failed) > 0:
		return inv.render(intentResult{Outcome: intentPartial, code: 1, Targets: targets, Data: data, text: lines,
			Summary: fmt.Sprintf("goal %s: %d job(s) stopped, %d not stopped", id, len(stopped), len(failed)),
			next:    inv.publicArgv("work", "status", id), nextReason: "shows each of the goal's jobs and its state"})
	case len(stopped) == 0:
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: data, text: lines,
			Summary: fmt.Sprintf("no job of goal %s is running; nothing was stopped", id), view: view})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: data, text: lines,
		Summary: fmt.Sprintf("goal %s: %d running job(s) stopped", id, len(stopped)), view: view})
}

// jobStopView is one job's stop: what happened to it, when it had already
// ended, and with --verbose its record as the launch owner reports it.
func jobStopView(done, endedAt, record string) func(*textui.Page) {
	return func(page *textui.Page) {
		if at, err := time.Parse(time.RFC3339, endedAt); err == nil {
			done += " at " + page.Env().Time(at)
		}
		page.Done(done)
		if page.Verbose() && record != "" {
			section := page.Section("Record", "")
			for _, line := range strings.Split(strings.TrimSpace(record), "\n") {
				section.Text(line)
			}
		}
	}
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
		result := processRefusalResult(targets, refusal, stopped)
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
			"start": map[string]any{"lines": nonNilLines(armed.Lines), "exitCode": armed.ExitCode}},
		view: inv.processDoneView(scope.Checkout, "restarted for", stoptransition.Report{ExitCode: armed.ExitCode}, lines)})
}

func runIntentEnroll(inv *intentInvocation) int {
	name := strings.TrimSpace(inv.input.text("name"))
	if name == "" {
		guess := inv.personName("")
		if guess == "" {
			guess = inv.owners.helm.withDefaults().account()
		}
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "enroll needs your name; nothing was done",
			next: inv.publicArgv("system", "enroll", "--name", guess), nextReason: "in a terminal you opened yourself, with your own name"})
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
			Data: map[string]any{"enrollment": report.value, "fleetPublished": true, "owner": ownerPublication(*report.result)},
			view: doneView("This terminal is already enrolled for " + name + ", and the fleet knows it")})
	case code == 0 && report.result != nil:
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Summary: "this terminal is enrolled for " + name + " and the fleet cutoff is published",
			Data: map[string]any{"enrollment": report.value, "fleetPublished": true, "owner": ownerPublication(*report.result)},
			view: doneView("This terminal is enrolled for " + name + ", and every machine of the fleet is told")})
	}
	return inv.render(ownerResult(report, code, intentResult{}))
}

func runIntentAsk(inv *intentInvocation) int {
	id, problem := inv.singleTarget()
	if problem != nil {
		return inv.render(*problem)
	}
	about := inv.input.text("about")
	switch {
	case about != "" && about != "lane" && about != "machine":
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("a question without a goal is about the lane or the machine, not %s, so nothing was asked", shellCommand([]string{about})),
			next: inv.retryWith([]string{"about"}, "--about", "lane"), nextReason: "or --about machine"})
	case about != "" && id != "":
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id), Summary: "a question names a goal or says what it is about, not both, so nothing was asked",
			next: inv.retryWith([]string{"about"}), nextReason: "about the goal alone"})
	case about != "" && inv.input.text("kind") != "" && inv.input.text("kind") != "other":
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("a %s question carries authority over a goal, so it cannot be asked --about %s; nothing was asked", inv.input.text("kind"), about),
			next: inv.retryWith([]string{"kind", "budget", "wants"}), nextReason: "as an ordinary question"})
	case id == "" && about == "":
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "question ask needs the goal the question is about, or --about lane or machine, so nothing was asked",
			next: inv.retryWith(nil, "GOAL"), nextReason: "with the goal's id"})
	}
	question := strings.TrimSpace(inv.input.text("question"))
	if question == "" || len(inv.input.values["option"]) == 0 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
			Summary: "question ask needs the question and at least one answer to choose, so nothing was asked",
			next:    inv.retryWith([]string{"question"}, "--question", "TEXT", "--option", "LABEL: CONSEQUENCE"), nextReason: "one --option per answer"})
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	// A question about the lane or the machine names no goal target.
	var askTargets []intentTarget
	if id != "" {
		askTargets = inv.targets(id)
	}
	kind := inv.input.text("kind")
	if kind == "" {
		// An ordinary question; the other kinds carry authority.
		kind = "other"
	}
	in := channelAskInput{Goal: id, About: about, Kind: kind, Facts: append([]string{question}, inv.input.values["fact"]...),
		Options: inv.input.values["option"], Recommendation: inv.input.text("recommend"), Wants: inv.input.text("wants")}
	if inv.input.has("budget") {
		if kind != "stop" && kind != "budget-above-norm" {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: askTargets,
				Summary: "--budget belongs to a stop or budget question only; nothing was asked",
				next:    inv.retryWith([]string{"budget"}), nextReason: "without --budget"})
		}
		reviewRoundMax, err := config.ReviewRoundMax(filepath.Join(inv.stateRoot, "metasystem.conf"))
		if err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the review round limit in metasystem.conf cannot be read, so nothing was asked",
				next: inv.publicArgv("settings", "check"), nextReason: "names the setting to fix", Details: []string{err.Error()}})
		}
		budget, err := goalbudget.ParseBox(inv.input.text("budget"), nil, reviewRoundMax)
		if err != nil {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: askTargets,
				Summary: fmt.Sprintf("--budget %s is not a whole budget box, so nothing was asked", shellCommand([]string{inv.input.text("budget")})),
				next:    inv.retryWith([]string{"budget"}, "--budget", "1d/10/720m/1/3"), nextReason: "with your own numbers in this shape", Details: []string{err.Error()}})
		}
		if kind == "stop" {
			in.Wants = goal.ResumeApprovalToken(id, budget)
		} else {
			in.Budget = &budget
		}
	} else if kind == "stop" || kind == "budget-above-norm" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: askTargets,
			Summary: fmt.Sprintf("a %s question proposes a budget box, so nothing was asked", kind),
			next:    inv.retryWith(nil, "--budget", "1d/10/720m/1/3"), nextReason: "with your own numbers in this shape"})
	}
	if kind == "carry" && !goal.ValidCarryToken(in.Wants) {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: askTargets,
			Summary: "a carry question needs --wants in its exact shape, so nothing was asked",
			next:    inv.retryWith([]string{"wants"}, "--wants", "carry workspace=SHA goal="+id+" past=NAME"), nextReason: "the workspace's 40-character commit, and who it carries past"})
	}
	q, warnings, code, err := inv.owners.processes.ask(inv.stateRoot, in)
	if err != nil && q.ID == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: max(code, 1), Targets: askTargets, Summary: err.Error() + "; nothing was asked", text: warnings,
			retry: "once the cause above is fixed"})
	}
	targets := []intentTarget{{Kind: "goal", ID: id}, {Kind: "question", ID: q.ID}}
	subject := "goal " + id
	if id == "" {
		targets, subject = targets[1:], "the "+about
	}
	delivery, pending := "posted to the channel", false
	switch {
	case q.Thread == nil && q.Undelivered > 0:
		delivery, pending = fmt.Sprintf("not delivered yet: posting to the channel failed %d time(s)", q.Undelivered), true
	case q.Thread == nil:
		delivery, pending = "not sent: no channel is configured for this repository", true
	}
	data := map[string]any{"question": q, "delivery": delivery, "replyInstructions": channel.ReplyInstructionsAt(inv.stateRoot, q)}
	lines := append(warnings, "delivery: "+delivery)
	wait := inv.publicArgv("question", "wait", "channel:"+q.ID)
	poll := inv.publicArgv("question", "retry", q.ID)
	if errors.Is(err, errQuestionAlreadyOpen) {
		// The same question already stands open (R-129-ui): success, and
		// nothing was written, posted or published again.
		repeat := intentResult{Outcome: intentUnchanged, Targets: targets, text: lines, Data: data,
			Summary: "question " + q.ID + " is already open for " + subject + " with this text and these options (" + delivery + "); nothing was asked again",
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
	lines = append(lines, "the person answers in the channel thread: "+channel.ReplyInstructionsAt(inv.stateRoot, q))
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Summary: "asked " + q.ID + " and " + delivery, text: lines,
		next: wait, nextReason: "wait for the authenticated answer", Data: data})
}

// answerMission records a person's answer through the mission owner, which
// applies it or changes nothing.
func (inv *intentInvocation) answerMission(missionID, askID, answer string) intentResult {
	targets := []intentTarget{{Kind: "mission", ID: missionID}, {Kind: "question", ID: askID}}
	if !missionIDRe.MatchString(missionID) || !missionIDRe.MatchString(askID) || strings.TrimSpace(answer) == "" || strings.ContainsRune(answer, 0) {
		return (intentResult{Outcome: intentRefused, code: 2, Targets: targets, Summary: "that is not a mission question with an answer, so nothing was answered",
			next: inv.publicArgv("question", "list"), nextReason: "lists the open questions with their ids",
			Details: []string{"a mission and question id are lowercase words with dashes, and the answer is non-empty text"}})
	}
	engine, err := inv.owners.processes.mission(inv.stateRoot, inv.layout.InstallationRoot, missionID)
	if err != nil {
		return (intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "mission " + missionID + " could not be opened, so nothing was answered",
			next: inv.publicArgv("mission", "status", missionID), nextReason: "what the mission is doing", Details: []string{err.Error()}})
	}
	var output, errs bytes.Buffer
	engine.Output, engine.Errors = &output, &errs
	askPath := inv.layout.InstallationRoot.Path("artifacts", "agents", "missions", missionID, "asks", askID+".json")
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
			Summary: "the mission did not take the answer; nothing changed", next: inv.publicArgv("question", "show", missionID+"/"+askID),
			nextReason: "the question and the answers it takes"})
	}
	return (intentResult{Outcome: intentFailed, code: code, Targets: targets, text: lines, Data: data, Summary: "the mission's state could not be read; nothing changed",
		next: inv.publicArgv("mission", "status", missionID), nextReason: "what the mission is doing"})
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

func runIntentDoctor(inv *intentInvocation) int {
	scope, _, problem := inv.selectProcessScope()
	if problem != nil {
		return inv.render(*problem)
	}
	owners := inv.owners.processes
	now, err := owners.healthNow(scope.Installation.Path())
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 2, Summary: "the test clock of this installation cannot be read, so nothing was checked",
			retry: "once the test clock file is fixed or removed", Details: []string{"fixture clock: " + err.Error()}})
	}
	verdict := owners.health(scope.Root.Path(), scope.Installation.Path(), now)
	stopped, _, _ := stopfence.Closed(scope.Installation.Path())
	lines, remedies := []string{}, []map[string]any{}
	var first []string
	var problems []doctorProblem
	for _, role := range verdict.Roles {
		if role.Status == steward.HealthAlive {
			continue
		}
		line := fmt.Sprintf("%s %s: %s", role.Role, role.Status, role.Reason)
		public, instruction := publicHealthRemedy(role, stopped)
		problem := doctorProblem{role: string(role.Role), status: string(role.Status), reason: role.Reason, fix: public, instruction: instruction}
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
			problem.instruction = "no command repairs this"
		}
		problems = append(problems, problem)
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
	lines = append(lines, diskCheckLine(scope.Installation.Path()))
	adapters, refused := adapterReport(scope.Installation.Path())
	lines = append(lines, adapters...)
	if refused > 0 {
		code = max(code, 1)
	}
	// Skills are where users extend the metasystem; their frontmatter and
	// naming rules are part of this checkout's health.
	skillsData := map[string]any{"valid": true}
	var skillLines strings.Builder
	if err := validate.SkillInventory(scope.Installation.Path(), &skillLines); err != nil {
		skillsData = map[string]any{"valid": false, "reason": err.Error()}
		lines = append(lines, "skills invalid: "+err.Error()+"; fix that skill's SKILL.md (its name and description frontmatter)")
		code = max(code, 1)
	}
	result := intentResult{Outcome: intentConfirmed, code: code, Targets: inv.checkoutTarget(scope),
		Summary: verdict.LineWithoutRemedies(), text: lines, Data: additiveData(steward.NewHookHealthPreview(verdict), map[string]any{"publicRemedies": remedies, "covenant": covenantData, "adapters": adapters, "skills": skillsData}),
		view: doctorView(inv.statusSeatName(scope.Checkout), string(verdict.Aggregate), problems, lines[len(problems):], first)}
	if first != nil {
		result.next, result.nextReason = first, "the first public remedy check found"
	}
	return inv.render(inv.withHelm(result, scope.Checkout))
}

// doneView is an act whose one line says all: what holds now.
func doneView(text string) func(*textui.Page) {
	return func(page *textui.Page) {
		if text != "" {
			text = strings.ToUpper(text[:1]) + text[1:]
		}
		page.Done(text)
	}
}

// doctorProblem is one role of the machinery that is not alive, with the
// public command that repairs it or the words that say what to do.
type doctorProblem struct {
	role, status, reason string
	fix                  []string
	instruction          string
}

// doctorView is system check's page: whether the checkout is healthy, each
// part of its machinery that is not with why and, when its repair is not
// the page's one hint, how to repair it; then the checkout's own findings
// (covenant, setup, disk, runtimes, skills) as their owners word them.
func doctorView(name, aggregate string, problems []doctorProblem, notes []string, hint []string) func(*textui.Page) {
	return func(page *textui.Page) {
		switch {
		case aggregate == "healthy" && len(problems) == 0:
			page.Headline(name + " is healthy")
		default:
			page.Headline(name+" is "+cmpOr(aggregate, "of unknown health"), textui.Count(len(problems), "part needs attention", "parts need attention"))
		}
		if len(problems) > 0 {
			table := page.Section("Machinery", "").Table(textui.Column{}, textui.Column{}, textui.Column{Flex: true, Wrap: true})
			for _, problem := range problems {
				state := textui.Failed
				if problem.status == string(steward.HealthUnknown) {
					state = textui.Unknown
				}
				why := problem.reason
				switch {
				case len(problem.fix) > 0 && !slices.Equal(problem.fix, hint):
					why += "; run " + shellCommand(problem.fix)
				case len(problem.fix) == 0 && problem.instruction != "":
					why += "; " + problem.instruction
				}
				table.Row(textui.Marked(state, problem.role), textui.Plain(problem.status), textui.Plain(why))
			}
		}
		if len(notes) > 0 {
			section := page.Section("This checkout", "")
			for _, note := range notes {
				section.Text(note)
			}
		}
	}
}

// runIntentWorkHistory reports how launches ended and why any was refused:
// every launch, one goal's, or one launch (j1:ID), through the launch
// report owner.
func runIntentWorkHistory(inv *intentInvocation) int {
	for _, other := range []string{"all", "work"} {
		if inv.input.has(other) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--%s does not go with --history; nothing was done", other),
				next: inv.retryWith([]string{other}), nextReason: "without --" + other})
		}
	}
	var args []string
	if len(inv.input.args) == 1 {
		switch kind, id := splitReference(inv.input.args[0]); kind {
		case refJ1:
			if inv.input.has("since") {
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--since narrows a goal's or every launch's history, not one launch's; nothing was done",
					next: inv.retryWith([]string{"since"}), nextReason: "the launch's whole history"})
			}
			args = []string{"--id", id}
		case "":
			args = []string{"--goal", inv.input.args[0]}
		default:
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("the history is kept for goals and launches, not for %s; nothing was done", inv.input.args[0]),
				next: inv.publicArgv("work", "status", inv.input.args[0]), nextReason: "its current state"})
		}
	}
	if inv.input.has("since") {
		args = append(args, "--since", inv.input.text("since"))
	}
	if inv.input.switched("json") {
		args = append(args, "--json")
	}
	return runLaunchReport(args, inv.stdout, inv.stderr)
}

// checkCovenantShape checks the app covenant's shape when the checkout has
// one, at its installation or its repository root: the path read, and the
// reason it does not parse. Shape is all it proves, never adequacy.
func checkCovenantShape(scope processScope) (string, error) {
	for _, dir := range []string{scope.Installation.Path(), scope.Checkout} {
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
	selected := hookswitch.ConfiguredRuntimes(layout.InstallationRoot.Path())
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
		if role.FailureEscalation == steward.AutoHealEnded {
			return strings.Fields(role.Remedy), ""
		}
		return []string{"metasystem", "goal", "list"}, ""
	case steward.RoleNonterminalJobs:
		return nil, "metasystem work stop j2:JOB records a job whose process is gone as ended; metasystem status lists the work"
	case steward.RoleRetroDebt:
		return nil, "run the retro and record its receipt"
	case steward.RoleTrunkRed:
		if strings.Contains(role.Reason, "cadence") {
			// The landing lane records the deep validation cadence when its
			// validation is due.
			return []string{"metasystem", "system", "start"}, ""
		}
		return []string{"metasystem", "incident", "list"}, ""
	case steward.RoleCapabilitySnapshots:
		if role.FailureEscalation == steward.AutoHealEnded {
			return nil, role.Remedy
		}
		return nil, "the steward tick probes each runtime on PATH with a missing or stale snapshot and records a fresh snapshot"
	case steward.RoleSpendFence:
		return nil, "a person raises the spend ceiling in metasystem.conf"
	case steward.RoleProofAttempts:
		return []string{"metasystem", "test", "run"}, ""
	case steward.RoleDisk:
		if strings.HasPrefix(role.Remedy, "metasystem ") {
			return strings.Fields(role.Remedy), ""
		}
		return []string{"metasystem", "disk", "show"}, ""
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
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: err.Error() + "; nothing was done",
			next: []string{"metasystem", "system", "check"}, nextReason: "names what is wrong here"})
	}
	options.seats = func() uiSeatInventory { return inv.uiOtherSeats(layout) }
	lifecycleResult, err := inv.owners.processes.ui(verb, roots, options)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: err.Error() + "; nothing was done",
			next: inv.publicArgv("ui", "status"), nextReason: "what the interface is doing now"})
	}
	result := lifecycleResult.Result
	data := uiSeatsData(lifecycleResult)
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
	// A restart that acted on another machine's interface targets that
	// seat, and its next steps name it.
	next := func(words ...string) []string { return append([]string{"metasystem", "ui"}, words...) }
	// On a branch whose summary is the first line naming the seat, the text
	// is the rest; every other branch keeps all lines.
	summary := func(fallback string) (string, []string) { return fallback, result.Lines }
	text := result.Lines
	if seat := lifecycleResult.Seat; seat != nil {
		targets = []intentTarget{{Kind: "ui", ID: seat.Checkout}}
		next = func(words ...string) []string {
			return append(append([]string{"metasystem", "ui"}, words...), "--repo", seat.Checkout)
		}
		summary = func(fallback string) (string, []string) {
			if len(result.Lines) > 0 {
				return result.Lines[0], uiLinesAfterSummary(result.Lines)
			}
			return fallback, nil
		}
	}
	restart := lifecycleResult.Restart
	if restart == nil {
		refusedSummary := "the interface could not be restarted; nothing was done"
		refused, refusedText := summary(refusedSummary)
		if lifecycleResult.Seat == nil && len(result.Lines) == 1 {
			// This seat's own refusal: its one line says it, as before.
			refused, refusedText = result.Lines[0], nil
		}
		// Refused before anything was stopped: a missing engine, an
		// address that does not resolve, or several machines to choose from.
		return inv.render(intentResult{Outcome: intentRefused, code: max(result.Code, 1), Targets: targets, text: refusedText, Data: data,
			Summary: refused, Decision: lifecycleResult.Decision})
	}
	data["stop"], data["started"] = restart.Stop, restart.Started
	switch {
	case restart.Started && restart.Start.Code == 0:
		restarted, restartedText := summary("the interface restarted")
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Summary: restarted, text: restartedText, Data: data})
	case restart.Started:
		// Nothing was stopped when nothing ran: the start alone failed.
		failed := "the interface stopped but did not start again"
		if restart.Stop == lifecycle.StopOutcome(lifecycle.Stopped) || restart.Stop == lifecycle.StopOutcome(lifecycle.Stale) {
			failed = "the interface was not running, and it did not start"
		}
		return inv.render(intentResult{Outcome: intentPartial, code: result.Code, Targets: targets, text: text, Data: data,
			Summary: failed, next: next("start"), nextReason: "start it once the problem above is fixed"})
	case restart.Stop == lifecycle.Timeout:
		return inv.render(intentResult{Outcome: intentPartial, code: max(result.Code, 1), Targets: targets, text: text, Data: data,
			Summary: "the interface was asked to stop but is still running, so it was not started again",
			next:    next("restart"), nextReason: "try again once it has stopped"})
	}
	return inv.render(intentResult{Outcome: intentRefused, code: max(result.Code, 1), Targets: targets, text: text, Data: data,
		Summary: "the interface could not be restarted; nothing was changed", next: inv.publicArgv("ui", "status"), nextReason: "what the interface is doing now"})
}

type uiIntentOptions struct {
	listen      string
	listenSet   bool
	waitSeconds int64
	// seats reads the other machines of this computer, only when a verb
	// needs them.
	seats func() uiSeatInventory
}

func (inv *intentInvocation) uiOptions() (uiIntentOptions, *intentResult) {
	options := uiIntentOptions{listen: inv.input.text("listen"), listenSet: inv.input.has("listen"), waitSeconds: 15}
	if inv.input.has("wait-seconds") {
		seconds, err := strconv.ParseInt(inv.input.text("wait-seconds"), 10, 64)
		if err != nil || seconds < 0 || seconds > int64((1<<63-1)/time.Second) {
			return options, &intentResult{Outcome: intentRefused, code: 2, Summary: "--wait-seconds needs a whole number of seconds, 0 or more; nothing was done",
				next: inv.retryWith([]string{"wait-seconds"}, "--wait-seconds", "15"), nextReason: "or the number you want"}
		}
		options.waitSeconds = seconds
	}
	return options, nil
}

func sameCanonicalPath(left, right string) bool {
	left, leftErr := canonicalPath(left)
	right, rightErr := canonicalPath(right)
	return leftErr == nil && rightErr == nil && left == right
}

// processFence reads the checkout's stop fence as "state/phase".
func processFence(scope processScope) (stopfence.Record, string) {
	record, err := stopfence.Read(scope.Installation.Path())
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
		return processRefusalResult(inv.checkoutTarget(scope), refusal, report)
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
		Summary: "start began but didn't finish: " + strings.TrimSuffix(strings.TrimSpace(refusal.sentence), ".") + "; new work is accepted, but not everything runs",
		next:    inv.publicArgv(append([]string{"system", "start"}, inv.forward("installation")...)...), nextReason: "start again once the problem above is fixed",
		Details: []string{"stop fence: " + fence}}
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
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "question answer takes the question and one quoted answer; nothing was answered",
			next: inv.publicArgv("question", "answer", args[0], strings.Join(args[1:], " ")), nextReason: "the answer as one quoted text"})
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
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "the typed answer and --answer-file differ; nothing was answered",
				next: inv.publicArgv("question", "answer", args[0], "--answer-file", path), nextReason: "give the answer once"})
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
			Summary: "question " + q.id + " is answered in its channel thread, where you are known; nothing was recorded here",
			next:    inv.publicArgv("question", "show", "channel:"+q.id), nextReason: "where and how to answer it",
			Details: []string{channel.ReplyInstructionsAt(inv.stateRoot, q.channel)}})
	}
	if strings.TrimSpace(text) == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "a mission question is answered with TEXT (or --answer-file FILE); nothing was answered",
			next: inv.publicArgv("question", "show", q.publicName()), nextReason: "the question and its options"})
	}
	if q.ask["answeredAt"] != nil {
		recorded, _ := q.ask["answer"].(string)
		if recorded != text {
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: []intentTarget{{Kind: "question", ID: q.publicName()}},
				Summary: fmt.Sprintf("mission %s's question %s is already answered with a different answer; nothing was changed", q.mission, q.id),
				next:    inv.publicArgv("question", "show", q.publicName()), nextReason: "the recorded answer"})
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
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("%q is not a mission name (lowercase words with dashes); nothing was done", mission),
			next: inv.retryWith(nil), nextReason: "with the mission's name as its contract spells it"})
	}
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	var ran intentProcessResult
	if verb == "status" {
		root, installation := inv.stateRoot, inv.layout.InstallationRoot
		// EM-08: a mission with no state here is not a status record; exit
		// 0 would tell a script the mission exists.
		if !missionrunner.HasState(installation.Path(), mission) {
			return inv.render(inv.missionStatusWithoutState(mission))
		}
		ran = ownerCall(func(stdout, stderr io.Writer) int {
			return inv.ownerCalls().missionStatus(stdout, stderr, root, installation, mission)
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
			return intentResult{Targets: targets, Outcome: intentRefused, code: 1,
				Summary: "mission " + mission + " has a contract but was never started; nothing was read",
				next:    inv.publicArgv("mission", "start", mission), nextReason: "starts the mission its contract describes"}
		}
	}
	return intentResult{Targets: targets, Outcome: intentRefused, code: 1,
		Summary: "no mission " + mission + " in this repository; nothing was read", next: inv.publicArgv("system", "status"), nextReason: "what runs here"}
}

// missionOwnerLaunch starts or resumes one mission through the runner in
// this process (design 6.2); this process is the caller a closed fence's
// reopening classifies.
func (inv *intentInvocation) missionOwnerLaunch(mission, mode string) intentProcessResult {
	caller, root, wait := ownercall.CurrentProcess(), inv.stateRoot, inv.input.switched("wait")
	return ownerCall(func(stdout, stderr io.Writer) int {
		return inv.ownerCalls().missionLaunch(caller, stdout, stderr, root, mission, mode, wait, inv.owners.processes.process.repositoryTop)
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
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "mission seal needs the mission's name or contract file, so nothing was done",
			next: inv.retryWith(nil, "MISSION"), nextReason: "with the mission's name"})
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
			retry: "once the contract is fixed", Details: []string{"a contract that already carries an approval line is sealed before the line is added"}})
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
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("mission %s needs the mission's name, so nothing was done", verb),
			next: inv.retryWith(nil, "MISSION"), nextReason: "with the mission's name"})
	}
	return runIntentMission(inv, verb, inv.input.args[0])
}

// runIntentQuestionList lists the open channel questions of this
// repository through the channel's own question walk.
func runIntentQuestionList(inv *intentInvocation) int {
	if inv.input.has("since") && !inv.input.switched("answered") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--since narrows the answered questions only, so nothing was listed",
			next: append(inv.typedArgv(), "--answered"), nextReason: "the questions answered or withdrawn in that window"})
	}
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	if inv.input.switched("answered") {
		return inv.render(inv.answeredQuestions())
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
		Summary: fmt.Sprintf("%d open channel question(s)", len(questions)), view: questionListView(questions)}
	if len(unreadable) > 0 {
		result.Outcome, result.code = intentPartial, 1
	}
	return inv.render(result)
}

// uiTarget is start ui, stop ui and status ui: the interface's own
// lifecycle verbs, refusing options that belong to the checkout.
// questionListView is question list: how many channel questions wait for
// a person, then each as a card with its reference, goal and when it was
// asked, and the question's first line; one question gets its show as the
// hint.
func questionListView(questions []channel.Question) func(*textui.Page) {
	return func(page *textui.Page) {
		env := page.Env()
		if len(questions) == 0 {
			page.Headline("No questions wait for a person")
			return
		}
		page.Headline(textui.Count(len(questions), "question waits", "questions wait") + " for a person")
		section := page.Section("", "")
		for _, q := range questions {
			facts := []string{"channel:" + q.ID}
			if q.Goal != "" {
				facts = append(facts, "goal "+q.Goal)
			}
			facts = append(facts, "asked "+env.Time(q.OpenedAt))
			separator := " · "
			if env.ASCII {
				separator = ", "
			}
			card := section.Item(textui.Alert, strings.Join(facts, separator))
			if len(q.Facts) > 0 {
				card.KV("asks", textui.Plain(q.Facts[0]))
			}
		}
		if len(questions) == 1 {
			page.Hint(textui.Hint{Argv: []string{"metasystem", "question", "show", "channel:" + questions[0].ID}, Reason: "the question and how it is answered"})
		}
	}
}

func (inv *intentInvocation) uiTarget(verb string) int {
	options, optionProblem := inv.uiOptions()
	if optionProblem != nil {
		return inv.render(*optionProblem)
	}
	for _, other := range []string{"temporary-human-word", "review-by", "lineage", "by", "work", "machines", "refresh"} {
		if inv.input.has(other) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("ui %s takes no --%s; nothing was done", verb, other),
				next: inv.retryWith([]string{other}), nextReason: "without --" + other})
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
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: err.Error() + "; nothing was done",
			next: []string{"metasystem", "system", "check"}, nextReason: "names what is wrong here"})
	}
	options.seats = func() uiSeatInventory { return inv.uiOtherSeats(layout) }
	ran, err := inv.owners.processes.ui(verb, roots, options)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: err.Error() + "; nothing was done",
			next: inv.publicArgv("ui", "status"), nextReason: "what the interface is doing now"})
	}
	data := uiSeatsData(ran)
	if ran.Seat != nil {
		// The verb acted on another machine's interface: that seat is the
		// target, and the line that says which is the summary.
		targets = []intentTarget{{Kind: "ui", ID: ran.Seat.Checkout}}
		summary := "the interface of machine " + ran.Seat.Name
		if len(ran.Result.Lines) > 0 {
			summary = ran.Result.Lines[0]
		}
		// The summary is the first line, so the text is the rest.
		text := uiLinesAfterSummary(ran.Result.Lines)
		switch {
		case ran.Result.Code != 0:
			return inv.render(intentResult{Outcome: intentRefused, code: ran.Result.Code, Targets: targets, Data: data, text: text, Summary: summary,
				next: []string{"metasystem", "ui", "status", "--repo", ran.Seat.Checkout}, nextReason: "what that interface is doing now"})
		case ran.Unchanged:
			return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: data, text: text, Summary: summary})
		}
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: data, text: text, Summary: summary})
	}
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
	next, nextReason := inv.publicArgv("ui", "status"), "what the interface is doing now"
	if verb == "start" && len(ran.Seats) == 1 {
		next, nextReason = []string{"metasystem", "ui", "stop", "--repo", ran.Seats[0].Checkout}, "stops the interface of machine "+ran.Seats[0].Machine+", which holds the address"
	}
	return inv.render(intentResult{Outcome: intentRefused, code: ran.Result.Code, Targets: targets, Data: data, text: ran.Result.Lines,
		Summary: "the interface did not " + verb + "; its report is below", next: next, nextReason: nextReason, Decision: ran.Decision})
}

// uiSeatsData is a lifecycle verb's JSON data: its lines and exit code,
// and the other machines of this computer it names, when it read them.
func uiSeatsData(ran uiLifecycleResult) map[string]any {
	data := map[string]any{"lines": nonNilLines(ran.Result.Lines), "exitCode": ran.Result.Code}
	if ran.Seats != nil || ran.SeatsProblems != nil {
		seats := ran.Seats
		if seats == nil {
			seats = []uiSeatView{}
		}
		data["seats"], data["seatsProblems"] = seats, nonNilLines(ran.SeatsProblems)
	}
	return data
}

// runIntentStartMachine adds one machine to the fleet through the seat
// launch owner: clone, build, configure, enroll and supervise.
func runIntentStartMachine(inv *intentInvocation) int {
	if len(inv.input.args) != 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "machine start needs the new machine's name, so nothing was done",
			next: inv.retryWith(nil, "NAME"), nextReason: "with the new machine's name"})
	}
	name := inv.input.args[0]
	for _, other := range []string{"lineage", "installation", "by"} {
		if inv.input.has(other) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("machine start takes no --%s; nothing was done", other),
				next: inv.retryWith([]string{other}), nextReason: "without --" + other})
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
	owner, problem, readErr := inv.engineVerb(seatLaunchVerb, args...)
	if problem != nil {
		return inv.render(*problem)
	}
	result := ownerEnvelopeResult(owner, readErr, []intentTarget{{Kind: "machine", ID: name}}, "machine "+name+" is launched and supervised")
	if owner, _ := result.Data.(map[string]any)["owner"].(map[string]any); result.Outcome == intentConfirmed && owner["alreadyLaunched"] == true {
		result.Outcome = intentUnchanged
		result.Summary = fmt.Sprintf("machine %s is already launched (launch %v, %v at %v)", name, owner["launch"], owner["outcome"], owner["destination"])
	}
	if result.Outcome != intentConfirmed && result.Outcome != intentUnchanged {
		result.next, result.nextReason = inv.publicArgv("machine", "list"), "every machine's presence"
	} else {
		owner, _ := result.Data.(map[string]any)["owner"].(map[string]any)
		result.view = machineStartView(name, result.Outcome == intentUnchanged, owner)
	}
	return inv.render(result)
}

// machineStartView is machine start's page: what happened to the machine,
// then its launch and where its clone is; the owner's record is --json's.
func machineStartView(name string, already bool, owner map[string]any) func(*textui.Page) {
	return func(page *textui.Page) {
		if already {
			page.Done(joinFacts(page, "Machine "+name+" is already launched and supervised", "nothing was done"))
		} else {
			page.Done("Machine " + name + " is launched and supervised")
		}
		section := page.Section("", "")
		if launch, _ := owner["launch"].(string); launch != "" {
			section.KV("launch", textui.Plain(launch))
		}
		if destination, _ := owner["destination"].(string); destination != "" {
			section.KV("clone", textui.Plain(page.Env().Path(destination)))
		}
	}
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
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--runtimes is empty; name the runtimes, or leave the option out; nothing was done",
				next: inv.retryWith([]string{"runtimes"}), nextReason: "the runtimes metasystem.runtimes enables"})
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
	outcome, summary := intentConfirmed, "this checkout is set up: its runtimes, hooks, commit fence and contract merges run the engine"
	if report.Unchanged() {
		outcome, summary = intentUnchanged, "this checkout is already set up; nothing was changed"
	}
	return inv.render(intentResult{Outcome: outcome, Targets: targets, Summary: summary, text: lines,
		Data: map[string]any{"engine": report.Engine, "runtimes": nonNilLines(report.Runtimes), "changed": nonNilLines(report.Changed), "fence": report.Fence, "fenceHook": report.FenceHook, "mergeDriver": report.MergeDriver},
		view: setupView(report)})
}

// setupView is system setup's page: whether the checkout is now set up or
// already was, then its runtimes, the files written, the commit fence and
// the contract merges; the engine and the fence's hook path are
// --verbose's.
func setupView(report hookswitch.Report) func(*textui.Page) {
	return func(page *textui.Page) {
		env := page.Env()
		if report.Unchanged() {
			page.Done("This checkout was already set up to work with its engine; nothing changed")
		} else {
			page.Done("This checkout is set up to work with its engine")
		}
		plain := func(text string) []textui.Span { return []textui.Span{textui.Plain(text)} }
		runtimes := "none registered"
		if len(report.Runtimes) > 0 {
			runtimes = strings.Join(report.Runtimes, ", ")
		}
		facts := []textui.KV{{Key: "runtimes", Value: plain(runtimes)}}
		if len(report.Changed) > 0 {
			facts = append(facts, textui.KV{Key: "written", Value: plain(strings.Join(report.Changed, ", "))})
		}
		fence := map[string]string{hookswitch.FenceReenrolled: "re-enrolled: the hook from before the engine guard now runs the engine",
			hookswitch.FenceEnrolled: "enrolled", hookswitch.FenceNoGit: "none, as there is no git repository"}[report.Fence]
		if fence == "" {
			fence = "already runs the engine"
		}
		facts = append(facts, textui.KV{Key: "commit fence", Value: plain(fence)})
		switch report.MergeDriver {
		case contractgit.DriverRegistered:
			facts = append(facts, textui.KV{Key: "contract merges", Value: plain("through the engine's merge driver")})
		case contractgit.DriverUnchanged:
			facts = append(facts, textui.KV{Key: "contract merges", Value: plain("already through the engine's merge driver")})
		}
		if page.Verbose() {
			facts = append(facts, textui.KV{Key: "engine", Value: plain(env.Path(report.Engine))})
			if report.FenceHook != "" {
				facts = append(facts, textui.KV{Key: "fence hook", Value: plain(env.Path(report.FenceHook))})
			}
		}
		page.Facts(facts...)
	}
}

// diskCheckLine is system check's one line about this checkout's disk pass
// (Part B 3.3): when it last ran and what it released, kept and left
// pending, and where the whole report is.
func diskCheckLine(root string) string {
	report, err := diskstore.ReadReport(diskstore.CheckoutReportPath(root))
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "disk: no pass has run for this checkout yet; metasystem disk clean --preview shows what one would do"
	case err != nil:
		return "disk: the last report is unreadable (" + err.Error() + "); metasystem disk clean writes a fresh one"
	}
	return fmt.Sprintf("disk: last pass %s released %d, kept %d, left %d pending; metasystem disk show prints the report",
		report.At.Format(time.RFC3339), len(report.Actions), len(report.Kept), len(report.Pending))
}

// uiLinesAfterSummary is a result's lines after the first, which the
// rendering prints as the summary.
func uiLinesAfterSummary(lines []string) []string {
	if len(lines) <= 1 {
		return nil
	}
	return lines[1:]
}
