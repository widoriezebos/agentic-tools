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
	amendNew  = "5555555555555555555555555555555555555555"
	amendRead = "6666666666666666666666666666666666666666"
	amendPlan = "7777777777777777777777777777777777777777"
	amendTail = "8888888888888888888888888888888888888888"
)

// amendFixture adds scripted amend observations to the existing strict policy fixture.
type amendFixture struct {
	*policyFixture
	ranges                                 map[string][]Commit
	next, tree, openBase, subject, trailer string
	suffix                                 []string
	kinds                                  map[string]KindInfo
	entries                                map[string][]Entry
	omitted                                []string
	omittedTree                            string
	publishErr, restoreErr                 error
	checkoutBackErr                        error
	applyErr                               error
	amending                               bool
	oldIndex, oldStatus, oldRef, oldHead   string
	installFiles                           map[string]*string
	beforeFiles                            map[string]*string
}

func newAmendFixture(t *testing.T) *amendFixture {
	f := &amendFixture{policyFixture: newPolicyFixture(t), ranges: map[string][]Commit{}, next: amendNew, tree: policyIndex, openBase: policyLocal,
		subject: "goal goal-a units u1", trailer: "Goal-Unit: goal-a/u1", kinds: map[string]KindInfo{}, entries: map[string][]Entry{}, installFiles: map[string]*string{}}
	f.local = policyLocal
	f.state.goal = policyLocal
	f.state.headRef = goalBranchRef("goal-a")
	f.state.headCommit = policyLocal
	f.observedHeadRef = goalBranchRef("goal-a")
	f.ranges[policyLocal] = []Commit{{ID: policyLocal, Kind: Unit, Unit: "u1", Units: []string{"u1"}}}
	f.ranges[amendNew] = []Commit{{ID: amendNew, Kind: Unit, Unit: "u1", Units: []string{"u1"}}}
	f.write("metasystem/code.go", "two")
	f.installFiles["metasystem/code.go"] = strptr("two")
	return f
}
func strptr(s string) *string { return &s }
func cloneCommits(in []Commit) []Commit {
	out := append([]Commit(nil), in...)
	for i := range out {
		out[i].Units = append([]string(nil), out[i].Units...)
	}
	return out
}
func (f *amendFixture) repository() commitRepository {
	r := f.policyFixture.repository()
	r.facts.Head = func(repo string) (string, error) {
		if repo == f.root {
			f.step("head-root")
			return f.state.headCommit, nil
		}
		f.step("head-scratch")
		f.args(repo == f.scratch, "Head", repo)
		return f.next, nil
	}
	r.facts.HeadRef = func(repo string) string {
		f.step("headref")
		f.args(repo == f.root, "HeadRef", repo)
		return f.state.headRef
	}
	r.facts.Tree = func(repo string) (string, error) {
		f.step("tree")
		f.args(repo == f.scratch, "Tree", repo)
		return f.tree, nil
	}
	r.facts.Index = func(repo string) (string, error) {
		f.step("index")
		f.args(repo == f.root, "Index", repo)
		return f.state.index, nil
	}
	r.facts.Range = func(repo, endpoint, tip, goal string) ([]Commit, error) {
		name := "range-local"
		if repo == f.scratch {
			name = "range-new"
		} else if tip == f.remote && f.remote != "" {
			name = "range-remote"
		}
		f.step(name)
		f.args((repo == f.root || repo == f.scratch) && endpoint == policyBase && goal == "goal-a" && ((repo == f.root && (tip == f.local || tip == f.remote)) || (repo == f.scratch && tip == f.next)), "Range", repo, endpoint, tip, goal)
		if name == "range-local" && f.rangeErr != nil {
			return nil, f.rangeErr
		}
		if name == "range-remote" && f.remoteRangeErr != nil {
			return nil, f.remoteRangeErr
		}
		v, ok := f.ranges[tip]
		if !ok {
			f.t.Fatalf("missing scripted range for %s", tip)
		}
		return cloneCommits(v), nil
	}
	r.facts.Suffix = func(repo, target, tip string) ([]string, error) {
		f.step("suffix")
		f.args(repo == f.root && target == f.openBase && tip == f.local, "Suffix", repo, target, tip)
		return append([]string(nil), f.suffix...), nil
	}
	r.facts.Kind = func(repo, commit, goal string) (KindInfo, error) {
		f.step("kind:" + commit)
		f.args(repo == f.scratch && goal == "goal-a", "Kind", repo, commit, goal)
		v, ok := f.kinds[commit]
		if !ok {
			f.t.Fatalf("missing kind %s", commit)
		}
		v.Units = append([]string(nil), v.Units...)
		return v, nil
	}
	r.facts.Entries = func(repo, commit string) ([]Entry, error) {
		f.step("entries:" + commit)
		f.args(repo == f.scratch, "Entries", repo, commit)
		v, ok := f.entries[commit]
		if !ok {
			f.t.Fatalf("missing entries %s", commit)
		}
		return append([]Entry(nil), v...), nil
	}
	r.facts.Changes = func(repo, index, tip string) ([]string, error) {
		f.step("changes")
		f.args(repo == f.root && index == f.state.index && tip == f.next, "Changes", repo, index, tip)
		return append([]string(nil), f.changes...), nil
	}
	r.effects.Open = func(repo, base string, amend bool) (string, func(), error) {
		f.step("open")
		f.args(repo == f.root && base == f.openBase && amend == f.wantAmend(), "Open", repo, base, amend)
		return f.scratch, func() { f.step("close") }, nil
	}
	r.effects.Commit = func(dir, subject, trailer string, amend bool) error {
		f.step("commit")
		f.args(dir == f.scratch && subject == f.subject && trailer == f.trailer && amend == f.wantAmend(), "Commit", dir, subject, trailer, amend)
		return f.commitErr
	}
	r.effects.Apply = func(dir string, patch []byte) error {
		f.step("apply")
		f.args(dir == f.scratch && bytes.Equal(patch, f.patch), "Apply", dir, patch)
		return f.applyErr
	}
	r.effects.Replay = func(dir, commit string) error {
		f.step("replay:" + commit)
		f.args(dir == f.scratch, "Replay", dir, commit)
		return nil
	}
	r.effects.WithoutPaths = func(repo, tree string, paths []string) (string, error) {
		f.step("without")
		f.args(repo == f.root && tree == f.state.index && reflect.DeepEqual(paths, f.omitted), "WithoutPaths", repo, tree, paths)
		if f.omittedTree != "" {
			return f.omittedTree, nil
		}
		return tree, nil
	}
	r.effects.Checkout = func(repo, before, after string) error {
		f.args(repo == f.root, "Checkout repo", repo, before, after)
		if after == f.next {
			f.step("checkout")
			f.args(before == f.state.index, "Checkout", before, after)
			if f.checkoutErr != nil {
				return f.checkoutErr
			}
			f.beforeFiles = map[string]*string{}
			for p, v := range f.installFiles {
				f.beforeFiles[p] = f.fileValue(p)
				f.setFile(p, v)
			}
			f.state.index = f.tree
			f.state.status = f.installedStatus
			return nil
		}
		f.step("checkout-back")
		f.args(before == f.next && after == f.oldIndex, "Checkout rollback", before, after)
		if f.checkoutBackErr != nil {
			return f.checkoutBackErr
		}
		for p, v := range f.beforeFiles {
			f.setFile(p, v)
		}
		f.state.index = f.oldIndex
		f.state.status = f.oldStatus
		return nil
	}
	r.effects.Restore = func(repo, ref, commit string) error {
		f.step("restore")
		f.args(repo == f.root && ref == f.oldRef && commit == f.oldHead, "Restore", repo, ref, commit)
		if f.restoreErr != nil {
			return f.restoreErr
		}
		f.state.headRef = ref
		f.state.headCommit = commit
		return nil
	}
	r.effects.Publish = func(repo, goal, old, next, origin string) error {
		f.step("publish")
		f.args(repo == f.root && goal == "goal-a" && old == f.local && next == f.next && origin == f.originValue(), "Publish", repo, goal, old, next, origin)
		if f.publishErr != nil {
			return f.publishErr
		}
		f.local = next
		f.state.goal = next
		f.state.origin = origin
		f.state.headCommit = next
		return nil
	}
	return r
}
func (f *amendFixture) wantAmend() bool { return f.amending }
func (f *amendFixture) fileValue(p string) *string {
	b, e := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(p)))
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e != nil {
		f.t.Fatal(e)
	}
	return strptr(string(b))
}
func (f *amendFixture) setFile(p string, v *string) {
	full := filepath.Join(f.root, filepath.FromSlash(p))
	if v == nil {
		if e := os.Remove(full); e != nil && !errors.Is(e, os.ErrNotExist) {
			f.t.Fatal(e)
		}
		return
	}
	f.write(p, *v)
}
func (f *amendFixture) observe(want policyState, files map[string]*string) {
	f.assertState(want)
	for p, v := range files {
		got := f.fileValue(p)
		if !reflect.DeepEqual(got, v) {
			f.t.Fatalf("after file %s=%v want %v", p, got, v)
		}
	}
}
func (f *amendFixture) prepare(amend bool) {
	f.amending = amend
	f.oldIndex = f.state.index
	f.oldStatus = f.state.status
	f.oldRef = f.state.headRef
	f.oldHead = f.state.headCommit
}
func (f *amendFixture) amendRequest(op string) CommitRequest {
	r := f.req(op)
	r.Amend = true
	return r
}
func amendCalls(away bool, suffix ...string) []string {
	c := []string{"claim", "tip-goal", "range-local", "remote", "tip-origin", "staged", "range-local", "patch", "index", "suffix", "open", "apply", "commit"}
	for _, s := range suffix {
		c = append(c, "kind:"+s)
		if s == amendRead {
			c = append(c, "entries:"+s)
		} else {
			c = append(c, "replay:"+s)
		}
	}
	c = append(c, "head-scratch", "tree", "without", "range-new", "claim", "index", "unstaged", "changes", "headref")
	if away {
		c = append(c, "worktrees")
	}
	c = append(c, "head-root", "checkout")
	if away {
		c = append(c, "attach")
	}
	return append(c, "publish", "close")
}

