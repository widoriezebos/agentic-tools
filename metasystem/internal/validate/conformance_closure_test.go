package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

type mergeClosureFixture struct {
	t               *testing.T
	fixture         *conformanceFixture
	implementer     map[string]any
	finalTree       string
	resolveEndpoint func(string) (goal.Endpoint, error)
	acceptedGoal    *acceptedGoalDecisionRepository
}

type acceptedGoalDecisionRepository struct {
	t     *testing.T
	tip   string
	files map[string][]byte
	reads int
}

func (r *acceptedGoalDecisionRepository) unexpected(method string) {
	r.t.Helper()
	r.t.Fatalf("unexpected goal repository method %s", method)
}

func (r *acceptedGoalDecisionRepository) Capture(string) (string, error) {
	r.unexpected("Capture")
	return "", nil
}

func (r *acceptedGoalDecisionRepository) Accepted() (string, bool, error) {
	r.reads++
	return r.tip, true, nil
}

func (r *acceptedGoalDecisionRepository) Files(commit string, prefixes ...string) (map[string][]byte, error) {
	r.t.Helper()
	if commit != r.tip || len(prefixes) != 2 || prefixes[0] != "plans/goals/" || prefixes[1] != "records/goals/" {
		r.t.Fatalf("unexpected accepted goal files request: commit %q prefixes %q", commit, prefixes)
	}
	r.reads++
	copy := make(map[string][]byte, len(r.files))
	for path, data := range r.files {
		copy[path] = append([]byte(nil), data...)
	}
	return copy, nil
}

func (r *acceptedGoalDecisionRepository) Build(string, string, []goal.Change, string) (string, error) {
	r.unexpected("Build")
	return "", nil
}

func (r *acceptedGoalDecisionRepository) Publish(string, string) (goal.CASOutcome, error) {
	r.unexpected("Publish")
	return "", nil
}

func (r *acceptedGoalDecisionRepository) AcceptedCAS(string, string) error {
	r.unexpected("AcceptedCAS")
	return nil
}

func (r *acceptedGoalDecisionRepository) IsAncestor(string, string) (bool, error) {
	r.unexpected("IsAncestor")
	return false, nil
}

func (r *acceptedGoalDecisionRepository) TrailerPresent(string, string) (bool, error) {
	r.unexpected("TrailerPresent")
	return false, nil
}

func (r *acceptedGoalDecisionRepository) CommitWithTrailer(string, string, string) (string, error) {
	r.unexpected("CommitWithTrailer")
	return "", nil
}

func (r *acceptedGoalDecisionRepository) CommitTime(commit string) (time.Time, error) {
	r.t.Helper()
	if commit != r.tip {
		r.t.Fatalf("unexpected accepted goal commit time request: %q", commit)
	}
	r.reads++
	return time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC), nil
}

func (r *acceptedGoalDecisionRepository) Release(string) error {
	r.unexpected("Release")
	return nil
}

func (f *mergeClosureFixture) requireAcceptedGoalRead() {
	f.t.Helper()
	if f.acceptedGoal == nil || f.acceptedGoal.reads != 3 {
		f.t.Fatalf("accepted goal repository read count = %v, want Accepted, Files, and CommitTime", f.acceptedGoal)
	}
}

func newFileMergeClosureFixture(t *testing.T) *mergeClosureFixture {
	t.Helper()
	return assembleMergeClosureFixture(t, newFileConformanceFixture(t))
}

func assembleMergeClosureFixture(t *testing.T, fixture *conformanceFixture) *mergeClosureFixture {
	t.Helper()
	implementer := map[string]any{
		"jobId": "implementation", "role": "implementer", "round": 1, "parentJob": nil,
		"status": "completed", "effectiveModel": "implementer-model",
	}
	fixture.writeJSON("artifacts/agents/jobs/implementation.json", implementer)
	return &mergeClosureFixture{t: t, fixture: fixture, implementer: implementer, finalTree: strings.Repeat("a", 40)}
}

