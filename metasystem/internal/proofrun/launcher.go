package proofrun

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

type CreationClaim interface {
	Path() string
	Close() error
}

type LaunchOptions struct {
	Suite              string
	Root               string
	ControlRoot        string
	AttemptID          string
	JoinedAttempt      bool
	Deadline           time.Time
	ConfPath           string
	ProgressPath       string
	LogPath            string
	TmpPaths           []string
	Banner             string
	ExpectedSections   []string
	TwiceConsulted     map[string]bool
	Silence            time.Duration
	SectionCap         time.Duration
	EvidenceTimeout    time.Duration
	EvidenceMax        int64
	Poll               time.Duration
	TermGrace          time.Duration
	KillGrace          time.Duration
	WatchdogExecutable string
	Command            []string
	Environment        []string
	HostResourceFiles  []*os.File
	// ScratchWriter is the run's scratch writer lock; the suite and its
	// custodian inherit it after the host resource files.
	ScratchWriter *os.File
	// RequireCustody anchors a public launch even when it borrows a legacy
	// parent's capacity without inheritable resource descriptors.
	RequireCustody    bool
	Output            io.Writer
	ErrorOutput       io.Writer
	FenceReader       func(string) (stopfence.Record, error)
	ClaimCreator      func(string, string, int64, identity.Ref) (CreationClaim, error)
	Prober            identity.Prober
	Signal            func(int, syscall.Signal) error
	PrepareSuccess    func(CompletionContext) (json.RawMessage, error)
	CommitTerminal    func(CompletionContext, json.RawMessage) error
	HintTerminal      func(root, attemptID, publicationID, bootID string, bootNanos int64)
	BeforeProcessDone func(CompletionContext) error
	Now               func() time.Time
}

var proofPublicationBootClock = identity.BootClock

type CompletionContext struct {
	ControlRoot   string
	ExecutionRoot string
	AttemptID     string
	RecordKey     string
	ExitStatus    int
	CompletedAt   time.Time
	InputIdentity string
	// ErrorOutput is the launcher's error stream, teed into launcher.log,
	// where the terminal commit notes what it waited behind.
	ErrorOutput io.Writer
}

