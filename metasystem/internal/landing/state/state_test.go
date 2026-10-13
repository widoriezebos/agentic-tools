package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
)

func events(t *testing.T, dir string) []Event {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var result []Event
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		result = append(result, event)
	}
	return result
}

func TestTransitionChecksActualWordAndRecordsDefect(t *testing.T) {
	t.Parallel()
	dir, now := t.TempDir(), time.Date(2026, 10, 12, 20, 0, 0, 0, time.UTC)
	initial := Record{Lane: Lane{Fact: Fact{Word: "running"}}, Version: 1}
	if _, err := replaceFixture(dir, now, initial, "fixture"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "events.jsonl")); err != nil {
		t.Fatal(err)
	}
	event, err := Transition(dir, now, "lane", "running", "paused", "person", "maintenance", func(r *Record) { r.Lane.Word = "paused"; r.Lane.Since = Since{At: now} })
	if err != nil {
		t.Fatal(err)
	}
	got, exists, err := Read(dir)
	if err != nil || !exists || got.Lane.Word != "paused" || got.Lane.Since.At != now {
		t.Fatalf("record: %+v %v %v", got, exists, err)
	}
	if list := events(t, dir); len(list) != 1 || list[0] != event {
		t.Fatalf("events: %+v", list)
	}
	_, err = Transition(dir, now, "lane", "running", "stopped", "agent", "bad precondition", func(*Record) { t.Fatal("refused transition applied") })
	var refused *Refused
	if !errors.As(err, &refused) || refused.Actual != "paused" {
		t.Fatalf("refusal: %v", err)
	}
	unchanged, _, err := Read(dir)
	if err != nil || !reflect.DeepEqual(got, unchanged) {
		t.Fatalf("refusal changed record: %+v %v", unchanged, err)
	}
	if list := events(t, dir); len(list) != 2 || list[1].Kind != "defect" || list[1].From != "paused" {
		t.Fatalf("events: %+v", list)
	}
}

func TestReplaceNamesPayloadAndSinceDifferences(t *testing.T) {
	t.Parallel()
	dir, now := t.TempDir(), time.Now().UTC()
	before := Record{Batch: Batch{Members: []Member{{Goal: "A", SHA: "a"}}}, Version: 1}
	if _, err := replaceFixture(dir, now, before, "fixture"); err != nil {
		t.Fatal(err)
	}
	after := before
	after.Batch.Members = append(append([]Member(nil), before.Batch.Members...), Member{Goal: "B", SHA: "b"})
	after.Lane.Reason = "maintenance"
	after.Lane.Since = Since{At: now}
	changed, err := replaceFixture(dir, now, after, "keeper")
	want := []string{"lane.since.at: " + fmtTime(time.Time{}) + " → " + fmtTime(now), "lane.reason:  → maintenance", "batch.members: [{A a}] → [{A a} {B b}]"}
	if err != nil || !reflect.DeepEqual(changed, want) {
		t.Fatalf("changed: %q; want %q (%v)", changed, want, err)
	}
	list := events(t, dir)
	if len(list) != 3 || list[1].Kind != "drift" || list[1].Evidence != strings.Join(changed, "\n") || !list[2].Rebuild || list[2].Evidence != strings.Join(changed, "\n") {
		t.Fatalf("events: %+v", list)
	}
	if changed, err = replaceFixture(dir, now.Add(time.Minute), after, "keeper"); err != nil || len(changed) != 0 || len(events(t, dir)) != 3 {
		t.Fatalf("unchanged replace: %q %v", changed, err)
	}
}

func fmtTime(at time.Time) string { return at.String() }

func TestRebuildDriftExplainsOnlyTransitionFieldsSinceLastRebuild(t *testing.T) {
	t.Parallel()
	dir, now := t.TempDir(), time.Date(2026, 10, 12, 20, 0, 0, 0, time.UTC)
	initial := Record{Lane: Lane{Fact: Fact{Word: "running"}}, Batch: Batch{Members: []Member{{Goal: "A", SHA: "a"}}}}
	if _, err := replaceFixture(dir, now, initial, "fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err := Transition(dir, now, "lane", "running", "paused", "person", "maintenance", func(r *Record) { r.Lane.Word = "paused"; r.Lane.Since = Since{At: now}; r.Lane.By = "person" }); err != nil {
		t.Fatal(err)
	}
	next, _, _ := Read(dir)
	next.Lane.Since, next.Lane.By = Since{At: now}, "person"
	next.Batch.Members = append(next.Batch.Members, Member{Goal: "B", SHA: "b"})
	if _, err := replaceFixture(dir, now, next, "keeper"); err != nil {
		t.Fatal(err)
	}
	list := events(t, dir)
	if len(list) != 4 || list[2].Kind != "drift" || list[2].Evidence != "batch.members: [{A a}] → [{A a} {B b}]" || !list[3].Rebuild {
		t.Fatalf("transition masked unrelated drift: %+v", list)
	}
	next.Lane.Reason = "handwritten"
	if _, err := replaceFixture(dir, now.Add(time.Minute), next, "keeper"); err != nil {
		t.Fatal(err)
	}
	list = events(t, dir)
	if len(list) != 6 || list[4].Kind != "drift" || list[4].Evidence != "lane.reason:  → handwritten" {
		t.Fatalf("an old transition hid new drift: %+v", list)
	}
}

