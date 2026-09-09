package proofrun

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

const coverageTestModule = "github.com/widoriezebos/agentic-tools/metasystem/"

func TestCoverageReceiptProducerConsumer(t *testing.T) {
	root, identity := proofAttemptFixture(t, "canonical-validator")
	baseline := coverageRatchetPath(root)
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	attempt, _, err := ReserveLocked(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 5, Identity: identity, Launcher: launcher, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	begin := CoverageBeginOptions{ControlRoot: root, ExecutionRoot: root, AttemptID: attempt.AttemptID,
		BaselinePath: baseline, ProducerClass: "seed", ProducerPID: int64(os.Getpid()), CallerPID: int64(os.Getpid())}
	if err := BeginCoverage(begin); err == nil {
		t.Fatal("seed producer claimed reusable coverage")
	}
	begin.ProducerClass = "full"
	if err := BeginCoverage(begin); err != nil {
		t.Fatal(err)
	}
	evidenceRoot := t.TempDir()
	coverageLog := filepath.Join(evidenceRoot, "coverage.log")
	inventory := filepath.Join(evidenceRoot, "packages.txt")
	if err := os.WriteFile(coverageLog, []byte("ok  "+coverageTestModule+"internal/proofrun 0.1s coverage: 85.0% of statements\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inventory, []byte(coverageTestModule+"internal/proofrun\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	evidence, err := CompleteCoverage(CoverageCompleteOptions{CoverageBeginOptions: begin, CoverageLog: coverageLog,
		PackageInventory: inventory, ModulePrefix: coverageTestModule})
	if err != nil || evidence.Measurements["internal/proofrun"] != 85 {
		t.Fatalf("coverage evidence = %+v, %v", evidence, err)
	}
	if _, found, err := ReusableCoverage(root, root, baseline, []string{"internal/proofrun"}); err != nil || found {
		t.Fatalf("provisional inner green was reusable before enclosing success: found=%v err=%v", found, err)
	}
	if _, err := FinalizeAttempt(root, attempt.AttemptID, TerminalSuccess, 0, "enclosing validator passed", []byte(`{"receipt":true}`), now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if reused, found, err := ReusableCoverage(root, root, baseline, []string{"internal/proofrun"}); err != nil || !found || reused.AttemptID != attempt.AttemptID {
		t.Fatalf("coverage was not reused after enclosing success: %+v found=%v err=%v", reused, found, err)
	}
	operatorNote := filepath.Join(root, "operator-note.txt")
	if err := os.WriteFile(operatorNote, []byte("unrelated to ENGINE coverage\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if reused, found, err := ReusableCoverage(root, root, baseline, []string{"internal/proofrun"}); err != nil || !found || reused.AttemptID != attempt.AttemptID {
		t.Fatalf("unrelated non-ENGINE note invalidated relevant coverage reuse: evidence=%+v found=%v err=%v", reused, found, err)
	}

	mutated := filepath.Join(root, "internal", "proofrun", "new.go")
	if err := os.MkdirAll(filepath.Dir(mutated), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mutated, []byte("package proofrun\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, found, err := ReusableCoverage(root, root, baseline, []string{"internal/proofrun"}); err != nil || found {
		t.Fatalf("source mutation reused coverage: found=%v err=%v", found, err)
	}
	if err := os.RemoveAll(filepath.Join(root, "internal")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(baseline, []byte(`{"floors":{"internal/proofrun":86},"exempt":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, found, err := ReusableCoverage(root, root, baseline, []string{"internal/proofrun"}); err != nil || found {
		t.Fatalf("floor mutation reused coverage: found=%v err=%v", found, err)
	}
}

func TestCoverageEligibilityKeepsAuthenticatedForeignDescendantOutOfProducerSlot(t *testing.T) {
	root, identity := proofAttemptFixture(t, "coverage-eligibility")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	attempt, _, err := ReserveLocked(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 5, Identity: identity, Launcher: launcher, Now: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	producer := CoverageBeginOptions{ControlRoot: root, ExecutionRoot: root, AttemptID: attempt.AttemptID,
		BaselinePath: coverageRatchetPath(root), ProducerClass: "full", ProducerPID: int64(os.Getpid()), CallerPID: int64(os.Getpid())}
	eligible, err := CoverageProducerEligible(producer)
	if err != nil || !eligible {
		t.Fatalf("admitted execution root eligibility=%v err=%v", eligible, err)
	}
	foreign := t.TempDir()
	producer.ExecutionRoot = foreign
	producer.BaselinePath = filepath.Join(foreign, "scripts", "agents", filepath.Base(coverageRatchetPath(root)))
	eligible, err = CoverageProducerEligible(producer)
	if err != nil || eligible {
		t.Fatalf("authenticated foreign fixture eligibility=%v err=%v", eligible, err)
	}
	if err := BeginCoverage(producer); err == nil {
		t.Fatal("foreign fixture claimed the admitted producer slot directly")
	}
	producer.CallerPID = int64(^uint32(0))
	if _, err := CoverageProducerEligible(producer); err == nil {
		t.Fatal("invalid descendant custody became a non-producer fallback")
	}
	if _, err := FinalizeAttempt(root, attempt.AttemptID, TerminalFailed, 1, "eligibility fixture cleanup", nil, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
}

func TestCoverageEligibilityAcceptsEquivalentSnapshotPath(t *testing.T) {
	root, identity := proofAttemptFixture(t, "coverage-snapshot")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	attempt, _, err := ReserveLocked(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 5, Identity: identity, Launcher: launcher, Now: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := t.TempDir()
	for _, relative := range []string{"metasystem.conf", "scripts/agents/coverage-ratchet.json", "scripts/agents/coverage-ratchet-linux.json"} {
		data, readErr := os.ReadFile(filepath.Join(root, relative))
		if readErr != nil {
			t.Fatal(readErr)
		}
		path := filepath.Join(snapshot, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	producer := CoverageBeginOptions{ControlRoot: root, ExecutionRoot: snapshot, AttemptID: attempt.AttemptID,
		BaselinePath: coverageRatchetPath(snapshot), ProducerClass: "full", ProducerPID: int64(os.Getpid()), CallerPID: int64(os.Getpid())}
	eligible, err := CoverageProducerEligible(producer)
	if err != nil || !eligible {
		t.Fatalf("equivalent snapshot eligibility=%v err=%v", eligible, err)
	}
}

func TestCoveragePlatformToolchainAndPolicyMismatchRefuse(t *testing.T) {
	for _, field := range []string{"platform", "toolchain", "policy"} {
		t.Run(field, func(t *testing.T) {
			root, attempt, baseline := completedCoverageFixture(t)
			stored, err := ReadAttempt(root, attempt.AttemptID)
			if err != nil {
				t.Fatal(err)
			}
			switch field {
			case "platform":
				stored.PendingCoverage.Evidence.Platform = runtime.GOOS + "/foreign"
			case "toolchain":
				stored.PendingCoverage.Evidence.Toolchain = "different"
			case "policy":
				stored.PendingCoverage.Evidence.BehaviorPolicy++
			}
			if err := writeAttempt(stored); err != nil {
				t.Fatal(err)
			}
			if _, found, err := ReusableCoverage(root, root, baseline, []string{"internal/proofrun"}); err != nil || found {
				t.Fatalf("%s mismatch reused coverage: found=%v err=%v", field, found, err)
			}
		})
	}
}

func TestCoverageReuseSeparatesControlAndExecutionRoots(t *testing.T) {
	controlRoot, _, _ := completedCoverageFixture(t)
	executionRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(executionRoot, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"metasystem.conf", "scripts/agents/coverage-ratchet.json", "scripts/agents/coverage-ratchet-linux.json"} {
		data, err := os.ReadFile(filepath.Join(controlRoot, relative))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(executionRoot, relative), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	executionBaseline := coverageRatchetPath(executionRoot)
	if reused, found, err := ReusableCoverage(controlRoot, executionRoot, executionBaseline, []string{"internal/proofrun"}); err != nil || !found || reused == nil {
		t.Fatalf("delegate execution root could not consume canonical proof evidence: evidence=%+v found=%v err=%v", reused, found, err)
	}
	if _, found, err := ReusableCoverage(executionRoot, executionRoot, executionBaseline, []string{"internal/proofrun"}); err != nil || found {
		t.Fatalf("missing delegate-local evidence was invented: found=%v err=%v", found, err)
	}
}

func completedCoverageFixture(t *testing.T) (string, Attempt, string) {
	t.Helper()
	root, identity := proofAttemptFixture(t, "coverage-negative")
	baseline := coverageRatchetPath(root)
	launcher, _ := CurrentProcessIdentity(nil)
	now := time.Now().UTC()
	attempt, _, err := ReserveLocked(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 2,
		AccountingRevision: 2, ReservedMinutes: 5, Identity: identity, Launcher: launcher, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	begin := CoverageBeginOptions{ControlRoot: root, ExecutionRoot: root, AttemptID: attempt.AttemptID, BaselinePath: baseline,
		ProducerClass: "full", ProducerPID: int64(os.Getpid()), CallerPID: int64(os.Getpid())}
	if err := BeginCoverage(begin); err != nil {
		t.Fatal(err)
	}
	evidenceRoot := t.TempDir()
	logPath, inventory := filepath.Join(evidenceRoot, "coverage.log"), filepath.Join(evidenceRoot, "packages.txt")
	_ = os.WriteFile(logPath, []byte("ok  "+coverageTestModule+"internal/proofrun 0.1s coverage: 85.0% of statements\n"), 0o600)
	_ = os.WriteFile(inventory, []byte(coverageTestModule+"internal/proofrun\n"), 0o600)
	if _, err := CompleteCoverage(CoverageCompleteOptions{CoverageBeginOptions: begin, CoverageLog: logPath,
		PackageInventory: inventory, ModulePrefix: coverageTestModule}); err != nil {
		t.Fatal(err)
	}
	if _, err := FinalizeAttempt(root, attempt.AttemptID, TerminalSuccess, 0, "green", []byte(`{"receipt":true}`), now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	return root, attempt, baseline
}
