package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/authority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
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
		if op.Message != "" {
			fmt.Fprintln(os.Stderr, op.Message)
		}
		return op.Code
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}

func runDispatchRecordCreate(args []string) int {
	flags := flag.NewFlagSet("job record-create", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	job := flags.String("job", "", "job id")
	source := flags.String("source", "", "initial pending-setup record file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *job == "" || *source == "" {
		fmt.Fprintln(os.Stderr, "job record-create: --root, --job, and --source are required")
		return 2
	}
	return recordExit(dispatchcore.RecordCreate(*root, *job, *source))
}

func runDispatchRecordSetup(args []string) int {
	flags := flag.NewFlagSet("job record-setup", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	job := flags.String("job", "", "job id")
	source := flags.String("source", "", "complete pending record file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *job == "" || *source == "" {
		fmt.Fprintln(os.Stderr, "job record-setup: --root, --job, and --source are required")
		return 2
	}
	return recordExit(dispatchcore.RecordSetup(*root, *job, *source))
}

func runDispatchRecordCAS(args []string) int {
	flags := flag.NewFlagSet("job record-cas", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	job := flags.String("job", "", "job id")
	expect := flags.String("expect", "", "status the record must currently hold")
	status := flags.String("status", "", "target status (equal to --expect for a metadata update)")
	patch := flags.String("patch", "", "JSON object patch file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *job == "" || *expect == "" || *status == "" || *patch == "" {
		fmt.Fprintln(os.Stderr, "job record-cas: --root, --job, --expect, --status, and --patch are required")
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
	flags := flag.NewFlagSet("job record-protocol-error", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	job := flags.String("job", "", "job id")
	expect := flags.String("expect", "", "status the record must currently hold")
	violation := flags.String("violation", "", "protocol violation text")
	violationFile := flags.String("violation-file", "", "file holding the violation text")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *job == "" || *expect == "" {
		fmt.Fprintln(os.Stderr, "job record-protocol-error: --root, --job, and --expect are required")
		return 2
	}
	return recordExit(dispatchcore.RecordProtocolError(*root, *job, *expect, *violation, *violationFile))
}

// runDispatchRepairClaim atomically claims the round's one paid
// repair: exit 0 won, exit 3 lost (already claimed or not running, the
// observation on stdout), exit 1 mechanical — the caller must treat 3 as
// a delegate-side outcome and 1 as a harness failure, never conflating.
func runDispatchRepairClaim(args []string) int {
	flags := flag.NewFlagSet("job repair-claim", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	job := flags.String("job", "", "job id")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *job == "" {
		fmt.Fprintln(os.Stderr, "job repair-claim: --root and --job are required")
		return 2
	}
	observed, err := dispatchcore.RepairClaim(*root, *job)
	if observed != "" {
		fmt.Println(observed)
	}
	return recordExit(err)
}

func runDispatchBuildSetup(args []string) int {
	flags := flag.NewFlagSet("job build-setup", flag.ContinueOnError)
	output := flags.String("output", "", "pending-setup record output file")
	job := flags.String("job", "", "job id")
	role := flags.String("role", "", "job role")
	parent := flags.String("parent", "", "parent job id for a follow-up reservation")
	mainID := flags.String("main-id", "", "dispatching main id")
	claimEpoch := flags.String("claim-epoch", "", "worktree-lease claim epoch")
	goalID := flags.String("goal", "", "goal id this job serves")
	goalRevision := flags.Uint64("goal-revision", 0, "accepted goal revision this reservation serves")
	machineID := flags.String("machine-id", "", "claim machine for a goal-bound reservation")
	approvedRef := flags.String("approved-ref", "", "recorded human approval for an oversized slice")
	capResolution := flags.String("cap-resolution", "", "final cap-resolution file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *output == "" || *job == "" || *role == "" || *capResolution == "" {
		fmt.Fprintln(os.Stderr, "job build-setup: --output, --job, --role, and --cap-resolution are required")
		return 2
	}
	return recordExit(dispatchcore.BuildSetup(*output, *job, *role, *parent, *mainID, *claimEpoch, *goalID, *goalRevision, *capResolution, *machineID, *approvedRef))
}

func runDispatchSliceAdmission(args []string) int {
	flags := flag.NewFlagSet("job slice-admission", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	capMinutes := flags.Uint64("cap-min", 0, "final reservation cap in minutes")
	approvedRef := flags.String("approved-ref", "", "recorded human approval reference")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *capMinutes == 0 {
		fmt.Fprintln(os.Stderr, "job slice-admission: --root and positive --cap-min are required")
		return 2
	}
	verdict, err := dispatchcore.EvaluateSliceAdmission(*root, *capMinutes, *approvedRef)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if verdict.Refused() {
		fmt.Fprintln(os.Stderr, verdict.Refusal)
		return 9
	}
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

func runDispatchBuildRecord(args []string) int {
	flags := flag.NewFlagSet("job build-record", flag.ContinueOnError)
	var p dispatchcore.BuildRecordParams
	flags.StringVar(&p.Output, "output", "", "pending record output file")
	flags.StringVar(&p.Job, "job", "", "job id")
	flags.StringVar(&p.Role, "role", "", "job role")
	flags.StringVar(&p.Mission, "mission", "", "mission id (optional)")
	flags.StringVar(&p.MissionTurn, "mission-turn", "", "mission turn id (optional)")
	flags.StringVar(&p.Stream, "stream", "", "mission stream this dispatch serves (optional)")
	flags.StringVar(&p.Root, "root", "", "dispatching checkout root (required with --mission)")
	flags.StringVar(&p.Runtime, "runtime", "", "runtime name")
	flags.StringVar(&p.Workspace, "workspace", "", "job workspace root")
	flags.StringVar(&p.CapResolution, "cap-resolution", "", "cap-resolution file")
	flags.StringVar(&p.Model, "model", "", "requested model")
	overridden := strictBool(flags, "overridden", "true", "false", "true when runtime or model was overridden")
	flags.StringVar(&p.Snapshot, "snapshot", "", "capability snapshot path")
	flags.Int64Var(&p.InputBytes, "input-bytes", 0, "brief size in bytes")
	flags.StringVar(&p.InputHash, "input-hash", "", "brief SHA-256")
	flags.StringVar(&p.Permissions, "permissions", "", "requested-permissions envelope file")
	flags.StringVar(&p.Fallbacks, "fallbacks", "", "capability fallbacks JSON")
	signal := strictBool(flags, "signal", "true", "false", "true when the runtime signals session establishment")
	flags.Int64Var(&p.HandshakeBudget, "handshake-budget", 0, "session-established timeout seconds")
	flags.StringVar(&p.ApprovalName, "approval-name", "", "escalation approval name (optional)")
	flags.StringVar(&p.ApprovedAt, "approved-at", "", "escalation approval timestamp")
	flags.StringVar(&p.RosterPair, "roster-pair", "", "roster runtime:model resolution")
	flags.StringVar(&p.RequestedPair, "requested-pair", "", "requested runtime:model pair")
	flags.StringVar(&p.CostDirection, "cost-direction", "", "displayed escalation cost direction")
	flags.StringVar(&p.Reviews, "reviews", "", "implementer job id under review (optional)")
	flags.StringVar(&p.GoalID, "goal", "", "goal id this job serves")
	flags.Uint64Var(&p.GoalRevision, "goal-revision", 0, "accepted goal revision this reservation serves")
	flags.StringVar(&p.MachineID, "machine-id", "", "claim machine for a goal-bound reservation")
	flags.StringVar(&p.ApprovedRef, "approved-ref", "", "recorded human approval for an oversized slice")
	flags.StringVar(&p.MainID, "main-id", "", "dispatching main id")
	flags.StringVar(&p.ClaimEpoch, "claim-epoch", "", "worktree-lease claim epoch")
	if flags.Parse(args) != nil {
		return 2
	}
	if p.Output == "" || p.Job == "" || p.Role == "" || p.Runtime == "" || p.Workspace == "" ||
		p.CapResolution == "" || p.Permissions == "" || p.Fallbacks == "" {
		fmt.Fprintln(os.Stderr, "job build-record: --output, --job, --role, --runtime, --workspace, --cap-resolution, --permissions, and --fallbacks are required")
		return 2
	}
	p.Overridden = *overridden
	p.Signal = *signal
	return recordExit(dispatchcore.BuildRecord(p))
}

func runDispatchBuildFollowRecord(args []string) int {
	flags := flag.NewFlagSet("job build-follow-record", flag.ContinueOnError)
	var p dispatchcore.BuildFollowRecordParams
	flags.StringVar(&p.Output, "output", "", "pending record output file")
	flags.StringVar(&p.Parent, "parent", "", "parent (latest) record file")
	flags.StringVar(&p.Job, "job", "", "follow-up job id")
	flags.Int64Var(&p.Round, "round", 0, "follow-up round number")
	flags.StringVar(&p.ParentJob, "parent-job", "", "parent job id")
	flags.StringVar(&p.Snapshot, "snapshot", "", "capability snapshot path")
	flags.StringVar(&p.Fallbacks, "fallbacks", "", "capability fallbacks JSON")
	signal := strictBool(flags, "signal", "true", "false", "true when the runtime signals session establishment")
	flags.Int64Var(&p.HandshakeBudget, "handshake-budget", 0, "session-established timeout seconds")
	flags.StringVar(&p.ResumeMode, "resume-mode", "", "resumed or fresh-context")
	flags.Int64Var(&p.InputBytes, "input-bytes", 0, "delivery size in bytes")
	flags.StringVar(&p.InputHash, "input-hash", "", "delivery SHA-256")
	flags.StringVar(&p.MissionTurn, "mission-turn", "", "mission turn id (optional)")
	flags.StringVar(&p.MainID, "main-id", "", "dispatching main id")
	flags.StringVar(&p.ClaimEpoch, "claim-epoch", "", "worktree-lease claim epoch")
	flags.StringVar(&p.CapResolution, "cap-resolution", "", "cap-resolution file")
	flags.StringVar(&p.Root, "root", "", "dispatching checkout root (required for mission chains)")
	flags.Uint64Var(&p.GoalRevision, "goal-revision", 0, "accepted goal revision this reservation serves")
	flags.StringVar(&p.ApprovedRef, "approved-ref", "", "recorded human approval for an oversized slice")
	if flags.Parse(args) != nil {
		return 2
	}
	if p.Output == "" || p.Parent == "" || p.Job == "" || p.Round < 2 || p.ParentJob == "" ||
		p.Fallbacks == "" || p.ResumeMode == "" || p.CapResolution == "" {
		fmt.Fprintln(os.Stderr, "job build-follow-record: --output, --parent, --job, --round (>=2), --parent-job, --fallbacks, --resume-mode, and --cap-resolution are required")
		return 2
	}
	p.Signal = *signal
	return recordExit(dispatchcore.BuildFollowRecord(p))
}

func runDispatchGoalRevision(args []string) int {
	flags := flag.NewFlagSet("job goal-revision", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *goalID == "" {
		fmt.Fprintln(os.Stderr, "job goal-revision: --root and --goal are required")
		return 2
	}
	revision, err := dispatchcore.ResolveGoalRevision(*root, *goalID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(revision)
	return 0
}

func runDispatchGoalBinding(args []string) int {
	flags := flag.NewFlagSet("job goal-binding", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *goalID == "" {
		fmt.Fprintln(os.Stderr, "job goal-binding: --root and --goal are required")
		return 2
	}
	now, err := goalCommandNow(*root)
	if err != nil {
		return recordExit(err)
	}
	binding, err := dispatchcore.ResolveGoalBinding(*root, *goalID, now)
	if err != nil {
		return recordExit(err)
	}
	printJSON(map[string]any{
		"goalId": binding.GoalID, "goalRevision": binding.Revision,
		"machineId": binding.Machine, "claimEpoch": binding.Capability.ClaimEpoch,
		"capabilityGeneration": binding.Capability.Generation,
		"fenceEpoch":           binding.Capability.FenceEpoch, "fenced": binding.Fence != nil,
	})
	return 0
}

func runDispatchGoalLockPath(args []string) int {
	flags := flag.NewFlagSet("job goal-lock-path", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	revision := flags.Uint64("revision", 0, "goal revision")
	if flags.Parse(args) != nil {
		return 2
	}
	path, err := dispatchcore.GoalRevisionLockDir(*root, *goalID, *revision)
	if err != nil {
		return recordExit(err)
	}
	fmt.Println(path)
	return 0
}

func runDispatchGoalAdmission(args []string) int {
	flags := flag.NewFlagSet("job goal-admission", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
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
	flags := flag.NewFlagSet("job goal-revision-admission", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	revision := flags.Uint64("revision", 0, "exact accepted goal revision")
	proposedCap := flags.Uint64("proposed-cap", 0, "reserved minutes proposed by this dispatch")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *goalID == "" || *revision == 0 {
		fmt.Fprintln(os.Stderr, "job goal-revision-admission: --root, --goal, and --revision are required")
		return 2
	}
	now, err := goalCommandNow(*root)
	if err != nil {
		return recordExit(err)
	}
	verdict, err := dispatchcore.EvaluateGoalRevisionAdmission(*root, *goalID, *revision, *proposedCap, now)
	if err != nil {
		return recordExit(err)
	}
	if !verdict.Refused() {
		return 0
	}
	for _, line := range dispatchcore.FormatGoalAdmission(dispatchcore.GoalAdmissionVerdict{Refusals: []dispatchcore.GoalAdmissionRefusal{*verdict.Refusal}}) {
		fmt.Println(line)
	}
	if verdict.LiveStopReason != "" {
		return 10
	}
	return 9
}

func runDispatchBreachStop(args []string) int {
	flags := flag.NewFlagSet("job breach-stop", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	revision := flags.Uint64("revision", 0, "exact accepted goal revision")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *goalID == "" || *revision == 0 {
		fmt.Fprintln(os.Stderr, "job breach-stop: --root, --goal, and --revision are required")
		return 2
	}
	caller, err := lease.ClassifyVerb(*root, int64(os.Getppid()))
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
	batch, err := dispatchcore.EnsureBreachStop(*root, *goalID, *revision, now)
	if err != nil {
		return recordExit(err)
	}
	printJSON(batch)
	return 0
}

func runDispatchBreachStopRoutes(args []string) int {
	flags := flag.NewFlagSet("job breach-stop-routes", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
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

func runDispatchStopBatchReconcile(args []string) int {
	flags := flag.NewFlagSet("job stop-batch-reconcile", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	stopID := flags.String("stop", "", "stop batch id")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *stopID == "" {
		return 2
	}
	now, err := goalCommandNow(*root)
	if err != nil {
		return recordExit(err)
	}
	batch, err := dispatchcore.ReconcileStopBatch(*root, *stopID, now)
	if err != nil {
		return recordExit(err)
	}
	printJSON(batch)
	if batch.State == "INDETERMINATE" {
		return 11
	}
	return 0
}

func runDispatchStopBatchPending(args []string) int {
	flags := flag.NewFlagSet("job stop-batch-pending", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	stopID := flags.String("stop", "", "stop batch id")
	if flags.Parse(args) != nil || *root == "" || *stopID == "" {
		return 2
	}
	batch, err := goal.ReadStopBatch(*root, *stopID)
	if err != nil {
		return recordExit(err)
	}
	for _, job := range batch.Pending {
		fmt.Println(job)
	}
	return 0
}

func runDispatchStopCancelAuthorize(args []string) int {
	flags := flag.NewFlagSet("job stop-cancel-authorize", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	stopID := flags.String("stop", "", "stop batch id")
	jobID := flags.String("job", "", "job id")
	if flags.Parse(args) != nil || *root == "" || *stopID == "" || *jobID == "" {
		return 2
	}
	return recordExit(dispatchcore.AuthorizeStopCancellation(*root, *stopID, *jobID))
}

// runDispatchVerifyChainIncarnation relays the pre-authorization incarnation
// check: the shell calls it before any cap/fence side effect so the named
// re-provision refusal is the FIRST refusal a stale chain can hit.
func runDispatchVerifyChainIncarnation(args []string) int {
	flags := flag.NewFlagSet("job verify-chain-incarnation", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	mission := flags.String("mission", "", "mission id")
	parent := flags.String("parent", "", "parent (latest) record file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *mission == "" || *parent == "" {
		fmt.Fprintln(os.Stderr, "job verify-chain-incarnation: --root, --mission, and --parent are required")
		return 2
	}
	record, err := dispatchcore.ReadRecordObject(*parent)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return recordExit(dispatchcore.VerifyChainIncarnation(*root, *mission, record))
}

func runDispatchLatestChainRecord(args []string) int {
	flags := flag.NewFlagSet("job latest-chain-record", flag.ContinueOnError)
	jobs := flags.String("jobs", "", "jobs directory")
	root := flags.String("root", "", "chain root job id")
	if flags.Parse(args) != nil {
		return 2
	}
	if *jobs == "" || *root == "" {
		fmt.Fprintln(os.Stderr, "job latest-chain-record: --jobs and --root are required")
		return 2
	}
	path, err := dispatchcore.LatestChainRecord(*jobs, *root)
	if err != nil {
		return recordExit(err)
	}
	fmt.Println(path)
	return 0
}

func runDispatchChainMembers(args []string) int {
	flags := flag.NewFlagSet("job chain-members", flag.ContinueOnError)
	jobs := flags.String("jobs", "", "jobs directory")
	root := flags.String("root", "", "chain root job id")
	terminalOnly := flags.Bool("terminal-only", false, "list only terminal records")
	if flags.Parse(args) != nil {
		return 2
	}
	if *jobs == "" || *root == "" {
		fmt.Fprintln(os.Stderr, "job chain-members: --jobs and --root are required")
		return 2
	}
	lines, err := dispatchcore.ChainMemberStatuses(*jobs, *root, *terminalOnly)
	if err != nil {
		return recordExit(err)
	}
	for _, line := range lines {
		fmt.Println(line)
	}
	return 0
}

func runDispatchChainUsage(args []string) int {
	flags := flag.NewFlagSet("job chain-usage", flag.ContinueOnError)
	jobs := flags.String("jobs", "", "jobs directory")
	root := flags.String("root", "", "chain root job id")
	output := flags.String("output", "", "chain-usage patch output file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *jobs == "" || *root == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "job chain-usage: --jobs, --root, and --output are required")
		return 2
	}
	unchanged, err := dispatchcore.ChainUsage(*jobs, *root, *output)
	if err != nil {
		return recordExit(err)
	}
	if unchanged {
		// The aggregate already matches the root record; the caller skips the
		// metadata CAS entirely.
		return 7
	}
	return 0
}

func runDispatchCustodyAdd(args []string) int {
	flags := flag.NewFlagSet("job custody-add", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	job := flags.String("job", "", "job id")
	pid := flags.Int64("pid", 0, "custody process id")
	pidStarted := flags.Int64("pid-started", 0, "custody process kernel start time (epoch seconds)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *job == "" || *pid < 1 || *pidStarted < 1 {
		fmt.Fprintln(os.Stderr, "job custody-add: --root, --job, --pid, and --pid-started are required")
		return 2
	}
	return recordExit(dispatchcore.CustodyAdd(*root, *job, *pid, *pidStarted))
}

func runDispatchHandshakeEval(args []string) int {
	flags := flag.NewFlagSet("job handshake-eval", flag.ContinueOnError)
	record := flags.String("record", "", "job record file")
	effective := flags.String("effective", "", "effective permissions file")
	session := flags.String("session", "", "session id the adapter reported")
	turn := flags.String("turn", "", "turn id the adapter reported")
	model := flags.String("model", "", "effective model the adapter reported")
	signal := strictBool(flags, "signal", "true", "false", "true when the runtime promised a session signal")
	output := flags.String("output", "", "output file for {target, patch}")
	if flags.Parse(args) != nil {
		return 2
	}
	if *record == "" || *effective == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "job handshake-eval: --record, --effective, and --output are required")
		return 2
	}
	return recordExit(dispatchcore.HandshakeEval(*record, *effective, *session, *turn, *model, *signal, *output))
}

func runDispatchReapFacts(args []string) int {
	flags := flag.NewFlagSet("job reap-facts", flag.ContinueOnError)
	record := flags.String("record", "", "job record file")
	grace := flags.Int64("grace", dispatchcore.HandshakeBackstopGraceSec, "seconds past the handshake deadline before the backstop acts")
	if flags.Parse(args) != nil {
		return 2
	}
	if *record == "" {
		fmt.Fprintln(os.Stderr, "job reap-facts: --record is required")
		return 2
	}
	facts, err := dispatchcore.ComputeReapFacts(*record, *grace, time.Now())
	if err != nil {
		return recordExit(err)
	}
	encoded, err := json.Marshal(facts)
	if err != nil {
		return recordExit(err)
	}
	fmt.Println(string(encoded))
	return 0
}

func runDispatchCensusFresh(args []string) int {
	flags := flag.NewFlagSet("job census-fresh", flag.ContinueOnError)
	verdict := flags.String("verdict", "", "last-census verdict file")
	state := flags.String("state", "", "supervision arming record file")
	arm := flags.String("arm", "", "re-arm command named in refusal messages")
	repo := flags.String("repo", "", "repository path named in refusal messages")
	root := flags.String("root", "", "metasystem root; when set, the verdict's fingerprint must match the armed code")
	if flags.Parse(args) != nil {
		return 2
	}
	if *verdict == "" || *state == "" {
		fmt.Fprintln(os.Stderr, "job census-fresh: --verdict and --state are required")
		return 2
	}
	expected := ""
	if *root != "" {
		fp, err := census.Fingerprint(*root, *repo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dispatch refused: census fingerprint cannot be computed: %v\n", err)
			return 1
		}
		expected = fp
	}
	if err := dispatchcore.CensusFresh(*verdict, *state, *arm, *repo, expected, time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		var armingWindow dispatchcore.ArmingWindowError
		if errors.As(err, &armingWindow) {
			return 9 // the retry-safe arming-window transient
		}
		return 1
	}
	return 0
}

func runDispatchWatcherCeiling(args []string) int {
	flags := flag.NewFlagSet("job watcher-ceiling", flag.ContinueOnError)
	state := flags.String("state", "", "supervision state file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *state == "" {
		fmt.Fprintln(os.Stderr, "job watcher-ceiling: --state is required")
		return 2
	}
	ceiling, err := dispatchcore.WatcherCeiling(*state, time.Now())
	if err != nil {
		return recordExit(err)
	}
	fmt.Println(ceiling)
	return 0
}

func runDispatchExpandPermissions(args []string) int {
	flags := flag.NewFlagSet("job expand-permissions", flag.ContinueOnError)
	source := flags.String("source", "", "permissions envelope file")
	repo := flags.String("repo", "", "repository root")
	workspace := flags.String("workspace", "", "job workspace root")
	worktree := strictBool(flags, "worktree", "1", "0", "1 when the workspace is a job worktree")
	preset := flags.String("preset", "", "preset name (or custom)")
	networkFloor := flags.String("network-floor", "", "repository network floor: deny, allow, or empty")
	output := flags.String("output", "", "expanded envelope output file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *source == "" || *repo == "" || *workspace == "" || *preset == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "job expand-permissions: --source, --repo, --workspace, --preset, and --output are required")
		return 2
	}
	return recordExit(dispatchcore.ExpandPermissions(*source, *repo, *workspace, *worktree, *preset, *networkFloor, *output))
}

func runDispatchValidateMission(args []string) int {
	flags := flag.NewFlagSet("job validate-mission", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	mission := flags.String("mission", "", "mission id")
	lease := flags.String("lease", "", "mission lease path")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *mission == "" || *lease == "" {
		fmt.Fprintln(os.Stderr, "job validate-mission: --root, --mission, and --lease are required")
		return 2
	}
	return recordExit(dispatchcore.ValidateMission(*root, *mission, *lease))
}

func runDispatchMirror(args []string) int {
	flags := flag.NewFlagSet("job mirror", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	checkout := flags.String("checkout", "", "repository scope the evidence segment derives from")
	evidence := flags.String("evidence", "", "evidence root (absolute, outside the repository)")
	rootJob := flags.String("root-job", "", "chain root job id")
	job := flags.String("job", "", "job id to mirror")
	result := flags.String("result", "", "mirror result output file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" || *checkout == "" || *evidence == "" || *rootJob == "" || *job == "" || *result == "" {
		fmt.Fprintln(os.Stderr, "job mirror: --repo, --checkout, --evidence, --root-job, --job, and --result are required")
		return 2
	}
	return recordExit(dispatchcore.Mirror(*repo, *checkout, *evidence, *rootJob, *job, *result))
}

func runDispatchCloseCheck(args []string) int {
	flags := flag.NewFlagSet("job close-check", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	root := flags.String("root", "", "chain root job id")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" || *root == "" {
		fmt.Fprintln(os.Stderr, "job close-check: --repo and --root are required")
		return 2
	}
	return recordExit(dispatchcore.CloseCheck(*repo, *root))
}

func runDispatchCritiqueExhaustion(args []string) int {
	flags := flag.NewFlagSet("job critique-exhaustion", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	rootJob := flags.String("root-job", "", "chain root job id")
	role := flags.String("role", "", "follow-up role")
	latest := flags.String("latest", "", "latest chain record file")
	message := flags.String("message", "", "successor message file")
	successor := flags.String("successor", "", "successor job id")
	output := flags.String("output", "", "exhaustion manifest output file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" || *rootJob == "" || *role == "" || *latest == "" || *message == "" || *successor == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "job critique-exhaustion: --repo, --root-job, --role, --latest, --message, --successor, and --output are required")
		return 2
	}
	action, err := dispatchcore.CritiqueExhaustionAction(*repo, *rootJob, *role, *latest, *message, *successor, *output)
	if err != nil {
		return recordExit(err)
	}
	fmt.Println(action)
	return 0
}

func runDispatchExhaustionPatches(args []string) int {
	flags := flag.NewFlagSet("job exhaustion-patches", flag.ContinueOnError)
	manifest := flags.String("manifest", "", "exhaustion manifest file")
	dir := flags.String("dir", "", "directory for the patch files")
	if flags.Parse(args) != nil {
		return 2
	}
	if *manifest == "" || *dir == "" {
		fmt.Fprintln(os.Stderr, "job exhaustion-patches: --manifest and --dir are required")
		return 2
	}
	lines, err := dispatchcore.ExhaustionPatches(*manifest, *dir)
	if err != nil {
		return recordExit(err)
	}
	for _, line := range lines {
		fmt.Println(line)
	}
	return 0
}

// runDispatchResolveCap relays `job resolve-cap`: the non-mission cap chain
// and the unsigned-mission-cap refusal live in dispatchcore. With
// --mission it only runs the refusal; the mission fence remains cap
// authority.
func runDispatchResolveCap(args []string) int {
	flags := flag.NewFlagSet("job resolve-cap", flag.ContinueOnError)
	conf := flags.String("conf", "", "path to metasystem.conf")
	role := flags.String("role", "", "dispatch role")
	runtime := flags.String("runtime", "", "resolved runtime")
	model := flags.String("model", "", "canonical model")
	requested := flags.String("requested", "", "explicit cap-min argument (optional)")
	mission := flags.Bool("mission", false, "only refuse unsigned mission cap overrides")
	output := flags.String("output", "", "cap-resolution output file (non-mission)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *conf == "" || *role == "" || *runtime == "" || *model == "" {
		fmt.Fprintln(os.Stderr, "job resolve-cap: --conf, --role, --runtime, and --model are required")
		return 2
	}
	if *mission {
		if err := dispatchcore.RefuseUnsignedMissionCap(*conf, *role, *runtime, *model); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	if *output == "" {
		fmt.Fprintln(os.Stderr, "job resolve-cap: --output is required without --mission")
		return 2
	}
	capMin, rule, origin, err := dispatchcore.ResolveCap(*conf, *role, *runtime, *model, *requested)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return recordExit(dispatchcore.WriteCapResolution(*output, capMin, rule, origin))
}

func runDispatchCapResolution(args []string) int {
	flags := flag.NewFlagSet("job cap-resolution", flag.ContinueOnError)
	capMin := flags.Int64("cap", 0, "authorized cap in minutes")
	rule := flags.String("rule", "", "resolution rule name")
	origin := flags.String("origin", "", "configuration origin of the rule")
	output := flags.String("output", "", "cap-resolution output file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *capMin < 1 || *rule == "" || *origin == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "job cap-resolution: --cap (>=1), --rule, --origin, and --output are required")
		return 2
	}
	return recordExit(dispatchcore.WriteCapResolution(*output, *capMin, *rule, *origin))
}

func runDispatchBriefMode(args []string) int {
	flags := flag.NewFlagSet("job brief-mode", flag.ContinueOnError)
	brief := flags.String("brief", "", "brief file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *brief == "" {
		fmt.Fprintln(os.Stderr, "job brief-mode: --brief is required")
		return 2
	}
	mode, err := dispatchcore.BriefMode(*brief)
	if err != nil {
		return recordExit(err)
	}
	fmt.Println(mode)
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

// runDispatchServingGoal resolves --serving-goal at dispatch setup: the
// section on stdout, or exit 3 when no usable Current goal exists — the
// refusal the orchestrator asked for by requesting a projection.
func runDispatchServingGoal(args []string) int {
	flags := flag.NewFlagSet("dispatch serving-goal", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	section, err := dispatchcore.ServingGoalSection(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 3
	}
	fmt.Print(section)
	return 0
}