func callPrefix(calls []string, stop string) []string {
	for i, c := range calls {
		if c == stop {
			return append([]string(nil), calls[:i]...)
		}
	}
	panic("missing call " + stop)
}
func (f *amendFixture) run(req CommitRequest) (string, error) {
	f.prepare(req.Amend)
	return commitStaged(req, f.repository())
}
func TestCommitCreatesBranchAndAmendsUnit(t *testing.T) {
	t.Parallel()
	f := newAmendFixture(t)
	f.local = ""
	f.state = policyState{headRef: "refs/heads/main", headCommit: policyBase, index: policyIndex, status: "staged:plan"}
	f.observedHeadRef = "refs/heads/main"
	f.openBase = policyBase
	f.next = policyNew
	f.subject = "goal goal-a plan"
	f.trailer = "Goal-Plan: goal-a"
	f.staged = []string{"metasystem/plans/goal-a.md"}
	f.installFiles = map[string]*string{"metasystem/plans/goal-a.md": strptr("plan")}
	f.ranges[policyNew] = []Commit{{ID: policyNew, Kind: Plan}}
	f.expect(creationCalls(true)...)
	planReq := f.req("commit-plan")
	planReq.Kind = Plan
	planReq.Unit = ""
	plan, e := f.run(planReq)
	if e != nil || plan != policyNew {
		t.Fatalf("plan=%s err=%v", plan, e)
	}
	f.observe(policyState{headRef: goalBranchRef("goal-a"), headCommit: policyNew, index: policyIndex, goal: policyNew, status: ""}, map[string]*string{"metasystem/plans/goal-a.md": strptr("plan")})
	f.next = policyLocal
	f.openBase = plan
	f.subject = "goal goal-a units u1"
	f.trailer = "Goal-Unit: goal-a/u1"
	f.staged = []string{"metasystem/code.go"}
	f.state.status = "staged:code"
	f.installFiles = map[string]*string{"metasystem/code.go": strptr("one")}
	f.write("metasystem/code.go", "one")
	f.ranges[policyLocal] = []Commit{{ID: plan, Kind: Plan}, {ID: policyLocal, Kind: Unit, Unit: "u1", Units: []string{"u1"}}}
	f.expect("claim", "tip-goal", "range-local", "remote", "tip-origin", "staged", "patch", "open", "apply", "commit", "head-scratch", "range-new", "claim", "close", "index", "unstaged", "changes", "headref", "head-root", "checkout", "publish")
	unit, e := f.run(f.req("commit-u1"))
	if e != nil || unit != policyLocal {
		t.Fatalf("unit=%s err=%v", unit, e)
	}
	f.observe(policyState{headRef: goalBranchRef("goal-a"), headCommit: policyLocal, index: policyIndex, goal: policyLocal, status: ""}, map[string]*string{"metasystem/code.go": strptr("one")})
	f.next = amendNew
	f.openBase = unit
	f.ranges[amendNew] = []Commit{{ID: plan, Kind: Plan}, {ID: amendNew, Kind: Unit, Unit: "u1", Units: []string{"u1"}}}
	f.write("metasystem/code.go", "two")
	f.state.status = "staged:code"
	f.installFiles = map[string]*string{"metasystem/code.go": strptr("two")}
	f.expect(amendCalls(false)...)
	replacement, e := f.run(f.amendRequest("amend-u1"))
	if e != nil || replacement == unit || replacement != amendNew {
		t.Fatalf("replacement=%s old=%s err=%v", replacement, unit, e)
	}
	if got := f.ranges[replacement]; len(got) != 2 || got[1].Unit != "u1" {
		t.Fatalf("replacement range=%+v", got)
	}
	f.observe(policyState{headRef: goalBranchRef("goal-a"), headCommit: amendNew, index: policyIndex, goal: amendNew, status: ""}, map[string]*string{"metasystem/code.go": strptr("two")})
	f.next = policyRemote
	f.openBase = replacement
	f.subject = "goal goal-a plan"
	f.trailer = "Goal-Plan: goal-a"
	f.staged = []string{"metasystem/plans/after.md"}
	f.state.status = "staged:tail"
	f.installFiles = map[string]*string{"metasystem/plans/after.md": strptr("later plan")}
	f.ranges[policyRemote] = []Commit{{ID: plan, Kind: Plan}, {ID: replacement, Kind: Unit, Unit: "u1", Units: []string{"u1"}}, {ID: policyRemote, Kind: Plan}}
	f.expect("claim", "tip-goal", "range-local", "remote", "tip-origin", "staged", "patch", "open", "apply", "commit", "head-scratch", "range-new", "claim", "close", "index", "unstaged", "changes", "headref", "head-root", "checkout", "publish")
	tailReq := f.req("commit-tail")
	tailReq.Kind = Plan
	tailReq.Unit = ""
	tail, e := f.run(tailReq)
	if e != nil || tail != policyRemote {
		t.Fatalf("tail=%s err=%v", tail, e)
	}
	f.next = amendTail
	f.openBase = replacement
	f.subject = "goal goal-a units u1"
	f.trailer = "Goal-Unit: goal-a/u1"
	f.staged = []string{"metasystem/code.go"}
	f.write("metasystem/code.go", "three")
	f.state.status = "staged:code"
	f.installFiles = map[string]*string{"metasystem/code.go": strptr("three")}
	f.suffix = []string{tail}
	f.kinds[tail] = KindInfo{Kind: Plan}
	f.ranges[amendTail] = []Commit{{ID: plan, Kind: Plan}, {ID: amendNew, Kind: Unit, Unit: "u1", Units: []string{"u1"}}, {ID: amendTail, Kind: Plan}}
	f.expect(amendCalls(false, tail)...)
	rewritten, e := f.run(f.amendRequest("amend-tail"))
	if e != nil || rewritten == tail || rewritten != amendTail {
		t.Fatalf("rewritten=%s tail=%s err=%v", rewritten, tail, e)
	}
	if got := f.ranges[rewritten]; len(got) != 3 || got[1].Unit != "u1" {
		t.Fatalf("rewritten range=%+v", got)
	}
	f.observe(policyState{headRef: goalBranchRef("goal-a"), headCommit: amendTail, index: policyIndex, goal: amendTail, status: ""}, map[string]*string{"metasystem/code.go": strptr("three"), "metasystem/plans/after.md": strptr("later plan")})
}

