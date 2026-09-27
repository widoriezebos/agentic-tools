package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/acp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atif"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/jsonedit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

// The Devin runtime adapter: the port of scripts/agents/adapters/devin.sh.
// Two transports share one record lifecycle. The legacy transport runs
// `devin -p` per turn and correlates the session from `devin list`; the ACP
// transport runs `devin acp` as a server child and drives the wire through
// the in-process ACP client (acp.RunFileTurn, the body of the deleted
// `acp turn` verb).

func init() {
	register(runtimeAdapter{
		name:           "devin",
		configIdentity: devinConfigIdentity,
		probe:          devinProbe,
		contract:       commonContract("devin"),
		outputStream:   devinOutputStream,
		supervise:      superviseDevin,
		// `metered`, not `native` or `unavailable`: which of those a Devin
		// turn reports depends on the ACCOUNT, not the runtime. A consumer
		// account reports token counts; an enterprise one reports ACU and no
		// tokens at all. Both must pass, and both must leave the turn
		// measured by something the mission fence can meter.
		//
		// The 900s ceiling is measured, not guessed: a design-critic turn on
		// swe-1-7 ran roughly five minutes and 400k tokens. This runtime ends
		// the turn when a tool is denied (no reply, no report), so the
		// permission leg runs its attempts as their own turns.
		selftest: selftestWith("devin", "metered", "symlinked-skill-discovery", nil, 900, true),
	})
}

// devinTranscriptCeiling is the export size above which a turn's transcript
// is its own named terminal (D64), checked before usage, settlement and
// collection read it.
const devinTranscriptCeiling = 8388608

func devinConfigIdentity(d Deps) (string, error) {
	version, err := d.cliVersion("devin")
	if err != nil {
		return "", err
	}
	configDir := d.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home := d.Getenv("HOME")
		if home == "" {
			return "", fmt.Errorf("HOME: parameter null or not set")
		}
		configDir = filepath.Join(home, ".config")
	}
	project, err := d.projectRoot()
	if err != nil {
		return "", err
	}
	return d.configIdentity("devin", version, []string{
		filepath.Join(configDir, "devin", "config.json"),
		filepath.Join(configDir, "devin", "hooks.v1.json"),
		filepath.Join(project, ".devin", "config.json"),
		filepath.Join(project, ".devin", "config.local.json"),
		filepath.Join(project, ".devin", "hooks.v1.json"),
	})
}

func devinProbe(d Deps, args []string) int {
	if len(args) != 0 {
		registry["devin"].usage(d)
		return 2
	}
	return probeCommon(d, "devin", devinConfigIdentity,
		func() bool { return d.cliAuthenticated("devin", "auth", "status") },
		"devin authentication is unavailable; run devin auth login",
		`["file","stdout","atif","acp"]`,
		`{"resume":true,"sessionEstablishedSignal":false,"sessionEstablishedTimeoutSec":30,"nativeStructuredOutput":false,"nativeEvents":false,"nativeUsage":false,"gracefulCancel":false,"hooks":true,"protocolServer":true,"nativeBudget":false}`,
		`{"unverified": ["readRoots", "writeRoots", "network"]}`)
}

// devinTransport is the transport selector (D81/D82): metasystem.conf's
// dispatch.transport.devin. The shipped template declares acp; an ABSENT
// key still resolves legacy (pre-flip configurations keep their meaning).
// An unreadable configuration or an unrecognized value REFUSES — a broken
// config must never fail open into D61's dangerous path. The second result
// is false on a refusal, whose reason is the first.
func devinTransport(d Deps) (string, bool) {
	value, err := d.configValue("dispatch.transport.devin", "legacy")
	if err != nil {
		return "transport-config-unreadable", false
	}
	switch value {
	case "legacy", "acp":
		return value, true
	}
	return "transport-config-invalid:" + value, false
}

func devinOutputStream(d Deps, roundDir string) (string, error) {
	transport, ok := devinTransport(d)
	if !ok {
		return "", fmt.Errorf("devin transport refused: %s", transport)
	}
	if transport == "acp" {
		return roundDir + "/acp-outcome.json", nil
	}
	return roundDir + "/raw.out", nil
}

func superviseDevin(s *Supervision, args []string) int {
	transport, ok := devinTransport(s.d)
	if !ok {
		fmt.Fprintf(s.d.Stderr, "devin transport refused: %s\n", transport)
		return 1
	}
	if transport == "acp" {
		return superviseDevinACP(s, args)
	}
	return superviseDevinLegacy(s, args)
}

// devinRound is the Devin-specific round state the shell kept in globals
// and supervise's locals, read by the repair hooks.
type devinRound struct {
	s *Supervision
	// configFile is the legacy per-turn --config file, named with the
	// instance tag. Empty on the ACP path, which makes the legacy repair
	// command unreachable there.
	configFile     string
	permissionMode string
	returnFile     string
	// cumulative, previous, and expectPrevious are the usage-delta inputs
	// of this round (empty on the ACP path, as the shell's supervise locals
	// were out of scope there).
	cumulative     string
	previous       string
	expectPrevious bool
}

