package main

import (
	"encoding/json"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dispatchmodel "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// worktreeCollectFixture is the shape of the 2026-10-03 machinery block: a
// primary checkout on main with live unstaged changes, whose system
// dispatched (and so recorded) a commit critic, and a linked goal worktree
// where goal/G is checked out. Everything is real Git; the declared fakes
// are the fast gate and the critic itself, whose closed record the fake
// delegate writes where the delegate boundary would: the dispatching
// installation's artifacts/agents.
type worktreeCollectFixture struct {
	primary, worktree, unit, job string
}

func newWorktreeCollectFixture(t *testing.T) worktreeCollectFixture {
	t.Helper()
	primary, _, _ := goalBranchMainCLIFixtureBelow(t, "m1", ".")
	primary, err := filepath.EvalSymlinks(primary)
	if err != nil {
		t.Fatal(err)
	}
	base := goalSyncMutationGit(t, primary, "rev-parse", "HEAD")
	worktree := filepath.Join(t.TempDir(), "goal-worktree")
	goalSyncMutationGit(t, primary, "worktree", "add", "-q", "-b", "goal/standing-validation", worktree, base)
	if worktree, err = filepath.EvalSymlinks(worktree); err != nil {
		t.Fatal(err)
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe holder: state=%s err=%v", state, err)
	}
	if _, err := lease.AnnounceWithPair(worktree, "goal-worktree-collect", exact.Pid, exact.StartedAt.Unix(),
		exact.StartTicks, exact.BootID, "fixture", "fake", "m1"); err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(worktree, "metasystem", "code.go"), []byte("package fixture\n"), 0o644)
	goalSyncMutationGit(t, worktree, "add", "metasystem/code.go")
	code, stdout, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return goalBranchTestCommand([]string{"commit", "--goal", "standing-validation", "--kind", "unit", "--unit", "u1", "--root", worktree}, stdout, stderr)
	})
	unit := strings.TrimSpace(stdout)
	if code != 0 || len(unit) != 40 {
		t.Fatalf("unit commit: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, _, stderr = runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return goalBranchTestCommand([]string{"push", "--goal", "standing-validation", "--root", worktree, "--opid", "worktree-collect-push"}, stdout, stderr)
	})
	if code != 0 {
		t.Fatalf("goal push: code=%d stderr=%q", code, stderr)
	}
	// The primary's live log change, unstaged on main (m1h's sighting).
	writeTestingFixtureFile(t, filepath.Join(primary, "metasystem", "memory", "receipts.log"), []byte("seed receipt\nlive receipt\n"), 0o644)
	return worktreeCollectFixture{primary: primary, worktree: worktree, unit: unit, job: "code-critic-worktree-collect"}
}

// writeClosedCritic writes the closed code-critic root of the unit commit
// into installation's job store, as a completed clean examination.
func (f worktreeCollectFixture) writeClosedCritic(t *testing.T, installation string) {
	t.Helper()
	subject, present, err := dispatchmodel.ComputeReadSubject(dispatchmodel.ReadSubjectRequest{
		RepoRoot: installation, Role: "code-critic", Reviews: "commit:" + f.unit})
	if err != nil || !present {
		t.Fatalf("critic subject present=%t err=%v", present, err)
	}
	record := map[string]any{"jobId": f.job, "role": "code-critic", "round": 1, "status": "completed",
		"reviews": "commit:" + f.unit, "goalId": "standing-validation", "goalRevision": 1,
		"findingRegister": []any{}, "findingRegisterRound": 1, "findingRegisterSubjectDigest": subject.Digest(),
		"chainClosed": true, "closure": map[string]any{"criticRoot": f.job, "round": 1, "subject": subject, "mechanism": "clean"}}
	for path, value := range map[string]any{
		"jobs/" + f.job + ".json":        record,
		f.job + "/rounds/1/subject.json": subject,
		f.job + "/rounds/1/return.json":  map[string]any{"jobId": f.job, "round": 1, "reviewedTree": subject.Tree},
	} {
		body, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		writeTestingFixtureFile(t, filepath.Join(installation, "artifacts", "agents", filepath.FromSlash(path)), append(body, '\n'), 0o644)
	}
}

