package supervisor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/jsonedit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/returnschema"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

// exit is a request to end the supervisor with a status, raised from deep in
// the lifecycle (a deadline enforced by the custodian, a record that cannot be
// read) the way the shell adapters called `exit`.
type exit struct{ code int }

func (e exit) Error() string { return fmt.Sprintf("supervisor exit %d", e.code) }

// exitCodeOf maps a lifecycle error to the supervisor's exit status.
func exitCodeOf(err error, otherwise int) int {
	var request exit
	if errors.As(err, &request) {
		return request.code
	}
	return otherwise
}

// errPrepare marks a failure before the round's evidence exists; the entry
// answers it with its usage text and status 2, as the shell adapters did.
var errPrepare = errors.New("supervision could not be prepared")

// Supervision is one delegate round's state: the paths the round writes and
// the handshake facts the lifecycle decides on.
type Supervision struct {
	d       Deps
	runtime string
	verb    string
	// usage writes the runtime's usage text (a failed preparation).
	usage func(Deps)

	job, gate, tag, capability string

	record, roundDir, prompt, logPath, raw, events string
	heartbeat, effective, schema, workspace        string
	round, rootJob                                 string
	requestedModel, requestedSession               string

	handshakeDone  bool
	sessionID      string
	effectiveModel string
	returnRepairs  int

	// log is the round's job log, open for appending; children write
	// their stderr to it.
	log *os.File
	// childEnv is the environment every runtime child starts from: the
	// supervisor's own plus the hook delegate context.
	childEnv []string

	deadlinesCached   bool
	handshakeDeadline int64
	capDeadline       string
}

var positiveIntegerRE = regexp.MustCompile(`^[1-9][0-9]*$`)

// intervalMS reads a positive-integer millisecond interval from the
// environment.
func (d Deps) intervalMS(name string, fallback int) (time.Duration, error) {
	raw := d.Getenv(name)
	if raw == "" {
		return time.Duration(fallback) * time.Millisecond, nil
	}
	if !positiveIntegerRE.MatchString(raw) {
		return 0, fmt.Errorf("adapter interval must be a positive integer in milliseconds")
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("adapter interval must be a positive integer in milliseconds")
	}
	return time.Duration(value) * time.Millisecond, nil
}

func (d Deps) nowISO() string { return d.Clock.Now().UTC().Format("2006-01-02T15:04:05Z") }

// field reads one dotted field of a JSON file the way the shell adapters'
// `json get` did: strings raw, numbers without a trailing .0, null as "null".
func field(path, name string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return jsonedit.Get(data, name, nil)
}

// fieldOr reads a field, returning fallback when it is absent or null.
func fieldOr(path, name, fallback string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return jsonedit.Get(data, name, &fallback)
}

func touch(path string) {
	now := time.Now()
	if err := os.Chtimes(path, now, now); err == nil {
		return
	}
	if file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		file.Close()
	}
}

func truncate(path string) error {
	return os.WriteFile(path, nil, 0o644)
}

// parseSupervisorArgs reads the round's launch arguments; all four are
// required.
func (s *Supervision) parseSupervisorArgs(args []string) error {
	for len(args) > 0 {
		if len(args) < 2 {
			return errPrepare
		}
		value := args[1]
		switch args[0] {
		case "--job":
			s.job = value
		case "--start-gate":
			s.gate = value
		case "--instance-tag":
			s.tag = value
		case "--launch-capability":
			s.capability = value
		default:
			return errPrepare
		}
		args = args[2:]
	}
	if s.job == "" || s.gate == "" || s.tag == "" || s.capability == "" {
		return errPrepare
	}
	return nil
}

func (s *Supervision) startReader() (identity.StartReader, error) {
	authorization, err := fixtureauth.New(s.d.Root)
	if err != nil {
		return nil, err
	}
	return identity.FixtureStartReader{Kernel: identity.KernelProber{}, Fixture: authorization.Identity()}, nil
}

