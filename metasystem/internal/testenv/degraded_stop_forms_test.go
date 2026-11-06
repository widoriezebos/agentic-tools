package testenv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const degradedStatusText = "Status un" + "available"

// TestDegradedFormsHaveOneSource keeps every degraded payload literal in the
// renderer: Go sources, tests and scripts name forms through it; only the
// golden contract, and the shipped launcher
// fallback carry the text.
func TestDegradedFormsHaveOneSource(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	allowed := map[string]bool{
		"internal/hooks/degraded_forms.go":                     true,
		"internal/hooks/testdata/stop-degraded-forms.golden":   true,
		"internal/runtimes/enforcement/claude-code-hooks.json": true,
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if entry.IsDir() {
			switch relative {
			case "artifacts", "records", "plans", "memory", "node_modules", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(relative, ".go") && !strings.HasSuffix(relative, ".sh") && !strings.HasSuffix(relative, ".json") && !strings.HasSuffix(relative, ".golden") {
			return nil
		}
		contents := degradedRead(t, path)
		if !strings.Contains(contents, degradedStatusText) {
			return nil
		}
		if !allowed[relative] {
			return fmt.Errorf("%s carries a degraded Stop payload literal outside its one source", relative)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func degradedRead(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
