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
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "enforcement", "claude-code-hooks.json"))
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
	shippedPath := filepath.Join("..", "..", "scripts", "enforcement", "claude-code-hooks.json")
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
	if handler.Type != "command" || handler.Command != "(bash scripts/agents/supervision-hook.sh claude tool) || true" || handler.Timeout != 5 {
		t.Fatalf("shipped PreToolUse handler = %#v", handler)
	}

	liveData, err := MergeSettings([]byte(`{}`), data, "claude", "metasystem", false)
	if err != nil {
		t.Fatal(err)
	}
	livePath := write(t, t.TempDir(), "settings.json", string(liveData))
	if err := CheckOwnHooks(livePath, shippedPath, `$repo/metasystem`); err != nil {
		t.Fatalf("installed PreToolUse hook failed the repository's own-hook check: %v", err)
	}
}

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The shipped fixture models the real configuration: the supervision hook
// rides SPECIFIC events (here SessionStart), nested the way the settings
// dialect nests commands.
const shipped = `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"cd \"$CLAUDE_PROJECT_DIR/metasystem\" && bash scripts/agents/supervision-hook.sh claude start"}]}],"Stop":[]}}`

func TestCheckOwnHooksAcceptsCompliant(t *testing.T) {
	dir := t.TempDir()
	live := write(t, dir, "live.json", `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"cd \"$CLAUDE_PROJECT_DIR/metasystem\" && bash scripts/agents/supervision-hook.sh claude start"}]}],"Stop":[]}}`)
	s := write(t, dir, "shipped.json", shipped)
	if err := CheckOwnHooks(live, s, "$CLAUDE_PROJECT_DIR/metasystem"); err != nil {
		t.Fatalf("a compliant repository should pass: %v", err)
	}
}

func TestCheckOwnHooksReportsMissingHook(t *testing.T) {
	dir := t.TempDir()
	live := write(t, dir, "live.json", `{"hooks":{"SessionStart":[{"hooks":[{"command":"$CLAUDE_PROJECT_DIR/metasystem/scripts/agents/supervision-hook.sh"}]}]}}`)
	s := write(t, dir, "shipped.json", shipped)
	err := CheckOwnHooks(live, s, "$CLAUDE_PROJECT_DIR/metasystem")
	if err == nil || !strings.Contains(err.Error(), "missing its own lifecycle hooks") {
		t.Fatalf("a missing Stop hook must be reported, got %v", err)
	}
}

func TestCheckOwnHooksRequiresSupervisionHook(t *testing.T) {
	dir := t.TempDir()
	live := write(t, dir, "live.json", `{"hooks":{"SessionStart":[{"hooks":[{"command":"$CLAUDE_PROJECT_DIR/metasystem/other.sh"}]}],"Stop":[]}}`)
	s := write(t, dir, "shipped.json", shipped)
	if err := CheckOwnHooks(live, s, "$CLAUDE_PROJECT_DIR/metasystem"); err == nil || !strings.Contains(err.Error(), "supervision hook") {
		t.Fatalf("absent supervision hook must be reported, got %v", err)
	}
}

// foundations-11's named false-passes: the supervision command moved to an
// event that never fires at session start, with the path string appearing
// in an unrelated hook's arguments, must NOT read as compliant.
func TestCheckOwnHooksRejectsMovedSupervisionHook(t *testing.T) {
	dir := t.TempDir()
	live := write(t, dir, "live.json", `{"hooks":{
		"SessionStart":[{"hooks":[{"command":"echo see $CLAUDE_PROJECT_DIR/metasystem and supervision-hook.sh docs"}]}],
		"Stop":[{"hooks":[{"command":"$CLAUDE_PROJECT_DIR/metasystem/scripts/agents/supervision-hook.sh"}]}]}}`)
	s := write(t, dir, "shipped.json", shipped)
	err := CheckOwnHooks(live, s, "$CLAUDE_PROJECT_DIR/metasystem")
	if err != nil {
		// The mention rides an unrelated echo INSIDE SessionStart, which
		// still satisfies a per-event substring scan — the structural
		// check must key on the command that actually invokes the hook.
		return
	}
	t.Fatal("a supervision hook moved out of its shipped event read as compliant")
}

func TestCheckOwnHooksRequiresVendoredEntry(t *testing.T) {
	dir := t.TempDir()
	live := write(t, dir, "live.json", `{"hooks":{"SessionStart":[{"hooks":[{"command":"bash scripts/agents/supervision-hook.sh claude start"}]}],"Stop":[]}}`)
	s := write(t, dir, "shipped.json", shipped)
	if err := CheckOwnHooks(live, s, "$CLAUDE_PROJECT_DIR/metasystem"); err == nil || !strings.Contains(err.Error(), "vendored metasystem directory") {
		t.Fatalf("hooks that do not enter the vendored dir must be reported, got %v", err)
	}
}
