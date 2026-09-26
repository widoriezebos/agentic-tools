package launch

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"golang.org/x/sys/unix"
)

// A standalone read is independent diagnostic feedback on current changes or
// a supplied patch, with no builder and no goal attestation. Its request is
// frozen before any launch: the candidate diff, its context base, the brief,
// the inputs and the resolved read lane. The request's digest is its public
// ref, so the same request reaches the same reads again. Each attempt is one
// read sequence under the policy builds use; a later attempt exists only by
// an explicit retry after a failed or stopped attempt whose launches are
// proved stopped. Nothing here writes a goal read, closes, commits, pushes or
// lands anything.

// ReadRequest is a caller's standalone read request.
type ReadRequest struct {
	// Directory is where the caller stands inside a Git checkout; the
	// checkout's top level is resolved from it.
	Directory string
	// Patch is a supplied patch file; empty captures the checkout's current
	// tracked and untracked, unignored changes against its HEAD.
	Patch string
	Brief string
	// Goal only associates the read with a goal for admission and records.
	Goal   string
	Inputs []string
	// Retry, when set, asks for one new attempt after displayed failed or
	// stopped attempt Retry.
	Retry int
	// Wait bounds how long this call advances the reads; the launch wait cap
	// still applies. Zero starts what can start and returns.
	Wait time.Duration
}

// ReadRequestRecord is the frozen request. It is written once and never
// changed.
type ReadRequestRecord struct {
	Ref      string `json:"ref"`
	Kind     string `json:"kind"`
	TopLevel string `json:"topLevel"`
	// Base is the context commit: the checkout's HEAD when the request was
	// made. A supplied patch is not claimed to match it.
	Base        string      `json:"base"`
	Goal        string      `json:"goal,omitempty"`
	Diff        string      `json:"diff"`
	DiffSHA256  string      `json:"diffSha256"`
	Brief       string      `json:"brief"`
	BriefSHA256 string      `json:"briefSha256"`
	Inputs      []namedFile `json:"inputs"`
	Files       []string    `json:"files"`
	Runtime     string      `json:"runtime"`
	Model       string      `json:"model"`
	Window      int64       `json:"window"`
	SplitLines  int64       `json:"splitLines"`
}

// ReadAttempt is one read sequence of a request.
type ReadAttempt struct {
	Number  int    `json:"number"`
	RetryOf int    `json:"retryOf,omitempty"`
	State   string `json:"state"`
	Outcome string `json:"outcome,omitempty"`
	// Context is the disposable directory holding Checkout, the reader's
	// working directory; it is removed once every launch has ended.
	Context        string    `json:"context,omitempty"`
	Checkout       string    `json:"checkout,omitempty"`
	ContextRemoved bool      `json:"contextRemoved,omitempty"`
	Limitation     string    `json:"limitation,omitempty"`
	Round          UnitRound `json:"round"`
}

const (
	readAttemptRunning  = "running"
	readAttemptFinished = "finished"
	readAttemptStopped  = "stopped"
)

// ReadReport is one read launch's retained report and completion state.
type ReadReport struct {
	Step        string        `json:"step"`
	Launch      string        `json:"launch"`
	State       UnitStepState `json:"state"`
	LaunchState State         `json:"launchState,omitempty"`
	Verdict     string        `json:"verdict,omitempty"`
	Counts      bool          `json:"counts"`
	Path        string        `json:"path"`
	Bytes       int64         `json:"bytes"`
	Retained    bool          `json:"retained"`
}

// ReadResult is what a caller shows for a standalone read.
type ReadResult struct {
	Ref     string            `json:"ref,omitempty"`
	Empty   bool              `json:"empty,omitempty"`
	Request ReadRequestRecord `json:"request"`
	Attempt ReadAttempt       `json:"attempt"`
	Reports []ReadReport      `json:"reports"`
	// Complete is true only when every required read passed and counted.
	Complete bool `json:"complete"`
	Capped   bool `json:"capped,omitempty"`
	// Stopping is a requested stop the sequence has not yet recorded.
	Stopping bool `json:"stopping,omitempty"`
	// Uncertain names launches whose processes could not be proved stopped.
	Uncertain []string `json:"uncertain,omitempty"`
}

var readRefPattern = regexp.MustCompile(`^read-[0-9a-f]{24}$`)

var errReadStopped = errors.New("READ_STOPPED: the read was stopped, so no further read starts")

