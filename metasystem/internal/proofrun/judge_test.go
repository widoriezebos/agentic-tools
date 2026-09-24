package proofrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gopackages"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
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
	closureModule(t, root)
	firstFiles := make(map[string]testSnapshotEntry)
	for _, name := range []string{
		"go.mod", "a/a.go", "a/a_test.go", "a/embedded.txt", "a/testdata/case.txt",
		"b/b.go", "b/b_test.go", "tdep/tdep.go", "c/c.go", "c/c_test.go", "c/testdata/data.txt",
	} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		firstFiles[name] = testSnapshotFile(string(data), 0o644)
	}
	copyFiles := func(from map[string]testSnapshotEntry) map[string]testSnapshotEntry {
		t.Helper()
		copy := make(map[string]testSnapshotEntry, len(from))
		for name, entry := range from {
			copy[name] = entry
		}
		return copy
	}
	secondFiles := copyFiles(firstFiles)
	secondFiles["b/b.go"] = testSnapshotFile(strings.Replace(string(firstFiles["b/b.go"].data), `+ "b"`, `+ "bb"`, 1), 0o644)
	thirdFiles := copyFiles(secondFiles)
	thirdFiles["c/testdata/data.txt"] = testSnapshotFile("changed data\n", 0o644)
	firstSnapshot := newTestSnapshotFactory(t, root, strings.Repeat("1", 40), firstFiles, 1)
	secondSnapshot := newTestSnapshotFactory(t, root, strings.Repeat("2", 40), secondFiles, 1)
	thirdSnapshot := newTestSnapshotFactory(t, root, strings.Repeat("3", 40), thirdFiles, 1)
	contract := testpolicy.Contract{SchemaVersion: 1, Groups: []testpolicy.Group{closureGroup("ga", "a"), closureGroup("gb", "b"), closureGroup("gc", "c")}}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeStandard, RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard,
		RequiredGroups: []string{"ga", "gb", "gc"}, SelectedGroups: []string{"ga", "gb", "gc"}, Stages: []testpolicy.Stage{{ID: "standard", Groups: []string{"ga", "gb", "gc"}}}}
	identitiesAt := func(snapshot *testSnapshotFactory) map[string]string {
		t.Helper()
		identities, err := GroupExecutionIdentities(context.Background(), TestRunRequest{ProjectRoot: root, CandidateTree: snapshot.tree, openCandidate: snapshot.open,
			BaseCommit: "HEAD", PolicyBaseCommit: "HEAD", Contract: contract, Plan: plan, Environment: os.Environ()})
		if err != nil {
			t.Fatalf("identities at %s: %v", snapshot.tree, err)
		}
		return identities
	}
	first := identitiesAt(firstSnapshot)
	second := identitiesAt(secondSnapshot)
	if first["ga"] != second["ga"] || first["gc"] != second["gc"] || first["gb"] == second["gb"] {
		t.Fatalf("an edit to b changed the wrong identities: before=%v after=%v", first, second)
	}
	third := identitiesAt(thirdSnapshot)
	if second["ga"] != third["ga"] || second["gb"] != third["gb"] || second["gc"] == third["gc"] {
		t.Fatalf("an edit to c's testdata changed the wrong identities: before=%v after=%v", second, third)
	}
}

