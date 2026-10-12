package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func mainMoveFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "main-move", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestLandingMainMoveClassificationStopsAtCodeAndPreservesErrors(t *testing.T) {
	t.Parallel()
	for _, first := range []string{"code", "error", "records"} {
		t.Run(first, func(t *testing.T) {
			t.Parallel()
			scans := 0
			broken := errors.New("cannot read commit paths")
			seams := ProveSeams{Git: func(_ string, args ...string) (string, error) {
				switch args[0] {
				case "rev-list":
					if args[1] == "--first-parent" {
						return "batch base", nil
					}
					return "first second", nil
				case "cat-file", "merge-base":
					return "", nil
				case "diff-tree":
					scans++
					if first == "error" || scans == 2 {
						return "", broken
					}
					if first == "code" {
						return "metasystem/internal/code.go\x00", nil
					}
					return "metasystem/plans/goals/backlog.md\x00", nil
				default:
					t.Fatalf("unexpected Git: %v", args)
					return "", nil
				}
			}}
			move, err := mainMove("fixture", "batch", "main", seams)
			if first == "code" {
				if err != nil || move.Kind != "code" || move.Count != 2 || scans != 1 {
					t.Fatalf("code classification scanned further: %+v scans=%d error=%v", move, scans, err)
				}
			} else if !errors.Is(err, broken) || move.Kind != "none" {
				t.Fatalf("failed scan reported a records-only move: %+v error=%v", move, err)
			}
		})
	}
}

// Unrelated Git history must give the same actionable refusal as a push
// that would rewrite main, rather than an unclassified engine error.
func TestLandingMainMoveGitAdapterRewrittenMainRefuses(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	sha := b.seat("seat-a", "goal-a")
	b.handIn("seat-a", "goal-a", sha)
	batch := b.merge("goal-a")
	tree := b.git(b.checkout, "rev-parse", "HEAD^{tree}")
	main := b.git(b.checkout, "commit-tree", tree, "-m", "unrelated main history")
	move, err := mainMove(b.checkout, batch, main, ProveSeams{})
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.Code != CodeNotFastForward || !strings.Contains(refusal.Next, "rebuild") || move.Kind != "none" || b.git(b.checkout, "rev-parse", "HEAD") != batch {
		t.Fatalf("rewritten main: %+v error=%v", move, err)
	}
}