func LaunchSuite(options LaunchOptions) int {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	if options.Output == nil {
		options.Output = io.Discard
	}
	if options.ErrorOutput == nil {
		options.ErrorOutput = io.Discard
	}
	if err := validateLaunchOptions(options); err != nil {
		fmt.Fprintln(options.ErrorOutput, "suite launcher:", err)
		return 2
	}
	controlRoot := options.ControlRoot
	if controlRoot == "" {
		controlRoot = options.Root
	}
	childEnvironment := proofChildEnvironment(options.Environment)
	readFence := options.FenceReader
	if readFence == nil {
		readFence = stopfence.Read
	}
	fence, err := readFence(controlRoot)
	if err != nil {
		fmt.Fprintln(options.ErrorOutput, "suite launcher: read stop fence:", err)
		return 1
	}
	if fence.State == stopfence.StateClosed {
		printStoppedRefusal(options.ErrorOutput, fenceCheckout(fence, controlRoot), fence)
		return 1
	}
	prober := options.Prober
	if prober == nil {
		prober = identity.KernelProber{}
	}
	launcherExact, state, err := prober.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		fmt.Fprintf(options.ErrorOutput, "suite launcher: cannot record exact launcher identity: %v (%s)\n", err, state)
		return 1
	}
	createClaim := options.ClaimCreator
	if createClaim == nil {
		createClaim = func(root, verb string, generation int64, ref identity.Ref) (CreationClaim, error) {
			return stopfence.Creating(root, verb, generation, ref)
		}
	}
	claim, err := createClaim(controlRoot, "proof-run-launch", fence.Generation, launcherExact.Ref())
	if err != nil {
		fmt.Fprintln(options.ErrorOutput, "suite launcher: open creation claim:", err)
		return 1
	}
	claimClosed := false
	defer func() {
		if !claimClosed {
			_ = claim.Close()
		}
	}()
	if err := os.MkdirAll(filepath.Dir(options.LogPath), 0o700); err != nil {
		fmt.Fprintln(options.ErrorOutput, "suite launcher:", err)
		return 1
	}
	log, err := os.OpenFile(options.LogPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintln(options.ErrorOutput, "suite launcher:", err)
		return 1
	}
	defer log.Close()
	combined := &lockedWriter{writers: []io.Writer{options.Output, log}}
	combinedErr := &lockedWriter{writers: []io.Writer{options.ErrorOutput, log}}
	resourceFiles := len(options.HostResourceFiles) != 0
	managedResources := resourceFiles || options.RequireCustody
	var spools *custodySpools
	var suiteSpools *custodySpools
	logPaths := []string{options.LogPath}
	if managedResources {
		spools, err = newCustodySpools(options.LogPath, "watchdog")
		if err != nil {
			fmt.Fprintln(combinedErr, "suite launcher: create durable watchdog output:", err)
			return 1
		}
		defer spools.close()
		logPaths = append(logPaths, spools.stdout.Name(), spools.stderr.Name())
	}
	suiteSpools, err = newCustodySpools(options.LogPath, "suite")
	if err != nil {
		fmt.Fprintln(combinedErr, "suite launcher: create durable suite output:", err)
		return 1
	}
	defer func() { _ = suiteSpools.finish() }()
	logPaths = append(logPaths, suiteSpools.stdout.Name(), suiteSpools.stderr.Name())
	if err := AppendProgressHeader(options.ProgressPath, ProgressHeader{TmpPaths: options.TmpPaths, LogPaths: logPaths}); err != nil {
		fmt.Fprintln(combinedErr, "suite launcher:", err)
		return 1
	}
	fmt.Fprintln(combined, options.Banner)
	var launchMutation *MutationLock
	launchInputIdentity := ""
	testingEngine := options.Command[0]
	releaseLaunchMutation := func() {
		if launchMutation != nil {
			_ = launchMutation.Release()
			launchMutation = nil
		}
	}
	defer releaseLaunchMutation()
	cardGoal := ""
	if options.AttemptID != "" {
		launchMutation, err = AcquireMutation(controlRoot)
		if err != nil {
			fmt.Fprintln(combinedErr, "suite launcher: acquire child-publication ownership:", err)
			return 1
		}
		if err := attemptLaunchAllowedLocked(controlRoot, options.AttemptID, launcherExact.Ref(), now().UTC()); err != nil {
			fmt.Fprintln(combinedErr, "suite launcher:", err)
			return 1
		}
		attempt, readErr := ReadAttempt(controlRoot, options.AttemptID)
		if readErr != nil {
			fmt.Fprintln(combinedErr, "suite launcher: read proof identity before launch:", readErr)
			return 1
		}
		if attempt.ProofIdentity.CommandClass == "testing" {
			// Admission binds the running shared engine. The selected trusted
			// worker may be a different executable and is bound separately by
			// the attempt's identity inputs.
			testingEngine, err = os.Executable()
			if err != nil {
				fmt.Fprintln(combinedErr, "suite launcher: locate testing engine:", err)
				return 1
			}
		}
		launchInputIdentity = attempt.ProofIdentity.IdentityDigest
		cardGoal = attempt.GoalID
		if options.JoinedAttempt {
			var context ExecutionContext
			var identityErr error
			if attempt.ProofIdentity.CommandClass == "testing" {
				// Joined suites may execute env, Bash, or an application command.
				// The running launcher is the engine owning their proof identity.
				context, identityErr = CaptureSharedExecutionContext(options.Root, options.ConfPath, childEnvironment, testingEngine, attempt.ProofIdentity.ManifestDigest)
			} else {
				context, identityErr = CaptureExecutionContext(options.Root, options.ConfPath, childEnvironment)
			}
			if identityErr != nil {
				fmt.Fprintln(combinedErr, "suite launcher: read joined proof inputs before launch:", identityErr)
				return 1
			}
			local := BuildProofIdentityForContext(context, attempt.ProofIdentity.ScopeClass,
				attempt.ProofIdentity.CommandClass, attempt.ProofIdentity.Sections, attempt.ProofIdentity.BehaviorPolicy)
			launchInputIdentity = BindIdentityInputs(local, attempt.ProofIdentity.IdentityInputs).IdentityDigest
		}
	}

	childEnvironment, err = identity.ExportRunOwner(childEnvironment)
	if err != nil {
		fmt.Fprintln(combinedErr, "suite launcher: export run owner:", err)
		return 1
	}
	suite := exec.Command(options.Command[0], options.Command[1:]...)
	suite.Dir = options.Root
	suite.ExtraFiles = append(suite.ExtraFiles, options.HostResourceFiles...)
	custodyFiles := options.HostResourceFiles
	if options.ScratchWriter != nil {
		suite.ExtraFiles = append(suite.ExtraFiles, options.ScratchWriter)
		custodyFiles = append(append([]*os.File{}, options.HostResourceFiles...), options.ScratchWriter)
	}
	suite.Env = append(childEnvironment,
		"METASYSTEM_PROOF_CONTROL_ROOT="+controlRoot,
		"METASYSTEM_PROOF_RECORD_KEY="+options.Suite,
		"METASYSTEM_PROOF_CREATION_CLAIM="+claim.Path(),
	)
	if executable, executableErr := os.Executable(); executableErr == nil {
		suite.Env = append(suite.Env, "METASYSTEM_PROOF_AUTH_BIN="+executable)
	}
	if options.AttemptID != "" {
		suite.Env = append(suite.Env,
			"METASYSTEM_PROOF_ATTEMPT="+options.AttemptID,
			identity.FixtureAttemptEnv+"="+options.AttemptID,
		)
	}
	if len(options.HostResourceFiles) != 0 {
		suite.Env = append(suite.Env, HostResourceFDEnvironment(options.HostResourceFiles))
	}
	donePath := options.LogPath + ".done"
	_ = os.Remove(donePath)
	var custody *resourceCustody
	var barrier *custodyBarrier
	if managedResources {
		if resourceFiles {
			if err := MarkHostResourcesDirty(options.HostResourceFiles); err != nil {
				fmt.Fprintln(combinedErr, "suite launcher: mark native custody active:", err)
				return 1
			}
		}
		custody, err = startResourceCustody(options, launcherExact.Ref(), donePath, custodyFiles, spools, nil)
		if err != nil {
			if resourceFiles {
				_ = MarkHostResourcesClean(options.HostResourceFiles)
			}
			fmt.Fprintln(combinedErr, "suite launcher: start resource custodian:", err)
			return 1
		}
		custody.spools.follow(combined, combinedErr, log)
		engine := options.WatchdogExecutable
		if engine == "" {
			engine, err = os.Executable()
		}
		if err == nil {
			barrier, err = prepareCustodyExec(suite, engine)
		}
		if err != nil {
			_ = custody.finish()
			fmt.Fprintln(combinedErr, "suite launcher: prepare resource start barrier:", err)
			return 1
		}
		defer barrier.close()
		suite.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: custody.group}
	} else {
		suite.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	suite.Stdout, suite.Stderr = suiteSpools.stdout, suiteSpools.stderr
	if err := suite.Start(); err != nil {
		if custody != nil {
			_ = custody.finish()
		}
		fmt.Fprintln(combinedErr, "suite launcher:", err)
		return 1
	}
	// The child has its own regular-file descriptors; the parent can follow
	// live output without keeping a write descriptor open.
	suiteSpools.close()
	suiteSpools.follow(combined, combinedErr, log)
	if barrier != nil {
		for _, file := range suite.ExtraFiles[len(suite.ExtraFiles)-2:] {
			_ = file.Close()
		}
		if err := barrier.await(nil); err != nil {
			_ = suite.Process.Kill()
			_ = suite.Wait()
			_ = custody.finish()
			fmt.Fprintln(combinedErr, "suite launcher: resource start barrier:", err)
			return 1
		}
	}
	suiteExact, state, probeErr := prober.Probe(int64(suite.Process.Pid))
	if probeErr != nil || state != identity.Alive {
		releaseLaunchMutation()
		_ = suite.Process.Kill()
		_ = suite.Wait()
		if custody != nil {
			_ = custody.finish()
		}
		fmt.Fprintf(combinedErr, "suite launcher: cannot record exact suite identity: %v (%s)\n", probeErr, state)
		return 1
	}
	if custody != nil {
		if err := custody.bind(suiteExact.Ref()); err != nil {
			_ = suite.Process.Kill()
			_ = suite.Wait()
			_ = custody.finish()
			fmt.Fprintln(combinedErr, "suite launcher: bind exact resource worker:", err)
			return 1
		}
	}

	var watchdog *exec.Cmd
	var watchdogOut, watchdogErr io.ReadCloser
	if custody != nil {
		watchdog = custody.command
	} else {
		watchdog = watchdogCommand(options, suiteExact.Ref(), donePath, fence.Generation)
		// The sibling watchdog belongs to this launch as much as the suite
		// does, so it carries the launch's environment, fixture ownership
		// included. It sits in its own process group; with the process's
		// ambient environment instead, a test that owns the launch could not
		// see or reap it after its launcher died (2026-09-28).
		watchdog.Env = childEnvironment
		watchdog.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		watchdogOut, err = watchdog.StdoutPipe()
	}
	if err != nil {
		releaseLaunchMutation()
		if custody != nil {
			_ = suite.Process.Kill()
		} else {
			_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
		}
		_ = suite.Wait()
		if custody != nil {
			_ = custody.finish()
		}
		fmt.Fprintln(combinedErr, "suite launcher:", err)
		return 1
	}
	if custody == nil {
		watchdogErr, err = watchdog.StderrPipe()
	}
	if err != nil {
		releaseLaunchMutation()
		if custody != nil {
			_ = suite.Process.Kill()
		} else {
			_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
		}
		_ = suite.Wait()
		if custody != nil {
			_ = custody.finish()
		}
		fmt.Fprintln(combinedErr, "suite launcher:", err)
		return 1
	}
	if custody == nil {
		err = watchdog.Start()
	}
	if err != nil {
		releaseLaunchMutation()
		if custody != nil {
			_ = suite.Process.Kill()
		} else {
			_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
		}
		_ = suite.Wait()
		if custody != nil {
			_ = custody.finish()
		}
		fmt.Fprintln(combinedErr, "suite launcher: start sibling watchdog:", err)
		return 1
	}
	watchdogExact, watchdogState, watchdogProbeErr := prober.Probe(int64(watchdog.Process.Pid))
	if watchdogProbeErr != nil || watchdogState != identity.Alive {
		releaseLaunchMutation()
		if custody == nil {
			_ = watchdog.Process.Kill()
			_ = watchdog.Wait()
		}
		if custody != nil {
			_ = suite.Process.Kill()
		} else {
			_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
		}
		_ = suite.Wait()
		if custody != nil {
			_ = custody.finish()
		}
		fmt.Fprintf(combinedErr, "suite launcher: cannot record exact watchdog identity: %v (%s)\n", watchdogProbeErr, watchdogState)
		return 1
	}
	// The watchdog's streams have their own group: its verdict is the last
	// thing it writes, and exec.Cmd.Wait closes the pipes when the process
	// exits, so waiting for the process first lost the verdict one launch in
	// five (2026-09-12, the suite-progress fixture's chatty scenario). The
	// watchdog's own children (evidence preservation, supervision shutdown)
	// end before it returns, so draining its pipes before Wait cannot hang.
	// Suite output uses durable files and an independently bounded final
	// drain because detached fixture children may retain inherited streams.
	var watchdogCopies sync.WaitGroup
	if custody == nil {
		watchdogCopies.Add(2)
		go copyStream(&watchdogCopies, combined, watchdogOut, false)
		go copyStream(&watchdogCopies, combinedErr, watchdogErr, false)
	}

	launcherPgid, _ := syscall.Getpgid(os.Getpid())
	record := Record{
		Suite: options.Suite, Root: options.Root, FenceGeneration: fence.Generation,
		Launcher:     processIdentity(launcherExact, int64(launcherPgid)),
		SuiteProcess: processIdentity(suiteExact, int64(suite.Process.Pid)),
		Watchdog:     processIdentity(watchdogExact, int64(watchdog.Process.Pid)),
		Status:       StatusRunning,
	}
	if custody != nil {
		record.SuiteProcess.Pgid = int64(custody.group)
	}
	if options.AttemptID != "" {
		launchID, launchErr := newLaunchID()
		if launchErr != nil {
			releaseLaunchMutation()
			if custody != nil {
				_ = suite.Process.Kill()
			} else {
				_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
			}
			_ = suite.Wait()
			if custody != nil {
				_ = custody.finish()
			} else {
				_ = watchdog.Process.Kill()
				_ = watchdog.Wait()
			}
			watchdogCopies.Wait()
			fmt.Fprintln(combinedErr, "suite launcher: create process launch identity:", launchErr)
			return 1
		}
		record.ControlRoot = controlRoot
		record.AttemptID = options.AttemptID
		record.LaunchID = launchID
	}
	if err := writeRecord(record); err != nil {
		releaseLaunchMutation()
		if custody != nil {
			_ = suite.Process.Kill()
		} else {
			_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
		}
		_ = suite.Wait()
		if custody != nil {
			_ = custody.finish()
		} else {
			_ = watchdog.Process.Kill()
			_ = watchdog.Wait()
		}
		watchdogCopies.Wait()
		fmt.Fprintln(combinedErr, "suite launcher: publish proof-run record:", err)
		return 1
	}
	if options.AttemptID != "" {
		if err := updateAttemptProcessesLocked(controlRoot, options.AttemptID, launcherExact.Ref(), []string{record.Key()}); err != nil {
			releaseLaunchMutation()
			if custody != nil {
				_ = suite.Process.Kill()
			} else {
				_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
			}
			_ = suite.Wait()
			if custody != nil {
				_ = custody.finish()
			} else {
				_ = watchdog.Process.Kill()
				_ = watchdog.Wait()
			}
			watchdogCopies.Wait()
			fmt.Fprintln(combinedErr, "suite launcher: publish proof attempt processes:", err)
			return 1
		}
		releaseLaunchMutation()
	}
	if barrier != nil {
		if err := barrier.releaseWork(); err != nil {
			_ = suite.Process.Kill()
			_ = suite.Wait()
			_ = custody.finish()
			fmt.Fprintln(combinedErr, "suite launcher: release resource worker:", err)
			return 1
		}
	}
	secondFence, secondFenceErr := readFence(controlRoot)
	stoppedDuringStart := secondFenceErr == nil && (secondFence.State == stopfence.StateClosed || secondFence.Generation != fence.Generation)
	var suiteDone <-chan struct{}
	var suiteWaitErr error
	if secondFenceErr != nil || stoppedDuringStart {
		done := make(chan struct{})
		suiteDone = done
		go func() {
			suiteWaitErr = suite.Wait()
			close(done)
		}()
		waitForSuite := func(duration time.Duration) {
			timer := time.NewTimer(duration)
			defer timer.Stop()
			select {
			case <-done:
			case <-timer.C:
			}
		}
		stopSuite := StopSuite
		if custody != nil {
			stopSuite = func(process ProcessIdentity, options StopOptions) StopOutcome {
				return StopRecordedIdentity("suite", process, options)
			}
		}
		outcome := stopSuite(record.SuiteProcess, StopOptions{
			TermGrace: options.TermGrace, KillGrace: options.KillGrace, Poll: options.Poll,
			Prober: prober, Signal: options.Signal, Sleep: waitForSuite,
		})
		if secondFenceErr != nil {
			fmt.Fprintln(combinedErr, "suite launcher: second stop-fence read:", secondFenceErr)
		}
		if outcome.Result == StopNotStopped {
			fmt.Fprintf(combinedErr, "suite launcher: proof-run suite was not stopped: %s\n", outcome.Reason)
			if err := claim.Close(); err != nil {
				fmt.Fprintln(combinedErr, "suite launcher: close creation claim:", err)
			} else {
				claimClosed = true
			}
			return 1
		}
	}
	closeAfterCleanup := secondFenceErr != nil || stoppedDuringStart
	if !closeAfterCleanup {
		if err := claim.Close(); err != nil {
			if custody != nil {
				_ = suite.Process.Kill()
			} else {
				_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
			}
			_ = suite.Wait()
			if custody != nil {
				_ = custody.finish()
			} else {
				_ = watchdog.Process.Kill()
				_ = watchdog.Wait()
			}
			watchdogCopies.Wait()
			fmt.Fprintln(combinedErr, "suite launcher: close creation claim:", err)
			return 1
		}
		claimClosed = true
	}

	cardTicker := time.NewTicker(proofCardPoll(options.Poll))
	stopCard := followProofCard(proofCardFollow{goal: cardGoal, attempt: options.AttemptID, suite: options.Suite,
		expected: options.ExpectedSections, progress: options.ProgressPath, owner: launcherExact, now: now}, cardTicker.C)
	if suiteDone == nil {
		suiteWaitErr = suite.Wait()
	} else {
		<-suiteDone
	}
	cardTicker.Stop()
	stopCard()
	beforeLauncherDone(donePath)
	doneErr := touchDone(donePath)
	if doneErr != nil {
		fmt.Fprintln(combinedErr, "suite launcher: write watchdog done file:", doneErr)
	}
	watchdogCopies.Wait()
	var watchdogErrWait error
	if custody != nil {
		watchdogErrWait = custody.finish()
	} else {
		watchdogErrWait = watchdog.Wait()
	}
	survivorFailure := false
	processes, processErr := census.EnumerateConfiguredProcesses(filepath.Dir(options.ConfPath))
	if processErr != nil {
		fmt.Fprintln(combinedErr, "suite launcher: fixture survivor table is unreadable:", processErr)
	} else {
		reapProber := prober
		signal := identity.SignalFunc(options.Signal)
		if os.Getenv("METASYSTEM_CENSUS_PROCESS_FILE") != "" {
			reapProber = census.FixtureProcessProber(processes)
			if signal == nil {
				signal = func(int, syscall.Signal) error { return nil }
			}
		} else if signal == nil {
			signal = syscall.Kill
		}
		survivorFailure = reapFixtureSurvivors(reapProber, processes, signal, fixtureAttemptOwnership{
			owner: launcherExact.Ref(), attemptID: options.AttemptID, startedAt: launcherExact.StartedAt,
		}, combinedErr)
	}
	// Custody and declared fixture cleanup have finished writing. Freeze one
	// finite final prefix before evaluating output or publishing the result.
	suiteSpoolErr := suiteSpools.finish()
	defer os.Remove(donePath)
	if closeAfterCleanup {
		if err := claim.Close(); err != nil {
			fmt.Fprintln(combinedErr, "suite launcher: close creation claim:", err)
			return 1
		}
		claimClosed = true
	}

	result := exitStatus(suiteWaitErr)
	if suiteSpoolErr != nil {
		fmt.Fprintln(log, "suite launcher: drain durable suite output:", suiteSpoolErr)
		result = 1
	}
	if outputErr := errors.Join(combined.Err(), combinedErr.Err()); outputErr != nil {
		fmt.Fprintln(log, "suite launcher: public output failed:", outputErr)
		result = 1
	}
	if doneErr != nil {
		result = 1
	}
	if survivorFailure {
		result = 1
	}
	if watchdogErrWait != nil {
		// The watchdog's own end is part of the record: a verdict it printed
		// ends in exit status 1; a watchdog that died inside its cleanup
		// ends in a signal, and its verdict never reached this output.
		fmt.Fprintln(combinedErr, "suite launcher: watchdog ended:", watchdogErrWait)
		result = 1
	}
	if err := assertBanner(options.LogPath, options.Banner); err != nil {
		fmt.Fprintln(combinedErr, "suite launcher:", err)
		result = 1
	}
	if len(options.ExpectedSections) > 0 {
		run, err := ReadLatestProgressRun(options.ProgressPath)
		if err == nil {
			err = AssertSectionProgress(run, options.Suite, options.ExpectedSections, options.TwiceConsulted)
		}
		if err != nil {
			fmt.Fprintln(combinedErr, "suite launcher:", err)
			result = 1
		}
	}
	if stoppedDuringStart {
		checkout := fenceCheckout(secondFence, controlRoot)
		if secondFence.State == stopfence.StateClosed {
			printStoppedDuringStartRefusal(combinedErr, checkout, secondFence)
		} else {
			printRearmedDuringStartRefusal(combinedErr, checkout, "the proof run")
		}
		result = 1
	}
	if secondFenceErr != nil {
		result = 1
	}
	parityIdentity := ""
	if options.AttemptID != "" && result == 0 {
		attempt, parityErr := ReadAttempt(controlRoot, options.AttemptID)
		if parityErr == nil {
			var context ExecutionContext
			if attempt.ProofIdentity.CommandClass == "testing" {
				context, parityErr = CaptureSharedExecutionContext(options.Root, options.ConfPath, childEnvironment, testingEngine, attempt.ProofIdentity.ManifestDigest)
			} else {
				context, parityErr = CaptureExecutionContext(options.Root, options.ConfPath, childEnvironment)
			}
			if parityErr == nil {
				current := BuildProofIdentityForContext(context, attempt.ProofIdentity.ScopeClass,
					attempt.ProofIdentity.CommandClass, attempt.ProofIdentity.Sections, attempt.ProofIdentity.BehaviorPolicy)
				current = BindIdentityInputs(current, attempt.ProofIdentity.IdentityInputs)
				parityIdentity = current.IdentityDigest
			}
			if parityErr == nil && parityIdentity != launchInputIdentity {
				parityErr = fmt.Errorf("relevant proof inputs changed during execution")
			}
		}
		if parityErr != nil {
			fmt.Fprintln(combinedErr, "suite launcher: proof input parity:", parityErr)
			result = 1
		}
	}
	completedAt := now().UTC()
	completion := CompletionContext{ControlRoot: controlRoot, ExecutionRoot: options.Root, AttemptID: options.AttemptID,
		RecordKey: record.Key(), ExitStatus: result, CompletedAt: completedAt, InputIdentity: parityIdentity, ErrorOutput: combinedErr}
	var receipt json.RawMessage
	if options.AttemptID != "" && result == 0 && options.PrepareSuccess != nil {
		receipt, err = options.PrepareSuccess(completion)
		if err != nil {
			fmt.Fprintln(combinedErr, "suite launcher: prepare successful proof delivery:", err)
			result = 1
			completion.ExitStatus = result
			completion.CompletedAt = now().UTC()
		}
		if err == nil {
			completion.CompletedAt = now().UTC()
		}
	}
	if options.AttemptID != "" && !options.JoinedAttempt {
		beganBootID, beganBootElapsed, beganBootErr := proofPublicationBootClock()
		if beganBootErr != nil {
			beganBootID, beganBootElapsed = "", 0
		}
		if options.CommitTerminal != nil {
			err = options.CommitTerminal(completion, receipt)
		} else {
			terminal := TerminalFailed
			if result == 0 {
				terminal = TerminalSuccess
			}
			_, err = FinalizeAttempt(controlRoot, options.AttemptID, terminal, result, "proof launcher completed", receipt, completion.CompletedAt)
		}
		if err != nil {
			fmt.Fprintln(combinedErr, "suite launcher: commit terminal proof result:", err)
			return 1
		}
		publicationID := ""
		if terminalAttempt, readErr := ReadAttempt(controlRoot, options.AttemptID); readErr == nil {
			publicationID = fmt.Sprintf("attempt:%s:%s", terminalAttempt.AttemptID, terminalAttempt.ProofIdentity.IdentityDigest)
		}
		hintTerminal := options.HintTerminal
		if hintTerminal == nil {
			hintTerminal = func(root, attemptID, publicationID, bootID string, bootNanos int64) {
				_, _ = metarun.NotifyWaiters(root, metarun.WaitHint{Kind: "attempt", TargetID: attemptID, PublicationID: publicationID, BeganBootID: bootID, BeganBootNanos: bootNanos})
			}
		}
		hintTerminal(controlRoot, options.AttemptID, publicationID, beganBootID, beganBootElapsed.Nanoseconds())
	}
	if options.BeforeProcessDone != nil {
		if err := options.BeforeProcessDone(completion); err != nil {
			fmt.Fprintln(combinedErr, "suite launcher: before process completion:", err)
			return 1
		}
	}
	if err := markDone(controlRoot, record, launcherExact.Ref()); err != nil {
		fmt.Fprintln(combinedErr, "suite launcher: mark proof-run done:", err)
		return 1
	}
	return result
}

