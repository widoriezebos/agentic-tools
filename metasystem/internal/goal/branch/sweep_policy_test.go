package branch

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	sweepPolicyGoal     = "goal-a"
	sweepPolicyEndpoint = "1111111111111111111111111111111111111111"
	sweepPolicyOrigin   = "2222222222222222222222222222222222222222"
	sweepPolicyLocal    = "3333333333333333333333333333333333333333"
	sweepPolicyBase     = "4444444444444444444444444444444444444444"
	sweepPolicyUnknown  = "5555555555555555555555555555555555555555"
)

type sweepPolicyInvocation struct {
	method, repo, remote, ref, destination string
	endpoint, tip, goal, older, newer      string
	args                                   []string
}

type sweepPolicyCall struct {
	want    sweepPolicyInvocation
	tip     string
	present bool
	bytes   []byte
	commits []Commit
	covered bool
	err     error
}

type sweepPolicyRig struct {
	t           *testing.T
	calls       []sweepPolicyCall
	next        int
	tips        map[string]string
	localTip    string
	clearedRefs map[string]bool
}

func newSweepPolicyRig(t *testing.T, tips map[string]string, localTip string, calls ...sweepPolicyCall) *sweepPolicyRig {
	t.Helper()
	rig := &sweepPolicyRig{t: t, calls: calls, tips: tips, localTip: localTip, clearedRefs: map[string]bool{}}
	t.Cleanup(func() {
		if rig.next != len(rig.calls) {
			t.Errorf("sweep consumed %d of %d planned calls; remaining: %+v", rig.next, len(rig.calls), rig.calls[rig.next:])
		}
	})
	return rig
}

func (r *sweepPolicyRig) take(got sweepPolicyInvocation) sweepPolicyCall {
	r.t.Helper()
	if r.next == len(r.calls) {
		r.t.Fatalf("unexpected sweep call: %+v", got)
	}
	call := r.calls[r.next]
	r.next++
	if !reflect.DeepEqual(got, call.want) {
		r.t.Fatalf("sweep call %d = %+v, want %+v", r.next, got, call.want)
	}
	return call
}

func (r *sweepPolicyRig) RemoteTip(repo, remote, ref string) (string, bool, error) {
	call := r.take(sweepPolicyInvocation{method: "RemoteTip", repo: repo, remote: remote, ref: ref})
	if r.tips[remote] != call.tip || (call.tip != "") != call.present {
		r.t.Fatalf("%s remote holds %q, planned reply is (%q, %t)", remote, r.tips[remote], call.tip, call.present)
	}
	return call.tip, call.present, call.err
}

func (r *sweepPolicyRig) Fetch(repo, remote, ref, destination string) error {
	return r.take(sweepPolicyInvocation{method: "Fetch", repo: repo, remote: remote, ref: ref, destination: destination}).err
}

func (r *sweepPolicyRig) Push(repo, remote, ref, expected, tip string) (CASOutcome, error) {
	r.t.Fatalf("sweep tried to push %s %s %s %s %s after a refusal", repo, remote, ref, expected, tip)
	return CASUnknown, errors.New("unexpected push")
}

func (r *sweepPolicyRig) dependencies() sweepDependencies {
	return sweepDependencies{
		localBranchTip: func(repo, ref string) (string, bool, error) {
			call := r.take(sweepPolicyInvocation{method: "LocalTip", repo: repo, ref: ref})
			if r.localTip != call.tip || (call.tip != "") != call.present {
				r.t.Fatalf("local ref holds %q, planned reply is (%q, %t)", r.localTip, call.tip, call.present)
			}
			return call.tip, call.present, call.err
		},
		gitOutput: func(repo string, args ...string) ([]byte, error) {
			call := r.take(sweepPolicyInvocation{method: "GitRead", repo: repo, args: args})
			return append([]byte(nil), call.bytes...), call.err
		},
		validateRange: func(repo, endpoint, tip, goal string) ([]Commit, error) {
			call := r.take(sweepPolicyInvocation{method: "ValidateRange", repo: repo, endpoint: endpoint, tip: tip, goal: goal})
			return append([]Commit(nil), call.commits...), call.err
		},
		ancestor: func(repo, older, newer string) (bool, error) {
			call := r.take(sweepPolicyInvocation{method: "Ancestor", repo: repo, older: older, newer: newer})
			return call.covered, call.err
		},
		clearRef: func(repo, ref string) error {
			call := r.take(sweepPolicyInvocation{method: "ClearRef", repo: repo, ref: ref})
			r.clearedRefs[ref] = true
			return call.err
		},
	}
}

