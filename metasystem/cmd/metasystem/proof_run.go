package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	runpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

var legacyProofFenceRead = stopfence.Read

func runProofRunLaunch(args []string) int {
	flags := flag.NewFlagSet("proof-run launch", flag.ContinueOnError)
	suite := flags.String("suite", "", "suite name")
	root := pathFlag(flags, "root", "", "metasystem root")
	controlRootFlag := flags.String("control-root", "", "canonical proof control root")
	goalID := flags.String("goal", "", "accepted goal owning the proof reservation")
	authorityGoalID := flags.String("authority", "", "claimed goal authorizing the proof reservation")
	capMin := flags.String("cap-min", "", "reserved proof minutes")
	retryDecision := flags.String("retry-decision", "", "accountable version-1 retry decision")
	resultPath := flags.String("result", "", "atomic structured launch result path")
	scopeClass := flags.String("scope", "full", "proof scope class")
	commandClass := flags.String("command-class", "", "proof command class")
	conf := flags.String("conf", "", "metasystem configuration")
	progress := flags.String("progress", "", "append-only progress JSONL")
	logPath := flags.String("log", "", "suite output log")
	banner := flags.String("banner", "", "one-line cost banner")
	selector := flags.String("selector", "", "validation section selector")
	selected := flags.String("selected", "", "single selected section")
	enumerated := flags.Bool("enumerated", false, "expect every section listed for this enumeration run")
	var tmpPaths repeatedFlag
	flags.Var(&tmpPaths, "tmp", "temporary evidence path (repeatable)")
	var identityInputs repeatedFlag
	flags.Var(&identityInputs, "identity-input", "normalized proof input (repeatable)")
	silenceMS := flags.Int64("silence-ms", 0, "fixture override for output-silence milliseconds")
	sectionCapMS := flags.Int64("section-cap-ms", 0, "fixture override for section-cap milliseconds")
	evidenceTimeoutMS := flags.Int64("evidence-timeout-ms", 0, "fixture override for evidence-copy milliseconds")
	evidenceMaxBytes := flags.Int64("evidence-max-bytes", 0, "fixture override for evidence-copy bytes")
	pollMS := flags.Int64("poll-ms", 1000, "watchdog poll milliseconds")
	termGraceMS := flags.Int64("term-grace-ms", 5000, "TERM grace milliseconds")
	killGraceMS := flags.Int64("kill-grace-ms", 1000, "KILL observation milliseconds")
	if flags.Parse(args) != nil {
		return 2
	}
	command := flags.Args()
	if len(command) == 0 || *root == "" || *conf == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem proof-run launch --suite S --root R --conf F --progress P --log L --banner B [--selector F] -- COMMAND...")
		return 2
	}
	executionRoot, err := canonicalProofRoot(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run launch:", err)
		return 2
	}
	controlRoot := *controlRootFlag
	if controlRoot == "" {
		for _, candidate := range []string{os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT"), os.Getenv("METASYSTEM_PROOF_RUN_ROOT"), os.Getenv("METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT")} {
			if candidate != "" {
				controlRoot = candidate
				break
			}
		}
		if controlRoot == "" {
			controlRoot = executionRoot
		}
	}
	controlRoot, err = canonicalProofRoot(controlRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run launch:", err)
		return 2
	}
	if *commandClass == "" {
		*commandClass = *suite
	}
	limits, err := resolveProofRunLimits(*conf)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run launch:", err)
		return 1
	}
	if *silenceMS > 0 {
		limits.silence = time.Duration(*silenceMS) * time.Millisecond
	}
	if *sectionCapMS > 0 {
		limits.sectionCap = time.Duration(*sectionCapMS) * time.Millisecond
	}
	if *evidenceTimeoutMS > 0 {
		limits.evidenceTimeout = time.Duration(*evidenceTimeoutMS) * time.Millisecond
	}
	if *evidenceMaxBytes > 0 {
		limits.evidenceMax = *evidenceMaxBytes
	}
	expected, repeated, err := selectedSections(*selector, *selected, *enumerated)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run launch:", err)
		return 1
	}
	if *goalID == "" && noProofLocatorEnvironment() && legacyProofLaunchAllowed(controlRoot) {
		// Bind the first open generation before waiting for host capacity. A
		// stop and rearm during that wait cannot authorize this older launch.
		fenceClosed := errors.New("proof stop fence closed")
		var fence stopfence.Record
		var fenceReadErr error
		var fenceStaleErr error
		initialGeneration := int64(-1)
		readFence := legacyProofFenceRead
		checkFence := func() error {
			fence, fenceReadErr = readFence(controlRoot)
			if fenceReadErr != nil {
				return fenceReadErr
			}
			if fence.State == stopfence.StateClosed {
				return fenceClosed
			}
			if initialGeneration < 0 {
				initialGeneration = fence.Generation
			} else if fence.Generation != initialGeneration {
				fenceStaleErr = fmt.Errorf("proof stop fence generation changed during host admission: %d to %d; retry proof launch", initialGeneration, fence.Generation)
				return fenceStaleErr
			}
			return nil
		}
		refuseFence := func() int {
			if fenceReadErr != nil {
				fmt.Fprintln(os.Stderr, "suite launcher: read stop fence:", fenceReadErr)
			} else if fenceStaleErr != nil {
				fmt.Fprintln(os.Stderr, "suite launcher:", fenceStaleErr)
			} else {
				description, descriptionErr := stopfence.ClosedDescription(fence, controlRoot)
				command, commandErr := stopfence.ClosedCommand(fence, controlRoot)
				if descriptionErr != nil || commandErr != nil {
					fmt.Fprintf(os.Stderr, "suite launcher: cannot render stopped refusal: %v %v\n", descriptionErr, commandErr)
				} else {
					fmt.Fprintln(os.Stderr, description)
					fmt.Fprintln(os.Stderr, "at an agent-free terminal, run: "+command)
				}
			}
			_ = proofrun.EncodeResult(os.Stderr, *resultPath, proofrun.LaunchResult{SchemaVersion: 1, Disposition: proofrun.DispositionFailed, ExitStatus: 1})
			return 1
		}
		if err := checkFence(); err != nil {
			return refuseFence()
		}
		lease, release, leaseErr := acquireManagedProofLaunchWithWaitCheck(context.Background(), controlRoot, *conf, checkFence)
		if leaseErr != nil {
			if errors.Is(leaseErr, fenceClosed) || fenceReadErr != nil || fenceStaleErr != nil {
				return refuseFence()
			}
			fmt.Fprintln(os.Stderr, "proof-run launch: admit native proof:", leaseErr)
			refusal := proofrun.LaunchResult{SchemaVersion: 1, Disposition: proofrun.DispositionAdmissionRefused, ExitStatus: proofrun.ExitAdmissionRefused}
			_ = proofrun.EncodeResult(os.Stderr, *resultPath, refusal)
			return proofrun.ExitAdmissionRefused
		}
		defer release()
		status := proofrun.LaunchSuite(proofrun.LaunchOptions{Suite: *suite, Root: executionRoot, ControlRoot: controlRoot,
			ConfPath: *conf, ProgressPath: *progress, LogPath: *logPath, TmpPaths: tmpPaths, Banner: *banner,
			ExpectedSections: expected, TwiceConsulted: repeated, Silence: limits.silence, SectionCap: limits.sectionCap,
			EvidenceTimeout: limits.evidenceTimeout, EvidenceMax: limits.evidenceMax, Poll: time.Duration(*pollMS) * time.Millisecond,
			TermGrace: time.Duration(*termGraceMS) * time.Millisecond, KillGrace: time.Duration(*killGraceMS) * time.Millisecond,
			Command: command, HostResourceFiles: lease.Files(), RequireCustody: true, Output: os.Stdout, ErrorOutput: os.Stderr,
			FenceReader: func(string) (stopfence.Record, error) {
				err := checkFence()
				if errors.Is(err, fenceClosed) {
					return fence, nil
				}
				return fence, err
			},
		})
		result := proofrun.LaunchResult{SchemaVersion: 1, Disposition: proofrun.DispositionExecuted, ExitStatus: status}
		if status != 0 {
			result.Disposition = proofrun.DispositionFailed
		}
		if err := proofrun.EncodeResult(os.Stderr, *resultPath, result); err != nil {
			return 1
		}
		return status
	}
	attempt, decision, joined, err := admitProofLaunch(proofLaunchAdmission{
		ControlRoot: controlRoot, ExecutionRoot: executionRoot, ConfPath: *conf, GoalID: *goalID, AuthorityGoalID: *authorityGoalID,
		CapMin: *capMin, RetryDecision: *retryDecision, ScopeClass: *scopeClass,
		CommandClass: *commandClass, Sections: expected, IdentityInputs: identityInputs,
	})
	if err != nil {
		refusal := proofrun.LaunchResult{SchemaVersion: 1, Disposition: proofrun.DispositionAdmissionRefused, ExitStatus: proofrun.ExitAdmissionRefused}
		_ = proofrun.EncodeResult(os.Stderr, *resultPath, refusal)
		fmt.Fprintln(os.Stderr, "proof-run launch:", err)
		return proofrun.ExitAdmissionRefused
	}
	if decision.Disposition != proofrun.DispositionExecuted {
		if decision.Reason != "" {
			fmt.Fprintln(os.Stderr, decision.Reason)
		}
		if err := proofrun.EncodeResult(os.Stderr, *resultPath, decision); err != nil {
			fmt.Fprintln(os.Stderr, "proof-run launch: publish launch result:", err)
			return 1
		}
		return decision.ExitStatus
	}
	deadline, _ := time.Parse(time.RFC3339Nano, attempt.Deadline)
	resourceContext, cancelResource := context.WithDeadline(context.Background(), deadline)
	defer cancelResource()
	lease, release, leaseErr := acquireManagedProofLaunch(resourceContext, controlRoot, *conf)
	if leaseErr != nil {
		fmt.Fprintln(os.Stderr, "proof-run launch: admit native proof:", leaseErr)
		status := retainIncompleteProofAttempt(controlRoot, attempt.AttemptID, joined, proofrun.ExitAdmissionRefused)
		decision.ExitStatus, decision.Disposition = status, proofrun.DispositionAdmissionRefused
		_ = proofrun.EncodeResult(os.Stderr, *resultPath, decision)
		return status
	}
	defer release()
	var outerTesting *proofrun.TestResult
	launchStatus := proofrun.LaunchSuite(proofrun.LaunchOptions{
		Suite: *suite, Root: executionRoot, ControlRoot: controlRoot, AttemptID: attempt.AttemptID, JoinedAttempt: joined,
		Deadline: deadline, ConfPath: *conf, ProgressPath: *progress, LogPath: *logPath,
		TmpPaths: tmpPaths, Banner: *banner, ExpectedSections: expected, TwiceConsulted: repeated,
		Silence: limits.silence, SectionCap: limits.sectionCap,
		EvidenceTimeout: limits.evidenceTimeout, EvidenceMax: limits.evidenceMax,
		Poll:      time.Duration(*pollMS) * time.Millisecond,
		TermGrace: time.Duration(*termGraceMS) * time.Millisecond,
		KillGrace: time.Duration(*killGraceMS) * time.Millisecond,
		Command:   command, HostResourceFiles: lease.Files(), RequireCustody: true, Output: os.Stdout, ErrorOutput: os.Stderr,
		PrepareSuccess: func(completion proofrun.CompletionContext) (json.RawMessage, error) {
			if joined {
				return nil, nil
			}
			current, readErr := proofrun.ReadAttempt(controlRoot, attempt.AttemptID)
			if readErr != nil || current.TestResult == nil || !current.TestResult.Delivery.Sufficient {
				return nil, readErr
			}
			receipt, payload, prepareErr := landing.PrepareTestingReceiptPayload(controlRoot,
				current.TestResult.CandidateTree, *current.TestResult, completion.CompletedAt)
			if prepareErr == nil && receipt.Testing != nil {
				copyResult := *receipt.Testing
				outerTesting = &copyResult
			}
			return payload, prepareErr
		},
		CommitTerminal: func(completion proofrun.CompletionContext, receipt json.RawMessage) error {
			return commitProofTerminalWithTestResult(completion, receipt, outerTesting)
		},
	})
	launchStatus = retainIncompleteProofAttempt(controlRoot, attempt.AttemptID, joined, launchStatus)
	decision.ExitStatus = launchStatus
	if launchStatus != 0 {
		decision.Disposition = proofrun.DispositionFailed
	}
	if err := proofrun.EncodeResult(os.Stderr, *resultPath, decision); err != nil {
		fmt.Fprintln(os.Stderr, "proof-run launch: publish launch result:", err)
		return 1
	}
	return launchStatus
}