// StartRead freezes a standalone read request and advances it, or rejoins
// the request's existing reads. Empty changes start nothing.
func (runner *UnitRunner) StartRead(request ReadRequest) (ReadResult, error) {
	if runner.Manager == nil {
		return ReadResult{}, errors.New("read launch manager is unavailable")
	}
	if request.Directory == "" || request.Brief == "" {
		return ReadResult{}, errors.New("READ_REQUEST_INVALID: a directory and a brief are required")
	}
	if request.Retry < 0 {
		return ReadResult{}, errors.New("READ_REQUEST_INVALID: retry names a displayed attempt number")
	}
	git := runner.git()
	topOutput, err := git.Run(request.Directory, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return ReadResult{}, fmt.Errorf("READ_CHECKOUT_UNAVAILABLE directory=%s: %w", request.Directory, err)
	}
	top := strings.TrimSpace(string(topOutput))
	headOutput, err := git.Run(top, nil, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return ReadResult{}, fmt.Errorf("READ_CHECKOUT_UNAVAILABLE directory=%s: %w", top, err)
	}
	head := strings.TrimSpace(string(headOutput))
	kind := "changes"
	var diff []byte
	if request.Patch != "" {
		kind = "patch"
		if diff, err = os.ReadFile(request.Patch); err != nil {
			return ReadResult{}, fmt.Errorf("READ_PATCH_UNREADABLE patch=%s: %w", request.Patch, err)
		}
	} else {
		// The caller's own brief inside the checkout is the read's brief,
		// never part of the reviewed changes; every other change stays.
		var excluded []string
		if path, inside := checkoutRelative(top, request.Brief); inside {
			excluded = append(excluded, path)
		}
		if diff, err = runner.worktreeDiff(top, head, excluded...); err != nil {
			return ReadResult{}, err
		}
	}
	if len(diff) == 0 {
		return ReadResult{Empty: true, Request: ReadRequestRecord{Kind: kind, TopLevel: top, Base: head, Goal: request.Goal}}, nil
	}
	files := diffFiles(diff)
	if len(files) == 0 {
		return ReadResult{}, fmt.Errorf("READ_PATCH_UNREADABLE: no file change found; supply a Git diff with diff --git headers")
	}
	brief, err := os.ReadFile(request.Brief)
	if err != nil || len(brief) == 0 {
		return ReadResult{}, fmt.Errorf("READ_BRIEF_MISSING brief=%s", request.Brief)
	}
	settings, err := runner.Manager.resolvedSettings()
	if err != nil {
		return ReadResult{}, err
	}
	model, _, window := settings.launchValues("read")
	record := ReadRequestRecord{Kind: kind, TopLevel: top, Base: head, Goal: request.Goal, DiffSHA256: digestBytes(diff), BriefSHA256: digestBytes(brief),
		Inputs: []namedFile{}, Files: files, Runtime: settings.launchRuntime("read"), Model: model, Window: window, SplitLines: settings.ReadSplitLines}
	inputs := make([][]byte, 0, len(request.Inputs))
	for _, path := range request.Inputs {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return ReadResult{}, fmt.Errorf("READ_INPUT_MISSING input=%s: %w", path, readErr)
		}
		absolute, _ := filepath.Abs(path)
		inputs = append(inputs, data)
		record.Inputs = append(record.Inputs, namedFile{Path: absolute, SHA256: digestBytes(data)})
	}
	identity, err := json.Marshal(record)
	if err != nil {
		return ReadResult{}, err
	}
	record.Ref = "read-" + digestBytes(identity)[:24]
	directory := runner.readDir(record.Ref)
	record.Diff, record.Brief = filepath.Join(directory, "candidate.diff"), filepath.Join(directory, "brief.md")
	for index := range record.Inputs {
		record.Inputs[index].Path = filepath.Join(directory, "inputs", fmt.Sprintf("%02d-%s", index+1, filepath.Base(record.Inputs[index].Path)))
	}
	lock, err := runner.readLock(record.Ref, false)
	if err != nil {
		return ReadResult{}, err
	}
	defer releaseUnitLock(lock)
	frozen, found, err := runner.readRequestRecord(record.Ref)
	if err != nil {
		return ReadResult{}, err
	}
	if found {
		if frozen.DiffSHA256 != record.DiffSHA256 || frozen.BriefSHA256 != record.BriefSHA256 || frozen.Base != record.Base || frozen.TopLevel != record.TopLevel {
			return ReadResult{}, fmt.Errorf("READ_REQUEST_CORRUPT ref=%s: the retained request does not match its ref", record.Ref)
		}
		return runner.rejoinRead(frozen, request)
	}
	if request.Retry != 0 {
		return ReadResult{}, fmt.Errorf("READ_RETRY_UNKNOWN ref=%s: this request has no attempt to retry; repeat it without retry", record.Ref)
	}
	if err := runner.freezeRead(record, diff, brief, inputs); err != nil {
		return ReadResult{}, err
	}
	attempt := ReadAttempt{Number: 1, State: readAttemptRunning}
	if err := runner.planReadAttempt(record, &attempt, true); err != nil {
		return ReadResult{}, err
	}
	if err := runner.writeReadRequest(record); err != nil {
		return ReadResult{}, err
	}
	if err := runner.saveReadAttempt(record.Ref, attempt); err != nil {
		return ReadResult{}, err
	}
	return runner.advanceReadLocked(record, &attempt, runner.readDeadline(request.Wait))
}

