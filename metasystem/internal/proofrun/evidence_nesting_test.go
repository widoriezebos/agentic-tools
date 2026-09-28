package proofrun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bundleNames walks a bundle and returns every slash path relative to it.
func bundleNames(t *testing.T, bundle string) []string {
	t.Helper()
	var names []string
	err := filepath.WalkDir(bundle, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(bundle, path)
		if err != nil {
			return err
		}
		names = append(names, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return names
}

// plantFixtureRepo writes an agent fixture repository that carries the
// engine's own artifact stores (a prior capture among them) and a git
// object store, beside the fixture's own diagnostic files.
func plantFixtureRepo(t *testing.T, repo string) {
	t.Helper()
	writeEvidenceFixture(t, filepath.Join(repo, "fixture.log"), "fixture-log\n")
	writeEvidenceFixture(t, filepath.Join(repo, ".git", "HEAD"), "ref\n")
	writeEvidenceFixture(t, filepath.Join(repo, ".git", "objects", "ab", "cdef"), "object\n")
	writeEvidenceFixture(t, filepath.Join(repo, "artifacts", "agents", "suite-failures", "20260928T000000Z-prior", "source-001-tmp", "prior.log"), "prior-capture\n")
	writeEvidenceFixture(t, filepath.Join(repo, "artifacts", "agents", "proof-runs", "attempts", "a.json"), "{}\n")
	writeEvidenceFixture(t, filepath.Join(repo, "artifacts", "agents", "candidate-engines", "v2", "engine"), "engine\n")
	writeEvidenceFixture(t, filepath.Join(repo, "artifacts", "agents", "steward", "engine-pins", "generation-1"), "pin\n")
}

func assertNoEngineStores(t *testing.T, names []string) {
	t.Helper()
	for _, name := range names {
		if strings.Contains(name, "artifacts/agents") && !strings.HasPrefix(name, "artifacts/agents") ||
			strings.Contains(name, ".git/objects") || strings.HasSuffix(name, "prior.log") {
			t.Fatalf("capture nests an engine artifact store or git object store: %s\nbundle=%v", name, names)
		}
	}
}

func hasSuffix(names []string, suffix string) bool {
	for _, name := range names {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// The watchdog's capture (proof-run preserve) copies the suite's tmp paths,
// where the agent fixture repositories live. A fixture repository that holds
// a prior capture must not have that capture copied into the new one, or
// captures grow geometrically on every red.
func TestPreserveEvidenceNeverNestsAPriorCaptureOrEngineStore(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	plantFixtureRepo(t, filepath.Join(tmp, "agent-fixture"))
	plantFixtureRepo(t, filepath.Join(tmp, "skew-repo", "nested", "clone"))
	destination := filepath.Join(t.TempDir(), "evidence")
	result, err := PreserveEvidence(destination, []string{tmp}, 1<<20)
	if err != nil || len(result.Errors) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	names := bundleNames(t, destination)
	assertNoEngineStores(t, names)
	for _, want := range []string{"agent-fixture/fixture.log", "agent-fixture/.git/HEAD", "clone/fixture.log"} {
		if !hasSuffix(names, want) {
			t.Fatalf("capture lost the fixture's own evidence %s: %v", want, names)
		}
	}
	note, err := os.ReadFile(filepath.Join(destination, "copy-note.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"agent-fixture/artifacts/agents (metasystem artifact store skipped)", "agent-fixture/.git/objects (git object store skipped)"} {
		if !strings.Contains(string(note), want) {
			t.Fatalf("copy note lacks %q:\n%s", want, note)
		}
	}
}

// A source that IS a store path (a log under artifacts/agents) is still
// copied: the exclusion is for stores nested inside a copied tree.
func TestPreserveEvidenceCopiesASourceThatIsItselfUnderTheStore(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	logs := filepath.Join(root, "artifacts", "agents", "test-logs")
	writeEvidenceFixture(t, filepath.Join(logs, "suite.log"), "log\n")
	destination := filepath.Join(t.TempDir(), "evidence")
	result, err := PreserveEvidence(destination, []string{logs}, 1<<20)
	if err != nil || len(result.Errors) != 0 || result.CopiedBytes != 4 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

// The detached capture copies a candidate's suite-failures tree; a prior
// bundle inside it that carried a fixture repository with its own store
// must not bring that store along.
func TestPreserveDetachedSuiteFailuresNeverNestsAPriorCapture(t *testing.T) {
	t.Parallel()
	control, candidate := t.TempDir(), t.TempDir()
	bundle := filepath.Join(candidate, "artifacts", "agents", "suite-failures", "20260928T010000Z-watchdog-1")
	writeEvidenceFixture(t, filepath.Join(bundle, "copy-note.txt"), "copied-bytes=1\n")
	plantFixtureRepo(t, filepath.Join(bundle, "source-001-tmp", "agent-fixture"))
	result, destination, err := PreserveDetachedSuiteFailures(control, candidate, "group", 0, DetachedEvidenceGroupMaxBytes)
	if err != nil || len(result.Errors) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	names := bundleNames(t, destination)
	for _, name := range names {
		if strings.Contains(name, "agent-fixture/artifacts/agents") || strings.HasSuffix(name, "prior.log") {
			t.Fatalf("detached capture nests a prior capture: %s\n%v", name, names)
		}
	}
	if !hasSuffix(names, "agent-fixture/fixture.log") {
		t.Fatalf("detached capture lost the fixture's own evidence: %v", names)
	}
}

// A bundle that reaches its byte cap says so in one clear marker line.
func TestPreserveEvidenceMarksATruncatedBundle(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	writeEvidenceFixture(t, filepath.Join(source, "first"), "1234")
	writeEvidenceFixture(t, filepath.Join(source, "second"), "5678")
	truncated := filepath.Join(t.TempDir(), "evidence")
	result, err := PreserveEvidence(truncated, []string{source}, 5)
	if err != nil || !result.Truncated {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	note, err := os.ReadFile(filepath.Join(truncated, "copy-note.txt"))
	if err != nil || !strings.Contains(string(note), "TRUNCATED bundle reached its 5-byte cap") {
		t.Fatalf("note=%q err=%v", note, err)
	}
	whole := filepath.Join(t.TempDir(), "evidence")
	result, err = PreserveEvidence(whole, []string{source}, 100)
	if err != nil || result.Truncated {
		t.Fatalf("untruncated result=%+v err=%v", result, err)
	}
	if note, err := os.ReadFile(filepath.Join(whole, "copy-note.txt")); err != nil || strings.Contains(string(note), "TRUNCATED") {
		t.Fatalf("untruncated note=%q err=%v", note, err)
	}
}
