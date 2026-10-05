package branch_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
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
	if _, err := branch.RunBranchRead(request); goal.RefusalCode(err) != branch.ReadBriefChangedCode {
		t.Fatalf("changed brief retry=%v", err)
	}
	request.Join = true
	if joined, err := branch.RunBranchRead(request); err != nil || joined.State != "open" || delegates != 1 {
		t.Fatalf("a request joining the started read under another brief=%+v delegates=%d err=%v", joined, delegates, err)
	}
	request.Join = false
	request.BriefPath = ""
	request.Runtime = "claude"
	if _, err := branch.RunBranchRead(request); goal.RefusalCode(err) != branch.ReadBriefChangedCode {
		t.Fatalf("changed runtime retry=%v", err)
	}
	request.Runtime, request.Model = "", ""
	result, err = branch.RunBranchRead(request)
	if err != nil || result.State != "open" || delegates != 1 {
		t.Fatalf("flagless resume=%+v delegates=%d err=%v", result, delegates, err)
	}
}

func TestFollowUpReadIsToldThePreviousDecisions(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, false)
	r.expectStart()
	r.expectGateAndBrief()
	input := filepath.Join(t.TempDir(), "follow-up.md")
	packet := "## Follow-up read of round 1\nCheck every fold first, citing the line that proves it\n1. Result is lost\n## Decisions on round 1\n| 1 | fixed | file.go:12 |\nDiff since round 1's tree\n+fixed line\nProof result of round 2: passed\n"
	if err := os.WriteFile(input, []byte("Build the fold.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(input), "read-context.md"), []byte(packet), 0o600); err != nil {
		t.Fatal(err)
	}
	request := branch.BranchReadRequest{Repo: r.root, EndpointTip: r.base, BranchTip: r.unit, GoalID: "goal-a", UnitCommit: r.unit,
		Repository: r, BriefPath: input, Join: true, CheckClaim: claimAllowed, Gate: func(string) (string, error) { return "green", nil },
		NewID: func(string) (string, error) { return "warm-gate", nil },
		Delegate: func(brief, _, _, _, _ string) (string, error) {
			data, err := os.ReadFile(brief)
			supplied := strings.Index(string(data), "# Supplied accepted")
			if supplied < 0 {
				supplied = strings.Index(string(data), "# Corrected implementation brief")
			}
			if err != nil || !strings.Contains(string(data), packet) || supplied < 0 || strings.Index(string(data), packet) > supplied {
				t.Fatalf("header lost packet: %s %v", data, err)
			}
			return "warm-critic", nil
		},
	}
	if result, err := branch.RunBranchRead(request); err != nil || result.State != "dispatched" {
		t.Fatalf("read: %+v %v", result, err)
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

func gleBranchReadRecord(t *testing.T, root, unit string) string {
	t.Helper()
	data, err := os.ReadFile(gleBranchReadRecordPath(t, root, unit))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// gleBranchReadCriticID is the job id the delegate boundary derives for a
// fresh code-critic of goal-a at a goal revision that reads a brief with this
// content: the id of the critic a review request with that frozen brief starts.
func gleBranchReadCriticID(t *testing.T, brief []byte, revision uint64) string {
	t.Helper()
	sum := sha256.Sum256(brief)
	id, err := dispatch.DefaultOperationID("goal-a", revision, dispatch.DispatchModeFresh, "code-critic", hex.EncodeToString(sum[:]), "")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// gleBranchReadFrozenCriticID is the id of the critic a review request starts
// at goal revision 1, derived from the frozen brief file the delegate is given.
func gleBranchReadFrozenCriticID(t *testing.T, briefPath string) string {
	t.Helper()
	brief, err := os.ReadFile(briefPath)
	if err != nil {
		t.Fatal(err)
	}
	return gleBranchReadCriticID(t, brief, 1)
}

// writeReadCriticRecord writes a critic job record as the delegate boundary
// leaves it before any process exists. Round 0 is a reservation whose setup
// never completed, which carries no round; a positive round is launchable.
// Revision 0 is a record that names no goal revision.
func writeReadCriticRecord(t *testing.T, root, job, goalID, commit, parent, createdAt string, round int, revision uint64) {
	t.Helper()
	record := map[string]any{"jobId": job, "role": "code-critic", "status": "pending-setup",
		"reviews": "commit:" + commit, "goalId": goalID, "createdAt": createdAt}
	if round > 0 {
		record["status"], record["round"] = "pending", round
	}
	if revision > 0 {
		record["goalRevision"] = revision
	}
	if parent != "" {
		record["parentJob"] = parent
	}
	writeJSONFixture(t, root, "artifacts/agents/jobs/"+job+".json", record)
}

func TestGLEBranchReadInterruptedLaunchWithoutJobRecordStartsTheChangedRequest(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, false)
	unit := r.unit
	r.expectStart()
	r.expectGateAndBrief()
	r.expectStart()
	r.expect(readFactCall{method: "Range", args: []string{r.root, r.base, r.unit, "goal-a"}, commits: r.rangeFacts()})
	r.expectStart()
	input := filepath.Join(t.TempDir(), "accepted.md")
	if err := os.WriteFile(input, []byte("accepted design A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	delegates := 0
	var firstBrief string
	request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: unit,
		GoalID: "goal-a", UnitCommit: unit, Repository: r, BriefPath: input, Runtime: "codex", Model: "gpt-5.6-sol",
		CheckClaim: claimAllowed, Gate: func(string) (string, error) { return "green", nil },
		Delegate: func(brief, goalID, commit, runtime, model string) (string, error) {
			delegates++
			body, err := os.ReadFile(brief)
			data := gleBranchReadRecord(t, r.root, unit)
			if err != nil || runtime != "codex" || model != "gpt-5.6-sol" || !strings.Contains(data, `"dispatchPending": true`) ||
				!strings.Contains(data, `"runtime": "codex"`) || !strings.Contains(data, `"model": "gpt-5.6-sol"`) {
				t.Fatalf("dispatch intent before launch=%q runtime=%q model=%q err=%v", data, runtime, model, err)
			}
			if delegates == 1 {
				firstBrief = string(body)
				panic("process interrupted before any job record")
			}
			if !strings.Contains(string(body), "accepted design B") || firstBrief == string(body) {
				t.Fatalf("the second dispatch is not the changed request: first=%q second=%q", firstBrief, body)
			}
			writeReadJobWithSubject(t, r.root, "critic-again", unit, "running", false, r.readSubject())
			return "critic-again", nil
		},
	}
	func() {
		defer func() {
			if got := recover(); got != "process interrupted before any job record" {
				t.Fatalf("interruption=%v", got)
			}
		}()
		_, _ = branch.RunBranchRead(request)
	}()
	// The job records say no critic was reserved, so the interrupted start
	// bound nothing: a changed request starts the review with its brief.
	if err := os.WriteFile(input, []byte("accepted design B\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := branch.RunBranchRead(request)
	if err != nil || result.State != "dispatched" || result.RootJob != "critic-again" || delegates != 2 {
		t.Fatalf("changed request after interruption=%+v err=%v delegates=%d", result, err, delegates)
	}
	request.BriefPath, request.Runtime, request.Model = "", "", ""
	if result, err = branch.RunBranchRead(request); err != nil || result.State != "open" || delegates != 2 {
		t.Fatalf("omitted options after the dispatch=%+v err=%v delegates=%d", result, err, delegates)
	}
}

func TestGLEBranchReadInterruptedLaunchWithJobRecordAdoptsItsCritic(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, false)
	unit := r.unit
	r.expectStart()
	r.expectGateAndBrief()
	r.expectStart()
	delegates := 0
	var critic string
	request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: unit,
		GoalID: "goal-a", UnitCommit: unit, Repository: r, CheckClaim: claimAllowed,
		Gate: func(string) (string, error) { return "green", nil },
		Delegate: func(brief, goalID, commit, runtime, model string) (string, error) {
			delegates++
			critic = gleBranchReadFrozenCriticID(t, brief)
			writeReadJobWithSubject(t, r.root, critic, unit, "running", false, r.readSubject())
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
	result, err := branch.RunBranchRead(request)
	if err != nil || result.State != "open" || critic == "" || result.RootJob != critic || delegates != 1 {
		t.Fatalf("run after interruption=%+v err=%v delegates=%d", result, err, delegates)
	}
	if data := gleBranchReadRecord(t, r.root, unit); !strings.Contains(data, `"rootJob": "`+critic+`"`) || strings.Contains(data, `"dispatchPending"`) {
		t.Fatalf("adopted record=%q", data)
	}
}

func TestGLEBranchReadFailedDispatchWithoutJobRecordMayBeRequestedAgain(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, false)
	unit := r.unit
	r.expectStart()
	r.expectGateAndBrief()
	r.expectStart()
	r.expect(readFactCall{method: "Range", args: []string{r.root, r.base, r.unit, "goal-a"}, commits: r.rangeFacts()})
	input := filepath.Join(t.TempDir(), "accepted.md")
	if err := os.WriteFile(input, []byte("accepted design before the failed dispatch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	delegates := 0
	var firstBrief string
	request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: unit,
		GoalID: "goal-a", UnitCommit: unit, Repository: r, BriefPath: input, Runtime: "codex", Model: "gpt-5.6-sol",
		CheckClaim: claimAllowed, Gate: func(string) (string, error) { return "green", nil },
		Delegate: func(brief, goalID, commit, runtime, model string) (string, error) {
			delegates++
			body, err := os.ReadFile(brief)
			if err != nil || runtime != "codex" || model != "gpt-5.6-sol" {
				t.Fatalf("delegate brief=%q runtime=%q model=%q err=%v", body, runtime, model, err)
			}
			if delegates == 1 {
				firstBrief = string(body)
				return "", errors.New("delegate outcome REFUSED-LEASE: no job was reserved")
			}
			if string(body) != firstBrief {
				t.Fatalf("second dispatch changed the frozen brief: first=%q second=%q", firstBrief, body)
			}
			writeReadJobWithSubject(t, r.root, "critic-second", unit, "running", false, r.readSubject())
			return "critic-second", nil
		},
	}
	var never *branch.ReadNeverLaunchedError
	if _, err := branch.RunBranchRead(request); !errors.As(err, &never) || !strings.Contains(err.Error(), "REFUSED-LEASE") || delegates != 1 {
		t.Fatalf("failed dispatch without a job record=%v delegates=%d", err, delegates)
	}
	if data := gleBranchReadRecord(t, r.root, unit); !strings.Contains(data, `"dispatchRetryable": true`) || strings.Contains(data, `"dispatchPending"`) {
		t.Fatalf("retryable record=%q", data)
	}
	result, err := branch.RunBranchRead(request)
	if err != nil || result.State != "dispatched" || result.RootJob != "critic-second" || delegates != 2 {
		t.Fatalf("second request=%+v err=%v delegates=%d", result, err, delegates)
	}
}

func TestGLEBranchReadFailedDispatchWithJobRecordAdoptsItsCritic(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, false)
	unit := r.unit
	r.expectStart()
	r.expectGateAndBrief()
	r.expectStart()
	delegates := 0
	var critic string
	request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: unit,
		GoalID: "goal-a", UnitCommit: unit, Repository: r, CheckClaim: claimAllowed,
		Gate: func(string) (string, error) { return "green", nil },
		Delegate: func(brief, goalID, commit, runtime, model string) (string, error) {
			delegates++
			critic = gleBranchReadFrozenCriticID(t, brief)
			writeReadJobWithSubject(t, r.root, critic, unit, "failed", false, r.readSubject())
			return "", errors.New("delegate outcome HANDSHAKE-FAILED: session did not establish")
		},
	}
	var never *branch.ReadNeverLaunchedError
	if _, err := branch.RunBranchRead(request); err == nil || errors.As(err, &never) || !strings.Contains(err.Error(), "HANDSHAKE-FAILED") {
		t.Fatalf("failed dispatch with a job record=%v", err)
	}
	if data := gleBranchReadRecord(t, r.root, unit); critic == "" || !strings.Contains(data, `"rootJob": "`+critic+`"`) || strings.Contains(data, `"dispatchPending"`) {
		t.Fatalf("adopted record=%q", data)
	}
	result, err := branch.RunBranchRead(request)
	if err != nil || result.State != "closed" || result.RootJob != critic || delegates != 1 {
		t.Fatalf("run after failed dispatch=%+v err=%v delegates=%d", result, err, delegates)
	}
}

func TestGLEBranchReadAnotherCallersCriticOfTheSameCommitIsNeverAdopted(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		// interrupted is a dispatch whose process died; otherwise it failed.
		interrupted bool
	}{
		{"this review's dispatch failed", false},
		{"this review's dispatch was interrupted", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newReadFactRepository(t, false)
			unit := r.unit
			r.expectStart()
			r.expectGateAndBrief()
			r.expectStart()
			// Another caller's examination of the same commit for the same
			// goal: a launchable root named after the brief that caller read.
			other := gleBranchReadCriticID(t, []byte("another caller's brief\n"), 1)
			writeReadJobWithSubject(t, r.root, other, unit, "running", false, r.readSubject())
			delegates := 0
			var firstBrief, critic string
			request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: unit,
				GoalID: "goal-a", UnitCommit: unit, Repository: r, CheckClaim: claimAllowed,
				Gate: func(string) (string, error) { return "green", nil },
				Delegate: func(brief, goalID, commit, runtime, model string) (string, error) {
					delegates++
					body, err := os.ReadFile(brief)
					if err != nil {
						t.Fatal(err)
					}
					if delegates == 1 {
						firstBrief = string(body)
						if tc.interrupted {
							panic("process interrupted before any job record")
						}
						return "", errors.New("delegate outcome REFUSED-LEASE: no job was reserved")
					}
					if string(body) != firstBrief {
						t.Fatalf("second dispatch changed the frozen brief: first=%q second=%q", firstBrief, body)
					}
					critic = gleBranchReadCriticID(t, body, 1)
					writeReadJobWithSubject(t, r.root, critic, unit, "running", false, r.readSubject())
					return critic, nil
				},
			}
			if tc.interrupted {
				func() {
					defer func() {
						if got := recover(); got != "process interrupted before any job record" {
							t.Fatalf("interruption=%v", got)
						}
					}()
					_, _ = branch.RunBranchRead(request)
				}()
				if data := gleBranchReadRecord(t, r.root, unit); !strings.Contains(data, `"dispatchPending": true`) || strings.Contains(data, `"rootJob"`) {
					t.Fatalf("interrupted record=%q", data)
				}
			} else {
				var never *branch.ReadNeverLaunchedError
				if _, err := branch.RunBranchRead(request); !errors.As(err, &never) || !strings.Contains(err.Error(), "REFUSED-LEASE") {
					t.Fatalf("failed dispatch beside another caller's critic=%v", err)
				}
				if data := gleBranchReadRecord(t, r.root, unit); !strings.Contains(data, `"dispatchRetryable": true`) || strings.Contains(data, `"dispatchPending"`) || strings.Contains(data, `"rootJob"`) {
					t.Fatalf("retryable record=%q", data)
				}
			}
			result, err := branch.RunBranchRead(request)
			if err != nil || result.State != "dispatched" || critic == other || result.RootJob != critic || delegates != 2 {
				t.Fatalf("next request=%+v err=%v delegates=%d other=%s", result, err, delegates, other)
			}
			if data := gleBranchReadRecord(t, r.root, unit); !strings.Contains(data, `"rootJob": "`+critic+`"`) || strings.Contains(data, other) {
				t.Fatalf("record after the second dispatch=%q", data)
			}
		})
	}
}

func TestGLEBranchReadAdoptsOnlyTheNewestLaunchableCriticRootOfItsCommitAndGoal(t *testing.T) {
	t.Parallel()
	other := strings.Repeat("9", 40)
	// Each record carries the id this request's critic has at the goal
	// revision the record names, so only the named difference keeps it from
	// being adopted. reserve returns the root the review adopts, or none.
	for _, tc := range []struct {
		name    string
		reserve func(t *testing.T, root, unit string, brief []byte) string
	}{
		{"another commit", func(t *testing.T, root, unit string, brief []byte) string {
			writeReadCriticRecord(t, root, gleBranchReadCriticID(t, brief, 1), "goal-a", other, "", "2026-01-02T03:04:05Z", 1, 1)
			return ""
		}},
		{"another goal", func(t *testing.T, root, unit string, brief []byte) string {
			writeReadCriticRecord(t, root, gleBranchReadCriticID(t, brief, 1), "goal-b", unit, "", "2026-01-02T03:04:05Z", 1, 1)
			return ""
		}},
		{"a later round of a chain", func(t *testing.T, root, unit string, brief []byte) string {
			writeReadCriticRecord(t, root, gleBranchReadCriticID(t, brief, 1), "goal-a", unit, "critic-root", "2026-01-02T03:04:05Z", 2, 1)
			return ""
		}},
		{"a reservation that ended in setup", func(t *testing.T, root, unit string, brief []byte) string {
			writeReadCriticRecord(t, root, gleBranchReadCriticID(t, brief, 1), "goal-a", unit, "", "2026-01-02T03:04:05Z", 0, 1)
			return ""
		}},
		{"an id derived from another brief", func(t *testing.T, root, unit string, brief []byte) string {
			writeReadCriticRecord(t, root, gleBranchReadCriticID(t, append([]byte("another brief\n"), brief...), 1), "goal-a", unit, "", "2026-01-02T03:04:05Z", 1, 1)
			return ""
		}},
		{"an id derived from another goal revision than the record names", func(t *testing.T, root, unit string, brief []byte) string {
			writeReadCriticRecord(t, root, gleBranchReadCriticID(t, brief, 1), "goal-a", unit, "", "2026-01-02T03:04:05Z", 1, 2)
			return ""
		}},
		{"a record that names no goal revision", func(t *testing.T, root, unit string, brief []byte) string {
			writeReadCriticRecord(t, root, gleBranchReadCriticID(t, brief, 1), "goal-a", unit, "", "2026-01-02T03:04:05Z", 1, 0)
			return ""
		}},
		{"a root reserved at a later goal revision", func(t *testing.T, root, unit string, brief []byte) string {
			critic := gleBranchReadCriticID(t, brief, 7)
			writeReadCriticRecord(t, root, critic, "goal-a", unit, "", "2026-01-02T03:04:05Z", 1, 7)
			return critic
		}},
		{"roots at three goal revisions and a newer reservation that ended in setup", func(t *testing.T, root, unit string, brief []byte) string {
			newest := gleBranchReadCriticID(t, brief, 2)
			writeReadCriticRecord(t, root, gleBranchReadCriticID(t, brief, 1), "goal-a", unit, "", "2026-01-02T03:04:05Z", 1, 1)
			writeReadCriticRecord(t, root, newest, "goal-a", unit, "", "2026-01-02T03:04:06Z", 1, 2)
			writeReadCriticRecord(t, root, gleBranchReadCriticID(t, brief, 3), "goal-a", unit, "", "2026-01-02T03:04:04Z", 1, 3)
			writeReadCriticRecord(t, root, gleBranchReadCriticID(t, brief, 4), "goal-a", unit, "", "2026-01-02T03:04:07Z", 0, 4)
			return newest
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newReadFactRepository(t, false)
			r.expectStart()
			r.expectGateAndBrief()
			var wantRoot string
			_, err := branch.RunBranchRead(branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: r.unit,
				GoalID: "goal-a", UnitCommit: r.unit, Repository: r, CheckClaim: claimAllowed,
				Gate: func(string) (string, error) { return "green", nil },
				Delegate: func(brief, goalID, commit, runtime, model string) (string, error) {
					body, err := os.ReadFile(brief)
					if err != nil {
						t.Fatal(err)
					}
					wantRoot = tc.reserve(t, r.root, r.unit, body)
					return "", errors.New("delegate failed")
				}})
			var never *branch.ReadNeverLaunchedError
			data := gleBranchReadRecord(t, r.root, r.unit)
			if wantRoot == "" {
				if !errors.As(err, &never) || !strings.Contains(data, `"dispatchRetryable": true`) || strings.Contains(data, `"rootJob"`) {
					t.Fatalf("unrelated job record: err=%v record=%q", err, data)
				}
				return
			}
			if err == nil || errors.As(err, &never) || !strings.Contains(data, `"rootJob": "`+wantRoot+`"`) {
				t.Fatalf("newest root: err=%v record=%q", err, data)
			}
		})
	}
}

