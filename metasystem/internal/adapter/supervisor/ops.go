package supervisor

import (
	"io"
)

// The runtime layer's operations (design verbs-object-action 3.5): the one
// Go interface every runtime implements — the Go built-ins now, an external
// executable speaking the same operations as JSON later (U6c). The shared
// layer (preparation, the envelope comparison and refusal, custody,
// deadlines and kill domains, handshake bookkeeping, record CAS and patches,
// return normalization and turn adjudication) only calls these.
//
//	describe  name, capabilities, signatures, the claim-bound invocation
//	          shape, config paths
//	probe     installed, version, configuration identity, the snapshot
//	selftest  the runtime's evidence for the shared self-test
//	prepare   argv, environment, stdin, settings and channels, artifacts,
//	          and the effective permission envelope, for role delegate or
//	          host
//	observe   the CLI's stdout and declared artifacts → events
//	finalize  exit status and artifacts → result candidate, usage,
//	          observed session identity
//	repair    one repair turn in the same session (when described)
//	cancel    runtime-specific cancellation beyond the group kill

// Role is who a turn serves.
type Role string

const (
	RoleDelegate Role = "delegate"
	RoleHost     Role = "host"
)

// Capabilities are what describe declares a runtime can do.
type Capabilities struct {
	Resume       bool   `json:"resume"`
	FollowUp     bool   `json:"followUp"`
	Repair       bool   `json:"repair"`
	WaitDelivery bool   `json:"waitDelivery"`
	Host         bool   `json:"host"`
	Usage        string `json:"usage"` // native, unavailable, or metered
}

// InvocationShape is the claim-bound invocation of a runtime's CLI: the argv
// words it carries and where the claim tag sits. The janitor keeps the kill
// proof in Go and takes these shapes declaratively (VOA-29).
type InvocationShape struct {
	Includes    []string `json:"includes"`
	TagFlag     string   `json:"tagFlag"`
	TagPrefix   string   `json:"tagPrefix,omitempty"`
	TagPathBase bool     `json:"tagPathBase,omitempty"`
}

// Description is describe's answer.
type Description struct {
	Name          string `json:"name"`
	SchemaVersion int    `json:"schemaVersion"`
	// CLI is the executable a turn needs installed ("" for none).
	CLI          string            `json:"cli,omitempty"`
	Capabilities Capabilities      `json:"capabilities"`
	Match        []string          `json:"match"`
	Exclude      []string          `json:"exclude"`
	Positive     string            `json:"positive"`
	Lookalike    string            `json:"lookalike"`
	Invocations  []InvocationShape `json:"invocations"`
	ConfigPaths  []string          `json:"configPaths"`
	// Enforcement is the static envelope-enforcement map, empty when the
	// runtime declares none.
	Enforcement string `json:"enforcement,omitempty"`
}

// OperationsSchemaVersion is the operations' schema version.
const OperationsSchemaVersion = 1

// Turn is the shared layer's view of one turn that the runtime operations
// read: who it serves, where its evidence lives, and what was requested.
type Turn struct {
	d    Deps
	Role Role
	// Verb is dispatch or follow-up for a delegate, start-turn for a host.
	Verb string
	// Runtime is the runtime name.
	Runtime string
	// Root is the installation root.
	Root string
	// Workspace is the directory the CLI runs in.
	Workspace string
	// Dir is the round (delegate) or turn (host) directory.
	Dir string
	// Record is the job record (delegate) or turn record (host).
	Record string
	// Job, Tag and Round identify a delegate round; Mission and TurnID a
	// host turn.
	Job, Tag, Round, RootJob string
	Mission, TurnID          string
	// Result is a host turn's result envelope path.
	Result                string
	Prompt, Schema, Model string
	// ResumeSession is the session being resumed ("" for a fresh turn).
	ResumeSession string
	// Effective is the effective permission envelope file the runtime's
	// prepare leaves; Requested is the requested envelope (the job record
	// for a delegate, the workspace preset for a host).
	Effective, Requested string
	// Events is the round's events stream (delegate).
	Events string
	// Log receives the runtime's diagnostics and its CLI's stderr.
	Log io.Writer
	// HandshakeDone and SessionID mirror the shared handshake state.
	HandshakeDone bool
	SessionID     string
	// Env is the environment the CLI starts from.
	Env []string
	// handshake, set by a shared layer that records handshakes itself (the
	// delegate round), records one now.
	handshake func(Events) bool
}

// Deps is the turn's process seams.
func (t *Turn) Deps() Deps { return t.d }

// Handshake records a handshake the runtime observed only at finalize,
// before the runtime reads the record it lands in (a collector matching the
// return's session against the job record). It reports false when this
// shared layer does not record handshakes in place; the runtime then answers
// with Final.Handshake.
func (t *Turn) Handshake(events Events) bool {
	if t.handshake == nil || events.Session == "" || t.HandshakeDone {
		return false
	}
	t.handshake(events)
	return true
}

// Refusal is a named failure the runtime chooses: the error code and phase
// the shared layer lands on the record (pending before the handshake,
// running after it).
type Refusal struct {
	Error, Phase string
}