// prepare is runtime-common's prepare_supervision: spend the launch
// capability, wait for the dispatcher's start gate, and lay out the round.
func (s *Supervision) prepare(args []string) error {
	if err := s.parseSupervisorArgs(args); err != nil {
		return err
	}
	s.record = filepath.Join(s.d.jobs(), s.job+".json")
	reader, err := s.startReader()
	if err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return errPrepare
	}
	if err := dispatchcore.ConsumeLaunchCapability(s.d.Root, s.job, s.capability, s.verb, s.tag, int64(s.d.Pid), reader); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return errPrepare
	}
	if tag, _ := field(s.record, "instanceTag"); tag != s.tag {
		fmt.Fprintf(s.d.Stderr, "%s adapter instance tag does not match job %s\n", s.runtime, s.job)
		return errPrepare
	}
	// Evidence-location hints are initialized only inside a job-bound
	// supervisor. Provider children, resumes, and secondary delivery-repair
	// children inherit them; a hook still verifies current process ancestry
	// against exact custody.
	s.childEnv = append(append([]string(nil), s.d.Environ...),
		"METASYSTEM_HOOK_DELEGATE_STATE_ROOT="+s.d.Root,
		"METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT="+s.d.Root,
		"METASYSTEM_HOOK_DELEGATE_JOB="+s.job,
	)
	poll, err := s.d.intervalMS("METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS", 10)
	if err != nil {
		fmt.Fprintf(s.d.Stderr, "%s %v\n", s.runtime, err)
		return errPrepare
	}
	// Capped like every host's wait: a dispatcher that dies before opening
	// the gate must not leave an immortal supervisor sleeping forever with
	// no heartbeat and no handshake deadline.
	gateTimeout := 10
	if raw := s.d.Getenv("METASYSTEM_HOST_START_GATE_TIMEOUT_SEC"); raw != "" {
		value, convErr := strconv.Atoi(raw)
		if convErr != nil || value < 1 {
			fmt.Fprintf(s.d.Stderr, "start gate never opened: %s\n", s.gate)
			return errPrepare
		}
		gateTimeout = value
	}
	if err := adapter.WaitStartGateFile(s.d.Root, s.gate, time.Duration(gateTimeout)*time.Second, poll, s.d.Getenv); err != nil {
		fmt.Fprintf(s.d.Stderr, "start gate never opened: %s\n", s.gate)
		return errPrepare
	}
	s.round, _ = field(s.record, "round")
	s.rootJob, err = usage.RootJobID(s.d.jobs(), s.job)
	if err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return errPrepare
	}
	s.roundDir = filepath.Join(s.d.agents(), s.rootJob, "rounds", s.round)
	s.prompt = filepath.Join(s.roundDir, "prompt.md")
	s.logPath = filepath.Join(s.d.jobs(), s.job+".log")
	s.raw = filepath.Join(s.roundDir, "raw.out")
	s.events = filepath.Join(s.roundDir, "events.jsonl")
	s.heartbeat = filepath.Join(s.d.agents(), "hb", s.job)
	s.effective = filepath.Join(s.roundDir, "effective-permissions.json")
	role, _ := field(s.record, "role")
	schemaVersion := 2
	switch role {
	case "design-critic", "code-critic", "warden":
		schemaVersion = 4
	}
	s.schema = filepath.Join(s.roundDir, fmt.Sprintf("return-schema.v%d.json", schemaVersion))
	if err := returnschema.Materialize(s.d.Root, role, schemaVersion, s.schema); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return errPrepare
	}
	s.workspace, _ = field(s.record, "workspaceRoot")
	s.requestedModel, _ = field(s.record, "requestedModel")
	if session, ok := field(s.record, "sessionId"); ok && session != "null" {
		s.requestedSession = session
	}
	if err := os.MkdirAll(s.roundDir, 0o755); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return errPrepare
	}
	if err := os.MkdirAll(filepath.Dir(s.heartbeat), 0o755); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return errPrepare
	}
	if err := os.WriteFile(s.logPath, []byte(fmt.Sprintf("%s adapter supervisor started value=%s\n", s.runtime, s.tag)), 0o644); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return errPrepare
	}
	s.log, err = os.OpenFile(s.logPath, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return errPrepare
	}
	if err := os.WriteFile(s.heartbeat, []byte(fmt.Sprintf("{\"pid\":%d,\"pgid\":%d,\"instanceTag\":\"%s\"}\n", s.d.Pid, s.d.Pid, s.tag)), 0o644); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return errPrepare
	}
	if err := adapter.MaterializeEffective(s.record, s.effective); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return errPrepare
	}
	return nil
}

func (s *Supervision) close() {
	if s.log != nil {
		s.log.Close()
	}
}

// logf appends one line to the round's job log.
func (s *Supervision) logf(format string, args ...any) {
	if s.log != nil {
		fmt.Fprintf(s.log, format, args...)
	}
}

