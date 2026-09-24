package branch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// followonScenario holds the remote and declared commit facts used by both verbs.
// Each client owns its checkout and refs; only remote state is shared.
type followonScenario struct {
	t         *testing.T
	remote    map[string]string
	ranges    map[string][]Commit
	trees     map[string][]string
	files     map[string]map[string]string
	known     map[string]bool
	ancestors map[[2]string]bool
	parents   map[string]string
	next      int
}
type followonClient struct {
	s            *followonScenario
	push         *pushPolicyFixture
	commit       *amendFixture
	ancestorWant [][2]string
	ranges       map[string][]Commit
	trees        map[string][]string
	files        map[string]map[string]string
	ancestors    map[[2]string]bool
}

func newFollowonScenario(t *testing.T) *followonScenario {
	t.Helper()
	return &followonScenario{t: t, remote: map[string]string{}, ranges: map[string][]Commit{}, trees: map[string][]string{}, files: map[string]map[string]string{}, known: map[string]bool{}, ancestors: map[[2]string]bool{}, parents: map[string]string{}}
}
func (s *followonScenario) id() string { s.next++; return fmt.Sprintf("%040x", s.next+100) }
func (s *followonScenario) child(parent, unit, path, body string) string {
	tip := s.id()
	commits := cloneCommits(s.ranges[parent])
	commits = append(commits, Commit{ID: tip, Kind: Unit, Unit: unit, Units: []string{unit}})
	s.ranges[tip] = commits
	s.files[tip] = cloneFileTree(s.files[parent])
	s.files[tip][path] = body
	s.known[path] = true
	s.trees[tip] = sortedPaths(s.files[tip])
	s.finishGraph(tip)
	s.ancestors[[2]string{parent, tip}] = true
	for pair, yes := range s.ancestors {
		if yes && pair[1] == parent {
			s.ancestors[[2]string{pair[0], tip}] = true
		}
	}
	return tip
}
func (s *followonScenario) replacement(old, unit, path, body string) string {
	tip := s.id()
	commits := cloneCommits(s.ranges[old])
	if len(commits) == 0 || commits[len(commits)-1].Unit != unit {
		s.t.Fatalf("cannot replace %s unit %s", old, unit)
	}
	commits[len(commits)-1] = Commit{ID: tip, Kind: Unit, Unit: unit, Units: []string{unit}}
	s.ranges[tip] = commits
	s.files[tip] = cloneFileTree(s.files[old])
	s.files[tip][path] = body
	s.known[path] = true
	s.trees[tip] = sortedPaths(s.files[tip])
	return tip
}
func cloneFileTree(in map[string]string) map[string]string {
	out := map[string]string{}
	for p, v := range in {
		out[p] = v
	}
	return out
}
func sortedPaths(in map[string]string) []string {
	out := make([]string, 0, len(in))
	for p := range in {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
func (c *followonClient) installTip(tip string) {
	if _, ok := c.s.ranges[tip]; !ok {
		c.s.t.Fatalf("undeclared local/fetched tip %s", tip)
	}
	for _, record := range c.s.ranges[tip] {
		id := record.ID
		c.ranges[id] = cloneCommits(c.s.ranges[id])
		c.trees[id] = append([]string(nil), c.s.trees[id]...)
		c.files[id] = cloneFileTree(c.s.files[id])
	}
	c.ranges[tip] = cloneCommits(c.s.ranges[tip])
	c.trees[tip] = append([]string(nil), c.s.trees[tip]...)
	c.files[tip] = cloneFileTree(c.s.files[tip])
	for pair, value := range c.s.ancestors {
		if _, old := c.ranges[pair[0]]; old {
			if _, newer := c.ranges[pair[1]]; newer {
				c.ancestors[pair] = value
			}
		}
	}
}
func (s *followonScenario) client() *followonClient {
	s.t.Helper()
	p := newPushPolicyFixture(s.t)
	a := newAmendFixture(s.t)
	a.root = p.repo
	a.local = ""
	a.remote = ""
	a.origin = ""
	a.state = policyState{headRef: "refs/heads/main", headCommit: policyBase, index: policyIndex}
	a.observedHeadRef = "refs/heads/main"
	a.installFiles = map[string]*string{}
	a.setFile("metasystem/code.go", nil)
	p.endpoint = policyBase
	p.remote = s.remote
	p.head = "refs/heads/main"
	p.checkout = policyBase
	c := &followonClient{s: s, push: p, commit: a, ranges: map[string][]Commit{}, trees: map[string][]string{}, files: map[string]map[string]string{}, ancestors: map[[2]string]bool{}}
	p.ranges = c.ranges
	p.trees = c.trees
	p.ancestors = c.ancestors
	a.ranges = c.ranges
	p.afterFetch = c.installTip
	p.afterOrigin = func(tip string) { a.state.origin = tip; a.origin = tip }
	p.afterMove = func(branch, origin string) {
		a.local = branch
		a.state.goal = branch
		a.state.origin = origin
		a.origin = origin
	}
	p.afterCheckout = func(tip string) {
		target, ok := c.files[tip]
		if !ok {
			s.t.Fatalf("undeclared checkout %s", tip)
		}
		for path := range s.known {
			if body, ok := target[path]; ok {
				a.setFile(path, strptr(body))
			} else {
				a.setFile(path, nil)
			}
		}
		a.state.headRef = p.head
		a.state.headCommit = tip
		a.state.index = "tree:" + tip
		a.state.status = ""
		p.staged = false
		p.unstaged = false
		a.staged = nil
	}
	return c
}
func (c *followonClient) request(op string) PushRequest { return c.push.request(op) }
func (c *followonClient) runPush(req PushRequest, calls ...pushExpectation) (PushResult, error) {
	c.push.expect(calls...)
	got, err := pushWithRepository(req, c.push.repository())
	c.push.done()
	return got, err
}
func ev(op string) pushExpectation { return pe(op) }
func remoteEvent() pushExpectation {
	return pushExpectation{call: pushEvent{operation: "remote", remote: "origin", ref: policyGoalRef}}
}
func tipEvent(ref string) pushExpectation {
	return pushExpectation{call: pushEvent{operation: "tip", ref: ref}}
}
func rangeEvent(tip string) pushExpectation {
	return pushExpectation{call: pushEvent{operation: "range", endpoint: policyBase, tip: tip, goal: "goal-a"}}
}
func fetchEvents(op, tip string) []pushExpectation {
	return []pushExpectation{{call: pushEvent{operation: "fetch", remote: "origin", ref: policyGoalRef, tip: fetchRef(op)}}, rangeEvent(tip), {call: pushEvent{operation: "clear", ref: fetchRef(op)}}}
}
func ancestryEvent(old, new string) pushExpectation {
	return pushExpectation{call: pushEvent{operation: "ancestor", older: old, newer: new}}
}
func originEvent(tip string) pushExpectation {
	return pushExpectation{call: pushEvent{operation: "origin", goal: "goal-a", tip: tip}}
}
func clearEvent(op string) pushExpectation {
	return pushExpectation{call: pushEvent{operation: "clear", ref: txnRef(op)}}
}
func (c *followonClient) publishCalls(op, tip string, outcome CASOutcome, pushErr error) []pushExpectation {
	p := c.push
	remote := p.remote[policyGoalRef]
	calls := []pushExpectation{ev("claim"), ev("txn-refs"), remoteEvent(), tipEvent(policyGoalRef), rangeEvent(tip), tipEvent(originTipRef("goal-a"))}
	if remote != "" {
		calls = append(calls, fetchEvents(op, remote)...)
		calls = append(calls, ancestryEvent(tip, remote))
		calls = append(calls, ancestryEvent(remote, tip))
	}
	txn := pushTxn{SchemaVersion: 1, Goal: "goal-a", Remote: "origin", Ref: policyGoalRef, Expected: remote, New: tip}
	calls = append(calls, pushExpectation{call: pushEvent{operation: "write-txn", opid: op, txn: txn}}, pushExpectation{call: pushEvent{operation: "push", remote: "origin", ref: policyGoalRef, expected: remote, tip: tip}, outcome: outcome, err: pushErr})
	switch outcome {
	case CASLanded:
		calls = append(calls, originEvent(tip), clearEvent(op), ev("claim"))
	case CASRefused:
		calls = append(calls, clearEvent(op))
	case CASUnknown:
		calls = append(calls, remoteEvent(), clearEvent(op))
	}
	return calls
}
func (c *followonClient) adoptCalls(op, local, remote, relation string) []pushExpectation {
	calls := []pushExpectation{ev("claim"), ev("txn-refs"), remoteEvent(), tipEvent(policyGoalRef)}
	if local != "" {
		calls = append(calls, rangeEvent(local), tipEvent(originTipRef("goal-a")))
	}
	calls = append(calls, fetchEvents(op, remote)...)
	if local != "" {
		calls = append(calls, ancestryEvent(local, remote))
		if relation == "replacement" {
			calls = append(calls, ancestryEvent(remote, local))
		}
	}
	calls = append(calls, ev("head"))
	if c.push.head == policyGoalRef {
		calls = append(calls, ev("clean"), pushExpectation{call: pushEvent{operation: "clean", cached: true}}, pushExpectation{call: pushEvent{operation: "detach", tip: remote}})
	}
	calls = append(calls, pushExpectation{call: pushEvent{operation: "move", goal: "goal-a", oldBranch: local, newBranch: remote, newOrigin: remote}})
	if c.push.head == policyGoalRef {
		calls = append(calls, pushExpectation{call: pushEvent{operation: "restore", ref: policyGoalRef}})
	} else if local == "" {
		calls = append(calls, pushExpectation{call: pushEvent{operation: "switch", goal: "goal-a"}})
	}
	return calls
}
func (c *followonClient) commitRepo() commitRepository {
	a, p := c.commit, c.push
	r := a.repository()
	r.facts.Tip = func(repo, ref string) (string, bool, error) {
		a.args(repo == a.root, "Tip repo", repo)
		switch ref {
		case policyGoalRef:
			a.step("tip-goal")
		case originTipRef("goal-a"):
			a.step("tip-origin")
		default:
			a.t.Fatalf("unexpected Tip ref %s", ref)
		}
		tip, ok := p.refs[ref]
		return tip, ok, nil
	}
	r.facts.Ancestor = func(repo, old, new string) (bool, error) {
		a.step("ancestor")
		if len(c.ancestorWant) == 0 {
			a.t.Fatalf("unexpected CommitStaged ancestry %s -> %s", old, new)
		}
		want := c.ancestorWant[0]
		c.ancestorWant = c.ancestorWant[1:]
		a.args(repo == a.root && old == want[0] && new == want[1], "Ancestor", repo, old, new)
		yes, ok := c.ancestors[[2]string{old, new}]
		if !ok {
			a.t.Fatalf("undeclared ancestry %s -> %s", old, new)
		}
		return yes, nil
	}
	clear := r.effects.ClearFetch
	r.effects.ClearFetch = func(repo, ref string) error {
		err := clear(repo, ref)
		if err == nil {
			delete(p.refs, ref)
		}
		return err
	}
	checkout := r.effects.Checkout
	r.effects.Checkout = func(repo, before, after string) error {
		err := checkout(repo, before, after)
		if err == nil && after == a.next {
			for path := range c.s.known {
				if body, ok := c.files[after][path]; ok {
					a.setFile(path, strptr(body))
				} else {
					a.setFile(path, nil)
				}
			}
			p.checkout = after
			p.staged = false
			p.unstaged = false
			p.setTrackedTree(after)
		}
		return err
	}
	attach := r.effects.Attach
	r.effects.Attach = func(repo, ref string) error {
		err := attach(repo, ref)
		if err == nil {
			p.head = ref
		}
		return err
	}
	publish := r.effects.Publish
	r.effects.Publish = func(repo, goal, old, next, origin string) error {
		err := publish(repo, goal, old, next, origin)
		if err == nil {
			p.refs[policyGoalRef] = next
			if origin != "" {
				p.refs[originTipRef(goal)] = origin
			}
		}
		return err
	}
	open := r.effects.Open
	var openedBase string
	r.effects.Open = func(repo, base string, amend bool) (string, func(), error) {
		dir, close, err := open(repo, base, amend)
		if err == nil {
			openedBase = base
		}
		return dir, close, err
	}
	commit := r.effects.Commit
	r.effects.Commit = func(dir, subject, trailer string, amend bool) error {
		err := commit(dir, subject, trailer, amend)
		if err == nil {
			c.s.parents[a.next] = openedBase
		}
		return err
	}
	return r
}
func (c *followonClient) expectCommitAncestors(inspect []string) {
	c.ancestorWant = nil
	for _, call := range inspect {
		if call == "ancestor" {
			if len(c.ancestorWant) == 0 {
				c.ancestorWant = append(c.ancestorWant, [2]string{c.commit.local, c.commit.remote})
			} else {
				c.ancestorWant = append(c.ancestorWant, [2]string{c.commit.remote, c.commit.local})
			}
		}
	}
}
func (c *followonClient) prepareCommit(tip, base, unit, path, body string, amend bool) CommitRequest {
	c.installTip(tip)
	a, p := c.commit, c.push
	a.local = p.refs[policyGoalRef]
	a.remote = c.s.remote[policyGoalRef]
	a.origin = p.refs[originTipRef("goal-a")]
	a.openBase = base
	a.next = tip
	a.amending = amend
	a.suffix = nil
	a.staged = []string{path}
	a.unstaged = nil
	a.changes = []string{path}
	a.patch = []byte("patch:" + tip)
	a.tree = "tree:" + tip
	a.state.index = a.tree
	a.state.status = "staged:" + path
	a.installedStatus = ""
	a.installFiles = map[string]*string{path: strptr(body)}
	a.subject = "goal goal-a units " + unit
	a.trailer = "Goal-Unit: goal-a/" + unit
	a.write(path, body)
	p.staged = true
	req := a.req("pending")
	req.Unit = unit
	req.Amend = amend
	req.Transport = policyTransport{remote: func(repo, remote, ref string) (string, bool, error) {
		a.step("remote")
		a.args(repo == a.root && remote == "origin" && ref == policyGoalRef, "remote", repo, remote, ref)
		tip, ok := c.s.remote[ref]
		return tip, ok, nil
	}, fetch: func(repo, remote, ref, dest string) error {
		a.step("fetch")
		a.args(repo == a.root && remote == "origin" && ref == policyGoalRef && dest == fetchRef(a.op), "fetch", repo, remote, ref, dest)
		tip, ok := c.s.remote[ref]
		if !ok {
			a.t.Fatal("fetch of absent ref")
		}
		p.refs[dest] = tip
		c.installTip(tip)
		return nil
	}}
	return req
}
func (c *followonClient) runCommit(op, tip, base, unit, path, body string, amend bool, inspect []string) (string, error) {
	req := c.prepareCommit(tip, base, unit, path, body, amend)
	a := c.commit
	a.op = op
	req.OpID = op
	calls := append([]string{"claim"}, inspect...)
	away := a.state.headRef != policyGoalRef
	if amend {
		rangeName := "range-local"
		if a.local != "" && a.local == a.remote {
			rangeName = "range-remote"
		}
		calls = append(calls, "staged", rangeName, "patch", "index", "suffix", "open", "apply", "commit", "head-scratch", "tree", "without", "range-new", "claim", "index", "unstaged", "changes", "headref")
	} else {
		calls = append(calls, "staged")
		if base != a.local && base != policyBase {
			calls = append(calls, "unstaged")
		}
		calls = append(calls, "patch", "open", "apply", "commit", "head-scratch", "range-new", "claim", "close", "index", "unstaged", "changes", "headref")
	}
	if away {
		calls = append(calls, "worktrees")
	}
	calls = append(calls, "head-root", "checkout")
	if away {
		calls = append(calls, "attach")
	}
	calls = append(calls, "publish")
	if amend {
		calls = append(calls, "close")
	}
	a.expect(calls...)
	a.prepare(amend)
	c.expectCommitAncestors(inspect)
	result, err := commitStaged(req, c.commitRepo())
	if a.at != len(a.expected) {
		c.s.t.Fatalf("%d CommitStaged replies unconsumed: %v", len(a.expected)-a.at, a.expected[a.at:])
	}
	if len(c.ancestorWant) != 0 {
		c.s.t.Fatalf("%d CommitStaged ancestry calls unconsumed", len(c.ancestorWant))
	}
	return result, err
}
func inspectNoRemote(local bool) []string {
	if local {
		return []string{"tip-goal", "range-local", "remote", "tip-origin"}
	}
	return []string{"tip-goal", "remote", "head-root", "tip-origin"}
}
func inspectRemote(local bool, ancestry int, equal bool) []string {
	calls := []string{"tip-goal"}
	if local {
		if equal {
			calls = append(calls, "range-remote")
		} else {
			calls = append(calls, "range-local")
		}
	}
	calls = append(calls, "remote", "fetch", "range-remote", "clear", "tip-origin")
	for i := 0; i < ancestry; i++ {
		calls = append(calls, "ancestor")
	}
	return calls
}
func (c *followonClient) mustState(goal, origin, head, stringStatus string) {
	c.s.t.Helper()
	a, p := c.commit, c.push
	if p.refs[policyGoalRef] != goal || p.refs[originTipRef("goal-a")] != origin || p.head != head || a.state.status != stringStatus || a.state.goal != goal || a.state.origin != origin {
		c.s.t.Fatalf("client refs/head/status = goal %s origin %s head %s status %s; commit state %+v", p.refs[policyGoalRef], p.refs[originTipRef("goal-a")], p.head, a.state.status, a.state)
	}
	if len(p.txns) != 0 {
		c.s.t.Fatalf("transactions remain: %+v", p.txns)
	}
	for ref := range p.refs {
		if strings.HasPrefix(ref, "refs/metasystem/goals/fetch/") {
			c.s.t.Fatalf("fetch ref remains: %s", ref)
		}
	}
}
func (c *followonClient) mustFile(path, body string) {
	c.s.t.Helper()
	got, err := os.ReadFile(filepath.Join(c.push.repo, path))
	if err != nil || string(got) != body {
		c.s.t.Fatalf("file %s=%q, %v, want %q", path, got, err, body)
	}
}
func (c *followonClient) mustParent(child, parent string) {
	c.s.t.Helper()
	if got := c.s.parents[child]; got != parent {
		c.s.t.Fatalf("commit %s opened on %s; want parent %s", child, got, parent)
	}
}
func (s *followonScenario) mustRemote(tip string) {
	s.t.Helper()
	if got := s.remote[policyGoalRef]; got != tip {
		s.t.Fatalf("remote=%s want %s", got, tip)
	}
}
func (s *followonScenario) finishGraph(tip string) {
	for other := range s.ranges {
		if other == tip {
			continue
		}
		for _, pair := range [][2]string{{tip, other}, {other, tip}} {
			if _, ok := s.ancestors[pair]; !ok {
				s.ancestors[pair] = false
			}
		}
	}
}
func (c *followonClient) pushed(op, tip string) PushResult {
	c.s.t.Helper()
	result, err := c.runPush(c.request(op), c.publishCalls(op, tip, CASLanded, nil)...)
	if err != nil || result.Tip != tip {
		c.s.t.Fatalf("push %s = %+v, %v", op, result, err)
	}
	c.s.mustRemote(tip)
	return result
}
func (c *followonClient) adopted(op, local, remote, relation string) {
	c.s.t.Helper()
	result, err := c.runPush(c.request(op), c.adoptCalls(op, local, remote, relation)...)
	if err != nil || result.State != "adopted" || result.Tip != remote {
		c.s.t.Fatalf("adopt %s = %+v, %v", op, result, err)
	}
	c.mustState(remote, remote, policyGoalRef, "")
}
func (c *followonClient) committed(op, tip, base, unit, path, body string, amend bool, inspect []string) {
	c.s.t.Helper()
	got, err := c.runCommit(op, tip, base, unit, path, body, amend, inspect)
	if err != nil || got != tip {
		c.s.t.Fatalf("commit %s = %s, %v, want %s", op, got, err, tip)
	}
	c.mustFile(path, body)
	if c.commit.state.headCommit != tip || c.commit.state.index != "tree:"+tip || c.commit.state.status != "" {
		c.s.t.Fatalf("commit after state = %+v", c.commit.state)
	}
}
func TestPushPublishesAmendAndSecondCloneAdopts(t *testing.T) {
	t.Parallel()
	s := newFollowonScenario(t)
	firstClient := s.client()
	first := s.child(policyBase, "u1", "metasystem/code.go", "one")
	s.finishGraph(first)
	firstClient.committed("commit-first", first, policyBase, "u1", "metasystem/code.go", "one", false, inspectNoRemote(false))
	result := firstClient.pushed("push-first", first)
	if result.State != "pushed" {
		t.Fatalf("first push state=%s", result.State)
	}
	replacement := s.replacement(first, "u1", "metasystem/code.go", "two")
	s.finishGraph(replacement)
	firstClient.committed("commit-amend", replacement, first, "u1", "metasystem/code.go", "two", true, inspectRemote(true, 0, true))
	if replacement == first {
		t.Fatal("amend retained first tip")
	}
	result = firstClient.pushed("push-amend", replacement)
	if result.State != "pushed" {
		t.Fatalf("amend push state=%s", result.State)
	}
	other := s.client()
	other.adopted("adopt", "", replacement, "")
	if got := other.push.refs[policyGoalRef]; got != replacement {
		t.Fatalf("adopted ref=%s", got)
	}
	other.mustFile("metasystem/code.go", "two")
	firstClient.mustState(replacement, replacement, policyGoalRef, "")
}
func TestPushAndCommitAdoptDescendantRemoteAfterLandedCrash(t *testing.T) {
	t.Parallel()
	for _, seedOrigin := range []bool{false, true} {
		for _, action := range []string{"push", "commit"} {
			name := "origin record absent"
			if seedOrigin {
				name = "origin record lags"
			}
			t.Run(name+" then "+action, func(t *testing.T) {
				t.Parallel()
				s := newFollowonScenario(t)
				holder := s.client()
				parent := policyBase
				if seedOrigin {
					seed := s.child(parent, "u0", "metasystem/seed.go", "seed")
					s.finishGraph(seed)
					holder.committed("commit-seed", seed, parent, "u0", "metasystem/seed.go", "seed", false, inspectNoRemote(false))
					holder.pushed("seed-origin", seed)
					parent = seed
				}
				landed := s.child(parent, "u1", "metasystem/code.go", "one")
				s.finishGraph(landed)
				inspect := inspectNoRemote(false)
				if seedOrigin {
					inspect = inspectRemote(true, 0, true)
				}
				holder.committed("commit-landed", landed, parent, "u1", "metasystem/code.go", "one", false, inspect)
				op := "landed-crash"
				calls := holder.publishCalls(op, landed, CASLanded, nil)
				calls = calls[:len(calls)-3]
				req := holder.request(op)
				req.Hooks.AfterPush = func() error { return errors.New("process stopped after push") }
				if _, err := holder.runPush(req, calls...); err == nil {
					t.Fatal("landed crash succeeded")
				}
				s.mustRemote(landed)
				if _, ok := holder.push.txns[txnRef(op)]; !ok {
					t.Fatal("landed crash lost transaction")
				}
				other := s.client()
				other.adopted("other-adopt", "", landed, "")
				remote := s.child(landed, "u2", "metasystem/other.go", "two")
				s.finishGraph(remote)
				other.committed("other-commit", remote, landed, "u2", "metasystem/other.go", "two", false, inspectRemote(true, 0, true))
				other.pushed("other-push", remote)
				recovery := []pushExpectation{ev("claim"), ev("txn-refs"), {call: pushEvent{operation: "txn", ref: txnRef(op)}}, remoteEvent()}
				recovery = append(recovery, fetchEvents("claim-returned", remote)...)
				recovery = append(recovery, ancestryEvent(landed, remote), originEvent(remote), clearEvent(op), ev("claim"))
				result, err := holder.runPush(holder.request("claim-returned"), recovery...)
				if err != nil || result.State != "reconciled" || result.Tip != remote {
					t.Fatalf("reconcile = %+v, %v", result, err)
				}
				if got := holder.push.refs[originTipRef("goal-a")]; got != remote {
					t.Fatalf("recorded origin=%s", got)
				}
				if len(holder.push.txns) != 0 {
					t.Fatalf("transaction remains: %+v", holder.push.txns)
				}
				if action == "push" {
					holder.adopted("adopt-after-reconcile", landed, remote, "descendant")
				}
				next := s.child(remote, "u3", "metasystem/next.go", "three")
				s.finishGraph(next)
				nextInspect := inspectRemote(true, 1, false)
				if action == "push" {
					nextInspect = inspectRemote(true, 0, true)
				}
				holder.committed("commit-after-reconcile", next, remote, "u3", "metasystem/next.go", "three", false, nextInspect)
				holder.mustParent(next, remote)
				holder.mustFile("metasystem/next.go", "three")
				holder.pushed("push-after-reconcile", next)
				holder.mustState(next, next, policyGoalRef, "")
			})
		}
	}
}
func TestPushLeaseAndClaimMovementRefuseWithoutRetry(t *testing.T) {
	t.Parallel()
	s := newFollowonScenario(t)
	c := s.client()
	first := s.child(policyBase, "u1", "metasystem/code.go", "one")
	s.finishGraph(first)
	c.committed("commit-first", first, policyBase, "u1", "metasystem/code.go", "one", false, inspectNoRemote(false))
	c.pushed("seed", first)
	local := s.replacement(first, "u1", "metasystem/code.go", "two")
	s.finishGraph(local)
	c.committed("commit-race", local, first, "u1", "metasystem/code.go", "two", true, inspectRemote(true, 0, true))
	competitor := s.child(first, "u2", "metasystem/competitor.go", "competing")
	s.finishGraph(competitor)
	req := c.request("lease-race")
	req.Hooks.AfterRemoteRead = func() error { s.remote[policyGoalRef] = competitor; return nil }
	_, err := c.runPush(req, c.publishCalls("lease-race", local, CASRefused, errors.New("lease moved"))...)
	requireCode(t, err, LeaseMovedCode)
	s.mustRemote(competitor)
	if competitor == local {
		t.Fatal("competitor equals local tip")
	}
	if len(c.push.txns) != 0 {
		t.Fatal("lease refusal left transaction")
	}
	c.installTip(competitor)
	c.push.refs[policyGoalRef] = competitor
	c.push.setTrackedTree(competitor)
	c.push.checkout = competitor
	c.push.afterCheckout(competitor)
	c.commit.local = competitor
	c.commit.state.goal = competitor
	current := []pushExpectation{ev("claim"), ev("txn-refs"), remoteEvent(), tipEvent(policyGoalRef), rangeEvent(competitor), originEvent(competitor)}
	result, err := c.runPush(c.request("after-lease-race"), current...)
	if err != nil || result.State != "current" || result.Tip != competitor {
		t.Fatalf("push after reset=%+v,%v", result, err)
	}
	c.mustState(competitor, competitor, policyGoalRef, "")
	s2 := newFollowonScenario(t)
	lost := s2.client()
	tip := s2.child(policyBase, "u1", "metasystem/code.go", "one")
	s2.finishGraph(tip)
	lost.committed("commit-first", tip, policyBase, "u1", "metasystem/code.go", "one", false, inspectNoRemote(false))
	held := true
	claim := lost.request("claim-race")
	claim.CheckClaim = func() error {
		lost.push.take(pushEvent{operation: "claim", repo: lost.push.repo})
		if !held {
			return errors.New("claim moved to machine-b+lineage-b")
		}
		return nil
	}
	claim.Hooks.AfterPush = func() error { held = false; return nil }
	_, err = lost.runPush(claim, lost.publishCalls("claim-race", tip, CASLanded, nil)...)
	requireCode(t, err, ClaimLostCode)
	s2.mustRemote(tip)
	lost.mustState(tip, tip, policyGoalRef, "")
}
func TestPushUnknownNotLandedDoesNotWedgeLaterAmend(t *testing.T) {
	t.Parallel()
	s := newFollowonScenario(t)
	c := s.client()
	first := s.child(policyBase, "u1", "metasystem/code.go", "one")
	s.finishGraph(first)
	c.committed("commit-first", first, policyBase, "u1", "metasystem/code.go", "one", false, inspectNoRemote(false))
	op := "unknown-not-landed"
	_, err := c.runPush(c.request(op), c.publishCalls(op, first, CASUnknown, errors.New("connection ended"))...)
	requireCode(t, err, PushUnknownCode)
	if len(c.push.txns) != 0 {
		t.Fatal("unknown outcome left transaction")
	}
	s.mustRemote("")
	replacement := s.replacement(first, "u1", "metasystem/code.go", "two")
	s.finishGraph(replacement)
	c.committed("amend-after-unknown", replacement, first, "u1", "metasystem/code.go", "two", true, inspectNoRemote(true))
	result := c.pushed("push-after-unknown", replacement)
	if result.Tip != replacement {
		t.Fatalf("push after amend=%+v", result)
	}
	c.mustState(replacement, replacement, policyGoalRef, "")
}
func TestPushAdoptsRemoteAdvanceAndCommitUsesIt(t *testing.T) {
	t.Parallel()
	s := newFollowonScenario(t)
	firstClient := s.client()
	first := s.child(policyBase, "u1", "metasystem/code.go", "one")
	s.finishGraph(first)
	firstClient.committed("commit-first", first, policyBase, "u1", "metasystem/code.go", "one", false, inspectNoRemote(false))
	firstClient.pushed("push-first", first)
	other := s.client()
	other.adopted("adopt-first", "", first, "")
	second := s.replacement(first, "u1", "metasystem/code.go", "two")
	s.finishGraph(second)
	other.committed("second-amend", second, first, "u1", "metasystem/code.go", "two", true, inspectRemote(true, 0, true))
	other.pushed("push-second", second)
	firstClient.adopted("adopt-second", first, second, "replacement")
	s.mustRemote(second)
	third := s.replacement(second, "u1", "metasystem/code.go", "three")
	s.finishGraph(third)
	other.committed("third-amend", third, second, "u1", "metasystem/code.go", "three", true, inspectRemote(true, 0, true))
	other.pushed("push-third", third)
	next := s.child(third, "u2", "metasystem/next.go", "next")
	s.finishGraph(next)
	firstClient.committed("commit-after-advance", next, third, "u2", "metasystem/next.go", "next", false, inspectRemote(true, 2, false))
	firstClient.mustParent(next, third)
	s.mustRemote(third)
	firstClient.mustFile("metasystem/code.go", "three")
	firstClient.mustState(next, third, policyGoalRef, "")
}
func TestPushAndCommitRefuseDivergedRemote(t *testing.T) {
	t.Parallel()
	s := newFollowonScenario(t)
	localClient := s.client()
	first := s.child(policyBase, "u1", "metasystem/code.go", "one")
	s.finishGraph(first)
	localClient.committed("commit-first", first, policyBase, "u1", "metasystem/code.go", "one", false, inspectNoRemote(false))
	localClient.pushed("seed", first)
	other := s.client()
	other.adopted("adopt", "", first, "")
	local := s.replacement(first, "u1", "metasystem/code.go", "local")
	s.finishGraph(local)
	localClient.committed("local-amend", local, first, "u1", "metasystem/code.go", "local", true, inspectRemote(true, 0, true))
	remote := s.replacement(first, "u1", "metasystem/code.go", "remote")
	s.finishGraph(remote)
	other.committed("remote-amend", remote, first, "u1", "metasystem/code.go", "remote", true, inspectRemote(true, 0, true))
	other.pushed("remote-push", remote)
	op := "stale-push"
	calls := []pushExpectation{ev("claim"), ev("txn-refs"), remoteEvent(), tipEvent(policyGoalRef), rangeEvent(local), tipEvent(originTipRef("goal-a"))}
	calls = append(calls, fetchEvents(op, remote)...)
	calls = append(calls, ancestryEvent(local, remote), ancestryEvent(remote, local))
	_, err := localClient.runPush(localClient.request(op), calls...)
	requireCode(t, err, StaleCode)
	if !strings.Contains(err.Error(), local) || !strings.Contains(err.Error(), remote) {
		t.Fatalf("stale push=%v", err)
	}
	s.mustRemote(remote)
	next := s.child(local, "u2", "metasystem/next.go", "next")
	s.finishGraph(next)
	req := localClient.prepareCommit(next, local, "u2", "metasystem/next.go", "next", false)
	localClient.commit.op = "stale-commit"
	req.OpID = "stale-commit"
	expected := append([]string{"claim"}, inspectRemote(true, 2, false)...)
	localClient.commit.expect(expected...)
	localClient.expectCommitAncestors(inspectRemote(true, 2, false))
	_, err = commitStaged(req, localClient.commitRepo())
	requireCode(t, err, StaleCode)
	if localClient.commit.at != len(localClient.commit.expected) {
		t.Fatalf("CommitStaged replies unconsumed: %v", localClient.commit.expected[localClient.commit.at:])
	}
	if len(localClient.ancestorWant) != 0 {
		t.Fatalf("ancestry calls unconsumed: %v", localClient.ancestorWant)
	}
	if localClient.push.refs[policyGoalRef] != local || s.remote[policyGoalRef] != remote {
		t.Fatalf("divergence changed refs local=%s remote=%s", localClient.push.refs[policyGoalRef], s.remote[policyGoalRef])
	}
	localClient.mustFile("metasystem/next.go", "next")
}
