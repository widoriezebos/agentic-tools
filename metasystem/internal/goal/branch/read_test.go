package branch_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func writeReadJob(t *testing.T, root, job, commit, status string, openFinding bool) {
	t.Helper()
	subject, present, err := dispatch.ComputeReadSubject(dispatch.ReadSubjectRequest{RepoRoot: root, Role: "code-critic", Reviews: "commit:" + commit})
	if err != nil || !present {
		t.Fatalf("subject present=%v err=%v", present, err)
	}
	writeReadJobWithSubject(t, root, job, commit, status, openFinding, subject)
}

func writeReadJobWithSubject(t *testing.T, root, job, commit, status string, openFinding bool, subject readsubject.ReadSubject) {
	t.Helper()
	register := []any{}
	if openFinding {
		register = append(register, map[string]any{"findingId": "defect-a", "critic": job, "rigorClass": "bounded",
			"factsDigest": strings.Repeat("0", 64), "status": "open", "evidenceDigest": strings.Repeat("1", 64), "multiplicity": 1})
	}
	record := map[string]any{"jobId": job, "role": "code-critic", "round": 1, "status": status,
		"reviews": "commit:" + commit, "goalId": "goal-a", "goalRevision": 1, "findingRegister": register,
		"findingRegisterRound": 1, "findingRegisterSubjectDigest": subject.Digest()}
	if status == "completed" {
		record["chainClosed"] = true
		record["closure"] = map[string]any{"criticRoot": job, "round": 1, "subject": subject, "mechanism": "clean"}
	}
	writeJSONFixture(t, root, "artifacts/agents/jobs/"+job+".json", record)
	if status == "completed" {
		writeJSONFixture(t, root, "artifacts/agents/"+job+"/rounds/1/subject.json", subject)
		writeJSONFixture(t, root, "artifacts/agents/"+job+"/rounds/1/return.json", map[string]any{"jobId": job, "round": 1, "reviewedTree": subject.Tree})
	}
}

type readFactCall struct {
	method  string
	args    []string
	commits []branch.Commit
	subject branch.AttestationSubject
	entries []branch.Entry
	err     error
}

type readFactRepository struct {
	t                      *testing.T
	root, base, unit, plan string
	mu                     sync.Mutex
	want                   []readFactCall
	seen                   []readFactCall
}

func newReadFactRepository(t *testing.T, plan bool) *readFactRepository {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &readFactRepository{t: t, root: root, base: strings.Repeat("a", 40), unit: strings.Repeat("b", 40)}
	if plan {
		r.plan = strings.Repeat("c", 40)
	}
	t.Cleanup(func() { r.assertConsumed() })
	return r
}

func (r *readFactRepository) readSubject() readsubject.ReadSubject {
	return readsubject.ReadSubject{Kind: readsubject.SubjectCommit, Commit: r.unit, Parent: r.base,
		Tree: strings.Repeat("d", 40), DiffDigest: strings.Repeat("e", 64)}
}

func (r *readFactRepository) attestationSubject() branch.AttestationSubject {
	s := r.readSubject()
	return branch.AttestationSubject{Commit: s.Commit, Parent: s.Parent, Tree: s.Tree,
		PatchDigest: s.DiffDigest, UnitDigest: strings.Repeat("f", 64)}
}

func (r *readFactRepository) rangeFacts() []branch.Commit {
	commits := []branch.Commit{}
	if r.plan != "" {
		commits = append(commits, branch.Commit{ID: r.plan, Kind: branch.Plan})
	}
	return append(commits, branch.Commit{ID: r.unit, Kind: branch.Unit, Unit: "u1", Units: []string{"u1"}})
}

func (r *readFactRepository) expect(calls ...readFactCall) { r.want = append(r.want, calls...) }

func (r *readFactRepository) expectStart() {
	r.expect(readFactCall{method: "Range", args: []string{r.root, r.base, r.unit, "goal-a"}, commits: r.rangeFacts()},
		readFactCall{method: "Subject", args: []string{r.root, r.unit}, subject: r.attestationSubject()},
		readFactCall{method: "CommonDir", args: []string{r.root}})
}

