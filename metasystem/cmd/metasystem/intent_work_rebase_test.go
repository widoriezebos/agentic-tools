package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

func rebaseIntentBed(t *testing.T) (*workBed, intentOwners, *int) {
	t.Helper()
	b := newWorkBed(t)
	owners := b.workOwners()
	writes := new(int)
	owners.connection.endpointTip = func(string, goal.Endpoint) (string, error) { return strings.Repeat("a", 40), nil }
	owners.connection.section = func(_ string, body func(func(func() error) error) error) error {
		return body(func(fn func() error) error { return fn() })
	}
	owners.connection.rebase = func(req branch.RebaseRequest) (branch.RebaseResult, error) {
		if req.Repo != b.worktree || req.GoalID != b.id || req.Remote != "origin" || req.CheckClaim == nil || req.Gate == nil {
			t.Fatalf("rebase request %+v", req)
		}
		result := branch.RebaseResult{State: "held", OldTip: strings.Repeat("b", 40), NewTip: strings.Repeat("b", 40), MainTip: req.EndpointTip, Carried: []string{}, NeedsReview: []string{}}
		if *writes == 0 {
			*writes++
			result.State = "rebased"
			result.OldTip = strings.Repeat("c", 40)
		}
		return result, nil
	}
	return b, owners, writes
}

func witnessWorkRebaseRepeat(t *testing.T) {
	b, owners, writes := rebaseIntentBed(t)
	code, first := b.runJSON(owners, "work", "rebase", b.id)
	if code != 0 || first.Outcome != intentConfirmed || *writes != 1 {
		t.Fatalf("first %+v code=%d", first, code)
	}
	files := workIdemSnapshot(t, b.root())
	code, again := b.runJSON(owners, "work", "rebase", b.id)
	if code != 0 || again.Outcome != intentUnchanged || *writes != 1 || !strings.Contains(again.Summary, "nothing was written") {
		t.Fatalf("repeat %+v code=%d", again, code)
	}
	workIdemSameFiles(t, "work rebase", files, workIdemSnapshot(t, b.root()))
}

func TestIntentWorkRebaseJSON(t *testing.T) {
	t.Parallel()
	b, owners, _ := rebaseIntentBed(t)
	expected := branch.RebaseResult{State: "rebased", OldTip: strings.Repeat("b", 40), NewTip: strings.Repeat("c", 40), MainTip: strings.Repeat("a", 40), Carried: []string{"u1"}, NeedsReview: []string{"u2"}}
	owners.connection.rebase = func(branch.RebaseRequest) (branch.RebaseResult, error) { return expected, nil }
	code, result := b.runJSON(owners, "work", "rebase", b.id)
	data, err := json.Marshal(result.Data)
	var got branch.RebaseResult
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if code != 0 || !reflect.DeepEqual(got, expected) {
		t.Fatalf("JSON %+v code=%d", got, code)
	}
}

func TestIntentWorkRebaseNeedsWorktree(t *testing.T) {
	t.Parallel()
	b, owners, writes := rebaseIntentBed(t)
	b.branchListed = false
	code, result := b.runJSON(owners, "work", "rebase", b.id)
	if code == 0 || *writes != 0 || result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), "work build "+b.id) {
		t.Fatalf("missing worktree %+v code=%d", result, code)
	}
}

func TestIntentWorkRebaseRepeat(t *testing.T) {
	t.Parallel()
	witnessWorkRebaseRepeat(t)
}

func TestIntentWorkRebaseConflictPaths(t *testing.T) {
	t.Parallel()
	b, owners, _ := rebaseIntentBed(t)
	owners.connection.rebase = func(branch.RebaseRequest) (branch.RebaseResult, error) {
		return branch.RebaseResult{}, &branch.OpError{Code: branch.RebaseConflictCode,
			Message: "rebase stopped at build u1; main is at aaaaaaa; nothing was changed\npaths:\ncode.go\nother.go\nrun: metasystem work status " + b.id}
	}
	code, stdout, stderr := b.run(owners, "work", "rebase", b.id)
	if code == 0 || stdout != "" || !strings.Contains(stderr, "code.go") || !strings.Contains(stderr, "other.go") {
		t.Fatalf("conflict code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestIntentWorkRebaseFromGoalWorktree(t *testing.T) {
	t.Parallel()
	b, owners, _ := rebaseIntentBed(t)
	command, rest, _ := resolveIntentArgv([]string{"work", "rebase", b.id, "--json"})
	var stdout, stderr bytes.Buffer
	if code := runIntentIn(command, rest, &stdout, &stderr, b.worktree, owners); code != 0 {
		t.Fatalf("goal worktree code=%d stdout=%s stderr=%s", code, &stdout, &stderr)
	}
}
