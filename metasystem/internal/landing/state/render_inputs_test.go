package state

import (
	"encoding/json"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPersistedStateHasJSONNamesAndNoRenderInputs(t *testing.T) {
	t.Parallel()
	var visit func(reflect.Type)
	visit = func(typ reflect.Type) {
		if typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
			visit(typ.Elem())
			return
		}
		if typ.Kind() != reflect.Struct || typ == reflect.TypeFor[time.Time]() {
			return
		}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.Tag.Get("json") == "" {
				t.Errorf("%s.%s has no JSON name", typ.Name(), field.Name)
			}
			visit(field.Type)
		}
	}
	visit(reflect.TypeFor[Record]())
	for typ, names := range map[reflect.Type][]string{
		reflect.TypeFor[Record]():     {"Waiting", "UnreadHandIn"},
		reflect.TypeFor[Proof]():      {"Minutes", "UnitsDone", "UnitsTotal"},
		reflect.TypeFor[Resolution](): {"ReadMinutes", "CheckMinutes"},
		reflect.TypeFor[Agent]():      {"PID", "Unready", "Unread"},
		reflect.TypeFor[Batch]():      {"MainCommits", "WaitingReasons", "RestartReasons"},
		reflect.TypeFor[HandIn]():     {"WaitNext", "WaitFix"},
		reflect.TypeFor[Question]():   {"FirstLine", "Policy"},
	} {
		for _, name := range names {
			if _, exists := typ.FieldByName(name); exists {
				t.Errorf("render input %s.%s persisted", typ.Name(), name)
			}
		}
	}
	data, err := json.Marshal(Record{Proof: Proof{Attempt: "proof"}})
	if err != nil || strings.Contains(string(data), "Minutes") {
		t.Fatalf("record: %s %v", data, err)
	}
	before := Record{Proof: Proof{LoadAtStart: &hostload.Sample{Load1m: 7}}}
	after := Record{Proof: Proof{LoadAtStart: &hostload.Sample{Load1m: 8}}}
	changes := strings.Join(Diff(before, after), "\n")
	if !strings.Contains(changes, "proof.load_at_start:") || strings.Contains(changes, "0x") || strings.Contains(changes, "proof.:") {
		t.Fatal(changes)
	}
}

func TestDescribeRedMainRetainsIncidentsDespiteFlakes(t *testing.T) {
	t.Parallel()
	r := Record{Main: Main{Fact: Fact{Word: Red}, Reason: "TestBroken", Flakes: "TestUnstable", OpenIncidents: []string{"incident-x"}}, HandIns: []HandIn{{Goal: "G", Fact: Fact{Word: Waiting}}}}
	d := Describe(r, RenderInputs{})
	for _, want := range []string{"main is red (incident incident-x)", "TestBroken", "or metasystem landing land-anyway G", "lets G land while main's incident incident-x is open", "undo: metasystem landing give-back G"} {
		if !strings.Contains(d.Headline, want) {
			t.Errorf("missing %q: %s", want, d.Headline)
		}
	}
	if strings.Contains(d.Headline, "unstable checks") {
		t.Fatal(d.Headline)
	}
}

func TestDescribeRunningProofNamesItsOwnSubject(t *testing.T) {
	t.Parallel()
	total, minutes := 10, 5.0
	for _, trunk := range []bool{true, false} {
		t.Run(map[bool]string{true: "main", false: "batch"}[trunk], func(t *testing.T) {
			t.Parallel()
			r := Record{Proof: Proof{Fact: Fact{Word: Running}, Trunk: trunk, Members: []Member{{Goal: "lane-rehearsal-before-landing"}, {Goal: "shared-state-in-the-ledger"}}}}
			d := Describe(r, RenderInputs{UnitsDone: 3, UnitsTotal: &total, Minutes: &minutes})
			want := "proving main (3 of 10 units, 5.0 min)"
			if !trunk {
				want = "proving the batch of 2 — lane-rehearsal-before-landing, shared-state-in-the-ledger (3 of 10 units, 5.0 min)"
			}
			if d.Headline != want+"; nothing to do" {
				t.Fatal(d.Headline)
			}
		})
	}
}

func TestDescribeHandInWaitsBelongToTheirRecordedOrdinal(t *testing.T) {
	t.Parallel()
	r := Record{HandIns: []HandIn{{Goal: "G", SHA: "old", Ordinal: 1, Fact: Fact{Word: Returned}}, {Goal: "G", SHA: "new", Ordinal: 2, Fact: Fact{Word: Waiting}}}}
	inputs := RenderInputs{HandIns: map[int]HandInInput{2: {WaitNext: true}}}
	d := Describe(r, inputs)
	if strings.Count(d.Summary, "G waits for the next batch") != 1 {
		t.Fatalf("wait leaked to an earlier hand-in: %s", d.Summary)
	}
}
