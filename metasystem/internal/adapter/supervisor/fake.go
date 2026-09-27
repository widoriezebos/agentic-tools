package supervisor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// The fake runtime: the deterministic protocol simulator the fixture beds
// dispatch instead of a real CLI (formerly scripts/agents/adapters/fake.sh).
// It reads FAKE:<behavior> markers from the round's prompt, and from the
// staged task direction when a body over the directive limit was staged, and
// acts each one out against the real record owners. The fake never sourced
// runtime-common.sh, so its round runs its OWN sequence here, not the shared
// lifecycle's prepare: the order of the record writes, log lines, events, and
// error codes is the fixture beds' contract.

// fakeConfigIdentity is the fake's fixed configuration identity: it has no
// runtime configuration inputs, so fixtures can select snapshots without
// probing.
const fakeConfigIdentity = `{"cliVersion":"fake-1","configHash":"fake-config-v1","configKeyHashes":{},"runtime":"fake"}`

func init() {
	register(runtimeAdapter{
		name:           "fake",
		configIdentity: func(Deps) (string, error) { return fakeConfigIdentity, nil },
		probe:          fakeProbe,
		contract:       fakeContract,
		outputStream: func(_ Deps, roundDir string) (string, error) {
			return roundDir + "/events.jsonl", nil
		},
		supervise:  superviseFake,
		selftest:   fakeSelftest,
		probeUsage: "probe --root ROOT [--profile current|old|unverified-network] [--age-days N]",
	})
}

// fakeRound is one fake delegate round's state beyond the shared
// Supervision paths.
type fakeRound struct {
	*Supervision
	heartbeatInterval time.Duration
	effectivePath     string
}

// fakeInterval validates one millisecond interval the way the script's
// fixture_milliseconds_to_sleep did.
func fakeInterval(d Deps, name string, fallback int) (time.Duration, bool) {
	raw := d.Getenv(name)
	if raw == "" {
		raw = strconv.Itoa(fallback)
	}
	value, err := strconv.Atoi(raw)
	if !positiveIntegerRE.MatchString(raw) || err != nil {
		fmt.Fprintln(d.Stderr, "fake adapter interval must be a positive integer in milliseconds")
		return 0, false
	}
	return time.Duration(value) * time.Millisecond, true
}

// behaviorSources are the files markers are read from: the round's prompt
// and, when a body over the directive limit was staged, its staged task
// direction (the prompt then carries only the reference stanza).
func (r *fakeRound) behaviorSources() []string {
	sources := []string{r.prompt}
	staged := filepath.Join(r.roundDir, "staged", "task-direction.md")
	if info, err := os.Stat(staged); err == nil && info.Mode().IsRegular() {
		sources = append(sources, staged)
	}
	return sources
}

// behaviorPresent is `grep -Fqi FAKE:<name>` over the sources: a
// case-insensitive substring anywhere in either file.
func (r *fakeRound) behaviorPresent(name string) bool {
	needle := strings.ToLower("FAKE:" + name)
	for _, source := range r.behaviorSources() {
		data, err := os.ReadFile(source)
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(string(data)), needle) {
			return true
		}
	}
	return false
}

// shellSpace is the [[:space:]] class within a line.
const shellSpace = " \t\v\f\r"

// behaviorValue is the value of a `FAKE:<name>=value` line: per source, the
// first line (leading whitespace allowed, case-sensitive) carrying the
// marker; the first source whose first such line has a non-empty value wins.
func (r *fakeRound) behaviorValue(name string) string {
	prefix := "FAKE:" + name + "="
	for _, source := range r.behaviorSources() {
		data, err := os.ReadFile(source)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if value, found := strings.CutPrefix(strings.TrimLeft(line, shellSpace), prefix); found {
				if value != "" {
					return value
				}
				break
			}
		}
	}
	return ""
}

// fakeArgument is the first `Fake-Argument:` line of the prompt, captured
// as data and never executed.
func fakeArgument(prompt string) string {
	data, err := os.ReadFile(prompt)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if value, found := strings.CutPrefix(line, "Fake-Argument:"); found {
			return strings.TrimLeft(value, shellSpace)
		}
	}
	return ""
}