func TestExpandedGoGroupIdentityTracksOnlyRelevantInputs(t *testing.T) {
	root := t.TempDir()
	contract := loadGoExpansionContract(t)
	template := goExpansionTemplate(t, contract)
	const baseTree = "opaque-base-identity-tree"
	const candidateTree = "opaque-candidate-identity-tree"
	selection := gopackages.Selection{
		Tree: candidateTree, ModulePath: "example.invalid/identity",
		Changed: []string{"./base"}, Dependents: []string{"./consumer"},
		Packages: []string{"./base", "./consumer"},
		InputDirs: map[string][]string{
			"./base": {"./base"}, "./consumer": {"./consumer", "./base"},
		},
	}
	selectorEnvironment := []string{"GOARCH=arm64", "GOOS=linux", "GOWORK=off", "PATH=/selector-tools"}
	calls := 0
	selector := func(moduleRoot, base, candidate string, tags, environment []string) (gopackages.Selection, error) {
		calls++
		if calls != 1 || moduleRoot != filepath.Join(root, "metasystem") || base != baseTree || candidate != candidateTree ||
			!slices.Equal(tags, template.BuildTags) || !slices.Equal(environment, selectorEnvironment) {
			t.Fatalf("selector call %d: root=%q base=%q candidate=%q tags=%v environment=%v", calls, moduleRoot, base, candidate, tags, environment)
		}
		return selection, nil
	}
	expanded, err := expandGoPackageGroupsWithSelector(contract, root, baseTree, candidateTree,
		[]string{"PATH=/selector-tools", "GOOS=linux", "GOARCH=arm64", "GOWORK=off"}, selector)
	if err != nil || calls != 1 {
		t.Fatalf("expand base group: calls=%d err=%v", calls, err)
	}
	group := concreteGoExpansionGroup(t, expanded, template.ID, "./base")
	files := map[string]testSnapshotEntry{
		"metasystem/go.mod":               testSnapshotFile("module example.invalid/identity\n\ngo 1.22\n", 0o644),
		"metasystem/base/base.go":         testSnapshotFile("package base\nconst Value = 1\n", 0o644),
		"metasystem/base/base_test.go":    testSnapshotFile("package base\nimport \"testing\"\nfunc TestBase(t *testing.T) {}\n", 0o644),
		"metasystem/consumer/consumer.go": testSnapshotFile("package consumer\n", 0o644),
		"metasystem/scripts/shared.sh":    testSnapshotFile("echo original\n", 0o644),
	}
	copyFiles := func() map[string]testSnapshotEntry {
		copy := make(map[string]testSnapshotEntry, len(files)+1)
		for name, entry := range files {
			copy[name] = entry
		}
		return copy
	}
	unrelatedFiles := copyFiles()
	unrelatedFiles["metasystem/added/added.go"] = testSnapshotFile("package added\n", 0o644)
	sharedFiles := copyFiles()
	sharedFiles["metasystem/scripts/shared.sh"] = testSnapshotFile("echo changed\n", 0o644)
	baseFiles := copyFiles()
	baseFiles["metasystem/base/base.go"] = testSnapshotFile("package base\nconst Value = 2\n", 0o644)
	snapshots := []*testSnapshotFactory{
		newTestSnapshotFactory(t, root, strings.Repeat("1", 40), files, 1),
		newTestSnapshotFactory(t, root, strings.Repeat("2", 40), unrelatedFiles, 1),
		newTestSnapshotFactory(t, root, strings.Repeat("3", 40), sharedFiles, 1),
		newTestSnapshotFactory(t, root, strings.Repeat("4", 40), baseFiles, 1),
	}
	identities := make([]string, len(snapshots))
	for i, snapshot := range snapshots {
		request := TestRunRequest{
			ProjectRoot: root, CandidateTree: snapshot.tree, openCandidate: snapshot.open,
			BaseCommit: "HEAD", PolicyBaseCommit: "HEAD", Contract: expanded,
			Plan:     testpolicy.Plan{SelectedGroups: []string{group.ID}},
			JudgeKey: "fixed-judge", BehaviorPolicyDigest: "fixed-behavior", Environment: os.Environ(),
		}
		result, _, _, err := PrepareGroupExecutionIdentities(context.Background(), request)
		if err != nil {
			t.Fatalf("prepare identity for snapshot %d: %v", i, err)
		}
		if result[group.ID] == "" {
			t.Fatalf("snapshot %d produced no base group identity", i)
		}
		identities[i] = result[group.ID]
	}
	if identities[0] != identities[1] || identities[0] == identities[2] || identities[0] == identities[3] {
		t.Fatalf("base identities across original, unrelated package, shared script, and base source = %v", identities)
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
	type gitRead struct {
		args   []string
		stdout string
		err    error
	}
	readKey := func(projectRoot, commit, prefix string, calls []gitRead) string {
		t.Helper()
		next := 0
		reader := judgeGitReader(func(_ context.Context, actualRoot string, args ...string) (string, error) {
			t.Helper()
			if next >= len(calls) {
				t.Fatalf("unexpected judge Git read in %q: %q", actualRoot, args)
			}
			want := calls[next]
			if actualRoot != projectRoot || !reflect.DeepEqual(args, want.args) {
				t.Fatalf("judge Git read %d = (%q, %q), want (%q, %q)", next, actualRoot, args, projectRoot, want.args)
			}
			next++
			return want.stdout, want.err
		})
		t.Cleanup(func() {
			if next != len(calls) {
				t.Errorf("used %d of %d declared judge Git reads for %q", next, len(calls), commit)
			}
		})
		return computeJudgeKey(context.Background(), projectRoot, commit, prefix, reader)
	}
	first, outside, inside := strings.Repeat("1", 40), strings.Repeat("2", 40), strings.Repeat("3", 40)
	cmdID, internalID, moduleID, editedInternalID := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40), strings.Repeat("d", 40)
	reads := func(commit, prefix string, ids [4]string) []gitRead {
		calls := []gitRead{{args: []string{"rev-parse", "--verify", "--quiet", commit + "^{commit}"}, stdout: commit}}
		for index, source := range []string{"cmd", "internal", "go.mod", "go.sum"} {
			path := prefix + "/" + source
			output := ""
			if ids[index] != "" {
				mode, kind := "100644", "blob"
				if source == "cmd" || source == "internal" {
					mode, kind = "040000", "tree"
				}
				output = fmt.Sprintf("%s %s %s\t%s\x00", mode, kind, ids[index], path)
			}
			calls = append(calls, gitRead{args: []string{"ls-tree", "-z", "--full-tree", commit, "--", path}, stdout: output})
		}
		return calls
	}
	baseIDs := [4]string{cmdID, internalID, moduleID, ""}
	key := readKey(root, first, "metasystem", reads(first, "metasystem", baseIDs))
	parts := strings.Split(key, ":")
	if len(parts) != 5 || parts[0] != JudgeCompatibilityVersion || !validTreeDigest(parts[1]) || !validTreeDigest(parts[2]) || !validTreeDigest(parts[3]) ||
		parts[1] != cmdID || parts[2] != internalID || parts[3] != moduleID || parts[4] != "absent" {
		t.Fatalf("judge key shape: %s", key)
	}
	if readKey(root, outside, "metasystem", reads(outside, "metasystem", baseIDs)) != key {
		t.Fatal("a commit outside the engine sources changed the judge key")
	}
	if readKey(root, inside, "metasystem", reads(inside, "metasystem", [4]string{cmdID, editedInternalID, moduleID, ""})) == key {
		t.Fatal("an edit to an engine source kept the judge key")
	}
	unreadableRoot := t.TempDir()
	unknown := strings.Repeat("0", 40)
	rootFailure := []gitRead{{args: []string{"rev-parse", "--verify", "--quiet", first + "^{commit}"}, err: errors.New("repository cannot be read")}}
	unknownFailure := []gitRead{{args: []string{"rev-parse", "--verify", "--quiet", unknown + "^{commit}"}, err: errors.New("commit does not exist")}}
	for _, unreadable := range []string{
		readKey(root, "", "metasystem", nil),
		readKey(unreadableRoot, first, "metasystem", rootFailure),
		readKey(root, unknown, "metasystem", unknownFailure),
	} {
		if unreadable == DefaultJudgeKey() || unreadable == key || !strings.HasPrefix(unreadable, JudgeCompatibilityVersion+":unreadable:") {
			t.Fatalf("an unreadable judge did not yield a key that matches nothing: %s", unreadable)
		}
	}
	firstUnreadable := readKey(unreadableRoot, first, "metasystem", rootFailure)
	secondUnreadable := readKey(unreadableRoot, first, "metasystem", rootFailure)
	if firstUnreadable == key || firstUnreadable == secondUnreadable {
		t.Fatal("two unreadable judges read alike")
	}
	if readKey(root, first, "elsewhere", reads(first, "elsewhere", [4]string{})) != DefaultJudgeKey() {
		t.Fatal("a commit without the engine sources did not read as the default key")
	}
}

