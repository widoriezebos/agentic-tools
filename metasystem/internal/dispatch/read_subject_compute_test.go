package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

func initReadSubjectRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitReadSubject(t, repo, "init", "-q")
	if err := os.MkdirAll(filepath.Join(repo, "metasystem"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("artifacts/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "metasystem", "page.md"), []byte("design one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitReadSubject(t, repo, "add", ".")
	gitReadSubject(t, repo, "-c", "user.name=test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
	return repo
}

func gitReadSubject(t *testing.T, repo string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repo}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func writeImplementerSubjectRound(t *testing.T, repo, root, job string, round int, workspace, tree string, patch []byte) {
	t.Helper()
	var parent any
	if job != root {
		parent = root
	}
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), job+".json", map[string]any{
		"jobId": job, "role": "implementer", "round": round, "parentJob": parent,
		"status": "completed", "workspaceRoot": workspace,
	})
	roundDir := filepath.Join(repo, "artifacts", "agents", root, "rounds", strconv.Itoa(round))
	writeJSONFile(t, roundDir, "review.json", map[string]any{"reviewedTree": tree})
	if err := os.WriteFile(filepath.Join(roundDir, "diff.patch"), patch, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestComputeReadSubjectByKind(t *testing.T) {
	repo := initReadSubjectRepo(t)
	reviewedTree, err := (gittree.Workspace{Dir: repo}).Snapshot("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	patch := []byte("diff --git a/metasystem/page.md b/metasystem/page.md\n")
	writeImplementerSubjectRound(t, repo, "implementer", "implementer", 1, repo, reviewedTree, patch)

	live, present, err := ComputeReadSubject(ReadSubjectRequest{RepoRoot: repo, Role: "code-critic", Reviews: "implementer"})
	if err != nil || !present {
		t.Fatalf("live subject = %+v, present=%v, err=%v", live, present, err)
	}
	patchDigest := sha256.Sum256(patch)
	if live.Kind != SubjectLive || live.ImplementerRoot != "implementer" || live.ReviewedMember != "implementer" ||
		live.ReviewedProjectTree != reviewedTree || live.DiffDigest != hex.EncodeToString(patchDigest[:]) {
		t.Fatalf("live subject fields = %+v", live)
	}
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), "critic.json", map[string]any{
		"jobId": "critic", "role": "code-critic", "round": 1, "parentJob": nil, "reviews": "implementer",
	})
	fromRoot, present, err := ComputeReadSubject(ReadSubjectRequest{RepoRoot: repo, Role: "code-critic", RootJob: "critic"})
	if err != nil || !present || !fromRoot.Equal(live) {
		t.Fatalf("root-job subject = %+v, present=%v, err=%v; want %+v", fromRoot, present, err, live)
	}

	if err := os.Remove(filepath.Join(repo, "artifacts", "agents", "implementer", "rounds", "1", "review.json")); err != nil {
		t.Fatal(err)
	}
	if subject, present, err := ComputeReadSubject(ReadSubjectRequest{RepoRoot: repo, Role: "code-critic", Reviews: "implementer"}); err != nil || present || subject != (ReadSubject{}) {
		t.Fatalf("missing review subject = %+v, present=%v, err=%v", subject, present, err)
	}

	if err := os.WriteFile(filepath.Join(repo, "metasystem", "commit.txt"), []byte("commit subject\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitReadSubject(t, repo, "add", "metasystem/commit.txt")
	gitReadSubject(t, repo, "-c", "user.name=test", "-c", "user.email=test@example.invalid", "commit", "-qm", "subject")
	commit := gitReadSubject(t, repo, "rev-parse", "HEAD")
	commitSubject, present, err := ComputeReadSubject(ReadSubjectRequest{RepoRoot: repo, Role: "code-critic", Reviews: "commit:" + commit})
	if err != nil || !present {
		t.Fatalf("commit subject = %+v, present=%v, err=%v", commitSubject, present, err)
	}
	commitPatch, err := gitRawOutput(repo, "diff", "--binary", "--full-index", commit+"^", commit)
	if err != nil {
		t.Fatal(err)
	}
	commitDigest := sha256.Sum256(commitPatch)
	if commitSubject.Kind != SubjectCommit || commitSubject.Commit != commit ||
		commitSubject.Parent != gitReadSubject(t, repo, "rev-parse", commit+"^") ||
		commitSubject.Tree != gitReadSubject(t, repo, "rev-parse", commit+"^{tree}") ||
		commitSubject.DiffDigest != hex.EncodeToString(commitDigest[:]) {
		t.Fatalf("commit subject fields = %+v", commitSubject)
	}

	outputsFile := filepath.Join(t.TempDir(), "outputs.txt")
	outputs := []string{"metasystem/a.go", "metasystem/z.go"}
	if err := os.WriteFile(outputsFile, []byte(strings.Join(outputs, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	design, present, err := ComputeReadSubject(ReadSubjectRequest{
		RepoRoot: repo, Role: "design-critic", Workspace: repo,
		Design: "metasystem/page.md", DeclaredOutputs: outputsFile,
	})
	if err != nil || !present {
		t.Fatalf("design subject = %+v, present=%v, err=%v", design, present, err)
	}
	content := []byte("design one\n")
	contentDigest := sha256.Sum256(content)
	if design.Kind != SubjectDesign || design.DesignPath != "metasystem/page.md" ||
		design.ContentDigest != hex.EncodeToString(contentDigest[:]) ||
		design.DeclaredOutputsDigest != digestDeclaredOutputs(outputs) || design.ReviewedCommit != commit {
		t.Fatalf("design subject fields = %+v", design)
	}
	recordedDigest := digestDeclaredOutputs([]string{"metasystem/recorded.go"})
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), "design-root.json", map[string]any{
		"jobId": "design-root", "role": "design-critic", "round": 1, "parentJob": nil,
		"design": "metasystem/page.md", "declaredOutputsDigest": recordedDigest,
	})
	fromDesignRoot, present, err := ComputeReadSubject(ReadSubjectRequest{
		RepoRoot: repo, Role: "design-critic", RootJob: "design-root", Workspace: repo,
		Design: "outside.md", DeclaredOutputs: filepath.Join(repo, "missing-outputs.txt"),
	})
	if err != nil || !present || fromDesignRoot.DesignPath != "metasystem/page.md" || fromDesignRoot.DeclaredOutputsDigest != recordedDigest {
		t.Fatalf("recorded design subject = %+v, present=%v, err=%v", fromDesignRoot, present, err)
	}
}

func TestLiveSubjectFollowsTheChangeNotTheMember(t *testing.T) {
	repo := initReadSubjectRepo(t)
	tree, err := (gittree.Workspace{Dir: repo}).Snapshot("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	patch := []byte("same patch\n")
	writeImplementerSubjectRound(t, repo, "implementer", "implementer", 1, repo, tree, patch)
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), "critic.json", map[string]any{
		"jobId": "critic", "role": "code-critic", "round": 1, "parentJob": nil, "reviews": "implementer",
	})
	first, present, err := ComputeReadSubject(ReadSubjectRequest{RepoRoot: repo, Role: "code-critic", RootJob: "critic"})
	if err != nil || !present {
		t.Fatalf("root subject: present=%v err=%v", present, err)
	}
	writeImplementerSubjectRound(t, repo, "implementer", "implementer-r2", 2, repo, tree, patch)
	follow, present, err := ComputeReadSubject(ReadSubjectRequest{RepoRoot: repo, Role: "code-critic", RootJob: "critic"})
	if err != nil || !present {
		t.Fatalf("follow-up subject: present=%v err=%v", present, err)
	}
	if !first.Equal(follow) || first.Digest() != follow.Digest() || follow.ReviewedMember != "implementer-r2" {
		t.Fatalf("unchanged follow-up subjects differ: first=%+v follow=%+v", first, follow)
	}

	if err := os.WriteFile(filepath.Join(repo, "metasystem", "page.md"), []byte("changed design\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changedTree, err := (gittree.Workspace{Dir: repo}).Snapshot("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "implementer", "rounds", "2"), "review.json", map[string]any{"reviewedTree": changedTree})
	changed, present, err := ComputeReadSubject(ReadSubjectRequest{RepoRoot: repo, Role: "code-critic", RootJob: "critic"})
	if err != nil || !present {
		t.Fatalf("changed subject: present=%v err=%v", present, err)
	}
	if first.Equal(changed) || first.Digest() == changed.Digest() {
		t.Fatalf("changed tree did not change subject identity: first=%+v changed=%+v", first, changed)
	}
}

func TestCriticFollowUpUsesFinalImplementerWorkRound(t *testing.T) {
	repo := initReadSubjectRepo(t)
	firstTree, err := (gittree.Workspace{Dir: repo}).Snapshot("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	writeImplementerSubjectRound(t, repo, "implementer", "implementer", 1, repo, firstTree, []byte("round one\n"))
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), "critic.json", map[string]any{
		"jobId": "critic", "role": "code-critic", "round": 1, "parentJob": nil, "reviews": "implementer",
	})

	if err := os.WriteFile(filepath.Join(repo, "metasystem", "page.md"), []byte("round two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	finalTree, err := (gittree.Workspace{Dir: repo}).Snapshot("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	finalPatch := []byte("round two patch\n")
	writeImplementerSubjectRound(t, repo, "implementer", "implementer-r2", 2, repo, finalTree, finalPatch)

	subject, present, err := ComputeReadSubject(ReadSubjectRequest{RepoRoot: repo, Role: "code-critic", RootJob: "critic"})
	if err != nil || !present {
		t.Fatalf("critic follow-up subject: %+v present=%v err=%v", subject, present, err)
	}
	patchDigest := sha256.Sum256(finalPatch)
	if subject.ReviewedMember != "implementer-r2" || subject.ReviewedProjectTree != finalTree || subject.DiffDigest != hex.EncodeToString(patchDigest[:]) {
		t.Fatalf("critic follow-up did not select final work round: %+v", subject)
	}
}

func TestLiveSubjectWorkspaceMismatchRefuses(t *testing.T) {
	repo := initReadSubjectRepo(t)
	tree, err := (gittree.Workspace{Dir: repo}).Snapshot("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	writeImplementerSubjectRound(t, repo, "implementer", "implementer", 1, repo, tree, []byte("patch\n"))
	if _, present, err := ComputeReadSubject(ReadSubjectRequest{RepoRoot: repo, Role: "code-critic", Reviews: "implementer"}); err != nil || !present {
		t.Fatalf("matching workspace: present=%v err=%v", present, err)
	}

	if err := os.WriteFile(filepath.Join(repo, "metasystem", "page.md"), []byte("drifted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err = ComputeReadSubject(ReadSubjectRequest{RepoRoot: repo, Role: "code-critic", Reviews: "implementer"})
	assertSubjectMismatch(t, err)

	recordPath := filepath.Join(repo, "artifacts", "agents", "jobs", "implementer.json")
	record := readJSONFile(t, recordPath)
	record["workspaceRoot"] = filepath.Join(repo, "missing-worktree")
	if err := writeRecord(recordPath, record); err != nil {
		t.Fatal(err)
	}
	_, _, err = ComputeReadSubject(ReadSubjectRequest{RepoRoot: repo, Role: "code-critic", Reviews: "implementer"})
	assertSubjectMismatch(t, err)
}

func assertSubjectMismatch(t *testing.T, err error) {
	t.Helper()
	var op *OpError
	if !errors.As(err, &op) || op.Code != 11 || op.Reason != readSubjectMismatchRefusal {
		t.Fatalf("subject mismatch = %T %v", err, err)
	}
}

func assertNoClosure(t *testing.T, repo, root string) {
	t.Helper()
	record := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", root+".json"))
	if _, present := record[closureField]; present {
		t.Fatalf("critic root %s gained closure %v", root, record[closureField])
	}
}

func assertChainOpen(t *testing.T, repo, root string) {
	t.Helper()
	record := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", root+".json"))
	if closed, _ := record["chainClosed"].(bool); closed {
		t.Fatalf("critic root %s is closed", root)
	}
}

func assertChainClosed(t *testing.T, repo, root string) {
	t.Helper()
	record := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", root+".json"))
	if closed, _ := record["chainClosed"].(bool); !closed {
		t.Fatalf("critic root %s is not closed", root)
	}
}

func closeReadyCriticChain(t *testing.T, repo, root string, members ...string) string {
	t.Helper()
	agents := filepath.Join(repo, "artifacts", "agents")
	rootPath := filepath.Join(agents, "jobs", root+".json")
	record := readJSONFile(t, rootPath)
	record["destructiveReach"] = HazardDesignBearing
	record["configurationObligations"] = requiredConfigurationByHazard[HazardDesignBearing]
	record["dispatchMode"] = DispatchModeFresh
	record["resumedSessionId"] = nil
	record["sessionId"] = "critic-chain-session"
	record["endedAt"] = "2026-08-30T10:00:00Z"
	if err := writeRecord(rootPath, record); err != nil {
		t.Fatal(err)
	}
	snapshot := "artifacts/agents/capabilities/close-ready.json"
	writeJSONFile(t, filepath.Join(agents, "capabilities"), "close-ready.json", map[string]any{"ok": true})
	for _, member := range members {
		memberPath := filepath.Join(agents, "jobs", member+".json")
		memberRecord := readJSONFile(t, memberPath)
		memberRecord["capabilitySnapshot"] = snapshot
		if err := writeRecord(memberPath, memberRecord); err != nil {
			t.Fatal(err)
		}
	}

	evidence := t.TempDir()
	result := filepath.Join(t.TempDir(), "mirror.json")
	for _, member := range members {
		if err := Mirror(repo, repo, evidence, root, member, result); err != nil {
			t.Fatal(err)
		}
	}
	mirrored := readJSONFile(t, result)
	record = readJSONFile(t, rootPath)
	record["mirror"] = map[string]any{"path": asString(mirrored["path"]), "manifest": mirrored["manifest"]}
	if err := writeRecord(rootPath, record); err != nil {
		t.Fatal(err)
	}
	for _, member := range members {
		if err := Mirror(repo, repo, evidence, root, member, result); err != nil {
			t.Fatal(err)
		}
	}
	return evidence
}

func TestFoldRefusesUnboundReturn(t *testing.T) {
	t.Run("mismatch", func(t *testing.T) {
		repo := t.TempDir()
		writeCriticRound(t, repo, "critic", "critic", 1, []any{}, []any{})
		setCriticSubject(t, repo, "critic", "implementer", "tree-a")
		if err := WriteReadSubject(filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "1", "subject.json"), ReadSubject{
			Kind: SubjectLive, ImplementerRoot: "implementer", ReviewedMember: "implementer",
			ReviewedProjectTree: "tree-b", DiffDigest: "diff",
		}); err != nil {
			t.Fatal(err)
		}
		if outcome, err := CritiqueRegisterAdvance(repo, "critic", "critic"); err != nil || outcome != "advanced" {
			t.Fatalf("advance = %q, %v", outcome, err)
		}
		items := readRegister(t, repo, "critic")
		if len(items) != 1 {
			t.Fatalf("unbound return register = %v", items)
		}
		entry := items[0].(map[string]any)
		if entry["rigorClass"] != "unproven" || entry["findingId"] != syntheticUnboundFindingID("code-critic", "critic") {
			t.Fatalf("unbound return finding = %v", entry)
		}
		if _, err := CritiqueRegisterClose(repo, "critic"); err == nil || !strings.Contains(err.Error(), "blocks close") {
			t.Fatalf("unbound register close = %v", err)
		}
	})

	t.Run("matching", func(t *testing.T) {
		repo := t.TempDir()
		writeCriticRound(t, repo, "critic", "critic", 1, []any{}, []any{})
		setCriticSubject(t, repo, "critic", "implementer", "tree-a")
		if err := WriteReadSubject(filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "1", "subject.json"), ReadSubject{
			Kind: SubjectLive, ImplementerRoot: "implementer", ReviewedMember: "implementer",
			ReviewedProjectTree: "tree-a", DiffDigest: "diff",
		}); err != nil {
			t.Fatal(err)
		}
		if outcome, err := CritiqueRegisterAdvance(repo, "critic", "critic"); err != nil || outcome != "advanced" {
			t.Fatalf("advance = %q, %v", outcome, err)
		}
		if items := readRegister(t, repo, "critic"); len(items) != 0 {
			t.Fatalf("matching return gained findings: %v", items)
		}
	})
}

func TestCloseWritesOneClosure(t *testing.T) {
	for field, value := range map[string]any{
		closureField: encodeClosure(Closure{
			CriticRoot: "job-a", Round: 1, Mechanism: "clean",
			Subject: ReadSubject{Kind: SubjectLive, ImplementerRoot: "implementer", ReviewedProjectTree: "tree-a", DiffDigest: "diff"},
		}),
		findingRegisterSubjectDigestField: "injected-digest",
	} {
		t.Run("record-cas-refuses-"+field, func(t *testing.T) {
			ownerRepo := sandbox(t)
			createPending(t, ownerRepo, "job-a")
			setupPending(t, ownerRepo, "job-a")
			emptyPatch := writeJSON(t, filepath.Join(t.TempDir(), "running.json"), map[string]any{})
			if _, err := RecordCAS(ownerRepo, "job-a", "pending", "running", emptyPatch); err != nil {
				t.Fatal(err)
			}
			injection := writeJSON(t, filepath.Join(t.TempDir(), "terminal.json"), map[string]any{field: value})
			if _, err := RecordCAS(ownerRepo, "job-a", "running", "cancelled", injection); err == nil {
				t.Fatalf("generic terminal transition injected dedicated field %s", field)
			}
			record := readRecord(t, ownerRepo, "job-a")
			if record["status"] != "running" {
				t.Fatalf("refused injection changed status to %v", record["status"])
			}
			if _, present := record[field]; present {
				t.Fatalf("refused injection wrote %s", field)
			}
		})
	}

	repo := t.TempDir()
	writeCriticRound(t, repo, "critic", "critic", 1, []any{}, []any{})
	setCriticSubject(t, repo, "critic", "implementer", "tree-a")
	subjectPath := filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "1", "subject.json")
	subject := ReadSubject{Kind: SubjectLive, ImplementerRoot: "implementer", ReviewedMember: "implementer", ReviewedProjectTree: "tree-a", DiffDigest: "diff"}
	if err := WriteReadSubject(subjectPath, subject); err != nil {
		t.Fatal(err)
	}
	if _, err := CritiqueRegisterAdvance(repo, "critic", "critic"); err != nil {
		t.Fatal(err)
	}
	if outcome, err := CritiqueRegisterClose(repo, "critic"); err != nil || outcome != "closed" {
		t.Fatalf("register close = %q, %v", outcome, err)
	}
	assertNoClosure(t, repo, "critic")
	assertChainOpen(t, repo, "critic")
	closeReadyCriticChain(t, repo, "critic", "critic")
	if err := CritiqueChainClose(repo, "critic", false); err != nil {
		t.Fatalf("chain close = %v", err)
	}
	rootPath := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
	root := readJSONFile(t, rootPath)
	closure, present, err := ReadClosure(root)
	if err != nil || !present || closure.CriticRoot != "critic" || closure.Round != 1 || closure.Mechanism != "clean" || !closure.Subject.Equal(subject) {
		t.Fatalf("closure = %+v, present=%v, err=%v", closure, present, err)
	}
	before, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := CritiqueChainClose(repo, "critic", false); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("idempotent close rewrote the root record")
	}

	assertChainClosed(t, repo, "critic")

	differentRepo := t.TempDir()
	writeCriticRound(t, differentRepo, "different", "different", 1, []any{}, []any{})
	setCriticSubject(t, differentRepo, "different", "implementer", "tree-a")
	differentSubject := ReadSubject{Kind: SubjectLive, ImplementerRoot: "implementer", ReviewedMember: "implementer", ReviewedProjectTree: "tree-a", DiffDigest: "diff"}
	if err := WriteReadSubject(filepath.Join(differentRepo, "artifacts", "agents", "different", "rounds", "1", "subject.json"), differentSubject); err != nil {
		t.Fatal(err)
	}
	if _, err := CritiqueRegisterAdvance(differentRepo, "different", "different"); err != nil {
		t.Fatal(err)
	}
	differentRootPath := filepath.Join(differentRepo, "artifacts", "agents", "jobs", "different.json")
	differentRoot := readJSONFile(t, differentRootPath)
	differentSubject.ReviewedProjectTree = "tree-b"
	differentRoot[closureField] = encodeClosure(Closure{CriticRoot: "different", Round: 1, Subject: differentSubject, Mechanism: "clean"})
	if err := writeRecord(differentRootPath, differentRoot); err != nil {
		t.Fatal(err)
	}
	closeReadyCriticChain(t, differentRepo, "different", "different")
	if err := CritiqueChainClose(differentRepo, "different", false); err == nil || !strings.Contains(err.Error(), "closure") {
		t.Fatalf("different closure replacement = %v", err)
	}
	assertChainOpen(t, differentRepo, "different")

	mutatedRepo := t.TempDir()
	writeCriticRound(t, mutatedRepo, "mutated", "mutated", 1, []any{}, []any{})
	setCriticSubject(t, mutatedRepo, "mutated", "implementer", "tree-a")
	mutatedSubjectPath := filepath.Join(mutatedRepo, "artifacts", "agents", "mutated", "rounds", "1", "subject.json")
	mutatedSubject := ReadSubject{Kind: SubjectLive, ImplementerRoot: "implementer", ReviewedMember: "implementer", ReviewedProjectTree: "tree-a", DiffDigest: "diff"}
	if err := WriteReadSubject(mutatedSubjectPath, mutatedSubject); err != nil {
		t.Fatal(err)
	}
	if _, err := CritiqueRegisterAdvance(mutatedRepo, "mutated", "mutated"); err != nil {
		t.Fatal(err)
	}
	// DiffDigest is part of the subject identity but is not echoed in a
	// critic return. Closure must still reject changing only that field.
	mutatedSubject.DiffDigest = "changed-diff"
	if err := WriteReadSubject(mutatedSubjectPath, mutatedSubject); err != nil {
		t.Fatal(err)
	}
	closeReadyCriticChain(t, mutatedRepo, "mutated", "mutated")
	if err := CritiqueChainClose(mutatedRepo, "mutated", false); err == nil || !strings.Contains(err.Error(), "not the folded subject") {
		t.Fatalf("mutated subject close = %v", err)
	}
	mutatedRoot := readJSONFile(t, filepath.Join(mutatedRepo, "artifacts", "agents", "jobs", "mutated.json"))
	if _, present := mutatedRoot[closureField]; present {
		t.Fatalf("mutated subject gained closure %v", mutatedRoot[closureField])
	}

	cancelledRepo := t.TempDir()
	writeCriticRound(t, cancelledRepo, "cancelled", "cancelled", 1, []any{}, []any{})
	cancelledRootPath := filepath.Join(cancelledRepo, "artifacts", "agents", "jobs", "cancelled.json")
	cancelledRoot := readJSONFile(t, cancelledRootPath)
	cancelledRoot["status"] = "cancelled"
	if err := writeRecord(cancelledRootPath, cancelledRoot); err != nil {
		t.Fatal(err)
	}
	setCriticSubject(t, cancelledRepo, "cancelled", "implementer", "tree-a")
	if err := WriteReadSubject(filepath.Join(cancelledRepo, "artifacts", "agents", "cancelled", "rounds", "1", "subject.json"), ReadSubject{
		Kind: SubjectLive, ImplementerRoot: "implementer", ReviewedMember: "implementer", ReviewedProjectTree: "tree-a", DiffDigest: "diff",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := CritiqueRegisterAdvance(cancelledRepo, "cancelled", "cancelled"); err != nil {
		t.Fatal(err)
	}
	if outcome, err := CritiqueRegisterClose(cancelledRepo, "cancelled"); err != nil || outcome != "closed" {
		t.Fatalf("cancelled register close = %q, %v", outcome, err)
	}
	assertNoClosure(t, cancelledRepo, "cancelled")
	assertChainOpen(t, cancelledRepo, "cancelled")
	closeReadyCriticChain(t, cancelledRepo, "cancelled", "cancelled")
	if err := CritiqueChainClose(cancelledRepo, "cancelled", false); err != nil {
		t.Fatalf("cancelled chain close = %v", err)
	}
	assertNoClosure(t, cancelledRepo, "cancelled")
	assertChainClosed(t, cancelledRepo, "cancelled")

	blockedRepo := t.TempDir()
	writeCriticRound(t, blockedRepo, "blocked", "blocked", 1,
		[]any{registerFindingValue("F-1", true, "evidence")}, []any{registerRigor("F-1", "severe")})
	setCriticSubject(t, blockedRepo, "blocked", "implementer", "tree-a")
	if err := WriteReadSubject(filepath.Join(blockedRepo, "artifacts", "agents", "blocked", "rounds", "1", "subject.json"), ReadSubject{
		Kind: SubjectLive, ImplementerRoot: "implementer", ReviewedProjectTree: "tree-a", DiffDigest: "diff",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := CritiqueRegisterAdvance(blockedRepo, "blocked", "blocked"); err != nil {
		t.Fatal(err)
	}
	if _, err := CritiqueRegisterClose(blockedRepo, "blocked"); err == nil {
		t.Fatal("open severe finding did not block close")
	}
	blockedRoot := readJSONFile(t, filepath.Join(blockedRepo, "artifacts", "agents", "jobs", "blocked.json"))
	if _, present := blockedRoot[closureField]; present {
		t.Fatalf("blocked root carries closure %v", blockedRoot[closureField])
	}
}

func TestCleanClosureWaitsForLastCriticRound(t *testing.T) {
	repo := t.TempDir()
	writeCriticRound(t, repo, "critic", "critic", 1, []any{}, []any{})
	setCriticSubject(t, repo, "critic", "implementer", "tree-a")
	subject := ReadSubject{Kind: SubjectLive, ImplementerRoot: "implementer", ReviewedMember: "implementer", ReviewedProjectTree: "tree-a", DiffDigest: "diff"}
	if err := WriteReadSubject(filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "1", "subject.json"), subject); err != nil {
		t.Fatal(err)
	}
	if _, err := CritiqueRegisterAdvance(repo, "critic", "critic"); err != nil {
		t.Fatal(err)
	}

	writeCriticRound(t, repo, "critic", "critic-r2", 2, []any{}, []any{})
	roundTwoPath := filepath.Join(repo, "artifacts", "agents", "jobs", "critic-r2.json")
	roundTwo := readJSONFile(t, roundTwoPath)
	roundTwo["status"] = "running"
	if err := writeRecord(roundTwoPath, roundTwo); err != nil {
		t.Fatal(err)
	}
	if outcome, err := CritiqueRegisterClose(repo, "critic"); err != nil || outcome != "closed" {
		t.Fatalf("register close while round two runs = %q, %v", outcome, err)
	}
	assertNoClosure(t, repo, "critic")
	assertChainOpen(t, repo, "critic")

	roundTwo["status"] = "completed"
	if err := writeRecord(roundTwoPath, roundTwo); err != nil {
		t.Fatal(err)
	}
	returnPath := filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "2", "return.json")
	result := readJSONFile(t, returnPath)
	result["reviewedTree"] = "tree-a"
	if err := writeRecord(returnPath, result); err != nil {
		t.Fatal(err)
	}
	subject.ReviewedMember = "implementer-r2"
	if err := WriteReadSubject(filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "2", "subject.json"), subject); err != nil {
		t.Fatal(err)
	}
	if outcome, err := CritiqueRegisterAdvance(repo, "critic", "critic-r2"); err != nil || outcome != "advanced" {
		t.Fatalf("round two advance = %q, %v", outcome, err)
	}
	if outcome, err := CritiqueRegisterClose(repo, "critic"); err != nil || outcome != "closed" {
		t.Fatalf("round two register close = %q, %v", outcome, err)
	}
	assertNoClosure(t, repo, "critic")
	assertChainOpen(t, repo, "critic")
	closeReadyCriticChain(t, repo, "critic", "critic", "critic-r2")
	if err := CritiqueChainClose(repo, "critic", false); err != nil {
		t.Fatalf("round two chain close = %v", err)
	}
	closure, present, err := ReadClosure(readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")))
	if err != nil || !present || closure.Round != 2 || !closure.Subject.Equal(subject) {
		t.Fatalf("round two closure = %+v present=%v err=%v", closure, present, err)
	}
	assertChainClosed(t, repo, "critic")
}

func TestNonCleanFoldClosesWithoutCleanClosure(t *testing.T) {
	t.Run("failed", func(t *testing.T) {
		repo := t.TempDir()
		writeCriticRound(t, repo, "critic", "critic", 1, nil, nil)
		rootPath := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
		root := readJSONFile(t, rootPath)
		root["status"] = "failed"
		root["error"] = "runtime_error"
		root["phase"] = "delivery"
		if err := writeRecord(rootPath, root); err != nil {
			t.Fatal(err)
		}
		setCriticSubject(t, repo, "critic", "implementer", "tree-a")
		if err := WriteReadSubject(filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "1", "subject.json"), ReadSubject{
			Kind: SubjectLive, ImplementerRoot: "implementer", ReviewedMember: "implementer", ReviewedProjectTree: "tree-a", DiffDigest: "diff",
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := CritiqueRegisterAdvance(repo, "critic", "critic"); err != nil {
			t.Fatal(err)
		}
		if err := CritiqueRegisterAcceptRisk(repo, "critic", syntheticProtocolFindingID("code-critic", "critic"), "decision-op"); err != nil {
			t.Fatal(err)
		}
		if outcome, err := CritiqueRegisterClose(repo, "critic"); err != nil || outcome != "closed" {
			t.Fatalf("failed round register close = %q, %v", outcome, err)
		}
		assertNoClosure(t, repo, "critic")
		assertChainOpen(t, repo, "critic")
		closeReadyCriticChain(t, repo, "critic", "critic")
		if err := CritiqueChainClose(repo, "critic", false); err != nil {
			t.Fatalf("failed round chain close = %v", err)
		}
		assertNoClosure(t, repo, "critic")
		assertChainClosed(t, repo, "critic")
	})

	t.Run("unbound-return", func(t *testing.T) {
		repo := t.TempDir()
		writeCriticRound(t, repo, "critic", "critic", 1, []any{}, []any{})
		setCriticSubject(t, repo, "critic", "implementer", "tree-a")
		if err := WriteReadSubject(filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "1", "subject.json"), ReadSubject{
			Kind: SubjectLive, ImplementerRoot: "implementer", ReviewedMember: "implementer", ReviewedProjectTree: "tree-b", DiffDigest: "diff",
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := CritiqueRegisterAdvance(repo, "critic", "critic"); err != nil {
			t.Fatal(err)
		}
		if err := CritiqueRegisterAcceptRisk(repo, "critic", syntheticUnboundFindingID("code-critic", "critic"), "decision-op"); err != nil {
			t.Fatal(err)
		}
		if outcome, err := CritiqueRegisterClose(repo, "critic"); err != nil || outcome != "closed" {
			t.Fatalf("unbound round register close = %q, %v", outcome, err)
		}
		assertNoClosure(t, repo, "critic")
		assertChainOpen(t, repo, "critic")
		closeReadyCriticChain(t, repo, "critic", "critic")
		if err := CritiqueChainClose(repo, "critic", false); err != nil {
			t.Fatalf("unbound round chain close = %v", err)
		}
		assertNoClosure(t, repo, "critic")
		assertChainClosed(t, repo, "critic")
	})
}

func TestCleanClosureRequiresWithdrawnRegister(t *testing.T) {
	for _, tc := range []struct {
		name  string
		class string
		close func(*testing.T, string)
	}{
		{
			name: "deferred", class: "bounded",
			close: func(t *testing.T, repo string) {
				rootPath := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
				root := readJSONFile(t, rootPath)
				register := root[findingRegisterField].([]any)
				entry := register[0].(map[string]any)
				entry["status"] = "deferred"
				entry["resolution"] = "deferred"
				entry["decisionOpid"] = "defer-op"
				if err := writeRecord(rootPath, root); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "accepted-risk", class: "severe",
			close: func(t *testing.T, repo string) {
				if err := CritiqueRegisterAcceptRisk(repo, "critic", "F-1", "decision-op"); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			writeCriticRound(t, repo, "critic", "critic", 1,
				[]any{registerFindingValue("F-1", true, "evidence")}, []any{registerRigor("F-1", tc.class)})
			setCriticSubject(t, repo, "critic", "implementer", "tree-a")
			if err := WriteReadSubject(filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "1", "subject.json"), ReadSubject{
				Kind: SubjectLive, ImplementerRoot: "implementer", ReviewedMember: "implementer", ReviewedProjectTree: "tree-a", DiffDigest: "diff",
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := CritiqueRegisterAdvance(repo, "critic", "critic"); err != nil {
				t.Fatal(err)
			}
			tc.close(t, repo)
			if outcome, err := CritiqueRegisterClose(repo, "critic"); err != nil || outcome != "closed" {
				t.Fatalf("register close = %q, %v", outcome, err)
			}
			assertNoClosure(t, repo, "critic")
			assertChainOpen(t, repo, "critic")
			closeReadyCriticChain(t, repo, "critic", "critic")
			for attempt := 1; attempt <= 2; attempt++ {
				if err := CritiqueChainClose(repo, "critic", false); err != nil {
					t.Fatalf("chain close attempt %d = %v", attempt, err)
				}
				assertNoClosure(t, repo, "critic")
				assertChainClosed(t, repo, "critic")
			}
		})
	}
}

func TestDesignChainClosesAtRoundOne(t *testing.T) {
	repo := t.TempDir()
	writeCriticRound(t, repo, "design", "design", 1, []any{}, []any{})
	rootPath := filepath.Join(repo, "artifacts", "agents", "jobs", "design.json")
	root := readJSONFile(t, rootPath)
	root["role"] = "design-critic"
	root[reviewRoundLimitField] = 2
	root["declaredOutputs"] = []any{"metasystem/plans/x.md"}
	if err := writeRecord(rootPath, root); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(repo, "artifacts", "agents", "design", "rounds", "1", "return.json")
	result := readJSONFile(t, resultPath)
	result["reviewedCommit"] = "commit-c"
	if err := writeRecord(resultPath, result); err != nil {
		t.Fatal(err)
	}
	subject := ReadSubject{Kind: SubjectDesign, DesignPath: "metasystem/plans/design.md", ContentDigest: "content", DeclaredOutputsDigest: "outputs", ReviewedCommit: "commit-c"}
	if err := WriteReadSubject(filepath.Join(repo, "artifacts", "agents", "design", "rounds", "1", "subject.json"), subject); err != nil {
		t.Fatal(err)
	}
	if outcome, err := CritiqueRegisterAdvance(repo, "design", "design"); err != nil || outcome != "advanced" {
		t.Fatalf("advance = %q, %v", outcome, err)
	}
	if outcome, err := CritiqueRegisterClose(repo, "design"); err != nil || outcome != "closed" {
		t.Fatalf("register close = %q, %v", outcome, err)
	}
	assertNoClosure(t, repo, "design")
	assertChainOpen(t, repo, "design")
	closeReadyCriticChain(t, repo, "design", "design")
	if err := CritiqueChainClose(repo, "design", true); err != nil {
		t.Fatalf("chain close = %v", err)
	}
	closedRoot := readJSONFile(t, rootPath)
	closure, present, err := ReadClosure(closedRoot)
	if err != nil || !present || closure.Round != 1 || closure.Subject.Kind != SubjectDesign || !closure.Subject.Equal(subject) {
		t.Fatalf("design closure = %+v, present=%v, err=%v", closure, present, err)
	}
	if closedRoot["runnerClosed"] != true {
		t.Fatalf("design close did not mark runner closed: %v", closedRoot["runnerClosed"])
	}
	assertChainClosed(t, repo, "design")
}

func TestClosureFollowsChainClose(t *testing.T) {
	repo := t.TempDir()
	writeCriticRound(t, repo, "critic", "critic", 1, []any{}, []any{})
	setCriticSubject(t, repo, "critic", "implementer", "tree-a")
	subject := ReadSubject{Kind: SubjectLive, ImplementerRoot: "implementer", ReviewedMember: "implementer", ReviewedProjectTree: "tree-a", DiffDigest: "diff"}
	if err := WriteReadSubject(filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "1", "subject.json"), subject); err != nil {
		t.Fatal(err)
	}
	if _, err := CritiqueRegisterAdvance(repo, "critic", "critic"); err != nil {
		t.Fatal(err)
	}
	if outcome, err := CritiqueRegisterClose(repo, "critic"); err != nil || outcome != "closed" {
		t.Fatalf("round one register close = %q, %v", outcome, err)
	}
	assertNoClosure(t, repo, "critic")
	assertChainOpen(t, repo, "critic")
	if err := CritiqueChainClose(repo, "critic", false); err == nil || !strings.Contains(err.Error(), "cannot close an unmirrored chain") {
		t.Fatalf("unmirrored close = %v", err)
	}
	assertNoClosure(t, repo, "critic")
	assertChainOpen(t, repo, "critic")

	writeCriticRound(t, repo, "critic", "critic-r2", 2, []any{}, []any{})
	resultPath := filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "2", "return.json")
	result := readJSONFile(t, resultPath)
	result["reviewedTree"] = "tree-a"
	if err := writeRecord(resultPath, result); err != nil {
		t.Fatal(err)
	}
	subject.ReviewedMember = "implementer-r2"
	if err := WriteReadSubject(filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "2", "subject.json"), subject); err != nil {
		t.Fatal(err)
	}
	if _, err := CritiqueRegisterAdvance(repo, "critic", "critic-r2"); err != nil {
		t.Fatal(err)
	}
	if outcome, err := CritiqueRegisterClose(repo, "critic"); err != nil || outcome != "closed" {
		t.Fatalf("round two register close = %q, %v", outcome, err)
	}
	closeReadyCriticChain(t, repo, "critic", "critic", "critic-r2")
	if err := CritiqueChainClose(repo, "critic", false); err != nil {
		t.Fatalf("round two chain close = %v", err)
	}
	rootPath := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
	closure, present, err := ReadClosure(readJSONFile(t, rootPath))
	if err != nil || !present || closure.Round != 2 || !closure.Subject.Equal(subject) {
		t.Fatalf("round two closure = %+v, present=%v, err=%v", closure, present, err)
	}
	assertChainClosed(t, repo, "critic")
	before, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := CritiqueChainClose(repo, "critic", false); err != nil {
		t.Fatalf("repeat close = %v", err)
	}
	after, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("repeat close rewrote the root record")
	}
}

func TestCancelledRoundAfterRefusedClose(t *testing.T) {
	repo := t.TempDir()
	writeCriticRound(t, repo, "critic", "critic", 1, []any{}, []any{})
	setCriticSubject(t, repo, "critic", "implementer", "tree-a")
	subject := ReadSubject{Kind: SubjectLive, ImplementerRoot: "implementer", ReviewedMember: "implementer", ReviewedProjectTree: "tree-a", DiffDigest: "diff"}
	if err := WriteReadSubject(filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "1", "subject.json"), subject); err != nil {
		t.Fatal(err)
	}
	if _, err := CritiqueRegisterAdvance(repo, "critic", "critic"); err != nil {
		t.Fatal(err)
	}
	if outcome, err := CritiqueRegisterClose(repo, "critic"); err != nil || outcome != "closed" {
		t.Fatalf("round one register close = %q, %v", outcome, err)
	}
	if err := CritiqueChainClose(repo, "critic", false); err == nil || !strings.Contains(err.Error(), "cannot close an unmirrored chain") {
		t.Fatalf("unmirrored close = %v", err)
	}
	assertNoClosure(t, repo, "critic")
	assertChainOpen(t, repo, "critic")

	writeCriticRound(t, repo, "critic", "critic-r2", 2, []any{}, []any{})
	roundTwoPath := filepath.Join(repo, "artifacts", "agents", "jobs", "critic-r2.json")
	roundTwo := readJSONFile(t, roundTwoPath)
	roundTwo["status"] = "cancelled"
	if err := writeRecord(roundTwoPath, roundTwo); err != nil {
		t.Fatal(err)
	}
	if _, err := CritiqueRegisterAdvance(repo, "critic", "critic-r2"); err != nil {
		t.Fatal(err)
	}
	if outcome, err := CritiqueRegisterClose(repo, "critic"); err != nil || outcome != "closed" {
		t.Fatalf("cancelled register close = %q, %v", outcome, err)
	}
	closeReadyCriticChain(t, repo, "critic", "critic", "critic-r2")
	if err := CritiqueChainClose(repo, "critic", false); err != nil {
		t.Fatalf("cancelled chain close = %v", err)
	}
	assertNoClosure(t, repo, "critic")
	assertChainClosed(t, repo, "critic")
	rootPath := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
	before, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := CritiqueChainClose(repo, "critic", false); err != nil {
		t.Fatalf("repeat close = %v", err)
	}
	after, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("repeat close rewrote the root record")
	}
}

func TestRegisterCloseNeverWritesClosure(t *testing.T) {
	repo := t.TempDir()
	writeCriticRound(t, repo, "critic", "critic", 1, []any{}, []any{})
	setCriticSubject(t, repo, "critic", "implementer", "tree-a")
	if err := WriteReadSubject(filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "1", "subject.json"), ReadSubject{
		Kind: SubjectLive, ImplementerRoot: "implementer", ReviewedMember: "implementer", ReviewedProjectTree: "tree-a", DiffDigest: "diff",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := CritiqueRegisterAdvance(repo, "critic", "critic"); err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 3; attempt++ {
		if outcome, err := CritiqueRegisterClose(repo, "critic"); err != nil || outcome != "closed" {
			t.Fatalf("register close attempt %d = %q, %v", attempt, outcome, err)
		}
		assertNoClosure(t, repo, "critic")
		assertChainOpen(t, repo, "critic")
	}
}

func TestCleanClosureSkipsEditedReturn(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(map[string]any)
	}{
		{name: "job-id", edit: func(result map[string]any) { result["jobId"] = "other-critic" }},
		{name: "reviewed-tree", edit: func(result map[string]any) { result["reviewedTree"] = "tree-b" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			writeCriticRound(t, repo, "critic", "critic", 1, []any{}, []any{})
			setCriticSubject(t, repo, "critic", "implementer", "tree-a")
			if err := WriteReadSubject(filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "1", "subject.json"), ReadSubject{
				Kind: SubjectLive, ImplementerRoot: "implementer", ReviewedMember: "implementer", ReviewedProjectTree: "tree-a", DiffDigest: "diff",
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := CritiqueRegisterAdvance(repo, "critic", "critic"); err != nil {
				t.Fatal(err)
			}
			if outcome, err := CritiqueRegisterClose(repo, "critic"); err != nil || outcome != "closed" {
				t.Fatalf("register close = %q, %v", outcome, err)
			}
			resultPath := filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "1", "return.json")
			result := readJSONFile(t, resultPath)
			tc.edit(result)
			if err := writeRecord(resultPath, result); err != nil {
				t.Fatal(err)
			}
			closeReadyCriticChain(t, repo, "critic", "critic")
			if err := CritiqueChainClose(repo, "critic", false); err != nil {
				t.Fatalf("chain close = %v", err)
			}
			assertNoClosure(t, repo, "critic")
			assertChainClosed(t, repo, "critic")
		})
	}
}