func appendText(path, text string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(file, text); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// touchStrict is touch(1): create or bump the mtime, reporting failure.
func touchStrict(path string) error {
	now := time.Now()
	if err := os.Chtimes(path, now, now); err == nil {
		return nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return file.Close()
}

// failed reports an error the way a failing engine verb under `set -e` ended
// the script: its message on stderr and status 1.
func (r *fakeRound) failed(err error) int {
	fmt.Fprintln(r.d.Stderr, err)
	return 1
}

// fakeRecordBuildCachePath mirrors job_build_cache_env for the fake's
// rounds, so they record the same cache path a real runtime's rounds would:
// only a job worktree under artifacts/agents/worktrees whose git dir is a
// linked worktree's, with the go-cache and go-tmp directories made. It
// differs from recordBuildCachePath in exactly the script's ways: no
// staticcheck directory, and a failed mkdir records an empty path.
func fakeRecordBuildCachePath(agents, workspace, roundDir string) {
	cache := ""
	if jobsRoot, ok := realDir(filepath.Join(agents, "worktrees")); ok {
		ws, _ := realDir(workspace)
		if strings.HasPrefix(ws+"/", jobsRoot+"/") {
			if gitdir, ok := gitOutput(workspace, "rev-parse", "--absolute-git-dir"); ok && strings.Contains(gitdir, "/.git/worktrees/") {
				cache = filepath.Join(gitdir, "metasystem-build-cache", "go-cache")
				if os.MkdirAll(cache, 0o755) != nil || os.MkdirAll(filepath.Join(gitdir, "metasystem-build-cache", "go-tmp"), 0o755) != nil {
					cache = ""
				}
			}
		}
	}
	_ = os.WriteFile(filepath.Join(roundDir, "build-cache.txt"), []byte(cache+"\n"), 0o644)
}

// startHold starts an `ENGINE util hold --tag TAG [flags]` child in this
// supervisor's process group; the janitor recognizes it by that argv.
func (r *fakeRound) startHold(flags ...string) (*child, error) {
	command := exec.Command(r.d.Engine, append([]string{"util", "hold", "--tag", r.tag}, flags...)...)
	command.Env = r.d.Environ
	command.Stdout = r.d.Stdout
	command.Stderr = r.d.Stderr
	return startChild(command)
}

// killSelf is `kill -KILL $$`: the supervisor dies without its terminal
// compare-and-swap. Delivery to a multithreaded process is asynchronous, so
// it waits for the signal rather than racing it to a plain exit; the status
// is returned only when the signal could not be sent.
func (r *fakeRound) killSelf() int {
	if err := syscall.Kill(r.d.Pid, syscall.SIGKILL); err != nil {
		fmt.Fprintln(r.d.Stderr, err)
		return 128 + int(syscall.SIGKILL)
	}
	for {
		time.Sleep(time.Second)
	}
}

func (r *fakeRound) writeLost() error {
	return os.WriteFile(r.heartbeat, []byte("{\"lost\":true}\n"), 0o644)
}

// casTerminal lands the running round terminal through the one patch
// writer; a lost compare (3) is not an error.
func (r *fakeRound) casTerminal(target, failure, phase string) int {
	patch := filepath.Join(r.roundDir, "terminal-patch.json")
	usageFile := filepath.Join(r.roundDir, "fake-usage.json")
	if err := adapter.WriteFakeUsage(usageFile); err != nil {
		return r.failed(err)
	}
	if err := adapter.WriteResultPatch(patch, failure, phase, usageFile); err != nil {
		return r.failed(err)
	}
	status := r.recordCAS("running", target, patch)
	if status == 3 {
		return 0
	}
	return status
}

// failRunning is `cas_terminal failed CODE PHASE; exit 1`: a failing
// compare's own status ends the round first.
func (r *fakeRound) failRunning(failure, phase string) int {
	if status := r.casTerminal("failed", failure, phase); status != 0 {
		return status
	}
	return 1
}

// failPendingPatch is the pending-round failure the script wrote under its
// own patch name; the compare's outcome is ignored and the round exits 1.
func (r *fakeRound) failPendingPatch(name, failure, phase string) int {
	patch := filepath.Join(r.roundDir, name)
	if err := adapter.WriteResultPatch(patch, failure, phase, ""); err != nil {
		return r.failed(err)
	}
	r.recordCAS("pending", "failed", patch)
	return 1
}

func (r *fakeRound) writeValidReturn() error {
	ports, err := delegate.PortsFor("fake")
	if err != nil {
		return err
	}
	if ports.Return == nil {
		return errors.New("fake delegate return port is not registered")
	}
	if err := ports.Return(r.record, r.prompt, filepath.Join(r.roundDir, "return.json")); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.roundDir, "return.md"), []byte("# Fake return\n\nCanonical JSON: return.json\n"), 0o644)
}

