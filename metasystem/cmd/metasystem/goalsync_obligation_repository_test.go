package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

type obligationCommit struct {
	parent, trailer string
	files           map[string][]byte
	at              time.Time
}

type obligationRepository struct {
	t                                                  *testing.T
	commits                                            map[string]obligationCommit
	canonical, accepted                                string
	serial                                             uint64
	captures, builds, publications, advances, releases int
	captureErr                                         error
}

func obligationFilesCopy(files map[string][]byte) map[string][]byte {
	copy := make(map[string][]byte, len(files))
	for path, data := range files {
		copy[path] = append([]byte(nil), data...)
	}
	return copy
}

func (r *obligationRepository) commit(id string) obligationCommit {
	r.t.Helper()
	commit, ok := r.commits[id]
	if !ok {
		r.t.Fatalf("unknown obligation commit %q", id)
	}
	return commit
}

func (r *obligationRepository) Capture(opid string) (string, error) {
	r.t.Helper()
	if opid == "" {
		r.t.Fatal("capture without an operation id")
	}
	if r.captureErr != nil {
		return "", r.captureErr
	}
	r.commit(r.canonical)
	r.captures++
	return r.canonical, nil
}

func (r *obligationRepository) Accepted() (string, bool, error) {
	r.t.Helper()
	r.commit(r.accepted)
	return r.accepted, true, nil
}

func (r *obligationRepository) Files(id string, prefixes ...string) (map[string][]byte, error) {
	r.t.Helper()
	if len(prefixes) == 0 {
		r.t.Fatal("obligation file read without a prefix")
	}
	files := r.commit(id).files
	selected := make(map[string][]byte)
	for path, data := range files {
		for _, prefix := range prefixes {
			if prefix == "" {
				r.t.Fatal("obligation file read with an empty prefix")
			}
			if strings.HasPrefix(path, prefix) {
				selected[path] = append([]byte(nil), data...)
				break
			}
		}
	}
	return selected, nil
}

func (r *obligationRepository) Build(opid, parent string, changes []goal.Change, message string) (string, error) {
	r.t.Helper()
	if opid == "" || message == "" || len(changes) == 0 {
		r.t.Fatal("incomplete obligation build")
	}
	base := r.commit(parent)
	files := obligationFilesCopy(base.files)
	for _, change := range changes {
		if change.Path == "" {
			r.t.Fatal("obligation build changed an empty path")
		}
		if change.Delete {
			delete(files, change.Path)
		} else {
			files[change.Path] = append([]byte(nil), change.Content...)
		}
	}
	r.serial++
	id := fmt.Sprintf("%040x", r.serial)
	r.commits[id] = obligationCommit{parent: parent, trailer: opid, files: files, at: syncRequestTestNow}
	r.builds++
	return id, nil
}

func (r *obligationRepository) Publish(parent, id string) (goal.CASOutcome, error) {
	r.t.Helper()
	if r.commit(id).parent != parent || r.canonical != parent {
		return goal.CASRefused, fmt.Errorf("canonical compare failed for %s", parent)
	}
	r.canonical = id
	r.publications++
	return goal.CASLanded, nil
}

func (r *obligationRepository) AcceptedCAS(old, next string) error {
	r.t.Helper()
	r.commit(next)
	if r.accepted != old {
		return fmt.Errorf("accepted compare failed for %s", old)
	}
	r.accepted = next
	r.advances++
	return nil
}

func (r *obligationRepository) IsAncestor(ancestor, descendant string) (bool, error) {
	r.t.Helper()
	r.commit(ancestor)
	for descendant != "" {
		if descendant == ancestor {
			return true, nil
		}
		descendant = r.commit(descendant).parent
	}
	return false, nil
}

func (r *obligationRepository) TrailerPresent(tip, opid string) (bool, error) {
	r.t.Helper()
	for tip != "" {
		commit := r.commit(tip)
		if commit.trailer == opid {
			return true, nil
		}
		tip = commit.parent
	}
	return false, nil
}

func (r *obligationRepository) CommitWithTrailer(string, string, string) (string, error) {
	r.t.Fatal("set-obligation unexpectedly queried carry history")
	return "", nil
}

func (r *obligationRepository) CommitTime(id string) (time.Time, error) {
	return r.commit(id).at, nil
}

func (r *obligationRepository) Release(opid string) error {
	if opid == "" {
		r.t.Fatal("release without an operation id")
	}
	r.releases++
	return nil
}

var _ goal.Repository = (*obligationRepository)(nil)

type obligationCommandFixture struct {
	t     *testing.T
	facts *syncRequestFacts
	repo  *obligationRepository
}

