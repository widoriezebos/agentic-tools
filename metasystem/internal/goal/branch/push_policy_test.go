package branch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const policyGoalRef = "refs/heads/goal/goal-a"

var (
	pushPolicyBase = strings.Repeat("a", 40)
	policyFirst    = strings.Repeat("b", 40)
	policySecond   = strings.Repeat("c", 40)
	policyMoved    = strings.Repeat("d", 40)
)

type pushEvent struct {
	operation, repo, ref, goal, remote, opid, endpoint, tip, older, newer, expected, oldBranch, newBranch, newOrigin string
	cached                                                                                                           bool
	txn                                                                                                              pushTxn
}

type pushExpectation struct {
	call    pushEvent
	err     error
	outcome CASOutcome
	landed  bool
}

type pushPolicyFixture struct {
	t                *testing.T
	repo             string
	endpoint         string
	want             []pushExpectation
	refs             map[string]string
	remote           map[string]string
	txns             map[string]pushTxn
	ranges           map[string][]Commit
	rangeErrors      map[string]error
	ancestors        map[[2]string]bool
	trees            map[string][]string
	tracked          map[string]bool
	head             string
	checkout         string
	staged, unstaged bool
	claimErr         error
	afterCheckout    func(string)
	afterMove        func(string, string)
	afterOrigin      func(string)
	afterFetch       func(string)
}

func newPushPolicyFixture(t *testing.T) *pushPolicyFixture {
	t.Helper()
	f := &pushPolicyFixture{t: t, repo: t.TempDir(), endpoint: pushPolicyBase, refs: map[string]string{}, remote: map[string]string{}, txns: map[string]pushTxn{}, ranges: map[string][]Commit{}, rangeErrors: map[string]error{}, ancestors: map[[2]string]bool{}, trees: map[string][]string{}, tracked: map[string]bool{}}
	f.ranges[policyFirst] = []Commit{{ID: policyFirst, Kind: Unit, Unit: "u1"}}
	f.ranges[policySecond] = []Commit{{ID: policyFirst, Kind: Unit, Unit: "u1"}, {ID: policySecond, Kind: Unit, Unit: "u2"}}
	f.trees[policyFirst] = []string{"metasystem/code.go"}
	f.trees[policySecond] = []string{"metasystem/code.go", "metasystem/other.go"}
	f.ancestors[[2]string{policyFirst, policySecond}] = true
	return f
}

func (f *pushPolicyFixture) expect(calls ...pushExpectation) { f.want = append(f.want, calls...) }
func pe(op string) pushExpectation                           { return pushExpectation{call: pushEvent{operation: op}} }

func (f *pushPolicyFixture) take(got pushEvent) pushExpectation {
	f.t.Helper()
	if len(f.want) == 0 {
		f.t.Fatalf("unexpected Push call: %+v", got)
	}
	want := f.want[0]
	f.want = f.want[1:]
	if got.repo != f.repo {
		f.t.Fatalf("Push repository = %q, want %q", got.repo, f.repo)
	}
	want.call.repo = f.repo
	if !reflect.DeepEqual(got, want.call) {
		f.t.Fatalf("Push call = %+v, want %+v", got, want.call)
	}
	return want
}

func (f *pushPolicyFixture) done() {
	f.t.Helper()
	if len(f.want) != 0 {
		f.t.Fatalf("%d Push calls unconsumed; next %+v", len(f.want), f.want[0])
	}
}

func (f *pushPolicyFixture) setTrackedTree(tip string) {
	f.t.Helper()
	paths, ok := f.trees[tip]
	if !ok {
		f.t.Fatalf("undeclared checkout tree %s", tip)
	}
	f.tracked = make(map[string]bool, len(paths))
	for _, path := range paths {
		f.tracked[path] = true
	}
}

func (f *pushPolicyFixture) request(opid string) PushRequest {
	return PushRequest{Repo: f.repo, Remote: "origin", EndpointTip: f.endpoint, GoalID: "goal-a", OpID: opid, CheckClaim: func() error {
		f.take(pushEvent{operation: "claim", repo: f.repo})
		return f.claimErr
	}, Transport: f}
}