func (f *mergeClosureFixture) seedGoalDecision(status string, includeAcceptedGoal bool) string {
	f.t.Helper()
	opened := goal.HistoryLine{
		At: "2026-08-20T10:00:00Z", Opid: "01J5X0000000000000000000B0-mac-a-1a2b3c4d",
		Verb: "open", Actor: "mac-a+lin-1", Targets: []string{"goal-a"}, Keep: -1,
	}
	file := &goal.GoalFile{
		Id: "goal-a", State: goal.StateQueued, Intent: "Carry the critique decision.",
		Origin: goal.OriginMain, OpenedAt: opened.At, Revision: 1, History: []goal.HistoryLine{opened},
	}
	if status == "deferred" {
		file.ReviewObligations = []goal.ReviewObligation{{
			Finding: "F-GOAL", Chain: "critic-a", Artifact: "internal/example.go",
			Test: "prove the finding", State: "open",
		}}
	} else {
		file.AcceptedRisks = []goal.AcceptedRiskRecord{{
			Finding: "F-GOAL", Chain: "critic-a", By: "wido", Opid: "decision-op",
		}}
	}
	root := &goal.RootRecord{
		Identity: "01J5X000000000000000000000", FormatVersion: "1", SyncMode: goal.SyncRemote,
		MigrationEpoch: "2026-08-20T00:00:00Z", ManifestDigest: strings.Repeat("ab", 32),
		MigrationMode: "manifest", Revision: 1,
		History: []goal.HistoryLine{{
			At: "2026-08-20T09:00:00Z", Opid: "01J5X0000000000000000000A0-mac-a-1a2b3c4d",
			Verb: "migrate", Actor: "mac-a+lin-1", Keep: -1,
		}},
	}
	goalsDir := filepath.Join(f.fixture.controller, "plans", "goals")
	if err := os.MkdirAll(goalsDir, 0o755); err != nil {
		f.t.Fatal(err)
	}
	rootBytes := goal.RenderRoot(root)
	goalBytes := goal.RenderFile(file)
	if err := os.WriteFile(filepath.Join(goalsDir, "backlog.md"), rootBytes, 0o644); err != nil {
		f.t.Fatal(err)
	}
	goalPath := filepath.Join(goalsDir, "goal-a.md")
	if err := os.WriteFile(goalPath, goalBytes, 0o644); err != nil {
		f.t.Fatal(err)
	}
	files := map[string][]byte{"plans/goals/backlog.md": append([]byte(nil), rootBytes...)}
	if includeAcceptedGoal {
		files["plans/goals/goal-a.md"] = append([]byte(nil), goalBytes...)
	}
	repo := &acceptedGoalDecisionRepository{t: f.t, tip: strings.Repeat("c", 40), files: files}
	f.acceptedGoal = repo
	f.resolveEndpoint = func(requestedRoot string) (goal.Endpoint, error) {
		if requestedRoot != f.fixture.controller {
			return goal.Endpoint{}, fmt.Errorf("unexpected goal endpoint root %q", requestedRoot)
		}
		return goal.Endpoint{Root: requestedRoot, Remote: "origin", Repository: repo}, nil
	}
	return goalPath
}

func mergeRegisterFinding(id, status, resolution string) map[string]any {
	return map[string]any{
		"findingId": id, "critic": "critic", "rigorClass": "bounded",
		"factsDigest": "facts", "facts": "facts", "artifact": "internal/example.go",
		"title": "finding", "status": status, "resolution": resolution,
		"decisionOpid": "decision-op", "evidence": "evidence",
		"evidenceDigest": "evidence-digest", "multiplicity": 1,
	}
}