func (r *readFactRepository) expectGateAndBrief() {
	r.expect(readFactCall{method: "Detached", args: []string{r.root, r.unit}},
		readFactCall{method: "Range", args: []string{r.root, r.base, r.unit, "goal-a"}, commits: r.rangeFacts()})
	if r.plan != "" {
		r.expect(readFactCall{method: "Entries", args: []string{r.root, r.plan}, entries: []branch.Entry{{Path: "metasystem/plans/accepted-design.md"}}})
	}
}

func (r *readFactRepository) next(method string, args ...string) readFactCall {
	r.mu.Lock()
	if len(r.want) == 0 {
		r.mu.Unlock()
		r.t.Fatalf("unexpected repository %s(%q) for %s", method, args, r.root)
	}
	call := r.want[0]
	if call.method != method || !reflect.DeepEqual(call.args, args) {
		r.mu.Unlock()
		r.t.Fatalf("repository call %s(%q), want %s(%q)", method, args, call.method, call.args)
	}
	r.want = r.want[1:]
	r.seen = append(r.seen, call)
	r.mu.Unlock()
	return call
}

func (r *readFactRepository) assertConsumed() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.want) != 0 {
		r.t.Errorf("repository %s has %d unconsumed calls: %+v", r.root, len(r.want), r.want)
	}
}

func (r *readFactRepository) Range(repo, endpoint, tip, goal string) ([]branch.Commit, error) {
	c := r.next("Range", repo, endpoint, tip, goal)
	return c.commits, c.err
}
func (r *readFactRepository) Subject(repo, commit string) (branch.AttestationSubject, error) {
	c := r.next("Subject", repo, commit)
	return c.subject, c.err
}
func (r *readFactRepository) CommonDir(repo string) (string, error) {
	c := r.next("CommonDir", repo)
	return filepath.Join(r.root, ".git"), c.err
}
func (r *readFactRepository) Entries(repo, commit string) ([]branch.Entry, error) {
	c := r.next("Entries", repo, commit)
	return c.entries, c.err
}
func (r *readFactRepository) Detached(repo, commit string) (string, func() error, error) {
	c := r.next("Detached", repo, commit)
	if c.err != nil {
		return "", nil, c.err
	}
	dir, err := os.MkdirTemp(r.root, "detached-")
	if err != nil {
		return "", nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "code.go"), []byte("declared unit tree\n"), 0o644); err != nil {
		return "", nil, err
	}
	return dir, func() error { return os.RemoveAll(dir) }, nil
}

func TestGLEBranchReadFreezesSuppliedBriefAndRejectsConflictingRetry(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, false)
	unit := r.unit
	r.expectStart()
	r.expectGateAndBrief()
	r.expectStart()
	r.expectStart()
	r.expectStart()
	input := filepath.Join(t.TempDir(), "accepted-design.md")
	original := []byte("Accepted design: exact wildcard ownership and input identity.\n")
	if err := os.WriteFile(input, original, 0o644); err != nil {
		t.Fatal(err)
	}
	var frozenPath, frozenBody string
	delegates := 0
	request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: unit,
		GoalID: "goal-a", UnitCommit: unit, Repository: r, BriefPath: input, Runtime: "codex", Model: "gpt-5.6-sol",
		CheckClaim: claimAllowed, Gate: func(string) (string, error) { return "green", nil },
		NewID: func(string) (string, error) { return "frozen-brief-gate", nil },
		Delegate: func(brief, goalID, commit, runtime, model string) (string, error) {
			delegates++
			body, err := os.ReadFile(brief)
			if err != nil {
				t.Fatal(err)
			}
			frozenPath, frozenBody = brief, string(body)
			if brief == input || goalID != "goal-a" || commit != unit || runtime != "codex" || model != "gpt-5.6-sol" ||
				!strings.Contains(frozenBody, string(original)) || strings.Contains(frozenBody, "goals-live-on-branches-design.md") {
				t.Fatalf("dispatch brief=%q body=%q goal=%q commit=%q runtime=%q model=%q", brief, body, goalID, commit, runtime, model)
			}
			writeReadJobWithSubject(t, r.root, "critic-frozen", unit, "running", false, r.readSubject())
			return "critic-frozen", nil
		},
	}
	result, err := branch.RunBranchRead(request)
	if err != nil || result.State != "dispatched" || delegates != 1 {
		t.Fatalf("dispatch=%+v delegates=%d err=%v", result, delegates, err)
	}
	if err := os.WriteFile(input, []byte("changed after dispatch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(frozenPath); err != nil || string(body) != frozenBody {
		t.Fatalf("frozen brief changed: %q err=%v", body, err)
	}
	if _, err := branch.RunBranchRead(request); err == nil || !strings.Contains(err.Error(), "different brief") {
		t.Fatalf("changed brief retry=%v", err)
	}
	request.BriefPath = ""
	request.Runtime = "claude"
	if _, err := branch.RunBranchRead(request); err == nil || !strings.Contains(err.Error(), "runtime/model") {
		t.Fatalf("changed runtime retry=%v", err)
	}
	request.Runtime, request.Model = "", ""
	result, err = branch.RunBranchRead(request)
	if err != nil || result.State != "open" || delegates != 1 {
		t.Fatalf("flagless resume=%+v delegates=%d err=%v", result, delegates, err)
	}
}

