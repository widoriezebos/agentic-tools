package launch

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReviewReadsTheBriefTheBuilderReceived pins finding 575: when the
// checked brief file changes after launch (a declaration repair prepends the
// engine preface again), the review's brief is the retained step copy whose
// digest is the one recorded at launch, not the rewritten file.
func TestReviewReadsTheBriefTheBuilderReceived(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	retained := filepath.Join(directory, "0-checked-build.md")
	if err := os.WriteFile(retained, []byte("preface\n\nthe brief\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := digestHex([]byte("preface\n\nthe brief\n"))
	round := UnitRound{BuildBriefSHA256: want, Steps: []UnitStep{{Name: "build", Retained: &StartSpec{Brief: retained}}}}
	if got := launchFrozenBrief(round, want); got != retained {
		t.Fatalf("retained copy with the recorded digest: got %q, want %q", got, retained)
	}
	for name, row := range map[string]struct {
		round UnitRound
		want  string
	}{
		"digest differs":   {UnitRound{BuildBriefSHA256: "other", Steps: []UnitStep{{Retained: &StartSpec{Brief: retained}}}}, "other"},
		"no retained copy": {UnitRound{BuildBriefSHA256: want, Steps: []UnitStep{{}}}, want},
		"no steps":         {UnitRound{BuildBriefSHA256: want}, want},
		"no digest":        {UnitRound{Steps: []UnitStep{{Retained: &StartSpec{Brief: retained}}}}, ""},
	} {
		if got := launchFrozenBrief(row.round, row.want); got != "" {
			t.Fatalf("%s: got %q, want no retained copy", name, got)
		}
	}
}
