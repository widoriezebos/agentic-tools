package landing

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
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
			f := newReceiptReaderFixture(t, register, false)
			f.receipt()
			appendReceiptFixtureFile(t, filepath.Join(f.root, register), "after-receipt\n")

			if _, err := f.read(); err != nil {
				t.Fatalf("receipt rejected append to %s: %v", register, err)
			}
		})
	}
}

func TestReadTestReceiptRefusesNonRegisterDrift(t *testing.T) {
	t.Run("unstaged product", func(t *testing.T) {
		f := newReceiptReaderFixture(t, "product.txt", false)
		f.receipt()
		appendReceiptFixtureFile(t, filepath.Join(f.root, "product.txt"), "after-receipt\n")

		_, err := f.read()
		if err == nil || !strings.Contains(err.Error(), "the index or working tree moved after the test receipt was created") {
			t.Fatalf("non-register drift error = %v", err)
		}
	})

	t.Run("staged register append", func(t *testing.T) {
		f := newReceiptReaderFixture(t, "memory/receipts.log", true)
		f.receipt()
		appendReceiptFixtureFile(t, filepath.Join(f.root, "memory", "receipts.log"), "after-receipt\n")

		_, err := f.read()
		if err == nil || !strings.Contains(err.Error(), "the index or working tree moved after the test receipt was created") {
			t.Fatalf("staged register drift error = %v", err)
		}
	})
}

