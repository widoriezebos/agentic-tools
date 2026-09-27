package supervisor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes/external"
)

// The external-executable implementation of the runtime operations (design
// verbs-object-action 3.5, U6c): every operation is one run of
// `<installation>/adapters/<name> OPERATION` with a JSON request on stdin
// and a JSON answer on stdout (observe: one JSON event per line). The same
// type serves an override of a built-in: an operation the executable
// answers with exit 64 is the built-in's. docs/agent-adapters.md is the
// contract; the shared layer (custody, deadlines, the envelope comparison,
// handshake bookkeeping, records, adjudication) is unchanged.

// externalOps is one external runtime's (or override's) operations.
type externalOps struct {
	adapter external.Adapter
	entry   external.Entry
	// builtin is the overridden built-in's operations; nil for a new
	// runtime.
	builtin Operations
	base    *runtimeAdapter

	// delegated remembers the operations the executable handed back (exit
	// 64), so a partial override is asked once per process, not once per
	// poll.
	mu        sync.Mutex
	delegated map[string]bool
}

// externalPrivate is the per-turn state: whose prepare made the launch, the
// executable's opaque private value, and the built-in's own.
type externalPrivate struct {
	external bool
	raw      json.RawMessage
	inner    any
}

func newExternalOps(entry external.Entry) *externalOps {
	ops := &externalOps{adapter: *entry.Adapter, entry: entry, delegated: map[string]bool{}}
	if base, ok := registry[entry.Name]; ok && entry.Builtin {
		ops.base = &base
		ops.builtin = base.ops
	}
	return ops
}

// call runs one operation unless it is known delegated. ErrDelegated is the
// executable's exit 64.
func (x *externalOps) call(d Deps, operation string, request map[string]any) ([]byte, error) {
	x.mu.Lock()
	known := x.delegated[operation]
	x.mu.Unlock()
	if known {
		return nil, external.ErrDelegated
	}
	if request == nil {
		request = map[string]any{}
	}
	request["root"] = d.Root
	response, err := x.adapter.Call(operation, request, d.Environ)
	if errors.Is(err, external.ErrDelegated) {
		x.mu.Lock()
		x.delegated[operation] = true
		x.mu.Unlock()
	}
	return response, err
}

// unimplemented is a new runtime's exit 64 on an operation it must answer.
func (x *externalOps) unimplemented(operation string) error {
	return fmt.Errorf("external runtime %s does not implement %s (it exited 64); see docs/agent-adapters.md", x.adapter.Name, operation)
}

// Describe is the registry's effective declaration in the operations' form.
// An override that leaves describe to its built-in, or omits the built-in's
// capabilities, keeps the built-in's.
func (x *externalOps) Describe(d Deps) (Description, error) {
	var base Description
	if x.builtin != nil {
		described, err := x.builtin.Describe(d)
		if err != nil {
			return Description{}, err
		}
		base = described
	}
	e := x.entry.Description
	out := Description{Name: x.adapter.Name, SchemaVersion: OperationsSchemaVersion, CLI: e.CLI,
		Capabilities: Capabilities{Resume: e.Capabilities.Resume, FollowUp: e.Capabilities.FollowUp, Repair: e.Capabilities.Repair,
			WaitDelivery: e.Capabilities.WaitDelivery, Host: e.Capabilities.Host, Usage: e.Capabilities.Usage},
		Match: e.Match, Exclude: e.Exclude, Positive: e.Positive, Lookalike: e.Lookalike, ConfigPaths: e.ConfigPaths}
	for _, shape := range e.Invocations {
		out.Invocations = append(out.Invocations, InvocationShape{Includes: shape.Includes, TagFlag: shape.TagFlag,
			TagPrefix: shape.TagPrefix, TagPathBase: shape.TagPathBase})
	}
	out.Enforcement, _ = enforcementJSON(e.Enforcement)
	if x.builtin != nil {
		if out.CLI == "" {
			out.CLI = base.CLI
		}
		if out.Capabilities == (Capabilities{}) {
			out.Capabilities = base.Capabilities
		}
		if out.Capabilities.Usage == "" {
			out.Capabilities.Usage = base.Capabilities.Usage
		}
		if len(out.Invocations) == 0 {
			out.Invocations = base.Invocations
		}
		if out.Enforcement == "" {
			out.Enforcement = base.Enforcement
		}
	}
	if out.Capabilities.Usage == "" {
		out.Capabilities.Usage = "unavailable"
	}
	return out, nil
}

