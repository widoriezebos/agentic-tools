package plain

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestRunningProofSurvivesOnlyLedgerTreeMoves(t *testing.T) {
	t.Parallel()
	for _, ledger := range []bool{true, false} {
		t.Run(map[bool]string{true: "ledger", false: "source"}[ledger], func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			git := b.seams.Git
			moved := false
			b.seams.Alive = func(Running) bool { return true }
			b.seams.Git = func(dir string, args ...string) (string, error) {
				if moved && args[0] == "rev-parse" && strings.HasPrefix(args[len(args)-1], "HEAD^") {
					if strings.HasSuffix(args[len(args)-1], "{tree}") {
						return "new-tree", nil
					}
					return "new-commit", nil
				}
				if args[0] == "diff" && args[len(args)-2] == "tree" && args[len(args)-1] == "new-tree" {
					if ledger {
						return "metasystem/plans/goals/next.md\nmetasystem/memory/receipts.log", nil
					}
					return "metasystem/internal/landing/plain/prove.go", nil
				}
				return git(dir, args...)
			}
			calls := 0
			b.seams.Command = func(cmd *exec.Cmd) error {
				calls++
				moved = true
				active, already, err := Start(b.install, b.checkout, b.seams)
				if ledger {
					if err != nil || !already || active.Attempt == "" || active.Tree != "tree" {
						t.Errorf("ledger move restarted/refused running proof: %+v already=%v err=%v", active, already, err)
					}
				} else {
					var busy *Busy
					if !errors.As(err, &busy) {
						t.Errorf("source move reused running proof: %+v %v", active, err)
					}
				}
				return cmd.Run()
			}
			result, err := Run(b.install, b.checkout, "printf 'LANDING-CHECKED\t0\n'", "", io.Discard, b.seams)
			if err != nil || result.Result != Green || calls != 1 {
				t.Fatalf("original check: %+v calls=%d err=%v", result, calls, err)
			}
			newer, found, err := ResultFor(b.install, "new-tree")
			if err != nil {
				t.Fatal(err)
			}
			if ledger {
				data, err := os.ReadFile(scopePath(b.install, result.Attempt))
				var scope scopeRecord
				if err != nil || json.Unmarshal(data, &scope) != nil || scope.Scope != "full" {
					t.Fatalf("carrying green rewrote the original execution scope: %s err=%v", data, err)
				}
				if !found || newer.Result != Green || newer.CountedFull || newer.Attempt != result.Attempt || newer.Commit != "new-commit" || newer.FullTree != result.Tree || newer.FullAt != result.At {
					t.Fatalf("new tree has no carried proof: %+v found=%v original=%+v", newer, found, result)
				}
				_, already, err := Start(b.install, b.checkout, b.seams)
				if err != nil || !already {
					t.Fatalf("finished ledger tree reran: %v %v", already, err)
				}
			} else {
				if found {
					t.Fatalf("source move inherited proof: %+v", newer)
				}
				b.seams.Launch = func([]string, string, string) (int64, error) { return int64(os.Getpid()), nil }
				started, already, err := Start(b.install, b.checkout, b.seams)
				if err != nil || already || started.Tree != "new-tree" {
					t.Fatalf("source move did not start fresh proof: %+v already=%v err=%v", started, already, err)
				}
			}
		})
	}
}
