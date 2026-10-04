package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/roots"
)

// The Stop timeout is a sixty-second budget by default: the registration
// templates ship it and adoption installs it. The deadline parent gives its
// worker the budget minus three seconds for arming, health, digest,
// watchdog and verdict work, then keeps three seconds to publish a
// provider-level answer. The parent's own notices never depend on the engine
// state or the payload: fixed text only.

var (
	budgetPattern       = regexp.MustCompile(`^([4-9]|[1-5][0-9]|60)$`)
	leadingZeroPattern  = regexp.MustCompile(`^0[0-9]+$`)
	payloadSessionField = regexp.MustCompile(`"session_id"[[:space:]]*:[[:space:]]*"([^"\\]*)"`)
	slugNonWord         = regexp.MustCompile(`[^a-z0-9._-]+`)
)

type deadlineParent struct {
	inv Invocation
	ops Ops

	budgetSec, workerSec int64
	startedEpoch         int64
	startedMono          time.Duration
	expires              time.Duration

	dir, stdoutPath, stderrPath, payloadPath string

	installation roots.Installation
	engine       string
	harnessRoot  string
	session      string
	// repo is the installation the engine resolved for the record
	// coordinates: the refusal record and the hook log are its run state.
	repo       roots.Installation
	record     string
	fromEngine bool
	logFailure string

	resolverDone    chan struct{}
	resolvedSession string
	resolvedRoot    string
}

