package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	runtimereg "github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// Code critique finding 8: a NEWLY declared instruction filename
// reaches the audit's outside-reference scan roots and instruction
// inventory the moment it is declared — proven by override, not by
// init-time freezing.
func TestDeclaredInstructionFileReachesAudit(t *testing.T) {
	restore := runtimereg.OverrideForTest(append(runtimereg.All(), runtimereg.Declaration{
		Name: "newrt", HasAdapter: true, TailoringPriority: 99,
		InstructionFile: "NEWRT.md",
	}))
	defer restore()

	found := false
	for _, root := range auditScanRoots() {
		if root == "NEWRT.md" {
			found = true
		}
	}
	if !found {
		t.Fatal("a declared instruction file did not join the scan roots")
	}

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "NEWRT.md"), []byte("# instructions\n"), 0o644)
	instructions, err := auditFindNamed(dir, []string{"."},
		instructionInventoryNames(), true)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(instructions, "\n")
	if !strings.Contains(joined, "NEWRT.md") {
		t.Fatalf("a declared instruction file did not join the inventory: %s", joined)
	}
}

// The Class-14 positive audit refuses a tree whose named documents
// drop their registry pointers.
func TestRegistryPointerAuditRefuses(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "docs", "design"), 0o755)
	os.WriteFile(filepath.Join(root, "docs", "project-adaptation.md"),
		[]byte("`goal open` only, no pointer\n"), 0o644)
	violations := auditGoalSystem(root)
	found := 0
	for _, violation := range violations {
		if strings.Contains(violation, "must point at the runtime registry") {
			found++
		}
	}
	// README is template-scoped; a bare tree owes the three shipped docs.
	if found < 3 {
		t.Fatalf("pointer refusals missing: %v", violations)
	}
	// With the template marker present, README joins the owed set.
	os.MkdirAll(filepath.Join(root, "development"), 0o755)
	os.WriteFile(filepath.Join(root, "development", "metasystem-design.md"), []byte("design\n"), 0o644)
	templateViolations := auditGoalSystem(root)
	readmeOwed := false
	for _, violation := range templateViolations {
		if strings.Contains(violation, "README.md must point") {
			readmeOwed = true
		}
	}
	if !readmeOwed {
		t.Fatalf("the template tree did not owe the README pointer: %v", templateViolations)
	}
}