func (s *Supervision) logWriter() io.Writer {
	if s.log == nil {
		return io.Discard
	}
	return s.log
}

// dispatch runs one delegate lifecycle callback with the supervisor's own
// output streams.
func (s *Supervision) dispatch(args ...string) int {
	return s.d.Dispatch.Run(s.d.Stdout, s.d.Stderr, args...)
}

func (s *Supervision) recordCAS(expect, status, patch string) int {
	return s.dispatch("__record-cas", "--job", s.job, "--expect", expect, "--status", status, "--patch", patch)
}

// markPrefork persists the pre-fork custody marker: advance evidence for the
// interval before the child's exact identity reaches custody. Its writer is
// this supervisor, and the child inherits this supervisor's process group.
func (s *Supervision) markPrefork() error {
	reader, err := s.startReader()
	if err != nil {
		return err
	}
	return dispatchcore.WritePreforkMarker(s.d.Root, s.job, s.tag, int64(s.d.Pid), int64(s.d.Pid), reader)
}

// registerCustody records the child's exact identity in the job's custody
// before anything else happens to it, polling for up to five seconds while
// the child lives.
func (s *Supervision) registerCustody(c *child) error {
	poll, err := s.d.intervalMS("METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS", 20)
	if err != nil {
		return err
	}
	deadline := s.d.Clock.Now().Add(5 * time.Second)
	for c.alive() {
		if s.dispatch("__register-custody", "--job", s.job, "--pid", strconv.Itoa(c.pid)) == 0 {
			return nil
		}
		if !s.d.Clock.Now().Before(deadline) {
			fmt.Fprintf(s.d.Stderr, "%s child custody registration ceiling reached for pid %d\n", s.runtime, c.pid)
			return errors.New("custody registration ceiling")
		}
		s.d.Clock.Sleep(poll)
	}
	fmt.Fprintf(s.d.Stderr, "%s child exited before custody identity was recorded\n", s.runtime)
	return errors.New("child exited before custody")
}

// failIfEffectiveWider refuses a launch whose effective grant is wider than
// the request. An unreadable comparison refuses too (fail closed): a launch
// never proceeds on a grant nobody could compare.
func (s *Supervision) failIfEffectiveWider() bool {
	mismatch, err := adapter.ComparePermissions(s.record, s.effective)
	if err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		s.failPending("permissions_check_unreadable", "handshake", "")
		return false
	}
	if mismatch != "" {
		s.failPending("permissions_mismatch:"+mismatch, "handshake", "")
		return false
	}
	return true
}

// verifyReferences re-reads every referenced file against the composition
// record before the launch; a mismatch fails the round pending.
func (s *Supervision) verifyReferences() bool {
	absRoot, err := filepath.Abs(s.d.Root)
	if err != nil {
		absRoot = s.d.Root
	}
	mismatches, err := dispatchcore.VerifyReferences(absRoot, filepath.Join(s.roundDir, "composition.json"))
	if err == nil && len(mismatches) == 0 {
		return true
	}
	first := ""
	if err != nil {
		s.logf("%v\n", err)
	}
	for _, mismatch := range mismatches {
		line := mismatch.Line()
		s.logf("%s\n", line)
		fmt.Fprintln(s.d.Stderr, line)
		if first == "" {
			first = referencePath(line)
		}
	}
	if first == "" {
		first = "composition"
	}
	s.failPending("reference_mismatch:"+first, "launch", "")
	return false
}

var referenceMismatchRE = regexp.MustCompile(`^REFERENCE_MISMATCH path=([^ ]*) `)

func referencePath(line string) string {
	if match := referenceMismatchRE.FindStringSubmatch(line); match != nil {
		return match[1]
	}
	return ""
}

// recordHandshake publishes the correlated session through the lifecycle's
// handshake callback. A session that differs from the one being resumed is
// a resume collision.
func (s *Supervision) recordHandshake(session, turn, model string) bool {
	if model == "" {
		model = s.requestedModel
	}
	if session == "" {
		return false
	}
	if s.requestedSession != "" && session != s.requestedSession {
		s.failPending("resume_collision", "resume", "")
		return false
	}
	signal, _ := field(s.record, "sessionEstablishedSignal")
	if s.dispatch("__handshake", "--job", s.job, "--session", session, "--turn", turn,
		"--model", model, "--effective", s.effective, "--signal", signal) != 0 {
		return false
	}
	s.sessionID = session
	s.effectiveModel = model
	s.handshakeDone = true
	return true
}