// deadlineBudget validates the configured Stop budget; anything outside
// four to sixty seconds is the default sixty.
func deadlineBudget(value string) int64 {
	if value == "" {
		value = "60"
	}
	for leadingZeroPattern.MatchString(value) {
		value = strings.TrimPrefix(value, "0")
	}
	if !budgetPattern.MatchString(value) {
		return 60
	}
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

func (p *deadlineParent) elapsedWholeSeconds() int64 {
	return int64((p.inv.Monotonic() - p.startedMono) / time.Second)
}

func runStopDeadlineParent(inv Invocation, ops Ops, harnessRoot string) int {
	if inv.Monotonic == nil {
		origin := time.Now()
		inv.Monotonic = func() time.Duration { return time.Since(origin) }
	}
	if inv.Sleep == nil {
		inv.Sleep = time.Sleep
	}
	p := &deadlineParent{inv: inv, ops: ops, harnessRoot: harnessRoot}
	p.budgetSec = deadlineBudget(inv.env(stopDeadlineBudgetEnv))
	p.workerSec = p.budgetSec - 3
	p.startedEpoch = inv.Now().Unix()
	dir, err := mkdirStaging(inv.TempDir, "metasystem-stop-deadline.")
	if err != nil {
		_ = writeLine(inv.Stdout, mustDegradedStopForm("allowed", "staging-failed"))
		return 0
	}
	p.dir = dir
	p.stdoutPath = filepath.Join(dir, "stdout")
	p.stderrPath = filepath.Join(dir, "stderr")
	p.payloadPath = filepath.Join(dir, "payload")
	if err := stagePayload(p.payloadPath, inv.Stdin); err != nil {
		_ = writeLine(inv.Stdout, mustDegradedStopForm("allowed", "staging-failed"))
		p.removeFiles()
		return 0
	}
	p.startedMono = inv.Monotonic()
	if world, ok := worldInstallation(ops, harnessRoot); ok {
		p.installation = world
		canonical := world.Path() + "/bin/metasystem"
		p.engine = inv.env("METASYSTEM_BIN")
		if p.engine == "" {
			p.engine = canonical
		}
		if !inv.IsExecutable(canonical) || !inv.IsExecutable(p.engine) {
			p.engine = ""
		}
	}

	var boundary *StopDeadlineBootBoundary
	if p.engine != "" && inv.Deadline.BootClock != nil {
		if bootID, origin, err := inv.Deadline.BootClock(); err == nil && bootID != "" && !strings.ContainsAny(bootID, " \t\n") {
			setupElapsed := p.elapsedWholeSeconds()
			if setupElapsed < 0 {
				setupElapsed = 0
			}
			remaining := p.workerSec - setupElapsed
			if remaining < 0 {
				remaining = 0
			}
			deadline := origin + time.Duration(remaining)*time.Second
			originAtLaunch := deadline - time.Duration(p.workerSec)*time.Second
			if deadline > originAtLaunch {
				boundary = &StopDeadlineBootBoundary{BootID: bootID, Origin: originAtLaunch, Deadline: deadline}
			}
		}
	}

	worker, workerErr := p.startWorker()
	p.expires = p.startedMono + time.Duration(p.workerSec)*time.Second

	// Resolve the record coordinates alongside the worker, never ahead of
	// it: the session and the engine-owned state root.
	if p.engine != "" {
		p.resolverDone = make(chan struct{})
		answered := inv.Deadline.ResolverAnswered
		go func() {
			// Deferred in reverse: the answer is adoptable before anyone
			// is told it exists.
			defer func() {
				if answered != nil {
					answered()
				}
			}()
			defer close(p.resolverDone)
			payload, _ := os.ReadFile(p.payloadPath)
			p.resolvedSession = jsonValue(string(payload), "session_id")
			root, status := ops.StateRoot(p.installation)
			if status == 0 {
				p.resolvedRoot = trimNewlines(root)
			}
		}()
	}
	p.session = payloadSession(p.payloadPath)
	if p.session == "" {
		p.session = "session-" + strconv.Itoa(inv.Pid)
	}

	waitState := ""
	var waited StopDeadlineWaitResult
	if workerErr == nil && p.engine != "" && boundary != nil {
		result, err := p.waitWorker(worker, *boundary)
		if err == nil {
			waited = result
			waitState = result.State
		} else {
			p.appendStderr("hooks stop-deadline-wait: " + err.Error() + "\n")
		}
	}
	if waitState != StopWorkerCompleted && waitState != StopDeadlineReached {
		// With no executable engine there is no typed owner. Keep the
		// provider's outer bound, but never let this fallback authorize a
		// signal; exact cleanup requires the recorded identity.
		for p.running(worker) && inv.Monotonic() < p.expires {
			inv.Sleep(50 * time.Millisecond)
		}
		if p.running(worker) {
			waitState = StopDeadlineReached
		} else {
			waitState = StopWorkerCompleted
		}
		waited = StopDeadlineWaitResult{}
	}
	p.captureCoordinates()

	if waitState == StopWorkerCompleted {
		status := 127
		if worker != nil {
			<-worker.Done()
			status = worker.Status()
		}
		p.copyStderr()
		raw := trimNewlines(readFileString(p.stdoutPath))
		if status == 0 && raw == internalSkipResult {
			p.captureCoordinates()
			p.removeFiles()
			return 0
		}
		valid := false
		if status == 0 && p.engine != "" {
			valid = validStopOutput(readFileString(p.stdoutPath), true)
		} else if status == 0 {
			valid = raw == mustDegradedStopForm("allowed", "engine-missing")
		}
		if status != 0 || !valid {
			for !p.resolved() && p.resolverRunning() && inv.Monotonic() < p.expires {
				inv.Sleep(50 * time.Millisecond)
			}
			p.captureCoordinates()
			p.logStopCondition("stop-hook-output-was-unreadable", "stop-worker")
			p.logStopOutcome("invalid-worker-output-allow", "")
			var qualifiers []string
			if p.logFailure != "" {
				qualifiers = append(qualifiers, "condition-log-failed")
			}
			if p.repo == "" {
				qualifiers = append(qualifiers, "no-resolved-checkout")
			}
			_ = writeLine(inv.Stdout, mustDegradedStopForm("allowed", "unreadable-output", qualifiers...))
		} else {
			p.captureCoordinates()
			_, _ = io.WriteString(inv.Stdout, readFileString(p.stdoutPath))
		}
		p.removeFiles()
		return 0
	}

	// The worker publishes its provider response before recording
	// completion. A complete response is already a safe decision even when
	// completion bookkeeping used the rest of the budget: validate it before
	// stopping the worker so a real verdict is never replaced.
	published := p.published()
	p.captureCoordinates()
	signalled, reaped := false, false
	cleanupState := StopWorkerUnverified
	if p.engine != "" && waited.State == StopDeadlineReached && worker != nil {
		result := p.cleanupWorker(waited.Worker)
		cleanupState = result.State
		signalled = result.TermSent
	}
	switch cleanupState {
	case StopWorkerTerminated, StopWorkerGone:
		<-worker.Done()
		reaped = true
		if !signalled {
			p.copyStderr()
			if p.published() {
				_, _ = io.WriteString(inv.Stdout, readFileString(p.stdoutPath))
				p.removeFiles()
				return 0
			}
		}
	case StopWorkerKilled:
		fmt.Fprintf(inv.Stderr, "stop deadline: worker %d was killed but has not confirmed it ended; it stays watched\n", workerPid(worker))
	default:
		fmt.Fprintf(inv.Stderr, "stop deadline: worker %d left running, command line unverifiable\n", workerPid(worker))
	}
	elapsed := inv.Now().Unix() - p.startedEpoch
	if elapsed < 0 {
		elapsed = 0
	}
	if p.repo != "" && p.engine != "" && inv.IsExecutable(p.engine) {
		ops.HookExpire(p.repo.Path(), elapsed)
	}
	if published {
		_, _ = io.WriteString(inv.Stdout, readFileString(p.stdoutPath))
		if reaped {
			p.removeFiles()
		}
		return 0
	}
	cause := "stop deadline expired"
	remedy := "A human or steward must restore supervision outside this seat, then retry."
	detail := "Metasystem Stop deadline expired before a turn verdict; stopping is allowed with degraded infrastructure."
	recordFailure := ""
	switch {
	case p.record == "":
		recordFailure = "the stop-refusal record coordinates could not be resolved"
	case p.engine == "":
		recordFailure = "the stop-refusal record engine was unavailable"
	default:
		// The refusal owner holds the locked occurrence record; its public
		// response is replaced by the one-line unavailable form.
		if _, status := ops.StopBlock(StopBlockRequest{
			Class: "infrastructure", RefusalRecord: p.record, Session: p.session, OpenWorkRoot: p.repo.Path(),
			Cause: cause, Remedy: remedy, Detail: detail,
		}); status != 0 {
			recordFailure = "the stop-refusal record could not be read or atomically updated"
		}
	}
	p.logStopCondition("stop-deadline-expired", "stop-deadline")
	var qualifiers []string
	if recordFailure != "" {
		qualifiers = append(qualifiers, "record-update-failed")
	}
	if p.logFailure != "" {
		qualifiers = append(qualifiers, "condition-log-failed")
	}
	if p.repo == "" {
		qualifiers = append(qualifiers, "no-resolved-checkout")
	}
	measured := strconv.FormatInt(elapsed, 10)
	if recordFailure != "" {
		p.logStopOutcome("deadline-expired-record-failure-allow", measured)
	} else {
		p.logStopOutcome("deadline-expired-allow", measured)
	}
	_ = writeLine(inv.Stdout, mustDegradedStopForm("allowed", "deadline-expired", qualifiers...))
	if reaped {
		p.removeFiles()
	}
	return 0
}

func stagePayload(path string, stdin io.Reader) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, stdin)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func (p *deadlineParent) startWorker() (Worker, error) {
	inv := p.inv
	if inv.StartWorker == nil {
		return nil, fmt.Errorf("no worker launcher")
	}
	stdin, err := os.Open(p.payloadPath)
	if err != nil {
		return nil, err
	}
	defer stdin.Close()
	stdout, err := os.OpenFile(p.stdoutPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	defer stdout.Close()
	stderr, err := os.OpenFile(p.stderrPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	defer stderr.Close()
	env := append(withoutEnv(inv.Environ(), stopDeadlineParentEnv, stopDeadlineStartedEnv),
		stopDeadlineParentEnv+"="+strconv.Itoa(inv.Pid),
		stopDeadlineStartedEnv+"="+strconv.FormatInt(p.startedEpoch, 10))
	return inv.StartWorker(p.harnessRoot, inv.Runtime, env, stdin, stdout, stderr)
}

func withoutEnv(environment []string, names ...string) []string {
	out := make([]string, 0, len(environment))
	for _, entry := range environment {
		keep := true
		for _, name := range names {
			if strings.HasPrefix(entry, name+"=") {
				keep = false
			}
		}
		if keep {
			out = append(out, entry)
		}
	}
	return out
}

func workerPid(worker Worker) int {
	if worker == nil {
		return 0
	}
	return worker.Pid()
}

func (p *deadlineParent) running(worker Worker) bool {
	if worker == nil {
		return false
	}
	select {
	case <-worker.Done():
		return false
	default:
		return true
	}
}

func (p *deadlineParent) deadlineDeps(ctx context.Context) StopDeadlineDependencies {
	inv := p.inv
	interval := inv.Deadline.EventInterval
	if interval <= 0 {
		interval = 20 * time.Millisecond
	}
	events := make(chan struct{}, 1)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case events <- struct{}{}:
				default:
				}
			}
		}
	}()
	after := inv.After
	if after == nil {
		after = time.After
	}
	return StopDeadlineDependencies{
		Now: inv.Now, After: after, Events: events, Prober: inv.Deadline.Prober,
		Signal: inv.Deadline.Signal, BootClock: inv.Deadline.BootClock,
	}
}

