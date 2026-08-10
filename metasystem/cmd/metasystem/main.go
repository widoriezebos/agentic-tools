// Command metasystem is the metasystem's one binary: each family
// groups the decisions the shell wrappers invoke, exposed as git-style
// verbs. Wrappers keep their historical names and exec into these
// verbs.
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
			name:    "identity",
			summary: "kernel process identity: exact start times, argv, three-way liveness",
			verbs: []verb{
				{"started-at", "print a pid's start time in epoch seconds", runIdentityStartedAt},
				{"probe", "print a pid's exact identity as JSON", runIdentityProbe},
			},
		},
		{
			name:    "census",
			summary: "process census classification",
			verbs: []verb{
				{"classify", "classify argvs against runtime signatures", runCensusClassify},
				{"fingerprint", "print a checkout's supervision fingerprint", runCensusFingerprint},
				{"run", "compute a fixture-driven census verdict", runCensusRun},
				{"alive", "exit 0 if a pid is live at its expected start", runCensusAlive},
				{"authentication-identity", "print a pid's start time and command from one source", runCensusAuthIdentity},
				{"signature-check", "verify an adapter's positive/lookalike signature contract", runCensusSignatureCheck},
			},
		},
		{
			name:    "capability",
			summary: "select and validate a capability snapshot",
			verbs: []verb{
				{"select", "select the capability snapshot matching a dispatch's identity", runCapabilitySelect},
			},
		},
		{
			name:    "config",
			summary: "configuration and identity helpers",
			verbs: []verb{
				{"canonical-model", "print the canonical model key for a name", runConfigCanonicalModel},
				{"identity", "print an adapter's canonical configuration identity", runConfigIdentity},
			},
		},
		{
			name:    "gate",
			summary: "gate-run markers: know when a gate is in flight",
			verbs: []verb{
				{"register", "record that this process is a running gate", runGateRegister},
				{"check", "print 1 when a gate is running in this checkout, else 0", runGateCheck},
			},
		},
		{
			name:    "authority",
			summary: "control-plane authority matrix",
			verbs: []verb{
				{"check", "exit 0 if a classified caller may write in a mode, else refuse", runAuthorityCheck},
			},
		},
		{
			name:    "report",
			summary: "turn-end report decisions",
			verbs: []verb{
				{"stop-block", "print the stop-hook block that refuses to end a turn with idle open work", runReportStopBlock},
				{"open-work", "report plans with an unblocked next step and no job in flight", runReportOpenWork},
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
				{"get", "print a dotted field from a JSON file", runJSONGet},
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
			},
		},
		{
			name:    "mission-state",
			summary: "the atomic, hash-chained mission state",
			verbs: []verb{
				{"init", "create a mission's initial state from its sealed contract", runMissionStateInit},
				{"write", "advance the state via a compare-and-write on its hash", runMissionStateWrite},
				{"verify", "validate the state's shape, aggregation, hash chain, and anchor", runMissionStateVerify},
				{"anchor", "write the local anchor commit binding the state hash and ledger", runMissionStateAnchor},
				{"reconcile", "reconcile the state against its ledger and anchor, parking on disagreement", runMissionStateReconcile},
			},
		},
		{
			name:    "mission-fence",
			summary: "the mission lifecycle fences, cap authority, and usage",
			verbs: []verb{
				{"check-job", "check the job fences without reserving", runMissionFenceReserve("check-job", false)},
				{"reserve-job", "check the job fences and reserve the job", runMissionFenceReserve("reserve-job", true)},
				{"reserve-cycle", "check the cycle fences and record a cycle", runMissionFenceReserveCycle},
				{"authorize-cap", "authorize a per-job cap for a runtime/model pair", runMissionFenceAuthorizeCap},
				{"aggregate-usage", "aggregate typed usage across the mission's finished jobs", runMissionFenceAggregateUsage},
				{"refuse", "raise a batched fence ask for a reason", runMissionFenceRefuse},
			},
		},
		{
			name:    "mission-contract",
			summary: "the mission contract parser, sealer, and preflight",
			verbs: []verb{
				{"validate", "validate a mission contract's authored block", runMissionContractValidate},
				{"seal", "seal a validated contract and print its digest", runMissionContractSeal},
				{"preflight", "preflight a sealed, signed contract and emit its verified bytes", runMissionContractPreflight},
			},
		},
		{
			name:    "mission-prompt",
			summary: "assemble a mission host-turn prompt",
			verbs: []verb{
				{"assemble", "assemble the byte-stable host-turn prompt", runMissionPromptAssemble},
			},
		},
		{
			name:    "mission-ledger",
			summary: "the mission stop-loss ledger",
			verbs: []verb{
				{"init", "create a ledger with cycle and no-gain budgets", runMissionLedgerInit},
				{"append", "append the next cycle's verdict", runMissionLedgerAppend},
				{"verify", "validate the ledger and print its cycle count", runMissionLedgerVerify},
				{"count", "print the number of recorded cycles", runMissionLedgerCount},
			},
		},
		{
			name:    "supervise",
			summary: "the supervision lifecycle (plans/supervision-lifecycle.md)",
			verbs: []verb{
				{"owner", "run the owner loop for a checkout (internal; launched by arm)", runSuperviseOwnerLoop},
				{"component", "run a supervised component (internal; launched by the owner)", runSuperviseComponent},
				{"status", "print the checkout's supervision state as JSON", runSuperviseStatus},
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
