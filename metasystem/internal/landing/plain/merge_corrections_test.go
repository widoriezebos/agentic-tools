package plain

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestSeatMergeProofRequiresResolutionAndRead(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"resolving", "reviewing"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			fix := &Fix{Goal: "goal", Units: []string{"lane-merge-1"}, Commit: "base", Tip: "tip", Job: "build", State: state, Attempt: "attempt", Paths: []conflict.Path{{Path: "code.go", Class: conflict.Builder}}}
			if err := WriteFix(root, fix); err != nil {
				t.Fatal(err)
			}
			seams := ProveSeams{Git: func(string, ...string) (string, error) {
				t.Fatal("proof read Git before finishing the resolution and read")
				return "", nil
			}}
			_, _, startErr := Start(root, root, seams)
			_, _, settledErr := Settled(root, root, seams)
			_, runErr := Run(root, root, "exit 0", "", &bytes.Buffer{}, seams)
			for _, err := range []error{startErr, settledErr, runErr} {
				if err == nil || !strings.Contains(err.Error(), "finish the resolution and its read") {
					t.Fatalf("unfinished %s proof admitted: %v", state, err)
				}
			}
		})
	}
}

// Git's marker check must see the staged bytes, including staged conflict markers.
func TestGitAdapterSeatMergeRejectsStagedMarkers(t *testing.T) {
	t.Parallel()
	b := newResolveGitAdapterFixture(t, false)
	b.contract.Generated = nil
	b.seams.AsSeat = true
	if _, err := b.resolve(); err != nil {
		t.Fatal(err)
	}
	fix, err := ReadFix(b.install)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.install, "out/conflict"), []byte("<<<<<<< main\nmain\n=======\ngoal\n>>>>>>> goal\n"), 0600); err != nil {
		t.Fatal(err)
	}
	b.mustGit("add", "metasystem/out/conflict")
	b.seams.Run = func(_ []string, _ string, _ *os.File, started func(int64) error) error { return started(0) }
	err = CompleteMerge(b.home, b.install, b.checkout, fix, "metasystem test impact", b.seams)
	if err == nil || !strings.Contains(err.Error(), "conflict markers") {
		t.Fatalf("staged markers admitted: %v", err)
	}
	if b.mustGit("rev-parse", "HEAD") != fix.Commit || b.mustGit("rev-parse", "MERGE_HEAD") != fix.Tip {
		t.Fatal("marker refusal changed the merge")
	}
}

