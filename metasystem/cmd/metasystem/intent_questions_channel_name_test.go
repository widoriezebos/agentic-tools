package main

import (
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
)

// question list and question show name a channel question channel:ID, so
// retry and withdraw take that name as well as the bare id.
func TestQuestionRetryAndWithdrawTakeTheChannelName(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	provider := &questionProvider{}
	owners := b.owners()
	owners.processes.question = channel.ReadQuestion
	owners.processes.channelLink = func(string) (channel.Provider, channel.DestinationConfig) {
		return provider, channel.DestinationConfig{}
	}
	root := b.root()
	writeQuestionFixture(t, filepath.Join(root, "artifacts", "agents", "channel", "questions", "q1.json"),
		map[string]any{"id": "q1", "goal": bedGoal, "kind": "other", "state": "open", "facts": []string{"Land it?"}, "options": []any{map[string]any{"label": "yes", "consequence": "land"}}})
	if code, result := b.runJSON(owners, "question", "retry", "channel:q1"); code != 0 || result.Outcome != intentConfirmed || len(provider.posts) != 1 {
		t.Fatalf("retry channel:q1 = %d %+v, posts %d; want the question delivered", code, result, len(provider.posts))
	}
	if code, result := b.runJSON(owners, "question", "withdraw", "channel:q1", "--reason", "the block cleared"); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("withdraw channel:q1 = %d %+v; want the question withdrawn", code, result)
	}
	if q, err := channel.ReadQuestion(root, "q1"); err != nil || q.State != "closed" {
		t.Fatalf("question q1 after withdraw channel:q1 = %+v %v; want it closed", q, err)
	}
}