func (f *pushPolicyFixture) repository() pushRepository {
	return pushRepository{
		TxnRefs: func(repo string) ([]string, error) {
			x := f.take(pushEvent{operation: "txn-refs", repo: repo})
			if x.err != nil {
				return nil, x.err
			}
			var refs []string
			for ref := range f.txns {
				refs = append(refs, ref)
			}
			// The fixture has at most one transaction in these parent scenarios.
			return append([]string(nil), refs...), nil
		},
		Txn: func(repo, ref string) (pushTxn, error) {
			x := f.take(pushEvent{operation: "txn", repo: repo, ref: ref})
			if x.err != nil {
				return pushTxn{}, x.err
			}
			txn, ok := f.txns[ref]
			if !ok {
				f.t.Fatalf("missing transaction %s", ref)
			}
			return txn, nil
		},
		Tip: func(repo, ref string) (string, bool, error) {
			x := f.take(pushEvent{operation: "tip", repo: repo, ref: ref})
			if x.err != nil {
				return "", false, x.err
			}
			tip, ok := f.refs[ref]
			return tip, ok, nil
		},
		Ancestor: func(repo, older, newer string) (bool, error) {
			x := f.take(pushEvent{operation: "ancestor", repo: repo, older: older, newer: newer})
			if x.err != nil {
				return false, x.err
			}
			result, ok := f.ancestors[[2]string{older, newer}]
			if !ok {
				f.t.Fatalf("undeclared ancestry %s -> %s", older, newer)
			}
			return result, nil
		},
		Range: func(repo, endpoint, tip, goal string) ([]Commit, error) {
			x := f.take(pushEvent{operation: "range", repo: repo, endpoint: endpoint, tip: tip, goal: goal})
			if x.err != nil {
				return nil, x.err
			}
			if err, ok := f.rangeErrors[tip]; ok {
				return nil, err
			}
			commits, ok := f.ranges[tip]
			if !ok {
				f.t.Fatalf("undeclared range %s", tip)
			}
			return cloneCommits(commits), nil
		},
		HeadRef: func(repo string) string {
			f.take(pushEvent{operation: "head", repo: repo})
			return f.head
		},
		TrackedClean: func(repo string, cached bool) (bool, error) {
			x := f.take(pushEvent{operation: "clean", repo: repo, cached: cached})
			if x.err != nil {
				return false, x.err
			}
			if cached {
				return !f.staged, nil
			}
			return !f.unstaged, nil
		},
		WriteTxn: func(repo, opid string, txn pushTxn) error {
			x := f.take(pushEvent{operation: "write-txn", repo: repo, opid: opid, txn: txn})
			if x.err == nil {
				f.txns[txnRef(opid)] = txn
			}
			return x.err
		},
		ClearRef: func(repo, ref string) error {
			x := f.take(pushEvent{operation: "clear", repo: repo, ref: ref})
			if x.err == nil {
				delete(f.txns, ref)
				delete(f.refs, ref)
			}
			return x.err
		},
		RecordOrigin: func(repo, goal, tip string) error {
			x := f.take(pushEvent{operation: "origin", repo: repo, goal: goal, tip: tip})
			if x.err == nil {
				f.refs[originTipRef(goal)] = tip
				if f.afterOrigin != nil {
					f.afterOrigin(tip)
				}
			}
			return x.err
		},
		Detach: func(repo, tip string) error {
			x := f.take(pushEvent{operation: "detach", repo: repo, tip: tip})
			if x.err == nil {
				f.setTrackedTree(tip)
				f.head = ""
				f.checkout = tip
				f.staged = false
				f.unstaged = false
				if f.afterCheckout != nil {
					f.afterCheckout(tip)
				}
			}
			return x.err
		},
		RestoreHead: func(repo, ref string) error {
			x := f.take(pushEvent{operation: "restore", repo: repo, ref: ref})
			if x.err == nil {
				f.head = ref
				f.checkout = f.refs[ref]
				if f.afterCheckout != nil {
					f.afterCheckout(f.checkout)
				}
			}
			return x.err
		},
		SwitchGoal: func(repo, goal string) error {
			x := f.take(pushEvent{operation: "switch", repo: repo, goal: goal})
			if x.err == nil {
				f.head = goalBranchRef(goal)
				f.checkout = f.refs[f.head]
				f.setTrackedTree(f.checkout)
				if f.afterCheckout != nil {
					f.afterCheckout(f.checkout)
				}
			}
			return x.err
		},
		MoveRefs: func(repo, goal, oldBranch, newBranch, newOrigin string) error {
			x := f.take(pushEvent{operation: "move", repo: repo, goal: goal, oldBranch: oldBranch, newBranch: newBranch, newOrigin: newOrigin})
			if x.err != nil {
				return x.err
			}
			ref := goalBranchRef(goal)
			if f.refs[ref] != oldBranch {
				return fmt.Errorf("goal ref moved from %s to %s", oldBranch, f.refs[ref])
			}
			f.refs[ref] = newBranch
			if newOrigin != "" {
				f.refs[originTipRef(goal)] = newOrigin
			}
			if f.afterMove != nil {
				f.afterMove(newBranch, newOrigin)
			}
			return nil
		},
	}
}

