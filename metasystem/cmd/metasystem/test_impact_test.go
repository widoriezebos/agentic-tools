package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// These adapter fixtures need real Git to distinguish committed trees from
// unstaged and untracked bytes through the public command.
func impactGitAdapterBed(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	contract := testpolicy.Contract{SchemaVersion: 2,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"}}
	group := func(id string) testpolicy.Group {
		return testpolicy.Group{ID: id, Kind: "unit", Adapter: "command", CWD: ".", Phase: "acceptance", EnvironmentMode: "inherit",
			Inputs: []string{"internal/launch/**"}, Platforms: []string{"any"}, TargetMS: 1000, Argv: []string{"/bin/sh", "-c", "test -f go.mod"}, Format: "exit-status"}
	}
	contract.Groups = []testpolicy.Group{group("canary"), group("standard")}
	contract.Groups[1].Inputs = []string{"outside/**"}
	contract.Surfaces = []testpolicy.Surface{{ID: "unit", Paths: []string{"internal/**", "cmd/**"}, Standard: []string{"standard"}}}
	contract.Unknown = []string{"standard"}
	contract.Always.Canary, contract.Always.Standard = []string{"canary"}, []string{"standard"}
	if err := contract.Validate(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	impactWrite(t, root, "testing.json", string(data))
	impactWrite(t, root, "metasystem.conf", "testing.contract=testing.json\n")
	impactWrite(t, root, "go.mod", "module example.invalid/impact\n\ngo 1.27\n")
	impactWrite(t, root, "internal/launch/value.go", "package launch\nconst Value = true\n")
	impactWrite(t, root, "internal/launch/value_test.go", "package launch\nimport \"testing\"\nfunc TestValue(t *testing.T) { t.Parallel(); if !Value { t.Fatal(\"changed value\") } }\n")
	impactWrite(t, root, "cmd/metasystem/main.go", "package main\nfunc main() {}\n")
	testingFixtureGit(t, root, "init", "-q", "-b", "main")
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "base")
	return root, strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
}

func impactWrite(t *testing.T, root, path, text string) {
	t.Helper()
	absolute := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(absolute), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absolute, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func impactPublic(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := dispatchWithFamilies(append([]string{"test", "impact", "--root", root}, args...), &stdout, &stderr, families())
	return code, stdout.String(), stderr.String()
}

func TestTestImpactWorkingSnapshotGoesRedGitAdapter(t *testing.T) {
	t.Parallel()
	root, base := impactGitAdapterBed(t)
	impactWrite(t, root, "internal/launch/value.go", "package launch\nconst Value = false\n")
	impactWrite(t, root, "cmd/metasystem/new_test.go", "package main\nimport \"testing\"\nfunc TestNew(t *testing.T) { t.Parallel() }\n")
	code, out, problem := impactPublic(t, root, "--base", base, "--plan")
	if code != 0 || !strings.Contains(out, "whole: ./internal/launch") || !strings.Contains(out, "by name: ./cmd/metasystem 1 tests (TestNew)") || strings.Contains(out, "whole: ./cmd/metasystem") || strings.Contains(out, "landing group ") || strings.Contains(out, "standard") {
		t.Fatalf("plan exit=%d out=%s problem=%s", code, out, problem)
	}
	code, out, problem = impactPublic(t, root, "--base", base)
	if code != 1 || !strings.Contains(out, "landing group unit/internal/launch red ") || !strings.Contains(problem, "TestValue") || strings.Contains(out, "landing group standard") {
		t.Fatalf("check exit=%d out=%s problem=%s", code, out, problem)
	}
	if strings.Index(out, "whole: ./internal/launch") > strings.Index(out, "landing group ") {
		t.Fatal("tests ran before the plan was printed")
	}
}

func TestTestImpactFixtureOnlySelectsOwnerGitAdapter(t *testing.T) {
	t.Parallel()
	root, base := impactGitAdapterBed(t)
	impactWrite(t, root, "internal/launch/testdata/fixture.txt", "new fixture\n")
	code, out, problem := impactPublic(t, root, "--base", base, "--plan")
	if code != 0 || !strings.Contains(out, "whole: ./internal/launch") {
		t.Fatalf("untracked fixture exit=%d out=%s problem=%s", code, out, problem)
	}
	testingFixtureGit(t, root, "add", "internal/launch/testdata/fixture.txt")
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	base = strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	impactWrite(t, root, "internal/launch/testdata/fixture.txt", "modified fixture\n")
	code, out, problem = impactPublic(t, root, "--base", base, "--plan")
	if code != 0 || !strings.Contains(out, "whole: ./internal/launch") {
		t.Fatalf("modified fixture exit=%d out=%s problem=%s", code, out, problem)
	}
}

func TestTestImpactUnreadableTestFailsGitAdapter(t *testing.T) {
	t.Parallel()
	root, base := impactGitAdapterBed(t)
	impactWrite(t, root, "internal/launch/broken_test.go", "package launch\nfunc TestBroken(\n")
	code, out, problem := impactPublic(t, root, "--base", base, "--plan")
	if code != 1 || !strings.Contains(problem, "broken_test.go") || strings.Contains(out, "landing group ") {
		t.Fatalf("parse refusal exit=%d out=%s problem=%s", code, out, problem)
	}
	code, _, problem = impactPublic(t, root, "--base", "", "--plan")
	if code != 2 || !strings.Contains(problem, "--base") || !strings.Contains(problem, "LANDING_PROOF_BASE") {
		t.Fatalf("missing base exit=%d problem=%s", code, problem)
	}
}

func TestTestImpactContractGroupsStayCheapGitAdapter(t *testing.T) {
	t.Parallel()
	root, base := impactGitAdapterBed(t)
	data, err := os.ReadFile(filepath.Join(root, "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contract testpolicy.Contract
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	contract.Groups[0].Inputs = []string{"cmd/metasystem/main.go"}
	for _, id := range []string{"admission", "acceptance", "section/full", "broad-cmd", "broad-canary"} {
		group := contract.Groups[0]
		group.ID, group.Phase, group.Kind = id, "admission", "static"
		if id == "acceptance" {
			group.Phase = "acceptance"
		}
		if strings.HasPrefix(id, "broad-") {
			group.Inputs = []string{"cmd/**"}
		}
		contract.Groups = append(contract.Groups, group)
	}
	contract.Always.Canary = append(contract.Always.Canary, "broad-canary")
	if err := contract.Validate(); err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	impactWrite(t, root, "testing.json", string(data))
	impactWrite(t, root, "cmd/metasystem/main.go", "package main\nfunc main() {} // edited\n")
	code, out, problem := impactPublic(t, root, "--base", base, "--plan")
	if code != 0 || out != "groups: canary, admission\nwhole: ./cmd/metasystem\n" {
		t.Fatalf("cheap groups exit=%d out=%s problem=%s", code, out, problem)
	}
}

func TestTestImpactCmdDeclarationHunksGitAdapter(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, before, after, reader string }{
		{"function", "func changed() string {\n localOnly := 1\n _ = localOnly\n return \"before-message\"\n}\n", "func changed() string {\n localOnly := 2\n _ = localOnly\n return \"after-message\"\n}\n", "_ = changed()"},
		{"method", "type receiver struct{}\nfunc (receiver) changed() int { return 1 }\n", "type receiver struct{}\nfunc (receiver) changed() int { return 2 }\n", "_ = (receiver{}).changed()"},
		{"type", "type changed int\n", "type changed string\n", "var _ changed"},
		{"const", "const changed = 1\n", "const changed = 2\n", "_ = changed"},
		{"var", "var changed = 1\n", "var changed = 2\n", "_ = changed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, _ := impactGitAdapterBed(t)
			impactWrite(t, root, "cmd/metasystem/changed.go", "package main\n"+tc.before+"func untouched() string { return \"untouched-message\" }\n")
			impactWrite(t, root, "cmd/metasystem/changed_test.go", "package main\nimport \"testing\"\nfunc TestOwn(t *testing.T) { t.Parallel(); "+tc.reader+" }\n")
			impactWrite(t, root, "cmd/metasystem/reader_test.go", "package main\nimport \"testing\"\nfunc TestReader(t *testing.T) { t.Parallel(); "+tc.reader+" }\n")
			for i := 0; i < 40; i++ {
				impactWrite(t, root, fmt.Sprintf("cmd/metasystem/noise%d_test.go", i), fmt.Sprintf("package main\nimport \"testing\"\nfunc TestNoise%d(t *testing.T) { t.Parallel(); _ = \"untouched-message\"; _ = untouched; _ = \"localOnly\"; _ = \"ok\" }\n", i))
			}
			testingFixtureGit(t, root, "add", ".")
			testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "declarations")
			base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
			impactWrite(t, root, "cmd/metasystem/changed.go", "package main\n"+tc.after+"func untouched() string { return \"untouched-message\" }\n")
			code, out, problem := impactPublic(t, root, "--base", base, "--plan")
			if code != 0 || !strings.Contains(out, "by name: ./cmd/metasystem 2 tests (TestOwn, TestReader)") || strings.Contains(out, "TestNoise") || strings.Contains(out, "whole: ./cmd/") {
				t.Fatalf("declaration selection exit=%d out=%s problem=%s", code, out, problem)
			}
		})
	}
}

func TestTestImpactCmdFixtureBasenameGitAdapter(t *testing.T) {
	t.Parallel()
	root, _ := impactGitAdapterBed(t)
	impactWrite(t, root, "cmd/metasystem/testdata/layout/help-test.txt", "before\n")
	impactWrite(t, root, "cmd/metasystem/layout_test.go", "package main\nimport (\"testing\"; \"os\"; \"path/filepath\")\nfunc TestHelpFixture(t *testing.T) { t.Parallel(); data, err := os.ReadFile(filepath.Join(\"testdata\", \"layout\", \"help-test.txt\")); if err != nil || string(data) != \"before\\n\" { t.Fatalf(\"fixture: %s %v\", data, err) } }\n")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "layout")
	base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	impactWrite(t, root, "cmd/metasystem/testdata/layout/help-test.txt", "after\n")
	code, out, problem := impactPublic(t, root, "--base", base)
	if code != 1 || !strings.Contains(out, "by name: ./cmd/metasystem 1 tests (TestHelpFixture)") || !strings.Contains(out, "landing group unit/cmd/metasystem red") || !strings.Contains(problem, "TestHelpFixture") {
		t.Fatalf("fixture selection exit=%d out=%s problem=%s", code, out, problem)
	}
}