func (r *sweepPolicyRig) request(repo, endpoint string) SweepRequest {
	return SweepRequest{Repo: repo, Remote: "origin", EndpointTip: endpoint, GoalID: sweepPolicyGoal,
		PushTransport: r, CheckClaim: func() error {
			return r.take(sweepPolicyInvocation{method: "CheckClaim"}).err
		}}
}

func sweepClaim() sweepPolicyCall {
	return sweepPolicyCall{want: sweepPolicyInvocation{method: "CheckClaim"}}
}

func sweepRemote(repo, remote, tip string) sweepPolicyCall {
	return sweepPolicyCall{want: sweepPolicyInvocation{method: "RemoteTip", repo: repo, remote: remote, ref: goalBranchRef(sweepPolicyGoal)},
		tip: tip, present: tip != ""}
}

func sweepLocal(repo, tip string) sweepPolicyCall {
	return sweepPolicyCall{want: sweepPolicyInvocation{method: "LocalTip", repo: repo, ref: goalBranchRef(sweepPolicyGoal)},
		tip: tip, present: tip != ""}
}

func sweepRead(repo string, data []byte, args ...string) sweepPolicyCall {
	return sweepPolicyCall{want: sweepPolicyInvocation{method: "GitRead", repo: repo, args: args}, bytes: data}
}

func sweepMissingGoal(repo, endpoint string) sweepPolicyCall {
	return sweepPolicyCall{want: sweepPolicyInvocation{method: "GitRead", repo: repo,
		args: []string{"show", endpoint + ":metasystem/records/goals/" + sweepPolicyGoal + ".md"}},
		err: errors.New("goal record absent")}
}

func sweepRange(repo, endpoint, tip string, commits []Commit, err error) sweepPolicyCall {
	return sweepPolicyCall{want: sweepPolicyInvocation{method: "ValidateRange", repo: repo, endpoint: endpoint,
		tip: tip, goal: sweepPolicyGoal}, commits: commits, err: err}
}

func sweepFetch(repo, remote string) sweepPolicyCall {
	return sweepPolicyCall{want: sweepPolicyInvocation{method: "Fetch", repo: repo, remote: remote,
		ref: goalBranchRef(sweepPolicyGoal), destination: fetchRef("sweep-" + remote + "-" + sweepPolicyGoal)}}
}

func sweepClear(repo, remote string) sweepPolicyCall {
	return sweepPolicyCall{want: sweepPolicyInvocation{method: "ClearRef", repo: repo,
		ref: fetchRef("sweep-" + remote + "-" + sweepPolicyGoal)}}
}

func sweepAncestor(repo, older, newer string, covered bool) sweepPolicyCall {
	return sweepPolicyCall{want: sweepPolicyInvocation{method: "Ancestor", repo: repo, older: older, newer: newer}, covered: covered}
}

func sweepWorktreeBytes(path, tip string) []byte {
	return []byte("worktree " + path + "\nHEAD " + tip + "\nbranch " + goalBranchRef(sweepPolicyGoal) + "\n\n")
}

func sweepWorktrees(repo string, data []byte) sweepPolicyCall {
	return sweepRead(repo, data, "worktree", "list", "--porcelain")
}

func sweepStatus(path string, data []byte) sweepPolicyCall {
	return sweepRead(path, data, "status", "--porcelain=v1", "--untracked-files=all")
}

func sweepCheckCalls(repo, endpoint, tip string, commits []Commit) []sweepPolicyCall {
	return []sweepPolicyCall{
		sweepRead(repo, []byte(sweepPolicyBase+"\n"), "merge-base", endpoint, tip),
		sweepRead(repo, []byte(sweepPolicyOrigin+"\n"), "log", "--format=%(trailers:key=Goal-Source,valueonly)", sweepPolicyBase+".."+endpoint),
		sweepRange(repo, endpoint, tip, commits, nil),
	}
}

func sweepCleanOriginCalls(repo string) []sweepPolicyCall {
	commits := []Commit{{ID: sweepPolicyOrigin, Kind: Plan}}
	calls := []sweepPolicyCall{sweepFetch(repo, "origin"), sweepRange(repo, sweepPolicyEndpoint, sweepPolicyOrigin, commits, nil), sweepClear(repo, "origin")}
	return append(calls, sweepCheckCalls(repo, sweepPolicyEndpoint, sweepPolicyOrigin, commits)...)
}

