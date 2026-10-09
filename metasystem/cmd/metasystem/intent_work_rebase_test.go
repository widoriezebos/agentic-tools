package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"path/filepath"
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

// The command uses the real range and carry owners: the claim is which Git
// evidence stays retained while a proven landed read leaves the active range.
func TestWorkRebaseOmitsProvenLandedReadGitAdapter(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"equivalent", "replayed equivalent", "same filename", "corrupt manifest"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b, owners, _ := rebaseIntentBed(t)
			repo := b.worktree
			connectionGit(t, repo, "init", "-q", "-b", "main")
			connectionGit(t, repo, "config", "user.name", "fixture")
			connectionGit(t, repo, "config", "user.email", "fixture@example.invalid")
			connectionGit(t, repo, "commit", "-qm", "base", "--allow-empty")
			base := connectionGit(t, repo, "rev-parse", "HEAD")
			remote := filepath.Join(t.TempDir(), "origin.git")
			connectionGit(t, filepath.Dir(remote), "init", "-q", "--bare", remote)
			connectionGit(t, repo, "remote", "add", "origin", remote)
			writeUnitCarryFile(t, filepath.Join(repo, "u1.go"), "original\n")
			connectionGit(t, repo, "add", "u1.go")
			original, err := branch.CommitStaged(branch.CommitRequest{Repo: repo, Remote: "origin", EndpointTip: base, GoalID: b.id, Unit: "u1", Kind: branch.Unit, OpID: "original", CheckClaim: func() error { return nil }})
			if err != nil {
				t.Fatal(err)
			}
			digest, err := branch.UnitDigest(repo, original)
			if err != nil {
				t.Fatal(err)
			}
			readPath := "metasystem/records/misc/read.md"
			writeUnitCarryFile(t, filepath.Join(repo, readPath), "Read "+original+" change "+digest)
			read, _, err := branch.CommitRead(branch.CommitReadRequest{Repo: repo, Remote: "origin", EndpointTip: base, GoalID: b.id, Unit: "u1", OpID: "read", ReaderRecord: readPath, CheckClaim: func() error { return nil }, GateRunID: "gate", GateTree: connectionGit(t, repo, "rev-parse", original+"^{tree}")})
			if err != nil {
				t.Fatal(err)
			}
			connectionGit(t, repo, "checkout", "-qb", "integrated", base)
			connectionGit(t, repo, "cherry-pick", "--no-commit", original)
			if state == "same filename" {
				writeUnitCarryFile(t, filepath.Join(repo, "u1.go"), "different change\n")
				connectionGit(t, repo, "add", "u1.go")
			} else if state == "corrupt manifest" {
				digest = strings.Repeat("0", 64)
			}
			if state == "same filename" {
				connectionGit(t, repo, "commit", "-qm", "provisional")
				var err error
				digest, err = branch.UnitDigest(repo, "HEAD")
				if err != nil {
					t.Fatal(err)
				}
				connectionGit(t, repo, "commit", "--amend", "-qm", "landed\n\nGoal-Unit: "+b.id+"/u1\nGoal-Digest: "+digest)
			} else {
				connectionGit(t, repo, "commit", "-qm", "landed\n\nGoal-Unit: "+b.id+"/u1\nGoal-Digest: "+digest)
			}
			main := connectionGit(t, repo, "rev-parse", "HEAD")
			connectionGit(t, repo, "push", "-q", "origin", "HEAD:main")
			if state == "replayed equivalent" {
				connectionGit(t, repo, "checkout", "-qB", "goal/"+b.id, read)
			} else {
				connectionGit(t, repo, "checkout", "-qB", "goal/"+b.id, main)
				connectionGit(t, repo, "cherry-pick", read)
			}
			writeUnitCarryFile(t, filepath.Join(repo, "u2.go"), "remaining\n")
			connectionGit(t, repo, "add", "u2.go")
			connectionGit(t, repo, "commit", "-qm", "remaining\n\nGoal-Unit: "+b.id+"/u2")
			connectionGit(t, repo, "push", "-q", "origin", "HEAD:goal/"+b.id)
			owners.connection.endpointTip = func(string, goal.Endpoint) (string, error) { return main, nil }
			owners.connection.rebase = branch.Rebase
			owners.connection.rebaseGate = func(string) (string, error) { t.Fatal("a landed read started a new check"); return "", nil }
			code, result := b.runJSON(owners, "work", "rebase", b.id)
			if state != "equivalent" && state != "replayed equivalent" {
				if code == 0 {
					t.Fatalf("unproved equivalence admitted: %+v", result)
				}
				return
			}
			if code != 0 || state == "equivalent" && result.Outcome != intentUnchanged || state == "replayed equivalent" && result.Outcome != intentConfirmed {
				t.Fatalf("rebase: %+v code=%d", result, code)
			}
			tip := connectionGit(t, repo, "rev-parse", "HEAD")
			commits, err := branch.ValidateRange(repo, main, tip, b.id)
			if err != nil || len(commits) != 1 || commits[0].Unit != "u2" {
				t.Fatalf("active range: %+v %v", commits, err)
			}
			if data := connectionGit(t, repo, "show", tip+":metasystem/records/reads/"+b.id+"/"+original+".json"); !strings.Contains(data, original) {
				t.Fatal("landed read evidence was deleted")
			}
		})
	}
}

