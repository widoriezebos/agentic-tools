package proofrun

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// closureModule lays out a module of three packages: a (leaf, with testdata
// and an embed), b (imports a), c (imports nothing, has testdata); b's test
// also imports the test-only package tdep. The closure of a group on a must
// not carry b: what imports a root cannot change the root's outcome.
func closureModule(t *testing.T, root string) {
	t.Helper()
	write := func(relative, body string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.invalid/closure\n\ngo 1.22\n")
	write("a/a.go", "package a\nimport _ \"embed\"\n//go:embed embedded.txt\nvar Embedded string\nfunc A() string { return Embedded }\n")
	write("a/embedded.txt", "embedded\n")
	write("a/a_test.go", "package a\nimport \"testing\"\nfunc TestA(t *testing.T) { if A() == \"\" { t.Fatal(\"empty\") } }\n")
	write("a/testdata/case.txt", "fixture\n")
	write("b/b.go", "package b\nimport \"example.invalid/closure/a\"\nfunc B() string { return a.A() + \"b\" }\n")
	write("b/b_test.go", "package b\nimport (\"testing\"; _ \"example.invalid/closure/tdep\")\nfunc TestB(t *testing.T) { if B() == \"\" { t.Fatal(\"empty\") } }\n")
	write("tdep/tdep.go", "package tdep\n")
	write("c/c.go", "package c\nfunc C() string { return \"c\" }\n")
	write("c/c_test.go", "package c\nimport \"testing\"\nfunc TestC(t *testing.T) { if C() == \"\" { t.Fatal(\"empty\") } }\n")
	write("c/testdata/data.txt", "data\n")
}

func closureGroup(id string, packages ...string) testpolicy.Group {
	return testpolicy.Group{ID: id, Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"go.mod"}, Outputs: []string{},
		Tools: []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}}, Obligations: []string{id}, Platforms: []string{"any"}, TargetMS: 1000,
		Packages: packages, Tests: json.RawMessage(`"all"`)}
}

func TestGoClosureFollowsDependenciesNotDependents(t *testing.T) {
	root := t.TempDir()
	closureModule(t, root)
	inputs := func(packages ...string) map[string]bool {
		t.Helper()
		_, _, discovery, _, err := goArguments(context.Background(), closureGroup("g", packages...), root, os.Environ())
		if err != nil {
			t.Fatalf("discovery of %v: %v", packages, err)
		}
		set := map[string]bool{}
		for _, path := range discovery.Inputs {
			set[path] = true
		}
		return set
	}
	onA := inputs("a")
	for _, want := range []string{"a/a.go", "a/a_test.go", "a/embedded.txt", "a/testdata/case.txt", "go.mod", "go.sum"} {
		if !onA[want] {
			t.Fatalf("closure of a lacks %s: %v", want, onA)
		}
	}
	for _, unwanted := range []string{"b/b.go", "b/b_test.go", "tdep/tdep.go", "c/c.go"} {
		if onA[unwanted] {
			t.Fatalf("closure of a carries %s, which only imports a: %v", unwanted, onA)
		}
	}
	onB := inputs("b")
	for _, want := range []string{"b/b.go", "b/b_test.go", "tdep/tdep.go", "a/a.go", "a/embedded.txt"} {
		if !onB[want] {
			t.Fatalf("closure of b lacks %s: %v", want, onB)
		}
	}
	// a's own tests do not run for b, so a's test files and testdata stay out.
	for _, unwanted := range []string{"a/a_test.go", "a/testdata/case.txt", "c/c.go"} {
		if onB[unwanted] {
			t.Fatalf("closure of b carries %s: %v", unwanted, onB)
		}
	}
	onC := inputs("c")
	if !onC["c/testdata/data.txt"] || onC["a/a.go"] || onC["b/b.go"] {
		t.Fatalf("closure of c is wrong: %v", onC)
	}
}