// A commit critic dispatched from the primary checkout records its job in
// the primary's store. Its read is collected where goal/G is checked out,
// the goal worktree: the read owner finds the critic's record in the store
// that holds it (the primary's, since the goal worktree has none of its
// own), and the Goal-Read commit is installed on the goal branch in the
// worktree; the primary's checkout and its unstaged change are untouched.
// From the primary itself the collect is still refused, because goal/G is
// checked out in another worktree.
func TestGoalWorktreeCollectReadsTheCriticRecordedByItsPrimaryCheckout(t *testing.T) {
	f := newWorktreeCollectFixture(t)
	deps := goalBranchReadDependencies{Gate: func(string) (string, error) { return "green", nil },
		Delegate: func(_, _, _, _, _ string) (string, error) { f.writeClosedCritic(t, f.primary); return f.job, nil }}
	read := func(root string, extra ...string) (int, string, string) {
		args := append([]string{"--root", root, "--goal", "standing-validation", "--unit", f.unit}, extra...)
		return runOnOwnStreams(func(stdout, stderr io.Writer) int { return runGoalBranchReadWith(args, deps, stdout, stderr) })
	}
	code, stdout, stderr := read(f.primary)
	if code != 0 || !strings.Contains(stdout, "state=dispatched root-job="+f.job) {
		t.Fatalf("dispatch from the primary: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(f.worktree, "artifacts", "agents", "jobs", f.job+".json")); !os.IsNotExist(err) {
		t.Fatalf("the goal worktree has its own record of the critic: %v", err)
	}
	primaryHead := goalSyncMutationGit(t, f.primary, "rev-parse", "HEAD")
	worktreeHead := goalSyncMutationGit(t, f.worktree, "rev-parse", "HEAD")

	code, stdout, stderr = read(f.primary, "--collect")
	if code == 0 || !strings.Contains(stderr, "checked out in another worktree") {
		t.Fatalf("collect from the primary: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	code, stdout, stderr = read(f.worktree)
	if code != 0 || !strings.Contains(stdout, "state=closed root-job="+f.job) {
		t.Fatalf("read from the goal worktree: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, stdout, stderr = read(f.worktree, "--collect")
	if code != 0 || !strings.Contains(stdout, "state=collected root-job="+f.job) {
		t.Fatalf("collect from the goal worktree: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	tip := goalSyncMutationGit(t, f.worktree, "rev-parse", "HEAD")
	if tip == worktreeHead || goalSyncMutationGit(t, f.worktree, "rev-parse", "HEAD^") != worktreeHead ||
		goalSyncMutationGit(t, f.worktree, "rev-parse", "refs/heads/goal/standing-validation") != tip {
		t.Fatalf("the Goal-Read commit is not installed on goal/standing-validation in the worktree: head %s, before %s", tip, worktreeHead)
	}
	info, err := branch.KindOf(f.worktree, tip, "standing-validation")
	if err != nil || info.Kind != branch.Read {
		t.Fatalf("the worktree's new tip is not a Goal-Read commit: %+v err=%v", info, err)
	}
	endpointTip := goalSyncMutationGit(t, f.worktree, "rev-parse", "refs/remotes/upstream/main")
	attestation, err := branch.ValidateAttestation(f.worktree, endpointTip, "standing-validation", "u1", f.unit)
	if err != nil || attestation.Source.RootJob != f.job {
		t.Fatalf("collected attestation=%+v err=%v", attestation, err)
	}
	if head := goalSyncMutationGit(t, f.primary, "rev-parse", "HEAD"); head != primaryHead {
		t.Fatalf("the primary checkout moved from %s to %s", primaryHead, head)
	}
	if branchName := goalSyncMutationGit(t, f.primary, "symbolic-ref", "--short", "HEAD"); branchName != "main" {
		t.Fatalf("the primary checkout left main for %s", branchName)
	}
	if data, err := os.ReadFile(filepath.Join(f.primary, "metasystem", "memory", "receipts.log")); err != nil || string(data) != "seed receipt\nlive receipt\n" {
		t.Fatalf("the primary's unstaged change was touched: %q err=%v", data, err)
	}

	code, stdout, stderr = read(f.worktree, "--collect")
	if code != 0 || !strings.Contains(stdout, "state=already-collected") || goalSyncMutationGit(t, f.worktree, "rev-parse", "HEAD") != tip {
		t.Fatalf("repeat collect: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

// review --commit from the primary checkout runs the review where goal/G is
// checked out, as review run and review --changes do: its installation is
// the goal worktree's, with the primary as the selected installation. The
// critic the primary dispatched before is found in the primary's store, the
// Goal-Read commit is installed in the goal worktree past the primary's
// unstaged change, and the production publication owner pushes it.
func TestReviewCommitFromThePrimaryCollectsInTheGoalWorktree(t *testing.T) {
	f := newWorktreeCollectFixture(t)
	deps := goalBranchReadDependencies{Gate: func(string) (string, error) { return "green", nil },
		Delegate: func(_, _, _, _, _ string) (string, error) { f.writeClosedCritic(t, f.primary); return f.job, nil }}
	code, stdout, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return runGoalBranchReadWith([]string{"--root", f.primary, "--goal", "standing-validation", "--unit", f.unit}, deps, stdout, stderr)
	})
	if code != 0 || !strings.Contains(stdout, "state=dispatched root-job="+f.job) {
		t.Fatalf("dispatch from the primary: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	primaryHead := goalSyncMutationGit(t, f.primary, "rev-parse", "HEAD")
	worktreeHead := goalSyncMutationGit(t, f.worktree, "rev-parse", "HEAD")
	var reads [][]string
	inv := &intentInvocation{
		layout: stateroot.Layout{GitRoot: f.primary, RepositoryRoot: f.primary, InstallationRoot: stateroottest.Installation(t, f.primary)},
		input:  intentInput{values: map[string][]string{"goal": {"standing-validation"}}},
		owners: intentOwners{resolver: stateroot.NewResolver(fakeTop(f.primary), noExecutable), delivery: &intentDeliveryOwners{
			branchRead: func(args []string) (branch.BranchReadResult, int, error) {
				reads = append(reads, args)
				return goalBranchReadRun(args, deps)
			},
			publishRead: goalBranchPublishRead,
		}},
	}
	result := inv.reviewCommit(f.unit)
	if result.Outcome != intentConfirmed || len(reads) != 2 {
		t.Fatalf("review --commit from the primary: reads=%q result=%+v", reads, result)
	}
	for _, args := range reads {
		if flagValue(args, "--root") != f.worktree || flagValue(args, "--selected-installation") != f.primary {
			t.Fatalf("the review ran at %q (selected %q), not the goal worktree %s selected by the primary",
				flagValue(args, "--root"), flagValue(args, "--selected-installation"), f.worktree)
		}
	}
	tip := goalSyncMutationGit(t, f.worktree, "rev-parse", "HEAD")
	if info, err := branch.KindOf(f.worktree, tip, "standing-validation"); err != nil || info.Kind != branch.Read ||
		goalSyncMutationGit(t, f.worktree, "rev-parse", "HEAD^") != worktreeHead {
		t.Fatalf("the goal worktree's tip %s is not the Goal-Read on %s: %+v err=%v", tip, worktreeHead, info, err)
	}
	if remote := goalSyncMutationGit(t, f.worktree, "ls-remote", "upstream", "refs/heads/goal/standing-validation"); !strings.HasPrefix(remote, tip) {
		t.Fatalf("the Goal-Read %s is not published: %q", tip, remote)
	}
	if head := goalSyncMutationGit(t, f.primary, "rev-parse", "HEAD"); head != primaryHead ||
		goalSyncMutationGit(t, f.primary, "symbolic-ref", "--short", "HEAD") != "main" {
		t.Fatalf("the primary checkout moved: %s, was %s on main", head, primaryHead)
	}
	if data, err := os.ReadFile(filepath.Join(f.primary, "metasystem", "memory", "receipts.log")); err != nil || string(data) != "seed receipt\nlive receipt\n" {
		t.Fatalf("the primary's unstaged change was touched: %q err=%v", data, err)
	}
}
