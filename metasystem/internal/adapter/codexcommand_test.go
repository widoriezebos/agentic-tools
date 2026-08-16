package adapter

import (
	"os"
	"path/filepath"
	"testing"
)

// Moved from claudecommand_test.go (placement audit, item 17).
func TestCodexPermissionSettings(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "job.json")
	os.WriteFile(record, []byte(`{"permissions":{"requested":{"writeRoots":[],"network":"allow"}}}`), 0o644)
	sandbox, network, err := CodexPermissionSettings("", record)
	if err != nil || sandbox != "read-only" || network != "true" {
		t.Fatalf("record derivation = (%s,%s,%v)", sandbox, network, err)
	}
	envelope := filepath.Join(dir, "perm.json")
	os.WriteFile(envelope, []byte(`{"writeRoots":["/ws"],"network":"deny"}`), 0o644)
	sandbox, network, err = CodexPermissionSettings(envelope, "")
	if err != nil || sandbox != "workspace-write" || network != "false" {
		t.Fatalf("envelope derivation = (%s,%s,%v)", sandbox, network, err)
	}
}
