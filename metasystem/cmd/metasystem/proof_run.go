package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	runpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

func runProofRunLaunch(args []string) int {
	flags := flag.NewFlagSet("proof-run launch", flag.ContinueOnError)
	suite := flags.String("suite", "", "suite name")
	root := flags.String("root", "", "metasystem root")
	controlRootFlag := flags.String("control-root", "", "canonical proof control root")
	goalID := flags.String("goal", "", "accepted goal owning the proof reservation")
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
		status := proofrun.LaunchSuite(proofrun.LaunchOptions{Suite: *suite, Root: executionRoot, ControlRoot: controlRoot,
			ConfPath: *conf, ProgressPath: *progress, LogPath: *logPath, TmpPaths: tmpPaths, Banner: *banner,
			ExpectedSections: expected, TwiceConsulted: repeated, Silence: limits.silence, SectionCap: limits.sectionCap,
			EvidenceTimeout: limits.evidenceTimeout, EvidenceMax: limits.evidenceMax, Poll: time.Duration(*pollMS) * time.Millisecond,
			TermGrace: time.Duration(*termGraceMS) * time.Millisecond, KillGrace: time.Duration(*killGraceMS) * time.Millisecond,
			Command: command, Output: os.Stdout, ErrorOutput: os.Stderr})
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
		ControlRoot: controlRoot, ExecutionRoot: executionRoot, ConfPath: *conf, GoalID: *goalID,
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
		if err := proofrun.EncodeResult(os.Stderr, *resultPath, decision); err != nil {
			fmt.Fprintln(os.Stderr, "proof-run launch: publish launch result:", err)
			return 1
		}
		return decision.ExitStatus
	}
	deadline, _ := time.Parse(time.RFC3339Nano, attempt.Deadline)
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
		Command:   command, Output: os.Stdout, ErrorOutput: os.Stderr,
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

func runProofRunWorkerAuthorized(args []string) int {
	flags := flag.NewFlagSet("proof-run worker-authorized", flag.ContinueOnError)
	executionRoot := flags.String("root", "", "suite execution root")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *executionRoot == "" {
		return 2
	}
	if _, err := canonicalProofRoot(*executionRoot); err != nil {
		fmt.Fprintln(os.Stderr, "proof-run worker-authorized:", err)
		return 2
	}
	controlRoot := os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT")
	if controlRoot == "" {
		fmt.Fprintln(os.Stderr, "proof-run worker-authorized: no proof control root")
		return 3
	}
	canonicalControl, err := canonicalProofRoot(controlRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run worker-authorized:", err)
		return 3
	}
	err = proofrun.AuthenticateWorker(canonicalControl, os.Getenv("METASYSTEM_PROOF_ATTEMPT"),
		os.Getenv("METASYSTEM_PROOF_RECORD_KEY"), os.Getenv("METASYSTEM_PROOF_CREATION_CLAIM"), int64(os.Getppid()))
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run worker-authorized:", err)
		return 3
	}
	return 0
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
	ControlRoot, ExecutionRoot, ConfPath, GoalID, CapMin, RetryDecision string
	ScopeClass, CommandClass                                            string
	Sections                                                            []string
	IdentityInputs                                                      []string
	Environment                                                         []string
	SharedEngine                                                        string
	SharedManifestDigest                                                string
	ComponentIdentities                                                 map[string]string
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

