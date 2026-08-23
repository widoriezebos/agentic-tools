package steward

// The narration is the system's running plain-English account of what
// it is doing — one sentence per tick, written for a person, kept
// beside the machine records it summarizes. It is strictly read-only
// observation: a narration failure never fails a tick, narration never
// decides anything, and the records remain the authority. The journey
// tells the story of what concluded; the narration tells the story of
// now.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// narrationCapLines bounds the file: old ticks scroll away, because a
// running account that grows forever stops being readable and starts
// being a disk problem.
const narrationCapLines = 2000

// NarrationPath is the account's one location.
func NarrationPath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "narration.log")
}

// Narrate appends one tick's sentence. Best-effort by contract: every
// failure path returns silently, because the tick's real duties must
// never hang on the storyteller.
func Narrate(repoRoot string, result TickResult) {
	line := narrationLine(repoRoot, result, time.Now())
	if line == "" {
		return
	}
	path := NarrationPath(repoRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	existing, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimRight(string(existing), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	lines = append(lines, line)
	if len(lines) > narrationCapLines {
		lines = lines[len(lines)-narrationCapLines:]
	}
	staged, err := os.CreateTemp(filepath.Dir(path), ".narration-*")
	if err != nil {
		return
	}
	if _, err := staged.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		staged.Close()
		os.Remove(staged.Name())
		return
	}
	staged.Close()
	_ = os.Rename(staged.Name(), path)
}

// narrationLine composes the sentence: when, who, what the machine is
// doing, and anything a person would want to know about this tick —
// in the narrator's plain-English register, no identifiers a reader
// would have to look up.
func narrationLine(repoRoot string, result TickResult, now time.Time) string {
	machine := "this machine"
	if name, err := goal.ResolveMachine(repoRoot); err == nil {
		machine = name
	}
	var doing string
	switch {
	case strings.HasPrefix(result.OpenWork, "claimed goal: "):
		doing = "working on " + strings.TrimPrefix(result.OpenWork, "claimed goal: ")
	case strings.HasPrefix(result.OpenWork, "current goal: "):
		doing = "working on " + strings.TrimPrefix(result.OpenWork, "current goal: ")
	case result.OpenWork == "goal-free declared":
		doing = "at rest — the all-clear is declared"
	case strings.Contains(result.OpenWork, "queued goals await"):
		doing = "idle while work waits in the queue (" + result.OpenWork + ")"
	case result.OpenWork == "":
		doing = "observing"
	default:
		doing = result.OpenWork
	}
	var notes []string
	if len(result.Reaped) > 0 {
		notes = append(notes, fmt.Sprintf("closed %d finished helper run(s)", len(result.Reaped)))
	}
	if result.Decision.Action == ActNotify {
		notes = append(notes, "flagged something for the operator: "+result.Decision.Reason)
	}
	if result.Decision.Action == ActRevive {
		notes = append(notes, "reviving stalled work: "+result.Decision.Reason)
	}
	sentence := now.Format("2006-01-02 15:04") + "  " + machine + " is " + doing
	if len(notes) > 0 {
		sentence += "; " + strings.Join(notes, "; ")
	}
	return sentence + "."
}