func (f *pushPolicyFixture) RemoteTip(repo, remote, ref string) (string, bool, error) {
	x := f.take(pushEvent{operation: "remote", repo: repo, remote: remote, ref: ref})
	if x.err != nil {
		return "", false, x.err
	}
	tip, ok := f.remote[ref]
	return tip, ok, nil
}
func (f *pushPolicyFixture) Fetch(repo, remote, ref, destination string) error {
	x := f.take(pushEvent{operation: "fetch", repo: repo, remote: remote, ref: ref, tip: destination})
	tip, ok := f.remote[ref]
	if !ok {
		f.t.Fatalf("fetch of absent remote ref %s", ref)
	}
	f.refs[destination] = tip
	if x.err == nil {
		if f.afterFetch != nil {
			f.afterFetch(tip)
		}
	}
	return x.err
}
func (f *pushPolicyFixture) Push(repo, remote, ref, expected, tip string) (CASOutcome, error) {
	x := f.take(pushEvent{operation: "push", repo: repo, remote: remote, ref: ref, expected: expected, tip: tip})
	if current := f.remote[ref]; current != expected && x.outcome != CASRefused {
		f.t.Fatalf("transport expected %s, remote holds %s", expected, current)
	}
	if x.outcome == CASLanded || x.landed {
		f.remote[ref] = tip
	}
	return x.outcome, x.err
}

func (f *pushPolicyFixture) verifyScratch(path string, want []byte) {
	f.t.Helper()
	got, err := os.ReadFile(filepath.Join(f.repo, path))
	if err != nil || !reflect.DeepEqual(got, want) {
		f.t.Fatalf("scratch bytes = %q, %v", got, err)
	}
	if f.tracked[path] {
		f.t.Fatal("scratch became tracked")
	}
}

