package batch

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func landingBed(t *testing.T) (assemblyBed, Store) {
	t.Helper()
	bed := assemblyFixture(t)
	prefixes, err := assembleUnits(bed.root, bed.base, bed.record.Units)
	must(t, err)
	bed.record.State, bed.record.PrefixTrees, bed.record.TipTree = StateLanding, prefixes, prefixes[len(prefixes)-1]
	bed.record.Proof = &Proof{Status: "green", AttemptID: "tip-attempt"}
	bed.record.History = append(bed.record.History, HistoryEntry{At: time.Unix(3, 0).UTC().Format(time.RFC3339Nano), Verb: "prove", From: StateProving, To: StateLanding, Actor: "owner"})
	bed.record.Receipts = map[string]PrefixReceipt{"goal-a": {GoalID: "goal-a", Tree: prefixes[0], AttemptID: "prefix-attempt"}}
	store := NewStore(bed.root, nil)
	must(t, store.Create(bed.record))
	return bed, store
}

func greenLandSeams(events *[]string) LandSeams {
	return LandSeams{
		Apply:         func(unit Unit) error { *events = append(*events, "apply:"+unit.GoalID); return nil },
		AppendReceipt: func(unit Unit, _ PrefixReceipt) error { *events = append(*events, "receipt:"+unit.GoalID); return nil },
		Commit: func(unit Unit, _ PrefixReceipt) (string, error) {
			*events = append(*events, "commit:"+unit.GoalID)
			return "commit-" + unit.GoalID, nil
		},
		Held:    func(_, _ string) error { *events = append(*events, "held"); return nil },
		Push:    func(_, _ string) error { *events = append(*events, "push"); return nil },
		Reset:   func(string) error { *events = append(*events, "reset"); return nil },
		Cleanup: func() error { *events = append(*events, "cleanup"); return nil },
	}
}

func TestBatchLandingTransportRunsWholeSeriesOnce(t *testing.T) {
	_, store := landingBed(t)
	var events []string
	must(t, LandSeries(store, testBatchID, "owner", time.Unix(4, 0), greenLandSeams(&events)))
	want := []string{"apply:goal-a", "receipt:goal-a", "commit:goal-a", "apply:goal-b", "receipt:goal-b", "commit:goal-b", "held", "push", "cleanup"}
	if !slices.Equal(events, want) {
		t.Fatalf("landing events=%v, want %v", events, want)
	}
	if progress := load(t, store).Landing; progress == nil || !progress.HeldChecked || !progress.PushComplete {
		t.Fatalf("landing progress=%+v", progress)
	}
}

func TestBatchLandingResumeRebuildsCompleteSeries(t *testing.T) {
	_, store := landingBed(t)
	must(t, store.Update(testBatchID, func(record *Record) error {
		record.Landing = &LandingProgress{Base: record.BaseTree, Commits: map[string]string{"goal-a": "lost-local-commit"}}
		return nil
	}))
	var events []string
	seams := greenLandSeams(&events)
	seams.Prepare = func(string) error { events = append(events, "prepare"); return nil }
	must(t, LandSeries(store, testBatchID, "owner", time.Unix(4, 0), seams))
	want := []string{"prepare", "apply:goal-a", "receipt:goal-a", "commit:goal-a", "apply:goal-b", "receipt:goal-b", "commit:goal-b", "held", "push", "cleanup"}
	if !slices.Equal(events, want) {
		t.Fatalf("resumed landing events=%v, want %v", events, want)
	}
}

func TestBatchDelegationRules(t *testing.T) {
	bed, _ := landingBed(t)
	bed.record.History = append(bed.record.History, HistoryEntry{At: time.Unix(3, 0).UTC().Format(time.RFC3339Nano), Verb: "prove", From: StateProving, To: StateLanding, Actor: "landing-owner"})
	store := NewStore(bed.root, nil)
	// landingBed already created this id in another root; this bed's root is
	// independent and the replacement store receives the actor-bound record.
	must(t, os.Remove(filepath.Join(bed.root, "artifacts", "agents", "landing-batches", testBatchID+".json")))
	must(t, store.Create(bed.record))
	if err := LandSeries(store, testBatchID, "other-owner", time.Unix(4, 0), greenLandSeams(&[]string{})); err == nil || !strings.Contains(err.Error(), "BATCH_LAND_DELEGATION_REFUSED") {
		t.Fatalf("foreign landing actor error=%v", err)
	}
	var events []string
	must(t, LandSeries(store, testBatchID, "landing-owner", time.Unix(4, 0), greenLandSeams(&events)))
	if !slices.Contains(events, "push") {
		t.Fatalf("owner landing events=%v", events)
	}
}