func TestAmendKeepsUnstagedTrackedEdits(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		away bool
	}{{"on goal branch", false}, {"away from goal branch", true}} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newAmendFixture(t)
			if tc.away {
				f.state.headRef = "refs/heads/other"
				f.state.headCommit = policyBase
				f.observedHeadRef = "refs/heads/other"
			}
			f.state.status = "staged:code unstaged:keep-a,keep-b"
			f.installedStatus = "unstaged:metasystem/keep-a.go,metasystem/keep-b.go"
			f.unstaged = []string{"metasystem/keep-a.go", "metasystem/keep-b.go"}
			a, b := "local a\nwith another line\n", "local b\n"
			f.write("metasystem/keep-a.go", a)
			f.write("metasystem/keep-b.go", b)
			f.expect(amendCalls(tc.away)...)
			tip, e := f.run(f.amendRequest("amend-with-unstaged"))
			if e != nil || tip != amendNew {
				t.Fatalf("tip=%s err=%v", tip, e)
			}
			f.observe(policyState{headRef: goalBranchRef("goal-a"), headCommit: amendNew, index: policyIndex, goal: amendNew, status: "unstaged:metasystem/keep-a.go,metasystem/keep-b.go"}, map[string]*string{"metasystem/code.go": strptr("two"), "metasystem/keep-a.go": &a, "metasystem/keep-b.go": &b})
		})
	}
}

