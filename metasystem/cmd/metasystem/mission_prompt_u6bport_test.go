package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mission-runner scenario, prompt-missing-turn: the prompt-assemble verb
// exits 1 on the assembler's refusal and writes no prompt.
func TestU6bPortPromptAssembleRefusalExitsOne(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	output := filepath.Join(t.TempDir(), "missing-prompt.md")
	var stderr strings.Builder
	code := missionPromptAssembleTo([]string{"--repo", repo, "--mission", "runner-cycle",
		"--turn", "runner-cycle-t99-missing", "--output", output}, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "mission prompt refused: missing turn record") {
		t.Fatalf("prompt-assemble refusal exited %d, want 1 naming the missing turn: %q", code, stderr.String())
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("a refused assembly wrote a prompt: %v", err)
	}
}