func (r *devinRound) ports() (delegate.Ports, bool) {
	ports, err := delegate.PortsFor("devin")
	if err != nil {
		fmt.Fprintln(r.s.d.Stderr, err)
		return delegate.Ports{}, false
	}
	return ports, true
}

// usage publishes this round's usage delta from a transcript's cumulative
// session metrics (the former `adapter devin-usage`).
func (r *devinRound) usage(usageFile, transcript, cumulative, previous string, expectPrevious bool, snapshot string) error {
	ports, ok := r.ports()
	if !ok || ports.TurnUsage == nil {
		return errors.New("devin delegate ports provide no turn usage")
	}
	return ports.TurnUsage(usageFile, transcript, snapshot, cumulative, previous, expectPrevious)
}

// settle is the engine's session certification (the former `adapter
// devin-settle`): the derived model, and whether the transcript's session
// agrees with the correlated one. A failure prints nothing, as the verb's
// nonzero exit did.
func (r *devinRound) settle(transcript, snapshot string, requireTranscript bool) (string, bool) {
	ports, ok := r.ports()
	if !ok || ports.Settle == nil {
		return "", false
	}
	model, certified, err := ports.Settle(transcript, snapshot, r.s.sessionID, r.s.roundDir, requireTranscript)
	if err != nil {
		fmt.Fprintln(r.s.d.Stderr, err)
		return "", false
	}
	return model, certified
}

// collect walks the delivery channels (the former `adapter devin-collect`)
// and returns its verdict bytes with the verb's exit status: 0 delivered
// (or any presence-only scan), 3 nothing qualified, 5 transcript over the
// ceiling, 1 mechanical.
func (r *devinRound) collect(in delegate.CollectInputs) ([]byte, int) {
	ports, ok := r.ports()
	if !ok || ports.Collect == nil {
		return nil, 1
	}
	encoded, delivered, err := ports.Collect(in)
	if err != nil {
		fmt.Fprintln(r.s.d.Stderr, err)
		if errors.Is(err, atif.ErrOversize) {
			return nil, 5
		}
		return nil, 1
	}
	if delivered || in.PresenceOnly {
		return encoded, 0
	}
	return encoded, 3
}

// replyPath reads the collect verdict's accepted reply snapshot.
func replyPath(verdict []byte) (string, bool) {
	return jsonedit.Get(verdict, "reply", nil)
}

// repairInvoke is the raw repair invocation: it reports the provider's exit
// alone, so the delivery walk can judge file-only deliveries (D64). It
// resumes the SAME session with the same model, envelope and config — a
// repair that changed any of those would not be a repair.
func (r *devinRound) repairInvoke(promptFile, outputFile string) int {
	s := r.s
	if s.sessionID == "" || r.configFile == "" {
		return 1
	}
	mode := r.permissionMode
	if mode == "" {
		mode = "auto"
	}
	output, err := os.OpenFile(outputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		s.logf("%v\n", err)
		return 1
	}
	defer output.Close()
	path, err := s.d.LookPath("devin")
	if err != nil {
		s.logf("devin: command not found\n")
		return 127
	}
	command := exec.Command(path, "-p",
		"--prompt-file", promptFile,
		"--respect-workspace-trust", "false",
		"--model", s.requestedModel,
		"--permission-mode", mode,
		"--config", r.configFile,
		"-r", s.sessionID,
		"--export", filepath.Join(s.roundDir, "transcript.repair-1.atif.json"))
	command.Args[0] = "devin"
	command.Dir = s.workspace
	command.Env = withEnv(s.childEnv, jobGitQuarantineEnv(s.workspace)...)
	command.Stdout = output
	command.Stderr = s.logWriter()
	return exitStatus(command.Run())
}

// repairTurn is the malformed-reply repair: a repair that exits nonzero has
// failed even if it printed something that parses, so a clean exit AND
// non-empty output are both required.
func (r *devinRound) repairTurn(promptFile, outputFile string) bool {
	if r.repairInvoke(promptFile, outputFile) != 0 {
		return false
	}
	info, err := os.Stat(outputFile)
	return err == nil && info.Size() > 0
}

// usageAfterRepair recomputes this round's usage from the REPAIR
// transcript: the repair resumed the same session, so its transcript
// carries the session totals including the repair turn. No repair
// transcript means the repair's spend cannot be read, so the usage is
// recorded unavailable rather than left undercounting provider budget.
func (r *devinRound) usageAfterRepair(usageFile string) {
	s := r.s
	repairTranscript := filepath.Join(s.roundDir, "transcript.repair-1.atif.json")
	if info, err := os.Stat(repairTranscript); err != nil || info.Size() == 0 {
		if err := adapter.WriteUnavailableUsage(usageFile); err != nil {
			fmt.Fprintln(s.d.Stderr, err)
		}
		return
	}
	cumulative := r.cumulative
	if cumulative == "" {
		cumulative = filepath.Join(s.roundDir, "session-usage.json")
	}
	if err := r.usage(usageFile, repairTranscript, cumulative, r.previous, r.expectPrevious,
		filepath.Join(s.roundDir, "transcript.repair.snapshot")); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
	}
}