func TestTestImpactImportersAndDeletedSymbolsGitAdapter(t *testing.T) {
	t.Parallel()
	root, _ := impactGitAdapterBed(t)
	impactWrite(t, root, "internal/launch/messages.go", "package launch\nfunc Removed() string { return \"removed-message\" }\nfunc Retained() string { return \"unchanged-message\" }\n")
	impactWrite(t, root, "cmd/metasystem/read_test.go", "package main\nimport (\"testing\"; \"example.invalid/impact/internal/launch\")\nfunc TestImporter(t *testing.T) { t.Parallel(); _ = launch.Removed }\n")
	impactWrite(t, root, "cmd/metasystem/string_test.go", "package main\nimport \"testing\"\nfunc TestOldMessage(t *testing.T) { t.Parallel(); _ = \"removed-message\" }\n")
	impactWrite(t, root, "cmd/metasystem/new_message_test.go", "package main\nimport \"testing\"\nfunc TestNewMessage(t *testing.T) { t.Parallel(); _ = \"replacement-message\" }\n")
	impactWrite(t, root, "cmd/metasystem/noise_test.go", "package main\nimport \"testing\"\nfunc TestNoise(t *testing.T) { t.Parallel(); _ = \"unchanged-message\"; _ = \"ok\" }\n")
	impactWrite(t, root, "internal/reader/reader_test.go", "package reader\nimport (\"testing\"; \"example.invalid/impact/internal/launch\")\nfunc TestInternalImporter(t *testing.T) { t.Parallel(); _ = launch.Removed }\n")
	impactWrite(t, root, "internal/reader/noise_test.go", "package reader\nimport \"testing\"\nfunc TestInternalNoise(t *testing.T) { t.Parallel() }\n")
	impactWrite(t, root, "cmd/unrelated/noise_test.go", "package unrelated\nimport \"testing\"\nfunc TestUnrelated(t *testing.T) { t.Parallel(); _ = \"removed-message\" }\n")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "importers")
	base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	impactWrite(t, root, "internal/launch/messages.go", "package launch\nfunc Added() string { return \"replacement-message\" }\nfunc Short() string { return \"ok\" }\nfunc Retained() string { return \"unchanged-message\" }\n")
	code, out, problem := impactPublic(t, root, "--base", base, "--plan")
	if code != 0 || !strings.Contains(out, "whole: ./internal/launch") || !strings.Contains(out, "by name: ./cmd/metasystem 3 tests (TestImporter, TestNewMessage, TestOldMessage)") || strings.Contains(out, "whole: ./cmd/metasystem") || !strings.Contains(out, "whole: ./internal/reader") || strings.Contains(out, "TestNoise") || strings.Contains(out, "TestUnrelated") {
		t.Fatalf("importer selection exit=%d out=%s problem=%s", code, out, problem)
	}
}

