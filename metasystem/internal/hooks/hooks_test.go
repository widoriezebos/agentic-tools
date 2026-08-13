package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	if err := CheckOwnHooks(live, s); err != nil {
		t.Fatalf("a compliant repository should pass: %v", err)
	}
}

func TestCheckOwnHooksReportsMissingHook(t *testing.T) {
	dir := t.TempDir()
	live := write(t, dir, "live.json", `{"hooks":{"SessionStart":[{"hooks":[{"command":"$CLAUDE_PROJECT_DIR/metasystem/scripts/agents/supervision-hook.sh"}]}]}}`)
	s := write(t, dir, "shipped.json", shipped)
	err := CheckOwnHooks(live, s)
	if err == nil || !strings.Contains(err.Error(), "missing its own lifecycle hooks") {
		t.Fatalf("a missing Stop hook must be reported, got %v", err)
	}
}

func TestCheckOwnHooksRequiresSupervisionHook(t *testing.T) {
	dir := t.TempDir()
	live := write(t, dir, "live.json", `{"hooks":{"SessionStart":[{"hooks":[{"command":"$CLAUDE_PROJECT_DIR/metasystem/other.sh"}]}],"Stop":[]}}`)
	s := write(t, dir, "shipped.json", shipped)
	if err := CheckOwnHooks(live, s); err == nil || !strings.Contains(err.Error(), "supervision hook") {
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
	err := CheckOwnHooks(live, s)
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
	if err := CheckOwnHooks(live, s); err == nil || !strings.Contains(err.Error(), "vendored metasystem directory") {
		t.Fatalf("hooks that do not enter the vendored dir must be reported, got %v", err)
	}
}