func TestGLEBranchReadMissingBriefRefusesBeforeGateAndDefaultNamesPlanFold(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, true)
	plan, unit := r.plan, r.unit
	r.expectStart()
	r.expectStart()
	r.expectGateAndBrief()
	gateCalls, delegates := 0, 0
	request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: unit,
		GoalID: "goal-a", UnitCommit: unit, Repository: r, BriefPath: filepath.Join(t.TempDir(), "missing.md"),
		CheckClaim: claimAllowed, Gate: func(string) (string, error) { gateCalls++; return "green", nil },
		Delegate: func(brief, goalID, commit, runtime, model string) (string, error) {
			delegates++
			body, err := os.ReadFile(brief)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), plan) || !strings.Contains(string(body), "metasystem/plans/accepted-design.md") ||
				strings.Contains(string(body), "goals-live-on-branches-design.md") {
				t.Fatalf("generic default brief=%q", body)
			}
			writeReadJobWithSubject(t, r.root, "critic-plan", unit, "running", false, r.readSubject())
			return "critic-plan", nil
		},
	}
	if _, err := branch.RunBranchRead(request); err == nil || !strings.Contains(err.Error(), "read goal branch brief") || gateCalls != 0 || delegates != 0 {
		t.Fatalf("missing brief err=%v gate=%d delegates=%d", err, gateCalls, delegates)
	}
	request.BriefPath = ""
	if result, err := branch.RunBranchRead(request); err != nil || result.State != "dispatched" || gateCalls != 1 || delegates != 1 {
		t.Fatalf("default dispatch=%+v gate=%d delegates=%d err=%v", result, gateCalls, delegates, err)
	}
}

func gleBranchReadRecordPath(t *testing.T, root, unit string) string {
	t.Helper()
	return filepath.Join(root, ".git", "metasystem", "goal-reads", "goal-a", unit+".json")
}

