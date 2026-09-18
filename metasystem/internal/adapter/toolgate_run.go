package adapter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

const (
	toolGateDeadline       = 100 * time.Millisecond
	toolGateObservationMax = int64(262144)

	// ShellEntryAllowanceMs reserves the portion of the hook budget spent
	// before this process entered when the shell's kernel birth is unreadable.
	ShellEntryAllowanceMs = 25
)

// ToolGateOptions supplies every process, clock, filesystem, and stream input
// needed to decide one Claude tool call.
type ToolGateOptions struct {
	ShellStartedAt time.Time
	Clock          func() time.Time
	MemoryDir      string
	Mode           string
	StateRoot      string
	Installation   string
	Stdin          io.Reader
	Stdout         io.Writer
	Stderr         io.Writer
}

type toolGatePayload struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	AgentID        string `json:"agent_id"`
	Call
}

type toolGateDecisionRow struct {
	Session   string `json:"session"`
	At        string `json:"at"`
	Tokens    int64  `json:"tokens"`
	Tool      string `json:"tool"`
	Mode      string `json:"mode"`
	Decision  string `json:"decision"`
	WouldDeny bool   `json:"wouldDeny"`
	Cause     string `json:"cause"`
	ElapsedMs int64  `json:"elapsedMs"`
	Birth     string `json:"birth"`
	Reason    string `json:"reason,omitempty"`
}

// RunToolGate decides one PreToolUse payload. Operational failures fail open
// and are recorded; malformed inputs and configuration are returned to the
// command boundary for diagnostics.
func RunToolGate(opts ToolGateOptions) error {
	if opts.Clock == nil {
		return fmt.Errorf("tool gate clock is required")
	}
	entry := opts.Clock()
	base := opts.ShellStartedAt
	birth := "unreadable"
	deadline := entry.Add(toolGateDeadline - time.Duration(ShellEntryAllowanceMs)*time.Millisecond)
	if !base.IsZero() {
		birth = base.UTC().Format(time.RFC3339Nano)
		deadline = base.Add(toolGateDeadline)
	} else {
		base = entry
	}

	var payload toolGatePayload
	if opts.Stdin == nil {
		return fmt.Errorf("tool gate input is required")
	}
	if err := json.NewDecoder(opts.Stdin).Decode(&payload); err != nil {
		return fmt.Errorf("decode Claude tool hook payload: %w", err)
	}
	if payload.AgentID != "" {
		return nil
	}

	class := Classify(payload.Call, opts.MemoryDir)
	if trivialToolGateAllow(class) {
		return nil
	}

	budget, err := config.ContextBudget(opts.Installation)
	if err != nil {
		return err
	}
	if opts.Mode != "observe" && opts.Mode != "deny" {
		return fmt.Errorf("invalid tool gate mode %q", opts.Mode)
	}

	readOptions := toolGateReadOptions(payload.TranscriptPath, deadline, opts.Clock)
	readOptions.Capability = usage.PerCall
	reading, readErr := usage.LatestCall(opts.StateRoot, "claude", payload.SessionID, readOptions)
	if readErr != nil {
		cause := "read-error"
		if toolGateBusy(readErr) {
			cause = "busy"
		}
		writeToolGateRow(opts, toolGateDecisionRow{
			Session: payload.SessionID, Tool: payload.Tool, Mode: opts.Mode,
			Decision: "allow", Cause: cause, Birth: birth,
		}, base, opts.Clock())
		return nil
	}
	if reading.Reason == "deadline" {
		writeToolGateRow(opts, toolGateDecisionRow{
			Session: payload.SessionID, Tool: payload.Tool, Mode: opts.Mode,
			Decision: "allow", Cause: "deadline", Birth: birth,
		}, base, opts.Clock())
		return nil
	}
	if reading.Latest == nil {
		writeToolGateRow(opts, toolGateDecisionRow{
			Session: payload.SessionID, Tool: payload.Tool, Mode: opts.Mode,
			Decision: "allow", Cause: "no-sample", Birth: birth,
		}, base, opts.Clock())
		return nil
	}

	tokens := reading.Latest.PromptTokens
	decision := Decide(class, tokens, budget, opts.Installation)
	decidedAt := opts.Clock()
	if !decidedAt.Before(deadline) {
		writeToolGateRow(opts, toolGateDecisionRow{
			Session: payload.SessionID, Tool: payload.Tool, Mode: opts.Mode,
			Decision: "allow", WouldDeny: decision.Deny, Cause: "deadline", Birth: birth,
			Tokens: tokens,
		}, base, decidedAt)
		return nil
	}
	if decision.Cause == "under-trigger" {
		return nil
	}

	row := toolGateDecisionRow{
		Session: payload.SessionID, Tool: payload.Tool, Mode: opts.Mode,
		Decision: "allow", WouldDeny: decision.Deny, Cause: decision.Cause, Birth: birth,
		Tokens: tokens,
	}
	if decision.Deny && opts.Mode == "deny" {
		row.Decision = "deny"
	}
	if decision.Deny && opts.Mode == "observe" {
		row.Reason = decision.Reason
	}
	writeToolGateRow(opts, row, base, decidedAt)

	if decision.Deny && opts.Mode == "deny" {
		output := append(decision.Output(), '\n')
		if opts.Stdout == nil {
			return fmt.Errorf("tool gate output is required")
		}
		if _, err := opts.Stdout.Write(output); err != nil {
			return fmt.Errorf("write Claude tool gate decision: %w", err)
		}
	}
	return nil
}

func trivialToolGateAllow(class Classification) bool {
	if class.Kind != NeverDenied && class.Kind != AllowedAtTrigger {
		return false
	}
	return class.Row != nil && class.Row.trigger && class.Row.ceiling
}

func toolGateReadOptions(transcript string, deadline time.Time, clock func() time.Time) usage.ReadOptions {
	return usage.ReadOptions{
		Transcript: transcript, NonBlocking: true, MaxBytes: toolGateObservationMax,
		Deadline: deadline, Clock: clock,
	}
}

func toolGateBusy(err error) bool {
	var storeBusy *usage.CallStoreBusyError
	var cursorBusy *usage.CursorBusyError
	return errors.As(err, &storeBusy) || errors.As(err, &cursorBusy)
}

func writeToolGateRow(opts ToolGateOptions, row toolGateDecisionRow, base, at time.Time) {
	row.At = at.UTC().Format(time.RFC3339Nano)
	row.ElapsedMs = at.Sub(base).Milliseconds()
	encoded, err := json.Marshal(row)
	if err == nil {
		path := filepath.Join(opts.StateRoot, "artifacts", "agents", "context", "tool-gate.jsonl")
		if err = os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
			var file *os.File
			file, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err == nil {
				_, err = file.Write(append(encoded, '\n'))
				closeErr := file.Close()
				if err == nil {
					err = closeErr
				}
			}
		}
	}
	if err != nil && opts.Stderr != nil {
		fmt.Fprintf(opts.Stderr, "metasystem adapter claude-tool-gate: write decision row: %v\n", err)
	}
}
