package dispatch

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func composeReferencedBrief(t *testing.T) (string, CompositionRecord, []byte) {
	t.Helper()
	root := compositionRepoRoot(t)
	temp := t.TempDir()
	briefPath := filepath.Join(temp, "brief.md")
	brief := bytes.Repeat([]byte("r"), 40*1024)
	if err := os.WriteFile(briefPath, brief, 0o644); err != nil {
		t.Fatal(err)
	}
	promptPath := filepath.Join(temp, "prompt.md")
	compositionPath := filepath.Join(temp, "composition.json")
	stageDir := compositionTemporaryStageDir(t)
	referenceDir := compositionStageDir(t)
	record, err := ComposeRolePacket(ComposeRolePacketParams{
		Root: root, Role: "verifier", Brief: briefPath, JobID: "reference-test", Runtime: "fake",
		Model: "fake-model", ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardMechanical,
		Output: promptPath, CompositionOutput: compositionPath, StageDir: stageDir, ReferenceDir: referenceDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(referenceDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(stageDir, referenceDir); err != nil {
		t.Fatal(err)
	}
	packet, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatal(err)
	}
	return compositionPath, record, packet
}

func TestVerifyReferencesRefusesAChangedFileByName(t *testing.T) {
	root := compositionRepoRoot(t)
	composition, record, _ := composeReferencedBrief(t)
	if mismatches, err := VerifyReferences(root, composition); err != nil || mismatches != nil {
		t.Fatalf("unchanged reference verification = %+v, %v", mismatches, err)
	}
	reference := record.References[0]
	handle, err := os.OpenFile(reference.OpenPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Write([]byte("x")); err != nil {
		handle.Close()
		t.Fatal(err)
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	changed, err := os.ReadFile(reference.OpenPath)
	if err != nil {
		t.Fatal(err)
	}
	mismatches, err := VerifyReferences(root, composition)
	if err != nil || len(mismatches) != 1 {
		t.Fatalf("changed reference verification = %+v, %v", mismatches, err)
	}
	mismatch := mismatches[0]
	wantExpected := fmt.Sprintf("%s:%d", reference.Digest, 40*1024)
	wantFound := fmt.Sprintf("%s:%d", digestBytes(changed), 40*1024+1)
	if mismatch.Path != reference.Path || mismatch.OpenPath != reference.OpenPath || mismatch.Expected != wantExpected || mismatch.Found != wantFound {
		t.Fatalf("changed reference mismatch = %+v", mismatch)
	}
	if !strings.HasPrefix(mismatch.Line(), "REFERENCE_"+"MISMATCH path=") {
		t.Fatalf("mismatch line = %q", mismatch.Line())
	}
	if err := os.Remove(reference.OpenPath); err != nil {
		t.Fatal(err)
	}
	mismatches, err = VerifyReferences(root, composition)
	if err != nil || len(mismatches) != 1 || mismatches[0].Found != "unreadable" {
		t.Fatalf("missing reference verification = %+v, %v", mismatches, err)
	}
}

func TestVerifyReferencesOpensTheControlRootCopyNotTheWorkspaceCopy(t *testing.T) {
	root := compositionRepoRoot(t)
	composition, record, packet := composeReferencedBrief(t)
	reference := record.References[0]
	workspace := t.TempDir()
	workspaceCopy := filepath.Join(workspace, filepath.FromSlash(reference.Path))
	if err := os.MkdirAll(filepath.Dir(workspaceCopy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workspaceCopy, []byte("different workspace bytes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if mismatches, err := VerifyReferences(root, composition); err != nil || mismatches != nil {
		t.Fatalf("workspace copy affected verification: %+v, %v", mismatches, err)
	}
	if !bytes.Contains(packet, []byte(reference.OpenPath)) || bytes.Contains(packet, []byte(workspaceCopy)) {
		t.Fatalf("packet names the wrong copy: open=%s workspace=%s", reference.OpenPath, workspaceCopy)
	}
	if err := os.WriteFile(reference.OpenPath, []byte("changed control-root bytes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mismatches, err := VerifyReferences(root, composition)
	if err != nil || len(mismatches) != 1 || mismatches[0].OpenPath != reference.OpenPath {
		t.Fatalf("control-root mismatch = %+v, %v", mismatches, err)
	}
}