// proofChildEnvironment removes authority locators owned by an enclosing
// launcher before the child receives this launcher's coherent context. In
// particular, a legacy launch must not pair its new control root with an
// inherited admitted-attempt identifier.
func proofChildEnvironment(environment []string) []string {
	owned := map[string]bool{
		"METASYSTEM_PROOF_CONTROL_ROOT":   true,
		"METASYSTEM_PROOF_ATTEMPT":        true,
		"METASYSTEM_PROOF_RECORD_KEY":     true,
		"METASYSTEM_PROOF_CREATION_CLAIM": true,
		"METASYSTEM_PROOF_AUTH_BIN":       true,
		"METASYSTEM_PROOF_RUN_ROOT":       true,
		"METASYSTEM_PROOF_RUN_ID":         true,
		identity.FixtureAttemptEnv:        true,
	}
	base := environment
	if len(base) == 0 {
		base = os.Environ()
	}
	result := make([]string, 0, len(base))
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if !owned[key] {
			result = append(result, entry)
		}
	}
	return result
}

func validateLaunchOptions(options LaunchOptions) error {
	if options.Suite == "" || options.Root == "" || options.ConfPath == "" || options.ProgressPath == "" || options.LogPath == "" || options.Banner == "" {
		return errors.New("suite, root, configuration, progress, log, and banner are required")
	}
	if options.Silence <= 0 || options.SectionCap <= 0 || options.EvidenceTimeout <= 0 || options.EvidenceMax < 1 {
		return errors.New("suite limits must be positive")
	}
	if options.Poll <= 0 || options.TermGrace <= 0 || options.KillGrace <= 0 {
		return errors.New("watchdog polling and signal grace periods must be positive")
	}
	if len(options.Command) == 0 {
		return errors.New("suite command is required")
	}
	if _, err := RecordPath(options.Root, options.Suite); err != nil {
		return err
	}
	if options.AttemptID != "" {
		controlRoot := options.ControlRoot
		if controlRoot == "" {
			controlRoot = options.Root
		}
		if _, err := AttemptPath(controlRoot, options.AttemptID); err != nil {
			return err
		}
		if options.Deadline.IsZero() {
			return errors.New("attempt-scoped suite requires an absolute deadline")
		}
	}
	return nil
}