func TestReadSchemaTwoTestingReceiptSurvivesRegisterAppend(t *testing.T) {
	f := newSchemaTwoReceiptFixture(t)
	projectRoot := f.repository
	workspace := gittree.Workspace{Dir: projectRoot, RawSource: f.raw}
	tree := f.remember(f.top(f.index))
	subtree, err := (gittree.Workspace{Dir: f.root, RawSource: f.raw}).TreeOf(tree)
	if err != nil {
		t.Fatal(err)
	}
	head := observeBaseTree
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
	attempt, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{
		ControlRoot: f.root, ExecutionRoot: f.root, GoalID: "goal", GoalRevision: 2,
		AccountingRevision: 2, ReservedMinutes: 5, Identity: identity, Launcher: launcher, Now: now,
	}, tree))
	if err != nil {
		t.Fatal(err)
	}
	requireProofReservationNotAdmissionRefused(t, decision)
	zero, admissionMaximum := 0, 0
	digest := strings.Repeat("a", 64)
	result := proofrun.TestResult{
		SchemaVersion: proofrun.TestResultSchemaVersion, CandidateEngineIdentityVersion: proofrun.CandidateEngineIdentitySchemaVersion,
		WorkerPolicyVersion: proofrun.TestWorkerPolicyVersion, Workers: 1, AdmissionMaximum: &admissionMaximum,
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
			IdentityVersion: proofrun.GroupExecutionIdentityVersion,
			InputManifest:   []string{"source/**"}, ExecutionIdentity: digest, CWD: ".",
			ToolIdentities: map[string]string{}, Status: "passed", NativeLaunched: true,
			NativeExitStatus: &zero, CollectionComplete: true, ReportDigests: map[string]string{},
		}},
	}
	result.RecomputeDelivery()
	completedAt := now.Add(time.Second)
	prepared, payload, err := prepareTestingReceiptPayloadWithWorkspace(f.root, tree, result, completedAt, workspace)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := proofrun.FinalizeAttemptWithTestResultLocked(
		f.root, attempt.AttemptID, proofrun.TerminalSuccess, 0, "fixture", payload, prepared.Testing, completedAt,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := publishCommittedReceiptAtWithWorkspace(f.root, attempt.AttemptID, tree, time.Now().UTC(), workspace); err != nil {
		t.Fatal(err)
	}
	register := "records/narrator-digest.log"
	appendReceiptFixtureFile(t, filepath.Join(f.root, register), "after-receipt\n")
	f.worktree[register] = append(bytes.Clone(f.worktree[register]), "after-receipt\n"...)
	f.nextPhase("register")

	if _, err := readTestReceiptWithWorkspace(ObserveParams{
		RepoRoot: f.root, CandidateTree: subtree, TestReceipt: TestReceiptPath(f.root, tree),
	}, workspace); err != nil {
		t.Fatalf("schema-2 testing receipt rejected register append: %v", err)
	}
	f.nextPhase("complete")
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
		`test "$GOTOOLCHAIN" = receipt-fixture && printf '%s\n' 'digest=during-receipt' >> "$LANDING_RECEIPT_LIVE_ROOT/records/narrator-digest.log"`,
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
	f := newReceiptReaderFixture(t, "", false)
	register := "records/narrator-digest.log"
	f.isolated.root = t.TempDir()
	f.isolated.after = make(map[string][]byte, len(f.before))
	for path, data := range f.before {
		f.isolated.after[path] = bytes.Clone(data)
	}
	f.isolated.after[register] = append(f.isolated.after[register], "digest=during-battery\n"...)
	command := `printf '%s\n' 'digest=during-battery' >> records/narrator-digest.log`
	var candidateRegister []byte
	closeCalls := 0
	checkout := func(workspace gittree.Workspace, tree string) (gittree.Workspace, func() error, error) {
		if workspace.Dir != f.root || tree != f.candidate {
			t.Fatalf("checkout input = %q, %q", workspace.Dir, tree)
		}
		for path, data := range f.before {
			full := filepath.Join(f.isolated.root, filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, data, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(full, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		f.checkFactFilesAt(f.isolated.root, f.before)
		return gittree.Workspace{Dir: f.isolated.root, RawSource: f.raw}, func() error {
			closeCalls++
			if closeCalls == 1 {
				var err error
				candidateRegister, err = os.ReadFile(filepath.Join(f.isolated.root, register))
				if err != nil {
					return err
				}
			}
			return os.RemoveAll(f.isolated.root)
		}, nil
	}
	receipt, err := createTestReceiptWithInputs(f.root, f.candidate, command, io.Discard, io.Discard,
		gittree.Workspace{Dir: f.root, RawSource: f.raw}, checkout)
	if err != nil {
		t.Fatalf("candidate register append refused: %v", err)
	}
	if closeCalls != 2 {
		t.Fatalf("candidate close calls = %d, want two", closeCalls)
	}
	if !bytes.Equal(candidateRegister, f.isolated.after[register]) {
		t.Fatalf("candidate register = %q, want %q", candidateRegister, f.isolated.after[register])
	}
	liveRegister, err := os.ReadFile(filepath.Join(f.root, register))
	if err != nil || !bytes.Equal(liveRegister, f.before[register]) {
		t.Fatalf("live register = %q, %v; want %q", liveRegister, err, f.before[register])
	}
	if receipt.SchemaVersion != 3 || receipt.Tree != f.candidate || receipt.Command != command || receipt.ExitStatus != 0 ||
		receipt.Binding != filteredReceiptBinding(f.candidate, f.identity) || receipt.WorktreeProjection == nil ||
		receipt.WorktreeProjection.Tree != f.identity || !slices.Equal(receipt.WorktreeProjection.Excludes, AppendOnlyRegisters()) {
		t.Fatalf("receipt = %+v, want candidate index and filtered projection", receipt)
	}
	published, err := os.ReadFile(TestReceiptPath(f.root, f.candidate))
	if err != nil || len(published) == 0 || published[len(published)-1] != '\n' {
		t.Fatalf("published receipt = %q, %v", published, err)
	}
	var decoded TestReceipt
	if err := json.Unmarshal(published, &decoded); err != nil || decoded.Binding != receipt.Binding {
		t.Fatalf("published receipt binding = %+v, %v", decoded.Binding, err)
	}
	accepted, err := f.read()
	if err != nil || accepted.Binding != receipt.Binding {
		t.Fatalf("published receipt rejected: %+v, %v", accepted, err)
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
			f := newReceiptReaderFixture(t, "", false)
			candidate, identity := f.candidate, f.identity
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

			_, err := f.read()
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
		ready, err := os.OpenFile(os.Getenv("LANDING_RECEIPT_SIGNAL_PROBE"), os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ready.WriteString(os.TempDir() + "\n"); err != nil {
			_ = ready.Close()
			t.Fatal(err)
		}
		if err := ready.Close(); err != nil {
			t.Fatal(err)
		}
		_, err = CreateTestReceipt(
			os.Getenv("LANDING_RECEIPT_SIGNAL_ROOT"),
			os.Getenv("LANDING_RECEIPT_SIGNAL_TREE"),
			`printf '%s\n' "$PWD" > "$LANDING_RECEIPT_SIGNAL_PROBE"; exec bash -c 'read -r _' < "$LANDING_RECEIPT_SIGNAL_HOLD"`,
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
	signalDir := t.TempDir()
	probe := filepath.Join(signalDir, "isolated-root")
	hold := filepath.Join(signalDir, "parent-lifetime")
	if err := syscall.Mkfifo(probe, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(hold, 0o600); err != nil {
		t.Fatal(err)
	}
	ready, err := os.OpenFile(probe, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ready.Close()
	held, err := os.OpenFile(hold, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	helper := exec.Command(os.Args[0], "-test.run=^TestCreateTestReceiptRemovesIsolatedWorktreeAfterSignal$")
	helper.Env = gittree.ScrubbedEnviron(
		"LANDING_RECEIPT_SIGNAL_HELPER=1",
		"LANDING_RECEIPT_SIGNAL_ROOT="+f.root,
		"LANDING_RECEIPT_SIGNAL_TREE="+candidate,
		"LANDING_RECEIPT_SIGNAL_PROBE="+probe,
		"LANDING_RECEIPT_SIGNAL_HOLD="+hold,
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
	readyReader := bufio.NewReader(ready)
	childTemp, err := readyReader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	childTemp = strings.TrimSpace(childTemp)
	if childTemp == "" {
		t.Fatal("signal helper exposed an empty temporary root")
	}
	isolatedRoot, err := readyReader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	isolatedRoot = strings.TrimSpace(isolatedRoot)
	if isolatedRoot == "" {
		t.Fatal("signal helper exposed an empty isolated root")
	}
	resolvedTemp, err := filepath.EvalSymlinks(childTemp)
	if err != nil {
		t.Fatal(err)
	}
	resolvedIsolatedRoot, err := filepath.EvalSymlinks(isolatedRoot)
	if err != nil {
		t.Fatal(err)
	}
	relativeToTemp, err := filepath.Rel(resolvedTemp, resolvedIsolatedRoot)
	if err != nil || relativeToTemp == ".." || strings.HasPrefix(relativeToTemp, ".."+string(filepath.Separator)) {
		t.Fatalf("isolated root %q is not under the signal helper temporary root %q", isolatedRoot, childTemp)
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
