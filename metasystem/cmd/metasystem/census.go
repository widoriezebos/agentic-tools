package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
)

// runCensusClassify classifies process argvs against runtime signatures: it
// reads a JSON job on stdin —
// {signatures: [{runtime, matches[], excludes[]}], argvs: [...]} — and prints
// the classification as JSON {assignments: [{index, runtime}]}.
func runCensusClassify(args []string) int {
	flags := flag.NewFlagSet("census classify", flag.ContinueOnError)
	if flags.Parse(args) != nil {
		return 2
	}
	var job struct {
		Signatures []struct {
			Runtime  string   `json:"runtime"`
			Matches  []string `json:"matches"`
			Excludes []string `json:"excludes"`
		} `json:"signatures"`
		Argvs []string `json:"argvs"`
	}
	decoder := json.NewDecoder(bufio.NewReader(os.Stdin))
	if err := decoder.Decode(&job); err != nil {
		fmt.Fprintln(os.Stderr, "census classify: unreadable job:", err)
		return 2
	}
	var signatures []census.Signature
	for _, raw := range job.Signatures {
		sig, err := census.CompileSignature(raw.Runtime, raw.Matches, raw.Excludes)
		if err != nil {
			fmt.Fprintln(os.Stderr, "census classify:", err)
			return 1
		}
		signatures = append(signatures, sig)
	}
	result := struct {
		Assignments []census.Assignment `json:"assignments"`
	}{Assignments: census.Classify(job.Argvs, signatures)}
	if result.Assignments == nil {
		result.Assignments = []census.Assignment{}
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintln(os.Stderr, "census classify:", err)
		return 1
	}
	fmt.Println(string(encoded))
	return 0
}

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
	processFile := os.Getenv("METASYSTEM_CENSUS_PROCESS_FILE")
	if *repo == "" || *output == "" || processFile == "" {
		fmt.Fprintln(os.Stderr, "census run: --repo, --output, and METASYSTEM_CENSUS_PROCESS_FILE required (fixture path)")
		return 2
	}
	metasystemRoot := *root
	if metasystemRoot == "" {
		metasystemRoot = *repo
	}
	// A fixed clock keeps the verdict deterministic; the time fields are
	// normalized downstream anyway.
	now := time.Unix(1786000000, 0)
	verdict, err := census.RunFixtureCensus(metasystemRoot, *repo, processFile, *fp, *interval, now)
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

func runCensusAuthIdentity(args []string) int {
	flags := flag.NewFlagSet("census authentication-identity", flag.ContinueOnError)
	pid := flags.Int64("pid", 0, "process id")
	if flags.Parse(args) != nil {
		return 2
	}
	id, err := census.AuthIdentity(*pid)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	encoded, _ := json.Marshal(id)
	fmt.Println(string(encoded))
	return 0
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