// settleAfterRepair certifies the repair transcript. The observed model
// records even when certification fails: the record must reflect what the
// transcript actually named.
func (r *devinRound) settleAfterRepair() bool {
	s := r.s
	observed, certified := r.settle(filepath.Join(s.roundDir, "transcript.repair-1.atif.json"),
		filepath.Join(s.roundDir, "transcript.repair.snapshot"), true)
	if observed != "" {
		s.recordResultEffectiveModel(observed)
	}
	return certified
}

func (r *devinRound) hooks() repairHooks {
	return repairHooks{turn: r.repairTurn, usageAfter: r.usageAfterRepair, settleAfter: r.settleAfterRepair}
}

func (r *devinRound) completeFromCLI(status int, usageFile, candidate, transcript string) int {
	return terminal(r.s.completeFromCLI(status, usageFile, candidate, transcript, r.hooks()))
}

// listSessions runs `devin list --format json` in the workspace into
// output, stderr to the job log, and returns its status.
func (r *devinRound) listSessions(output string) int {
	s := r.s
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		s.logf("%v\n", err)
		return 1
	}
	defer file.Close()
	path, err := s.d.LookPath("devin")
	if err != nil {
		s.logf("devin: command not found\n")
		return 127
	}
	command := exec.Command(path, "list", "--format", "json")
	command.Args[0] = "devin"
	command.Dir = s.workspace
	command.Env = s.childEnv
	command.Stdout = file
	command.Stderr = s.logWriter()
	return exitStatus(command.Run())
}

// previousRoundArtifact is the previous round's copy of a file, if any.
func (s *Supervision) previousRoundArtifact(name string) string {
	round, err := strconv.Atoi(s.round)
	if err != nil || round-1 < 1 {
		return ""
	}
	candidate := filepath.Join(s.d.agents(), s.rootJob, "rounds", strconv.Itoa(round-1), name)
	if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
		return candidate
	}
	return ""
}

func nonEmptyFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

func (s *Supervision) appendEvent(line string) {
	file, err := os.OpenFile(s.events, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return
	}
	defer file.Close()
	fmt.Fprintln(file, line)
}

// writeDevinPrompt writes the schema-augmented prompt copy the CLI reads:
// this runtime has no native structured output, so the schema itself goes
// in the prompt, and the named return file is the delivery channel this
// model actually uses (D62). The dispatcher's prompt stays untouched as
// evidence.
func (s *Supervision) writeDevinPrompt(devinPrompt, returnFile string) bool {
	if err := adapter.DevinPrompt(s.prompt, s.schema, devinPrompt, returnFile); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return false
	}
	return true
}

