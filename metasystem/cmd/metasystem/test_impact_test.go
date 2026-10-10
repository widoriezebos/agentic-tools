package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func impactAdapterBed(t *testing.T) string {
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
	return root
}

// The adapter fixtures use Git to compare committed trees with working bytes.
func impactGitAdapterBed(t *testing.T) (string, string) {
	t.Helper()
	root := impactAdapterBed(t)
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
	if code != 0 || !strings.Contains(out, "selection: internal/launch\n") || !strings.Contains(out, "selection: cmd/metasystem=TestNew") || strings.Contains(out, "selection: cmd/metasystem\n") || strings.Contains(out, "landing group ") || strings.Contains(out, "standard") {
		t.Fatalf("plan exit=%d out=%s problem=%s", code, out, problem)
	}
	code, out, problem = impactPublic(t, root, "--base", base)
	if code != 1 || !strings.Contains(out, "landing group unit/internal/launch red ") || !strings.Contains(problem, "TestValue") || strings.Contains(out, "landing group standard") {
		t.Fatalf("check exit=%d out=%s problem=%s", code, out, problem)
	}
	if strings.Index(out, "selection: internal/launch\n") > strings.Index(out, "landing group ") {
		t.Fatal("tests ran before the plan was printed")
	}
}

func TestTestImpactFixtureOnlySelectsOwnerGitAdapter(t *testing.T) {
	t.Parallel()
	root, base := impactGitAdapterBed(t)
	impactWrite(t, root, "internal/launch/testdata/fixture.txt", "new fixture\n")
	code, out, problem := impactPublic(t, root, "--base", base, "--plan")
	if code != 0 || !strings.Contains(out, "selection: internal/launch\n") {
		t.Fatalf("untracked fixture exit=%d out=%s problem=%s", code, out, problem)
	}
	testingFixtureGit(t, root, "add", "internal/launch/testdata/fixture.txt")
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	base = strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	impactWrite(t, root, "internal/launch/testdata/fixture.txt", "modified fixture\n")
	code, out, problem = impactPublic(t, root, "--base", base, "--plan")
	if code != 0 || !strings.Contains(out, "selection: internal/launch\n") {
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
	if code != 0 || out != "plan: base "+base+" (base)\nselection: canary\nselection: admission\nselection: cmd/metasystem\n" {
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
			if code != 0 || !strings.Contains(out, "selection: cmd/metasystem=TestOwn,TestReader") || strings.Contains(out, "TestNoise") || strings.Contains(out, "selection: cmd/metasystem\n") {
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
	if code != 1 || !strings.Contains(out, "selection: cmd/metasystem=TestHelpFixture") || !strings.Contains(out, "landing group unit/cmd/metasystem red") || !strings.Contains(problem, "TestHelpFixture") {
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
	if code != 0 || !strings.Contains(out, "selection: internal/launch\n") || !strings.Contains(out, "selection: cmd/metasystem=TestImporter,TestNewMessage,TestOldMessage") || strings.Contains(out, "selection: cmd/metasystem\n") || !strings.Contains(out, "selection: internal/reader\n") || strings.Contains(out, "TestNoise") || strings.Contains(out, "TestUnrelated") {
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
	if code != 0 || !strings.Contains(out, "selection: cmd/metasystem=TestEdited") || strings.Contains(out, "TestUntouched") {
		t.Fatalf("changed tests exit=%d out=%s problem=%s", code, out, problem)
	}
}

func TestTestImpactChangedTestMainSelectsNoTestNameGitAdapter(t *testing.T) {
	t.Parallel()
	root, _ := impactGitAdapterBed(t)
	before := "package main\nimport (\"os\"; \"testing\")\nfunc TestMain(m *testing.M) { _ = 1; os.Exit(m.Run()) }\nfunc TestUntouched(t *testing.T) { t.Parallel() }\n"
	impactWrite(t, root, "cmd/metasystem/main_test.go", before)
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "test entrypoint fixture")
	base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	impactWrite(t, root, "cmd/metasystem/main_test.go", strings.Replace(before, "_ = 1", "_ = 2", 1))
	code, out, problem := impactPublic(t, root, "--base", base, "--plan")
	if code != 0 || !strings.Contains(out, "selection: cmd/metasystem\n") || strings.Contains(out, "cmd/metasystem=") {
		t.Fatalf("TestMain change must select the package without test names: exit=%d out=%s problem=%s", code, out, problem)
	}
}

func TestTestImpactSelectsOnlyGoTestNamesGitAdapter(t *testing.T) {
	t.Parallel()
	root, base := impactGitAdapterBed(t)
	source := `package main
import ("os"; "testing")
func Test(t *testing.T) { t.Parallel(); if err := os.WriteFile("bare-test-ran", []byte("ran"), 0600); err != nil { t.Fatal(err) } }
func Testhelper(t *testing.T) { t.Parallel() }
func TestéHelper(t *testing.T) { t.Parallel() }
type helper struct{}
func (helper) TestMethod(t *testing.T) { t.Parallel() }
func TestPresent(t *testing.T) { t.Parallel() }
func TestÉPresent(t *testing.T) { t.Parallel() }
`
	impactWrite(t, root, "cmd/metasystem/helpers_test.go", source)
	code, out, problem := impactPublic(t, root, "--base", base, "--plan")
	if code != 0 || !strings.Contains(out, "selection: cmd/metasystem=Test,TestPresent,TestÉPresent\n") {
		t.Fatalf("Go test names only: exit=%d out=%s problem=%s", code, out, problem)
	}
	// Go's vet rejects lowercase pseudo-test declarations in runnable fixtures.
	impactWrite(t, root, "cmd/metasystem/helpers_test.go", strings.NewReplacer("Testhelper", "helperLower", "TestéHelper", "helperUnicodeLower").Replace(source))
	code, out, problem = impactPublic(t, root, "--base", base)
	if code != 0 || !strings.Contains(out, "landing group unit/cmd/metasystem green") {
		t.Fatalf("Go test execution: exit=%d out=%s problem=%s", code, out, problem)
	}
	if _, err := os.Stat(filepath.Join(root, "cmd/metasystem/bare-test-ran")); err != nil {
		t.Fatalf("bare Test did not execute: %v", err)
	}
}

func TestTestImpactTestMainReferencesSelectDependentGitAdapter(t *testing.T) {
	t.Parallel()
	for _, separate := range []bool{false, true} {
		t.Run(fmt.Sprintf("separate-test-file=%t", separate), func(t *testing.T) {
			t.Parallel()
			root, _ := impactGitAdapterBed(t)
			impactWrite(t, root, "internal/launch/setup.go", "package launch\nfunc Setup() bool { return false }\n")
			mainSource := `package tool
import ("os"; "testing"; "example.invalid/impact/internal/launch")
func TestMain(m *testing.M) { code := m.Run(); if launch.Setup() { os.Exit(7) }; os.Exit(code) }
`
			testSource := "func TestA(t *testing.T) { t.Parallel() }\n"
			if separate {
				impactWrite(t, root, "cmd/tool/tool_test.go", "package tool\nimport \"testing\"\n"+testSource)
			} else {
				mainSource += testSource
			}
			impactWrite(t, root, "cmd/tool/main_test.go", mainSource)
			testingFixtureGit(t, root, "add", ".")
			testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "setup fixture")
			base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
			impactWrite(t, root, "internal/launch/setup.go", "package launch\nfunc Setup() bool { return true }\n")
			selection := "selection: cmd/tool=TestA\n"
			if separate {
				selection = "selection: cmd/tool\n"
			}
			code, out, problem := impactPublic(t, root, "--base", base, "--plan")
			if code != 0 || !strings.Contains(out, selection) || strings.Contains(out, "TestMain") {
				t.Fatalf("TestMain dependency selection: exit=%d out=%s problem=%s", code, out, problem)
			}
			code, out, problem = impactPublic(t, root, "--base", base)
			if code != 1 || !strings.Contains(out, "landing group unit/cmd/tool red") || !strings.Contains(out, "LANDING-FAILED\tcmd/tool\t") {
				t.Fatalf("TestMain failure must be red: exit=%d out=%s problem=%s", code, out, problem)
			}
		})
	}
}

func TestTestImpactReplayIgnoresTestMain(t *testing.T) {
	t.Parallel()
	for _, selection := range []string{"TestMain", "TestMain,TestPresent"} {
		t.Run(selection, func(t *testing.T) {
			t.Parallel()
			root := impactAdapterBed(t)
			impactWrite(t, root, "cmd/metasystem/main_test.go", `package main
import ("os"; "testing")
func TestMain(m *testing.M) { os.WriteFile("main-ran", []byte("ran"), 0600); os.Exit(m.Run()) }
func TestPresent(t *testing.T) { t.Parallel(); os.WriteFile("test-ran", []byte("ran"), 0600) }
func TestNoise(t *testing.T) { t.Parallel(); t.Fatal("unselected test ran") }
`)
			command := exec.Command(commandTestExecutable(t), "test", "impact", "--root", root)
			command.Dir = root
			command.Env = append(slices.DeleteFunc(fixtureCommandEnvironment(t), func(e string) bool {
				return strings.HasPrefix(e, "LANDING_ONLY=") || strings.HasPrefix(e, "LANDING_PROOF_BASE=")
			}), "LANDING_ONLY=cmd/metasystem="+selection)
			data, err := command.CombinedOutput()
			if err != nil || !strings.Contains(string(data), "landing group unit/cmd/metasystem green") || !strings.HasSuffix(string(data), "LANDING-CHECKED\t0\n") {
				t.Fatalf("TestMain replay: error=%v output=%s", err, data)
			}
			if _, err := os.Stat(filepath.Join(root, "cmd/metasystem/main-ran")); err != nil {
				t.Fatalf("TestMain did not execute: %v", err)
			}
			_, err = os.Stat(filepath.Join(root, "cmd/metasystem/test-ran"))
			if selection == "TestMain" && !os.IsNotExist(err) || selection != "TestMain" && err != nil {
				t.Fatalf("selected test execution: selection=%s error=%v", selection, err)
			}
		})
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
	if code != 0 || !strings.Contains(stdout.String(), "selection: internal/launch\n") {
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
	if code != 0 || !strings.Contains(out, "selection: cmd/metasystem=TestCarry") || strings.Contains(out, "TestUnrelated") {
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
	if code != 0 || !strings.Contains(out, "selection: cmd/metasystem\n") {
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
	if code != 0 || !strings.Contains(out, "selection: cmd/metasystem=TestSignedInLaunch") || strings.Contains(out, "TestUnrelated") || strings.Contains(out, "selection: cmd/metasystem\n") {
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
			if code != 0 || !strings.Contains(out, "selection: internal/reader\n") {
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
		if code != 0 || !strings.Contains(out, "selection: "+pkg+"\n") {
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
	if code != 1 || !strings.Contains(out, "selection: cmd/metasystem=TestHelperCaller") || !strings.Contains(out, "landing group unit/cmd/metasystem red ") || !strings.Contains(problem, "TestHelperCaller") || strings.Contains(out, "TestUnrelated") || strings.Contains(out, "selection: cmd/metasystem\n") {
		t.Fatalf("helper caller exit=%d out=%s problem=%s", code, out, problem)
	}
}

func TestTestImpactNonCommitBaseRefusesGitAdapter(t *testing.T) {
	t.Parallel()
	root, _ := impactGitAdapterBed(t)
	for _, base := range []string{"HEAD^{tree}", "HEAD:go.mod", "missing-commit"} {
		code, out, problem := impactPublic(t, root, "--base", base, "--plan")
		if code != 2 || !strings.Contains(problem, "is not a commit in this repository") || !strings.Contains(problem, base) || !strings.Contains(problem, "rev-parse") || out != "" {
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
	if code != 1 || !strings.Contains(out, "selection: internal/reader\n") || !strings.Contains(out, "landing group unit/internal/reader red ") || !strings.Contains(problem, "TestInternalNoise") {
		t.Fatalf("internal importer exit=%d out=%s problem=%s", code, out, problem)
	}
}

func TestTestImpactNonAncestorBaseRefusesGitAdapter(t *testing.T) {
	t.Parallel()
	root, base := impactGitAdapterBed(t)
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-qm", "other")
	other := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	testingFixtureGit(t, root, "checkout", "--detach", base)
	code, out, problem := impactPublic(t, root, "--base", other, "--plan")
	if code != 2 || out != "" || !strings.Contains(problem, other+" is not an ancestor of HEAD; nothing was selected") || !strings.Contains(problem, "--base <the round's base>") {
		t.Fatalf("non-ancestor exit=%d out=%s problem=%s", code, out, problem)
	}
	code, out, problem = impactPublic(t, root, "--base", "HEAD", "--plan")
	if code != 0 || !strings.HasPrefix(out, "plan: base "+base+" (base)\n") {
		t.Fatalf("base plan exit=%d out=%s problem=%s", code, out, problem)
	}
}

func TestTestImpactLandingOnlyAndReports(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"replay", "replay-no-base", "red", "green", "build", "not-run"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			root, base := impactAdapterBed(t), "ignored"
			impactWrite(t, root, "internal/a/a_test.go", "package a\nimport \"testing\"\nfunc TestA(t *testing.T) { t.Parallel() }\n")
			failure := ""
			if mode == "red" {
				failure = "t.Fatal(\"red\")"
			}
			impactWrite(t, root, "cmd/tool/tool_test.go", "package tool\nimport \"testing\"\nfunc TestOne(t *testing.T) { t.Parallel(); "+failure+" }\n")
			if strings.HasPrefix(mode, "replay") {
				impactWrite(t, root, "cmd/tool/other_test.go", "package tool\nimport \"testing\"\nfunc TestOther(t *testing.T) { t.Parallel(); t.Fatal(\"must not run\") }\n")
				impactWrite(t, root, "internal/unselected/noise_test.go", "package unselected\nimport \"testing\"\nfunc TestNoise(t *testing.T) { t.Parallel(); t.Fatal(\"must not select\") }\n")
				impactWrite(t, root, "cmd/metasystem/selected_test.go", "package main\nimport \"testing\"\nfunc TestSelected(t *testing.T) { t.Parallel(); t.Fatal(\"test selected\") }\n")
			}
			if mode == "build" {
				impactWrite(t, root, "cmd/tool/broken.go", "package tool\nvar _ = missing\n")
			}
			if mode == "not-run" {
				impactWrite(t, root, "cmd/tool/broken.go", "package tool\nfunc (\n")
			}
			args := []string{"test", "impact", "--root", root, "--base", base}
			command := exec.Command(commandTestExecutable(t), args...)
			command.Dir = root
			command.Env = slices.DeleteFunc(fixtureCommandEnvironment(t), func(e string) bool {
				return strings.HasPrefix(e, "LANDING_ONLY=") || strings.HasPrefix(e, "LANDING_PROOF_BASE=")
			})
			if mode == "red" || mode == "green" || mode == "build" {
				command.Env = append(command.Env, "LANDING_ONLY=metasystem/internal/a metasystem/cmd/tool=TestOne")
			}
			if mode == "not-run" {
				command.Env = append(command.Env, "LANDING_ONLY=metasystem/missing")
			}
			if strings.HasPrefix(mode, "replay") {
				command.Args = append(command.Args, "--base", "missing", "--plan")
				command.Env = append(command.Env, "LANDING_ONLY=metasystem/internal/a metasystem/cmd/tool=TestOne metasystem/cmd/metasystem=TestSelected", "LANDING_PROOF_BASE=missing")
				if mode == "replay-no-base" {
					command.Args = command.Args[:5]
					command.Env = slices.DeleteFunc(command.Env, func(e string) bool { return strings.HasPrefix(e, "LANDING_PROOF_BASE=") })
				}
			}
			var out, problem bytes.Buffer
			command.Stdout, command.Stderr = &out, &problem
			err := command.Run()
			code := 0
			if err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatal(err)
				}
				code = exit.ExitCode()
			}
			text := out.String()
			if mode == "not-run" {
				if code != 1 || !strings.HasSuffix(text, "LANDING-NOT-RUN\tenvironment\n") {
					t.Fatalf("not run exit=%d out=%s problem=%s", code, text, problem.String())
				}
				return
			}
			if strings.HasPrefix(mode, "replay") {
				if code != 1 || strings.Contains(text, "plan:") || strings.Contains(text, "selection:") || strings.Contains(problem.String(), "TestOther") || strings.Contains(problem.String(), "TestNoise") || !strings.Contains(problem.String(), "TestSelected") || !strings.Contains(text, "replay selections ignore the comparison base from flags and environment") || !strings.Contains(text, "landing group unit/internal/a green") || !strings.Contains(text, "landing group unit/cmd/tool green") {
					t.Fatalf("replay exit=%d out=%s problem=%s", code, text, problem.String())
				}
				return
			}
			want := []plain.FailedUnit(nil)
			if mode == "red" {
				want = []plain.FailedUnit{{Unit: "metasystem/cmd/tool", Tests: []string{"TestOne"}}}
			}
			if mode == "build" {
				want = []plain.FailedUnit{{Unit: "metasystem/cmd/tool", Tests: []string{}}}
			}
			if !reflect.DeepEqual(plain.FailedChecks([]byte(text)), want) || !strings.HasSuffix(text, fmt.Sprintf("LANDING-CHECKED\t%d\n", len(want))) || strings.Count(text, "LANDING-FAILED\t") != len(want) || code != min(1, len(want)) {
				t.Fatalf("report exit=%d out=%s problem=%s want=%+v", code, text, problem.String(), want)
			}
		})
	}
}

// Git supplies the adapter's committed comparison base in the template layout.
func TestImpactRunsInTheModuleAndRunsGroupsByIDGitAdapter(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	helmMust(t, err)
	installation := filepath.Join(root, "metasystem")
	helmMust(t, os.Rename(impactAdapterBed(t), installation))
	impactWrite(t, installation, "metasystem.conf", "metasystem.template=true\ntesting.contract=testing.json\n")
	data, err := os.ReadFile(filepath.Join(installation, "testing.json"))
	helmMust(t, err)
	var contract testpolicy.Contract
	helmMust(t, json.Unmarshal(data, &contract))
	recorder := filepath.Join(t.TempDir(), "record-group")
	helmMust(t, testexec.WriteFile(recorder, []byte("#!/bin/sh\nprintf '%s\\n' \"$PWD\" > \"$1\"\n"), 0700))
	contract.Groups[0].ID = "fast-static-build"
	contract.Groups[0].Inputs = []string{"metasystem/internal/**"}
	contract.Groups[0].Argv = []string{recorder, filepath.Join(root, "group-ran")}
	contract.Always.Canary = []string{"fast-static-build"}
	contract.Groups[1].Inputs = []string{"outside/**"}
	contract.Surfaces[0].Paths = []string{"metasystem/internal/**", "metasystem/cmd/**"}
	data, err = json.Marshal(contract)
	helmMust(t, err)
	impactWrite(t, installation, "testing.json", string(data))
	impactWrite(t, installation, "internal/a/a_test.go", `package a
import ("testing"; "os")
func TestA(t *testing.T) { t.Parallel(); cwd, err := os.Getwd(); if err != nil { t.Fatal(err) }; if err := os.WriteFile("`+filepath.Join(root, "package-ran")+`", []byte(cwd), 0600); err != nil { t.Fatal(err) } }
`)
	testingFixtureGit(t, root, "init", "-q", "-b", "main")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "base")
	base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	impactWrite(t, installation, "internal/a/a.go", "package a\nconst Value = true\n")
	run := func(only *string) (int, string) {
		t.Helper()
		cmd := exec.Command(commandTestExecutable(t), "test", "impact", "--root", installation, "--base", base)
		cmd.Dir = installation
		cmd.Env = slices.DeleteFunc(fixtureCommandEnvironment(t), func(e string) bool {
			return strings.HasPrefix(e, "LANDING_ONLY=") || strings.HasPrefix(e, "LANDING_PROOF_BASE=")
		})
		if only != nil {
			cmd.Env = append(cmd.Env, "LANDING_ONLY="+*only)
		}
		output, err := cmd.CombinedOutput()
		code := 0
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatal(err)
			}
			code = exit.ExitCode()
		}
		return code, string(output)
	}
	for _, only := range []*string{nil, new("fast-static-build metasystem/internal/a"), new("fast-static-build internal/a")} {
		helmMust(t, os.RemoveAll(filepath.Join(root, "package-ran")))
		helmMust(t, os.RemoveAll(filepath.Join(root, "group-ran")))
		code, out := run(only)
		if code != 0 || !strings.Contains(out, "landing group unit/internal/a green") || !strings.Contains(out, "landing group fast-static-build green") {
			t.Fatalf("module proof exit=%d: %s", code, out)
		}
		data, err := os.ReadFile(filepath.Join(root, "package-ran"))
		if err != nil || string(data) != filepath.Join(installation, "internal/a") {
			t.Fatalf("package directory=%q, error=%v", data, err)
		}
		data, err = os.ReadFile(filepath.Join(root, "group-ran"))
		if err != nil || strings.TrimSpace(string(data)) != root {
			t.Fatalf("declared group directory=%q, error=%v", data, err)
		}
	}
	code, out := run(new("no-such-thing"))
	if code != 1 || !strings.Contains(out, "LANDING-NOT-RUN\tenvironment") {
		t.Fatalf("unknown entry exit=%d: %s", code, out)
	}
}

func TestTestImpactUsesInstallationPrefixWithoutBatchTagGitAdapter(t *testing.T) {
	t.Parallel()
	root, _ := impactGitAdapterBed(t)
	impactWrite(t, root, "cmd/metasystem/main.go", "package main\nfunc main() {}\nvar replayTagged bool\nfunc value() bool { return true }\n")
	impactWrite(t, root, "cmd/metasystem/main_test.go", "package main\nimport \"testing\"\nfunc TestValue(t *testing.T) { t.Parallel(); if !value() || replayTagged { t.Fatal(\"value or replay tag\") } }\n")
	impactWrite(t, root, "cmd/metasystem/tag_test.go", "//go:build batchtest\n\npackage main\nimport \"testing\"\nfunc init() { replayTagged = true }\nfunc TestTagged(t *testing.T) { t.Parallel(); t.Fatal(\"replay tag\") }\n")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "tagged test fixture")
	base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	impactWrite(t, root, "cmd/metasystem/main.go", "package main\nfunc main() {}\nvar replayTagged bool\nfunc value() bool { return 1 == 1 }\n")
	code, out, problem := impactPublic(t, root, "--base", base)
	if code != 0 || !strings.Contains(out, "selection: cmd/metasystem=") || !strings.Contains(out, "landing group unit/cmd/metasystem green") || !strings.HasSuffix(out, "LANDING-CHECKED\t0\n") {
		t.Fatalf("ordinary check must use installation paths without replay tags: code=%d out=%s problem=%s", code, out, problem)
	}
	// A normal red uses the same installation-relative unit in its report.
	impactWrite(t, root, "cmd/metasystem/main.go", "package main\nfunc main() {}\nvar replayTagged bool\nfunc value() bool { return false }\n")
	code, out, problem = impactPublic(t, root, "--base", base)
	if code != 1 || !reflect.DeepEqual(plain.FailedChecks([]byte(out)), []plain.FailedUnit{{Unit: "cmd/metasystem", Tests: []string{"TestValue"}}}) {
		t.Fatalf("ordinary red report: code=%d out=%s problem=%s", code, out, problem)
	}
	impactWrite(t, root, "cmd/metasystem/main.go", "package main\nfunc main() {}\nvar replayTagged bool\nfunc value() bool { return true }\n")
	command := exec.Command(commandTestExecutable(t), "test", "impact", "--root", root)
	command.Dir = root
	command.Env = append(slices.DeleteFunc(fixtureCommandEnvironment(t), func(e string) bool {
		return strings.HasPrefix(e, "LANDING_ONLY=") || strings.HasPrefix(e, "LANDING_PROOF_BASE=")
	}), "LANDING_ONLY=cmd/metasystem=TestValue")
	data, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(data), "landing group unit/cmd/metasystem green") || !strings.HasSuffix(string(data), "LANDING-CHECKED\t0\n") {
		t.Fatalf("replay must run without the batch tag: error=%v output=%s", err, data)
	}
}