// assertReturnComplete is `assert-return-complete.sh --job JOB >VIOLATION
// 2>&1`: each violation as "violation: X" in the violation file, true when
// there are none.
func (r *fakeRound) assertReturnComplete(violationPath string) (bool, error) {
	violations := validate.ReturnCompleteJob(r.d.Root, r.job)
	var text strings.Builder
	for _, violation := range violations {
		fmt.Fprintf(&text, "violation: %s\n", violation)
	}
	if err := os.WriteFile(violationPath, []byte(text.String()), 0o644); err != nil {
		return false, err
	}
	return len(violations) == 0, nil
}

// protocolError appends the violation to the job log and lands the round a
// protocol error; its status is the round's.
func (r *fakeRound) protocolError(violationPath string) int {
	if data, err := os.ReadFile(violationPath); err == nil {
		r.logWriter().Write(data)
	}
	return r.dispatch("__protocol-error", "--job", r.job, "--expect", "running", "--violation-file", violationPath)
}

// completeValid writes the canned valid return and lands the round by its
// validation: completed, or a protocol error.
func (r *fakeRound) completeValid() int {
	violation := filepath.Join(r.roundDir, "protocol-violation.txt")
	if err := r.writeValidReturn(); err != nil {
		return r.failed(err)
	}
	ok, err := r.assertReturnComplete(violation)
	if err != nil {
		return r.failed(err)
	}
	if ok {
		os.Remove(violation)
		return r.casTerminal("completed", "null", "completed")
	}
	return r.protocolError(violation)
}

var fakeReferencePathRE = regexp.MustCompile(`^REFERENCE_MISMATCH path=([^ ]*) `)

// verifyReferences is `job verify-references` as the fake ran it: the
// verb's errors into the job log, its mismatch report to the log and stderr,
// and a pending failure naming the first mismatched path.
func (r *fakeRound) verifyReferences() bool {
	absRoot, err := filepath.Abs(r.d.Root)
	if err != nil {
		absRoot = r.d.Root
	}
	composition := filepath.Join(r.roundDir, "composition.json")
	var report []string
	failed := false
	mismatches, err := dispatchcore.VerifyReferences(absRoot, composition)
	switch {
	case err != nil:
		r.logf("%v\n", err)
		failed = true
	case len(mismatches) != 0:
		for _, mismatch := range mismatches {
			report = append(report, mismatch.Line())
		}
		failed = true
	default:
		// The verb re-read the record to print its count; an unreadable
		// record at that point failed the verb.
		var record dispatchcore.CompositionRecord
		data, readErr := os.ReadFile(composition)
		if readErr == nil {
			readErr = json.Unmarshal(data, &record)
		}
		if readErr != nil {
			r.logf("%v\n", readErr)
			failed = true
		}
	}
	if !failed {
		return true
	}
	text := strings.Join(report, "\n") + "\n"
	r.logf("%s", text)
	fmt.Fprint(r.d.Stderr, text)
	path := ""
	if len(report) > 0 {
		if match := fakeReferencePathRE.FindStringSubmatch(report[0]); match != nil {
			path = match[1]
		}
	}
	if path == "" {
		path = "composition"
	}
	r.failPendingPatch("reference-mismatch.json", "reference_mismatch:"+path, "launch")
	return false
}

// heartbeatLoop keeps the liveness sidecar fresh forever: the timeout,
// concurrent-turn, and cap-hold rounds hold until the dispatcher stops them.
func (r *fakeRound) heartbeatLoop() {
	for {
		touch(r.heartbeat)
		r.d.Clock.Sleep(r.heartbeatInterval)
	}
}

// startRecordedHold starts a hold with the round's stopped file and records
// its pid in child.pid.
func (r *fakeRound) startRecordedHold() (*child, error) {
	hold, err := r.startHold("--stopped-file", filepath.Join(r.roundDir, "child.stopped"))
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(r.roundDir, "child.pid"), []byte(strconv.Itoa(hold.pid)+"\n"), 0o644); err != nil {
		return nil, err
	}
	return hold, nil
}

// stopHold is `kill -TERM PID; wait PID`.
func stopHold(hold *child) {
	_ = hold.command.Process.Signal(syscall.SIGTERM)
	hold.wait()
}

