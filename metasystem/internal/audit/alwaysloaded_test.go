package audit

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAlwaysLoadedInstructionsFitTheAuditBudget(t *testing.T) {
	root := filepath.Join("..", "..")
	total, perFile, err := auditWordCounts(root, []string{"AGENTS.md", "wow.md"})
	if err != nil {
		t.Fatal(err)
	}
	if total > DefaultMaxAlwaysLoadedWords {
		t.Fatalf("always-loaded instructions exceed the audit budget:\n%s\n%8d total\n%8d budget",
			strings.Join(perFile, "\n"), total, DefaultMaxAlwaysLoadedWords)
	}
}
