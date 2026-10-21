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
			q.Plain = &Plain{Decision: "The landing lane needs your word before it goes on", Act: "metasystem landing resume", IfNothing: "the lane does not do this; this question closes when the act is recorded"}
			want := "The landing lane needs your word before it goes on.\nTo go on: metasystem landing resume\nIf you do nothing: the lane does not do this; this question closes when the act is recorded."
			if kind == "lane stop" {
				q.Facts[0] = "the lane stopped: g needs a person's decision; run: metasystem landing run --goals g"
				q.Plain = &Plain{Decision: "the lane stopped: g needs a person's decision", IfNothing: "the stop stays; this question closes when a later lane record closes it"}
				want = "The lane stopped: g needs a person's decision.\nTo go on: metasystem landing run --goals g\nIf you do nothing: the stop stays; this question closes when a later lane record closes it."
			}
			text := renderPlain(q)
			if text != want {
				t.Fatalf("post = %q, want %q", text, want)
			}
			for _, word := range []string{"q-fixture", "fingerprint", "recorded selection", "\"attempt\""} {
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
	q.Plain = &Plain{Decision: "The landing lane needs your word before it goes on", Act: "metasystem landing land-anyway g --reason TEXT (lets g land while main's failing checks remain unresolved; undo: metasystem landing give-back g --reason TEXT)", IfNothing: "the lane does not do this; this question closes when the act is recorded"}
	text := renderPlain(q)
	want := "The landing lane needs your word before it goes on.\nTo go on: metasystem landing land-anyway g --reason TEXT (lets g land while main's failing checks remain unresolved; undo: metasystem landing give-back g --reason TEXT)\nIf you do nothing: the lane does not do this; this question closes when the act is recorded."
	if text != want {
		t.Fatalf("post = %q, want %q", text, want)
	}
	for _, word := range []string{"incident-x", "TestBroken", "q-fixture"} {
		if strings.Contains(text, word) {
			t.Errorf("identity %q in post: %s", word, text)
		}
	}
	if !strings.Contains(q.Facts[0], "incident-x") || !strings.Contains(q.Facts[0], "TestBroken") {
		t.Fatalf("record lost decision facts: %q", q.Facts)
	}
}
