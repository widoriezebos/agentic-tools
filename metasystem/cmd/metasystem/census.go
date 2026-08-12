package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
)

// runCensusFingerprint prints the supervision fingerprint for --repo, using
// --root as the metasystem root (defaults to the binary's checkout).
func runCensusFingerprint(args []string) int {
	flags := flag.NewFlagSet("census fingerprint", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root to fingerprint")
	root := flags.String("root", "", "metasystem root (defaults to this checkout)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "census fingerprint: --repo is required")
		return 2
	}
	metasystemRoot := *root
	if metasystemRoot == "" {
		if exe, err := os.Executable(); err == nil {
			metasystemRoot = filepath.Dir(filepath.Dir(filepath.Dir(exe)))
		}
	}
	fp, err := census.Fingerprint(metasystemRoot, *repo)
	if err != nil {
		fmt.Fprintln(os.Stderr, "census fingerprint:", err)
		return 1
	}
	fmt.Println(fp)
	return 0
}

// runCensusRun computes a fixture-driven census verdict and writes it to
// --output, printing the inventory and diagnostic lines for the run.
func runCensusRun(args []string) int {
	flags := flag.NewFlagSet("census run", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	root := flags.String("root", "", "metasystem root (defaults to --repo)")
	fp := flags.String("fingerprint", "", "fingerprint to stamp")
	interval := flags.Int("interval", 60, "interval seconds")
	output := flags.String("output", "", "verdict output path")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "census run: --repo and --output are required")
		return 2
	}
	metasystemRoot := *root
	if metasystemRoot == "" {
		metasystemRoot = *repo
	}
	// With a recorded process table the verdict is fixture-driven and uses a
	// fixed clock for a deterministic result; otherwise it scans the live
	// process table with the real clock.
	var (
		verdict census.Verdict
		err     error
	)
	if processFile := os.Getenv("METASYSTEM_CENSUS_PROCESS_FILE"); processFile != "" {
		verdict, err = census.RunFixtureCensus(metasystemRoot, *repo, processFile, *fp, *interval, time.Unix(1786000000, 0))
	} else {
		verdict, err = census.RunProductionCensus(metasystemRoot, *repo, *fp, *interval, time.Now().UTC())
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "census run:", err)
		return 1
	}
	encoded, err := json.MarshalIndent(verdict, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "census run:", err)
		return 1
	}
	if err := os.WriteFile(*output, append(encoded, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "census run:", err)
		return 1
	}
	return 0
}

func runCensusAlive(args []string) int {
	flags := flag.NewFlagSet("census alive", flag.ContinueOnError)
	pid := flags.Int64("pid", 0, "process id")
	start := flags.Int64("start-time", 0, "expected start epoch seconds")
	if flags.Parse(args) != nil {
		return 2
	}
	if census.Alive(*pid, *start) {
		return 0
	}
	return 1
}

func runCensusSignatureCheck(args []string) int {
	flags := flag.NewFlagSet("census signature-check", flag.ContinueOnError)
	adapter := flags.String("adapter", "", "adapter path")
	positive := flags.String("positive", "", "argv that must classify")
	lookalike := flags.String("lookalike", "", "argv that must NOT classify")
	if flags.Parse(args) != nil {
		return 2
	}
	if err := census.SignatureCheck(*adapter, *positive, *lookalike); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runCensusFindAncestor walks the live process tree from --pid and prints the
// first signature-matched agent ancestor as compact JSON.
func runCensusFindAncestor(args []string) int {
	flags := flag.NewFlagSet("census find-ancestor", flag.ContinueOnError)
	repo := flags.String("repo", "", "metasystem root")
	pid := flags.Int64("pid", 0, "process id to walk up from")
	runtime := flags.String("runtime", "", "restrict to one runtime (optional)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" || *pid == 0 {
		fmt.Fprintln(os.Stderr, "census find-ancestor: --repo and --pid are required")
		return 2
	}
	ancestor, err := census.FindAncestorProduction(*repo, *pid, *runtime)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	encoded, _ := json.Marshal(ancestor)
	fmt.Println(string(encoded))
	return 0
}
