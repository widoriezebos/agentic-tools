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