func (f *mergeClosureFixture) writeClosedCritic(id, reviewedMember, tree string) {
	f.t.Helper()
	subject := readsubject.ReadSubject{
		Kind: readsubject.SubjectLive, ImplementerRoot: "implementation", ReviewedMember: reviewedMember,
		ReviewedProjectTree: tree, DiffDigest: strings.Repeat(id[len(id)-1:], 64),
	}
	root := map[string]any{
		"jobId": id, "role": "code-critic", "round": 1, "parentJob": nil,
		"reviews": reviewedMember, "status": "completed", "effectiveModel": id + "-model",
		"chainClosed": true, "findingRegister": []any{}, "findingRegisterRound": 1,
		"findingRegisterSubjectDigest": subject.Digest(),
		"closure":                      readsubject.Closure{CriticRoot: id, Round: 1, Subject: subject, Mechanism: "clean"},
	}
	f.fixture.writeJSON("artifacts/agents/jobs/"+id+".json", root)
	f.fixture.writeJSON("artifacts/agents/"+id+"/rounds/1/subject.json", subject)
	f.fixture.writeJSON("artifacts/agents/"+id+"/rounds/1/return.json", map[string]any{
		"jobId": id, "round": 1, "reviewedTree": tree,
		"findings": []any{}, "verdictMaterialCount": 0,
	})
}

func (f *mergeClosureFixture) writeHistoricalCritic(id, tree string, register []any) {
	f.t.Helper()
	f.fixture.writeJSON("artifacts/agents/jobs/"+id+".json", map[string]any{
		"jobId": id, "role": "code-critic", "round": 1, "parentJob": nil,
		"reviews": "implementation", "status": "completed", "effectiveModel": id + "-model",
		"chainClosed": true, "findingRegister": register, "findingRegisterRound": 1,
	})
	f.fixture.writeJSON("artifacts/agents/"+id+"/rounds/1/return.json", map[string]any{
		"jobId": id, "round": 1, "reviewedTree": tree,
		"findings": []any{}, "verdictMaterialCount": 0,
	})
}

func (f *mergeClosureFixture) writeRegisterlessMaterialCritic(id, tree string) {
	f.t.Helper()
	f.fixture.writeJSON("artifacts/agents/jobs/"+id+".json", map[string]any{
		"jobId": id, "role": "code-critic", "round": 1, "parentJob": nil,
		"reviews": "implementation", "status": "completed", "effectiveModel": id + "-model",
		"chainClosed": true,
	})
	f.fixture.writeJSON("artifacts/agents/"+id+"/rounds/1/return.json", map[string]any{
		"jobId": id, "round": 1, "reviewedTree": tree,
		"findings":             []any{map[string]any{"id": "F-LEGACY", "material": true}},
		"verdictMaterialCount": 1,
	})
}

// A return that names a material finding but carries no readable count: the
// count is optional in records written before it existed, and the finding
// itself must still refuse.
func (f *mergeClosureFixture) writeMaterialCriticWithoutCount(id, tree string) {
	f.t.Helper()
	f.fixture.writeJSON("artifacts/agents/jobs/"+id+".json", map[string]any{
		"jobId": id, "role": "code-critic", "round": 1, "parentJob": nil,
		"reviews": "implementation", "status": "completed", "effectiveModel": id + "-model",
		"chainClosed": true,
	})
	f.fixture.writeJSON("artifacts/agents/"+id+"/rounds/1/return.json", map[string]any{
		"jobId": id, "round": 1, "reviewedTree": tree,
		"findings": []any{map[string]any{"id": "F-NOCOUNT", "material": true}},
	})
}

func (f *mergeClosureFixture) writeCriticWithoutReturn(id, status string) {
	f.t.Helper()
	f.fixture.writeJSON("artifacts/agents/jobs/"+id+".json", map[string]any{
		"jobId": id, "role": "code-critic", "round": 1, "parentJob": nil,
		"reviews": "implementation", "status": status, "effectiveModel": id + "-model",
	})
}