// waitWorker is the typed wait on the exact worker: it records the worker's
// first exact identity so later cleanup cannot signal a reused pid.
func (p *deadlineParent) waitWorker(worker Worker, boundary StopDeadlineBootBoundary) (StopDeadlineWaitResult, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deps := p.deadlineDeps(ctx)
	if p.inv.Deadline.FixtureDeadline != nil {
		fixture, err := p.inv.Deadline.FixtureDeadline(ctx, p.installation)
		if err != nil {
			return StopDeadlineWaitResult{}, err
		}
		if fixture != nil {
			deps.After = func(time.Duration) <-chan time.Time { return fixture }
		}
	}
	if deps.Prober == nil {
		return StopDeadlineWaitResult{}, fmt.Errorf("process prober is unavailable")
	}
	return WaitForStopDeadline(ctx, int64(worker.Pid()), boundary, deps)
}

// cleanupWorker stops the exact worker after its deadline; a worker that is
// no longer this parent's child is never signalled.
func (p *deadlineParent) cleanupWorker(worker *identity.Ref) StopDeadlineCleanupResult {
	if worker == nil || p.inv.Deadline.Prober == nil {
		return StopDeadlineCleanupResult{State: StopWorkerUnverified}
	}
	prober := p.inv.Deadline.Prober
	exact, state, err := prober.Probe(worker.Pid)
	switch {
	case state == identity.Dead:
		return StopDeadlineCleanupResult{State: StopWorkerGone}
	case err != nil || state == identity.Unknown:
		return StopDeadlineCleanupResult{State: StopWorkerUnverified}
	case exact.Zombie || !identity.SameIdentity(exact, *worker):
		return StopDeadlineCleanupResult{State: StopWorkerGone}
	}
	if parent := p.inv.Deadline.ParentPid; parent != nil {
		if owner, known := parent(worker.Pid); !known || owner != int64(p.inv.Pid) {
			return StopDeadlineCleanupResult{State: StopWorkerUnverified}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return StopDeadlineWorker(ctx, worker, 200*time.Millisecond, p.deadlineDeps(ctx))
}

func (p *deadlineParent) resolved() bool {
	if p.resolverDone == nil {
		return false
	}
	select {
	case <-p.resolverDone:
		return true
	default:
		return false
	}
}

func (p *deadlineParent) resolverRunning() bool {
	return p.resolverDone != nil && !p.resolved()
}

// captureCoordinates adopts the resolver's session and engine-owned state
// root once it has answered.
func (p *deadlineParent) captureCoordinates() {
	if p.fromEngine || !p.resolved() {
		return
	}
	if p.resolvedSession != "" {
		p.session = p.resolvedSession
	}
	if oneLine(p.resolvedRoot) {
		if repo, ok := physicalInstallation(p.resolvedRoot); ok {
			p.repo = repo
		} else {
			p.repo = ""
		}
	}
	p.record = ""
	p.resolveRecord()
	p.fromEngine = true
}

// resolveRecord names the per-session refusal record under the state root.
func (p *deadlineParent) resolveRecord() {
	if p.session == "" || p.repo == "" {
		return
	}
	lines := strings.Split(strings.ToLower(p.session), "\n")
	for index, line := range lines {
		line = slugNonWord.ReplaceAllString(line, "-")
		line = strings.TrimLeft(line, "-.")
		lines[index] = strings.TrimRight(line, "-.")
	}
	slug := trimNewlines(strings.Join(lines, "\n"))
	if slug == "" {
		slug = "session"
	}
	p.record = p.repo.Path("artifacts", "agents", "supervision", "stop-refusals", slug+".json")
}

// payloadSession reads the session without the engine: the first plain
// session_id string in the staged payload.
func payloadSession(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		matches := payloadSessionField.FindAllStringSubmatch(line, -1)
		if len(matches) > 0 {
			return matches[len(matches)-1][1]
		}
	}
	return ""
}

func (p *deadlineParent) logStopOutcome(outcome, measured string) {
	if p.repo == "" {
		return
	}
	directory := p.repo.Path("artifacts", "agents", "supervision")
	_ = os.MkdirAll(directory, 0o755)
	elapsed := measured
	if !digits.MatchString(elapsed) {
		value := p.inv.Now().Unix() - p.startedEpoch
		if value < 0 {
			value = 0
		}
		elapsed = strconv.FormatInt(value, 10)
	} else {
		parsed, _ := strconv.ParseInt(elapsed, 10, 64)
		elapsed = strconv.FormatInt(parsed, 10)
	}
	_ = appendFile(filepath.Join(directory, "hooks.log"), nowStamp(p.inv.Now())+" stop response outcome="+outcome+" elapsed="+elapsed+"s\n")
}

// logStopCondition writes one hook-log line per infrastructure condition the
// parent decides on its own. The root is set only after the resolver
// answers; a failed line is said in the notice, never fatal.
func (p *deadlineParent) logStopCondition(code, component string) {
	p.logFailure = ""
	end := p.startedEpoch + p.budgetSec
	if p.repo == "" {
		p.logFailure = "the infrastructure stop condition could not be logged: the payload named no checkout"
		return
	}
	logPath := p.repo.Path("artifacts", "agents", "supervision", "hooks.log")
	if os.MkdirAll(filepath.Dir(logPath), 0o755) != nil {
		p.logFailure = "the infrastructure stop condition log directory could not be prepared"
		return
	}
	if appendFile(logPath, "stop-condition infrastructure "+code+" "+component+" - "+strconv.FormatInt(end, 10)+" degraded-allow\n") != nil {
		p.logFailure = "the infrastructure stop condition could not be appended to the hook log"
	}
}

func appendFile(path, text string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = file.WriteString(text)
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	return err
}

func readFileString(path string) string {
	data, _ := os.ReadFile(path)
	return string(data)
}

func (p *deadlineParent) appendStderr(text string) { _ = appendFile(p.stderrPath, text) }

func (p *deadlineParent) copyStderr() {
	_, _ = io.WriteString(p.inv.Stderr, readFileString(p.stderrPath))
}

func (p *deadlineParent) published() bool {
	if p.engine == "" {
		return false
	}
	raw := readFileString(p.stdoutPath)
	if raw == "" {
		return false
	}
	return validStopOutput(raw, false)
}

func (p *deadlineParent) removeFiles() {
	for _, name := range []string{"stdout", "stderr", "payload"} {
		_ = os.Remove(filepath.Join(p.dir, name))
	}
	_ = os.Remove(p.dir)
}

// validStopOutput accepts exactly the two provider shapes a Stop publishes: a
// block with a string reason (and, when strict, an optional non-empty string
// message), or a non-empty string system message alone.
func validStopOutput(raw string, strict bool) bool {
	decision, decisionRC := jsonValueStatus(raw, "decision")
	reason, reasonRC := jsonValueStatus(raw, "reason")
	message, messageRC := jsonValueStatus(raw, "systemMessage")
	unknown, shapeRC := jsonStrip(raw, "decision", "reason", "systemMessage")
	if shapeRC != 0 || unknown != "{}" {
		return false
	}
	var object map[string]any
	if json.Unmarshal([]byte(raw), &object) != nil {
		return false
	}
	_, reasonIsString := object["reason"].(string)
	_, messageIsString := object["systemMessage"].(string)
	reasonString := reasonRC == 0 && reasonIsString
	messageString := messageRC == 0 && messageIsString
	block := decision == "block" && reason != "" && reasonString
	if strict {
		block = block && (messageRC != 0 || (message != "" && messageString))
	}
	allow := decisionRC != 0 && reasonRC != 0 && message != "" && messageString
	return block || allow
}