// Git must actually merge and publish the diverged histories; a fake cannot
// prove the resulting parents, tree or remote reference.
func TestLandingMainMoveGitAdapterFoldPush(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	sha := b.seat("seat-a", "goal-a")
	b.handIn("seat-a", "goal-a", sha)
	proven := b.merge("goal-a")
	var batch Batch
	if err := json.Unmarshal(mainMoveFixture(t, "batch.json"), &batch); err != nil {
		t.Fatal(err)
	}
	batch.Lane.Root, batch.Lane.Install = b.checkout, b.install
	batch.Base, batch.State = b.originMain(), BatchRunning
	batch.Members = []GoalSHA{{Goal: "goal-a", SHA: sha}}
	if err := writeBatch(b.install, &batch); err != nil {
		t.Fatal(err)
	}
	var result Result
	if err := json.Unmarshal(mainMoveFixture(t, "results.jsonl"), &result); err != nil {
		t.Fatal(err)
	}
	result.Trunk, result.Commit, result.Tree = false, proven, b.git(b.checkout, "rev-parse", "HEAD^{tree}")
	result.BatchID, result.BatchMembers, result.Goals = batch.ID, batch.Members, batch.Members
	result.Reason = "flaky TestFoo; fix goal fix-flaky-foo"
	if err := appendLine(resultsPath(b.install), result); err != nil {
		t.Fatal(err)
	}
	seed := filepath.Join(b.root, "seed")
	var folded []string
	for _, goal := range []string{"lane-first-run-fixes", "lane-rehearsal-before-landing"} {
		b.write(filepath.Join(seed, "metasystem/plans/goals", goal+".md"), string(mainMoveFixture(t, goal+".md")))
		b.write(filepath.Join(seed, "metasystem/plans/goals/backlog.md"), string(mainMoveFixture(t, "backlog.md")))
		b.git(seed, "add", "metasystem/plans/goals")
		b.git(seed, "commit", "-qm", "goal slice-start "+goal)
		folded = append(folded, b.git(seed, "rev-parse", "HEAD"))
	}
	b.git(seed, "push", "--quiet", "origin", "main")
	refused := errors.New("design needs a decision")
	out, err := PushChecked(b.install, b.checkout, bedNow, func(_, candidate string) error {
		if candidate == proven {
			t.Fatal("publication checked the unfolded candidate")
		}
		if _, _, err := HandIn(b.install, Line{Goal: "goal-a", SHA: sha}); err != nil {
			return err
		}
		return refused
	})
	if !errors.Is(err, refused) || out.Changed || b.originMain() != folded[len(folded)-1] {
		t.Fatalf("refused folded publication: %+v %v", out, err)
	}
	out, err = PushChecked(b.install, b.checkout, bedNow, nil)

	if err != nil || !out.Changed {
		t.Fatalf("records-only push: %+v %v", out, err)
	}
	parents := strings.Fields(b.git(b.checkout, "show", "-s", "--format=%P", out.Commit))
	if !reflect.DeepEqual(parents, []string{proven, folded[len(folded)-1]}) || b.originMain() != out.Commit {
		t.Fatalf("fold parents=%v remote=%s", parents, b.originMain())
	}
	inherited, ok, err := ResultFor(b.install, out.Tree)
	if err != nil || !ok || !strings.Contains(inherited.Reason, result.Reason) || inherited.FullAt != result.FullAt || inherited.CountedFull {
		t.Fatalf("lost proof provenance: %+v %v", inherited, err)
	}
	push, ok, err := LastPush(b.install)
	if err != nil || !ok || !reflect.DeepEqual(push.FoldedCommits, folded) {
		t.Fatalf("folded push record: %+v %v", push, err)
	}
	if !ledgerOnlyBetween(Git, b.checkout, result.Tree, out.Tree) {
		t.Fatal("fold changed proven code")
	}
}

// The checkout has a real conflicted index and MERGE_HEAD; Git is injected
// so the resolver's job and generated-file seams remain under test control.
func TestLandingMainMoveGitAdapterPendingResolution(t *testing.T) {
	t.Parallel()
	b := newResolveGitAdapterFixture(t, false)
	b.contract.Generated = nil
	b.seams.AsSeat = true
	if _, err := b.resolve(); err != nil {
		t.Fatal(err)
	}
	fix, err := ReadFix(b.install)
	if err != nil || fix == nil {
		t.Fatalf("fix=%+v %v", fix, err)
	}
	origin := filepath.Join(t.TempDir(), "origin.git")
	b.mustGit("init", "--bare", "--quiet", "-b", "main", origin)
	b.mustGit("remote", "add", "origin", origin)
	b.mustGit("push", "--quiet", "origin", "main")
	seed := t.TempDir()
	if _, err := b.seams.Git(seed, "clone", "--quiet", origin, "."); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"lane-first-run-fixes.md", "backlog.md"} {
		path := filepath.Join(seed, "metasystem/plans/goals", name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, mainMoveFixture(t, name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-qm", "goal slice-start lane-first-run-fixes"}, {"push", "--quiet", "origin", "main"}} {
		if _, err := b.seams.Git(seed, args...); err != nil {
			t.Fatal(err)
		}
	}
	b.mustGit("fetch", "--quiet", "origin")
	record := lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1}
	batch := &Batch{ID: "pending", Base: fix.Commit, Lane: record, State: BatchRunning, Members: []GoalSHA{{Goal: fix.Goal, SHA: fix.Tip}}}
	if err := writeBatch(b.install, batch); err != nil {
		t.Fatal(err)
	}
	status := ReadStatus(b.home, record, lane.View{Root: &b.checkout}, ProveSeams{Git: b.seams.Git})
	if status.MainMove.Kind != "records-only" || status.MainMove.Count != 1 || status.RunningFix == nil || status.RunningFix.State != "resolving" || !strings.Contains(status.Summary, "main gained 1 record commits since the batch base; the batch stands") {
		t.Fatalf("record movement lost pending resolution: %+v", status)
	}
	if b.mustGit("rev-parse", "MERGE_HEAD") != fix.Tip || b.mustGit("rev-parse", "HEAD") != fix.Commit {
		t.Fatal("status dropped the merge")
	}
	stored, err := FixForAttempt(b.install, fix.Attempt)
	if err != nil || stored.State != "resolving" {
		t.Fatalf("resolution closed: %+v %v", stored, err)
	}
	data, err := json.Marshal(status)
	if err != nil || !strings.Contains(string(data), `"main_move":{"kind":"records-only","count":1`) {
		t.Fatalf("status JSON=%s %v", data, err)
	}
	b.seams.Run = func(_ []string, _ string, _ *os.File, started func(int64) error) error { return started(0) }
	fix.Job = "resolution-job"
	if err := WriteFix(b.install, fix); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.install, "out/conflict"), []byte("main\ngoal\n"), 0600); err != nil {
		t.Fatal(err)
	}
	b.mustGit("add", "metasystem/out/conflict")
	if err := CompleteMerge(b.home, b.install, b.checkout, fix, "metasystem test impact", b.seams); err != nil {
		t.Fatal(err)
	}
	status = ReadStatus(b.home, record, lane.View{Root: &b.checkout}, ProveSeams{Git: b.seams.Git})
	if status.MainMove.Kind != "records-only" || status.RunningFix == nil || status.RunningFix.State != "reviewing" || status.RunningFix.Job != "resolution-job" {
		t.Fatalf("record move invalidated pending read: %+v", status)
	}
	if err := proofFixReady(b.install, ProveSeams{}); err == nil {
		t.Fatal("record move bypassed the merge read")
	}

}