func TestTestImpactChangedTestHunksGitAdapter(t *testing.T) {
	t.Parallel()
	root, _ := impactGitAdapterBed(t)
	before := "package main\nimport \"testing\"\nfunc TestEdited(t *testing.T) { t.Parallel(); _ = 1 }\nfunc TestUntouched(t *testing.T) { t.Parallel() }\n"
	impactWrite(t, root, "cmd/metasystem/shared_test.go", before)
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "tests")
	base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	impactWrite(t, root, "cmd/metasystem/shared_test.go", strings.Replace(before, "_ = 1", "_ = 2", 1))
	code, out, problem := impactPublic(t, root, "--base", base, "--plan")
	if code != 0 || !strings.Contains(out, "by name: ./cmd/metasystem 1 tests (TestEdited)") || strings.Contains(out, "TestUntouched") {
		t.Fatalf("changed tests exit=%d out=%s problem=%s", code, out, problem)
	}
}

func TestTestImpactShippedCheapDeclarationGitAdapter(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "metasystem.conf"))
	if err != nil {
		t.Fatal(err)
	}
	declaration := ""
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "proof.cheap=") {
			declaration = strings.TrimPrefix(line, "proof.cheap=")
		}
	}
	if declaration != "metasystem test impact" {
		t.Fatalf("shipped cheap proof = %q", declaration)
	}
	root, base := impactGitAdapterBed(t)
	impactWrite(t, root, "internal/launch/value.go", "package launch\nconst Value = false\n")
	var stdout, stderr bytes.Buffer
	args := append(strings.Fields(declaration)[1:], "--root", root, "--base", base, "--plan")
	code := dispatchWithFamilies(args, &stdout, &stderr, families())
	if code != 0 || !strings.Contains(stdout.String(), "whole: ./internal/launch") {
		t.Fatalf("shipped command exit=%d out=%s problem=%s", code, &stdout, &stderr)
	}
}

