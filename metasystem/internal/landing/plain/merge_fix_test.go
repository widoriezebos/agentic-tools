package plain

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Git's index, unstaged bytes and merge parents are the collection's observable result.
func TestACollectionIgnoresOnlyCoordinationOutsideTheMerge(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name         string
		paths        []string
		goalNarrator bool
		nilPolicy    bool
		refused      bool
	}{
		{name: "shared records", paths: []string{"records/narrator-digest.log", "memory/receipts.log"}},
		{name: "review record", paths: []string{"records/reads/x.json"}, refused: true},
		{name: "merged coordination", paths: []string{"records/narrator-digest.log"}, goalNarrator: true, refused: true},
		{name: "product edit", paths: []string{"out/other"}, refused: true},
		{name: "no policy", paths: []string{"records/narrator-digest.log", "memory/receipts.log"}, nilPolicy: true, refused: true},
		{name: "coordination directory", paths: []string{"plans/goals/x.md"}},
		{name: "directory lookalike", paths: []string{"plans/goalsX/x.md"}, refused: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := newResolveGitAdapterFixture(t, false)
			b.mustGit("merge", "--abort")
			b.mustGit("checkout", "--detach", b.mustGit("merge-base", "main", "goal"))
			write := func(path, content string) {
				t.Helper()
				path = filepath.Join(b.install, path)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			for _, path := range []string{"records/narrator-digest.log", "memory/receipts.log", "records/reads/x.json", "out/other", "plans/goals/x.md", "plans/goalsX/x.md"} {
				write(path, "base\n")
			}
			b.mustGit("add", ".")
			b.mustGit("commit", "--quiet", "-m", "coordination base")
			base := b.mustGit("rev-parse", "HEAD")
			b.mustGit("checkout", "-B", "goal")
			write("out/conflict", "goal\n")
			if row.goalNarrator {
				write("records/narrator-digest.log", "goal record\n")
			}
			b.mustGit("commit", "--quiet", "-am", "goal")
			tip := b.mustGit("rev-parse", "HEAD")
			b.mustGit("checkout", "-B", "main", base)
			write("out/conflict", "main\n")
			b.mustGit("commit", "--quiet", "-am", "main")
			main := b.mustGit("rev-parse", "HEAD")
			b.mustGit("update-ref", "refs/remotes/origin/main", main)
			if _, err := b.seams.Git(b.checkout, "merge", "--no-commit", "goal"); err == nil {
				t.Fatal("fixture did not conflict")
			}
			if err := os.Remove(queuePath(b.install)); err != nil {
				t.Fatal(err)
			}
			if _, _, err := HandIn(b.install, Line{At: bedNow.Format(time.RFC3339), Goal: "goal", SHA: tip}); err != nil {
				t.Fatal(err)
			}
			b.contract.Generated, b.seams.AsSeat = nil, true
			if out, err := b.resolve(); err != nil || out.Outcome != "resolving" {
				t.Fatalf("take conflict: %+v %v", out, err)
			}
			fix, err := ReadFix(b.install)
			if err != nil || fix == nil {
				t.Fatalf("fix: %+v %v", fix, err)
			}
			write("out/conflict", "main\ngoal\n")
			b.mustGit("add", "metasystem/out/conflict")
			if !row.nilPolicy {
				b.seams.Coordination = []string{"memory/receipts.log", "records/narrator-digest.log", "plans/goals", "plans/goals-accepted.json", "plans/goals.md", "records/counselor", "records/goals"}
			}
			for _, path := range row.paths {
				write(path, "unstaged coordination or edit\n")
			}
			b.seams.Run = func(_ []string, _ string, _ *os.File, started func(int64) error) error { return started(0) }
			err = CompleteMerge(b.home, b.install, b.checkout, fix, "metasystem test impact", b.seams)
			if row.refused {
				if !errors.Is(err, ErrUnstaged) {
					t.Fatalf("collection should refuse unstaged edits: %v", err)
				}
				for _, path := range row.paths {
					if !strings.Contains(err.Error(), "metasystem/"+path) {
						t.Fatalf("refusal omitted %s: %v", path, err)
					}
				}
				if got := b.mustGit("rev-parse", "HEAD"); got != main {
					t.Fatalf("refused collection moved HEAD: %s", got)
				}
			} else {
				if err != nil {
					t.Fatalf("collect shared coordination: %v", err)
				}
				if got := b.mustGit("show", "-s", "--format=%P", "HEAD"); got != main+" "+tip {
					t.Fatalf("merge parents=%s", got)
				}
			}
			dirty := nulPaths(b.mustGit("diff", "--name-only", "-z", "--"))
			if len(dirty) != len(row.paths) {
				t.Fatalf("unstaged paths=%q; want %q", dirty, row.paths)
			}
			for _, path := range row.paths {
				if data, err := os.ReadFile(filepath.Join(b.install, path)); err != nil || string(data) != "unstaged coordination or edit\n" {
					t.Fatalf("unstaged %s=%q: %v", path, data, err)
				}
				if !row.refused && b.mustGit("show", "HEAD:metasystem/"+path) != "base" {
					t.Fatalf("collection committed unstaged %s", path)
				}
			}
		})
	}
}

