package landing

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestReadTestReceiptSurvivesRegisterAppendAfterReceipt(t *testing.T) {
	for _, register := range []string{"records/narrator-digest.log", "memory/receipts.log"} {
		t.Run(register, func(t *testing.T) {
			f := newObserveFixture(t)
			f.write("product.txt", "candidate\n")
			f.git("add", "--", "product.txt")
			candidate, err := (gittree.Workspace{Dir: f.root}).StagedTree()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := CreateTestReceipt(f.root, candidate, "true", io.Discard, io.Discard); err != nil {
				t.Fatal(err)
			}
			appendReceiptFixtureFile(t, filepath.Join(f.root, register), "after-receipt\n")

			if _, err := readTestReceipt(ObserveParams{
				RepoRoot: f.root, CandidateTree: candidate, TestReceipt: TestReceiptPath(f.root, candidate),
			}); err != nil {
				t.Fatalf("receipt rejected append to %s: %v", register, err)
			}
		})
	}
}

func TestReadTestReceiptRefusesNonRegisterDrift(t *testing.T) {
	t.Run("unstaged product", func(t *testing.T) {
		f := newObserveFixture(t)
		f.write("product.txt", "candidate\n")
		f.git("add", "--", "product.txt")
		candidate, err := (gittree.Workspace{Dir: f.root}).StagedTree()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CreateTestReceipt(f.root, candidate, "true", io.Discard, io.Discard); err != nil {
			t.Fatal(err)
		}
		appendReceiptFixtureFile(t, filepath.Join(f.root, "product.txt"), "after-receipt\n")

		_, err = readTestReceipt(ObserveParams{
			RepoRoot: f.root, CandidateTree: candidate, TestReceipt: TestReceiptPath(f.root, candidate),
		})
		if err == nil || !strings.Contains(err.Error(), "the index or working tree moved after the test receipt was created") {
			t.Fatalf("non-register drift error = %v", err)
		}
	})

	t.Run("staged register append", func(t *testing.T) {
		f := newObserveFixture(t)
		f.write("product.txt", "candidate\n")
		f.git("add", "--", "product.txt")
		candidate, err := (gittree.Workspace{Dir: f.root}).StagedTree()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CreateTestReceipt(f.root, candidate, "true", io.Discard, io.Discard); err != nil {
			t.Fatal(err)
		}
		appendReceiptFixtureFile(t, filepath.Join(f.root, "memory", "receipts.log"), "after-receipt\n")
		f.git("add", "--", "memory/receipts.log")

		_, err = readTestReceipt(ObserveParams{
			RepoRoot: f.root, CandidateTree: candidate, TestReceipt: TestReceiptPath(f.root, candidate),
		})
		if err == nil || !strings.Contains(err.Error(), "the index or working tree moved after the test receipt was created") {
			t.Fatalf("staged register drift error = %v", err)
		}
	})
}