func TestAdoptRemoteTipRestoresHeadWhenTheRefUpdateFails(t *testing.T) {
	t.Parallel()
	f := newPushPolicyFixture(t)
	f.refs[policyGoalRef] = policyFirst
	f.refs[originTipRef("goal-a")] = policyFirst
	f.remote[policyGoalRef] = policySecond
	f.head, f.checkout = policyGoalRef, policyFirst
	op := "restore-failed-update"
	f.expect(pe("claim"), pe("txn-refs"), pushExpectation{call: pushEvent{operation: "remote", remote: "origin", ref: policyGoalRef}},
		pushExpectation{call: pushEvent{operation: "tip", ref: policyGoalRef}},
		pushExpectation{call: pushEvent{operation: "range", endpoint: pushPolicyBase, tip: policyFirst, goal: "goal-a"}},
		pushExpectation{call: pushEvent{operation: "tip", ref: originTipRef("goal-a")}},
		pushExpectation{call: pushEvent{operation: "fetch", remote: "origin", ref: policyGoalRef, tip: fetchRef(op)}},
		pushExpectation{call: pushEvent{operation: "range", endpoint: pushPolicyBase, tip: policySecond, goal: "goal-a"}},
		pushExpectation{call: pushEvent{operation: "clear", ref: fetchRef(op)}},
		pushExpectation{call: pushEvent{operation: "ancestor", older: policyFirst, newer: policySecond}},
		pe("head"), pe("clean"), pushExpectation{call: pushEvent{operation: "clean", cached: true}},
		pushExpectation{call: pushEvent{operation: "detach", tip: policySecond}},
		pushExpectation{call: pushEvent{operation: "move", goal: "goal-a", oldBranch: policyFirst, newBranch: policySecond, newOrigin: policySecond}},
		pushExpectation{call: pushEvent{operation: "restore", ref: policyGoalRef}})
	req := f.request(op)
	req.Hooks.BeforeAdoptionRefMove = func() error { f.refs[policyGoalRef] = policyMoved; return nil }
	if _, err := pushWithRepository(req, f.repository()); err == nil {
		t.Fatal("adoption with failed ref transaction succeeded")
	}
	f.done()
	if f.head != policyGoalRef || f.checkout != policyMoved {
		t.Fatalf("HEAD was not restored: ref=%q tip=%q", f.head, f.checkout)
	}
	if f.refs[policyGoalRef] != policyMoved || f.refs[originTipRef("goal-a")] != policyFirst {
		t.Fatal("failed transaction changed refs")
	}
	if _, ok := f.refs[fetchRef(op)]; ok {
		t.Fatal("fetch ref survived")
	}
}

