package channel

import (
	"errors"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"os"
	"testing"
	"time"
)

func TestQ1ActFailureIsScopedToMatchingQuestions(t *testing.T) {
	t.Parallel()
	b := newNamedActBed(t)
	unrelated := b.ask(t, "Accept?", nil)
	act := NamedAct{Verb: "work stop", Goal: "g", Target: "run-x"}
	matching := b.ask(t, "Cancel?", func(r *AskRequest) { r.Options[0].NamedAct = &act })
	if err := os.WriteFile(questionPath(b.root, "damaged"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	failed := func(string) (goal.Endpoint, error) { return goal.Endpoint{}, errors.New("unavailable") }
	results := CompleteNamedAct(b.root, b.root, act, "Wido", "acting", b.now.Add(time.Second), b.proof, failed)
	if len(results) != 1 || results[0].Question != matching.ID || results[0].Err == nil {
		t.Fatalf("results %+v", results)
	}
	if b.read(t, unrelated).Answer != nil {
		t.Fatal("unrelated question changed")
	}
	b.complete(t, act, b.now.Add(2*time.Second))
	if b.read(t, matching).State != "closed" {
		t.Fatal("matching question stayed open")
	}
}

func TestQ1NoMatchingQuestionDoesNoPublication(t *testing.T) {
	t.Parallel()
	b := newNamedActBed(t)
	never := func(string) (goal.Endpoint, error) {
		t.Fatal("resolved publication without a match")
		return goal.Endpoint{}, nil
	}
	for range 2 {
		if results := CompleteNamedAct(b.root, b.root, b.act, "Wido", "acting", b.now, b.proof, never); len(results) != 0 {
			t.Fatal(results)
		}
	}
	if _, err := os.Stat(channelRoot(b.root) + "/named-acts"); !os.IsNotExist(err) {
		t.Fatal("created completion store")
	}
}

func TestQ1ActCannotAnswerAnotherInstallation(t *testing.T) {
	t.Parallel()
	b := newNamedActBed(t)
	q := b.ask(t, "Choose", nil)
	elsewhere := t.TempDir()
	CompleteNamedAct(elsewhere, b.root, b.act, "Wido", "acting", b.now.Add(time.Second), b.proof, b.resolve)
	WalkOpenQuestions(b.root)
	if b.read(t, q).Answer != nil {
		t.Fatal("another installation answered this question")
	}
}

func TestQ1QuestionInstallationAndAuthorityRootCanDiffer(t *testing.T) {
	t.Parallel()
	b := newNamedActBed(t)
	authorityRoot := b.root
	b.root = t.TempDir()
	q := b.ask(t, "Choose", nil)
	results := CompleteNamedAct(b.root, authorityRoot, b.act, "Wido", "acting", b.now.Add(time.Second), b.proof, b.resolve)
	if len(results) != 1 || results[0].Err != nil || b.read(t, q).State != "closed" {
		t.Fatalf("adopted question not answered: %+v", results)
	}
}
