package mission

import (
	"path/filepath"
	"strings"
	"testing"
)

// mission-runner scenario, prompt-missing-turn: the assembler refuses a
// turn id with no turn record and names the refusal, so the operator sees
// what is missing rather than a bare failure.
func TestU6bPortAssemblePromptNamesTheMissingTurnRecord(t *testing.T) {
	repo := promptSandbox(t)
	err := AssemblePrompt(repo, "m1", "m1-t99-missing", filepath.Join(t.TempDir(), "prompt.md"))
	if err == nil || !strings.Contains(err.Error(), "missing turn record") {
		t.Fatalf("a missing turn record must be refused by name: %v", err)
	}
}