// enforcementJSON is a declared enforcement map in the canonical field
// order; ok is false unless it covers every field.
func enforcementJSON(m map[string]string) (string, bool) {
	if len(m) == 0 {
		return "", false
	}
	parts := make([]string, 0, len(runtimes.EnforcementFields))
	for _, field := range runtimes.EnforcementFields {
		value, ok := m[field]
		if !ok {
			return "", false
		}
		parts = append(parts, fmt.Sprintf("%q:%q", field, value))
	}
	return "{" + strings.Join(parts, ",") + "}", true
}

// probeAnswer is the probe operation's answer.
type probeAnswer struct {
	SchemaVersion  int             `json:"schemaVersion"`
	Installed      bool            `json:"installed"`
	Version        string          `json:"version"`
	ConfigIdentity json.RawMessage `json:"configIdentity"`
	// Authenticated false refuses the probe with AuthFailure (the
	// runtime's own permission and authentication check).
	Authenticated *bool           `json:"authenticated,omitempty"`
	AuthFailure   string          `json:"authFailure,omitempty"`
	Transports    json.RawMessage `json:"transports,omitempty"`
	Capabilities  json.RawMessage `json:"capabilities,omitempty"`
	Permissions   json.RawMessage `json:"permissions,omitempty"`
	Enforcement   json.RawMessage `json:"enforcement,omitempty"`
}

func (x *externalOps) probe(d Deps, stage string, args []string) (probeAnswer, error) {
	response, err := x.call(d, "probe", map[string]any{"stage": stage, "args": args})
	if err != nil {
		return probeAnswer{}, err
	}
	var answer probeAnswer
	if err := external.Decode(response, &answer); err != nil {
		return probeAnswer{}, err
	}
	if !answer.Installed {
		cli := x.entry.Description.CLI
		if cli == "" {
			cli = x.adapter.Name
		}
		return probeAnswer{}, fmt.Errorf("%s CLI is not installed", cli)
	}
	if len(answer.ConfigIdentity) == 0 {
		return probeAnswer{}, fmt.Errorf("external adapter %s probe answered no configIdentity", x.adapter.Name)
	}
	return answer, nil
}

// ConfigIdentity is the probe's configuration identity (stage identity).
func (x *externalOps) ConfigIdentity(d Deps) (string, error) {
	answer, err := x.probe(d, "identity", nil)
	if errors.Is(err, external.ErrDelegated) && x.base != nil {
		return x.base.configIdentity(d)
	}
	if err != nil {
		return "", err
	}
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, answer.ConfigIdentity); err != nil {
		return "", err
	}
	return compacted.String(), nil
}

// Probe writes a capability snapshot from the probe's answer (stage
// snapshot).
func (x *externalOps) Probe(d Deps, args []string) int {
	answer, err := x.probe(d, "snapshot", args)
	if errors.Is(err, external.ErrDelegated) && x.base != nil {
		return x.base.probe(d, args)
	}
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	version, hash, keyHashes, err := identityFields(string(answer.ConfigIdentity))
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	if answer.Authenticated != nil && !*answer.Authenticated {
		failure := answer.AuthFailure
		if failure == "" {
			failure = x.adapter.Name + " authentication is unavailable"
		}
		fmt.Fprintln(d.Stderr, failure)
		return 1
	}
	enforcement := string(answer.Enforcement)
	if enforcement == "" {
		described, _ := x.Describe(d)
		enforcement = described.Enforcement
	}
	if enforcement == "" {
		fmt.Fprintf(d.Stderr, "external adapter %s declares no envelope enforcement (describe's \"enforcement\")\n", x.adapter.Name)
		return 1
	}
	transports, capabilities, permissions := orDefault(answer.Transports, `["stdin"]`),
		orDefault(answer.Capabilities, x.defaultCapabilities()), orDefault(answer.Permissions, `{"unverified": []}`)
	if err := d.writeCapabilitySnapshot(x.adapter.Name, version, hash, transports, capabilities, permissions, enforcement, keyHashes); err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	return 0
}