func TestPushReconcilesUnknownOutcomeAndPreparedTransaction(t *testing.T) {
	t.Parallel()
	t.Run("unknown landed", func(t *testing.T) {
		f := newPushPolicyFixture(t)
		f.refs[policyGoalRef] = policyFirst
		op := "unknown"
		txn := pushTxn{SchemaVersion: 1, Goal: "goal-a", Remote: "origin", Ref: policyGoalRef, New: policyFirst}
		f.expect(pe("claim"), pe("txn-refs"), pushExpectation{call: pushEvent{operation: "remote", remote: "origin", ref: policyGoalRef}},
			pushExpectation{call: pushEvent{operation: "tip", ref: policyGoalRef}}, pushExpectation{call: pushEvent{operation: "range", endpoint: pushPolicyBase, tip: policyFirst, goal: "goal-a"}},
			pushExpectation{call: pushEvent{operation: "tip", ref: originTipRef("goal-a")}},
			pushExpectation{call: pushEvent{operation: "write-txn", opid: op, txn: txn}},
			pushExpectation{call: pushEvent{operation: "push", remote: "origin", ref: policyGoalRef, tip: policyFirst}, outcome: CASUnknown, landed: true, err: errors.New("connection ended before the result was read")},
			pushExpectation{call: pushEvent{operation: "remote", remote: "origin", ref: policyGoalRef}},
			pushExpectation{call: pushEvent{operation: "origin", goal: "goal-a", tip: policyFirst}},
			pushExpectation{call: pushEvent{operation: "clear", ref: txnRef(op)}}, pe("claim"))
		result, err := pushWithRepository(f.request(op), f.repository())
		if err != nil || result.Tip != policyFirst || f.remote[policyGoalRef] != policyFirst {
			t.Fatalf("unknown reconciliation = %+v, %v", result, err)
		}
		f.done()
		if f.refs[originTipRef("goal-a")] != policyFirst || len(f.txns) != 0 {
			t.Fatal("unknown landing did not record origin and clear transaction")
		}
	})
	t.Run("prepared landed recovery", func(t *testing.T) {
		f := newPushPolicyFixture(t)
		f.refs[policyGoalRef] = policyFirst
		op := "crash-window"
		txn := pushTxn{SchemaVersion: 1, Goal: "goal-a", Remote: "origin", Ref: policyGoalRef, New: policyFirst}
		f.expect(pe("claim"), pe("txn-refs"), pushExpectation{call: pushEvent{operation: "remote", remote: "origin", ref: policyGoalRef}},
			pushExpectation{call: pushEvent{operation: "tip", ref: policyGoalRef}}, pushExpectation{call: pushEvent{operation: "range", endpoint: pushPolicyBase, tip: policyFirst, goal: "goal-a"}},
			pushExpectation{call: pushEvent{operation: "tip", ref: originTipRef("goal-a")}}, pushExpectation{call: pushEvent{operation: "write-txn", opid: op, txn: txn}},
			pushExpectation{call: pushEvent{operation: "push", remote: "origin", ref: policyGoalRef, tip: policyFirst}, outcome: CASLanded})
		req := f.request(op)
		req.Hooks.AfterPush = func() error { return errors.New("simulated process death") }
		if _, err := pushWithRepository(req, f.repository()); err == nil || !strings.Contains(err.Error(), "simulated process death") {
			t.Fatalf("crash window = %v", err)
		}
		if _, ok := f.txns[txnRef(op)]; !ok {
			t.Fatal("prepared transaction lost at crash")
		}
		f.expect(pe("claim"), pe("txn-refs"), pushExpectation{call: pushEvent{operation: "txn", ref: txnRef(op)}},
			pushExpectation{call: pushEvent{operation: "remote", remote: "origin", ref: policyGoalRef}},
			pushExpectation{call: pushEvent{operation: "origin", goal: "goal-a", tip: policyFirst}},
			pushExpectation{call: pushEvent{operation: "clear", ref: txnRef(op)}}, pe("claim"))
		result, err := pushWithRepository(f.request("recovery"), f.repository())
		if err != nil || result.State != "reconciled" || result.Tip != policyFirst {
			t.Fatalf("transaction recovery = %+v, %v", result, err)
		}
		f.done()
		if len(f.txns) != 0 || f.refs[originTipRef("goal-a")] != policyFirst {
			t.Fatal("recovery state incorrect")
		}
	})
	t.Run("prepared unlanded recovery and retry", func(t *testing.T) {
		f := newPushPolicyFixture(t)
		f.refs[policyGoalRef] = policyFirst
		op := "before-push-crash"
		txn := pushTxn{SchemaVersion: 1, Goal: "goal-a", Remote: "origin", Ref: policyGoalRef, New: policyFirst}
		f.expect(pe("claim"), pe("txn-refs"), pushExpectation{call: pushEvent{operation: "remote", remote: "origin", ref: policyGoalRef}},
			pushExpectation{call: pushEvent{operation: "tip", ref: policyGoalRef}}, pushExpectation{call: pushEvent{operation: "range", endpoint: pushPolicyBase, tip: policyFirst, goal: "goal-a"}},
			pushExpectation{call: pushEvent{operation: "tip", ref: originTipRef("goal-a")}}, pushExpectation{call: pushEvent{operation: "write-txn", opid: op, txn: txn}})
		req := f.request(op)
		req.Hooks.AfterRemoteRead = func() error { return errors.New("simulated crash before push") }
		if _, err := pushWithRepository(req, f.repository()); err == nil {
			t.Fatal("crash before push succeeded")
		}
		if _, ok := f.txns[txnRef(op)]; !ok {
			t.Fatal("prepared transaction lost at crash")
		}
		f.expect(pe("claim"), pe("txn-refs"), pushExpectation{call: pushEvent{operation: "txn", ref: txnRef(op)}},
			pushExpectation{call: pushEvent{operation: "remote", remote: "origin", ref: policyGoalRef}},
			pushExpectation{call: pushEvent{operation: "clear", ref: txnRef(op)}})
		_, err := pushWithRepository(f.request("report-not-landed"), f.repository())
		var refusal *OpError
		if !errors.As(err, &refusal) || refusal.Code != PushUnknownCode {
			t.Fatalf("unlanded recovery = %v", err)
		}
		if len(f.txns) != 0 {
			t.Fatal("unlanded recovery left transaction")
		}
		newOp := "retry-after-report"
		f.expect(pe("claim"), pe("txn-refs"), pushExpectation{call: pushEvent{operation: "remote", remote: "origin", ref: policyGoalRef}},
			pushExpectation{call: pushEvent{operation: "tip", ref: policyGoalRef}}, pushExpectation{call: pushEvent{operation: "range", endpoint: pushPolicyBase, tip: policyFirst, goal: "goal-a"}},
			pushExpectation{call: pushEvent{operation: "tip", ref: originTipRef("goal-a")}}, pushExpectation{call: pushEvent{operation: "write-txn", opid: newOp, txn: txn}},
			pushExpectation{call: pushEvent{operation: "push", remote: "origin", ref: policyGoalRef, tip: policyFirst}, outcome: CASLanded},
			pushExpectation{call: pushEvent{operation: "origin", goal: "goal-a", tip: policyFirst}},
			pushExpectation{call: pushEvent{operation: "clear", ref: txnRef(newOp)}}, pe("claim"))
		result, err := pushWithRepository(f.request(newOp), f.repository())
		if err != nil || result.Tip != policyFirst || f.remote[policyGoalRef] != policyFirst {
			t.Fatalf("retry after unlanded recovery = %+v, %v", result, err)
		}
		f.done()
		if len(f.txns) != 0 || f.refs[originTipRef("goal-a")] != policyFirst {
			t.Fatal("retry state incorrect")
		}
	})
}