func TestUnpublishedTemporaryLeavesOldRecordReadable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	old := Record{Lane: Lane{Fact: Fact{Word: "running"}}, Version: 1}
	if _, err := replaceFixture(dir, time.Now(), old, "fixture"); err != nil {
		t.Fatal(err)
	}
	file, err := os.CreateTemp(dir, "state.json.*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(file).Encode(Record{Lane: Lane{Fact: Fact{Word: "paused"}}}); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	got, exists, err := Read(dir)
	if err != nil || !exists || !reflect.DeepEqual(got, old) {
		t.Fatalf("old record: %+v %v %v", got, exists, err)
	}
}

func TestStateLockSerializesConcurrentTransitions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := replaceFixture(dir, time.Now(), Record{Lane: Lane{Fact: Fact{Word: "running"}}}, "fixture"); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for i := 0; i < 12; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if _, err := Transition(dir, time.Now(), "lane", "running", "running", "fixture", "increment", func(r *Record) { r.Holds.Barren++ }); err != nil {
				t.Error(err)
			}
		}()
	}
	workers.Wait()
	got, _, err := Read(dir)
	if err != nil || got.Holds.Barren != 12 || len(events(t, dir)) != 13 {
		t.Fatalf("lost update: %+v %v", got, err)
	}
}

func TestTransitionIndependentBatchWordsAndInvalidKinds(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := Record{Batch: Batch{Execution: Fact{Word: "running"}, Admission: Fact{Word: "open"}}}
	if _, err := replaceFixture(dir, time.Now(), r, "fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err := Transition(dir, time.Now(), "batch.admission", "open", "sealed", "keeper", "full proof", func(r *Record) { r.Batch.Admission.Word = "sealed" }); err != nil {
		t.Fatal(err)
	}
	got, _, _ := Read(dir)
	if got.Batch.Execution.Word != "running" || got.Batch.Admission.Word != "sealed" {
		t.Fatal(got.Batch)
	}
	for _, kind := range []string{"missing", "engine"} {
		if _, err := Transition(dir, time.Now(), kind, "", "", "", "", func(*Record) {}); err == nil {
			t.Fatalf("accepted %q", kind)
		}
	}
}

