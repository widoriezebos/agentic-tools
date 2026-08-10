package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
)

// runCensusClassify is the differential-conformance surface for the census
// signature port (plans/go-migration.md): it reads a JSON job on stdin —
// {signatures: [{runtime, matches[], excludes[]}], argvs: [...]} — and
// prints the classification as JSON {assignments: [{index, runtime}]}. The
// conformance harness feeds the SAME job to this and to a python reference
// and diffs, proving the Go classifier is indistinguishable.
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
// --root as the metasystem root (defaults to the binary's checkout). Strict
// port of process-census.py fingerprint; the conformance harness diffs it
// against the python.
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
