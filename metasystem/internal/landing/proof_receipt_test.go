package landing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func candidateProofAdmission(request proofrun.AdmissionRequest, tree string) proofrun.AdmissionRequest {
	request.CandidateGoalID = request.GoalID
	request.CandidateRevision = request.AccountingRevision
	request.CandidateBudgetEpoch = request.BudgetEpoch
	if request.Identity.CommandClass == "testing" {
		request.CandidateTree = tree
	}
	return proofrun.WithTestHostLoadSampler(request, "0")
}

func requireProofReservationNotAdmissionRefused(t *testing.T, decision proofrun.LaunchResult) {
	t.Helper()
	if decision.Disposition == proofrun.DispositionAdmissionRefused {
		t.Fatalf("proof reservation fixture was admission-refused: %+v", decision)
	}
}

func TestCanonicalReceiptProofContext(t *testing.T) {
	fixture := newFileOnlyCanonicalReceiptFixture(t)
	workspace := fixture.workspace()
	if fixture.receipt.Proof == nil || fixture.receipt.Proof.AttemptID != fixture.attempt.AttemptID ||
		fixture.receipt.Proof.ControlRoot != fixture.root || fixture.receipt.Proof.GoalID != "goal-a" ||
		fixture.receipt.Proof.AccountingRevision != 2 || fixture.receipt.Proof.Deadline != fixture.attempt.Deadline ||
		fixture.receipt.Coverage == nil {
		t.Fatalf("canonical receipt lost admitted proof or measured coverage context: %+v", fixture.receipt)
	}
	if _, err := publishCommittedReceiptAtWithWorkspace(fixture.root, fixture.attempt.AttemptID, fixture.tree, time.Now().UTC(), workspace); err == nil {
		t.Fatal("receipt projection was published before the atomic terminal-success commit")
	}
	fixture.commit(t)
	currentManifest, err := proofrun.FullDigest(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	if fixture.attempt.PendingCoverage == nil || fixture.attempt.PendingCoverage.Evidence == nil {
		t.Fatal("retained attempt has no completed coverage evidence")
	}
	if recorded := fixture.attempt.PendingCoverage.Evidence.InputManifest; recorded != currentManifest {
		t.Fatalf("retained coverage manifest=%q current=%q", recorded, currentManifest)
	}
	published, err := publishCommittedReceiptAtWithWorkspace(fixture.root, fixture.attempt.AttemptID, fixture.tree, time.Now().UTC(), workspace)
	if err != nil || published.Time != fixture.receipt.Time || !fullReceiptCommandAccepted(published) {
		t.Fatalf("canonical committed receipt was not accepted: receipt=%+v err=%v", published, err)
	}
	read, err := readTestReceiptWithWorkspace(ObserveParams{RepoRoot: fixture.root, CandidateTree: fixture.tree,
		TestReceipt: TestReceiptPath(fixture.root, fixture.tree)}, workspace)
	if err != nil || read.Proof == nil || read.Proof.AttemptID != fixture.attempt.AttemptID {
		t.Fatalf("normal receipt consumer rejected the retained proof: receipt=%+v err=%v", read, err)
	}
	selected := published
	selected.Command = CanonicalValidatorCommand + " selected-section"
	if fullReceiptCommandAccepted(selected) {
		t.Fatal("selected validator command qualified as canonical full proof")
	}
	withoutCoverage := published
	withoutCoverage.Coverage = nil
	if fullReceiptCommandAccepted(withoutCoverage) {
		t.Fatal("canonical command without producer coverage qualified as full proof")
	}
}

func TestCanonicalReceiptCoverageRefusesChangedParentProjectInput(t *testing.T) {
	t.Parallel()
	fixture := newCanonicalReceiptFixture(t)
	fixture.commit(t)
	if _, err := PublishCommittedReceipt(fixture.root, fixture.attempt.AttemptID, fixture.tree); err != nil {
		t.Fatal(err)
	}
	parentInput := filepath.Join(filepath.Dir(fixture.root), "development", "metasystem-design.md")
	if err := os.WriteFile(parentInput, []byte("changed parent project input\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readTestReceipt(ObserveParams{RepoRoot: fixture.root, CandidateTree: fixture.tree,
		TestReceipt: TestReceiptPath(fixture.root, fixture.tree)}); err == nil ||
		!strings.Contains(err.Error(), "coverage no longer matches current project inputs") {
		t.Fatalf("receipt-bound coverage accepted changed parent input: %v", err)
	}
}

func TestCanonicalReceiptCoverageRefusesChangedProjectInputFileOnly(t *testing.T) {
	fixture := newFileOnlyCanonicalReceiptFixture(t)
	fixture.commit(t)
	workspace := fixture.workspace()
	if _, err := publishCommittedReceiptAtWithWorkspace(fixture.root, fixture.attempt.AttemptID, fixture.tree, time.Now().UTC(), workspace); err != nil {
		t.Fatal(err)
	}
	projectInput := filepath.Join(fixture.root, "development", "metasystem-design.md")
	if err := os.WriteFile(projectInput, []byte("changed project input\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readTestReceiptWithWorkspace(ObserveParams{RepoRoot: fixture.root, CandidateTree: fixture.tree,
		TestReceipt: TestReceiptPath(fixture.root, fixture.tree)}, workspace); err == nil ||
		!strings.Contains(err.Error(), "coverage no longer matches current project inputs") {
		t.Fatalf("receipt-bound coverage accepted changed project input: %v", err)
	}
}

func TestReceiptPreparationCopiesUnreachableStagedTreeIntoStandaloneSnapshot(t *testing.T) {
	t.Parallel()
	f := newObserveFixture(t)
	f.write("staged-only.txt", "unreachable candidate\n")
	f.git("add", "staged-only.txt")
	tree, err := (gittree.Workspace{Dir: f.root}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(f.git("rev-list", "--objects", "--all"), "\n") {
		if fields := strings.Fields(line); len(fields) > 0 && fields[0] == tree {
			t.Fatalf("staged tree %s unexpectedly reachable from a source ref", tree)
		}
	}

	preparation, err := PrepareTestReceipt(f.root, tree, "true")
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			_ = preparation.Close()
		}
	})
	if preparation.AcceptedIndexTree() != tree {
		t.Fatalf("accepted index = %s, want %s", preparation.AcceptedIndexTree(), tree)
	}

	sourceProject := filepath.Dir(f.root)
	unavailableSource := sourceProject + "-unavailable"
	if err := os.Rename(sourceProject, unavailableSource); err != nil {
		t.Fatal(err)
	}
	restored := false
	defer func() {
		if !restored {
			_ = os.Rename(unavailableSource, sourceProject)
		}
	}()
	candidate := gittree.Workspace{Dir: preparation.ExecutionRoot()}
	if got, err := candidate.StagedTree(); err != nil || got != tree {
		t.Fatalf("standalone candidate index = %s, %v; want %s", got, err, tree)
	}
	command := exec.Command("git", "-C", preparation.ExecutionRoot(), "fsck", "--connectivity-only", "--no-dangling", "HEAD", tree)
	command.Env = gittree.ScrubbedEnviron()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("standalone candidate lost object connectivity: %v\n%s", err, output)
	}
	if err := os.Rename(unavailableSource, sourceProject); err != nil {
		t.Fatal(err)
	}
	restored = true
	if err := preparation.Close(); err != nil {
		t.Fatal(err)
	}
	closed = true
}