func TestComputeJudgeKeyWaitsForSlowSuccessfulGitAndKeepsTheStableKey(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	blocked := false
	digest := strings.Repeat("a", 40)
	readGit := func(ctx context.Context, _ string, args ...string) (string, error) {
		if _, hasDeadline := ctx.Deadline(); hasDeadline {
			return "", errors.New("judge Git received a derived deadline")
		}
		if !blocked {
			blocked = true
			close(entered)
			<-release
		}
		if args[0] == "rev-parse" {
			return digest, nil
		}
		sourcePath := args[len(args)-1]
		return fmt.Sprintf("040000 tree %s\t%s\x00", digest, sourcePath), nil
	}
	result := make(chan string, 1)
	go func() {
		result <- computeJudgeKey(context.Background(), "/slow-repository", digest, "metasystem", readGit)
	}()
	<-entered
	select {
	case key := <-result:
		t.Fatalf("judge key returned before successful Git completed: %s", key)
	default:
	}
	close(release)
	first := <-result
	second := computeJudgeKey(context.Background(), "/slow-repository", digest, "metasystem", readGit)
	if first != second || !strings.HasPrefix(first, JudgeCompatibilityVersion+":"+digest+":") {
		t.Fatalf("slow successful Git produced unstable judge keys: first=%q second=%q", first, second)
	}
}