func (f *mergeClosureFixture) replaceClosureSubject(id string, subject readsubject.ReadSubject) {
	f.t.Helper()
	root := f.criticRoot(id)
	root["closure"] = readsubject.Closure{CriticRoot: id, Round: 1, Subject: subject, Mechanism: "clean"}
	root["findingRegisterSubjectDigest"] = subject.Digest()
	f.fixture.writeJSON("artifacts/agents/jobs/"+id+".json", root)
	f.fixture.writeJSON("artifacts/agents/"+id+"/rounds/1/subject.json", subject)
}

func (f *mergeClosureFixture) run(selected string) ([]string, []string, int) {
	f.t.Helper()
	run := &conformanceRun{
		root: f.fixture.controller, rootJob: "implementation", record: f.implementer,
		criticRoot: selected, resolveEndpoint: f.resolveEndpoint,
	}
	return run.mergeCritique("", f.finalTree, "fake", "")
}

func (f *mergeClosureFixture) criticRoot(id string) map[string]any {
	f.t.Helper()
	root, ok := readJobRecord(filepath.Join(f.fixture.controller, "artifacts", "agents", "jobs"), id)
	if !ok {
		f.t.Fatalf("critic root %s is unreadable", id)
	}
	return root
}

func (f *mergeClosureFixture) requireAccepted(selected string) {
	f.t.Helper()
	_, errs, code := f.run(selected)
	if code != 0 {
		f.t.Fatalf("merge refused with code %d: %v", code, errs)
	}
}

func (f *mergeClosureFixture) requireRefused(selected, contains string) {
	f.t.Helper()
	_, errs, code := f.run(selected)
	joined := strings.Join(errs, "\n")
	if code != 1 || !strings.Contains(joined, contains) {
		f.t.Fatalf("merge result code %d errors %v, want %q", code, errs, contains)
	}
}

