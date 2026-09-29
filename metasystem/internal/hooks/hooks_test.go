package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

func TestShippedStartMatcherComesFromRuntimeRegistry(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "runtimes", "enforcement", "claude-code-hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Hooks struct {
			SessionStart []struct {
				Matcher string `json:"matcher"`
			} `json:"SessionStart"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	declaration, ok := runtimes.Lookup("claude")
	if !ok || len(config.Hooks.SessionStart) != 1 {
		t.Fatalf("claude declaration or shipped SessionStart hook missing")
	}
	want := strings.Join(declaration.StartContextSources, "|")
	if got := config.Hooks.SessionStart[0].Matcher; got != want {
		t.Fatalf("shipped SessionStart matcher %q, want registry sources %q", got, want)
	}
}

func TestShippedHooksInstallPreToolUseWithFallback(t *testing.T) {
	shippedPath := filepath.Join("..", "..", "internal", "runtimes", "enforcement", "claude-code-hooks.json")
	data, err := os.ReadFile(shippedPath)
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Hooks struct {
			PreToolUse []struct {
				Matcher *string `json:"matcher"`
				Hooks   []struct {
					Type    string `json:"type"`
					Command string `json:"command"`
					Timeout int    `json:"timeout"`
				} `json:"hooks"`
			} `json:"PreToolUse"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Hooks.PreToolUse) != 1 {
		t.Fatalf("shipped PreToolUse groups = %d, want 1", len(config.Hooks.PreToolUse))
	}
	group := config.Hooks.PreToolUse[0]
	if group.Matcher != nil {
		t.Fatalf("shipped PreToolUse matcher = %q, want no matcher", *group.Matcher)
	}
	if len(group.Hooks) != 1 {
		t.Fatalf("shipped PreToolUse handlers = %d, want 1", len(group.Hooks))
	}
	handler := group.Hooks[0]
	if handler.Type != "command" || handler.Command != "(metasystem internal hook claude tool) || true" || handler.Timeout != 5 {
		t.Fatalf("shipped PreToolUse handler = %#v", handler)
	}

	liveData, err := MergeSettings([]byte(`{}`), data, "claude", "metasystem", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(liveData), `"PreToolUse"`) || !strings.Contains(string(liveData), "internal hook claude tool") {
		t.Fatalf("merged settings did not install the PreToolUse hook:\n%s", liveData)
	}
}
