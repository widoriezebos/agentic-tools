package dispatch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMirrorIncludesReadRefusals(t *testing.T) {
	repo, evidence, root := mirrorFixture(t)
	refusalPath := filepath.Join(repo, "artifacts", "agents", root, "reads-refused.jsonl")
	if _, err := appendUnderRegisterLock(t, repo, refusalPath, refusalEvent("mirror-one")); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(t.TempDir(), "mirror-result.json")
	if err := Mirror(repo, repo, evidence, root, root, resultPath); err != nil {
		t.Fatal(err)
	}
	first := readJSONFile(t, resultPath)
	manifestPath := filepath.Join(asString(first["path"]), "manifest.json")
	manifest := readJSONFile(t, manifestPath)
	files := manifest["files"].(map[string]any)
	entry, present := files["reads-refused.jsonl"].(map[string]any)
	if !present {
		t.Fatalf("mirror manifest lacks reads-refused.jsonl: %v", files)
	}
	wantHash, err := sha256File(refusalPath)
	if err != nil || asString(entry["sha256"]) != wantHash {
		t.Fatalf("refusal hash = %v, want %s, err %v", entry, wantHash, err)
	}

	if err := Mirror(repo, repo, evidence, root, root, resultPath); err != nil {
		t.Fatal(err)
	}
	if repeated := readJSONFile(t, resultPath); repeated["unchanged"] != true {
		t.Fatalf("unchanged refusal mirror repeated work: %v", repeated)
	}
	if _, err := appendUnderRegisterLock(t, repo, refusalPath, refusalEvent("mirror-two")); err != nil {
		t.Fatal(err)
	}
	if err := Mirror(repo, repo, evidence, root, root, resultPath); err != nil {
		t.Fatal(err)
	}
	changed := readJSONFile(t, resultPath)
	if changed["unchanged"] != false {
		t.Fatalf("changed refusal file was not mirrored: %v", changed)
	}
	manifest = readJSONFile(t, filepath.Join(asString(changed["path"]), "manifest.json"))
	entry = manifest["files"].(map[string]any)["reads-refused.jsonl"].(map[string]any)
	wantHash, _ = sha256File(refusalPath)
	if asString(entry["sha256"]) != wantHash {
		t.Fatalf("changed refusal hash = %v, want %s", entry, wantHash)
	}

	child := root + "-r2"
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), child+".json", map[string]any{
		"jobId": child, "round": 2, "parentJob": root, "status": "completed", "role": "implementer",
		"capabilitySnapshot": "artifacts/agents/capabilities/snap.json",
	})
	if err := os.MkdirAll(filepath.Join(repo, "artifacts", "agents", root, "rounds", "2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Mirror(repo, repo, evidence, root, child, resultPath); err != nil {
		t.Fatal(err)
	}
	childManifest := readJSONFile(t, filepath.Join(asString(readJSONFile(t, resultPath)["path"]), "manifest.json"))
	if _, present := childManifest["files"].(map[string]any)["reads-refused.jsonl"]; !present {
		t.Fatal("child mirror omitted the chain-level refusal file")
	}

	absentRepo, absentEvidence, absentRoot := mirrorFixture(t)
	if err := Mirror(absentRepo, absentRepo, absentEvidence, absentRoot, absentRoot, resultPath); err != nil {
		t.Fatal(err)
	}
	absentManifest := readJSONFile(t, filepath.Join(asString(readJSONFile(t, resultPath)["path"]), "manifest.json"))
	if _, present := absentManifest["files"].(map[string]any)["reads-refused.jsonl"]; present {
		t.Fatal("absent refusal file appeared in the mirror")
	}
}
