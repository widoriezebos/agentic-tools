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

// The review rules are in the Partner's instructions (g1-s65 D3), in the copy
// the binary ships as well as the kit's own: the five parts, every claim
// anchored, what was not tried, findings offered, the desk, and never whether
// to accept.
func TestTheInstructionsCarryTheReviewRules(t *testing.T) {
	t.Parallel()
	for _, rule := range []string{
		"Asked, Built, Examined, Proven, Behaves",
		"anchor every claim",
		"Name what the examination did not try, what the tests assume and what is not recorded",
		"as a `finding`",
		"with `present`",
		"`changes`, which reads the reviewed tree",
		"Draw when words would be longer",
		"recommend one decision per finding",
		"never give the verdict",
	} {
		testutil.Expect(t, "the shipped skill carries: "+rule, strings.Contains(skillMarkdown, rule), true)
	}
	// The reviewer recommends now (review-findings-read-as-decisions, open
	// question 1); a shaping sitting still weighs nothing.
	testutil.Expect(t, "and no longer forbids it", strings.Contains(skillMarkdown, "Never say whether to accept"), false)
	testutil.Expect(t, "a shaping sitting still never recommends",
		strings.Contains(skillMarkdown, "In a sitting that shapes a record, never recommend"), true)
}

// The trouble rule is in the Partner's instructions (g1-s68 D3), in the copy
// the binary ships as well as the kit's own: three parts ending in the
// recovery, read before saying why, a card only for the ten goal acts, a link
// for a press elsewhere in the interface, the exact verb for a human's, and
// honesty where the cause is not in the records.
func TestTheInstructionsCarryTheTroubleRule(t *testing.T) {
	t.Parallel()
	for _, rule := range []string{
		"**What happened**, **Why** and **How to recover**",
		"`refusal(code)`",
		"Read before you say why",
		"propose it as a card and say what Apply will do",
		"give the place as a link, never a card",
		"say the exact verb and that it runs from an enrolled terminal",
		"say what to watch and where",
		"say that it is not in the records and what would establish it",
		"Never a cause you did not read, and never a card for something Apply cannot do",
	} {
		testutil.Expect(t, "the shipped skill carries: "+rule, strings.Contains(skillMarkdown, rule), true)
	}
}
