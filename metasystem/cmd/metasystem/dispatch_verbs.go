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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/janitor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"golang.org/x/sys/unix"
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

type hazardClassFlag dispatchcore.HazardClass

func (value *hazardClassFlag) String() string       { return string(*value) }
func (value *hazardClassFlag) Set(raw string) error { *value = hazardClassFlag(raw); return nil }

func (values *repeatedStringFlag) String() string { return strings.Join(*values, ",") }

func (values *repeatedStringFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func runDispatchComposeRolePacket(args []string) int {
	flags := flag.NewFlagSet("job compose-role-packet", flag.ContinueOnError)
	root := flags.String("root", "", "metasystem checkout root")
	role := flags.String("role", "", "role recipe")
	brief := flags.String("brief", "", "task-direction file")
	job := flags.String("job", "", "job id")
	runtimeName := flags.String("runtime", "", "runtime name")
	model := flags.String("model", "", "model name")
	toolPolicy := flags.String("tool-policy", "", "resolved permission tool policy")
	round := flags.Int64("round", 0, "job round")
	mission := flags.String("mission", "", "mission id")
	destructiveReach := flags.String("destructive-reach", "", "MECHANICAL, DESIGN-BEARING, or DESTRUCTIVE-REACH")
	output := flags.String("output", "", "assembled packet output")
	composition := flags.String("composition", "", "composition record output")
	validateOnly := flags.Bool("validate-only", false, "validate asserted sources without reading or writing a packet")
	var sources repeatedStringFlag
	var continuations repeatedStringFlag
	flags.Var(&sources, "source", "asserted packet source (repeatable)")
	flags.Var(&continuations, "continuation", "engine continuation slot=path (repeatable)")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "job compose-role-packet: positional arguments are not accepted")
		return 2
	}
	if *validateOnly {
		if *root == "" || *role == "" {
			fmt.Fprintln(os.Stderr, "job compose-role-packet --validate-only requires --root and --role")
			return 2
		}
		_, _, err := dispatchcore.ValidateRolePacketSources(*root, *role, sources)
		if err == nil {
			return 0
		}
		var refusal *dispatchcore.CompositionRefusal
		if errors.As(err, &refusal) {
			printJSON(map[string]any{"outcome": refusal.Code, "headline": "refused", "source": refusal.Source, "detail": refusal.Detail})
			return 9
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	continuationInputs := make([]dispatchcore.CompositionContinuation, 0, len(continuations))
	for _, encoded := range continuations {
		slot, path, found := strings.Cut(encoded, "=")
		if !found || slot == "" || path == "" {
			fmt.Fprintln(os.Stderr, "job compose-role-packet: --continuation must be slot=path")
			return 2
		}
		continuationInputs = append(continuationInputs, dispatchcore.CompositionContinuation{Slot: slot, Path: path})
	}
	_, err := dispatchcore.ComposeRolePacket(dispatchcore.ComposeRolePacketParams{
		Root: *root, Role: *role, Brief: *brief, JobID: *job, Runtime: *runtimeName,
		Model: *model, ToolPolicy: *toolPolicy, Round: *round, Mission: *mission, DestructiveReach: dispatchcore.HazardClass(*destructiveReach), Output: *output,
		CompositionOutput: *composition, ExtraSources: sources, Continuations: continuationInputs,
	})
	if err == nil {
		return 0
	}
	var refusal *dispatchcore.CompositionRefusal
	if errors.As(err, &refusal) {
		printJSON(map[string]any{"outcome": refusal.Code, "headline": "refused", "source": refusal.Source, "detail": refusal.Detail})
		return 9
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}

func runDispatchOperationID(args []string) int {
	flags := flag.NewFlagSet("job operation-id", flag.ContinueOnError)
	goalID := flags.String("goal", "", "goal id")
	revision := flags.Uint64("goal-revision", 0, "accepted goal revision")
	mode := flags.String("dispatch-mode", "", "fresh or follow-up")
	role := flags.String("role", "", "role")
	briefDigest := flags.String("brief-digest", "", "task-direction SHA-256")
	parentJob := flags.String("parent-job", "", "direct parent job for a follow-up")
	if flags.Parse(args) != nil {
		return 2
	}
	id, err := dispatchcore.DefaultOperationID(*goalID, *revision, dispatchcore.DispatchMode(*mode), *role, *briefDigest, *parentJob)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	fmt.Println(id)
	return 0
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

func runDispatchClaimLaunch(args []string) int {
	if refuseRepeatedFlags("job claim-launch", args) {
		return 2
	}
	flags := flag.NewFlagSet("job claim-launch", flag.ContinueOnError)
	root := flags.String("root", "", "Git checkout root")
	opid := flags.String("opid", "", "idempotent launch operation id")
	operationID := flags.String("operation-id", "", "reservation operation identity; defaults to opid")
	session := flags.String("session", "", "namespaced session key")
	dispatchMode := flags.String("dispatch-mode", "", "fresh or follow-up")
	resumedSession := flags.String("resumed-session", "", "runtime session id resumed by a follow-up")
	runtimeName := flags.String("runtime", "", "runtime name")
	model := flags.String("model", "", "requested model name")
	aliasSource := flags.String("alias-source", "", "canonical model alias source (optional)")
	role := flags.String("role", "", "dispatch role")
	launchMode := flags.String("launch-mode", "", "worktree or shared-checkout")
	permissionDigest := flags.String("permission-envelope-digest", "", "requested permission envelope SHA-256")
	capMin := flags.String("cap-min", "", "requested cap minutes; omitted uses configured defaults")
	conf := flags.String("conf", "", "configuration file used for cap defaults")
	inputHash := flags.String("input-hash", "", "launch input SHA-256")
	mainID := flags.String("main-id", "", "dispatching main id")
	claimEpoch := flags.String("claim-epoch", "", "worktree-lease claim epoch")
	goalID := flags.String("goal", "", "goal id this launch serves")
	goalRevision := flags.Uint64("goal-revision", 0, "accepted goal revision this launch binds")
	goalTier := flags.Uint("goal-tier", 0, "claimed-revision goal tier")
	machineID := flags.String("machine-id", "", "claiming machine for the bound goal")
	reviews := flags.String("reviews", "", "reviewed job id for critic, warden, or verifier evidence")
	approvedRef := flags.String("approved-ref", "", "recorded approval reference")
	destructiveReach := flags.String("destructive-reach", "", "admitted hazard class")
	adapterVerb := flags.String("adapter-verb", "", "actual adapter launch verb")
	wait := flags.Bool("wait", false, "wait through the bounded same-operation reservation loop")
	preflight := flags.Bool("preflight", false, "compare retry identity without occupancy or reservation")
	creatorPID := flags.Int64("creator-pid", 0, "long-lived launcher pid recorded for pending-setup liveness")
	occupancyPreparationPath := flags.String("occupancy-preparation", "", "off-lock occupancy preparation hand-off")
	var productRoots []string
	flags.Func("product-root", "declared product root (repeatable)", func(value string) error {
		productRoots = append(productRoots, value)
		return nil
	})
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 || *root == "" || *opid == "" || *session == "" || *dispatchMode == "" ||
		*runtimeName == "" || *model == "" || *role == "" || *launchMode == "" ||
		*permissionDigest == "" || *inputHash == "" || *goalTier > 3 {
		fmt.Fprintln(os.Stderr, "job claim-launch: --root, --opid, --session, --dispatch-mode, --runtime, --model, --role, --launch-mode, --permission-envelope-digest, and --input-hash are required")
		return 2
	}
	claimOperationID := *operationID
	if claimOperationID == "" {
		claimOperationID = *opid
	}
	claimBinding := dispatchcore.DelegateClaimCapabilityBinding{
		JobID: *opid, OperationID: claimOperationID,
		DispatchMode: dispatchcore.DispatchMode(*dispatchMode), AdapterVerb: *adapterVerb,
	}
	if !claimLaunchInternalAuthorized(*root, claimBinding, *preflight) {
		printJSON(dispatchcore.ClaimResult{
			Outcome: dispatchcore.ClaimRefusedInternalSurface,
			Evidence: map[string]any{
				"resolution": "delegate-verb-required",
				"remedy":     "use metasystem delegate",
			},
		})
		return 1
	}
	confPath := *conf
	if confPath == "" {
		confPath = filepath.Join(*root, "metasystem.conf")
	}
	modelKey := config.CanonicalModel(*model)
	resolvedCap, _, _, err := dispatchcore.ResolveCap(confPath, *role, *runtimeName, modelKey, *aliasSource, *capMin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	resumed := *resumedSession
	claimParams := dispatchcore.ClaimLaunchParams{
		Root: *root, OpID: *opid, OperationID: *operationID,
		MainID: *mainID, ClaimEpoch: *claimEpoch, GoalID: *goalID,
		GoalRevision: *goalRevision, GoalTier: uint8(*goalTier), MachineID: *machineID, ApprovedRef: *approvedRef, AdapterVerb: *adapterVerb,
		Reviews: *reviews,
		Request: dispatchcore.LaunchFingerprintRequest{
			SessionKey: *session, DispatchMode: dispatchcore.DispatchMode(*dispatchMode),
			ResumedSessionID: &resumed, Runtime: *runtimeName, Model: *model, Role: *role,
			LaunchMode: dispatchcore.LaunchMode(*launchMode), PermissionEnvelopeDigest: *permissionDigest,
			ProductRoots: productRoots, CapMinutes: resolvedCap, InputHash: *inputHash,
			GoalID: *goalID, GoalRevision: *goalRevision,
			DestructiveReach: dispatchcore.HazardClass(*destructiveReach),
		},
		DefaultCapMinutes: resolvedCap,
		Wait:              *wait,
	}
	if *preflight {
		result, preflightErr := dispatchcore.ClaimLaunchPreflight(claimParams)
		if preflightErr != nil {
			fmt.Fprintln(os.Stderr, preflightErr)
			return 1
		}
		printJSON(result)
		return dispatchcore.ClaimOutcomeExitCode(result.Outcome)
	}
	var occupancyPreparation *dispatchcore.SessionOccupancyPreparation
	if *occupancyPreparationPath != "" {
		prepared, readErr := dispatchcore.ReadClaimOccupancyPreparation(*occupancyPreparationPath, *session)
		if readErr != nil {
			fmt.Fprintln(os.Stderr, readErr)
			return 1
		}
		occupancyPreparation = &prepared
	}
	startReader, err := dispatchStartReader(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	claimParams.OccupancyPreparation = occupancyPreparation
	result, err := dispatchcore.ClaimLaunch(claimParams, dispatchcore.ClaimLaunchDependencies{
		CreatorPID: *creatorPID, IdentityReader: startReader, ProcessVerifier: commandClaimProcessVerifier{},
		Reconcile: func(root, job string) (dispatchcore.ReconciliationResult, error) {
			return dispatchcore.ReconcileReservation(root, job, dispatchcore.ReconciliationDependencies{
				Scanner: commandTaggedProcessScanner{root: root}, Creator: startReader,
				Emit: func(line string) { fmt.Fprintln(os.Stderr, line) },
			})
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printJSON(result)
	return dispatchcore.ClaimOutcomeExitCode(result.Outcome)
}

// claimLaunchInternalAuthorized keeps reservation publication behind the
// delegate verb. The environment marker identifies the internal route but
// carries no authority by itself: a real delegate must also present the
// short-lived bearer word it minted, and the authoritative claim spends that
// word under the job record lock. The complete process-table fixture remains
// the one test seam that may exercise the custody machine directly.
func claimLaunchInternalAuthorized(root string, binding dispatchcore.DelegateClaimCapabilityBinding, preflight bool) bool {
	if os.Getenv("METASYSTEM_DELEGATE_INTERNAL") != "1" {
		return false
	}
	if claimLaunchFakeFixtureAuthorized(root) {
		return true
	}
	raw := os.Getenv(delegateClaimCapabilityEnv)
	if raw == "" {
		return false
	}
	var err error
	if preflight {
		err = dispatchcore.ValidateDelegateClaimCapability(root, raw, binding)
	} else {
		err = dispatchcore.ConsumeDelegateClaimCapability(root, raw, binding)
	}
	return err == nil
}

func claimLaunchFakeFixtureAuthorized(root string) bool {
	processFile := os.Getenv("METASYSTEM_CENSUS_PROCESS_FILE")
	identityFile := os.Getenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE")
	for _, path := range []string{processFile, identityFile} {
		if path == "" {
			return false
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	configuration, err := os.ReadFile(filepath.Join(root, "metasystem.conf"))
	if err != nil {
		return false
	}
	return strings.Contains("\n"+string(configuration), "\nmetasystem.runtimes=fake\n")
}

func runDispatchClaimOccupancyPrepare(args []string) int {
	if refuseRepeatedFlags("job claim-occupancy-prepare", args) {
		return 2
	}
	flags := flag.NewFlagSet("job claim-occupancy-prepare", flag.ContinueOnError)
	root := flags.String("root", "", "Git checkout root")
	session := flags.String("session", "", "namespaced session key")
	output := flags.String("output", "", "transient occupancy preparation output")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 || *root == "" || *session == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "job claim-occupancy-prepare: --root, --session, and --output are required")
		return 2
	}
	return recordExit(dispatchcore.WriteClaimOccupancyPreparation(*root, *session, *output))
}

func runDispatchLaunchCapabilityConsume(args []string) int {
	flags := flag.NewFlagSet("job launch-capability-consume", flag.ContinueOnError)
	root := flags.String("root", "", "Git checkout root")
	job := flags.String("job", "", "admitted job id")
	capability := flags.String("capability", "", "opaque one-shot launch capability")
	adapterVerb := flags.String("adapter-verb", "", "dispatch or follow-up")
	instanceTag := flags.String("instance-tag", "", "admitted instance tag")
	supervisorPID := flags.Int64("supervisor-pid", 0, "adapter supervisor pid")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 || *root == "" || *job == "" || *capability == "" || *adapterVerb == "" || *instanceTag == "" || *supervisorPID < 1 {
		fmt.Fprintln(os.Stderr, "job launch-capability-consume requires --root, --job, --capability, --adapter-verb, --instance-tag, and positive --supervisor-pid")
		return 2
	}
	reader, err := dispatchStartReader(*root)
	if err != nil {
		return recordExit(err)
	}
	return recordExit(dispatchcore.ConsumeLaunchCapability(*root, *job, *capability, *adapterVerb, *instanceTag, *supervisorPID, reader))
}

type commandClaimProcessVerifier struct{}

func (commandClaimProcessVerifier) Verify(pid int64, instanceTag string) identity.Verification {
	return identity.VerifyProcess(identity.KernelProber{}, pid, func(argv []string) bool {
		_, matches := janitor.MatchShape(janitor.DefaultShapes(), argv, instanceTag)
		return matches
	})
}

type commandTaggedProcessScanner struct {
	root string
}

func (s commandTaggedProcessScanner) ScanTag(tag string, reservationCreatedAt time.Time) census.TaggedProcessCensus {
	dependencies := census.TaggedScanDependencies{
		MatchesTag: positionedJobTag, ReservationCreatedAt: reservationCreatedAt,
	}
	processes, configured, err := census.ConfiguredProcessFixture(s.root)
	if err != nil {
		return census.TaggedProcessCensus{EnumerationError: err.Error()}
	}
	if configured {
		reader, readerErr := dispatchStartReader(s.root)
		if readerErr != nil {
			return census.TaggedProcessCensus{EnumerationError: readerErr.Error()}
		}
		pids := make([]int64, 0, len(processes))
		byPID := make(map[int64]census.Process, len(processes))
		argv := make(map[int64][]string, len(processes))
		argvKnown := make(map[int64]bool, len(processes))
		for _, process := range processes {
			if !process.Alive {
				continue
			}
			pids = append(pids, process.Pid)
			byPID[process.Pid] = process
			// Fixture process rows store a flat command string. Only simple
			// whitespace-separated commands can recover exact token boundaries;
			// quoted or escaped commands remain unreadable and cannot prove a tag.
			if !strings.ContainsAny(process.Argv, "'\"\\") {
				argv[process.Pid] = strings.Fields(process.Argv)
				argvKnown[process.Pid] = true
			}
		}
		dependencies.PIDs = func() ([]int64, error) { return pids, nil }
		dependencies.Signal = func(pid int64) error {
			if _, exists := byPID[pid]; !exists {
				return unix.ESRCH
			}
			return nil
		}
		dependencies.PGID = func(pid int64) (int64, error) {
			process, exists := byPID[pid]
			if !exists {
				return 0, unix.ESRCH
			}
			return process.PGID, nil
		}
		dependencies.Reader = commandConfiguredProcessReader{starts: reader, argv: argv, argvKnown: argvKnown}
	}
	return census.ScanTaggedProcesses(tag, dependencies)
}

type commandConfiguredProcessReader struct {
	starts    identity.StartReader
	argv      map[int64][]string
	argvKnown map[int64]bool
}

func (r commandConfiguredProcessReader) ReadStart(pid int64) (identity.Exact, identity.Liveness, error) {
	return r.starts.ReadStart(pid)
}

func (r commandConfiguredProcessReader) ReadArgv(pid int64) ([]string, bool) {
	return r.argv[pid], r.argvKnown[pid]
}

func positionedJobTag(argv []string, tag string) bool {
	_, matches := janitor.MatchShape(janitor.DefaultShapes(), argv, tag)
	return matches
}

func runDispatchPreforkMark(args []string) int {
	if refuseRepeatedFlags("job prefork-mark", args) {
		return 2
	}
	flags := flag.NewFlagSet("job prefork-mark", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	job := flags.String("job", "", "job id")
	tag := flags.String("tag", "", "reservation instance tag")
	supervisor := flags.Int64("supervisor-pid", 0, "supervisor process id")
	pgid := flags.Int64("intended-pgid", 0, "process group the child will join")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *job == "" || *tag == "" || *supervisor < 1 || *pgid < 2 {
		fmt.Fprintln(os.Stderr, "job prefork-mark: --root, --job, --tag, --supervisor-pid, and --intended-pgid are required")
		return 2
	}
	reader, err := dispatchStartReader(*root)
	if err != nil {
		return recordExit(err)
	}
	return recordExit(dispatchcore.WritePreforkMarker(*root, *job, *tag, *supervisor, *pgid, reader))
}

func runDispatchCustodyGroups(args []string) int {
	if refuseRepeatedFlags("job custody-groups", args) {
		return 2
	}
	flags := flag.NewFlagSet("job custody-groups", flag.ContinueOnError)
	recordPath := flags.String("record", "", "job record")
	if flags.Parse(args) != nil {
		return 2
	}
	if *recordPath == "" {
		fmt.Fprintln(os.Stderr, "job custody-groups: --record is required")
		return 2
	}
	record, err := dispatchcore.ReadRecordObject(*recordPath)
	if err != nil {
		return recordExit(err)
	}
	groups, err := dispatchcore.CustodyGroupTargets(record, func(pid int64) (int64, error) {
		group, groupErr := unix.Getpgid(int(pid))
		return int64(group), groupErr
	})
	if err != nil {
		return recordExit(err)
	}
	for _, group := range groups {
		fmt.Println(group)
	}
	return 0
}

func runDispatchReconcileReservation(args []string) int {
	if refuseRepeatedFlags("job reconcile-reservation", args) {
		return 2
	}
	flags := flag.NewFlagSet("job reconcile-reservation", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	job := flags.String("job", "", "job id")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *job == "" {
		fmt.Fprintln(os.Stderr, "job reconcile-reservation: --root and --job are required")
		return 2
	}
	reader, err := dispatchStartReader(*root)
	if err != nil {
		return recordExit(err)
	}
	result, err := dispatchcore.ReconcileReservation(*root, *job, dispatchcore.ReconciliationDependencies{
		Scanner: commandTaggedProcessScanner{root: *root}, Creator: reader,
		Emit: func(line string) { fmt.Fprintln(os.Stderr, line) },
	})
	if err != nil {
		return recordExit(err)
	}
	printJSON(result)
	return 0
}

func runDispatchOwnershipPatch(args []string) int {
	if refuseRepeatedFlags("job ownership-patch", args) {
		return 2
	}
	flags := flag.NewFlagSet("job ownership-patch", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	output := flags.String("output", "", "ownership patch output file")
	pid := flags.Int64("pid", 0, "supervisor process id")
	pgid := flags.Int64("pgid", 0, "supervisor process group id")
	tag := flags.String("instance-tag", "", "recorded instance tag")
	provenAt := flags.String("proven-at", "", "UTC proof timestamp")
	handshakeDeadline := flags.Int64("handshake-deadline", 0, "optional handshake deadline epoch second")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *output == "" || *pid < 1 || *pgid < 2 || *tag == "" || *provenAt == "" {
		fmt.Fprintln(os.Stderr, "job ownership-patch: --root, --output, --pid, --pgid, --instance-tag, and --proven-at are required")
		return 2
	}
	reader, err := dispatchStartReader(*root)
	if err != nil {
		return recordExit(err)
	}
	return recordExit(dispatchcore.BuildOwnershipPatch(
		*output, *pid, *pgid, *tag, *provenAt, *handshakeDeadline, reader,
	))
}

func dispatchStartReader(root string) (identity.StartReader, error) {
	authorization, err := fixtureauth.New(root)
	if err != nil {
		return nil, err
	}
	return identity.FixtureStartReader{
		Kernel: identity.KernelProber{}, Fixture: authorization.Identity(),
	}, nil
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
	root := flags.String("root", "", "checkout root used to prove an approval")
	output := flags.String("output", "", "pending-setup record output file")
	job := flags.String("job", "", "job id")
	role := flags.String("role", "", "job role")
	parent := flags.String("parent", "", "parent job id for a follow-up reservation")
	mainID := flags.String("main-id", "", "dispatching main id")
	claimEpoch := flags.String("claim-epoch", "", "worktree-lease claim epoch")
	goalID := flags.String("goal", "", "goal id this job serves")
	goalRevision := flags.Uint64("goal-revision", 0, "accepted goal revision this reservation serves")
	goalTier := flags.Uint("goal-tier", 0, "claimed-revision goal tier")
	machineID := flags.String("machine-id", "", "claim machine for a goal-bound reservation")
	approvedRef := flags.String("approved-ref", "", "recorded human approval for an oversized slice")
	capResolution := flags.String("cap-resolution", "", "final cap-resolution file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *output == "" || *job == "" || *role == "" || *capResolution == "" || *goalTier > 3 {
		fmt.Fprintln(os.Stderr, "job build-setup: --output, --job, --role, and --cap-resolution are required")
		return 2
	}
	return recordExit(dispatchcore.BuildSetup(*root, *output, *job, *role, *parent, *mainID, *claimEpoch, *goalID, *goalRevision, uint8(*goalTier), *capResolution, *machineID, *approvedRef))
}

func runDispatchSliceAdmission(args []string) int {
	flags := flag.NewFlagSet("job slice-admission", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	capMinutes := flags.Uint64("cap-min", 0, "final reservation cap in minutes")
	approvedRef := flags.String("approved-ref", "", "recorded human approval reference")
	goalID := flags.String("goal", "", "goal id this approval covers")
	goalRevision := flags.Uint64("goal-revision", 0, "accepted goal revision this approval covers")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *capMinutes == 0 {
		fmt.Fprintln(os.Stderr, "job slice-admission: --root and positive --cap-min are required")
		return 2
	}
	verdict, err := dispatchcore.EvaluateSliceAdmission(*root, *capMinutes, *approvedRef, *goalID, *goalRevision)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if verdict.Refused() {
		printJSON(map[string]any{"outcome": verdict.Reason, "headline": "refused", "detail": verdict.Refusal})
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

func runDispatchResolveModelAlias(args []string) int {
	flags := flag.NewFlagSet("job resolve-model-alias", flag.ContinueOnError)
	conf := flags.String("conf", "", "path to metasystem.conf")
	runtimeName := flags.String("runtime", "", "resolved runtime")
	model := flags.String("model", "", "model identifier")
	if flags.Parse(args) != nil {
		return 2
	}
	if *conf == "" || *runtimeName == "" || *model == "" {
		fmt.Fprintln(os.Stderr, "job resolve-model-alias: --conf, --runtime, and --model are required")
		return 2
	}
	canonical, aliased, err := config.ResolveModelAlias(*conf, *runtimeName, *model)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var source any
	if aliased {
		source = *model
	}
	printJSON(map[string]any{"model": canonical, "aliasedFrom": source})
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
	flags.StringVar(&p.AliasedFrom, "aliased-from", "", "effective model alias source (optional)")
	flags.StringVar(&p.RosterAliasedFrom, "roster-aliased-from", "", "roster model alias source (optional)")
	flags.StringVar(&p.ReasoningEffort, "reasoning-effort", "", "requested reasoning effort (optional)")
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
	goalTier := flags.Uint("goal-tier", 0, "claimed-revision goal tier")
	flags.StringVar(&p.MachineID, "machine-id", "", "claim machine for a goal-bound reservation")
	flags.StringVar(&p.ApprovedRef, "approved-ref", "", "recorded human approval for an oversized slice")
	flags.Var((*hazardClassFlag)(&p.DestructiveReach), "destructive-reach", "MECHANICAL, DESIGN-BEARING, or DESTRUCTIVE-REACH")
	flags.StringVar(&p.MainID, "main-id", "", "dispatching main id")
	flags.StringVar(&p.ClaimEpoch, "claim-epoch", "", "worktree-lease claim epoch")
	flags.StringVar(&p.OutputStream, "output-stream", "", "child stdout event stream path")
	flags.StringVar(&p.Composition, "composition", "", "closed-packet composition record")
	launchMode := flags.String("launch-mode", "", "worktree or shared-checkout")
	flags.Func("product-root", "canonical declared product root (repeatable)", func(value string) error {
		p.ProductRoots = append(p.ProductRoots, value)
		return nil
	})
	if flags.Parse(args) != nil {
		return 2
	}
	if p.Output == "" || p.Job == "" || p.Role == "" || p.Runtime == "" || p.Workspace == "" ||
		p.CapResolution == "" || p.Permissions == "" || p.Fallbacks == "" || p.OutputStream == "" || p.Composition == "" || *launchMode == "" || *goalTier > 3 {
		fmt.Fprintln(os.Stderr, "job build-record: --output, --job, --role, --runtime, --workspace, --cap-resolution, --permissions, --fallbacks, --composition, --launch-mode, and --output-stream are required")
		return 2
	}
	p.Overridden = *overridden
	p.Signal = *signal
	p.GoalTier = uint8(*goalTier)
	p.LaunchMode = dispatchcore.LaunchMode(*launchMode)
	return recordExit(dispatchcore.BuildRecord(p))
}

func runDispatchBuildFollowRecord(args []string) int {
	flags := flag.NewFlagSet("job build-follow-record", flag.ContinueOnError)
	var p dispatchcore.BuildFollowRecordParams
	flags.StringVar(&p.Output, "output", "", "pending record output file")
	flags.StringVar(&p.Parent, "parent", "", "parent (latest) record file")
	flags.StringVar(&p.Job, "job", "", "follow-up job id")
	flags.StringVar(&p.OperationID, "operation-id", "", "v2 reservation operation identity")
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
	flags.StringVar(&p.Model, "model", "", "canonical requested model")
	flags.StringVar(&p.AliasedFrom, "aliased-from", "", "model alias source (optional)")
	flags.StringVar(&p.Root, "root", "", "dispatching checkout root (required for mission chains)")
	flags.Uint64Var(&p.GoalRevision, "goal-revision", 0, "accepted goal revision this reservation serves")
	goalTier := flags.Uint("goal-tier", 0, "claimed-revision goal tier")
	flags.StringVar(&p.ApprovedRef, "approved-ref", "", "recorded human approval for an oversized slice")
	flags.Var((*hazardClassFlag)(&p.DestructiveReach), "destructive-reach", "inherited hazard class")
	flags.StringVar(&p.OutputStream, "output-stream", "", "child stdout event stream path")
	flags.StringVar(&p.Composition, "composition", "", "closed-packet composition record")
	launchMode := flags.String("launch-mode", "", "worktree or shared-checkout")
	if flags.Parse(args) != nil {
		return 2
	}
	if p.Output == "" || p.Parent == "" || p.Job == "" || p.OperationID == "" || p.Round < 2 || p.ParentJob == "" ||
		p.Fallbacks == "" || p.ResumeMode == "" || p.CapResolution == "" || p.Model == "" || p.OutputStream == "" || p.Composition == "" || *launchMode == "" || *goalTier > 3 {
		fmt.Fprintln(os.Stderr, "job build-follow-record: --output, --parent, --job, --operation-id, --round (>=2), --parent-job, --fallbacks, --resume-mode, --cap-resolution, --model, --composition, --launch-mode, and --output-stream are required")
		return 2
	}
	p.Signal = *signal
	p.GoalTier = uint8(*goalTier)
	p.LaunchMode = dispatchcore.LaunchMode(*launchMode)
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
	revision, _, err := dispatchcore.ResolveGoalRevision(*root, *goalID)
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
		"goalTier":  binding.Tier,
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
	destructiveReach := flags.String("destructive-reach", "", "MECHANICAL, DESIGN-BEARING, or DESTRUCTIVE-REACH")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *goalID == "" || *revision == 0 || *destructiveReach == "" {
		fmt.Fprintln(os.Stderr, "job goal-revision-admission: --root, --goal, --revision, and --destructive-reach are required")
		return 2
	}
	now, err := goalCommandNow(*root)
	if err != nil {
		return recordExit(err)
	}
	verdict, err := dispatchcore.EvaluateGoalRevisionAdmission(*root, *goalID, *revision, *proposedCap, now, dispatchcore.HazardClass(*destructiveReach))
	if err != nil {
		return recordExit(err)
	}
	if !verdict.Refused() {
		return 0
	}
	if verdict.PolicyRefusal != "" {
		fmt.Fprintln(os.Stderr, verdict.PolicyRefusal)
		return 9
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
	// The caller's recorded start second remains a binding cross-check so a
	// recycled pid cannot be registered with a stale claim.
	reader, readerErr := dispatchStartReader(*root)
	if readerErr != nil {
		return recordExit(readerErr)
	}
	exact, state, probeErr := reader.ReadStart(*pid)
	if probeErr != nil || state != identity.Alive {
		fmt.Fprintf(os.Stderr, "job custody-add: pid %d identity unreadable or not alive\n", *pid)
		return 1
	}
	if exact.StartedAt.Unix() != *pidStarted {
		fmt.Fprintf(os.Stderr, "job custody-add: pid %d start %d does not match the recorded %d\n", *pid, exact.StartedAt.Unix(), *pidStarted)
		return 1
	}
	return recordExit(dispatchcore.CustodyAdd(*root, *job, *pid, reader))
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

func runDispatchReviewReferenceReconcile(args []string) int {
	if refuseRepeatedFlags("job review-reference-reconcile", args) {
		return 2
	}
	flags := flag.NewFlagSet("job review-reference-reconcile", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	rootJob := flags.String("root-job", "", "reviewed chain root job id")
	evidenceJob := flags.String("evidence-job", "", "completed critic, warden, or verifier job id")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 || *repo == "" || *rootJob == "" || *evidenceJob == "" {
		fmt.Fprintln(os.Stderr, "job review-reference-reconcile: --repo, --root-job, and --evidence-job are required")
		return 2
	}
	return recordExit(dispatchcore.ReconcileReviewReference(*repo, *rootJob, *evidenceJob))
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
	repo := flags.String("repo", "", "checkout root")
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

func runDispatchCritiqueOpenFindingIDs(args []string) int {
	flags := flag.NewFlagSet("job critique-open-finding-ids", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	rootJob := flags.String("root-job", "", "critic chain root job id")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" || *rootJob == "" {
		fmt.Fprintln(os.Stderr, "job critique-open-finding-ids: --repo and --root-job are required")
		return 2
	}
	ids, err := dispatchcore.CritiqueOpenFindingIDs(*repo, *rootJob)
	if err != nil {
		return recordExit(err)
	}
	for _, id := range ids {
		fmt.Println(id)
	}
	return 0
}

func runDispatchCritiqueExhaustionAdvance(args []string) int {
	if refuseRepeatedFlags("job critique-exhaustion-advance", args) {
		return 2
	}
	flags := flag.NewFlagSet("job critique-exhaustion-advance", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	rootJob := flags.String("root-job", "", "chain root job id")
	role := flags.String("role", "", "follow-up role")
	message := flags.String("message", "", "successor message file")
	successor := flags.String("successor", "", "successor job id")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" || *rootJob == "" || *role == "" || *message == "" || *successor == "" {
		fmt.Fprintln(os.Stderr, "job critique-exhaustion-advance: --repo, --root-job, --role, --message, and --successor are required")
		return 2
	}
	action, err := dispatchcore.CritiqueExhaustionAdvance(*repo, *rootJob, *role, *message, *successor)
	if err != nil {
		return recordExit(err)
	}
	fmt.Println(action)
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
	aliasSource := flags.String("alias-source", "", "canonical model alias source (optional)")
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
		if err := dispatchcore.RefuseUnsignedMissionCap(*conf, *role, *runtime, *model, *aliasSource); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	if *output == "" {
		fmt.Fprintln(os.Stderr, "job resolve-cap: --output is required without --mission")
		return 2
	}
	capMin, rule, origin, err := dispatchcore.ResolveCap(*conf, *role, *runtime, *model, *aliasSource, *requested)
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
	baseTree := flags.String("base-tree", "", "Git checkout whose HEAD is the delegate base tree")
	diskRoot := flags.String("disk-root", "", "live checkout root for runtime artifact paths")
	authorityOnly := flags.Bool("authority-only", false, "check cited authority without requiring a Working Mode header")
	if flags.Parse(args) != nil {
		return 2
	}
	if *brief == "" {
		fmt.Fprintln(os.Stderr, "job brief-mode: --brief is required")
		return 2
	}
	if (*baseTree == "") != (*diskRoot == "") {
		fmt.Fprintln(os.Stderr, "job brief-mode: --base-tree and --disk-root must be provided together")
		return 2
	}
	if *authorityOnly && *baseTree == "" {
		fmt.Fprintln(os.Stderr, "job brief-mode: --authority-only requires --base-tree and --disk-root")
		return 2
	}
	if *baseTree != "" {
		if err := dispatchcore.ValidateBriefAuthority(*brief, *baseTree, *diskRoot); err != nil {
			return recordExit(err)
		}
	}
	if *authorityOnly {
		return 0
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