func TestMergeCritiqueClosureAndUnion(t *testing.T) {
	t.Run("bound current closure", func(t *testing.T) {
		fixture := newFileMergeClosureFixture(t)
		fixture.writeClosedCritic("critic-a", "implementation", fixture.finalTree)
		fixture.requireAccepted("")
	})

	for _, status := range []string{"open", "disputed"} {
		t.Run("older "+status+" root remains in union", func(t *testing.T) {
			fixture := newFileMergeClosureFixture(t)
			fixture.writeClosedCritic("critic-a", "implementation", fixture.finalTree)
			fixture.writeHistoricalCritic("critic-b", strings.Repeat("b", 40), []any{mergeRegisterFinding("F-OLD", status, "")})
			fixture.requireRefused("", "critic-b: canonical finding register has unresolved finding 'F-OLD'")
		})
	}

	t.Run("selected root does not hide older unresolved root", func(t *testing.T) {
		fixture := newFileMergeClosureFixture(t)
		fixture.writeClosedCritic("critic-a", "implementation", fixture.finalTree)
		fixture.writeHistoricalCritic("critic-b", strings.Repeat("b", 40), []any{mergeRegisterFinding("F-OLD", "open", "")})
		fixture.requireRefused("critic-a", "critic-b: canonical finding register has unresolved finding 'F-OLD'")
	})

	t.Run("older registerless material return remains in union", func(t *testing.T) {
		fixture := newFileMergeClosureFixture(t)
		fixture.writeClosedCritic("critic-a", "implementation", fixture.finalTree)
		fixture.writeRegisterlessMaterialCritic("critic-b", strings.Repeat("b", 40))
		fixture.requireRefused("", "critic-b: final round still has material findings despite any dispositions: F-LEGACY")
	})

	t.Run("material finding without a count still refuses", func(t *testing.T) {
		fixture := newFileMergeClosureFixture(t)
		fixture.writeMaterialCriticWithoutCount("critic-a", fixture.finalTree)
		fixture.requireRefused("", "critic-a: final round still has material findings despite any dispositions: F-NOCOUNT")
	})

	t.Run("older material finding without a count blocks a matching current root", func(t *testing.T) {
		fixture := newFileMergeClosureFixture(t)
		fixture.writeClosedCritic("critic-a", "implementation", fixture.finalTree)
		fixture.writeMaterialCriticWithoutCount("critic-b", strings.Repeat("b", 40))
		fixture.requireRefused("", "critic-b: final round still has material findings despite any dispositions: F-NOCOUNT")
	})

	t.Run("old clean root is harmless beside current closure", func(t *testing.T) {
		fixture := newFileMergeClosureFixture(t)
		fixture.writeClosedCritic("critic-a", "implementation", fixture.finalTree)
		fixture.writeHistoricalCritic("critic-b", strings.Repeat("b", 40), []any{})
		fixture.requireAccepted("")
	})

	t.Run("closure must name the implementation root", func(t *testing.T) {
		fixture := newFileMergeClosureFixture(t)
		fixture.writeClosedCritic("critic-a", "implementation", fixture.finalTree)
		fixture.replaceClosureSubject("critic-a", readsubject.ReadSubject{
			Kind: readsubject.SubjectLive, ImplementerRoot: "other-implementation", ReviewedMember: "implementation",
			ReviewedProjectTree: fixture.finalTree, DiffDigest: strings.Repeat("a", 64),
		})
		fixture.requireRefused("", "names implementation root 'other-implementation' instead of 'implementation'")
	})

	t.Run("closure reviewed member must belong to implementation", func(t *testing.T) {
		fixture := newFileMergeClosureFixture(t)
		fixture.writeClosedCritic("critic-a", "implementation", fixture.finalTree)
		fixture.replaceClosureSubject("critic-a", readsubject.ReadSubject{
			Kind: readsubject.SubjectLive, ImplementerRoot: "implementation", ReviewedMember: "other-member",
			ReviewedProjectTree: fixture.finalTree, DiffDigest: strings.Repeat("a", 64),
		})
		fixture.requireRefused("", "reviewed member 'other-member' outside implementation root 'implementation'")
	})

	t.Run("absent selected root", func(t *testing.T) {
		fixture := newFileMergeClosureFixture(t)
		fixture.writeClosedCritic("critic-a", "implementation", fixture.finalTree)
		fixture.requireRefused("critic-missing", "requested code-critic chain 'critic-missing' is absent")
	})

	t.Run("invalid selected root", func(t *testing.T) {
		fixture := newFileMergeClosureFixture(t)
		fixture.writeClosedCritic("critic-a", "implementation", strings.Repeat("b", 40))
		fixture.writeClosedCritic("critic-b", "implementation", fixture.finalTree)
		fixture.requireRefused("critic-a", "requested code-critic chain 'critic-a' is not valid and current")
	})
}

func TestMergeCritiqueIgnoresIncompleteStaleRootsWhenCurrentRootMatches(t *testing.T) {
	for _, status := range []string{"running", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			fixture := newFileMergeClosureFixture(t)
			fixture.writeClosedCritic("critic-a", "implementation", fixture.finalTree)
			fixture.writeCriticWithoutReturn("critic-b", status)
			fixture.requireAccepted("")
		})
	}
}

func TestMergeCritiqueRejectsStaleClosureRound(t *testing.T) {
	for _, status := range []string{"running", "failed", "cancelled", "completed"} {
		t.Run(status, func(t *testing.T) {
			fixture := newFileMergeClosureFixture(t)
			fixture.writeClosedCritic("critic-a", "implementation", fixture.finalTree)
			fixture.writeClosedCritic("critic-b", "implementation", strings.Repeat("b", 40))
			fixture.fixture.writeJSON("artifacts/agents/jobs/critic-b-r2.json", map[string]any{
				"jobId": "critic-b-r2", "role": "code-critic", "round": 2,
				"parentJob": "critic-b", "status": status, "effectiveModel": "critic-b-model",
			})
			fixture.fixture.writeJSON("artifacts/agents/critic-b/rounds/2/return.json", map[string]any{
				"jobId": "critic-b-r2", "round": 2, "reviewedTree": strings.Repeat("b", 40),
				"findings": []any{}, "verdictMaterialCount": 0,
			})
			fixture.requireRefused("", "critic-b: closure")
		})
	}
}

