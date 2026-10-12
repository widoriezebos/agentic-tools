package channel

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLaneQuestionTextKeepsMechanismsInRecord(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"lane policy", "lane stop"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			record := `{"required":["metasystem","landing","run","--goals","g"],"measure":{"name":"fingerprint"},"attempt":2}`
			q := Question{ID: "q-fixture", About: "lane", Lineage: "landing-agent", Facts: []string{"metasystem landing resume", kind + ": " + record, "evidence: recorded selection"}, Recommendation: "Run the command above"}
			if command := LaneStopCommand(q); command != "metasystem landing run --goals g" {
				t.Fatalf("record lost executable command: %q", command)
			}
			text := renderQuestion(q, true)
			if !strings.Contains(text, "q-fixture") {
				t.Fatal("question id lost")
			}
			if !strings.HasPrefix(text, "metasystem landing resume\n") || !strings.Contains(text, "metasystem landing resume;") {
				t.Fatalf("missing intent: %s", text)
			}
			for _, word := range []string{"fingerprint", "recorded selection", "run --goals", "\"attempt\""} {
				if strings.Contains(text, word) {
					t.Fatalf("mechanism %q in text: %s", word, text)
				}
			}
			data, err := json.Marshal(q)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "fingerprint") || !strings.Contains(string(data), "recorded selection") {
				t.Fatal("text rendering erased record details")
			}
		})
	}
}

func TestLaneQuestionTextRetainsDecisionFacts(t *testing.T) {
	t.Parallel()
	q := Question{ID: "q-fixture", About: "lane", Lineage: "landing-agent", Facts: []string{"main is red (incident incident-x): TestBroken; metasystem landing land-anyway g --reason TEXT (lets g land while main's incident incident-x is open; undo: metasystem landing give-back g --reason TEXT)", "lane policy: " + `{"required":["metasystem","landing","land-anyway","g","--reason","TEXT"]}`, "evidence: recorded selection"}}
	text := renderQuestion(q, true)
	for _, want := range []string{"q-fixture", "incident-x", "TestBroken", "lets g land", "undo: metasystem landing give-back g --reason TEXT"} {
		if !strings.Contains(text, want) {
			t.Errorf("lost %q: %s", want, text)
		}
	}
}
