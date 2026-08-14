package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
)

// These verbs back the adapter return delivery and the full-contract
// self-test that runtime-common.sh drives: normalizing a runtime's reply into
// the canonical return, asserting typed usage, reading the newest capability
// snapshot's envelope declaration, running the one-shot network tripwire, and
// writing the pass record.

// runAdapterNormalizeReturn extracts the return object from the runtime's
// output and transcript, reconciles session and model identity against what
// the adapter observed, and writes return.json and return.md.
func runAdapterNormalizeReturn(args []string) int {
	flags := flag.NewFlagSet("adapter normalize-return", flag.ContinueOnError)
	candidate := flags.String("candidate", "", "runtime output file holding the reply")
	transcript := flags.String("transcript", "", "runtime transcript file (optional)")
	record := flags.String("record", "", "job record file")
	output := flags.String("output", "", "canonical return.json output file")
	markdown := flags.String("markdown", "", "return.md output file")
	session := flags.String("session", "", "session id the adapter observed (optional)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *candidate == "" || *record == "" || *output == "" || *markdown == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter normalize-return --candidate FILE --record FILE --output FILE --markdown FILE [--transcript FILE] [--session SID]")
		return 2
	}
	if err := adapter.NormalizeReturn(*candidate, *transcript, *record, *output, *markdown, *session); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runAdapterSelftestUsage asserts a self-test job's typed usage matches the
// native, unavailable, or metered expectation.
func runAdapterSelftestUsage(args []string) int {
	flags := flag.NewFlagSet("adapter selftest-usage", flag.ContinueOnError)
	record := flags.String("record", "", "job record file")
	expect := flags.String("expect", "", "native, unavailable, or metered")
	if flags.Parse(args) != nil {
		return 2
	}
	if *record == "" || (*expect != "native" && *expect != "unavailable" && *expect != "metered") {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter selftest-usage --record FILE --expect native|unavailable|metered")
		return 2
	}
	if err := adapter.SelftestUsageCheck(*record, *expect); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runAdapterSelftestEnvelope prints the newest capability snapshot's declared
// enforcement for one envelope field: mapped or notEnforced.
func runAdapterSelftestEnvelope(args []string) int {
	flags := flag.NewFlagSet("adapter selftest-envelope", flag.ContinueOnError)
	dir := flags.String("dir", "", "capabilities directory")
	runtime := flags.String("runtime", "", "runtime name")
	field := flags.String("field", "", "envelope field (writeRoots, readRoots, network)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *dir == "" || *runtime == "" || *field == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter selftest-envelope --dir DIR --runtime RT --field FIELD")
		return 2
	}
	fmt.Println(adapter.SelftestEnvelopeDeclaration(*dir, *runtime, *field))
	return 0
}

// runAdapterSelftestRecord writes the self-test pass record, deriving the
// proven behaviors from the envelope declarations and usage mode.
func runAdapterSelftestRecord(args []string) int {
	flags := flag.NewFlagSet("adapter selftest-record", flag.ContinueOnError)
	output := flags.String("output", "", "pass record output file")
	runtime := flags.String("runtime", "", "runtime name")
	job := flags.String("job", "", "main self-test job id")
	usage := flags.String("usage", "", "native, unavailable, or metered")
	devinChecks := strictBool(flags, "devin-checks", "1", "0", "1 when the Devin-only probes ran")
	writeEnforcement := flags.String("write-enforcement", "", "declared writeRoots enforcement")
	networkEnforcement := flags.String("network-enforcement", "", "declared network enforcement")
	if flags.Parse(args) != nil {
		return 2
	}
	if *output == "" || *runtime == "" || *job == "" || *usage == "" ||
		*writeEnforcement == "" || *networkEnforcement == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter selftest-record --output FILE --runtime RT --job ID --usage MODE --write-enforcement E --network-enforcement E [--devin-checks 0|1]")
		return 2
	}
	if err := adapter.WriteSelftestRecord(*output, *runtime, *job, *usage,
		*devinChecks, *writeEnforcement, *networkEnforcement); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runAdapterSelftestListener binds an ephemeral loopback port, writes it to
// the port file, answers exactly one request into the request log, and exits.
// An idle timeout exits cleanly with no request log written.
func runAdapterSelftestListener(args []string) int {
	flags := flag.NewFlagSet("adapter selftest-listener", flag.ContinueOnError)
	portFile := flags.String("port-file", "", "file the bound port is written to")
	requestLog := flags.String("request-log", "", "file the received request is written to")
	timeoutSeconds := flags.Float64("timeout-seconds", 180, "seconds to wait for the one request")
	if flags.Parse(args) != nil {
		return 2
	}
	if *portFile == "" || *requestLog == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter selftest-listener --port-file FILE --request-log FILE [--timeout-seconds SEC]")
		return 2
	}
	timeout := time.Duration(*timeoutSeconds * float64(time.Second))
	if err := adapter.SelftestListener(*portFile, *requestLog, timeout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