func superviseDevinLegacy(s *Supervision, args []string) int {
	if !s.prepareOrUsage(args) {
		return 2
	}
	born, ok := fieldOr(s.record, "transport", "")
	if !ok {
		fmt.Fprintf(s.d.Stderr, "cannot read job record %s\n", s.record)
		return 1
	}
	if born == "acp" {
		// D82 fix-forward: a chain born on ACP never rides legacy, whatever
		// the flag says today.
		s.failPending("transport_switch_refused", "setup", "")
		return 1
	}
	// The CLI has no free-form metadata flag. Its per-turn config path is
	// already an argv field and is private to this round, so naming that
	// file with the instance tag gives ownership checks the same exact
	// positional proof.
	r := &devinRound{s: s, configFile: filepath.Join(s.roundDir, s.tag)}
	transcript := filepath.Join(s.roundDir, "transcript.atif.json")
	beforeSessions := filepath.Join(s.roundDir, "devin-sessions-before.json")
	currentSessions := filepath.Join(s.roundDir, "devin-sessions-current.json")
	signalFile := filepath.Join(s.roundDir, "devin-session-signal.json")
	usageFile := filepath.Join(s.roundDir, "usage.json")
	devinPrompt := filepath.Join(s.roundDir, "prompt.devin.md")
	r.returnFile = filepath.Join(s.roundDir, "devin-return.json")
	// A pre-existing named file is evidence of a crashed earlier attempt:
	// surfaced, never clobbered (D64).
	if _, err := os.Lstat(r.returnFile); err == nil {
		s.failPending("stale_named_return", "setup", "")
		return 1
	}
	if !s.writeDevinPrompt(devinPrompt, r.returnFile) {
		return 1
	}
	if err := s.recordWorkspaceWriteScope(); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	if !s.failIfEffectiveWider() {
		return 1
	}
	if err := truncate(s.events); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	if err := truncate(s.raw); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	// --config REPLACES the user configuration, so the job's config is the
	// user's file with the permissions block swapped in; the permission
	// mode rides beside it from the same envelope decision.
	if err := adapter.BuildDevinConfig(s.record, r.configFile, filepath.Join(s.roundDir, "devin-config-provenance.json")); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	mode, err := adapter.DevinPermissionMode(s.record)
	if err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	r.permissionMode = mode
	// A baseline that failed to list is a refusal, not an empty baseline:
	// with no baseline every pre-existing session looks new, which is how a
	// peer's session becomes this job's recorded identity.
	if r.listSessions(beforeSessions) != 0 {
		s.failPending("session_baseline_unavailable", "handshake", "")
		return 1
	}
	// Succeeded but unparseable is not an empty baseline either.
	if data, err := os.ReadFile(beforeSessions); err != nil || !json.Valid(data) {
		s.failPending("session_baseline_unreadable", "handshake", "")
		return 1
	}
	// Since D61 (the human's waiver, 2026-08-15) every dispatch runs the
	// mode the engine decides (`dangerous`).
	command := []string{"devin", "-p",
		"--prompt-file", devinPrompt,
		"--respect-workspace-trust", "false",
		"--model", s.requestedModel,
		"--permission-mode", r.permissionMode,
		"--config", r.configFile,
		"--export", transcript,
	}
	if s.verb == runtimes.SupervisorFollowUp {
		// D2 residual gate: the documented `-r <session-id>` mapping.
		command = append(command, "-r", s.requestedSession)
	}
	if !s.verifyReferences() {
		return 1
	}
	if err := s.markPrefork(); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		s.failPending("prefork_marker", "handshake", "")
		return 1
	}
	// Existing Devin hooks can backfill the signal file from their stable
	// session_id payload; the baseline remains `devin list`.
	env := withEnv(s.childEnv, "METASYSTEM_DEVIN_SESSION_SIGNAL="+signalFile)
	env = withEnv(env, jobGitQuarantineEnv(s.workspace)...)
	cli, err := s.launch(command, env, "", s.raw, nil)
	if err != nil {
		s.failPending("custody_registration", "handshake", "")
		return 1
	}
	if err := s.registerCustody(cli); err != nil {
		cli.terminate()
		s.failPending("custody_registration", "handshake", "")
		return 1
	}

	// Correlation polls from LAUNCH, not from first output: on this runtime
	// stdout is the final reply, and a turn with no reply would otherwise
	// never correlate.
	for cli.alive() {
		resolved := ""
		if s.verb == runtimes.SupervisorFollowUp {
			// A resumed turn correlates nothing: its session is the one
			// being resumed.
			resolved = s.requestedSession
		} else {
			r.listSessions(currentSessions)
			id, candidates := adapter.DevinSessionCorrelate(beforeSessions, currentSessions, signalFile, s.workspace)
			if id == "" && len(candidates) > 1 {
				// Two new sessions in this workspace at once: guessing
				// records a peer's session as this job's identity.
				s.logf("ambiguous-session-correlation:%s\n", strings.Join(candidates, ","))
				s.failPending("ambiguous_session_correlation", "handshake", "")
				cli.terminate()
				return 1
			}
			resolved = id
		}
		if resolved != "" {
			if !s.recordHandshake(resolved, "", s.requestedModel) {
				cli.terminate()
				return 1
			}
			s.appendEvent(fmt.Sprintf(`{"type":"session-correlated","session_id":"%s","predicate":"listed-for-this-workspace-plus-live-process"}`, resolved))
			break
		}
		touch(s.heartbeat)
		s.d.Clock.Sleep(50 * time.Millisecond)
	}
	status, err := s.waitForCLI(cli)
	if err != nil {
		return exitCodeOf(err, 1)
	}
	// D2 residual gate: zero means candidate success and every nonzero
	// value is preserved as the generic runtime_error path.
	s.logf("devin cli exit status=%d\n", status)
	r.cumulative = filepath.Join(s.roundDir, "session-usage.json")
	if s.verb == runtimes.SupervisorFollowUp {
		r.previous = s.previousRoundArtifact("session-usage.json")
		r.expectPrevious = true
	}
	// The transcript ceiling is checked up front (D64): an over-ceiling
	// export is its own named terminal — never identity disagreement, never
	// an empty reply, never a paid repair.
	if info, err := os.Stat(transcript); err == nil && info.Size() > devinTranscriptCeiling {
		s.finishRunning("failed", "transcript_oversize", "delivery", usageFile)
		return 1
	}
	attemptSnapshot := filepath.Join(s.roundDir, "transcript.initial.snapshot")
	if err := r.usage(usageFile, transcript, r.cumulative, r.previous, r.expectPrevious, attemptSnapshot); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	observed, certified := r.settle(transcript, attemptSnapshot, false)
	if observed != "" {
		s.recordResultEffectiveModel(observed)
	}
	// The transcript is authoritative for session identity once the turn
	// ends.
	if s.handshakeDone && !certified {
		s.finishRunning("failed", "session_identity_disagreement", "delivery", usageFile)
		return 1
	}

	// THE DELIVERY WALK (D64): every decision is the engine's collector;
	// this supervisor only routes verdicts.
	if status == 0 && !s.handshakeDone && !nonEmptyFile(s.raw) {
		// No session and empty stdout: the presence scan decides between the
		// two pinned outcomes without any validation.
		presence, _ := r.collect(delegate.CollectInputs{
			Root: s.d.Root, Job: s.job, RoundDir: s.roundDir, RecordPath: s.record,
			StdoutPath: s.raw, NamedPath: r.returnFile, PresenceOnly: true,
		})
		if strings.Contains(string(presence), `"candidatesPresent":true`) {
			s.failPending("handshake_missing_session_id", "handshake", usageFile)
			return 1
		}
		s.failPending("empty_reply", "delivery", usageFile)
		return 1
	}
	if status == 0 && s.handshakeDone {
		verdict, rc := r.collect(delegate.CollectInputs{
			Root: s.d.Root, Job: s.job, RoundDir: s.roundDir, Workspace: s.workspace,
			StdoutPath: s.raw, NamedPath: r.returnFile, TranscriptPath: transcript,
			RecordPath: s.record, Attempt: "initial", Session: s.sessionID,
		})
		switch rc {
		case 0:
			reply, ok := replyPath(verdict)
			if !ok {
				return 1
			}
			return r.completeFromCLI(status, usageFile, reply, transcript)
		case 3:
			return r.deliveryRepair(usageFile)
		case 5:
			s.finishRunning("failed", "transcript_oversize", "delivery", usageFile)
			return 1
		default:
			s.finishRunning("failed", "collect_mechanical", "delivery", usageFile)
			return 1
		}
	}
	return r.completeFromCLI(status, usageFile, s.raw, transcript)
}