// rejoinRead continues a frozen request: the latest attempt, or one new
// attempt for an explicit retry. The caller holds the request lock.
func (runner *UnitRunner) rejoinRead(record ReadRequestRecord, request ReadRequest) (ReadResult, error) {
	attempt, err := runner.latestReadAttempt(record.Ref)
	if err != nil {
		return ReadResult{}, err
	}
	deadline := runner.readDeadline(request.Wait)
	if request.Retry == 0 || attempt.RetryOf == request.Retry && attempt.Number == request.Retry+1 {
		return runner.advanceReadLocked(record, &attempt, deadline)
	}
	if attempt.Number != request.Retry {
		return ReadResult{}, fmt.Errorf("READ_RETRY_NOT_LATEST ref=%s attempt=%d: only the latest displayed attempt can be retried", record.Ref, attempt.Number)
	}
	if runner.readStopRequested(record.Ref, attempt.Number) && attempt.State == readAttemptRunning {
		if err := runner.finishStoppedRead(record, &attempt); err != nil {
			return ReadResult{}, err
		}
	}
	switch {
	case attempt.State == readAttemptRunning:
		return ReadResult{}, fmt.Errorf("READ_RETRY_RUNNING ref=%s attempt=%d: the attempt has not ended; wait for it or stop it first", record.Ref, attempt.Number)
	case attempt.Outcome == "green":
		return ReadResult{}, fmt.Errorf("READ_RETRY_COMPLETE ref=%s attempt=%d: the attempt completed; nothing to retry", record.Ref, attempt.Number)
	}
	runner.recoverStrandedReads(attempt)
	if unproven := runner.unprovenReadLaunches(attempt); len(unproven) != 0 {
		return ReadResult{}, fmt.Errorf("READ_RETRY_UNPROVEN ref=%s attempt=%d launches=%s: these launches are not proved stopped, so no new attempt starts; stop the read and repeat", record.Ref, attempt.Number, strings.Join(unproven, ","))
	}
	next := ReadAttempt{Number: attempt.Number + 1, RetryOf: attempt.Number, State: readAttemptRunning}
	if err := runner.planReadAttempt(record, &next, false); err != nil {
		return ReadResult{}, err
	}
	if err := runner.saveReadAttempt(record.Ref, next); err != nil {
		return ReadResult{}, err
	}
	return runner.advanceReadLocked(record, &next, deadline)
}

// AdvanceRead advances a request's latest attempt until it ends or the
// timeout passes. While another caller advances it, this call follows the
// retained state instead.
func (runner *UnitRunner) AdvanceRead(ref string, timeout time.Duration) (ReadResult, error) {
	if runner.Manager == nil {
		return ReadResult{}, errors.New("read launch manager is unavailable")
	}
	record, err := runner.frozenRead(ref)
	if err != nil {
		return ReadResult{}, err
	}
	deadline := runner.readDeadline(timeout)
	for {
		lock, lockErr := runner.readLock(ref, false)
		if lockErr == nil {
			defer releaseUnitLock(lock)
			attempt, err := runner.latestReadAttempt(ref)
			if err != nil {
				return ReadResult{}, err
			}
			return runner.advanceReadLocked(record, &attempt, deadline)
		}
		if !strings.HasPrefix(lockErr.Error(), "READ_BUSY") {
			return ReadResult{}, lockErr
		}
		result, err := runner.InspectRead(ref)
		if err != nil || result.Attempt.State != readAttemptRunning {
			return result, err
		}
		if !runner.Manager.Now().Before(deadline) {
			result.Capped = true
			return result, nil
		}
		runner.Manager.Sleep(runner.Manager.Poll)
	}
}

// InspectRead reports a request's latest attempt from retained state and
// launch records, writing nothing.
func (runner *UnitRunner) InspectRead(ref string) (ReadResult, error) {
	record, err := runner.frozenRead(ref)
	if err != nil {
		return ReadResult{}, err
	}
	attempt, err := runner.latestReadAttempt(ref)
	if err != nil {
		return ReadResult{}, err
	}
	return runner.readResult(record, attempt), nil
}

