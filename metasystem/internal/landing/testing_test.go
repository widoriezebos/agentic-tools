package landing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestFreshReceiptCannotMatchOrdinaryVerification(t *testing.T) {
	t.Parallel()
	ordinary := proofrun.TestResult{SelectedGroups: []string{"check"}, RequiredGroups: []string{"check"},
		Groups: []proofrun.GroupResult{{ID: "check", ExecutionIdentity: strings.Repeat("a", 64)}}}
	fresh := ordinary
	fresh.FreshnessEpisode, fresh.FreshnessBinding = strings.Repeat("b", 64), strings.Repeat("c", 64)
	if differences := testingIdentityDifferences(ordinary, fresh); len(differences) != 1 || differences[0] != "check" {
		t.Fatalf("ordinary verification matched a fresh receipt: %v", differences)
	}
}

func TestFreshReceiptRequiresItsNativeProducerEpisodeBindingAndExpiry(t *testing.T) {
	f := &observeFixture{t: t, root: t.TempDir()}
	f.write("metasystem.conf", "dispatch.cap-max=120\nmetasystem.runtimes=fake\n")
	t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", f.root)
	t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", filepath.Join(t.TempDir(), "proof-admission"))
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	identity, err := proofrun.BuildProofIdentity(f.root, filepath.Join(f.root, "metasystem.conf"), "selected", "testing", nil, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	episode, binding, groupIdentity := strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat("3", 64)
	expiry := now.Add(time.Hour).Format(time.RFC3339Nano)
	attempt, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{ControlRoot: f.root, ExecutionRoot: f.root,
		GoalID: "goal", GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 5, Identity: identity, Launcher: launcher,
		Now: now, SharedComponents: true, ComponentIdentities: map[string]string{"check": groupIdentity},
		FreshnessEpisode: episode, FreshnessBinding: binding, FreshnessExpiresAt: expiry}, strings.Repeat("b", 40)))
	if err != nil || decision.Disposition != proofrun.DispositionExecuted {
		t.Fatalf("fresh reservation: %+v %+v %v", attempt, decision, err)
	}
	result := proofrun.TestResult{AttemptID: attempt.AttemptID, FreshnessEpisode: episode, FreshnessBinding: binding, FreshnessExpiresAt: expiry,
		Groups: []proofrun.GroupResult{{ID: "check", ExecutionIdentity: groupIdentity, Status: "passed", NativeLaunched: true, CollectionComplete: true}}}
	if _, err := validateTestingAttemptOwnersAt(f.root, result, true, now); err != nil {
		t.Fatalf("matching live native producer refused: %v", err)
	}
	for _, testCase := range []struct {
		name   string
		change func(*proofrun.TestResult)
	}{
		{"missing episode", func(value *proofrun.TestResult) { value.FreshnessEpisode = "" }},
		{"wrong episode", func(value *proofrun.TestResult) { value.FreshnessEpisode = strings.Repeat("4", 64) }},
		{"wrong binding", func(value *proofrun.TestResult) { value.FreshnessBinding = strings.Repeat("5", 64) }},
		{"wrong expiry", func(value *proofrun.TestResult) {
			value.FreshnessExpiresAt = now.Add(2 * time.Hour).Format(time.RFC3339Nano)
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			changed := result
			testCase.change(&changed)
			if _, err := validateTestingAttemptOwnersAt(f.root, changed, true, now); err == nil {
				t.Fatal("fresh receipt accepted an unmatched producer")
			}
		})
	}
	expired := attempt
	expired.FreshnessExpiresAt = now.Format(time.RFC3339Nano)
	encoded, err := json.Marshal(expired)
	if err != nil {
		t.Fatal(err)
	}
	path, err := proofrun.AttemptPath(f.root, attempt.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	result.FreshnessExpiresAt = expired.FreshnessExpiresAt
	if _, err := validateTestingAttemptOwnersAt(f.root, result, true, now); err == nil {
		t.Fatal("expired native producer satisfied fresh receipt")
	}
}