func TestBatchCommitRefusalIsAtomicAndKeepsJoinOrder(t *testing.T) {
	bed := assemblyFixture(t)
	bed.record.Units = append(bed.record.Units, Unit{GoalID: "goal-c", Chain: "chain-b", Claim: bed.record.Units[0].Claim, State: UnitJoined})
	bed.record.State, bed.record.PrefixTrees, bed.record.TipTree = StateLanding, []string{"prefix-a", "prefix-b", "prefix-c"}, "prefix-c"
	bed.record.Proof = &Proof{Status: "green", AttemptID: "tip"}
	bed.record.History = append(bed.record.History, HistoryEntry{At: time.Unix(3, 0).UTC().Format(time.RFC3339Nano), Verb: "prove", From: StateProving, To: StateLanding, Actor: "owner"})
	bed.record.Receipts = map[string]PrefixReceipt{
		"goal-a": {GoalID: "goal-a", Tree: "prefix-a"},
		"goal-b": {GoalID: "goal-b", Tree: "prefix-b"},
	}
	store := NewStore(bed.root, nil)
	must(t, store.Create(bed.record))
	var events []string
	seams := greenLandSeams(&events)
	seams.Commit = func(unit Unit, _ PrefixReceipt) (string, error) {
		events = append(events, "commit:"+unit.GoalID)
		if unit.GoalID == "goal-b" {
			return "", os.ErrPermission
		}
		return "commit-" + unit.GoalID, nil
	}
	must(t, LandSeries(store, testBatchID, "owner", time.Unix(4, 0), seams))
	record := load(t, store)
	if slices.Contains(events, "push") || !slices.Contains(events, "reset") || record.Units[1].State != UnitReturnPending || record.State != StateOpen || record.Units[0].State != UnitJoined || record.Units[2].State != UnitJoined {
		t.Fatalf("events=%v record=%+v", events, record)
	}
	if got := []string{record.Units[0].GoalID, record.Units[2].GoalID}; !slices.Equal(got, []string{"goal-a", "goal-c"}) {
		t.Fatalf("survivor order=%v", got)
	}
}

func TestBatchAfterPushRecoveryFinalizesEachTrailerOnce(t *testing.T) {
	_, store := landingBed(t)
	var events []string
	must(t, LandSeries(store, testBatchID, "owner", time.Unix(4, 0), greenLandSeams(&events)))
	finalized := map[string]int{}
	seams := RecoverySeams{
		OriginCommit: func(unit Unit) (string, bool, error) { return "origin-" + unit.Chain, true, nil },
		Finalize:     func(unit Unit, _ string) error { finalized[unit.GoalID]++; return nil },
	}
	must(t, RecoverPushedSeries(store, testBatchID, "owner", time.Unix(5, 0), seams))
	must(t, RecoverPushedSeries(store, testBatchID, "owner", time.Unix(6, 0), seams))
	if finalized["goal-a"] != 1 || finalized["goal-b"] != 1 || load(t, store).State != StateLanded {
		t.Fatalf("finalized=%v record=%+v", finalized, load(t, store))
	}
}

func TestApplyCertifiedPatchStagesTheTransportedBytes(t *testing.T) {
	bed := assemblyFixture(t)
	worktree := t.TempDir()
	baseCommit := strings.TrimSpace(runGitOutput(t, bed.root, "commit-tree", bed.base, "-m", "fixture base"))
	must(t, exec.Command("git", "-C", bed.root, "worktree", "add", "--detach", worktree, baseCommit).Run())
	t.Cleanup(func() { _ = exec.Command("git", "-C", bed.root, "worktree", "remove", "--force", worktree).Run() })
	must(t, ApplyCertifiedPatch(bed.root, worktree, "chain-a"))
	if got := strings.TrimSpace(runGitOutput(t, worktree, "diff", "--cached", "--name-only")); got != "a.go" {
		t.Fatalf("staged paths=%q", got)
	}
}

func TestCommitWithWrapperUsesTempRepoTokenAndExplicitIdentity(t *testing.T) {
	root := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755))
	must(t, os.MkdirAll(filepath.Join(root, "artifacts", "agents", "mains"), 0o755))
	must(t, os.WriteFile(filepath.Join(root, "artifacts", "agents", "mains", "worktree-commit-token.json"), []byte("{}\n"), 0o644))
	wrapper := filepath.Join(root, "scripts", "agents", "commit.sh")
	script := `#!/usr/bin/env bash
set -euo pipefail
test -f artifacts/agents/mains/worktree-commit-token.json
printf '%s\n' "$*" >wrapper.args
printf '%s <%s>|%s <%s>|%s\n' "$GIT_AUTHOR_NAME" "$GIT_AUTHOR_EMAIL" "$GIT_COMMITTER_NAME" "$GIT_COMMITTER_EMAIL" "$METASYSTEM_LANDED_BY" >wrapper.env
git commit -q "$@"
`
	must(t, os.WriteFile(wrapper, []byte(script), 0o755))
	must(t, exec.Command("git", "init", "-q", "-b", "main", root).Run())
	must(t, os.WriteFile(filepath.Join(root, "file"), []byte("one\n"), 0o644))
	must(t, exec.Command("git", "-C", root, "add", "file").Run())
	// The fixture wrapper consumes the boundary flags before delegating.
	script = strings.ReplaceAll(script, "git commit -q \"$@\"", `while (( $# )); do case "$1" in --chain|--goal|--test-receipt) shift 2;; *) break;; esac; done
git commit -q "$@"`)
	must(t, os.WriteFile(wrapper, []byte(script), 0o755))
	t.Setenv("GIT_AUTHOR_NAME", "Ambient Other")
	t.Setenv("GIT_AUTHOR_EMAIL", "ambient@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Ambient Other")
	t.Setenv("GIT_COMMITTER_EMAIL", "ambient@example.com")
	must(t, CommitWithWrapper(root, "chain-a", "goal-a", "receipt.json", "land goal a\n", "Wido", "wido@example.com", "m1l+landing-m1l"))
	if env := string(contents(t, filepath.Join(root, "wrapper.env"))); !strings.Contains(env, "Wido <wido@example.com>|Wido <wido@example.com>|m1l+landing-m1l") {
		t.Fatalf("wrapper env=%q", env)
	}
	if args := string(contents(t, filepath.Join(root, "wrapper.args"))); !strings.Contains(args, "--chain chain-a --goal goal-a --test-receipt receipt.json -F") {
		t.Fatalf("wrapper args=%q", args)
	}
}

