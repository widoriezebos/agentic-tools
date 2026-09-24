package validate

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

type recertificationClosureFixture struct {
	t            *testing.T
	fixture      *conformanceFixture
	run          *conformanceRun
	subject      readsubject.ReadSubject
	patch        []byte
	reviewedTree string
}

func newRecertificationClosureFixture(t *testing.T) *recertificationClosureFixture {
	t.Helper()
	return assembleRecertificationClosureFixture(t, newConformanceFixture(t))
}

func newFileRecertificationClosureFixture(t *testing.T) *recertificationClosureFixture {
	t.Helper()
	return assembleRecertificationClosureFixture(t, newFileConformanceFixture(t))
}

func assembleRecertificationClosureFixture(t *testing.T, fixture *conformanceFixture) *recertificationClosureFixture {
	t.Helper()
	reviewedTree := strings.Repeat("a", 40)
	patch := []byte("same reviewed patch bytes\n")
	implementation := map[string]any{
		"jobId": "implementation", "role": "implementer", "round": 1, "parentJob": nil,
		"status": "completed", "effectiveModel": "implementer-model", "chainClosed": true,
		"independentCritiqueJobRef": "critic", "workspaceRoot": fixture.worktree,
	}
	later := map[string]any{
		"jobId": "implementation-r2", "role": "implementer", "round": 2,
		"parentJob": "implementation", "status": "completed", "effectiveModel": "implementer-model",
		"workspaceRoot": fixture.worktree,
	}
	fixture.writeJSON("artifacts/agents/jobs/implementation.json", implementation)
	fixture.writeJSON("artifacts/agents/jobs/implementation-r2.json", later)
	writeRecertificationReview(t, fixture, 1, "implementation", reviewedTree, patch, nil)
	writeRecertificationReview(t, fixture, 2, "implementation-r2", reviewedTree, patch, nil)
	subject := readsubject.ReadSubject{
		Kind: readsubject.SubjectLive, ImplementerRoot: "implementation", ReviewedMember: "implementation",
		ReviewedProjectTree: reviewedTree, DiffDigest: sha256Bytes(patch),
	}
	criticRoot := map[string]any{
		"jobId": "critic", "role": "code-critic", "round": 1, "parentJob": nil,
		"reviews": "implementation", "status": "completed", "effectiveModel": "critic-model",
		"chainClosed": true, "findingRegister": []any{}, "findingRegisterRound": 2,
		"findingRegisterSubjectDigest": subject.Digest(),
		"closure":                      readsubject.Closure{CriticRoot: "critic", Round: 2, Subject: subject, Mechanism: "clean"},
	}
	fixture.writeJSON("artifacts/agents/jobs/critic.json", criticRoot)
	fixture.writeJSON("artifacts/agents/jobs/critic-r2.json", map[string]any{
		"jobId": "critic-r2", "role": "code-critic", "round": 2,
		"parentJob": "critic", "status": "completed", "effectiveModel": "critic-model",
	})
	fixture.writeJSON("artifacts/agents/critic/rounds/1/return.json", map[string]any{
		"jobId": "critic", "round": 1, "reviewedTree": reviewedTree,
		"findings": []any{}, "verdictMaterialCount": 0,
	})
	fixture.writeJSON("artifacts/agents/critic/rounds/2/subject.json", subject)
	fixture.writeJSON("artifacts/agents/critic/rounds/2/return.json", map[string]any{
		"jobId": "critic-r2", "round": 2, "reviewedTree": reviewedTree,
		"findings": []any{}, "verdictMaterialCount": 0,
	})
	run := &conformanceRun{
		root: fixture.controller, rootJob: "implementation", job: "implementation-r2", record: later,
	}
	return &recertificationClosureFixture{
		t: t, fixture: fixture, run: run, subject: subject, patch: patch,
		reviewedTree: reviewedTree,
	}
}

func writeRecertificationReview(t *testing.T, fixture *conformanceFixture, round int, implementer, tree string, patch []byte, extra map[string]any) {
	t.Helper()
	review := map[string]any{
		"diffArtifact": "diff.patch", "implementerJob": implementer, "reviewedTree": tree,
	}
	for key, value := range extra {
		review[key] = value
	}
	fixture.writeJSON(filepath.Join("artifacts", "agents", "implementation", "rounds", strconv.Itoa(round), "review.json"), review)
	path := filepath.Join(fixture.controller, "artifacts", "agents", "implementation", "rounds", strconv.Itoa(round), "diff.patch")
	if err := os.WriteFile(path, patch, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *recertificationClosureFixture) readCriticRoot() map[string]any {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.fixture.controller, "artifacts", "agents", "jobs", "critic.json"))
	if err != nil {
		f.t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		f.t.Fatal(err)
	}
	return root
}

