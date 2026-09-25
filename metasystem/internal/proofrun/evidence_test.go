package proofrun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreserveEvidenceCapsBytesAndNotesDroppedContent(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "first"), []byte("1234"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "second"), []byte("5678"), 0o600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "evidence")
	result, err := PreserveEvidence(destination, []string{source}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if result.CopiedBytes != 5 || len(result.Dropped) == 0 {
		t.Fatalf("result = %+v", result)
	}
	note, err := os.ReadFile(filepath.Join(destination, "copy-note.txt"))
	if err != nil || !strings.Contains(string(note), "DROPPED") {
		t.Fatalf("note = %q, %v", note, err)
	}
}

func TestPreserveEvidenceHandlesDirectFilesSymlinksAndMissingSources(t *testing.T) {
	root := t.TempDir()
	direct := filepath.Join(root, "direct file")
	if err := os.WriteFile(direct, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink("direct file", link); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "evidence")
	result, err := PreserveEvidence(destination, []string{direct, link, filepath.Join(root, "missing")}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.CopiedBytes != int64(len("content")+len("direct file")) || len(result.Errors) != 1 {
		t.Fatalf("result = %+v", result)
	}
	target, err := os.Readlink(filepath.Join(destination, "source-002-link"))
	if err != nil || target != "direct file" {
		t.Fatalf("copied symlink = %q, %v", target, err)
	}
	if got := safeEvidenceName("space/name"); got != "space_name" {
		t.Fatalf("safe evidence name = %q", got)
	}
}

func TestPreserveEvidenceRejectsMissingBounds(t *testing.T) {
	if _, err := PreserveEvidence("", nil, 1); err == nil {
		t.Fatal("empty destination passed")
	}
	if _, err := PreserveEvidence(t.TempDir(), nil, 0); err == nil {
		t.Fatal("zero byte cap passed")
	}
}

func TestPreserveEvidenceKeepsGenericSymlinksGitAndLargeFiles(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	large := strings.Repeat("x", int(detachedEvidenceFileMaxBytes)+1)
	writeEvidenceFixture(t, filepath.Join(source, "large.bin"), large)
	writeEvidenceFixture(t, filepath.Join(source, ".git", "HEAD"), "ref\n")
	if err := os.Symlink("large.bin", filepath.Join(source, "link")); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "evidence")
	result, err := PreserveEvidence(destination, []string{source}, 4*detachedEvidenceFileMaxBytes)
	if err != nil || len(result.Dropped) != 0 || len(result.Errors) != 0 {
		t.Fatalf("generic preservation result=%+v err=%v", result, err)
	}
	copied := filepath.Join(destination, "source-001-"+safeEvidenceName(filepath.Base(source)))
	if data, err := os.ReadFile(filepath.Join(copied, "large.bin")); err != nil || len(data) != len(large) {
		t.Fatalf("generic large file copy len=%d err=%v", len(data), err)
	}
	if _, err := os.Stat(filepath.Join(copied, ".git", "HEAD")); err != nil {
		t.Fatalf("generic copy skipped .git: %v", err)
	}
	if target, err := os.Readlink(filepath.Join(copied, "link")); err != nil || target != "large.bin" {
		t.Fatalf("generic symlink copy = %q, %v", target, err)
	}
}

func TestPreserveDetachedSuiteFailuresBoundsBundleWithoutError(t *testing.T) {
	t.Parallel()
	control, candidate, outside := t.TempDir(), t.TempDir(), t.TempDir()
	secret := filepath.Join(outside, "secret")
	writeEvidenceFixture(t, secret, "outside-bytes\n")
	failures := filepath.Join(candidate, "artifacts", "agents", "suite-failures", "nested")
	writeEvidenceFixture(t, filepath.Join(failures, "failure.log"), "small-log\n")
	writeEvidenceFixture(t, filepath.Join(failures, "blob.bin"), strings.Repeat("b", int(detachedEvidenceFileMaxBytes)+1))
	writeEvidenceFixture(t, filepath.Join(failures, ".git", "HEAD"), "ref\n")
	if err := os.Symlink(secret, filepath.Join(failures, "outside")); err != nil {
		t.Fatal(err)
	}
	// A suite-failure tree inside the candidate's .git is never discovered.
	writeEvidenceFixture(t, filepath.Join(candidate, ".git", "artifacts", "agents", "suite-failures", "hidden.log"), "hidden\n")

	result, bundle, err := PreserveDetachedSuiteFailures(control, candidate, "group", 0, DetachedEvidenceGroupMaxBytes)
	if err != nil {
		t.Fatalf("intentional omissions were reported as an error: %v (result=%+v)", err, result)
	}
	if len(result.Errors) != 0 || len(result.Dropped) != 3 || result.CopiedBytes != int64(len("small-log\n")) {
		t.Fatalf("bounded result=%+v", result)
	}
	copied := map[string]bool{}
	walkErr := filepath.WalkDir(bundle, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		copied[entry.Name()] = true
		return nil
	})
	if walkErr != nil {
		t.Fatal(walkErr)
	}
	if !copied["failure.log"] || copied["blob.bin"] || copied[".git"] || copied["outside"] || copied["hidden.log"] {
		t.Fatalf("bundle entries=%v", copied)
	}
	note, err := os.ReadFile(filepath.Join(bundle, "copy-note.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"blob.bin (1048577 bytes over the 1048576-byte per-file cap)", ".git (git directory skipped)", "outside (symlink not followed)"} {
		if !strings.Contains(string(note), want) {
			t.Fatalf("copy note lacks %q:\n%s", want, note)
		}
	}
	if data, err := os.ReadFile(secret); err != nil || string(data) != "outside-bytes\n" {
		t.Fatalf("symlink target outside the candidate changed: %q, %v", data, err)
	}

	// The group byte cap truncates without turning the bundle into an error.
	truncated, truncatedBundle, err := PreserveDetachedSuiteFailures(control, candidate, "group", 0, 4)
	if err != nil || truncated.CopiedBytes != 4 || len(truncated.Dropped) != 4 {
		t.Fatalf("truncated result=%+v err=%v", truncated, err)
	}
	if note, err := os.ReadFile(filepath.Join(truncatedBundle, "copy-note.txt")); err != nil || !strings.Contains(string(note), "failure.log (6 bytes beyond size cap)") {
		t.Fatalf("truncation note=%q err=%v", note, err)
	}
}

func TestPreserveDetachedSuiteFailuresReportsRealCollectionError(t *testing.T) {
	t.Parallel()
	control, candidate := t.TempDir(), t.TempDir()
	writeEvidenceFixture(t, filepath.Join(candidate, "artifacts", "agents", "suite-failures", "failure.log"), "log\n")
	// The control evidence directory is a regular file, so the copy cannot
	// create its destination: a real IO failure, not an intentional drop.
	writeEvidenceFixture(t, filepath.Join(control, "artifacts", "agents", "suite-failures"), "not a directory\n")
	if _, _, err := PreserveDetachedSuiteFailures(control, candidate, "group", 0, DetachedEvidenceGroupMaxBytes); err == nil ||
		!strings.Contains(err.Error(), "preserve detached suite-failure evidence") {
		t.Fatalf("collection IO failure err=%v", err)
	}
}

func writeEvidenceFixture(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}
