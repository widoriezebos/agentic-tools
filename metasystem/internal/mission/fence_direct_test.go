package mission

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Direct tests for the fence's small cold paths (Phase 6).

func TestRefuseValidatesItsReason(t *testing.T) {
	if _, err := Refuse(t.TempDir(), "m1", "not-a-fence"); err == nil ||
		!strings.Contains(err.Error(), "unknown mission fence refusal reason") {
		t.Fatalf("bad reason accepted: %v", err)
	}
	// A lawful reason writes the batched ask and returns its path.
	repo := t.TempDir()
	askPath, err := Refuse(repo, "m1", "cycles")
	if err != nil {
		t.Fatalf("lawful refusal failed: %v", err)
	}
	if askPath == "" {
		t.Fatal("no ask path returned")
	}
	data, readErr := os.ReadFile(askPath)
	if readErr != nil {
		t.Fatalf("ask not written: %v", readErr)
	}
	if !strings.Contains(string(data), "cycles") {
		t.Fatalf("ask does not carry the reason: %s", data)
	}
}

func TestJobStatusReadsAndDegrades(t *testing.T) {
	repo := t.TempDir()
	jobs := filepath.Join(repo, "artifacts", "agents", "jobs")
	os.MkdirAll(jobs, 0o755)
	os.WriteFile(filepath.Join(jobs, "j1.json"), []byte(`{"status":"running"}`), 0o644)
	if got := jobStatus(repo, "j1"); got != "running" {
		t.Fatalf("status: %q", got)
	}
	if got := jobStatus(repo, "absent"); got != "" {
		t.Fatalf("absent record must read empty: %q", got)
	}
	os.WriteFile(filepath.Join(jobs, "j2.json"), []byte(`{broken`), 0o644)
	if got := jobStatus(repo, "j2"); got != "" {
		t.Fatalf("malformed record must read empty: %q", got)
	}
}
