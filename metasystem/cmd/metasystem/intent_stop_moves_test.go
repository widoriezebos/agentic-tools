package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/audit"
)

func init() {
	registerIdempotency("test declare-moves", idemStateful, "the same moves and reason write the same declaration; a repeat rewrites nothing", witnessTestDeclareMovesRepeat)
}

// stopMovesBed is an installation whose declaration owner is recorded: it
// writes one fixed declaration under docs/stop-decision-moves, or fails
// with fail when set.
func stopMovesBed(t *testing.T, fail error) (*intentBed, intentOwners, *[]string) {
	t.Helper()
	b := newIntentBed(t, false, nil)
	owners := b.owners()
	var calls []string
	owners.stopMovesDeclare = func(root string, options audit.StopSurfaceOptions, goalID, reason string) (string, error) {
		if options.GoalRecord == nil {
			t.Errorf("the declaration owner got no goal reader")
		}
		calls = append(calls, goalID+"|"+reason)
		if fail != nil {
			return "", fail
		}
		path := filepath.Join(root, "docs", "stop-decision-moves", goalID+"-0123456789ab.txt")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		return path, os.WriteFile(path, []byte("goal="+goalID+"\nreason="+reason+"\n"), 0o644)
	}
	return b, owners, &calls
}

// TestTestDeclareMovesRecordsTheDeclaration is the public home of the
// former audit stop-decision-surface --declare: the goal and reason reach
// the declaration owner, the result names the file to commit, and an owner
// refusal (a goal not allowed stop-test changes) names the person's allow.
func TestTestDeclareMovesRecordsTheDeclaration(t *testing.T) {
	t.Parallel()
	b, owners, calls := stopMovesBed(t, nil)
	code, result := b.runJSON(owners, "test", "declare-moves", bedGoal, "--reason", "the hook moved its Stop checks")
	if code != 0 || result.Outcome != intentConfirmed || len(*calls) != 1 || (*calls)[0] != bedGoal+"|the hook moved its Stop checks" ||
		!strings.Contains(result.Summary, "docs/stop-decision-moves/") {
		t.Fatalf("test declare-moves = %d %+v calls %v", code, result, *calls)
	}
	if code, result := b.runJSON(owners, "test", "declare-moves", bedGoal); code != 2 || result.Outcome != intentRefused {
		t.Fatalf("test declare-moves without a reason = %d %+v", code, result)
	}
	refused, refusedOwners, _ := stopMovesBed(t, errors.New("STOP_SURFACE_GOAL_REFUSED: goal "+bedGoal+" does not allow stop-test changes"))
	code, result = refused.runJSON(refusedOwners, "test", "declare-moves", bedGoal, "--reason", "r")
	if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "does not allow stop-test changes") ||
		result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), "goal allow "+bedGoal+" stop-test-changes") {
		t.Fatalf("refused declaration = %d %+v", code, result)
	}
}

func witnessTestDeclareMovesRepeat(t *testing.T) {
	b, owners, _ := stopMovesBed(t, nil)
	if code, result := b.runJSON(owners, "test", "declare-moves", bedGoal, "--reason", "r"); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("first declaration = %d %+v", code, result)
	}
	if code, result := b.runJSON(owners, "test", "declare-moves", bedGoal, "--reason", "r"); code != 0 || result.Outcome != intentUnchanged {
		t.Fatalf("repeated declaration = %d %+v", code, result)
	}
}
