package uitools_test

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// The design verb in the Partner's grammar (g1-s66 D1): design review in its
// two public forms, Send to critique and Answer the round. The design is named
// as the terminal names it, positionally, under design; the funding goal is
// the goal every proposal names; the reader budget is required, because the
// engine refuses a review without one and never invents one.

const loopDesign = "plans/designs/user-interface/g1-s66-the-loop-from-the-room.md"

func TestTheDesignReviewIsProposedInItsTwoForms(t *testing.T) {
	t.Parallel()
	send := prepared(t, uitools.Args{"verb": "design review", "goal": "partner-runs-the-design-loop",
		"design": loopDesign, "tool-calls": 30, "explanation": "the sitting is recorded"})
	for _, want := range []string{"Proposal: design-review\n", "Goal: partner-runs-the-design-loop\n", "Design: " + loopDesign + "\n", "Tool calls: 30\n"} {
		if !strings.Contains(send, want) {
			t.Fatalf("send to critique lacks %q:\n%s", want, send)
		}
	}
	answer := prepared(t, uitools.Args{"verb": "design review", "goal": "partner-runs-the-design-loop", "design": loopDesign,
		"dispositions": "metasystem/artifacts/agents/rev1/rounds/1/decisions.md", "after": 1, "tool-calls": 30, "explanation": "every card has its row"})
	for _, want := range []string{"Dispositions: metasystem/artifacts/agents/rev1/rounds/1/decisions.md\n", "After: 1\n"} {
		if !strings.Contains(answer, want) {
			t.Fatalf("answer the round lacks %q:\n%s", want, answer)
		}
	}
}

func TestADesignReviewProposalIsRefusedInWords(t *testing.T) {
	t.Parallel()
	for _, refused := range []struct {
		named string
		args  uitools.Args
		words string
	}{
		{"no budget", uitools.Args{"verb": "design review", "goal": "g", "design": loopDesign, "explanation": "x"},
			"design review needs tool-calls: the engine refuses a review without a reader budget and never invents one"},
		{"no design", uitools.Args{"verb": "design review", "goal": "g", "tool-calls": 30, "explanation": "x"},
			"design review needs design"},
		{"not a design page", uitools.Args{"verb": "design review", "goal": "g", "design": "plans/x.txt", "tool-calls": 30, "explanation": "x"},
			"design is the design page's checkout-relative path"},
		{"decisions without the round", uitools.Args{"verb": "design review", "goal": "g", "design": loopDesign, "tool-calls": 30,
			"dispositions": "metasystem/artifacts/agents/rev1/rounds/1/decisions.md", "explanation": "x"}, "dispositions and after answer one round together"},
		{"another round's file", uitools.Args{"verb": "design review", "goal": "g", "design": loopDesign, "tool-calls": 30,
			"dispositions": "metasystem/artifacts/agents/rev1/rounds/2/decisions.md", "after": 1, "explanation": "x"},
			"dispositions is the round's own decisions file, rounds/1/decisions.md beside that round's return"},
		{"a zero budget", uitools.Args{"verb": "design review", "goal": "g", "design": loopDesign, "tool-calls": 0, "explanation": "x"},
			"a reader budget is a number of tool calls, 1 or more"},
		{"a retry", uitools.Args{"verb": "design review", "goal": "g", "design": loopDesign, "tool-calls": 30, "retry": 1, "explanation": "x"},
			"retry is not a flag a proposal carries"},
	} {
		if words := refusedPropose(t, refused.named, refused.args); !strings.Contains(words, refused.words) {
			t.Fatalf("%s: refused as %q, want %q", refused.named, words, refused.words)
		}
	}
}
