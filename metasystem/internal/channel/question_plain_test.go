package channel

import (
	"os"
	"testing"
	"time"
)

func TestQuestionWordsAreRecordedOutsideItsIdentity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	provider := &testProvider{}
	r := AskRequest{RepoRoot: root, Goal: "g", Kind: "other", Facts: []string{"Proceed?"}, Options: []Option{{Label: "yes", Consequence: "continue"}}, Plain: &Plain{Decision: "Proceed?", Act: "continue the work", IfNothing: "the work waits"}, Provider: provider, Now: time.Date(2031, 1, 1, 9, 0, 0, 0, time.UTC)}
	q, _, err := AskOrFind(r)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := ReadQuestion(root, q.ID)
	if err != nil || stored.Plain == nil || *stored.Plain != *r.Plain {
		t.Fatalf("stored words: %+v %v", stored.Plain, err)
	}
	before, err := os.ReadFile(questionPath(root, q.ID))
	if err != nil {
		t.Fatal(err)
	}
	r.Plain = &Plain{Decision: "Other words", Act: "another act", IfNothing: "another consequence"}
	same, existing, err := AskOrFind(r)
	after, readErr := os.ReadFile(questionPath(root, q.ID))
	qs, unread := WalkQuestions(root)
	if err != nil || readErr != nil || !existing || same.ID != q.ID || string(before) != string(after) || len(qs) != 1 || len(unread) != 0 || len(provider.posts) != 1 {
		t.Fatalf("other words changed the question: %+v %v %v, %d posts", same, err, readErr, len(provider.posts))
	}
}