func newObligationCommandFixture(t *testing.T) *obligationCommandFixture {
	t.Helper()
	facts := newSyncRequestFacts(t)
	root := facts.root
	now := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	openedAt := now.Add(-time.Hour).Format(time.RFC3339)
	claimAt := now.Add(-55 * time.Minute).Format(time.RFC3339)
	approvedAt := now.Add(-54 * time.Minute).Format(time.RFC3339)
	budget := &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 2, ReviewRoundLimit: 3}
	risk := &goal.RiskRecord{Severity: 3, Novelty: 3, Exposure: 1, Accumulation: 1, Basis: "The fixture exercises an admitted tier-three goal."}
	approvalOpid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAZ", "mac-cli", "m1")
	file := &goal.GoalFile{
		Id: "standing-validation", State: goal.StateClaimed, Tier: 3, Intent: "Govern validation.", Origin: goal.OriginMain,
		NextStep: "Run it.", OpenedAt: openedAt, Revision: 3,
		Budget: budget, Risk: risk,
		Claimed: &goal.ClaimRecord{Machine: "mac-cli", Lineage: "m1", At: claimAt, Revision: 2},
		Approved: &goal.ApprovalRecord{
			By: "human:Wido", At: approvedAt, Revision: 3, Opid: approvalOpid,
			Authority: goal.ApprovalAuthorityProven, Digest: goal.ApprovalDigest("Govern validation.", 3, *budget, risk),
		},
		History: []goal.HistoryLine{
			{At: openedAt, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAA", "mac-cli", "m1"), Verb: "open", Actor: "mac-cli+m1", Targets: []string{"standing-validation"}, Keep: -1},
			{At: claimAt, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAB", "mac-cli", "m1"), Verb: "claim", Actor: "mac-cli+m1", Targets: []string{"standing-validation"}, Keep: -1},
			{At: approvedAt, Opid: approvalOpid, Verb: "approve", Actor: "human:Wido", Targets: []string{"standing-validation"}, Keep: -1},
		},
	}
	goalPath := filepath.Join(root, "plans", "goals", "standing-validation.md")
	goalBytes := goal.RenderFile(file)
	if err := os.WriteFile(goalPath, goalBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte)
	for _, path := range []string{"metasystem.conf", "plans/goals/backlog.md", "plans/goals/standing-validation.md", "scripts/agents/pre-commit-guard.sh"} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		files[path] = bytes.Clone(data)
	}
	id := fmt.Sprintf("%040x", 1)
	repo := &obligationRepository{t: t, commits: map[string]obligationCommit{id: {files: obligationFilesCopy(files), at: now}}, canonical: id, accepted: id, serial: 1}
	return &obligationCommandFixture{t: t, facts: facts, repo: repo}
}

func (f *obligationCommandFixture) root() string { return f.facts.root }

func (f *obligationCommandFixture) commandNow(root string) (time.Time, error) {
	if _, err := f.facts.commandNow(root); err != nil {
		return time.Time{}, err
	}
	return time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC), nil
}

func (f *obligationCommandFixture) dependencies() syncRequestDependencies {
	dependencies := f.facts.dependencies()
	endpoint := dependencies.endpoint
	dependencies.endpoint = func(root string) (goal.Endpoint, error) {
		e, err := endpoint(root)
		e.Repository = f.repo
		return e, err
	}
	return dependencies
}

func (f *obligationCommandFixture) run(args []string, prove goalAuthorityProver) (string, string, int) {
	f.t.Helper()
	var stdout string
	stderr, code := captureStderr(f.t, func() int {
		var innerCode int
		stdout, innerCode = captureStdout(f.t, func() int {
			return runGoalSetObligationWithAuthorityFactsAtWithDependencies(args, prove, f.commandNow, f.dependencies())
		})
		return innerCode
	})
	return stdout, stderr, code
}

func (f *obligationCommandFixture) acceptedGoal() (*goal.GoalFile, []byte) {
	f.t.Helper()
	rendered := f.repo.commit(f.repo.accepted).files["plans/goals/standing-validation.md"]
	file, problems := goal.ParseFile(rendered)
	if file == nil || len(problems) != 0 {
		f.t.Fatalf("accepted goal record did not parse: record=%+v problems=%v", file, problems)
	}
	return file, rendered
}

func (f *obligationCommandFixture) expectTransactions(publications, rejectedMutations int) {
	f.t.Helper()
	wantCaptures := publications*2 + rejectedMutations
	if f.repo.builds != publications || f.repo.publications != publications || f.repo.advances != publications ||
		f.repo.captures != wantCaptures || f.repo.releases != wantCaptures || f.repo.canonical != f.repo.accepted {
		f.t.Fatalf("obligation operations: builds=%d publications=%d accepted advances=%d captures=%d releases=%d canonical=%s accepted=%s; want %d publications and %d rejected mutations",
			f.repo.builds, f.repo.publications, f.repo.advances, f.repo.captures, f.repo.releases, f.repo.canonical, f.repo.accepted, publications, rejectedMutations)
	}
}

func (f *obligationCommandFixture) expectFacts(classifications, requests int) {
	f.t.Helper()
	sequence := make([]string, 0, classifications*8)
	for index := 0; index < classifications; index++ {
		sequence = append(sequence, "repository top", "ledger identity", "clock", "clock")
		if index < requests {
			sequence = append(sequence, "guard", "endpoint", "machine", "clock")
		}
	}
	f.facts.expect(classifications, requests, 0, classifications*2+requests, 0, 0, sequence...)
}

func (f *obligationCommandFixture) proofFiles() []string {
	f.t.Helper()
	matches, err := filepath.Glob(filepath.Join(f.root(), "artifacts", "agents", "authority", "proofs", "*.json"))
	if err != nil {
		f.t.Fatal(err)
	}
	return matches
}

func (f *obligationCommandFixture) proofRecord() []byte {
	f.t.Helper()
	matches := f.proofFiles()
	if len(matches) != 1 {
		f.t.Fatalf("temporary CLI mutation wrote %d local proof files; want one", len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		f.t.Fatal(err)
	}
	return data
}

func (f *obligationCommandFixture) runReal(args []string) (string, string, int) {
	return f.run(args, humanauthority.ProveOrTemporaryGoalAuthority)
}
