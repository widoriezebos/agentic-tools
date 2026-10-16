package channel

import (
	"context"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
)

func TestQuestionBookkeepingCandidateGitAdapter(t *testing.T) {
	t.Parallel()
	for _, reference := range []string{"held", "missing"} {
		t.Run(reference, func(t *testing.T) {
			t.Parallel()
			f := testgoal.NewPublicationFixture(t)
			candidate := f.Candidate(reference)
			_, err := Ask(AskRequest{RepoRoot: f.Root, Goal: "held", Kind: "other", Machine: "fixture", Lineage: "session", Facts: []string{"A decision is needed"}, Provider: &testProvider{}, Now: f.Now})
			tip := f.Git("rev-parse", goal.LocalLedgerBranch)
			if reference == "missing" {
				f.AssertRefusal(err)
				if tip != candidate {
					t.Fatalf("refused question published %s", tip)
				}
			} else if err != nil || tip == candidate {
				t.Fatalf("valid question: tip=%s error=%v", tip, err)
			}
		})
	}
}

func TestInboxCandidateGitAdapter(t *testing.T) {
	t.Parallel()
	for _, reference := range []string{"held", "missing"} {
		t.Run(reference, func(t *testing.T) {
			t.Parallel()
			f := testgoal.NewPublicationFixture(t)
			candidate := f.Candidate(reference)
			code, err := TOTPCode("JBSWY3DPEHPK3PXP", f.Now)
			if err != nil {
				t.Fatal(err)
			}
			provider := &testProvider{inbound: []Inbound{{Ref: MessageRef{ID: "2"}, UserID: "UWIDO", Text: "answer " + code, SentAt: f.Now, Ack: "2"}}}
			_, err = Poll(context.Background(), PollConfig{RepoRoot: f.Root, Destination: "fleet", ProviderName: "fake", HumanUserID: "UWIDO", TOTPSecret: "JBSWY3DPEHPK3PXP", Machine: "fixture", Lineage: "session", Provider: provider, Now: f.Now})
			tip := f.Git("rev-parse", goal.LocalLedgerBranch)
			if reference == "missing" {
				f.AssertRefusal(err)
				if tip != candidate || provider.confirms != 0 {
					t.Fatalf("refused inbox published=%t confirms=%d", tip != candidate, provider.confirms)
				}
			} else if err != nil || tip == candidate || provider.confirms != 1 {
				t.Fatalf("valid inbox: tip=%s confirms=%d error=%v", tip, provider.confirms, err)
			}
		})
	}
}