func printStoppedRefusal(output io.Writer, checkout string, fence stopfence.Record) {
	description, descriptionErr := stopfence.ClosedDescription(fence, checkout)
	command, commandErr := stopfence.ClosedCommand(fence, checkout)
	if descriptionErr != nil || commandErr != nil {
		fmt.Fprintf(output, "suite launcher: cannot render stopped refusal: %v %v\n", descriptionErr, commandErr)
		return
	}
	fmt.Fprintln(output, description)
	fmt.Fprintln(output, "at an agent-free terminal, run: "+command)
}

func printStoppedDuringStartRefusal(output io.Writer, checkout string, fence stopfence.Record) {
	description, descriptionErr := stopfence.ClosedDescription(fence, checkout)
	command, commandErr := stopfence.ClosedCommand(fence, checkout)
	if descriptionErr != nil || commandErr != nil {
		fmt.Fprintf(output, "suite launcher: cannot render stopped refusal: %v %v\n", descriptionErr, commandErr)
		return
	}
	fmt.Fprintln(output, description+"; while the proof run started, it has been ended")
	fmt.Fprintln(output, "at an agent-free terminal, run: "+command)
}

func printRearmedDuringStartRefusal(output io.Writer, checkout, thing string) {
	fmt.Fprintf(output, "the checkout %s was stopped and armed again while %s started; %s has been ended; the caller may retry\n", checkout, thing, thing)
}

