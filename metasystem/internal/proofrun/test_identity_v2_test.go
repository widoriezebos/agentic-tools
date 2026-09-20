package proofrun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestEffectiveGroupIdentitySeparatesPolicyFromExecution(t *testing.T) {
	t.Parallel()
	group := testpolicy.Group{ID: "a", Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"source.txt"},
		Obligations: []string{"old"}, Platforms: []string{"any"}, TargetMS: 1000, Argv: []string{"true"}, Format: "exit-status"}
	request := TestRunRequest{ContractDigest: strings.Repeat("a", 64), BaseContractDigest: strings.Repeat("b", 64),
		JudgeKey: "judge", BehaviorPolicyDigest: strings.Repeat("c", 64)}
	key := func(r TestRunRequest, g testpolicy.Group, input, env string, tools map[string]string) string {
		return groupExecutionIdentity(r, g, ".", input, env, tools, nil)
	}
	base := key(request, group, "input", "env", map[string]string{"tool": "one"})
	request.ContractDigest, request.BaseContractDigest = strings.Repeat("d", 64), strings.Repeat("e", 64)
	group.Obligations = []string{"new"}
	group.Platforms = []string{"linux"}
	if got := key(request, group, "input", "env", map[string]string{"tool": "one"}); got != base {
		t.Fatalf("unrelated policy edit changed A execution: %s -> %s", base, got)
	}
	for name, changed := range map[string]string{
		"inputs":      key(request, group, "changed", "env", map[string]string{"tool": "one"}),
		"environment": key(request, group, "input", "changed", map[string]string{"tool": "one"}),
		"tool":        key(request, group, "input", "env", map[string]string{"tool": "two"}),
	} {
		if changed == base {
			t.Fatalf("%s change retained A execution identity", name)
		}
	}
	group.Argv = []string{"false"}
	if key(request, group, "input", "env", map[string]string{"tool": "one"}) == base {
		t.Fatal("effective command change retained A execution identity")
	}
	group.Argv = []string{"true"}
	group.TargetMS++
	if key(request, group, "input", "env", map[string]string{"tool": "one"}) == base {
		t.Fatal("adapter-affecting target change retained A execution identity")
	}
}

