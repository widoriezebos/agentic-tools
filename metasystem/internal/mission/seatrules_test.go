package mission

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeatRulesPresentInOrchestrationDoc(t *testing.T) {
	// These rules come from plans/seats-spend-tokens-in-bounded-sessions-design.md, section P6.
	rules := []struct {
		name string
		text string
	}{
		{"S1 over the trigger", "every remaining multi-call step runs as a fresh bounded delegate"},
		{"S2 turns bounded", "At most 12 main-thread calls per turn; at most one background Bash per unit at a time"},
		{"S3 delegates fresh and bounded", "every delegate prompt starts with `Kind: design`, `build-read`, `critique`, or `other`"},
		{"S4 messages", "A `SendMessage` is at most 400 characters, covers one subject"},
		{"S5 reads by path", "The main session never reads a tool result over 20,000 characters"},
		{"S6 first act", "Run `metasystem context resume` first when a handoff waits; otherwise read the `context-budget` line and run `goal next`"},
	}

	doc, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "orchestration.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range rules {
		if !strings.Contains(string(doc), rule.text) {
			t.Errorf("%s is absent from docs/orchestration.md", rule.name)
		}
	}
}