func orDefault(raw json.RawMessage, fallback string) string {
	if len(raw) == 0 {
		return fallback
	}
	return string(raw)
}

func (x *externalOps) defaultCapabilities() string {
	c := x.entry.Description.Capabilities
	return fmt.Sprintf(`{"resume":%t,"sessionEstablishedSignal":true,"sessionEstablishedTimeoutSec":30,"nativeStructuredOutput":false,"nativeEvents":false,"nativeUsage":%t,"gracefulCancel":false,"hooks":false,"protocolServer":false,"nativeBudget":false}`,
		c.Resume, c.Usage == "native")
}

// Contract is the contract snapshot from the declared enforcement map.
func (x *externalOps) Contract(d Deps) ([]byte, error) {
	if enforcement, ok := enforcementJSON(x.entry.Description.Enforcement); ok {
		return contractSnapshot(x.adapter.Name, enforcement)
	}
	if x.base != nil && x.base.contract != nil {
		return x.base.contract(d)
	}
	return nil, fmt.Errorf("external adapter %s declares no envelope-enforcement map", x.adapter.Name)
}

// Selftest runs the shared full-contract self-test through this runtime:
// its identity and probe, and its custom probe's stages through the
// selftest operation. An override without its own self-test declaration
// runs the built-in's.
func (x *externalOps) Selftest(d Deps) int {
	spec := x.entry.Description.Selftest
	if spec == nil && x.base != nil {
		return x.base.selftest(d)
	}
	if spec == nil {
		spec = &external.Selftest{}
	}
	described, _ := x.Describe(d)
	p := adapter.SelftestParams{
		Root: d.Root, Runtime: x.adapter.Name, Usage: described.Capabilities.Usage,
		TurnCeilingSec: spec.TurnCeilingSec, DenialEndsTurn: spec.DenialEndsTurn, Engine: d.Self,
		RunIdentity: func() error {
			_, err := x.ConfigIdentity(d)
			return err
		},
		RunProbe: func() error {
			quiet := d
			quiet.Stdout = discard{}
			if code := x.Probe(quiet, nil); code != 0 {
				return fmt.Errorf("%s probe failed", x.adapter.Name)
			}
			return nil
		},
	}
	if p.TurnCeilingSec == 0 {
		p.TurnCeilingSec = 240
	}
	if spec.Probe != nil {
		p.Probe = x.selftestProbe(d, *spec.Probe)
	}
	model, err := d.configValue("role.default.model."+x.adapter.Name, "")
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	if err := adapter.SelftestRun(p, model, d.Stdout); err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	return 0
}

// selftestProbe binds a declared custom probe's stages to the selftest
// operation.
func (x *externalOps) selftestProbe(d Deps, probe external.SelftestProbe) *adapter.SelftestProbe {
	stage := func(name string, fields map[string]any) ([]byte, error) {
		fields["stage"], fields["probe"] = name, probe.Name
		return x.call(d, "selftest", fields)
	}
	return &adapter.SelftestProbe{
		Name: probe.Name, BehaviorLabels: probe.BehaviorLabels,
		PrepareScratch: func(scratch, nonce string) error {
			_, err := stage("prepare-scratch", map[string]any{"scratch": scratch, "nonce": nonce})
			return err
		},
		PromptText: func(nonce string) string {
			response, err := stage("prompt-text", map[string]any{"nonce": nonce})
			if err != nil {
				return ""
			}
			var answer struct {
				Text string `json:"text"`
			}
			if external.Decode(response, &answer) != nil {
				return ""
			}
			return answer.Text
		},
		VerifyEvidence: func(returnPath, nonce string) error {
			_, err := stage("verify-evidence", map[string]any{"returnPath": returnPath, "nonce": nonce})
			return err
		},
	}
}