// acquireManagedProofLaunch gives an executing public proof one host phase.
// A retry or reusable decision calls no launcher and takes no phase slot.
func acquireManagedProofLaunch(ctx context.Context, controlRoot, confPath string) (*proofrun.HostResourceLease, func(), error) {
	return acquireManagedProofLaunchWithWaitCheck(ctx, controlRoot, confPath, nil)
}

func acquireManagedProofLaunchWithWaitCheck(ctx context.Context, controlRoot, confPath string, check func() error) (*proofrun.HostResourceLease, func(), error) {
	unmark, err := proofrun.MarkManagedProofProcess()
	if err != nil {
		return nil, nil, fmt.Errorf("mark host proof launcher managed: %w", err)
	}
	ctx = proofrun.WithHostResourceWaitObserver(ctx, func() {
		fmt.Fprintln(os.Stderr, proofCapacityWaitLine)
	})
	lease, err := proofrun.AcquireHostResourcesWithWaitCheck(ctx, controlRoot, confPath, "heavy", nil, check)
	if err != nil {
		unmark()
		return nil, nil, err
	}
	return lease, func() { _ = lease.Close(); unmark() }, nil
}

const proofCapacityWaitLine = "proof-run launch: waiting for host proof capacity"

func runProofRunGoGateTests(args []string) int {
	flags := flag.NewFlagSet("proof-run go-gate-tests", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "metasystem source root")
	logRoot := pathFlag(flags, "log-root", "", "private native partition log directory")
	workers := flags.Int("workers", 0, "inherited positive test-worker allowance")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *root == "" || *logRoot == "" || *workers < 1 {
		fmt.Fprintln(os.Stderr, "usage: metasystem proof-run go-gate-tests --root DIR --log-root DIR --workers N")
		return 2
	}
	if code, err := authorizeProofWorker(*root); err != nil {
		fmt.Fprintln(os.Stderr, "proof-run go-gate-tests:", err)
		return code
	}
	inheritedWorkers, err := strconv.Atoi(os.Getenv(proofrun.TestWorkersEnvironment))
	if err != nil || inheritedWorkers < 1 {
		fmt.Fprintln(os.Stderr, "proof-run go-gate-tests: authenticated parent supplied no positive test-worker allowance")
		return 3
	}
	if *workers > inheritedWorkers {
		fmt.Fprintf(os.Stderr, "proof-run go-gate-tests: requested workers %d exceed inherited allowance %d\n", *workers, inheritedWorkers)
		return 3
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	controlRoot, err := canonicalProofRoot(os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run go-gate-tests:", err)
		return 3
	}
	lease, err := proofrun.AcquireHostResources(ctx, controlRoot, filepath.Join(controlRoot, "metasystem.conf"), "heavy", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run go-gate-tests: inherit admitted host resources:", err)
		return 3
	}
	defer lease.Close()
	ctx = proofrun.WithHostResourceLease(ctx, lease)
	result, status, err := proofrun.RunGoGateTests(ctx, proofrun.GoGateTestRequest{
		Root: *root, LogRoot: *logRoot, Environment: os.Environ(), Workers: *workers,
	})
	if len(result.Output) != 0 {
		_, _ = os.Stdout.Write(result.Output)
	}
	for _, rerun := range result.Reruns {
		fmt.Fprintf(os.Stderr, "go gate diagnostic rerun: %s.%s first=%s second=%s log=%s\n",
			rerun.Package, rerun.Test, rerun.First, rerun.Second, rerun.LogPath)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "proof-run go-gate-tests: %v (native log: %s)\n", err, result.LogPath)
	}
	return status
}

func runProofRunWorkerAuthorized(args []string) int {
	flags := flag.NewFlagSet("proof-run worker-authorized", flag.ContinueOnError)
	executionRoot := pathFlag(flags, "root", "", "suite execution root")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *executionRoot == "" {
		return 2
	}
	code, err := authorizeProofWorker(*executionRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run worker-authorized:", err)
	}
	return code
}

func authorizeProofWorker(executionRoot string) (int, error) {
	canonicalExecution, err := canonicalProofRoot(executionRoot)
	if err != nil {
		return 2, err
	}
	controlRoot := os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT")
	if controlRoot == "" {
		return 3, fmt.Errorf("no proof control root")
	}
	canonicalControl, err := canonicalProofRoot(controlRoot)
	if err != nil {
		return 3, err
	}
	err = proofrun.AuthenticateWorker(canonicalControl, os.Getenv("METASYSTEM_PROOF_ATTEMPT"),
		os.Getenv("METASYSTEM_PROOF_RECORD_KEY"), os.Getenv("METASYSTEM_PROOF_CREATION_CLAIM"), int64(os.Getppid()))
	if err != nil {
		return 3, err
	}
	authorizedRoot := canonicalControl
	if attemptID := os.Getenv("METASYSTEM_PROOF_ATTEMPT"); attemptID != "" {
		attempt, readErr := proofrun.ReadAttempt(canonicalControl, attemptID)
		if readErr != nil {
			return 3, readErr
		}
		authorizedRoot, err = canonicalProofRoot(attempt.ExecutionRoot)
		if err != nil {
			return 3, err
		}
	}
	if canonicalExecution != authorizedRoot && !authorizedWitnessSnapshot(canonicalExecution, authorizedRoot) &&
		!authorizedSectionWorktree(canonicalExecution, authorizedRoot, canonicalControl, os.Getenv("METASYSTEM_PROOF_ATTEMPT")) {
		return 3, fmt.Errorf("supplied root %q does not match admitted execution root %q", canonicalExecution, authorizedRoot)
	}
	return 0, nil
}

// A schema-1 policy worker runs each section in a temporary linked worktree.
// Its child retains the admitted proof locator, but the section's cwd differs
// from the attempt execution root. Admit only that registered candidate tree,
// after AuthenticateWorker has already proved the exact launcher lineage.
func authorizedSectionWorktree(sectionRoot, executionRoot, controlRoot, attemptID string) bool {
	if attemptID == "" {
		return false
	}
	actualCWD, err := canonicalProofRoot(".")
	if err != nil || actualCWD != sectionRoot {
		return false
	}
	attempt, err := proofrun.ReadAttempt(controlRoot, attemptID)
	if err != nil || attempt.ProofIdentity.CommandClass != "testing" {
		return false
	}
	candidateTree, ok := attempt.CandidateTreeDigest()
	if !ok {
		return false
	}
	base := gittree.Workspace{Dir: executionRoot}
	section := gittree.Workspace{Dir: sectionRoot}
	baseTop, err := base.TopLevel()
	if err != nil {
		return false
	}
	sectionTop, err := section.TopLevel()
	if err != nil || sectionTop == baseTop {
		return false
	}
	relative, err := filepath.Rel(baseTop, executionRoot)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	candidateRoot := filepath.Join(sectionTop, relative)
	childPath, err := filepath.Rel(candidateRoot, sectionRoot)
	if err != nil || childPath == ".." || strings.HasPrefix(childPath, ".."+string(filepath.Separator)) {
		return false
	}
	// NewDetachedWorktree uses a paired random directory and linked worktree
	// name. A foreign checkout with the same tree is not this worker's section.
	suffix, named := strings.CutPrefix(filepath.Base(filepath.Dir(sectionTop)), "metasystem-landing-receipt.")
	if !named || suffix == "" || filepath.Base(sectionTop) != "worktree-"+suffix {
		return false
	}
	if !section.RegisteredDetachedWorktreeOf(base) {
		return false
	}
	// A nested native suite retains its exact-root process record. Direct
	// command groups have no nested record and remain inside the old testing
	// worker's recorded suite process; both cases require attempt custody.
	records, err := proofrun.ReadRecords(controlRoot)
	if err != nil {
		return false
	}
	sectionLauncher := false
	sectionRecordAtRoot := false
	outerTestingWorker := false
	for _, record := range records {
		if record.AttemptID != attemptID || record.ControlRoot != controlRoot {
			continue
		}
		recordRoot, canonicalErr := canonicalProofRoot(record.Root)
		if canonicalErr != nil {
			continue
		}
		if recordRoot == sectionRoot {
			// A nested native launcher owns this exact root. A completed or
			// unlisted record must not fall through to the outer worker.
			sectionRecordAtRoot = true
		}
		if record.Status != proofrun.StatusRunning {
			continue
		}
		registered := false
		for _, key := range attempt.ProcessKeys {
			if key == record.Key() {
				registered = true
				break
			}
		}
		if !registered {
			continue
		}
		if proofrun.AuthenticateAncestor(int64(os.Getppid()), record.SuiteProcess) == nil {
			if recordRoot == sectionRoot {
				sectionLauncher = true
			}
			if recordRoot == executionRoot && record.Suite == "testing" {
				outerTestingWorker = true
			}
		}
	}
	// The old testing worker runs command groups directly under its own live
	// suite process, without a second section record. Its exact process
	// ancestry plus the registered clean candidate worktree is the custody
	// boundary for those commands.
	if !sectionLauncher && (!outerTestingWorker || sectionRecordAtRoot) {
		return false
	}
	candidate := gittree.Workspace{Dir: candidateRoot}
	actualTree, err := candidate.HeadTree()
	if err != nil || actualTree != candidateTree {
		return false
	}
	stagedTree, err := candidate.StagedTree()
	if err != nil || stagedTree != candidateTree {
		return false
	}
	status, err := section.Status()
	// Git-ignored generated files do not appear here. Any tracked or
	// nonignored untracked input could change the section's behavior.
	return err == nil && len(status) == 0
}

const proofWitnessExecutionRootEnv = "METASYSTEM_PROOF_EXECUTION_ROOT"

// A witness producer may run the worker from the snapshot it just created.
// Its carried execution root must still name the root retained by the attempt.
func authorizedWitnessSnapshot(snapshotRoot, authorizedRoot string) bool {
	if os.Getenv("METASYSTEM_GATE_WITNESS_WRITE") == "" {
		return false
	}
	executionRoot := os.Getenv(proofWitnessExecutionRootEnv)
	if executionRoot == "" {
		return false
	}
	canonicalExecution, err := canonicalProofRoot(executionRoot)
	if err != nil || canonicalExecution != authorizedRoot {
		return false
	}
	workingRoot, err := canonicalProofRoot(".")
	return err == nil && workingRoot == snapshotRoot
}

func retainIncompleteProofAttempt(root, attemptID string, joined bool, status int) int {
	if joined {
		return status
	}
	attempt, err := proofrun.ReadAttempt(root, attemptID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run launch: read incomplete attempt:", err)
		return 1
	}
	if attempt.Terminal != nil {
		return status
	}
	if status == 0 {
		status = 1
	}
	if _, err := proofrun.FinalizeAttempt(root, attemptID, proofrun.TerminalUnknown, status,
		"proof launcher ended before its ordered terminal commit", nil, time.Now().UTC()); err != nil {
		fmt.Fprintln(os.Stderr, "proof-run launch: retain unknown terminal outcome:", err)
		return 1
	}
	return status
}

func noProofLocatorEnvironment() bool {
	for _, name := range []string{"METASYSTEM_PROOF_CONTROL_ROOT", "METASYSTEM_PROOF_ATTEMPT", "METASYSTEM_PROOF_RUN_ROOT",
		"METASYSTEM_PROOF_RUN_ID", "METASYSTEM_HOOK_DELEGATE_STATE_ROOT", "METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT", "METASYSTEM_HOOK_DELEGATE_JOB"} {
		if os.Getenv(name) != "" {
			return false
		}
	}
	return true
}