func TestPushAdoptionValidatesRangeAndCleansFetchRef(t *testing.T) {
	t.Parallel()
	f := newPushPolicyFixture(t)
	f.remote[policyGoalRef] = policySecond
	f.rangeErrors[policySecond] = &RangeError{Code: RangeCode, Commit: policySecond, Reason: "invalid commit"}
	for _, row := range []struct {
		op       string
		fetchErr error
	}{{op: "invalid-adoption"}, {op: "failed-fetch", fetchErr: errors.New("fetch result was lost")}} {
		f.expect(pe("claim"), pe("txn-refs"), pushExpectation{call: pushEvent{operation: "remote", remote: "origin", ref: policyGoalRef}},
			pushExpectation{call: pushEvent{operation: "tip", ref: policyGoalRef}},
			pushExpectation{call: pushEvent{operation: "fetch", remote: "origin", ref: policyGoalRef, tip: fetchRef(row.op)}, err: row.fetchErr})
		if row.fetchErr == nil {
			f.expect(pushExpectation{call: pushEvent{operation: "range", endpoint: pushPolicyBase, tip: policySecond, goal: "goal-a"}})
		}
		f.expect(pushExpectation{call: pushEvent{operation: "clear", ref: fetchRef(row.op)}})
		_, err := pushWithRepository(f.request(row.op), f.repository())
		if row.fetchErr == nil {
			var refusal *RangeError
			if !errors.As(err, &refusal) {
				t.Fatalf("invalid adoption = %v", err)
			}
		} else if err == nil || !strings.Contains(err.Error(), "fetch result was lost") {
			t.Fatalf("failed fetch = %v", err)
		}
		if _, present := f.refs[policyGoalRef]; present {
			t.Fatal("invalid remote tip was adopted")
		}
		if _, present := f.refs[fetchRef(row.op)]; present {
			t.Fatal("adoption left fetch ref")
		}
	}
	f.done()
}

