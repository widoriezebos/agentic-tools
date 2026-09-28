package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
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
	if err == nil {
		return 0
	}
	var op *dispatchcore.OpError
	if errors.As(err, &op) {
		if op.Message != "" || op.Reason != "" {
			fmt.Fprintln(os.Stderr, op.Error())
		}
		return op.Code
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}

type repeatedStringFlag []string

func (values *repeatedStringFlag) String() string { return strings.Join(*values, ",") }

func (values *repeatedStringFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

// runDispatchResolveRoster relays `job resolve-roster`: the roster, tier,
// and escalation decisions live in dispatchcore.ResolveRoster; the
// shell keeps only the approval ladder.
func runDispatchResolveRoster(args []string) int {
	flags := flag.NewFlagSet("job resolve-roster", flag.ContinueOnError)
	conf := flags.String("conf", "", "path to metasystem.conf")
	role := flags.String("role", "", "dispatch role")
	mode := flags.String("mode", "", "working mode scope")
	runtimeOverride := flags.String("runtime-override", "", "requested runtime (optional)")
	modelOverride := flags.String("model-override", "", "requested model (optional)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *conf == "" || *role == "" {
		fmt.Fprintln(os.Stderr, "job resolve-roster: --conf and --role are required")
		return 2
	}
	resolution, err := dispatchcore.ResolveRoster(dispatchcore.RosterParams{
		ConfPath: *conf, Role: *role, Mode: *mode,
		RuntimeOverride: *runtimeOverride, ModelOverride: *modelOverride,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printJSON(resolution)
	return 0
}

// runDispatchCapContinuation writes the prior-worktree paragraph a
// continuation round is told after its predecessor was cut off at its cap.
func runDispatchCapContinuation(args []string) int {
	flags := flag.NewFlagSet("job cap-continuation", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "MetaSystem installation root (relative paths resolve against it)")
	parent := flags.String("parent", "", "the capped parent round's record")
	worktree := flags.String("worktree", "", "the chain's job worktree")
	output := flags.String("output", "", "paragraph output file")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "job cap-continuation: positional arguments are not accepted")
		return 2
	}
	if *parent == "" || *worktree == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "job cap-continuation: --parent, --worktree and --output are required")
		return 2
	}
	resolve := func(path string) string {
		if filepath.IsAbs(path) {
			return path
		}
		return filepath.Join(*root, path)
	}
	*parent, *worktree, *output = resolve(*parent), resolve(*worktree), resolve(*output)
	text, err := dispatchcore.CapContinuationText(*parent, *worktree)
	if err != nil {
		fmt.Fprintln(os.Stderr, "job cap-continuation:", err)
		return 1
	}
	if err := os.WriteFile(*output, []byte(text), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "job cap-continuation:", err)
		return 1
	}
	return 0
}

func runDispatchGoalAdmission(args []string) int {
	flags := flag.NewFlagSet("job goal-admission", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "checkout root")
	stopLineage := flags.String("stop-lineage", "", "lineage whose owned claim may refuse admission")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" {
		fmt.Fprintln(os.Stderr, "job goal-admission: --root is required")
		return 2
	}
	now, err := goalCommandNow(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	verdict, err := dispatchcore.EvaluateGoalAdmission(*root, *stopLineage, now)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, line := range dispatchcore.FormatGoalAdmission(verdict) {
		fmt.Println(line)
	}
	if verdict.Refused() {
		for _, refusal := range verdict.Refusals {
			if refusal.LiveStopReason != "" {
				return 10
			}
		}
		return 9
	}
	return 0
}

// runDispatchOwnerLock claims or releases the dispatch owner lock.
// Exit codes: 0 done, 3 busy, 4 not-owner.
func runDispatchOwnerLock(args []string) int {
	flags := flag.NewFlagSet("job owner-lock", flag.ContinueOnError)
	command := flags.String("command", "", "claim | release")
	directory := flags.String("dir", "", "lock directory")
	pid := flags.Int64("pid", 0, "claimant pid")
	tag := flags.String("tag", "", "claimant instance tag")
	if flags.Parse(args) != nil {
		return 2
	}
	if *directory == "" || *pid < 1 || *tag == "" {
		fmt.Fprintln(os.Stderr, "job owner-lock: --command, --dir, --pid, and --tag are required")
		return 2
	}
	switch *command {
	case "claim":
		switch err := dispatchcore.OwnerLockClaim(*directory, *pid, *tag); err {
		case nil:
			return 0
		case dispatchcore.ErrOwnerLockBusy:
			return 3
		default:
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	case "release":
		switch err := dispatchcore.OwnerLockRelease(*directory, *pid, *tag); err {
		case nil:
			return 0
		case dispatchcore.ErrOwnerLockNotOwner:
			return 4
		default:
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	fmt.Fprintln(os.Stderr, "job owner-lock: --command must be claim or release")
	return 2
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