func superviseFake(s *Supervision, args []string) int {
	if err := s.parseSupervisorArgs(args); err != nil {
		registry["fake"].usage(s.d)
		return 2
	}
	r := &fakeRound{Supervision: s}
	s.record = filepath.Join(s.d.jobs(), s.job+".json")
	reader, err := s.startReader()
	if err != nil {
		return r.failed(err)
	}
	if err := dispatchcore.ConsumeLaunchCapability(s.d.Root, s.job, s.capability, s.verb, s.tag, int64(s.d.Pid), reader); err != nil {
		return r.failed(err)
	}
	gatePoll, ok := fakeInterval(s.d, "METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS", 10)
	if !ok {
		return 2
	}
	if r.heartbeatInterval, ok = fakeInterval(s.d, "METASYSTEM_HEARTBEAT_INTERVAL_MS", 200); !ok {
		return 2
	}
	gateTimeout := 10
	gateOK := true
	if raw := s.d.Getenv("METASYSTEM_HOST_START_GATE_TIMEOUT_SEC"); raw != "" {
		value, convErr := strconv.Atoi(raw)
		gateTimeout, gateOK = value, convErr == nil && value >= 1
	}
	if gateOK {
		gateOK = adapter.WaitStartGateFile(s.d.Root, s.gate, time.Duration(gateTimeout)*time.Second, gatePoll, s.d.Getenv) == nil
	}
	if !gateOK {
		fmt.Fprintf(s.d.Stderr, "start gate never opened: %s\n", s.gate)
		return 1
	}
	var found bool
	if s.round, found = field(s.record, "round"); !found {
		return r.failed(fmt.Errorf("job record %s has no round", s.record))
	}
	if s.rootJob, err = usage.RootJobID(s.d.jobs(), s.job); err != nil {
		return r.failed(err)
	}
	s.roundDir = filepath.Join(s.d.agents(), s.rootJob, "rounds", s.round)
	s.prompt = filepath.Join(s.roundDir, "prompt.md")
	s.logPath = filepath.Join(s.d.jobs(), s.job+".log")
	s.raw = filepath.Join(s.roundDir, "raw.out")
	s.events = filepath.Join(s.roundDir, "events.jsonl")
	s.heartbeat = filepath.Join(s.d.agents(), "hb", s.job)
	for _, dir := range []string{s.roundDir, filepath.Dir(s.heartbeat)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return r.failed(err)
		}
	}
	workspace, _ := field(s.record, "workspaceRoot")
	fakeRecordBuildCachePath(s.d.agents(), workspace, s.roundDir)
	if err := os.WriteFile(s.logPath, []byte(fmt.Sprintf("fake supervisor started value=%s\n", s.tag)), 0o644); err != nil {
		return r.failed(err)
	}
	if s.log, err = os.OpenFile(s.logPath, os.O_WRONLY|os.O_APPEND, 0o644); err != nil {
		return r.failed(err)
	}
	if err := os.WriteFile(s.raw, []byte("fake raw output\n"), 0o644); err != nil {
		return r.failed(err)
	}
	if err := os.WriteFile(s.heartbeat, []byte(fmt.Sprintf("{\"pid\":%d,\"pgid\":%d,\"instanceTag\":\"%s\"}\n", s.d.Pid, s.d.Pid, s.tag)), 0o644); err != nil {
		return r.failed(err)
	}

	if r.behaviorPresent("pending-process-loss") {
		if err := r.writeLost(); err != nil {
			return r.failed(err)
		}
		return r.killSelf()
	}
	if r.behaviorPresent("no-session-signal") {
		s.logf("ordinary output without a session-established event\n")
		hold, err := r.startHold()
		if err != nil {
			return r.failed(err)
		}
		hold.wait()
	}
	if r.behaviorPresent("handshake-failure") {
		// Through the one patch writer like every other failure path; the
		// patch carries the explicit usage:null.
		return r.failPendingPatch("handshake-failure.json", "authentication_failed", "handshake")
	}

	r.effectivePath = filepath.Join(s.roundDir, "effective-permissions.json")
	if err := adapter.MaterializeEffective(s.record, r.effectivePath); err != nil {
		return r.failed(err)
	}
	if r.behaviorPresent("effective-wider") {
		if err := adapter.SetEffectiveNetwork(r.effectivePath, "allow"); err != nil {
			return r.failed(err)
		}
	}
	if r.behaviorPresent("effective-narrower") {
		if err := adapter.SetEffectiveNetwork(r.effectivePath, "deny"); err != nil {
			return r.failed(err)
		}
	}
	session := "fake-session-" + s.rootJob
	if round, convErr := strconv.Atoi(s.round); convErr == nil && s.verb == "dispatch" && round > 1 {
		session = "fake-session-" + s.rootJob + "-fresh-" + s.round
	}
	if s.verb == "follow-up" {
		if session, found = field(s.record, "sessionId"); !found {
			return r.failed(fmt.Errorf("job record %s has no sessionId", s.record))
		}
	}
	if r.behaviorPresent("missing-session-id") {
		session = ""
	}
	signalName, found := field(s.record, "sessionEstablishedSignal")
	if !found {
		return r.failed(fmt.Errorf("job record %s has no sessionEstablishedSignal", s.record))
	}
	staged := filepath.Join(s.roundDir, "staged", "task-direction.md")
	if s.d.Getenv("METASYSTEM_FAKE_TAMPER_REFERENCE") != "" {
		if info, err := os.Stat(staged); err == nil && info.Mode().IsRegular() {
			if err := appendText(staged, "tampered\n"); err != nil {
				return r.failed(err)
			}
		}
	}
	if !r.verifyReferences() {
		return 1
	}
	model, _ := field(s.record, "requestedModel")
	if s.dispatch("__handshake", "--job", s.job, "--session", session, "--turn", "fake-turn-"+s.round,
		"--model", model, "--effective", r.effectivePath, "--signal", signalName) != 0 {
		return 1
	}
	if !r.behaviorPresent("no-event-stream") {
		if err := appendText(s.events, fmt.Sprintf("{\"event\":\"session-established\",\"sessionId\":\"%s\",\"round\":%s}\n", session, s.round)); err != nil {
			return r.failed(err)
		}
	}

	if s.verb == "follow-up" && session != "fake-session-"+s.rootJob {
		return r.failRunning("resume_collision", "resume")
	}
	if r.behaviorPresent("resume-collision") {
		return r.failRunning("resume_collision", "resume")
	}
	if release := r.behaviorValue("custodial-critique"); release != "" {
		if status, done := r.custodialCritique(release); done {
			return status
		}
	}
	if r.behaviorPresent("process-loss") {
		if _, err := r.startRecordedHold(); err != nil {
			return r.failed(err)
		}
		if err := r.writeLost(); err != nil {
			return r.failed(err)
		}
		return r.killSelf()
	}
	if r.behaviorPresent("return-then-process-loss") {
		// The recollection scenario: the delivered return lands on disk,
		// then the supervisor dies before its terminal compare-and-swap.
		if err := r.writeValidReturn(); err != nil {
			return r.failed(err)
		}
		if err := r.writeLost(); err != nil {
			return r.failed(err)
		}
		return r.killSelf()
	}
	// worktree-file=<path relative to the workspace>: the round writes that
	// file through the guarded write before anything else happens to it, so
	// a round cut off at its cap leaves work in its worktree for the
	// continuation fixtures to find. Round 1 only, so a continuation that
	// inherits the brief through the prior-brief slot cannot recreate what
	// its predecessor wrote; the path is a clean relative path inside the
	// workspace.
	if file := r.behaviorValue("worktree-file"); file != "" && s.round == "1" {
		if status, done := r.worktreeFile(file); done {
			return status
		}
	}
	// cap-hold-round=<n>: hold (as timeout does) in that round only, so a
	// continuation round that inherits the brief completes instead of
	// holding again.
	holdRound := r.behaviorValue("cap-hold-round")
	if r.behaviorPresent("timeout") || r.behaviorPresent("concurrent-turn") || (holdRound != "" && holdRound == s.round) {
		if _, err := r.startRecordedHold(); err != nil {
			return r.failed(err)
		}
		r.heartbeatLoop()
	}
	if r.behaviorPresent("cancel-race") {
		return r.cancelRace()
	}
	if r.behaviorPresent("malformed-return") {
		return r.malformedReturn()
	}
	if r.behaviorPresent("interrupted-atomic-write") {
		if err := os.WriteFile(filepath.Join(s.d.agents(), "record-locks", s.job+".interrupted"), []byte(`{"status":"corrupt`), 0o644); err != nil {
			return r.failed(err)
		}
	}
	if r.behaviorPresent("nested-agent-events") {
		if err := appendText(s.events, "{\"event\":\"agent.completed\",\"agent\":\"nested\",\"topLevel\":false}\n"+
			"{\"event\":\"turn.completed\",\"agent\":\"root\",\"topLevel\":true}\n"); err != nil {
			return r.failed(err)
		}
	} else if !r.behaviorPresent("no-event-stream") {
		if err := appendText(s.events, "{\"event\":\"turn.completed\",\"topLevel\":true}\n"); err != nil {
			return r.failed(err)
		}
	}
	if r.behaviorPresent("hook-unavailable") {
		s.logf("hooks unavailable; polling fallback used\n")
	}
	if r.behaviorPresent("mirror-failure") {
		if err := touchStrict(filepath.Join(s.d.agents(), s.rootJob, ".mirror-fail-once")); err != nil {
			return r.failed(err)
		}
	}
	if argument := fakeArgument(s.prompt); argument != "" {
		if err := appendText(s.raw, fmt.Sprintf("provider argument value=%s\n", argument)); err != nil {
			return r.failed(err)
		}
	}
	return r.completeValid()
}