// deliveryRepair is the delivery repair (D64): adjudication recommends,
// the durable claim is won BEFORE the paid call, the repair CLI's exit is
// reported separately, and delivery is judged by the post-repair collect
// walk.
func (r *devinRound) deliveryRepair(usageFile string) int {
	s := r.s
	repairNamed := filepath.Join(s.roundDir, "devin-return.repair-1.json")
	repairTranscript := filepath.Join(s.roundDir, "transcript.repair-1.atif.json")
	verdict := s.adjudicate(adapter.AdjudicateParams{
		Stage: "empty-delivery", HandshakeDone: true, NamedRepairPath: repairNamed,
		RepairAvailable: s.returnRepairs == 0 && s.sessionID != "",
	})
	if verdict != "delivery-repair" {
		words := strings.Fields(verdict)
		switch {
		case len(words) >= 3 && words[0] == "fail-pending":
			s.failPending(words[1], words[2], usageFile)
		case len(words) >= 4 && words[0] != "fail-pending":
			s.finishRunning(words[1], words[2], words[3], usageFile)
		}
		return 1
	}
	claim := s.d.Dispatch.Run(s.logWriter(), s.logWriter(), "__repair-claim", "--job", s.job)
	if claim == 3 {
		// The repair is spent (this round or the malformed flow): the
		// pinned empty outcome stands.
		s.finishRunning("failed", "empty_reply", "delivery", usageFile)
		return 1
	} else if claim != 0 {
		s.finishRunning("failed", "collect_mechanical", "delivery", usageFile)
		return 1
	}
	s.returnRepairs = 1
	s.logf("%s delivery repair attempt 1: no return was delivered, asking session %s to write %s\n",
		s.d.nowISO(), s.sessionID, repairNamed)
	repairRC := r.repairInvoke(filepath.Join(s.roundDir, "repair-1.prompt.md"), filepath.Join(s.roundDir, "repair-1.out"))
	r.usageAfterRepair(usageFile)
	violation := filepath.Join(s.roundDir, "protocol-violation.txt")
	if repairRC != 0 {
		_ = os.WriteFile(violation, []byte(fmt.Sprintf("delivery repair provider call failed rc=%d\n", repairRC)), 0o644)
		s.finishProtocolError(violation)
		return 1
	}
	collected, rc := r.collect(delegate.CollectInputs{
		Root: s.d.Root, Job: s.job, RoundDir: s.roundDir, Workspace: s.workspace,
		StdoutPath: filepath.Join(s.roundDir, "repair-1.out"), NamedPath: repairNamed,
		TranscriptPath: repairTranscript, RecordPath: s.record,
		Attempt: "repair", Session: s.sessionID,
	})
	if rc != 0 {
		_ = os.WriteFile(violation, []byte(fmt.Sprintf("delivery repair produced no qualifying return (collect rc=%d)\n", rc)), 0o644)
		s.finishProtocolError(violation)
		return 1
	}
	// Delivered: the repaired session settles before the pipeline runs.
	if !r.settleAfterRepair() {
		s.finishRunning("failed", "session_identity_disagreement", "delivery", usageFile)
		return 1
	}
	reply, ok := replyPath(collected)
	if !ok {
		return 1
	}
	return r.completeFromCLI(0, usageFile, reply, "")
}

