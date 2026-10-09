package plain

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProofBudgetCountsOnlyCompletedFullAttempts(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"two reds", "two greens", "red and green", "flake green", "duplicate repeat marker", "not run", "scoped", "unrelated batch", "changed hand-in", "closed"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			goals := []GoalSHA{{Goal: "a", SHA: "sha-a"}, {Goal: "b", SHA: "sha-b"}, {Goal: "c", SHA: "sha-c"}}
			first := Result{Tree: "first", Commit: "first", Result: Red, Attempt: "first", Goals: goals, CountedFull: true}
			second := Result{Tree: "second", Commit: "second", Result: Green, Attempt: "second", Goals: goals[:2], CountedFull: true}
			refused := false
			switch name {
			case "two reds":
				second.Result, refused = Red, true
			case "flake green":
				second.Attempt = first.Attempt
			case "two greens":
				first.Result = Green
			case "duplicate repeat marker":
				second.Attempt, second.Result, refused = first.Attempt, Red, false
			case "not run", "scoped":
				second.CountedFull, refused = false, false
			case "unrelated batch":
				second.Goals, refused = []GoalSHA{{Goal: "other", SHA: "other-sha"}}, false
			case "changed hand-in":
				second.Result, refused = Red, true
				second.Goals = []GoalSHA{{Goal: "a", SHA: "new-sha"}}
			case "closed":
				second.LoopClosed, refused = true, false
			}
			if _, _, err := HandIn(install, Line{Goal: "a", SHA: "sha-a"}); err != nil {
				t.Fatal(err)
			}
			if err := withLock(install, func() error {
				if err := appendLine(resultsPath(install), first); err != nil {
					return err
				}
				if err := appendLine(resultsPath(install), second); err != nil {
					return err
				}
				if name == "flake green" {
					third := first
					third.Attempt = "third"
					return appendLine(resultsPath(install), third)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			seams := ProveSeams{Git: func(string, ...string) (string, error) { return "", nil }}
			err := withLock(install, func() error { return checkProofBudget(install, "checkout", "third", seams) })
			var budget *ProofBudget
			if errors.As(err, &budget) && err.Error() != "this batch used two full checks; goals hold; ask a person to run: metasystem landing prove" {
				t.Fatalf("budget remedy: %v", err)
			}
			if errors.As(err, &budget) != refused || err != nil && !errors.As(err, &budget) {
				t.Fatalf("%s: refused=%v err=%v", name, refused, err)
			}
		})
	}
}

func TestReplayBatchRejectsMismatchedFirstParents(t *testing.T) {
	t.Parallel()
	for _, mismatch := range []string{"first merge", "later merge"} {
		t.Run(mismatch, func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			firstParent, laterParent := "main", "merge-a"
			if mismatch == "first merge" {
				firstParent = "old-main"
			} else {
				laterParent = "unrelated-commit"
			}
			git := stubGit{installPrefix: "metasystem"}
			b.seams.Git = func(dir string, args ...string) (string, error) {
				switch {
				case strings.Join(args, " ") == "rev-parse --verify --quiet refs/remotes/origin/main^{commit}":
					return "main", nil
				case len(args) == 4 && strings.Join(args[:3], " ") == "show -s --format=%P":
					return "main sha-b", nil
				case args[0] == "log":
					return fmt.Sprintf("merge-a %s sha-a\nmerge-b %s sha-b", firstParent, laterParent), nil
				case len(args) == 3 && args[0] == "rev-parse" && strings.HasSuffix(args[2], "^{tree}"):
					return args[2] + "-tree", nil
				default:
					return git.run(dir, args...)
				}
			}
			checks := 0
			b.seams.Command = func(cmd *exec.Cmd) error {
				checks++
				for _, env := range cmd.Env {
					if env == "LANDING_COMMIT=merge-b" {
						fmt.Fprint(cmd.Stdout, "LANDING-FAILED\tu/a\tTestA\nLANDING-CHECKED\t1\n")
						return errors.New("red")
					}
				}
				fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
				return nil
			}
			red := Result{Result: Red, Goals: []GoalSHA{{Goal: "a", SHA: "sha-a"}, {Goal: "b", SHA: "sha-b"}}, Failed: []FailedUnit{{Unit: "u/a", Tests: []string{"TestA"}}}}
			result := replayBatch(b.seams, b.install, b.checkout, "fixture", Running{Attempt: "replay", Commit: "merge-b"}, red, Result{})
			if result.Cause == nil || result.Cause.Kind != "unclassified" || result.Cause.Goal != "" || result.Cause.SHA != "" || result.Repeat != "" || checks != 0 {
				t.Fatalf("discontinuous replay attributed a goal or allowed a repeat: %+v cause=%+v checks=%d", result, result.Cause, checks)
			}
		})
	}
}

func TestReplayFirstTreeOutsideMainCannotAttributeOwn(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	red := Result{Result: Red, Failed: []FailedUnit{{Unit: "u/a", Tests: []string{"TestA"}}}, Cause: &Cause{Kind: "unclassified"}}
	if err := os.MkdirAll(filepath.Join(Dir(b.install), "proofs"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A gate supplies its prior tree without claiming that it is origin/main.
	result := classifyReplay(b.seams, b.install, b.checkout, failedReport, Running{Attempt: "replay"}, red, Result{}, []replayTree{{Running: Running{Commit: "commit"}, Goal: GoalSHA{Goal: "a", SHA: "sha-a"}}})
	if result.Cause.Kind != "unclassified" || result.Repeat != "" {
		t.Fatalf("prior-tree failure: %+v", result)
	}

}
