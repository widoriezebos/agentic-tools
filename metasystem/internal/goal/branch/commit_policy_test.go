package branch

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	policyBase    = "1111111111111111111111111111111111111111"
	policyLocal   = "2222222222222222222222222222222222222222"
	policyRemote  = "3333333333333333333333333333333333333333"
	policyNew     = "4444444444444444444444444444444444444444"
	policyIndex   = "index-tree"
	policyNewTree = "new-tree"
)

type policyTransport struct {
	remote func(string, string, string) (string, bool, error)
	fetch  func(string, string, string, string) error
}

func (p policyTransport) RemoteTip(a, b, c string) (string, bool, error) { return p.remote(a, b, c) }
func (p policyTransport) Fetch(a, b, c, d string) error                  { return p.fetch(a, b, c, d) }
func (policyTransport) Push(string, string, string, string, string) (CASOutcome, error) {
	panic("unexpected Push")
}

type policyState struct {
	headRef, headCommit, index string
	goal, origin               string
	status                     string
}
type policyFixture struct {
	t                                                          *testing.T
	root, scratch                                              string
	op                                                         string
	expected                                                   []string
	at                                                         int
	local, remote, origin                                      string
	endpointHead, observedHeadRef                              string
	staged, unstaged, changes                                  []string
	patch                                                      []byte
	worktrees                                                  []commitWorktree
	rangeErr, remoteRangeErr, commitErr, checkoutErr, clearErr error
	installPath, installBody                                   string
	installedStatus                                            string
	state                                                      policyState
}