// Git's pending merge, parent topology and hook admission cannot be proved by a Git stub.
func TestGitAdapterSeatMergeCompletesAndProves(t *testing.T) {
	t.Parallel()
	for _, generated := range []bool{false, true} {
		t.Run(map[bool]string{false: "source", true: "generated"}[generated], func(t *testing.T) {
			t.Parallel()
			b := newResolveGitAdapterFixture(t, false)
			main, tip := b.mustGit("rev-parse", "HEAD"), b.mustGit("rev-parse", "MERGE_HEAD")
			if err := os.Remove(queuePath(b.install)); err != nil {
				t.Fatal(err)
			}
			if _, _, err := HandIn(b.install, Line{At: bedNow.Format(time.RFC3339), Goal: "goal", SHA: tip}); err != nil {
				t.Fatal(err)
			}
			if !generated {
				b.contract.Generated = nil
			}
			b.seams.AsSeat = true
			out, err := b.resolve()
			if err != nil || out.Outcome != "resolving" || len(out.Conflict.Paths) != 1 || b.mustGit("rev-parse", "MERGE_HEAD") != tip {
				t.Fatalf("take conflict: %+v %v", out, err)
			}
			fix, err := ReadFix(b.install)
			if err != nil || fix == nil || fix.State != "resolving" {
				t.Fatalf("fix: %+v %v", fix, err)
			}
			fix.Job = "fake-build-1"
			if err := WriteFix(b.install, fix); err != nil {
				t.Fatal(err)
			}
			build := func() {
				if !generated {
					if err := os.WriteFile(filepath.Join(b.install, "out/conflict"), []byte("main\ngoal\n"), 0o644); err != nil {
						t.Fatal(err)
					}
					b.mustGit("add", "metasystem/out/conflict")
				} else {
					// Even a builder that stages an output cannot substitute it for regeneration.
					b.mustGit("restore", "--source=HEAD", "--staged", "--worktree", "--", "metasystem/out/conflict")
				}
			}
			build()
			batch := &Batch{ID: "batch", Base: main, State: BatchRunning, Lane: lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1}, Members: []GoalSHA{{Goal: "goal", SHA: tip}}}
			if err := writeBatch(b.install, batch); err != nil {
				t.Fatal(err)
			}
			git := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				if args[0] == "commit" {
					if !strings.HasSuffix(args[2], "\n\nGoal-Unit: goal/lane-merge-1") {
						t.Fatalf("engine omitted merge trailer: %q", args[2])
					}

				}
				return git(dir, args...)
			}
			generator := b.seams.Run
			checks := 0
			b.seams.Run = func(argv []string, dir string, log *os.File, started func(int64) error) error {
				if reflect.DeepEqual(argv, []string{"/usr/bin/env", "LANDING_PROOF_BASE=" + main, "/bin/sh", "-c", "metasystem test impact"}) {
					checks++
					checked := *fix
					checked.CheckMinutes, checked.CheckResult = 2.28, "static-only"
					if err := WriteFix(b.install, &checked); err != nil {
						return err
					}
					return started(0)
				}
				return generator(argv, dir, log, started)
			}
			b.seams.AsSeat = false
			if err := CompleteMerge(b.home, b.install, b.checkout, fix, "metasystem test impact", b.seams); err != nil {
				t.Fatal(err)
			}
			if checks != 1 || (generated && b.runs != 1) {
				t.Fatalf("checks=%d generators=%d", checks, b.runs)
			}
			checked, err := FixForAttempt(b.install, fix.Attempt)
			if err != nil || checked.CheckResult != "static-only" || checked.CheckMinutes != 2.28 {
				t.Fatalf("merge completion lost the check: %+v error=%v", checked, err)
			}
			if got := b.mustGit("show", "-s", "--format=%P", "HEAD"); got != main+" "+tip {
				t.Fatalf("parents=%s", got)
			}
			if got := b.mustGit("show", "-s", "--format=%B", "HEAD"); !strings.Contains(got, "Goal-Unit: goal/lane-merge-1") {
				t.Fatalf("message=%s", got)
			}
			want := "main\ngoal\n"
			if generated {
				want = "regenerated\n"
			}
			if data, err := os.ReadFile(filepath.Join(b.install, "out/conflict")); err != nil || string(data) != want {
				t.Fatalf("resolution=%q %v", data, err)
			}
			if _, err := CheckBatch(b.install, b.checkout, fix.Commit, main, false, ProveSeams{Git: git}); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if _, err := Run(b.install, b.checkout, "exit 0", "", &output, ProveSeams{Git: git}); err == nil || !strings.Contains(err.Error(), "finish the resolution and its read") {
				t.Fatalf("unread merge admitted: %v", err)
			}
			fix.State = "done"
			if err := WriteFix(b.install, fix); err != nil {
				t.Fatal(err)
			}
			proof, err := Run(b.install, b.checkout, "exit 0", "", &output, ProveSeams{Git: git})
			if err != nil || proof.Result != Green || proof.Commit != fix.Commit {
				t.Fatalf("proof=%+v err=%v output=%s", proof, err, output.String())
			}
		})
	}
}