func TestReceiptPublicationRecovery(t *testing.T) {
	fixture := newFileOnlyCanonicalReceiptFixture(t)
	fixture.commit(t)
	workspace := fixture.workspace()
	landingDir := filepath.Dir(filepath.Dir(TestReceiptPath(fixture.root, fixture.tree)))
	if err := os.MkdirAll(landingDir, 0o700); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(landingDir, "receipts")
	if err := os.WriteFile(blocker, []byte("projection blocked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := publishCommittedReceiptAtWithWorkspace(fixture.root, fixture.attempt.AttemptID, fixture.tree, time.Now().UTC(), workspace); err == nil {
		t.Fatal("projection write fault unexpectedly reported success")
	}
	retained, err := proofrun.ReadAttempt(fixture.root, fixture.attempt.AttemptID)
	if err != nil || retained.Terminal == nil || retained.Terminal.Result != proofrun.TerminalSuccess || len(retained.DeliveryReceipt) == 0 {
		t.Fatalf("projection fault lost atomic receipt payload: attempt=%+v err=%v", retained, err)
	}
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	recovered, err := publishCommittedReceiptAtWithWorkspace(fixture.root, fixture.attempt.AttemptID, fixture.tree, time.Now().UTC(), workspace)
	if err != nil || recovered.Time != fixture.receipt.Time {
		t.Fatalf("retained payload was not recoverable without execution: receipt=%+v err=%v", recovered, err)
	}
	projected, err := os.ReadFile(TestReceiptPath(fixture.root, fixture.tree))
	if err != nil || !bytes.Equal(bytes.TrimSpace(projected), bytes.TrimSpace(retained.DeliveryReceipt)) {
		t.Fatalf("recovered projection differs from atomic payload: err=%v", err)
	}
	attempts, err := proofrun.ReadAttempts(fixture.root)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("receipt recovery created %d proof attempts: %v", len(attempts), err)
	}
}