func fenceCheckout(record stopfence.Record, fallback string) string {
	if record.Checkout != "" {
		return record.Checkout
	}
	return fallback
}

type lockedWriter struct {
	mu       sync.Mutex
	writers  []io.Writer
	disabled []bool
	err      error
}

func (w *lockedWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.disabled == nil {
		w.disabled = make([]bool, len(w.writers))
	}
	var writeErr error
	for index, writer := range w.writers {
		if writer == nil || w.disabled[index] {
			continue
		}
		n, err := writer.Write(data)
		if err == nil && n != len(data) {
			err = io.ErrShortWrite
		}
		if err != nil {
			w.disabled[index] = true
			w.err = errors.Join(w.err, err)
		}
		writeErr = errors.Join(writeErr, err)
	}
	if writeErr != nil {
		return 0, writeErr
	}
	return len(data), nil
}

func (w *lockedWriter) Err() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}

func (w *lockedWriter) recordError(err error) {
	w.mu.Lock()
	w.err = errors.Join(w.err, err)
	w.mu.Unlock()
}

func copyStream(group *sync.WaitGroup, destination *lockedWriter, source io.Reader, allowClosedSource bool) {
	defer group.Done()
	buffer := make([]byte, 32*1024)
	for {
		n, readErr := source.Read(buffer)
		if n > 0 {
			// A failed public sink is disabled by lockedWriter. Keep draining
			// the pipe into the suite log so the child and custodian can end.
			_, _ = destination.Write(buffer[:n])
		}
		if readErr != nil {
			if readErr != io.EOF && !(allowClosedSource && errors.Is(readErr, os.ErrClosed)) {
				destination.recordError(readErr)
			}
			return
		}
	}
}