func TestSeatMergeCannotResolveReturnsPathsAndJob(t *testing.T) {
	t.Parallel()
	b := newResolveFixture(t)
	b.paths = "metasystem/src/file.go\x00"
	b.seams.AsSeat = true
	if out, err := b.resolve(); err != nil || out.Outcome != "resolving" || len(b.writes) != 0 {
		t.Fatalf("first attempt: %+v %v writes=%v", out, err, b.writes)
	}
	fix, err := ReadFix(b.install)
	if err != nil {
		t.Fatal(err)
	}
	fix.Job = "cannot-resolve-job"
	if err := WriteFix(b.install, fix); err != nil {
		t.Fatal(err)
	}
	b.seams.AsSeat = false
	out, err := b.resolve()
	if err != nil || out.Outcome != "returned" || !strings.Contains(out.Reason, "cannot-resolve-job") || out.Conflict.Paths[0].Path != "metasystem/src/file.go" {
		t.Fatalf("fallback: %+v %v", out, err)
	}
	if !reflect.DeepEqual(b.writes, [][]string{{"merge", "--abort"}}) {
		t.Fatalf("fallback writes=%v", b.writes)
	}
	if fix, err := ReadFix(b.install); err != nil || fix != nil {
		t.Fatalf("returned job still resolving: %+v %v", fix, err)
	}
	if _, _, err := HandIn(b.install, Line{At: bedNow.Format(time.RFC3339), Goal: "goal", SHA: "goal-sha", Again: true}); err != nil {
		t.Fatal(err)
	}
	b.seams.AsSeat = true
	b.seams.Proof.NewID = func() string { return "next-attempt" }
	if _, err := b.resolve(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(Dir(b.install), "fixes", fix.Attempt+".json"))
	var previous Fix
	if err != nil || json.Unmarshal(data, &previous) != nil || previous.State != "returned" || previous.Job != "cannot-resolve-job" {
		t.Fatalf("next attempt replaced its predecessor: %+v %v", previous, err)
	}
}