func newPolicyFixture(t *testing.T) *policyFixture {
	t.Helper()
	f := &policyFixture{t: t, root: t.TempDir(), scratch: t.TempDir(), endpointHead: policyBase, observedHeadRef: "refs/heads/main",
		staged: []string{"metasystem/code.go"}, patch: []byte("patch\x00content"), changes: []string{"metasystem/code.go"},
		installPath: "metasystem/code.go", installBody: "one", state: policyState{headRef: "refs/heads/main", headCommit: policyBase, index: policyIndex, status: "staged:metasystem/code.go"}}
	t.Cleanup(func() {
		if f.at != len(f.expected) {
			t.Errorf("unconsumed repository calls: %v", f.expected[f.at:])
		}
	})
	return f
}
func (f *policyFixture) expect(calls ...string) { f.expected = append(f.expected, calls...) }
func (f *policyFixture) step(call string) {
	f.t.Helper()
	if f.at >= len(f.expected) || f.expected[f.at] != call {
		f.t.Fatalf("repository call %q at %d; want remaining %v", call, f.at, f.expected[f.at:])
	}
	f.at++
}
func (f *policyFixture) args(ok bool, name string, got ...any) {
	f.t.Helper()
	if !ok {
		f.t.Fatalf("%s arguments: %v", name, got)
	}
}
func (f *policyFixture) req(op string) CommitRequest {
	f.op = op
	return CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: policyBase, GoalID: "goal-a", Unit: "u1", OpID: op, Kind: Unit,
		CheckClaim: func() error {
			if f.at < len(f.expected) && f.expected[f.at] == "claim-denied" {
				f.step("claim-denied")
				return errors.New("claim belongs to machine-b+lineage-b")
			}
			if f.at < len(f.expected) && f.expected[f.at] == "claim-lost" {
				f.step("claim-lost")
				return errors.New("claim moved")
			}
			f.step("claim")
			return nil
		},
		Transport: policyTransport{remote: func(repo, remote, ref string) (string, bool, error) {
			f.step("remote")
			f.args(repo == f.root && remote == "origin" && ref == goalBranchRef("goal-a"), "remote", repo, remote, ref)
			return f.remote, f.remote != "", nil
		},
			fetch: func(repo, remote, ref, dest string) error {
				f.step("fetch")
				f.args(repo == f.root && remote == "origin" && ref == goalBranchRef("goal-a") && dest == fetchRef(op), "fetch", repo, remote, ref, dest)
				return nil
			}}}
}
func (f *policyFixture) repository() commitRepository {
	return commitRepository{facts: commitFacts{
		Tip: func(repo, ref string) (string, bool, error) {
			f.args(repo == f.root, "Tip repo", repo)
			switch ref {
			case goalBranchRef("goal-a"):
				f.step("tip-goal")
				return f.local, f.local != "", nil
			case originTipRef("goal-a"):
				f.step("tip-origin")
				return f.origin, f.origin != "", nil
			default:
				f.t.Fatalf("unexpected Tip ref %s", ref)
				return "", false, nil
			}
		},
		Head: func(repo string) (string, error) {
			if repo == f.root {
				f.step("head-root")
				return f.endpointHead, nil
			}
			f.step("head-scratch")
			f.args(repo == f.scratch, "Head", repo)
			return policyNew, nil
		},
		Index: func(repo string) (string, error) {
			f.step("index")
			f.args(repo == f.root, "Index", repo)
			return policyIndex, nil
		},
		Staged: func(repo string) ([]string, error) {
			f.step("staged")
			f.args(repo == f.root, "Staged", repo)
			return append([]string(nil), f.staged...), nil
		},
		Unstaged: func(repo string) ([]string, error) {
			f.step("unstaged")
			f.args(repo == f.root, "Unstaged", repo)
			return append([]string(nil), f.unstaged...), nil
		},
		Patch: func(repo string) ([]byte, error) {
			f.step("patch")
			f.args(repo == f.root, "Patch", repo)
			return bytes.Clone(f.patch), nil
		},
		Range: func(repo, endpoint, tip, goal string) ([]Commit, error) {
			call := "range-new"
			if repo == f.root {
				call = "range-local"
				if tip == f.remote {
					call = "range-remote"
				}
			}
			f.step(call)
			f.args(endpoint == policyBase && goal == "goal-a" && ((repo == f.root && (tip == f.local || tip == f.remote)) || (repo == f.scratch && tip == policyNew)), "Range", repo, endpoint, tip, goal)
			if f.rangeErr != nil && call == "range-local" {
				return nil, f.rangeErr
			}
			if f.remoteRangeErr != nil && call == "range-remote" {
				return nil, f.remoteRangeErr
			}
			commits := map[string][]Commit{
				policyLocal:  {{ID: policyLocal, Kind: Unit, Units: []string{"u1"}, Unit: "u1"}},
				policyRemote: {{ID: policyRemote, Kind: Unit, Units: []string{"u1"}, Unit: "u1"}},
				policyNew:    {{ID: policyNew, Kind: Unit, Units: []string{"u1"}, Unit: "u1"}},
			}
			value := append([]Commit(nil), commits[tip]...)
			for i := range value {
				value[i].Units = append([]string(nil), value[i].Units...)
			}
			return value, nil
		},
		Ancestor: func(repo, older, newer string) (bool, error) {
			f.step("ancestor")
			f.args(repo == f.root && older == f.local && newer == f.remote, "Ancestor", repo, older, newer)
			return true, nil
		},
		Changes: func(repo, index, tip string) ([]string, error) {
			f.step("changes")
			f.args(repo == f.root && index == policyIndex && tip == policyNew, "Changes", repo, index, tip)
			return append([]string(nil), f.changes...), nil
		},
		HeadRef: func(repo string) string {
			f.step("headref")
			f.args(repo == f.root, "HeadRef", repo)
			return f.observedHeadRef
		},
		Worktrees: func(repo string) ([]commitWorktree, error) {
			f.step("worktrees")
			f.args(repo == f.root, "Worktrees", repo)
			return append([]commitWorktree(nil), f.worktrees...), nil
		},
	}, effects: commitEffects{
		ClearFetch: func(repo, ref string) error {
			f.step("clear")
			f.args(repo == f.root && ref == fetchRef(f.op), "ClearFetch", repo, ref)
			return f.clearErr
		},
		Open: func(repo, base string, amend bool) (string, func(), error) {
			f.step("open")
			want := policyBase
			if f.remote != "" {
				want = f.remote
			}
			if f.local != "" && f.remote == "" {
				want = f.local
			}
			f.args(repo == f.root && base == want && !amend, "Open", repo, base, amend)
			return f.scratch, func() { f.step("close") }, nil
		},
		Apply: func(dir string, patch []byte) error {
			f.step("apply")
			f.args(dir == f.scratch && bytes.Equal(patch, f.patch), "Apply", dir, patch)
			return nil
		},
		Commit: func(dir, subject, trailer string, amend bool) error {
			f.step("commit")
			f.args(dir == f.scratch && subject == "goal goal-a units u1" && trailer == "Goal-Unit: goal-a/u1" && !amend, "Commit", dir, subject, trailer, amend)
			return f.commitErr
		},
		Checkout: func(repo, before, after string) error {
			f.step("checkout")
			f.args(repo == f.root && before == policyIndex && after == policyNew, "Checkout", repo, before, after)
			if f.checkoutErr != nil {
				return f.checkoutErr
			}
			f.state.index = policyNewTree
			f.state.status = f.installedStatus
			if f.installPath != "" {
				f.write(f.installPath, f.installBody)
			}
			return nil
		},
		Attach: func(repo, ref string) error {
			f.step("attach")
			f.args(repo == f.root && ref == goalBranchRef("goal-a"), "Attach", repo, ref)
			f.state.headRef = ref
			return nil
		},
		Restore: func(repo, ref, commit string) error {
			f.t.Fatalf("unexpected Restore %s %s %s", repo, ref, commit)
			return nil
		},
		Publish: func(repo, goal, old, next, origin string) error {
			f.step("publish")
			f.args(repo == f.root && goal == "goal-a" && old == f.local && next == policyNew && origin == f.originValue(), "Publish", repo, goal, old, next, origin)
			f.state.goal = next
			f.state.origin = origin
			f.state.headCommit = next
			return nil
		},
	}}
}
func (f *policyFixture) originValue() string {
	if f.remote != "" {
		return f.remote
	}
	return f.origin
}
func (f *policyFixture) write(path, body string) {
	f.t.Helper()
	full := filepath.Join(f.root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0644); err != nil {
		f.t.Fatal(err)
	}
}
func (f *policyFixture) read(path string) string {
	f.t.Helper()
	b, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(path)))
	if err != nil {
		f.t.Fatal(err)
	}
	return string(b)
}
func (f *policyFixture) assertState(want policyState) {
	f.t.Helper()
	if !reflect.DeepEqual(f.state, want) {
		f.t.Fatalf("after state=%+v, want %+v", f.state, want)
	}
}
func (f *policyFixture) assertNoFile(path string) {
	f.t.Helper()
	_, err := os.Stat(filepath.Join(f.root, filepath.FromSlash(path)))
	if !errors.Is(err, os.ErrNotExist) {
		f.t.Fatalf("unexpected file %s: %v", path, err)
	}
}
func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	var op *OpError
	if !errors.As(err, &op) || op.Code != code {
		t.Fatalf("error=%v; want %s", err, code)
	}
}
func creationCalls(away bool) []string {
	calls := []string{"claim", "tip-goal", "remote", "head-root", "tip-origin", "staged", "patch", "open", "apply", "commit", "head-scratch", "range-new", "claim", "close", "index", "unstaged", "changes", "headref"}
	if away {
		calls = append(calls, "worktrees")
	}
	return append(calls, "head-root", "checkout", "attach", "publish")
}