func watchdogCommand(options LaunchOptions, ref identity.Ref, donePath string, fenceGeneration int64) *exec.Cmd {
	executable := options.WatchdogExecutable
	if executable == "" {
		executable, _ = os.Executable()
	}
	args := []string{
		"proof-run", "watchdog",
		"--suite", options.Suite,
		"--root", options.Root,
		"--conf", options.ConfPath,
		"--progress", options.ProgressPath,
		"--done", donePath,
		"--suite-pid", strconv.FormatInt(ref.Pid, 10),
		"--suite-started-at", strconv.FormatInt(ref.StartedAtSec, 10),
		"--suite-start-ticks", strconv.FormatInt(ref.StartTicks, 10),
		"--suite-boot-id", ref.BootID,
		"--fence-generation", strconv.FormatInt(fenceGeneration, 10),
		"--silence-ms", strconv.FormatInt(options.Silence.Milliseconds(), 10),
		"--section-cap-ms", strconv.FormatInt(options.SectionCap.Milliseconds(), 10),
		"--evidence-timeout-ms", strconv.FormatInt(options.EvidenceTimeout.Milliseconds(), 10),
		"--evidence-max-bytes", strconv.FormatInt(options.EvidenceMax, 10),
		"--poll-ms", strconv.FormatInt(options.Poll.Milliseconds(), 10),
		"--term-grace-ms", strconv.FormatInt(options.TermGrace.Milliseconds(), 10),
		"--kill-grace-ms", strconv.FormatInt(options.KillGrace.Milliseconds(), 10),
	}
	if !options.Deadline.IsZero() {
		args = append(args, "--deadline", options.Deadline.UTC().Format(time.RFC3339Nano))
	}
	if options.ControlRoot != "" && options.AttemptID != "" {
		args = append(args, "--control-root", options.ControlRoot, "--attempt", options.AttemptID)
	}
	for _, path := range []string{options.LogPath} {
		args = append(args, "--log", path)
	}
	return exec.Command(executable, args...)
}