func admitProofLaunch(request proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
	classifiedCaller, err := classifyVerbCaller(request.ControlRoot, int64(os.Getppid()))
	if err != nil {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof caller classification failed: %w", err)
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
		if request.GoalID != "" && request.GoalID != attempt.GoalID {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof parent goal %s does not match requested goal %s", attempt.GoalID, request.GoalID)
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
			componentRequest := proofrun.AdmissionRequest{ControlRoot: request.ControlRoot, GoalID: attempt.GoalID,
				GoalRevision: attempt.GoalRevision, AccountingRevision: attempt.AccountingRevision,
				RetryDecisionPath: request.RetryDecision, ComponentIdentities: request.ComponentIdentities}
			decision, decided, decisionErr := proofrun.JoinedComponentDecisionLocked(componentRequest)
			if decisionErr != nil {
				return proofrun.Attempt{}, proofrun.LaunchResult{}, false, decisionErr
			}
			if decided {
				return proofrun.Attempt{}, decision, false, nil
			}
			attempt, lockErr = proofrun.BindJoinedTestComponentsLocked(request.ControlRoot, attempt.AttemptID, request.ComponentIdentities)
			if lockErr != nil {
				return proofrun.Attempt{}, proofrun.LaunchResult{}, false, lockErr
			}
		}
		return attempt, proofrun.LaunchResult{SchemaVersion: 1, Disposition: proofrun.DispositionExecuted, AttemptID: attempt.AttemptID}, true, nil
	}
	var reservationOwner *proofrun.ReservationOwner
	delegateRevision := uint64(0)
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
		if request.GoalID != "" && request.GoalID != record.GoalId {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("governed run goal %s does not match requested goal %s", record.GoalId, request.GoalID)
		}
		request.GoalID = record.GoalId
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
		if request.GoalID != "" && request.GoalID != resolvedGoal {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("native delegate goal %s does not match requested goal %s", resolvedGoal, request.GoalID)
		}
		request.GoalID, delegateRevision = resolvedGoal, resolvedRevision
	}
	if request.GoalID == "" && classifiedCaller.Class == lease.ClassMain && classifiedCaller.Holder {
		request.GoalID, err = uniqueActiveProofGoal(request.ControlRoot, time.Now().UTC())
		if err != nil {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
		}
	}
	if request.GoalID == "" {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("top-level proof launch requires --goal")
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
	now, err := goalCommandNow(request.ControlRoot)
	if err != nil {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
	}
	binding, err := dispatchcore.ResolveGoalBinding(request.ControlRoot, request.GoalID, now)
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
	heldGoal, err := goalrevision.Acquire(request.ControlRoot, request.GoalID, binding.Revision, "proof-admission")
	if err != nil {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
	}
	defer heldGoal.Release()
	heldProof, err := proofrun.AcquireMutation(request.ControlRoot)
	if err != nil {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
	}
	defer heldProof.Release()
	classifiedCaller, err = classifyVerbCaller(request.ControlRoot, int64(os.Getppid()))
	if err != nil {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof caller reclassification failed under admission lock: %w", err)
	}
	if reservationOwner != nil {
		runRecord, readErr := (&runpkg.Store{Root: request.ControlRoot}).Read(reservationOwner.RunID)
		if readErr != nil || runRecord == nil || runRecord.Governed == nil || runRecord.Status != runpkg.StatusRunning ||
			runRecord.Pid == nil || runRecord.PidStartedAt == nil || runRecord.Generation != reservationOwner.RunGeneration ||
			runRecord.LaunchNonce != reservationOwner.LaunchNonce || runRecord.GoalId != request.GoalID ||
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
		if verifyErr != nil || !verified.Delegate || verified.JobID != delegateJob || readErr != nil ||
			lens.GoalID() != request.GoalID || !revisionOK || revision != binding.Revision {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("native delegate proof custody changed before reservation")
		}
	} else if classifiedCaller.Class == lease.ClassMain {
		machine, machineErr := goal.ResolveMachine(request.ControlRoot)
		if machineErr != nil || !classifiedCaller.Holder || classifiedCaller.ClaimEpoch == nil ||
			*classifiedCaller.ClaimEpoch != binding.Capability.ClaimEpoch || machine != binding.Machine {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("active coordinator does not own the claimed goal reservation")
		}
	} else if classifiedCaller.Class != lease.ClassHuman {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof caller has no authenticated goal reservation context")
	}
	binding, err = dispatchcore.ResolveGoalBinding(request.ControlRoot, request.GoalID, now)
	if err != nil || binding.Fence != nil || reservationOwner != nil && binding.Revision != reservationOwner.GoalRevision {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof reservation lost its accepted goal authority")
	}
	projection := dispatchcore.ProjectBudget(request.ControlRoot, binding.File, now)
	if projection.Status != dispatchcore.BudgetKnown {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof reservation budget projection is unknown")
	}
	accountingRevision := binding.File.Claimed.AccountingRevision
	if accountingRevision == 0 {
		accountingRevision = binding.Revision
	}
	checkoutFence, fenceErr := stopfence.Read(request.ControlRoot)
	if fenceErr != nil || checkoutFence.State == stopfence.StateClosed {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof reservation lost checkout-fence authority")
	}
	reservation := proofrun.AdmissionRequest{
		ControlRoot: request.ControlRoot, ExecutionRoot: request.ExecutionRoot, GoalID: request.GoalID,
		GoalRevision: binding.Revision, AccountingRevision: accountingRevision, BudgetEpoch: projection.WeightEpoch,
		ReservedMinutes: uint64(capValue), Identity: proofIdentity, Launcher: launcher, ReservationOwner: reservationOwner,
		RetryDecisionPath: request.RetryDecision, Now: now,
		ComponentIdentities: request.ComponentIdentities,
	}
	decision, noChild, err := proofrun.NoChildDecisionLocked(reservation)
	if err != nil {
		return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
	}
	if noChild {
		return proofrun.Attempt{}, decision, false, nil
	}
	if reservationOwner == nil {
		verdict, err := dispatchcore.EvaluateGoalRevisionAdmissionForDispatch(request.ControlRoot, request.GoalID, binding.Revision,
			uint64(capValue), now, "implementer", "fresh", dispatchcore.HazardMechanical)
		if err != nil {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, err
		}
		if verdict.Refused() {
			return proofrun.Attempt{}, proofrun.LaunchResult{}, false, fmt.Errorf("proof reservation refused for goal %s revision %d", request.GoalID, binding.Revision)
		}
	}
	attempt, decision, err := proofrun.ReserveLocked(reservation)
	return attempt, decision, false, err
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
	attempt, err := proofrun.ReadAttempt(completion.ControlRoot, completion.AttemptID)
	if err != nil {
		return err
	}
	current, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		return err
	}
	transition, err := stopfence.Acquire(completion.ControlRoot, "proof-finalize", current.Ref(), 1000)
	if err != nil {
		return err
	}
	defer transition.Release()
	heldGoal, err := goalrevision.Acquire(completion.ControlRoot, attempt.GoalID, attempt.GoalRevision, "proof-finalize")
	if err != nil {
		return err
	}
	defer heldGoal.Release()
	heldProof, err := proofrun.AcquireMutation(completion.ControlRoot)
	if err != nil {
		return err
	}
	defer heldProof.Release()
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
		completion.ExitStatus, "proof launcher completed", receipt, testResult, finalizedAt)
	return err
}