func TestGLEBranchReadUnreadableJobRecordKeepsTheRequestPending(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"a torn record": "{",
		"this request's critic root under another file name": `{"jobId": "CRITIC", "role": "code-critic", "round": 1, "goalId": "goal-a", "goalRevision": 1, "reviews": "commit:UNIT"}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newReadFactRepository(t, false)
			unit := r.unit
			r.expectStart()
			r.expectGateAndBrief()
			r.expectStart()
			r.expectStart()
			torn := filepath.Join(r.root, "artifacts", "agents", "jobs", "torn.json")
			delegates := 0
			request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: unit,
				GoalID: "goal-a", UnitCommit: unit, Repository: r, CheckClaim: claimAllowed,
				Gate: func(string) (string, error) { return "green", nil },
				Delegate: func(brief, goalID, commit, runtime, model string) (string, error) {
					delegates++
					if delegates == 1 {
						write(t, r.root, "artifacts/agents/jobs/torn.json", strings.NewReplacer("UNIT", unit, "CRITIC", gleBranchReadFrozenCriticID(t, brief)).Replace(body))
						return "", errors.New("delegate failed")
					}
					return "critic-after-repair", nil
				},
			}
			var never *branch.ReadNeverLaunchedError
			for _, run := range []string{"failed dispatch", "next request"} {
				_, err := branch.RunBranchRead(request)
				if goal.RefusalCode(err) != branch.ReadDispatchPendingCode || errors.As(err, &never) || !strings.Contains(err.Error(), "torn.json") || delegates != 1 {
					t.Fatalf("%s with an unreadable job record=%v delegates=%d", run, err, delegates)
				}
				if data := gleBranchReadRecord(t, r.root, unit); !strings.Contains(data, `"dispatchPending": true`) || strings.Contains(data, `"rootJob"`) {
					t.Fatalf("%s left record=%q", run, data)
				}
			}
			if err := os.Remove(torn); err != nil {
				t.Fatal(err)
			}
			result, err := branch.RunBranchRead(request)
			if err != nil || result.State != "dispatched" || result.RootJob != "critic-after-repair" || delegates != 2 {
				t.Fatalf("request after repair=%+v err=%v delegates=%d", result, err, delegates)
			}
		})
	}
}

func TestGLEBranchReadRecordWriteFailureAdoptsTheStartedCritic(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, false)
	unit := r.unit
	r.expectStart()
	r.expectGateAndBrief()
	r.expectStart()
	recordPath := gleBranchReadRecordPath(t, r.root, unit)
	delegates := 0
	var critic string
	request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: unit,
		GoalID: "goal-a", UnitCommit: unit, Repository: r, CheckClaim: claimAllowed,
		Gate: func(string) (string, error) { return "green", nil },
		Delegate: func(brief, goalID, commit, runtime, model string) (string, error) {
			delegates++
			critic = gleBranchReadFrozenCriticID(t, brief)
			writeReadJobWithSubject(t, r.root, critic, unit, "running", false, r.readSubject())
			if err := os.Chmod(filepath.Dir(recordPath), 0o500); err != nil {
				t.Fatal(err)
			}
			return critic, nil
		},
	}
	defer os.Chmod(filepath.Dir(recordPath), 0o755)
	if _, err := branch.RunBranchRead(request); err == nil || !strings.Contains(goal.RecordText(err), branch.ReadDispatchPendingCode) || delegates != 1 {
		t.Fatalf("post-launch record failure=%v delegates=%d", err, delegates)
	}
	if err := os.Chmod(filepath.Dir(recordPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if data := gleBranchReadRecord(t, r.root, unit); !strings.Contains(data, `"dispatchPending": true`) || strings.Contains(data, `"rootJob"`) {
		t.Fatalf("retained pending intent=%q", data)
	}
	result, err := branch.RunBranchRead(request)
	if err != nil || result.State != "open" || critic == "" || result.RootJob != critic || delegates != 1 {
		t.Fatalf("run after record failure=%+v err=%v delegates=%d", result, err, delegates)
	}
	if data := gleBranchReadRecord(t, r.root, unit); !strings.Contains(data, `"rootJob": "`+critic+`"`) || strings.Contains(data, `"dispatchPending"`) {
		t.Fatalf("adopted record=%q", data)
	}
}

func TestGLEBranchReadPrelaunchRefusalRetriesFrozenSelectionOnce(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, false)
	unit := r.unit
	r.expectStart()
	r.expectGateAndBrief()
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
	request.BriefPath, request.Runtime, request.Model = "", "", ""
	result, err := branch.RunBranchRead(request)
	if err != nil || result.State != "dispatched" || delegates != 2 || launches != 1 {
		t.Fatalf("frozen retry=%+v err=%v delegates=%d launches=%d", result, err, delegates, launches)
	}
}

// TestBranchReadRefusedDispatchBindsNothing: a dispatch refused before any
// critic started binds nothing, so a request with a corrected brief starts
// the review with it (runtime and model as it names them); once a critic
// was dispatched, the review stays bound to that brief.
func TestReviewWithABriefNeverJoins(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, false)
	unit := r.unit
	r.expectStart()
	r.expectGateAndBrief()
	r.expectStart()
	r.expect(readFactCall{method: "Range", args: []string{r.root, r.base, r.unit, "goal-a"}, commits: r.rangeFacts()})
	r.expectStart()
	dir := t.TempDir()
	refused, corrected := filepath.Join(dir, "refused.md"), filepath.Join(dir, "corrected.md")
	if err := os.WriteFile(refused, []byte("cites a path the tree lacks\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corrected, []byte("cites only what the tree holds\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var dispatched []string
	request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: unit,
		GoalID: "goal-a", UnitCommit: unit, Repository: r, BriefPath: refused, Runtime: "codex",
		CheckClaim: claimAllowed, Gate: func(string) (string, error) { return "green", nil },
		Delegate: func(brief, _, _, runtime, model string) (string, error) {
			body, err := os.ReadFile(brief)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(body), "cites a path the tree lacks") {
				return "", &branch.ReadNeverLaunchedError{Err: errors.New("brief authority admission refused\nsecond line")}
			}
			dispatched = append(dispatched, runtime+"/"+model)
			writeReadJobWithSubject(t, r.root, "critic-corrected", unit, "running", false, r.readSubject())
			return "critic-corrected", nil
		},
	}
	if _, err := branch.RunBranchRead(request); err == nil || len(dispatched) != 0 {
		t.Fatalf("refused dispatch=%v dispatched=%v", err, dispatched)
	}
	record := gleBranchReadRecord(t, r.root, unit)
	if !strings.Contains(record, `"dispatchRefusal": "brief authority admission refused"`) || strings.Contains(record, "second line") {
		t.Fatalf("refused start lost its first line: %s", record)
	}
	request.BriefPath, request.Runtime = corrected, ""
	result, err := branch.RunBranchRead(request)
	if err != nil || result.State != "dispatched" || len(dispatched) != 1 || dispatched[0] != "/" {
		t.Fatalf("corrected brief after a refused dispatch=%+v err=%v dispatched=%v", result, err, dispatched)
	}
	if record := gleBranchReadRecord(t, r.root, unit); strings.Contains(record, "dispatchRefusal") || strings.Contains(record, "dispatchRetryable") {
		t.Fatalf("restart kept refusal: %s", record)
	}
	request.BriefPath = refused
	if _, err := branch.RunBranchRead(request); goal.RefusalCode(err) != branch.ReadBriefChangedCode || !strings.Contains(err.Error(), "critic-corrected") || len(dispatched) != 1 {
		t.Fatalf("another brief after a dispatch=%v dispatched=%v", err, dispatched)
	}
	r.expectStart()
	writeReadJobWithSubject(t, r.root, "critic-corrected", unit, "completed", false, r.readSubject())
	if _, err := branch.RunBranchRead(request); goal.RefusalCode(err) != branch.ReadBriefChangedCode || !strings.Contains(err.Error(), "critic-corrected") || len(dispatched) != 1 {
		t.Fatalf("another brief after examination=%v dispatched=%v", err, dispatched)
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
	if err == nil || !strings.Contains(goal.RecordText(err), branch.ReadUngatedCode) ||
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

func TestAReviewRefusedBeforeReservationKeepsItsFirstLine(t *testing.T) {
	t.Parallel()
	t.Run("refused start", TestReviewWithABriefNeverJoins)
}
