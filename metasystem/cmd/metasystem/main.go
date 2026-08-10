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