func TestGLEBranchReadInterruptedLaunchKeepsFrozenPendingIntent(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, false)
	unit := r.unit
	r.expectStart()
	r.expectGateAndBrief()
	r.expectStart()
	r.expectStart()
	input := filepath.Join(t.TempDir(), "accepted.md")
	if err := os.WriteFile(input, []byte("accepted design A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	recordPath := gleBranchReadRecordPath(t, r.root, unit)
	delegates := 0
	request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: unit,
		GoalID: "goal-a", UnitCommit: unit, Repository: r, BriefPath: input, Runtime: "codex", Model: "gpt-5.6-sol",
		CheckClaim: claimAllowed, Gate: func(string) (string, error) { return "green", nil },
		Delegate: func(brief, goalID, commit, runtime, model string) (string, error) {
			delegates++
			data, err := os.ReadFile(recordPath)
			if err != nil || !strings.Contains(string(data), `"dispatchPending": true`) ||
				!strings.Contains(string(data), `"runtime": "codex"`) || !strings.Contains(string(data), `"model": "gpt-5.6-sol"`) {
				t.Fatalf("dispatch intent before launch=%q err=%v", data, err)
			}
			panic("process interrupted after critic launch")
		},
	}
	func() {
		defer func() {
			if got := recover(); got != "process interrupted after critic launch" {
				t.Fatalf("interruption=%v", got)
			}
		}()
		_, _ = branch.RunBranchRead(request)
	}()
	if err := os.WriteFile(input, []byte("accepted design B\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := branch.RunBranchRead(request); err == nil || !strings.Contains(err.Error(), "different brief") {
		t.Fatalf("changed request after interruption=%v", err)
	}
	request.BriefPath, request.Runtime, request.Model = "", "", ""
	if _, err := branch.RunBranchRead(request); err == nil || !strings.Contains(err.Error(), branch.ReadDispatchPendingCode) || delegates != 1 {
		t.Fatalf("omitted options after interruption=%v delegates=%d", err, delegates)
	}
}

func TestGLEBranchReadRecordWriteFailureNeverDispatchesAgain(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, false)
	unit := r.unit
	r.expectStart()
	r.expectGateAndBrief()
	r.expectStart()
	recordPath := gleBranchReadRecordPath(t, r.root, unit)
	delegates := 0
	request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: unit,
		GoalID: "goal-a", UnitCommit: unit, Repository: r, CheckClaim: claimAllowed,
		Gate: func(string) (string, error) { return "green", nil },
		Delegate: func(brief, goalID, commit, runtime, model string) (string, error) {
			delegates++
			if err := os.Chmod(filepath.Dir(recordPath), 0o500); err != nil {
				t.Fatal(err)
			}
			return "critic-write-failure", nil
		},
	}
	defer os.Chmod(filepath.Dir(recordPath), 0o755)
	if _, err := branch.RunBranchRead(request); err == nil || !strings.Contains(err.Error(), branch.ReadDispatchPendingCode) || delegates != 1 {
		t.Fatalf("post-launch record failure=%v delegates=%d", err, delegates)
	}
	if err := os.Chmod(filepath.Dir(recordPath), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(recordPath)
	if err != nil || !strings.Contains(string(data), `"dispatchPending": true`) || strings.Contains(string(data), `"rootJob"`) {
		t.Fatalf("retained pending intent=%q err=%v", data, err)
	}
	if _, err := branch.RunBranchRead(request); err == nil || !strings.Contains(err.Error(), branch.ReadDispatchPendingCode) || delegates != 1 {
		t.Fatalf("retry after record failure=%v delegates=%d", err, delegates)
	}
}

func TestGLEBranchReadPrelaunchRefusalRetriesFrozenSelectionOnce(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, false)
	unit := r.unit
	r.expectStart()
	r.expectGateAndBrief()
	r.expectStart()
	r.expectStart()
	input := filepath.Join(t.TempDir(), "accepted.md")
	original := []byte("accepted design before refusal\n")
	if err := os.WriteFile(input, original, 0o644); err != nil {
		t.Fatal(err)
	}
	delegates, launches := 0, 0
	var firstBrief string
	request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: unit,
		GoalID: "goal-a", UnitCommit: unit, Repository: r, BriefPath: input, Runtime: "codex", Model: "gpt-5.6-sol",
		CheckClaim: claimAllowed, Gate: func(string) (string, error) { return "green", nil },
		Delegate: func(brief, goalID, commit, runtime, model string) (string, error) {
			delegates++
			body, err := os.ReadFile(brief)
			if err != nil || runtime != "codex" || model != "gpt-5.6-sol" || !strings.Contains(string(body), string(original)) {
				t.Fatalf("retry delegate brief=%q runtime=%q model=%q err=%v", body, runtime, model, err)
			}
			if delegates == 1 {
				firstBrief = string(body)
				return "", &branch.ReadNeverLaunchedError{Err: errors.New("REFUSED-ROSTER before claim")}
			}
			if string(body) != firstBrief {
				t.Fatalf("retry changed frozen brief: first=%q second=%q", firstBrief, body)
			}
			launches++
			writeReadJobWithSubject(t, r.root, "critic-retry", unit, "running", false, r.readSubject())
			return "critic-retry", nil
		},
	}
	if _, err := branch.RunBranchRead(request); err == nil || !strings.Contains(err.Error(), "REFUSED-ROSTER") || delegates != 1 || launches != 0 {
		t.Fatalf("prelaunch refusal=%v delegates=%d launches=%d", err, delegates, launches)
	}
	record, err := os.ReadFile(gleBranchReadRecordPath(t, r.root, unit))
	if err != nil || !strings.Contains(string(record), `"dispatchRetryable": true`) || strings.Contains(string(record), `"dispatchPending": true`) {
		t.Fatalf("retryable record=%q err=%v", record, err)
	}
	if err := os.WriteFile(input, []byte("changed source after refusal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := branch.RunBranchRead(request); err == nil || !strings.Contains(err.Error(), "different brief") || delegates != 1 {
		t.Fatalf("conflicting retry=%v delegates=%d", err, delegates)
	}
	request.BriefPath, request.Runtime, request.Model = "", "", ""
	result, err := branch.RunBranchRead(request)
	if err != nil || result.State != "dispatched" || delegates != 2 || launches != 1 {
		t.Fatalf("frozen retry=%+v err=%v delegates=%d launches=%d", result, err, delegates, launches)
	}
}

