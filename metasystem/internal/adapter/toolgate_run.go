package adapter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
	"golang.org/x/sys/unix"
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
	Session     string `json:"session"`
	At          string `json:"at"`
	Tokens      int64  `json:"tokens"`
	Tool        string `json:"tool"`
	Mode        string `json:"mode"`
	Decision    string `json:"decision"`
	WouldDeny   bool   `json:"wouldDeny"`
	Cause       string `json:"cause"`
	ElapsedMs   int64  `json:"elapsedMs"`
	Birth       string `json:"birth"`
	Reason      string `json:"reason,omitempty"`
	ReserveUsed bool   `json:"reserveUsed,omitempty"`
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
	reserve := ReserveReading{Size: budget.Reserve}
	decision := Decide(class, tokens, budget, reserve, opts.Installation)
	if decision.Cause == "under-trigger" {
		return nil
	}
	reserveClass := isReserveClass(class)
	var locked *os.File
	if reserveClass {
		reserve, locked = readToolGateReserve(opts, payload.SessionID, reserve, deadline)
		decision = Decide(class, tokens, budget, reserve, opts.Installation)
		if locked != nil {
			defer locked.Close()
			defer unix.Flock(int(locked.Fd()), unix.LOCK_UN)
		}
	}
	decidedAt := opts.Clock()
	if !reserveClass && !decidedAt.Before(deadline) {
		writeToolGateRow(opts, toolGateDecisionRow{
			Session: payload.SessionID, Tool: payload.Tool, Mode: opts.Mode,
			Decision: "allow", WouldDeny: decision.Deny, Cause: "deadline", Birth: birth,
			Tokens: tokens,
		}, base, decidedAt)
		return nil
	}

	row := toolGateDecisionRow{
		Session: payload.SessionID, Tool: payload.Tool, Mode: opts.Mode,
		Decision: "allow", WouldDeny: decision.Deny, Cause: decision.Cause, Birth: birth,
		Tokens:      tokens,
		ReserveUsed: reserveClass && (!decision.Deny || opts.Mode == "observe"),
	}
	if decision.Deny && opts.Mode == "deny" {
		row.Decision = "deny"
	}
	if decision.Deny && opts.Mode == "observe" {
		row.Reason = decision.Reason
	}
	var writeErr error
	if locked != nil {
		writeErr = appendToolGateRow(locked, row, base, decidedAt, true)
	} else {
		writeToolGateRow(opts, row, base, decidedAt)
	}
	if writeErr != nil && reserve.Fault == "" {
		reserve.Fault = "reserve-unrecordable"
		decision = Decide(class, tokens, budget, reserve, opts.Installation)
		row.Decision, row.WouldDeny, row.Cause, row.ReserveUsed = "deny", true, decision.Cause, opts.Mode == "observe"
		if opts.Mode == "observe" {
			row.Decision, row.Reason = "allow", decision.Reason
		}
		writeToolGateRow(opts, row, base, opts.Clock())
	}

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
	return class.Row != nil && class.Row.trigger && class.Row.ceiling == ceilingAllow
}

func isReserveClass(class Classification) bool {
	return class.Row != nil && class.Row.ceiling == ceilingDenyReserve
}

func readToolGateReserve(opts ToolGateOptions, session string, reserve ReserveReading, deadline time.Time) (ReserveReading, *os.File) {
	fail := func(cause string) (ReserveReading, *os.File) { reserve.Fault = cause; return reserve, nil }
	path := toolGateLogPath(opts.StateRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fail("reserve-unrecordable")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return fail("reserve-unrecordable")
	}

	for {
		err = unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			break
		}
		if err != unix.EWOULDBLOCK && err != unix.EAGAIN {
			file.Close()
			return fail("reserve-unrecordable")
		}
		if !opts.Clock().Before(deadline) {
			file.Close()
			return fail("reserve-busy")
		}
	}
	reserve.Used, err = countReserveRows(file, session)
	if err != nil {
		reserve.Fault = "reserve-unrecordable"
	}
	return reserve, file
}

func countReserveRows(file *os.File, session string) (int64, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return 0, err
	}
	var used int64
	lines := strings.Split(string(data), "\n")
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		used++
		lines = lines[:len(lines)-1]
	}
	for _, line := range lines {
		if line == "" {
			continue
		}
		var row toolGateDecisionRow
		if json.Unmarshal([]byte(line), &row) != nil {
			used++
			continue
		}
		if row.Session == session && row.ReserveUsed {
			used++
		}
	}
	return used, nil
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

func appendToolGateRow(file *os.File, row toolGateDecisionRow, base, at time.Time, sync bool) error {
	row.At = at.UTC().Format(time.RFC3339Nano)
	row.ElapsedMs = at.Sub(base).Milliseconds()
	encoded, err := json.Marshal(row)
	if err != nil {
		return err
	}
	if _, err = file.Write(append(encoded, '\n')); err != nil {
		return err
	}
	if sync {
		return file.Sync()
	}
	return nil
}

func writeToolGateRow(opts ToolGateOptions, row toolGateDecisionRow, base, at time.Time) {
	path := toolGateLogPath(opts.StateRoot)
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err == nil {
		var file *os.File
		file, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			err = appendToolGateRow(file, row, base, at, false)
			if closeErr := file.Close(); err == nil {
				err = closeErr
			}
		}
	}
	if err != nil && opts.Stderr != nil {
		fmt.Fprintf(opts.Stderr, "metasystem adapter claude-tool-gate: write decision row: %v\n", err)
	}
}

func toolGateLogPath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "context", "tool-gate.jsonl")
}