func TestAmendRefusesBeforeOverwritingUnstagedTrackedEdit(t *testing.T) {
	t.Parallel()
	f := newAmendFixture(t)
	path := "metasystem/records/reads/goal-a/" + policyLocal + ".json"
	f.write(path, "locally edited read")
	f.unstaged = []string{path}
	f.changes = []string{path}
	f.state.status = "staged:code unstaged:read"
	f.suffix = []string{amendRead}
	f.kinds[amendRead] = KindInfo{Kind: Read, CommitID: policyLocal}
	f.entries[amendRead] = []Entry{{Path: path}}
	f.omitted = []string{path}
	f.omittedTree = policyIndex
	before := f.state
	calls := callPrefix(amendCalls(false, amendRead), "headref")
	f.expect(append(calls, "close")...)
	_, e := f.run(f.amendRequest("amend-overwrite-refusal"))
	requireCode(t, e, StaleCode)
	f.observe(before, map[string]*string{path: strptr("locally edited read"), "metasystem/code.go": strptr("two")})
}

func testAmendRollback(t *testing.T, away bool) {
	t.Helper()
	f := newAmendFixture(t)
	if away {
		f.state.headRef = "refs/heads/other"
		f.state.headCommit = policyBase
		f.observedHeadRef = "refs/heads/other"
	}
	f.state.status = "staged:code"
	f.publishErr = errors.New("cannot lock ref refs/heads/goal/goal-a")
	files := map[string]*string{"metasystem/code.go": strptr("two")}
	if away {
		path := "metasystem/records/reads/goal-a/" + policyLocal + ".json"
		f.write(path, "{}")
		files[path] = strptr("{}")
		f.installFiles[path] = nil
		f.suffix = []string{amendRead}
		f.kinds[amendRead] = KindInfo{Kind: Read, CommitID: policyLocal}
		f.entries[amendRead] = []Entry{{Path: path}}
		f.omitted = []string{path}
		f.omittedTree = policyIndex
	} else {
		f.write("metasystem/keep-a.go", "local a")
		files["metasystem/keep-a.go"] = strptr("local a")
	}
	before := f.state
	calls := amendCalls(away, f.suffix...)
	calls = append(calls[:len(calls)-1], "restore", "checkout-back", "close")
	f.expect(calls...)
	_, e := f.run(f.amendRequest("amend-install-failure"))
	if e == nil || !strings.Contains(e.Error(), "cannot lock ref") {
		t.Fatalf("lock error=%v", e)
	}
	f.observe(before, files)
}
func TestAmendInstallFailureRestoresCheckout(t *testing.T) { t.Parallel(); testAmendRollback(t, false) }
func TestAmendInstallFailureRollsBackMovedCheckout(t *testing.T) {
	t.Parallel()
	testAmendRollback(t, true)
}

