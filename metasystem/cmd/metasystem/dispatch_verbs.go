package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/authority"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
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

func runDispatchVerifyReferences(args []string) int {
	flags := flag.NewFlagSet("job verify-references", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "control root")
	composition := flags.String("composition", "", "composition record")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 || *root == "" || *composition == "" {
		fmt.Fprintln(os.Stderr, "job verify-references: --root and --composition are required")
		return 2
	}
	absRoot, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	mismatches, err := dispatchcore.VerifyReferences(absRoot, *composition)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(mismatches) != 0 {
		for _, mismatch := range mismatches {
			fmt.Println(mismatch.Line())
		}
		return 9
	}
	data, err := os.ReadFile(*composition)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var record dispatchcore.CompositionRecord
	if err := json.Unmarshal(data, &record); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("references-verified count=%d\n", len(record.References))
	return 0
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

func runDispatchBreachStop(args []string) int {
	flags := flag.NewFlagSet("job breach-stop", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	revision := flags.Uint64("revision", 0, "exact accepted goal revision")
	by := flags.String("by", "", "the ordering person's name (a person's stop; default: the enrolled terminal's name)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *goalID == "" || *revision == 0 {
		fmt.Fprintln(os.Stderr, "job breach-stop: --root, --goal, and --revision are required")
		return 2
	}
	caller, err := classifyVerbCaller(*root, int64(os.Getppid()))
	if err != nil {
		return recordExit(fmt.Errorf("job breach-stop: caller authority is unreadable: %w", err))
	}
	if err := authority.Authorize("stop-custodian", map[string]any{
		"class": caller.Class, "holder": caller.Holder,
	}, ""); err != nil {
		return recordExit(err)
	}
	now, err := goalCommandNow(*root)
	if err != nil {
		return recordExit(err)
	}
	human, err := breachStopOrderingHuman(*root, caller, strings.TrimSpace(*by), now, humanauthority.Prove)
	if err != nil {
		return recordExit(err)
	}
	var batch goal.StopBatch
	if human != "" {
		batch, err = dispatchcore.EnsureBreachStopOrderedBy(*root, *goalID, *revision, now, human)
	} else {
		batch, err = dispatchcore.EnsureBreachStop(*root, *goalID, *revision, now)
	}
	if err != nil {
		return recordExit(err)
	}
	printJSON(batch)
	return 0
}

// breachStopOrderingHuman names the person a human-ordered breach stop
// records as its actor (rule H1): the enrolled terminal's recorded name,
// proven exactly as the other human verbs prove it (intentInvocation.actingAs
// through resolveGoalHuman). A --by must match that name. It returns "" for
// the machinery custodians, which record the custodian lineage as before,
// and refuses a --by from any caller that is not a person, since the name
// would then be forged attribution.
func breachStopOrderingHuman(root string, caller lease.ClassifyResult, by string, now time.Time,
	prove func(string, int64, humanauthority.Reader, time.Time) (humanauthority.Proof, error)) (string, error) {
	return breachStopOrderingHumanWith(root, caller, by, now, func(root string, now time.Time) (string, error) {
		proof, err := prove(root, int64(os.Getppid()), nil, now)
		if err != nil {
			return "", err
		}
		flags := &syncFlags{root: root}
		if err := resolveGoalHuman(flags, proof); err != nil {
			return "", err
		}
		return flags.by, nil
	})
}

func breachStopOrderingHumanWith(root string, caller lease.ClassifyResult, by string, now time.Time,
	enrolledName func(string, time.Time) (string, error)) (string, error) {
	typed := strings.TrimPrefix(by, "human:")
	if caller.Class != lease.ClassHuman {
		if typed != "" {
			return "", fmt.Errorf("job breach-stop: --by names the person ordering the stop; a %s caller records the custodian and takes no --by", caller.Class)
		}
		return "", nil
	}
	name, err := enrolledName(root, now)
	if err != nil {
		return "", fmt.Errorf("job breach-stop: the stop is admitted for a person and records who ordered it, and no enrolled person was proven here (%v); run it at the enrolled terminal, or enroll this one with metasystem system enroll --name NAME", err)
	}
	if typed != "" && typed != name {
		return "", fmt.Errorf("job breach-stop: --by %s is not the person enrolled at this terminal (%s); nothing was done", typed, name)
	}
	return name, nil
}

func runDispatchBreachStopRoutes(args []string) int {
	flags := flag.NewFlagSet("job breach-stop-routes", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "checkout root")
	if flags.Parse(args) != nil || *root == "" {
		return 2
	}
	now, err := goalCommandNow(*root)
	if err != nil {
		return recordExit(err)
	}
	routes, err := dispatchcore.FindBreachStops(*root, now)
	if err != nil {
		return recordExit(err)
	}
	for _, route := range routes {
		fmt.Printf("%s\t%d\t%s\t%s\n", route.GoalID, route.Revision, route.StopID, route.Failure)
	}
	return 0
}

// refuseRepeatedFlags is the strict-parse gate for authority-bearing
// verbs: a repeated flag would let a caller redirect the endpoint AFTER
// its wrapper's authority check authorized the first occurrence
// (certified finding DCD-AUTH-001), so any repetition refuses before
// parsing.
func refuseRepeatedFlags(name string, args []string) bool {
	seen := map[string]bool{}
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		flagName := strings.TrimLeft(arg, "-")
		if i := strings.IndexByte(flagName, '='); i >= 0 {
			flagName = flagName[:i]
		}
		if flagName == "" {
			continue
		}
		if seen[flagName] {
			fmt.Fprintf(os.Stderr, "%s: flag --%s repeated; authority-bearing flags parse strictly\n", name, flagName)
			return true
		}
		seen[flagName] = true
	}
	return false
}

func runDispatchCritiqueRegisterAdvance(args []string) int {
	if refuseRepeatedFlags("job critique-register-advance", args) {
		return 2
	}
	flags := flag.NewFlagSet("job critique-register-advance", flag.ContinueOnError)
	repo := pathFlag(flags, "repo", "", "checkout root")
	rootJob := flags.String("root-job", "", "critic chain root job id")
	roundJob := flags.String("round-job", "", "critic round job id")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" || *rootJob == "" || *roundJob == "" {
		fmt.Fprintln(os.Stderr, "job critique-register-advance: --repo, --root-job, and --round-job are required")
		return 2
	}
	outcome, err := dispatchcore.CritiqueRegisterAdvance(*repo, *rootJob, *roundJob)
	if err != nil {
		return recordExit(err)
	}
	fmt.Println(outcome)
	return 0
}

func runDispatchCritiqueRegisterClose(args []string) int {
	flags := flag.NewFlagSet("job critique-register-close", flag.ContinueOnError)
	repo := pathFlag(flags, "repo", ".", "checkout root")
	rootJob := flags.String("root-job", "", "critic root")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *rootJob == "" {
		fmt.Fprintln(os.Stderr, "job critique-register-close: --root-job is required")
		return 2
	}
	outcome, err := dispatchcore.CritiqueRegisterClose(*repo, *rootJob)
	if err != nil {
		return recordExit(err)
	}
	fmt.Println(outcome)
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
