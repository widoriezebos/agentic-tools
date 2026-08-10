package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

// The dispatch family is the job-record lifecycle surface (internal/dispatch):
// the single writer that creates a job's record, completes its setup, stamps a
// protocol error, and compare-and-swaps its status. Each write holds the
// exclusive per-record lock and lands atomically.

// recordExit maps a lifecycle error to a process exit code, printing any
// message the refusal carries to stderr. A nil error is exit 0.
func recordExit(err error) int {
	if err == nil {
		return 0
	}
	var op *dispatchcore.OpError
	if errors.As(err, &op) {
		if op.Message != "" {
			fmt.Fprintln(os.Stderr, op.Message)
		}
		return op.Code
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}

func runDispatchRecordCreate(args []string) int {
	flags := flag.NewFlagSet("dispatch record-create", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	job := flags.String("job", "", "job id")
	source := flags.String("source", "", "initial pending-setup record file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *job == "" || *source == "" {
		fmt.Fprintln(os.Stderr, "dispatch record-create: --root, --job, and --source are required")
		return 2
	}
	return recordExit(dispatchcore.RecordCreate(*root, *job, *source))
}

func runDispatchRecordSetup(args []string) int {
	flags := flag.NewFlagSet("dispatch record-setup", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	job := flags.String("job", "", "job id")
	source := flags.String("source", "", "complete pending record file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *job == "" || *source == "" {
		fmt.Fprintln(os.Stderr, "dispatch record-setup: --root, --job, and --source are required")
		return 2
	}
	return recordExit(dispatchcore.RecordSetup(*root, *job, *source))
}

func runDispatchRecordCAS(args []string) int {
	flags := flag.NewFlagSet("dispatch record-cas", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	job := flags.String("job", "", "job id")
	expect := flags.String("expect", "", "status the record must currently hold")
	status := flags.String("status", "", "target status (equal to --expect for a metadata update)")
	patch := flags.String("patch", "", "JSON object patch file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *job == "" || *expect == "" || *status == "" || *patch == "" {
		fmt.Fprintln(os.Stderr, "dispatch record-cas: --root, --job, --expect, --status, and --patch are required")
		return 2
	}
	observed, err := dispatchcore.RecordCAS(*root, *job, *expect, *status, *patch)
	if observed != "" {
		// The lost-compare observation goes to stdout so the caller can witness
		// exactly which status this atomic compare saw.
		fmt.Println(observed)
	}
	return recordExit(err)
}

func runDispatchRecordProtocolError(args []string) int {
	flags := flag.NewFlagSet("dispatch record-protocol-error", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	job := flags.String("job", "", "job id")
	expect := flags.String("expect", "", "status the record must currently hold")
	violation := flags.String("violation", "", "protocol violation text")
	violationFile := flags.String("violation-file", "", "file holding the violation text")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *job == "" || *expect == "" {
		fmt.Fprintln(os.Stderr, "dispatch record-protocol-error: --root, --job, and --expect are required")
		return 2
	}
	return recordExit(dispatchcore.RecordProtocolError(*root, *job, *expect, *violation, *violationFile))
}