func TestGLEPathTestingReceiptPostureReadsLiteralManifest(t *testing.T) {
	t.Parallel()
	projectRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	installationRoot := filepath.Join(projectRoot, "metasystem")
	paths := []string{"metasystem/inputs/[literal].go", "metasystem/metasystem.conf", "metasystem/testing.json"}
	before := map[string][]byte{
		paths[0]: []byte("candidate\n"), paths[1]: []byte("testing.contract=testing.json\n"), paths[2]: []byte("{}\n"),
	}
	for path, data := range before {
		full := filepath.Join(projectRoot, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	after := map[string][]byte{}
	for path, data := range before {
		after[path] = bytes.Clone(data)
	}
	after[paths[0]] = []byte("working edit\n")
	raw := &literalReceiptRaw{t: t, root: projectRoot, paths: paths, before: before, after: after,
		candidate: receiptFactID(before), edited: receiptFactID(after)}
	workspace := gittree.Workspace{Dir: projectRoot, RawSource: raw.answer}
	result := proofrun.TestResult{ProjectRoot: projectRoot, CandidateTree: raw.candidate,
		Groups: []proofrun.GroupResult{{InputManifest: []string{pathpattern.EncodeLiteral(paths[0])}}}}
	index, working, err := testingReceiptPostureWithWorkspace(installationRoot, result, workspace)
	if err != nil || index != raw.candidate || working != raw.candidate {
		t.Fatalf("clean literal input posture: index=%s working=%s err=%v", index, working, err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, filepath.FromSlash(paths[0])), after[paths[0]], 0o644); err != nil {
		t.Fatal(err)
	}
	index, working, err = testingReceiptPostureWithWorkspace(installationRoot, result, workspace)
	if err != nil || index != raw.candidate || working != raw.edited || working == raw.candidate {
		t.Fatalf("receipt posture missed literal input drift: index=%s working=%s err=%v", index, working, err)
	}
	if raw.calls != 20 || raw.literalSelections != 2 || raw.editedReads != 1 {
		t.Fatalf("literal receipt raw transcript: calls=%d selectors=%d edited reads=%d", raw.calls, raw.literalSelections, raw.editedReads)
	}
}

// literalReceiptRaw declares only the raw repository answers used by the
// receipt posture. The owner's real staged and relevant-snapshot code chooses
// the literal paths; the raw source verifies those requests against files.
type literalReceiptRaw struct {
	t                  *testing.T
	root               string
	paths              []string
	before, after      map[string][]byte
	candidate, edited  string
	calls              int
	literalSelections  int
	editedReads        int
	staged, projection string
}

func (f *literalReceiptRaw) answer(request gittree.RawRequest) gittree.RawResult {
	f.t.Helper()
	pins := []string{
		"-c", "core.fileMode=true", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false",
		"-c", "apply.ignoreWhitespace=no", "-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false",
		"-c", "gc.auto=0", "-c", "maintenance.auto=false",
	}
	prefix := append([]string{"-C", f.root}, pins...)
	if request.Dir != f.root || len(request.Args) < len(prefix) || !slices.Equal(request.Args[:len(prefix)], prefix) {
		f.t.Fatalf("unexpected raw receipt root or pins: %q %q", request.Dir, request.Args)
	}
	args := request.Args[len(prefix):]
	if request.Operation != "git "+strings.Join(args, " ") {
		f.t.Fatalf("raw receipt operation %q for %q", request.Operation, args)
	}
	private := ""
	for _, entry := range request.Env {
		if strings.HasPrefix(entry, "GIT_INDEX_FILE=") {
			private = strings.TrimPrefix(entry, "GIT_INDEX_FILE=")
		}
	}
	if private != "" && (filepath.Base(private) != "index" || !filepath.IsAbs(private)) {
		f.t.Fatalf("invalid private index path %q", private)
	}
	phase, step := f.calls/10, f.calls%10
	if phase > 1 {
		f.t.Fatalf("extra raw receipt call %q", args)
	}
	want := func(expected ...string) {
		if !slices.Equal(args, expected) {
			f.t.Fatalf("raw receipt call %d = %q, want %q", f.calls, args, expected)
		}
	}
	answer := func(value string) gittree.RawResult { return gittree.RawResult{Stdout: []byte(value)} }
	entries := func(format string) string {
		var out strings.Builder
		for _, path := range f.paths {
			fmt.Fprintf(&out, format, chainBlobOID(f.before[path]), path)
		}
		return out.String()
	}
	var result gittree.RawResult
	switch step {
	case 0:
		want("ls-files", "--stage", "-z")
		result = answer(entries("100644 %s 0\t%s\x00"))
	case 1:
		want("read-tree", "--empty")
		f.staged = private
	case 2:
		want("rev-parse", "--show-toplevel")
		result = answer(f.root + "\n")
	case 3:
		want("update-index", "-z", "--index-info")
		if private != f.staged || !bytes.Equal(request.Stdin, []byte(entries("100644 %s 0\t%s\x00"))) {
			f.t.Fatalf("staged index or entries changed: %q", request.Stdin)
		}
	case 4:
		want("write-tree")
		if private != f.staged {
			f.t.Fatal("staged write used another index")
		}
		result = answer(f.candidate + "\n")
	case 5:
		want("rev-parse", "--show-prefix")
		result = answer("")
	case 6:
		want(append([]string{"--literal-pathspecs", "ls-tree", "-r", "-z", "--full-tree", f.candidate, "--"}, f.paths...)...)
		f.literalSelections++
		result = answer(entries("100644 blob %s\t%s\x00"))
	case 7:
		want("read-tree", f.candidate)
		f.projection = private
	case 8:
		want(append([]string{"add", "-A", "-f", "--"}, f.paths...)...)
		if private != f.projection {
			f.t.Fatal("relevant add used another index")
		}
		for _, path := range f.paths {
			data, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(path)))
			wantData := f.before[path]
			if phase == 1 {
				wantData = f.after[path]
			}
			if err != nil || !bytes.Equal(data, wantData) {
				f.t.Fatalf("selected input %q = %q, %v; want %q", path, data, err, wantData)
			}
			if phase == 1 && path == f.paths[0] {
				f.editedReads++
			}
		}
	case 9:
		want("write-tree")
		if private != f.projection {
			f.t.Fatal("relevant write used another index")
		}
		if phase == 0 {
			result = answer(f.candidate + "\n")
		} else {
			result = answer(f.edited + "\n")
		}
	}
	if (step == 0 || step == 2 || step == 5 || step == 6) != (private == "") {
		f.t.Fatalf("raw receipt private index mismatch at call %d: %q", f.calls, private)
	}
	f.calls++
	return result
}