// turnJSON is the turn context every per-turn operation receives.
func turnJSON(t *Turn) map[string]any {
	return map[string]any{
		"role": string(t.Role), "verb": t.Verb, "runtime": t.Runtime, "root": t.Root, "workspace": t.Workspace,
		"dir": t.Dir, "record": t.Record, "job": t.Job, "tag": t.Tag, "round": t.Round, "rootJob": t.RootJob,
		"mission": t.Mission, "turnId": t.TurnID, "result": t.Result, "prompt": t.Prompt, "schema": t.Schema,
		"model": t.Model, "resumeSession": t.ResumeSession, "requested": t.Requested, "effective": t.Effective,
		"events": t.Events, "handshakeDone": t.HandshakeDone, "sessionId": t.SessionID,
	}
}

type wireRefusal struct {
	Error string `json:"error"`
	Phase string `json:"phase"`
}

func (w *wireRefusal) refusal() *Refusal {
	if w == nil {
		return nil
	}
	return &Refusal{Error: w.Error, Phase: w.Phase}
}

// prepareAnswer is the prepare operation's answer.
type prepareAnswer struct {
	SchemaVersion int          `json:"schemaVersion"`
	EarlyRefusal  *wireRefusal `json:"earlyRefusal,omitempty"`
	Refusal       *wireRefusal `json:"refusal,omitempty"`
	Argv          []string     `json:"argv"`
	Argv0         string       `json:"argv0,omitempty"`
	Env           []string     `json:"env,omitempty"`
	Stdin         string       `json:"stdin,omitempty"`
	Stdout        string       `json:"stdout,omitempty"`
	TruncateLog   bool         `json:"truncateLog,omitempty"`
	// Effective is the effective permission envelope the command and its
	// settings produce; the shared layer compares it with the request
	// before anything starts. Absent, the request stands as the grant.
	Effective json.RawMessage `json:"effective,omitempty"`
	Private   json.RawMessage `json:"private,omitempty"`
}

// Prepare asks the executable for the launch; exit 64 on an override
// prepares through the built-in.
func (x *externalOps) Prepare(t *Turn) (Launch, error) {
	d := t.Deps()
	response, err := x.call(d, "prepare", map[string]any{"turn": turnJSON(t)})
	if errors.Is(err, external.ErrDelegated) {
		if x.builtin == nil {
			return Launch{}, x.unimplemented("prepare")
		}
		launch, err := x.builtin.Prepare(t)
		launch.Private = externalPrivate{inner: launch.Private}
		return launch, err
	}
	if err != nil {
		return Launch{}, err
	}
	var answer prepareAnswer
	if err := external.Decode(response, &answer); err != nil {
		return Launch{}, fmt.Errorf("external adapter %s prepare: %w", x.adapter.Name, err)
	}
	launch := Launch{EarlyRefusal: answer.EarlyRefusal.refusal(), Refusal: answer.Refusal.refusal(),
		Argv: answer.Argv, Argv0: answer.Argv0, Env: answer.Env, StdinPath: answer.Stdin, StdoutPath: answer.Stdout,
		TruncateLog: answer.TruncateLog, Private: externalPrivate{external: true, raw: answer.Private}}
	if launch.EarlyRefusal != nil || launch.Refusal != nil {
		return launch, nil
	}
	if len(launch.Argv) == 0 {
		return Launch{}, fmt.Errorf("external adapter %s prepare answered no argv", x.adapter.Name)
	}
	if launch.StdoutPath == "" {
		stream := x.entry.Description.OutputStream
		if stream == "" {
			stream = "events.jsonl"
		}
		launch.StdoutPath = filepath.Join(t.Dir, stream)
	}
	if len(answer.Effective) > 0 && t.Role == RoleDelegate {
		// The runtime's own mapping of the request (VOA-26): the shared
		// comparison judges exactly what it reports.
		if err := os.WriteFile(t.Effective, append(bytes.TrimSpace(answer.Effective), '\n'), 0o644); err != nil {
			return Launch{}, err
		}
	}
	return launch, nil
}

