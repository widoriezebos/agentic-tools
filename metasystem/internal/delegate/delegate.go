// Package delegate is the seam between the metasystem core and every
// delegate runtime: ONE generic, ACP-shaped delegate-session contract —
// open, prompt turn, event stream, ask, answer, cancel, usage, result —
// with capability DECLARATION instead of runtime types. The core sees
// this package's vocabulary and never a runtime name; runtime knowledge
// lives below the seam, in the owner packages that register here.
//
// The contract is deliberately stateful and complete (the full session
// shape slice two's native ACP driver implements), while registration
// stays honest: today's CLI runtimes provide only read-side ports
// (usage, result interpretation) and register exactly those — a partial
// implementation must never register as a conforming Driver, and
// RegisterDriver refuses one. The adapter script owns process launch;
// a Driver speaks pure protocol over pre-opened pipes (Endpoint), per
// the ratified launch/protocol split.
package delegate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"
)

// Row is the typed outcome of one delegate turn — the ACP-shaped
// vocabulary every driver maps into. Parity with internal/acp's rows
// is test-pinned; the seam owns the contract, the protocol package
// conforms to it.
type Row string

const (
	RowDelivered       Row = "delivered"
	RowVersionMismatch Row = "version-mismatch"
	RowAuthRequired    Row = "auth-required"
	RowSetupError      Row = "setup-error"
	RowProtocolError   Row = "protocol-error"
	RowTurnFailed      Row = "turn-failed"
	RowCancelled       Row = "cancelled"
	RowRefused         Row = "refused"
	RowIncomplete      Row = "incomplete"
)

// Rows enumerates the vocabulary, for conformance tests.
func Rows() []Row {
	return []Row{RowDelivered, RowVersionMismatch, RowAuthRequired,
		RowSetupError, RowProtocolError, RowTurnFailed, RowCancelled,
		RowRefused, RowIncomplete}
}

// Declaration is the EXACT projection of the ratified capability
// vocabulary the adapters already snapshot — nothing more. The runtime
// and transport are registry keys, never type members: a Declaration
// describes what a session can do, not who provides it. The honest-
// emulator gaps (a runtime declaring nativeEvents false) are data the
// core plans around, not branches on a name.
type Declaration struct {
	Resume                   bool `json:"resume"`
	SessionEstablishedSignal bool `json:"sessionEstablishedSignal"`
	NativeStructuredOutput   bool `json:"nativeStructuredOutput"`
	NativeEvents             bool `json:"nativeEvents"`
	NativeUsage              bool `json:"nativeUsage"`
	GracefulCancel           bool `json:"gracefulCancel"`
	ProtocolServer           bool `json:"protocolServer"`
	NativeBudget             bool `json:"nativeBudget"`
}

// Endpoint is the pre-opened wire a Driver speaks over. The adapter
// script owns the launch that produced these pipes; the seam never
// learns how the process came to be.
type Endpoint struct {
	FromAgent io.Reader
	ToAgent   io.Writer
	Journal   io.Writer
}

// OpenRequest opens (or resumes) one delegate session.
type OpenRequest struct {
	Workspace       string
	ResumeSessionID string
	Endpoint        Endpoint
}

// Envelope is the permission envelope a prompt turn runs under. It
// mirrors the protocol package's envelope; parity is test-pinned.
type Envelope struct {
	ReadRoots  []string
	WriteRoots []string
	Network    string
	Approvals  string
	Tools      string
}

// PromptRequest is one prompt turn.
type PromptRequest struct {
	Prompt        []byte
	Envelope      Envelope
	Mode          string
	PromptTimeout time.Duration
	LateWindow    time.Duration
	CancelGrace   time.Duration
}

// Event is one advisory session update. Records commit, notifications
// accelerate: the stream is never the evidence, the journal is.
type Event struct {
	Seq    uint64
	Kind   string
	Params []byte
}

// AskOption mirrors the protocol's permission option; parity is
// test-pinned.
type AskOption struct {
	OptionID string
	Kind     string
	Name     string
}