func TestAmendRechecksClaimBeforeBranchUpdate(t *testing.T) {
	t.Parallel()
	f := newAmendFixture(t)
	before := f.state
	calls := amendCalls(false)
	i := 0
	for n, c := range calls {
		if c == "claim" {
			i++
			if i == 2 {
				calls = append(append([]string(nil), calls[:n]...), "claim-lost", "close")
				break
			}
		}
	}
	f.expect(calls...)
	_, e := f.run(f.amendRequest("claim-recheck"))
	requireCode(t, e, NotHolderCode)
	f.observe(before, map[string]*string{"metasystem/code.go": strptr("two")})
}

func TestAmendDropsReadOfReplacedBuildAndReplaysLaterPlan(t *testing.T) {
	t.Parallel()
	f := newAmendFixture(t)
	path := "metasystem/records/reads/goal-a/" + policyLocal + ".json"
	plan := "metasystem/plans/later.md"
	f.write(path, "{}")
	f.write(plan, "later")
	f.suffix = []string{amendRead, amendPlan}
	f.kinds[amendRead] = KindInfo{Kind: Read, CommitID: policyLocal}
	f.kinds[amendPlan] = KindInfo{Kind: Plan}
	f.entries[amendRead] = []Entry{{Path: path}}
	f.omitted = []string{path}
	f.omittedTree = policyIndex
	f.installFiles[path] = nil
	f.installFiles[plan] = strptr("later")
	f.ranges[amendNew] = []Commit{{ID: amendNew, Kind: Unit, Unit: "u1", Units: []string{"u1"}}, {ID: amendPlan, Kind: Plan}}
	f.expect(amendCalls(false, f.suffix...)...)
	tip, e := f.run(f.amendRequest("amend-stale-read"))
	if e != nil || tip != amendNew {
		t.Fatalf("tip=%s err=%v", tip, e)
	}
	got := f.ranges[tip]
	if len(got) != 2 || got[0].Kind != Unit || got[1].Kind != Plan {
		t.Fatalf("rewritten suffix=%+v", got)
	}
	f.observe(policyState{headRef: goalBranchRef("goal-a"), headCommit: amendNew, index: policyIndex, goal: amendNew, status: ""}, map[string]*string{path: nil, plan: strptr("later"), "metasystem/code.go": strptr("two")})
}

