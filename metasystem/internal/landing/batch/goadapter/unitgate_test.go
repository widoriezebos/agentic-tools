package goadapter

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

func TestBatchJoinGateRedNamesFailingDependentTests(t *testing.T) {
	t.Parallel()
	selection := gateConsumerPackages()
	direct := GateStep{Name: "dependent package ./direct", Args: []string{"go", "test", "-trimpath", "-count=1", "-timeout", "40m", "./direct"}}
	output := "--- FAIL: TestDirectContract (0.00s)\nFAIL\texample.invalid/unitgate/direct\t0.01s\n"
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
	t                                                                *testing.T
	root, prefix, baseTree, candidateTree, topTree, candidateTopTree string
	base, candidate                                                  map[string]string
}

func newPackageTreeFixture(t *testing.T, root string, base, candidate map[string]string, prefix string) *packageTreeFixture {
	return &packageTreeFixture{t: t, root: root, prefix: prefix, baseTree: strings.Repeat("a", 40), candidateTree: strings.Repeat("b", 40), topTree: strings.Repeat("c", 40), candidateTopTree: strings.Repeat("d", 40), base: clonePackageFiles(base), candidate: candidate}
}

func (f *packageTreeFixture) openSnapshot(tree string) (string, func() error, error) {
	f.t.Helper()
	files, ok := f.treeFiles(tree)
	if !ok {
		f.t.Fatalf("unknown snapshot tree %q", tree)
	}
	root := f.t.TempDir()
	for path, content := range files {
		absolute := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			f.t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
			f.t.Fatal(err)
		}
	}
	return root, func() error { return nil }, nil
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
	case f.candidateTopTree:
		files := map[string]string{"outside.txt": "changed outside\n"}
		for path, content := range f.candidate {
			files[f.prefix+path] = content
		}
		return files, true
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
		f.t.Fatalf("unexpected raw Git binding: dir=%q args=%q environment matches=%t stdin present=%t", request.Dir, request.Args, reflect.DeepEqual(request.Env, gittree.ScrubbedEnviron()), request.Stdin != nil)
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
	case slices.Equal(args, []string{"rev-parse", f.topTree + "^{tree}"}):
		return gittree.RawResult{Stdout: []byte(f.topTree + "\n")}
	case slices.Equal(args, []string{"rev-parse", f.candidateTopTree + "^{tree}"}):
		return gittree.RawResult{Stdout: []byte(f.candidateTopTree + "\n")}
	case slices.Equal(args, []string{"rev-parse", f.topTree + ":" + strings.TrimSuffix(f.prefix, "/")}) && f.prefix != "":
		return gittree.RawResult{Stdout: []byte(f.baseTree + "\n")}
	case slices.Equal(args, []string{"rev-parse", f.candidateTopTree + ":" + strings.TrimSuffix(f.prefix, "/")}) && f.prefix != "":
		return gittree.RawResult{Stdout: []byte(f.candidateTree + "\n")}
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

// A deleted package selects its nearest existing parent directory, since the
// package itself no longer exists to test; a file created and deleted inside
// the change (absent from the base) selects nothing. This carries the
// guarantee of the retired changedGoPackages test into the production
// working selection.
func TestDeletedGoPackagesSelectNearestExistingDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "outer"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "outer", "keep.txt"), []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	packages, err := changedWorkingGoPackages(root, gateChanges{"outer/missing/inner/value.go": {Deleted: true}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(packages, []string{"./outer/..."}) {
		t.Fatalf("nested deletion packages=%v, want the nearest existing parent", packages)
	}
	packages, err = changedWorkingGoPackages(root, gateChanges{"gone/inner/value.go": {Deleted: true}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(packages, []string{"./..."}) {
		t.Fatalf("deletion with no existing parent packages=%v, want the module", packages)
	}
	packages, err = changedWorkingGoPackages(root, gateChanges{"fresh/inner/value.go": {Deleted: true, BaseAbsent: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 0 {
		t.Fatalf("create-delete path selected packages=%v", packages)
	}
}