// StopRead stops a request's latest attempt: no further read of it starts,
// and every live launch it owns is cancelled. A launch whose processes
// cannot be proved stopped is reported as uncertain. Repeating it is safe.
func (runner *UnitRunner) StopRead(ref string) (ReadResult, error) {
	if runner.Manager == nil {
		return ReadResult{}, errors.New("read launch manager is unavailable")
	}
	record, err := runner.frozenRead(ref)
	if err != nil {
		return ReadResult{}, err
	}
	attempt, err := runner.latestReadAttempt(ref)
	if err != nil {
		return ReadResult{}, err
	}
	if attempt.State != readAttemptRunning {
		return runner.readResult(record, attempt), nil
	}
	marker := runner.readStopPath(ref, attempt.Number)
	if _, err := os.Stat(marker); errors.Is(err, fs.ErrNotExist) {
		data, _ := json.Marshal(map[string]string{"requestedAt": runner.Manager.Now().UTC().Format(time.RFC3339Nano)})
		if _, err := atomicfile.WriteText(marker, string(data)+"\n", runner.root()); err != nil {
			return ReadResult{}, err
		}
	}
	// Every launch starts under the start lock after checking the marker,
	// so once it is held no start is in flight and none can follow; the
	// retained attempt then names every launch that exists.
	start, err := runner.readLock(ref, true)
	if err != nil {
		return ReadResult{}, err
	}
	defer releaseUnitLock(start)
	if attempt, err = runner.latestReadAttempt(ref); err != nil {
		return ReadResult{}, err
	}
	var uncertain []string
	for _, step := range attempt.Round.Steps {
		if step.LaunchID == "" {
			continue
		}
		launchRecord, readErr := runner.Manager.Store.Read(step.LaunchID)
		if errors.Is(readErr, fs.ErrNotExist) {
			continue
		}
		if readErr == nil && launchRecord.State.Terminal() {
			continue
		}
		// A start recorded without its supervisor may still get one;
		// cancelling it would record it proved dead. Only the launch
		// startup recovery settles it; until it does, it stays uncertain.
		if readErr == nil && launchRecord.State == Starting && launchRecord.Supervisor == nil {
			if _, recovered, recoverErr := runner.Manager.RecoverStrandedStart(step.LaunchID); recoverErr != nil || !recovered {
				uncertain = append(uncertain, step.LaunchID)
			}
			continue
		}
		if readErr == nil {
			_, readErr = runner.Manager.Cancel(step.LaunchID)
		}
		if readErr != nil {
			uncertain = append(uncertain, step.LaunchID)
		}
	}
	result := runner.readResult(record, attempt)
	result.Uncertain = append(result.Uncertain, uncertain...)
	sort.Strings(result.Uncertain)
	result.Uncertain = slices.Compact(result.Uncertain)
	return result, nil
}

// advanceReadLocked runs one attempt's reads; the caller holds the request
// lock.
func (runner *UnitRunner) advanceReadLocked(record ReadRequestRecord, attempt *ReadAttempt, deadline time.Time) (ReadResult, error) {
	if attempt.State != readAttemptRunning {
		return runner.readResult(record, *attempt), nil
	}
	if runner.readStopRequested(record.Ref, attempt.Number) {
		if err := runner.finishStoppedRead(record, attempt); err != nil {
			return ReadResult{}, err
		}
		return runner.readResult(record, *attempt), nil
	}
	if attempt.Checkout == "" {
		if err := runner.prepareReadContext(record, attempt); err != nil {
			return ReadResult{}, err
		}
		if err := runner.saveReadAttempt(record.Ref, *attempt); err != nil {
			return ReadResult{}, err
		}
	}
	sequence := runner.standaloneSequence(record, attempt)
	readStart, err := sequence.plan()
	if err != nil {
		return ReadResult{}, err
	}
	outcome, _, capped, err := sequence.advance(readStart, deadline)
	if errors.Is(err, errReadStopped) || runner.readStopRequested(record.Ref, attempt.Number) {
		if err := runner.finishStoppedRead(record, attempt); err != nil {
			return ReadResult{}, err
		}
		return runner.readResult(record, *attempt), nil
	}
	if err != nil {
		return runner.readResult(record, *attempt), err
	}
	if capped {
		result := runner.readResult(record, *attempt)
		result.Capped = true
		return result, nil
	}
	attempt.State, attempt.Outcome = readAttemptFinished, outcome
	runner.removeReadContext(attempt)
	if err := runner.saveReadAttempt(record.Ref, *attempt); err != nil {
		return ReadResult{}, err
	}
	return runner.readResult(record, *attempt), nil
}