func legacyProofLaunchAllowed(root string) bool {
	if fixtureauth.FixtureModeRoot(root) || !goal.NewWorld(root) {
		return true
	}
	classification, err := classifyVerbCaller(root, int64(os.Getppid()))
	return err == nil && classification.Class == lease.ClassHuman
}

type proofLaunchAdmission struct {
	ControlRoot, ExecutionRoot, ConfPath, GoalID, AuthorityGoalID, CapMin, RetryDecision string
	CandidateTree                                                                        string
	ScopeClass, CommandClass                                                             string
	CandidateRevision, ExpectedGoalRevision, ExpectedAccountingRevision                  uint64
	Now                                                                                  time.Time
	Sections                                                                             []string
	IdentityInputs                                                                       []string
	Environment                                                                          []string
	SharedEngine                                                                         string
	SharedManifestDigest                                                                 string
	ComponentIdentities                                                                  map[string]string
	FreshGroups                                                                          map[string]bool
	FreshnessEpisode, FreshnessBinding, FreshnessExpiresAt                               string
	BeforePublish                                                                        func(*proofrun.AdmissionRequest)
	ForceAttempt                                                                         bool
	ForceGroups                                                                          bool
	ManagedCapacity                                                                      bool
	RequireDiagnosticHeadroom                                                            bool
}

func canonicalProofRoot(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

type proofGoalRoles struct {
	Candidate         *goal.GoalFile
	Authority         *goal.GoalFile
	CandidateRevision uint64
}

var (
	proofAdmissionUnderLocks    func()
	proofAdmissionBeforePublish func(*proofrun.AdmissionRequest)
	proofAdmissionAfterPublish  func()
	proofAdmissionLockOrder     func([]string)
)

type proofAdmissionGoalSnapshot struct {
	ID, State, FenceStopID, ClaimMachine, ClaimLineage   string
	Revision, LockRevision, EpisodeRevision, WeightEpoch uint64
	Budget                                               goal.Budget
	Present, HasBudget, HasWeightEpoch                   bool
}

type proofAdmissionSnapshots struct {
	Candidate proofAdmissionGoalSnapshot
	Authority proofAdmissionGoalSnapshot
}

func proofAdmissionGoalState(root string, file *goal.GoalFile, now time.Time) (proofAdmissionGoalSnapshot, error) {
	if file == nil {
		return proofAdmissionGoalSnapshot{}, nil
	}
	snapshot := proofAdmissionGoalSnapshot{ID: file.Id, State: file.State, Revision: file.Revision,
		LockRevision: file.Revision, EpisodeRevision: goal.BudgetEpisodeRevision(file), Present: true}
	if file.Budget != nil {
		snapshot.Budget, snapshot.HasBudget = *file.Budget, true
	}
	if file.StopFence != nil {
		snapshot.FenceStopID = file.StopFence.StopID
	}
	if file.Claimed != nil {
		snapshot.ClaimMachine, snapshot.ClaimLineage = file.Claimed.Machine, file.Claimed.Lineage
		if file.Claimed.Revision != 0 {
			snapshot.LockRevision = file.Claimed.Revision
		}
	}
	projection := dispatchcore.ProjectConsumption(root, file, now)
	if projection.Status != dispatchcore.BudgetKnown {
		return proofAdmissionGoalSnapshot{}, fmt.Errorf("goal %s consumption projection is unknown", file.Id)
	}
	if projection.WeightEpoch != nil {
		snapshot.WeightEpoch, snapshot.HasWeightEpoch = *projection.WeightEpoch, true
	}
	return snapshot, nil
}

func proofAdmissionGoalStates(root, candidateID, authorityID string, now time.Time) (proofAdmissionSnapshots, error) {
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return proofAdmissionSnapshots{}, err
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		return proofAdmissionSnapshots{}, err
	}
	candidate, err := proofAdmissionGoalState(root, projection.Tree.Live[candidateID], now)
	if err != nil {
		return proofAdmissionSnapshots{}, err
	}
	authority := candidate
	if authorityID != candidateID {
		authority, err = proofAdmissionGoalState(root, projection.Tree.Live[authorityID], now)
		if err != nil {
			return proofAdmissionSnapshots{}, err
		}
	}
	return proofAdmissionSnapshots{Candidate: candidate, Authority: authority}, nil
}

func proofAdmissionMoved(before, after proofAdmissionSnapshots, candidateID, authorityID string) error {
	if before == after {
		return nil
	}
	return fmt.Errorf("CANDIDATE_GOAL_MOVED: candidate goal %s revision %d->%d and authority goal %s revision %d->%d changed during proof admission; retry from the accepted goals",
		candidateID, before.Candidate.Revision, after.Candidate.Revision,
		authorityID, before.Authority.Revision, after.Authority.Revision)
}

func acquireProofAdmissionGoalLocks(root string, snapshots proofAdmissionSnapshots) ([]*goalrevision.Held, error) {
	coordinates := []proofAdmissionGoalSnapshot{snapshots.Authority}
	if snapshots.Candidate.ID != snapshots.Authority.ID {
		coordinates = append(coordinates, snapshots.Candidate)
	}
	sort.Slice(coordinates, func(i, j int) bool { return coordinates[i].ID < coordinates[j].ID })
	order := make([]string, len(coordinates))
	for i := range coordinates {
		order[i] = coordinates[i].ID
	}
	if proofAdmissionLockOrder != nil {
		proofAdmissionLockOrder(append([]string(nil), order...))
	}
	held := make([]*goalrevision.Held, 0, len(coordinates))
	for _, coordinate := range coordinates {
		lock, err := goalrevision.Acquire(root, coordinate.ID, coordinate.LockRevision, "proof-admission")
		if err != nil {
			for i := len(held) - 1; i >= 0; i-- {
				_ = held[i].Release()
			}
			return nil, fmt.Errorf("proof admission candidate=%s authority=%s could not acquire %s: %w",
				snapshots.Candidate.ID, snapshots.Authority.ID, coordinate.ID, err)
		}
		held = append(held, lock)
	}
	return held, nil
}

func releaseProofAdmissionGoalLocks(held []*goalrevision.Held) {
	for i := len(held) - 1; i >= 0; i-- {
		_ = held[i].Release()
	}
}

func candidateGoalRefusal(id, state, detail string) error {
	if detail != "" {
		detail = " " + detail
	}
	return fmt.Errorf("CANDIDATE_GOAL_REFUSED: candidate goal %s state=%s%s", id, state, detail)
}

func candidateGoalForProof(tree *goal.TreeGoals, id, machine string) (*goal.GoalFile, uint64, error) {
	if tree == nil {
		return nil, 0, candidateGoalRefusal(id, "absent", "accepted goal projection is empty")
	}
	file := tree.Live[id]
	if file == nil {
		if tree.Done[id] != nil {
			return nil, 0, candidateGoalRefusal(id, goal.StateDone, "")
		}
		return nil, 0, candidateGoalRefusal(id, "absent", "")
	}
	switch file.State {
	case goal.StateQueued, goal.StateDone, goal.StateParked, goal.StateAbandoned:
		return nil, 0, candidateGoalRefusal(id, file.State, "")
	case goal.StateApproved, goal.StateClaimed:
	default:
		return nil, 0, candidateGoalRefusal(id, file.State, "")
	}
	if file.Budget == nil || file.Approved == nil {
		return nil, 0, candidateGoalRefusal(id, "no-budget", "")
	}
	if file.IsFencedClaim() {
		return nil, 0, candidateGoalRefusal(id, "fenced", "stopId="+file.StopFence.StopID)
	}
	if file.State == goal.StateClaimed && (file.Claimed == nil || file.Claimed.Machine != machine) {
		claimedMachine := "unknown"
		if file.Claimed != nil && file.Claimed.Machine != "" {
			claimedMachine = file.Claimed.Machine
		}
		return nil, 0, candidateGoalRefusal(id, "claimed", "machine="+claimedMachine)
	}
	revision := goal.BudgetEpisodeRevision(file)
	if revision == 0 {
		return nil, 0, candidateGoalRefusal(id, "no-budget-episode", "")
	}
	return file, revision, nil
}

func proofAuthorityRefusal(id, detail string) error {
	return fmt.Errorf("PROOF_AUTHORITY_REQUIRED: authority goal %s %s", id, detail)
}

func enforceBoundProofGoals(kind, boundGoal, candidateGoal, authorityGoal string) error {
	if authorityGoal != "" || candidateGoal != "" && candidateGoal != boundGoal {
		return proofAuthorityRefusal(boundGoal, "is fixed by the "+kind+"; --goal and --authority cannot select another goal")
	}
	return nil
}

func resolveProofGoalRoles(root, candidateID, authorityID string, now time.Time) (proofGoalRoles, error) {
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return proofGoalRoles{}, err
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		return proofGoalRoles{}, fmt.Errorf("resolve proof goals: %w", err)
	}
	machine, machineErr := goal.ResolveMachine(root)
	// The pre-enrollment human path already admits a proof bound to the
	// candidate's own claim. Preserve that compatibility without allowing an
	// unclaimed candidate to select some other machine's authority.
	if machineErr != nil {
		file := projection.Tree.Live[candidateID]
		if file == nil || file.State != goal.StateClaimed || file.Claimed == nil ||
			(authorityID != "" && authorityID != candidateID) {
			return proofGoalRoles{}, machineErr
		}
		machine = file.Claimed.Machine
	}
	candidate, candidateRevision, err := candidateGoalForProof(projection.Tree, candidateID, machine)
	if err != nil {
		return proofGoalRoles{}, err
	}
	liveAuthority := func(id string) (*goal.GoalFile, error) {
		file := projection.Tree.Live[id]
		if file == nil || file.State != goal.StateClaimed || file.Claimed == nil {
			return nil, proofAuthorityRefusal(id, "is not a live claim")
		}
		if file.Claimed.Machine != machine {
			return nil, proofAuthorityRefusal(id, "is claimed on machine "+file.Claimed.Machine)
		}
		if file.IsFencedClaim() {
			return nil, proofAuthorityRefusal(id, "is fenced by stop "+file.StopFence.StopID)
		}
		return file, nil
	}
	if candidate.State == goal.StateClaimed {
		if authorityID != "" && authorityID != candidateID {
			return proofGoalRoles{}, proofAuthorityRefusal(authorityID, "cannot replace the candidate's own live claim "+candidateID)
		}
		return proofGoalRoles{Candidate: candidate, Authority: candidate, CandidateRevision: candidateRevision}, nil
	}
	var authority *goal.GoalFile
	if authorityID != "" {
		authority, err = liveAuthority(authorityID)
		if err != nil {
			return proofGoalRoles{}, err
		}
	} else {
		for _, id := range goal.OrderedOpenGoalIDs(projection.Tree.Live) {
			file := projection.Tree.Live[id]
			if file.State != goal.StateClaimed || file.Claimed == nil || file.Claimed.Machine != machine || file.IsFencedClaim() {
				continue
			}
			if authority != nil {
				return proofGoalRoles{}, proofAuthorityRefusal("", "is ambiguous; pass --authority <goal-id>")
			}
			authority = file
		}
		if authority == nil {
			return proofGoalRoles{}, proofAuthorityRefusal("", "has no live unfenced claim on machine "+machine)
		}
	}
	if candidate.Arc != "" && candidate.Arc == authority.Arc {
		return proofGoalRoles{}, fmt.Errorf("PROOF_AUTHORITY_ARC_MATE_REFUSED: candidate goal %s and authority goal %s share arc %s; claim %s as its own authority",
			candidate.Id, authority.Id, candidate.Arc, candidate.Id)
	}
	return proofGoalRoles{Candidate: candidate, Authority: authority, CandidateRevision: candidateRevision}, nil
}