func TestLegacyReceiptPreservesNestedSuiteFailureEvidenceBeforeCandidateCleanup(t *testing.T) {
	for _, success := range []bool{true, false} {
		name := "failed enclosing receipt"
		if success {
			name = "successful enclosing receipt"
		}
		t.Run(name, func(t *testing.T) {
			f := newObserveFixture(t)
			f.write("metasystem.conf", "suite.evidence-copy-timeout-sec=5\nsuite.evidence-copy-max-mb=1\n")
			f.git("add", ".")
			tree, err := (gittree.Workspace{Dir: f.root}).StagedTree()
			if err != nil {
				t.Fatal(err)
			}
			preparation, err := PrepareTestReceipt(f.root, tree, "true")
			if err != nil {
				t.Fatal(err)
			}
			candidate := preparation.ExecutionRoot()
			failure := filepath.Join(candidate, "artifacts", "agents", "suite-failures", "nested")
			if err := os.MkdirAll(failure, 0o700); err != nil {
				t.Fatal(err)
			}
			for file, data := range map[string]string{"marker.txt": "marker-exact\n", "pid": "pid-exact\n", "birth": "birth-exact\n"} {
				if err := os.WriteFile(filepath.Join(failure, file), []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if success {
				if _, err := preparation.Complete(proofrun.Attempt{}, time.Now()); err != nil {
					t.Fatal(err)
				}
			} else if err := preparation.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(candidate); !os.IsNotExist(err) {
				t.Fatalf("real detached candidate still exists after preservation: %s (%v)", candidate, err)
			}
			preserved := map[string]string{}
			err = filepath.WalkDir(filepath.Join(f.root, "artifacts", "agents", "suite-failures"), func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.Type().IsRegular() {
					data, readErr := os.ReadFile(path)
					if readErr != nil {
						return readErr
					}
					preserved[entry.Name()] = string(data)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			for file, want := range map[string]string{"marker.txt": "marker-exact\n", "pid": "pid-exact\n", "birth": "birth-exact\n"} {
				if preserved[file] != want {
					t.Fatalf("preserved %s=%q, want %q", file, preserved[file], want)
				}
			}
		})
	}
}

func TestReceiptPreparationSurvivesCanonicalRecordMotion(t *testing.T) {
	const path = "records/steward/narration.txt"
	content := []byte("record motion\n")
	f := newFileOnlyCanonicalReceiptFixtureWithBeforeComplete(t, func(f *fileOnlyCanonicalReceiptFixture) {
		full := filepath.Join(f.root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(full, 0o644); err != nil {
			t.Fatal(err)
		}
	})
	originalTree, originalTime := f.tree, f.receipt.Time
	f.files[path] = content
	f.tree = receiptFactID(f.files)
	f.checkFiles(f.root)
	f.commit(t)
	if _, err := publishCommittedReceiptAtWithWorkspace(f.root, f.attempt.AttemptID, originalTree, time.Now().UTC(), f.workspace()); err == nil {
		t.Fatal("ordinary publication ignored the moved live candidate posture")
	}
	if err := os.Remove(filepath.Join(f.root, path)); err != nil {
		t.Fatal(err)
	}
	delete(f.files, path)
	f.tree = receiptFactID(f.files)
	if f.tree != originalTree {
		t.Fatalf("restored candidate tree = %s, want %s", f.tree, originalTree)
	}
	f.checkFiles(f.root)
	recovered, err := publishCommittedReceiptAtWithWorkspace(f.root, f.attempt.AttemptID, originalTree, time.Now().UTC(), f.workspace())
	if err != nil || recovered.Time != originalTime {
		t.Fatalf("exact prepared receipt was not recoverable after ordinary posture returned: receipt=%+v err=%v", recovered, err)
	}
	attempts, err := proofrun.ReadAttempts(f.root)
	if err != nil || len(attempts) != 1 || attempts[0].AttemptID != f.attempt.AttemptID {
		t.Fatalf("record motion changed proof attempts: attempts=%+v err=%v", attempts, err)
	}
}

type canonicalReceiptFixture struct {
	root, tree string
	attempt    proofrun.Attempt
	receipt    TestReceipt
	committed  bool
}

func newCanonicalReceiptFixture(t *testing.T) *canonicalReceiptFixture {
	t.Helper()
	f := newObserveFixture(t)
	f.write("metasystem.conf", "metasystem.runtimes=fake\ndispatch.cap-min=5\ndispatch.cap-max=120\n")
	f.write("internal/proofrun/stub.go", "package proofrun\n")
	for _, name := range []string{"coverage-ratchet.json", "coverage-ratchet-linux.json"} {
		f.write(filepath.Join("scripts", "agents", name), `{"floors":{"internal/proofrun":1},"exempt":{}}`)
	}
	f.git("add", ".")
	tree, err := (gittree.Workspace{Dir: f.root}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	preparation, err := PrepareTestReceipt(f.root, tree, CanonicalValidatorCommand)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = preparation.Close() })
	executionRoot := preparation.ExecutionRoot()
	parentInput := filepath.Join(filepath.Dir(executionRoot), "development", "metasystem-design.md")
	if data, err := os.ReadFile(parentInput); err != nil || string(data) != "fixture\n" {
		t.Fatalf("actual receipt preparation lost untracked parent input before admission: bytes=%q err=%v", data, err)
	}
	identity, err := proofrun.BuildProofIdentity(executionRoot, filepath.Join(executionRoot, "metasystem.conf"),
		"full", "landing-test-receipt", []string{"gate"}, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	request := candidateProofAdmission(proofrun.AdmissionRequest{ControlRoot: f.root,
		ExecutionRoot: executionRoot, GoalID: "goal-a", GoalRevision: 2, AccountingRevision: 2,
		ReservedMinutes: 5, Identity: identity, Launcher: launcher, Now: now}, tree)
	request = proofrun.WithTestHostAdmissionDirectory(request, filepath.Join(t.TempDir(), "host-admission"))
	attempt, decision, err := proofrun.ReserveLocked(request)
	if err != nil {
		t.Fatal(err)
	}
	requireProofReservationNotAdmissionRefused(t, decision)
	baselineName := "coverage-ratchet.json"
	if runtime.GOOS == "linux" {
		baselineName = "coverage-ratchet-linux.json"
	}
	begin := proofrun.CoverageBeginOptions{ControlRoot: f.root, ExecutionRoot: executionRoot,
		AttemptID: attempt.AttemptID, BaselinePath: filepath.Join(executionRoot, "scripts", "agents", baselineName),
		ProducerClass: "full", ProducerPID: int64(os.Getpid()), CallerPID: int64(os.Getpid())}
	if err := proofrun.BeginCoverage(begin); err != nil {
		t.Fatal(err)
	}
	evidenceDir := t.TempDir()
	coverageLog := filepath.Join(evidenceDir, "coverage.log")
	packages := filepath.Join(evidenceDir, "packages.txt")
	const module = "example.invalid/metasystem/"
	if err := os.WriteFile(coverageLog, []byte("ok  "+module+"internal/proofrun 0.1s coverage: 85.0% of statements\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packages, []byte(module+"internal/proofrun\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := proofrun.CompleteCoverage(proofrun.CoverageCompleteOptions{CoverageBeginOptions: begin,
		CoverageLog: coverageLog, PackageInventory: packages, ModulePrefix: module}); err != nil {
		t.Fatal(err)
	}
	current, err := proofrun.ReadAttempt(f.root, attempt.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := preparation.Complete(current, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	return &canonicalReceiptFixture{root: f.root, tree: tree, attempt: current, receipt: receipt}
}

func (fixture *canonicalReceiptFixture) commit(t *testing.T) {
	t.Helper()
	if fixture.committed {
		return
	}
	payload, err := json.Marshal(fixture.receipt)
	if err != nil {
		t.Fatal(err)
	}
	ended, err := time.Parse(time.RFC3339Nano, fixture.receipt.Time)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := proofrun.FinalizeAttempt(fixture.root, fixture.attempt.AttemptID, proofrun.TerminalSuccess, 0,
		"canonical receipt complete", payload, ended)
	if err != nil {
		t.Fatal(err)
	}
	fixture.attempt = committed
	fixture.committed = true
}

type fileOnlyCanonicalReceiptFixture struct {
	canonicalReceiptFixture
	t                  *testing.T
	files              map[string][]byte
	frozen             string
	acceptedTree       string
	acceptedProjection string
	indices            map[string]receiptPrivateIndex
}

func newFileOnlyCanonicalReceiptFixture(t *testing.T) *fileOnlyCanonicalReceiptFixture {
	t.Helper()
	return newFileOnlyCanonicalReceiptFixtureWithBeforeComplete(t, nil)
}

func newFileOnlyCanonicalReceiptFixtureWithBeforeComplete(t *testing.T, beforeComplete func(*fileOnlyCanonicalReceiptFixture)) *fileOnlyCanonicalReceiptFixture {
	t.Helper()
	f := &fileOnlyCanonicalReceiptFixture{t: t, files: map[string][]byte{}, indices: map[string]receiptPrivateIndex{}}
	f.root = t.TempDir()
	f.write(".gitignore", "artifacts/\n")
	f.write("metasystem.conf", "metasystem.runtimes=fake\ndispatch.cap-min=5\ndispatch.cap-max=120\n")
	f.write("development/metasystem-design.md", "fixture\n")
	f.write("internal/proofrun/stub.go", "package proofrun\n")
	for _, name := range []string{"coverage-ratchet.json", "coverage-ratchet-linux.json"} {
		f.write(filepath.Join("scripts", "agents", name), `{"floors":{"internal/proofrun":1},"exempt":{}}`)
	}
	for _, name := range []string{"path-classes.txt", "landing-classes.json"} {
		content, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", name))
		if err != nil {
			t.Fatal(err)
		}
		f.write(filepath.Join("scripts", "agents", name), string(content))
	}
	f.tree = receiptFactID(f.files)
	f.acceptedTree = f.tree
	f.acceptedProjection = receiptFactID(receiptWithoutRegisters(f.files))
	frozen, err := proofrun.Freeze(f.root)
	if err != nil {
		t.Fatal(err)
	}
	f.frozen = frozen.Root
	t.Cleanup(func() { _ = frozen.Close() })
	f.checkFiles(f.frozen)
	if got, err := os.ReadFile(filepath.Join(f.frozen, "development", "metasystem-design.md")); err != nil || string(got) != "fixture\n" {
		t.Fatalf("frozen export lost project input: %q, %v", got, err)
	}
	candidate := gittree.Workspace{Dir: frozen.Root, RawSource: f.strictRaw}
	indexBefore, worktreeBefore, err := receiptPosture(candidate)
	if err != nil || indexBefore != f.tree || worktreeBefore != f.acceptedProjection {
		t.Fatalf("file-only candidate posture: index=%s worktree=%s err=%v", indexBefore, worktreeBefore, err)
	}
	preparation := &ReceiptPreparation{root: f.root, tree: f.tree, command: CanonicalValidatorCommand,
		frozen: &frozen, candidate: candidate, identity: worktreeBefore,
		indexBefore: indexBefore, worktreeBefore: worktreeBefore}
	preparation.evidenceTimeout, preparation.evidenceMax = receiptEvidenceLimits(f.root)
	t.Cleanup(func() { _ = preparation.Close() })
	identity, err := proofrun.BuildProofIdentity(frozen.Root, filepath.Join(frozen.Root, "metasystem.conf"),
		"full", "landing-test-receipt", []string{"gate"}, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	request := candidateProofAdmission(proofrun.AdmissionRequest{ControlRoot: f.root,
		ExecutionRoot: frozen.Root, GoalID: "goal-a", GoalRevision: 2, AccountingRevision: 2,
		ReservedMinutes: 5, Identity: identity, Launcher: launcher, Now: now}, f.tree)
	request = proofrun.WithTestHostAdmissionDirectory(request, filepath.Join(t.TempDir(), "host-admission"))
	attempt, decision, err := proofrun.ReserveLocked(request)
	if err != nil {
		t.Fatal(err)
	}
	requireProofReservationNotAdmissionRefused(t, decision)
	baselineName := "coverage-ratchet.json"
	if runtime.GOOS == "linux" {
		baselineName = "coverage-ratchet-linux.json"
	}
	begin := proofrun.CoverageBeginOptions{ControlRoot: f.root, ExecutionRoot: frozen.Root,
		AttemptID: attempt.AttemptID, BaselinePath: filepath.Join(frozen.Root, "scripts", "agents", baselineName),
		ProducerClass: "full", ProducerPID: int64(os.Getpid()), CallerPID: int64(os.Getpid())}
	if err := proofrun.BeginCoverage(begin); err != nil {
		t.Fatal(err)
	}
	evidenceDir := t.TempDir()
	coverageLog := filepath.Join(evidenceDir, "coverage.log")
	packages := filepath.Join(evidenceDir, "packages.txt")
	const module = "example.invalid/metasystem/"
	if err := os.WriteFile(coverageLog, []byte("ok  "+module+"internal/proofrun 0.1s coverage: 85.0% of statements\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packages, []byte(module+"internal/proofrun\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := proofrun.CompleteCoverage(proofrun.CoverageCompleteOptions{CoverageBeginOptions: begin,
		CoverageLog: coverageLog, PackageInventory: packages, ModulePrefix: module}); err != nil {
		t.Fatal(err)
	}
	current, err := proofrun.ReadAttempt(f.root, attempt.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	if beforeComplete != nil {
		beforeComplete(f)
	}
	f.receipt, err = preparation.Complete(current, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(f.frozen); !os.IsNotExist(err) {
		t.Fatalf("completed preparation retained frozen candidate: %v", err)
	}
	f.attempt = current
	return f
}

func (f *fileOnlyCanonicalReceiptFixture) write(path, content string) {
	f.t.Helper()
	full := filepath.Join(f.root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Chmod(full, 0o644); err != nil {
		f.t.Fatal(err)
	}
	f.files[filepath.ToSlash(path)] = []byte(content)
}

func (f *fileOnlyCanonicalReceiptFixture) workspace() gittree.Workspace {
	return gittree.Workspace{Dir: f.root, RawSource: f.strictRaw}
}

func (f *fileOnlyCanonicalReceiptFixture) checkFiles(root string) {
	f.t.Helper()
	for path, want := range f.files {
		full := filepath.Join(root, filepath.FromSlash(path))
		got, err := os.ReadFile(full)
		if err != nil || !bytes.Equal(got, want) {
			f.t.Fatalf("snapshot file %s = %q, %v; want %q", path, got, err, want)
		}
		info, err := os.Lstat(full)
		if err != nil || !info.Mode().IsRegular() {
			f.t.Fatalf("snapshot mode %s = %v, %v; want regular file", path, info, err)
		}
		mode := info.Mode().Perm()
		if root == f.frozen {
			if mode&^os.FileMode(0o644) != 0 || mode&0o600 != 0o600 {
				f.t.Fatalf("frozen snapshot mode %s = %o; want umask-restricted 0644", path, mode)
			}
		} else if mode != 0o644 {
			f.t.Fatalf("source snapshot mode %s = %o; want 0644", path, mode)
		}
	}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "artifacts" && entry.IsDir() {
			return filepath.SkipDir // .gitignore excludes the proof ledger from Git snapshots.
		}
		if !entry.IsDir() {
			if _, declared := f.files[rel]; !declared {
				return fmt.Errorf("undeclared snapshot path %s", rel)
			}
		}
		return nil
	}); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fileOnlyCanonicalReceiptFixture) entries() []byte {
	paths := make([]string, 0, len(f.files))
	for path := range f.files {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	var lines bytes.Buffer
	for _, path := range paths {
		fmt.Fprintf(&lines, "100644 %s 0\t%s\x00", chainBlobOID(f.files[path]), path)
	}
	return lines.Bytes()
}

func (f *fileOnlyCanonicalReceiptFixture) strictRaw(request gittree.RawRequest) gittree.RawResult {
	f.t.Helper()
	if request.Dir != f.root && request.Dir != f.frozen {
		f.t.Fatalf("raw Git cwd = %q", request.Dir)
	}
	pins := []string{"-c", "core.fileMode=true", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false",
		"-c", "apply.ignoreWhitespace=no", "-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false",
		"-c", "gc.auto=0", "-c", "maintenance.auto=false"}
	prefix := append([]string{"-C", request.Dir}, pins...)
	if len(request.Args) < len(prefix) || !slices.Equal(request.Args[:len(prefix)], prefix) {
		f.t.Fatalf("raw Git pins/cwd = %q, want prefix %q", request.Args, prefix)
	}
	args := request.Args[len(prefix):]
	if request.Operation != "git "+strings.Join(args, " ") {
		f.t.Fatalf("raw Git operation = %q, args = %q", request.Operation, args)
	}
	private := ""
	for _, entry := range request.Env {
		if strings.HasPrefix(entry, "GIT_INDEX_FILE=") {
			if private != "" {
				f.t.Fatal("multiple private index paths")
			}
			private = strings.TrimPrefix(entry, "GIT_INDEX_FILE=")
		}
	}
	wantEnv := gittree.ScrubbedEnviron()
	if private != "" {
		if !filepath.IsAbs(private) || filepath.Base(private) != "index" {
			f.t.Fatalf("private index path = %q", private)
		}
		wantEnv = gittree.ScrubbedEnviron("GIT_INDEX_FILE=" + private)
	}
	if !reflect.DeepEqual(request.Env, wantEnv) {
		f.t.Fatal("raw Git environment differs from scrubbed environment and private index")
	}
	if request.Stdin != nil && !(private != "" && slices.Equal(args, []string{"update-index", "-z", "--index-info"})) {
		f.t.Fatalf("unexpected raw stdin for %q", args)
	}
	answer := func(id string) gittree.RawResult { return gittree.RawResult{Stdout: []byte(id + "\n")} }
	state, known := f.indices[private]
	switch {
	case private == "" && slices.Equal(args, []string{"rev-parse", "--show-prefix"}):
		return answer("")
	case private == "" && slices.Equal(args, []string{"rev-parse", "--show-toplevel"}):
		return answer(request.Dir)
	case private == "" && slices.Equal(args, []string{"ls-files", "--stage", "-z"}):
		return gittree.RawResult{Stdout: f.entries()}
	case private != "" && (!known || state.phase == "done") && slices.Equal(args, []string{"read-tree", "--empty"}):
		f.indices[private] = receiptPrivateIndex{kind: "staged", phase: "seeded"}
		return gittree.RawResult{}
	case private != "" && (!known || state.phase == "done") && slices.Equal(args, []string{"read-tree", "HEAD"}):
		f.indices[private] = receiptPrivateIndex{kind: "snapshot", phase: "seeded"}
		return gittree.RawResult{}
	case private != "" && (!known || state.phase == "done") && len(args) == 2 && args[0] == "read-tree" &&
		(args[1] == f.tree || args[1] == f.acceptedTree):
		f.indices[private] = receiptPrivateIndex{kind: "filter", phase: "seeded", seed: args[1]}
		return gittree.RawResult{}
	case private != "" && known && state.kind == "staged" && state.phase == "seeded" &&
		slices.Equal(args, []string{"update-index", "-z", "--index-info"}) && bytes.Equal(request.Stdin, f.entries()):
		state.phase = "ready"
	case private != "" && known && state.kind == "snapshot" && state.phase == "seeded" &&
		slices.Equal(args, []string{"add", "-A", "--", "."}):
		f.checkFiles(request.Dir)
		state.phase = "ready"
	case private != "" && known && state.kind == "filter" && state.phase == "seeded" &&
		slices.Equal(args, append([]string{"update-index", "--force-remove", "--"}, appendOnlyRegisters...)):
		state.phase = "ready"
	case private != "" && known && state.phase == "ready" && slices.Equal(args, []string{"write-tree"}):
		state.phase = "done"
		f.indices[private] = state
		switch state.kind {
		case "staged":
			return answer(f.tree)
		case "snapshot":
			f.checkFiles(request.Dir)
			return answer(f.tree)
		case "filter":
			if state.seed == f.acceptedTree {
				return answer(f.acceptedProjection)
			}
			return answer(receiptFactID(receiptWithoutRegisters(f.files)))
		}
	default:
		f.t.Fatalf("undeclared raw receipt request: cwd=%q args=%q stdin=%q index=%+v", request.Dir, args, request.Stdin, state)
	}
	f.indices[private] = state
	return gittree.RawResult{}
}
