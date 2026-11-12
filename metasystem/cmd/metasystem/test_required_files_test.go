package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// These beds use Git because the public impact command compares committed and
// working trees; independent packages keep the selection below the cost cap.
func requiredFilesGitAdapterBed(t *testing.T, inputs []string) (root, base, markers string) {
	t.Helper()
	root, markers = impactAdapterBed(t), t.TempDir()
	contract, err := testpolicy.Load(filepath.Join(root, "testing.json"))
	helmMust(t, err)
	contract.Groups = append(contract.Groups, testpolicy.Group{ID: "required-public", Kind: "unit", Adapter: "go", CWD: ".", Phase: "acceptance", EnvironmentMode: "inherit", Inputs: inputs, Platforms: []string{"any"}, TargetMS: 1000, Packages: []string{"./cmd/metasystem"}, RequiredTestFiles: []string{"cmd/metasystem/required_test.go"}})
	helmMust(t, contract.Validate())
	data, err := json.Marshal(contract)
	helmMust(t, err)
	impactWrite(t, root, "testing.json", string(data))
	impactWrite(t, root, "metasystem.conf", "testing.contract=testing.json\nlanding.impact-max-share=50\n")
	impactWrite(t, root, ".gitignore", "artifacts/\n")
	impactWrite(t, root, "cmd/metasystem/required_test.go", fmt.Sprintf("package main\nimport (\"testing\"; \"os\")\nfunc TestRequiredFirst(t *testing.T) { t.Parallel(); if err := os.WriteFile(%q, []byte(\"ran\"), 0600); err != nil { t.Fatal(err) } }\nfunc TestRequiredSecond(t *testing.T) { t.Parallel(); if err := os.WriteFile(%q, []byte(\"ran\"), 0600); err != nil { t.Fatal(err) } }\n", filepath.Join(markers, "first"), filepath.Join(markers, "second")))
	impactWrite(t, root, "cmd/metasystem/unmapped_test.go", fmt.Sprintf("package main\nimport (\"testing\"; \"os\")\nfunc TestUnmapped(t *testing.T) { t.Parallel(); _ = os.WriteFile(%q, nil, 0600) }\n", filepath.Join(markers, "unmapped")))
	for i := range 8 {
		impactWrite(t, root, fmt.Sprintf("untouched/p%d/value_test.go", i), "package untouched\nimport \"testing\"\nfunc TestUnchanged(t *testing.T) { t.Parallel() }\n")
	}
	testingFixtureGit(t, root, "init", "-q", "-b", "main")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "required public files")
	return root, strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD")), markers
}

func proveRequiredPublicFiles(t *testing.T, inputs []string) {
	t.Helper()
	root, base, markers := requiredFilesGitAdapterBed(t, inputs)
	impactWrite(t, root, "internal/launch/value.go", "package launch\nconst Value = true // edited\n")
	testingFixtureGit(t, root, "add", ".")
	code, out, problem := impactPublic(t, root, "--base", base, "--plan", "--json", "--check")
	result, err := verbresult.Read([]byte(out), "test impact", code, problem)
	var plan plain.ImpactPlan
	if code != 0 || err != nil || result.DecodeData(&plan) != nil {
		t.Fatalf("plan exit=%d err=%v out=%s stderr=%s", code, err, out, problem)
	}
	var share, full int
	if _, err := fmt.Sscanf(plan.CheckReason, "check: impact; covers %d%% of %d packages", &share, &full); err != nil || share >= 50 || full < 10 {
		t.Fatalf("fixture did not exercise cheap selection: %+v error=%v", plan, err)
	}
	t.Log(plan.CheckReason)
	if plan.Check != "impact" || !slices.Contains(plan.Selections, "cmd/metasystem=TestRequiredFirst,TestRequiredSecond") || slices.Contains(plan.Selections, "required-public") {
		t.Fatalf("required file tests absent from named selection: %+v", plan)
	}
	code, out, problem = impactPublic(t, root, "--base", base, "--check")
	if code != 0 || !strings.Contains(out, "required: required-public selects 2 tests of cmd/metasystem/required_test.go") {
		t.Fatalf("check exit=%d stdout=%s stderr=%s", code, out, problem)
	}
	for _, name := range []string{"first", "second"} {
		data, err := os.ReadFile(filepath.Join(markers, name))
		if err != nil || string(data) != "ran" {
			t.Fatalf("required test %s did not run: %q %v", name, data, err)
		}
	}
	if _, err := os.Stat(filepath.Join(markers, "unmapped")); !os.IsNotExist(err) {
		t.Fatalf("unmapped test ran: %v", err)
	}
	t.Log("both required test markers exist; the unmapped test marker is absent")
}