// settleResultIdentity is the three-way decision after the CLI exits: a late
// handshake adopts the observed session, an observed session that differs
// from the handshaken one is a resume collision, and a completed result's
// model is recorded when the runtime reports one.
func (s *Supervision) settleResultIdentity(observedSession, observedTurn, handshakeModel, resultModel, usageFile string) bool {
	switch {
	case !s.handshakeDone && observedSession != "":
		return s.recordHandshake(observedSession, observedTurn, handshakeModel)
	case s.handshakeDone && observedSession != "" && observedSession != s.sessionID:
		s.finishRunning("failed", "resume_collision", "resume", usageFile)
		return false
	case s.handshakeDone && resultModel != "":
		return s.recordResultEffectiveModel(resultModel)
	}
	return true
}

// recordResultEffectiveModel records the effective model the completed
// runtime result reports.
func (s *Supervision) recordResultEffectiveModel(model string) bool {
	if model == "" {
		return false
	}
	patch := filepath.Join(s.roundDir, "result-model-patch.json")
	if err := adapter.WriteModelPatch(patch, model); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return false
	}
	if s.recordCAS("running", "running", patch) != 0 {
		return false
	}
	s.effectiveModel = model
	return true
}

func (s *Supervision) writePatch(output, failure, phase, usageFile string) {
	if err := adapter.WriteResultPatch(output, failure, phase, usageFile); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
	}
}

// addPatchField sets one top-level field of a JSON patch file.
func addPatchField(path, key string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var patch map[string]any
	if err := json.Unmarshal(data, &patch); err != nil {
		return fmt.Errorf("patch %s: %w", path, err)
	}
	patch[key] = value
	encoded, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o644)
}

// failPending lands a pending round failed; a lost compare (3) is not an
// error: the record already moved on.
func (s *Supervision) failPending(failure, phase, usageFile string) bool {
	patch := filepath.Join(s.roundDir, "pending-failure.json")
	s.writePatch(patch, failure, phase, usageFile)
	status := s.recordCAS("pending", "failed", patch)
	return status == 0 || status == 3
}

// finishRunning lands a running round terminal.
func (s *Supervision) finishRunning(target, failure, phase, usageFile string) bool {
	patch := filepath.Join(s.roundDir, "terminal-patch.json")
	s.writePatch(patch, failure, phase, usageFile)
	status := s.recordCAS("running", target, patch)
	return status == 0 || status == 3
}

func (s *Supervision) finishProtocolError(violation string) bool {
	status := s.dispatch("__protocol-error", "--job", s.job, "--expect", "running", "--violation-file", violation)
	return status == 0 || status == 3
}

func (s *Supervision) recordReturnRepairs(count int) {
	patch := filepath.Join(s.roundDir, "repair-count-patch.json")
	if err := adapter.WriteRepairsPatch(patch, count); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return
	}
	s.recordCAS("running", "running", patch)
}

// The custodian enforces the record's OWN deadlines from inside (F4, D32).
// The deadlines are immutable once stamped, so they are read ONCE, bounded;
// a record that cannot be read within the bound fails closed (sweep and
// exit) rather than waiting unbounded on an unreadable record.
// handshakeDeadline is epoch seconds; capDeadline is a UTC ISO second,
// compared lexically against a same-format now — sound for one fixed format.
func (s *Supervision) cacheRecordDeadlines() error {
	if s.deadlinesCached {
		return nil
	}
	for attempt := 0; attempt < 3; attempt++ {
		handshake, ok := fieldOr(s.record, "handshakeDeadline", "")
		if ok {
			s.capDeadline, _ = fieldOr(s.record, "capDeadline", "")
			s.deadlinesCached = true
			if positiveIntegerRE.MatchString(handshake) {
				s.handshakeDeadline, _ = strconv.ParseInt(handshake, 10, 64)
			}
			return nil
		}
		s.d.Clock.Sleep(200 * time.Millisecond)
	}
	s.logf("%s deadline cache failed: record unreadable; failing closed\n", s.d.nowISO())
	s.sweepKillDomain()
	return exit{1}
}

