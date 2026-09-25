package branch

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
)

const (
	statusBase = "1111111111111111111111111111111111111111"
	statusTip  = "2222222222222222222222222222222222222222"
	statusU1   = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	statusR1   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	statusU2   = "cccccccccccccccccccccccccccccccccccccccc"
	statusU3   = "dddddddddddddddddddddddddddddddddddddddd"
	statusR3   = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	statusNext = "ffffffffffffffffffffffffffffffffffffffff"
)

// Every fact names the repository and the complete ordered argument list.
type statusFact struct {
	operation string
	args      []string
	commits   []Commit
	info      KindInfo
	att       Attestation
	tip       string
	present   bool
	err       error
}

func rangeFact(repo, endpoint, tip, goal string, commits []Commit, err error) statusFact {
	return statusFact{operation: "range", args: []string{repo, endpoint, tip, goal}, commits: commits, err: err}
}
func kindFact(repo, commit, goal string, info KindInfo, err error) statusFact {
	return statusFact{operation: "kind", args: []string{repo, commit, goal}, info: info, err: err}
}
func attestFact(repo, snapshot, endpoint, goal, unit, commit string, att Attestation, err error) statusFact {
	return statusFact{operation: "attestation", args: []string{repo, snapshot, endpoint, goal, unit, commit}, att: att, err: err}
}
func tipFact(repo, ref, tip string, present bool, err error) statusFact {
	return statusFact{operation: "local-tip", args: []string{repo, ref}, tip: tip, present: present, err: err}
}
func copyStatusCommits(in []Commit) []Commit {
	out := append([]Commit(nil), in...)
	for i := range out {
		out[i].Units = append([]string(nil), out[i].Units...)
	}
	return out
}
func copyStatusAttestation(in Attestation) Attestation {
	out := in
	out.Folds = append([]Fold(nil), in.Folds...)
	out.TestsChanged = append([]TestChange(nil), in.TestsChanged...)
	if in.Carry != nil {
		carry := *in.Carry
		out.Carry = &carry
	}
	return out
}

type statusFacts struct {
	t        *testing.T
	mu       sync.Mutex
	expected []statusFact
	calls    []string
}

func newStatusFacts(t *testing.T, expected ...statusFact) *statusFacts {
	t.Helper()
	f := &statusFacts{t: t, expected: append([]statusFact(nil), expected...)}
	for i := range f.expected {
		f.expected[i].args = append([]string(nil), expected[i].args...)
		f.expected[i].commits = copyStatusCommits(expected[i].commits)
		f.expected[i].info.Units = append([]string(nil), expected[i].info.Units...)
		f.expected[i].att = copyStatusAttestation(expected[i].att)
	}
	t.Cleanup(func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		if len(f.calls) != len(f.expected) {
			t.Errorf("unconsumed status facts: consumed %d of %d; calls %v", len(f.calls), len(f.expected), f.calls)
		}
	})
	return f
}

func (f *statusFacts) take(operation string, args ...string) statusFact {
	f.mu.Lock()
	defer f.mu.Unlock()
	i := len(f.calls)
	f.calls = append(f.calls, operation)
	if i >= len(f.expected) {
		f.t.Errorf("unexpected status call %s(%q)", operation, args)
		return statusFact{err: errors.New("unexpected status call")}
	}
	want := f.expected[i]
	if want.operation != operation || !reflect.DeepEqual(want.args, args) {
		f.t.Errorf("status call %d: got %s(%q), want %s(%q)", i, operation, args, want.operation, want.args)
		return statusFact{err: errors.New("unexpected status arguments")}
	}
	return want
}
func (f *statusFacts) dependencies() statusDependencies {
	return statusDependencies{
		validatedRange: func(repo, endpoint, tip, goal string) ([]Commit, error) {
			fact := f.take("range", repo, endpoint, tip, goal)
			return copyStatusCommits(fact.commits), fact.err
		},
		kind: func(repo, commit, goal string) (KindInfo, error) {
			fact := f.take("kind", repo, commit, goal)
			info := fact.info
			info.Units = append([]string(nil), info.Units...)
			return info, fact.err
		},
		attestation: func(repo, snapshot, endpoint, goal, unit, commit string) (Attestation, error) {
			fact := f.take("attestation", repo, snapshot, endpoint, goal, unit, commit)
			return copyStatusAttestation(fact.att), fact.err
		},
		localTip: func(repo, ref string) (string, bool, error) {
			fact := f.take("local-tip", repo, ref)
			return fact.tip, fact.present, fact.err
		},
	}
}
func (f *statusFacts) assertCalls(t *testing.T, want ...string) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("calls = %v, want %v", f.calls, want)
	}
}
func (f *statusFacts) assertCallCount(t *testing.T, want int) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != want {
		t.Errorf("remote read after %d dependency calls, want %d: %v", len(f.calls), want, f.calls)
	}
}

