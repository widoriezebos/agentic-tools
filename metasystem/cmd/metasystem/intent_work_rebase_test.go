package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

func rebaseIntentBed(t *testing.T) (*workBed, intentOwners, *int) {
	t.Helper()
	b := newWorkBed(t)
	owners := b.workOwners()
	owners.connection.recordRebase = func(*intentInvocation, string, branch.RebaseResult) error { return nil }
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

func rebaseJudgementFixture(t *testing.T) *branch.RebaseConflict {
	t.Helper()
	refused := &branch.RebaseConflict{OpError: &branch.OpError{Code: branch.RebaseJudgementCode, Message: "choose the source version"}}
	err := json.Unmarshal([]byte(`{"MainTip":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","Unit":"u1","Paths":[{"Path":"source.go","Original":"base-blob","Main":"main-blob","Goal":"goal-blob","FirstLine":2,"LastLine":3,"MainCommit":"main-commit","MainGoal":"peer"},{"Path":"added.go","Main":"main-added","Goal":"goal-added","MainCommit":"another-commit"}]}`), refused)
	if err != nil {
		t.Fatal(err)
	}
	return refused
}

func TestIntentWorkRebaseJudgementAsksPerPath(t *testing.T) {
	t.Parallel()
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			t.Parallel()
			b, owners, _ := rebaseIntentBed(t)
			refused := rebaseJudgementFixture(t)
			owners.connection.rebase = func(branch.RebaseRequest) (branch.RebaseResult, error) {
				return branch.RebaseResult{}, fmt.Errorf("branch stopped: %w", refused)
			}
			var inputs []channelAskInput
			owners.connection.askRebase = func(root string, in channelAskInput) (channel.Question, []string, int, error) {
				if realpath.ResolveExisting(root) != realpath.ResolveExisting(b.root()) {
					t.Fatalf("question root %s want %s", root, b.root())
				}
				inputs = append(inputs, in)
				if failure {
					return channel.Question{}, nil, 1, errors.New("channel unavailable")
				}
				return channel.Question{ID: fmt.Sprintf("q%d", len(inputs))}, nil, 0, nil
			}
			code, stdout, stderr := b.run(owners, "work", "rebase", b.id)
			if code == 0 || stdout != "" || len(inputs) != 2 || !strings.Contains(stderr, "main is at aaaaaaaaaaaa") || !strings.Contains(stderr, "nothing was changed") {
				t.Fatalf("refusal %d %q %q inputs %v", code, stdout, stderr, inputs)
			}
			text := func(s string) string { return strings.Join(strings.Fields(s), " ") }
			wantQuestions := []string{
				fmt.Sprintf("Goal %s and main both changed lines 2 to 3 of source.go. Keep main's, keep the goal's, or write a third.", b.id),
				fmt.Sprintf("Goal %s and main both changed added.go where no common version exists. Keep main's, keep the goal's, or write a third.", b.id),
			}
			for i, in := range inputs {
				if in.Goal != b.id || in.Kind != "other" || len(in.Facts) != 3 || len(in.Options) != 3 || text(in.Facts[0]) != wantQuestions[i] {
					t.Fatalf("question %+v", in)
				}
				change := "peer"
				if i == 1 {
					change = "another-commit"
				}
				impact := fmt.Sprintf("Impact: main's drops what %s did there and unit u1 loses its read; the goal's undoes main's change there (%s); a third is read again. Nothing lands until you answer.", b.id, change)
				if text(in.Facts[1]) != impact {
					t.Fatalf("impact %q want %q", in.Facts[1], impact)
				}
				want := []string{fmt.Sprintf("keep main's: main's drops what %s did there and unit u1 loses its read", b.id), fmt.Sprintf("keep the goal's: the goal's undoes main's change there (%s)", change), "write a third: a third is read again"}
				if !reflect.DeepEqual(in.Options, want) {
					t.Fatalf("options %q", in.Options)
				}
				if failure && !strings.Contains(text(stderr), wantQuestions[i]) {
					t.Fatalf("missing fallback %q", stderr)
				}
				if !failure && !strings.Contains(stderr, fmt.Sprintf("metasystem question wait q%d", i+1)) {
					t.Fatalf("missing wait %q", stderr)
				}
			}
			if inputs[0].Facts[2] != "Blobs: original base-blob, main main-blob, goal goal-blob." {
				t.Fatal(inputs[0].Facts[2])
			}
		})
	}
}

func TestIntentWorkRebaseJudgementCode(t *testing.T) {
	t.Parallel()
	b, owners, _ := rebaseIntentBed(t)
	owners.connection.rebase = func(branch.RebaseRequest) (branch.RebaseResult, error) {
		return branch.RebaseResult{}, rebaseJudgementFixture(t)
	}
	owners.connection.askRebase = func(string, channelAskInput) (channel.Question, []string, int, error) {
		return channel.Question{ID: "question-id"}, nil, 0, nil
	}
	code, result := b.runJSON(owners, "work", "rebase", b.id)
	if code == 0 || result.Data.(map[string]any)["code"] != branch.RebaseJudgementCode || result.Next == nil || strings.Join(result.Next.Argv, " ") != "metasystem question wait question-id" {
		t.Fatalf("refusal %+v code %d", result, code)
	}
}
