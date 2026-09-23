package partner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The skill a Partner is given is the skill the kit keeps.
//
// The copy beside this package is what a binary built anywhere carries, and
// the kit's own file is what a maintainer edits. They are the same words or
// this fails with both paths named, so the drift is caught by the test run
// that already exists rather than by a human noticing that the Partner is
// working from last month's instructions.
func TestTheEmbeddedSkillIsTheKitsOwnFile(t *testing.T) {
	t.Parallel()
	canonical := filepath.Join("..", "..", "..", filepath.FromSlash(SkillPath))
	written, err := os.ReadFile(canonical)
	testutil.Require(t, "the kit's own "+SkillPath, err, nil)
	testutil.Expect(t, "is what this package embeds", Skill(), string(written))
	testutil.Expect(t, "and it is not empty", len(strings.TrimSpace(Skill())) > 500, true)
}

// The prompt carries the instructions, not the front matter: the name and the
// description are the kit's routing, read by whatever loads a skill.
func TestThePromptCarriesTheSkillsBodyAndNamesItsFile(t *testing.T) {
	t.Parallel()
	block := skillBlock()
	testutil.Expect(t, "it names where it came from",
		strings.Contains(block, "from this kit's own "+SkillPath), true)
	testutil.Expect(t, "it carries the instructions",
		strings.Contains(block, "## Where to Look"), true)
	testutil.Expect(t, "and leaves the routing behind",
		strings.Contains(block, "description: Answer a human's questions"), false)
}

// The three territories the skill keeps apart, and the tool that answers each.
// A skill that stopped naming one would be a Partner guessing at that one.
func TestTheSkillRoutesTheThreeTerritories(t *testing.T) {
	t.Parallel()
	for _, named := range []string{
		"interface()", "kit(topic)", "document(id)", "records(kind)", "questions()", "search(text)",
		"The interface.", "The project's memory.", "The metasystem itself.",
	} {
		testutil.Expect(t, "the skill names "+named, strings.Contains(Skill(), named), true)
	}
}
