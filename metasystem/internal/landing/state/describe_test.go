package state

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestWordsCoverEveryRecordedSectionAndConstant(t *testing.T) {
	t.Parallel()
	sections := map[string][]string{
		"lane": {Running, Paused, Stopped}, "agent": {Idle, Stepping, StepProcessGone, Ended, ""},
		"batch.execution": {Prepared, Running, Closed}, "batch.admission": {Open, Sealed},
		"hand_in": {Waiting, Returned, Landed, Superseded}, "resolution": {Resolving, Resolved, Building, Running, Reviewing, Abandoned, Done, Closed},
		"proof": {Running, Died, Pending, Failed, Green, Red, Held, Skipped, Cancelled}, "main": {Green, Red, Unproven}, "question": {Current, Gone, Unknown},
		"holds": {"held", "pending"}, "engine": {"incident"},
	}
	seen := map[string]bool{}
	for section, words := range sections {
		for _, word := range words {
			seen[word] = true
			if _, ok := Words[WordKey{section, word}]; !ok {
				t.Errorf("missing (%s, %s)", section, word)
			}
		}
	}
	file, err := parser.ParseFile(token.NewFileSet(), "words.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ast.Inspect(file, func(n ast.Node) bool {
		v, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for _, expr := range v.Values {
			literal, ok := expr.(*ast.BasicLit)
			if !ok {
				t.Fatal("word constant is not a literal")
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			if !seen[value] {
				t.Errorf("word constant %q has no declared section", value)
			}
		}
		return true
	})
}

func TestDescribeLaneGoldenRecords(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 12, 19, 2, 0, 0, time.UTC)
	total, minutes := 234, 52.0
	cases := []struct {
		name                   string
		r                      Record
		inputs                 RenderInputs
		headline, verb, person string
	}{
		{"idle", Record{Agent: Agent{Word: Ended}}, RenderInputs{}, "idle; its landing agent starts when there is work", "none", "nothing to do"},
		{"proving", Record{Proof: Proof{Fact: Fact{Word: Running, Since: Since{At: at}}}}, RenderInputs{UnitsDone: 233, UnitsTotal: &total, Minutes: &minutes}, "proving (233 of 234 units, 52.0 min); proving main (233 of 234 units, 52.0 min); the lane proves main; the agent waits for the keeper; nothing to do", "none", "nothing to do"},
		{"red proof with incidents", Record{Main: Main{Fact: Fact{Word: Red}, Reason: "TestBroken", OpenIncidents: []string{"incident-x"}}}, RenderInputs{}, "main is red (incident incident-x): TestBroken; land a fix on main, then metasystem landing prove --trunk", "landing prove", "land a fix on main, then metasystem landing prove --trunk"},
		{"person's pause", Record{Lane: Lane{Fact: Fact{Word: Paused, Since: Since{At: at}}, By: "Wido", Reason: "inspect the proof"}, Agent: Agent{Word: Ended}}, RenderInputs{}, "paused by Wido (inspect the proof)", "landing resume", "resume when the pause is no longer needed"},
		{"barren hold", Record{Holds: Holds{Barren: 2}}, RenderInputs{}, "held after 2 runs that left the lane unchanged; a person runs metasystem landing resume", "landing run --goals", "choose the waiting goals to run"},
		{"stale merge", Record{Resolution: Resolution{Fact: Fact{Word: Abandoned}, Reason: "main moved"}}, RenderInputs{}, "the resolution was abandoned: main moved; run: metasystem landing resume", "landing resume", "resume on the current main"},
		{"engine incident", Record{Engine: Engine{Incidents: []EngineIncident{{At: at, Verb: "prove", Panic: "broken engine"}}}}, RenderInputs{}, "an engine incident at " + at.Local().Format("15:04") + " in landing prove: broken engine; run: metasystem landing resume", "landing resume", "examine the engine incident before resuming"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			d := Describe(c.r, c.inputs)
			if d.Headline != c.headline || d.Verb != c.verb || d.Person != c.person {
				t.Fatalf("got %+v; want %q, %q, %q", d, c.headline, c.verb, c.person)
			}
		})
	}
}

func TestDescribeEverySinceIsLocalAndObservationIsHonest(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 12, 19, 2, 0, 0, time.UTC)
	since := Since{At: at, Observed: true, Source: "batch.json"}
	minutes := 52.0
	r := Record{Lane: Lane{Fact: Fact{Word: Running, Since: Since{At: at}}}, Agent: Agent{Word: Idle, Since: Since{At: at}}, Batch: Batch{Execution: Fact{Word: Running, Since: since}, Admission: Fact{Word: Open, Since: Since{At: at}}}, Proof: Proof{Fact: Fact{Word: Running, Since: since}}, Resolution: Resolution{Fact: Fact{Word: Building, Since: Since{At: at}}}, Main: Main{Fact: Fact{Word: Green, Since: Since{At: at}}}, HandIns: []HandIn{{Goal: "g", Fact: Fact{Word: Returned, Since: Since{At: at}}}}, Holds: Holds{Barren: 1, Since: Since{At: at}}}
	before := r
	d := Describe(r, RenderInputs{Minutes: &minutes})
	if !strings.Contains(strings.Join(d.Facts, "\n"), "seen since "+at.Local().Format("15:04")+" (the batch record carries no running time)") || strings.Contains(d.Headline, "52") {
		t.Fatalf("observation became a duration: %+v", d)
	}
	for _, fact := range d.Facts {
		if !strings.Contains(fact, at.Local().Format("15:04")) {
			t.Errorf("since was lost: %s", fact)
		}
	}
	if !reflect.DeepEqual(before, r) {
		t.Fatal("Describe changed its input")
	}
}

func TestDescribeQuestionsAndBatchCarryTheirOwnDetails(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 12, 19, 2, 0, 0, time.UTC)
	r := Record{Agent: Agent{Word: Ended}, Questions: []Question{{ID: "q", Premise: Current, Since: Since{At: at}}}, Batch: Batch{SealedAt: at, Admission: Fact{Word: Sealed, Since: Since{At: at}}, Members: []Member{{Goal: "g"}}}, HandIns: []HandIn{{Goal: "h", Fact: Fact{Word: Waiting}}}}
	d := Describe(r, RenderInputs{Questions: map[string]QuestionInput{"q": {FirstLine: "May I restore the checkout? Private details."}}, HandIns: map[int]HandInInput{0: {WaitNext: true}}})
	for _, want := range []string{"question q since " + at.Local().Format("2006-01-02 15:04 MST"), "May I restore the checkout?", "question show channel:q", "batch sealed " + at.Local().Format("15:04") + ": 1 members", "h waits for the next batch"} {
		if !strings.Contains(d.Summary, want) {
			t.Errorf("missing %q: %s", want, d.Summary)
		}
	}
	if strings.Contains(d.Summary, "Private details") {
		t.Fatal(d.Summary)
	}
}