// custodialCritique holds one registered child until the fixture creates
// the release file, bounded by METASYSTEM_FAKE_CRITIQUE_HOLD_CAP_SEC. done is
// true when the round ends here.
func (r *fakeRound) custodialCritique(release string) (status int, done bool) {
	if !strings.HasPrefix(release, "/") {
		return r.failRunning("invalid_fixture_control", "execute"), true
	}
	rawCap := r.d.Getenv("METASYSTEM_FAKE_CRITIQUE_HOLD_CAP_SEC")
	if rawCap == "" {
		rawCap = "30"
	}
	capSeconds, err := strconv.ParseInt(rawCap, 10, 64)
	if !positiveIntegerRE.MatchString(rawCap) || err != nil {
		return r.failRunning("invalid_fixture_control", "execute"), true
	}
	hold, err := r.startHold()
	if err != nil {
		return r.failed(err), true
	}
	if r.dispatch("__register-custody", "--job", r.job, "--pid", strconv.Itoa(hold.pid)) != 0 {
		stopHold(hold)
		return r.failRunning("custody_registration", "execute"), true
	}
	if err := os.WriteFile(filepath.Join(r.roundDir, "custody-child.pid"), []byte(strconv.Itoa(hold.pid)+"\n"), 0o644); err != nil {
		return r.failed(err), true
	}
	deadline := r.d.Clock.Now().Unix() + capSeconds
	for {
		if _, err := os.Stat(release); err == nil {
			break
		}
		if r.d.Clock.Now().Unix() >= deadline {
			stopHold(hold)
			return r.failRunning("fixture_release_timeout", "execute"), true
		}
		touch(r.heartbeat)
		r.d.Clock.Sleep(r.heartbeatInterval)
	}
	stopHold(hold)
	return 0, false
}

