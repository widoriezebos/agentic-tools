package batch

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func gatePatch(paths ...string) []byte {
	var patch strings.Builder
	for _, name := range paths {
		to := "b/" + name
		if strings.HasPrefix(name, "!") {
			name = strings.TrimPrefix(name, "!")
			to = "/dev/null"
		}
		fmt.Fprintf(&patch, "diff --git a/%s b/%s\n--- a/%s\n+++ %s\n@@ -1 +1 @@\n--- metasystem/not-a-header.go\n+++ metasystem/not-a-header.go\n", name, name, name, to)
	}
	return []byte(patch.String())
}
func readRepo(t *testing.T, path ...string) []byte {
	data, err := os.ReadFile(filepath.Join(append([]string{"..", "..", ".."}, path...)...))
	must(t, err)
	return data
}
func fixtureMap(t *testing.T) []byte {
	return readRepo(t, "scripts", "agents", "fixture-bed-groups.tsv")
}
func successfulGate(calls *[]gateStep) batchGateExec {
	return func(_ string, step gateStep) gateStepResult {
		*calls = append(*calls, step)
		return gateStepResult{RunID: fmt.Sprintf("self-%d", len(*calls))}
	}
}
func TestBatchJoinRunsItsOwnGate(t *testing.T) {
	patch := append(gatePatch("metasystem/internal/landing/batch/gate.go", "!metasystem/internal/refusal/register.go", "metasystem/scripts/agents/other-fixtures.sh"), []byte("diff --git a/metasystem/scripts/agents/runtime-hook-fixtures.sh b/metasystem/scripts/agents/runtime-hook-fixtures.sh\nold mode 100644\nnew mode 100755\ndiff --git a/metasystem/scripts/agents/land-fixtures.sh b/metasystem/scripts/agents/land-checks.sh\nsimilarity index 100%\nrename from metasystem/scripts/agents/land-fixtures.sh\nrename to metasystem/scripts/agents/land-checks.sh\ndiff --git a/metasystem/cmd/legacy/file.go b/metasystem/internal/landing/batch/renamed.go\nsimilarity index 100%\nrename from metasystem/cmd/legacy/file.go\nrename to metasystem/internal/landing/batch/renamed.go\n")...)
	groups := append(fixtureMap(t), []byte("scripts/agents/other-fixtures.sh\tsection/other-fixtures\n")...)
	var calls []gateStep
	var trees []string
	unit := Unit{Gate: []string{"supplied"}}
	execute := successfulGate(&calls)
	must(t, runJoinGate("unit-tree", patch, groups, &unit, func(tree string, step gateStep) gateStepResult {
		trees = append(trees, tree)
		return execute(tree, step)
	}))
	if !slices.Equal(unit.Gate, []string{"self-1", "self-2", "self-3", "self-4", "self-5", "self-6", "self-7"}) || len(calls) != 7 || !slices.Equal(trees, []string{"unit-tree", "unit-tree", "unit-tree", "unit-tree", "unit-tree", "unit-tree", "unit-tree"}) {
		t.Fatalf("gate runs=%v calls=%v trees=%v", unit.Gate, calls, trees)
	}
	if !slices.Equal(calls[0].Args, []string{"bash", "scripts/agents/go-gate.sh", "--fast"}) ||
		!slices.Equal(calls[1].Args, []string{"go", "test", "-count=1", "-timeout", "900s", "./cmd/..."}) ||
		!slices.Equal(calls[2].Args, []string{"go", "test", "-count=1", "-timeout", "900s", "./internal/..."}) ||
		!slices.Equal(calls[3].Args, []string{"go", "test", "-count=1", "-timeout", "900s", "./internal/landing/batch"}) ||
		!slices.Equal(calls[4].Args, []string{"bin/metasystem", "test", "run", "--root", ".", "--purpose", "diagnostic", "--groups", "section/land-fixtures", "--mode", "canary", "--tree", "unit-tree"}) ||
		!slices.Equal(calls[5].Args, []string{"bin/metasystem", "test", "run", "--root", ".", "--purpose", "diagnostic", "--groups", "section/other-fixtures", "--mode", "canary", "--tree", "unit-tree"}) ||
		!slices.Equal(calls[6].Args, []string{"bin/metasystem", "test", "run", "--root", ".", "--purpose", "diagnostic", "--groups", "section/supervision-and-census-fixtures", "--mode", "canary", "--tree", "unit-tree"}) {
		t.Fatalf("gate commands=%v", calls)
	}
}

