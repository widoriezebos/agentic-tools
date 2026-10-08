package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

type pendingDropGit struct{}

func (pendingDropGit) Run(dir string, env []string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	body, err := cmd.Output()
	if err != nil {
		return body, fmt.Errorf("git %v: %w: %s", args, err, stderr.String())
	}
	return body, nil
}

type pendingDropProofStarter struct{ f *dropFixture }

func (s pendingDropProofStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	ref, err := (dropProofStarter{s.f}).StartSupervisor(id, state)
	r, _ := s.f.bed.manager.Store.Read(id)
	if r.Kind == "proof" {
		if e := os.WriteFile(filepath.Join(s.f.bed.worktree, "other.go"), []byte("moved during proof\n"), 0600); e != nil {
			s.f.t.Fatal(e)
		}
	}
	return ref, err
}

// Git's exact binary subtraction and inverse merge are the adapter contract.
// The public review uses the retained-check executor and goal/question owners;
// Git owns the index, commit and checkout; remote publication uses the fixture transport.
func TestWorkReviewDropsRetainedPatch(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"patch", "mixed", "staged", "red-proof", "moved-tree", "conflict", "record-repair", "lost-response", "first-review", "missing-patch"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			runRetainedPatchDrop(t, mode)
		})
	}
}

func TestWorkReviewDropCommittedRoundKeepsCleanTreeGate(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"committed-round", "committed-patch"} {
		t.Run(mode, func(t *testing.T) { t.Parallel(); runRetainedPatchDrop(t, mode) })
	}
}

func TestWorkReviewDropResumesAfterPendingRemoval(t *testing.T) {
	t.Parallel()
	runRetainedPatchDrop(t, "mixed-interrupted")
}

