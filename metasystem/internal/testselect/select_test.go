package testselect

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSelectFullRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		root    string
		changed []string
		pkg     string
		mode    Mode
		reason  string
	}{
		{name: "module files", root: "rules", changed: []string{"go.mod"}, pkg: "./anchor", mode: Whole, reason: "module files changed"},
		{name: "owning package", root: "rules", changed: []string{"owner/nested/notes.txt"}, pkg: "./owner", mode: Whole, reason: "changed"},
		{name: "reverse test importer", root: "rules", changed: []string{"changed/changed.go"}, pkg: "./testimporter", mode: Whole, reason: "imports ./changed"},
		{name: "anchor reader", root: "rules", changed: []string{"changed/changed.go"}, pkg: "./anchor", mode: Whole, reason: "names a changed file: changed.go"},
		{name: "everything else", root: "rules", changed: []string{"README.md"}, pkg: "./bare", mode: Skip, reason: "not reached by changes"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			selection, err := Select(Request{ModuleRoot: fixtureRoot(t, test.root), Changed: test.changed})
			if err != nil {
				t.Fatal(err)
			}
			assertDecision(t, selection, test.pkg, test.mode, test.reason)
		})
	}
}

func TestSelectModuleFileChangesEveryPackageWithoutOtherEvaluation(t *testing.T) {
	t.Parallel()
	runner := &countingRunner{reject: true}
	selection, err := selectWithRunner(Request{
		ModuleRoot: fixtureRoot(t, "modulechange"),
		Changed:    []string{"go.sum", "a/a.go"},
	}, runner)
	if err != nil {
		t.Fatal(err)
	}
	if runner.calls != 0 {
		t.Fatalf("command calls = %d, want 0", runner.calls)
	}
	if len(selection.Packages) != 1 {
		t.Fatalf("packages = %#v, want one", selection.Packages)
	}
	assertDecision(t, selection, "./a", Whole, "module files changed")
}