// sweepKillDomain stops every member of this supervisor's process group but
// itself (membership survives reparenting, so orphaned grandchildren stay
// enumerable). The result is the DEATH PROOF: true only when no member but
// this process remains. An indeterminable enumeration refuses — a sweep must
// never act on an undercount.
func (s *Supervision) sweepKillDomain() bool {
	for _, signal := range []string{"TERM", "KILL"} {
		members, err := s.d.GroupMembers(s.d.Pid, s.d.Pid)
		if err != nil {
			return false
		}
		if len(members) == 0 {
			return true
		}
		for _, pid := range members {
			signalPid(pid, signal)
		}
		deadline := s.d.Clock.Now().Add(2 * time.Second)
		for s.d.Clock.Now().Before(deadline) {
			members, err = s.d.GroupMembers(s.d.Pid, s.d.Pid)
			if err != nil {
				return false
			}
			if len(members) == 0 {
				return true
			}
			s.d.Clock.Sleep(50 * time.Millisecond)
		}
	}
	members, err := s.d.GroupMembers(s.d.Pid, s.d.Pid)
	return err == nil && len(members) == 0
}

// enforceExpiredDeadline is one expired deadline's enforcement: stand down
// BEFORE ANY SIGNAL on the in-process handshake state; sweep; land the
// terminal verdict only behind the death proof. An unproven domain leaves
// the record nonterminal — logged, retried next tick, census-visible.
func (s *Supervision) enforceExpiredDeadline(kind string, cli *child) error {
	if kind == "handshake" && s.handshakeDone {
		return nil
	}
	// The direct child FIRST, through the path that also REAPS it: an
	// unreaped zombie keeps its group membership, and the sweep could never
	// prove a domain that still holds it.
	cli.terminate()
	if !s.sweepKillDomain() {
		s.logf("%s %s-deadline sweep left the kill domain unproven; record stays nonterminal\n", s.d.nowISO(), kind)
		return nil
	}
	provenAt := s.d.nowISO()
	if kind == "handshake" {
		s.failPending("handshake_timeout", "handshake", "")
	} else {
		// The reaper's and waiter's spelling, with the reaper's death
		// proof: one record reads one way whoever lands the cap first.
		patch := filepath.Join(s.roundDir, "terminal-patch.json")
		s.writePatch(patch, "budget-cap", "supervision", "")
		if err := addPatchField(patch, "groupDeathProvenAt", provenAt); err != nil {
			fmt.Fprintln(s.d.Stderr, err)
		}
		s.recordCAS("running", "timeout", patch)
	}
	s.logf("%s %s deadline enforced by the custodian (D32)\n", s.d.nowISO(), kind)
	return exit{0}
}

// checkRecordDeadlines is one tick's deadline verdicts; it may end the
// supervisor.
func (s *Supervision) checkRecordDeadlines(cli *child) error {
	if err := s.cacheRecordDeadlines(); err != nil {
		return err
	}
	if s.handshakeDeadline > 0 && !s.handshakeDone {
		if s.d.Clock.Now().Unix() > s.handshakeDeadline {
			if err := s.enforceExpiredDeadline("handshake", cli); err != nil {
				return err
			}
		}
	}
	if s.capDeadline != "" && s.handshakeDone {
		if s.d.nowISO() > s.capDeadline {
			if err := s.enforceExpiredDeadline("cap", cli); err != nil {
				return err
			}
		}
	}
	return nil
}

// waitForCLI keeps the liveness sidecar fresh and the record's deadlines
// enforced while the child runs, then returns its exit status.
func (s *Supervision) waitForCLI(cli *child) (int, error) {
	interval, err := s.d.intervalMS("METASYSTEM_HEARTBEAT_INTERVAL_MS", 100)
	if err != nil {
		return 0, err
	}
	for cli.alive() {
		touch(s.heartbeat)
		if err := s.checkRecordDeadlines(cli); err != nil {
			return 0, err
		}
		s.d.Clock.Sleep(interval)
	}
	return cli.wait(), nil
}

// adjudicate is one stage of the terminal-outcome state machine
// (adapter.AdjudicateTurn): the engine validates the candidate, chooses every
// error code and phase name, and decides whether the one bounded repair turn
// runs.
func (s *Supervision) adjudicate(p adapter.AdjudicateParams) string {
	p.Root = s.d.Root
	p.Job = s.job
	p.RecordPath = s.record
	p.SessionID = s.sessionID
	p.SchemaPath = s.schema
	p.ReturnPath = filepath.Join(s.roundDir, "return.json")
	p.MarkdownPath = filepath.Join(s.roundDir, "return.md")
	p.ViolationPath = filepath.Join(s.roundDir, "protocol-violation.txt")
	p.RepairPromptPath = filepath.Join(s.roundDir, "repair-1.prompt.md")
	p.LogPath = s.logPath
	verdict, err := adapter.AdjudicateTurn(p)
	if err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return ""
	}
	return verdict
}