func TestRetainedMetadataReconstructsAcrossUnrelatedContract(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	writeTestResultFile(t, filepath.Join(root, "source.txt"), []byte("first\n"), 0o644)
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "first")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	toolRoot := t.TempDir()
	versionPath := filepath.Join(toolRoot, "version")
	versionCalls := filepath.Join(toolRoot, "version-calls")
	toolPath := filepath.Join(toolRoot, "fixture-tool")
	writeTestResultFile(t, versionPath, []byte("v1\n"), 0o644)
	writeTestResultFile(t, toolPath, []byte("#!/bin/sh\nprintf x >> '"+versionCalls+"'\ncat '"+versionPath+"'\n"), 0o755)
	group := testpolicy.Group{ID: "a", Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"source.txt"},
		Obligations: []string{"a"}, Platforms: []string{"any"}, TargetMS: 1000, Argv: []string{"true"}, Format: "exit-status",
		ExternalInputs: []testpolicy.ExternalInput{{ID: "tool-version", Path: versionPath}},
		Tools:          []testpolicy.Tool{{ID: "fixture-tool", Executable: toolPath, VersionArgs: []string{"--version"}}}}
	plan := testpolicy.Plan{SelectedGroups: []string{"a"}}
	request := TestRunRequest{ProjectRoot: root, CandidateTree: tree, Contract: testpolicy.Contract{Groups: []testpolicy.Group{group}}, Plan: plan,
		Environment: os.Environ(), ContractDigest: strings.Repeat("a", 64), BaseContractDigest: strings.Repeat("b", 64),
		JudgeKey: "judge", BehaviorPolicyDigest: strings.Repeat("c", 64)}
	identities, prepared, launches, err := PrepareGroupExecutionIdentities(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if launches != 1 {
		t.Fatalf("tool version command launch count=%d, want 1", launches)
	}
	metadata := prepared["a"]
	probeCount := func() int {
		data, readErr := os.ReadFile(versionCalls)
		if readErr != nil {
			t.Fatal(readErr)
		}
		return len(data)
	}
	preparedProbeCount := probeCount()
	if preparedProbeCount == 0 {
		t.Fatal("metadata preparation did not execute the version probe")
	}
	result := NewTestResult(request)
	result.Groups = []GroupResult{{ID: "a", IdentityVersion: GroupExecutionIdentityVersion, ExecutionIdentity: identities["a"],
		InputDigest: metadata.InputDigest, InputManifest: []string{"source.txt"}, EnvironmentDigest: metadata.EnvironmentDigest,
		ToolIdentities: metadata.ToolIdentities, ExecutableDigests: metadata.ExecutableDigests, Argv: metadata.Argv,
		Expected: metadata.Expected, Status: "passed", CollectionComplete: true}}
	attempt := Attempt{AttemptID: "earlier", StartedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Terminal: &AttemptTerminal{Result: TerminalSuccess}, TestResult: &result}
	request.ContractDigest, request.BaseContractDigest = strings.Repeat("d", 64), strings.Repeat("e", 64)
	request.Contract.Groups[0].Obligations = []string{"current"}
	request.Contract.Groups = append(request.Contract.Groups, testpolicy.Group{ID: "b", Kind: "unit", Adapter: "command", CWD: ".",
		Inputs: []string{"other.txt"}, Obligations: []string{"b"}, Platforms: []string{"any"}, TargetMS: 1000,
		Argv: []string{"true"}, Format: "exit-status"})
	reconstructed, err := RevalidateRetainedGroupExecutionIdentities(context.Background(), request, []Attempt{attempt})
	if err != nil || reconstructed["a"] != identities["a"] {
		t.Fatalf("unrelated contract edit lost A metadata: current=%v original=%v err=%v", reconstructed, identities, err)
	}
	if got := probeCount(); got != preparedProbeCount {
		t.Fatalf("read-only revalidation launched version command: calls=%d want=%d", got, preparedProbeCount)
	}
	result.SchemaVersion = LegacyTestResultSchemaVersion
	legacy, err := RevalidateRetainedGroupExecutionIdentities(context.Background(), request, []Attempt{attempt})
	if err != nil || legacy["a"] == identities["a"] {
		t.Fatalf("legacy result was treated as identity-v2 reusable metadata: current=%v err=%v", legacy, err)
	}
	result.SchemaVersion = TestResultSchemaVersion
	writeTestResultFile(t, versionPath, []byte("v2\n"), 0o644)
	reconstructed, err = RevalidateRetainedGroupExecutionIdentities(context.Background(), request, []Attempt{attempt})
	if err != nil || reconstructed["a"] == identities["a"] {
		t.Fatalf("changed external tool version retained A identity: current=%v original=%v err=%v", reconstructed, identities, err)
	}
	if got := probeCount(); got != preparedProbeCount {
		t.Fatalf("changed input revalidation launched version command: calls=%d want=%d", got, preparedProbeCount)
	}
	writeTestResultFile(t, versionPath, []byte("v1\n"), 0o644)
	request.Environment = append(append([]string(nil), request.Environment...), "METASYSTEM_FIXTURE_MODE=changed")
	reconstructed, err = RevalidateRetainedGroupExecutionIdentities(context.Background(), request, []Attempt{attempt})
	if err != nil || reconstructed["a"] == identities["a"] {
		t.Fatalf("changed environment retained A identity: current=%v original=%v err=%v", reconstructed, identities, err)
	}
	request.Environment = request.Environment[:len(request.Environment)-1]
	writeTestResultFile(t, filepath.Join(root, "source.txt"), []byte("second\n"), 0o644)
	runTestResultGit(t, root, "add", "source.txt")
	request.CandidateTree, err = (gittree.Workspace{Dir: root}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	reconstructed, err = RevalidateRetainedGroupExecutionIdentities(context.Background(), request, []Attempt{attempt})
	if err != nil || reconstructed["a"] == identities["a"] {
		t.Fatalf("changed source retained A identity: current=%v original=%v err=%v", reconstructed, identities, err)
	}
}

func TestCurrentPolicyComposesOnlyRequiredCompatibleObservations(t *testing.T) {
	t.Parallel()
	identity := strings.Repeat("7", 64)
	source := componentAttemptResult("old", "a", identity, "passed")
	source.SchemaVersion = TestResultSchemaVersion
	source.Groups[0].IdentityVersion = GroupExecutionIdentityVersion
	source.Groups[0].EndedAt = "2026-09-20T09:00:01Z"
	source.ContractDigest = strings.Repeat("1", 64)
	source.BaseContractDigest = strings.Repeat("2", 64)
	old := Attempt{SchemaVersion: IdentityAttemptSchemaVersion, AttemptID: "old", StartedAt: "2026-09-20T09:00:00Z",
		TestInventory: map[string]string{"a": identity}, TestOwned: map[string]string{"a": identity},
		Terminal: &AttemptTerminal{Result: TerminalSuccess}, TestResult: &source}
	template := source
	template.Groups = nil
	template.ContractDigest = strings.Repeat("3", 64)
	template.BaseContractDigest = strings.Repeat("4", 64)
	template.RequiredGroups = []string{"a", "b"}
	template.SelectedGroups = []string{"a", "b"}
	contract := testpolicy.Contract{Groups: []testpolicy.Group{
		{ID: "a", Kind: "unit", Inputs: []string{"source.txt"}, Obligations: []string{"current-a"}},
		{ID: "b", Kind: "unit", Inputs: []string{"other.txt"}, Obligations: []string{"b"}},
	}}
	composed := ReusedTestResult(template, []Attempt{old}, map[string]string{"a": identity, "b": strings.Repeat("8", 64)}, contract)
	if composed.Delivery.Sufficient || composed.Groups[0].Status != "reused" || composed.Groups[1].Status != "not-run" ||
		len(composed.Groups[0].Obligations) != 1 || composed.Groups[0].Obligations[0] != "current-a" {
		t.Fatalf("current policy did not require B and remap A obligation: %+v", composed)
	}
	template.RequiredGroups, template.SelectedGroups = []string{"a"}, []string{"a"}
	composed = ReusedTestResult(template, []Attempt{old}, map[string]string{"a": identity}, contract)
	if !composed.Delivery.Sufficient {
		t.Fatalf("compatible A execution was not reusable: %+v", composed)
	}
	legacy := source
	legacy.SchemaVersion = LegacyTestResultSchemaVersion
	legacy.Groups = append([]GroupResult(nil), source.Groups...)
	legacy.Groups[0].IdentityVersion = 0
	legacyAttempt := old
	legacyAttempt.SchemaVersion = AttemptSchemaVersion
	legacyAttempt.TestInventory, legacyAttempt.TestOwned = nil, nil
	legacyAttempt.PendingTestGroups = map[string]string{"a": identity}
	legacyAttempt.TestResult = &legacy
	if legacyProjection := ReusedTestResult(template, []Attempt{legacyAttempt}, map[string]string{"a": identity}, contract); legacyProjection.Delivery.Sufficient {
		t.Fatalf("old-format record crossed a policy decision: %+v", legacyProjection)
	}
	failed := source
	failed.Groups = append([]GroupResult(nil), source.Groups...)
	failed.Groups[0].Status, failed.Groups[0].EndedAt = "failed", "2026-09-20T09:00:03Z"
	later := Attempt{SchemaVersion: IdentityAttemptSchemaVersion, AttemptID: "later", StartedAt: "2026-09-20T09:00:02Z",
		TestInventory: map[string]string{"a": identity}, TestOwned: map[string]string{"a": identity},
		Terminal: &AttemptTerminal{Result: TerminalFailed}, TestResult: &failed}
	composed = ReusedTestResult(template, []Attempt{old, later}, map[string]string{"a": identity}, contract)
	if composed.Delivery.Sufficient || composed.Groups[0].NotRunReason != "newest-observation-failed" {
		t.Fatalf("newer failure hid behind old pass: %+v", composed)
	}
	live := Attempt{SchemaVersion: IdentityAttemptSchemaVersion, AttemptID: "live", StartedAt: "2026-09-20T09:00:04Z",
		TestInventory: map[string]string{"a": identity}, TestOwned: map[string]string{"a": identity}}
	composed = ReusedTestResult(template, []Attempt{old, live}, map[string]string{"a": identity}, contract)
	if composed.Delivery.Sufficient || composed.Groups[0].NotRunReason != "live-observation-blocks-reuse" {
		t.Fatalf("live reservation hid behind old pass: %+v", composed)
	}
}
