package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The unit launcher passes the sandbox its record carries to codex exec:
// workspace-write when the record has none, as it always has, and
// danger-full-access when the admitting installation said so. A value outside
// the two modes is refused.
func TestCodexExecRunsUnderTheRecordedSandbox(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	briefPath := filepath.Join(root, "brief.md")
	if err := os.WriteFile(briefPath, []byte("build it\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stateDir := t.TempDir()
	for _, test := range []struct{ sandbox, want string }{
		{"", "workspace-write"},
		{"workspace-write", "workspace-write"},
		{"danger-full-access", "danger-full-access"},
	} {
		data := map[string]json.RawMessage{}
		setString(data, "brief", briefPath)
		setString(data, "model", "gpt-fixture")
		setString(data, "effort", "high")
		if test.sandbox != "" {
			setString(data, "sandbox", test.sandbox)
		}
		command, err := (CodexExec{Binary: "codex"}).Command(Record{Kind: "build", WorkingDirectory: root, AdapterData: data}, stateDir)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"exec", "-m", "gpt-fixture", "-c", "model_reasoning_effort=high", "-C", root,
			"-s", test.want, "-o", filepath.Join(stateDir, "last-message.txt"), "-"}
		if strings.Join(command.Args, "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("sandbox %q argv:\n got %q\nwant %q", test.sandbox, command.Args, want)
		}
	}
	data := map[string]json.RawMessage{}
	setString(data, "brief", briefPath)
	setString(data, "sandbox", "none")
	if _, err := (CodexExec{Binary: "codex"}).Command(Record{Kind: "build", WorkingDirectory: root, AdapterData: data}, stateDir); err == nil || !strings.Contains(err.Error(), "workspace-write or danger-full-access") {
		t.Fatalf("an unknown recorded sandbox = %v", err)
	}
}

// The host's sandbox resolves from its local settings like every launch
// setting, defaults to workspace-write, and refuses a value outside the two
// modes.
func TestSettingsResolveTheCodexSandbox(t *testing.T) {
	t.Parallel()
	none := func(string) (string, bool) { return "", false }
	for _, test := range []struct{ local, want, source string }{
		{"", "workspace-write", "default"},
		{CodexSandboxKey + "=danger-full-access\n", "danger-full-access", "conf-local"},
	} {
		conf := filepath.Join(t.TempDir(), "settings.conf")
		if err := os.WriteFile(conf, []byte("metasystem.runtimes=claude,codex\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if test.local != "" {
			if err := os.WriteFile(conf+".local", []byte(test.local), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		settings, err := ResolveSettings(conf, none)
		if err != nil {
			t.Fatal(err)
		}
		source := ""
		for _, value := range settings.Values {
			if value.Key == CodexSandboxKey {
				source = value.Source
			}
		}
		if settings.CodexSandbox != test.want || source != test.source {
			t.Fatalf("local %q: sandbox = %q from %q, want %q from %q", test.local, settings.CodexSandbox, source, test.want, test.source)
		}
	}
	conf := filepath.Join(t.TempDir(), "settings.conf")
	if err := os.WriteFile(conf, []byte(CodexSandboxKey+"=other\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveSettings(conf, none)
	if err == nil || !strings.HasPrefix(ErrorDetail(err), "LAUNCH_SETTING_INVALID key="+CodexSandboxKey) || !strings.Contains(err.Error(), "workspace-write or danger-full-access") {
		t.Fatalf("an unknown sandbox = %v", err)
	}
}
