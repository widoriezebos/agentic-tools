package batch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

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
