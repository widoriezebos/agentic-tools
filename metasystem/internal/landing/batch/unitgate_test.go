package batch

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

func TestReverseDependentsIncludeDirectTransitiveTestAndTaggedImports(t *testing.T) {
	t.Parallel()
	root, files := dependencyModuleTree(t)
	candidate := clonePackageFiles(files)
	candidate["base/base.go"] = "package base\nconst Changed = true\n"
	fixture := newPackageTreeFixture(t, root, files, candidate, "")
	tree := fixture.baseTree

	got, err := reverseDependentsWithSnapshot(fixture.workspace(), tree, []string{"./base"}, fixture.openSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"./cmd/metasystem", "./direct", "./tagged", "./testonly", "./transitive"}
	if !slices.Equal(got, want) {
		t.Fatalf("reverse dependents = %v, want %v", got, want)
	}
	if err := os.WriteFile(filepath.Join(root, "base", "base.go"), []byte("package base\nconst Changed = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	candidateTree := fixture.candidateTree
	selection, err := selectUnitPackagesWithSnapshot(fixture.workspace(), tree, candidateTree, fixture.openSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	if selection.Tree != candidateTree || !slices.Equal(selection.Changed, []string{"./base"}) ||
		!slices.Equal(selection.Dependents, []string{"./cmd/metasystem", "./direct", "./tagged", "./testonly", "./transitive"}) {
		t.Fatalf("exact-tree package closure=%+v", selection)
	}
}

func TestReverseDependentsDefaultUsesDetachedTree(t *testing.T) {
	root, _ := dependencyModuleTree(t)
	bedGit(t, root, "init", "-q", "-b", "main")
	bedGit(t, root, "config", "user.name", "Fixture")
	bedGit(t, root, "config", "user.email", "fixture@example.invalid")
	bedGit(t, root, "add", ".")
	bedGit(t, root, "commit", "-qm", "fixture")
	tree := bedGit(t, root, "rev-parse", "HEAD^{tree}")
	got, err := ReverseDependents(root, tree, []string{"./base"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"./cmd/metasystem", "./direct", "./tagged", "./testonly", "./transitive"}
	if !slices.Equal(got, want) {
		t.Fatalf("native reverse dependents = %v, want %v", got, want)
	}
	if err := os.WriteFile(filepath.Join(root, "base", "base.go"), []byte("package base\nconst Changed = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bedGit(t, root, "add", "base/base.go")
	candidateTree := bedGit(t, root, "write-tree")
	selection, err := SelectUnitPackages(root, tree, candidateTree)
	if err != nil {
		t.Fatal(err)
	}
	if selection.Tree != candidateTree || !slices.Equal(selection.Changed, []string{"./base"}) || !slices.Equal(selection.Dependents, want) {
		t.Fatalf("native exact-tree selection=%+v", selection)
	}
}

func TestWorkingUnitSelectionIncludesTrackedAndUntrackedPackages(t *testing.T) {
	t.Parallel()
	root, files := dependencyModuleTree(t)
	fixture := newPackageTreeFixture(t, root, files, nil, "")
	if err := os.WriteFile(filepath.Join(root, "base", "base.go"), []byte("package base\nconst Changed = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(root, "fresh", "fresh.go")
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("package fresh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ignoredPath := filepath.Join(root, "ignored", "ignored.go")
	if err := os.MkdirAll(filepath.Dir(ignoredPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ignoredPath, []byte("package ignored\nimport _ \"example.invalid/unitgate/base\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	selection, err := selectWorkingUnitPackagesWithWorkspace(fixture.workspace(), "HEAD", packageWorkingGit(t, root, []string{"base/base.go\x00", "fresh/fresh.go\x00", "base/base.go\x00cmd/metasystem/main.go\x00direct/direct.go\x00fresh/fresh.go\x00none/none.go\x00tagged/tagged.go\x00testonly/value.go\x00testonly/value_test.go\x00transitive/value.go\x00"}))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(selection.Changed, []string{"./base", "./fresh"}) {
		t.Fatalf("working changed packages = %v", selection.Changed)
	}
	wantDependents := []string{"./cmd/metasystem", "./direct", "./tagged", "./testonly", "./transitive"}
	if !slices.Equal(selection.Dependents, wantDependents) {
		t.Fatalf("working dependents = %v, want %v", selection.Dependents, wantDependents)
	}
}

func TestWorkingUnitSelectionAcceptsNestedModuleTreeBase(t *testing.T) {
	t.Parallel()
	top := t.TempDir()
	moduleRoot := filepath.Join(top, "metasystem")
	for path, content := range map[string]string{
		"outside.txt":                    "outside\n",
		"metasystem/go.mod":              "module example.invalid/nested\n\ngo 1.27\n",
		"metasystem/base/base.go":        "package base\n",
		"metasystem/direct/direct.go":    "package direct\nimport _ \"example.invalid/nested/base\"\n",
		"metasystem/unrelated/value.go":  "package unrelated\n",
		"metasystem/unrelated/second.go": "package unrelated\n",
	} {
		absolute := filepath.Join(top, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	baseFiles := map[string]string{
		"go.mod":              "module example.invalid/nested\n\ngo 1.27\n",
		"base/base.go":        "package base\n",
		"direct/direct.go":    "package direct\nimport _ \"example.invalid/nested/base\"\n",
		"unrelated/value.go":  "package unrelated\n",
		"unrelated/second.go": "package unrelated\n",
	}
	fixture := newPackageTreeFixture(t, moduleRoot, baseFiles, nil, "metasystem/")
	baseTree := fixture.baseTree
	if err := os.WriteFile(filepath.Join(moduleRoot, "base", "base.go"), []byte("package base\nconst Changed = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	selection, err := selectWorkingUnitPackagesWithWorkspace(fixture.workspace(), baseTree, packageWorkingGit(t, moduleRoot, []string{"base/base.go\x00", "", "base/base.go\x00direct/direct.go\x00unrelated/second.go\x00unrelated/value.go\x00"}))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(selection.Changed, []string{"./base"}) || !slices.Equal(selection.Dependents, []string{"./direct"}) {
		t.Fatalf("nested module selection changed=%v dependents=%v", selection.Changed, selection.Dependents)
	}
}

func TestUnitGatePackageStepsChangedThenSortedDependentsAndBatchTests(t *testing.T) {
	t.Parallel()
	selection := gateConsumerPackages()
	steps := JoinGatePackageSteps(selection)
	want := [][]string{
		{"go", "test", "-count=1", "-timeout", "900s", "./base"},
		{"go", "test", "-count=1", "-timeout", "40m", "./cmd/metasystem"},
		{"go", "test", "-count=1", "-timeout", "40m", "./direct"},
		{"go", "test", "-count=1", "-timeout", "40m", "./tagged"},
		{"go", "test", "-count=1", "-timeout", "40m", "./testonly"},
		{"go", "test", "-count=1", "-timeout", "40m", "./transitive"},
		{"go", "test", "-count=1", "-timeout", "40m", "-tags", "batchtest", "./cmd/metasystem"},
	}
	if len(steps) != len(want) {
		t.Fatalf("unit gate steps=%v want %d", steps, len(want))
	}
	for index := range want {
		if !slices.Equal(steps[index].Args, want[index]) {
			t.Errorf("step %d=%v want %v", index, steps[index].Args, want[index])
		}
	}
}

func TestUnitGateFailureDetailNamesFailingDependentTests(t *testing.T) {
	t.Parallel()
	output := "--- FAIL: TestDirectContract (0.00s)\nFAIL\n"
	if got := GateFailureDetail(output); !strings.Contains(got, "TestDirectContract") {
		t.Fatalf("dependent test detail=%q", got)
	}
}

func TestBatchJoinGateStepsChangedThenSortedDependentsAndBatchTests(t *testing.T) {
	t.Parallel()
	selection := gateConsumerPackages()
	steps := JoinGatePackageSteps(selection)
	want := []string{"package ./base", "dependent package ./cmd/metasystem", "dependent package ./direct",
		"dependent package ./tagged", "dependent package ./testonly", "dependent package ./transitive", "package ./cmd/metasystem batchtest"}
	if len(steps) != len(want) {
		t.Fatalf("package steps=%v want names=%v", steps, want)
	}
	for i, name := range want {
		if steps[i].Name != name {
			t.Errorf("step %d=%q want %q", i, steps[i].Name, name)
		}
	}
}

func TestBatchJoinGateRedNamesFailingDependentTests(t *testing.T) {
	t.Parallel()
	selection := gateConsumerPackages()
	var direct GateStep
	for _, step := range JoinGatePackageSteps(selection) {
		if step.Name == "dependent package ./direct" {
			direct = step
			break
		}
	}
	if direct.Name == "" {
		t.Fatalf("changed base omitted direct dependent: %+v", selection)
	}
	output := "--- FAIL: TestDirectContract (0.00s)\nFAIL\texample.invalid/unitgate/direct\t0.01s\n"
	if detail := GateFailureDetail(output); !strings.Contains(detail, "TestDirectContract") {
		t.Fatalf("dependent test name was lost: %q", detail)
	}
	reds := GateReds(direct, selection.ModulePath, output)
	if !slices.Equal(reds, []GateRed{{Package: "./direct", Test: "TestDirectContract"}}) {
		t.Fatalf("dependent refusal attribution=%v", reds)
	}
}

func gateConsumerPackages() UnitPackages {
	return UnitPackages{
		Tree:       "unit-gate-tree",
		ModulePath: "example.invalid/unitgate",
		Changed:    []string{"./base"},
		Dependents: []string{"./cmd/metasystem", "./direct", "./tagged", "./testonly", "./transitive"},
	}
}

func dependencyModuleTree(t *testing.T) (string, map[string]string) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                 "module example.invalid/unitgate\n\ngo 1.27\n",
		".gitignore":             "ignored/\n",
		"base/base.go":           "package base\n",
		"direct/direct.go":       "package direct\nimport _ \"example.invalid/unitgate/base\"\n",
		"transitive/value.go":    "package transitive\nimport _ \"example.invalid/unitgate/direct\"\n",
		"testonly/value.go":      "package testonly\n",
		"testonly/value_test.go": "package testonly\nimport _ \"example.invalid/unitgate/base\"\n",
		"tagged/tagged.go":       "//go:build unitgate_never\n\npackage tagged\nimport _ \"example.invalid/unitgate/base\"\n",
		"none/none.go":           "package none\n",
		"cmd/metasystem/main.go": "package main\nimport _ \"example.invalid/unitgate/transitive\"\nfunc main() {}\n",
	}
	for path, content := range files {
		absolute := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, files
}

func clonePackageFiles(files map[string]string) map[string]string {
	copy := make(map[string]string, len(files))
	for path, content := range files {
		copy[path] = content
	}
	return copy
}

type packageTreeFixture struct {
	t                                              *testing.T
	root, prefix, baseTree, candidateTree, topTree string
	base, candidate                                map[string]string
}

func newPackageTreeFixture(t *testing.T, root string, base, candidate map[string]string, prefix string) *packageTreeFixture {
	return &packageTreeFixture{t: t, root: root, prefix: prefix, baseTree: strings.Repeat("a", 40), candidateTree: strings.Repeat("b", 40), topTree: strings.Repeat("c", 40), base: clonePackageFiles(base), candidate: candidate}
}

func (f *packageTreeFixture) workspace() gittree.Workspace {
	return gittree.Workspace{Dir: f.root, RawSource: f.raw}
}

func (f *packageTreeFixture) treeFiles(tree string) (map[string]string, bool) {
	switch tree {
	case f.baseTree:
		return f.base, true
	case f.candidateTree:
		if f.candidate != nil {
			return f.candidate, true
		}
	case "HEAD", f.topTree:
		if f.prefix == "" {
			return f.base, true
		}
		files := map[string]string{"outside.txt": "outside\n"}
		for path, content := range f.base {
			files[f.prefix+path] = content
		}
		return files, true
	}
	return nil, false
}

func packageBlobID(content string) string {
	h := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(content), content)))
	return hex.EncodeToString(h[:])
}

func (f *packageTreeFixture) raw(request gittree.RawRequest) gittree.RawResult {
	f.t.Helper()
	pins := []string{"-c", "core.fileMode=true", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false", "-c", "apply.ignoreWhitespace=no", "-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}
	bound := append([]string{"-C", f.root}, pins...)
	if request.Dir != f.root || len(request.Args) < len(bound) || !slices.Equal(request.Args[:len(bound)], bound) || !reflect.DeepEqual(request.Env, gittree.ScrubbedEnviron()) || request.Stdin != nil {
		f.t.Fatalf("unexpected raw Git binding: dir=%q args=%q env=%q stdin=%q", request.Dir, request.Args, request.Env, request.Stdin)
	}
	args := request.Args[len(bound):]
	if request.Operation != "git "+strings.Join(args, " ") {
		f.t.Fatalf("raw Git operation=%q args=%q", request.Operation, args)
	}
	switch {
	case slices.Equal(args, []string{"rev-parse", "--show-prefix"}):
		return gittree.RawResult{Stdout: []byte(f.prefix + "\n")}
	case slices.Equal(args, []string{"rev-parse", "HEAD^{tree}"}):
		if f.prefix != "" {
			return gittree.RawResult{Stdout: []byte(f.topTree + "\n")}
		}
		return gittree.RawResult{Stdout: []byte(f.baseTree + "\n")}
	case slices.Equal(args, []string{"rev-parse", f.topTree + ":" + strings.TrimSuffix(f.prefix, "/")}) && f.prefix != "":
		return gittree.RawResult{Stdout: []byte(f.baseTree + "\n")}
	case len(args) >= 7 && slices.Equal(args[:5], []string{"diff", "--name-only", "-z", "--no-renames", "--no-ext-diff"}):
		want := []string{"diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", "--ignore-submodules=none"}
		if len(args) != 10 || !slices.Equal(args[:7], want) || args[9] != "--" {
			f.t.Fatalf("unexpected raw diff argv=%q", args)
		}
		before, okBefore := f.treeFiles(args[7])
		after, okAfter := f.treeFiles(args[8])
		if !okBefore || !okAfter {
			f.t.Fatalf("unknown raw diff trees=%q", args[7:9])
		}
		var paths []string
		for path, content := range before {
			if next, present := after[path]; !present || next != content {
				paths = append(paths, path)
			}
		}
		for path := range after {
			if _, present := before[path]; !present {
				paths = append(paths, path)
			}
		}
		sort.Strings(paths)
		if len(paths) == 0 {
			return gittree.RawResult{}
		}
		return gittree.RawResult{Stdout: []byte(strings.Join(paths, "\x00") + "\x00")}
	case len(args) >= 7 && slices.Equal(args[:5], []string{"--literal-pathspecs", "ls-tree", "-r", "-z", "--full-tree"}):
		if args[6] != "--" || len(args) == 7 {
			f.t.Fatalf("unexpected raw ls-tree argv=%q", args)
		}
		files, ok := f.treeFiles(args[5])
		if !ok {
			f.t.Fatalf("unknown raw ls-tree tree=%q", args[5])
		}
		var paths []string
		for path := range files {
			for _, requested := range args[7:] {
				if path == requested || strings.HasPrefix(path, strings.TrimSuffix(requested, "/")+"/") {
					paths = append(paths, path)
					break
				}
			}
		}
		sort.Strings(paths)
		var out strings.Builder
		for _, path := range paths {
			fmt.Fprintf(&out, "100644 blob %s\t%s\x00", packageBlobID(files[path]), path)
		}
		return gittree.RawResult{Stdout: []byte(out.String())}
	case len(args) == 3 && args[0] == "cat-file" && args[1] == "blob":
		for _, tree := range []string{f.baseTree, f.candidateTree} {
			files, ok := f.treeFiles(tree)
			if !ok {
				continue
			}
			for _, content := range files {
				if packageBlobID(content) == args[2] {
					return gittree.RawResult{Stdout: []byte(content)}
				}
			}
		}
	}
	f.t.Fatalf("unexpected raw Git argv=%q", args)
	return gittree.RawResult{}
}

func (f *packageTreeFixture) openSnapshot(tree string) (string, func() error, error) {
	files, ok := f.treeFiles(tree)
	if !ok || tree == "HEAD" || tree == f.topTree {
		return "", nil, fmt.Errorf("unknown module snapshot %q", tree)
	}
	dir, err := os.MkdirTemp("", "package-tree-snapshot.")
	if err != nil {
		return "", nil, err
	}
	for path, content := range files {
		absolute := filepath.Join(dir, filepath.FromSlash(path))
		if err = os.MkdirAll(filepath.Dir(absolute), 0o755); err == nil {
			err = os.WriteFile(absolute, []byte(content), 0o644)
		}
		if err != nil {
			_ = os.RemoveAll(dir)
			return "", nil, err
		}
	}
	return dir, func() error { return os.RemoveAll(dir) }, nil
}

func packageWorkingGit(t *testing.T, root string, streams []string) func(*exec.Cmd) error {
	t.Helper()
	commands := [][]string{
		{"git", "-C", root, "diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", "--ignore-submodules=none", "--relative", "HEAD", "--"},
		{"git", "-C", root, "ls-files", "--others", "--exclude-standard", "-z", "--"},
		{"git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", "*.go"},
	}
	index := 0
	t.Cleanup(func() {
		if index != len(commands) {
			t.Errorf("working Git commands consumed %d of %d", index, len(commands))
		}
	})
	return func(command *exec.Cmd) error {
		t.Helper()
		path, lookupErr := exec.LookPath("git")
		if lookupErr != nil {
			path = "git"
		}
		if index >= len(commands) || command.Path != path || !slices.Equal(command.Args, commands[index]) || command.Dir != "" || !reflect.DeepEqual(command.Env, gittree.ScrubbedEnviron()) || command.Stdout == nil || command.Stdout != command.Stderr {
			t.Fatalf("unexpected working Git command: path=%q args=%q dir=%q env=%q", command.Path, command.Args, command.Dir, command.Env)
		}
		_, err := command.Stdout.Write([]byte(streams[index]))
		index++
		return err
	}
}