func admitProofLaunch(request proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
	classifiedCaller, err := classifyVerbCaller(request.ControlRoot, int64(os.Getppid()))
	if err != nil {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof caller classification failed: %w", err)
	}
	now := request.Now
	if now.IsZero() {
		now, err = goalCommandNow(request.ControlRoot)
		if err != nil {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
		}
	}
	parentRoot, parentAttempt := os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT"), os.Getenv("METASYSTEM_PROOF_ATTEMPT")
	if parentRoot != "" || parentAttempt != "" {
		if parentRoot == "" || parentAttempt == "" {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof parent locator is incomplete")
		}
		canonicalParent, err := canonicalProofRoot(parentRoot)
		if err != nil || canonicalParent != request.ControlRoot {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof parent control root does not match the requested root")
		}
		attempt, err := proofrun.AuthenticateContext(request.ControlRoot, parentAttempt, int64(os.Getppid()))
		if err != nil {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
		}
		if request.GoalID != "" && request.GoalID != attempt.AccountedGoal() {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof parent candidate goal %s does not match requested goal %s", attempt.AccountedGoal(), request.GoalID)
		}
		if request.ExpectedGoalRevision != 0 && (attempt.GoalRevision != request.ExpectedGoalRevision ||
			attempt.AccountingRevision != request.ExpectedAccountingRevision) {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("GOAL_REVISION_MOVED: expected goal/accounting revisions %d/%d, found %d/%d",
				request.ExpectedGoalRevision, request.ExpectedAccountingRevision, attempt.GoalRevision, attempt.AccountingRevision)
		}
		if len(request.ComponentIdentities) > 0 {
			heldGoal, lockErr := goalrevision.Acquire(request.ControlRoot, attempt.GoalID, attempt.GoalRevision, "joined-testing-admission")
			if lockErr != nil {
				return proofrun.Attempt{}, proofrun.LaunchResult{}, false, lockErr
			}
			defer heldGoal.Release()
			heldProof, lockErr := proofrun.AcquireMutation(request.ControlRoot)
			if lockErr != nil {
				return proofrun.Attempt{}, proofrun.LaunchResult{}, false, lockErr
			}
			defer heldProof.Release()
			attempt, lockErr = proofrun.AuthenticateContext(request.ControlRoot, parentAttempt, int64(os.Getppid()))
			if lockErr != nil {
				return proofrun.Attempt{}, proofrun.LaunchResult{}, false, lockErr
			}
			candidateTree, _ := attempt.CandidateTreeDigest()
			componentRequest := proofrun.AdmissionRequest{ControlRoot: request.ControlRoot, GoalID: attempt.GoalID,
				GoalRevision: attempt.GoalRevision, AccountingRevision: attempt.AccountingRevision,
				CandidateGoalID: attempt.AccountedGoal(), CandidateRevision: attempt.AccountedRevision(),
				CandidateBudgetEpoch: attempt.AccountedBudgetEpoch(), CandidateTree: candidateTree,
				RetryDecisionPath: request.RetryDecision, ComponentIdentities: request.ComponentIdentities, ForceAttempt: request.ForceAttempt,
				SharedComponents: true, ForceGroups: request.ForceGroups, FreshnessEpisode: request.FreshnessEpisode,
				FreshnessBinding: request.FreshnessBinding, FreshnessExpiresAt: request.FreshnessExpiresAt, Now: now}
			componentRequest.FreshGroups = request.FreshGroups
			decision, decided, decisionErr := proofrun.JoinedComponentDecisionLocked(componentRequest)
			if decisionErr != nil {
				return proofrun.Attempt{}, proofrun.LaunchResult{}, false, decisionErr
			}
			if decided {
				return proofrun.Attempt{}, decision, false, nil
			}
			attempt, lockErr = proofrun.BindJoinedTestOwnershipLocked(request.ControlRoot, attempt.AttemptID, componentRequest)
			if lockErr != nil {
				return proofrun.Attempt{}, proofrun.LaunchResult{}, false, lockErr
			}
		}
		return attempt, proofrun.LaunchResult{SchemaVersion: 1, Disposition: proofrun.DispositionExecuted, AttemptID: attempt.AttemptID}, true, nil
	}
	var reservationOwner *proofrun.ReservationOwner
	delegateRevision := uint64(0)
	boundAuthority := false
	delegateState, delegateInstallation, delegateJob := "", "", ""
	governedRoot, governedID := os.Getenv("METASYSTEM_PROOF_RUN_ROOT"), os.Getenv("METASYSTEM_PROOF_RUN_ID")
	if governedRoot != "" || governedID != "" {
		if governedRoot == "" || governedID == "" {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("governed proof locator is incomplete")
		}
		canonicalGoverned, err := canonicalProofRoot(governedRoot)
		if err != nil || canonicalGoverned != request.ControlRoot {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("governed proof control root does not match the requested root")
		}
		record, err := (&runpkg.Store{Root: request.ControlRoot}).Read(governedID)
		if err != nil || record == nil || record.Governed == nil || record.Status != runpkg.StatusRunning ||
			record.Pid == nil || record.PidStartedAt == nil {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("governed proof locator does not name a live bound run")
		}
		ancestor := proofrun.ProcessIdentity{Pid: *record.Pid, PidStartedAt: *record.PidStartedAt,
			PidStartTicks: record.PidStartTicks, BootID: record.BootID}
		if err := proofrun.AuthenticateAncestor(int64(os.Getppid()), ancestor); err != nil {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("governed proof custody: %w", err)
		}
		if err := enforceBoundProofGoals("governed run", record.GoalId, request.GoalID, request.AuthorityGoalID); err != nil {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
		}
		request.GoalID, request.AuthorityGoalID, boundAuthority = record.GoalId, record.GoalId, true
		started, err := time.Parse(time.RFC3339, record.StartedAt)
		if err != nil || record.Governed.ExecutionCostMinutes == 0 {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("governed run has no valid proof deadline")
		}
		deadline := started.Add(time.Duration(record.Governed.ExecutionCostMinutes) * time.Minute).UTC().Format(time.RFC3339Nano)
		reservationOwner = &proofrun.ReservationOwner{ControlRoot: request.ControlRoot, RunID: record.RunId,
			RunGeneration: record.Generation, LaunchNonce: record.LaunchNonce, GoalRevision: record.Governed.GoalRevision,
			ObligationRevision: record.Governed.ObligationRevision, AttemptOrdinal: record.Governed.AttemptOrdinal,
			BudgetEpoch: record.Governed.BudgetEpoch, Deadline: deadline}
	}
	delegateState = os.Getenv("METASYSTEM_HOOK_DELEGATE_STATE_ROOT")
	delegateInstallation = os.Getenv("METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT")
	delegateJob = os.Getenv("METASYSTEM_HOOK_DELEGATE_JOB")
	if delegateState != "" || delegateInstallation != "" || delegateJob != "" {
		if delegateState == "" || delegateInstallation == "" || delegateJob == "" {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("native delegate proof locator is incomplete")
		}
		verified, err := lease.HookDelegate(delegateState, delegateInstallation, delegateJob, int64(os.Getppid()))
		if err != nil || !verified.Delegate || verified.JobID != delegateJob {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("native delegate proof locator is not authenticated")
		}
		record, err := dispatchcore.ReadRecordObject(filepath.Join(delegateState, "artifacts", "agents", "jobs", delegateJob+".json"))
		if err != nil {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
		}
		lens := dispatchcore.JobRecordOf(record)
		resolvedGoal := lens.GoalID()
		resolvedRevision, revisionOK := lens.GoalRevision()
		if resolvedGoal == "" || !revisionOK {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("native delegate reservation has no accepted goal binding")
		}
		if err := enforceBoundProofGoals("native delegate", resolvedGoal, request.GoalID, request.AuthorityGoalID); err != nil {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
		}
		request.GoalID, request.AuthorityGoalID, delegateRevision, boundAuthority = resolvedGoal, resolvedGoal, resolvedRevision, true
	}
	if request.GoalID == "" && classifiedCaller.Class == lease.ClassMain && classifiedCaller.Holder {
		request.GoalID, err = uniqueActiveProofGoal(request.ControlRoot, now)
		if err != nil {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
		}
	}
	if request.GoalID == "" {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("top-level proof launch requires --goal")
	}
	roles, err := resolveProofGoalRoles(request.ControlRoot, request.GoalID, request.AuthorityGoalID, now)
	if err != nil {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
	}
	if boundAuthority && roles.Authority.Id != request.GoalID {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, proofAuthorityRefusal(request.AuthorityGoalID, "does not match the bound proof context")
	}
	authorityGoalID := roles.Authority.Id
	preLockSnapshots, err := proofAdmissionGoalState(request.ControlRoot, roles.Candidate, now)
	if err != nil {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
	}
	preLockAuthority := preLockSnapshots
	if authorityGoalID != request.GoalID {
		preLockAuthority, err = proofAdmissionGoalState(request.ControlRoot, roles.Authority, now)
		if err != nil {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
		}
	}
	admissionSnapshots := proofAdmissionSnapshots{Candidate: preLockSnapshots, Authority: preLockAuthority}
	candidateRevision := roles.CandidateRevision
	if request.CandidateRevision != 0 {
		if request.CandidateRevision != candidateRevision {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, candidateGoalRefusal(request.GoalID, "moved", fmt.Sprintf("budgetEpisodeRevision=%d expected=%d", candidateRevision, request.CandidateRevision))
		}
		candidateRevision = request.CandidateRevision
	}
	var capValue int64
	if reservationOwner != nil {
		runRecord, readErr := (&runpkg.Store{Root: request.ControlRoot}).Read(reservationOwner.RunID)
		if readErr != nil || runRecord == nil || runRecord.Governed == nil {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("governed proof owner became unreadable while resolving its reservation")
		}
		capValue = int64(runRecord.Governed.ExecutionCostMinutes)
	} else {
		capValue, _, _, err = dispatchcore.ResolveCap(request.ConfPath, "proof", "main", "proof", "", request.CapMin)
		if err != nil || capValue < 1 {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("resolve proof reservation: %w", err)
		}
	}
	binding, err := dispatchcore.ResolveGoalBinding(request.ControlRoot, authorityGoalID, now)
	if err != nil {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
	}
	if delegateRevision != 0 && binding.Revision != delegateRevision {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("native delegate goal revision moved from %d to %d", delegateRevision, binding.Revision)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
	}
	var context proofrun.ExecutionContext
	if request.SharedEngine != "" {
		context, err = proofrun.CaptureSharedExecutionContext(request.ExecutionRoot, request.ConfPath, request.Environment, request.SharedEngine, request.SharedManifestDigest)
	} else {
		context, err = proofrun.CaptureExecutionContext(request.ExecutionRoot, request.ConfPath, request.Environment)
	}
	if err != nil {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
	}
	proofIdentity := proofrun.BuildProofIdentityForContext(context, request.ScopeClass,
		request.CommandClass, request.Sections, behaviorsurface.SupportedVersion)
	proofIdentity = proofrun.BindIdentityInputs(proofIdentity, request.IdentityInputs)
	candidateTree := proofAdmissionCandidateTree(request)
	heldGoals, err := acquireProofAdmissionGoalLocks(request.ControlRoot, admissionSnapshots)
	if err != nil {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
	}
	defer releaseProofAdmissionGoalLocks(heldGoals)
	heldProof, err := proofrun.AcquireMutation(request.ControlRoot)
	if err != nil {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
	}
	defer heldProof.Release()
	if proofAdmissionUnderLocks != nil {
		proofAdmissionUnderLocks()
	}
	lockedSnapshots, err := proofAdmissionGoalStates(request.ControlRoot, request.GoalID, authorityGoalID, now)
	if err != nil {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
	}
	if err := proofAdmissionMoved(admissionSnapshots, lockedSnapshots, request.GoalID, authorityGoalID); err != nil {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
	}
	classifiedCaller, err = classifyVerbCaller(request.ControlRoot, int64(os.Getppid()))
	if err != nil {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof caller reclassification failed under admission lock: %w", err)
	}
	extensionAuthorized := false
	extensionCallerClass := classifiedCaller.Class
	if reservationOwner != nil {
		runRecord, readErr := (&runpkg.Store{Root: request.ControlRoot}).Read(reservationOwner.RunID)
		if readErr != nil || runRecord == nil || runRecord.Governed == nil || runRecord.Status != runpkg.StatusRunning ||
			runRecord.Pid == nil || runRecord.PidStartedAt == nil || runRecord.Generation != reservationOwner.RunGeneration ||
			runRecord.LaunchNonce != reservationOwner.LaunchNonce || runRecord.GoalId != authorityGoalID ||
			runRecord.Governed.GoalRevision != reservationOwner.GoalRevision ||
			runRecord.Governed.ObligationRevision != reservationOwner.ObligationRevision ||
			runRecord.Governed.AttemptOrdinal != reservationOwner.AttemptOrdinal {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("governed proof owner changed before reservation")
		}
		ancestor := proofrun.ProcessIdentity{Pid: *runRecord.Pid, PidStartedAt: *runRecord.PidStartedAt,
			PidStartTicks: runRecord.PidStartTicks, BootID: runRecord.BootID}
		if err := proofrun.AuthenticateAncestor(int64(os.Getppid()), ancestor); err != nil {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("governed proof custody changed before reservation: %w", err)
		}
	} else if delegateJob != "" {
		verified, verifyErr := lease.HookDelegate(delegateState, delegateInstallation, delegateJob, int64(os.Getppid()))
		record, readErr := dispatchcore.ReadRecordObject(filepath.Join(delegateState, "artifacts", "agents", "jobs", delegateJob+".json"))
		lens := dispatchcore.JobRecordOf(record)
		revision, revisionOK := lens.GoalRevision()
		claimEpoch, claimEpochOK := lens.ClaimEpoch()
		if verifyErr != nil || !verified.Delegate || verified.JobID != delegateJob || readErr != nil ||
			lens.GoalID() != authorityGoalID || !revisionOK || revision != binding.Revision ||
			lens.MachineID() != binding.Machine || !claimEpochOK || claimEpoch != binding.Capability.ClaimEpoch {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("native delegate proof custody changed before reservation")
		}
		extensionAuthorized = true
		extensionCallerClass = lease.ClassDelegate
	} else if classifiedCaller.Class == lease.ClassMain {
		machine, machineErr := goal.ResolveMachine(request.ControlRoot)
		if machineErr != nil || !classifiedCaller.Holder || classifiedCaller.ClaimEpoch == nil ||
			*classifiedCaller.ClaimEpoch != binding.Capability.ClaimEpoch || machine != binding.Machine {
			if machineErr == nil && classifiedCaller.Holder && classifiedCaller.ClaimEpoch != nil &&
				machine == binding.Machine && *classifiedCaller.ClaimEpoch != binding.Capability.ClaimEpoch &&
				binding.File.Claimed != nil {
				holder, holderErr := lease.CurrentHolder(request.ControlRoot)
				if holderErr == nil && holder.OwnerLineage == binding.File.Claimed.Lineage {
					return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf(
						"active coordinator does not own the claimed goal reservation: lease claim epoch %d differs from stop capability claim epoch %d; run metasystem goal restamp --id %s",
						*classifiedCaller.ClaimEpoch, binding.Capability.ClaimEpoch, authorityGoalID)
				}
			}
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("active coordinator does not own the claimed goal reservation")
		}
		extensionAuthorized = true
	} else if classifiedCaller.Class != lease.ClassHuman {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof caller has no authenticated goal reservation context")
	}
	binding, err = dispatchcore.ResolveGoalBinding(request.ControlRoot, authorityGoalID, now)
	if err != nil || binding.Fence != nil || reservationOwner != nil && binding.Revision != reservationOwner.GoalRevision {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof reservation lost its accepted goal authority")
	}
	projection := dispatchcore.ProjectBudget(request.ControlRoot, binding.File, now)
	if projection.Status != dispatchcore.BudgetKnown {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof reservation budget projection is unknown")
	}
	candidateProjection := dispatchcore.ProjectConsumption(request.ControlRoot, roles.Candidate, now)
	if candidateProjection.Status != dispatchcore.BudgetKnown {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, candidateGoalRefusal(request.GoalID, "budget-unknown", "consumption projection is unknown")
	}
	accountingRevision := binding.File.Claimed.AccountingRevision
	if accountingRevision == 0 {
		accountingRevision = binding.Revision
	}
	if request.ExpectedGoalRevision != 0 && (binding.Revision != request.ExpectedGoalRevision || accountingRevision != request.ExpectedAccountingRevision) {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("GOAL_REVISION_MOVED: expected goal/accounting revisions %d/%d, found %d/%d",
			request.ExpectedGoalRevision, request.ExpectedAccountingRevision, binding.Revision, accountingRevision)
	}
	if request.RequireDiagnosticHeadroom {
		attemptHeadroom := projection.Limits.AttemptLimit >= projection.Attempts && projection.Limits.AttemptLimit-projection.Attempts >= 2
		minuteHeadroom := uint64(capValue) <= ^uint64(0)/2 && projection.Limits.ReservedJobMinutesLimit >= projection.ReservedJobMinutes &&
			projection.Limits.ReservedJobMinutesLimit-projection.ReservedJobMinutes >= 2*uint64(capValue)
		if !attemptHeadroom || !minuteHeadroom {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("BATCH_MEMBER_BUDGET_REFUSED: goal %s needs two attempts and %d reserved minutes of P2 headroom", authorityGoalID, 2*uint64(capValue))
		}
	}
	checkoutFence, fenceErr := stopfence.Read(request.ControlRoot)
	if fenceErr != nil || checkoutFence.State == stopfence.StateClosed {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof reservation lost checkout-fence authority")
	}
	reservation := proofrun.AdmissionRequest{
		ControlRoot: request.ControlRoot, ExecutionRoot: request.ExecutionRoot, ConfPath: request.ConfPath, GoalID: authorityGoalID,
		GoalRevision: binding.Revision, AccountingRevision: accountingRevision, BudgetEpoch: projection.WeightEpoch,
		CandidateGoalID: request.GoalID, CandidateRevision: candidateRevision, CandidateBudgetEpoch: candidateProjection.WeightEpoch,
		CandidateTree:   candidateTree,
		ReservedMinutes: uint64(capValue), Identity: proofIdentity, Launcher: launcher, ReservationOwner: reservationOwner,
		RetryDecisionPath: request.RetryDecision, Now: now,
		ComponentIdentities: request.ComponentIdentities, ForceAttempt: request.ForceAttempt, ForceGroups: request.ForceGroups,
		SharedComponents: request.CommandClass == "testing" && len(request.ComponentIdentities) > 0,
		ManagedCapacity:  request.ManagedCapacity,
		FreshnessEpisode: request.FreshnessEpisode, FreshnessBinding: request.FreshnessBinding,
		FreshnessExpiresAt: request.FreshnessExpiresAt,
		FreshGroups:        request.FreshGroups,
	}
	decision, noChild, err := proofrun.NoChildDecisionLocked(reservation)
	if err != nil {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
	}
	if noChild {
		return proofrun.Attempt{}, decision, false, nil
	}
	if reservationOwner == nil {
		candidateFile := roles.Candidate
		if request.GoalID == authorityGoalID {
			candidateFile = binding.File
		}
		verdict, err := dispatchcore.EvaluateProofAdmissionForDispatch(request.ControlRoot, authorityGoalID, binding.Revision,
			candidateFile, candidateRevision, uint64(capValue), now, "implementer", "fresh", dispatchcore.HazardMechanical)
		if err != nil {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
		}
		if verdict.Authority.Extension != nil {
			if !extensionAuthorized {
				return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof reservation found an extension for goal %s, but only its claim holder pair may apply it", authorityGoalID)
			}
			if delegateJob == "" {
				holder, holderErr := lease.CurrentHolder(request.ControlRoot)
				if holderErr != nil || holder.OwnerLineage != binding.Lineage {
					return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof reservation could not bind the budget extension to the claim holder pair")
				}
			}
			endpoint, endpointErr := goal.ResolveEndpoint(request.ControlRoot)
			ulid, ulidErr := goalUlid()
			if endpointErr != nil || ulidErr != nil {
				return proofrun.Attempt{}, proofrun.LaunchResult{}, false, errors.Join(endpointErr, ulidErr)
			}
			offer := verdict.Authority.Extension.GoalOffer()
			extensionRequest := goal.VerbRequest{Endpoint: endpoint,
				Actor: goal.Actor{Machine: binding.Machine, Lineage: binding.Lineage}, Ulid: ulid, Now: now,
				CallerClass: extensionCallerClass}
			extended, extendErr := goal.ExtendBudget(extensionRequest, authorityGoalID, offer)
			if extendErr != nil {
				return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof reservation extend budget: %w", extendErr)
			}
			if extended.Outcome != goal.OutcomeConfirmed {
				return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof reservation extend budget ended %s: %s", extended.Outcome, extended.Detail)
			}
			binding, err = dispatchcore.ResolveGoalBinding(request.ControlRoot, authorityGoalID, now)
			if err != nil || binding.Revision != reservation.GoalRevision {
				return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof reservation lost its goal binding after budget extension")
			}
			marker := binding.File.BudgetExtension
			if binding.Machine != extensionRequest.Actor.Machine || binding.Lineage != extensionRequest.Actor.Lineage ||
				marker == nil || marker.AttemptLimitFrom != offer.AttemptLimitFrom || marker.AttemptLimitTo != offer.AttemptLimitTo ||
				marker.ReservedJobMinutesFrom != offer.ReservedJobMinutesFrom || marker.ReservedJobMinutesTo != offer.ReservedJobMinutesTo {
				return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof reservation budget extension is not visible on its authoritative claim pair")
			}
			projection = dispatchcore.ProjectBudget(request.ControlRoot, binding.File, now)
			if projection.Status != dispatchcore.BudgetKnown {
				return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof reservation budget projection became unknown after extension")
			}
			reservation.BudgetEpoch = projection.WeightEpoch
			reservation.CandidateBudgetEpoch = projection.WeightEpoch
			lockedSnapshots, err = proofAdmissionGoalStates(request.ControlRoot, request.GoalID, authorityGoalID, now)
			if err != nil {
				return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
			}
			verdict, err = dispatchcore.EvaluateProofAdmissionForDispatch(request.ControlRoot, authorityGoalID, binding.Revision,
				binding.File, candidateRevision, uint64(capValue), now, "implementer", "fresh", dispatchcore.HazardMechanical)
			if err != nil {
				return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
			}
		}
		if verdict.Refused() {
			lines := dispatchcore.FormatProofAdmission(verdict)
			detail := strings.Join(lines, "; ")
			if detail == "" {
				detail = "proof admission refused without a printable reason"
			}
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof reservation refused for candidate %s under authority %s revision %d: %s",
				request.GoalID, authorityGoalID, binding.Revision, detail)
		}
	}
	if proofAdmissionBeforePublish != nil {
		proofAdmissionBeforePublish(&reservation)
	}
	if request.BeforePublish != nil {
		request.BeforePublish(&reservation)
	}
	attempt, decision, err := proofrun.ReserveLocked(reservation)
	if err != nil || attempt.AttemptID == "" {
		return attempt, decision, false, err
	}
	if proofAdmissionAfterPublish != nil {
		proofAdmissionAfterPublish()
	}
	publishedSnapshots, snapshotErr := proofAdmissionGoalStates(request.ControlRoot, request.GoalID, authorityGoalID, now)
	if snapshotErr == nil {
		snapshotErr = proofAdmissionMoved(lockedSnapshots, publishedSnapshots, request.GoalID, authorityGoalID)
	}
	if snapshotErr != nil {
		withdrawErr := proofrun.WithdrawReservationLocked(request.ControlRoot, attempt.AttemptID)
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, errors.Join(snapshotErr, withdrawErr)
	}
	return attempt, decision, false, nil
}