func TestBatchJoinRefusesRedStep(t *testing.T) {
	unit := Unit{}
	static := runJoinGate("unit-tree", nil, fixtureMap(t), &unit,
		func(_ string, step gateStep) gateStepResult {
			return gateStepResult{RunID: "red-static", ExitCode: 1, Detail: "staticcheck: unused assignment"}
		})
	if static == nil || !strings.Contains(static.Error(), "BATCH_JOIN_GATE_RED") || !strings.Contains(static.Error(), "fast gate") || !strings.Contains(static.Error(), "staticcheck") {
		t.Fatalf("red static refusal=%v", static)
	}
	err := runJoinGate("unit-tree", gatePatch("metasystem/internal/landing/batch/gate.go"), fixtureMap(t), &unit,
		func(_ string, step gateStep) gateStepResult {
			if strings.Contains(step.Name, "./internal/landing/batch") {
				return gateStepResult{RunID: "red-package", ExitCode: 1}
			}
			return gateStepResult{RunID: "green-fast"}
		})
	if err == nil || !strings.Contains(err.Error(), "BATCH_JOIN_GATE_RED") || !strings.Contains(err.Error(), "./internal/landing/batch") {
		t.Fatalf("red package refusal=%v", err)
	}
	err = runJoinGate("unit-tree", nil, fixtureMap(t), &unit, func(string, gateStep) gateStepResult { return gateStepResult{} })
	if err == nil || !strings.Contains(err.Error(), "returned no run id") {
		t.Fatalf("missing run refusal=%v", err)
	}
}