// standaloneSequence is an attempt's reads under the shared partition
// policy. Every read writes its own retained report, so reads run one at a
// time, each started under the start lock after the stop marker is checked.
func (runner *UnitRunner) standaloneSequence(record ReadRequestRecord, attempt *ReadAttempt) readSequence {
	directory := runner.readAttemptDir(record.Ref, attempt.Number)
	inputs := make([]string, 0, len(record.Inputs))
	for _, input := range record.Inputs {
		inputs = append(inputs, input.Path)
	}
	notice := "Diagnostic feedback only: this read certifies nothing. No goal read, closure, commit, push or landing follows from it, and a verdict of land is feedback wording only.\n" +
		fmt.Sprintf("Request: %s (%s) at %s\nContext base: %s\nCandidate SHA-256: %s\nSelected files: %s\n", record.Ref, record.Kind, record.TopLevel, record.Base, record.DiffSHA256, strings.Join(record.Files, ", "))
	if attempt.Limitation != "" {
		notice += "Context limitation: " + attempt.Limitation + "\n"
	}
	driver := stepDriver{manager: runner.Manager, round: &attempt.Round,
		launchID: func(index int) string { return fmt.Sprintf("%s-a%d-s%d", record.Ref, attempt.Number, index+1) },
		save:     func() error { return runner.saveReadAttempt(record.Ref, *attempt) },
		before:   func(StartSpec) error { return runner.verifyFrozenRead(record) },
		start: func(spec StartSpec) (Record, error) {
			lock, err := runner.readLock(record.Ref, true)
			if err != nil {
				return Record{}, err
			}
			defer releaseUnitLock(lock)
			if runner.readStopRequested(record.Ref, attempt.Number) {
				return Record{}, errReadStopped
			}
			return runner.Manager.Start(spec)
		}}
	return readSequence{driver: driver, model: record.Model, diff: record.Diff, serial: true, notice: notice,
		spec: StartSpec{Kind: "read", Goal: record.Goal, Tag: record.Ref, WorkingDirectory: attempt.Checkout, Brief: record.Brief, Inputs: inputs, Model: record.Model},
		outputs: func(step UnitStep) []string {
			return []string{filepath.Join(directory, "reports", readReportName(step.Name))}
		}}
}

// planReadAttempt plans an attempt's first pass of reads. A first attempt's
// reads are admitted before the request is retained, so a refused request
// reserves nothing.
func (runner *UnitRunner) planReadAttempt(record ReadRequestRecord, attempt *ReadAttempt, admit bool) error {
	attempt.Round = UnitRound{Number: attempt.Number, Directory: runner.readAttemptDir(record.Ref, attempt.Number), ReadModel: record.Model}
	if err := os.MkdirAll(filepath.Join(attempt.Round.Directory, "reports"), 0o700); err != nil {
		return err
	}
	sequence := runner.standaloneSequence(record, attempt)
	sequence.driver.save = func() error { return nil }
	if _, err := sequence.plan(); err != nil {
		return err
	}
	if !admit {
		return nil
	}
	for _, step := range attempt.Round.Steps {
		spec, err := sequence.stepSpec(step)
		if err != nil {
			return err
		}
		spec.WorkingDirectory = record.TopLevel
		if err := runner.Manager.Admit(spec); err != nil {
			return err
		}
	}
	return nil
}

// finishStoppedRead records a stopped attempt: its unstarted reads are
// skipped, and its context is removed once every launch has ended.
func (runner *UnitRunner) finishStoppedRead(record ReadRequestRecord, attempt *ReadAttempt) error {
	runner.recoverStrandedReads(*attempt)
	for index := range attempt.Round.Steps {
		step := &attempt.Round.Steps[index]
		if step.State == StepPending || step.State == StepStarting && !runner.launchExists(step.LaunchID) {
			step.State, step.Reason = StepSkipped, "stopped"
			continue
		}
		if step.State == StepStarting || step.State == StepRunning {
			if launchRecord, err := runner.Manager.Store.Read(step.LaunchID); err == nil && launchRecord.State.Terminal() {
				(stepDriver{manager: runner.Manager, round: &attempt.Round}).endStep(index, launchRecord)
			}
		}
	}
	if len(runner.unprovenReadLaunches(*attempt)) != 0 {
		// A launch still runs or is unproved; the attempt stays running
		// under its stop marker and a later call records it.
		return runner.saveReadAttempt(record.Ref, *attempt)
	}
	attempt.State, attempt.Outcome = readAttemptStopped, "stopped"
	runner.removeReadContext(attempt)
	return runner.saveReadAttempt(record.Ref, *attempt)
}

