package landing

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func TestCanonicalReceiptProofContext(t *testing.T) {
	fixture := newCanonicalReceiptFixture(t)
	if fixture.receipt.Proof == nil || fixture.receipt.Proof.AttemptID != fixture.attempt.AttemptID ||
		fixture.receipt.Proof.ControlRoot != fixture.root || fixture.receipt.Proof.GoalID != "goal-a" ||
		fixture.receipt.Proof.AccountingRevision != 2 || fixture.receipt.Proof.Deadline != fixture.attempt.Deadline ||
		fixture.receipt.Coverage == nil {
		t.Fatalf("canonical receipt lost admitted proof or measured coverage context: %+v", fixture.receipt)
	}
	if _, err := PublishCommittedReceipt(fixture.root, fixture.attempt.AttemptID); err == nil {
		t.Fatal("receipt projection was published before the atomic terminal-success commit")
	}
	fixture.commit(t)
	published, err := PublishCommittedReceipt(fixture.root, fixture.attempt.AttemptID)
	if err != nil || published.Time != fixture.receipt.Time || !fullReceiptCommandAccepted(published) {
		t.Fatalf("canonical committed receipt was not accepted: receipt=%+v err=%v", published, err)
	}
	read, err := readTestReceipt(ObserveParams{RepoRoot: fixture.root, CandidateTree: fixture.tree,
		TestReceipt: TestReceiptPath(fixture.root, fixture.tree)})
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

func TestReceiptPublicationRecovery(t *testing.T) {
	fixture := newCanonicalReceiptFixture(t)
	fixture.commit(t)
	landingDir := filepath.Dir(filepath.Dir(TestReceiptPath(fixture.root, fixture.tree)))
	if err := os.MkdirAll(landingDir, 0o700); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(landingDir, "receipts")
	if err := os.WriteFile(blocker, []byte("projection blocked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := PublishCommittedReceipt(fixture.root, fixture.attempt.AttemptID); err == nil {
		t.Fatal("projection write fault unexpectedly reported success")
	}
	retained, err := proofrun.ReadAttempt(fixture.root, fixture.attempt.AttemptID)
	if err != nil || retained.Terminal == nil || retained.Terminal.Result != proofrun.TerminalSuccess || len(retained.DeliveryReceipt) == 0 {
		t.Fatalf("projection fault lost atomic receipt payload: attempt=%+v err=%v", retained, err)
	}
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	recovered, err := PublishCommittedReceipt(fixture.root, fixture.attempt.AttemptID)
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
	f := newObserveFixture(t)
	f.write("metasystem.conf", "dispatch.cap-max=120\n")
	f.write("internal/proofrun/stub.go", "package proofrun\n")
	for _, name := range []string{"coverage-ratchet.json", "coverage-ratchet-linux.json"} {
		f.write(filepath.Join("scripts", "agents", name), `{"floors":{"internal/proofrun":1},"exempt":{}}`)
	}
	f.git("add", ".")
	tree, err := (gittree.Workspace{Dir: f.root}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	preparation, err := PrepareTestReceipt(f.root, tree, "true")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = preparation.Close() })
	proofIdentity, err := proofrun.BuildProofIdentity(preparation.ExecutionRoot(), filepath.Join(preparation.ExecutionRoot(), "metasystem.conf"),
		"full", "landing-test-receipt", nil, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	attempt, _, err := proofrun.ReserveLocked(proofrun.AdmissionRequest{ControlRoot: f.root, ExecutionRoot: preparation.ExecutionRoot(),
		GoalID: "goal-a", GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 5, Identity: proofIdentity, Launcher: launcher, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	f.write("records/steward/narration.txt", "record motion\n")
	f.git("add", "records/steward/narration.txt")
	receipt, err := preparation.Complete(attempt, now.Add(time.Second))
	if err != nil {
		t.Fatalf("canonical record motion invalidated detached candidate proof: %v", err)
	}
	payload, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := proofrun.FinalizeAttempt(f.root, attempt.AttemptID, proofrun.TerminalSuccess, 0, "prepared receipt", payload, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := PublishCommittedReceipt(f.root, attempt.AttemptID); err == nil {
		t.Fatal("ordinary publication ignored the moved live candidate posture")
	}
	f.git("reset", "-q", "HEAD", "records/steward/narration.txt")
	if err := os.Remove(filepath.Join(f.root, "records", "steward", "narration.txt")); err != nil {
		t.Fatal(err)
	}
	recovered, err := PublishCommittedReceipt(f.root, attempt.AttemptID)
	if err != nil || recovered.Time != receipt.Time {
		t.Fatalf("exact prepared receipt was not recoverable after ordinary posture returned: receipt=%+v err=%v", recovered, err)
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
	f.write("metasystem.conf", "dispatch.cap-min=5\ndispatch.cap-max=120\n")
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
	identity, err := proofrun.BuildProofIdentity(preparation.ExecutionRoot(), filepath.Join(preparation.ExecutionRoot(), "metasystem.conf"),
		"full", "landing-test-receipt", []string{"gate"}, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	attempt, _, err := proofrun.ReserveLocked(proofrun.AdmissionRequest{ControlRoot: f.root,
		ExecutionRoot: preparation.ExecutionRoot(), GoalID: "goal-a", GoalRevision: 2, AccountingRevision: 2,
		ReservedMinutes: 5, Identity: identity, Launcher: launcher, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	baselineName := "coverage-ratchet.json"
	if runtime.GOOS == "linux" {
		baselineName = "coverage-ratchet-linux.json"
	}
	begin := proofrun.CoverageBeginOptions{ControlRoot: f.root, ExecutionRoot: preparation.ExecutionRoot(),
		AttemptID: attempt.AttemptID, BaselinePath: filepath.Join(preparation.ExecutionRoot(), "scripts", "agents", baselineName),
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