func TestEnvironmentDigestIgnoresCacheAndTemporaryLocations(t *testing.T) {
	base := []string{"PATH=/usr/bin", "HOME=/Users/seat", "GOFLAGS=-mod=mod", "GOCACHE=/a/go-cache", "GOTMPDIR=/a/go-tmp", "STATICCHECK_CACHE=/a/staticcheck", "TMPDIR=/a/tmp"}
	moved := []string{"PATH=/usr/bin", "HOME=/Users/seat", "GOFLAGS=-mod=mod", "GOCACHE=/b/go-cache", "GOTMPDIR=/b/go-tmp", "STATICCHECK_CACHE=/b/staticcheck", "TMPDIR=/b/tmp"}
	if digestEnvironment(base) != digestEnvironment(moved) {
		t.Fatal("a moved build cache or scratch changed the environment digest")
	}
	firstOwner := append(append([]string(nil), base...), identity.RunOwnerEnv+"=first-exact-ref")
	secondOwner := append(append([]string(nil), base...), identity.RunOwnerEnv+"=second-exact-ref")
	if digestEnvironment(firstOwner) != digestEnvironment(secondOwner) {
		t.Fatal("a new run owner's exact process reference changed the environment digest")
	}
	firstAttempt := append(append([]string(nil), base...), identity.FixtureAttemptEnv+"=attempt-a")
	secondAttempt := append(append([]string(nil), base...), identity.FixtureAttemptEnv+"=attempt-b")
	if digestEnvironment(firstAttempt) != digestEnvironment(secondAttempt) {
		t.Fatal("a new fixture attempt tag changed the environment digest")
	}
	firstRoot := append(append([]string(nil), base...), proofExecutionRootEnvironment+"=/first/project")
	secondRoot := append(append([]string(nil), base...), proofExecutionRootEnvironment+"=/second/project")
	if digestEnvironment(firstRoot) != digestEnvironment(secondRoot) {
		t.Fatal("a moved authenticated project root changed the inherited environment digest")
	}
	explicit := testpolicy.Group{EnvironmentMode: "explicit"}
	if digestGroupEnvironment(explicit, firstRoot) != digestGroupEnvironment(explicit, secondRoot) {
		t.Fatal("a moved authenticated project root changed the explicit environment digest")
	}
	changed := []string{"PATH=/usr/bin", "HOME=/Users/seat", "GOFLAGS=-mod=vendor", "GOCACHE=/a/go-cache"}
	if digestEnvironment(base) == digestEnvironment(changed) {
		t.Fatal("a changed build flag kept the environment digest")
	}
	changed = append(changed, proofExecutionRootEnvironment+"=/second/project")
	if digestGroupEnvironment(explicit, firstRoot) == digestGroupEnvironment(explicit, changed) {
		t.Fatal("excluding the explicit root locator also excluded a real test-condition input")
	}
}