// recordACPTransportPin is best-effort after the handshake; the refusal
// guards enforce.
func (s *Supervision) recordACPTransportPin() {
	patch := filepath.Join(s.roundDir, "transport-patch.json")
	if err := adapter.WriteTransportPatch(patch, "acp"); err != nil {
		s.logf("%v\n", err)
		return
	}
	s.d.Dispatch.Run(s.logWriter(), s.logWriter(), "__record-cas", "--job", s.job, "--expect", "running", "--status", "running", "--patch", patch)
}

// signalHooks holds, per installation root, a test's delivery of a
// termination signal to that root's supervisor, so a test signals the
// supervisor it drives without signalling the test process. Production
// roots have no hook and take the real signals.
var signalHooks sync.Map // root -> func(chan<- os.Signal)

// notifyTermination delivers TERM and INT to c and returns the stop.
func notifyTermination(root string, c chan<- os.Signal) (stop func()) {
	if hook, ok := signalHooks.Load(root); ok {
		hook.(func(chan<- os.Signal))(c)
		return func() {}
	}
	signal.Notify(c, syscall.SIGTERM, os.Interrupt)
	return func() { signal.Stop(c) }
}

// acpSignalBound is how long a signalled supervisor waits for the
// in-process client's courtesy session/cancel before it ends regardless.
const acpSignalBound = 3 * time.Second

// signalExitCode is the shell's status for a death by signal.
func signalExitCode(sig os.Signal) int32 {
	if sig == os.Interrupt {
		return 130
	}
	return 143
}