// Remote-main freshness and the reported count require the real Git adapter.
func TestWorkRebaseReportsRemoteMainBehindGitAdapter(t *testing.T) {
	t.Parallel()
	b, owners, _ := rebaseIntentBed(t)
	repo := b.worktree
	connectionGit(t, repo, "init", "-q", "-b", "main")
	connectionGit(t, repo, "config", "user.name", "fixture")
	connectionGit(t, repo, "config", "user.email", "fixture@example.invalid")
	connectionGit(t, repo, "commit", "-qm", "base", "--allow-empty")
	base := connectionGit(t, repo, "rev-parse", "HEAD")
	remote := filepath.Join(t.TempDir(), "origin.git")
	connectionGit(t, filepath.Dir(remote), "init", "-q", "--bare", remote)
	connectionGit(t, repo, "remote", "add", "origin", remote)
	connectionGit(t, repo, "push", "-q", "origin", "HEAD:main")
	connectionGit(t, repo, "checkout", "-qb", "goal/"+b.id)
	writeUnitCarryFile(t, filepath.Join(repo, "u1.go"), "work\n")
	connectionGit(t, repo, "add", "u1.go")
	connectionGit(t, repo, "commit", "-qm", "unit\n\nGoal-Unit: "+b.id+"/u1")
	old := connectionGit(t, repo, "rev-parse", "HEAD")
	connectionGit(t, repo, "push", "-q", "origin", "HEAD:goal/"+b.id)
	peer := filepath.Join(t.TempDir(), "peer")
	connectionGit(t, filepath.Dir(peer), "clone", "-q", "--branch", "main", remote, peer)
	connectionGit(t, peer, "config", "user.name", "fixture")
	connectionGit(t, peer, "config", "user.email", "fixture@example.invalid")
	for i := 1; i <= 3; i++ {
		writeUnitCarryFile(t, filepath.Join(peer, "plans/goals/history.md"), fmt.Sprintf("history %d\n", i))
		connectionGit(t, peer, "add", "plans/goals/history.md")
		connectionGit(t, peer, "commit", "-qm", "goal history")
	}
	main := connectionGit(t, peer, "rev-parse", "HEAD")
	connectionGit(t, peer, "push", "-q", "origin", "HEAD:main")
	if tip := connectionGit(t, repo, "rev-parse", "refs/remotes/origin/main"); tip != base {
		t.Fatal("fixture's remote tracking ref is not stale")
	}
	owners.connection.endpointTip = branch.EndpointTip
	owners.connection.rebase = branch.Rebase
	code, result := b.runJSON(owners, "work", "rebase", b.id)
	data, err := json.Marshal(result.Data)
	var got branch.RebaseResult
	if err != nil || json.Unmarshal(data, &got) != nil || code != 0 || got.State != "held" || got.MainTip != main || got.Behind != 3 || got.NewTip != old || !strings.Contains(result.Summary, "3 commits behind current remote main") || strings.Contains(result.Summary, "already on main") {
		t.Fatalf("fresh main and count: result=%+v rebase=%+v code=%d err=%v", result, got, code, err)
	}
}

