package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
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
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter root-job --jobs DIR --job ID")
		return 2
	}
	root, err := adapter.RootJobID(*jobs, *job)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(root)
	return 0
}

// runAdapterEffectiveInit materializes the job's requested permissions into the
// effective-permissions file.
func runAdapterEffectiveInit(args []string) int {
	flags := flag.NewFlagSet("adapter effective-init", flag.ContinueOnError)
	record := flags.String("record", "", "job record file")
	output := flags.String("output", "", "effective-permissions output file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *record == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter effective-init --record FILE --output FILE")
		return 2
	}
	if err := adapter.MaterializeEffective(*record, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runAdapterEffectiveWorkspace pins the effective file's writeRoots to the
// resolved workspace root.
func runAdapterEffectiveWorkspace(args []string) int {
	flags := flag.NewFlagSet("adapter effective-workspace", flag.ContinueOnError)
	effective := flags.String("effective", "", "effective-permissions file")
	workspace := flags.String("workspace", "", "workspace root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *effective == "" || *workspace == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter effective-workspace --effective FILE --workspace DIR")
		return 2
	}
	if err := adapter.RewriteWriteScope(*effective, *workspace); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runAdapterPermissionCheck prints the comma-joined permission fields where the
// effective grant is wider than the request, or nothing when it is a subset.
func runAdapterPermissionCheck(args []string) int {
	flags := flag.NewFlagSet("adapter permission-check", flag.ContinueOnError)
	record := flags.String("record", "", "job record file")
	effective := flags.String("effective", "", "effective-permissions file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *record == "" || *effective == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter permission-check --record FILE --effective FILE")
		return 2
	}
	mismatch, err := adapter.ComparePermissions(*record, *effective)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(mismatch)
	return 0
}

// runAdapterModelPatch writes an {effectiveModel} compare-and-swap patch file.
func runAdapterModelPatch(args []string) int {
	flags := flag.NewFlagSet("adapter model-patch", flag.ContinueOnError)
	output := flags.String("output", "", "patch output file")
	model := flags.String("model", "", "effective model")
	if flags.Parse(args) != nil {
		return 2
	}
	if *output == "" || *model == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter model-patch --output FILE --model MODEL")
		return 2
	}
	if err := adapter.WriteModelPatch(*output, *model); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runAdapterRepairsPatch writes a {returnRepairs} compare-and-swap patch file.
func runAdapterRepairsPatch(args []string) int {
	flags := flag.NewFlagSet("adapter repairs-patch", flag.ContinueOnError)
	output := flags.String("output", "", "patch output file")
	count := flags.Int("count", -1, "return-repair count")
	if flags.Parse(args) != nil {
		return 2
	}
	if *output == "" || *count < 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter repairs-patch --output FILE --count N")
		return 2
	}
	if err := adapter.WriteRepairsPatch(*output, *count); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
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
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter result-patch --output FILE --error CODE|null --phase PHASE [--usage FILE]")
		return 2
	}
	if err := adapter.WriteResultPatch(*output, *failure, *phase, *usage); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runAdapterCapabilitySnapshot validates a probe result and writes the runtime's
// capability snapshot, printing the written path.
func runAdapterCapabilitySnapshot(args []string) int {
	flags := flag.NewFlagSet("adapter capability-snapshot", flag.ContinueOnError)
	dir := flags.String("dir", "", "capabilities directory")
	runtime := flags.String("runtime", "", "runtime name")
	version := flags.String("version", "", "CLI version")
	configHash := flags.String("config-hash", "", "configuration hash")
	transports := flags.String("transports", "", "transports JSON")
	capabilities := flags.String("capabilities", "", "capabilities JSON")
	permissions := flags.String("permissions", "", "permissions JSON")
	envelope := flags.String("envelope-enforcement", "", "envelope enforcement JSON")
	keyHashes := flags.String("config-key-hashes", "", "configuration key hashes JSON")
	if flags.Parse(args) != nil {
		return 2
	}
	if *dir == "" || *runtime == "" || *version == "" || *configHash == "" ||
		*transports == "" || *capabilities == "" || *permissions == "" ||
		*envelope == "" || *keyHashes == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter capability-snapshot --dir DIR --runtime RT --version V --config-hash H --transports J --capabilities J --permissions J --envelope-enforcement J --config-key-hashes J")
		return 2
	}
	path, err := adapter.WriteCapabilitySnapshot(*dir, *runtime, *version, *configHash,
		*transports, *capabilities, *permissions, *envelope, *keyHashes)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(path)
	return 0
}