// inner is the launch as the built-in prepared it.
func innerLaunch(launch Launch) Launch {
	if private, ok := launch.Private.(externalPrivate); ok {
		launch.Private = private.inner
	}
	return launch
}

func privateOf(launch Launch) externalPrivate {
	private, _ := launch.Private.(externalPrivate)
	return private
}

func launchJSON(launch Launch) map[string]any {
	return map[string]any{"argv": launch.Argv, "stdin": launch.StdinPath, "stdout": launch.StdoutPath}
}

// request is a per-turn operation's request.
func (x *externalOps) request(t *Turn, launch Launch, fields map[string]any) map[string]any {
	private := privateOf(launch)
	request := map[string]any{"turn": turnJSON(t), "launch": launchJSON(launch)}
	if private.external && len(private.raw) > 0 {
		request["private"] = private.raw
	}
	for key, value := range fields {
		request[key] = value
	}
	return request
}

// fallback runs an operation the executable delegated through the built-in,
// which can only continue a launch it prepared itself.
func (x *externalOps) fallback(operation string, launch Launch) error {
	if x.builtin == nil {
		return x.unimplemented(operation)
	}
	if privateOf(launch).external {
		return fmt.Errorf("external adapter %s prepared this turn itself, so it must answer %s too (it exited 64)", x.adapter.Name, operation)
	}
	return nil
}

// observedEvent is one line of observe's answer.
type observedEvent struct {
	Event   string `json:"event"`
	Session string `json:"session,omitempty"`
	Turn    string `json:"turn,omitempty"`
	Model   string `json:"model,omitempty"`
	Error   string `json:"error,omitempty"`
	Phase   string `json:"phase,omitempty"`
	Line    string `json:"line,omitempty"`
	Patch   string `json:"patch,omitempty"`
}

func decodeEvents(response []byte) (Events, error) {
	var events Events
	scanner := bufio.NewScanner(bytes.NewReader(response))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var event observedEvent
		if err := json.Unmarshal(line, &event); err != nil {
			return Events{}, fmt.Errorf("observe answered a line that is not a JSON event: %s", line)
		}
		switch event.Event {
		case "handshake":
			events.Session, events.Turn, events.Model = event.Session, event.Turn, event.Model
		case "refusal":
			events.Refusal = &Refusal{Error: event.Error, Phase: event.Phase}
		case "line":
			events.Lines = append(events.Lines, event.Line)
		case "record-patch":
			events.RecordPatch = event.Patch
		default:
			return Events{}, fmt.Errorf("observe answered an unknown event %q", event.Event)
		}
	}
	return events, scanner.Err()
}

// Observe asks the executable for events while the CLI runs.
func (x *externalOps) Observe(t *Turn, o Observation) (Events, error) {
	response, err := x.call(t.Deps(), "observe", x.request(t, o.Launch, map[string]any{"running": o.Running}))
	if errors.Is(err, external.ErrDelegated) {
		if err := x.fallback("observe", o.Launch); err != nil {
			return Events{}, err
		}
		o.Launch = innerLaunch(o.Launch)
		return x.builtin.Observe(t, o)
	}
	if err != nil {
		return Events{}, err
	}
	return decodeEvents(response)
}