// Git's merge removal and preservation of commit messages require the real adapter.
func TestWorkRebaseLinearizesMergedMainGitAdapter(t *testing.T) {
	t.Parallel()
	b, owners, _ := rebaseIntentBed(t)
	repo := b.worktree
	git := func(args ...string) string { t.Helper(); return connectionGit(t, repo, args...) }
	git("init", "-q", "-b", "main")
	git("config", "user.name", "fixture")
	git("config", "user.email", "fixture@example.invalid")
	git("commit", "-qm", "base", "--allow-empty")
	base := git("rev-parse", "HEAD")
	remote := filepath.Join(t.TempDir(), "origin.git")
	connectionGit(t, filepath.Dir(remote), "init", "-q", "--bare", remote)
	git("remote", "add", "origin", remote)
	git("checkout", "-qb", "goal/"+b.id)
	writeUnitCarryFile(t, filepath.Join(repo, "u1.go"), "work\n")
	git("add", "u1.go")
	message := "keep the unit message\n\nGoal-Unit: " + b.id + "/u1"
	git("commit", "-qm", message)
	if _, err := branch.Push(branch.PushRequest{Repo: repo, Remote: "origin", EndpointTip: base, GoalID: b.id, OpID: "publish-unit", CheckClaim: func() error { return nil }}); err != nil {
		t.Fatal(err)
	}
	git("checkout", "-q", "main")
	writeUnitCarryFile(t, filepath.Join(repo, "main.txt"), "main moved\n")
	git("add", "main.txt")
	git("commit", "-qm", "advance main")
	main := git("rev-parse", "HEAD")
	git("push", "-q", "origin", "main")
	git("checkout", "-q", "goal/"+b.id)
	git("merge", "--no-ff", "-qm", "merge main by hand", "main")
	old := git("rev-parse", "HEAD")
	if git("merge-base", main, old) != main || git("rev-list", "--merges", "--count", main+".."+old) != "1" || base == main {
		t.Fatal("fixture must have main as an ancestor and one merge above it")
	}
	owners.connection.endpointTip = branch.EndpointTip
	owners.connection.rebase = branch.Rebase
	code, result := b.runJSON(owners, "work", "rebase", b.id)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("rebase: exit %d %+v; main=%s old=%s tip=%s merges=%s", code, result, main, old, git("rev-parse", "HEAD"), git("rev-list", "--merges", "--count", main+"..HEAD"))
	}
	tip := git("rev-parse", "HEAD")
	if merges := git("rev-list", "--merges", "--count", main+".."+tip); merges != "0" {
		t.Fatalf("rebase left %s merges above main", merges)
	}
	commits, err := branch.ValidateRange(repo, main, tip, b.id)
	if err != nil || len(commits) != 1 || commits[0].Unit != "u1" || git("show", "-s", "--format=%B", tip) != message || git("show", tip+":u1.go") != "work" || connectionGit(t, remote, "rev-parse", "refs/heads/goal/"+b.id) != tip {
		t.Fatalf("replayed unit or publication changed: commits=%+v err=%v", commits, err)
	}
}

// The Git adapter must replace a pushed merge with the replayed unit history.
func TestWorkRebaseLinearizesAPushedMergeGitAdapter(t *testing.T) {
	t.Parallel()
	b, owners, _ := rebaseIntentBed(t)
	repo := b.worktree
	git := func(args ...string) string { t.Helper(); return connectionGit(t, repo, args...) }
	git("init", "-q", "-b", "main")
	git("config", "user.name", "fixture")
	git("config", "user.email", "fixture@example.invalid")
	git("commit", "-qm", "base", "--allow-empty")
	remote := filepath.Join(t.TempDir(), "origin.git")
	connectionGit(t, filepath.Dir(remote), "init", "-q", "--bare", remote)
	git("remote", "add", "origin", remote)
	git("checkout", "-qb", "goal/"+b.id)
	writeUnitCarryFile(t, filepath.Join(repo, "u1.go"), "work\n")
	git("add", "u1.go")
	message := "keep the unit message\n\nGoal-Unit: " + b.id + "/u1"
	git("commit", "-qm", message)
	git("push", "-q", "origin", "goal/"+b.id)
	git("checkout", "-q", "main")
	writeUnitCarryFile(t, filepath.Join(repo, "main.txt"), "main moved\n")
	git("add", "main.txt")
	git("commit", "-qm", "advance main")
	main := git("rev-parse", "HEAD")
	git("push", "-q", "origin", "main")
	git("checkout", "-q", "goal/"+b.id)
	git("merge", "--no-ff", "-qm", "merge main by hand", "main")
	git("push", "-q", "origin", "goal/"+b.id)
	git("fetch", "-q", "origin")
	old := git("rev-parse", "origin/goal/"+b.id)
	if merges := git("rev-list", "--merges", "--count", "main..origin/goal/"+b.id); merges != "1" {
		t.Fatalf("fixture must have one pushed merge above main, got %s", merges)
	}
	if _, err := branch.InspectStatus(repo, main, old, b.id); err == nil || !strings.Contains(err.Error(), "2 parents") || !strings.Contains(err.Error(), "work rebase "+b.id) {
		t.Fatalf("work land's refusal shape: %v", err)
	}
	owners.connection.endpointTip = branch.EndpointTip
	owners.connection.rebase = branch.Rebase
	code, result := b.runJSON(owners, "work", "rebase", b.id)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("rebase: exit %d %+v", code, result)
	}
	git("fetch", "-q", "origin")
	tip := git("rev-parse", "origin/goal/"+b.id)
	if merges := git("rev-list", "--merges", "--count", "main..origin/goal/"+b.id); merges != "0" {
		t.Fatalf("rebase left %s merges on origin above main", merges)
	}
	status, err := branch.InspectStatus(repo, main, tip, b.id)
	if err != nil || len(status.Commits) != 1 || status.Commits[0].Unit != "u1" || git("show", "-s", "--format=%B", tip) != message || git("show", tip+":u1.go") != "work" || git("rev-parse", "HEAD") != tip {
		t.Fatalf("replayed unit, publication, or work land inspection changed: status=%+v err=%v", status, err)
	}
}