func TestSelectTestFileChangeDoesNotReachImporters(t *testing.T) {
	t.Parallel()
	selection, err := Select(Request{
		ModuleRoot: fixtureRoot(t, "rules"),
		Changed:    []string{"changed/changed_test.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertDecision(t, selection, "./changed", Whole, "changed")
	assertDecision(t, selection, "./importer", Skip, "not reached by changes")
	assertDecision(t, selection, "./testimporter", Skip, "not reached by changes")
}

func TestSelectAnchorRequiresFileLineStringLiteral(t *testing.T) {
	t.Parallel()
	selection, err := Select(Request{
		ModuleRoot: fixtureRoot(t, "rules"),
		Changed:    []string{"changed/changed.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertDecision(t, selection, "./anchor", Whole, "names a changed file: changed.go")
	assertDecision(t, selection, "./bare", Skip, "not reached by changes")
	assertDecision(t, selection, "./comment", Skip, "not reached by changes")
}

func TestSelectOwnershipEdges(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		changed string
		owner   string
	}{
		{name: "directory without Go source", changed: "owner/nested/notes.txt", owner: "./owner"},
		{name: "skipped testdata directory", changed: "owner/testdata/input.txt", owner: "./owner"},
		{name: "module root without Go source", changed: "README.md"},
		{name: "no ancestor with Go source", changed: "orphan/files/note.txt"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			selection, err := Select(Request{
				ModuleRoot: fixtureRoot(t, "rules"),
				Changed:    []string{test.changed},
			})
			if err != nil {
				t.Fatal(err)
			}
			if test.owner != "" {
				assertDecision(t, selection, test.owner, Whole, "changed")
				return
			}
			for _, decision := range selection.Packages {
				if decision.Mode != Skip || decision.Reason != "not reached by changes" {
					t.Fatalf("decision for unowned change = %#v, want Skip", decision)
				}
			}
		})
	}
}

func TestSelectPackageDiscoverySkipsExcludedDirectories(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.test/discovery\n\ngo 1.27\n")
	writeTestFile(t, filepath.Join(root, "pkg", "pkg.go"), "package pkg\n")
	for _, path := range []string{
		".hidden/ignored.go",
		"testdata/ignored.go",
		"vendor/ignored/ignored.go",
		"artifacts/ignored.go",
		"bin/ignored.go",
	} {
		writeTestFile(t, filepath.Join(root, filepath.FromSlash(path)), "package ignored\n")
	}
	selection, err := Select(Request{
		ModuleRoot: root,
		Changed:    []string{"go.mod"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(selection.Packages) != 1 || selection.Packages[0].Package != "./pkg" {
		t.Fatalf("discovered packages = %#v, want only ./pkg", selection.Packages)
	}
}

func TestSelectRunsGoListAtMostOnce(t *testing.T) {
	t.Parallel()
	runner := &countingRunner{output: strings.Join([]string{
		"example.test/rules/changed|",
		"example.test/rules/importer|example.test/rules/changed",
		"example.test/rules/testimporter_test|example.test/rules/changed",
	}, "\n")}
	selection, err := selectWithRunner(Request{
		ModuleRoot: fixtureRoot(t, "rules"),
		Changed:    []string{"changed/changed.go", "owner/owner.go"},
	}, runner)
	if err != nil {
		t.Fatal(err)
	}
	if runner.goListCalls != 1 {
		t.Fatalf("go list calls = %d, want 1", runner.goListCalls)
	}
	assertDecision(t, selection, "./importer", Whole, "imports ./changed")
	assertDecision(t, selection, "./testimporter", Whole, "imports ./changed")
}

func TestGoListImportVariantsNormalizeToPackage(t *testing.T) {
	t.Parallel()
	for _, importPath := range []string{
		"example.test/rules/changed.test",
		"example.test/rules/changed_test",
		"example.test/rules/changed [example.test/rules/changed.test]",
	} {
		if got := normalizeListedImport(importPath); got != "example.test/rules/changed" {
			t.Errorf("normalizeListedImport(%q) = %q", importPath, got)
		}
	}
}

func TestSelectPreviousResultsAreNotBuiltYet(t *testing.T) {
	t.Parallel()
	_, err := Select(Request{Previous: &Results{}})
	if err == nil || !strings.Contains(err.Error(), "U1b is not built yet") {
		t.Fatalf("error = %v, want U1b not built yet", err)
	}
}

func TestSelectIsDeterministic(t *testing.T) {
	t.Parallel()
	request := Request{
		ModuleRoot: fixtureRoot(t, "rules"),
		Changed:    []string{"owner/owner.go", "changed/changed.go", "changed/changed.go"},
	}
	first, err := Select(request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Select(request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("selections differ:\nfirst:  %#v\nsecond: %#v", first, second)
	}
	for index := 1; index < len(first.Packages); index++ {
		if first.Packages[index-1].Package >= first.Packages[index].Package {
			t.Fatalf("packages are not sorted: %#v", first.Packages)
		}
	}
	assertDecision(t, first, "./importer", Whole, "imports ./changed, ./owner")
	assertDecision(t, first, "./anchor", Whole, "names a changed file: changed.go, owner.go")
}

func TestSelectGitChangedFilesMatchesExplicitList(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	moduleRoot := filepath.Join(repository, "nested", "module")
	writeTestFile(t, filepath.Join(moduleRoot, "go.mod"), "module example.test/gitfixture\n\ngo 1.27\n")
	writeTestFile(t, filepath.Join(moduleRoot, "a", "a.go"), "package a\n\nconst Value = 2\n")
	writeTestFile(t, filepath.Join(moduleRoot, "b", "b.go"), "package b\n\nimport \"example.test/gitfixture/a\"\n\nconst Value = a.Value\n")
	base, head := strings.Repeat("1", 40), strings.Repeat("2", 40)
	goListArgs := []string{"list", "-e", "-test", "-f", `{{.ImportPath}}|{{join .Deps ","}}`, "./..."}
	goListOutput := []byte("example.test/gitfixture/a|\nexample.test/gitfixture/b|example.test/gitfixture/a\n")
	gitRunner := &strictSelectRunner{t: t, steps: []selectCommand{
		{moduleRoot, "git", []string{"rev-parse", "--show-prefix"}, []byte("nested/module/\n"), nil},
		{moduleRoot, "git", []string{"diff", "--name-only", "--no-renames", base, head, "--"}, []byte("records/note.md\nnested/module/a/a.go\n"), nil},
		{moduleRoot, "go", goListArgs, goListOutput, nil},
	}}
	t.Cleanup(gitRunner.checkConsumed)
	fromGit, err := selectWithRunner(Request{ModuleRoot: moduleRoot, Base: base, Head: head}, gitRunner)
	if err != nil {
		t.Fatal(err)
	}
	explicitRunner := &strictSelectRunner{t: t, steps: []selectCommand{
		{moduleRoot, "go", goListArgs, goListOutput, nil},
	}}
	t.Cleanup(explicitRunner.checkConsumed)
	explicit, err := selectWithRunner(Request{ModuleRoot: moduleRoot, Changed: []string{"a/a.go"}}, explicitRunner)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fromGit, explicit) {
		t.Fatalf("git selection differs from explicit list:\ngit:      %#v\nexplicit: %#v", fromGit, explicit)
	}
	assertDecision(t, fromGit, "./b", Whole, "imports ./a")
}

type selectCommand struct {
	dir, name string
	args      []string
	output    []byte
	err       error
}

type strictSelectRunner struct {
	t     *testing.T
	steps []selectCommand
	next  int
}

func (runner *strictSelectRunner) Run(dir, name string, args ...string) ([]byte, error) {
	runner.t.Helper()
	if runner.next >= len(runner.steps) {
		runner.t.Fatalf("unexpected command: %s %s in %s", name, strings.Join(args, " "), dir)
	}
	want := runner.steps[runner.next]
	if dir != want.dir || name != want.name || !reflect.DeepEqual(args, want.args) {
		runner.t.Fatalf("command %d = (%q, %q, %q), want (%q, %q, %q)", runner.next, dir, name, args, want.dir, want.name, want.args)
	}
	runner.next++
	return want.output, want.err
}

func (runner *strictSelectRunner) checkConsumed() {
	runner.t.Helper()
	if runner.next != len(runner.steps) {
		runner.t.Errorf("used %d of %d declared commands", runner.next, len(runner.steps))
	}
}

func TestNonTestImportsAreStandardLibrary(t *testing.T) {
	t.Parallel()
	command := exec.Command("go", "list", "-deps", "-f", `{{if not .Standard}}{{.ImportPath}}{{end}}`, ".")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("go list non-standard dependencies: %v: %s", err, output)
	}
	var nonStandard []string
	for _, line := range strings.Split(string(output), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			nonStandard = append(nonStandard, line)
		}
	}
	want := []string{"github.com/widoriezebos/agentic-tools/metasystem/internal/testselect"}
	if !reflect.DeepEqual(nonStandard, want) {
		t.Fatalf("non-standard non-test imports = %v, want only this package", nonStandard)
	}
}

type countingRunner struct {
	output      string
	reject      bool
	calls       int
	goListCalls int
}

func (runner *countingRunner) Run(_ string, name string, args ...string) ([]byte, error) {
	runner.calls++
	if runner.reject {
		return nil, fmt.Errorf("unexpected command: %s %s", name, strings.Join(args, " "))
	}
	if name != "go" || len(args) == 0 || args[0] != "list" {
		return nil, fmt.Errorf("unexpected command: %s %s", name, strings.Join(args, " "))
	}
	runner.goListCalls++
	return []byte(runner.output), nil
}

func fixtureRoot(t *testing.T, name string) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func assertDecision(t *testing.T, selection Selection, pkg string, mode Mode, reason string) {
	t.Helper()
	for _, decision := range selection.Packages {
		if decision.Package != pkg {
			continue
		}
		if decision.Mode != mode || decision.Reason != reason || decision.Tag != "plain" {
			t.Fatalf("decision for %s = %#v, want mode %d, tag plain, reason %q", pkg, decision, mode, reason)
		}
		if len(decision.Tests) != 0 {
			t.Fatalf("tests for %s = %v, want none", pkg, decision.Tests)
		}
		return
	}
	t.Fatalf("no decision for %s in %#v", pkg, selection.Packages)
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