// finalAnswer is the finalize operation's answer.
type finalAnswer struct {
	SchemaVersion int          `json:"schemaVersion"`
	Refusal       *wireRefusal `json:"refusal,omitempty"`
	Handshake     *struct {
		Session string   `json:"session"`
		Turn    string   `json:"turn,omitempty"`
		Model   string   `json:"model,omitempty"`
		Lines   []string `json:"lines,omitempty"`
	} `json:"handshake,omitempty"`
	Candidate      string `json:"candidate,omitempty"`
	Transcript     string `json:"transcript,omitempty"`
	Usage          string `json:"usage,omitempty"`
	Session        string `json:"session,omitempty"`
	Turn           string `json:"turn,omitempty"`
	HandshakeModel string `json:"handshakeModel,omitempty"`
	ResultModel    string `json:"resultModel,omitempty"`
	RecordModel    string `json:"recordModel,omitempty"`
	DeliveryRepair bool   `json:"deliveryRepair,omitempty"`
	Status         *int   `json:"status,omitempty"`
	// A host turn's finish.
	HostSession      string `json:"hostSession,omitempty"`
	HostRaw          string `json:"hostRaw,omitempty"`
	HostReturn       string `json:"hostReturn,omitempty"`
	HostAccepted     string `json:"hostAccepted,omitempty"`
	HostTransport    string `json:"hostTransport,omitempty"`
	HostRequireReply bool   `json:"hostRequireReply,omitempty"`
	HostExit         *int   `json:"hostExit,omitempty"`
}

// Finalize asks the executable for the turn's outcome.
func (x *externalOps) Finalize(t *Turn, in FinalInput) (Final, error) {
	response, err := x.call(t.Deps(), "finalize", x.request(t, in.Launch, map[string]any{"status": in.Status, "usage": in.Usage}))
	if errors.Is(err, external.ErrDelegated) {
		if err := x.fallback("finalize", in.Launch); err != nil {
			return Final{}, err
		}
		in.Launch = innerLaunch(in.Launch)
		return x.builtin.Finalize(t, in)
	}
	if err != nil {
		return Final{}, err
	}
	var answer finalAnswer
	if err := external.Decode(response, &answer); err != nil {
		return Final{}, fmt.Errorf("external adapter %s finalize: %w", x.adapter.Name, err)
	}
	final := Final{Refusal: answer.Refusal.refusal(), Candidate: answer.Candidate, Transcript: answer.Transcript,
		Usage: answer.Usage, Session: answer.Session, Turn: answer.Turn, HandshakeModel: answer.HandshakeModel,
		ResultModel: answer.ResultModel, RecordModel: answer.RecordModel, DeliveryRepair: answer.DeliveryRepair,
		Status: answer.Status, HostSession: answer.HostSession, HostRaw: answer.HostRaw, HostReturn: answer.HostReturn,
		HostAccepted: answer.HostAccepted, HostTransport: answer.HostTransport, HostRequireReply: answer.HostRequireReply,
		HostExit: answer.HostExit}
	if answer.Handshake != nil {
		final.Handshake = &Events{Session: answer.Handshake.Session, Turn: answer.Handshake.Turn,
			Model: answer.Handshake.Model, Lines: answer.Handshake.Lines}
	}
	return final, nil
}

// repairAnswer is the repair operation's answer.
type repairAnswer struct {
	SchemaVersion int    `json:"schemaVersion"`
	Status        int    `json:"status"`
	Candidate     string `json:"candidate,omitempty"`
	Violation     string `json:"violation,omitempty"`
	NamedPath     string `json:"namedPath,omitempty"`
	Settled       bool   `json:"settled,omitempty"`
	Model         string `json:"model,omitempty"`
}

// Repair runs one repair stage through the executable; the shared layer
// calls it only when describe declares repair.
func (x *externalOps) Repair(t *Turn, in RepairInput) RepairResult {
	response, err := x.call(t.Deps(), "repair", x.request(t, in.Launch, map[string]any{
		"stage": string(in.Stage), "promptFile": in.PromptFile, "outputFile": in.OutputFile,
		"namedPath": in.NamedPath, "usage": in.Usage,
	}))
	if errors.Is(err, external.ErrDelegated) {
		if fallbackErr := x.fallback("repair", in.Launch); fallbackErr != nil {
			fmt.Fprintln(t.Log, fallbackErr)
			return RepairResult{Status: 1}
		}
		in.Launch = innerLaunch(in.Launch)
		return x.builtin.Repair(t, in)
	}
	if err != nil {
		fmt.Fprintln(t.Log, err)
		return RepairResult{Status: 1}
	}
	var answer repairAnswer
	if err := external.Decode(response, &answer); err != nil {
		fmt.Fprintf(t.Log, "external adapter %s repair: %v\n", x.adapter.Name, err)
		return RepairResult{Status: 1}
	}
	return RepairResult{Status: answer.Status, Candidate: answer.Candidate, Violation: answer.Violation,
		NamedPath: answer.NamedPath, Settled: answer.Settled, Model: answer.Model}
}