// Launch is prepare's answer.
type Launch struct {
	// EarlyRefusal ends the turn before the envelope comparison; Refusal
	// ends it after the comparison, before any launch. For a host turn the
	// refusal's Error is the message and the host exits 3.
	EarlyRefusal, Refusal *Refusal
	// SetupError is a launch-environment failure (a scratch directory that
	// could not be made); it fails the launch like a child that died before
	// custody.
	SetupError error
	// Argv starts the CLI; Argv0 renames its argv[0] (a census-distinct
	// process name); Env are assignments over the turn's environment.
	Argv  []string
	Argv0 string
	Env   []string
	// StdinPath is fed to the CLI's stdin ("" is no stdin).
	StdinPath string
	// StdoutPath captures the CLI's stdout.
	StdoutPath string
	// TruncateLog starts the CLI's stderr log afresh (the host's shape).
	TruncateLog bool
	// Protocol, when set, is a shared transport the shared layer drives
	// against the launched server (ACP).
	Protocol *ProtocolLaunch
	// Simulated, when set, is an in-process CLI stand-in (the fake runtime)
	// run instead of Argv; its return is the CLI's exit status.
	Simulated func(stop <-chan struct{}) int
	// OnHandshake, when set, is called once the shared layer recorded the
	// handshake observe reported (a simulated CLI continues past it).
	OnHandshake func()
	// Private is the runtime's own per-turn state, handed back to observe,
	// finalize and repair.
	Private any
	// BeforeLaunch, when set, runs after the envelope comparison and the
	// refusal, before reference verification: a pre-launch step whose
	// refusal lands pending (Devin's session baseline, which must never run
	// for a turn the comparison refuses). An error ends the round (1).
	BeforeLaunch func() (*Refusal, error)
}

// ProtocolLaunch asks the shared layer to drive a protocol client against
// the launched server over the named pipes.
//
// The shared layer makes the per-attempt fifo pair (it fills in ServerOut
// and ServerIn), launches the server from Argv/Argv0/Env with its stdout and
// stdin on the pair, drives the in-process client over Envelope, and owns
// the server's custody, the client's cancellation on TERM/INT, and the
// pair's removal. Its own failure codes are named by Kind: KIND_fifo_setup
// (setup) and KIND_server_died (handshake).
type ProtocolLaunch struct {
	Kind                string // "acp"
	ServerOut, ServerIn string
	// Envelope is the permission envelope file the client enforces.
	Envelope         string
	Journal, Outcome string
	SessionFile      string
	PromptFile       string
	Mode             string
	ExpectedProtocol int64
	LoadSession      string
	Cleanup          func()
}

// Observation is what observe reads while the CLI runs.
type Observation struct {
	Launch  Launch
	Running bool
}

// Events is observe's answer: at most the handshake facts, or a refusal.
type Events struct {
	Session, Turn, Model string
	Refusal              *Refusal
	// Lines are events-stream lines the runtime records (session
	// correlation evidence).
	Lines []string
	// RecordPatch, when set, is a record patch the shared layer applies
	// running→running right after the handshake it reports, best-effort,
	// its output to the job log (a transport pin).
	RecordPatch string
}

// FinalInput is what finalize reads after the CLI exits.
type FinalInput struct {
	Launch Launch
	Status int
	Usage  string // the usage file finalize writes
}

// Final is finalize's answer.
type Final struct {
	// Handshake, when it names a session and the turn has none yet, is
	// recorded before anything else of this answer (a session only the
	// settled outcome names); its Lines and RecordPatch follow a successful
	// handshake.
	Handshake *Events
	// Refusal ends the turn with a named failure instead of adjudication.
	Refusal *Refusal
	// Candidate and Transcript are what adjudication validates; Usage is
	// the host turn's usage file.
	Candidate, Transcript, Usage string
	// Session, Turn and ResultModel are the observed identity; HandshakeModel
	// is the model a late handshake records.
	Session, Turn, HandshakeModel, ResultModel string
	// ModelOnly records a result model even when it cannot certify the
	// session (Devin's settlement).
	RecordModel string
	// DeliveryRepair asks the shared layer for the delivery repair: nothing
	// was delivered, and the runtime can ask the same session again.
	DeliveryRepair bool
	// Status overrides the CLI status adjudication judges (an in-process
	// delivery the CLI exit does not describe).
	Status *int

	// A host turn's finish (host.FinishTurn): the session, the raw and
	// return paths, the accepted reply, whether a reply is required, the
	// transport pin, and what to do with the finish code.
	HostSession, HostRaw, HostReturn, HostAccepted, HostTransport string
	HostRequireReply                                              bool
	AfterFinish                                                   func(code int)
	// HostExit ends a host turn with this code before any finish.
	HostExit *int
}

// RepairInput is one step of a repair.
type RepairInput struct {
	Launch                 Launch
	Stage                  RepairStage
	PromptFile, OutputFile string
	// NamedPath is the delivery repair's named return file.
	NamedPath string
	Usage     string
}

// RepairResult is repair's answer.
type RepairResult struct {
	// Status is the repair CLI's exit status.
	Status int
	// Candidate, for a delivery repair, is the collected reply (empty when
	// nothing qualified, with Violation saying why).
	Candidate, Violation string
	// NamedPath is the delivery repair's named return file.
	NamedPath string
	// Settled reports the repaired session and model re-certified.
	Settled bool
	Model   string
}

// Admitter is an optional operation: a refusal before the round is even
// prepared — nothing spent, no record written — whose message goes to
// stderr, ending the round with 1 (a runtime configuration that cannot be
// read, or names no known transport).
type Admitter interface {
	Admit(d Deps, verb string) error
}

// Operations is the operation interface of one runtime.
type Operations interface {
	Describe(d Deps) (Description, error)
	ConfigIdentity(d Deps) (string, error)
	Probe(d Deps, args []string) int
	Contract(d Deps) ([]byte, error)
	Selftest(d Deps) int
	Prepare(t *Turn) (Launch, error)
	Observe(t *Turn, o Observation) (Events, error)
	Finalize(t *Turn, in FinalInput) (Final, error)
	Repair(t *Turn, in RepairInput) RepairResult
	Cancel(d Deps, job string) int
}
