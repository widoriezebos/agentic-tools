package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/acp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
)

// The acp family is the shell-callable surface of internal/acp:
// the script owns launch, custody, and killing (the fifo topology
// per plans/acp-transport-design.md); these verbs own only the
// wire. `preflight` is the pre-launch envelope check so a refusal
// happens before any process is spawned; `turn` drives one prompt
// attempt over already-created pipe paths and emits the typed
// outcome as one JSON object on stdout.

type envelopeFile struct {
	ReadRoots  []string `json:"readRoots"`
	WriteRoots []string `json:"writeRoots"`
	Network    string   `json:"network"`
	Approvals  string   `json:"approvals"`
	Tools      string   `json:"tools"`
}

func loadEnvelope(path string) (acp.Envelope, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return acp.Envelope{}, err
	}
	var parsed envelopeFile
	if err := json.Unmarshal(body, &parsed); err != nil {
		return acp.Envelope{}, fmt.Errorf("envelope %s: %w", path, err)
	}
	return acp.Envelope{
		ReadRoots:  parsed.ReadRoots,
		WriteRoots: parsed.WriteRoots,
		Network:    parsed.Network,
		Approvals:  parsed.Approvals,
		Tools:      parsed.Tools,
	}, nil
}

func runACPPreflight(args []string) int {
	f := flag.NewFlagSet("acp preflight", flag.ContinueOnError)
	envelopePath := f.String("envelope-file", "", "path to the expanded five-field envelope JSON")
	if err := f.Parse(args); err != nil || *envelopePath == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem acp preflight --envelope-file F")
		return 2
	}
	envelope, err := loadEnvelope(*envelopePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if reason := acp.PreflightACP(envelope); reason != "" {
		fmt.Println(reason)
		return 1
	}
	return 0
}

// turnOutcome is the stdout wire shape the adapter script consumes.
type turnOutcome struct {
	Row          string          `json:"row"`
	StopReason   string          `json:"stopReason,omitempty"`
	SessionID    string          `json:"sessionId,omitempty"`
	Candidate    *string         `json:"candidate,omitempty"`
	Usage        json.RawMessage `json:"usage,omitempty"`
	Violations   int             `json:"violations"`
	Detail       string          `json:"detail,omitempty"`
	JournalError string          `json:"journalError,omitempty"`
}

