package mission

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesignRoundRuleIsStatedBySkillAndDocs(t *testing.T) {
	phrases := []string{
		"Design critique has five rounds at tiers 2 and 3 (Wido 2026-10-01) and does not exist at tier 1.",
		"A design-critic chain's `reviewRoundLimit` is frozen at 5 at dispatch whatever the goal's stored member says",
		"The cap is a backstop, not the stop criterion",
		"The final round folded with no material finding closes the loop.",
		"The final round folded with only mechanical findings on a falling trajectory (its material count below the round before) closes with one review obligation per finding naming its fixture.",
		"Any other final-round residue refuses with `cap-exhausted-human-raise`, naming `metasystem goal accept-risk` and a re-scope by `metasystem goal edit`.",
		"There is no design round past the cap and none can be bought.",
		"`closed at round N on M fixture obligations`",
	}
	for _, rel := range []string{"skills/design-critique/SKILL.md", "docs/orchestration.md"} {
		body, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		for _, phrase := range phrases {
			if !strings.Contains(string(body), phrase) {
				t.Errorf("%s is missing exact design-round phrase %q", rel, phrase)
			}
		}
	}
}

// TestCritiqueCapIsFiveBackstopIsStatedBySkills: both critique skills state
// the five-round cap for tiers 2 and 3 as a backstop, with materiality under
// the threat model as the stop criterion (Wido 2026-10-01).
func TestCritiqueCapIsFiveBackstopIsStatedBySkills(t *testing.T) {
	t.Parallel()
	phrases := []string{
		"The round count is a backstop, not the stop criterion (Wido 2026-10-01).",
		"judged against the brief's threat model",
		"zero rounds\nfor Tier 1 and five for Tiers 2 and 3",
	}
	for _, rel := range []string{"skills/design-critique/SKILL.md", "skills/code-critique/SKILL.md"} {
		body, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		for _, phrase := range phrases {
			if !strings.Contains(strings.ReplaceAll(text, "\n", " "), strings.ReplaceAll(phrase, "\n", " ")) {
				t.Errorf("%s is missing critique-cap phrase %q", rel, phrase)
			}
		}
		for _, stale := range []string{"One fix round is the norm", "three-round ceiling", "no third design round", "silent fourth round"} {
			if strings.Contains(text, stale) {
				t.Errorf("%s still states the retired round rule %q", rel, stale)
			}
		}
	}
}