// Real Git ancestry distinguishes the original base from main already rebuilt on.
func TestLandingMainMoveGitAdapterClassification(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"metasystem/metasystem.conf", "metasystem/testing.json", "metasystem/internal/code.go", "metasystem/plans/README.md", "metasystem/records/reads/read.json", "metasystem/plans/goals/backlog.md", "rebuilt code", "reverted code"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			b := newBed(t)
			original := b.originMain()
			sha := b.seat("seat-a", "goal-a")
			b.handIn("seat-a", "goal-a", sha)
			seed := filepath.Join(b.root, "seed")
			if path == "rebuilt code" {
				b.write(filepath.Join(seed, "metasystem/internal/code.go"), "package code\n")
				b.git(seed, "add", ".")
				b.git(seed, "commit", "-qm", "code already rebuilt on")
				b.git(seed, "push", "--quiet", "origin", "main")
			}
			commit := b.merge("goal-a")
			batch := &Batch{ID: "batch", Base: original, State: BatchRunning, Lane: lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1}, Members: []GoalSHA{{Goal: "goal-a", SHA: sha}}}
			if err := writeBatch(b.install, batch); err != nil {
				t.Fatal(err)
			}
			move, err := mainMove(b.checkout, commit, b.originMain(), ProveSeams{})
			if err != nil || move.Kind != "none" || move.Count != 0 {
				t.Fatalf("unchanged main: %+v %v", move, err)
			}
			changed := path
			if path == "rebuilt code" {
				changed = "metasystem/plans/goals/backlog.md"
			}
			if path == "reverted code" {
				changed = "metasystem/internal/code.go"
			}
			b.write(filepath.Join(seed, changed), "changed\n")
			b.git(seed, "add", ".")
			b.git(seed, "commit", "-qm", "advance main")
			want, count := "code", 1
			if path == "reverted code" {
				b.git(seed, "revert", "--no-edit", "HEAD")
				count = 2
			}
			if path == "rebuilt code" || path == "metasystem/plans/goals/backlog.md" {
				want = "records-only"
			}
			b.git(seed, "push", "--quiet", "origin", "main")
			b.git(b.checkout, "fetch", "--quiet", "origin")
			move, err = mainMove(b.checkout, commit, b.originMain(), ProveSeams{})
			if err != nil || move.Kind != want || move.Count != count {
				t.Fatalf("classification: %+v %v want %s/%d", move, err, want, count)
			}
			status := ReadStatus(t.TempDir(), batch.Lane, lane.View{Root: &b.checkout}, ProveSeams{})
			if status.MainMove.Kind != want || (want == "code" && !strings.Contains(status.Summary, "main moved: code change; run: metasystem landing resume")) {
				t.Fatalf("status=%+v", status)
			}
			if want == "code" {
				tree := b.git(b.checkout, "rev-parse", "HEAD^{tree}")
				if err := appendLine(resultsPath(b.install), Result{Commit: commit, Tree: tree, Result: Green}); err != nil {
					t.Fatal(err)
				}
				out, err := PushChecked(b.install, b.checkout, bedNow, nil)
				var refusal *Refusal
				if !errors.As(err, &refusal) || refusal.Code != CodeNotFastForward || out.Changed || b.git(b.checkout, "rev-parse", "HEAD") != commit {
					t.Fatalf("code move pushed: %+v %v", out, err)
				}
			}
		})
	}
}