func TestGitAdapterSeatMergeRefusesUnresolvedSource(t *testing.T) {
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
	err = CompleteMerge(b.home, b.install, b.checkout, fix, "metasystem test impact", b.seams)
	if err == nil || !strings.Contains(err.Error(), "source conflicts") {
		t.Fatalf("unresolved source accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(b.checkout, ".git/MERGE_HEAD")); err != nil {
		t.Fatal(err)
	}
}

func TestGitAdapterSeatMergeWithBatchMember(t *testing.T) {
	t.Parallel()
	for _, resolved := range []bool{true, false} {
		t.Run(map[bool]string{true: "resolved", false: "cannot resolve"}[resolved], func(t *testing.T) {
			t.Parallel()
			b := newResolveGitAdapterFixture(t, false)
			b.mustGit("merge", "--abort")
			member, tip := b.mustGit("rev-parse", "HEAD"), b.mustGit("rev-parse", "goal")
			base := b.mustGit("merge-base", member, tip)
			b.mustGit("update-ref", "refs/remotes/origin/main", base)
			b.mustGit("checkout", "--detach", base)
			b.mustGit("merge", "--no-ff", "--no-edit", member)
			if _, err := b.seams.Git(b.checkout, "merge", "--no-commit", tip); err == nil {
				t.Fatal("fixture did not conflict with batch member")
			}
			if err := os.Remove(queuePath(b.install)); err != nil {
				t.Fatal(err)
			}
			for _, line := range []Line{{At: bedNow.Format(time.RFC3339), Goal: "earlier", SHA: member}, {At: bedNow.Format(time.RFC3339), Goal: "goal", SHA: tip}} {
				if _, _, err := HandIn(b.install, line); err != nil {
					t.Fatal(err)
				}
			}
			batch := &Batch{ID: "batch", Base: base, State: BatchRunning, Lane: lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1}, Members: []GoalSHA{{Goal: "earlier", SHA: member}, {Goal: "goal", SHA: tip}}}
			if err := writeBatch(b.install, batch); err != nil {
				t.Fatal(err)
			}
			b.contract.Generated, b.seams.AsSeat = nil, true
			if out, err := b.resolve(); err != nil || out.Outcome != "resolving" || out.Held {
				t.Fatalf("take batch conflict=%+v %v", out, err)
			}
			fix, err := ReadFix(b.install)
			if err != nil {
				t.Fatal(err)
			}
			fix.Job = "batch-merge-job"
			if err := WriteFix(b.install, fix); err != nil {
				t.Fatal(err)
			}
			b.seams.AsSeat = false
			if !resolved {
				out, err := b.resolve()
				if err != nil || !out.Held || out.Entry == nil || !reflect.DeepEqual(out.Entry.After, []GoalSHA{{Goal: "earlier", SHA: member}}) || !strings.Contains(out.Reason, fix.Job) || len(out.Conflict.Paths) != 1 {
					t.Fatalf("fallback=%+v %v", out, err)
				}
				entry, _, err := Latest(b.install, "goal")
				if err != nil || entry.Conflict == nil || len(entry.Conflict.Paths) != 1 {
					t.Fatalf("held paths=%+v %v", entry, err)
				}
				return
			}
			if err := os.WriteFile(filepath.Join(b.install, "out/conflict"), []byte("main\ngoal\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			b.mustGit("add", "metasystem/out/conflict")
			b.seams.Run = func(_ []string, _ string, _ *os.File, started func(int64) error) error { return started(0) }
			if err := CompleteMerge(b.home, b.install, b.checkout, fix, "metasystem test impact", b.seams); err != nil {
				t.Fatal(err)
			}
			if _, err := CheckBatch(b.install, b.checkout, fix.Commit, base, false, ProveSeams{Git: b.seams.Git}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGitAdapterSeatMergeGeneratorFailureKeepsPendingMergeForReturn(t *testing.T) {
	t.Parallel()
	b := newResolveGitAdapterFixture(t, false)
	b.seams.AsSeat = true
	if _, err := b.resolve(); err != nil {
		t.Fatal(err)
	}
	fix, err := ReadFix(b.install)
	if err != nil {
		t.Fatal(err)
	}
	fix.Job = "failed-generator-job"
	if err := WriteFix(b.install, fix); err != nil {
		t.Fatal(err)
	}
	b.seams.Run = func(_ []string, _ string, _ *os.File, started func(int64) error) error {
		if err := started(0); err != nil {
			return err
		}
		return errors.New("generator stopped")
	}
	if err := CompleteMerge(b.home, b.install, b.checkout, fix, "metasystem test impact", b.seams); err == nil {
		t.Fatal("generator failure committed")
	}
	if _, err := os.Stat(resolveBegunPath(b.install)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed regeneration retained begun record: %v", err)
	}
	if b.mustGit("rev-parse", "HEAD") != fix.Commit || b.mustGit("rev-parse", "MERGE_HEAD") != fix.Tip {
		t.Fatal("generator failure lost the merge")
	}
	b.seams.AsSeat = false
	out, err := b.resolve()
	if err != nil || out.Outcome != "returned" || !strings.Contains(out.Reason, fix.Job) {
		t.Fatalf("fallback=%+v %v", out, err)
	}
	if dirty := b.mustGit("status", "--porcelain"); dirty != "" {
		t.Fatalf("fallback left generated changes: %s", dirty)
	}
}
