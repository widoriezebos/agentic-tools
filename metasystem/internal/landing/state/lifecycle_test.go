package state

import (
	"errors"
	"testing"
	"time"
)

func TestLifecycleTransitionFailureAndReplay(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Unix(10, 0)
	failed := errors.New("fixture write failed")
	if _, err := Transition(dir, now, "result:proof", "present", "retired", "keeper", "outside retention", nil, func() error { return failed }); !errors.Is(err, failed) {
		t.Fatalf("failure=%v", err)
	}
	events, _, err := ReadEvents(dir)
	if err != nil || len(events) != 0 {
		t.Fatalf("failed retirement recorded: %+v %v", events, err)
	}
	if _, err := Transition(dir, now, "result:proof", "present", "retired", "keeper", "outside retention", nil); err != nil {
		t.Fatal(err)
	}
	writes := 0
	_, err = Transition(dir, now, "result:proof", "present", "retired", "keeper", "outside retention", nil, func() error { writes++; return nil })
	var refused *Refused
	if !errors.As(err, &refused) || writes != 0 {
		t.Fatalf("replay=%v writes=%d", err, writes)
	}
}

func TestLifecycleRows(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	due := map[string]LifecycleFacts{
		"loop-stop": {Owner: Loop, Subjects: []string{"landed"}, Subject: "fixture", Detail: "landed"},
		"question":  {Premise: Gone}, "fix": {Word: "done"},
		"submission": {Word: "returned", At: now.Add(-SubmissionWindow - time.Second)},
		"result":     {At: now.Add(-ResultRetention - time.Second)},
		"agent-step": {ProcessGone: true}, "batch": {Word: "closed", Empty: true},
	}
	for _, row := range Lifecycle {
		t.Run(row.Kind, func(t *testing.T) {
			t.Parallel()
			if row.Retires == "" {
				t.Fatal("missing human-readable rule")
			}
			if got, _ := row.Due(LifecycleFacts{}, now); got {
				t.Fatal("unknown facts retired")
			}
			f, exists := due[row.Kind]
			if got, reason := row.Due(f, now); got != exists || got && reason == "" {
				t.Fatalf("due=%v reason=%q", got, reason)
			}
		})
	}
	for _, f := range []LifecycleFacts{{Owner: Person, Subjects: []string{"landed"}}, {Owner: Person, Attempt: "old", CurrentAttempt: "new"}} {
		if got, _ := LifecycleDue("loop-stop", f, now); got {
			t.Fatal("retired a person's stop")
		}
	}
	if got, _ := LifecycleDue("loop-stop", LifecycleFacts{Owner: Loop, Attempt: "old", CurrentAttempt: "new"}, now); !got {
		t.Fatal("obsolete proof survived")
	}
	for _, word := range []string{"returned", "landed", "waiting"} {
		if got, _ := LifecycleDue("submission", LifecycleFacts{Word: word, At: now.Add(-SubmissionWindow)}, now); got {
			t.Fatal("day boundary retired")
		}
	}
	if got, _ := LifecycleDue("result", LifecycleFacts{At: now.Add(-ResultRetention)}, now); got {
		t.Fatal("retention boundary retired")
	}
	if got, _ := LifecycleDue("result", LifecycleFacts{At: now.Add(-20 * SubmissionWindow), Latest: true}, now); got {
		t.Fatal("latest result retired")
	}
	for _, subjects := range [][]string{nil, {"waiting"}, {"unknown"}, {"landed", "waiting"}} {
		if got, _ := LifecycleDue("fix", LifecycleFacts{Word: Abandoned, Subjects: subjects, Attempt: "old", CurrentAttempt: "new"}, now); got {
			t.Fatal("abandoned repair retired before its subject ended")
		}
	}
	for _, word := range []string{"landed", "returned", "superseded"} {
		if got, _ := LifecycleDue("fix", LifecycleFacts{Word: Abandoned, Subjects: []string{word}}, now); !got {
			t.Fatal("abandoned repair survived its subject")
		}
	}
}