func runACPTurn(args []string) int {
	f := flag.NewFlagSet("acp turn", flag.ContinueOnError)
	serverOut := f.String("server-out", "", "path to the server's stdout pipe (this verb's read side)")
	serverIn := f.String("server-in", "", "path to the server's stdin pipe (this verb's write side)")
	journalPath := f.String("journal", "", "path for the raw wire journal (created, append-only)")
	workspace := f.String("workspace", "", "session cwd (absolute)")
	envelopePath := f.String("envelope-file", "", "path to the expanded five-field envelope JSON")
	promptFile := f.String("prompt-file", "", "path to the prompt text")
	loadSession := f.String("load-session", "", "session id to load instead of creating one")
	modeID := f.String("mode", "", "dialect-resolved session mode to set (empty: leave default)")
	expectedProtocol := f.Int64("expected-protocol", 1, "expected ACP protocol version")
	handshakeSec := f.Int("handshake-timeout-sec", 120, "handshake phase deadline")
	promptSec := f.Int("prompt-timeout-sec", 1800, "prompt phase deadline")
	lateMs := f.Int("late-window-ms", 2000, "late-frame drain window")
	if err := f.Parse(args); err != nil {
		return 2
	}
	if *serverOut == "" || *serverIn == "" || *journalPath == "" || *workspace == "" || *envelopePath == "" || *promptFile == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem acp turn --server-out P --server-in P --journal P --workspace D --envelope-file F --prompt-file F [--load-session ID] [--mode M] [--expected-protocol N] [--*-timeout-sec N]")
		return 2
	}
	envelope, err := loadEnvelope(*envelopePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if reason := acp.PreflightACP(envelope); reason != "" {
		fmt.Fprintln(os.Stderr, "preflight refused: "+reason)
		return 1
	}
	prompt, err := os.ReadFile(*promptFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	// One attempt, one journal: an existing file is a path
	// collision that would concatenate stale sessions into
	// settlement evidence — refuse, never append or truncate.
	journal, err := os.OpenFile(*journalPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "journal create: "+err.Error())
		return 1
	}

	// Open order is the script's contract: the read side first
	// (the server opens its write side symmetrically), then the
	// write side. Regular files work too, which the fixtures use.
	readEnd, err := os.OpenFile(*serverOut, os.O_RDONLY, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "server-out open: "+err.Error())
		return 1
	}
	writeEnd, err := os.OpenFile(*serverIn, os.O_WRONLY, 0)
	if err != nil {
		readEnd.Close()
		fmt.Fprintln(os.Stderr, "server-in open: "+err.Error())
		return 1
	}

	// The signal bridge: scripts keep kill authority, and a TERM or
	// INT lands here as parent-context cancellation so the courtesy
	// session/cancel and the typed cancelled outcome still happen —
	// the critique's live probe showed exit 143 with no outcome
	// without this.
	ctx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stopSignals()

	conn := acp.NewConn(readEnd, writeEnd, journal)
	outcome := acp.RunTurn(ctx, conn, acp.TurnConfig{
		ExpectedProtocolVersion: *expectedProtocol,
		WorkspaceDir:            *workspace,
		LoadSessionID:           *loadSession,
		ModeID:                  *modeID,
		Prompt:                  string(prompt),
		Envelope:                envelope,
		HandshakeTimeout:        time.Duration(*handshakeSec) * time.Second,
		PromptTimeout:           time.Duration(*promptSec) * time.Second,
		LateFrameWindow:         time.Duration(*lateMs) * time.Millisecond,
	})

	// Quiesce before sampling journal health: close the pipe ends
	// so the read loop terminates, wait for it (bounded), then sync
	// and close the journal — the settlement journal must be whole
	// BEFORE the outcome claims it is.
	writeEnd.Close()
	readEnd.Close()
	select {
	case <-conn.Done():
	case <-time.After(3 * time.Second):
	}
	journalIssue := conn.JournalErr()
	if err := journal.Sync(); err != nil && journalIssue == nil {
		journalIssue = err
	}
	if err := journal.Close(); err != nil && journalIssue == nil {
		journalIssue = err
	}

	wire := turnOutcome{
		Row:        string(outcome.Row),
		StopReason: outcome.StopReason,
		SessionID:  outcome.SessionID,
		Usage:      outcome.UsageResult,
		Violations: outcome.Violations,
		Detail:     outcome.Detail,
	}
	if outcome.Candidate != nil {
		text := string(outcome.Candidate)
		wire.Candidate = &text
	}
	if journalIssue != nil {
		// A thinned journal must be visible to settlement even when
		// the turn otherwise delivered.
		wire.JournalError = journalIssue.Error()
	}
	payload, _ := json.Marshal(wire)
	fmt.Println(string(payload))
	return 0
}

// runACPMode resolves the adapter-owned dialect: which session
// mode a runtime's envelope tools grade maps to. The mode is the
// enforcement lever on this transport, so the mapping is a lookup
// in declared, behaviorally evidenced data — never computed at
// dispatch time.
func runACPMode(args []string) int {
	f := flag.NewFlagSet("acp mode", flag.ContinueOnError)
	runtimeName := f.String("runtime", "", "runtime whose dialect to consult")
	tools := f.String("tools", "", "envelope tools grade")
	if err := f.Parse(args); err != nil || *runtimeName == "" || *tools == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem acp mode --runtime R --tools GRADE")
		return 2
	}
	dialect, err := adapter.ACPDialectFor(*runtimeName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	mode := dialect.ModeForTools[*tools]
	if mode == "" {
		fmt.Fprintln(os.Stderr, "no mode mapped for tools="+*tools)
		return 1
	}
	fmt.Println(mode)
	return 0
}
