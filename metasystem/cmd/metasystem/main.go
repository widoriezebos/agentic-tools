// Command metasystem is the metasystem's one binary. Operator verbs such as
// up and health route directly; internal families group the narrower
// decisions that plumbing invokes. Compatibility wrappers keep historical
// names only long enough to exec into these verbs. File naming is one file
// per routed surface; cross-family helpers live in helpers.go and nowhere
// else.
package main

import (
	"fmt"
	"os"
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
			name:    "proof-run",
			summary: "priced validation runs with structural progress and a sibling watchdog",
			verbs: []verb{
				{"banner", "print the suite witness state, duration class, heartbeat, and log paths", runProofRunBanner},
				{"heartbeat", "print the deepest live suite section under a root", runProofRunHeartbeat},
				{"launch", "launch a suite in its own process group with a sibling watchdog", runProofRunLaunch},
				{"watchdog", "watch suite output growth and enforce the section ceiling (internal)", runProofRunWatchdog},
				{"preserve", "copy bounded watchdog evidence (internal)", runProofRunPreserve},
				{"assert", "assert selector sections produced well-formed start and end events", runProofRunAssert},
			},
		},
		{
			name:    "proc",
			summary: "process identity and census: who is running, provably",
			verbs: []verb{
				{"started-at", "print a pid's start time in epoch seconds", runIdentityStartedAt},
				{"probe", "print a pid's exact identity as JSON", runIdentityProbe},
				{"exists", "exit 0 if the pid exists (permission denial proves existence)", runIdentityExists},
				{"group-exists", "exit 0 if the process group exists", runIdentityGroupExists},
				{"group-owned", "exit 0 only when a group member carries a tag in a shipped argv position", runIdentityGroupOwned},
				{"group-members", "print a process group's live member pids, optionally excluding one", runProcGroupMembers},
				{"census", "compute a fixture-driven census verdict", runCensusRun},
				{"alive", "exit 0 if a pid is live at its expected start", runCensusAlive},
				{"classify", "print live, stale, dead, or unknown for a recorded pid and tag", runProcClassify},
				{"signature-check", "verify an adapter's positive/lookalike signature contract", runCensusSignatureCheck},
				{"find-ancestor", "walk up the process tree to the first agent-signature ancestor", runCensusFindAncestor},
				{"acknowledge", "record one exact untracked pid as human-judged-harmless; the end-of-turn report then stays silent about it (KI-23)", runProcAcknowledge},
			},
		},
		{
			name:    "config",
			summary: "configuration and identity helpers",
			verbs: []verb{
				{"canonical-model", "print the canonical model key for a name", runConfigCanonicalModel},
				{"identity", "print an adapter's canonical configuration identity", runConfigIdentity},
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
				{"wrapper-token", "prove the caller's ancestry contains the live commit wrapper", runValidateWrapperToken},
				{"session-isolation", "copy adapter local config into a second-session worktree and audit isolation", runValidateSessionIsolation},
				{"refactor-baseline", "record or check the trusted refactor baseline", runValidateRefactorBaseline},
				{"return-complete", "validate an agent return against its role schema and job identity", runValidateReturnComplete},
				{"design-obligations", "check the structure and declared state of design-obligation matrices", runValidateDesignObligations},
				{"conformance", "review or merge conformance for an implementer job", runValidateConformance},
				{"stop-loss", "block further investigation cycles when a ledger trigger fired", runValidateStopLoss},
			},
		},
		{
			name:    "landing",
			summary: "classify and record the two bars for a prospective landing",
			verbs: []verb{
				{"observe", "emit a non-refusing provenance verdict for the prospective project tree", runLandingObserve},
			},
		},
		{
			name:    "job",
			summary: "the delegate-job domain: records, chains, locks, caps, snapshots, authority",
			verbs: []verb{
				{"compose-role-packet", "assemble a role's closed packet and provenance record", runDispatchComposeRolePacket},
				{"operation-id", "derive the v2 default delegate operation identity", runDispatchOperationID},
				{"record-create", "reserve a job by writing its pending-setup record", runDispatchRecordCreate},
				{"record-setup", "complete a reservation into the full pending record", runDispatchRecordSetup},
				{"record-cas", "compare-and-swap a record's status and fields", runDispatchRecordCAS},
				{"record-protocol-error", "stamp a job failed with a protocol violation", runDispatchRecordProtocolError},
				{"repair-claim", "atomically claim the round's one paid repair (0 won, 3 lost, 1 mechanical)", runDispatchRepairClaim},
				{"build-setup", "assemble a pending-setup reservation record", runDispatchBuildSetup},
				{"resolve-roster", "resolve a role's roster pair and classify escalation", runDispatchResolveRoster},
				{"serving-goal", "print the brief section projecting the current goal (exit 3 when none is usable)", runDispatchServingGoal},
				{"goal-revision", "print a live accepted goal's revision for reservation binding", runDispatchGoalRevision},
				{"goal-binding", "print a claimed goal's stop-capability binding", runDispatchGoalBinding},
				{"goal-lock-path", "print the ranked lock path for one goal revision", runDispatchGoalLockPath},
				{"goal-admission", "judge the structured goal budget before reservation", runDispatchGoalAdmission},
				{"goal-revision-admission", "judge one exact revision and proposed cap under its lock", runDispatchGoalRevisionAdmission},
				{"slice-admission", "judge a reservation cap against the configured slice norm", runDispatchSliceAdmission},
				{"breach-stop", "close a breached revision's fence and initialize its stop batch", runDispatchBreachStop},
				{"breach-stop-routes", "list steward and dispatch breach-stop routes", runDispatchBreachStopRoutes},
				{"stop-batch-reconcile", "advance one stop batch from authoritative job records", runDispatchStopBatchReconcile},
				{"stop-batch-pending", "list the matching non-terminal jobs in a stop batch", runDispatchStopBatchPending},
				{"stop-cancel-authorize", "authorize cancellation of one exact stop-batch job", runDispatchStopCancelAuthorize},
				{"build-record", "assemble the full pending job record", runDispatchBuildRecord},
				{"build-follow-record", "assemble a follow-up round's record from its parent", runDispatchBuildFollowRecord},
				{"verify-chain-incarnation", "refuse a follow-up whose mission was re-provisioned", runDispatchVerifyChainIncarnation},
				{"latest-chain-record", "print the newest record path in a job chain", runDispatchLatestChainRecord},
				{"chain-members", "list a chain's jobs and statuses", runDispatchChainMembers},
				{"chain-usage", "aggregate a chain's usage (exit 7 when unchanged)", runDispatchChainUsage},
				{"custody-add", "append a custody process to a job record under its lock", runDispatchCustodyAdd},
				{"claim-launch", "reserve a launch operation through the typed claim state machine", runDispatchClaimLaunch},
				{"launch-capability-consume", "verify and spend one admitted adapter launch capability", runDispatchLaunchCapabilityConsume},
				{"claim-occupancy-prepare", "prepare session-occupancy evidence off the record lock", runDispatchClaimOccupancyPrepare},
				{"prefork-mark", "persist the pre-fork custody marker for an imminent launch", runDispatchPreforkMark},
				{"custody-groups", "print a record's custody process-group kill targets", runDispatchCustodyGroups},
				{"reconcile-reservation", "run the adoption engine over one reservation", runDispatchReconcileReservation},
				{"ownership-patch", "build the launch ownership patch with a proven identity", runDispatchOwnershipPatch},
				{"handshake-eval", "evaluate a handshake into its record patch", runDispatchHandshakeEval},
				{"reap-facts", "print a record's reap verdict facts", runDispatchReapFacts},
				{"census-fresh", "require a fresh successful census for dispatch", runDispatchCensusFresh},
				{"watcher-ceiling", "print the attested watcher ceiling", runDispatchWatcherCeiling},
				{"expand-permissions", "expand a role's permission preset for a workspace", runDispatchExpandPermissions},
				{"validate-mission", "validate a mission id and lease for dispatch", runDispatchValidateMission},
				{"mirror", "mirror a job's evidence with a manifest", runDispatchMirror},
				{"close-check", "validate a chain is closable", runDispatchCloseCheck},
				{"review-reference-reconcile", "derive a pre-stamping review evidence pointer", runDispatchReviewReferenceReconcile},
				{"critique-register-advance", "fold one critic round into its canonical register", runDispatchCritiqueRegisterAdvance},
				{"critique-open-finding-ids", "print a critic register's open finding identifiers", runDispatchCritiqueOpenFindingIDs},
				{"critique-exhaustion-advance", "atomically advance register-backed critique exhaustion", runDispatchCritiqueExhaustionAdvance},
				{"cap-resolution", "write a cap-resolution record", runDispatchCapResolution},
				{"resolve-cap", "resolve the non-mission cap chain or refuse an unsigned mission cap", runDispatchResolveCap},
				{"brief-mode", "check a brief names a known mode", runDispatchBriefMode},
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
				{"root-job", "print a job's root ancestor by walking parentJob", runAdapterRootJob},
				{"effective-init", "materialize the effective permissions from a job record", runAdapterEffectiveInit},
				{"effective-workspace", "pin the effective writeRoots to the resolved workspace", runAdapterEffectiveWorkspace},
				{"permission-check", "report which effective permission fields are wider than requested", runAdapterPermissionCheck},
				{"model-patch", "write an {effectiveModel} record patch", runAdapterModelPatch},
				{"transport-patch", "write a {transport} record patch (the D82 chain pin)", runAdapterTransportPatch},
				{"repairs-patch", "write a {returnRepairs} record patch", runAdapterRepairsPatch},
				{"result-patch", "write an {error,phase,usage} record patch", runAdapterResultPatch},
				{"capability-snapshot", "write a validated capability snapshot", runAdapterCapabilitySnapshot},
				{"version-parse", "extract the semver from CLI version output on stdin", runAdapterVersionParse},
				{"codex-event", "read the session or turn field from a Codex event stream", runAdapterCodexEvent},
				{"codex-usage", "extract Codex usage from its event stream", runAdapterCodexUsage},
				{"codex-command", "build the Codex delegate argv (NUL-terminated)", runAdapterCodexCommand},
				{"claude-command", "build the Claude argv (NUL-terminated)", runAdapterClaudeCommand},
				{"claude-derive-result", "derive claude-result.json from a streamed round", runAdapterClaudeDeriveResult},
				{"claude-settings", "build the Claude job settings from a record", runAdapterClaudeSettings},
				{"claude-usage", "extract Claude usage from its result", runAdapterClaudeUsage},
				{"claude-result-field", "read a Claude result field with modelUsage collapse", runAdapterClaudeResultField},
				{"claude-read-roots", "list a record's readRoots minus the workspace", runAdapterClaudeReadRoots},
				{"claude-append-result", "append a Claude result to the event stream", runAdapterClaudeAppendResult},
				{"claude-session-signal", "record the Claude session-established signal", runAdapterClaudeSessionSignal},
				{"devin-config", "build the Devin job config from the user config", runAdapterDevinConfig},
				{"adjudicate-turn", "decide a turn's terminal outcome, repair, or settle stage", runAdapterAdjudicateTurn},
				{"devin-session", "correlate the new Devin session against the baseline", runAdapterDevinSession},
				{"devin-settle", "certify the transcript session and derive the effective model", runAdapterDevinSettle},
				{"devin-collect", "walk the delivery channels for a devin turn (0 delivered, 3 empty, 5 oversize)", runAdapterDevinCollect},
				{"devin-usage", "compute the Devin per-round usage delta", runAdapterDevinUsage},
				{"acp-usage", "the acp transport's typed usage from a turn outcome", runAdapterACPUsage},
				{"usage-unavailable", "write the unavailable-usage record", runAdapterUsageUnavailable},
				{"fake-return", "write the fake runtime's canned role return", runAdapterFakeReturn},
				{"fake-usage", "write the fake runtime's fixed native usage", runAdapterFakeUsage},
				{"fake-effective-network", "edit the effective network for permission fixtures", runAdapterFakeEffectiveNetwork},
				{"fake-guarded-write", "attempt a permission-guarded write (77 = refused)", runAdapterFakeGuardedWrite},
				{"fake-guarded-network", "attempt a permission-guarded connection (77 = refused)", runAdapterFakeGuardedNetwork},
				{"fake-capability-snapshot", "write a fake capability-snapshot profile", runAdapterFakeCapabilitySnapshot},
				{"fake-selftest-record", "write the fake selftest pass record", runAdapterFakeSelftestRecord},
				{"normalize-return", "normalize the runtime reply into return.json/return.md", runAdapterNormalizeReturn},
				{"selftest-usage", "assert a selftest job's typed usage", runAdapterSelftestUsage},
				{"selftest-envelope", "print the newest snapshot's envelope declaration for a field", runAdapterSelftestEnvelope},
				{"selftest-record", "write the selftest pass record", runAdapterSelftestRecord},
				{"selftest-listener", "one-shot loopback listener for the denied-fetch probe", runAdapterSelftestListener},
				{"selftest-run", "run the full-contract adapter self-test", runAdapterSelftestRun},
				{"devin-prompt", "write the schema-augmented prompt copy the Devin CLI reads", runAdapterDevinPrompt},
			},
		},
		{
			name:    "host",
			summary: "host-loop plumbing: result envelopes, usage, and return extraction",
			verbs: []verb{
				{"result-write", "write a host turn's result envelope", runHostResultWrite},
				{"finish", "adjudicate a host turn outcome and write its envelope", runHostFinish},
				{"devin-collect", "walk a devin host turn's delivery channels (0 delivered, 3 empty, 5 oversize)", runHostDevinCollect},
				{"json-compact", "print a JSON file as one line", runHostJSONCompact},
				{"claude-result", "extract the Claude return and usage", runHostClaudeResult},
				{"devin-config", "assemble the Devin job config", runHostDevinConfig},
				{"devin-return", "extract the Devin return", runHostDevinReturn},
				{"devin-usage", "compute the Devin per-round usage delta", runHostDevinUsage},
				{"fake-return", "write the fake-runtime return and terminal record", runHostFakeReturn},
				{"fake-result", "write the fake-runtime result envelope", runHostFakeResult},
			},
		},
		{
			name:    "audit",
			summary: "mechanical fences the gate bootstrap consults between steps",
			verbs: []verb{
				{"coverage-ratchet", "judge go test -cover output against the checked-in per-package floors", runAuditCoverageRatchet},
				{"metasystem", "instruction-asset audit: required files, outside references, placeholders, word budgets", runAuditMetasystem},
			},
		},
		{
			name:    "behavior-surface",
			summary: "versioned byte projections shared by witness, landing, adoption, and weight laws",
			verbs: []verb{
				{"policy", "print the embedded versioned policy", runBehaviorSurfacePolicy},
				{"classify", "classify one path and optionally report projection membership", runBehaviorSurfaceClassify},
				{"select", "filter newline- or NUL-delimited paths through one projection", runBehaviorSurfaceSelect},
				{"list", "list existing members of one projection", runBehaviorSurfaceList},
				{"digest", "print a projection-scoped digest report naming its endpoint", runBehaviorSurfaceDigest},
				{"skip-allowed", "exit 0 only for a family declared under the caller's witness or delivery scope", runBehaviorSurfaceSkipAllowed},
			},
		},
		{
			name:    "path",
			summary: "path oracles: which law owns and governs a repository path",
			verbs: []verb{
				{"owner", "print <ownership> <mode>: metasystem-generic, app-owned, runtime, or outside", runPathOwner},
				{"class", "print the manifest class governing one path", runPathClass},
			},
		},
		{
			name:    "gate",
			summary: "gate-run markers: know when a gate is in flight",
			verbs: []verb{
				{"register", "record that this process is a running gate", runGateRegister},
				{"check", "print 1 when a gate is running in this checkout, else 0", runGateCheck},
				{"fence", "exit 1 naming every live gate run foreign to --self-pid's chain", runGateFence},
				{"controller-descendant", "exit 0 only when a consumer descends from one exact live controller identity", runGateControllerDescendant},
				{"witness-freeze", "export a stable private tree and print its manifest digest and path", runGateWitnessFreeze},
				{"witness-verify", "compare a tree's manifest digest with a witness", runGateWitnessVerify},
				{"guard-acquire", "wait for exclusive checkout execution or join the caller's owning chain", runGateGuardAcquire},
				{"guard-release", "release the invoking process's checkout execution membership", runGateGuardRelease},
				{"weight-add", "fold a landing's measured weight into the validation accumulator (numstat on stdin)", runGateWeightAdd},
				{"weight-check", "exit 1 when accumulated feature weight makes governed direct validation due", runGateWeightCheck},
				{"weight-discharge", "reset validation weight at the exact authorized green-run boundary", runGateWeightDischarge},
			},
		},
		{
			name:    "report",
			summary: "turn-end report decisions",
			verbs: []verb{
				{"stop-block", "print the stop-hook block that refuses to end a turn with idle open work", runReportStopBlock},
				{"turn-verdict", "the one structured turn-end decision: scan, goal, block-once state", runReportTurnVerdict},
				{"open-work", "report plans with an unblocked next step and no job in flight", runReportOpenWork},
				{"running-work", "print the turn-end active clause: live jobs, missions, gate runs", runReportRunningWork},
				{"scan-jobs", "one watcher classification pass over job files", runReportScanJobs},
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
			name:    "counselor",
			summary: "read-only drift counsel from durable records",
			verbs: []verb{
				{"brief", "print the current narrative drift brief in dry-run mode", runCounselorBrief},
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
				{"list", "runtime names in priority order (--adoptable/--with-* filter)", runRuntimeList},
				{"signature-vectors", "a runtime's declared positive/lookalike process vectors", runRuntimeSignatureVectors},
				{"collision-roots", "the deduplicated full population of adoption collision roots", runRuntimeCollisionRoots},
				{"enforcement-map", "a runtime's static envelope-enforcement map as canonical JSON", runRuntimeEnforcementMap},
				{"registration", "a runtime's declared registration rows (registration/v1 wire)", runRuntimeRegistration},
				{"adoption-default", "the one default adoption runtime", runRuntimeAdoptionDefault},
				{"dirs", "a runtime's adopted registration directories", runRuntimeDirs},
				{"enforcement-config", "a runtime's shipped enforcement config filename", runRuntimeEnforcementConfig},
				{"self-check", "a runtime's live self-check vendored marker", runRuntimeSelfCheck},
				{"instruction-file", "a runtime's instruction-bearing filename", runRuntimeInstructionFile},
				{"session-env", "a runtime's project-dir environment variable", runRuntimeSessionEnv},
				{"acp-expectation", "a runtime's expected ACP transport declaration as JSON", runRuntimeACPExpectation},
			},
		},
		{
			name:    "acp",
			summary: "the ACP transport client (wire only; launch and custody stay with scripts)",
			verbs: []verb{
				{"preflight", "check an envelope's ACP-v1 eligibility before any launch", runACPPreflight},
				{"mode", "resolve a runtime's session mode for an envelope tools grade", runACPMode},
				{"turn", "drive one prompt attempt over pre-created pipes, emitting the typed outcome", runACPTurn},
			},
		},
		{
			name:    "janitor",
			summary: "disk-hygiene: headroom guard and (later) the artifact-class sweep",
			verbs: []verb{
				{"headroom", "check free space per filesystem against a floor (exit 3 below)", runJanitorHeadroom},
			},
		},
		{
			name:    "hooks",
			summary: "self-check that the repo runs under its own metasystem",
			verbs: []verb{
				{"check", "verify live settings carry the shipped lifecycle hooks", runHooksCheck},
			},
		},
		{
			name:    "util",
			summary: "small utilities for shell callers",
			verbs: []verb{
				{"token-hex", "print a random hex token of --bytes length", runUtilTokenHex},
				{"sha256", "print the hex sha-256 of --file or stdin", runUtilSHA256},
				{"slug", "print a stable slug of the argument (matches the sanitize rule)", runUtilSlug},
				{"json-validate", "exit 0 if --file/--value is valid JSON, else 1", runUtilJSONValidate},
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
				{"get", "print a dotted field from a JSON file or string", runJSONGet},
				{"object", "build a compact JSON object from key=value args", runJSONObject},
				{"set", "set top-level fields in a JSON object file atomically", runJSONSet},
				{"strip", "print a JSON object with named top-level keys removed", runJSONStrip},
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
			name:    "channel",
			summary: "fleet status and authenticated question threads",
			verbs: []verb{
				{"status", "compose or post this machine's durable status", runChannelStatus},
				{"ask", "open one durable question thread", runChannelAsk},
				{"show", "show one question record", runChannelShow},
				{"wait", "wait for one recorded answer", runChannelWait},
				{"poll", "receive and durably disposition replies", runChannelPoll},
				{"close", "withdraw one question", runChannelClose},
				{"fake", "fixture-only fake serve and code verbs", runChannelFake},
				{"telegram", "inspect pending Telegram bot updates", runChannelTelegram},
			},
		},
		{
			name:    "goal",
			summary: "the goal ledger: the thread of intent that survives every turn (D67)",
			verbs: []verb{
				{"open", "declare a goal; Current when none exists, queued otherwise", runGoalOpen},
				{"set-next", "rewrite the Current goal's next step", runGoalSetNext},
				{"promote", "move a queued goal to Current", runGoalPromote},
				{"park", "park a goal; parking the Current one requires --then or --and-none", runGoalPark},
				{"unpark", "return a parked goal to the queue", runGoalUnpark},
				{"done", "conclude the Current goal; requires --then or --and-none", runGoalDone},
				{"reopen", "return a done goal to the queue with a fresh --next", runGoalReopen},
				{"declare-free", "declare (or renew) the absence of intent over the current plans world", runGoalDeclareFree},
				{"prune", "drop done goals beyond the newest ten, reporting every drop", runGoalPrune},
				{"claim", "claim a goal (or its whole arc with --arc) for this machine", runGoalClaim},
				{"approve", "human-only: approve exact goal intent and budget for execution, or run the grandfather sweep", runGoalApprove},
				{"unapprove", "human-only: withdraw execution approval and park any standing claim", runGoalUnapprove},
				{"set-budget", "human-only: replace a claimed goal's approved tuple and bind a new revision", runGoalSetBudget},
				{"split", "atomize one goal into independently claimable arc members; conclude the parent as decomposed", runGoalSplit},
				{"set-obligation", "human-only: bind a governed recurrence and typed assumptions to the existing budget", runGoalSetObligation},
				{"enroll-terminal", "enroll this agent-free interactive terminal for human-only goal authority", runGoalEnrollTerminal},
				{"resume", "human-only: resume a stopped goal under its standing valid approval", runGoalResume},
				{"release", "release this machine's claim (or the arc's with --arc)", runGoalRelease},
				{"steal", "take over another machine's claim (displacement-bearing)", runGoalSteal},
				{"edit", "edit a goal's intent or next step in place", runGoalEdit},
				{"set-arc", "move a goal into an arc under the membership rules", runGoalSetArc},
				{"set-pin", "pin a goal to one machine (\"-\" clears); only that machine may claim it", runGoalSetPin},
				{"detach", "take a goal out of its arc (releases a riding claim)", runGoalDetach},
				{"list", "print the parsed ledger as JSON (read-only; --pretty for eyes)", runGoalList},
				{"show", "print ONE goal whole: fields, claim, park, history (read-only)", runGoalShow},
				{"next", "print the one orientation line (read-only, the universal fallback)", runGoalNext},
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
			summary: "the announced main session's human-only stop authority",
			verbs: []verb{
				{"stop", "human-only: authorize one quiet stop for the current announced main session", runSessionStop},
				{"end", "retire any unused quiet-stop authorization for one ended session (internal)", runSessionEnd},
			},
		},
		{
			name:    "steward",
			summary: "the idle watchdog: open delegated work is never silently idle (D121)",
			verbs: []verb{
				{"health", "print the typed roles-alive verdict", runStewardHealth},
				{"tick", "one scheduled observation: decide, age the evidence, report the action", runStewardTick},
				{"status", "the operator's view: evidence age, live intents, pending notifications", runStewardStatus},
				{"authorize-dispatch", "gate the unattended continuation: steward caller, consumed unstamped intent, staged tuple out", runStewardAuthorizeDispatch},
				{"revive", "one revival end to end: stage, mint, arbitrate, dispatch once", runStewardRevive},
				{"run", "the runner's body: tick until disarmed (spawned by arm; callable by any external ticker)", runStewardRun},
				{"arm", "explicit human enrollment: pin this engine generation and spawn the detached runner", runStewardArm},
				{"restart", "replace the runner and arm it again after a stalled pass", runStewardRestart},
				{"disarm", "end the runner", runStewardDisarm},
				{"pending", "one line naming undelivered incidents; empty means none", runStewardPending},
				{"hook-attempt", "record a supervision-hook attempt before turn work (internal)", runStewardHookAttempt},
				{"hook-complete", "record a supervision-hook completion after payload emission (internal)", runStewardHookComplete},
				{"digest-pending", "print narrator highlights and lowlights since the last check-in (internal)", runStewardDigestPending},
				{"digest-advance", "advance the narrator digest after payload emission (internal)", runStewardDigestAdvance},
			},
		},
		{
			name:    "run",
			summary: "tracked long-running work: launch, watch, conclude (the monitor facility)",
			verbs: []verb{
				{"launch", "reserve, spawn the wrapped command detached, print the watch line", runRunLaunch},
				{"wrap", "the setsid leader: bind, run the workload, write the exit sidecar (internal)", runRunWrap},
				{"watch", "block until the run is terminal; exit with its pinned code", runRunWatch},
				{"register", "bind an already-running process as an adopted run", runRunRegister},
				{"adopt", "rebind a running record to a successor process", runRunAdopt},
				{"ack", "acknowledge a terminal run", runRunAck},
				{"conclude", "one assessment pass over a run (the watcher's verb)", runRunConclude},
				{"status", "print one run record", runRunStatus},
				{"list", "print every run record plus unreadable paths", runRunList},
				{"prune", "drop acked terminal runs older than 14 days, reporting drops", runRunPrune},
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
				{"renew", "bump the holder's lease revision", runLeaseRenew},
				{"run-held", "run a command while holding the lease lock (gated on holdership)", runLeaseRunHeld},
				{"protocol-growth", "report new protocol errors since a main last advanced its cursor", runLeaseProtocolGrowth},
				{"protocol-advance", "merge a main's protocol-error counts into its cursor", runLeaseProtocolAdvance},
				{"commit-token", "atomically write the live commit wrapper token", runLeaseCommitToken},
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
				{"fence-check-job", "check the job fences without reserving", runMissionFenceReserve("fence-check-job", false)},
				{"fence-reserve-job", "check the job fences and reserve the job", runMissionFenceReserve("fence-reserve-job", true)},
				{"fence-reserve-cycle", "check the cycle fences and record a cycle", runMissionFenceReserveCycle},
				{"fence-authorize-cap", "authorize a per-job cap for a runtime/model pair", runMissionFenceAuthorizeCap},
				{"fence-aggregate-usage", "aggregate typed usage across the mission's finished jobs", runMissionFenceAggregateUsage},
				{"fence-refuse", "raise a batched fence ask for a reason", runMissionFenceRefuse},
				{"fence-release-job", "release a husked dispatch's fence reservation", runMissionFenceReleaseJob},
				{"contract-validate", "validate a mission contract's authored block", runMissionContractValidate},
				{"contract-seal", "seal a validated contract and print its digest", runMissionContractSeal},
				{"contract-hash", "print a contract's canonical signed-bytes digest", runMissionContractHash},
				{"contract-preflight", "preflight a sealed, signed contract and emit its verified bytes", runMissionContractPreflight},
				{"contract-measure", "run the gate and guards and classify metrics against a prior measurement", runMissionContractMeasure},
				{"contract-envelope-allows", "exit 0 when the signed contract's dispatch-allow carries a pair", runMissionContractEnvelopeAllows},
				{"prompt-assemble", "assemble the byte-stable host-turn prompt", runMissionPromptAssemble},
				{"start", "start a mission's detached run loop", runMissionRunnerStart},
				{"resume", "resume a parked or interrupted mission", runMissionRunnerResume},
				{"status", "print the mission's runner status line", runMissionRunnerStatus},
				{"answer", "record a human answer to an open ask", runMissionRunnerAnswer},
				{"resolve-taint", "apply a typed human resolution (--restore <treeId> | --adopt --waives <claim>) to a workspace taint", runMissionRunnerResolveTaint},
				{"run-loop", "the detached mission loop (internal; spawned by start/resume)", runMissionRunnerRunLoop},
				{"turn-adjudicate", "validate and adjudicate an orchestrator return into a turn verdict", runMissionTurnAdjudicate},
				{"turn-conclude", "conclude a turn into the proposed next mission state", runMissionTurnConclude},
				{"turn-record-failure", "propose the state after a failed turn", runMissionTurnRecordFailure},
				{"turn-park", "propose the parked state and its asks for a reason", runMissionTurnPark},
				{"ledger-init", "create a ledger with cycle and no-gain budgets", runMissionLedgerInit},
				{"ledger-append", "append the next cycle's verdict", runMissionLedgerAppend},
			},
		},
		{
			name:    "evidence",
			summary: "durable-evidence lifecycle: collect mirrored chains, prune residue, age archives",
			verbs: []verb{
				{"gc", "collect closed mirrored chains, prune residue, age flight-recorder archives", runEvidenceGC},
			},
		},
		{
			name:    "supervise",
			summary: "the supervision lifecycle (docs/design/supervision-lifecycle.md)",
			verbs: []verb{
				{"fingerprint", "print a checkout's supervision fingerprint (code, signatures, configuration)", runCensusFingerprint},
				{"derive-ceiling", "derive the watcher cap ceiling from config, environment, and the declared maximum", runSuperviseDeriveCeiling},
				{"verify-armed", "exit 0 when supervision is verifiably armed at this instant", runSuperviseVerifyArmed},
				{"owner", "run the owner loop for a checkout (internal; launched by up)", runSuperviseOwnerLoop},
				{"component", "run a supervised component (internal; launched by the owner)", runSuperviseComponent},
				{"status", "print the checkout's supervision state as JSON", runSuperviseStatus},
				{"blocking-reserved-cap", "print the highest live reservation at or above a ceiling", runSuperviseBlockingReservedCap},
				{"launch-detached", "start a command in its own session with logged output", runSuperviseLaunchDetached},
				{"watchdog-report", "report a stale census, untracked processes, and dead components", runSuperviseWatchdogReport},
				{"heartbeat", "atomically write a component heartbeat with its kernel identity", runSuperviseHeartbeat},
				{"watcher-pass", "run one census pass as a standalone writer under the census lock", runSuperviseWatcherPass},
			},
		},
	}
}

func main() {
	os.Exit(dispatch(os.Args[1:]))
}

func dispatch(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		usage()
		return 2
	}
	if args[0] == "up" {
		return runUp(args[1:])
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
	if args[0] == "delegate" {
		return runDelegate(args[1:])
	}
	for _, fam := range families() {
		if fam.name != args[0] {
			continue
		}
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "metasystem %s: a verb is required\n", fam.name)
			usage()
			return 2
		}
		for _, v := range fam.verbs {
			if v.name == args[1] {
				return v.run(args[2:])
			}
		}
		fmt.Fprintf(os.Stderr, "metasystem %s: unknown verb %q\n", fam.name, args[1])
		return 2
	}
	fmt.Fprintf(os.Stderr, "metasystem: unknown family %q\n", args[0])
	usage()
	return 2
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: metasystem <family> <verb> [flags]")
	fmt.Fprintln(os.Stderr, "       metasystem up [--repo <checkout>] [--pid <pid> --start-time <epoch>]")
	fmt.Fprintln(os.Stderr, "       metasystem up --print-scheduler-entry [--repo <checkout>]")
	fmt.Fprintln(os.Stderr, "       metasystem health --repo <checkout>")
	fmt.Fprintln(os.Stderr, "       metasystem health acknowledge-alert --episode <id> [--repo <checkout>]")
	fmt.Fprintln(os.Stderr, "       metasystem watch [--root <checkout>] [--json]")
	fmt.Fprintln(os.Stderr, "       metasystem watch --job <id> [--root <checkout>] [--poll-ms <milliseconds>]")
	fmt.Fprintln(os.Stderr, "       metasystem delegate --role <role> --brief <file> --goal <id|none-explicit> --destructive-reach <class> [--op <id>]")
	fmt.Fprintln(os.Stderr, "       metasystem delegate --follow-up <job> --brief <file>")
	fmt.Fprintln(os.Stderr, "       metasystem delegate --cancel <job>")
	for _, fam := range families() {
		fmt.Fprintf(os.Stderr, "  %-10s %s\n", fam.name, fam.summary)
		for _, v := range fam.verbs {
			if fam.name == "job" && (v.name == "claim-launch" || v.name == "claim-occupancy-prepare" || v.name == "compose-role-packet" || v.name == "operation-id") {
				continue
			}
			fmt.Fprintf(os.Stderr, "    %-14s %s\n", v.name, v.summary)
		}
	}
}