func TestSchemaTwoReceiptRequiresSuccessfulTerminalGroupOwners(t *testing.T) {
	f := newSchemaTwoReceiptFixture(t)
	defer f.assertConsumed()
	projectRoot := f.repository
	workspace := gittree.Workspace{Dir: projectRoot, RawSource: f.raw}
	readWorkspace := gittree.Workspace{Dir: f.root, RawSource: f.raw}
	tree, err := workspace.StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	subtree, err := readWorkspace.TreeOf(tree)
	if err != nil {
		t.Fatal(err)
	}
	head := strings.Repeat("b", 40)
	identity, err := proofrun.BuildProofIdentity(f.root, filepath.Join(f.root, "metasystem.conf"), "selected", "testing", nil, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	episode, binding := strings.Repeat("1", 64), strings.Repeat("2", 64)
	expires := now.Add(time.Hour)
	request := candidateProofAdmission(proofrun.AdmissionRequest{ControlRoot: f.root, ExecutionRoot: f.root,
		GoalID: "goal", GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 5, Identity: identity, Launcher: launcher, Now: now,
		SharedComponents: true, ComponentIdentities: map[string]string{"application": strings.Repeat("a", 64)}, FreshGroups: map[string]bool{"application": true},
		FreshnessEpisode: episode, FreshnessBinding: binding, FreshnessExpiresAt: expires.Format(time.RFC3339Nano)}, tree)
	request = proofrun.WithTestHostAdmissionDirectory(request, filepath.Join(t.TempDir(), "host-admission"))
	attempt, decision, err := proofrun.ReserveLocked(request)
	if err != nil {
		t.Fatal(err)
	}
	requireProofReservationNotAdmissionRefused(t, decision)
	zero, admissionMaximum := 0, 0
	digest := strings.Repeat("a", 64)
	result := proofrun.TestResult{SchemaVersion: proofrun.TestResultSchemaVersion,
		WorkerPolicyVersion: proofrun.TestWorkerPolicyVersion, Workers: 1, AdmissionMaximum: &admissionMaximum,
		CandidateEngineIdentityVersion: proofrun.CandidateEngineIdentitySchemaVersion, AttemptID: attempt.AttemptID,
		FreshnessEpisode: episode, FreshnessBinding: binding, FreshnessExpiresAt: expires.Format(time.RFC3339Nano), FreshGroups: map[string]bool{"application": true},
		Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeAuto, RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard,
		ProjectRoot: projectRoot, BaseCommit: head, CandidateTree: tree, PolicyBaseCommit: head,
		ContractDigest: digest, BaseContractDigest: digest, PolicyEngineDigest: digest, CandidateEngineDigest: strings.Repeat("e", 64),
		CandidateEngineBuildIdentity: strings.Repeat("f", 40), BehaviorPolicyDigest: digest, PlanDigest: digest,
		RequiredGroups: []string{"application"}, SelectedGroups: []string{"application"}, LaunchCounts: proofrun.LaunchCounts{Test: 1, CountsComplete: true},
		StartedAt: now.Add(-2 * time.Second).Format(time.RFC3339Nano), Cost: proofrun.TestCost{DeclaredTargetMS: 1}, Groups: []proofrun.GroupResult{{ID: "application", Kind: "unit", Obligations: []string{"behavior"}, IdentityVersion: proofrun.GroupExecutionIdentityVersion,
			InputDigest: digest, InputManifest: []string{"source/**"}, ExecutionIdentity: digest, CWD: ".", ToolIdentities: map[string]string{},
			Status: "passed", NativeLaunched: true, NativeExitStatus: &zero, CollectionComplete: true, ReportDigests: map[string]string{}}}}
	result.RecomputeDelivery()
	f.write("records/narrator-digest.log", "ordinary append\n")
	completedAt := now.Add(time.Second)
	preparedReceipt, payload, err := prepareTestingReceiptPayloadWithWorkspace(f.root, tree, result, completedAt, workspace)
	if err != nil || preparedReceipt.Time != completedAt.Format(time.RFC3339Nano) {
		t.Fatalf("prepare atomic schema-2 receipt: receipt=%+v err=%v", preparedReceipt, err)
	}
	if preparedReceipt.Testing == nil || preparedReceipt.Testing.Cost.PublicationDurationMS < 1 ||
		preparedReceipt.Testing.Cost.ActualDurationMS < 2000 || preparedReceipt.Testing.EndedAt == "" {
		t.Fatalf("schema-2 terminal preparation did not retain whole-command timing: %+v", preparedReceipt.Testing)
	}
	if _, err := publishCommittedReceiptAtWithWorkspace(f.root, attempt.AttemptID, tree, now, workspace); err == nil {
		t.Fatal("live schema-2 attempt published before terminal success")
	}
	if _, err := proofrun.FinalizeAttemptWithTestResultLocked(f.root, attempt.AttemptID, proofrun.TerminalSuccess, 0, "fixture", payload, preparedReceipt.Testing, completedAt); err != nil {
		t.Fatal(err)
	}
	published, err := publishCommittedReceiptAtWithWorkspace(f.root, attempt.AttemptID, tree, now, workspace)
	if err != nil || published.Time != preparedReceipt.Time {
		t.Fatalf("publish atomic schema-2 receipt: receipt=%+v err=%v", published, err)
	}
	committed := append([]byte(nil), payload...)
	published, err = publishCommittedReceiptAtWithWorkspace(f.root, attempt.AttemptID, tree, expires.Add(-time.Nanosecond), workspace)
	if err != nil || published.Time != preparedReceipt.Time {
		t.Fatalf("publish immediately before owner expiry: receipt=%+v err=%v", published, err)
	}
	if projected, readErr := os.ReadFile(TestReceiptPath(f.root, tree)); readErr != nil ||
		!bytes.Equal(bytes.TrimSpace(projected), bytes.TrimSpace(committed)) {
		t.Fatalf("expiry-aware publication changed committed bytes: err=%v", readErr)
	}
	if _, err := publishCommittedReceiptAtWithWorkspace(f.root, attempt.AttemptID, tree, expires, workspace); err == nil {
		t.Fatal("publication accepted an owner at its exact expiry")
	}
	projected, err := os.ReadFile(TestReceiptPath(f.root, tree))
	if err != nil || !bytes.Equal(bytes.TrimSpace(projected), bytes.TrimSpace(payload)) {
		t.Fatalf("schema-2 projection changed committed payload: err=%v\nprojected=%s\npayload=%s", err, projected, payload)
	}
	receipt, err := createTestingReceiptAtWithWorkspace(f.root, tree, result, expires.Add(-time.Nanosecond), workspace)
	if err != nil || receipt.SchemaVersion != 2 || len(receipt.AttemptIDs) != 1 || !fullReceiptCommandAccepted(receipt) {
		t.Fatalf("schema-2 receipt=%+v err=%v", receipt, err)
	}
	if receipt.PolicyEngineDigest != result.PolicyEngineDigest || receipt.CandidateEngineDigest != result.CandidateEngineDigest ||
		receipt.CandidateEngineBuildIdentity != result.CandidateEngineBuildIdentity || receipt.ProvedTree != tree {
		t.Fatalf("schema-2 receipt lost policy engine, candidate engine, or candidate tree: %+v", receipt)
	}
	if _, err := createTestingReceiptAtWithWorkspace(f.root, tree, result, expires, workspace); err == nil {
		t.Fatal("composition accepted an owner at its exact expiry")
	}
	if _, err := readTestReceiptWithWorkspace(ObserveParams{RepoRoot: f.root, CandidateTree: subtree, TestReceipt: TestReceiptPath(f.root, tree), Now: expires.Add(-time.Nanosecond)}, readWorkspace); err != nil {
		t.Fatalf("schema-2 receipt consumer: %v", err)
	}
	if _, err := readTestReceiptWithWorkspace(ObserveParams{RepoRoot: f.root, CandidateTree: subtree, TestReceipt: TestReceiptPath(f.root, tree), Now: expires}, readWorkspace); err == nil {
		t.Fatal("receipt read accepted an owner at its exact expiry")
	}
	writeReceipt := func(t *testing.T, value TestReceipt) {
		t.Helper()
		encoded, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(TestReceiptPath(f.root, tree), append(encoded, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	baseVerified := *receipt.Testing
	verify := func(value proofrun.TestResult, verifyErr error) func() (proofrun.TestResult, error) {
		return func() (proofrun.TestResult, error) { return value, verifyErr }
	}
	stageCandidate := func(t *testing.T, paths map[string]string) (string, string) {
		t.Helper()
		for path, content := range paths {
			f.stage(path, content)
		}
		whole, err := workspace.StagedTree()
		if err != nil {
			t.Fatal(err)
		}
		installationTree, err := readWorkspace.TreeOf(whole)
		if err != nil {
			t.Fatal(err)
		}
		return whole, installationTree
	}
	unstageCandidate := func(t *testing.T, paths ...string) {
		t.Helper()
		for _, path := range paths {
			f.unstage(path)
		}
	}

	t.Run("preparation and publication accept the judged index across goal-ledger motion", func(t *testing.T) {
		whole, _ := stageCandidate(t, map[string]string{"plans/goals/x.md": "goal revision\n"})
		defer unstageCandidate(t, "plans/goals/x.md")
		if _, _, err := prepareTestingReceiptPayloadWithWorkspace(f.root, tree, result, completedAt, workspace); err != nil {
			t.Fatalf("workspace-equivalent index invalidated receipt preparation: %v", err)
		}
		if _, err := publishCommittedReceiptAtWithWorkspace(f.root, attempt.AttemptID, whole, now, workspace); err != nil {
			t.Fatalf("publication refused the accepted index tree: %v", err)
		}
		if _, err := publishCommittedReceiptAtWithWorkspace(f.root, attempt.AttemptID, tree, time.Now().UTC(), workspace); err == nil {
			t.Fatal("publication accepted an index that moved after the reuse decision")
		}
	})

	t.Run("workspace path accepts goal-ledger motion", func(t *testing.T) {
		writeReceipt(t, receipt)
		whole, installationTree := stageCandidate(t, map[string]string{
			"plans/goals/x.md": "goal revision\n",
			"plans/goals.md":   "legacy goal revision\n",
		})
		defer unstageCandidate(t, "plans/goals/x.md", "plans/goals.md")
		current := baseVerified
		current.CandidateTree = whole
		if _, err := readTestReceiptWithWorkspace(ObserveParams{RepoRoot: f.root, CandidateTree: installationTree,
			TestReceipt: TestReceiptPath(f.root, tree), Now: now, VerifyTesting: verify(current, nil)}, readWorkspace); err != nil {
			t.Fatalf("workspace-equivalent receipt was refused: %v", err)
		}
	})

	t.Run("execution-identity key accepts an unrelated records path", func(t *testing.T) {
		writeReceipt(t, receipt)
		keyWhole, keyInstallation := stageCandidate(t, map[string]string{"records/misc/x.md": "record revision\n"})
		defer unstageCandidate(t, "records/misc/x.md")
		current := baseVerified
		current.CandidateTree = keyWhole
		if _, err := readTestReceiptWithWorkspace(ObserveParams{RepoRoot: f.root, CandidateTree: keyInstallation,
			TestReceipt: TestReceiptPath(f.root, tree), Now: now, VerifyTesting: verify(current, nil)}, readWorkspace); err != nil {
			t.Fatalf("identity-equivalent receipt was refused: %v", err)
		}
	})

	t.Run("execution-identity key names a differing group", func(t *testing.T) {
		writeReceipt(t, receipt)
		whole, installationTree := stageCandidate(t, map[string]string{"records/misc/x.md": "record revision\n"})
		defer unstageCandidate(t, "records/misc/x.md")
		current := baseVerified
		current.CandidateTree = whole
		current.Groups = append([]proofrun.GroupResult(nil), current.Groups...)
		current.Groups[0].ExecutionIdentity = strings.Repeat("f", 64)
		_, err := readTestReceiptWithWorkspace(ObserveParams{RepoRoot: f.root, CandidateTree: installationTree,
			TestReceipt: TestReceiptPath(f.root, tree), VerifyTesting: verify(current, nil)}, readWorkspace)
		if err == nil || !strings.Contains(err.Error(), "application") {
			t.Fatalf("differing execution identity did not name its group: %v", err)
		}
	})

	for _, test := range []struct {
		name, want string
		result     proofrun.TestResult
		err        error
	}{
		{name: "verify core error", want: "testing-receipt: verify core failed: unavailable", result: baseVerified, err: fmt.Errorf("unavailable")},
		{name: "insufficient result", want: "testing-receipt: retained proof is not sufficient for the index; missing groups: application", result: func() proofrun.TestResult {
			value := baseVerified
			value.Delivery.Sufficient = false
			value.Delivery.MissingGroups = []string{"application"}
			return value
		}()},
		{name: "other tree", want: "testing-receipt: verify core judged tree", result: func() proofrun.TestResult {
			keyWhole, _ := stageCandidate(t, map[string]string{"records/misc/step-zero-other-tree.md": "other tree\n"})
			unstageCandidate(t, "records/misc/step-zero-other-tree.md")
			value := baseVerified
			value.CandidateTree = keyWhole
			return value
		}()},
	} {
		t.Run(test.name+" refuses before exact coverage", func(t *testing.T) {
			writeReceipt(t, receipt)
			_, err := readTestReceiptWithWorkspace(ObserveParams{RepoRoot: f.root, CandidateTree: subtree,
				TestReceipt: TestReceiptPath(f.root, tree), VerifyTesting: verify(test.result, test.err)}, readWorkspace)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("step-zero refusal = %v, want %q", err, test.want)
			}
		})
	}

	t.Run("workspace tree is recomputed", func(t *testing.T) {
		tampered := receipt
		projection := *receipt.Workspace
		projection.Tree = strings.Repeat("0", 40)
		tampered.Workspace = &projection
		writeReceipt(t, tampered)
		if _, err := readTestReceiptWithWorkspace(ObserveParams{RepoRoot: f.root, CandidateTree: subtree,
			TestReceipt: TestReceiptPath(f.root, tree), VerifyTesting: verify(baseVerified, nil)}, readWorkspace); err == nil {
			t.Fatal("receipt with a false workspace tree was accepted")
		}
	})

	t.Run("workspace exclusions must equal the engine list", func(t *testing.T) {
		tampered := receipt
		projection := *receipt.Workspace
		projection.Excludes = append([]string(nil), projection.Excludes...)
		projection.Excludes = projection.Excludes[1:]
		tampered.Workspace = &projection
		writeReceipt(t, tampered)
		if _, err := readTestReceiptWithWorkspace(ObserveParams{RepoRoot: f.root, CandidateTree: subtree,
			TestReceipt: TestReceiptPath(f.root, tree), VerifyTesting: verify(baseVerified, nil)}, readWorkspace); err == nil {
			t.Fatal("receipt with a different workspace exclusion list was accepted")
		}
	})

	t.Run("receipt without workspace keeps exact coverage and skips verification", func(t *testing.T) {
		legacy := receipt
		legacy.Workspace = nil
		writeReceipt(t, legacy)
		called := false
		if _, err := readTestReceiptWithWorkspace(ObserveParams{RepoRoot: f.root, CandidateTree: subtree,
			TestReceipt: TestReceiptPath(f.root, tree), Now: now, VerifyTesting: func() (proofrun.TestResult, error) {
				called = true
				return proofrun.TestResult{}, fmt.Errorf("must not be called")
			}}, readWorkspace); err != nil {
			t.Fatalf("legacy exact receipt was refused: %v", err)
		}
		if called {
			t.Fatal("receipt without workspace called the verify core")
		}
	})
	writeReceipt(t, receipt)
	legacyPayload := legacyTestingReceiptPayload(t, receipt)
	t.Run("landing observe and tier-one read an unmarked schema-2 payload", func(t *testing.T) {
		if err := os.WriteFile(TestReceiptPath(f.root, tree), legacyPayload, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readTestReceiptWithWorkspace(ObserveParams{RepoRoot: f.root, CandidateTree: subtree, TestReceipt: TestReceiptPath(f.root, tree), Now: now}, readWorkspace); err != nil {
			t.Fatalf("shared landing receipt reader rejected an old-format schema-2 payload: %v", err)
		}
	})
	t.Run("committed receipt recovery reads an unmarked schema-2 payload", func(t *testing.T) {
		attemptPath, err := proofrun.AttemptPath(f.root, attempt.AttemptID)
		if err != nil {
			t.Fatal(err)
		}
		originalAttempt, err := os.ReadFile(attemptPath)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if restoreErr := os.WriteFile(attemptPath, originalAttempt, 0o600); restoreErr != nil {
				t.Errorf("restore current-format attempt: %v", restoreErr)
			}
		}()
		retained, err := proofrun.ReadAttempt(f.root, attempt.AttemptID)
		if err != nil {
			t.Fatal(err)
		}
		var legacyReceipt TestReceipt
		if err := json.Unmarshal(legacyPayload, &legacyReceipt); err != nil {
			t.Fatal(err)
		}
		retained.TestResult = legacyReceipt.Testing
		retained.DeliveryReceipt = nil
		retained.DeliveryReceiptBytes = append([]byte(nil), legacyPayload...)
		encodedAttempt, err := json.MarshalIndent(retained, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(attemptPath, append(encodedAttempt, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		published, err := publishCommittedReceiptAtWithWorkspace(f.root, attempt.AttemptID, tree, now, workspace)
		if err != nil || published.Testing == nil || published.Testing.CandidateEngineIdentityVersion != 0 {
			t.Fatalf("committed receipt recovery rejected an old-format schema-2 payload: receipt=%+v err=%v", published, err)
		}
	})
	failed := result
	failed.AttemptID = "missing-attempt"
	if _, err := createTestingReceiptAtWithWorkspace(f.root, tree, failed, time.Now().UTC(), workspace); err == nil || !strings.Contains(err.Error(), "no successful terminal outer attempt") {
		t.Fatalf("receipt projection accepted a group without terminal outer authority: %v", err)
	}
	f.writeOutside("source/changed.go", "changed\n")
	if _, _, err := prepareTestingReceiptPayloadWithWorkspace(f.root, tree, result, completedAt, workspace); err == nil {
		t.Fatal("relevant source mutation accepted")
	}
	if _, err := publishCommittedReceiptAtWithWorkspace(f.root, attempt.AttemptID, tree, time.Now().UTC(), workspace); err == nil {
		t.Fatal("recovery accepted relevant source mutation")
	}
}

func legacyTestingReceiptPayload(t *testing.T, receipt TestReceipt) []byte {
	t.Helper()
	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	delete(payload, "policyEngineDigest")
	delete(payload, "candidateEngineDigest")
	testingPayload, ok := payload["testing"].(map[string]any)
	if !ok {
		t.Fatal("schema-2 fixture has no nested testing payload")
	}
	delete(testingPayload, "candidateEngineIdentityVersion")
	delete(testingPayload, "candidateEngineDigest")
	encoded, err = json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestAdoptionRulingsPreserveApplicationAndLandingAuthority(t *testing.T) {
	t.Parallel()
	source := newRepositoryObservationFixture(t)
	target := t.TempDir()
	data, err := AdoptionRulings(source.root, target)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "| R-1 |") || !strings.Contains(string(data), "| R-35-m0 |") || !strings.Contains(string(data), "| R-54-m1 |") {
		t.Fatalf("fresh register has incorrect authority rows: %s", data)
	}
	path := filepath.Join(target, "memory", "rulings.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	custom := "# Application history\n\n| R-900-app | tailored application ruling |\n"
	if err := os.WriteFile(path, []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err = AdoptionRulings(source.root, target)
	if err != nil || !strings.HasPrefix(string(data), custom) {
		t.Fatalf("existing application memory changed: %s, %v", data, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	repeated, err := AdoptionRulings(source.root, target)
	if err != nil || string(repeated) != string(data) {
		t.Fatalf("re-adoption duplicates or changes rulings: %s, %v", repeated, err)
	}
	conflict := custom + "| R-35-m0 | different application authority |\n"
	if err := os.WriteFile(path, []byte(conflict), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := AdoptionRulings(source.root, target); err == nil {
		t.Fatal("conflicting authority was accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != conflict {
		t.Fatalf("conflicting target was modified: %s, %v", after, err)
	}
}

// A delivery retry reuses a group that passed in a predecessor which then
// failed at another group (R-96-m1e); the receipt accepts that owner for the
// delivery purpose alone, and only because the owner's own record of the
// group is a complete pass.
func TestSchemaTwoReceiptAcceptsReuseFromAFailedDeliveryPredecessor(t *testing.T) {
	f := newSchemaTwoReceiptFixture(t)
	f.manifestSelectors = []string{"application/**", "later/**"}
	t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", filepath.Join(t.TempDir(), "host-admission"))
	t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", f.root)
	projectRoot := f.repository
	workspace := gittree.Workspace{Dir: projectRoot, RawSource: f.raw}
	tree := f.remember(f.top(f.index))
	installationTree, err := (gittree.Workspace{Dir: f.root, RawSource: f.raw}).TreeOf(tree)
	if err != nil {
		t.Fatal(err)
	}
	head := observeBaseTree
	identity, err := proofrun.BuildProofIdentity(f.root, filepath.Join(f.root, "metasystem.conf"), "selected", "testing", nil, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	// The retry is a new attempt with its own proof identity; the same
	// identity after a failed terminal would answer retry-required instead.
	reserve := func(identity proofrun.ProofIdentity) string {
		t.Helper()
		attempt, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{ControlRoot: f.root, ExecutionRoot: f.root,
			GoalID: "goal", GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 5, Identity: identity, Launcher: launcher, Now: now}, tree))
		if err != nil {
			t.Fatal(err)
		}
		requireProofReservationNotAdmissionRefused(t, decision)
		return attempt.AttemptID
	}
	zero, failedExit, admissionMaximum := 0, 24, 0
	digest := strings.Repeat("a", 64)
	group := func(id, status string, exit *int) proofrun.GroupResult {
		identityByte := id[:1]
		if id == "later" {
			identityByte = "b"
		}
		return proofrun.GroupResult{ID: id, Kind: "unit", Obligations: []string{id}, IdentityVersion: proofrun.GroupExecutionIdentityVersion,
			InputDigest: digest, InputManifest: []string{id + "/**"}, ExecutionIdentity: strings.Repeat(identityByte, 64), CWD: ".", ToolIdentities: map[string]string{},
			Status: status, NativeLaunched: true, NativeExitStatus: exit, CollectionComplete: true, ReportDigests: map[string]string{}}
	}
	resultFor := func(attemptID string, purpose testpolicy.Purpose, groups []proofrun.GroupResult) proofrun.TestResult {
		result := proofrun.TestResult{SchemaVersion: proofrun.TestResultSchemaVersion,
			WorkerPolicyVersion: proofrun.TestWorkerPolicyVersion, Workers: 1, AdmissionMaximum: &admissionMaximum,
			CandidateEngineIdentityVersion: proofrun.CandidateEngineIdentitySchemaVersion, AttemptID: attemptID,
			Purpose: purpose, RequestedMode: testpolicy.ModeAuto, RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard,
			ProjectRoot: projectRoot, BaseCommit: head, CandidateTree: tree, PolicyBaseCommit: head,
			ContractDigest: digest, BaseContractDigest: digest, PolicyEngineDigest: digest, CandidateEngineDigest: strings.Repeat("e", 64),
			CandidateEngineBuildIdentity: strings.Repeat("f", 40), BehaviorPolicyDigest: digest, PlanDigest: digest,
			RequiredGroups: []string{"application", "later"}, SelectedGroups: []string{"application", "later"}, LaunchCounts: proofrun.LaunchCounts{Test: 2, CountsComplete: true},
			StartedAt: now.Add(-2 * time.Second).Format(time.RFC3339Nano), Cost: proofrun.TestCost{DeclaredTargetMS: 1}, Groups: groups}
		result.RecomputeDelivery()
		return result
	}
	predecessor := reserve(identity)
	stopped := resultFor(predecessor, testpolicy.PurposeDelivery, []proofrun.GroupResult{group("application", "passed", &zero), group("later", "failed", &failedExit)})
	if _, err := proofrun.FinalizeAttemptWithTestResultLocked(f.root, predecessor, proofrun.TerminalFailed, 24, "stopped at later", nil, &stopped, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	retry := reserve(proofrun.BindIdentityInputs(identity, []string{"plan:retry"}))
	reused := group("application", "reused", nil)
	reused.NativeLaunched, reused.ReuseAttempt = false, predecessor
	result := resultFor(retry, testpolicy.PurposeDelivery, []proofrun.GroupResult{reused, group("later", "passed", &zero)})
	result.LaunchCounts = proofrun.LaunchCounts{Test: 1, ReusedTest: 1, CountsComplete: true}
	f.write("records/narrator-digest.log", "ordinary append\n")
	if _, _, err := prepareTestingReceiptPayloadWithWorkspace(f.root, tree, result, now.Add(2*time.Second), workspace); err != nil {
		t.Fatalf("delivery retry reusing a failed predecessor's pass was refused at the receipt: %v", err)
	}
	episode, binding := strings.Repeat("7", 64), strings.Repeat("8", 64)
	expires := now.Add(time.Hour)
	for _, attemptID := range []string{predecessor, retry} {
		stored, err := proofrun.ReadAttempt(f.root, attemptID)
		if err != nil {
			t.Fatal(err)
		}
		stored.FreshnessEpisode, stored.FreshnessBinding = episode, binding
		stored.FreshnessExpiresAt = expires.Format(time.RFC3339Nano)
		stored.TestFreshGroups = map[string]bool{"application": true, "later": true}
		stored.TestInventory = map[string]string{"application": strings.Repeat("a", 64), "later": strings.Repeat("b", 64)}
		if attemptID == predecessor {
			stored.TestAdmission = 1
			stored.TestOwned = map[string]string{"application": strings.Repeat("a", 64), "later": strings.Repeat("b", 64)}
		} else {
			stored.TestAdmission = 2
			stored.TestOwned = map[string]string{"later": strings.Repeat("b", 64)}
			stored.TestSources = map[string]string{"application": predecessor}
		}
		if stored.TestResult != nil {
			stored.TestResult.FreshnessEpisode, stored.TestResult.FreshnessBinding = episode, binding
			stored.TestResult.FreshnessExpiresAt = expires.Format(time.RFC3339Nano)
			stored.TestResult.FreshGroups = map[string]bool{"application": true, "later": true}
		}
		path, err := proofrun.AttemptPath(f.root, attemptID)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.MarshalIndent(stored, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
			t.Fatalf("stamp reused owner freshness: %v", err)
		}
		if _, err := proofrun.ReadAttempt(f.root, attemptID); err != nil {
			t.Fatalf("read freshness-stamped reused owner %s: %v", attemptID, err)
		}
	}
	result.FreshnessEpisode, result.FreshnessBinding = episode, binding
	result.FreshnessExpiresAt = expires.Format(time.RFC3339Nano)
	result.FreshGroups = map[string]bool{"application": true, "later": true}
	receipt, payload, err := prepareTestingReceiptPayloadWithWorkspace(f.root, tree, result, now.Add(2*time.Second), workspace)
	if err != nil {
		t.Fatalf("prepare exact-episode reused-owner receipt: %v", err)
	}
	if _, err := proofrun.FinalizeAttemptWithTestResultLocked(f.root, retry, proofrun.TerminalSuccess, 0, "retry passed", payload, receipt.Testing, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := publishCommittedReceiptAtWithWorkspace(f.root, retry, tree, expires.Add(-time.Nanosecond), workspace); err != nil {
		t.Fatalf("reused-owner publication before expiry: %v", err)
	}
	if _, err := publishCommittedReceiptAtWithWorkspace(f.root, retry, tree, expires, workspace); err == nil {
		t.Fatal("reused-owner publication accepted exact expiry")
	}
	if _, err := createTestingReceiptAtWithWorkspace(f.root, tree, result, expires.Add(-time.Nanosecond), workspace); err != nil {
		t.Fatalf("reused-owner composition before expiry: %v", err)
	}
	if _, err := createTestingReceiptAtWithWorkspace(f.root, tree, result, expires, workspace); err == nil {
		t.Fatal("reused-owner composition accepted exact expiry")
	}
	verify := func() (proofrun.TestResult, error) { return result, nil }
	if _, err := readTestReceiptWithWorkspace(ObserveParams{RepoRoot: f.root, CandidateTree: installationTree, TestReceipt: TestReceiptPath(f.root, tree), Now: expires.Add(-time.Nanosecond), VerifyTesting: verify}, workspace); err != nil {
		t.Fatalf("reused-owner read before expiry: %v", err)
	}
	if _, err := readTestReceiptWithWorkspace(ObserveParams{RepoRoot: f.root, CandidateTree: installationTree, TestReceipt: TestReceiptPath(f.root, tree), Now: expires, VerifyTesting: verify}, workspace); err == nil {
		t.Fatal("reused-owner read accepted exact expiry")
	}
	cadence := result
	cadence.Purpose = testpolicy.PurposeCadence
	if _, _, err := prepareTestingReceiptPayloadWithWorkspace(f.root, tree, cadence, now.Add(2*time.Second), workspace); err == nil || !strings.Contains(err.Error(), "reuses nothing") {
		t.Fatalf("cadence result carrying a reused group was accepted at the receipt: %v", err)
	}
	broken := result
	broken.Groups = []proofrun.GroupResult{reused, group("later", "passed", &zero)}
	broken.Groups[0].ExecutionIdentity = strings.Repeat("z", 64)
	if _, _, err := prepareTestingReceiptPayloadWithWorkspace(f.root, tree, broken, now.Add(2*time.Second), workspace); err == nil {
		t.Fatal("a reuse whose identity the failed predecessor never proved was accepted at the receipt")
	}
	f.nextPhase("complete")
}

// schemaTwoReceiptFixture names tree and blob facts while keeping proof attempts
// and receipt files on disk. Every raw answer is checked against the file phase.
type schemaTwoReceiptFixture struct {
	t                 *testing.T
	repository, root  string
	worktree, index   map[string][]byte
	outside           map[string][]byte
	trees             map[string]map[string][]byte
	indices           map[string]*schemaTwoPrivateIndex
	calls             map[string]int
	phase             string
	phaseCalls        map[string]map[string]int
	manifestSelectors []string
}

type schemaTwoPrivateIndex struct {
	files  map[string][]byte
	phase  string
	listed []byte
}

func newSchemaTwoReceiptFixture(t *testing.T) *schemaTwoReceiptFixture {
	t.Helper()
	base := newRepositoryObservationFixture(t)
	f := &schemaTwoReceiptFixture{t: t, repository: base.repository, root: base.root,
		worktree: schemaTwoCopy(base.baseFiles), index: schemaTwoCopy(base.baseFiles), outside: map[string][]byte{"development/metasystem-design.md": []byte("fixture\n")},
		trees: map[string]map[string][]byte{}, indices: map[string]*schemaTwoPrivateIndex{},
		calls: map[string]int{}, phase: "base", phaseCalls: map[string]map[string]int{}, manifestSelectors: []string{"source/**"}}
	f.write("metasystem.conf", "testing.contract=testing.json\nmetasystem.runtimes=fake\ndispatch.cap-max=120\n")
	f.index["metasystem.conf"] = bytes.Clone(f.worktree["metasystem.conf"])
	f.remember(f.top(f.index))
	f.checkFiles()
	return f
}

func schemaTwoCopy(files map[string][]byte) map[string][]byte {
	copy := make(map[string][]byte, len(files))
	for path, data := range files {
		copy[path] = bytes.Clone(data)
	}
	return copy
}

func (f *schemaTwoReceiptFixture) top(files map[string][]byte) map[string][]byte {
	top := map[string][]byte{"development/metasystem-design.md": []byte("fixture\n")}
	for path, data := range files {
		top["metasystem/"+path] = bytes.Clone(data)
	}
	return top
}

func (f *schemaTwoReceiptFixture) currentTop() map[string][]byte {
	top := f.top(f.worktree)
	for path, data := range f.outside {
		top[path] = bytes.Clone(data)
	}
	return top
}

func (f *schemaTwoReceiptFixture) remember(files map[string][]byte) string {
	id := receiptFactID(files)
	f.trees[id] = schemaTwoCopy(files)
	return id
}

func (f *schemaTwoReceiptFixture) write(path, content string) {
	f.t.Helper()
	file := filepath.Join(f.root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Chmod(file, 0o644); err != nil {
		f.t.Fatal(err)
	}
	f.worktree[path] = []byte(content)
}

func (f *schemaTwoReceiptFixture) writeOutside(path, content string) {
	f.t.Helper()
	file := filepath.Join(f.repository, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Chmod(file, 0o644); err != nil {
		f.t.Fatal(err)
	}
	f.outside[path] = []byte(content)
	if path == "source/changed.go" {
		f.nextPhase("source")
	}
}

func (f *schemaTwoReceiptFixture) stage(path, content string) {
	f.write(path, content)
	f.index[path] = bytes.Clone(f.worktree[path])
	if strings.HasPrefix(path, "plans/goals") {
		f.nextPhase("ledger")
	}
	if strings.HasPrefix(path, "records/misc/") {
		f.nextPhase("records")
	}
}

func (f *schemaTwoReceiptFixture) unstage(path string) {
	f.t.Helper()
	delete(f.index, path)
	delete(f.worktree, path)
	if err := os.Remove(filepath.Join(f.root, filepath.FromSlash(path))); err != nil {
		f.t.Fatal(err)
	}
	f.nextPhase("base")
}

func (f *schemaTwoReceiptFixture) nextPhase(phase string) {
	f.t.Helper()
	for path, state := range f.indices {
		if state.phase != "done" {
			f.t.Fatalf("private index %s stopped at %s", path, state.phase)
		}
	}
	f.indices = map[string]*schemaTwoPrivateIndex{}
	f.phase = phase
}

func (f *schemaTwoReceiptFixture) checkFiles() {
	f.t.Helper()
	for path, want := range f.worktree {
		full := filepath.Join(f.root, filepath.FromSlash(path))
		got, err := os.ReadFile(full)
		if err != nil || !bytes.Equal(got, want) {
			f.t.Fatalf("declared file %s = %q, %v; want %q", path, got, err, want)
		}
		info, err := os.Lstat(full)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o644 {
			f.t.Fatalf("declared mode %s = %v, %v", path, info, err)
		}
	}
	for path, want := range f.outside {
		got, err := os.ReadFile(filepath.Join(f.repository, filepath.FromSlash(path)))
		if err != nil || !bytes.Equal(got, want) {
			f.t.Fatalf("declared outside file %s = %q, %v", path, got, err)
		}
	}
}

func schemaTwoMatches(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if path == prefix || strings.HasPrefix(path, strings.TrimSuffix(prefix, "/")+"/") {
			return true
		}
	}
	return false
}

func schemaTwoPaths(files map[string][]byte, prefixes []string) []string {
	var paths []string
	for path := range files {
		if len(prefixes) == 0 || schemaTwoMatches(path, prefixes) {
			paths = append(paths, path)
		}
	}
	slices.Sort(paths)
	return paths
}

func schemaTwoNul(paths []string) []byte {
	if len(paths) == 0 {
		return nil
	}
	return []byte(strings.Join(paths, "\x00") + "\x00")
}

func (f *schemaTwoReceiptFixture) indexEntries() []byte {
	files := f.top(f.index)
	paths := schemaTwoPaths(files, nil)
	var entries bytes.Buffer
	for _, path := range paths {
		fmt.Fprintf(&entries, "100644 %s 0\t%s\x00", chainBlobOID(files[path]), path)
	}
	return entries.Bytes()
}

func (f *schemaTwoReceiptFixture) raw(request gittree.RawRequest) gittree.RawResult {
	f.t.Helper()
	f.checkFiles()
	pins := []string{"-c", "core.fileMode=true", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false",
		"-c", "apply.ignoreWhitespace=no", "-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false",
		"-c", "gc.auto=0", "-c", "maintenance.auto=false"}
	if request.Dir != f.root && request.Dir != f.repository {
		f.t.Fatalf("raw cwd = %q", request.Dir)
	}
	prefix := append([]string{"-C", request.Dir}, pins...)
	if len(request.Args) < len(prefix) || !slices.Equal(request.Args[:len(prefix)], prefix) {
		f.t.Fatalf("raw pins/cwd = %q", request.Args)
	}
	args := request.Args[len(prefix):]
	if request.Operation != "git "+strings.Join(args, " ") {
		f.t.Fatalf("raw operation = %q args=%q", request.Operation, args)
	}
	private := ""
	for _, entry := range request.Env {
		if strings.HasPrefix(entry, "GIT_INDEX_FILE=") {
			if private != "" {
				f.t.Fatal("duplicate private index")
			}
			private = strings.TrimPrefix(entry, "GIT_INDEX_FILE=")
		}
	}
	wantEnv := gittree.ScrubbedEnviron()
	if private != "" {
		if request.Dir != f.repository {
			f.t.Fatalf("private index cwd = %q", request.Dir)
		}
		if !filepath.IsAbs(private) || filepath.Base(private) != "index" {
			f.t.Fatalf("private index path = %q", private)
		}
		wantEnv = gittree.ScrubbedEnviron("GIT_INDEX_FILE=" + private)
	}
	if !reflect.DeepEqual(request.Env, wantEnv) {
		f.t.Fatal("raw environment differs from scrubbed environment")
	}
	if request.Stdin != nil && !(private != "" && request.Dir == f.repository &&
		(slices.Equal(args, []string{"update-index", "-z", "--index-info"}) || slices.Equal(args, []string{"update-index", "-z", "--force-remove", "--stdin"}))) {
		f.t.Fatalf("unexpected raw stdin for %q", args)
	}
	answer := func(id string) gittree.RawResult { return gittree.RawResult{Stdout: []byte(id + "\n")} }
	f.calls[request.Operation]++
	if f.phaseCalls[f.phase] == nil {
		f.phaseCalls[f.phase] = map[string]int{}
	}
	f.phaseCalls[f.phase][request.Operation]++
	state, known := f.indices[private]
	wantInputs := []string{"metasystem/metasystem.conf", "metasystem/testing.json"}
	for _, selector := range f.manifestSelectors {
		if !strings.HasSuffix(selector, "/**") {
			f.t.Fatalf("undeclared manifest selector %q", selector)
		}
		wantInputs = append(wantInputs, strings.TrimSuffix(selector, "/**"))
	}
	slices.Sort(wantInputs)
	switch {
	case slices.Equal(args, []string{"rev-parse", "--show-toplevel"}):
		return answer(f.repository)
	case slices.Equal(args, []string{"rev-parse", "--show-prefix"}):
		if request.Dir == f.root {
			return answer("metasystem/")
		}
		return answer("")
	case private == "" && request.Dir == f.root && len(args) == 2 && args[0] == "rev-parse" && strings.HasSuffix(args[1], "^{tree}"):
		id := strings.TrimSuffix(args[1], "^{tree}")
		if _, ok := f.trees[id]; !ok {
			f.t.Fatalf("unknown tree %s", id)
		}
		return answer(id)
	case private == "" && request.Dir == f.root && len(args) == 2 && args[0] == "rev-parse" && strings.HasSuffix(args[1], ":metasystem"):
		id := strings.TrimSuffix(args[1], ":metasystem")
		files, ok := f.trees[id]
		if !ok {
			f.t.Fatalf("unknown whole tree %s", id)
		}
		sub := map[string][]byte{}
		for path, data := range files {
			if strings.HasPrefix(path, "metasystem/") {
				sub[strings.TrimPrefix(path, "metasystem/")] = data
			}
		}
		return answer(f.remember(sub))
	case private == "" && request.Dir == f.repository && slices.Equal(args, []string{"ls-files", "--stage", "-z"}):
		return gittree.RawResult{Stdout: f.indexEntries()}
	case private == "" && request.Dir == f.repository && len(args) == 7+len(wantInputs) && slices.Equal(args[:5], []string{"--literal-pathspecs", "ls-tree", "-r", "-z", "--full-tree"}) && args[6] == "--" && slices.Equal(args[7:], wantInputs):
		files, ok := f.trees[args[5]]
		if !ok {
			f.t.Fatalf("unknown entries tree %s", args[5])
		}
		paths := schemaTwoPaths(files, args[7:])
		var entries bytes.Buffer
		for _, path := range paths {
			fmt.Fprintf(&entries, "100644 blob %s\t%s\x00", chainBlobOID(files[path]), path)
		}
		return gittree.RawResult{Stdout: entries.Bytes()}
	case private == "" && request.Dir == f.repository && len(args) == 10 && slices.Equal(args[:7], []string{"diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", "--ignore-submodules=none"}) && args[9] == "--":
		before, beforeOK := f.trees[args[7]]
		after, afterOK := f.trees[args[8]]
		if !beforeOK || !afterOK {
			f.t.Fatalf("undeclared changed-path trees %q", args)
		}
		all := map[string]bool{}
		for path := range before {
			all[path] = true
		}
		for path := range after {
			all[path] = true
		}
		var changed []string
		for path := range all {
			if !bytes.Equal(before[path], after[path]) {
				changed = append(changed, path)
			}
		}
		slices.Sort(changed)
		if f.phase != "source" || !slices.Equal(changed, []string{"source/changed.go"}) {
			f.t.Fatalf("undeclared changed paths in phase %s: %q", f.phase, changed)
		}
		return gittree.RawResult{Stdout: schemaTwoNul(changed)}
	case private != "" && (!known || state.phase == "done") && len(args) == 2 && args[0] == "read-tree" && (args[1] == "--empty" || f.trees[args[1]] != nil):
		files := map[string][]byte{}
		if args[1] != "--empty" {
			files = schemaTwoCopy(f.trees[args[1]])
		}
		f.indices[private] = &schemaTwoPrivateIndex{files: files, phase: "seeded"}
		return gittree.RawResult{}
	case private != "" && known && state.phase == "seeded" && request.Dir == f.repository && slices.Equal(args, []string{"update-index", "-z", "--index-info"}) && bytes.Equal(request.Stdin, f.indexEntries()):
		state.files = f.top(f.index)
		if f.phase == "source" {
			if _, present := state.files["source/changed.go"]; present {
				f.t.Fatal("relevant source edit entered the staged index")
			}
		}
		state.phase = "ready"
		return gittree.RawResult{}
	case private != "" && known && state.phase == "seeded" && request.Dir == f.repository && len(args) >= 5 && slices.Equal(args[:4], []string{"add", "-A", "-f", "--"}) && (slices.Equal(args[4:], []string{"metasystem/metasystem.conf"}) || slices.Equal(args[4:], []string{"metasystem/metasystem.conf", "source"})):
		current := f.currentTop()
		for path := range state.files {
			if schemaTwoMatches(path, args[4:]) {
				delete(state.files, path)
			}
		}
		for path, data := range current {
			if schemaTwoMatches(path, args[4:]) {
				state.files[path] = bytes.Clone(data)
			}
		}
		state.phase = "ready"
		return gittree.RawResult{}
	case private != "" && known && state.phase == "seeded" && request.Dir == f.repository && len(args) >= 4 && slices.Equal(args[:3], []string{"ls-files", "-z", "--"}):
		want := append([]string(nil), WorkspaceExclusions()...)
		prefixed := make([]string, len(want))
		for i, path := range want {
			prefixed[i] = "metasystem/" + path
		}
		if !slices.Equal(args[3:], want) && !slices.Equal(args[3:], prefixed) {
			f.t.Fatalf("undeclared filter paths %q", args[3:])
		}
		state.listed = schemaTwoNul(schemaTwoPaths(state.files, args[3:]))
		if len(state.listed) == 0 {
			state.phase = "ready"
		} else {
			state.phase = "listed"
		}
		return gittree.RawResult{Stdout: state.listed}
	case private != "" && known && state.phase == "listed" && request.Dir == f.repository && slices.Equal(args, []string{"update-index", "-z", "--force-remove", "--stdin"}) && bytes.Equal(request.Stdin, state.listed):
		for _, path := range bytes.Split(bytes.TrimSuffix(state.listed, []byte{0}), []byte{0}) {
			delete(state.files, string(path))
		}
		state.phase = "ready"
		return gittree.RawResult{}
	case private != "" && known && state.phase == "ready" && slices.Equal(args, []string{"write-tree"}):
		state.phase = "done"
		return answer(f.remember(state.files))
	}
	f.t.Fatalf("undeclared raw schema-2 request: cwd=%q args=%q stdin=%q private=%q state=%+v", request.Dir, args, request.Stdin, private, state)
	return gittree.RawResult{}
}

func (f *schemaTwoReceiptFixture) assertConsumed() {
	f.t.Helper()
	for path, state := range f.indices {
		if state.phase != "done" {
			f.t.Errorf("private index %s stopped at %s", path, state.phase)
		}
	}
	for _, operation := range []string{"git ls-files --stage -z", "git read-tree --empty", "git update-index -z --index-info", "git write-tree", "git rev-parse --show-prefix",
		"git --literal-pathspecs ls-tree -r -z --full-tree", "git diff --name-only -z --no-renames --no-ext-diff --no-textconv --ignore-submodules=none"} {
		found := false
		for call, count := range f.calls {
			if strings.HasPrefix(call, operation) && count > 0 {
				found = true
			}
		}
		if !found {
			f.t.Errorf("required raw call %q not consumed", operation)
		}
	}
	for phase, prefix := range map[string]string{"base": "git ls-files --stage -z", "ledger": "git ls-files --stage -z", "records": "git ls-files --stage -z", "source": "git diff --name-only -z"} {
		found := false
		for call, count := range f.phaseCalls[phase] {
			if strings.HasPrefix(call, prefix) && count > 0 {
				found = true
			}
		}
		if !found {
			f.t.Errorf("phase %s did not consume %s", phase, prefix)
		}
	}
	if f.phaseCalls["source"]["git add -A -f -- metasystem/metasystem.conf source"] == 0 {
		f.t.Error("source edit did not reach the relevant worktree snapshot")
	}
}