func TestMergeCritiqueClosureAbsence(t *testing.T) {
	t.Run("modern clean fold", func(t *testing.T) {
		fixture := newFileMergeClosureFixture(t)
		fixture.writeClosedCritic("critic-a", "implementation", fixture.finalTree)
		root := fixture.criticRoot("critic-a")
		delete(root, "closure")
		fixture.fixture.writeJSON("artifacts/agents/jobs/critic-a.json", root)
		fixture.requireRefused("", "clean subject-bearing fold at round 1 has no closure")
	})

	t.Run("registerless historical", func(t *testing.T) {
		fixture := newFileMergeClosureFixture(t)
		fixture.writeHistoricalCritic("critic-a", fixture.finalTree, nil)
		root := fixture.criticRoot("critic-a")
		delete(root, "findingRegister")
		delete(root, "findingRegisterRound")
		fixture.fixture.writeJSON("artifacts/agents/jobs/critic-a.json", root)
		fixture.requireAccepted("")
	})

	t.Run("lawful out-of-scope decision", func(t *testing.T) {
		fixture := newFileMergeClosureFixture(t)
		fixture.writeHistoricalCritic("critic-a", fixture.finalTree, []any{mergeRegisterFinding("F-OOS", "resolved", "out-of-scope")})
		fixture.requireAccepted("")
	})

	t.Run("malformed present closure", func(t *testing.T) {
		fixture := newFileMergeClosureFixture(t)
		fixture.writeClosedCritic("critic-a", "implementation", fixture.finalTree)
		root := fixture.criticRoot("critic-a")
		root["closure"] = "malformed"
		fixture.fixture.writeJSON("artifacts/agents/jobs/critic-a.json", root)
		fixture.requireRefused("", "closure must be an object")
	})

	t.Run("changed folded subject digest", func(t *testing.T) {
		fixture := newFileMergeClosureFixture(t)
		fixture.writeClosedCritic("critic-a", "implementation", fixture.finalTree)
		root := fixture.criticRoot("critic-a")
		root["findingRegisterSubjectDigest"] = "changed"
		fixture.fixture.writeJSON("artifacts/agents/jobs/critic-a.json", root)
		fixture.requireRefused("", "does not equal folded subject changed")
	})
}

func TestMergeCritiqueGoalDecisionsUseAcceptedRepository(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"deferred", "accepted-risk"} {
		t.Run(status+" still requires matching goal record", func(t *testing.T) {
			fixture := newFileMergeClosureFixture(t)
			fixture.writeHistoricalCritic("critic-a", fixture.finalTree, []any{mergeRegisterFinding("F-GOAL", status, status)})
			root := fixture.criticRoot("critic-a")
			root["goalId"] = "goal-a"
			fixture.fixture.writeJSON("artifacts/agents/jobs/critic-a.json", root)
			fixture.seedGoalDecision(status, false)
			fixture.requireRefused("", status+" finding 'F-GOAL' has no readable matching goal record")
			fixture.requireAcceptedGoalRead()
		})
		t.Run(status+" accepts matching goal record", func(t *testing.T) {
			fixture := newFileMergeClosureFixture(t)
			fixture.writeHistoricalCritic("critic-a", fixture.finalTree, []any{mergeRegisterFinding("F-GOAL", status, status)})
			root := fixture.criticRoot("critic-a")
			root["goalId"] = "goal-a"
			fixture.fixture.writeJSON("artifacts/agents/jobs/critic-a.json", root)
			goalPath := fixture.seedGoalDecision(status, true)
			if err := os.Remove(goalPath); err != nil {
				t.Fatal(err)
			}
			fixture.requireAccepted("")
			fixture.requireAcceptedGoalRead()
		})
	}
}