func proofAdmissionCandidateTree(request proofLaunchAdmission) string {
	if request.CommandClass != "testing" {
		return ""
	}
	if validProofCandidateTree(request.CandidateTree) {
		return request.CandidateTree
	}
	identity := proofrun.ProofIdentity{CommandClass: request.CommandClass, IdentityInputs: request.IdentityInputs}
	if candidate, ok := proofrun.CandidateTreeFromProofIdentity(identity); ok {
		return candidate
	}
	return ""
}

func validProofCandidateTree(value string) bool {
	if len(value) != 40 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func uniqueActiveProofGoal(root string, now time.Time) (string, error) {
	machine, err := goal.ResolveMachine(root)
	if err != nil {
		return "", err
	}
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return "", err
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		return "", fmt.Errorf("resolve active claimed goal for proof: %w", err)
	}
	if projection.Tree == nil {
		return "", fmt.Errorf("resolve active claimed goal for proof: accepted goal projection is empty")
	}
	selected := ""
	fenced := make([]*goal.GoalFile, 0)
	for _, id := range goal.OrderedOpenGoalIDs(projection.Tree.Live) {
		file := projection.Tree.Live[id]
		if file.State != goal.StateClaimed || file.Claimed == nil || file.Claimed.Machine != machine {
			continue
		}
		if file.IsFencedClaim() {
			fenced = append(fenced, file)
			continue
		}
		if selected != "" {
			return "", fmt.Errorf("proof accounting is ambiguous: machine %s has multiple claimed goals; pass --goal", machine)
		}
		selected = id
	}
	if selected == "" {
		if len(fenced) == 1 {
			return "", fmt.Errorf(
				"proof accounting has no live claimed goal for machine %s; the only claim here is breach-stopped: %s (stop %s); pass --goal",
				machine, fenced[0].Id, fenced[0].StopFence.StopID,
			)
		}
		return "", fmt.Errorf("proof accounting is ambiguous: machine %s has no claimed goal; pass --goal", machine)
	}
	return selected, nil
}