func sweepPolicyTips(origin, transport string) map[string]string {
	return map[string]string{"origin": origin, "transport": transport}
}

type sweepWorktreeEntry struct {
	info os.FileInfo
	data []byte
}

func sweepWorktreeSnapshot(t *testing.T, root string) map[string]sweepWorktreeEntry {
	t.Helper()
	entries := make(map[string]sweepWorktreeEntry)
	var visit func(string)
	visit = func(path string) {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		entry := sweepWorktreeEntry{info: info}
		if info.IsDir() {
			children, err := os.ReadDir(path)
			if err != nil {
				t.Fatal(err)
			}
			entries[name] = entry
			for _, child := range children {
				visit(filepath.Join(path, child.Name()))
			}
			return
		}
		if !info.Mode().IsRegular() {
			t.Fatalf("worktree entry %s has unsupported mode %s", path, info.Mode())
		}
		entry.data, err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		entries[name] = entry
	}
	visit(root)
	return entries
}

func TestSweepAbandonedStillValidatesRemoteRange(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	refusal := &RangeError{Code: RangeCode, Commit: sweepPolicyUnknown, Reason: "expected exactly one kind trailer"}
	rig := newSweepPolicyRig(t, sweepPolicyTips(sweepPolicyUnknown, ""), "",
		sweepClaim(), sweepRemote(repo, "origin", sweepPolicyUnknown), sweepLocal(repo, ""), sweepWorktrees(repo, nil),
		sweepFetch(repo, "origin"), sweepRange(repo, sweepPolicyEndpoint, sweepPolicyUnknown, nil, refusal), sweepClear(repo, "origin"))
	req := rig.request(repo, sweepPolicyEndpoint)
	req.Abandoned = true
	_, err := sweepWithDependencies(req, rig.dependencies())
	var got *RangeError
	if !errors.As(err, &got) || got.Code != RangeCode || got.Commit != sweepPolicyUnknown || got.Reason != refusal.Reason {
		t.Fatalf("abandoned unknown commit sweep=%v", err)
	}
	if rig.tips["origin"] != sweepPolicyUnknown {
		t.Fatalf("origin ref after refused sweep = %s, want %s", rig.tips["origin"], sweepPolicyUnknown)
	}
	if !rig.clearedRefs[fetchRef("sweep-origin-"+sweepPolicyGoal)] {
		t.Fatal("unknown remote fetch ref was not cleared")
	}
}

func TestSweepRefusesUnlandedLocalTip(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	calls := []sweepPolicyCall{sweepClaim(), sweepRemote(repo, "origin", sweepPolicyOrigin), sweepLocal(repo, sweepPolicyLocal), sweepWorktrees(repo, nil)}
	calls = append(calls, sweepCleanOriginCalls(repo)...)
	calls = append(calls, sweepAncestor(repo, sweepPolicyLocal, sweepPolicyOrigin, false))
	calls = append(calls, sweepCheckCalls(repo, sweepPolicyEndpoint, sweepPolicyLocal,
		[]Commit{{ID: sweepPolicyOrigin, Kind: Plan}, {ID: sweepPolicyLocal, Kind: Plan}})...)
	calls = append(calls, sweepMissingGoal(repo, sweepPolicyEndpoint))
	rig := newSweepPolicyRig(t, sweepPolicyTips(sweepPolicyOrigin, ""), sweepPolicyLocal, calls...)
	_, err := sweepWithDependencies(rig.request(repo, sweepPolicyEndpoint), rig.dependencies())
	var refusal *OpError
	if !errors.As(err, &refusal) || refusal.Code != SweepUnlandedCode ||
		!strings.Contains(refusal.Message, "goal/goal-a") || !strings.Contains(refusal.Message, "local") ||
		!strings.Contains(refusal.Message, sweepPolicyLocal+" (plan )") {
		t.Fatalf("unlanded local sweep = %v", err)
	}
}