func TestDiffNamesEveryRecordField(t *testing.T) {
	t.Parallel()
	base := Record{HandIns: []HandIn{{Goal: "A", SHA: "a", Ordinal: 1}}, Questions: []Question{{ID: "Q"}}}
	at := time.Date(2026, 10, 12, 20, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		field  string
		change func(*Record)
	}{
		{"lane.word", func(r *Record) { r.Lane.Word = "paused" }},
		{"lane.since.at", func(r *Record) { r.Lane.Since.At = at }},
		{"lane.since.observed", func(r *Record) { r.Lane.Since.Observed = true }},
		{"lane.since.source", func(r *Record) { r.Lane.Since.Source = "pause.json" }},
		{"lane.by", func(r *Record) { r.Lane.By = "person" }},
		{"lane.reason", func(r *Record) { r.Lane.Reason = "maintenance" }},
		{"agent.word", func(r *Record) { r.Agent.Word = "ended" }},
		{"agent.step.args", func(r *Record) { r.Agent.Step.Args = []string{"--gate"} }},
		{"agent.step.started_at", func(r *Record) { r.Agent.Step.StartedAt = "2026-10-12T20:00:00Z" }},
		{"agent.session", func(r *Record) { r.Agent.Session = "session" }},
		{"agent.step.verb", func(r *Record) { r.Agent.Step = Step{Verb: "prove"} }},
		{"agent.since.at", func(r *Record) { r.Agent.Since.At = at }},
		{"batch.id", func(r *Record) { r.Batch.ID = "batch" }},
		{"batch.base", func(r *Record) { r.Batch.Base = "main" }},
		{"batch.execution.word", func(r *Record) { r.Batch.Execution.Word = "running" }},
		{"batch.execution.since.at", func(r *Record) { r.Batch.Execution.Since.At = at }},
		{"batch.admission.word", func(r *Record) { r.Batch.Admission.Word = "sealed" }},
		{"batch.admission.since.at", func(r *Record) { r.Batch.Admission.Since.At = at }},
		{"batch.members", func(r *Record) { r.Batch.Members = []Member{{Goal: "B", SHA: "b"}} }},
		{"hand_ins[0].goal", func(r *Record) { r.HandIns[0].Goal = "B" }},
		{"hand_ins[0].sha", func(r *Record) { r.HandIns[0].SHA = "b" }},
		{"hand_ins[0].ordinal", func(r *Record) { r.HandIns[0].Ordinal = 2 }},
		{"hand_ins[0].word", func(r *Record) { r.HandIns[0].Word = "returned" }},
		{"hand_ins[0].since.at", func(r *Record) { r.HandIns[0].Since.At = at }},
		{"hand_ins[0].at", func(r *Record) { r.HandIns[0].At = at }},
		{"hand_ins[0].reason", func(r *Record) { r.HandIns[0].Reason = "own test failed" }},
		{"resolution.goal", func(r *Record) { r.Resolution.Goal = "A" }},
		{"resolution.job", func(r *Record) { r.Resolution.Job = "job" }},
		{"resolution.word", func(r *Record) { r.Resolution.Word = "resolved" }},
		{"resolution.since.at", func(r *Record) { r.Resolution.Since.At = at }},
		{"proof.attempt", func(r *Record) { r.Proof.Attempt = "attempt" }},
		{"proof.scope", func(r *Record) { r.Proof.Scope = "gate" }},
		{"proof.word", func(r *Record) { r.Proof.Word = "green" }},
		{"proof.since.at", func(r *Record) { r.Proof.Since.At = at }},
		{"proof.load_at_start", func(r *Record) { r.Proof.LoadAtStart = &hostload.Sample{Available: true, Cores: 8, Load1m: 7} }},
		{"main.commit", func(r *Record) { r.Main.Commit = "commit" }},
		{"main.word", func(r *Record) { r.Main.Word = "red" }},
		{"main.since.at", func(r *Record) { r.Main.Since.At = at }},
		{"main.open_incidents", func(r *Record) { r.Main.OpenIncidents = []string{"incident"} }},
		{"questions[0].id", func(r *Record) { r.Questions[0].ID = "Q2" }},
		{"questions[0].premise", func(r *Record) { r.Questions[0].Premise = "gone" }},
		{"questions[0].since.at", func(r *Record) { r.Questions[0].Since.At = at }},
		{"holds.barren", func(r *Record) { r.Holds.Barren = 2 }},
		{"holds.selection_pending", func(r *Record) { r.Holds.SelectionPending = true }},
		{"holds.since.at", func(r *Record) { r.Holds.Since.At = at }},
		{"engine.stamp", func(r *Record) { r.Engine.Stamp = "engine" }},
	} {
		t.Run(tc.field, func(t *testing.T) {
			t.Parallel()
			next := base
			next.HandIns = slicesCopy(base.HandIns)
			next.Questions = slicesCopy(base.Questions)
			tc.change(&next)
			changed := Diff(base, next)
			if len(changed) != 1 || !strings.HasPrefix(changed[0], tc.field+": ") {
				t.Fatalf("%s disappeared from diff: %q", tc.field, changed)
			}
		})
	}
}

func slicesCopy[T any](items []T) []T { return append([]T(nil), items...) }

