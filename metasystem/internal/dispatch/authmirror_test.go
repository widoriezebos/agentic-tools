package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A mission chain's integration authorizations mirror with the chain and
// the close attests them: an unmirrored, tampered, or vanished record
// refuses the close, because a closed chain must testify alone.
func TestMirrorCarriesAuthorizationsAndCloseAttestsThem(t *testing.T) {
	repo, evidence, job := mirrorFixture(t)
	agents := filepath.Join(repo, "artifacts", "agents")

	// Stamp the job as mission work and issue its authorization record.
	record := readJSONFile(t, filepath.Join(agents, "jobs", job+".json"))
	record["mission"] = "m1"
	writeJSONFile(t, filepath.Join(agents, "jobs"), job+".json", record)
	authDir := filepath.Join(agents, "missions", "m1", "authorizations")
	digest := strings.Repeat("a", 64)
	writeJSONFile(t, authDir, digest+".json", map[string]any{
		"schemaVersion": 1, "authorizationDigest": digest, "jobId": job,
		"changedPaths": []any{"src/x"}, "supersedes": []any{},
	})

	result := filepath.Join(t.TempDir(), "result.json")
	stampFromResult := func() {
		t.Helper()
		outcome := readJSONFile(t, result)
		stamped := readJSONFile(t, filepath.Join(agents, "jobs", job+".json"))
		stamped["mirror"] = map[string]any{"path": outcome["path"], "manifest": outcome["manifest"]}
		writeJSONFile(t, filepath.Join(agents, "jobs"), job+".json", stamped)
	}
	if err := Mirror(repo, repo, evidence, job, job, result); err != nil {
		t.Fatalf("Mirror: %v", err)
	}
	stampFromResult()
	mirrored := readJSONFile(t, filepath.Join(agents, "jobs", job+".json"))
	mirror, _ := mirrored["mirror"].(map[string]any)
	manifest := readJSONFile(t, filepath.Join(asString(mirror["path"]), "manifest.json"))
	files, _ := manifest["files"].(map[string]any)
	if _, present := files["authorizations/"+digest+".json"]; !present {
		t.Fatalf("the mirror manifest does not carry the authorization record: %v", files)
	}
	if err := CloseCheck(repo, job); err != nil {
		t.Fatalf("CloseCheck over intact evidence: %v", err)
	}

	// Tampered bytes after mirroring: the manifest digest is stale.
	authPath := filepath.Join(authDir, digest+".json")
	original, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(authPath, []byte(`{"jobId":"`+job+`","tampered":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CloseCheck(repo, job); err == nil || !strings.Contains(err.Error(), "stale integration authorization") {
		t.Fatalf("tampered authorization closed anyway: %v", err)
	}
	if err := os.WriteFile(authPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	// Vanished record: the manifest knows evidence the disk lost.
	if err := os.Remove(authPath); err != nil {
		t.Fatal(err)
	}
	if err := CloseCheck(repo, job); err == nil || !strings.Contains(err.Error(), "vanished after mirroring") {
		t.Fatalf("vanished authorization closed anyway: %v", err)
	}
	if err := os.WriteFile(authPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	// A record issued after the last mirror pass: present on disk,
	// absent from the manifest — the close demands a re-mirror.
	late := strings.Repeat("b", 64)
	writeJSONFile(t, authDir, late+".json", map[string]any{
		"schemaVersion": 1, "authorizationDigest": late, "jobId": job,
		"changedPaths": []any{"src/y"}, "supersedes": []any{},
	})
	if err := CloseCheck(repo, job); err == nil || !strings.Contains(err.Error(), "not mirrored") {
		t.Fatalf("late unmirrored authorization closed anyway: %v", err)
	}
	if err := Mirror(repo, repo, evidence, job, job, result); err != nil {
		t.Fatalf("re-mirror: %v", err)
	}
	stampFromResult()
	if err := CloseCheck(repo, job); err != nil {
		t.Fatalf("CloseCheck after re-mirror: %v", err)
	}
}
