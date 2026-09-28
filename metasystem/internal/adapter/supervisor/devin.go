package supervisor

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/acp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atif"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/host"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/jsonedit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

// The Devin runtime's operations: the port of scripts/agents/adapters/devin.sh
// (role delegate) and scripts/agents/hosts/devin.sh (role host). Two
// transports share one lifecycle. The legacy transport runs `devin -p` per
// turn and correlates the session from `devin list`; the ACP transport
// declares a protocol launch (`devin acp` as the server child) that the
// shared layer drives with its in-process client.

func init() {
	ops := devinOps{builtinOps{name: "devin", cli: "devin", usage: "metered", host: true, repair: true,
		configIdentity: devinConfigIdentity, probe: devinProbe, contract: commonContract("devin"),
		// `metered`, not `native` or `unavailable`: which of those a Devin
		// turn reports depends on the ACCOUNT, not the runtime (tokens on a
		// consumer account, ACU on an enterprise one).
		//
		// The 900s ceiling is measured, not guessed: a design-critic turn on
		// swe-1-7 ran roughly five minutes and 400k tokens. This runtime ends
		// the turn when a tool is denied, so the permission leg runs its
		// attempts as their own turns.
		selftest: selftestWith("devin", "metered", "symlinked-skill-discovery", nil, 900, true),
		// The per-turn --config file is named with the instance tag (the
		// claim-bound invocation shape, runtimes.CLIInvocations).
	}}
	register(runtimeAdapter{
		name:           "devin",
		configIdentity: ops.configIdentity,
		probe:          ops.probe,
		contract:       ops.contract,
		outputStream:   devinOutputStream,
		selftest:       ops.selftest,
		ops:            ops,
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
// dispatch.transport.devin, for delegate rounds and host turns alike. An
// ABSENT key resolves legacy (pre-flip configurations keep their meaning);
// an unreadable configuration or an unrecognized value REFUSES — a broken
// config must never fail open into D61's dangerous path. The second result
// is false on a refusal, whose reason is the first.
func devinTransport(d Deps) (string, bool) {
	value, err := d.configValue("dispatch.transport.devin", config.MustDefault("dispatch.transport.devin"))
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

// devinOps is the Devin runtime's operations.
type devinOps struct{ builtinOps }

// devinTurn is one Devin turn's state, handed back to observe, finalize and
// repair.
type devinTurn struct {
	transport  string
	returnFile string
	prompt     string
	// Legacy: the per-turn --config file (named with the instance tag on a
	// delegate round; empty on the ACP path, which makes the legacy repair
	// command unreachable there), the permission mode, the transcript, the
	// session listings, and the usage-delta inputs.
	configFile, permissionMode, transcript string
	before, current, signal                string
	cumulative, previous                   string
	expectPrevious                         bool
	// ACP: the typed outcome and the wire's session file.
	outcome, sessionFile string
	// Host: the raw capture, the return, the usage file.
	raw, hostReturn, usage string
}

// Admit refuses a round whose transport configuration cannot be resolved,
// before anything is spent.
func (devinOps) Admit(d Deps, _ string) error {
	if transport, ok := devinTransport(d); !ok {
		return fmt.Errorf("devin transport refused: %s", transport)
	}
	return nil
}

func (o devinOps) Prepare(t *Turn) (Launch, error) {
	d := t.Deps()
	transport, ok := devinTransport(d)
	if t.Role == RoleHost {
		if !ok {
			message := "devin host: transport configuration unreadable"
			if value, invalid := strings.CutPrefix(transport, "transport-config-invalid:"); invalid {
				message = "devin host: transport configuration invalid: " + value
			}
			return Launch{EarlyRefusal: &Refusal{Error: message}}, nil
		}
		return o.prepareHost(t, transport)
	}
	if !ok {
		return Launch{}, fmt.Errorf("devin transport refused: %s", transport)
	}
	p := &devinTurn{transport: transport,
		returnFile: filepath.Join(t.Dir, "devin-return.json"),
		prompt:     filepath.Join(t.Dir, "prompt.devin.md")}
	// A chain never switches transports (D82 fix-forward): a record already
	// born on one transport refuses the other.
	recorded, readable := fieldOr(t.Record, "transport", "")
	if !readable {
		return Launch{}, fmt.Errorf("cannot read job record %s", t.Record)
	}
	if (transport == "acp" && recorded != "" && recorded != "acp") || (transport == "legacy" && recorded == "acp") {
		return Launch{EarlyRefusal: &Refusal{Error: "transport_switch_refused", Phase: "setup"}}, nil
	}
	// A pre-existing named file is evidence of a crashed earlier attempt:
	// surfaced, never clobbered (D64).
	if _, err := os.Lstat(p.returnFile); err == nil {
		return Launch{EarlyRefusal: &Refusal{Error: "stale_named_return", Phase: "setup"}}, nil
	}
	// This runtime has no native structured output: the schema itself goes
	// in the prompt, and the named return file is the delivery channel this
	// model actually uses (D62). The dispatcher's prompt stays untouched.
	if err := adapter.DevinPrompt(t.Prompt, t.Schema, p.prompt, p.returnFile); err != nil {
		return Launch{}, err
	}
	// The working directory is the write boundary.
	if err := adapter.RewriteWriteScope(t.Effective, t.Workspace); err != nil {
		return Launch{}, err
	}
	if err := truncate(t.Events); err != nil {
		return Launch{}, err
	}
	if transport == "acp" {
		return o.prepareACP(t, p)
	}
	return o.prepareLegacy(t, p)
}

func (devinOps) prepareLegacy(t *Turn, p *devinTurn) (Launch, error) {
	d := t.Deps()
	raw := filepath.Join(t.Dir, "raw.out")
	if err := truncate(raw); err != nil {
		return Launch{}, err
	}
	// The CLI has no free-form metadata flag. Its per-turn config path is
	// already an argv field and is private to this round, so naming that
	// file with the instance tag gives ownership checks the same exact
	// positional proof.
	p.configFile = filepath.Join(t.Dir, t.Tag)
	p.transcript = filepath.Join(t.Dir, "transcript.atif.json")
	p.before = filepath.Join(t.Dir, "devin-sessions-before.json")
	p.current = filepath.Join(t.Dir, "devin-sessions-current.json")
	p.signal = filepath.Join(t.Dir, "devin-session-signal.json")
	p.cumulative = filepath.Join(t.Dir, "session-usage.json")
	if t.Verb == runtimes.SupervisorFollowUp {
		p.previous = previousRoundArtifact(d, t, "session-usage.json")
		p.expectPrevious = true
	}
	// Since D61 (the human's waiver, 2026-08-15) every dispatch runs the
	// mode the engine decides (`dangerous`); an unreadable record refuses.
	mode, err := adapter.DevinPermissionMode(t.Record)
	if err != nil {
		return Launch{}, err
	}
	p.permissionMode = mode
	command := []string{"devin", "-p",
		"--prompt-file", p.prompt,
		"--respect-workspace-trust", "false",
		"--model", t.Model,
		"--permission-mode", p.permissionMode,
		"--config", p.configFile,
		"--export", p.transcript,
	}
	if t.Verb == runtimes.SupervisorFollowUp {
		// D2 residual gate: the documented `-r <session-id>` mapping.
		command = append(command, "-r", t.ResumeSession)
	}
	// Existing Devin hooks can backfill the signal file from their stable
	// session_id payload; the baseline remains `devin list`.
	env := withEnv([]string{"METASYSTEM_DEVIN_SESSION_SIGNAL=" + p.signal}, jobGitQuarantineEnv(d.git(), t.Workspace)...)
	return Launch{Argv: command, Env: env, StdoutPath: raw, Private: p,
		BeforeLaunch: func() (*Refusal, error) {
			// --config REPLACES the user configuration, so the job's config
			// is the user's file with the permissions block swapped in.
			if err := adapter.BuildDevinConfig(t.Record, p.configFile, filepath.Join(t.Dir, "devin-config-provenance.json")); err != nil {
				return nil, err
			}
			// A baseline that failed to list is a refusal, not an empty
			// baseline: with no baseline every pre-existing session looks
			// new, which is how a peer's session becomes this job's
			// recorded identity. Succeeded but unparseable is not an empty
			// baseline either.
			if listSessions(t, p.before) != 0 {
				return &Refusal{Error: "session_baseline_unavailable", Phase: "handshake"}, nil
			}
			if data, err := os.ReadFile(p.before); err != nil || !json.Valid(data) {
				return &Refusal{Error: "session_baseline_unreadable", Phase: "handshake"}, nil
			}
			return nil, nil
		}}, nil
}

func (devinOps) prepareACP(t *Turn, p *devinTurn) (Launch, error) {
	d := t.Deps()
	p.outcome = filepath.Join(t.Dir, "acp-outcome.json")
	p.sessionFile = filepath.Join(t.Dir, "acp-session-id")
	protocol, refusal := devinProtocol(t, t.Effective)
	if refusal != "" {
		return Launch{Refusal: &Refusal{Error: refusal, Phase: "setup"}, Private: p}, nil
	}
	protocol.Journal = filepath.Join(t.Dir, "acp-journal.log")
	protocol.Outcome, protocol.SessionFile, protocol.PromptFile = p.outcome, p.sessionFile, p.prompt
	// Follow-up rides session/load; the transport never switches mid-chain.
	if t.Verb == runtimes.SupervisorFollowUp {
		protocol.LoadSession = t.ResumeSession
	}
	// argv0 devin-delegate-acp: the census signature distinguishes THIS
	// delegate-side server from the host CLI's internal raw `devin acp`
	// helper (issue #12).
	return Launch{Argv: []string{"devin", "acp"}, Argv0: "devin-delegate-acp",
		Env: jobGitQuarantineEnv(d.git(), t.Workspace), Protocol: protocol, Private: p}, nil
}

// devinProtocol is the ACP launch for an envelope: the pre-launch envelope
// check, the dialect's session mode for its tools grade, and the registry's
// protocol expectation (queried, never re-defaulted downstream — critique
// F4). A refusal names the delegate's error code; the host maps it to its
// message.
func devinProtocol(t *Turn, envelopePath string) (*ProtocolLaunch, string) {
	envelope, err := acp.LoadEnvelopeFile(envelopePath)
	if err != nil {
		fmt.Fprintln(t.Log, err)
		return nil, "acp_preflight_refused"
	}
	if reason := acp.PreflightACP(envelope); reason != "" {
		fmt.Fprintln(t.Log, reason)
		return nil, "acp_preflight_refused"
	}
	grade, _ := fieldOr(envelopePath, "tools", "")
	mode, err := devinACPMode(grade)
	if err != nil {
		fmt.Fprintln(t.Log, err)
		return nil, "acp_mode_unmapped"
	}
	declaration, found := runtimes.Lookup("devin")
	if !found || declaration.ExpectedACP == nil {
		return nil, "acp_expectation_missing"
	}
	return &ProtocolLaunch{Kind: "acp", Envelope: envelopePath, Mode: mode,
		ExpectedProtocol: declaration.ExpectedACP.ExpectedProtocolVersion}, ""
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

// Observe correlates the turn's session: on the legacy transport from the
// session listing (a resumed turn's session is the one being resumed), on
// ACP from the wire's session file, which lands at setup success —
// minutes before the turn settles.
func (devinOps) Observe(t *Turn, o Observation) (Events, error) {
	p := o.Launch.Private.(*devinTurn)
	if p.transport == "acp" {
		if t.HandshakeDone || !nonEmptyFile(p.sessionFile) {
			return Events{}, nil
		}
		session := firstLine(p.sessionFile)
		return Events{Session: session,
			Lines:       []string{fmt.Sprintf(`{"type":"session-correlated","session_id":"%s","predicate":"acp-wire-typed"}`, session)},
			RecordPatch: transportPin(t)}, nil
	}
	resolved := ""
	if t.Verb == runtimes.SupervisorFollowUp {
		resolved = t.ResumeSession
	} else {
		listSessions(t, p.current)
		id, candidates := adapter.DevinSessionCorrelate(p.before, p.current, p.signal, t.Workspace)
		if id == "" && len(candidates) > 1 {
			// Two new sessions in this workspace at once: guessing records
			// a peer's session as this job's identity.
			fmt.Fprintf(t.Log, "ambiguous-session-correlation:%s\n", strings.Join(candidates, ","))
			return Events{Refusal: &Refusal{Error: "ambiguous_session_correlation", Phase: "handshake"}}, nil
		}
		resolved = id
	}
	if resolved == "" {
		return Events{}, nil
	}
	return Events{Session: resolved, Model: t.Model,
		Lines: []string{fmt.Sprintf(`{"type":"session-correlated","session_id":"%s","predicate":"listed-for-this-workspace-plus-live-process"}`, resolved)}}, nil
}

// transportPin writes the acp transport patch and names it; a patch that
// cannot be written is logged and skipped (best-effort; the refusal guards
// enforce).
func transportPin(t *Turn) string {
	patch := filepath.Join(t.Dir, "transport-patch.json")
	if err := adapter.WriteTransportPatch(patch, "acp"); err != nil {
		fmt.Fprintln(t.Log, err)
		return ""
	}
	return patch
}

func (o devinOps) Finalize(t *Turn, in FinalInput) (Final, error) {
	p := in.Launch.Private.(*devinTurn)
	if t.Role == RoleHost {
		if p.transport == "acp" {
			return o.finalizeHostACP(t, p, in)
		}
		return o.finalizeHostLegacy(t, p, in)
	}
	if p.transport == "acp" {
		return o.finalizeACP(t, p, in)
	}
	return o.finalizeLegacy(t, p, in)
}

func (devinOps) finalizeLegacy(t *Turn, p *devinTurn, in FinalInput) (Final, error) {
	status := in.Status
	// D2 residual gate: zero means candidate success and every nonzero
	// value is preserved as the generic runtime_error path.
	fmt.Fprintf(t.Log, "devin cli exit status=%d\n", status)
	// The transcript ceiling is checked up front (D64): an over-ceiling
	// export is its own named terminal — never identity disagreement, never
	// an empty reply, never a paid repair.
	if info, err := os.Stat(p.transcript); err == nil && info.Size() > devinTranscriptCeiling {
		return Final{Refusal: &Refusal{Error: "transcript_oversize", Phase: "delivery"}}, nil
	}
	attemptSnapshot := filepath.Join(t.Dir, "transcript.initial.snapshot")
	if err := devinUsage(in.Usage, p.transcript, p.cumulative, p.previous, p.expectPrevious, attemptSnapshot); err != nil {
		return Final{}, err
	}
	observed, certified := devinSettle(t, p.transcript, attemptSnapshot, false)
	final := Final{RecordModel: observed}
	// The transcript is authoritative for session identity once the turn
	// ends.
	if t.HandshakeDone && !certified {
		final.Refusal = &Refusal{Error: "session_identity_disagreement", Phase: "delivery"}
		return final, nil
	}
	raw := in.Launch.StdoutPath
	// THE DELIVERY WALK (D64): every decision is the engine's collector;
	// the runtime only routes verdicts.
	if status == 0 && !t.HandshakeDone && !nonEmptyFile(raw) {
		// No session and empty stdout: the presence scan decides between
		// the two pinned outcomes without any validation.
		presence, _ := devinCollect(t, delegate.CollectInputs{
			Root: t.Root, Job: t.Job, RoundDir: t.Dir, RecordPath: t.Record,
			StdoutPath: raw, NamedPath: p.returnFile, PresenceOnly: true,
		})
		if strings.Contains(string(presence), `"candidatesPresent":true`) {
			final.Refusal = &Refusal{Error: "handshake_missing_session_id", Phase: "handshake"}
		} else {
			final.Refusal = &Refusal{Error: "empty_reply", Phase: "delivery"}
		}
		return final, nil
	}
	if status == 0 && t.HandshakeDone {
		verdict, rc := devinCollect(t, delegate.CollectInputs{
			Root: t.Root, Job: t.Job, RoundDir: t.Dir, Workspace: t.Workspace,
			StdoutPath: raw, NamedPath: p.returnFile, TranscriptPath: p.transcript,
			RecordPath: t.Record, Attempt: "initial", Session: t.SessionID,
		})
		switch rc {
		case 0:
			reply, ok := jsonedit.Get(verdict, "reply", nil)
			if !ok {
				return Final{}, errors.New("devin collect verdict names no reply")
			}
			final.Candidate, final.Transcript = reply, p.transcript
		case 3:
			final.DeliveryRepair = true
		case 5:
			final.Refusal = &Refusal{Error: "transcript_oversize", Phase: "delivery"}
		default:
			final.Refusal = &Refusal{Error: "collect_mechanical", Phase: "delivery"}
		}
		return final, nil
	}
	final.Candidate, final.Transcript = raw, p.transcript
	return final, nil
}

// finalizeACP maps the typed outcome. The terminal-writer discipline
// (critique F7) is the shared layer's: pending before the handshake,
// running after it.
func (devinOps) finalizeACP(t *Turn, p *devinTurn, in FinalInput) (Final, error) {
	if in.Status != 0 || !nonEmptyFile(p.outcome) {
		return Final{Refusal: &Refusal{Error: "acp_client_mechanical", Phase: "delivery"}}, nil
	}
	if err := usage.ACPUsage(in.Usage, p.outcome); err != nil {
		// A usage owner that cannot even write UNAVAILABLE is a mechanical
		// failure, never silently null usage (critique F9).
		fmt.Fprintln(t.Log, err)
		return Final{Refusal: &Refusal{Error: "acp_usage_mechanical", Phase: "delivery"}}, nil
	}
	row, ok := fieldOr(p.outcome, "row", "")
	if !ok {
		return Final{}, fmt.Errorf("cannot read the acp outcome %s", p.outcome)
	}
	final := Final{}
	session, handshaken := t.SessionID, t.HandshakeDone
	// The session comes from the WIRE for both verbs: a follow-up whose load
	// failed has no sessionId in its outcome, and a fabricated handshake is
	// worse than a failed one (critique F6).
	if !handshaken {
		if wire, _ := fieldOr(p.outcome, "sessionId", ""); wire != "" {
			// Recorded before the collector reads the job record, whose
			// session the return must match; a shared layer that does not
			// record in place takes it from the answer.
			events := Events{Session: wire, RecordPatch: transportPin(t)}
			if !t.Handshake(events) {
				final.Handshake = &events
			}
			session, handshaken = wire, true
		}
	}
	switch row {
	case "delivered":
	case "auth-required":
		final.Refusal = &Refusal{Error: "acp_auth_required", Phase: "handshake"}
	case "version-mismatch", "setup-error":
		final.Refusal = &Refusal{Error: "acp_" + strings.ReplaceAll(row, "-", "_"), Phase: "setup"}
	case "protocol-error":
		// Post-handshake protocol deaths are delivery-phase facts; only a
		// pre-session one is setup (critique F7).
		phase := "setup"
		if handshaken {
			phase = "delivery"
		}
		final.Refusal = &Refusal{Error: "acp_protocol_error", Phase: phase}
	default:
		final.Refusal = &Refusal{Error: "acp_" + strings.ReplaceAll(row, "-", "_"), Phase: "delivery"}
	}
	if final.Refusal != nil {
		return final, nil
	}
	if !handshaken {
		final.Refusal = &Refusal{Error: "acp_delivered_without_session", Phase: "handshake"}
		return final, nil
	}
	verdict, rc := devinCollect(t, delegate.CollectInputs{
		Root: t.Root, Job: t.Job, RoundDir: t.Dir, Workspace: t.Workspace,
		ACPOutcomePath: p.outcome, RecordPath: t.Record, Attempt: "initial", Session: session,
	})
	switch rc {
	case 0:
		reply, ok := jsonedit.Get(verdict, "reply", nil)
		if !ok {
			return Final{}, errors.New("devin collect verdict names no reply")
		}
		zero := 0
		final.Candidate, final.Status = reply, &zero
	case 3:
		// Repair on ACP is disabled pre-claim (the spec's first no-repair
		// case): no claim is written, the failure is named.
		final.Refusal = &Refusal{Error: "acp_undelivered", Phase: "delivery"}
	default:
		final.Refusal = &Refusal{Error: "collect_mechanical", Phase: "delivery"}
	}
	return final, nil
}

// Repair is the repair in the same session: the shape repair turn, its
// usage and settlement, and the delivery repair (D64).
func (devinOps) Repair(t *Turn, in RepairInput) RepairResult {
	p := in.Launch.Private.(*devinTurn)
	repairTranscript := filepath.Join(t.Dir, "transcript.repair-1.atif.json")
	switch in.Stage {
	case RepairTurn:
		return RepairResult{Status: repairInvoke(t, p, in.PromptFile, in.OutputFile)}
	case RepairUsage:
		usageAfterRepair(t, p, in.Usage)
		return RepairResult{}
	case RepairSettle:
		// The observed model records even when certification fails: the
		// record must reflect what the transcript actually named.
		model, certified := devinSettle(t, repairTranscript, filepath.Join(t.Dir, "transcript.repair.snapshot"), true)
		return RepairResult{Settled: certified, Model: model}
	case RepairNamedPath:
		return RepairResult{NamedPath: filepath.Join(t.Dir, "devin-return.repair-1.json")}
	case RepairDelivery:
		rc := repairInvoke(t, p, in.PromptFile, in.OutputFile)
		usageAfterRepair(t, p, in.Usage)
		if rc != 0 {
			return RepairResult{Status: rc}
		}
		verdict, collected := devinCollect(t, delegate.CollectInputs{
			Root: t.Root, Job: t.Job, RoundDir: t.Dir, Workspace: t.Workspace,
			StdoutPath: in.OutputFile, NamedPath: in.NamedPath, TranscriptPath: repairTranscript,
			RecordPath: t.Record, Attempt: "repair", Session: t.SessionID,
		})
		if collected != 0 {
			return RepairResult{Violation: fmt.Sprintf("delivery repair produced no qualifying return (collect rc=%d)", collected)}
		}
		reply, _ := jsonedit.Get(verdict, "reply", nil)
		return RepairResult{Candidate: reply}
	}
	return RepairResult{Status: 1}
}

// repairInvoke is the raw repair invocation: it reports the provider's exit
// alone. It resumes the SAME session with the same model, envelope and
// config — a repair that changed any of those would not be a repair.
func repairInvoke(t *Turn, p *devinTurn, promptFile, outputFile string) int {
	d := t.Deps()
	if t.SessionID == "" || p.configFile == "" {
		return 1
	}
	mode := p.permissionMode
	if mode == "" {
		mode = "auto"
	}
	output, err := os.OpenFile(outputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		fmt.Fprintln(t.Log, err)
		return 1
	}
	defer output.Close()
	path, err := d.LookPath("devin")
	if err != nil {
		fmt.Fprintln(t.Log, "devin: command not found")
		return 127
	}
	command := exec.Command(path, "-p",
		"--prompt-file", promptFile,
		"--respect-workspace-trust", "false",
		"--model", t.Model,
		"--permission-mode", mode,
		"--config", p.configFile,
		"-r", t.SessionID,
		"--export", filepath.Join(t.Dir, "transcript.repair-1.atif.json"))
	command.Args[0] = "devin"
	command.Dir = t.Workspace
	command.Env = withEnv(t.Env, jobGitQuarantineEnv(d.git(), t.Workspace)...)
	command.Stdout = output
	command.Stderr = t.Log
	return exitStatus(command.Run())
}

// usageAfterRepair recomputes the round's usage from the REPAIR transcript:
// the repair resumed the same session, so its transcript carries the
// session totals including the repair turn. No repair transcript means the
// repair's spend cannot be read, so the usage is recorded unavailable
// rather than left undercounting provider budget.
func usageAfterRepair(t *Turn, p *devinTurn, usageFile string) {
	repairTranscript := filepath.Join(t.Dir, "transcript.repair-1.atif.json")
	if !nonEmptyFile(repairTranscript) {
		if err := adapter.WriteUnavailableUsage(usageFile); err != nil {
			fmt.Fprintln(t.Deps().Stderr, err)
		}
		return
	}
	cumulative := p.cumulative
	if cumulative == "" {
		cumulative = filepath.Join(t.Dir, "session-usage.json")
	}
	if err := devinUsage(usageFile, repairTranscript, cumulative, p.previous, p.expectPrevious,
		filepath.Join(t.Dir, "transcript.repair.snapshot")); err != nil {
		fmt.Fprintln(t.Deps().Stderr, err)
	}
}

func devinPorts() (delegate.Ports, error) {
	return delegate.PortsFor("devin")
}

// devinUsage publishes a turn's usage delta from a transcript's cumulative
// session metrics (the former `adapter devin-usage`).
func devinUsage(usageFile, transcript, cumulative, previous string, expectPrevious bool, snapshot string) error {
	ports, err := devinPorts()
	if err != nil || ports.TurnUsage == nil {
		return errors.New("devin delegate ports provide no turn usage")
	}
	return ports.TurnUsage(usageFile, transcript, snapshot, cumulative, previous, expectPrevious)
}

// devinSettle is the engine's session certification (the former `adapter
// devin-settle`): the derived model, and whether the transcript's session
// agrees with the correlated one. A failure yields nothing, as the verb's
// nonzero exit did.
func devinSettle(t *Turn, transcript, snapshot string, requireTranscript bool) (string, bool) {
	ports, err := devinPorts()
	if err != nil || ports.Settle == nil {
		return "", false
	}
	model, certified, err := ports.Settle(transcript, snapshot, t.SessionID, t.Dir, requireTranscript)
	if err != nil {
		fmt.Fprintln(t.Deps().Stderr, err)
		return "", false
	}
	return model, certified
}

// devinCollect walks the delivery channels (the former `adapter
// devin-collect`) and returns its verdict bytes with the verb's exit
// status: 0 delivered (or any presence-only scan), 3 nothing qualified, 5
// transcript over the ceiling, 1 mechanical.
func devinCollect(t *Turn, in delegate.CollectInputs) ([]byte, int) {
	ports, err := devinPorts()
	if err != nil || ports.Collect == nil {
		return nil, 1
	}
	encoded, delivered, err := ports.Collect(in)
	if err != nil {
		fmt.Fprintln(t.Deps().Stderr, err)
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

// listSessions runs `devin list --format json` in the workspace into
// output, stderr to the turn log, and returns its status.
func listSessions(t *Turn, output string) int {
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		fmt.Fprintln(t.Log, err)
		return 1
	}
	defer file.Close()
	path, err := t.Deps().LookPath("devin")
	if err != nil {
		fmt.Fprintln(t.Log, "devin: command not found")
		return 127
	}
	command := exec.Command(path, "list", "--format", "json")
	command.Args[0] = "devin"
	command.Dir = t.Workspace
	command.Env = t.Env
	command.Stdout = file
	command.Stderr = t.Log
	return exitStatus(command.Run())
}

// previousRoundArtifact is the previous round's copy of a file, if any.
func previousRoundArtifact(d Deps, t *Turn, name string) string {
	round, err := strconv.Atoi(t.Round)
	if err != nil || round-1 < 1 {
		return ""
	}
	candidate := filepath.Join(d.agents(), t.RootJob, "rounds", strconv.Itoa(round-1), name)
	if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
		return candidate
	}
	return ""
}

func nonEmptyFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

// prepareHost is the host turn: the model, the host config, the named
// return file and the augmented prompt, then the transport's launch.
func (devinOps) prepareHost(t *Turn, transport string) (Launch, error) {
	model, ok := field(t.Record, "model")
	if !ok {
		return Launch{}, fmt.Errorf("cannot read the model of %s", t.Record)
	}
	p := &devinTurn{transport: transport,
		returnFile: filepath.Join(t.Dir, "devin-return.json"),
		prompt:     filepath.Join(t.Dir, "prompt.devin.md"),
		raw:        filepath.Join(t.Dir, "raw.out"),
		hostReturn: filepath.Join(t.Dir, "return.json"),
		usage:      filepath.Join(t.Dir, "usage.json"),
		transcript: filepath.Join(t.Dir, "transcript.atif.json"),
		cumulative: filepath.Join(t.Dir, "session-usage.json"),
		configFile: filepath.Join(t.Dir, "devin-config.json"),
	}
	// The host edits AND executes in the repository it is advancing; on
	// this CLI a working non-interactive host requires the dangerous mode
	// (the bm-2d evidence), and the boundary is NOT enforced on this runtime
	// — the capability snapshot declares it (D83 confines devin hosts to the
	// VM for that reason).
	if err := host.DevinConfig(t.Root, p.configFile); err != nil {
		return Launch{}, err
	}
	// The named return path (D64 phase 2): this model delivers by writing
	// files; the prompt names the exact path and the walk reads it.
	if _, err := os.Lstat(p.returnFile); err == nil {
		return Launch{EarlyRefusal: &Refusal{Error: "stale named return file from a crashed earlier attempt: " + p.returnFile}}, nil
	}
	if err := adapter.DevinPrompt(t.Prompt, t.Schema, p.prompt, p.returnFile); err != nil {
		return Launch{}, err
	}
	if transport == "acp" {
		// The ACP host turn: the same wire the delegate rides, without the
		// dispatch record machinery. The session mode comes from the host
		// permission envelope's tools grade, so the session carries the v1
		// grades rather than the dangerous mode.
		p.outcome = filepath.Join(t.Dir, "acp-outcome.json")
		p.sessionFile = filepath.Join(t.Dir, "acp-session-id")
		protocol, refusal := devinProtocol(t, t.Requested)
		if refusal != "" {
			grade, _ := fieldOr(t.Requested, "tools", "")
			message := map[string]string{
				"acp_preflight_refused":   "devin host: ACP preflight refused the permission envelope",
				"acp_mode_unmapped":       fmt.Sprintf("devin host: no ACP session mode maps to tools grade '%s'", grade),
				"acp_expectation_missing": "devin host: the runtime registry declares no ACP protocol expectation",
			}[refusal]
			return Launch{Refusal: &Refusal{Error: message}, Private: p}, nil
		}
		protocol.Journal = filepath.Join(t.Dir, "acp-journal.log")
		protocol.Outcome, protocol.SessionFile, protocol.PromptFile = p.outcome, p.sessionFile, p.prompt
		protocol.LoadSession = t.ResumeSession
		// argv0 devin-host-acp: the census signature distinguishes the
		// HOST's server child from both the raw CLI helper and the
		// delegate-side server.
		return Launch{Argv: []string{"devin", "acp"}, Argv0: "devin-host-acp", Protocol: protocol, Private: p}, nil
	}
	command := []string{"devin", "-p",
		"--prompt-file", p.prompt,
		"--respect-workspace-trust", "false",
		"--model", model,
		"--permission-mode", "dangerous",
		"--config", p.configFile,
		"--export", p.transcript,
	}
	if t.ResumeSession != "" {
		command = append(command, "-r", t.ResumeSession)
	}
	return Launch{Argv: command, StdoutPath: p.raw, TruncateLog: true, Private: p}, nil
}

// finalizeHostLegacy is the host's delivery walk and its per-session usage.
func (devinOps) finalizeHostLegacy(t *Turn, p *devinTurn, in FinalInput) (Final, error) {
	ensureEmptyFile(p.raw)
	ports, err := devinPorts()
	if err != nil || ports.HostCollect == nil || ports.HostReturn == nil || ports.HostTurnUsage == nil {
		return Final{}, errors.New("devin host ports are not registered")
	}
	// The delivery walk (D64 phase 2): stdout, the named file, then the
	// transcript's designated writes — the engine decides. With nothing
	// delivered the old raw path stands and the runner's validation reports
	// the absence.
	accepted := ""
	verdict, delivered, err := ports.HostCollect(delegate.HostCollectInputs{
		Root: t.Root, TurnRecordPath: t.Record, TurnDir: t.Dir, Workspace: t.Root,
		StdoutPath: p.raw, NamedPath: p.returnFile, TranscriptPath: p.transcript,
	})
	if err != nil {
		fmt.Fprintln(t.Deps().Stderr, err)
	}
	if err == nil && delivered {
		reply, ok := jsonedit.Get(verdict, "reply", nil)
		if !ok {
			return Final{}, errors.New("devin host collect verdict names no reply")
		}
		accepted = reply
		if err := ports.HostReturn(accepted, p.hostReturn); err != nil {
			return Final{}, err
		}
	} else if err := ports.HostReturn(p.raw, p.hostReturn); err != nil {
		return Final{}, err
	}
	// final_metrics is CUMULATIVE for a session and consumers ADD turn
	// records, so each turn publishes the delta against its predecessor's
	// stored totals, keyed by SESSION in a store in the turns' parent.
	parent, err := filepath.EvalSymlinks(filepath.Join(t.Dir, ".."))
	if err != nil {
		return Final{}, err
	}
	store := filepath.Join(parent, ".session-usage")
	if err := os.MkdirAll(store, 0o755); err != nil {
		return Final{}, err
	}
	previous := ""
	if t.ResumeSession != "" {
		candidate := filepath.Join(store, lease.Slug(t.ResumeSession)+".json")
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			previous = candidate
		}
	}
	if err := ports.HostTurnUsage(p.usage, p.transcript, p.cumulative, previous, t.ResumeSession != ""); err != nil {
		return Final{}, err
	}
	session := ""
	if data, err := os.ReadFile(p.transcript); err == nil {
		fallback := ""
		session, _ = jsonedit.Get(data, "session_id", &fallback)
	}
	return Final{HostSession: session, HostRaw: p.raw, HostReturn: p.hostReturn, HostAccepted: accepted,
		HostRequireReply: true, Usage: p.usage,
		// Publish this turn's cumulative totals so the next turn of THIS
		// session subtracts the right predecessor; only on a completion.
		AfterFinish: func(code int) {
			if code != 0 {
				return
			}
			if data, err := os.ReadFile(p.cumulative); err == nil && len(data) > 0 {
				_ = os.WriteFile(filepath.Join(store, lease.Slug(session)+".json"), data, 0o644)
			}
		}}, nil
}

// finalizeHostACP turns the wire's delivered candidate into the reply.
func (devinOps) finalizeHostACP(t *Turn, p *devinTurn, in FinalInput) (Final, error) {
	ensureEmptyFile(p.raw)
	accepted := ""
	if in.Status == 0 && nonEmptyFile(p.outcome) {
		data, _ := os.ReadFile(p.outcome)
		fallback := ""
		row, ok := jsonedit.Get(data, "row", &fallback)
		if !ok {
			return Final{}, fmt.Errorf("cannot read the acp outcome %s", p.outcome)
		}
		if row == "delivered" {
			// The wire candidate is the reply; the raw capture doubles as
			// the accepted snapshot path finish and the extractor read.
			candidate, ok := jsonedit.Get(data, "candidate", &fallback)
			if !ok {
				return Final{}, fmt.Errorf("cannot read the acp outcome %s", p.outcome)
			}
			if err := os.WriteFile(p.raw, []byte(candidate+"\n"), 0o644); err != nil {
				return Final{}, err
			}
			accepted = p.raw
			ports, err := devinPorts()
			if err != nil || ports.HostReturn == nil {
				return Final{}, errors.New("devin host ports are not registered")
			}
			if err := ports.HostReturn(p.raw, p.hostReturn); err != nil {
				return Final{}, err
			}
		}
	}
	if err := usage.ACPUsage(p.usage, p.outcome); err != nil {
		fmt.Fprintln(t.Log, err)
	}
	session := ""
	if nonEmptyFile(p.sessionFile) {
		session = firstLine(p.sessionFile)
	}
	return Final{HostSession: session, HostRaw: p.raw, HostReturn: p.hostReturn, HostAccepted: accepted,
		HostRequireReply: true, HostTransport: "acp", Usage: p.usage}, nil
}

func ensureEmptyFile(path string) {
	if _, err := os.Stat(path); err != nil {
		_ = os.WriteFile(path, nil, 0o644)
	}
}

// firstLine is `head -1 FILE`.
func firstLine(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(string(data), "\n")
	return line
}