func TestReStageOutsideAClosureKeepsTheIdentity(t *testing.T) {
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	closureModule(t, root)
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "closure")
	contract := testpolicy.Contract{SchemaVersion: 1, Groups: []testpolicy.Group{closureGroup("ga", "a"), closureGroup("gb", "b"), closureGroup("gc", "c")}}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeStandard, RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard,
		RequiredGroups: []string{"ga", "gb", "gc"}, SelectedGroups: []string{"ga", "gb", "gc"}, Stages: []testpolicy.Stage{{ID: "standard", Groups: []string{"ga", "gb", "gc"}}}}
	identitiesAt := func(tree string) map[string]string {
		t.Helper()
		identities, err := GroupExecutionIdentities(context.Background(), TestRunRequest{ProjectRoot: root, CandidateTree: tree, BaseCommit: "HEAD", PolicyBaseCommit: "HEAD",
			Contract: contract, Plan: plan, Environment: os.Environ()})
		if err != nil {
			t.Fatalf("identities at %s: %v", tree, err)
		}
		return identities
	}
	first := identitiesAt(runTestResultGit(t, root, "rev-parse", "HEAD^{tree}"))
	if err := os.WriteFile(filepath.Join(root, "b", "b.go"), []byte("package b\nimport \"example.invalid/closure/a\"\nfunc B() string { return a.A() + \"bb\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "edit b")
	second := identitiesAt(runTestResultGit(t, root, "rev-parse", "HEAD^{tree}"))
	if first["ga"] != second["ga"] || first["gc"] != second["gc"] || first["gb"] == second["gb"] {
		t.Fatalf("an edit to b changed the wrong identities: before=%v after=%v", first, second)
	}
	if err := os.WriteFile(filepath.Join(root, "c", "testdata", "data.txt"), []byte("changed data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "edit testdata of c")
	third := identitiesAt(runTestResultGit(t, root, "rev-parse", "HEAD^{tree}"))
	if second["ga"] != third["ga"] || second["gb"] != third["gb"] || second["gc"] == third["gc"] {
		t.Fatalf("an edit to c's testdata changed the wrong identities: before=%v after=%v", second, third)
	}
}

func TestJudgeKeyKeepsIdentityAcrossARebuild(t *testing.T) {
	cwd := t.TempDir()
	group := testpolicy.Group{ID: "unit", Adapter: "go", CWD: ".", Packages: []string{"internal/x"}}
	base := TestRunRequest{ContractDigest: strings.Repeat("1", 64), BaseContractDigest: strings.Repeat("2", 64), PolicyEngineDigest: strings.Repeat("a", 64),
		BehaviorPolicyDigest: strings.Repeat("3", 64), CandidateEngineDigest: strings.Repeat("e", 64), JudgeKey: "judge/v1:t1:t2:t3"}
	identity := groupExecutionIdentity(base, group, cwd, "inputs", "environment", nil, nil)
	rebuilt := base
	rebuilt.PolicyEngineDigest = strings.Repeat("b", 64)
	if groupExecutionIdentity(rebuilt, group, cwd, "inputs", "environment", nil, nil) != identity {
		t.Fatal("a rebuilt judge with the same key changed a group's execution identity")
	}
	changedJudge := base
	changedJudge.JudgeKey = "judge/v1:t1:t2:changed"
	if groupExecutionIdentity(changedJudge, group, cwd, "inputs", "environment", nil, nil) == identity {
		t.Fatal("a changed judge key kept a group's execution identity")
	}
	section := testpolicy.Group{ID: "bed", Adapter: "section", CWD: ".", Section: "bed"}
	sectionIdentity := groupExecutionIdentity(base, section, cwd, "inputs", "environment", nil, nil)
	changedCandidate := base
	changedCandidate.CandidateEngineDigest = strings.Repeat("f", 64)
	if groupExecutionIdentity(changedCandidate, section, cwd, "inputs", "environment", nil, nil) == sectionIdentity {
		t.Fatal("a section group ignored a changed candidate engine")
	}
	if groupExecutionIdentity(TestRunRequest{}, group, cwd, "inputs", "environment", nil, nil) !=
		groupExecutionIdentity(TestRunRequest{JudgeKey: DefaultJudgeKey()}, group, cwd, "inputs", "environment", nil, nil) {
		t.Fatal("an empty judge key is not the default key")
	}
}

func TestComputeJudgeKeyFollowsTheJudgeSourceNotTheBuild(t *testing.T) {
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	write := func(relative, body string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("metasystem/internal/proofrun/runner.go", "package proofrun\n")
	write("metasystem/internal/testpolicy/select.go", "package testpolicy\n")
	write("metasystem/cmd/main.go", "package main\n")
	write("metasystem/go.mod", "module example.invalid/engine\n")
	write("metasystem/records/note.md", "a record\n")
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "judge")
	first := runTestResultGit(t, root, "rev-parse", "HEAD")
	key := ComputeJudgeKey(context.Background(), root, first, "metasystem")
	parts := strings.Split(key, ":")
	if len(parts) != 5 || parts[0] != JudgeCompatibilityVersion || !validTreeDigest(parts[1]) || !validTreeDigest(parts[2]) || !validTreeDigest(parts[3]) || parts[4] != "absent" {
		t.Fatalf("judge key shape: %s", key)
	}
	write("metasystem/records/note.md", "a record landing outside the engine sources\n")
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "outside the judge")
	if ComputeJudgeKey(context.Background(), root, runTestResultGit(t, root, "rev-parse", "HEAD"), "metasystem") != key {
		t.Fatal("a commit outside the engine sources changed the judge key")
	}
	write("metasystem/internal/audit/coverage.go", "package audit\n// a verdict changed\n")
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "judge edit")
	if ComputeJudgeKey(context.Background(), root, runTestResultGit(t, root, "rev-parse", "HEAD"), "metasystem") == key {
		t.Fatal("an edit to an engine source kept the judge key")
	}
	for _, unreadable := range []string{ComputeJudgeKey(context.Background(), root, "", "metasystem"), ComputeJudgeKey(context.Background(), t.TempDir(), first, "metasystem"),
		ComputeJudgeKey(context.Background(), root, strings.Repeat("0", 40), "metasystem")} {
		if unreadable == DefaultJudgeKey() || unreadable == key || !strings.HasPrefix(unreadable, JudgeCompatibilityVersion+":unreadable:") {
			t.Fatalf("an unreadable judge did not yield a key that matches nothing: %s", unreadable)
		}
	}
	firstUnreadable := ComputeJudgeKey(context.Background(), t.TempDir(), first, "metasystem")
	secondUnreadable := ComputeJudgeKey(context.Background(), t.TempDir(), first, "metasystem")
	if firstUnreadable == key || firstUnreadable == secondUnreadable {
		t.Fatal("two unreadable judges read alike")
	}
	if ComputeJudgeKey(context.Background(), root, first, "elsewhere") != DefaultJudgeKey() {
		t.Fatal("a commit without the engine sources did not read as the default key")
	}
}