func TestCommitRefusesNonHolder(t *testing.T) {
	t.Parallel()
	f := newPolicyFixture(t)
	f.write("metasystem/code.go", "one")
	f.expect("claim-denied")
	req := f.req("non-holder")
	_, err := commitStaged(req, f.repository())
	requireCode(t, err, NotHolderCode)
	f.assertState(policyState{headRef: "refs/heads/main", headCommit: policyBase, index: policyIndex, status: "staged:metasystem/code.go"})
	f.assertNoFile("metasystem/other.go")
	if f.read("metasystem/code.go") != "one" {
		t.Fatal("staged bytes changed")
	}
	lost := newPolicyFixture(t)
	lost.write("metasystem/code.go", "one")
	lost.expect("claim", "tip-goal", "remote", "head-root", "tip-origin", "staged", "patch", "open", "apply", "commit", "head-scratch", "range-new", "claim-lost", "close")
	_, err = commitStaged(lost.req("claim-recheck"), lost.repository())
	requireCode(t, err, NotHolderCode)
	lost.assertState(policyState{headRef: "refs/heads/main", headCommit: policyBase, index: policyIndex, status: "staged:metasystem/code.go"})
	if lost.read("metasystem/code.go") != "one" {
		t.Fatal("lost claim changed staged bytes")
	}
}
func TestCommitRefusalsPreserveCheckout(t *testing.T) {
	t.Parallel()
	t.Run("class from another branch", func(t *testing.T) {
		t.Parallel()
		f := newPolicyFixture(t)
		f.local = policyLocal
		f.state.goal = policyLocal
		f.state.headRef = "refs/heads/other"
		f.observedHeadRef = "refs/heads/other"
		f.staged = []string{"metasystem/plans/wrong.md"}
		f.state.status = "staged:metasystem/plans/wrong.md"
		f.write("metasystem/plans/wrong.md", "wrong")
		f.expect("claim", "tip-goal", "range-local", "remote", "tip-origin", "staged")
		_, err := commitStaged(f.req("class-refusal"), f.repository())
		requireCode(t, err, RangeCode)
		f.assertState(policyState{headRef: "refs/heads/other", headCommit: policyBase, index: policyIndex, goal: policyLocal, status: "staged:metasystem/plans/wrong.md"})
		if f.read("metasystem/plans/wrong.md") != "wrong" {
			t.Fatal("staged bytes changed")
		}
	})
	t.Run("invalid range", func(t *testing.T) {
		t.Parallel()
		f := newPolicyFixture(t)
		f.local = policyLocal
		f.state.goal = policyLocal
		f.rangeErr = &RangeError{Code: RangeCode, Commit: policyLocal, Reason: "invalid"}
		f.write("metasystem/code.go", "one")
		f.expect("claim", "tip-goal", "range-local")
		_, err := commitStaged(f.req("range-refusal"), f.repository())
		var re *RangeError
		if !errors.As(err, &re) {
			t.Fatalf("range refusal=%v", err)
		}
		f.assertState(policyState{headRef: "refs/heads/main", headCommit: policyBase, index: policyIndex, goal: policyLocal, status: "staged:metasystem/code.go"})
		if f.read("metasystem/code.go") != "one" {
			t.Fatal("staged bytes changed")
		}
	})
	cleanup := newPolicyFixture(t)
	cleanup.remote = policyRemote
	cleanup.clearErr = errors.New("fetch cleanup failed")
	cleanup.expect("claim", "tip-goal", "remote", "fetch", "range-remote", "clear")
	_, err := commitStaged(cleanup.req("cleanup-error"), cleanup.repository())
	if !errors.Is(err, cleanup.clearErr) {
		t.Fatalf("cleanup failure=%v", err)
	}
	cleanup.assertState(policyState{headRef: "refs/heads/main", headCommit: policyBase, index: policyIndex, status: "staged:metasystem/code.go"})
	precedence := newPolicyFixture(t)
	precedence.remote = policyRemote
	precedence.remoteRangeErr = &RangeError{Code: RangeCode, Commit: policyRemote, Reason: "invalid remote range"}
	precedence.clearErr = errors.New("fetch cleanup failed")
	precedence.expect("claim", "tip-goal", "remote", "fetch", "range-remote", "clear")
	_, err = commitStaged(precedence.req("cleanup-precedence"), precedence.repository())
	if !errors.Is(err, precedence.remoteRangeErr) {
		t.Fatalf("range/cleanup precedence=%v", err)
	}
	precedence.assertState(policyState{headRef: "refs/heads/main", headCommit: policyBase, index: policyIndex, status: "staged:metasystem/code.go"})
}
func TestPlainCommitKeepsDirtyLedgerWithoutAdoption(t *testing.T) {
	t.Parallel()
	f := newPolicyFixture(t)
	f.write("metasystem/memory/receipts.log", "dirty ledger\n")
	f.write("metasystem/code.go", "one")
	f.unstaged = []string{"metasystem/memory/receipts.log"}
	f.state.status = "staged:metasystem/code.go;unstaged:metasystem/memory/receipts.log"
	f.installedStatus = "unstaged:metasystem/memory/receipts.log"
	f.expect(creationCalls(true)...)
	tip, err := commitStaged(f.req("plain-dirty-ledger"), f.repository())
	if err != nil || tip != policyNew {
		t.Fatalf("commit=%s err=%v", tip, err)
	}
	f.assertState(policyState{headRef: goalBranchRef("goal-a"), headCommit: policyNew, index: policyNewTree, goal: policyNew, status: "unstaged:metasystem/memory/receipts.log"})
	if got := f.read("metasystem/memory/receipts.log"); got != "dirty ledger\n" {
		t.Fatalf("ledger=%q", got)
	}
	if got := f.read("metasystem/code.go"); got != "one" {
		t.Fatalf("code=%q", got)
	}
}
func TestAdoptionUntrackedCollisionRefusesStale(t *testing.T) {
	t.Parallel()
	f := newPolicyFixture(t)
	f.local = policyLocal
	f.remote = policyRemote
	f.origin = policyLocal
	f.state.goal = policyLocal
	f.state.origin = policyLocal
	f.state.headRef = goalBranchRef("goal-a")
	f.observedHeadRef = goalBranchRef("goal-a")
	f.state.headCommit = policyLocal
	f.installPath = "metasystem/collision.go"
	f.write(f.installPath, "local untracked")
	f.write("metasystem/next.go", "next")
	f.staged = []string{"metasystem/next.go"}
	f.state.status = "staged:metasystem/next.go;untracked:metasystem/collision.go"
	f.checkoutErr = errors.New("Untracked working tree file would be overwritten")
	f.expect("claim", "tip-goal", "range-local", "remote", "fetch", "range-remote", "clear", "tip-origin", "ancestor", "staged", "unstaged", "patch", "open", "apply", "commit", "head-scratch", "range-new", "claim", "close", "index", "unstaged", "changes", "headref", "head-root", "checkout")
	_, err := commitStaged(f.req("collision-local"), f.repository())
	requireCode(t, err, StaleCode)
	if !strings.Contains(err.Error(), "Untracked working tree") {
		t.Fatalf("collision=%v", err)
	}
	f.assertState(policyState{headRef: goalBranchRef("goal-a"), headCommit: policyLocal, index: policyIndex, goal: policyLocal, origin: policyLocal, status: "staged:metasystem/next.go;untracked:metasystem/collision.go"})
	if f.read(f.installPath) != "local untracked" || f.read("metasystem/next.go") != "next" {
		t.Fatal("collision changed files")
	}
}
func TestCommitAdoptsRemoteWithoutEndpointCheckout(t *testing.T) {
	t.Parallel()
	f := newPolicyFixture(t)
	f.remote = policyRemote
	f.endpointHead = policyLocal
	f.state.headCommit = policyLocal
	f.write("unrelated-untracked.txt", "keep local bytes")
	f.write("metasystem/next.go", "next")
	f.staged = []string{"metasystem/next.go"}
	f.state.status = "staged:metasystem/next.go;untracked:unrelated-untracked.txt"
	f.installedStatus = "untracked:unrelated-untracked.txt"
	f.installPath = "metasystem/next.go"
	f.installBody = "next"
	f.expect("claim", "tip-goal", "remote", "fetch", "range-remote", "clear", "tip-origin", "staged", "unstaged", "patch", "open", "apply", "commit", "head-scratch", "range-new", "claim", "close", "index", "unstaged", "changes", "headref", "worktrees", "head-root", "checkout", "attach", "publish")
	tip, err := commitStaged(f.req("adopt-away-from-endpoint"), f.repository())
	if err != nil || tip != policyNew {
		t.Fatalf("adopt=%s err=%v", tip, err)
	}
	f.assertState(policyState{headRef: goalBranchRef("goal-a"), headCommit: policyNew, index: policyNewTree, goal: policyNew, origin: policyRemote, status: "untracked:unrelated-untracked.txt"})
	if f.read("unrelated-untracked.txt") != "keep local bytes" || f.read("metasystem/next.go") != "next" {
		t.Fatal("adoption changed unrelated bytes")
	}
}
func TestFailedCommitLeavesCheckoutAndRefsUnchanged(t *testing.T) {
	t.Parallel()
	f := newPolicyFixture(t)
	f.commitErr = errors.New("fixture hook refusal")
	f.write("metasystem/code.go", "one")
	f.expect("claim", "tip-goal", "remote", "head-root", "tip-origin", "staged", "patch", "open", "apply", "commit", "close")
	_, err := commitStaged(f.req("hook-refusal"), f.repository())
	if err == nil || !strings.Contains(err.Error(), "fixture hook refusal") {
		t.Fatalf("hook refusal=%v", err)
	}
	f.assertState(policyState{headRef: "refs/heads/main", headCommit: policyBase, index: policyIndex, status: "staged:metasystem/code.go"})
	if f.read("metasystem/code.go") != "one" {
		t.Fatal("hook changed file")
	}
	f.assertNoFile("metasystem/other.go")
}
