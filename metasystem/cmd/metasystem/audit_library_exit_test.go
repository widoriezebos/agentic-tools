package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// A library package that ends the process from a goroutine or a select
// branch races its own bounded work: the supervisor's signal guard ran
// os.Exit(143) when a 3 s timer beat a client whose own waits are bounded at
// 5 s, so a loaded host ended the supervisor mid-settle, and in-process tests
// lost the whole test binary with no output (no-flaky-tests, 2026-10-02).
// Only package main owns the process; a library ends through its caller's
// flow, or through an exit seam its caller hands it.

var auditLibraryExitCalls = map[string]bool{"os.Exit": true, "syscall.Exit": true, "unix.Exit": true}

// auditLibraryExitLines returns the lines of process exits inside a go
// statement or a select branch of a non-main package.
func auditLibraryExitLines(fileSet *token.FileSet, file *ast.File) []int {
	if file.Name.Name == "main" {
		return nil
	}
	var lines []int
	seen := map[token.Pos]bool{}
	collect := func(root ast.Node) {
		ast.Inspect(root, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if ok && auditLibraryExitCalls[ratchetQualifiedName(call.Fun)] && !seen[call.Pos()] {
				seen[call.Pos()] = true
				lines = append(lines, fileSet.Position(call.Pos()).Line)
			}
			return true
		})
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.GoStmt:
			collect(node.Call)
		case *ast.CommClause:
			for _, statement := range node.Body {
				collect(statement)
			}
		}
		return true
	})
	return lines
}

func TestAuditNoLibraryExitsFromAGoroutineOrSelect(t *testing.T) {
	t.Parallel()
	_, module := verbRatchetRoots(t)
	walkRatchetFiles(t, module, []string{".git", "node_modules", "artifacts", "bin", "testdata"}, nil, func(path, rel string) {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return
		}
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		for _, line := range auditLibraryExitLines(fileSet, parsed) {
			t.Errorf("%s:%d ends the process from a goroutine or a select branch of a library package: it races the work it guards and takes a test binary down with it; let the flow return its code, or call an exit seam the caller hands in", rel, line)
		}
	})
}

// The witness sees an exit in a goroutine, in a select branch and in both,
// and passes package main, a plain exit and an exit seam.
func TestAuditLibraryExitWitnessSeesEachWay(t *testing.T) {
	t.Parallel()
	for source, want := range map[string]int{
		`package p; import "os"; func f() { go func() { os.Exit(1) }() }`:                                                                         1,
		`package p; import ("os"; "time"); func f(c chan int) { select { case <-c: case <-time.After(time.Second): os.Exit(143) } }`:              1,
		`package p; import ("os"; "time"); func f(c chan int) { go func() { select { case <-c: return; case <-time.After(1): }; os.Exit(2) }() }`: 1,
		`package p; import ("os"; "time"); func f(c chan int) { go func() { select { case <-c: os.Exit(3) } }() }`:                                1,
		`package p; import "syscall"; func f() { go syscall.Exit(1) }`:                                                                            1,
		`package main; import "os"; func f() { go func() { os.Exit(1) }() }`:                                                                      0,
		`package p; import "os"; func f() { os.Exit(1) }`:                                                                                         0,
		`package p; func f(exit func(int), c chan int) { go func() { <-c; exit(1) }() }`:                                                          0,
	} {
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, "x.go", source, 0)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(auditLibraryExitLines(fileSet, parsed)); got != want {
			t.Errorf("%s: %d sites, want %d", source, got, want)
		}
	}
}
