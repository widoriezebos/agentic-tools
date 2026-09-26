package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// TestLaunchManagerReadsClaudeTranscriptsWhereClaudeKeepsThem: the Claude
// adapter measures a session from the transcripts under CLAUDE_CONFIG_DIR
// when it is set, as Claude Code writes them there, else under ~/.claude.
func TestLaunchManagerReadsClaudeTranscriptsWhereClaudeKeepsThem(t *testing.T) {
	config := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", config)
	adapter, ok := newLaunchManager().Adapters["claude-headless"].(launch.ClaudeHeadless)
	if !ok || adapter.ProjectsRoot != filepath.Join(config, "projects") {
		t.Fatalf("claude projects root = %+v", adapter)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if adapter := newLaunchManager().Adapters["claude-headless"].(launch.ClaudeHeadless); adapter.ProjectsRoot != filepath.Join(home, ".claude", "projects") {
		t.Fatalf("default claude projects root = %s", adapter.ProjectsRoot)
	}
}