// worktreeFile is the round-1 guarded write into the workspace.
func (r *fakeRound) worktreeFile(file string) (status int, done bool) {
	wrapped := "/" + file + "/"
	if strings.HasPrefix(wrapped, "//") || strings.Contains(wrapped, "/../") || strings.Contains(wrapped, "/./") {
		return r.failRunning("worktree_file_path", "execute"), true
	}
	workspace, found := field(r.record, "workspaceRoot")
	if !found {
		return r.failed(fmt.Errorf("job record %s has no workspaceRoot", r.record)), true
	}
	target := workspace + "/" + file
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return r.failRunning("worktree_write_refused", "execute"), true
	}
	allowed, err := adapter.FakeGuardedWrite(r.effectivePath, target)
	if err != nil {
		fmt.Fprintln(r.d.Stderr, err)
	}
	if err != nil || !allowed {
		return r.failRunning("worktree_write_refused", "execute"), true
	}
	return 0, false
}

// cancelRace holds like timeout, and a SIGTERM completes the round valid:
// the cancel races a completion the record owner must arbitrate.
func (r *fakeRound) cancelRace() int {
	terms := make(chan os.Signal, 1)
	signal.Notify(terms, syscall.SIGTERM)
	defer signal.Stop(terms)
	if _, err := r.startRecordedHold(); err != nil {
		return r.failed(err)
	}
	for {
		touch(r.heartbeat)
		select {
		case <-terms:
			return r.completeValid()
		default:
		}
		r.d.Clock.Sleep(r.heartbeatInterval)
		select {
		case <-terms:
			return r.completeValid()
		default:
		}
	}
}

// malformedReturn writes an unparseable return and lands the validation's
// verdict; the round then exits 0.
func (r *fakeRound) malformedReturn() int {
	if err := os.WriteFile(filepath.Join(r.roundDir, "return.json"), []byte("{malformed\n"), 0o644); err != nil {
		return r.failed(err)
	}
	r.logf("malformed return\n")
	violation := filepath.Join(r.roundDir, "protocol-violation.txt")
	ok, err := r.assertReturnComplete(violation)
	if err != nil {
		return r.failed(err)
	}
	var status int
	if ok {
		status = r.casTerminal("completed", "null", "completed")
	} else {
		status = r.protocolError(violation)
	}
	return status
}

// fakeGuardedStatus is the retired guarded verbs' exit taxonomy: 0 allowed,
// 77 refused by the envelope, 1 an error.
func fakeGuardedStatus(d Deps, allowed bool, err error) int {
	switch {
	case err != nil:
		fmt.Fprintln(d.Stderr, err)
		return 1
	case allowed:
		return 0
	}
	return 77
}

