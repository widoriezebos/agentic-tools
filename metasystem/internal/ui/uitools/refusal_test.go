package uitools_test

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// The refusal reader answers a code with the register's own row (g1-s68 D4):
// the register is the engine's account of why it refused and what carries
// past it, so a Partner asked "what happened" quotes it rather than guessing.

// rowFor is the register's own row for one code, so every assertion below names
// something that can only be in the answer because the register supplied it.
func rowFor(t *testing.T, code string) refusal.Row {
	t.Helper()
	for _, row := range refusal.Rows {
		if row.Code == code {
			return row
		}
	}
	t.Fatalf("the register has no row %s", code)
	return refusal.Row{}
}

func TestTheRefusalReaderAnswersAKnownCodeWithItsRow(t *testing.T) {
	t.Parallel()
	row := rowFor(t, "slice-approval-refused")
	result := uitools.Readers{}.Answer(uitools.OpRefusal, uitools.Args{"code": row.Code})
	testutil.Require(t, "the read did not fail", result.Problem, "")
	text := result.Text()
	testutil.Expect(t, "it names its source", strings.Contains(text, "Source: the refusal register"), true)
	testutil.Expect(t, "it counts one row", strings.Contains(text, "Supplied: 1 of 1"), true)
	testutil.Expect(t, "the owner", strings.Contains(text, "Owner: "+row.Owner), true)
	testutil.Expect(t, "the shape", strings.Contains(text, "Shape: "+string(row.Shape)), true)
	testutil.Expect(t, "what the shape means", strings.Contains(text, "binds an agent's act only"), true)
	testutil.Expect(t, "the human verb past it", strings.Contains(text, "Carried past by: "+row.Override), true)
	testutil.Expect(t, "and how many commands that takes", strings.Contains(text, "Commands: 1"), true)
}

func TestTheRefusalReaderGivesTheStandingOfAQuestion(t *testing.T) {
	t.Parallel()
	row := rowFor(t, "UNIT_ROUND_LIMIT")
	text := uitools.Readers{}.Answer(uitools.OpRefusal, uitools.Args{"code": row.Code}).Text()
	question := rowFor(t, "LAUNCH_SETTING_INVALID")
	asked := uitools.Readers{}.Answer(uitools.OpRefusal, uitools.Args{"code": question.Code}).Text()
	testutil.Expect(t, "a question's H1 standing is given",
		strings.Contains(asked, "H1 standing: input"), true)
	testutil.Expect(t, "and a guide's forward command",
		strings.Contains(text, "metasystem goal budget"), true)
}

func TestTheRefusalReaderRefusesAnUnknownCodeInWords(t *testing.T) {
	t.Parallel()
	for _, code := range []string{"NO_SUCH_CODE", "one-line", "partner", "FETCH_HEAD"} {
		result := uitools.Readers{}.Answer(uitools.OpRefusal, uitools.Args{"code": code})
		testutil.Expect(t, "an unknown code is a failed read "+code, result.Failed(), true)
		testutil.Expect(t, "said in words "+code, result.Problem, "the register has no row for "+code)
	}
	empty := uitools.Readers{}.Answer(uitools.OpRefusal, uitools.Args{})
	testutil.Expect(t, "no code is refused too", empty.Problem, "this tool needs the code a refusal named")
}

// The reader joins the catalogue of reads, not the proposal actions: it is
// named, published and admitted, and it is a read.
func TestTheRefusalReaderIsInTheCatalogue(t *testing.T) {
	t.Parallel()
	testutil.Expect(t, "the permission rule admits it", uitools.Names(uitools.OpRefusal), true)
	published := map[string]uitools.Tool{}
	for _, tool := range uitools.Catalogue() {
		published[tool.Name] = tool
	}
	tool, offered := published[uitools.OpRefusal]
	testutil.Require(t, "the catalogue publishes it", offered, true)
	testutil.Expect(t, "with a description a runtime can choose it by",
		strings.Contains(tool.Description, "refusal register"), true)
	testutil.Expect(t, "it takes the code",
		tool.InputSchema["properties"].(map[string]any)["code"] != nil, true)
	reads := 0
	for _, operation := range uitools.Operations {
		if operation != uitools.OpSuggest && operation != uitools.OpDeposit &&
			operation != uitools.OpPropose && operation != uitools.OpPresent {
			reads++
		}
	}
	testutil.Expect(t, "the reads grow by one, to thirteen", reads, 13)
}