// Ask is one server-initiated permission request awaiting an Answer.
// Seq places the ask in the SAME order stream as the events — the two
// views share one pump, and a consumer must be able to interleave
// them faithfully.
type Ask struct {
	ID      string
	Seq     uint64
	Options []AskOption
}

// Answer resolves one Ask.
type Answer struct {
	AskID    string
	OptionID string
	Cancel   bool
}

// EventStream and AskStream are views over the turn's ONE owned pump —
// the protocol reality is a single event loop, and two independent
// consumers must still observe protocol order and correlation.
type EventStream interface {
	Next(context.Context) (Event, bool)
}

// AskStream yields the asks the turn surfaces.
type AskStream interface {
	Next(context.Context) (Ask, bool)
}

// Usage is the turn's uniform usage accounting; Raw carries the
// runtime's native document for the usage owners.
type Usage struct {
	Available bool
	Raw       []byte
}

// Result is the turn's typed conclusion.
type Result struct {
	Row        Row
	StopReason string
	SessionID  string
	Candidate  []byte
	Violations int
	Detail     string
}

// Turn is one prompt turn in flight: its streams, its controls, and
// its settled outcome.
type Turn interface {
	EventStream() EventStream
	AskStream() AskStream
	Answer(context.Context, Answer) error
	Cancel(context.Context) error
	Usage(context.Context) (Usage, error)
	Result(context.Context) (Result, error)
}

// Session is one open delegate session. A Session MAY be one turn
// wide: PromptTurn reports exhaustion with ErrSessionExhausted, and
// reopening is the caller's move — the cardinality of TURNS per
// session is the driver's declaration, never a silent assumption.
type Session interface {
	PromptTurn(context.Context, PromptRequest) (Turn, error)
}

// ErrSessionExhausted is the typed refusal a one-shot Session
// returns for every PromptTurn after the first CLAIM of the session
// — the claim is consumed by entry, not by success.
var ErrSessionExhausted = errors.New("delegate: session exhausted; reopen to prompt again")

// Driver is a COMPLETE delegate-session implementation: it declares
// its capabilities and opens sessions. Read-side-only runtimes must
// register ports instead — see ports.go.
type Driver interface {
	Declaration() Declaration
	Open(context.Context, OpenRequest) (Session, error)
}

// Key is the ratified driver identity: (runtime, transport). The two
// dimensions are both data from the job record and the transport
// selection — a runtime can carry a native protocol driver and a CLI
// emulator side by side, and the registry must hold both without a
// redesign.
type Key struct {
	Runtime   string
	Transport string
}

func (k Key) String() string { return k.Runtime + "/" + k.Transport }

var (
	driversMu sync.Mutex
	drivers   = map[Key]Driver{}
)

// RegisterDriver registers a complete Driver under its (runtime,
// transport) key. A nil driver, a duplicate key, or an empty
// dimension panics at init time: the registry is wiring, and broken
// wiring must not survive to runtime.
func RegisterDriver(key Key, d Driver) {
	driversMu.Lock()
	defer driversMu.Unlock()
	if key.Runtime == "" || key.Transport == "" || d == nil {
		panic("delegate: driver registration needs a runtime, a transport, and an implementation")
	}
	if _, exists := drivers[key]; exists {
		panic(fmt.Sprintf("delegate: driver %s registered twice", key))
	}
	drivers[key] = d
}

// DriverFor returns the registered complete driver for a key.
func DriverFor(key Key) (Driver, error) {
	driversMu.Lock()
	defer driversMu.Unlock()
	d, ok := drivers[key]
	if !ok {
		return nil, fmt.Errorf("delegate: no complete driver registered for %s (registered: %v)", key, driverKeysLocked())
	}
	return d, nil
}

func driverKeysLocked() []string {
	keys := make([]string, 0, len(drivers))
	for key := range drivers {
		keys = append(keys, key.String())
	}
	sort.Strings(keys)
	return keys
}
