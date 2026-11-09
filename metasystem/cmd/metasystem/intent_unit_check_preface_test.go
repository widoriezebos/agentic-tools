package main

import (
	"strings"
	"testing"
)

// TestEnginePrefaceIsWrittenOnce pins finding 575: a repair re-reads the
// checked brief, which already carries the engine preface; writing it again
// must not double the preface, or the brief's launch digest no longer holds.
// Only the complete preface is stripped: a brief that opens with the same
// words keeps every byte (Astra, 02:32).
func TestEnginePrefaceIsWrittenOnce(t *testing.T) {
	t.Parallel()
	preface := enginePreface("/tmp/round-1", []string{"metasystem/plans/goals.md"})
	brief := "# The unit\n\nBuild it.\n"
	once := preface + string(withoutEnginePreface([]byte(brief), preface))
	twice := preface + string(withoutEnginePreface([]byte(once), preface))
	thrice := preface + string(withoutEnginePreface([]byte(preface+once), preface))
	if once != twice || once != thrice {
		t.Fatalf("the preface doubled:\n%q\n%q\n%q", once, twice, thrice)
	}
	if strings.Count(twice, enginePrefaceStart) != 1 || !strings.HasSuffix(twice, brief) {
		t.Fatalf("one preface then the brief, got %q", twice)
	}
	for name, lookalike := range map[string]string{
		"same opening, blank line":    enginePrefaceStart + " preserve the public API.\n\nBody.\n",
		"same opening, no blank line": enginePrefaceStart + " preserve the public API.\nBody.\n",
		"plain brief":                 brief,
		"other directory's preface":   enginePreface("/tmp/round-2", []string{"metasystem/plans/goals.md"}) + brief,
	} {
		if got := string(withoutEnginePreface([]byte(lookalike), preface)); got != lookalike {
			t.Fatalf("%s: a brief that is not the exact preface must keep every byte, got %q", name, got)
		}
	}
}