func TestPushAdoptionKeepsUntrackedScratchFile(t *testing.T) {
	t.Parallel()
	f := newPushPolicyFixture(t)
	f.remote[policyGoalRef] = policyFirst
	const scratch = "notes/scratch.txt"
	if err := os.MkdirAll(filepath.Join(f.repo, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte("keep me")
	if err := os.WriteFile(filepath.Join(f.repo, scratch), want, 0o644); err != nil {
		t.Fatal(err)
	}
	op := "adopt-with-scratch"
	f.expect(pe("claim"), pe("txn-refs"), pushExpectation{call: pushEvent{operation: "remote", remote: "origin", ref: policyGoalRef}},
		pushExpectation{call: pushEvent{operation: "tip", ref: policyGoalRef}}, pushExpectation{call: pushEvent{operation: "fetch", remote: "origin", ref: policyGoalRef, tip: fetchRef(op)}},
		pushExpectation{call: pushEvent{operation: "range", endpoint: pushPolicyBase, tip: policyFirst, goal: "goal-a"}},
		pushExpectation{call: pushEvent{operation: "clear", ref: fetchRef(op)}}, pe("head"),
		pushExpectation{call: pushEvent{operation: "move", goal: "goal-a", newBranch: policyFirst, newOrigin: policyFirst}},
		pushExpectation{call: pushEvent{operation: "switch", goal: "goal-a"}})
	result, err := pushWithRepository(f.request(op), f.repository())
	if err != nil || result.State != "adopted" || result.Tip != policyFirst {
		t.Fatalf("adoption=%+v err=%v", result, err)
	}
	f.done()
	f.verifyScratch(scratch, want)
	if f.head != policyGoalRef || f.checkout != policyFirst || f.refs[policyGoalRef] != policyFirst {
		t.Fatal("adoption state incorrect")
	}
}

func TestPushAdoptionCrashCannotStageAReversal(t *testing.T) {
	t.Parallel()
	f := newPushPolicyFixture(t)
	f.refs[policyGoalRef] = policyFirst
	f.refs[originTipRef("goal-a")] = policyFirst
	f.remote[policyGoalRef] = policySecond
	f.head, f.checkout = policyGoalRef, policyFirst
	op := "crash-after-adoption-ref"
	f.expect(pe("claim"), pe("txn-refs"), pushExpectation{call: pushEvent{operation: "remote", remote: "origin", ref: policyGoalRef}},
		pushExpectation{call: pushEvent{operation: "tip", ref: policyGoalRef}}, pushExpectation{call: pushEvent{operation: "range", endpoint: pushPolicyBase, tip: policyFirst, goal: "goal-a"}},
		pushExpectation{call: pushEvent{operation: "tip", ref: originTipRef("goal-a")}},
		pushExpectation{call: pushEvent{operation: "fetch", remote: "origin", ref: policyGoalRef, tip: fetchRef(op)}},
		pushExpectation{call: pushEvent{operation: "range", endpoint: pushPolicyBase, tip: policySecond, goal: "goal-a"}},
		pushExpectation{call: pushEvent{operation: "clear", ref: fetchRef(op)}},
		pushExpectation{call: pushEvent{operation: "ancestor", older: policyFirst, newer: policySecond}},
		pe("head"), pe("clean"), pushExpectation{call: pushEvent{operation: "clean", cached: true}},
		pushExpectation{call: pushEvent{operation: "detach", tip: policySecond}},
		pushExpectation{call: pushEvent{operation: "move", goal: "goal-a", oldBranch: policyFirst, newBranch: policySecond, newOrigin: policySecond}})
	req := f.request(op)
	req.Hooks.AfterAdoptionRefMove = func() error { return errors.New("simulated adoption crash") }
	if _, err := pushWithRepository(req, f.repository()); err == nil || !strings.Contains(err.Error(), "simulated adoption crash") {
		t.Fatalf("crash result=%v", err)
	}
	f.done()
	if f.refs[policyGoalRef] != policySecond || f.refs[originTipRef("goal-a")] != policySecond {
		t.Fatal("goal and origin refs did not move")
	}
	if f.head != "" || f.checkout != policySecond {
		t.Fatalf("checkout = %q/%q, want detached second", f.head, f.checkout)
	}
	if f.staged || f.unstaged {
		t.Fatal("adoption crash staged a reversal")
	}
	if _, ok := f.refs[fetchRef(op)]; ok {
		t.Fatal("fetch ref survived")
	}
}
