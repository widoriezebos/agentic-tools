package plain

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestReplayBatchUsesTrueMainBase(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"two members own", "one member own", "main red", "moved main"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			tip, fetched := "merge-b", "main"
			if name == "one member own" {
				tip = "merge-a"
			}
			if name == "moved main" {
				fetched = "new-main"
			}
			git := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				switch {
				case strings.Join(args, " ") == "rev-parse --verify --quiet refs/remotes/origin/main^{commit}":
					return fetched, nil
				case args[0] == "cat-file":
					return "", nil
				case args[0] == "merge-base":
					if args[2] == "main" {
						return "", nil
					}
					return "", &exec.ExitError{ProcessState: replayExitOne(t)}
				case len(args) == 4 && args[0] == "show":
					if args[3] == "merge-b" {
						return "merge-a sha-b", nil
					}
					return "main sha-a", nil
				case args[0] == "log":
					if args[len(args)-1] != "main.."+tip {
						t.Fatalf("wrong replay range: %v", args)
					}
					out := "merge-a main sha-a"
					if tip == "merge-b" {
						out += "\nmerge-b merge-a sha-b"
					}
					return out, nil
				case args[0] == "rev-parse" && strings.HasSuffix(args[len(args)-1], "^{tree}"):
					return strings.TrimSuffix(args[len(args)-1], "^{tree}") + "-tree", nil
				default:
					return git(dir, args...)
				}
			}
			var prefixes []string
			b.seams.Command = func(cmd *exec.Cmd) error {
				commit := ""
				for _, env := range cmd.Env {
					if strings.HasPrefix(env, "LANDING_COMMIT=") {
						commit = strings.TrimPrefix(env, "LANDING_COMMIT=")
					}
				}
				prefixes = append(prefixes, commit)
				if commit != "main" || name == "main red" {
					fmt.Fprint(cmd.Stdout, "LANDING-FAILED\taudit\tTestAuditA\nLANDING-CHECKED\t1\n")
					return errors.New("red audit")
				}
				fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
				return nil
			}
			incidents := 0
			b.seams.RecordMain = func(checks []Result) error { incidents += len(checks); return nil }
			goals := []GoalSHA{{Goal: "a", SHA: "sha-a"}, {Goal: "b", SHA: "sha-b"}}
			red := Result{Result: Red, Goals: goals, Failed: []FailedUnit{{Unit: "audit", Tests: []string{"TestAuditA"}}}}
			got := replayBatch(b.seams, b.install, b.checkout, "fixture", Running{Attempt: "red", Commit: tip}, red, Result{})
			if name == "main red" {
				if got.Cause.Kind != "main" || incidents != 1 {
					t.Fatalf("main: %+v incidents=%d", got.Cause, incidents)
				}
			} else {
				if got.Cause.Kind != "own" || got.Cause.Goal != "a" || incidents != 0 {
					t.Fatalf("own: %+v incidents=%d", got.Cause, incidents)
				}
				want := []string{"main", "merge-a"}
				if tip == "merge-a" {
					want = want[:1]
				}
				if !slices.Equal(prefixes, want) {
					t.Fatalf("prefixes=%v want=%v", prefixes, want)
				}
			}
		})
	}
}

func replayExitOne(t *testing.T) *os.ProcessState {
	t.Helper()
	cmd := exec.Command("sh", "-c", "exit 1")
	_ = cmd.Run()
	return cmd.ProcessState
}

