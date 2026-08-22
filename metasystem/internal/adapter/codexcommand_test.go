package adapter

import (
	"os"
	"path/filepath"
	"testing"
)

// The envelope-to-flag derivation from both sources: a job record's
// requested envelope and a bare envelope file.
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