func TestBatchJoinRefusesDroppedProtectedTest(t *testing.T) {
	contract := testpolicy.Contract{Groups: []testpolicy.Group{{
		ID: "landing-command-standard", Packages: []string{"cmd/metasystem", "internal/landing/batch"},
		Tests: json.RawMessage(`["TestProtectedJoin","TestSharedName"]`),
	}}}
	trees := map[string]map[string]map[string]bool{
		"base": {
			"cmd/metasystem":         {"TestProtectedJoin": true, "TestSharedName": true},
			"internal/landing/batch": {"TestSharedName": true},
		},
		"candidate": {
			"cmd/metasystem":         {"TestSharedName": true},
			"internal/landing/batch": {"TestSharedName": true},
		},
	}
	read := func(tree, pkg string) (map[string]bool, error) { return trees[tree][pkg], nil }
	err := checkProtectedTestContract(contract, "base", "candidate", read)
	if err == nil || !strings.Contains(err.Error(), "BATCH_JOIN_TEST_DROPPED") ||
		!strings.Contains(err.Error(), "landing-command-standard") || !strings.Contains(err.Error(), "cmd/metasystem") ||
		!strings.Contains(err.Error(), "TestProtectedJoin") {
		t.Fatalf("dropped protected test refusal=%v", err)
	}
	trees["candidate"]["cmd/metasystem"]["TestProtectedJoin"] = true
	if err := checkProtectedTestContract(contract, "base", "candidate", read); err != nil {
		t.Fatalf("preserved protected tests refused: %v", err)
	}
	delete(trees["candidate"]["internal/landing/batch"], "TestSharedName")
	err = checkProtectedTestContract(contract, "base", "candidate", read)
	if err == nil || !strings.Contains(err.Error(), "internal/landing/batch") || !strings.Contains(err.Error(), "TestSharedName") {
		t.Fatalf("duplicate base definition was not protected in each package: %v", err)
	}

	root := t.TempDir()
	for _, arguments := range [][]string{{"init", "-q"}, {"config", "user.name", "Fixture"}, {"config", "user.email", "fixture@example.invalid"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, commandErr := command.CombinedOutput(); commandErr != nil {
			t.Fatalf("git %v: %v: %s", arguments, commandErr, output)
		}
	}
	group := testpolicy.Group{ID: "protected", Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"pkg/**"},
		Obligations: []string{"protected"}, Platforms: []string{"any"}, TargetMS: 1, Packages: []string{"./pkg"}, Tests: json.RawMessage(`["TestProtected"]`)}
	fixtureContract := testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:    []testpolicy.Surface{{ID: "app", Paths: []string{"pkg/**"}, Standard: []string{"protected"}, Critical: []string{"protected"}}},
		Groups:      []testpolicy.Group{group}, Always: testpolicy.Always{Canary: []string{"protected"}}, Unknown: []string{"protected"}, Cadence: []string{}}
	contractData, marshalErr := json.Marshal(fixtureContract)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	for path, data := range map[string][]byte{
		"metasystem/testing.json":      contractData,
		"metasystem/pkg/value_test.go": []byte("package pkg\nimport \"testing\"\nfunc TestProtected(t *testing.T) {}\n"),
	} {
		path = filepath.Join(root, filepath.FromSlash(path))
		if mkdirErr := os.MkdirAll(filepath.Dir(path), 0o755); mkdirErr != nil {
			t.Fatal(mkdirErr)
		}
		if writeErr := os.WriteFile(path, data, 0o644); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	for _, arguments := range [][]string{{"add", "."}, {"commit", "-q", "-m", "base"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, commandErr := command.CombinedOutput(); commandErr != nil {
			t.Fatalf("git %v: %v: %s", arguments, commandErr, output)
		}
	}
	base := bedGit(t, root, "rev-parse", "HEAD^{tree}")
	testPath := filepath.Join(root, "metasystem", "pkg", "value_test.go")
	if writeErr := os.WriteFile(testPath, []byte("package pkg\nimport \"testing\"\nfunc TestRenamed(t *testing.T) {}\n"), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	bedGit(t, root, "add", ".")
	candidate := bedGit(t, root, "write-tree")
	err = CheckProtectedTests(root, base, candidate)
	if err == nil || !strings.Contains(err.Error(), "BATCH_JOIN_TEST_DROPPED") || !strings.Contains(err.Error(), "protected") || !strings.Contains(err.Error(), "./pkg") || !strings.Contains(err.Error(), "TestProtected") {
		t.Fatalf("real candidate dropped-test refusal=%v", err)
	}
}

func TestBatchProtectedTestsAcceptTheBaseTree(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	tree, err := (gittree.Workspace{Dir: root}).TreeOf("HEAD")
	must(t, err)
	if err := CheckProtectedTests(root, tree, tree); err != nil {
		t.Fatalf("base tree rejected its own listed tests: %v", err)
	}
}

func TestBatchJoinRefusesUnmappedFixtureBed(t *testing.T) {
	for _, path := range []string{"metasystem/scripts/agents/unmapped-fixtures.sh", "metasystem/scripts/root-fixtures.sh"} {
		var calls []gateStep
		err := runJoinGate("unit-tree", gatePatch(path), fixtureMap(t), &Unit{}, successfulGate(&calls))
		if err == nil || !strings.Contains(err.Error(), "BATCH_JOIN_GATE_RED") || !strings.Contains(err.Error(), strings.TrimPrefix(path, "metasystem/")) || len(calls) != 0 {
			t.Errorf("unmapped bed %s: refusal=%v calls=%v", path, err, calls)
		}
	}
}

func TestBatchJoinIgnoresSuppliedResults(t *testing.T) {
	tree := t.TempDir()
	prior := filepath.Join(tree, "artifacts", "agents", "proof-runs", "supplied.json")
	must(t, os.MkdirAll(filepath.Dir(prior), 0o755))
	must(t, os.WriteFile(prior, []byte(`{"runId":"supplied"}`), 0o644))
	var calls []gateStep
	unit := Unit{Gate: []string{"supplied"}}
	patch := gatePatch("metasystem/internal/landing/batch/gate.go", "metasystem/scripts/agents/runtime-hook-fixtures.sh")
	must(t, runJoinGate(tree, patch, fixtureMap(t), &unit, successfulGate(&calls)))
	if len(calls) != 3 || !slices.Equal(unit.Gate, []string{"self-1", "self-2", "self-3"}) {
		t.Fatalf("supplied result shortened or entered gate: calls=%d runs=%v", len(calls), unit.Gate)
	}
}

func TestDeletedGoPackagesSelectNearestExistingDirectory(t *testing.T) {
	root := t.TempDir()
	for _, arguments := range [][]string{{"init", "-q", "-b", "main"}, {"config", "user.name", "Fixture"}, {"config", "user.email", "fixture@example.invalid"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
	}
	for path, content := range map[string]string{
		"metasystem/outer/keep.txt":               "keep\n",
		"metasystem/outer/missing/inner/value.go": "package inner\n",
	} {
		absolute := filepath.Join(root, filepath.FromSlash(path))
		must(t, os.MkdirAll(filepath.Dir(absolute), 0o755))
		must(t, os.WriteFile(absolute, []byte(content), 0o644))
	}
	for _, arguments := range [][]string{{"add", "."}, {"commit", "-qm", "base"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		must(t, command.Run())
	}
	base := bedGit(t, root, "rev-parse", "HEAD^{tree}")
	must(t, os.Remove(filepath.Join(root, "metasystem", "outer", "missing", "inner", "value.go")))
	bedGit(t, root, "add", "-u")
	tip := bedGit(t, root, "write-tree")
	patch := []byte(bedGit(t, root, "diff", "--cached", "--binary", "HEAD"))
	packages, err := changedGoPackages(root, tip, patchGateChanges(patch))
	must(t, err)
	if !slices.Equal(packages, []string{"./outer/..."}) {
		t.Fatalf("nested deletion packages=%v, want nearest existing parent", packages)
	}

	createDelete := []byte("diff --git a/metasystem/fresh/inner/value.go b/metasystem/fresh/inner/value.go\nnew file mode 100644\n--- /dev/null\n+++ b/metasystem/fresh/inner/value.go\n@@ -0,0 +1 @@\n+package inner\ndiff --git a/metasystem/fresh/inner/value.go b/metasystem/fresh/inner/value.go\ndeleted file mode 100644\n--- a/metasystem/fresh/inner/value.go\n+++ /dev/null\n@@ -1 +0,0 @@\n-package inner\n")
	packages, err = changedGoPackages(root, base, patchGateChanges(createDelete))
	must(t, err)
	if len(packages) != 0 {
		t.Fatalf("create-delete path selected packages=%v", packages)
	}
}

func TestFixtureGroupsForChangedBeds(t *testing.T) {
	groups := parseFixtureBedGroups(fixtureMap(t))
	cases := []struct {
		name, path string
		want       fixtureSelection
	}{
		{"shared bed", "metasystem/scripts/watch-background-jobs.sh", fixtureSelection{Groups: []string{"section/watch-background-jobs-fixtures", "section/workflow-tooling-fixtures"}}},
		{"scenario harness", "metasystem/scripts/agents/fixture-bed-scenarios.sh", fixtureSelection{CoveredByProof: true}},
		{"harness", "metasystem/scripts/agents/fixture-budget.sh", fixtureSelection{CoveredByProof: true}},
		{"assert harness", "metasystem/scripts/agents/fixture-assert.sh", fixtureSelection{CoveredByProof: true}},
		{"go only", "metasystem/internal/landing/batch/gate.go", fixtureSelection{}},
	}
	for _, test := range cases {
		if got := fixtureGroupsForChangedBeds(patchChange{test.path: false}, groups); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s selection=%v, want %v", test.name, got, test.want)
		}
	}
}

var sectionScriptPattern = regexp.MustCompile(`^scripts/(?:agents/)?[[:alnum:]_.-]+\.sh$`)

func scriptsInSection(source string) []string {
	var paths []string
	for _, line := range strings.Split(source, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		for _, field := range strings.Fields(line) {
			path := strings.TrimPrefix(strings.Trim(field, `"'();`), "$root/")
			if sectionScriptPattern.MatchString(path) {
				paths = append(paths, path)
			}
		}
	}
	return paths
}

func fixtureRowsFromSuite(suite []byte) []string {
	text := strings.ReplaceAll(string(suite), "\\\n", " ")
	rows := map[string]bool{}
	offset := 0
	for _, match := range regexp.MustCompile(`(?m)\brun_section[ \t]+(\S+-fixtures)[ \t]+\S+[ \t]+(\S+)[^\n]*`).FindAllStringSubmatch(text, -1) {
		at := offset + strings.Index(text[offset:], match[0])
		source := match[0]
		if start := strings.Index(text[:at], "\n"+match[2]+"() {"); start >= 0 {
			start += len(match[2]) + 5
			if end := strings.LastIndex(text[start:at], "\n}\n"); end >= 0 {
				source += text[start : start+end]
			}
		}
		offset = at + len(match[0])
		for _, script := range scriptsInSection(source) {
			if !isSharedFixtureHarness(script) {
				rows[script+"\tsection/"+match[1]] = true
			}
		}
	}
	result := make([]string, 0, len(rows))
	for row := range rows {
		result = append(result, row)
	}
	sort.Strings(result)
	return result
}

func TestFixtureBedGroupMapMatchesSections(t *testing.T) {
	want := fixtureRowsFromSuite(readRepo(t, "scripts", "validate-metasystem.sh"))
	mapPath := filepath.Join("..", "..", "..", "scripts", "agents", "fixture-bed-groups.tsv")
	if os.Getenv("UPDATE_FIXTURE_BED_GROUPS") == "1" {
		must(t, os.WriteFile(mapPath, []byte(strings.Join(want, "\n")+"\n"), 0o644))
	}
	got := strings.Split(strings.TrimSpace(string(fixtureMap(t))), "\n")
	sort.Strings(got)
	if !slices.Equal(got, want) {
		t.Fatalf("fixture bed map=%v, section beds=%v", got, want)
	}
	var contract struct {
		Groups []struct {
			ID string `json:"id"`
		} `json:"groups"`
	}
	must(t, json.Unmarshal(readRepo(t, "testing.json"), &contract))
	known := map[string]bool{}
	for _, group := range contract.Groups {
		known[group.ID] = true
	}
	for _, row := range want {
		if group := strings.SplitN(row, "\t", 2)[1]; !known[group] {
			t.Errorf("fixture group %s is absent from testing.json", group)
		}
	}
}