// repairHooks are a runtime's optional repair-turn behaviors.
type repairHooks struct {
	// turn runs the one repair turn in the same session and reports
	// success.
	turn func(promptFile, outputFile string) bool
	// usageAfter recomputes the round's usage after a repair.
	usageAfter func(usageFile string)
	// settleAfter re-certifies session and model identity from the repair
	// transcript.
	settleAfter func() bool
}

func (s *Supervision) appendViolationToLog() {
	if data, err := os.ReadFile(filepath.Join(s.roundDir, "protocol-violation.txt")); err == nil {
		s.log.Write(data)
	}
}

// completeFromCLI runs the terminal-outcome state machine for a finished
// CLI turn and lands its verdict. It returns true for a completed round.
func (s *Supervision) completeFromCLI(status int, usageFile, candidate, transcript string, hooks repairHooks) bool {
	violation := filepath.Join(s.roundDir, "protocol-violation.txt")
	repairAvailable := hooks.turn != nil && s.returnRepairs == 0 && s.sessionID != ""
	verdict := s.adjudicate(adapter.AdjudicateParams{
		Stage: "initial", CLIStatus: int64(status), CandidatePath: candidate, TranscriptPath: transcript,
		RepairAvailable: repairAvailable, HandshakeDone: s.handshakeDone,
	})
	switch {
	case verdict == "finish completed null completed":
		os.Remove(violation)
		s.finishRunning("completed", "null", "completed", usageFile)
		return true
	case strings.HasPrefix(verdict, "fail-pending "):
		words := strings.Fields(verdict)
		if len(words) >= 3 {
			s.failPending(words[1], words[2], usageFile)
		}
		return false
	case strings.HasPrefix(verdict, "finish "):
		words := strings.Fields(verdict)
		if len(words) >= 4 {
			s.finishRunning(words[1], words[2], words[3], usageFile)
		}
		return false
	case verdict == "protocol-error":
		s.appendViolationToLog()
		s.finishProtocolError(violation)
		return false
	case verdict == "repair" && hooks.turn != nil:
	default:
		s.logf("adjudicate-turn returned an unknown verdict: %s\n", verdict)
		return false
	}

	s.appendViolationToLog()
	s.logf("%s return repair attempt 1: reply did not validate, asking again in session %s\n", s.d.nowISO(), s.sessionID)
	s.returnRepairs = 1
	repairRC := 0
	if !hooks.turn(filepath.Join(s.roundDir, "repair-1.prompt.md"), filepath.Join(s.roundDir, "repair-1.out")) {
		repairRC = 1
	}
	// The repair RAN, whether or not it produced a usable reply. Record it
	// and re-fence its usage BEFORE branching on the outcome: a failed repair
	// still spent provider budget on a cumulative-usage runtime.
	s.recordReturnRepairs(1)
	if hooks.usageAfter != nil {
		hooks.usageAfter(usageFile)
	}
	verdict = s.adjudicate(adapter.AdjudicateParams{
		Stage: "after-repair", RepairRC: int64(repairRC),
		RepairCandidate: filepath.Join(s.roundDir, "repair-1.out"), SettleAvailable: hooks.settleAfter != nil,
	})
	if verdict == "settle" {
		// The repair transcript is now the final turn, so it — not the
		// pre-repair transcript — is authoritative for session and model
		// identity.
		verdict = s.adjudicate(adapter.AdjudicateParams{Stage: "settle-result", SettleOK: hooks.settleAfter()})
	}
	switch {
	case verdict == "finish completed null completed":
		s.logf("%s return repaired in session %s; the first reply is kept as evidence\n", s.d.nowISO(), s.sessionID)
		os.Remove(violation)
		s.finishRunning("completed", "null", "completed", usageFile)
		return true
	case strings.HasPrefix(verdict, "finish "):
		words := strings.Fields(verdict)
		if len(words) >= 4 {
			s.finishRunning(words[1], words[2], words[3], usageFile)
		}
		return false
	default:
		s.appendViolationToLog()
		s.finishProtocolError(violation)
		return false
	}
}

// terminal writes a round's status as the shell adapters' supervise returned
// it: 0 for a completed round, 1 otherwise.
func terminal(completed bool) int {
	if completed {
		return 0
	}
	return 1
}
