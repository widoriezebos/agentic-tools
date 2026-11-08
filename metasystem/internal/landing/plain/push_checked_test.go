package plain

import (
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestBatchPushNeedsAFullGreen(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"impact", "full"} {
		t.Run(scope, func(t *testing.T) {
			t.Parallel()
			b := newScopeBed(t)
			batch := &Batch{ID: "batch", Base: "main", State: BatchRunning, Members: []GoalSHA{{Goal: "g", SHA: "member"}},
				Lane: lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1}}
			if _, _, err := HandIn(b.install, Line{Goal: "g", SHA: "member", At: bedNow.Format(time.RFC3339)}); err != nil {
				t.Fatal(err)
			}
			if err := writeBatch(b.install, batch); err != nil {
				t.Fatal(err)
			}
			result := Result{Commit: b.git.commit, Tree: b.git.tree, Result: Green, Scope: scope, At: bedNow.Format(time.RFC3339),
				BatchID: batch.ID, BatchMembers: batch.Members, BaseCommit: batch.Base,
				Executions: []ExecutionAdmission{{DepthScope: scope, DepthBase: batch.Base, BatchID: batch.ID, Tree: b.git.tree}}}
			if scope == "full" {
				result.FullTree, result.FullAt = result.Tree, result.At
			}
			b.save(t, result)
			pushed := false
			notAncestor := exec.Command("/usr/bin/false").Run()
			b.seams.Git = func(dir string, args ...string) (string, error) {
				switch args[0] {
				case "fetch", "cat-file":
					return "", nil
				case "rev-parse":
					if args[2] == "refs/remotes/origin/main^{commit}" {
						return batch.Base, nil
					}
				case "merge-base":
					if args[3] == b.git.commit && (args[2] == batch.Base || args[2] == "member") {
						return "", nil
					}
					return "", notAncestor
				case "rev-list":
					if args[1] == "--first-parent" {
						return b.git.commit + " main member", nil
					}
				case "show":
					if strings.Join(args, " ") == "show -s --format=%cI HEAD" {
						return result.At, nil
					}
				case "push":
					want := []string{"push", "--quiet", "--force-with-lease=refs/heads/main:main", "origin", b.git.commit + ":refs/heads/main"}
					if !reflect.DeepEqual(args, want) {
						t.Fatalf("push args=%v, want %v", args, want)
					}
					pushed = true
					return "", nil
				}
				return b.git.run(dir, args...)
			}
			outcome, err := PushChecked(b.install, b.checkout, bedNow, nil, b.seams)
			if scope == "impact" {
				var refusal *Refusal
				if !errors.As(err, &refusal) || refusal.Code != CodeUnproven || refusal.Reason != "an impact check never lands a batch; the batch needs a full check" || refusal.Next != "metasystem landing prove" || pushed || outcome.Changed {
					t.Fatalf("impact push: %+v %v pushed=%v", outcome, err, pushed)
				}
			} else if err != nil || !pushed || !outcome.Changed {
				t.Fatalf("full push: %+v %v pushed=%v", outcome, err, pushed)
			}
		})
	}
}

// This adapter test uses real Git to observe the fetched main and publication;
// a stub cannot prove that the callback's refusal leaves the remote unchanged.
func TestPushCheckedAdapterPublicationBoundary(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	old := b.originMain()
	sha := b.seat("seat-a", "goal-a")
	head := b.merge("goal-a")
	if _, err := PushChecked(b.install, b.checkout, bedNow, func(string, string) error {
		t.Fatal("checked an unproven push")
		return nil
	}); err == nil {
		t.Fatal("an unproven push was not refused")
	}
	b.prove(b.greenScript)
	// Advance main to a parent of HEAD while the checkout still holds old.
	b.git(b.root, "--git-dir", b.origin, "update-ref", "refs/heads/main", sha)
	if got := b.git(b.checkout, "rev-parse", "refs/remotes/origin/main"); got != old {
		t.Fatalf("fixture main was not stale: %s", got)
	}
	refused := errors.New("design does not stand")
	checks := 0
	before := func(fetched, candidate string) error {
		checks++
		if fetched != sha || candidate != head || b.originMain() != sha {
			t.Fatalf("check before publication: main=%s head=%s remote=%s", fetched, candidate, b.originMain())
		}
		return refused
	}
	outcome, err := PushChecked(b.install, b.checkout, bedNow, before)
	if !errors.Is(err, refused) || outcome.Changed || outcome.Old != sha || outcome.Commit != head || checks != 1 || b.originMain() != sha {
		t.Fatalf("refused publication: outcome=%+v err=%v checks=%d remote=%s", outcome, err, checks, b.originMain())
	}
	if _, ok, err := LastPush(b.install); err != nil || ok {
		t.Fatalf("a refused check recorded a push: present=%v err=%v", ok, err)
	}
	outcome, err = PushChecked(b.install, b.checkout, bedNow, func(fetched, candidate string) error {
		if fetched != sha || candidate != head {
			t.Fatalf("checked the wrong commits: main=%s head=%s", fetched, candidate)
		}
		return nil
	})
	if err != nil || !outcome.Changed || b.originMain() != head {
		t.Fatalf("allowed publication: outcome=%+v err=%v remote=%s", outcome, err, b.originMain())
	}
	if push, ok, err := LastPush(b.install); err != nil || !ok || push.Old != sha || push.Commit != head {
		t.Fatalf("allowed push record: push=%+v present=%v err=%v", push, ok, err)
	}
	outcome, err = PushChecked(b.install, b.checkout, bedNow, func(string, string) error {
		t.Fatal("checked a push already on main")
		return nil
	})
	if err != nil || outcome.Changed {
		t.Fatalf("already on main: outcome=%+v err=%v", outcome, err)
	}
}