// Cancel runs the executable's own cancellation (exit 64: none), then the
// shared one: the owned process groups are the shared layer's.
func (x *externalOps) Cancel(d Deps, job string) int {
	if _, err := x.call(d, "cancel", map[string]any{"job": job}); err != nil && !errors.Is(err, external.ErrDelegated) {
		fmt.Fprintln(d.Stderr, err)
	}
	if x.builtin != nil {
		return x.builtin.Cancel(d, job)
	}
	return d.Dispatch.Run(d.Stdout, d.Stderr, "__cancel-owned", "--job", job)
}

// externalRuntime is the delegate-supervisor verbs of an external runtime
// or an overridden built-in.
type externalRuntime struct {
	ops *externalOps
}

func (r externalRuntime) Name() string { return r.ops.adapter.Name }
func (r externalRuntime) Signature(Deps) (string, error) {
	return r.ops.entry.Signature, nil
}
func (r externalRuntime) ConfigIdentity(d Deps) (string, error) { return r.ops.ConfigIdentity(d) }
func (r externalRuntime) LocalConfigPaths(Deps) ([]string, error) {
	return append([]string(nil), r.ops.entry.Description.ConfigPaths...), nil
}
func (r externalRuntime) EnforcementMap(d Deps) (string, bool, error) {
	described, err := r.ops.Describe(d)
	if err != nil {
		return "", false, err
	}
	return described.Enforcement, described.Enforcement != "", nil
}
func (r externalRuntime) Contract(d Deps) ([]byte, error) { return r.ops.Contract(d) }
func (r externalRuntime) Probe(d Deps, args []string) int { return r.ops.Probe(d, args) }
func (r externalRuntime) OutputStream(d Deps, roundDir string) (string, error) {
	if stream := r.ops.entry.Description.OutputStream; stream != "" {
		return filepath.Join(roundDir, stream), nil
	}
	if r.ops.base != nil && r.ops.base.outputStream != nil {
		return r.ops.base.outputStream(d, roundDir)
	}
	return filepath.Join(roundDir, "events.jsonl"), nil
}
func (r externalRuntime) Supervise(s *Supervision, args []string) int {
	return superviseRound(s, args, r.ops)
}
func (r externalRuntime) Cancel(d Deps, job string) int { return r.ops.Cancel(d, job) }
func (r externalRuntime) WaitDelivery(d Deps, waitID, nonce, deadline, session string) (bool, error) {
	described, err := r.ops.Describe(d)
	if err != nil {
		return false, err
	}
	return described.Capabilities.WaitDelivery && WaitDeliveryAccepted(waitID, nonce, deadline, session), nil
}
func (r externalRuntime) Selftest(d Deps) int { return r.ops.Selftest(d) }
func (r externalRuntime) Usage(d Deps) {
	_, enforcement, _ := r.EnforcementMap(d)
	writeUsage(d.Stderr, usageLines(r.ops.adapter.Name, "probe --root ROOT", enforcement))
}

// Admit forwards the overridden built-in's admission.
func (x *externalOps) Admit(d Deps, verb string) error {
	if admitter, ok := x.builtin.(Admitter); ok {
		return admitter.Admit(d, verb)
	}
	return nil
}

// HostPreflight forwards the overridden built-in's host preflight.
func (x *externalOps) HostPreflight(t *Turn) (int, bool) {
	if preflight, ok := x.builtin.(interface {
		HostPreflight(*Turn) (int, bool)
	}); ok {
		return preflight.HostPreflight(t)
	}
	return 0, false
}