func (f *recertificationClosureFixture) writeCriticRoot(root map[string]any) {
	f.t.Helper()
	f.fixture.writeJSON("artifacts/agents/jobs/critic.json", root)
}

func (f *recertificationClosureFixture) writeClosureSubject(subject readsubject.ReadSubject) {
	f.t.Helper()
	root := f.readCriticRoot()
	root["findingRegisterSubjectDigest"] = subject.Digest()
	root["closure"] = readsubject.Closure{
		CriticRoot: "critic", Round: 2, Subject: subject, Mechanism: "clean",
	}
	f.writeCriticRoot(root)
	f.fixture.writeJSON("artifacts/agents/critic/rounds/2/subject.json", subject)
	f.subject = subject
}

func (f *recertificationClosureFixture) prepareVerifiedRecertification() string {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.fixture.worktree, "source.txt"), []byte("reviewed change\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
	project := gittree.Workspace{Dir: f.fixture.worktree}
	reviewedTree, err := project.Snapshot("HEAD")
	if err != nil {
		f.t.Fatal(err)
	}
	baseTree, err := project.TreeOf(f.fixture.baseSha)
	if err != nil {
		f.t.Fatal(err)
	}
	patch, err := project.Diff(baseTree, reviewedTree)
	if err != nil {
		f.t.Fatal(err)
	}
	subject := f.subject
	subject.ReviewedProjectTree = reviewedTree
	subject.DiffDigest = sha256Bytes(patch)
	f.writeClosureSubject(subject)
	f.patch = patch
	f.reviewedTree = reviewedTree
	writeRecertificationReview(f.t, f.fixture, 1, "implementation", reviewedTree, patch, nil)
	writeRecertificationReview(f.t, f.fixture, 2, "implementation-r2", reviewedTree, patch, nil)
	for round := 1; round <= 2; round++ {
		implementerJob, criticJob := "implementation", "critic"
		if round == 2 {
			implementerJob, criticJob = "implementation-r2", "critic-r2"
		}
		f.fixture.writeJSON(filepath.Join("artifacts", "agents", "implementation", "rounds", strconv.Itoa(round), "return.json"), map[string]any{
			"jobId": implementerJob, "round": round, "diffBoundary": []string{"source.txt"},
		})
		f.fixture.writeJSON(filepath.Join("artifacts", "agents", "critic", "rounds", strconv.Itoa(round), "return.json"), map[string]any{
			"jobId": criticJob, "round": round, "reviewedTree": reviewedTree,
			"findings": []any{}, "verdictMaterialCount": 0,
		})
	}
	if err := os.WriteFile(filepath.Join(f.fixture.controller, "docs", "note.md"), []byte("target change\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
	f.fixture.git(f.fixture.controller, "add", "docs/note.md")
	f.fixture.git(f.fixture.controller, "-c", "user.name=m", "-c", "user.email=m@x", "commit", "-qm", "target change")
	f.run.workspace = f.fixture.worktree
	f.run.roundText = "2"
	out, errs, code := f.run.recertify("true")
	if code != 0 {
		f.t.Fatalf("valid recertification production failed: code=%d out=%v errs=%v", code, out, errs)
	}
	for _, line := range out {
		if path, ok := strings.CutPrefix(line, "recertification="); ok {
			if _, err := VerifyRecertification(f.fixture.controller, "implementation", path); err != nil {
				f.t.Fatalf("valid recertification verification failed: %v", err)
			}
			return path
		}
	}
	f.t.Fatalf("recertification output has no proof path: %v", out)
	return ""
}

func addNewerCriticMember(f *recertificationClosureFixture) {
	f.fixture.writeJSON("artifacts/agents/jobs/critic-r3.json", map[string]any{
		"jobId": "critic-r3", "role": "code-critic", "round": 3,
		"parentJob": "critic-r2", "status": "completed", "effectiveModel": "critic-model",
	})
}

func changeClosurePatchDigest(f *recertificationClosureFixture) {
	subject := f.subject
	subject.DiffDigest = sha256Bytes([]byte("different closure patch identity\n"))
	f.writeClosureSubject(subject)
}

func changeClosureRoot(f *recertificationClosureFixture) {
	root := f.readCriticRoot()
	root["closure"] = readsubject.Closure{CriticRoot: "other-critic", Round: 2, Subject: f.subject, Mechanism: "clean"}
	f.writeCriticRoot(root)
}

func removeRequiredClosure(f *recertificationClosureFixture) {
	root := f.readCriticRoot()
	delete(root, "closure")
	f.writeCriticRoot(root)
}

func TestRecertificationUsesClosureForNoOpMember(t *testing.T) {
	fixture := newFileRecertificationClosureFixture(t)
	certification, err := fixture.run.selectOriginalCertification()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(filepath.ToSlash(certification.reviewPath), "/implementation/rounds/2/review.json") ||
		!strings.HasSuffix(filepath.ToSlash(certification.patchPath), "/implementation/rounds/2/diff.patch") ||
		certification.reviewed != fixture.reviewedTree || certification.criticRoot != "critic" || certification.criticRound != 2 ||
		!bytes.Equal(certification.patch, fixture.patch) {
		t.Fatalf("no-op member certification = %+v", certification)
	}
	if err := verifyOriginalCritic(fixture.run, certification); err != nil {
		t.Fatalf("no-op member original critic verification failed: %v", err)
	}

	t.Run("conflicting duplicate review copies still refuse", func(t *testing.T) {
		duplicate := newFileRecertificationClosureFixture(t)
		writeRecertificationReview(t, duplicate.fixture, 3, "implementation-r2", duplicate.reviewedTree, duplicate.patch,
			map[string]any{"copy": "different review bytes"})
		if _, err := duplicate.run.selectOriginalCertification(); err == nil || !strings.Contains(err.Error(), "conflicting original review copies") {
			t.Fatalf("conflicting copy error = %v", err)
		}
	})
}

func TestSelectOriginalCertificationRejectsChangedClosureSelection(t *testing.T) {
	tests := []struct {
		name string
		want string
		edit func(*recertificationClosureFixture)
	}{
		{
			name: "newer critic member", want: "last critic round", edit: addNewerCriticMember,
		},
		{
			name: "changed closure patch digest", want: "no review joined to closure subject", edit: changeClosurePatchDigest,
		},
		{
			name: "wrong closure root", want: "does not match supplied root job", edit: changeClosureRoot,
		},
		{
			name: "missing modern clean closure", want: "clean subject-bearing fold", edit: removeRequiredClosure,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newFileRecertificationClosureFixture(t)
			_, err := fixture.run.selectOriginalCertification()
			if err != nil {
				t.Fatalf("valid setup did not select: %v", err)
			}
			test.edit(fixture)
			if _, err := fixture.run.selectOriginalCertification(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("selection error = %v, want %q", err, test.want)
			}
		})
	}
}

func assertVerifyRecertificationReviewSelectionRefusal(t *testing.T, want string, edit func(*recertificationClosureFixture)) {
	t.Helper()
	fixture, path, raw := newRawRecertificationClosure(t)
	verified, err := verifyRecertificationWithRaw(fixture.fixture.controller, "implementation", path, raw.answer, nil)
	if err != nil || verified.Record.RecordDigest != raw.record.RecordDigest || verified.RecordPath != path || !bytes.Equal(verified.Patch, raw.mergedPatch) {
		t.Fatalf("untampered recertification did not verify: %+v, %v", verified, err)
	}
	raw.consumed()
	edit(fixture)
	raw.resetForRefusal()
	_, err = verifyRecertificationWithRaw(fixture.fixture.controller, "implementation", path, raw.answer, nil)
	raw.consumed()
	var failure *RecertificationFailure
	if !errors.As(err, &failure) || failure.Reason != "chain-recertification-unproven" || failure.Detail != "review-selection" ||
		!strings.Contains(err.Error(), want) {
		t.Fatalf("verification error = %v, want chain-recertification-unproven detail=review-selection containing %q", err, want)
	}
}

func TestVerifyRecertificationRejectsStaleClosureRound(t *testing.T) {
	assertVerifyRecertificationReviewSelectionRefusal(t, "last critic round", addNewerCriticMember)
}

func TestVerifyRecertificationRejectsChangedClosure(t *testing.T) {
	assertVerifyRecertificationReviewSelectionRefusal(t, "no review joined to closure subject", changeClosurePatchDigest)
}

func TestVerifyRecertificationRejectsWrongClosureRoot(t *testing.T) {
	assertVerifyRecertificationReviewSelectionRefusal(t, "does not match supplied root job", changeClosureRoot)
}

func TestVerifyRecertificationRejectsMissingClosure(t *testing.T) {
	assertVerifyRecertificationReviewSelectionRefusal(t, "clean subject-bearing fold", removeRequiredClosure)
}