func TestTestImpactCmdCommentSelectsOwnTestsGitAdapter(t *testing.T) {
	t.Parallel()
	root, _ := impactGitAdapterBed(t)
	source := "package main\nfunc carry() {}\n"
	impactWrite(t, root, "cmd/metasystem/carry.go", source)
	impactWrite(t, root, "cmd/metasystem/carry_test.go", "package main\nimport \"testing\"\nfunc TestCarry(t *testing.T) { t.Parallel(); carry() }\n")
	impactWrite(t, root, "cmd/metasystem/unrelated_test.go", "package main\nimport \"testing\"\nfunc TestUnrelated(t *testing.T) { t.Parallel() }\n")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "carry")
	base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	impactWrite(t, root, "cmd/metasystem/carry.go", source+"// appended comment\n")
	code, out, problem := impactPublic(t, root, "--base", base, "--plan")
	if code != 0 || !strings.Contains(out, "by name: ./cmd/metasystem 1 tests (TestCarry)") || strings.Contains(out, "TestUnrelated") {
		t.Fatalf("own test selection exit=%d out=%s problem=%s", code, out, problem)
	}
}

func TestTestImpactChangedPackageWithoutSelectionRunsWholeGitAdapter(t *testing.T) {
	t.Parallel()
	root, _ := impactGitAdapterBed(t)
	source := "package main\nfunc toolVersionLine() string { return \"version\" }\n"
	impactWrite(t, root, "cmd/metasystem/app.go", source)
	impactWrite(t, root, "cmd/metasystem/other_test.go", "package main\nimport \"testing\"\nfunc TestExisting(t *testing.T) { t.Parallel(); t.Fatal(\"existing package failure\") }\n")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "unreferenced function")
	base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	impactWrite(t, root, "cmd/metasystem/app.go", strings.Replace(source, "return", "_ = 0; return", 1))
	code, out, problem := impactPublic(t, root, "--base", base, "--plan")
	if code != 0 || !strings.Contains(out, "whole: ./cmd/metasystem") {
		t.Fatalf("unselected changed package exit=%d out=%s problem=%s", code, out, problem)
	}
	code, out, problem = impactPublic(t, root, "--base", base)
	if code != 1 || !strings.Contains(out, "landing group unit/cmd/metasystem red ") || !strings.Contains(problem, "TestExisting") {
		t.Fatalf("whole package check exit=%d out=%s problem=%s", code, out, problem)
	}
}