func commitProofTerminal(completion proofrun.CompletionContext, receipt json.RawMessage) error {
	return commitProofTerminalWithTestResult(completion, receipt, nil)
}

func commitProofTerminalWithTestResult(completion proofrun.CompletionContext, receipt json.RawMessage, testResult *proofrun.TestResult) error {
	return commitProofTerminalWithReason(completion, receipt, testResult, "proof launcher completed")
}

// terminalCommitTries is how many times the terminal commit tries its locks
// before it gives the attempt up as incomplete with the holder named
// (proof-groups-detect-hangs-by-progress-not-the-clock, slice 2c). Each try
// is the lock's own bounded wait (ten seconds on the stop fence, one on the
// goal revision), so a healthy holder that runs long under load is waited
// out across the tries instead of losing a green proof at the first refusal;
// a wedged holder is still refused, by name, within a minute or so. The stop
// fence is released before a refused goal-revision try is repeated, so a
// holder there never parks every landing's terminal commit behind this one.
const terminalCommitTries = 6

// terminalCommitPause is the gap between two tries. The lock's other
// waiters poll every 25 ms, so a fence released and retaken within a few
// milliseconds would never be seen free; the pause is what makes the
// release between tries real (the Opus read of this slice, F-1).
const terminalCommitPause = 250 * time.Millisecond

// terminalLockSeam is how the terminal commit takes its two ranked locks,
// pauses between tries and notes a refused try when no launcher stream is
// at hand; tests script it and run in no wall time. The proof mutation lock
// is not part of it: it is waited for, never refused.
type terminalLockSeam struct {
	stopFence    func(root, verb string, ref identity.Ref, scaleMilli int) (*lock.Lock, error)
	goalRevision func(root, goalID string, revision uint64, tag string) (*goalrevision.Held, error)
	pause        func(time.Duration)
	notes        io.Writer
}

var terminalLocks = terminalLockSeam{stopFence: stopfence.Acquire, goalRevision: goalrevision.Acquire, pause: time.Sleep, notes: os.Stderr}

// describeRefusal names the holder a refusal waited behind, with the owner
// file's read error when that is what made the holder unprovable.
func describeRefusal(err error) string {
	var holder *lock.HolderError
	if errors.As(err, &holder) && holder.Cause != nil {
		return fmt.Sprintf("%v; owner file: %v", err, holder.Cause)
	}
	return err.Error()
}

// terminalCommitRefused is the last refusal after every try, carrying the
// holder the commit waited behind.
type terminalCommitRefused struct {
	Tries  int
	Holder error
}

func (e *terminalCommitRefused) Error() string {
	return fmt.Sprintf("proof terminal commit refused %d times; the last holder: %s", e.Tries, describeRefusal(e.Holder))
}

func (e *terminalCommitRefused) Unwrap() error { return e.Holder }

// terminalLockRefused reports whether an acquisition error names a holder
// (live, or unproven: an unreadable owner file is a holder by the lock's
// rule), the one case a later try can succeed on. Every other error is
// final on the first try.
func terminalLockRefused(err error) bool {
	var holder *lock.HolderError
	if errors.As(err, &holder) {
		return true
	}
	var busy *goalrevision.Busy
	return errors.As(err, &busy)
}

// heldTerminalLocks is the stop fence, the goal revision and the proof
// mutation lock, taken in that order and released in reverse.
type heldTerminalLocks struct {
	fence *lock.Lock
	goal  *goalrevision.Held
	proof *proofrun.MutationLock
}

func (h *heldTerminalLocks) release() {
	if h == nil {
		return
	}
	if h.proof != nil {
		_ = h.proof.Release()
	}
	if h.goal != nil {
		_ = h.goal.Release()
	}
	if h.fence != nil {
		_ = h.fence.Release()
	}
}

func tryTerminalLocks(root, goalID string, revision uint64, ref identity.Ref) (*heldTerminalLocks, error) {
	fence, err := terminalLocks.stopFence(root, "proof-finalize", ref, 1000)
	if err != nil {
		return nil, err
	}
	heldGoal, err := terminalLocks.goalRevision(root, goalID, revision, "proof-finalize")
	if err != nil {
		_ = fence.Release()
		return nil, err
	}
	heldProof, err := proofrun.AcquireMutation(root)
	if err != nil {
		_ = heldGoal.Release()
		_ = fence.Release()
		return nil, err
	}
	return &heldTerminalLocks{fence: fence, goal: heldGoal, proof: heldProof}, nil
}

// acquireTerminalLocks takes the commit's locks, trying a refusal again up
// to terminalCommitTries times, a pause apart, with the holder it waited
// behind noted on each refused try. The last refusal is returned as
// terminalCommitRefused.
func acquireTerminalLocks(root, goalID string, revision uint64, ref identity.Ref, notes io.Writer) (*heldTerminalLocks, error) {
	var last error
	for try := 1; try <= terminalCommitTries; try++ {
		held, err := tryTerminalLocks(root, goalID, revision, ref)
		if err == nil {
			return held, nil
		}
		if !terminalLockRefused(err) {
			return nil, err
		}
		last = err
		if try < terminalCommitTries {
			fmt.Fprintf(notes, "proof terminal commit: try %d of %d waits behind %s\n", try, terminalCommitTries, describeRefusal(err))
			terminalLocks.pause(terminalCommitPause)
		}
	}
	return nil, &terminalCommitRefused{Tries: terminalCommitTries, Holder: last}
}

// retainRefusedTerminal records the attempt as incomplete with the holder
// named once every try was refused, under the proof mutation lock alone: the
// record says what the commit waited behind, so the next reader (a retry
// decision, a person) can act on it, and the launcher's fallback finds a
// terminal already written. An attempt that a stop batch had already asked
// to cancel is recorded cancelled, as the commit would have recorded it.
func retainRefusedTerminal(completion proofrun.CompletionContext, refused error, notes io.Writer) {
	status := completion.ExitStatus
	if status == 0 {
		status = 1
	}
	result := proofrun.TerminalUnknown
	if attempt, err := proofrun.ReadAttempt(completion.ControlRoot, completion.AttemptID); err == nil && attempt.CancellationIntent != "" {
		result = proofrun.TerminalCancelled
	}
	if _, err := proofrun.FinalizeAttempt(completion.ControlRoot, completion.AttemptID, result, status,
		refused.Error(), nil, time.Now().UTC()); err != nil {
		fmt.Fprintf(notes, "proof terminal commit: retain the refused attempt: %v\n", err)
	}
}

func commitProofTerminalWithReason(completion proofrun.CompletionContext, receipt json.RawMessage, testResult *proofrun.TestResult, reason string) error {
	attempt, err := proofrun.ReadAttempt(completion.ControlRoot, completion.AttemptID)
	if err != nil {
		return err
	}
	current, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		return err
	}
	notes := terminalLocks.notes
	if completion.ErrorOutput != nil {
		notes = completion.ErrorOutput
	}
	held, err := acquireTerminalLocks(completion.ControlRoot, attempt.GoalID, attempt.GoalRevision, current.Ref(), notes)
	if err != nil {
		var refused *terminalCommitRefused
		if errors.As(err, &refused) {
			retainRefusedTerminal(completion, err, notes)
		}
		return err
	}
	defer held.release()
	attempt, err = proofrun.ReadAttempt(completion.ControlRoot, completion.AttemptID)
	if err != nil {
		return err
	}
	record, err := proofrun.ReadProcessRecord(completion.ControlRoot, completion.RecordKey)
	if err != nil || record.Status != proofrun.StatusRunning {
		return fmt.Errorf("proof process inventory is not live at terminal commit")
	}
	if completion.ExitStatus == 0 && completion.InputIdentity != attempt.ProofIdentity.IdentityDigest {
		return fmt.Errorf("proof terminal commit has no matching pre-cleanup input-parity observation")
	}
	fence, err := stopfence.Read(completion.ControlRoot)
	if err != nil || fence.State == stopfence.StateClosed || fence.Generation != record.FenceGeneration {
		return fmt.Errorf("proof terminal commit lost stop-fence authority")
	}
	finalizedAt := time.Now().UTC()
	binding, err := dispatchcore.ResolveGoalBinding(completion.ControlRoot, attempt.GoalID, finalizedAt)
	if err != nil || binding.Revision != attempt.GoalRevision || binding.Fence != nil {
		return fmt.Errorf("proof terminal commit lost goal-revision authority")
	}
	result := proofrun.TerminalFailed
	if attempt.CancellationIntent != "" {
		result = proofrun.TerminalCancelled
	} else if completion.ExitStatus == 0 {
		result = proofrun.TerminalSuccess
	}
	_, err = proofrun.FinalizeAttemptWithTestResultLocked(completion.ControlRoot, completion.AttemptID, result,
		completion.ExitStatus, reason, receipt, testResult, finalizedAt)
	return err
}

type proofRunLimits struct {
	silence         time.Duration
	sectionCap      time.Duration
	evidenceTimeout time.Duration
	evidenceMax     int64
	concurrency     int
}

// defaultTestingConcurrency is how many groups of one stage run at once when
// metasystem.conf names no testing.concurrency: a third of the cores, at
// most six, at least one. Half the cores oversubscribed an 18-core Mac in
// the first pooled cadence runs of 2026-09-11, when two serial giants and
// the whole race gate ran as single processes, and a quarter (four) was the
// measured answer that afternoon. By the end of that day the giants ran in
// shards and every bed's scenarios side by side, and cadence run 8 spent
// 969 s with the four slots full and 8.9 of 18 cores busy on average: the
// slot count, not the box, was the wall (the groups summed to 3796 s). Run
// 9 then measured six slots: 896 s, but every heavy group 30 to 60 percent
// slower (the walls summed to 5110 s) and one more fixed fixture cap
// tripped, so the 73 s were bought with contention that turns into reds.
// A quarter stays the default (four there, one on a 4-vCPU VM) until the
// duplicated coverage work at cadence is gone; a wide box that wants six
// names it in metasystem.conf.
func defaultTestingConcurrency() int {
	cap := runtime.NumCPU() / 4
	if cap > 6 {
		cap = 6
	}
	if cap < 1 {
		cap = 1
	}
	return cap
}

