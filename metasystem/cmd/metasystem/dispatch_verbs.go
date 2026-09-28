package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// The dispatch family is the job-record lifecycle surface (internal/dispatch):
// the single writer that creates a job's record, completes its setup, stamps a
// protocol error, and compare-and-swaps its status. Each write holds the
// exclusive per-record lock and lands atomically.

// recordExit maps a lifecycle error to a process exit code, printing any
// message the refusal carries to stderr. A nil error is exit 0.
func recordExit(err error) int {
	return recordExitTo(os.Stderr, err)
}

// recordExitTo writes an owner error's refusal to stderr and returns its
// exit.
func recordExitTo(stderr io.Writer, err error) int {
	if err == nil {
		return 0
	}
	var op *dispatchcore.OpError
	if errors.As(err, &op) {
		if op.Message != "" || op.Reason != "" {
			fmt.Fprintln(stderr, op.Error())
		}
		return op.Code
	}
	fmt.Fprintln(stderr, err)
	return 1
}

type repeatedStringFlag []string

func (values *repeatedStringFlag) String() string { return strings.Join(*values, ",") }

func (values *repeatedStringFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func breachStopOrderingHumanWith(root string, caller lease.ClassifyResult, by string, now time.Time,
	enrolledName func(string, time.Time) (string, error)) (string, error) {
	typed := strings.TrimPrefix(by, "human:")
	if caller.Class != lease.ClassHuman {
		if typed != "" {
			return "", fmt.Errorf("breach stop: --by names the person ordering the stop; a %s caller records the custodian and takes no --by", caller.Class)
		}
		return "", nil
	}
	name, err := enrolledName(root, now)
	if err != nil {
		return "", fmt.Errorf("breach stop: the stop is admitted for a person and records who ordered it, and no enrolled person was proven here (%v); run it at the enrolled terminal, or enroll this one with metasystem system enroll --name NAME", err)
	}
	if typed != "" && typed != name {
		return "", fmt.Errorf("breach stop: --by %s is not the person enrolled at this terminal (%s); nothing was done", typed, name)
	}
	return name, nil
}

func runDispatchGoalRevisionAdmission(args []string) int {
	return runDispatchGoalRevisionAdmissionWithReads(args, dispatchcore.ConcreteProofAdmissionReads(), goalCommandNow)
}

func runDispatchGoalRevisionAdmissionWithReads(args []string, reads dispatchcore.ProofAdmissionReads, commandNow func(string) (time.Time, error)) int {
	flags := flag.NewFlagSet("job goal-revision-admission", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	revision := flags.Uint64("revision", 0, "exact accepted goal revision")
	proposedCap := flags.Uint64("proposed-cap", 0, "reserved minutes proposed by this dispatch")
	role := flags.String("role", "implementer", "role proposed by this dispatch")
	dispatchMode := flags.String("dispatch-mode", "fresh", "fresh or follow-up")
	destructiveReach := flags.String("destructive-reach", "", "MECHANICAL, DESIGN-BEARING, or DESTRUCTIVE-REACH")
	format := flags.String("format", "text", "text or json")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *goalID == "" || *revision == 0 || *proposedCap == 0 || *destructiveReach == "" {
		fmt.Fprintln(os.Stderr, "job goal-revision-admission: --root, --goal, --revision, a positive --proposed-cap, and --destructive-reach are required")
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintln(os.Stderr, "job goal-revision-admission: --format must be text or json")
		return 2
	}
	now, err := commandNow(*root)
	if err != nil {
		return recordExit(err)
	}
	verdict, err := dispatchcore.EvaluateGoalRevisionAdmissionForDispatchWithReads(*root, *goalID, *revision, *proposedCap, now, *role, *dispatchMode, reads, dispatchcore.HazardClass(*destructiveReach))
	if err != nil {
		return recordExit(err)
	}
	if *format == "json" {
		printJSON(verdict)
		if verdict.LiveStopReason != "" {
			return 10
		}
		if verdict.Refused() {
			return 9
		}
		return 0
	}
	if verdict.PolicyNotice != "" {
		fmt.Println(verdict.PolicyNotice)
	}
	if !verdict.Refused() {
		return 0
	}
	if verdict.PolicyRefusal != "" {
		fmt.Fprintln(os.Stderr, verdict.PolicyRefusal)
		return 9
	}
	for _, line := range dispatchcore.FormatGoalRevisionAdmission(verdict) {
		fmt.Println(line)
	}
	if verdict.LiveStopReason != "" {
		return 10
	}
	return 9
}
