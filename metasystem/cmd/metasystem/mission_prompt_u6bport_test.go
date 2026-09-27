package main

import (
	"os"
	"path/filepath"
	"testing"
)

// mission-runner scenario, prompt-missing-turn: the prompt-assemble verb
// exits 1 on the assembler's refusal and writes no prompt.
func TestU6bPortPromptAssembleRefusalExitsOne(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	output := filepath.Join(t.TempDir(), "missing-prompt.md")
	code := runMissionPromptAssemble([]string{"--repo", repo, "--mission", "runner-cycle",
		"--turn", "runner-cycle-t99-missing", "--output", output})
	if code != 1 {
		t.Fatalf("prompt-assemble refusal exited %d, want 1", code)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("a refused assembly wrote a prompt: %v", err)
	}
}