func threeUnitCommits() []Commit {
	return []Commit{
		{ID: statusU1, Kind: Unit, Unit: "u1", Units: []string{"u1"}, Digest: "digest-one"},
		{ID: statusR1, Kind: Read, Unit: "u1"},
		{ID: statusU2, Kind: Unit, Unit: "u2", Units: []string{"u2"}, Digest: "digest-two"},
		{ID: statusU3, Kind: Unit, Unit: "u3", Units: []string{"u3"}, Digest: "digest-three"},
		{ID: statusR3, Kind: Read, Unit: "u3"},
	}
}
func threeUnitFacts(repo string) []statusFact {
	return []statusFact{
		rangeFact(repo, statusBase, statusTip, "goal-a", threeUnitCommits(), nil),
		kindFact(repo, statusR1, "goal-a", KindInfo{Kind: Read, CommitID: statusU1}, nil),
		attestFact(repo, statusTip, statusBase, "goal-a", "u1", statusU1, Attestation{Unit: "u1"}, nil),
		kindFact(repo, statusR3, "goal-a", KindInfo{Kind: Read, CommitID: statusU3}, nil),
		attestFact(repo, statusTip, statusBase, "goal-a", "u3", statusU3, Attestation{Unit: "u3"}, nil),
	}
}

func TestStatusLandReadyPrefixAndParkSafety(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	ref := goalBranchRef("goal-a")
	facts := threeUnitFacts(repo)
	facts = append(facts, tipFact(repo, ref, statusTip, true, nil))
	facts = append(facts, threeUnitFacts(repo)...)
	facts = append(facts,
		tipFact(repo, ref, statusNext, true, nil),
		tipFact(repo, ref, statusTip, true, nil),
		tipFact(repo, ref, "", false, nil),
		tipFact(repo, ref, "", false, nil),
		tipFact(repo, ref, "", false, nil),
		kindFact(repo, statusU3, "goal-a", KindInfo{Kind: Unit, CommitID: statusU3}, nil),
		tipFact(repo, ref, "", false, nil),
		kindFact(repo, statusU3, "goal-a", KindInfo{Kind: Unit, CommitID: statusU3}, nil),
	)
	f := newStatusFacts(t, facts...)
	deps := f.dependencies()
	status, err := inspectStatus(repo, statusBase, statusTip, "goal-a", deps)
	if err != nil || status.Prefix != 1 || len(status.Units) != 3 {
		t.Fatalf("status = %+v, err = %v", status, err)
	}
	for i, want := range []UnitStatus{
		{Unit: "u1", Units: []string{"u1"}, Commit: statusU1, Digest: "digest-one", ReadState: "read clean"},
		{Unit: "u2", Units: []string{"u2"}, Commit: statusU2, Digest: "digest-two", ReadState: "built"},
		{Unit: "u3", Units: []string{"u3"}, Commit: statusU3, Digest: "digest-three", ReadState: "read clean"},
	} {
		if !reflect.DeepEqual(status.Units[i], want) {
			t.Fatalf("unit %d = %+v, want %+v", i, status.Units[i], want)
		}
	}
	if status.Tip != statusTip || !reflect.DeepEqual(status.Commits, threeUnitCommits()) {
		t.Fatalf("status tip/commits = %+v", status)
	}
	remoteCalls := 0
	remote := func() (string, string, bool, error) {
		remoteCalls++
		if remoteCalls == 1 {
			f.assertCallCount(t, 6)
		} else {
			f.assertCallCount(t, 12)
		}
		return statusBase, statusTip, true, nil
	}
	parked, err := checkParkBranch(repo, "goal-a", "continue", remote, deps)
	wantSummary := fmt.Sprintf("goal/goal-a last unit u3 commit %s is read clean", statusU3)
	if err != nil || !parked.Branch || parked.Summary != wantSummary || remoteCalls != 1 {
		t.Fatalf("park state = %+v, err = %v, remote calls = %d", parked, err, remoteCalls)
	}
	_, err = checkParkBranch(repo, "goal-a", "continue", remote, deps)
	var refusal *OpError
	wantRefusal := fmt.Sprintf("local goal/goal-a is %s while origin is %s; push the branch before parking", statusNext, statusTip)
	if !errors.As(err, &refusal) || refusal.Code != ParkUnpushedCode || refusal.Message != wantRefusal || remoteCalls != 2 {
		t.Fatalf("unpushed park = %v, remote calls = %d", err, remoteCalls)
	}
	_, err = checkParkBranch(repo, "goal-a", "continue", func() (string, string, bool, error) {
		remoteCalls++
		f.assertCallCount(t, 13)
		return statusBase, "", false, nil
	}, deps)
	if !errors.As(err, &refusal) || refusal.Code != ParkUnpushedCode || refusal.Message != "local goal/goal-a is "+statusTip+" while origin is <absent>; push the branch before parking" || remoteCalls != 3 {
		t.Fatalf("absent origin park = %v, remote calls = %d", err, remoteCalls)
	}
	unreadable := func() (string, string, bool, error) {
		remoteCalls++
		return "", "", false, errors.New("remote unavailable")
	}
	if state, err := checkParkBranch(repo, "goal-a", "ordinary next step", unreadable, deps); err != nil || state.Branch || remoteCalls != 3 {
		t.Fatalf("branchless park = %+v, err = %v, remote calls = %d", state, err, remoteCalls)
	}
	if sweep, err := shouldSweep(repo, "goal-a", "ordinary next step", deps); err != nil || sweep {
		t.Fatalf("ordinary branchless sweep = %v, err = %v", sweep, err)
	}
	narrated := "resume commit (" + statusU3 + ")," // The full ID survives punctuation trimming.
	if sweep, err := shouldSweep(repo, "goal-a", "ignore "+statusU3[:39]+"; "+narrated, deps); err != nil || !sweep {
		t.Fatalf("narrated sweep = %v, err = %v", sweep, err)
	}
	_, err = checkParkBranch(repo, "goal-a", narrated, unreadable, deps)
	if !errors.As(err, &refusal) || refusal.Code != ParkUnpushedCode || refusal.Message != "this checkout has no goal/goal-a; fetch it and check it out" || remoteCalls != 3 {
		t.Fatalf("missing narrated branch = %v, remote calls = %d", err, remoteCalls)
	}
	f.assertCalls(t, "range", "kind", "attestation", "kind", "attestation", "local-tip", "range", "kind", "attestation", "kind", "attestation", "local-tip", "local-tip", "local-tip", "local-tip", "local-tip", "kind", "local-tip", "kind")
}