// superviseDevinACP is the ACP transport path: this supervisor owns the
// fifos, the server child, custody and killing; acp.RunFileTurn owns the
// wire (records/acp/acp-transport-design.md). The session id surfaces
// MID-TURN through the session file, so the handshake is recorded inside
// the dispatcher's deadline (critique F1); delivery rides the collector's
// EXCLUSIVE acp channel.
//
// The client is in process: a goroutine of this supervisor, cancelled
// (never signalled) to terminate it. It is not registered in custody — it
// has no process of its own, and this supervisor is already the custody
// root — and the acp-launched event names it with this supervisor's pid.
// The second pre-fork mark stays where the client launch was.
func superviseDevinACP(s *Supervision, args []string) int {
	if !s.prepareOrUsage(args) {
		return 2
	}
	usageFile := filepath.Join(s.roundDir, "usage.json")
	outcomeFile := filepath.Join(s.roundDir, "acp-outcome.json")
	journalFile := filepath.Join(s.roundDir, "acp-journal.log")
	sessionFile := filepath.Join(s.roundDir, "acp-session-id")
	devinPrompt := filepath.Join(s.roundDir, "prompt.devin.md")
	r := &devinRound{s: s}

	// A chain never switches transports (D82 fix-forward): a record already
	// born on one transport refuses the other.
	recorded, ok := fieldOr(s.record, "transport", "")
	if !ok {
		fmt.Fprintf(s.d.Stderr, "cannot read job record %s\n", s.record)
		return 1
	}
	if recorded != "" && recorded != "acp" {
		s.failPending("transport_switch_refused", "setup", "")
		return 1
	}
	r.returnFile = filepath.Join(s.roundDir, "devin-return.json")
	if _, err := os.Lstat(r.returnFile); err == nil {
		s.failPending("stale_named_return", "setup", "")
		return 1
	}
	// The same augmented prompt as the legacy path: its delivery
	// instruction already demands the JSON as the FINAL MESSAGE too, which
	// is exactly the acp channel.
	if !s.writeDevinPrompt(devinPrompt, r.returnFile) {
		return 1
	}
	if err := s.recordWorkspaceWriteScope(); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	if !s.failIfEffectiveWider() {
		return 1
	}
	if err := truncate(s.events); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}

	envelope, err := acp.LoadEnvelopeFile(s.effective)
	if err != nil {
		s.logf("%v\n", err)
		s.failPending("acp_preflight_refused", "setup", "")
		return 1
	}
	if reason := acp.PreflightACP(envelope); reason != "" {
		s.logf("%s\n", reason)
		s.failPending("acp_preflight_refused", "setup", "")
		return 1
	}
	grade, _ := fieldOr(s.effective, "tools", "")
	mode, modeErr := devinACPMode(grade)
	if modeErr != nil {
		s.logf("%v\n", modeErr)
		s.failPending("acp_mode_unmapped", "setup", "")
		return 1
	}
	// The registry's declaration reaches the launch (critique F4): the
	// expectation is queried, never re-defaulted downstream.
	declaration, found := runtimes.Lookup("devin")
	if !found || declaration.ExpectedACP == nil {
		s.failPending("acp_expectation_missing", "setup", "")
		return 1
	}
	expectedProtocol := declaration.ExpectedACP.ExpectedProtocolVersion

	// Per-attempt fifo names (critique F2): never reused.
	pipes, err := ACPPipeNames(s.roundDir)
	if err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	if err := pipes.Make(); err != nil {
		s.logf("%v\n", err)
		s.failPending("acp_fifo_setup", "setup", "")
		return 1
	}
	// Both endpoints dead ⇒ the pair is inert and removed at every exit
	// (KI-42), the custodian's deadline exit included.
	var server, client *child
	defer func() {
		if client != nil {
			client.terminate()
		}
		if server != nil {
			server.terminate()
		}
		pipes.Remove()
	}()

	if !s.verifyReferences() {
		return 1
	}
	if err := s.markPrefork(); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		s.failPending("prefork_marker", "handshake", "")
		return 1
	}
	// argv0 devin-delegate-acp: the census signature distinguishes THIS
	// delegate-side server from the host CLI's internal raw `devin acp`
	// helper (issue #12).
	server, err = s.startACPServer(pipes)
	if err != nil {
		s.failPending("custody_registration", "handshake", "")
		return 1
	}
	if err := s.registerCustody(server); err != nil {
		server.terminate()
		s.failPending("custody_registration", "handshake", "")
		return 1
	}

	turn := acp.FileTurnConfig{
		ServerOut: pipes.ServerOut, ServerIn: pipes.ServerIn,
		JournalPath: journalFile, Workspace: s.workspace,
		EnvelopePath: s.effective, PromptFile: devinPrompt,
		Mode: mode, ExpectedProtocol: expectedProtocol,
		SessionFile: sessionFile,
	}
	// Follow-up rides session/load; the transport never switches
	// mid-chain.
	if s.verb == runtimes.SupervisorFollowUp {
		turn.LoadSession = s.requestedSession
	}
	if err := s.markPrefork(); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		server.terminate()
		s.failPending("prefork_marker", "handshake", "")
		return 1
	}
	if err := truncate(outcomeFile); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	client = startInProcessChild(s.d.Pid, func(ctx context.Context) int {
		return runACPClient(ctx, turn, outcomeFile, s.logWriter())
	})
	s.appendEvent(fmt.Sprintf(`{"type":"acp-launched","server_pid":%d,"client_pid":%d,"client":"in-process","mode":"%s"}`,
		server.pid, client.pid, mode))

	// A TERM or INT lands as the client's cancellation, so the courtesy
	// session/cancel and the typed cancelled outcome still happen; the
	// supervisor then ends with the shell's signal status.
	var signalled atomic.Int32
	finished := make(chan struct{})
	defer close(finished)
	signals := make(chan os.Signal, 1)
	stopSignals := notifyTermination(s.d.Root, signals)
	defer stopSignals()
	go func() {
		select {
		case sig := <-signals:
			code := signalExitCode(sig)
			signalled.Store(code)
			client.stop()
			select {
			case <-finished:
				return
			case <-time.After(acpSignalBound):
			}
			server.terminate()
			pipes.Remove()
			os.Exit(int(code))
		case <-finished:
		}
	}()

	// The handshake loop: the session id lands in the session file at setup
	// success — minutes before the turn settles — and a server that dies
	// before setup bounds the missing-peer path (F2).
	for client.alive() {
		if !s.handshakeDone && nonEmptyFile(sessionFile) {
			session := firstLine(sessionFile)
			if !s.recordHandshake(session, "", "") {
				client.terminate()
				server.terminate()
				return 1
			}
			s.appendEvent(fmt.Sprintf(`{"type":"session-correlated","session_id":"%s","predicate":"acp-wire-typed"}`, session))
			s.recordACPTransportPin()
		}
		if !s.handshakeDone && !server.alive() {
			client.terminate()
			s.failPending("acp_server_died", "handshake", "")
			return 1
		}
		if s.handshakeDone {
			break
		}
		touch(s.heartbeat)
		s.d.Clock.Sleep(50 * time.Millisecond)
	}
	status, err := s.waitForCLI(client)
	if err != nil {
		return exitCodeOf(err, 1)
	}
	if code := signalled.Load(); code != 0 {
		return int(code)
	}
	// The client has closed its ends; releasing the keepers gives the
	// server the stdin EOF it met when the client verb exited.
	pipes.Release()
	server.terminate()
	pipes.Remove()
	s.logf("acp client exit status=%d\n", status)

	// Terminal-writer discipline (critique F7): before the handshake the
	// record is pending; after it the record is running.
	acpTerminal := func(reason, phase string) int {
		if s.handshakeDone {
			s.finishRunning("failed", reason, phase, usageFile)
		} else {
			s.failPending(reason, phase, "")
		}
		return 1
	}
	if status != 0 || !nonEmptyFile(outcomeFile) {
		return acpTerminal("acp_client_mechanical", "delivery")
	}
	if err := usage.ACPUsage(usageFile, outcomeFile); err != nil {
		// A usage owner that cannot even write UNAVAILABLE is a mechanical
		// failure, never silently null usage (critique F9).
		s.logf("%v\n", err)
		return acpTerminal("acp_usage_mechanical", "delivery")
	}
	row, ok := fieldOr(outcomeFile, "row", "")
	if !ok {
		return 1
	}
	// The session comes from the WIRE for both verbs: a follow-up whose load
	// failed has no sessionId in its outcome, and a fabricated handshake is
	// worse than a failed one (critique F6).
	if !s.handshakeDone {
		if session, _ := fieldOr(outcomeFile, "sessionId", ""); session != "" {
			if s.recordHandshake(session, "", "") {
				s.recordACPTransportPin()
			}
		}
	}
	switch row {
	case "delivered":
	case "auth-required":
		return acpTerminal("acp_auth_required", "handshake")
	case "version-mismatch", "setup-error":
		return acpTerminal("acp_"+strings.ReplaceAll(row, "-", "_"), "setup")
	case "protocol-error":
		// Post-handshake protocol deaths are delivery-phase facts; only a
		// pre-session one is setup (critique F7).
		if s.handshakeDone {
			return acpTerminal("acp_protocol_error", "delivery")
		}
		return acpTerminal("acp_protocol_error", "setup")
	default:
		return acpTerminal("acp_"+strings.ReplaceAll(row, "-", "_"), "delivery")
	}
	if !s.handshakeDone {
		return acpTerminal("acp_delivered_without_session", "handshake")
	}
	verdict, rc := r.collect(delegate.CollectInputs{
		Root: s.d.Root, Job: s.job, RoundDir: s.roundDir, Workspace: s.workspace,
		ACPOutcomePath: outcomeFile, RecordPath: s.record,
		Attempt: "initial", Session: s.sessionID,
	})
	switch rc {
	case 0:
		reply, ok := replyPath(verdict)
		if !ok {
			return 1
		}
		return r.completeFromCLI(0, usageFile, reply, "")
	case 3:
		// Repair on ACP is disabled pre-claim (the spec's first no-repair
		// case): no claim is written, the failure is named.
		s.finishRunning("failed", "acp_undelivered", "delivery", usageFile)
		return 1
	default:
		s.finishRunning("failed", "collect_mechanical", "delivery", usageFile)
		return 1
	}
}

