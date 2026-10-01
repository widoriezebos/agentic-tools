package agentgate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The landing launch's settings carry one PreToolUse handler for every tool
// that runs the lane engine's hook entry and exits 2 when it cannot; a path
// the shell would misread is refused rather than rendered.
func TestClaudeSettingsCarryTheFailClosedGate(t *testing.T) {
	t.Parallel()
	data, err := ClaudeSettings("/lane/metasystem/bin/metasystem", "/lane")
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Hooks map[string][]struct {
			Matcher *string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
				Timeout int    `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	groups := settings.Hooks["PreToolUse"]
	if len(settings.Hooks) != 1 || len(groups) != 1 || groups[0].Matcher != nil || len(groups[0].Hooks) != 1 {
		t.Fatalf("settings = %s", data)
	}
	handler := groups[0].Hooks[0]
	if handler.Type != "command" || handler.Timeout != HookTimeoutSeconds ||
		!strings.Contains(handler.Command, LineageEnv+"="+Lineage+" '/lane/metasystem/bin/metasystem' internal hook claude tool") ||
		!strings.HasSuffix(handler.Command, "exit 2; }") {
		t.Fatalf("handler = %+v", handler)
	}
	for _, bad := range [][2]string{{"bin/metasystem", "/lane"}, {"/lane/bin/metasystem", "/la'ne"}, {"/lane/bin/metasystem", "/lane\n"}} {
		if _, err := ClaudeSettings(bad[0], bad[1]); err == nil {
			t.Fatalf("rendered %q %q", bad[0], bad[1])
		}
	}
}

// WriteClaudeSettings is what the landing launch calls: it writes the gate's
// settings file into the launch's own state directory, never into the lane
// checkout the agent may edit, and returns the path the launch records under
// AdapterData "settings".
func TestWriteClaudeSettingsOutsideTheCheckout(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	checkout := filepath.Join(root, "lane")
	engine := filepath.Join(checkout, "metasystem", "bin", "metasystem")
	state := filepath.Join(root, "state", "launch-1")
	path, err := WriteClaudeSettings(state, engine, checkout)
	if err != nil {
		t.Fatal(err)
	}
	want, err := ClaudeSettings(engine, checkout)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(want) || filepath.Dir(path) != state {
		t.Fatalf("path %s content %q err %v", path, got, err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode: %v %v", info, err)
	}
	for _, bad := range []string{filepath.Join(checkout, "state"), checkout, "relative/state"} {
		if _, err := WriteClaudeSettings(bad, engine, checkout); err == nil {
			t.Errorf("wrote the gate's settings at %s", bad)
		}
	}
}