func TestLandingBranchLeasesEndpointAndDeletesCandidateAtomically(t *testing.T) {
	origin, root := filepath.Join(t.TempDir(), "origin.git"), t.TempDir()
	run := func(args ...string) {
		t.Helper()
		command := exec.Command("git", args...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	run("init", "-q", "--bare", origin)
	run("init", "-q", "-b", "main", root)
	run("-C", root, "config", "user.name", "Fixture")
	run("-C", root, "config", "user.email", "fixture@example.com")
	must(t, os.WriteFile(filepath.Join(root, "file"), []byte("base\n"), 0o644))
	run("-C", root, "add", "file")
	run("-C", root, "commit", "-qm", "base")
	run("-C", root, "remote", "add", "origin", origin)
	run("-C", root, "push", "-q", "-u", "origin", "main")
	base := strings.TrimSpace(runGitOutput(t, root, "rev-parse", "HEAD"))
	must(t, PrepareLandingBranch(root, testBatchID, base))
	must(t, os.WriteFile(filepath.Join(root, "file"), []byte("tip\n"), 0o644))
	run("-C", root, "add", "file")
	tree := strings.TrimSpace(runGitOutput(t, root, "write-tree"))
	mid := strings.TrimSpace(runGitOutput(t, root, "commit-tree", tree, "-p", base, "-m", "candidate prefix"))
	tip := strings.TrimSpace(runGitOutput(t, root, "commit-tree", tree, "-p", mid, "-m", "candidate tip"))
	run("-C", root, "update-ref", "refs/heads/landing/"+testBatchID, tip)
	run("-C", root, "reset", "--hard", "-q", tip)
	must(t, PublishLandingBranch(root, testBatchID, "", tip))
	alien := strings.TrimSpace(runGitOutput(t, root, "commit-tree", tree, "-p", tip, "-m", "alien"))
	branchRef := "refs/heads/landing/" + testBatchID
	run("-C", root, "push", "-q", "--force", "origin", alien+":"+branchRef)
	if err := LandLandingBranch(root, testBatchID, base, tip); err == nil || !strings.Contains(err.Error(), "BATCH_LAND_PUSH_REFUSED") {
		t.Fatalf("moved landing branch error=%v", err)
	}
	if got := strings.TrimSpace(runGitOutput(t, origin, "rev-parse", "refs/heads/main")); got != base {
		t.Fatalf("atomic refusal moved endpoint to %s", got)
	}
	run("--git-dir", origin, "update-ref", branchRef, tip, alien)
	// A forward move to a commit in the candidate ancestry would be accepted
	// without the endpoint lease; the lease is the only refusal here.
	run("-C", root, "push", "-q", "--force", "origin", mid+":refs/heads/main")
	if err := LandLandingBranch(root, testBatchID, base, tip); err == nil || !strings.Contains(err.Error(), "BATCH_LAND_PUSH_REFUSED") {
		t.Fatalf("moved endpoint error=%v", err)
	}
	if err := exec.Command("git", "--git-dir", origin, "show-ref", "--verify", "--quiet", branchRef).Run(); err != nil {
		t.Fatalf("atomic endpoint refusal deleted candidate branch: %v", err)
	}
	run("--git-dir", origin, "update-ref", "refs/heads/main", base, mid)
	must(t, LandLandingBranch(root, testBatchID, base, tip))
	if got := strings.TrimSpace(runGitOutput(t, origin, "rev-parse", "refs/heads/main")); got != tip {
		t.Fatalf("endpoint=%s want %s", got, tip)
	}
	if err := exec.Command("git", "--git-dir", origin, "show-ref", "--verify", "--quiet", branchRef).Run(); err == nil {
		t.Fatalf("candidate branch %s survived atomic landing", branchRef)
	}
}

func runGitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	must(t, err)
	return string(out)
}
