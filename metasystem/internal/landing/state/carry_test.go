package state

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestCarryKeepsOnlyUnchangedIdentityAndWord(t *testing.T) {
	t.Parallel()
	first := Since{At: time.Date(2026, 10, 12, 19, 40, 0, 0, time.UTC), Observed: true, Source: "queue.jsonl"}
	later := first
	later.At = first.At.Add(10 * time.Minute)
	previous := Record{Lane: Lane{Fact: Fact{Word: "running", Since: first}}, Agent: Agent{Session: "s", Word: "step-process-gone", Since: first}, Batch: Batch{ID: "b", Execution: Fact{Word: "running", Since: first}, Admission: Fact{Word: "sealed", Since: first}}, HandIns: []HandIn{{Fact: Fact{Word: "landed", Since: first}, Goal: "g", SHA: "sha", Ordinal: 1}}, Resolution: Resolution{Fact: Fact{Word: "resolving", Since: first}, Goal: "g", Job: "j"}, Proof: Proof{Fact: Fact{Word: "died", Since: first}, Attempt: "p"}, Main: Main{Fact: Fact{Word: "unproven", Since: first}, Commit: "m"}, Questions: []Question{{ID: "q", Premise: "current", Since: first}}, Holds: Holds{Since: first}}
	next := previous
	next.HandIns = append([]HandIn(nil), previous.HandIns...)
	next.Questions = append([]Question(nil), previous.Questions...)
	next.Lane.Since, next.Agent.Since, next.Batch.Execution.Since, next.Batch.Admission.Since = later, later, later, later
	next.HandIns[0].Since, next.Resolution.Since, next.Proof.Since, next.Main.Since, next.Questions[0].Since, next.Holds.Since = later, later, later, later, later, later
	if got := Carry(&previous, next); len(Diff(previous, got)) != 0 {
		t.Fatalf("unchanged state drifted: %v", Diff(previous, got))
	}
	next.HandIns[0].Ordinal++
	next.Agent.Word = "ended"
	next.Batch.ID = "another"
	next.Proof.Attempt = "another"
	next.Main.Commit = "another"
	got := Carry(&previous, next)
	for _, since := range []Since{got.HandIns[0].Since, got.Agent.Since, got.Batch.Execution.Since, got.Proof.Since, got.Main.Since} {
		if since != later {
			t.Fatalf("different identity reused time: %+v", since)
		}
	}
	next.Lane.Since = Since{At: later.At}
	if got := Carry(&previous, next); got.Lane.Since != next.Lane.Since {
		t.Fatal("transition time overwritten")
	}
	if !reflect.DeepEqual(previous.HandIns[0].Since, first) {
		t.Fatal("carry changed its input")
	}
}

func TestStalePublicationAndFailedObservationPreserveSections(t *testing.T) {
	t.Parallel()
	dir, now := t.TempDir(), time.Now().UTC()
	initial := Record{Lane: Lane{Fact: Fact{Word: "running"}}}
	if _, err := replaceFixture(dir, now, initial, "fixture"); err != nil {
		t.Fatal(err)
	}
	saved, _, _ := Read(dir)
	candidate := saved
	candidate.Lane.Word = "paused"
	for _, expect := range []struct {
		Version     int
		Fingerprint Fingerprint
	}{{Version: 0, Fingerprint: Fingerprint{Value: "a", Current: func() string { return "a" }}}, {Version: saved.Version, Fingerprint: Fingerprint{Value: "a", Current: func() string { return "b" }}}} {
		_, err := Replace(dir, now, candidate, "stalled", expect.Version, expect.Fingerprint)
		var stale *Stale
		if !errors.As(err, &stale) {
			t.Fatalf("not stale: %v", err)
		}
		got, _, _ := Read(dir)
		if !reflect.DeepEqual(got, saved) || len(events(t, dir)) != 1 {
			t.Fatal("stale publication wrote state or event")
		}
	}
	if err := NoteObservation(dir, now, []string{"queue.jsonl unreadable"}); err != nil {
		t.Fatal(err)
	}
	got, _, _ := Read(dir)
	if len(Diff(saved, got)) != 0 || got.Version != saved.Version+1 || got.LastObservation.Problems[0] != "queue.jsonl unreadable" {
		t.Fatalf("failure lost state: %+v", got)
	}
	if list := events(t, dir); len(list) != 2 || list[1].Kind != "observation-failed" {
		t.Fatal(list)
	}
	if err := NoteUnstable(dir, now); err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil || len(bytes) == 0 {
		t.Fatal(err)
	}
	if list := events(t, dir); len(list) != 3 || list[2].Kind != "observation-unstable" {
		t.Fatal(list)
	}
}