func TestProofBudgetResetsOnBatchBase(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	goals := []GoalSHA{{Goal: "a", SHA: "sha-a"}}
	seams := ProveSeams{Git: func(string, ...string) (string, error) { return "", nil }}
	_, _, err := HandIn(install, Line{Goal: "a", SHA: "sha-a"})
	if err != nil {
		t.Fatal(err)
	}
	record := func(base, id string) {
		t.Helper()
		if err := appendLine(resultsPath(install), Result{BatchBase: base, Result: Red, Attempt: id, CountedFull: true, Goals: goals}); err != nil {
			t.Fatal(err)
		}
	}
	check := func(wantHold bool) {
		t.Helper()
		err := checkProofBudget(install, "checkout", "tip", seams)
		var held *ProofBudget
		if errors.As(err, &held) != wantHold || err != nil && !errors.As(err, &held) {
			t.Fatalf("hold=%v err=%v", wantHold, err)
		}
	}
	if err := writeBatch(install, &Batch{ID: "batch", Base: "A", State: BatchRunning, Lane: lane.Record{Root: install, Install: install, CustodyEpoch: 1}}); err != nil {
		t.Fatal(err)
	}
	record("A", "a1")
	record("A", "a2")
	check(true)
	if err := writeBatch(install, &Batch{ID: "batch", Base: "B", State: BatchRunning, Lane: lane.Record{Root: install, Install: install, CustodyEpoch: 1}}); err != nil {
		t.Fatal(err)
	}
	check(false)
	record("B", "b1")
	check(false)
	record("B", "b2")
	check(true)
	history, err := Results(install)
	if err != nil || len(history) != 4 || history[0].BatchBase != "A" || history[3].BatchBase != "B" {
		t.Fatalf("history=%+v err=%v", history, err)
	}
}

func TestClassifyAttemptKeepsSavedBatchBase(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	if err := os.MkdirAll(Dir(b.install), 0755); err != nil {
		t.Fatal(err)
	}
	red := Result{Attempt: "saved-red", Result: Red, Commit: "commit", Tree: "tree", BatchBase: "saved-main", Cause: &Cause{Kind: "environment", Name: "lost-process"}}
	if err := appendLine(resultsPath(b.install), red); err != nil {
		t.Fatal(err)
	}
	b.seams.CommandForCommit = func(string) (string, error) { return "fixture", nil }
	b.seams.Person = &ActProvenance{Person: "Wido", Root: b.install}
	got, err := ClassifyAttempt(b.install, b.checkout, red.Attempt, b.seams)
	if err != nil || got.BatchBase != red.BatchBase {
		t.Fatalf("saved red: %+v err=%v", got, err)
	}
	running, ok, _, err := ReadRunning(b.install, b.seams)
	if err != nil || !ok || running.BatchBase != red.BatchBase {
		t.Fatalf("saved checkpoint: %+v err=%v", running, err)
	}
}

func TestProofBudgetNeverBorrowsABaseForLegacyAttempts(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	goals := []GoalSHA{{Goal: "a", SHA: "sha-a"}}
	_, _, err := HandIn(install, Line{Goal: "a", SHA: "sha-a"})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeBatch(install, &Batch{ID: "batch", Base: "current-main", State: BatchRunning, Lane: lane.Record{Root: install, Install: install, CustodyEpoch: 1}}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"old-1", "old-2"} {
		if err := appendLine(resultsPath(install), Result{Result: Red, Attempt: id, CountedFull: true, Goals: goals}); err != nil {
			t.Fatal(err)
		}
	}
	if err := checkProofBudget(install, "checkout", "tip", ProveSeams{Git: func(string, ...string) (string, error) { return "", nil }}); err != nil {
		t.Fatalf("attempts with no recorded base held the current batch: %v", err)
	}
}

func TestProofBudgetIgnoresLateOldBaseResult(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	goals := []GoalSHA{{Goal: "a", SHA: "sha-a"}}
	_, _, err := HandIn(install, Line{Goal: "a", SHA: "sha-a"})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeBatch(install, &Batch{ID: "batch", Base: "B", State: BatchRunning, Lane: lane.Record{Root: install, Install: install, CustodyEpoch: 1}}); err != nil {
		t.Fatal(err)
	}
	for _, attempt := range []struct{ base, id string }{{"B", "b1"}, {"B", "b2"}, {"A", "late-a"}} {
		if err := appendLine(resultsPath(install), Result{BatchBase: attempt.base, Result: Red, Attempt: attempt.id, CountedFull: true, Goals: goals}); err != nil {
			t.Fatal(err)
		}
	}
	err = checkProofBudget(install, "checkout", "tip", ProveSeams{Git: func(string, ...string) (string, error) { return "", nil }})
	var held *ProofBudget
	if !errors.As(err, &held) {
		t.Fatalf("old-base result reopened the current base's spent budget: %v", err)
	}
}