func TestTestImpactFixtureFolderSelectsNamedTestsGitAdapter(t *testing.T) {
	t.Parallel()
	root, _ := impactGitAdapterBed(t)
	impactWrite(t, root, "cmd/metasystem/testdata/signedinlaunchgit/input.txt", "before\n")
	impactWrite(t, root, "cmd/metasystem/signed_in_launch_e2e_test.go", "package main\nimport (\"testing\"; \"os\"; \"path/filepath\")\nfunc TestSignedInLaunch(t *testing.T) { t.Parallel(); root := filepath.Join(\"testdata\", \"signedinlaunchgit\"); entries, err := os.ReadDir(root); if err != nil { t.Fatal(err) }; data, err := os.ReadFile(filepath.Join(root, entries[0].Name())); if err != nil || string(data) != \"before\\n\" { t.Fatalf(\"fixture: %s %v\", data, err) } }\n")
	impactWrite(t, root, "cmd/metasystem/other_test.go", "package main\nimport \"testing\"\nfunc TestUnrelated(t *testing.T) { t.Parallel() }\n")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture folder")
	base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	impactWrite(t, root, "cmd/metasystem/testdata/signedinlaunchgit/input.txt", "after\n")
	code, out, problem := impactPublic(t, root, "--base", base, "--plan")
	if code != 0 || !strings.Contains(out, "by name: ./cmd/metasystem 1 tests (TestSignedInLaunch)") || strings.Contains(out, "TestUnrelated") || strings.Contains(out, "whole: ./cmd/metasystem") {
		t.Fatalf("fixture folder exit=%d out=%s problem=%s", code, out, problem)
	}
	code, out, problem = impactPublic(t, root, "--base", base)
	if code != 1 || !strings.Contains(out, "landing group unit/cmd/metasystem red ") || !strings.Contains(problem, "TestSignedInLaunch") {
		t.Fatalf("fixture check exit=%d out=%s problem=%s", code, out, problem)
	}
}