func TestReadSchemaTwoTestingReceiptSurvivesRegisterAppend(t *testing.T) {
	f := newObserveFixture(t)
	f.write("metasystem.conf", "testing.contract=testing.json\ndispatch.cap-max=120\n")
	f.git("add", ".", "../development/metasystem-design.md")
	projectRoot, err := (gittree.Workspace{Dir: f.root}).TopLevel()
	if err != nil {
		t.Fatal(err)
	}
	tree, err := (gittree.Workspace{Dir: projectRoot}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	subtree, err := (gittree.Workspace{Dir: f.root}).TreeOf(tree)
	if err != nil {
		t.Fatal(err)
	}
	head := f.git("rev-parse", "HEAD")
	identity, err := proofrun.BuildProofIdentity(
		f.root,
		filepath.Join(f.root, "metasystem.conf"),
		"selected",
		"testing",
		nil,
		behaviorsurface.SupportedVersion,
	)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	attempt, _, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{
		ControlRoot: f.root, ExecutionRoot: f.root, GoalID: "goal", GoalRevision: 2,
		AccountingRevision: 2, ReservedMinutes: 5, Identity: identity, Launcher: launcher, Now: now,
	}, tree))
	if err != nil {
		t.Fatal(err)
	}
	zero := 0
	digest := strings.Repeat("a", 64)
	result := proofrun.TestResult{
		SchemaVersion: proofrun.TestResultSchemaVersion, CandidateEngineIdentityVersion: proofrun.CandidateEngineIdentitySchemaVersion,
		AttemptID: attempt.AttemptID,
		Purpose:   testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeAuto,
		RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard,
		ProjectRoot: projectRoot, BaseCommit: head, CandidateTree: tree, PolicyBaseCommit: head,
		ContractDigest: digest, BaseContractDigest: digest, PolicyEngineDigest: digest,
		CandidateEngineDigest: strings.Repeat("e", 64), CandidateEngineBuildIdentity: strings.Repeat("f", 40),
		BehaviorPolicyDigest: digest, PlanDigest: digest,
		RequiredGroups: []string{"application"}, SelectedGroups: []string{"application"},
		LaunchCounts: proofrun.LaunchCounts{Test: 1, CountsComplete: true},
		StartedAt:    now.Add(-time.Second).Format(time.RFC3339Nano),
		Cost:         proofrun.TestCost{DeclaredTargetMS: 1},
		Groups: []proofrun.GroupResult{{
			ID: "application", Kind: "unit", Obligations: []string{"behavior"}, InputDigest: digest,
			InputManifest: []string{"source/**"}, ExecutionIdentity: digest, CWD: ".",
			ToolIdentities: map[string]string{}, Status: "passed", NativeLaunched: true,
			NativeExitStatus: &zero, CollectionComplete: true, ReportDigests: map[string]string{},
		}},
	}
	result.RecomputeDelivery()
	completedAt := now.Add(time.Second)
	prepared, payload, err := PrepareTestingReceiptPayload(f.root, tree, result, completedAt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := proofrun.FinalizeAttemptWithTestResultLocked(
		f.root, attempt.AttemptID, proofrun.TerminalSuccess, 0, "fixture", payload, prepared.Testing, completedAt,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := PublishCommittedReceipt(f.root, attempt.AttemptID, tree); err != nil {
		t.Fatal(err)
	}
	appendReceiptFixtureFile(t, filepath.Join(f.root, "records", "narrator-digest.log"), "after-receipt\n")

	if _, err := readTestReceipt(ObserveParams{
		RepoRoot: f.root, CandidateTree: subtree, TestReceipt: TestReceiptPath(f.root, tree),
	}); err != nil {
		t.Fatalf("schema-2 testing receipt rejected register append: %v", err)
	}
}

func appendReceiptFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateTestReceiptIgnoresLiveWorkspaceMotion(t *testing.T) {
	f := newObserveFixture(t)
	f.write("product.txt", "candidate\n")
	f.git("add", "--", "product.txt")
	candidate, err := (gittree.Workspace{Dir: f.root}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	worktreesBefore := f.git("worktree", "list", "--porcelain")
	t.Setenv("LANDING_RECEIPT_LIVE_ROOT", f.root)
	t.Setenv("GOTOOLCHAIN", "receipt-fixture")

	receipt, err := CreateTestReceipt(
		f.root,
		candidate,
		`test "$GOTOOLCHAIN" = receipt-fixture && printf '%s\n' 'digest=during-receipt' >> "$LANDING_RECEIPT_LIVE_ROOT/records/narrator-digest.log" && sleep 0.1`,
		io.Discard,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("receipt rejected live workspace motion: %v", err)
	}
	if receipt.ExitStatus != 0 {
		t.Fatalf("receipt exit status = %d, want zero", receipt.ExitStatus)
	}
	identity, err := receiptIdentity(gittree.Workspace{Dir: f.root}, candidate)
	if err != nil {
		t.Fatal(err)
	}
	wantBinding := TestReceiptBinding{
		IndexTreeBefore: candidate, WorktreeTreeBefore: identity,
		IndexTreeAfter: candidate, WorktreeTreeAfter: identity,
	}
	if receipt.SchemaVersion != 3 || receipt.Tree != candidate || receipt.Binding != wantBinding ||
		receipt.WorktreeProjection == nil || receipt.WorktreeProjection.Tree != identity {
		t.Fatalf("receipt = %+v, want tree and all bindings %s", receipt, candidate)
	}
	digest, err := os.ReadFile(filepath.Join(f.root, "records", "narrator-digest.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(digest), "digest=during-receipt\n") {
		t.Fatalf("live narrator digest did not retain the command's append:\n%s", digest)
	}
	if got := f.git("worktree", "list", "--porcelain"); got != worktreesBefore {
		t.Fatalf("temporary receipt worktree remained after success:\n%s", got)
	}
	if _, err := readTestReceipt(ObserveParams{
		RepoRoot: f.root, CandidateTree: candidate, TestReceipt: TestReceiptPath(f.root, candidate),
	}); err != nil {
		t.Fatalf("receipt rejected live register motion: %v", err)
	}
}

func TestCreateTestReceiptToleratesCandidateRegisterAppend(t *testing.T) {
	f := newObserveFixture(t)
	candidate, err := (gittree.Workspace{Dir: f.root}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := CreateTestReceipt(
		f.root,
		candidate,
		`printf '%s\n' 'digest=during-battery' >> records/narrator-digest.log`,
		io.Discard,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("candidate register append refused: %v", err)
	}
	identity, err := receiptIdentity(gittree.Workspace{Dir: f.root}, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Binding.IndexTreeAfter != candidate || receipt.Binding.WorktreeTreeAfter != identity {
		t.Fatalf("receipt bindings = %+v, want exact index %s and projection %s", receipt.Binding, candidate, identity)
	}
}

func TestReadTestReceiptVersions(t *testing.T) {
	tests := []struct {
		name       string
		version    int
		binding    func(candidate, identity string) TestReceiptBinding
		projection func(identity string) *TestReceiptProjection
		wantError  string
	}{
		{
			name: "version 1", version: 1,
			binding: exactReceiptBinding,
		},
		{
			name: "version 1 with filtered bindings", version: 1,
			binding:   filteredReceiptBinding,
			wantError: "test receipt binding does not equal the candidate tree",
		},
		{
			name: "version 3 with raw worktree bindings", version: 3,
			binding: exactReceiptBinding,
			projection: func(identity string) *TestReceiptProjection {
				return &TestReceiptProjection{Excludes: AppendOnlyRegisters(), Tree: identity}
			},
			wantError: "test receipt binding does not equal the candidate tree",
		},
		{
			name: "version 1 with projection", version: 1,
			binding: exactReceiptBinding,
			projection: func(identity string) *TestReceiptProjection {
				return &TestReceiptProjection{Excludes: AppendOnlyRegisters(), Tree: identity}
			},
			wantError: "test receipt mixes schema versions",
		},
		{
			name: "version 2 with projection", version: 2,
			binding: exactReceiptBinding,
			projection: func(identity string) *TestReceiptProjection {
				return &TestReceiptProjection{Excludes: AppendOnlyRegisters(), Tree: identity}
			},
			wantError: "test receipt mixes schema versions",
		},
		{
			name: "version 3 without projection", version: 3,
			binding:   filteredReceiptBinding,
			wantError: "test receipt mixes schema versions",
		},
		{
			name: "version 3 with different register set", version: 3,
			binding: filteredReceiptBinding,
			projection: func(identity string) *TestReceiptProjection {
				return &TestReceiptProjection{Excludes: AppendOnlyRegisters()[:1], Tree: identity}
			},
			wantError: "test receipt excludes a different register set than this engine",
		},
		{
			name: "unsupported version", version: 4,
			binding:   exactReceiptBinding,
			wantError: "test receipt does not record a successful command for the candidate tree",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newObserveFixture(t)
			f.write("product.txt", "candidate\n")
			f.git("add", "--", "product.txt")
			candidate, err := (gittree.Workspace{Dir: f.root}).StagedTree()
			if err != nil {
				t.Fatal(err)
			}
			identity, err := receiptIdentity(gittree.Workspace{Dir: f.root}, candidate)
			if err != nil {
				t.Fatal(err)
			}
			receipt := TestReceipt{
				SchemaVersion: test.version,
				Tree:          candidate,
				Command:       "true",
				Time:          "2026-09-09T00:00:00Z",
				Binding:       test.binding(candidate, identity),
			}
			if test.projection != nil {
				receipt.WorktreeProjection = test.projection(identity)
			}
			writeTestReceiptFixture(t, f.root, candidate, receipt)

			_, err = readTestReceipt(ObserveParams{
				RepoRoot: f.root, CandidateTree: candidate, TestReceipt: TestReceiptPath(f.root, candidate),
			})
			if test.wantError == "" && err != nil {
				t.Fatalf("receipt refused: %v", err)
			}
			if test.wantError != "" && (err == nil || !strings.Contains(err.Error(), test.wantError)) {
				t.Fatalf("receipt error = %v, want %q", err, test.wantError)
			}
		})
	}
}

func exactReceiptBinding(candidate, _ string) TestReceiptBinding {
	return TestReceiptBinding{
		IndexTreeBefore: candidate, WorktreeTreeBefore: candidate,
		IndexTreeAfter: candidate, WorktreeTreeAfter: candidate,
	}
}

func filteredReceiptBinding(candidate, identity string) TestReceiptBinding {
	return TestReceiptBinding{
		IndexTreeBefore: candidate, WorktreeTreeBefore: identity,
		IndexTreeAfter: candidate, WorktreeTreeAfter: identity,
	}
}

func writeTestReceiptFixture(t *testing.T, root, candidate string, receipt TestReceipt) {
	t.Helper()
	data, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	path := TestReceiptPath(root, candidate)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCreateTestReceiptRefusesIsolatedCandidateMotion(t *testing.T) {
	f := newObserveFixture(t)
	f.write("product.txt", "candidate\n")
	f.git("add", "--", "product.txt")
	candidate, err := (gittree.Workspace{Dir: f.root}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	worktreesBefore := f.git("worktree", "list", "--porcelain")

	_, err = CreateTestReceipt(
		f.root,
		candidate,
		`printf '%s\n' changed-by-command > product.txt`,
		io.Discard,
		io.Discard,
	)
	if err == nil || !strings.Contains(err.Error(), "test receipt refused: the candidate changed while the command ran") {
		t.Fatalf("candidate-changing command error = %v", err)
	}
	if _, statErr := os.Stat(TestReceiptPath(f.root, candidate)); !os.IsNotExist(statErr) {
		t.Fatalf("refused receipt remains available: %v", statErr)
	}
	liveProduct, readErr := os.ReadFile(filepath.Join(f.root, "product.txt"))
	if readErr != nil || string(liveProduct) != "candidate\n" {
		t.Fatalf("candidate-changing command reached the live product: bytes=%q error=%v", liveProduct, readErr)
	}
	if got := f.git("worktree", "list", "--porcelain"); got != worktreesBefore {
		t.Fatalf("temporary receipt worktree remained after refusal:\n%s", got)
	}
}

func TestCreateTestReceiptRemovesIsolatedWorktreeAfterCommandFailure(t *testing.T) {
	f := newObserveFixture(t)
	candidate, err := (gittree.Workspace{Dir: f.root}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	worktreesBefore := f.git("worktree", "list", "--porcelain")

	receipt, err := CreateTestReceipt(f.root, candidate, "exit 23", io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("record failing command: %v", err)
	}
	if receipt.ExitStatus != 23 {
		t.Fatalf("receipt exit status = %d, want 23", receipt.ExitStatus)
	}
	if got := f.git("worktree", "list", "--porcelain"); got != worktreesBefore {
		t.Fatalf("temporary receipt worktree remained after command failure:\n%s", got)
	}
}

func TestCreateTestReceiptChecksOutWholeTreeAtRepositoryRoot(t *testing.T) {
	f := newAdoptedObserveFixture(t)
	if err := os.Remove(filepath.Join(f.root, "product.txt")); err != nil {
		t.Fatal(err)
	}
	f.write("candidate.txt", "candidate\n")
	f.git("add", "-A", "--", ".")
	candidate, err := (gittree.Workspace{Dir: f.root}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}

	receipt, err := CreateTestReceipt(
		f.root,
		candidate,
		`test ! -e product.txt && test "$(cat candidate.txt)" = candidate`,
		io.Discard,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("receipt against repository-root candidate: %v", err)
	}
	identity, err := receiptIdentity(gittree.Workspace{Dir: f.root}, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Tree != candidate || receipt.Binding.IndexTreeAfter != candidate || receipt.Binding.WorktreeTreeAfter != identity {
		t.Fatalf("receipt = %+v, want candidate %s", receipt, candidate)
	}
}

func TestCreateTestReceiptRemovesIsolatedWorktreeAfterSignal(t *testing.T) {
	if os.Getenv("LANDING_RECEIPT_SIGNAL_HELPER") == "1" {
		_, err := CreateTestReceipt(
			os.Getenv("LANDING_RECEIPT_SIGNAL_ROOT"),
			os.Getenv("LANDING_RECEIPT_SIGNAL_TREE"),
			`printf '%s\n' "$PWD" > "$LANDING_RECEIPT_SIGNAL_PROBE"; exec sleep 5`,
			io.Discard,
			io.Discard,
		)
		if err == nil || !strings.Contains(err.Error(), "landing test receipt interrupted by") {
			t.Fatalf("signaled receipt error = %v", err)
		}
		return
	}

	f := newObserveFixture(t)
	candidate, err := (gittree.Workspace{Dir: f.root}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	worktreesBefore := f.git("worktree", "list", "--porcelain")
	probe := filepath.Join(t.TempDir(), "isolated-root")
	helper := exec.Command(os.Args[0], "-test.run=^TestCreateTestReceiptRemovesIsolatedWorktreeAfterSignal$")
	helper.Env = gittree.ScrubbedEnviron(
		"LANDING_RECEIPT_SIGNAL_HELPER=1",
		"LANDING_RECEIPT_SIGNAL_ROOT="+f.root,
		"LANDING_RECEIPT_SIGNAL_TREE="+candidate,
		"LANDING_RECEIPT_SIGNAL_PROBE="+probe,
	)
	var output bytes.Buffer
	helper.Stdout = &output
	helper.Stderr = &output
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	helperWaited := false
	t.Cleanup(func() {
		if !helperWaited {
			_ = helper.Process.Signal(os.Interrupt)
			_ = helper.Wait()
		}
	})
	// The helper is a second copy of this test binary doing real git work;
	// under a loaded box its probe takes longer than a quiet box's few
	// seconds, so the bound is one only a hang trips (2026-09-11: ten
	// seconds failed inside the pooled battery). And the helper's output is
	// read only after the helper has been waited for: its stdout copier
	// writes the buffer until then, which the race detector reported.
	deadline := time.Now().Add(90 * time.Second)
	var isolatedRoot string
	for time.Now().Before(deadline) {
		data, readErr := os.ReadFile(probe)
		if readErr == nil {
			isolatedRoot = strings.TrimSpace(string(data))
			break
		}
		if !os.IsNotExist(readErr) {
			t.Fatal(readErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if isolatedRoot == "" {
		_ = helper.Process.Signal(os.Interrupt)
		_ = helper.Wait()
		helperWaited = true
		t.Fatalf("signal helper did not expose its isolated root:\n%s", output.String())
	}
	resolvedTemp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	resolvedIsolatedRoot, err := filepath.EvalSymlinks(isolatedRoot)
	if err != nil {
		t.Fatal(err)
	}
	relativeToTemp, err := filepath.Rel(resolvedTemp, resolvedIsolatedRoot)
	if err != nil || relativeToTemp == ".." || strings.HasPrefix(relativeToTemp, ".."+string(filepath.Separator)) {
		t.Fatalf("isolated root %q is not under the system temporary directory", isolatedRoot)
	}
	if err := helper.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	waitErr := helper.Wait()
	helperWaited = true
	if waitErr != nil {
		t.Fatalf("signal helper failed: %v\n%s", waitErr, output.String())
	}
	if _, err := os.Stat(isolatedRoot); !os.IsNotExist(err) {
		t.Fatalf("signaled receipt left isolated root %s: %v", isolatedRoot, err)
	}
	if got := f.git("worktree", "list", "--porcelain"); got != worktreesBefore {
		t.Fatalf("temporary receipt worktree remained after signal:\n%s", got)
	}
}

func TestReceiptCommandBoundResolution(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	if got := receiptCommandBound(conf); got.Limit != 40*time.Minute || got.Key != "landing.receipt-bound-min" {
		t.Fatalf("absent receipt bound = %+v", got)
	}
	if err := os.WriteFile(conf, []byte("landing.receipt-bound-min=17\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := receiptCommandBound(conf); got.Limit != 17*time.Minute || got.Key != "landing.receipt-bound-min" {
		t.Fatalf("configured receipt bound = %+v", got)
	}
	for _, malformed := range []string{"0", "nonsense"} {
		if err := os.WriteFile(conf, []byte("landing.receipt-bound-min="+malformed+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := receiptCommandBound(conf); got.Limit != 40*time.Minute {
			t.Fatalf("malformed receipt bound %q did not fall back: %+v", malformed, got)
		}
	}
}