func TestGLEBranchReadRepositoryFactFailurePrecedesSideEffects(t *testing.T) {
	t.Parallel()
	for _, failed := range []string{"Range", "Subject", "CommonDir"} {
		t.Run(failed, func(t *testing.T) {
			r := newReadFactRepository(t, false)
			factErr := errors.New("declared " + failed + " failure")
			calls := []readFactCall{
				{method: "Range", args: []string{r.root, r.base, r.unit, "goal-a"}, commits: r.rangeFacts()},
				{method: "Subject", args: []string{r.root, r.unit}, subject: r.attestationSubject()},
				{method: "CommonDir", args: []string{r.root}},
			}
			for i := range calls {
				if calls[i].method == failed {
					calls[i].err = factErr
					r.expect(calls[:i+1]...)
					break
				}
			}
			gate, ids, delegates := 0, 0, 0
			_, err := branch.RunBranchRead(branch.BranchReadRequest{Repo: r.root, Repository: r,
				EndpointTip: r.base, BranchTip: r.unit, GoalID: "goal-a", UnitCommit: r.unit,
				CheckClaim: claimAllowed,
				Gate:       func(string) (string, error) { gate++; return "green", nil },
				NewID:      func(string) (string, error) { ids++; return "id", nil },
				Delegate:   func(string, string, string, string, string) (string, error) { delegates++; return "job", nil },
			})
			if !errors.Is(err, factErr) || gate != 0 || ids != 0 || delegates != 0 {
				t.Fatalf("fact failure=%v gate=%d ids=%d delegates=%d", err, gate, ids, delegates)
			}
			if _, err := os.Stat(gleBranchReadRecordPath(t, r.root, r.unit)); !os.IsNotExist(err) {
				t.Fatalf("fact failure wrote journal: %v", err)
			}
		})
	}
}

func TestGLEBranchReadRedGateClosesDetachedWorkspace(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, false)
	r.expectStart()
	r.expect(readFactCall{method: "Detached", args: []string{r.root, r.unit}})
	var detached string
	ids, delegates := 0, 0
	_, err := branch.RunBranchRead(branch.BranchReadRequest{Repo: r.root, Repository: r,
		EndpointTip: r.base, BranchTip: r.unit, GoalID: "goal-a", UnitCommit: r.unit,
		CheckClaim: claimAllowed,
		Gate: func(dir string) (string, error) {
			detached = dir
			if data, err := os.ReadFile(filepath.Join(dir, "code.go")); err != nil || string(data) != "declared unit tree\n" {
				t.Fatalf("gate workspace=%q err=%v", data, err)
			}
			return "go gate: staticcheck red", errors.New("exit 1")
		},
		NewID:    func(string) (string, error) { ids++; return "id", nil },
		Delegate: func(string, string, string, string, string) (string, error) { delegates++; return "job", nil },
	})
	if err == nil || !strings.Contains(err.Error(), branch.ReadUngatedCode) ||
		!strings.Contains(err.Error(), "staticcheck red") || detached == "" || ids != 0 || delegates != 0 {
		t.Fatalf("red gate err=%v detached=%q ids=%d delegates=%d", err, detached, ids, delegates)
	}
	if _, err := os.Stat(detached); !os.IsNotExist(err) {
		t.Fatalf("detached workspace remains: %v", err)
	}
	if _, err := os.Stat(gleBranchReadRecordPath(t, r.root, r.unit)); !os.IsNotExist(err) {
		t.Fatalf("red gate wrote journal: %v", err)
	}
}