func proofRunConfigProblems(confPath string) ([]string, error) {
	var problems []string
	for _, knob := range []struct {
		name    string
		minimum int
		maximum int
	}{
		{"suite.progress-silence-min", 1, 600},
		{"suite.section-cap-min", 1, 600},
		{"suite.evidence-copy-timeout-sec", 1, 600},
		{"suite.evidence-copy-max-mb", 1, 10240},
		{"testing.concurrency", 1, 64},
	} {
		raw, found, err := config.ConfLookup(confPath, knob.name)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < knob.minimum || parsed > knob.maximum {
			problems = append(problems, fmt.Sprintf("%s must be an integer from %d through %d, got %q", knob.name, knob.minimum, knob.maximum, raw))
		}
	}
	return problems, nil
}

func resolveProofRunLimits(confPath string) (proofRunLimits, error) {
	read := func(key string, def, minimum, maximum int) (int, error) {
		value, _, err := config.Get(config.GetParams{
			Key: key, Default: strconv.Itoa(def), DefaultSet: true, ConfPath: confPath,
		})
		if err != nil {
			return 0, err
		}
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < minimum || parsed > maximum {
			return 0, fmt.Errorf("%s must be an integer from %d through %d", key, minimum, maximum)
		}
		return parsed, nil
	}
	silence, err := read("suite.progress-silence-min", 30, 1, 600)
	if err != nil {
		return proofRunLimits{}, err
	}
	section, err := read("suite.section-cap-min", 45, 1, 600)
	if err != nil {
		return proofRunLimits{}, err
	}
	evidenceTimeout, err := read("suite.evidence-copy-timeout-sec", 60, 1, 600)
	if err != nil {
		return proofRunLimits{}, err
	}
	evidenceMB, err := read("suite.evidence-copy-max-mb", 512, 1, 10240)
	if err != nil {
		return proofRunLimits{}, err
	}
	concurrency, err := read("testing.concurrency", defaultTestingConcurrency(), 1, 64)
	if err != nil {
		return proofRunLimits{}, err
	}
	return proofRunLimits{
		silence:         time.Duration(silence) * time.Minute,
		sectionCap:      time.Duration(section) * time.Minute,
		evidenceTimeout: time.Duration(evidenceTimeout) * time.Second,
		evidenceMax:     int64(evidenceMB) * 1024 * 1024,
		concurrency:     concurrency,
	}, nil
}

func selectedSections(selector, selected string, enumerated bool) ([]string, map[string]bool, error) {
	if selector == "" {
		if selected != "" {
			return []string{selected}, map[string]bool{}, nil
		}
		return nil, map[string]bool{}, nil
	}
	command := exec.Command("bash", selector, "list")
	output, err := command.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("list validation selector: %w", err)
	}
	var sections []string
	known := map[string]bool{}
	for _, line := range bytes.Split(bytes.TrimSpace(output), []byte{'\n'}) {
		id, _, found := bytes.Cut(line, []byte{'\t'})
		if !found || len(id) == 0 {
			return nil, nil, fmt.Errorf("selector emitted invalid row %q", line)
		}
		section := string(id)
		sections = append(sections, section)
		known[section] = true
	}
	// Enumeration owns one progress interval for every row returned by this
	// invocation of the selector. Twice-consulted declarations apply only to
	// a full validation run, where separate call sites may revisit a section.
	if enumerated {
		return sections, map[string]bool{}, nil
	}

	command = exec.Command("bash", selector, "twice")
	output, err = command.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("read twice-consulted validation sections: %w", err)
	}
	declaredTwice := map[string]bool{}
	trimmed := bytes.TrimSpace(output)
	if len(trimmed) > 0 {
		for _, line := range bytes.Split(trimmed, []byte{'\n'}) {
			section := string(line)
			if !known[section] {
				return nil, nil, fmt.Errorf("twice-consulted section %q is absent from the selector", section)
			}
			declaredTwice[section] = true
		}
	}
	if selected == "" {
		return sections, declaredTwice, nil
	}
	// A selected run drives one call site, so it expects one interval even
	// when the full validation run consults that section more than once.
	return []string{selected}, map[string]bool{}, nil
}

func runProofRunWatchdog(args []string) int {
	flags := flag.NewFlagSet("proof-run watchdog", flag.ContinueOnError)
	suite := flags.String("suite", "", "suite name")
	root := pathFlag(flags, "root", "", "metasystem root")
	conf := flags.String("conf", "", "metasystem configuration")
	progress := flags.String("progress", "", "progress JSONL")
	done := flags.String("done", "", "launcher done file")
	suitePID := flags.Int64("suite-pid", 0, "suite process identifier")
	suiteStarted := flags.Int64("suite-started-at", 0, "suite start epoch second")
	suiteTicks := flags.Int64("suite-start-ticks", 0, "suite boot-relative start ticks")
	suiteBoot := flags.String("suite-boot-id", "", "suite boot identity")
	fenceGeneration := flags.Int64("fence-generation", 0, "stop-fence generation")
	silenceMS := flags.Int64("silence-ms", 0, "output-silence milliseconds")
	sectionCapMS := flags.Int64("section-cap-ms", 0, "section-cap milliseconds")
	evidenceTimeoutMS := flags.Int64("evidence-timeout-ms", 0, "evidence timeout milliseconds")
	evidenceMax := flags.Int64("evidence-max-bytes", 0, "evidence byte cap")
	pollMS := flags.Int64("poll-ms", 1000, "poll milliseconds")
	termGraceMS := flags.Int64("term-grace-ms", 5000, "TERM grace milliseconds")
	killGraceMS := flags.Int64("kill-grace-ms", 1000, "KILL observation milliseconds")
	deadlineRaw := flags.String("deadline", "", "absolute proof deadline")
	watchControlRoot := flags.String("control-root", "", "control root of the attempt whose cancellation intent ends the suite")
	watchAttempt := flags.String("attempt", "", "attempt whose cancellation intent ends the suite")
	resourceCustody := flags.Bool("resource-custody", false, "anchor a resource-active process group")
	custodyParentPID := flags.Int64("custody-parent-pid", 0, "exact launcher pid")
	custodyParentStarted := flags.Int64("custody-parent-started-at", 0, "exact launcher start second")
	custodyParentTicks := flags.Int64("custody-parent-start-ticks", 0, "exact launcher start ticks")
	custodyParentBoot := flags.String("custody-parent-boot-id", "", "exact launcher boot id")
	custodyParentMicro := flags.Int64("custody-parent-start-micro", 0, "exact launcher start microseconds")
	custodyControlFD := flags.Int("custody-control-fd", 0, "custody control descriptor")
	custodyReadyFD := flags.Int("custody-ready-fd", 0, "custody readiness descriptor")
	custodyMarkerFD := flags.Int("custody-marker-fd", 0, "custody marker descriptor")
	custodyMarkerPath := flags.String("custody-marker-path", "", "custody marker path")
	var logs repeatedFlag
	flags.Var(&logs, "log", "watched output log (repeatable)")
	if flags.Parse(args) != nil || flags.NArg() != 0 || (!*resourceCustody && *conf == "") {
		return 2
	}
	// Re-read the layered configuration in the sibling process immediately
	// before watchdog startup. A conf.local or environment change between
	// launcher resolution and spawn can therefore only refuse, never install
	// an unlawful operational window. Fixture duration overrides remain the
	// passed values after this effective-value validation.
	if *conf != "" {
		if _, err := resolveProofRunLimits(*conf); err != nil {
			fmt.Fprintln(os.Stderr, "proof-run watchdog:", err)
			return 1
		}
	}
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run watchdog:", err)
		return 1
	}
	var deadline time.Time
	if *deadlineRaw != "" {
		deadline, err = time.Parse(time.RFC3339Nano, *deadlineRaw)
		if err != nil {
			fmt.Fprintln(os.Stderr, "proof-run watchdog: invalid absolute deadline:", err)
			return 2
		}
	}
	options := proofrun.WatchdogOptions{
		Suite: *suite, Root: *root, ProgressPath: *progress, DonePath: *done, LogPaths: logs,
		ControlRoot: *watchControlRoot, AttemptID: *watchAttempt,
		SuiteIdentity:   identity.Ref{Pid: *suitePID, StartedAtSec: *suiteStarted, StartTicks: *suiteTicks, BootID: *suiteBoot},
		FenceGeneration: *fenceGeneration,
		Deadline:        deadline,
		Silence:         time.Duration(*silenceMS) * time.Millisecond,
		SectionCap:      time.Duration(*sectionCapMS) * time.Millisecond,
		EvidenceTimeout: time.Duration(*evidenceTimeoutMS) * time.Millisecond,
		EvidenceMax:     *evidenceMax, Poll: time.Duration(*pollMS) * time.Millisecond,
		TermGrace:  time.Duration(*termGraceMS) * time.Millisecond,
		KillGrace:  time.Duration(*killGraceMS) * time.Millisecond,
		Executable: executable, Output: os.Stdout, ErrorOutput: os.Stderr,
	}
	if *resourceCustody {
		err = proofrun.RunResourceCustodian(proofrun.ResourceCustodyOptions{Watchdog: options,
			Launcher: identity.Ref{Pid: *custodyParentPID, StartedAtSec: *custodyParentStarted,
				StartedAtUnixMicro: *custodyParentMicro, StartTicks: *custodyParentTicks, BootID: *custodyParentBoot},
			ControlFD: *custodyControlFD, ReadyFD: *custodyReadyFD,
			MarkerFD: *custodyMarkerFD, MarkerPath: *custodyMarkerPath, ConfPath: *conf})
	} else {
		err = proofrun.RunWatchdog(options)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "suite watchdog:", err)
		return 1
	}
	return 0
}

func runProofRunCustodyExec(args []string) int {
	flags := flag.NewFlagSet("proof-run custody-exec", flag.ContinueOnError)
	readyFD := flags.Int("ready-fd", 0, "readiness descriptor")
	releaseFD := flags.Int("release-fd", 0, "start barrier descriptor")
	path := flags.String("path", "", "selected executable path")
	if flags.Parse(args) != nil || flags.NArg() < 1 || *readyFD < 3 || *releaseFD < 3 || *path == "" {
		return 2
	}
	ready := os.NewFile(uintptr(*readyFD), "custody-exec-ready")
	release := os.NewFile(uintptr(*releaseFD), "custody-exec-release")
	if _, err := ready.Write([]byte("ready\n")); err != nil {
		return 1
	}
	_ = ready.Close()
	var token [1]byte
	if count, err := release.Read(token[:]); err != nil || count != 1 || token[0] != 1 {
		return 1
	}
	_ = release.Close()
	if err := syscall.Exec(*path, flags.Args(), os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, "proof-run custody-exec:", err)
		return 1
	}
	return 0
}

func runProofRunPreserve(args []string) int {
	flags := flag.NewFlagSet("proof-run preserve", flag.ContinueOnError)
	destination := flags.String("destination", "", "evidence destination")
	maxBytes := flags.Int64("max-bytes", 0, "evidence byte cap")
	var sources repeatedFlag
	flags.Var(&sources, "source", "evidence source (repeatable)")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	result, err := proofrun.PreserveEvidence(*destination, sources, *maxBytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run preserve:", err)
		return 1
	}
	fmt.Printf("copied %d bytes; dropped %d paths; copy errors %d\n", result.CopiedBytes, len(result.Dropped), len(result.Errors))
	for _, dropped := range result.Dropped {
		fmt.Printf("DROPPED %s\n", dropped)
	}
	for _, copyError := range result.Errors {
		fmt.Printf("ERROR %s\n", copyError)
	}
	return 0
}