func TestParkRefusalDoesNotClaimOriginState(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	f := newStatusFacts(t,
		tipFact(repo, goalBranchRef("goal-a"), "", false, nil),
		kindFact(repo, statusU1, "goal-a", KindInfo{Kind: Unit, CommitID: statusU1}, nil),
	)
	remoteCalls := 0
	_, err := checkParkBranch(repo, "goal-a", "resume commit "+statusU1, func() (string, string, bool, error) {
		remoteCalls++
		return "", "", false, nil
	}, f.dependencies())
	var refusal *OpError
	if !errors.As(err, &refusal) || refusal.Code != ParkUnpushedCode || refusal.Message != "this checkout has no goal/goal-a; fetch it and check it out" || remoteCalls != 0 {
		t.Fatalf("missing local branch refusal = %v, remote calls = %d", err, remoteCalls)
	}
	f.assertCalls(t, "local-tip", "kind")
}

func TestStatusAndReadBindWholeBuildList(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	units := []string{"5", "6", "7a", "7b"}
	commits := []Commit{{ID: statusU1, Kind: Unit, Unit: "5+6+7a+7b", Units: units, Digest: "multi-digest"}, {ID: statusR1, Kind: Read, Unit: "5+6+7a+7b"}}
	f := newStatusFacts(t,
		rangeFact(repo, statusBase, statusTip, "goal-a", commits, nil),
		kindFact(repo, statusR1, "goal-a", KindInfo{Kind: Read, CommitID: statusU1, Units: units}, nil),
		attestFact(repo, statusTip, statusBase, "goal-a", "5+6+7a+7b", statusU1, Attestation{Unit: "5+6+7a+7b"}, nil),
	)
	units[0] = "changed"
	status, err := inspectStatus(repo, statusBase, statusTip, "goal-a", f.dependencies())
	if err != nil || status.Prefix != 1 || len(status.Units) != 1 || status.Units[0].Unit != "5+6+7a+7b" ||
		!reflect.DeepEqual(status.Units[0].Units, []string{"5", "6", "7a", "7b"}) || status.Units[0].Commit != statusU1 || status.Units[0].Digest != "multi-digest" || status.Units[0].ReadState != "read clean" {
		t.Fatalf("multi-unit status = %+v, err = %v", status, err)
	}
	status.Commits[0].Units[0] = "mutated"
	if status.Units[0].Units[0] != "5" {
		t.Fatal("unit status aliases the range commit units")
	}
	f.assertCalls(t, "range", "kind", "attestation")
}

