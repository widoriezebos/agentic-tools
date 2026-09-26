package mission

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesignRoundRuleIsStatedBySkillAndDocs(t *testing.T) {
	phrases := []string{
		"Design critique has two rounds at tier 3 and does not exist below tier 3.",
		"A design-critic chain's `reviewRoundLimit` is 2 whatever the goal's stored member says.",
		"Round 2 folded with no material finding closes the loop.",
		"Round 2 folded with only mechanical findings on a falling trajectory closes with one review obligation per finding naming its fixture.",
		"Any other round-2 residue refuses with `cap-exhausted-human-raise`, naming `metasystem decide` and a re-scope by `metasystem edit`.",
		"There is no third design round and none can be bought.",
		"`closed at round 2 on N fixture obligations`",
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
