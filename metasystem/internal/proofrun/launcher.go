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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
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
	Output             io.Writer
	ErrorOutput        io.Writer
	FenceReader        func(string) (stopfence.Record, error)
	ClaimCreator       func(string, string, int64, identity.Ref) (CreationClaim, error)
	Prober             identity.Prober
	Signal             func(int, syscall.Signal) error
	PrepareSuccess     func(CompletionContext) (json.RawMessage, error)
	CommitTerminal     func(CompletionContext, json.RawMessage) error
	BeforeProcessDone  func(CompletionContext) error
}

type CompletionContext struct {
	ControlRoot   string
	ExecutionRoot string
	AttemptID     string
	RecordKey     string
	ExitStatus    int
	CompletedAt   time.Time
	InputIdentity string
}

func LaunchSuite(options LaunchOptions) int {
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
	if err := AppendProgressHeader(options.ProgressPath, ProgressHeader{TmpPaths: options.TmpPaths, LogPaths: []string{options.LogPath}}); err != nil {
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
	if options.AttemptID != "" {
		launchMutation, err = AcquireMutation(controlRoot)
		if err != nil {
			fmt.Fprintln(combinedErr, "suite launcher: acquire child-publication ownership:", err)
			return 1
		}
		if err := attemptLaunchAllowedLocked(controlRoot, options.AttemptID, launcherExact.Ref(), time.Now().UTC()); err != nil {
			fmt.Fprintln(combinedErr, "suite launcher:", err)
			return 1
		}
		attempt, readErr := ReadAttempt(controlRoot, options.AttemptID)
		if readErr != nil {
			fmt.Fprintln(combinedErr, "suite launcher: read proof identity before launch:", readErr)
			return 1
		}
		launchInputIdentity = attempt.ProofIdentity.IdentityDigest
		if options.JoinedAttempt {
			var context ExecutionContext
			var identityErr error
			if attempt.ProofIdentity.CommandClass == "testing" {
				// Joined suites may execute env, Bash, or an application command.
				// The running launcher is the engine owning their proof identity.
				testingEngine, identityErr = os.Executable()
				if identityErr == nil {
					context, identityErr = CaptureSharedExecutionContext(options.Root, options.ConfPath, childEnvironment, testingEngine, attempt.ProofIdentity.ManifestDigest)
				}
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

	suite := exec.Command(options.Command[0], options.Command[1:]...)
	suite.Dir = options.Root
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
		)
	}
	suite.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	suiteOut, err := suite.StdoutPipe()
	if err != nil {
		fmt.Fprintln(combinedErr, "suite launcher:", err)
		return 1
	}
	suiteErr, err := suite.StderrPipe()
	if err != nil {
		fmt.Fprintln(combinedErr, "suite launcher:", err)
		return 1
	}
	if err := suite.Start(); err != nil {
		fmt.Fprintln(combinedErr, "suite launcher:", err)
		return 1
	}
	suiteExact, state, probeErr := prober.Probe(int64(suite.Process.Pid))
	if probeErr != nil || state != identity.Alive {
		releaseLaunchMutation()
		_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
		_ = suite.Wait()
		fmt.Fprintf(combinedErr, "suite launcher: cannot record exact suite identity: %v (%s)\n", probeErr, state)
		return 1
	}

	var copies sync.WaitGroup
	copies.Add(2)
	go copyStream(&copies, combined, suiteOut)
	go copyStream(&copies, combinedErr, suiteErr)
	donePath := options.LogPath + ".done"
	_ = os.Remove(donePath)
	watchdog := watchdogCommand(options, suiteExact.Ref(), donePath, fence.Generation)
	watchdog.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	watchdogOut, err := watchdog.StdoutPipe()
	if err != nil {
		releaseLaunchMutation()
		_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
		_ = suite.Wait()
		fmt.Fprintln(combinedErr, "suite launcher:", err)
		return 1
	}
	watchdogErr, err := watchdog.StderrPipe()
	if err != nil {
		releaseLaunchMutation()
		_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
		_ = suite.Wait()
		fmt.Fprintln(combinedErr, "suite launcher:", err)
		return 1
	}
	if err := watchdog.Start(); err != nil {
		releaseLaunchMutation()
		_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
		_ = suite.Wait()
		fmt.Fprintln(combinedErr, "suite launcher: start sibling watchdog:", err)
		return 1
	}
	watchdogExact, watchdogState, watchdogProbeErr := prober.Probe(int64(watchdog.Process.Pid))
	if watchdogProbeErr != nil || watchdogState != identity.Alive {
		releaseLaunchMutation()
		_ = watchdog.Process.Kill()
		_ = watchdog.Wait()
		_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
		_ = suite.Wait()
		fmt.Fprintf(combinedErr, "suite launcher: cannot record exact watchdog identity: %v (%s)\n", watchdogProbeErr, watchdogState)
		return 1
	}
	copies.Add(2)
	go copyStream(&copies, combined, watchdogOut)
	go copyStream(&copies, combinedErr, watchdogErr)

	launcherPgid, _ := syscall.Getpgid(os.Getpid())
	record := Record{
		Suite: options.Suite, Root: options.Root, FenceGeneration: fence.Generation,
		Launcher:     processIdentity(launcherExact, int64(launcherPgid)),
		SuiteProcess: processIdentity(suiteExact, int64(suite.Process.Pid)),
		Watchdog:     processIdentity(watchdogExact, int64(watchdog.Process.Pid)),
		Status:       StatusRunning,
	}
	if options.AttemptID != "" {
		launchID, launchErr := newLaunchID()
		if launchErr != nil {
			releaseLaunchMutation()
			_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
			_ = suite.Wait()
			_ = watchdog.Process.Kill()
			_ = watchdog.Wait()
			copies.Wait()
			fmt.Fprintln(combinedErr, "suite launcher: create process launch identity:", launchErr)
			return 1
		}
		record.ControlRoot = controlRoot
		record.AttemptID = options.AttemptID
		record.LaunchID = launchID
	}
	if err := writeRecord(record); err != nil {
		releaseLaunchMutation()
		_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
		_ = suite.Wait()
		_ = watchdog.Process.Kill()
		_ = watchdog.Wait()
		copies.Wait()
		fmt.Fprintln(combinedErr, "suite launcher: publish proof-run record:", err)
		return 1
	}
	if options.AttemptID != "" {
		if err := updateAttemptProcessesLocked(controlRoot, options.AttemptID, launcherExact.Ref(), []string{record.Key()}); err != nil {
			releaseLaunchMutation()
			_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
			_ = suite.Wait()
			_ = watchdog.Process.Kill()
			_ = watchdog.Wait()
			copies.Wait()
			fmt.Fprintln(combinedErr, "suite launcher: publish proof attempt processes:", err)
			return 1
		}
		releaseLaunchMutation()
	}
	secondFence, secondFenceErr := readFence(controlRoot)
	stoppedDuringStart := secondFenceErr == nil && (secondFence.State == stopfence.StateClosed || secondFence.Generation != fence.Generation)
	var suiteWait <-chan error
	if secondFenceErr != nil || stoppedDuringStart {
		waited := make(chan error, 1)
		suiteWait = waited
		go func() { waited <- suite.Wait() }()
		outcome := StopSuite(record.SuiteProcess, StopOptions{
			TermGrace: options.TermGrace, KillGrace: options.KillGrace, Poll: options.Poll,
			Prober: prober, Signal: options.Signal,
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
			_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
			_ = suite.Wait()
			_ = watchdog.Process.Kill()
			_ = watchdog.Wait()
			copies.Wait()
			fmt.Fprintln(combinedErr, "suite launcher: close creation claim:", err)
			return 1
		}
		claimClosed = true
	}

	var suiteErrWait error
	if suiteWait == nil {
		suiteErrWait = suite.Wait()
	} else {
		suiteErrWait = <-suiteWait
	}
	doneErr := touchDone(donePath)
	if doneErr != nil {
		fmt.Fprintln(combinedErr, "suite launcher: write watchdog done file:", doneErr)
	}
	watchdogErrWait := watchdog.Wait()
	copies.Wait()
	defer os.Remove(donePath)
	if closeAfterCleanup {
		if err := claim.Close(); err != nil {
			fmt.Fprintln(combinedErr, "suite launcher: close creation claim:", err)
			return 1
		}
		claimClosed = true
	}

	result := exitStatus(suiteErrWait)
	if doneErr != nil {
		result = 1
	}
	if watchdogErrWait != nil {
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
	completedAt := time.Now().UTC()
	completion := CompletionContext{ControlRoot: controlRoot, ExecutionRoot: options.Root, AttemptID: options.AttemptID,
		RecordKey: record.Key(), ExitStatus: result, CompletedAt: completedAt, InputIdentity: parityIdentity}
	var receipt json.RawMessage
	if options.AttemptID != "" && result == 0 && options.PrepareSuccess != nil {
		receipt, err = options.PrepareSuccess(completion)
		if err != nil {
			fmt.Fprintln(combinedErr, "suite launcher: prepare successful proof delivery:", err)
			result = 1
			completion.ExitStatus = result
			completion.CompletedAt = time.Now().UTC()
		}
		if err == nil {
			completion.CompletedAt = time.Now().UTC()
		}
	}
	if options.AttemptID != "" && !options.JoinedAttempt {
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
	mu      sync.Mutex
	writers []io.Writer
}

func (w *lockedWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, writer := range w.writers {
		if writer == nil {
			continue
		}
		if _, err := writer.Write(data); err != nil {
			return 0, err
		}
	}
	return len(data), nil
}

func copyStream(group *sync.WaitGroup, destination io.Writer, source io.Reader) {
	defer group.Done()
	_, _ = io.Copy(destination, source)
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
	for _, path := range []string{options.LogPath} {
		args = append(args, "--log", path)
	}
	return exec.Command(executable, args...)
}

func touchDone(path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
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
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if scanner.Text() == banner {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("cost banner appeared %d times in the suite log; expected exactly once", count)
	}
	return nil
}

func ReadSelectorSections(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var sections []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		id, _, found := strings.Cut(line, "\t")
		if !found || id == "" {
			return nil, fmt.Errorf("selector row is invalid: %q", line)
		}
		sections = append(sections, id)
	}
	return sections, scanner.Err()
}
