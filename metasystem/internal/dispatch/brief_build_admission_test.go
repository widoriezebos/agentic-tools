package dispatch

import (
	"errors"
	"testing"
)

func TestBriefAuthoritySkipsTheBriefTheBuildAdmitted(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body        string
		reviewed, refused bool
	}{
		{"frozen", "# Supplied accepted implementation brief (frozen at dispatch)\nRead `docs/absent.md`.\n", true, false},
		{"before frozen", "Read `docs/absent.md`.\n# Supplied accepted implementation brief (frozen at dispatch)\nRead `docs/absent.md`.\n", true, true},
		{"corrected", "# Corrected implementation brief (given at review)\nRead `docs/absent.md`.\n", true, true},
		{"no reviewed commit", "# Supplied accepted implementation brief (frozen at dispatch)\nRead `docs/absent.md`.\n", false, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			repo := newBriefAuthorityRepo(t)
			reviewed := ""
			if row.reviewed {
				reviewed = repo.facts.current
			}
			brief := writeBriefAuthorityFile(t, repo.root, "brief.md", "Working Mode: implement\n\n"+row.body)
			_, err := readBriefAdmissionWithFacts(brief, repo.root, repo.root, repo.root, reviewed, false, repo.facts)
			var refusal *BriefAuthorityRefusal
			if row.refused {
				if !errors.As(err, &refusal) || len(refusal.MissingPaths) != 1 || refusal.MissingPaths[0] != "docs/absent.md" {
					t.Fatalf("want missing citation refused, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("frozen build brief refused: %v", err)
			}
		})
	}
}
