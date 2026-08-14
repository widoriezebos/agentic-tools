// Command metasystem is the metasystem's one binary: each family
// groups the decisions the shell wrappers invoke, exposed as git-style
// verbs. Wrappers keep their historical names and exec into these
// verbs. File naming: one file per verb family (a verb lives in the file
// of the family it REGISTERS under); cross-family helpers live in
// helpers.go and nowhere else.
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
			name:    "proc",
			summary: "process identity and census: who is running, provably",
			verbs: []verb{
				{"started-at", "print a pid's start time in epoch seconds", runIdentityStartedAt},
				{"probe", "print a pid's exact identity as JSON", runIdentityProbe},
				{"exists", "exit 0 if the pid exists (permission denial proves existence)", runIdentityExists},
				{"group-exists", "exit 0 if the process group exists", runIdentityGroupExists},
				{"census", "compute a fixture-driven census verdict", runCensusRun},
				{"alive", "exit 0 if a pid is live at its expected start", runCensusAlive},
				{"classify", "print live, stale, dead, or unknown for a recorded pid and tag", runProcClassify},
				{"signature-check", "verify an adapter's positive/lookalike signature contract", runCensusSignatureCheck},
				{"find-ancestor", "walk up the process tree to the first agent-signature ancestor", runCensusFindAncestor},
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
				{"return-complete", "validate an agent return against its role schema and job identity", runValidateReturnComplete},
				{"design-obligations", "check the structure and declared state of design-obligation matrices", runValidateDesignObligations},
				{"conformance", "review or merge conformance for an implementer job", runValidateConformance},
				{"stop-loss", "block further investigation cycles when a ledger trigger fired", runValidateStopLoss},
			},
		},
		{
			name:    "job",
			summary: "the delegate-job domain: records, chains, locks, caps, snapshots, authority",
			verbs: []verb{
				{"record-create", "reserve a job by writing its pending-setup record", runDispatchRecordCreate},
				{"record-setup", "complete a reservation into the full pending record", runDispatchRecordSetup},
				{"record-cas", "compare-and-swap a record's status and fields", runDispatchRecordCAS},
				{"record-protocol-error", "stamp a job failed with a protocol violation", runDispatchRecordProtocolError},
				{"build-setup", "assemble a pending-setup reservation record", runDispatchBuildSetup},
				{"resolve-roster", "resolve a role's roster pair and classify escalation", runDispatchResolveRoster},
				{"build-record", "assemble the full pending job record", runDispatchBuildRecord},
				{"build-follow-record", "assemble a follow-up round's record from its parent", runDispatchBuildFollowRecord},
				{"latest-chain-record", "print the newest record path in a job chain", runDispatchLatestChainRecord},
				{"chain-members", "list a chain's jobs and statuses", runDispatchChainMembers},
				{"chain-usage", "aggregate a chain's usage (exit 7 when unchanged)", runDispatchChainUsage},
				{"custody-add", "append a custody process to a job record under its lock", runDispatchCustodyAdd},
				{"handshake-eval", "evaluate a handshake into its record patch", runDispatchHandshakeEval},
				{"reap-facts", "print a record's reap verdict facts", runDispatchReapFacts},
				{"census-fresh", "require a fresh successful census for dispatch", runDispatchCensusFresh},
				{"watcher-ceiling", "print the attested watcher ceiling", runDispatchWatcherCeiling},
				{"expand-permissions", "expand a role's permission preset for a workspace", runDispatchExpandPermissions},
				{"validate-mission", "validate a mission id and lease for dispatch", runDispatchValidateMission},
				{"mirror", "mirror a job's evidence with a manifest", runDispatchMirror},
				{"close-check", "validate a chain is closable", runDispatchCloseCheck},
				{"critique-exhaustion", "decide a critique-exhaustion action", runDispatchCritiqueExhaustion},
				{"exhaustion-patches", "list the exhaustion patches a manifest names", runDispatchExhaustionPatches},
				{"cap-resolution", "write a cap-resolution record", runDispatchCapResolution},
				{"resolve-cap", "resolve the non-mission cap chain or refuse an unsigned mission cap", runDispatchResolveCap},
				{"brief-mode", "check a brief names a known mode", runDispatchBriefMode},
				{"owner-lock", "claim or release the dispatch owner lock (0 done, 3 busy, 4 not-owner)", runDispatchOwnerLock},
				{"snapshot-select", "select the capability snapshot matching a dispatch's identity", runCapabilitySelect},
				{"authority-check", "check a control-plane write against the authority matrix", runAuthorityCheck},
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
				{"repairs-patch", "write a {returnRepairs} record patch", runAdapterRepairsPatch},
				{"result-patch", "write an {error,phase,usage} record patch", runAdapterResultPatch},
				{"capability-snapshot", "write a validated capability snapshot", runAdapterCapabilitySnapshot},
				{"version-parse", "extract the semver from CLI version output on stdin", runAdapterVersionParse},
				{"codex-event", "read the session or turn field from a Codex event stream", runAdapterCodexEvent},
				{"codex-usage", "extract Codex usage from its event stream", runAdapterCodexUsage},
				{"codex-command", "build the Codex delegate argv (NUL-terminated)", runAdapterCodexCommand},
				{"claude-settings", "build the Claude job settings from a record", runAdapterClaudeSettings},
				{"claude-usage", "extract Claude usage from its result", runAdapterClaudeUsage},
				{"claude-result-field", "read a Claude result field with modelUsage collapse", runAdapterClaudeResultField},
				{"claude-read-roots", "list a record's readRoots minus the workspace", runAdapterClaudeReadRoots},
				{"claude-append-result", "append a Claude result to the event stream", runAdapterClaudeAppendResult},
				{"claude-session-signal", "record the Claude session-established signal", runAdapterClaudeSessionSignal},
				{"devin-config", "build the Devin job config from the user config", runAdapterDevinConfig},
				{"devin-session", "correlate the new Devin session against the baseline", runAdapterDevinSession},
				{"devin-usage", "compute the Devin per-round usage delta", runAdapterDevinUsage},
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
			},
		},
		{
			name:    "host",
			summary: "host-loop plumbing: result envelopes, usage, and return extraction",
			verbs: []verb{
				{"result-write", "write a host turn's result envelope", runHostResultWrite},
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
			name:    "gate",
			summary: "gate-run markers: know when a gate is in flight",
			verbs: []verb{
				{"register", "record that this process is a running gate", runGateRegister},
				{"check", "print 1 when a gate is running in this checkout, else 0", runGateCheck},
				{"fence", "exit 1 naming every live gate run foreign to --self-pid's chain", runGateFence},
			},
		},
		{
			name:    "report",
			summary: "turn-end report decisions",
			verbs: []verb{
				{"stop-block", "print the stop-hook block that refuses to end a turn with idle open work", runReportStopBlock},
				{"open-work", "report plans with an unblocked next step and no job in flight", runReportOpenWork},
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
			name:    "schema",
			summary: "role-return schema materialization",
			verbs: []verb{
				{"materialize", "write a role's return schema at a version", runSchemaMaterialize},
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
				{"contract-preflight", "preflight a sealed, signed contract and emit its verified bytes", runMissionContractPreflight},
				{"contract-measure", "run the gate and guards and classify metrics against a prior measurement", runMissionContractMeasure},
				{"contract-envelope-allows", "exit 0 when the signed contract's dispatch-allow carries a pair", runMissionContractEnvelopeAllows},
				{"prompt-assemble", "assemble the byte-stable host-turn prompt", runMissionPromptAssemble},
				{"start", "start a mission's detached run loop", runMissionRunnerStart},
				{"resume", "resume a parked or interrupted mission", runMissionRunnerResume},
				{"status", "print the mission's runner status line", runMissionRunnerStatus},
				{"answer", "record a human answer to an open ask", runMissionRunnerAnswer},
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
			summary: "the supervision lifecycle (plans/supervision-lifecycle.md)",
			verbs: []verb{
				{"fingerprint", "print a checkout's supervision fingerprint (code, signatures, configuration)", runCensusFingerprint},
				{"derive-ceiling", "derive the watcher cap ceiling from config, environment, and the declared maximum", runSuperviseDeriveCeiling},
				{"owner", "run the owner loop for a checkout (internal; launched by arm)", runSuperviseOwnerLoop},
				{"component", "run a supervised component (internal; launched by the owner)", runSuperviseComponent},
				{"status", "print the checkout's supervision state as JSON", runSuperviseStatus},
				{"blocking-reserved-cap", "print the highest live reservation at or above a ceiling", runSuperviseBlockingReservedCap},
				{"write-owner-identity", "atomically write the owner-identity record", runSuperviseWriteOwnerIdentity},
				{"component-identity", "print a recorded component's pid, start, and tag", runSuperviseComponentIdentity},
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
	for _, fam := range families() {
		fmt.Fprintf(os.Stderr, "  %-10s %s\n", fam.name, fam.summary)
		for _, v := range fam.verbs {
			fmt.Fprintf(os.Stderr, "    %-14s %s\n", v.name, v.summary)
		}
	}
}
