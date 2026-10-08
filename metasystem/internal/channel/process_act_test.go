package channel

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProcessActTextAnswerRemainsPending(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("channel.human.answer-code=off\n"), 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	q, err := Ask(AskRequest{RepoRoot: root, About: "machine", Kind: "other", ProcessAct: "operation", Now: now, Facts: []string{"A setting is proposed."}, Wants: "metasystem settings set KEY VALUE --repo CHECKOUT --act operation"})
	if err != nil {
		t.Fatal(err)
	}
	q.Thread = &MessageRef{ID: "thread", ThreadID: "thread"}
	q.Answer = &Answer{Text: q.Wants, At: now, Phase: "matched"}
	if err := writeJSON(questionPath(root, q.ID), q); err != nil {
		t.Fatal(err)
	}
	if _, err := Poll(context.Background(), PollConfig{RepoRoot: root, Now: now, Provider: &testProvider{}}); err != nil {
		t.Fatal(err)
	}
	q, err = ReadQuestion(root, q.ID)
	if err != nil || q.State != "open" || q.Answer.Phase != "matched" {
		t.Fatalf("text stood for a successful act: %+v %v", q, err)
	}
}