func TestReplaceRebuildsUnreadablePredecessorAndKeepsPublicationFence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 10, 12, 20, 0, 0, 0, time.UTC)
	path := filepath.Join(dir, "state.json")
	broken := []byte(`{"version":99,"lane":{"word":17}}`)
	if err := os.WriteFile(path, broken, 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Replace(dir, now, Record{}, "fixture", 0, Fingerprint{Value: "before", Current: func() string { return "after" }})
	var stale *Stale
	if !errors.As(err, &stale) {
		t.Fatalf("stale unreadable record was accepted: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != string(broken) {
		t.Fatalf("refused publication changed predecessor: %s %v", data, err)
	}
	changed, err := Replace(dir, now, Record{}, "fixture", 0, Fingerprint{Current: func() string { return "" }})
	if err != nil || len(changed) != 1 || !strings.HasPrefix(changed[0], "previous record unreadable: ") {
		t.Fatalf("rebuild: %v %v", changed, err)
	}
	got, exists, err := Read(dir)
	list := events(t, dir)
	if err != nil || !exists || got.Version != 1 || len(list) != 1 || list[0].Kind != "rebuild" || !list[0].Rebuild || list[0].Evidence != changed[0] {
		t.Fatalf("unreadable predecessor: record=%+v exists=%v error=%v events=%+v", got, exists, err, list)
	}
}

func TestReplaceObservationRecoveryWithAndWithoutFactChanges(t *testing.T) {
	t.Parallel()
	for _, changedFact := range []bool{false, true} {
		t.Run(map[bool]string{false: "same-facts", true: "changed-facts"}[changedFact], func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			now := time.Date(2026, 10, 12, 20, 0, 0, 0, time.UTC)
			initial := Record{Lane: Lane{Fact: Fact{Word: "running"}}}
			if _, err := replaceFixture(dir, now, initial, "fixture"); err != nil {
				t.Fatal(err)
			}
			if err := NoteObservation(dir, now, []string{"read failed"}); err != nil {
				t.Fatal(err)
			}
			before, _, err := Read(dir)
			if err != nil {
				t.Fatal(err)
			}
			if changedFact {
				initial.Lane.Word = "paused"
			}
			if _, err := replaceFixture(dir, now, initial, "fixture"); err != nil {
				t.Fatal(err)
			}
			got, _, err := Read(dir)
			list := events(t, dir)
			wantEvents := 3
			if changedFact {
				wantEvents += 2
				if list[2].Kind != "drift" || list[2].Evidence != "lane.word: running → paused" {
					t.Fatalf("recovery lost drift: %+v", list)
				}
			}
			if err != nil || got.LastObservation != nil || got.Version != before.Version+1 || len(list) != wantEvents || list[len(list)-1].Kind != "observation-recovered" || list[len(list)-1].Rebuild {
				t.Fatalf("recovery: record=%+v error=%v events=%+v", got, err, list)
			}
			if _, err := replaceFixture(dir, now, initial, "fixture"); err != nil || len(events(t, dir)) != wantEvents {
				t.Fatalf("unchanged refresh repeated recovery: %v", err)
			}
		})
	}
}

func replaceFixture(dir string, now time.Time, next Record, by string) ([]string, error) {
	previous, _, err := Read(dir)
	if err != nil {
		return nil, err
	}
	return Replace(dir, now, next, by, previous.Version, Fingerprint{Current: func() string { return "" }})
}

func TestReplaceSkipsTornEventAndRecordsEvidence(t *testing.T) {
	t.Parallel()
	for _, tail := range []string{"\n", ""} {
		t.Run(map[string]string{"\n": "line", "": "tail"}[tail], func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			now := time.Date(2026, 10, 12, 20, 0, 0, 0, time.UTC)
			r := Record{Lane: Lane{Fact: Fact{Word: "running"}}}
			if _, err := replaceFixture(dir, now, r, "fixture"); err != nil {
				t.Fatal(err)
			}
			file, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				t.Fatal(err)
			}
			_, writeErr := file.WriteString(`{"kind":` + tail)
			if err := errors.Join(writeErr, file.Close()); err != nil {
				t.Fatal(err)
			}
			// Even unchanged facts must acknowledge the damaged history on the next rebuild.
			if _, err := replaceFixture(dir, now, r, "keeper"); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			var last Event
			if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil {
				t.Fatal(err)
			}
			if !last.Rebuild || last.By != "keeper" || !strings.Contains(last.Evidence, "events.jsonl: 1 undecodable line skipped") {
				t.Fatalf("rebuild lost damage evidence: %+v", last)
			}
			if _, err := replaceFixture(dir, now, r, "keeper"); err != nil {
				t.Fatal(err)
			}
			list, skipped, err := ReadEvents(dir)
			if err != nil || skipped != 1 || len(list) != 2 {
				t.Fatalf("unchanged refresh repeated rebuild: %+v %d %v", list, skipped, err)
			}
			r.Lane.Word = "paused"
			if _, err := replaceFixture(dir, now.Add(time.Minute), r, "keeper"); err != nil {
				t.Fatal(err)
			}
			list, skipped, err = ReadEvents(dir)
			if err != nil || skipped != 1 || len(list) != 4 || list[2].Kind != "drift" || list[3].Kind != "rebuild" {
				t.Fatalf("later events disappeared past torn line: %+v %d %v", list, skipped, err)
			}
		})
	}
}