func tempBase(d Deps) string {
	if dir := d.Getenv("TMPDIR"); dir != "" {
		return dir
	}
	return "/tmp"
}

// probeFakeEnvelopeMechanism proves the fake's envelope refuses a denied
// write and a denied network call before a snapshot declares them mapped.
func probeFakeEnvelopeMechanism(d Deps) error {
	dir, err := os.MkdirTemp(tempBase(d), "metasystem-fake-envelope-probe.*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	permissions := filepath.Join(dir, "permissions.json")
	target := filepath.Join(dir, "denied-write.txt")
	if err := os.WriteFile(permissions, []byte("{\"readRoots\":[],\"writeRoots\":[],\"network\":\"deny\"}\n"), 0o644); err != nil {
		return err
	}
	allowed, err := adapter.FakeGuardedWrite(permissions, target)
	writeStatus := fakeGuardedStatus(d, allowed, err)
	allowed, err = adapter.FakeGuardedNetwork(permissions, "127.0.0.1", "9")
	networkStatus := fakeGuardedStatus(d, allowed, err)
	_, statErr := os.Lstat(target)
	if writeStatus != 77 || networkStatus != 77 || statErr == nil {
		return errors.New("fake envelope mechanism did not refuse a denied write and network call")
	}
	if result := d.Getenv("METASYSTEM_FAKE_ENVELOPE_PROBE_RESULT"); result != "" {
		document := fmt.Sprintf("{\"network\":{\"exitStatus\":%d,\"observed\":\"denied\"},\"writeRoots\":{\"exitStatus\":%d,\"observed\":\"denied\"}}\n", networkStatus, writeStatus)
		if err := os.WriteFile(result, []byte(document), 0o644); err != nil {
			return err
		}
	}
	return nil
}

var fixtureScaleRE = regexp.MustCompile(`^([0-9]+)(?:\.([0-9]+))?$`)

// fakeHandshakeSeconds is the simulator's session-established window: the
// adapter-handshake fixture cap (base 2 seconds) scaled by the fixture cap
// scale like every other fixture ceiling, and capped at 60. The scale is
// METASYSTEM_FIXTURE_CAP_SCALE_MILLI, else METASYSTEM_FIXTURE_CAP_SCALE (a
// decimal 1..48, times 1000 rounded up), else 1000: the census calibration
// probe fixture-budget.sh ran when neither was set is retired.
func fakeHandshakeSeconds(d Deps) (int, error) {
	milli := int64(1000)
	if raw := d.Getenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if !positiveIntegerRE.MatchString(raw) || err != nil {
			return 0, errors.New("fixture cap scale is not initialized")
		}
		milli = value
	} else if raw := d.Getenv("METASYSTEM_FIXTURE_CAP_SCALE"); raw != "" {
		invalid := errors.New("METASYSTEM_FIXTURE_CAP_SCALE must be a decimal from 1 through 48")
		match := fixtureScaleRE.FindStringSubmatch(raw)
		if match == nil || len(match[1]) > 6 {
			return 0, invalid
		}
		whole, _ := strconv.ParseInt(match[1], 10, 64)
		fraction := match[2]
		thousandths := int64(0)
		roundUp, fractional := false, false
		for index, digit := range fraction {
			value := int64(digit - '0')
			if value != 0 {
				fractional = true
			}
			if index < 3 {
				thousandths = thousandths*10 + value
			} else if value != 0 {
				roundUp = true
			}
		}
		for index := len(fraction); index < 3; index++ {
			thousandths *= 10
		}
		// 1 <= scale <= 48 on the exact decimal value.
		if whole < 1 || whole > 48 || (whole == 48 && fractional) {
			return 0, invalid
		}
		milli = whole*1000 + thousandths
		if roundUp {
			milli++
		}
	}
	handshake := (2*milli + 999) / 1000
	if handshake > 60 {
		handshake = 60
	}
	return int(handshake), nil
}

var digitsRE = regexp.MustCompile(`^[0-9]+$`)