func TestStatusDependenciesPropagateErrors(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rangeErr := errors.New("range unavailable")
	kindErr := errors.New("kind unavailable")
	tipErr := errors.New("local tip unavailable")
	attErr := errors.New("attestation unavailable")
	t.Run("range", func(t *testing.T) {
		f := newStatusFacts(t, rangeFact(repo, statusBase, statusTip, "goal-a", nil, rangeErr))
		_, err := inspectStatus(repo, statusBase, statusTip, "goal-a", f.dependencies())
		if !errors.Is(err, rangeErr) || err.Error() != "range unavailable" {
			t.Fatalf("range error = %v", err)
		}
		f.assertCalls(t, "range")
	})
	t.Run("kind", func(t *testing.T) {
		f := newStatusFacts(t, rangeFact(repo, statusBase, statusTip, "goal-a", []Commit{{ID: statusR1, Kind: Read, Unit: "u1"}}, nil), kindFact(repo, statusR1, "goal-a", KindInfo{}, kindErr))
		_, err := inspectStatus(repo, statusBase, statusTip, "goal-a", f.dependencies())
		if !errors.Is(err, kindErr) || err.Error() != "kind unavailable" {
			t.Fatalf("kind error = %v", err)
		}
		f.assertCalls(t, "range", "kind")
	})
	t.Run("sweep local tip", func(t *testing.T) {
		f := newStatusFacts(t, tipFact(repo, goalBranchRef("goal-a"), "", false, tipErr))
		sweep, err := shouldSweep(repo, "goal-a", "resume "+statusU1, f.dependencies())
		if sweep || !errors.Is(err, tipErr) || err.Error() != "local tip unavailable" {
			t.Fatalf("sweep = %v, err = %v", sweep, err)
		}
		f.assertCalls(t, "local-tip")
	})
	t.Run("park local tip", func(t *testing.T) {
		f := newStatusFacts(t, tipFact(repo, goalBranchRef("goal-a"), "", false, tipErr))
		remoteCalls := 0
		state, err := checkParkBranch(repo, "goal-a", "resume "+statusU1, func() (string, string, bool, error) { remoteCalls++; return "", "", false, nil }, f.dependencies())
		if state != (ParkBranchState{}) || !errors.Is(err, tipErr) || err.Error() != "local tip unavailable" || remoteCalls != 0 {
			t.Fatalf("park = %+v, err = %v, remote calls = %d", state, err, remoteCalls)
		}
		f.assertCalls(t, "local-tip")
	})
	t.Run("attestation", func(t *testing.T) {
		f := newStatusFacts(t,
			rangeFact(repo, statusBase, statusTip, "goal-a", []Commit{{ID: statusU1, Kind: Unit, Unit: "u1", Units: []string{"u1"}}, {ID: statusR1, Kind: Read, Unit: "u1"}}, nil),
			kindFact(repo, statusR1, "goal-a", KindInfo{Kind: Read, CommitID: statusU1}, nil),
			attestFact(repo, statusTip, statusBase, "goal-a", "u1", statusU1, Attestation{}, attErr),
		)
		status, err := inspectStatus(repo, statusBase, statusTip, "goal-a", f.dependencies())
		if err != nil || status.Prefix != 0 || len(status.Units) != 1 || status.Units[0].ReadState != "needs read" {
			t.Fatalf("attestation status = %+v, err = %v", status, err)
		}
		f.assertCalls(t, "range", "kind", "attestation")
	})
	t.Run("repeated unit", func(t *testing.T) {
		unit := Commit{ID: statusU1, Kind: Unit, Unit: "u1", Units: []string{"u1"}}
		f := newStatusFacts(t, rangeFact(repo, statusBase, statusTip, "goal-a", []Commit{unit, unit}, nil))
		_, err := inspectStatus(repo, statusBase, statusTip, "goal-a", f.dependencies())
		var refusal *OpError
		if !errors.As(err, &refusal) || refusal.Code != RangeCode || refusal.Message != "goal branch repeats unit commit "+statusU1 {
			t.Fatalf("repeated unit = %v", err)
		}
		f.assertCalls(t, "range")
	})
	t.Run("unmatched read", func(t *testing.T) {
		f := newStatusFacts(t,
			rangeFact(repo, statusBase, statusTip, "goal-a", []Commit{{ID: statusU1, Kind: Unit, Unit: "u1"}, {ID: statusR1, Kind: Read, Unit: "u1"}}, nil),
			kindFact(repo, statusR1, "goal-a", KindInfo{Kind: Read, CommitID: statusU2}, nil),
		)
		status, err := inspectStatus(repo, statusBase, statusTip, "goal-a", f.dependencies())
		if err != nil || len(status.Units) != 1 || status.Units[0].ReadState != "built" || status.Prefix != 0 {
			t.Fatalf("unmatched read status = %+v, err = %v", status, err)
		}
		f.assertCalls(t, "range", "kind")
	})
}