func TestGLEBranchReadConcurrentRepositoriesKeepRootsIsolated(t *testing.T) {
	t.Parallel()
	left, right := newReadFactRepository(t, false), newReadFactRepository(t, true)
	if left.root == right.root {
		t.Fatal("concurrent repositories share a root")
	}
	for _, r := range []*readFactRepository{left, right} {
		r.expectStart()
		r.expectGateAndBrief()
	}
	var wg sync.WaitGroup
	results := make([]branch.BranchReadResult, 2)
	errs := make([]error, 2)
	for i, r := range []*readFactRepository{left, right} {
		wg.Add(1)
		go func(i int, r *readFactRepository) {
			defer wg.Done()
			results[i], errs[i] = branch.RunBranchRead(branch.BranchReadRequest{Repo: r.root, Repository: r,
				EndpointTip: r.base, BranchTip: r.unit, GoalID: "goal-a", UnitCommit: r.unit,
				CheckClaim: claimAllowed, Gate: func(dir string) (string, error) {
					if !strings.HasPrefix(dir, r.root+string(os.PathSeparator)) {
						t.Errorf("detached root %q for %q", dir, r.root)
					}
					return "green", nil
				}, NewID: func(string) (string, error) { return "gate-" + filepath.Base(r.root), nil },
				Delegate: func(brief, _, _, _, _ string) (string, error) {
					if !strings.HasPrefix(brief, filepath.Join(r.root, ".git")+string(os.PathSeparator)) {
						t.Errorf("brief root %q for %q", brief, r.root)
					}
					return "critic-" + filepath.Base(r.root), nil
				},
			})
		}(i, r)
	}
	wg.Wait()
	for i, r := range []*readFactRepository{left, right} {
		if errs[i] != nil || results[i].State != "dispatched" || results[i].RootJob != "critic-"+filepath.Base(r.root) {
			t.Fatalf("read %d result=%+v err=%v", i, results[i], errs[i])
		}
		if _, err := os.Stat(gleBranchReadRecordPath(t, r.root, r.unit)); err != nil {
			t.Fatalf("read %d journal: %v", i, err)
		}
		wantCalls := 5
		if r.plan != "" {
			wantCalls++
		}
		if len(r.seen) != wantCalls {
			t.Fatalf("read %d recorded %d repository calls, want %d", i, len(r.seen), wantCalls)
		}
		for _, call := range r.seen {
			if call.args[0] != r.root {
				t.Fatalf("read %d crossed repository roots: %+v", i, call)
			}
		}
	}
}

func TestGLEBranchReadFrozenBriefCarriesOneDispatchableWorkingMode(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, supplied, mode string
	}{
		{"headerless prose", "Build the requested outcome and watch it work.\n", "implement"},
		{"explicit mode", "Working Mode: review\n\nRead the unit against its accepted design.\n", "review"},
		{"repeated header", "Working Mode: review\nWorking Mode: design\n", ""},
		{"empty header", "Working Mode:\n\nPlain prose.\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newReadFactRepository(t, false)
			r.expectStart()
			r.expectGateAndBrief()
			input := filepath.Join(t.TempDir(), "brief.md")
			if err := os.WriteFile(input, []byte(tc.supplied), 0o644); err != nil {
				t.Fatal(err)
			}
			delegates := 0
			request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: r.unit,
				GoalID: "goal-a", UnitCommit: r.unit, Repository: r, BriefPath: input,
				CheckClaim: claimAllowed, Gate: func(string) (string, error) { return "green", nil },
				Delegate: func(brief, goalID, commit, runtime, model string) (string, error) {
					delegates++
					mode, err := dispatch.BriefModeOnly(brief)
					body, _ := os.ReadFile(brief)
					if err != nil || mode != tc.mode || !strings.Contains(string(body), tc.supplied) {
						t.Fatalf("dispatch mode=%q err=%v body=%q", mode, err, body)
					}
					writeReadJobWithSubject(t, r.root, "critic-mode", r.unit, "running", false, r.readSubject())
					return "critic-mode", nil
				},
			}
			result, err := branch.RunBranchRead(request)
			if tc.mode == "" {
				if err == nil || !strings.Contains(err.Error(), "exactly one filled Working Mode header") || delegates != 0 {
					t.Fatalf("malformed mode result=%+v delegates=%d err=%v", result, delegates, err)
				}
				return
			}
			if err != nil || result.State != "dispatched" || delegates != 1 {
				t.Fatalf("dispatch=%+v delegates=%d err=%v", result, delegates, err)
			}
		})
	}
}
