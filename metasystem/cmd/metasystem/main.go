// Command metasystem is the metasystem's one binary: every decision
// the shell scripts used to delegate to Python lives behind a verb
// here (plans/go-migration.md). Families are git-style; wrappers keep
// their historical names and exec into these verbs, so the rest of
// the system cannot tell the engine changed until the originals are
// deleted.
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
				{"started-at", "print a pid's start time in epoch seconds (port of process-census.py started-at)", runIdentityStartedAt},
				{"probe", "print a pid's exact identity as JSON", runIdentityProbe},
			},
		},
		{
			name:    "census",
			summary: "process census classification (port of process-census.py)",
			verbs: []verb{
				{"classify", "classify argvs against runtime signatures (differential-conformance surface)", runCensusClassify},
				{"fingerprint", "print a checkout's supervision fingerprint (port of process-census.py fingerprint)", runCensusFingerprint},
				{"run", "compute a fixture-driven census verdict (port of process-census.py census)", runCensusRun},
				{"alive", "exit 0 if a pid is live at its expected start (port of alive)", runCensusAlive},
				{"authentication-identity", "print a pid's start time and command from one source", runCensusAuthIdentity},
				{"signature-check", "verify an adapter's positive/lookalike signature contract", runCensusSignatureCheck},
			},
		},
		{
			name:    "config",
			summary: "configuration and identity helpers (canonical-model, config-identity, return-schema)",
			verbs: []verb{
				{"canonical-model", "print the canonical model key for a name (port of canonical-model.py)", runConfigCanonicalModel},
				{"identity", "print an adapter's canonical configuration identity (port of config-identity.py)", runConfigIdentity},
			},
		},
		{
			name:    "gate",
			summary: "gate-run markers: know when a gate is in flight (port of gate-run.py)",
			verbs: []verb{
				{"register", "record that this process is a running gate", runGateRegister},
				{"check", "print 1 when a gate is running in this checkout, else 0", runGateCheck},
			},
		},
		{
			name:    "json",
			summary: "JSON field access for shell callers (replaces the python heredocs)",
			verbs: []verb{
				{"get", "print a dotted field from a JSON file", runJSONGet},
			},
		},
		{
			name:    "lease",
			summary: "checkout write-authority: announce/classify/hold/renew (port of worktree-lease.py)",
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