func TestLandingMainMoveFoldRefusesConflictOrUnitDifference(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"conflict", "unit difference", "dirty index", "committed unit difference"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			pushes, commits, checkouts := 0, 0, 0
			checks := 0
			seams := ProveSeams{Git: func(_ string, args ...string) (string, error) {
				switch args[0] {
				case "merge-tree":
					if !reflect.DeepEqual(args, []string{"merge-tree", "--write-tree", "fold", "new-main"}) {
						t.Fatalf("merge plumbing: %v", args)
					}
					if failure == "conflict" {
						return "", errors.New("record conflict")
					}
					return "folded-tree", nil
				case "diff":
					if args[1] == "--quiet" || args[1] == "--cached" {
						if failure == "dirty index" {
							return "", errors.New("dirty")
						}
						return "", nil
					}
					checks++
					if failure == "unit difference" || failure == "committed unit difference" && checks > 1 {
						return "metasystem/internal/code.go", nil
					}
					return "metasystem/plans/goals/backlog.md", nil
				case "commit-tree":
					commits++
					return "fold", nil
				case "checkout":
					checkouts++
				case "push":
					pushes++
				case "rev-parse":
					if strings.Contains(args[2], "tree") {
						return "folded-tree", nil
					}
					return "fold", nil
				default:
					t.Fatalf("unexpected Git %v", args)
				}
				return "", nil
			}}
			_, _, err := foldRecords(install, install, "new-main", "proven-tree", MainMove{Kind: "records-only", Commits: []string{"record"}}, seams)
			var refusal *Refusal
			if !errors.As(err, &refusal) || refusal.Code != CodeNotFastForward || pushes != 0 || !strings.Contains(refusal.Next, "rebuild") {
				t.Fatalf("fold refusal %v pushes=%d", err, pushes)
			}
			if failure == "conflict" || failure == "unit difference" {
				if commits != 0 || checkouts != 0 {
					t.Fatalf("refused fold commits=%d checkouts=%d", commits, checkouts)
				}
			}
		})
	}
}

// A real main advance during the detached proof must leave that exact proof
// running, then permit its green tree to authorize the record merge.
func TestLandingMainMoveGitAdapterRunningProof(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	sha := b.seat("seat-a", "goal-a")
	b.handIn("seat-a", "goal-a", sha)
	commit := b.merge("goal-a")
	record := lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1}
	batch := &Batch{ID: "proof", Lane: record, Base: b.originMain(), State: BatchRunning, Members: []GoalSHA{{Goal: "goal-a", SHA: sha}}}
	if err := writeBatch(b.install, batch); err != nil {
		t.Fatal(err)
	}
	observed := false
	seams := ProveSeams{Now: func() time.Time { return bedNow }, NewID: func() string { return "running-proof" }, Command: func(command *exec.Cmd) error {
		seed := filepath.Join(b.root, "seed")
		b.write(filepath.Join(seed, "metasystem/plans/goals/backlog.md"), string(mainMoveFixture(t, "backlog.md")))
		b.git(seed, "add", ".")
		b.git(seed, "commit", "-qm", "goal claim")
		b.git(seed, "push", "--quiet", "origin", "main")
		b.git(b.checkout, "fetch", "--quiet", "origin")
		status := ReadStatus(t.TempDir(), record, lane.View{Root: &b.checkout}, ProveSeams{Alive: func(Running) bool { return true }, Now: func() time.Time { return bedNow }})
		if status.MainMove.Kind != "records-only" || status.RunningProof == nil || status.RunningProof.Commit != commit || status.RunningProof.State != "running" {
			t.Fatalf("main move voided running proof: %+v", status)
		}
		observed = true
		fmt.Fprintln(command.Stdout, "LANDING-CHECKED\t0")
		return nil
	}}
	result, err := Run(b.install, b.checkout, "fixture", "", io.Discard, seams)
	if err != nil || !observed || result.Result != Green || result.Commit != commit {
		t.Fatalf("proof after main advance: %+v %v", result, err)
	}
	out, err := PushChecked(b.install, b.checkout, bedNow, nil)
	if err != nil || !out.Changed {
		t.Fatalf("push running proof's green: %+v %v", out, err)
	}
}

