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
