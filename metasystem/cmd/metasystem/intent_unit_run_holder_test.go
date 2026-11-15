package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

func nonHolderOwners(b *workBed) intentOwners {
	owners := b.workOwners()
	owners.connection.claimCheck = func(string, string, goal.Endpoint) func() error {
		return func() error {
			return refusal.New("OWNED-ELSEWHERE", "holder=another-session", fmt.Errorf("this checkout is held by another session\nfor a separate copy to write in, run: metasystem session isolate"))
		}
	}
	return owners
}

func TestACommandWithoutTheClaimWritesNothing(t *testing.T) {
	t.Parallel()
	b, run, build := completedRound(t)
	before := workIdemSnapshot(t, b.unitRoot)
	launched := b.starter.launched()
	code, result := collectWithOwners(t, b, run, nonHolderOwners(b))
	assertCallerErrorSummary(t, result, "The next step could not start; no failure was recorded")
	if code != 1 || result.Outcome != intentRefused || resultData(t, result)["state"] != "running" || !slices.Equal(launched, b.starter.launched()) {
		t.Fatalf("non-holder advanced the run: code=%d outcome=%s state=%v launches=%v", code, result.Outcome, resultData(t, result)["state"], b.starter.launched())
	}
	workIdemSameFiles(t, "refused wait", before, workIdemSnapshot(t, b.unitRoot))
	reservation, err := dispatchcore.ReadRecordObject(filepath.Join(b.root(), "artifacts", "agents", "jobs", build+".json"))
	if err != nil || reservation["status"] != "pending" {
		t.Fatalf("non-holder collected the build: %+v %v", reservation, err)
	}
	code, result = collectWithOwners(t, b, run, b.workOwners())
	if code != 0 || resultData(t, result)["state"] != "awaiting-judgement" || !slices.Equal(b.starter.launched(), []string{"build", "proof"}) {
		t.Fatalf("holder could not finish: %d %s %v", code, result.Summary, b.starter.launched())
	}
	before = workIdemSnapshot(t, b.unitRoot)
	code, result = collectWithOwners(t, b, run, nonHolderOwners(b))
	if code != 0 || resultData(t, result)["state"] != "awaiting-judgement" {
		t.Fatalf("non-holder could not read the judgement: %d %s", code, result.Summary)
	}
	workIdemSameFiles(t, "judged wait", before, workIdemSnapshot(t, b.unitRoot))
}