// A missing MERGE_HEAD is a real Git state, distinct from a failed Git adapter.
func TestGitAdapterSeatMergeAbandonedAllowsNextConflict(t *testing.T) {
	t.Parallel()
	for _, reader := range []string{"resolve", "status", "complete"} {
		t.Run(reader, func(t *testing.T) {
			t.Parallel()
			b := newResolveGitAdapterFixture(t, false)
			b.contract.Generated = nil
			b.seams.AsSeat = true
			if _, err := b.resolve(); err != nil {
				t.Fatal(err)
			}
			fix, err := ReadFix(b.install)
			if err != nil {
				t.Fatal(err)
			}
			mergeHead := b.mustGit("rev-parse", "--path-format=absolute", "--git-path", "MERGE_HEAD")
			if err := os.Remove(mergeHead); err != nil {
				t.Fatal(err)
			}
			switch reader {
			case "resolve":
				out, err := b.resolve()
				if err != nil || out.Outcome != "abandoned" || !strings.Contains(out.Reason, "pending merge is gone") {
					t.Fatalf("abandon=%+v %v", out, err)
				}
			case "status":
				record := lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1}
				status := ReadStatus(b.home, record, lane.View{Root: &b.checkout}, ProveSeams{Git: b.seams.Git})
				if status.RunningFix != nil || !strings.Contains(status.Summary, "pending merge is gone") {
					t.Fatalf("abandon status=%+v", status)
				}
			case "complete":
				err := CompleteMerge(b.home, b.install, b.checkout, fix, "metasystem test impact", b.seams)
				if err == nil || !strings.Contains(err.Error(), "pending merge is gone") {
					t.Fatalf("missing merge completed: %v", err)
				}
			}
			data, err := os.ReadFile(filepath.Join(Dir(b.install), "fixes", fix.Attempt+".json"))
			var closed Fix
			if err != nil || json.Unmarshal(data, &closed) != nil || closed.State != "abandoned" || closed.Reason == "" {
				t.Fatalf("closed=%+v err=%v", closed, err)
			}
			if active, err := ReadFix(b.install); err != nil || active != nil {
				t.Fatalf("abandoned remains active: %+v %v", active, err)
			}
			record := lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1}
			status := ReadStatus(b.home, record, lane.View{Root: &b.checkout}, ProveSeams{Git: b.seams.Git})
			if status.RunningFix != nil || !strings.Contains(status.Summary, "resolution was abandoned") {
				t.Fatalf("status lost the closed repair's reason: %+v", status)
			}
			// Restore the pending merge metadata so Git can clean up its index before the next conflict.
			if err := os.WriteFile(mergeHead, []byte(fix.Tip+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			b.mustGit("merge", "--abort")
			if _, err := b.seams.Git(b.checkout, "merge", "--no-commit", "goal"); err == nil {
				t.Fatal("new merge did not conflict")
			}
			b.seams.AsSeat = true
			out, err := b.resolve()
			if err != nil || out.Outcome != "resolving" {
				t.Fatalf("new conflict=%+v %v", out, err)
			}
			next, err := ReadFix(b.install)
			if err != nil || next == nil || next.Attempt == fix.Attempt {
				t.Fatalf("new repair=%+v %v", next, err)
			}
			if err := os.WriteFile(filepath.Join(b.install, "out/conflict"), []byte("main\ngoal\n"), 0600); err != nil {
				t.Fatal(err)
			}
			b.mustGit("add", "metasystem/out/conflict")
			b.seams.Run = func(_ []string, _ string, _ *os.File, started func(int64) error) error { return started(0) }
			if err := CompleteMerge(b.home, b.install, b.checkout, next, "metasystem test impact", b.seams); err != nil {
				t.Fatal(err)
			}
			if next.State != "reviewing" {
				t.Fatalf("next merge state=%s", next.State)
			}
		})
	}
}

// AUTO_MERGE contains the goal's clean additions, so only hand resolution appears in its diff.
func TestGitAdapterSeatMergePatchExcludesCleanGoalChanges(t *testing.T) {
	t.Parallel()
	b := newResolveGitAdapterFixture(t, false)
	b.mustGit("merge", "--abort")
	b.mustGit("checkout", "goal")
	if err := os.WriteFile(filepath.Join(b.checkout, "goal-only.txt"), []byte("goal addition\n"), 0600); err != nil {
		t.Fatal(err)
	}
	b.mustGit("add", "goal-only.txt")
	b.mustGit("commit", "-qm", "goal addition")
	tip := b.mustGit("rev-parse", "HEAD")
	b.mustGit("checkout", "main")
	if _, err := b.seams.Git(b.checkout, "merge", "--no-commit", "goal"); err == nil {
		t.Fatal("new goal did not conflict")
	}
	if _, _, err := HandIn(b.install, Line{Goal: "goal", SHA: tip}); err != nil {
		t.Fatal(err)
	}
	b.contract.Generated = nil
	b.seams.AsSeat = true
	if _, err := b.resolve(); err != nil {
		t.Fatal(err)
	}
	fix, err := ReadFix(b.install)
	if err != nil {
		t.Fatal(err)
	}
	base := fix.Commit
	if err := os.WriteFile(filepath.Join(b.install, "out/conflict"), []byte("main\ngoal\n"), 0600); err != nil {
		t.Fatal(err)
	}
	b.mustGit("add", "metasystem/out/conflict")
	b.seams.Run = func(_ []string, _ string, _ *os.File, started func(int64) error) error { return started(0) }
	if err := CompleteMerge(b.home, b.install, b.checkout, fix, "metasystem test impact", b.seams); err != nil {
		t.Fatal(err)
	}
	patch, err := os.ReadFile(filepath.Join(Dir(b.install), "fixes", fix.Attempt+".patch"))
	if err != nil || strings.Contains(string(patch), "goal-only.txt") || !strings.Contains(string(patch), "-<<<<<<< HEAD") || !strings.Contains(string(patch), "\n main\n") || !strings.Contains(string(patch), "\n goal\n") {
		t.Fatalf("resolution patch=%s err=%v", patch, err)
	}
	if context := b.mustGit("diff", "--binary", base, fix.Commit); !strings.Contains(context, "goal-only.txt") {
		t.Fatalf("first-parent context lost the clean addition: %s", context)
	}
}

