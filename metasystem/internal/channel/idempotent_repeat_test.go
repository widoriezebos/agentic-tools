package channel

import (
	"os"
	"testing"
	"time"
)

// Asking the exact open question again finds it and asks nothing: no second
// question file, no second post (R-129-ui). The same facts with another
// option is another question and is asked.
func TestAskOrFindReturnsTheSameOpenQuestion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := &testProvider{}
	request := AskRequest{RepoRoot: root, Goal: "g", Kind: "other", Machine: "m", Facts: []string{"Land it?"},
		Options: []Option{{Label: "yes", Consequence: "land"}}, Recommendation: "yes", Provider: p, Now: time.Unix(1, 0)}
	first, existing, err := AskOrFind(request)
	if err != nil || existing || first.Thread == nil {
		t.Fatalf("first ask = %+v existing %v err %v", first, existing, err)
	}
	again, existing, err := AskOrFind(request)
	if err != nil || !existing || again.ID != first.ID || len(p.posts) != 1 {
		t.Fatalf("repeat ask = %+v existing %v posts %d err %v", again, existing, len(p.posts), err)
	}
	if files, _ := os.ReadDir(channelRoot(root) + "/questions"); len(files) != 1 {
		t.Fatalf("a repeated ask wrote %d question files", len(files))
	}
	request.Options = append(request.Options, Option{Label: "no", Consequence: "wait"})
	other, existing, err := AskOrFind(request)
	if err != nil || existing || other.ID == first.ID || len(p.posts) != 2 {
		t.Fatalf("an ask with another option = %+v existing %v err %v", other, existing, err)
	}
}

// Withdrawing a closed question again posts and writes nothing.
func TestWithdrawAClosedQuestionChangesNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := &testProvider{}
	asked, err := Ask(AskRequest{RepoRoot: root, Goal: "g", Kind: "other", Machine: "m", Facts: []string{"fact"}, Provider: p, Now: time.Unix(1, 0)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Withdraw(root, asked.ID, "decided", p, DestinationConfig{}); err != nil {
		t.Fatal(err)
	}
	closed, err := os.ReadFile(questionPath(root, asked.ID))
	if err != nil {
		t.Fatal(err)
	}
	q, err := Withdraw(root, asked.ID, "again", p, DestinationConfig{})
	if err != nil || q.State != "closed" || len(p.posts) != 2 {
		t.Fatalf("repeat withdraw = %+v posts %q err %v", q, p.posts, err)
	}
	if again, _ := os.ReadFile(questionPath(root, asked.ID)); string(again) != string(closed) {
		t.Fatalf("a repeated withdrawal rewrote the question")
	}
}
