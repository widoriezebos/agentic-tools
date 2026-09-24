package batch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func readRepo(t *testing.T, path ...string) []byte {
	data, err := os.ReadFile(filepath.Join(append([]string{"..", "..", ".."}, path...)...))
	must(t, err)
	return data
}
func fixtureMap(t *testing.T) []byte {
	return readRepo(t, "scripts", "agents", "fixture-bed-groups.tsv")
}
func TestBatchJoinRefusesDroppedProtectedTest(t *testing.T) {
	root := t.TempDir()
	group := testpolicy.Group{ID: "protected", Kind: "unit", Adapter: "go", CWD: "metasystem", Inputs: []string{"pkg/**", "second/**"},
		Obligations: []string{"protected"}, Platforms: []string{"any"}, TargetMS: 1, Packages: []string{"./pkg", "./second"}, Tests: json.RawMessage(`["TestProtected"]`)}
	fixtureContract := testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:    []testpolicy.Surface{{ID: "app", Paths: []string{"pkg/**", "second/**"}, Standard: []string{"protected"}, Critical: []string{"protected"}}},
		Groups:      []testpolicy.Group{group}, Always: testpolicy.Always{Canary: []string{"protected"}}, Unknown: []string{"protected"}, Cadence: []string{}}
	contractData, marshalErr := json.Marshal(fixtureContract)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	baseFiles := map[string][]byte{
		"metasystem/testing.json":         contractData,
		"metasystem/pkg/value_test.go":    []byte("package pkg\nimport \"testing\"\nfunc TestProtected(t *testing.T) {}\n"),
		"metasystem/second/value_test.go": []byte("package second\nimport \"testing\"\nfunc TestProtected(t *testing.T) {}\n"),
	}
	const base = "1111111111111111111111111111111111111111"
	const pkgDropped = "2222222222222222222222222222222222222222"
	const secondDropped = "3333333333333333333333333333333333333333"
	pkgFiles := map[string][]byte{
		"metasystem/testing.json":         contractData,
		"metasystem/pkg/value_test.go":    []byte("package pkg\nimport \"testing\"\nfunc TestRenamed(t *testing.T) {}\n"),
		"metasystem/second/value_test.go": baseFiles["metasystem/second/value_test.go"],
	}
	secondFiles := map[string][]byte{
		"metasystem/testing.json":         contractData,
		"metasystem/pkg/value_test.go":    baseFiles["metasystem/pkg/value_test.go"],
		"metasystem/second/value_test.go": []byte("package second\nimport \"testing\"\nfunc TestRenamed(t *testing.T) {}\n"),
	}
	workspace := protectedTestWorkspace(t, root, map[string]map[string][]byte{
		base: baseFiles, pkgDropped: pkgFiles, secondDropped: secondFiles,
	}, []string{"metasystem/testing.json", "metasystem/pkg", "metasystem/pkg/value_test.go", "metasystem/second", "metasystem/second/value_test.go"}, "")
	err := proofrun.CheckProtectedGoTests(workspace, base, pkgDropped, fixtureContract)
	if err == nil || !strings.Contains(err.Error(), "protected") || !strings.Contains(err.Error(), "./pkg") || !strings.Contains(err.Error(), "TestProtected") {
		t.Fatalf("real candidate dropped-test refusal=%v", err)
	}
	err = proofrun.CheckProtectedGoTests(workspace, base, secondDropped, fixtureContract)
	if err == nil || !strings.Contains(err.Error(), "./second") || !strings.Contains(err.Error(), "TestProtected") {
		t.Fatalf("duplicate base definition was not protected in each package: %v", err)
	}
	fixtureContract.Groups[0].Tests = json.RawMessage(`["TestNotInBase"]`)
	err = proofrun.CheckProtectedGoTests(workspace, base, secondDropped, fixtureContract)
	var missing *proofrun.ProtectedGoTestMissing
	if !errors.As(err, &missing) || missing.Tree != "base" || missing.Test != "TestNotInBase" {
		t.Fatalf("stale base inventory refusal=%v", err)
	}
}

func TestBatchProtectedTestsAcceptTheBaseTree(t *testing.T) {
	root := t.TempDir()
	const base = "4444444444444444444444444444444444444444"
	workspace := protectedTestWorkspace(t, root, map[string]map[string][]byte{
		base: {"app/pkg/value_test.go": []byte("package pkg\nimport \"testing\"\nfunc TestProtected(t *testing.T) {}\n")},
	}, []string{"app/pkg", "app/pkg/value_test.go"}, base)
	tree, err := workspace.TreeOf("HEAD")
	must(t, err)
	contract := testpolicy.Contract{Groups: []testpolicy.Group{{ID: "protected", Adapter: "go", CWD: "app", Packages: []string{"./pkg"}, Tests: json.RawMessage(`["TestProtected"]`)}}}
	if err := proofrun.CheckProtectedGoTests(workspace, tree, tree, contract); err != nil {
		t.Fatalf("base tree rejected its own listed tests: %v", err)
	}
}