func TestAmendDroppingReadStillChecksReplayedTree(t *testing.T) {
	t.Parallel()
	f := newAmendFixture(t)
	path := "metasystem/records/reads/goal-a/" + policyLocal + ".json"
	f.write(path, "{}")
	f.suffix = []string{amendRead}
	f.kinds[amendRead] = KindInfo{Kind: Read, CommitID: policyLocal}
	f.entries[amendRead] = []Entry{{Path: path}}
	f.omitted = []string{path}
	f.omittedTree = policyIndex
	f.tree = "injected-tree"
	before := f.state
	calls := callPrefix(amendCalls(false, amendRead), "range-new")
	f.expect(append(calls, "close")...)
	_, e := f.run(f.amendRequest("amend-tree-check"))
	requireCode(t, e, RangeCode)
	if !strings.Contains(e.Error(), "replayed branch") {
		t.Fatal(e)
	}
	f.observe(before, map[string]*string{path: strptr("{}"), "metasystem/code.go": strptr("two")})
}

func TestAmendReplacesWholeBuildList(t *testing.T) {
	t.Parallel()
	units := []string{"5", "6", "7a", "7b"}
	f := newAmendFixture(t)
	f.ranges[policyLocal] = []Commit{{ID: policyLocal, Kind: Unit, Units: append([]string(nil), units...)}}
	f.ranges[amendNew] = []Commit{{ID: amendNew, Kind: Unit, Units: append([]string(nil), units...)}}
	f.subject = "goal goal-a units 5+6+7a+7b"
	f.trailer = "Goal-Unit: goal-a/5+6+7a+7b"
	f.staged = []string{"metasystem/multi.go"}
	f.local = ""
	f.state = policyState{headRef: "refs/heads/main", headCommit: policyBase, index: policyIndex, status: "staged:multi"}
	f.next = policyLocal
	f.openBase = policyBase
	f.write("metasystem/multi.go", "one")
	f.installFiles = map[string]*string{"metasystem/multi.go": strptr("one")}
	f.expect(creationCalls(true)...)
	create := f.req("multi-build")
	create.Unit = ""
	create.Units = units
	old, e := f.run(create)
	if e != nil || old != policyLocal {
		t.Fatalf("created multi=%s err=%v", old, e)
	}
	f.observe(policyState{headRef: goalBranchRef("goal-a"), headCommit: policyLocal, index: policyIndex, goal: policyLocal, status: ""}, map[string]*string{"metasystem/multi.go": strptr("one")})
	f.next = amendNew
	f.openBase = old
	f.state.status = "staged:multi"
	f.write("metasystem/multi.go", "two")
	f.installFiles = map[string]*string{"metasystem/multi.go": strptr("two")}
	f.expect(amendCalls(false)...)
	req := f.amendRequest("multi-amend")
	req.Unit = ""
	req.Units = units
	tip, e := f.run(req)
	if e != nil || tip == old || tip != amendNew {
		t.Fatalf("multi tip=%s err=%v", tip, e)
	}
	if got := f.ranges[tip][0].Units; !reflect.DeepEqual(got, units) {
		t.Fatalf("units=%v", got)
	}
	f.observe(policyState{headRef: goalBranchRef("goal-a"), headCommit: amendNew, index: policyIndex, goal: amendNew, status: ""}, map[string]*string{"metasystem/multi.go": strptr("two")})
	for _, mode := range []string{"duplicate", "missing"} {
		g := newAmendFixture(t)
		if mode == "duplicate" {
			g.ranges[policyLocal] = append(g.ranges[policyLocal], Commit{ID: amendPlan, Kind: Unit, Unit: "u1", Units: []string{"u1"}})
		} else {
			g.ranges[policyLocal] = []Commit{{ID: policyLocal, Kind: Plan}}
		}
		before := g.state
		g.expect("claim", "tip-goal", "range-local", "remote", "tip-origin", "staged", "range-local")
		_, e := g.run(g.amendRequest(mode))
		requireCode(t, e, RangeCode)
		g.observe(before, map[string]*string{"metasystem/code.go": strptr("two")})
	}
	unrelated := newAmendFixture(t)
	unrelated.suffix = []string{policyRemote}
	unrelated.kinds[policyRemote] = KindInfo{Kind: Read, CommitID: amendPlan}
	unrelated.expect(amendCalls(false, policyRemote)...)
	_, e = unrelated.run(unrelated.amendRequest("unrelated-read"))
	if e != nil {
		t.Fatal(e)
	}
	unrelated.observe(policyState{headRef: goalBranchRef("goal-a"), headCommit: amendNew, index: policyIndex, goal: amendNew, status: ""}, map[string]*string{"metasystem/code.go": strptr("two")})
	cleanup := newAmendFixture(t)
	cleanup.applyErr = errors.New("apply failed")
	before := cleanup.state
	cleanup.expect("claim", "tip-goal", "range-local", "remote", "tip-origin", "staged", "range-local", "patch", "index", "suffix", "open", "apply", "close")
	_, e = cleanup.run(cleanup.amendRequest("apply-failed"))
	requireCode(t, e, RangeCode)
	if !strings.Contains(e.Error(), "apply failed") {
		t.Fatal(e)
	}
	cleanup.observe(before, map[string]*string{"metasystem/code.go": strptr("two")})
	for _, invalid := range []bool{false, true} {
		fetch := newAmendFixture(t)
		fetch.local = ""
		fetch.state.goal = ""
		fetch.remote = policyRemote
		fetch.ranges[policyRemote] = []Commit{{ID: policyRemote, Kind: Unit, Unit: "u1", Units: []string{"u1"}}}
		fetch.clearErr = errors.New("fetch cleanup failed")
		if invalid {
			fetch.remoteRangeErr = &RangeError{Code: RangeCode, Commit: policyRemote, Reason: "invalid remote range"}
		}
		before := fetch.state
		fetch.expect("claim", "tip-goal", "remote", "fetch", "range-remote", "clear")
		_, err := fetch.run(fetch.amendRequest("fetch-cleanup"))
		want := fetch.clearErr
		if invalid {
			want = fetch.remoteRangeErr
		}
		if !errors.Is(err, want) {
			t.Fatalf("fetch cleanup precedence invalid=%t error=%v", invalid, err)
		}
		fetch.observe(before, map[string]*string{"metasystem/code.go": strptr("two")})
	}
}
