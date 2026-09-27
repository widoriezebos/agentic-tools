package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"

	usagepkg "github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

// The adapter family is the shared lifecycle plumbing (internal/adapter) every
// runtime adapter calls from runtime-common.sh: the job root-ancestor walk, the
// effective-permissions materialize/bound/compare handshake, the compare-and-
// swap patch writers, and the capability-snapshot writer.

// runAdapterRootJob prints the root of a job's parentJob chain.
func runAdapterRootJob(args []string) int {
	flags := flag.NewFlagSet("adapter root-job", flag.ContinueOnError)
	jobs := flags.String("jobs", "", "jobs directory")
	job := flags.String("job", "", "job id")
	if flags.Parse(args) != nil {
		return 2
	}
	if *jobs == "" || *job == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem internal adapter root-job --jobs DIR --job ID")
		return 2
	}
	root, err := usagepkg.RootJobID(*jobs, *job)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(root)
	return 0
}

// runAdapterResultPatch writes an {error,phase,usage} terminal patch file.
func runAdapterResultPatch(args []string) int {
	flags := flag.NewFlagSet("adapter result-patch", flag.ContinueOnError)
	output := flags.String("output", "", "patch output file")
	failure := flags.String("error", "", "failure code, or the literal null")
	phase := flags.String("phase", "", "phase the round settled in")
	usage := flags.String("usage", "", "typed usage file (optional)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *output == "" || *phase == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem internal adapter result-patch --output FILE --error CODE|null --phase PHASE [--usage FILE]")
		return 2
	}
	if err := adapter.WriteResultPatch(*output, *failure, *phase, *usage); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
