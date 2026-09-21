package proofrun

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// ProtectedGoTestMissing identifies a named base test that a proposed tree
// cannot execute. The caller decides how to present this testing refusal.
type ProtectedGoTestMissing struct {
	Group, Package, Test, Tree string
}

func (missing *ProtectedGoTestMissing) Error() string {
	return fmt.Sprintf("group %s package %s test %s is absent from the %s tree",
		missing.Group, missing.Package, missing.Test, missing.Tree)
}

// CheckProtectedGoTests checks source presence before a batch member changes
// ownership. Package paths are relative to each Go group's declared cwd,
// which may be outside the installation holding testing.json.
func CheckProtectedGoTests(workspace gittree.Workspace, baseTree, candidateTree string, base testpolicy.Contract) error {
	type packageTests struct {
		names map[string]bool
		err   error
	}
	cache := map[string]packageTests{}
	read := func(tree string, group testpolicy.Group, pkg string) (map[string]bool, error) {
		directory := path.Join(group.CWD, strings.TrimPrefix(pkg, "./"))
		key := tree + "\x00" + directory
		if result, ok := cache[key]; ok {
			return result.names, result.err
		}
		result := packageTests{names: map[string]bool{}}
		entries, err := workspace.Entries(tree, []string{directory})
		if err != nil {
			result.err = err
			cache[key] = result
			return nil, err
		}
		for filePath := range entries {
			if path.Dir(filePath) != directory || !strings.HasSuffix(filePath, "_test.go") {
				continue
			}
			source, present, readErr := workspace.FileAt(tree, filePath)
			if readErr != nil || !present {
				result.err = readErr
				if readErr == nil {
					result.err = fmt.Errorf("test file %s disappeared from tree %s", filePath, tree)
				}
				break
			}
			file, parseErr := parser.ParseFile(token.NewFileSet(), filePath, source, 0)
			if parseErr != nil {
				result.err = fmt.Errorf("parse candidate test file %s: %w", filePath, parseErr)
				break
			}
			for _, declaration := range file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if ok && function.Recv == nil && strings.HasPrefix(function.Name.Name, "Test") {
					result.names[function.Name.Name] = true
				}
			}
		}
		cache[key] = result
		return result.names, result.err
	}
	for _, group := range base.Groups {
		if group.Adapter != "go" || len(group.Tests) == 0 {
			continue
		}
		all, names, err := testpolicy.GoTests(group)
		if err != nil {
			return fmt.Errorf("read base testing group %s: %w", group.ID, err)
		}
		if all {
			continue
		}
		for _, name := range names {
			var definingPackages []string
			for _, pkg := range group.Packages {
				baseNames, readErr := read(baseTree, group, pkg)
				if readErr != nil {
					return readErr
				}
				if baseNames[name] {
					definingPackages = append(definingPackages, pkg)
				}
			}
			if len(definingPackages) == 0 {
				return &ProtectedGoTestMissing{Group: group.ID, Package: strings.Join(group.Packages, ","), Test: name, Tree: "base"}
			}
			for _, pkg := range definingPackages {
				candidateNames, readErr := read(candidateTree, group, pkg)
				if readErr != nil {
					return readErr
				}
				if !candidateNames[name] {
					return &ProtectedGoTestMissing{Group: group.ID, Package: pkg, Test: name, Tree: "candidate"}
				}
			}
		}
	}
	return nil
}