// beforeLauncherDone runs after the suite ended and before the launcher
// publishes its done marker: a test's point to place a marker the custodian
// published first, which only an ended suite may meet.
var beforeLauncherDone = func(string) {}

func touchDone(path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			// The launcher and custodian can each be first to publish the
			// completion signal. Only an existing regular marker is equivalent.
			if info, statErr := os.Lstat(path); statErr == nil && info.Mode().IsRegular() {
				return nil
			}
		}
		return err
	}
	return file.Close()
}

func exitStatus(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if code := exitErr.ExitCode(); code >= 0 {
			return code
		}
	}
	return 1
}

func assertBanner(path, banner string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("read cost banner log: %w", err)
	}
	defer file.Close()
	count := 0
	reader := bufio.NewReader(file)
	for {
		matches := true
		compared := 0
		for {
			fragment, more, readErr := reader.ReadLine()
			if readErr != nil {
				if errors.Is(readErr, io.EOF) {
					if count != 1 {
						return fmt.Errorf("cost banner appeared %d times in the suite log; expected exactly once", count)
					}
					return nil
				}
				return readErr
			}
			if matches {
				end := compared + len(fragment)
				if end > len(banner) || string(fragment) != banner[compared:end] {
					matches = false
				}
			}
			compared += len(fragment)
			if more {
				continue
			}
			if matches && compared == len(banner) {
				count++
			}
			break
		}
	}
}