// unprovenReadLaunches names an attempt's launches whose processes are not
// proved stopped, reading only. A launch never recorded has no process. A
// launch still starting is unproved whatever it records; only the launch
// startup recovery settles a start no supervisor claimed.
func (runner *UnitRunner) unprovenReadLaunches(attempt ReadAttempt) []string {
	var unproven []string
	for _, step := range attempt.Round.Steps {
		if step.LaunchID == "" {
			continue
		}
		launchRecord, err := runner.Manager.Store.Read(step.LaunchID)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil || !launchRecord.State.Terminal() || !runner.Manager.provenDead(launchRecord) {
			unproven = append(unproven, step.LaunchID)
		}
	}
	return unproven
}

// recoverStrandedReads settles an attempt's starts that no supervisor ever
// claimed, through the launch startup recovery; any other launch keeps its
// state and its proof-of-death rules. Only callers that may change launch
// state call it; InspectRead never does.
func (runner *UnitRunner) recoverStrandedReads(attempt ReadAttempt) {
	for _, step := range attempt.Round.Steps {
		if step.LaunchID == "" {
			continue
		}
		if launchRecord, err := runner.Manager.Store.Read(step.LaunchID); err == nil && launchRecord.State == Starting {
			_, _, _ = runner.Manager.RecoverStrandedStart(step.LaunchID)
		}
	}
}

func (runner *UnitRunner) launchExists(id string) bool {
	if id == "" {
		return false
	}
	_, err := runner.Manager.Store.Read(id)
	return !errors.Is(err, fs.ErrNotExist)
}

// readResult is the display of an attempt: every read's retained report and
// completion state. Only a finished green attempt is complete.
func (runner *UnitRunner) readResult(record ReadRequestRecord, attempt ReadAttempt) ReadResult {
	result := ReadResult{Ref: record.Ref, Request: record, Attempt: attempt, Reports: []ReadReport{}}
	result.Complete = attempt.State == readAttemptFinished && attempt.Outcome == "green"
	result.Stopping = attempt.State == readAttemptRunning && runner.readStopRequested(record.Ref, attempt.Number)
	directory := runner.readAttemptDir(record.Ref, attempt.Number)
	for _, step := range attempt.Round.Steps {
		if !strings.HasPrefix(step.Name, "read") {
			continue
		}
		report := ReadReport{Step: step.Name, Launch: step.LaunchID, State: step.State, Verdict: step.Verdict, Counts: unitStepVerdictCounts(step),
			Path: filepath.Join(directory, "reports", readReportName(step.Name))}
		if step.LaunchID != "" {
			if launchRecord, err := runner.Manager.Store.Read(step.LaunchID); err == nil {
				report.LaunchState = launchRecord.State
				// A started read's report is the output its launch was
				// actually given, whatever name this code would choose now.
				var declared []string
				if json.Unmarshal(launchRecord.AdapterData["declaredOutputs"], &declared) == nil && len(declared) == 1 {
					report.Path = declared[0]
				}
			}
		}
		if info, err := os.Stat(report.Path); err == nil {
			report.Bytes, report.Retained = info.Size(), true
		}
		result.Reports = append(result.Reports, report)
	}
	if result.Stopping {
		result.Uncertain = runner.unprovenReadLaunches(attempt)
	}
	return result
}