// A selected goal parent can reach main independently. It remains a goal
// parent of the batch; it cannot replace the batch's main parent.
func TestLandingMainMoveGitAdapterSelectedParentsAreNotMainBase(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	base := b.originMain()
	members := []GoalSHA{}
	for _, goal := range []string{"goal-a", "goal-b"} {
		sha := b.seat("seat-"+goal, goal)
		b.handIn("seat-"+goal, goal, sha)
		members = append(members, GoalSHA{Goal: goal, SHA: sha})
	}
	commit := b.merge("goal-a", "goal-b")
	seed := filepath.Join(b.root, "seed")
	b.git(seed, "fetch", "--quiet", "origin")
	b.git(seed, "merge", "--no-ff", "--no-edit", "origin/goal/goal-b")
	b.git(seed, "push", "--quiet", "origin", "main")
	b.git(b.checkout, "fetch", "--quiet", "origin")
	move, err := mainMove(b.checkout, commit, b.originMain(), ProveSeams{}, members...)
	if err != nil || move.Kind != "code" || move.Count != 2 {
		t.Fatalf("selected goal became main base: base=%s move=%+v %v", base, move, err)
	}
}

// Merging an older ledger branch does not count main's code again. The
// merge's own resolution is still checked against its main first parent.
func TestLandingMainMoveGitAdapterRecordMergeOnRebuiltMain(t *testing.T) {
	t.Parallel()
	for _, resolutionCode := range []bool{false, true} {
		t.Run(fmt.Sprint(resolutionCode), func(t *testing.T) {
			t.Parallel()
			b := newBed(t)
			seed := filepath.Join(b.root, "seed")
			b.git(seed, "branch", "ledger")
			b.write(filepath.Join(seed, "metasystem/internal/code.go"), "package code\n")
			b.git(seed, "add", ".")
			b.git(seed, "commit", "-qm", "code rebuilt on")
			b.git(seed, "push", "--quiet", "origin", "main")
			sha := b.seat("seat-a", "goal-a")
			b.handIn("seat-a", "goal-a", sha)
			commit := b.merge("goal-a")
			b.git(seed, "checkout", "ledger")
			b.write(filepath.Join(seed, "metasystem/plans/goals/backlog.md"), string(mainMoveFixture(t, "backlog.md")))
			b.git(seed, "add", ".")
			b.git(seed, "commit", "-qm", "goal claim")
			b.git(seed, "checkout", "main")
			b.git(seed, "merge", "--no-ff", "--no-commit", "ledger")
			want := "records-only"
			if resolutionCode {
				want = "code"
				b.write(filepath.Join(seed, "metasystem/internal/code.go"), "package code\nconst Added = true\n")
				b.git(seed, "add", ".")
			}
			b.git(seed, "commit", "--no-edit")
			b.git(seed, "push", "--quiet", "origin", "main")
			b.git(b.checkout, "fetch", "--quiet", "origin")
			move, err := mainMove(b.checkout, commit, b.originMain(), ProveSeams{})
			if err != nil || move.Kind != want || move.Count != 2 {
				t.Fatalf("record merge: %+v %v want %s", move, err, want)
			}
		})
	}
}