func runProofRunAssert(args []string) int {
	flags := flag.NewFlagSet("proof-run assert", flag.ContinueOnError)
	progress := flags.String("progress", "", "progress JSONL")
	suite := flags.String("suite", "", "suite name")
	selector := flags.String("selector", "", "section selector")
	selected := flags.String("selected", "", "single selected section")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	expected, repeated, err := selectedSections(*selector, *selected, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run assert:", err)
		return 1
	}
	run, err := proofrun.ReadLatestProgressRun(*progress)
	if err == nil {
		err = proofrun.AssertSectionProgress(run, *suite, expected, repeated)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run assert:", err)
		return 1
	}
	return 0
}

func runProofRunCoverageBegin(args []string) int {
	flags := flag.NewFlagSet("proof-run coverage-begin", flag.ContinueOnError)
	executionRoot := pathFlag(flags, "root", "", "executing full-gate root")
	baseline := flags.String("baseline", "", "selected coverage ratchet")
	producerPID := flags.Int64("producer-pid", 0, "full-gate producer process")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *executionRoot == "" || *baseline == "" || *producerPID < 1 {
		return 2
	}
	controlRoot, attemptID := os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT"), os.Getenv("METASYSTEM_PROOF_ATTEMPT")
	if controlRoot == "" || attemptID == "" {
		fmt.Fprintln(os.Stderr, "proof-run coverage-begin: no admitted parent proof context")
		return 3
	}
	err := proofrun.BeginCoverage(proofrun.CoverageBeginOptions{ControlRoot: controlRoot, ExecutionRoot: *executionRoot,
		AttemptID: attemptID, BaselinePath: *baseline, ProducerClass: "full", ProducerPID: *producerPID, CallerPID: int64(os.Getppid())})
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run coverage-begin:", err)
		return 1
	}
	return 0
}

func runProofRunCoverageEligible(args []string) int {
	flags := flag.NewFlagSet("proof-run coverage-eligible", flag.ContinueOnError)
	executionRoot := pathFlag(flags, "root", "", "executing full-gate root")
	baseline := flags.String("baseline", "", "selected coverage ratchet")
	producerPID := flags.Int64("producer-pid", 0, "full-gate producer process")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *executionRoot == "" || *baseline == "" || *producerPID < 1 {
		return 2
	}
	controlRoot, attemptID := os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT"), os.Getenv("METASYSTEM_PROOF_ATTEMPT")
	if controlRoot == "" || attemptID == "" {
		fmt.Fprintln(os.Stderr, "proof-run coverage-eligible: no admitted parent proof context")
		return 1
	}
	eligible, err := proofrun.CoverageProducerEligible(proofrun.CoverageBeginOptions{ControlRoot: controlRoot,
		ExecutionRoot: *executionRoot, AttemptID: attemptID, BaselinePath: *baseline,
		ProducerClass: "full", ProducerPID: *producerPID, CallerPID: int64(os.Getppid())})
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run coverage-eligible:", err)
		return 1
	}
	if !eligible {
		return 3
	}
	return 0
}

func runProofRunCoverageComplete(args []string) int {
	flags := flag.NewFlagSet("proof-run coverage-complete", flag.ContinueOnError)
	executionRoot := pathFlag(flags, "root", "", "executing full-gate root")
	baseline := flags.String("baseline", "", "selected coverage ratchet")
	coverageLog := flags.String("input", "", "actual go test coverage log")
	packages := flags.String("packages", "", "independent package inventory")
	module := flags.String("module", "github.com/widoriezebos/agentic-tools/metasystem/", "module prefix")
	producerPID := flags.Int64("producer-pid", 0, "full-gate producer process")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *executionRoot == "" || *baseline == "" || *coverageLog == "" || *packages == "" || *producerPID < 1 {
		return 2
	}
	controlRoot, attemptID := os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT"), os.Getenv("METASYSTEM_PROOF_ATTEMPT")
	if controlRoot == "" || attemptID == "" {
		fmt.Fprintln(os.Stderr, "proof-run coverage-complete: no admitted parent proof context")
		return 3
	}
	evidence, err := proofrun.CompleteCoverage(proofrun.CoverageCompleteOptions{CoverageBeginOptions: proofrun.CoverageBeginOptions{
		ControlRoot: controlRoot, ExecutionRoot: *executionRoot, AttemptID: attemptID, BaselinePath: *baseline,
		ProducerClass: "full", ProducerPID: *producerPID, CallerPID: int64(os.Getppid())}, CoverageLog: *coverageLog,
		PackageInventory: *packages, ModulePrefix: *module})
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run coverage-complete:", err)
		return 1
	}
	encoded, _ := json.Marshal(evidence)
	fmt.Println(string(encoded))
	return 0
}

func runProofRunCoverageReuse(args []string) int {
	flags := flag.NewFlagSet("proof-run coverage-reuse", flag.ContinueOnError)
	executionRoot := pathFlag(flags, "root", "", "source root whose coverage inputs are checked")
	controlRoot := flags.String("control-root", "", "canonical root retaining proof evidence")
	baseline := flags.String("baseline", "", "selected coverage ratchet")
	var packages repeatedFlag
	flags.Var(&packages, "package", "selected relative package (repeatable)")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *executionRoot == "" || *baseline == "" || len(packages) == 0 {
		return 2
	}
	canonicalExecution, err := canonicalProofRoot(*executionRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run coverage-reuse:", err)
		return 1
	}
	if *controlRoot == "" {
		for _, candidate := range []string{os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT"), os.Getenv("METASYSTEM_PROOF_RUN_ROOT"), os.Getenv("METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT")} {
			if candidate != "" {
				*controlRoot = candidate
				break
			}
		}
		if *controlRoot == "" {
			*controlRoot = canonicalExecution
		}
	}
	canonicalControl, err := canonicalProofRoot(*controlRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run coverage-reuse:", err)
		return 1
	}
	evidence, found, err := proofrun.ReusableCoverage(canonicalControl, canonicalExecution, *baseline, packages)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run coverage-reuse:", err)
		return 1
	}
	if !found {
		return 3
	}
	for _, pkg := range packages {
		fmt.Printf("coverage reuse: ./%s: %.1f%%\n", pkg, evidence.Measurements[pkg])
	}
	return 0
}

func runProofRunBanner(args []string) int {
	flags := flag.NewFlagSet("proof-run banner", flag.ContinueOnError)
	suite := flags.String("suite", "", "suite name")
	root := pathFlag(flags, "root", "", "metasystem root")
	progress := flags.String("progress", "", "progress JSONL path")
	logPath := flags.String("log", "", "suite log path")
	if flags.Parse(args) != nil {
		return 2
	}
	if *suite == "" || *root == "" || *progress == "" || *logPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem proof-run banner --suite S --root R --progress P --log L")
		return 2
	}
	state := proofRunWitnessState(*root)
	duration := "minutes"
	if state == "unarmed" {
		duration = "full-gate"
	}
	fmt.Printf("suite-cost suite=%s witness=%s duration=%s heartbeat=%s logs=%s\n",
		*suite, state, duration, proofRunDisplayPath(*root, *progress), proofRunDisplayPath(*root, *logPath))
	return 0
}

func runProofRunHeartbeat(args []string) int {
	flags := flag.NewFlagSet("proof-run heartbeat", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "watched root")
	if flags.Parse(args) != nil || *root == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem proof-run heartbeat --root R")
		return 2
	}
	heartbeat, ok := deepestSuiteHeartbeat(*root, time.Now())
	if !ok {
		return 1
	}
	fmt.Println(heartbeat)
	return 0
}

func runProofRunFixtureSelection(args []string) int {
	flags := flag.NewFlagSet("proof-run fixture-selection", flag.ContinueOnError)
	family := flags.String("family", "", "fixture family")
	selection := flags.String("selection", "", "all or comparison")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *family == "" || *selection == "" {
		fmt.Fprintln(os.Stderr, "proof-run fixture-selection requires --family and --selection")
		return 2
	}
	scenarios, err := proofrun.FixtureScenarios(*family, *selection)
	if err != nil {
		return recordExit(err)
	}
	for _, scenario := range scenarios {
		fmt.Println(scenario)
	}
	return 0
}

func deepestSuiteHeartbeat(root string, now time.Time) (string, bool) {
	progress := filepath.Join(root, "artifacts", "agents", "supervision", "suite-progress.jsonl")
	run, err := proofrun.ReadLatestProgressRun(progress)
	if err != nil {
		return "", false
	}
	return proofrun.DeepestLiveHeartbeat(run, now)
}

func startSuiteProgressPrinter(root string, interval time.Duration, output io.Writer) func() {
	if root == "" || output == nil {
		return func() {}
	}
	if interval <= 0 {
		interval = 2 * time.Second
	}
	last := ""
	printChanged := func() {
		heartbeat, ok := deepestSuiteHeartbeat(root, time.Now())
		if ok && heartbeat != last {
			fmt.Fprintln(output, heartbeat)
			last = heartbeat
		}
	}
	printChanged()
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				printChanged()
			case <-stop:
				return
			}
		}
	}()
	return func() {
		close(stop)
		<-done
	}
}

func proofRunDisplayPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	return path
}

func proofRunWitnessState(root string) string {
	if os.Getenv("METASYSTEM_GATE_WITNESS") != "" {
		if proofRunWitnessUsable(root) {
			if export := os.Getenv("METASYSTEM_GATE_WITNESS_EXPORT"); export != "" {
				if info, err := os.Stat(export); err == nil && info.IsDir() {
					return "frozen"
				}
				return "unarmed"
			}
			return "armed"
		}
		return "unarmed"
	}
	if proofRunFrozenWillRun(root) {
		return "frozen"
	}
	return "unarmed"
}

func proofRunWitnessUsable(root string) bool {
	script := filepath.Join(root, "scripts", "agents", "go-gate.sh")
	command := exec.Command("bash", script, "--witness-check-only")
	command.Dir = root
	command.Env = proofRunEnvironment("METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE", "ENGINE")
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	return command.Run() == nil
}

func proofRunEnvironment(name, value string) []string {
	prefix := name + "="
	environment := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, prefix) {
			environment = append(environment, entry)
		}
	}
	return append(environment, prefix+value)
}

func proofRunFrozenWillRun(root string) bool {
	if os.Getenv("METASYSTEM_COVERAGE_RATCHET_SEED") == "1" ||
		os.Getenv("METASYSTEM_GATE_FORCE") == "1" ||
		os.Getenv("METASYSTEM_DELIVERY_CONTRACT") == "1" || proofRunAlternateGoInputs() {
		return false
	}
	dirty, proved := proofRunEngineDirty(root)
	return proved && dirty
}

func proofRunAlternateGoInputs() bool {
	fields := strings.Fields(os.Getenv("GOFLAGS"))
	for _, field := range fields {
		if field == "-modfile" || strings.HasPrefix(field, "-modfile=") ||
			field == "-overlay" || strings.HasPrefix(field, "-overlay=") {
			return true
		}
	}
	return false
}

func proofRunEngineDirty(root string) (bool, bool) {
	prefixBytes, err := exec.Command("git", "-C", root, "rev-parse", "--show-prefix").Output()
	if err != nil {
		return false, false
	}
	gitRootBytes, err := exec.Command("git", "-C", root, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return false, false
	}
	policy, err := behaviorsurface.Load()
	if err != nil {
		return false, false
	}
	prefix := strings.TrimSpace(string(prefixBytes))
	gitRoot := strings.TrimSpace(string(gitRootBytes))
	commands := [][]string{
		{"diff", "--no-renames", "--name-only", "-z", "HEAD", "--"},
		{"ls-files", "--others", "--exclude-standard", "--full-name", "-z"},
		{"ls-files", "--others", "-i", "--exclude-standard", "--full-name", "-z"},
	}
	for _, arguments := range commands {
		output, err := exec.Command("git", append([]string{"-C", gitRoot}, arguments...)...).Output()
		if err != nil {
			return false, false
		}
		for _, path := range bytes.Split(output, []byte{0}) {
			if len(path) == 0 {
				continue
			}
			included, err := policy.Includes(behaviorsurface.Engine, string(path), prefix)
			if err != nil {
				return false, false
			}
			if included {
				return true, true
			}
		}
	}
	return false, true
}