// prepareReadContext materializes the attempt's context tree in a private
// disposable directory outside the checkout: the base commit's tracked
// files through a private index, then the candidate applied as plain files.
// Ignored source files and local configuration are never part of it, and it
// has no Git directory of its own; it is not a sandbox. A supplied patch
// that does not apply leaves the base and records that limitation.
func (runner *UnitRunner) prepareReadContext(record ReadRequestRecord, attempt *ReadAttempt) error {
	// The name is the attempt's own, so an attempt interrupted before it
	// recorded its context replaces only what it made itself.
	context := filepath.Join(os.TempDir(), fmt.Sprintf("metasystem-read-context.%s-a%d", record.Ref, attempt.Number))
	if err := os.Mkdir(context, 0o700); errors.Is(err, fs.ErrExist) {
		if err := os.RemoveAll(context); err != nil {
			return err
		}
		err = os.Mkdir(context, 0o700)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	checkout := filepath.Join(context, "checkout")
	fail := func(err error) error {
		_ = os.RemoveAll(context)
		return err
	}
	if err := os.Mkdir(checkout, 0o700); err != nil {
		return fail(err)
	}
	git := runner.git()
	environment := []string{"GIT_INDEX_FILE=" + filepath.Join(context, "index"), "GIT_OPTIONAL_LOCKS=0"}
	if _, err := git.Run(record.TopLevel, environment, "read-tree", record.Base); err != nil {
		return fail(err)
	}
	if _, err := git.Run(record.TopLevel, environment, "checkout-index", "--all", "--prefix="+checkout+string(os.PathSeparator)); err != nil {
		return fail(err)
	}
	if err := os.Remove(filepath.Join(context, "index")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fail(err)
	}
	if _, err := git.Run(checkout, []string{"GIT_CEILING_DIRECTORIES=" + context}, "apply", "--binary", record.Diff); err != nil {
		if record.Kind != "patch" {
			return fail(fmt.Errorf("READ_CONTEXT_UNAVAILABLE ref=%s: captured changes do not apply to their own base: %w", record.Ref, err))
		}
		attempt.Limitation = fmt.Sprintf("the supplied patch does not apply to base %s, so the reader's checkout holds the base only: %v", record.Base, err)
	}
	attempt.Context, attempt.Checkout = context, checkout
	return nil
}

// removeReadContext removes an attempt's disposable context once its
// launches have ended; reports are retained outside it.
func (runner *UnitRunner) removeReadContext(attempt *ReadAttempt) {
	if attempt.Context == "" || attempt.ContextRemoved || len(runner.unprovenReadLaunches(*attempt)) != 0 {
		return
	}
	if !strings.HasPrefix(filepath.Base(attempt.Context), "metasystem-read-context.") || attempt.Checkout != filepath.Join(attempt.Context, "checkout") {
		return
	}
	if err := os.RemoveAll(attempt.Context); err == nil {
		attempt.ContextRemoved = true
	}
}

// verifyFrozenRead refuses a launch whose frozen bytes or read lane changed.
func (runner *UnitRunner) verifyFrozenRead(record ReadRequestRecord) error {
	settings, err := runner.Manager.resolvedSettings()
	if err != nil {
		return err
	}
	model, _, window := settings.launchValues("read")
	changed := settings.launchRuntime("read") != record.Runtime || model != record.Model || window != record.Window || settings.ReadSplitLines != record.SplitLines
	for _, file := range append([]namedFile{{Path: record.Diff, SHA256: record.DiffSHA256}, {Path: record.Brief, SHA256: record.BriefSHA256}}, record.Inputs...) {
		data, readErr := os.ReadFile(file.Path)
		changed = changed || readErr != nil || digestBytes(data) != file.SHA256
	}
	if changed {
		return fmt.Errorf("READ_INPUT_CHANGED ref=%s: the request's retained inputs or read settings no longer match it, so nothing more was launched", record.Ref)
	}
	return nil
}

func (runner *UnitRunner) freezeRead(record ReadRequestRecord, diff, brief []byte, inputs [][]byte) error {
	if err := os.MkdirAll(filepath.Join(runner.readDir(record.Ref), "inputs"), 0o700); err != nil {
		return err
	}
	if _, err := atomicfile.WriteText(record.Diff, string(diff), runner.root()); err != nil {
		return err
	}
	if _, err := atomicfile.WriteText(record.Brief, string(brief), runner.root()); err != nil {
		return err
	}
	for index, input := range record.Inputs {
		if _, err := atomicfile.WriteText(input.Path, string(inputs[index]), runner.root()); err != nil {
			return err
		}
	}
	return nil
}

func (runner *UnitRunner) readDeadline(wait time.Duration) time.Time {
	cap, err := runner.Manager.WaitCap()
	if err == nil && (wait > cap || wait < 0) {
		wait = cap
	}
	if wait < 0 {
		wait = 0
	}
	return runner.Manager.Now().Add(wait)
}

func (runner *UnitRunner) git() GitRunner {
	if runner.Git == nil {
		return OSGitRunner{}
	}
	return runner.Git
}

func (runner *UnitRunner) readDir(ref string) string {
	return filepath.Join(runner.root(), ".reads", ref)
}

func (runner *UnitRunner) readAttemptDir(ref string, number int) string {
	return filepath.Join(runner.readDir(ref), fmt.Sprintf("attempt-%d", number))
}

func (runner *UnitRunner) readStopPath(ref string, number int) string {
	return filepath.Join(runner.readAttemptDir(ref, number), "stop.json")
}

func (runner *UnitRunner) readStopRequested(ref string, number int) bool {
	_, err := os.Stat(runner.readStopPath(ref, number))
	return err == nil
}

// readLock holds a request: its advancing lock refuses a second caller as
// busy, and its start lock, taken waiting, serializes launch starts with a
// stop.
func (runner *UnitRunner) readLock(ref string, start bool) (*os.File, error) {
	directory := runner.readDir(ref)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	name, mode := ".lock", unix.LOCK_EX|unix.LOCK_NB
	if start {
		name, mode = ".start.lock", unix.LOCK_EX
	}
	file, err := os.OpenFile(filepath.Join(directory, name), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), mode); err != nil {
		file.Close()
		if !lockWouldBlock(err) {
			return nil, fmt.Errorf("READ_LOCK_FAILED ref=%s: %w", ref, err)
		}
		return nil, fmt.Errorf("READ_BUSY ref=%s: another caller is advancing this read; repeat to follow it", ref)
	}
	return file, nil
}