func runRetainedPatchDrop(t *testing.T, mode string) {
	t.Helper()

	f := newDropFixture(t)
	f.connect()
	if code, result := f.review(t); code != 1 {
		t.Fatalf("prepare: %d %+v", code, result)
	}
	adapter := pendingDropGit{}
	git := func(dir string, args ...string) []byte {
		t.Helper()
		b, err := adapter.Run(dir, nil, args...)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	writeVersion := 0
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(f.bed.worktree, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		writeVersion++
		at := f.bed.manager.Now().Add(time.Duration(writeVersion) * time.Second)
		if err := os.Chtimes(filepath.Join(f.bed.worktree, name), at, at); err != nil {
			t.Fatal(err)
		}
	}
	git(f.bed.worktree, "init", "-q", "-b", "goal/"+f.bed.id)
	git(f.bed.worktree, "config", "user.name", "fixture")
	git(f.bed.worktree, "config", "user.email", "fixture@example.invalid")
	if err := os.WriteFile(filepath.Join(f.bed.worktree, ".git", "info", "exclude"), []byte("artifacts/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	write("unit.go", "base\n")
	write("other.go", "V remains unread\n")
	write("shared.txt", "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n")
	git(f.bed.worktree, "add", "unit.go", "other.go", "shared.txt")
	git(f.bed.worktree, "commit", "-qm", "base")
	f.base = strings.TrimSpace(string(git(f.bed.worktree, "rev-parse", "HEAD")))
	committed := "base\n"
	committedRound := mode == "committed-round" || mode == "committed-patch"
	mixed := mode == "mixed" || mode == "mixed-interrupted" || committedRound
	var covered []string
	var committedPatch []byte
	if mixed {
		if mode == "committed-round" {
			write("unit.go", committed+"original\n")
			git(f.bed.worktree, "add", "unit.go")
			committedPatch = git(f.bed.worktree, "diff", "--cached", "--binary", "--full-index", "HEAD")
		}
		committed += "original\ncorrection\n"
		write("unit.go", committed)
		git(f.bed.worktree, "add", "unit.go")
		git(f.bed.worktree, "commit", "-qm", "corrected unit\n\nGoal-Unit: "+f.bed.id+"/stopped")
		covered = []string{strings.TrimSpace(string(git(f.bed.worktree, "rev-parse", "HEAD")))}
	}

	f.commit = strings.TrimSpace(string(git(f.bed.worktree, "rev-parse", "HEAD")))
	write("v.txt", "V committed\n")
	git(f.bed.worktree, "add", "v.txt")
	git(f.bed.worktree, "commit", "-qm", "other unit\n\nGoal-Unit: "+f.bed.id+"/V")
	f.v = strings.TrimSpace(string(git(f.bed.worktree, "rev-parse", "HEAD")))
	f.bed.head = f.v
	write("unit.go", committed+"pending\n")
	write("pending.bin", "\x00\x01optional\xff")
	write("shared.txt", "ONE\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n")
	git(f.bed.worktree, "add", "unit.go", "pending.bin", "shared.txt")
	patch := git(f.bed.worktree, "diff", "--cached", "--binary", "--full-index", "HEAD")
	if committedRound {
		// The final committed correction retains its result.patch after publication.
		patch = git(f.bed.worktree, "diff", "--binary", "--full-index", f.base, covered[0])
		if mode == "committed-round" {
			patch = committedPatch
		}
		write("unit.go", committed)
		if err := os.Remove(filepath.Join(f.bed.worktree, "pending.bin")); err != nil {
			t.Fatal(err)
		}
		write("shared.txt", "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n")
	}
	if mode != "staged" {
		git(f.bed.worktree, "read-tree", "HEAD")
	}
	// Another unit changes the same file outside this unit's retained hunk.
	if !committedRound {
		write("shared.txt", "ONE\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nTEN\n")
	}
	write("other.go", "V remains unread\nV pending\n")
	f.scratch = filepath.Join(t.TempDir(), "tree")
	git(f.bed.worktree, "clone", "-q", f.bed.worktree, f.scratch)
	units := f.owners.work.units
	f.owners.work.units = func(layout stateroot.Layout) *launch.UnitRunner {
		r := units(layout)
		r.Git = adapter
		return r
	}
	f.runner.Git = adapter
	// Use the production Git commit owner: the tree, commit and checkout must
	// agree, including after an interrupted installation.
	var expectedCommitTree string
	if mixed {
		expected := filepath.Join(t.TempDir(), "expected")
		git(f.bed.worktree, "clone", "-q", f.bed.worktree, expected)
		git(expected, append([]string{"revert", "--no-commit"}, covered...)...)
		expectedCommitTree = strings.TrimSpace(string(git(expected, "write-tree")))
	}
	interrupted := false
	f.owners.connection.commit = func(req branch.CommitRequest) (id string, err error) {
		before := req.BeforeCommit
		req.BeforeCommit = func(dir, parent, tree string) error {
			f.scratch = dir
			return before(dir, parent, tree)
		}
		install := req.BeforeInstall
		req.BeforeInstall = func() (func() error, error) {
			if mixed && !committedRound {
				prepared := f.retained(t).Subjects[0].Drop
				tree := strings.TrimSpace(string(git(f.scratch, "rev-parse", "HEAD^{tree}")))
				if tree != expectedCommitTree || prepared.CommitTree != expectedCommitTree {
					t.Fatalf("mixed commit includes pending work or misses covered changes: tree=%s recorded=%s want=%s", tree, prepared.CommitTree, expectedCommitTree)
				}
			}
			undo, err := install()
			if err == nil && mode == "mixed-interrupted" && !interrupted {
				interrupted = true
				panic("interrupted after pending removal")
			}
			return undo, err
		}
		defer func() {
			tip := strings.TrimSpace(string(git(f.scratch, "rev-parse", "HEAD")))
			if tip != f.v {
				f.inverse = tip
				f.commits = len(strings.Fields(string(git(f.scratch, "rev-list", f.v+"..HEAD"))))
			}
			f.bed.head = strings.TrimSpace(string(git(f.bed.worktree, "rev-parse", "HEAD")))
		}()
		id, err = branch.CommitStaged(req)
		if err == nil && f.lose {
			return "", fmt.Errorf("commit response lost after installation")
		}
		return id, err
	}

	f.owners.work.git = func(dir string, args ...string) ([]byte, error) {
		if dir != f.bed.worktree && dir != f.scratch {
			return f.raw(dir, args...)
		}
		if len(args) > 0 && args[0] == "worktree" && args[1] == "add" {
			f.scratch = args[3]
		}
		if len(args) > 0 && args[0] == "revert" {
			f.inversions++
		}
		return adapter.Run(dir, nil, args...)
	}
	// Update only the retained subject and incremental patch, through the run owner.
	r := f.retained(t)
	patchPath := filepath.Join(r.Rounds[1].Directory, "result.patch")
	if err := os.WriteFile(patchPath, patch, 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.runner.ReviewSubject(f.run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
		subject := *review.Subject
		subject.Operation = "pending-source"

		subject.Commit, subject.Tip, subject.Published = "", "", ""
		if mode == "committed-round" {
			subject.Commit, subject.Tip, subject.Published = f.commit, f.v, f.v
		}

		return retain(subject)
	}); err != nil {
		t.Fatal(err)
	}
	r = f.retained(t)
	r.Rounds[1].Result = &launch.RoundResult{Parent: f.v, PatchDigest: launch.UnitResultDigest(string(patch))}
	if mode == "first-review" {
		r.Subjects = nil
	}
	// Existing fixture writers retain synthetic run evidence without settings reads.
	transferWriteJSON(t, filepath.Join(f.bed.unitRoot, f.run, "run.json"), r)
	path := filepath.Join(r.Rounds[1].Directory, "stop-dispositions.md")
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := readReviewBinding(saved)
	if err != nil {
		t.Fatal(err)
	}
	if mode == "first-review" {
		bound.Subject = launch.UnitResultDigest(string(mustPendingFile(t, filepath.Join(r.Rounds[1].Directory, "worktree.diff"))))
		bound.Round = 2
	} else {
		bound.Subject = reviewSubjectIdentity(r.Subjects[0])
	}
	saved = []byte(strings.Replace(string(saved), strings.Split(string(saved), "\n")[2], bound.line(), 1))
	if err := os.WriteFile(path, saved, 0600); err != nil {
		t.Fatal(err)
	}
	qs, bad := channel.WalkOpenQuestions(f.bed.stateRoot())
	if len(bad) != 0 || len(qs) != 2 {
		t.Fatalf("prepared asks: %v %v", qs, bad)
	}

	if mode == "red-proof" {
		f.bed.starter.fail["proof"] = true
	}
	if mode == "moved-tree" {
		f.bed.manager.Supervisor = pendingDropProofStarter{f}
	}
	if mode == "conflict" {
		write("unit.go", "different\n")
	}

	if mode == "record-repair" {
		endpoint := f.owners.dependencies.endpoint
		lost := &lostDropOutcome{Repository: f.bed.repo}
		f.owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) { e, err := endpoint(root); e.Repository = lost; return e, err }
	}
	f.lose = mode == "lost-response"
	if mode == "missing-patch" {
		if err := os.WriteFile(patchPath, append(patch, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}

	if committedRound {
		rc, refused := f.review(t, "--dispositions", path, "--by", "Wido", "--reason", "Remove the optional work")
		if rc != 1 || !strings.Contains(strings.Join(refused.Details, " "), "owned tree must be clean") || f.retained(t).Subjects[0].Drop != nil {
			t.Fatalf("committed drop bypassed clean-tree gate: %d %+v", rc, refused)
		}
		write("other.go", "V remains unread\n")
	}
	if mode == "mixed-interrupted" {
		func() {
			defer func() {
				if got := recover(); got != "interrupted after pending removal" {
					t.Fatalf("interruption: %v", got)
				}
			}()
			f.review(t, "--dispositions", path, "--by", "Wido", "--reason", "Remove the optional work")
		}()
		if !interrupted || f.commits != 1 || f.bed.head != f.v || len(f.bed.goalFile(f.bed.id).UnitDrops) != 0 {
			t.Fatalf("interruption did not precede install: interrupted=%v commits=%d head=%s", interrupted, f.commits, f.bed.head)
		}
		if got := string(mustPendingFile(t, filepath.Join(f.bed.worktree, "unit.go"))); got != committed {
			t.Fatalf("pending patch remains: %q", got)
		}
	}
	code, result := f.review(t, "--dispositions", path, "--by", "Wido", "--reason", "Remove the optional work")
	if mode == "record-repair" || mode == "lost-response" {
		if code == 0 {
			t.Fatalf("response was not lost: %+v", result)
		}
		if mode == "record-repair" {
			if rc, synced := transferPublic(t, f.bed, f.owners, "goal", "sync", "--recover"); rc != 0 {
				t.Fatalf("sync: %d %+v", rc, synced)
			}
		}
		f.lose = false
		code, result = f.review(t, "--dispositions", path, "--by", "Wido", "--reason", "Remove the optional work")
	}

	good := mode != "red-proof" && mode != "moved-tree" && mode != "conflict" && mode != "missing-patch"
	if !good {
		if code == 0 || len(f.bed.goalFile(f.bed.id).UnitDrops) != 0 {
			t.Fatalf("unsafe drop: %d %+v", code, result)
		}
		qs, bad := channel.WalkOpenQuestions(f.bed.stateRoot())
		if len(bad) != 0 || len(qs) != 2 {
			t.Fatalf("pending asks: %v %v", qs, bad)
		}
		if mode != "conflict" {
			return
		}
		pending := f.retained(t).Subjects[0].Drop
		conflictTree := pending.Subject.GateWorktree
		write("unit.go", committed+"pending\n")
		f.scratch = filepath.Join(t.TempDir(), "recovered")
		git(f.bed.worktree, "clone", "-q", f.bed.worktree, f.scratch)
		code, result = f.review(t, "--dispositions", path, "--by", "Wido", "--reason", "Remove the optional work")
		pending = f.retained(t).Subjects[0].Drop
		if len(pending.ConflictTrees) != 1 || pending.ConflictTrees[0] != conflictTree {
			t.Fatalf("lost conflict evidence: %+v", pending)
		}
	}
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("drop: %d %+v", code, result)
	}
	outcome := f.bed.goalFile(f.bed.id).UnitDrops
	wantCommits := 0
	if mixed {
		wantCommits = 1
	}
	if len(outcome) != 1 || f.checks != 1 || f.commits != wantCommits || outcome[0].PatchDigest != func() string {
		if committedRound {
			return ""
		}
		return launch.UnitResultDigest(string(patch))
	}() || outcome[0].Actor != "Wido" || outcome[0].Reason != "Remove the optional work" || outcome[0].Impact == "" {
		t.Fatalf("outcome=%+v checks=%d commits=%d", outcome, f.checks, f.commits)
	}
	if strings.Join(outcome[0].Covered, ",") != strings.Join(covered, ",") {
		t.Fatalf("covered: %v want %v", outcome[0].Covered, covered)
	}

	if mixed {
		actual := strings.TrimSpace(string(git(f.bed.worktree, "rev-parse", "HEAD^{tree}")))
		if actual != expectedCommitTree || !committedRound && outcome[0].CommitTree != expectedCommitTree {
			t.Fatalf("mixed commit includes pending work or misses covered changes: tree=%s recorded=%s want=%s", actual, outcome[0].CommitTree, expectedCommitTree)
		}
	}
	wants := map[string]string{"unit.go": "base\n", "other.go": "V remains unread\nV pending\n", "shared.txt": "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nTEN\n"}
	if committedRound {
		wants["other.go"] = "V remains unread\n"
		wants["shared.txt"] = "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"
	}
	for name, want := range wants {
		body, err := os.ReadFile(filepath.Join(f.bed.worktree, name))
		if err != nil || string(body) != want {
			t.Fatalf("%s=%q want %q: %v", name, body, want, err)
		}
	}
	if _, err := os.Stat(filepath.Join(f.bed.worktree, "pending.bin")); !os.IsNotExist(err) {
		t.Fatalf("optional binary remains: %v", err)
	}
	original, err := os.ReadFile(patchPath)
	if err != nil || !bytes.Equal(original, patch) {
		t.Fatalf("retained patch changed: %v", err)
	}
	qs, bad = channel.WalkQuestions(f.bed.stateRoot())
	if len(bad) != 0 || len(qs) != 2 {
		t.Fatalf("asks: %v %v", qs, bad)
	}
	for _, q := range qs {
		if q.State != "closed" || q.UnitStop.ClosedBy != outcome[0].Operation {
			t.Fatalf("ask closed by another act: %+v", q)
		}
	}
	if committedRound {
		return
	}
	code, result = f.review(t, "--dispositions", path, "--by", "Wido", "--reason", "Remove the optional work")
	if code != 0 || f.checks != 1 || f.commits != wantCommits {
		t.Fatalf("replay: %d %+v", code, result)
	}
}

func mustPendingFile(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