func TestStatusDependenciesAreIsolatedPerCall(t *testing.T) {
	t.Parallel()
	repoA, repoB := t.TempDir(), t.TempDir()
	ref := goalBranchRef("goal-a")
	a := newStatusFacts(t,
		rangeFact(repoA, statusBase, statusTip, "goal-a", []Commit{{ID: statusU1, Kind: Unit, Unit: "u1", Units: []string{"u1"}}}, nil),
		tipFact(repoA, ref, statusTip, true, nil),
		tipFact(repoA, ref, statusTip, true, nil),
		rangeFact(repoA, statusBase, statusTip, "goal-a", []Commit{{ID: statusU1, Kind: Unit, Unit: "u1", Units: []string{"u1"}}}, nil),
	)
	b := newStatusFacts(t,
		rangeFact(repoB, statusBase, statusTip, "goal-a", nil, nil),
		tipFact(repoB, ref, "", false, nil),
		tipFact(repoB, ref, "", false, nil),
	)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		deps := a.dependencies()
		status, err := inspectStatus(repoA, statusBase, statusTip, "goal-a", deps)
		if err != nil || status.Prefix != 0 || len(status.Units) != 1 || status.Units[0].ReadState != "built" {
			results <- fmt.Errorf("A status = %+v, err = %v", status, err)
			return
		}
		sweep, err := shouldSweep(repoA, "goal-a", "ordinary", deps)
		if err != nil || !sweep {
			results <- fmt.Errorf("A sweep = %v, err = %v", sweep, err)
			return
		}
		remoteCalls := 0
		park, err := checkParkBranch(repoA, "goal-a", "ordinary", func() (string, string, bool, error) {
			remoteCalls++
			a.assertCallCount(t, 3)
			return statusBase, statusTip, true, nil
		}, deps)
		if err != nil || !park.Branch || park.Summary != "goal/goal-a last unit u1 commit "+statusU1+" is built" || remoteCalls != 1 {
			results <- fmt.Errorf("A park = %+v, err = %v, remote calls = %d", park, err, remoteCalls)
			return
		}
		results <- nil
	}()
	go func() {
		defer wg.Done()
		<-start
		deps := b.dependencies()
		status, err := inspectStatus(repoB, statusBase, statusTip, "goal-a", deps)
		if err != nil || status.Prefix != 0 || len(status.Units) != 0 {
			results <- fmt.Errorf("B status = %+v, err = %v", status, err)
			return
		}
		sweep, err := shouldSweep(repoB, "goal-a", "ordinary", deps)
		if err != nil || sweep {
			results <- fmt.Errorf("B sweep = %v, err = %v", sweep, err)
			return
		}
		remoteCalls := 0
		park, err := checkParkBranch(repoB, "goal-a", "ordinary", func() (string, string, bool, error) { remoteCalls++; return "", "", false, nil }, deps)
		if err != nil || park.Branch || remoteCalls != 0 {
			results <- fmt.Errorf("B park = %+v, err = %v, remote calls = %d", park, err, remoteCalls)
			return
		}
		results <- nil
	}()
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Error(err)
		}
	}
	a.assertCalls(t, "range", "local-tip", "local-tip", "range")
	b.assertCalls(t, "range", "local-tip", "local-tip")
}
