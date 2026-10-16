package project

import (
	"errors"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
)

func TestPublicationCommitGitAdapter(t *testing.T) {
	t.Parallel()
	for _, reference := range []string{"missing", "held"} {
		t.Run(reference, func(t *testing.T) {
			t.Parallel()
			f := testgoal.NewPublicationFixture(t)
			commit := f.Candidate(reference)
			err := CheckCommit(f.Root, commit)
			if reference == "missing" {
				f.AssertRefusal(err)
			} else if err != nil {
				t.Fatal(err)
			}
			f.Write("plans/designs/draft.md", "# Draft\n\n- Kind: design\n- Id: draft\n- Status: draft\n- Goals: absent-draft\n")
			if err := CheckCommit(f.Root, f.Base); err != nil {
				t.Fatalf("unstaged draft affected committed decision: %v", err)
			}
			if got := f.Git("rev-parse", "HEAD"); got != f.Base {
				t.Fatalf("checker moved caller to %s", got)
			}
			if got := f.Git("worktree", "list", "--porcelain"); containsDetachedCandidate(got, commit) {
				t.Fatal("candidate worktree survived check")
			}
		})
	}
}

func containsDetachedCandidate(list, commit string) bool {
	// The caller is at Base; a candidate entry means temporary worktree cleanup failed.
	return len(list) > 0 && strings.Contains(list, "HEAD "+commit+"\n")
}

func TestPublicationRebuiltCandidateGitAdapter(t *testing.T) {
	t.Parallel()
	f := testgoal.NewPublicationFixture(t)
	bad := f.Candidate("missing")
	f.Git("update-ref", goal.LocalLedgerBranch, f.Base)
	endpoint, err := PublicationEndpoint(f.Root)
	if err != nil {
		t.Fatal(err)
	}
	checks, pushes := 0, 0
	check := endpoint.CheckCandidate
	endpoint.CheckCandidate = func(commit string) error { checks++; return check(commit) }
	endpoint = endpoint.WithCASPublisher(func(_ goal.Endpoint, tip, commit string) (goal.CASOutcome, error) {
		pushes++
		f.Git("update-ref", goal.LocalLedgerBranch, bad, tip)
		return goal.CASRefused, errors.New("tip advanced")
	})
	result, err := goal.Publish(endpoint, goal.PublishRequest{
		Opid: goal.Opid("01J5X000000000000000000003", "fixture", "session"), Machine: "fixture", Lineage: "session", Message: "fixture records", Intent: goal.Intent{Verb: "inbox"},
		Mutate: func(string) ([]goal.Change, error) {
			return []goal.Change{{Path: "memory/one.md", Content: []byte("one")}}, nil
		},
	})
	if err != nil || result.Outcome != goal.OutcomeRejected {
		t.Fatalf("retry result=%+v error=%v", result, err)
	}
	f.AssertRefusal(errors.New(result.Detail))
	if checks != 2 || pushes != 1 {
		t.Fatalf("checks=%d publications=%d; want 2, 1", checks, pushes)
	}
}

func TestPublicationRecordCauseGitAdapter(t *testing.T) {
	t.Parallel()
	f := testgoal.NewPublicationFixture(t)
	f.Write("plans/designs/change.md", "# Change\n\n- Kind: design\n- Id: change\n- Status: wrong\n")
	candidate := f.Commit()
	f.Git("checkout", "-q", "--detach", f.Base)
	err := CheckCommit(f.Root, candidate)
	if err == nil || !strings.Contains(err.Error(), "the page change.md: the status wrong is not one of draft, accepted, superseded, done; the push waits") || strings.Contains(err.Error(), "plans/designs") {
		t.Fatalf("actionable refusal: %v", err)
	}
}