func TestTestImpactDeletedPackageSelectsImporterGitAdapter(t *testing.T) {
	t.Parallel()
	for _, directory := range []string{"internal/gone", "."} {
		t.Run(directory, func(t *testing.T) {
			t.Parallel()
			root, _ := impactGitAdapterBed(t)
			source := filepath.Join(directory, "gone.go")
			imported := filepath.ToSlash(filepath.Join("example.invalid/impact", directory))
			impactWrite(t, root, source, "package gone\nfunc Removed() {}\n")
			impactWrite(t, root, "internal/reader/reader_test.go", fmt.Sprintf("package reader\nimport (\"testing\"; %q)\nfunc TestDeletedImporter(t *testing.T) { t.Parallel(); gone.Removed() }\n", imported))
			testingFixtureGit(t, root, "add", ".")
			testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "imported package")
			base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
			if err := os.Remove(filepath.Join(root, source)); err != nil {
				t.Fatal(err)
			}
			code, out, problem := impactPublic(t, root, "--base", base, "--plan")
			if code != 0 || !strings.Contains(out, "whole: ./internal/reader") {
				t.Fatalf("deleted package importer exit=%d out=%s problem=%s", code, out, problem)
			}
			code, out, problem = impactPublic(t, root, "--base", base)
			if code != 1 || !strings.Contains(problem, "testing group unit/internal/reader: go package discovery failed") || !strings.Contains(problem, imported) || strings.Contains(out, "landing group unit/internal/reader green ") {
				t.Fatalf("deleted import check exit=%d out=%s problem=%s", code, out, problem)
			}
		})
	}
}

func TestTestImpactReverseDependentsRunWholeGitAdapter(t *testing.T) {
	t.Parallel()
	root, _ := impactGitAdapterBed(t)
	bridge := t.TempDir()
	impactWrite(t, bridge, "go.mod", "module example.invalid/impact/bridge\n\ngo 1.27\n")
	impactWrite(t, bridge, "bridge.go", "package bridge\nimport _ \"example.invalid/impact/internal/launch\"\n")
	impactWrite(t, root, "go.mod", fmt.Sprintf("module example.invalid/impact\n\ngo 1.27\nrequire example.invalid/impact/bridge v0.0.0\nreplace example.invalid/impact/bridge => %s\n", bridge))
	for path, source := range map[string]string{
		"internal/direct/value.go":        "package direct\nimport _ \"example.invalid/impact/internal/launch\"\n",
		"internal/transitive/value.go":    "package transitive\nimport _ \"example.invalid/impact/internal/direct\"\n",
		"internal/testonly/value.go":      "package testonly\n",
		"internal/testonly/value_test.go": "package testonly\nimport (\"testing\"; _ \"example.invalid/impact/internal/launch\")\nfunc TestContract(t *testing.T) { t.Parallel() }\n",
		"internal/external/value.go":      "package external\n",
		"internal/external/value_test.go": "package external_test\nimport (\"testing\"; _ \"example.invalid/impact/internal/launch\")\nfunc TestContract(t *testing.T) { t.Parallel() }\n",
		"cmd/metasystem/import.go":        "package main\nimport _ \"example.invalid/impact/internal/launch\"\n",
		"internal/unrelated/value.go":     "package unrelated\n",
		"internal/throughmodule/value.go": "package throughmodule\nimport _ \"example.invalid/impact/bridge\"\n",
	} {
		impactWrite(t, root, path, source)
	}
	impactWrite(t, root, "internal/direct/value_test.go", "package direct\nimport \"testing\"\nfunc TestContract(t *testing.T) { t.Parallel(); t.Fatal(\"importer fixture broke\") }\n")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "dependencies")
	base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	impactWrite(t, root, "internal/launch/value.go", "package launch\nconst Value = true // changed implementation\n")
	code, out, problem := impactPublic(t, root, "--base", base, "--plan")
	for _, pkg := range []string{"internal/direct", "internal/transitive", "internal/testonly", "internal/external", "internal/throughmodule"} {
		if code != 0 || !strings.Contains(out, "whole: ./"+pkg+"\n") {
			t.Fatalf("dependent %s exit=%d out=%s problem=%s", pkg, code, out, problem)
		}
	}
	if strings.Contains(out, "internal/unrelated") || strings.Contains(out, "cmd/metasystem") {
		t.Fatalf("unrelated package selected: %s", out)
	}
	code, out, problem = impactPublic(t, root, "--base", base)
	if code != 1 || !strings.Contains(out, "landing group unit/internal/direct red ") || !strings.Contains(problem, "TestContract") {
		t.Fatalf("importer failure exit=%d out=%s problem=%s", code, out, problem)
	}
}

