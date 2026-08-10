package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"

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