// proofCardFollow is what the launcher knows about a goal-bound proof: the
// planned sections and the progress file the suite appends to.
type proofCardFollow struct {
	home, goal, attempt, suite string
	expected                   []string
	progress                   string
	owner                      identity.Exact
	now                        func() time.Time
}

// proofCardPoll is how often the launcher reads its progress file for the
// board card: the launch's poll, at least a second.
func proofCardPoll(poll time.Duration) time.Duration {
	return max(poll, time.Second)
}

// followProofCard advances the goal's live board card to unit-proof with
// this launcher as owner and, on every tick, with the number of distinct
// planned sections that have ended (D14, R24): a repeated end event counts
// once, so a stuck proof that re-ends one section leaves done, and the
// card's last progress stamp, where they are. Each tick's time stamps the
// write it causes. The returned stop writes the final count and leaves the
// claim idle; the card is a projection, and a card that cannot be written is
// reported while the proof runs on.
func followProofCard(follow proofCardFollow, tick <-chan time.Time) func() {
	follower, ok := newProofCardFollower(follow)
	if !ok {
		return func() {}
	}
	follower.write(board.StageUnitProof, follow.now())
	stop, finished := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		for {
			select {
			case <-stop:
				return
			case at := <-tick:
				follower.write(board.StageUnitProof, at)
			}
		}
	}()
	return func() {
		close(stop)
		<-finished
		at := follow.now()
		follower.write(board.StageUnitProof, at)
		follower.write(board.StageClaimedIdle, at)
	}
}

// proofCardFollower holds the live card a proof advances and the count it
// last wrote.
type proofCardFollower struct {
	follow proofCardFollow
	base   board.Card
	done   int
}

// newProofCardFollower finds the goal's live card; false when the proof is
// not goal-bound, has no plan, or its goal has no live card this launcher
// may advance (a joined unit's batch proof, a hand landing).
func newProofCardFollower(follow proofCardFollow) (*proofCardFollower, bool) {
	if follow.goal == "" || len(follow.expected) == 0 {
		return nil, false
	}
	if follow.home == "" {
		home, err := board.Home()
		if err != nil {
			return nil, false
		}
		follow.home = home
	}
	base, ok := board.LiveCard(follow.home, follow.goal)
	if !ok || base.Stage == board.StageJoined || base.Stage == board.StageLanding {
		return nil, false
	}
	return &proofCardFollower{follow: follow, base: base, done: -1}, true
}

// write publishes the card at stage; within unit-proof only a changed count
// is written.
func (f *proofCardFollower) write(stage board.Stage, at time.Time) {
	run, _ := ReadLatestProgressRun(f.follow.progress)
	ended := distinctEndedSections(run, f.follow.suite, f.follow.expected)
	if stage == board.StageUnitProof && ended == f.done {
		return
	}
	f.done = ended
	card := board.Card{Seat: f.base.Seat, Goal: f.follow.goal, Stage: stage, Round: f.base.Round, Job: f.base.Job,
		Proof:  &board.Proof{Attempt: f.follow.attempt, Done: ended, Planned: len(f.follow.expected)},
		Writer: board.Writer{Component: "proof-run", At: at}}
	if stage == board.StageUnitProof {
		card.Owner = &board.Owner{Pid: f.follow.owner.Pid, PidStartedAt: f.follow.owner.StartedAt.Unix()}
	}
	if err := board.WriteAt(f.follow.home, card); err != nil {
		fmt.Fprintf(os.Stderr, "suite launcher: the board card was not written: %v\n", err)
	}
}

// distinctEndedSections counts the planned sections of suite that have an
// end event, each once, clamped to the plan.
func distinctEndedSections(run ProgressRun, suite string, expected []string) int {
	planned := make(map[string]bool, len(expected))
	for _, section := range expected {
		planned[section] = true
	}
	ended := map[string]bool{}
	for _, event := range run.Events {
		if event.Suite == suite && event.Event == "end" && planned[event.Section] {
			ended[event.Section] = true
		}
	}
	return min(len(ended), len(expected))
}