func (runner *UnitRunner) frozenRead(ref string) (ReadRequestRecord, error) {
	if !readRefPattern.MatchString(ref) {
		return ReadRequestRecord{}, fmt.Errorf("READ_REF_INVALID ref=%q", ref)
	}
	record, found, err := runner.readRequestRecord(ref)
	if err == nil && !found {
		err = fmt.Errorf("READ_REF_UNKNOWN ref=%s", ref)
	}
	return record, err
}

func (runner *UnitRunner) readRequestRecord(ref string) (ReadRequestRecord, bool, error) {
	data, err := os.ReadFile(filepath.Join(runner.readDir(ref), "request.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return ReadRequestRecord{}, false, nil
	}
	if err != nil {
		return ReadRequestRecord{}, false, err
	}
	var record ReadRequestRecord
	if err := json.Unmarshal(data, &record); err != nil || record.Ref != ref {
		return ReadRequestRecord{}, false, fmt.Errorf("READ_REQUEST_CORRUPT ref=%s", ref)
	}
	return record, true, nil
}

func (runner *UnitRunner) writeReadRequest(record ReadRequestRecord) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(filepath.Join(runner.readDir(record.Ref), "request.json"), string(data)+"\n", runner.root())
	return err
}

func (runner *UnitRunner) saveReadAttempt(ref string, attempt ReadAttempt) error {
	data, err := json.MarshalIndent(attempt, "", "  ")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(filepath.Join(runner.readAttemptDir(ref, attempt.Number), "attempt.json"), string(data)+"\n", runner.root())
	return err
}

func (runner *UnitRunner) latestReadAttempt(ref string) (ReadAttempt, error) {
	for number := 1; ; number++ {
		if _, err := os.Stat(filepath.Join(runner.readAttemptDir(ref, number+1), "attempt.json")); err == nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join(runner.readAttemptDir(ref, number), "attempt.json"))
		if err != nil {
			return ReadAttempt{}, fmt.Errorf("READ_ATTEMPT_MISSING ref=%s attempt=%d: %w", ref, number, err)
		}
		var attempt ReadAttempt
		if err := json.Unmarshal(data, &attempt); err != nil || attempt.Number != number {
			return ReadAttempt{}, fmt.Errorf("READ_ATTEMPT_CORRUPT ref=%s attempt=%d", ref, number)
		}
		return attempt, nil
	}
}

// checkoutRelative is path's slash-separated location inside the checkout
// at top, when it is inside it.
func checkoutRelative(top, path string) (string, bool) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	if real, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = real
	}
	if real, err := filepath.EvalSymlinks(top); err == nil {
		top = real
	}
	relative, err := filepath.Rel(top, absolute)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

// diffFiles lists a diff's changed paths from its headers, so binary and
// mode-only changes are listed too.
func diffFiles(diff []byte) []string {
	seen := map[string]bool{}
	var files []string
	for _, line := range strings.Split(string(diff), "\n") {
		header, ok := strings.CutPrefix(line, "diff --git ")
		if !ok {
			continue
		}
		index := strings.LastIndex(header, " b/")
		if index < 0 {
			continue
		}
		name := header[index+3:]
		if !seen[name] {
			seen[name] = true
			files = append(files, name)
		}
	}
	sort.Strings(files)
	return files
}

// readReportName is a step's report file: its name made safe for a path,
// with a digest of the exact name so two steps never share a file.
func readReportName(step string) string {
	var builder strings.Builder
	for _, character := range step {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '.' || character == '-' {
			builder.WriteRune(character)
		} else {
			builder.WriteByte('_')
		}
	}
	return builder.String() + "-" + digestBytes([]byte(step))[:12] + ".md"
}

func digestBytes(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
