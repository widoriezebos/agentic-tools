package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The host's Codex sandbox reads from a synthetic local settings file, takes
// its compiled default when unset, and is no proof input: it is a property of
// the host, not of what a proof proves.
func TestCodexSandboxReadsTheLocalSettings(t *testing.T) {
	t.Parallel()
	none := func(string) (string, bool) { return "", false }
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	if err := os.WriteFile(conf, []byte("metasystem.runtimes=claude,codex\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if mode, err := CodexSandbox(conf, none); err != nil || mode != CodexSandboxWorkspaceWrite {
		t.Fatalf("unset sandbox = %q, %v; want workspace-write", mode, err)
	}
	if err := os.WriteFile(conf+".local", []byte(CodexSandboxKey+"=danger-full-access\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if mode, err := CodexSandbox(conf, none); err != nil || mode != CodexSandboxFullAccess {
		t.Fatalf("local sandbox = %q, %v; want danger-full-access", mode, err)
	}
	if err := os.WriteFile(conf+".local", []byte(CodexSandboxKey+"=other\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if mode, err := CodexSandbox(conf, none); err == nil {
		t.Fatalf("an unknown sandbox resolved to %q", mode)
	}
	setting, ok := compiledSetting(CodexSandboxKey)
	if !ok || setting.Default != CodexSandboxWorkspaceWrite || setting.ProofInput {
		t.Fatalf("compiled setting = %+v, %v", setting, ok)
	}
}

// settings set's check refuses a sandbox value outside the two modes, naming
// both, and leaves every other key to settings check.
func TestSettingValueProblemNamesBothSandboxModes(t *testing.T) {
	t.Parallel()
	for _, value := range []string{CodexSandboxWorkspaceWrite, CodexSandboxFullAccess} {
		if err := SettingValueProblem(CodexSandboxKey, value); err != nil {
			t.Fatalf("%s refused: %v", value, err)
		}
	}
	err := SettingValueProblem(CodexSandboxKey, "other")
	if err == nil || !strings.Contains(err.Error(), "workspace-write or danger-full-access") || !strings.Contains(err.Error(), `"other"`) {
		t.Fatalf("other = %v, want a refusal naming both modes", err)
	}
	if err := SettingValueProblem("launch.read.model", "anything"); err != nil {
		t.Fatalf("an open key was refused: %v", err)
	}
}
