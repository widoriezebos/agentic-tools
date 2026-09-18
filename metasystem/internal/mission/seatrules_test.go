package mission

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeatRulesPresentInOrchestrationDoc(t *testing.T) {
	// S1-S6 come from plans/seats-spend-tokens-in-bounded-sessions-design.md section P6; S7, S8, and the efficiency line come from ruling R-115-m1e.
	rules := []struct {
		name, today, check string
	}{
		{"S1 over the trigger", "every remaining multi-call step runs as a fresh bounded delegate", "After the first sample at or over the trigger, no turn has more than three main-thread calls; a tool result contains `handoff recorded:`; no turn ends with a question to Wido; no allowed Stop occurs while the seat's claim has a next step."},
		{"S2 turns bounded", "At most 12 main-thread calls per turn; at most one background Bash per unit at a time", "Calls between two Stop verdicts are at most 12; Monitor uses zero."},
		{"S3 delegates fresh and bounded", "every delegate prompt starts with `Kind: design`, `build-read`, `critique`, or `other`", "`resumedAgentId` and `--follow-up` results are zero outside critique chains."},
		{"S4 messages", "A `SendMessage` is at most 400 characters, covers one subject", "Check length and count per unit."},
		{"S5 reads by path", "The main session never reads a tool result over 20,000 characters", "Results over 20,000 characters are zero."},
		{"S6 first act", "Run `metasystem context resume` first when a handoff waits; otherwise read the `context-budget` line and run `goal next`", "Check the first tool calls."},
		{"S7 handoff keeps lessons", "A handoff is recorded only after the seat's configured memory note holds this session's lessons, its reasoning in flight, and every declared delegate's output path. Claude defaults to its project memory directory; each other runtime configures `context.handoff.note-directory.<runtime>`.", "The note is a regular file in that directory, its modification time follows the session start and precedes the handoff record, and its digest changes when the write shares the session-start second with a previous handoff."},
		{"S8 relayed provenance", "Every relayed number or rule names who said it and why.", "Each peer message carrying a number or rule names its source."},
	}

	doc, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "orchestration.md"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(doc), "\n")
	for _, rule := range rules {
		var row string
		for _, line := range lines {
			if strings.Contains(line, rule.today) {
				row = line
				break
			}
		}
		if row == "" {
			t.Errorf("%s today text is absent from docs/orchestration.md", rule.name)
			continue
		}
		if !strings.Contains(row, rule.check) {
			t.Errorf("%s check text is absent from its row in docs/orchestration.md", rule.name)
		}
	}
	if !strings.Contains(string(doc), "Efficiency never regresses functionality: no unit lowers a proof floor, removes a witness or a gate, or narrows a DONE to save tokens, and each unit's read checks it; S7, S8 and this line come from ruling R-115-m1e") {
		t.Errorf("efficiency lead line is absent from docs/orchestration.md")
	}
}