func TestTestImpactChangedHelperSelectsCallersGitAdapter(t *testing.T) {
	t.Parallel()
	root, _ := impactGitAdapterBed(t)
	helper := "package main\nfunc helperValue() bool {\n return true\n}\n"
	impactWrite(t, root, "cmd/metasystem/z_helper_test.go", helper)
	impactWrite(t, root, "cmd/metasystem/a_caller_test.go", "package main\nimport \"testing\"\nfunc TestHelperCaller(t *testing.T) { t.Parallel(); if !helperValue() { t.Fatal(\"helper changed\") } }\n")
	impactWrite(t, root, "cmd/metasystem/noise_test.go", "package main\nimport \"testing\"\nfunc TestUnrelated(t *testing.T) { t.Parallel() }\n")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "helper")
	base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	impactWrite(t, root, "cmd/metasystem/z_helper_test.go", strings.Replace(helper, "true", "false", 1))
	code, out, problem := impactPublic(t, root, "--base", base)
	if code != 1 || !strings.Contains(out, "by name: ./cmd/metasystem 1 tests (TestHelperCaller)") || !strings.Contains(out, "landing group unit/cmd/metasystem red ") || !strings.Contains(problem, "TestHelperCaller") || strings.Contains(out, "TestUnrelated") || strings.Contains(out, "whole: ./cmd/metasystem") {
		t.Fatalf("helper caller exit=%d out=%s problem=%s", code, out, problem)
	}
}

func TestTestImpactNonCommitBaseRefusesGitAdapter(t *testing.T) {
	t.Parallel()
	root, _ := impactGitAdapterBed(t)
	for _, base := range []string{"HEAD^{tree}", "HEAD:go.mod", "missing-commit"} {
		code, out, problem := impactPublic(t, root, "--base", base, "--plan")
		if code != 1 || !strings.Contains(problem, "is not a readable commit") || !strings.Contains(problem, base) || out != "" {
			t.Fatalf("non-commit base %q exit=%d out=%s problem=%s", base, code, out, problem)
		}
	}
}

func TestTestImpactInternalImporterRunsWholeGitAdapter(t *testing.T) {
	t.Parallel()
	root, _ := impactGitAdapterBed(t)
	impactWrite(t, root, "internal/reader/reader.go", "package reader\nimport _ \"example.invalid/impact/internal/launch\"\n")
	impactWrite(t, root, "internal/reader/noise_test.go", "package reader\nimport \"testing\"\nfunc TestInternalNoise(t *testing.T) { t.Parallel(); t.Fatal(\"importer contract broke\") }\n")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "internal importer")
	base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	impactWrite(t, root, "internal/launch/value.go", "package launch\nconst Value = true // changed implementation\n")
	code, out, problem := impactPublic(t, root, "--base", base)
	if code != 1 || !strings.Contains(out, "whole: ./internal/reader\n") || !strings.Contains(out, "landing group unit/internal/reader red ") || !strings.Contains(problem, "TestInternalNoise") {
		t.Fatalf("internal importer exit=%d out=%s problem=%s", code, out, problem)
	}
}