func TestSweepRefusalPreservesRefsAndWorktree(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "goal-worktree")
	if err := os.Mkdir(worktree, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(worktree, "kept.txt")
	if err := os.WriteFile(marker, []byte("keep this worktree\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Join(worktree, ".git")
	if err := os.Mkdir(gitDir, 0o700); err != nil {
		t.Fatal(err)
	}
	branchMarker := filepath.Join(gitDir, "HEAD")
	wantBranch := []byte("ref: " + goalBranchRef(sweepPolicyGoal) + "\n")
	if err := os.WriteFile(branchMarker, wantBranch, 0o600); err != nil {
		t.Fatal(err)
	}
	worktreeBefore := sweepWorktreeSnapshot(t, worktree)
	worktreesBefore := sweepWorktreeBytes(worktree, sweepPolicyLocal)
	calls := []sweepPolicyCall{sweepClaim(), sweepRemote(repo, "origin", sweepPolicyOrigin),
		sweepRemote(repo, "transport", sweepPolicyOrigin), sweepLocal(repo, sweepPolicyLocal),
		sweepWorktrees(repo, worktreesBefore), sweepStatus(worktree, nil)}
	calls = append(calls, sweepCleanOriginCalls(repo)...)
	calls = append(calls, sweepFetch(repo, "transport"), sweepRange(repo, sweepPolicyEndpoint, sweepPolicyOrigin,
		[]Commit{{ID: sweepPolicyOrigin, Kind: Plan}}, nil), sweepClear(repo, "transport"))
	calls = append(calls, sweepAncestor(repo, sweepPolicyLocal, sweepPolicyOrigin, false))
	calls = append(calls, sweepCheckCalls(repo, sweepPolicyEndpoint, sweepPolicyLocal,
		[]Commit{{ID: sweepPolicyOrigin, Kind: Plan}, {ID: sweepPolicyLocal, Kind: Plan}})...)
	calls = append(calls, sweepMissingGoal(repo, sweepPolicyEndpoint))
	rig := newSweepPolicyRig(t, sweepPolicyTips(sweepPolicyOrigin, sweepPolicyOrigin), sweepPolicyLocal, calls...)
	req := rig.request(repo, sweepPolicyEndpoint)
	req.Transport = "transport"
	_, err := sweepWithDependencies(req, rig.dependencies())
	var refusal *OpError
	if !errors.As(err, &refusal) || refusal.Code != SweepUnlandedCode {
		t.Fatalf("unlanded local sweep = %v", err)
	}
	for _, remote := range []string{"origin", "transport"} {
		if got := rig.tips[remote]; got != sweepPolicyOrigin {
			t.Fatalf("%s ref after refusal = %q, want %s", remote, got, sweepPolicyOrigin)
		}
	}
	if rig.localTip != sweepPolicyLocal {
		t.Fatalf("local ref after refusal = %s, want %s", rig.localTip, sweepPolicyLocal)
	}
	worktreeAfter := sweepWorktreeSnapshot(t, worktree)
	if len(worktreeAfter) != len(worktreeBefore) {
		t.Fatalf("worktree membership after refusal = %v, want %v", worktreeAfter, worktreeBefore)
	}
	for name, before := range worktreeBefore {
		after, present := worktreeAfter[name]
		if !present || !os.SameFile(before.info, after.info) || before.info.Mode() != after.info.Mode() || !bytes.Equal(before.data, after.data) {
			t.Fatalf("worktree entry %q changed after refusal: present=%t, before=%q, after=%q", name, present, before.data, after.data)
		}
	}
	if got, readErr := os.ReadFile(branchMarker); readErr != nil || !bytes.Equal(got, wantBranch) {
		t.Fatalf("worktree branch attachment after refusal = %q, %v", got, readErr)
	}
	if got, readErr := os.ReadFile(marker); readErr != nil || string(got) != "keep this worktree\n" {
		t.Fatalf("worktree contents after refusal = %q, %v", got, readErr)
	}
}

func TestSweepRefusalNamesEveryUnlandedCommit(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	commits := make([]string, 10)
	rangeCommits := []Commit{{ID: sweepPolicyOrigin, Kind: Plan}}
	for index := range commits {
		commits[index] = fmt.Sprintf("%040x", 100+index)
		rangeCommits = append(rangeCommits, Commit{ID: commits[index], Kind: Unit, Unit: fmt.Sprintf("tail-%d", index)})
	}
	local := commits[9]
	calls := []sweepPolicyCall{sweepClaim(), sweepRemote(repo, "origin", sweepPolicyOrigin), sweepLocal(repo, local), sweepWorktrees(repo, nil)}
	calls = append(calls, sweepCleanOriginCalls(repo)...)
	calls = append(calls, sweepAncestor(repo, local, sweepPolicyOrigin, false))
	calls = append(calls, sweepCheckCalls(repo, sweepPolicyEndpoint, local, rangeCommits)...)
	for range commits {
		calls = append(calls, sweepMissingGoal(repo, sweepPolicyEndpoint))
	}
	rig := newSweepPolicyRig(t, sweepPolicyTips(sweepPolicyOrigin, ""), local, calls...)
	_, err := sweepWithDependencies(rig.request(repo, sweepPolicyEndpoint), rig.dependencies())
	var refusal *OpError
	if !errors.As(err, &refusal) || refusal.Code != SweepUnlandedCode {
		t.Fatalf("unlanded local sweep = %v", err)
	}
	wantPrefix := fmt.Sprintf("goal/goal-a at local tip %s has unlanded commits: ", local)
	if !strings.HasPrefix(refusal.Message, wantPrefix) {
		t.Fatalf("refusal = %q, want prefix %q", refusal.Message, wantPrefix)
	}
	previous := -1
	for index, commit := range commits[:8] {
		entry := fmt.Sprintf("%s (unit tail-%d)", commit, index)
		at := strings.Index(refusal.Message, entry)
		if at <= previous {
			t.Fatalf("refusal does not name commits oldest first: %q", refusal.Message)
		}
		previous = at
	}
	if strings.Contains(refusal.Message, commits[8]) || strings.Contains(refusal.Message, commits[9]+" (unit tail-9)") ||
		!strings.HasSuffix(refusal.Message, "and 2 more") {
		t.Fatalf("refusal cap = %q", refusal.Message)
	}
}

func TestSweepRefusesDirtyGoalWorktree(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	dirty := filepath.Join(repo, "metasystem", "one.go")
	if err := os.MkdirAll(filepath.Dir(dirty), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dirty, []byte("dirty worktree\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rig := newSweepPolicyRig(t, sweepPolicyTips(sweepPolicyOrigin, ""), sweepPolicyOrigin,
		sweepClaim(), sweepRemote(repo, "origin", sweepPolicyOrigin), sweepLocal(repo, sweepPolicyOrigin),
		sweepWorktrees(repo, sweepWorktreeBytes(repo, sweepPolicyOrigin)), sweepStatus(repo, []byte(" M metasystem/one.go\n")))
	_, err := sweepWithDependencies(rig.request(repo, sweepPolicyEndpoint), rig.dependencies())
	if err == nil || !strings.Contains(err.Error(), repo) || !strings.Contains(err.Error(), "metasystem/one.go") {
		t.Fatalf("dirty worktree sweep = %v", err)
	}
	if rig.tips["origin"] == "" {
		t.Fatal("dirty worktree refusal deleted origin first")
	}
	if got, readErr := os.ReadFile(dirty); readErr != nil || string(got) != "dirty worktree\n" {
		t.Fatalf("dirty worktree contents after refusal = %q, %v", got, readErr)
	}
}

func TestSweepRefusesTransportTipWithUnlandedCommit(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	calls := []sweepPolicyCall{sweepClaim(), sweepRemote(repo, "origin", sweepPolicyOrigin),
		sweepRemote(repo, "transport", sweepPolicyLocal), sweepLocal(repo, ""), sweepWorktrees(repo, nil)}
	calls = append(calls, sweepCleanOriginCalls(repo)...)
	calls = append(calls, sweepFetch(repo, "transport"), sweepRange(repo, sweepPolicyEndpoint, sweepPolicyLocal,
		[]Commit{{ID: sweepPolicyOrigin, Kind: Plan}, {ID: sweepPolicyLocal, Kind: Plan}}, nil), sweepClear(repo, "transport"))
	calls = append(calls, sweepAncestor(repo, sweepPolicyLocal, sweepPolicyOrigin, false))
	calls = append(calls, sweepCheckCalls(repo, sweepPolicyEndpoint, sweepPolicyLocal,
		[]Commit{{ID: sweepPolicyOrigin, Kind: Plan}, {ID: sweepPolicyLocal, Kind: Plan}})...)
	calls = append(calls, sweepMissingGoal(repo, sweepPolicyEndpoint))
	rig := newSweepPolicyRig(t, sweepPolicyTips(sweepPolicyOrigin, sweepPolicyLocal), "", calls...)
	req := rig.request(repo, sweepPolicyEndpoint)
	req.Transport = "transport"
	_, err := sweepWithDependencies(req, rig.dependencies())
	var refusal *OpError
	if !errors.As(err, &refusal) || refusal.Code != SweepUnlandedCode ||
		!strings.Contains(refusal.Message, "at transport tip "+sweepPolicyLocal) {
		t.Fatalf("unlanded transport sweep = %v", err)
	}
	if rig.tips["origin"] == "" {
		t.Fatal("transport refusal deleted origin first")
	}
}