func TestDeletedGoPackagesSelectNearestExistingDirectory(t *testing.T) {
	root := t.TempDir()
	baseFiles := map[string]string{
		"metasystem/outer/keep.txt":               "keep\n",
		"metasystem/outer/missing/inner/value.go": "package inner\n",
	}
	for path, content := range baseFiles {
		absolute := filepath.Join(root, filepath.FromSlash(path))
		must(t, os.MkdirAll(filepath.Dir(absolute), 0o755))
		must(t, os.WriteFile(absolute, []byte(content), 0o644))
	}
	candidate := clonePackageFiles(baseFiles)
	delete(candidate, "metasystem/outer/missing/inner/value.go")
	fixture := newPackageTreeFixture(t, root, baseFiles, candidate, "")
	base, tip := fixture.baseTree, fixture.candidateTree
	must(t, os.Remove(filepath.Join(root, "metasystem", "outer", "missing", "inner", "value.go")))
	deletionPatch := []byte("diff --git a/metasystem/outer/missing/inner/value.go b/metasystem/outer/missing/inner/value.go\ndeleted file mode 100644\n--- a/metasystem/outer/missing/inner/value.go\n+++ /dev/null\n@@ -1 +0,0 @@\n-package inner\n")
	packages, err := changedGoPackagesWithWorkspace(root, tip, patchGateChanges(deletionPatch), fixture.workspace())
	must(t, err)
	if !slices.Equal(packages, []string{"./outer/..."}) {
		t.Fatalf("nested deletion packages=%v, want nearest existing parent", packages)
	}

	createDelete := []byte("diff --git a/metasystem/fresh/inner/value.go b/metasystem/fresh/inner/value.go\nnew file mode 100644\n--- /dev/null\n+++ b/metasystem/fresh/inner/value.go\n@@ -0,0 +1 @@\n+package inner\ndiff --git a/metasystem/fresh/inner/value.go b/metasystem/fresh/inner/value.go\ndeleted file mode 100644\n--- a/metasystem/fresh/inner/value.go\n+++ /dev/null\n@@ -1 +0,0 @@\n-package inner\n")
	packages, err = changedGoPackagesWithWorkspace(root, base, patchGateChanges(createDelete), fixture.workspace())
	must(t, err)
	if len(packages) != 0 {
		t.Fatalf("create-delete path selected packages=%v", packages)
	}
	t.Run("branch-member-patch-deletion", func(t *testing.T) {
		foldPatch := []byte("diff --git a/metasystem/outer/missing/inner/value.go b/metasystem/outer/missing/inner/value.go\n--- a/metasystem/outer/missing/inner/value.go\n+++ b/metasystem/outer/missing/inner/value.go\n@@ -1 +1 @@\n-package inner\n+package inner // folded\n")
		member := BranchMember{Builds: []BranchBuild{{Folds: []goalbranch.Commit{{ID: "fold"}}, Commit: "delete"}}}
		calls := []string{}
		patch, err := branchMemberPatchWithReader(root, member, func(repo, commit string) ([]byte, error) {
			if repo != root {
				t.Fatalf("branch patch repo=%q, want %q", repo, root)
			}
			calls = append(calls, commit)
			switch commit {
			case "fold":
				return foldPatch, nil
			case "delete":
				return deletionPatch, nil
			default:
				t.Fatalf("unknown branch patch commit=%q", commit)
				return nil, nil
			}
		})
		must(t, err)
		if !slices.Equal(calls, []string{"fold", "delete"}) || string(patch) != string(foldPatch)+string(deletionPatch) {
			t.Fatalf("branch patch order=%v bytes=%q", calls, patch)
		}
		changes := patchGateChanges(patch)
		if change, ok := changes["metasystem/outer/missing/inner/value.go"]; len(changes) != 1 || !ok || !change.Deleted {
			t.Fatalf("branch deletion changes=%v", changes)
		}
	})
}

func isSharedFixtureHarness(path string) bool {
	switch path {
	case "scripts/agents/fixture-bed-scenarios.sh", "scripts/agents/fixture-budget.sh", "scripts/agents/fixture-assert.sh":
		return true
	}
	return false
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

func TestFixtureGroupsForChangedBeds(t *testing.T) {
	t.Parallel()
	contract, err := testpolicy.Load("../../../testing.json")
	must(t, err)
	for _, row := range strings.Split(strings.TrimSpace(string(fixtureMap(t))), "\n") {
		fields := strings.Split(row, "\t")
		if len(fields) != 2 {
			t.Fatalf("invalid fixture bed row %q", row)
		}
		path, required := "metasystem/"+fields[0], fields[1]
		plan, err := testpolicy.Select(contract, testpolicy.SelectionRequest{
			ChangedPaths: []string{path}, RequestedMode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery,
		})
		if err != nil {
			t.Errorf("%s: select delivery proof: %v", path, err)
			continue
		}
		if !slices.Contains(plan.SelectedGroups, required) {
			t.Errorf("%s: fixture group %s absent from delivery selection; affected=%v selected=%v", path, required,
				plan.AffectedSurfaces, plan.SelectedGroups)
		}
	}
}
