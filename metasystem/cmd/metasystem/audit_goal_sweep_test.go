package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// auditGoalSweepInsideItsSection names the functions allowed to call the
// goal-branch sweep directly: the steward's retry, which the disk pass
// already runs inside the worktree's critical section.
var auditGoalSweepInsideItsSection = map[string]bool{"goalSweepRun": true}

// Round D3 N1: every production goal-branch sweep (goal done's, the
// hand-landing's last-landing sweep, the batch lane's P6 sweep) runs inside
// the critical section of the goal's registered worktrees, as the function
// literal handed to steward.SweepGoalWorktrees.
func TestAuditEveryGoalBranchSweepRunsInsideTheWorktreeSection(t *testing.T) {
	t.Parallel()
	var sites []ratchetSite
	_, module := verbRatchetRoots(t)
	walkRatchetFiles(t, module, []string{".git", "node_modules", "artifacts", "testdata", "bin"}, nil, func(path, rel string) {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || strings.HasPrefix(rel, "internal/goal/branch/") {
			return
		}
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		for _, line := range auditUnsectionedGoalSweeps(fileSet, parsed) {
			sites = append(sites, ratchetSite{path: rel, line: line})
		}
	})
	for _, site := range sites {
		t.Errorf("%s:%d sweeps a goal branch outside steward.SweepGoalWorktrees", site.path, site.line)
	}
}

// auditUnsectionedGoalSweeps returns the lines of goal-branch sweep calls
// (branch.Sweep, goalbranch.Sweep, BatchGoalBranchSweep) that no enclosing
// SweepGoalWorktrees call carries and no allowed function holds.
func auditUnsectionedGoalSweeps(fileSet *token.FileSet, file *ast.File) []int {
	var lines []int
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil || auditGoalSweepInsideItsSection[function.Name.Name] {
			continue
		}
		var stack []ast.Node
		ast.Inspect(function.Body, func(node ast.Node) bool {
			if node == nil {
				stack = stack[:len(stack)-1]
				return false
			}
			stack = append(stack, node)
			call, ok := node.(*ast.CallExpr)
			if !ok || !auditIsGoalSweep(call.Fun) {
				return true
			}
			for _, outer := range stack[:len(stack)-1] {
				if enclosing, ok := outer.(*ast.CallExpr); ok && auditCallName(enclosing.Fun) == "SweepGoalWorktrees" {
					return true
				}
			}
			lines = append(lines, fileSet.Position(call.Pos()).Line)
			return true
		})
	}
	return lines
}

func auditCallName(fun ast.Expr) string {
	switch callee := fun.(type) {
	case *ast.SelectorExpr:
		return callee.Sel.Name
	case *ast.Ident:
		return callee.Name
	}
	return ""
}

func auditIsGoalSweep(fun ast.Expr) bool {
	switch callee := fun.(type) {
	case *ast.SelectorExpr:
		if ident, ok := callee.X.(*ast.Ident); ok && (ident.Name == "branch" || ident.Name == "goalbranch") && callee.Sel.Name == "Sweep" {
			return true
		}
	case *ast.Ident:
		return callee.Name == "BatchGoalBranchSweep"
	}
	return false
}