func TestRequiredPublicFilesBypassSymbolReachability(t *testing.T) {
	t.Parallel()
	proveRequiredPublicFiles(t, []string{"internal/launch/**"})
}

func TestRequiredPublicFilesSurviveGroupFiltering(t *testing.T) {
	t.Parallel()
	proveRequiredPublicFiles(t, []string{"internal/launch/**", "cmd/**"})
}

func TestRequiredPublicFilesUnionWithExistingSelection(t *testing.T) {
	t.Parallel()
	for _, whole := range []bool{false, true} {
		t.Run(fmt.Sprint(whole), func(t *testing.T) {
			t.Parallel()
			root, base, _ := requiredFilesGitAdapterBed(t, []string{"internal/launch/**"})
			impactWrite(t, root, "internal/launch/value.go", "package launch\nconst Value = true // edited\n")
			want := "cmd/metasystem=TestRequiredFirst,TestRequiredSecond,TestUnmapped"
			if whole {
				impactWrite(t, root, "cmd/metasystem/fixture.txt", "changed package resource\n")
				want = "cmd/metasystem"
			} else {
				path := "cmd/metasystem/unmapped_test.go"
				data, err := os.ReadFile(filepath.Join(root, path))
				helmMust(t, err)
				impactWrite(t, root, path, strings.Replace(string(data), "t.Parallel();", "t.Parallel(); t.Log(42);", 1))
			}
			code, out, problem := impactPublic(t, root, "--base", base, "--plan", "--json")
			result, err := verbresult.Read([]byte(out), "test impact", code, problem)
			var plan plain.ImpactPlan
			if code != 0 || err != nil || result.DecodeData(&plan) != nil || !slices.Contains(plan.Selections, want) {
				t.Fatalf("union exit=%d plan=%+v want=%s error=%v stdout=%s stderr=%s", code, plan, want, err, out, problem)
			}
		})
	}
}

func TestRequiredPublicFilesExpansionRefusesBeforeExecution(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"missing", "unparsable"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			root, _, markers := requiredFilesGitAdapterBed(t, []string{"internal/launch/**"})
			path := "cmd/metasystem/required_test.go"
			if failure == "missing" {
				helmMust(t, os.Remove(filepath.Join(root, path)))
			} else {
				impactWrite(t, root, path, "package main\nfunc TestBroken(\n")
			}
			// Only the declared input changes after this commit, leaving the
			// mapped file's error to required-file expansion.
			testingFixtureGit(t, root, "add", ".")
			testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "mapped file unavailable")
			base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
			impactWrite(t, root, "internal/launch/value.go", "package launch\nconst Value = true // edited\n")
			testingFixtureGit(t, root, "add", ".")
			code, out, problem := impactPublic(t, root, "--base", base, "--check")
			if code != 1 || !strings.Contains(problem, "required test selector required-public") || !strings.Contains(problem, path) || strings.Contains(out, "landing group ") {
				t.Fatalf("expansion failure exit=%d stdout=%s stderr=%s", code, out, problem)
			}
			for _, name := range []string{"first", "second", "unmapped"} {
				if _, err := os.Stat(filepath.Join(markers, name)); !os.IsNotExist(err) {
					t.Fatalf("test %s ran despite expansion error: %v", name, err)
				}
			}
		})
	}
}

// The same candidate contract crosses the unit, lane and trusted-policy
// command readers. Diagnostic planning avoids unrelated goal accounting.
func TestRequiredSelectorContractIsReadByEveryReader(t *testing.T) {
	t.Parallel()
	root, base, _ := requiredFilesGitAdapterBed(t, []string{"internal/launch/**"})
	testingFixtureGit(t, root, "update-ref", "refs/remotes/origin/main", base)
	testingFixtureGit(t, root, "config", "metasystem.steward.landing-ref", "refs/remotes/origin/main")
	enrollFixturePolicyEngine(t, root, time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC), base, &fixturePolicyEngine{path: filepath.Join(t.TempDir(), "metasystem")})
	tree := strings.TrimSpace(testingFixtureGit(t, root, "write-tree"))
	for _, args := range [][]string{
		{"test", "impact", "--root", root, "--base", base, "--check"},
		{"test", "impact", "--root", root, "--base", base, "--plan", "--json"},
		{"test", "plan", "--root", root, "--tree", tree, "--json", "--policy-child", "--purpose", "diagnostic"},
	} {
		var out, problem bytes.Buffer
		if code := dispatchWithFamilies(args, &out, &problem, families()); code != 0 {
			t.Fatalf("reader %v exit=%d stdout=%s stderr=%s", args, code, &out, &problem)
		}
	}
}