func fakeProbe(d Deps, args []string) int {
	// Fault hook for the snapshot self-heal fixtures: an unhealable probe.
	if d.Getenv("METASYSTEM_FAKE_PROBE_FAIL") != "" {
		fmt.Fprintln(d.Stderr, "scripted probe failure")
		return 1
	}
	profile, ageRaw := "current", "0"
	for len(args) > 0 {
		if len(args) < 2 {
			registry["fake"].usage(d)
			return 2
		}
		switch args[0] {
		case "--profile":
			profile = args[1]
		case "--age-days":
			ageRaw = args[1]
		default:
			registry["fake"].usage(d)
			return 2
		}
		args = args[2:]
	}
	switch profile {
	case "current", "old", "unverified-network":
	default:
		registry["fake"].usage(d)
		return 2
	}
	ageDays, err := strconv.Atoi(ageRaw)
	if !digitsRE.MatchString(ageRaw) || err != nil {
		registry["fake"].usage(d)
		return 2
	}
	if err := probeFakeEnvelopeMechanism(d); err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	handshake, err := fakeHandshakeSeconds(d)
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	path, err := adapter.WriteFakeCapabilitySnapshot(filepath.Join(d.agents(), "capabilities"), profile, ageDays, handshake)
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	fmt.Fprintln(d.Stdout, path)
	return 0
}

// fakeContract rides the fake's real construction path — the profile-driven
// snapshot writer — with the deterministic current profile: no shared
// lifecycle helper, because the standalone shape is exactly what fake
// proves possible.
func fakeContract(d Deps) ([]byte, error) {
	dir, err := os.MkdirTemp(tempBase(d), "metasystem-contract.*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	path, err := adapter.WriteFakeCapabilitySnapshot(dir, "current", 0, 1)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

// designBrief is the brief template with its Working Mode line set to
// design (the script's sed).
func designBrief(template string) ([]byte, error) {
	data, err := os.ReadFile(template)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	for index, line := range lines {
		if strings.HasPrefix(line, "Working Mode:") {
			lines[index] = "Working Mode: design"
		}
	}
	return []byte(strings.Join(lines, "\n")), nil
}

// engineDelegate runs `ENGINE internal delegate ARGS` with extra environment
// and returns its status.
func engineDelegate(d Deps, stdout io.Writer, env []string, args ...string) int {
	command := exec.Command(d.Engine, append([]string{"internal", "delegate"}, args...)...)
	command.Env = withEnv(d.Environ, env...)
	command.Stdout = stdout
	command.Stderr = d.Stderr
	return exitStatus(command.Run())
}

// fakeSelftest drives the full protocol sequence through the engine's
// delegate front door — a design dispatch, its follow-up, and a held round
// cancelled — then records the pass.
func fakeSelftest(d Deps) int {
	quiet := d
	quiet.Stdout = discard{}
	if code := fakeProbe(quiet, nil); code != 0 {
		return code
	}
	dir, err := os.MkdirTemp(tempBase(d), "metasystem-fake-selftest.*")
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	id := fmt.Sprintf("fake-selftest-%s-%d", d.Clock.Now().UTC().Format("20060102t150405z"), d.Pid)
	templates := filepath.Join(d.Root, "scripts", "agents", "templates")
	rootEnv := "METASYSTEM_DELEGATE_ROOT=" + d.Root
	internalEnv := "METASYSTEM_DELEGATE_SELFTEST_INTERNAL=1"

	brief, err := designBrief(filepath.Join(templates, "brief.md"))
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, "brief.md"), brief, 0o644)
	}
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	if code := engineDelegate(d, d.Stdout, []string{rootEnv, internalEnv}, "--adapter-selftest", "fake",
		"--brief", filepath.Join(dir, "brief.md"), "--workspace", d.Root, "--op", id, "--wait"); code != 0 {
		return code
	}
	follow, err := os.ReadFile(filepath.Join(templates, "follow-up.md"))
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, "follow.md"), follow, 0o644)
	}
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	if code := engineDelegate(d, d.Stdout, []string{rootEnv}, "--follow-up", id,
		"--brief", filepath.Join(dir, "follow.md"), "--wait"); code != 0 {
		return code
	}
	cancel, err := designBrief(filepath.Join(templates, "brief.md"))
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, "cancel.md"), append(cancel, []byte("\nFAKE:timeout\n")...), 0o644)
	}
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	if code := engineDelegate(d, discard{}, []string{rootEnv, internalEnv}, "--adapter-selftest", "fake",
		"--brief", filepath.Join(dir, "cancel.md"), "--workspace", d.Root, "--op", id+"-cancel"); code != 0 {
		return code
	}
	if code := engineDelegate(d, d.Stdout, []string{rootEnv}, "--cancel", id+"-cancel"); code != 0 {
		return code
	}
	selftests := filepath.Join(d.agents(), "selftests")
	if err := os.MkdirAll(selftests, 0o755); err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	if err := adapter.WriteFakeSelftestRecord(filepath.Join(selftests, id+".json"), id); err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	fmt.Fprintln(d.Stdout, "fake adapter selftest passed: full protocol sequence and denied-envelope mechanism probes")
	return 0
}
