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
				{"merge", "three-way merge a testing contract by surface, group, and list value", runTestingMerge},
				{"add-tests", "add verified Go test names to one testing group", runTestingAddTests},
				{"merge-driver", "run the testing contract Git merge driver, or print its setup", runTestingMergeDriver},
			},
		},
		{
			name:    "test",
			summary: "risk-selected common application testing with retained proof and reuse",
			verbs: []verb{
				{"list", "list every group in the committed testing contract", runTestList},
				{"check", "validate the committed contract and declared tools without running tests", runTestCheck},
				{"plan", "compute the candidate's risk-selected groups without running tests", runTestPlan},
				{"run", "admit and execute the recomputed selected test plan", runTestRun},
				{"verify", "verify sufficient retained proof without launching tests or builds", runTestVerify},
				{"report", "summarize measured cost from one retained test result", runTestReport},
				{"worker-capabilities", "report the installed testing worker protocol (internal)", runTestWorkerCapabilities},
				{"worker", "execute one admitted selected plan (internal)", runTestWorker},
			},
		},
		{
			name:    "brain",
			summary: "the fleet brain seat: designation, boot context, and checkout-local fences",
			verbs: []verb{
				{"declare", "human-only: designate this quiescent checkout as the brain", runBrainDeclare},
				{"show", "show the declared, undeclared, or corrupt brain state", runBrainShow},
				{"withdraw", "human-only: remove this checkout's brain declaration", runBrainWithdraw},
				{"fence", "report whether a guarded act is refused", runBrainFence},
				{"boot", "compose the declared brain's bounded standing context", runBrainBoot},
				{"boot-inputs", "read optional brain boot inputs in the bounded child (internal)", runBrainBootInputs},
			},
		},
		{
			name:    "proof-run",
			summary: "priced validation runs with structural progress and a sibling watchdog",
			verbs: []verb{
				{"banner", "print the suite witness state, duration class, heartbeat, and log paths", runProofRunBanner},
				{"heartbeat", "print the deepest live suite section under a root", runProofRunHeartbeat},
				{"launch", "launch a suite in its own process group with a sibling watchdog", runProofRunLaunch},
				{"worker-authorized", "authenticate a suite worker against its live parent proof", runProofRunWorkerAuthorized},
				{"watchdog", "watch suite output growth and enforce the section ceiling (internal)", runProofRunWatchdog},
				{"custody-exec", "hold a resource-active command until its custodian binds exact identity (internal)", runProofRunCustodyExec},
				{"preserve", "copy bounded watchdog evidence (internal)", runProofRunPreserve},
				{"assert", "assert selector sections produced well-formed start and end events", runProofRunAssert},
				{"coverage-delta", "check coverage for the packages a landing names or touches against their ratchet floors", runProofRunCoverageDelta},
				{"fixture-selection", "select the owned fixture scenario set (internal)", runProofRunFixtureSelection},
			},
		},
		{
			name:    "proc",
			summary: "process identity and census: who is running, provably",
			verbs: []verb{
				{"fixture-survivors", "name or reap fixture children that outlived their owner", runFixtureSurvivors},
				{"started-at", "print a pid's start time in epoch seconds", runIdentityStartedAt},
				{"group-owned", "exit 0 only when a group member carries a tag in a shipped argv position", runIdentityGroupOwned},
				{"census", "compute a fixture-driven census verdict", runCensusRun},
				{"classify", "print live, stale, dead, or unknown for a recorded pid and tag", runProcClassify},
				{"find-ancestor", "walk up the process tree to the first agent-signature ancestor", runCensusFindAncestor},
				{"setsid", "run a command as the leader of a new session and exit with its status (proc setsid -- cmd args...)", runProcSetsid},
				{"default-signals", "run a command with INT, QUIT, and HUP restored to their default disposition", runProcDefaultSignals},
			},
		},
		{
			name:    "config",
			summary: "configuration and identity helpers",
			verbs: []verb{
				{"canonical-model", "print the canonical model key for a name", runConfigCanonicalModel},
				{"get", "resolve a config key with flag/env/local/mode/conf/default precedence", runConfigGet},
				{"validate", "validate the whole metasystem.conf domain", runConfigValidate},
				{"keys", "enumerate config keys, optionally by prefix", runConfigKeys},
				{"conf-value", "print a single conf value (exit 3 absent, 1 on duplicate)", runConfigConfValue},
				{"tailor", "rewrite metasystem.conf in place for a selected runtime set", runConfigTailor},
			},
		},
		{
			name:    "validate",
			summary: "whole-artifact validators the assert scripts exec into",
			verbs: []verb{
				{"turn-prompt", "validate an assembled host-turn prompt against its turn record and the shipped preamble", runValidateTurnPrompt},
				{"plan-consistency", "report retired terms still prescribed in plans", runValidatePlanConsistency},
				{"critique-closed", "join a critic return's findings against the dispositions table", runValidateCritiqueClosed},
				{"preamble-quotes", "verify role-preamble quote blocks are byte-exact substrings of their sources", runValidatePreambleQuotes},
				{"session-isolation", "copy adapter local config into a second-session worktree and audit isolation", runValidateSessionIsolation},
				{"refactor-baseline", "record or check the trusted refactor baseline", runValidateRefactorBaseline},
				{"skills", "validate every present skill's SKILL.md frontmatter, or the named skill directories", runValidateSkills},
				{"return-complete", "validate an agent return against its role schema and job identity", runValidateReturnComplete},
				{"design-obligations", "check the structure and declared state of design-obligation matrices", runValidateDesignObligations},
				{"moved-effects", "check a design page's moved-effect inventory and code paths", runValidateMovedEffects},
				{"conformance", "review, recertify, or merge conformance for an implementer job", runValidateConformance},
				{"stop-loss", "block further investigation cycles when a ledger trigger fired", runValidateStopLoss},
			},
		},
		{
			name:    "landing",
			summary: "classify and record the two bars for a prospective landing",
			verbs: []verb{
				{"batch", "join, status, withdraw, owner, tick, or wait for a guarded landing batch", runLandingBatch},
				{"observe", "emit a provenance verdict for the prospective project tree", runLandingObserve},
				{"workspace", "print the delivery workspace projection of a whole-project tree", runLandingWorkspace},
				{"park", "durably record one stopped recertified landing attempt", runLandingPark},
				{"adoption-rulings", "prepare required landing authority while preserving application rulings", runLandingAdoptionRulings},
				{"test-receipt", "run tests against one exact candidate tree and record their result", runLandingTestReceipt},
			},
		},
		{
			name:    "job",
			summary: "the delegate-job domain: records, chains, locks, caps, snapshots, authority",
			verbs: []verb{
				{"verify-references", "re-read every referenced file against the composition record before a launch", runDispatchVerifyReferences},
				{"resolve-roster", "resolve a role's roster pair and classify escalation", runDispatchResolveRoster},
				{"goal-revision", "print a live accepted goal's revision for reservation binding", runDispatchGoalRevision},
				{"goal-binding", "print a claimed goal's stop-capability binding", runDispatchGoalBinding},
				{"goal-admission", "judge the structured goal budget before reservation", runDispatchGoalAdmission},
				{"goal-revision-admission", "judge one exact revision and proposed cap under its lock", runDispatchGoalRevisionAdmission},
				{"breach-stop", "close a breached revision's fence and initialize its stop batch", runDispatchBreachStop},
				{"breach-stop-routes", "list steward and dispatch breach-stop routes", runDispatchBreachStopRoutes},
				{"stop-batch-reconcile", "advance one stop batch from authoritative job records", runDispatchStopBatchReconcile},
				{"cap-continuation", "write the prior-worktree paragraph a continuation round is told after its predecessor was cut off at its cap", runDispatchCapContinuation},
				{"examination-retry", "admit one fresh examination round after a critic round ended without a return (internal)", runJobExaminationRetry},
				{"critique-register-advance", "fold one critic round into its canonical register", runDispatchCritiqueRegisterAdvance},
				{"critique-register-close", "close or defer a critic register", runDispatchCritiqueRegisterClose},
				{"critique-budget-rebind", "copy the goal review-round limit onto a critic root", runDispatchCritiqueBudgetRebind},
				{"prove-round", "prove a chain round's committed worktree tree on this installation's engine and record the attempt in the round directory", runDispatchProveRound},
				{"owner-lock", "claim or release the dispatch owner lock (0 done, 3 busy, 4 not-owner)", runDispatchOwnerLock},
				{"snapshot-select", "select the capability snapshot matching a dispatch's identity", runCapabilitySelect},
				{"authority-check", "check a control-plane write against the authority matrix", runAuthorityCheck},
				{"watch", "block until a delegate job is terminal; exit with its pinned code", runJobWatchVerb},
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
			name:    "audit",
			summary: "mechanical fences the gate bootstrap consults between steps",
			verbs: []verb{
				{"dependency-ratchet", "refuse undeclared executable interpreter dependencies in shell sources", runAuditDependencyRatchet},
				{"hook-start-exits", "refuse SessionStart paths around the outcome owner", runAuditHookStartExits},
				{"metasystem", "instruction-asset audit: required files, outside references, placeholders, word budgets", runAuditMetasystem},
				{"stop-decision-surface", "report additions and refuse undeclared moves of Stop assertions", runAuditStopDecisionSurface},
				{"production-commands", "name each production command this host lacks, with its package", runAuditProductionCommands},
			},
		},
		{
			name:    "behavior-surface",
			summary: "versioned byte projections shared by witness, landing, adoption, and weight laws",
			verbs: []verb{
				{"select", "filter newline- or NUL-delimited paths through one projection", runBehaviorSurfaceSelect},
				{"skip-allowed", "exit 0 only for a family declared under the caller's witness or delivery scope", runBehaviorSurfaceSkipAllowed},
			},
		},
		{
			name:    "path",
			summary: "path oracles: which law owns and governs a repository path",
			verbs: []verb{
				{"class", "print the manifest class governing one path", runPathClass},
				{"state-root", "validate and print one metasystem installation", runPathStateRoot},
			},
		},
		{
			name:    "gate",
			summary: "unit validation and gate-run state",
			verbs: []verb{
				{"unit", "gate changed Go packages and every transitive reverse dependent", runGateUnit},
				{"register", "record that this process is a running gate", runGateRegister},
				{"fence", "exit 1 naming every live gate run foreign to --self-pid's chain", runGateFence},
				{"weight-discharge", "reset validation weight at the exact authorized green-run boundary", runGateWeightDischarge},
				{"cadence-tick", "run one landing-owner deep validation cadence decision", runGateCadenceTick},
			},
		},
		{
			name:    "report",
			summary: "turn-end report decisions",
			verbs: []verb{
				{"stop-status", "read one exact immutable Stop report", runReportStopStatus},
				{"turn-verdict", "the one structured turn-end decision: scan, goal, block-once state", runReportTurnVerdict},
				{"open-work", "report plans with an unblocked next step and no job in flight", runReportOpenWork},
				{"running-work", "print the turn-end active clause: live jobs, missions, gate runs", runReportRunningWork},
				{"watch-jobs", "watch background job records and report every reportable job", runReportWatchJobs},
				{"frontier", "record, challenge, or show the measured-improvement frontier", runReportFrontier},
			},
		},
		{
			name:    "receipt",
			summary: "the task-receipt ledger and retro cadence",
			verbs: []verb{
				{"add", "append one task receipt at completion", func(args []string) int { return runReceipt(append([]string{"add"}, args...)) }},
				{"correct", "append a correction referencing an existing receipt line", func(args []string) int { return runReceipt(append([]string{"correct"}, args...)) }},
				{"check", "exit 1 when a metasystem retro is due", func(args []string) int { return runReceipt(append([]string{"check"}, args...)) }},
				{"stats", "print the period numbers as key=value lines", func(args []string) int { return runReceipt(append([]string{"stats"}, args...)) }},
				{"retro", "record that a retro ran and reset the cadence", func(args []string) int { return runReceipt(append([]string{"retro"}, args...)) }},
			},
		},
		{
			name:    "metrics",
			summary: "actionable process, proof, lifecycle, delegation, collision, and cost measures",
			verbs: []verb{
				{"report", "compute and atomically publish a period or per-goal report", runMetricsReport},
			},
		},
		{
			name:    "schema",
			summary: "role-return schema materialization",
			verbs: []verb{
				{"materialize", "write a role's return schema at a version", runSchemaMaterialize},
			},
		},
		{
			name:    "runtime",
			summary: "the declared agent-runtime registry (list, lookups)",
			verbs: []verb{
				{"setup", "install or check host runtime entry points without selecting an active runtime", runRuntimeSetup},
				{"list", "runtime names in priority order (--adoptable/--with-* filter)", runRuntimeList},
				{"signature-vectors", "a runtime's declared positive/lookalike process vectors", runRuntimeSignatureVectors},
				{"collision-roots", "the deduplicated full population of adoption collision roots", runRuntimeCollisionRoots},
				{"enforcement-map", "a runtime's static envelope-enforcement map as canonical JSON", runRuntimeEnforcementMap},
				{"registration", "a runtime's declared registration rows (registration/v1 wire)", runRuntimeRegistration},
				{"adoption-default", "the one default adoption runtime", runRuntimeAdoptionDefault},
				{"dirs", "a runtime's adopted registration directories", runRuntimeDirs},
				{"instruction-file", "a runtime's instruction-bearing filename", runRuntimeInstructionFile},
			},
		},
		{
			name:    "launch",
			summary: "owned external agent processes with durable state and exact cancellation",
			verbs: []verb{
				{"start", "start an agent process and return after its child is recorded", runLaunchStart}, {"supervise", "own one launch child through its terminal state (internal)", runLaunchSupervise}, {"wait", "wait up to a bounded timeout for a terminal launch", runLaunchWait}, {"status", "show and reconcile one launch", runLaunchStatus}, {"cancel", "end a launch group and prove every recorded process dead", runLaunchCancel}, {"round-task", "derive a critique task from its predecessor", runLaunchRoundTask}, {"settings", "show launch and context settings with their sources", runLaunchSettings}, {"report", "summarize launch outcomes and refusals", runLaunchReport},
			},
		},
		{
			name:    "output",
			summary: "bounded command output retained under the control root",
			verbs: []verb{
				{"prune", "remove retained output files older than a duration", runOutputPrune},
			},
		},
		{
			name:    "context",
			summary: "the coordinator's recorded provider-call context budget",
			verbs: []verb{
				{"status", "show the holder's current context-budget evidence", runContextStatus},
				{"report", "publish one seven-day UTC context cohort", runContextReport},
				{"handoff", "record a noted task-aware handoff, or cancel one", runContextHandoff},
				{"verify", "verify an immutable coordinator handoff", runContextVerify},
				{"prune", "retire old coordinator context evidence", runContextPrune},
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
				{"sha256", "print the hex sha-256 of --file or stdin", runUtilSHA256},
				{"now-ns", "print the current wall-clock time in nanoseconds", runUtilNowNs},
				{"hold", "stay alive carrying --tag until SIGTERM, then write the stopped file", runUtilHold},
			},
		},
		{
			name:    "event",
			summary: "append a flight-recorder event",
			verbs: []verb{
				{"emit", "append one event (key=value args); best-effort, never fails", runEventEmit},
			},
		},
		{
			name:    "json",
			summary: "JSON field access for shell callers",
			verbs: []verb{
				{"object", "build a compact JSON object from key=value args", runJSONObject},
			},
		},
		{
			name:    "covenant",
			summary: "the app's covenant: the versioned declaration binding intent to proofs",
			verbs: []verb{
				{"validate", "structural check of a covenant document (shape only; adequacy is never a parser's to prove)", runCovenantValidate},
				{"evidence", "the traceability gate: every requirement backed by the evidence table, declared deps present (statuses stay claims)", runCovenantEvidence},
			},
		},
		{
			name:    "project",
			summary: "the project's memory: intent, doctrine, decisions, designs and open questions, declared in their records",
			verbs: []verb{
				{"id", "print a fresh ULID for a new record", runProjectID},
				{"check", "refuse every fault in the homes, anchored at its file and line", runProjectCheck},
				{"design-of", "the design records that name one ledger goal, or the refusal that there are none", runProjectDesignOf},
			},
		},
		{
			name:    "channel",
			summary: "fleet status and authenticated question threads",
			verbs: []verb{
				{"status", "compose or post this machine's durable status", runChannelStatus},
				{"ask", "open one durable question thread", runChannelAsk},
				{"show", "show one question record", runChannelShow},
				{"wait", "wait for one recorded answer", runChannelWait},
				{"poll", "receive and durably disposition replies", runChannelPoll},
				{"fake", "fixture-only fake serve and code verbs", runChannelFake},
			},
		},
		{
			name:    "goal",
			summary: "the goal ledger: the thread of intent that survives every turn (D67)",
			verbs: []verb{
				{"branch", "inspect, commit, land, verify, and sweep goal branches", runGoalBranch},
				{"handover", "transfer this holder's claim to the guarded landing owner", runGoalHandover},
				{"open", "declare a goal; Current when none exists, queued otherwise", runGoalOpen},
				{"abandon", "human-only: record that a goal will never be worked and why; retained with its reason, satisfies no dependency, never pruned, reopenable", runGoalAbandon},
				{"carry", "human-only: name the live successor carrying an abandoned goal, or carry a landing past one named refusal or testing group", runGoalCarry},
				{"set-next", "rewrite the Current goal's next step", runGoalSetNext},
				{"read-items", "record, close, or list non-breaking read findings", runGoalReadItems},
				{"promote", "move a queued goal to Current", runGoalPromote},
				{"park", "park a goal; parking the Current one requires --then or --and-none", runGoalPark},
				{"unpark", "return a parked goal to the queue", runGoalUnpark},
				{"block", "record that one goal waits for another (--id waits, --blocker is waited for); it parks unless the blocker is already done", runGoalBlock},
				{"unblock", "remove one blocker edge; the park lifts only when the dependency created it and every remaining blocker is done, and removing an unfinished blocker is a human act", runGoalUnblock},
				{"done", "conclude the Current goal; requires --then or --and-none", runGoalDone},
				{"reopen", "return a done goal or an abandoned goal, as a proven human act, to the queue with a fresh --next", runGoalReopen},
				{"declare-free", "declare (or renew) the absence of intent over the current plans world", runGoalDeclareFree},
				{"prune", "drop done goals beyond the newest ten, reporting every drop", runGoalPrune},
				{"claim", "claim a goal (or its whole arc with --arc) for this machine", runGoalClaim},
				{"restamp", "move this holder's stop capability to the live lease epoch", runGoalRestamp},
				{"approve", "human-only (or a seat --under a power of attorney): approve a goal for execution (its box goes through goal budget), or run the grandfather sweep", runGoalApprove},
				{"budget", "human-only: give a live goal a box (1d/10/720m/1/3, norm or keep); approves, re-approves or rebinds as the goal's state requires", runGoalBudget},
				{"classify-sweep", "human-confirmed classification of every open tierless goal", runGoalClassifySweep},
				{"tier-probe", "report the backlog's recorded and derived tiers and the goals a person may lower", runGoalTierProbe},
				{"unapprove", "human-only: withdraw execution approval and park any standing claim", runGoalUnapprove},
				{"set-budget", "human-only long form of goal budget, kept for one release (or a seat --under a power of attorney)", runGoalSetBudget},
				{"extend-budget", "extend attempts and reserved minutes once after accepted advancement consumes the box", runGoalExtendBudget},
				{"grant", "human-only: record a power of attorney (tiers 1 and 2; approve, set-budget; expires within seven days) a seat acts under with --under", runGoalGrant},
				{"revoke", "human-only: close a power of attorney early", runGoalRevoke},
				{"carrying", "reserve a carry or create its local pre-push intent", runGoalCarrying},
				{"carried", "complete or repair a carried landing record", runGoalCarried},
				{"accept-risk", "human-only: accept one severe or unproven critic finding", runGoalAcceptRisk},
				{"discharge-review-obligation", "discharge one chain-qualified review obligation", runGoalDischargeReviewObligation},
				{"split", "atomize one goal into independently claimable arc members; conclude the parent as decomposed", runGoalSplit},
				{"set-obligation", "human-only: bind a governed recurrence and typed assumptions to the existing budget", runGoalSetObligation},
				{"enroll-terminal", "enroll this agent-free interactive terminal for human-only goal authority", runGoalEnrollTerminal},
				{"resume", "human-only: resume a stopped goal under its standing valid approval", runGoalResume},
				{"release", "release this machine's claim (or the arc's with --arc)", runGoalRelease},
				{"steal", "take over another machine's claim (displacement-bearing)", runGoalSteal},
				{"trunk-red", "own or close a trunk-red register entry (own is an agent act; close, and own --by, are human acts)", runGoalTrunkRed},
				{"land-ready", "mark this machine's claimed goal as built and waiting to land; the claim leaves the one-claim quota and its elapsed fence until it lands", runGoalLandReady},
				{"edit", "edit a goal's intent or next step in place", runGoalEdit},
				{"set-arc", "move a goal into an arc under the membership rules", runGoalSetArc},
				{"set-pin", "pin a goal to one machine (\"-\" clears); only that machine may claim it", runGoalSetPin},
				{"set-priority", "human-only: place an open goal in priority 1, 2, or 3 and re-sequence its peers", runGoalSetPriority},
				{"detach", "take a goal out of its arc (releases a riding claim)", runGoalDetach},
				{"list", "print a bounded ledger summary (--json for the records without history, --json --history for the full records, --done to list archived goals)", runGoalList},
				{"show", "print one goal without ledger history (--history for the full record)", runGoalShow},
				{"next", "select ordered claimable work for one machine (read-only)", runGoalNext},
				{"reconcile", "adopt, restore, or authority-replay bytes the verbs did not write", runGoalReconcile},
				{"migrate", "the cutover: one commit turns the legacy ledger into the multi-machine tree (human act, reviewed bytes)", runGoalMigrate},
				{"fetch", "the read-side advance: validate the canonical tip and move the accepted ref", runGoalFetch},
				{"repair", "human-only: accept a non-descending canonical tip with --accept-remote", runGoalRepair},
				{"source-digest", "print goals.md's sha256 — the reviewed literal the migration demands", runGoalSourceDigest},
				{"recover", "run the one recovery rule over the journal: confirm, correct, complete, or close every stranded entry", runGoalRecover},
			},
		},
		{
			name:    "session",
			summary: "the announced main session's restart orientation and human-only stop authority",
			verbs: []verb{
				{"start", "print durable wait recovery commands for this holder session", runSessionStart},
				{"stop", "human-only: authorize one quiet stop for the current announced main session", runSessionStop},
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
				{"status", "the operator's view: evidence age, live intents, pending notifications", runStewardStatus},
				{"authorize-dispatch", "gate the unattended continuation: steward caller, consumed unstamped intent, staged tuple out", runStewardAuthorizeDispatch},
				{"revive", "one revival end to end: stage, mint, arbitrate, dispatch once", runStewardRevive},
				{"run", "the runner's body: tick until disarmed (spawned by arm; callable by any external ticker)", runStewardRun},
				{"arm", "explicit human enrollment and runner start (long form of metasystem system start)", runStewardArm},
				{"restart", "replace and re-arm the runner (long form of metasystem system start)", runStewardRestart},
				{"disarm", "end the runner", runStewardDisarm},
				{"hook-complete", "record a supervision-hook completion after payload emission (internal)", runStewardHookComplete},
			},
		},
		{
			name:    "run",
			summary: "tracked long-running work: launch, watch, conclude (the monitor facility)",
			verbs: []verb{
				{"launch", "reserve, spawn the wrapped command detached, print the watch line", runRunLaunch},
				{"wrap", "the setsid leader: bind, run the workload, write the exit sidecar (internal)", runRunWrap},
				{"watch", "block until the run is terminal; exit with its pinned code", runRunWatch},
			},
		},
		{
			name:    "lease",
			summary: "checkout write-authority: announce/classify/hold/renew",
			verbs: []verb{
				{"announce", "record this process as a main and claim the checkout lease", runLeaseAnnounce},
				{"retire", "remove this process's announcement", runLeaseRetire},
				{"classify", "classify a caller and report holdership as JSON", runLeaseClassify},
				{"require-holder", "gate a write on the caller being the authenticated holder", runLeaseRequireHolder},
				{"run-held", "run a command while holding the lease lock (gated on holdership)", runLeaseRunHeld},
			},
		},
		{
			name:    "mission",
			summary: "the mission domain: state, fences, contract, prompt, runner, turns, ledger",
			verbs: []verb{
				{"state-init", "create a mission's initial state from its sealed contract", runMissionStateInit},
				{"state-write", "advance the state via a compare-and-write on its hash", runMissionStateWrite},
				{"state-verify", "validate the state's shape, aggregation, hash chain, and anchor", runMissionStateVerify},
				{"state-anchor", "write the local anchor commit binding the state hash and ledger", runMissionStateAnchor},
				{"state-reconcile", "reconcile the state against its ledger and anchor, parking on disagreement", runMissionStateReconcile},
				{"fence-reserve-job", "check the job fences and reserve the job", runMissionFenceReserve("fence-reserve-job", true)},
				{"fence-authorize-cap", "authorize a per-job cap for a runtime/model pair", runMissionFenceAuthorizeCap},
				{"fence-aggregate-usage", "aggregate typed usage across the mission's finished jobs", runMissionFenceAggregateUsage},
				{"fence-refuse", "raise a batched fence ask for a reason", runMissionFenceRefuse},
				{"fence-release-job", "release a husked dispatch's fence reservation", runMissionFenceReleaseJob},
				{"contract-validate", "validate a mission contract's authored block", runMissionContractValidate},
				{"contract-seal", "seal a validated contract and print its digest", runMissionContractSeal},
				{"contract-hash", "print a contract's canonical signed-bytes digest", runMissionContractHash},
				{"contract-preflight", "preflight a sealed, signed contract and emit its verified bytes", runMissionContractPreflight},
				{"contract-envelope-allows", "exit 0 when the signed contract's dispatch-allow carries a pair", runMissionContractEnvelopeAllows},
				{"prompt-assemble", "assemble the byte-stable host-turn prompt", runMissionPromptAssemble},
				{"start", "start a mission's detached run loop", runMissionRunnerStart},
				{"resume", "resume a parked or interrupted mission", runMissionRunnerResume},
				{"status", "print the mission's runner status line", runMissionRunnerStatus},
				{"answer", "record a human answer to an open ask", runMissionRunnerAnswer},
				{"resolve-taint", "apply a typed human resolution (--restore <treeId> | --adopt --waives <claim>) to a workspace taint", runMissionRunnerResolveTaint},
				{"run-loop", "the detached mission loop (internal; spawned by start/resume)", runMissionRunnerRunLoop},
				{"ledger-init", "create a ledger with cycle and no-gain budgets", runMissionLedgerInit},
			},
		},
		{
			name:    "supervise",
			summary: "the supervision lifecycle (docs/design/supervision-lifecycle.md)",
			verbs: []verb{
				{"owner", "run the owner loop for a checkout (internal; launched by up)", runSuperviseOwnerLoop},
				{"component", "run a supervised component (internal; launched by the owner)", runSuperviseComponent},
				{"status", "print the checkout's supervision state as JSON", runSuperviseStatus},
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
	if args[0] == "stop" {
		return runProcessStop(args[1:])
	}
	if args[0] == "status" {
		return runProcessStatus(args[1:])
	}
	if args[0] == "arm" {
		return runProcessArm(args[1:])
	}
	if args[0] == "health" {
		if len(args) > 1 && args[1] == "acknowledge-alert" {
			return runHealthAcknowledgeAlert(args[2:])
		}
		return runStewardHealth(args[1:])
	}
	if args[0] == "watch" {
		return runWatch(args[1:])
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
		fmt.Fprintln(w, "example: metasystem internal launch status --id <id>")
		fmt.Fprintln(w, "example: metasystem internal launch wait --id <id> [--timeout <duration>]")
		fmt.Fprintln(w, "example: metasystem internal launch cancel --id <id>")
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
	fmt.Fprintln(w, "       metasystem internal stop [--repo <path>] [--installation <dir>] [--all]")
	fmt.Fprintln(w, "       metasystem internal status [--repo <path>] [--installation <dir>] [--all]")
	fmt.Fprintln(w, "       metasystem internal arm [--repo <path>] [--installation <dir>] [--all] [--temporary-human-word <word> --review-by <date>]")
	fmt.Fprintln(w, "       metasystem internal health --repo <checkout>")
	fmt.Fprintln(w, "       metasystem internal health acknowledge-alert --episode <id> [--repo <checkout>]")
	fmt.Fprintln(w, "       metasystem internal watch [--root <checkout>] [--json]")
	fmt.Fprintln(w, "       metasystem internal watch --job <id> [--root <checkout>] [--poll-ms <milliseconds>]")
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