func TestGitAdapterSeatMergeCommandsAllowHandIn(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"generator", "check"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			b := newResolveGitAdapterFixture(t, false)
			if command == "check" {
				b.contract.Generated = nil
			}
			b.seams.AsSeat = true
			if _, err := b.resolve(); err != nil {
				t.Fatal(err)
			}
			fix, err := ReadFix(b.install)
			if err != nil {
				t.Fatal(err)
			}
			if command == "check" {
				if err := os.WriteFile(filepath.Join(b.install, "out/conflict"), []byte("main\ngoal\n"), 0600); err != nil {
					t.Fatal(err)
				}
				b.mustGit("add", "metasystem/out/conflict")
			}
			generator := b.seams.Run
			handInDone := make(chan error, 1)
			attempted := false
			b.seams.Run = func(argv []string, dir string, log *os.File, started func(int64) error) error {
				isCheck := len(argv) > 0 && argv[0] == "env"
				if (command == "check") == isCheck {
					attempted = true
					go func() { _, _, err := HandIn(b.install, Line{Goal: "another", SHA: "another-tip"}); handInDone <- err }()
					select {
					case err := <-handInDone:
						if err != nil {
							return err
						}
					case <-time.After(5 * time.Second):
						return errors.New("the command held the queue lock; a hand-in could not finish")
					}
				}
				if isCheck {
					return started(0)
				}
				return generator(argv, dir, log, started)
			}
			if err := CompleteMerge(b.home, b.install, b.checkout, fix, "metasystem test impact", b.seams); err != nil {
				t.Fatal(err)
			}
			if !attempted {
				t.Fatal("command did not exercise a concurrent hand-in")
			}
			if entry, ok, err := Latest(b.install, "another"); err != nil || !ok || entry.State != StateWaiting {
				t.Fatalf("hand-in=%+v %v", entry, err)
			}
		})
	}
}

func TestSeatMergeStaleStatusKeepsCompletedRecord(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"reviewing", "done", "abandoned"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			fix := &Fix{Goal: "goal", Units: []string{"lane-merge-1"}, Commit: "base", Tip: "tip", State: "resolving", Attempt: "attempt", Paths: []conflict.Path{{Path: "code.go", Class: conflict.Builder}}}
			if err := WriteFix(root, fix); err != nil {
				t.Fatal(err)
			}
			stale, collection := *fix, *fix
			fix.State, fix.Commit, fix.Job = state, "merge", "build"
			if state == "abandoned" {
				fix.Reason = "the pending merge is gone; its resolution was abandoned"
			}
			if err := WriteFix(root, fix); err != nil {
				t.Fatal(err)
			}
			seams := ProveSeams{Git: func(string, ...string) (string, error) {
				t.Fatal("completed repair queried a pending merge")
				return "", nil
			}}
			if err := RefreshMerge(root, root, &stale, seams); err != nil {
				t.Fatal(err)
			}
			if stale.State != state || stale.Commit != "merge" {
				t.Fatalf("stale status rewrote completed repair: %+v", stale)
			}
			completionErr := CompleteMerge(t.TempDir(), root, root, &collection, "metasystem test impact", ResolveSeams{Git: seams.Git})
			if state == "abandoned" {
				if completionErr == nil || !strings.Contains(completionErr.Error(), "resolution was abandoned") {
					t.Fatalf("stale collection accepted an abandoned merge: %v", completionErr)
				}
			} else if completionErr != nil || collection.State != state {
				t.Fatalf("stale collection lost the current state: %+v %v", collection, completionErr)
			}
			if state == "reviewing" {
				if _, _, err := Start(root, root, seams); err == nil || !strings.Contains(err.Error(), "finish the resolution and its read") {
					t.Fatalf("stale status bypassed the required read: %v", err)
				}
			}
			data, err := os.ReadFile(filepath.Join(Dir(root), "fixes", fix.Attempt+".json"))
			var saved Fix
			if err != nil || json.Unmarshal(data, &saved) != nil || saved.State != state || saved.Commit != "merge" {
				t.Fatalf("completed record=%+v %v", saved, err)
			}
		})
	}
}