// DevinACPMode resolves the adapter-owned dialect: the session mode a tools
// grade maps to (the former `acp mode --runtime devin --tools GRADE`).
func DevinACPMode(grade string) (string, error) { return devinACPMode(grade) }

func devinACPMode(grade string) (string, error) {
	if grade == "" {
		return "", errors.New("usage: acp mode --runtime R --tools GRADE")
	}
	dialect, err := adapter.ACPDialectFor("devin")
	if err != nil {
		return "", err
	}
	mode := dialect.ModeForTools[grade]
	if mode == "" {
		return "", fmt.Errorf("no mode mapped for tools=%s", grade)
	}
	return mode, nil
}

// startACPServer launches `devin acp` with argv0 devin-delegate-acp in the
// workspace under the git quarantine. A server that cannot start fails the
// launch the way the shell's dead subshell failed custody registration.
func (s *Supervision) startACPServer(pipes *ACPPipes) (*child, error) {
	fail := func(err error) (*child, error) {
		s.logf("%v\n", err)
		fmt.Fprintf(s.d.Stderr, "%s child exited before custody identity was recorded\n", s.runtime)
		return nil, err
	}
	path, err := s.d.LookPath("devin")
	if err != nil {
		return fail(err)
	}
	env := withEnv(s.childEnv, jobGitQuarantineEnv(s.workspace)...)
	command, err := pipes.ServerCommand(path, "devin-delegate-acp", s.workspace, env, s.logWriter())
	if err != nil {
		return fail(err)
	}
	c, err := startChild(command)
	pipes.Started()
	if err != nil {
		return fail(err)
	}
	return c, nil
}

// runACPClient is one in-process prompt attempt: the outcome document is
// written to outcomeFile, and the status is the former verb's: 0 with an
// outcome (the cancelled one included), 1 when no attempt ran.
func runACPClient(ctx context.Context, turn acp.FileTurnConfig, outcomeFile string, log io.Writer) int {
	outcome, err := acp.RunFileTurn(ctx, turn)
	if err != nil {
		fmt.Fprintln(log, err)
		return 1
	}
	if err := acp.WriteFileTurnOutcome(outcomeFile, outcome); err != nil {
		fmt.Fprintln(log, err)
		return 1
	}
	return 0
}

// ConfigValue resolves one metasystem.conf key for this installation (the
// host turn's transport selector reads the delegate's key).
func (d Deps) ConfigValue(key, fallback string) (string, error) {
	return d.configValue(key, fallback)
}

// FirstLine is `head -1 FILE`.
func FirstLine(path string) string { return firstLine(path) }

// firstLine is `head -1 FILE`.
func firstLine(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(string(data), "\n")
	return line
}