type proofRunLimits struct {
	silence         time.Duration
	sectionCap      time.Duration
	evidenceTimeout time.Duration
	evidenceMax     int64
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
	return proofRunLimits{
		silence:         time.Duration(silence) * time.Minute,
		sectionCap:      time.Duration(section) * time.Minute,
		evidenceTimeout: time.Duration(evidenceTimeout) * time.Second,
		evidenceMax:     int64(evidenceMB) * 1024 * 1024,
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
	root := flags.String("root", "", "metasystem root")
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
	var logs repeatedFlag
	flags.Var(&logs, "log", "watched output log (repeatable)")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *conf == "" {
		return 2
	}
	// Re-read the layered configuration in the sibling process immediately
	// before watchdog startup. A conf.local or environment change between
	// launcher resolution and spawn can therefore only refuse, never install
	// an unlawful operational window. Fixture duration overrides remain the
	// passed values after this effective-value validation.
	if _, err := resolveProofRunLimits(*conf); err != nil {
		fmt.Fprintln(os.Stderr, "proof-run watchdog:", err)
		return 1
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
	err = proofrun.RunWatchdog(proofrun.WatchdogOptions{
		Suite: *suite, Root: *root, ProgressPath: *progress, DonePath: *done, LogPaths: logs,
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
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "suite watchdog:", err)
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
	executionRoot := flags.String("root", "", "executing full-gate root")
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
	executionRoot := flags.String("root", "", "executing full-gate root")
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
	executionRoot := flags.String("root", "", "executing full-gate root")
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
	executionRoot := flags.String("root", "", "source root whose coverage inputs are checked")
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
	root := flags.String("root", "", "metasystem root")
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
	root := flags.String("root", "", "watched root")
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
