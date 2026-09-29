package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// A test writes only under its own temporary root, which goes with the test:
// Wido's disk clean of 2026-09-29 found about 1,500 engine-prefixed entries
// in the host temp roots from one day of test runs. testenv points TMPDIR at
// the process namespace before any test runs, so what still reaches the host
// root is a path spelled from /tmp, a path computed before TestMain (a
// package-level initializer), or a test of another language that keeps what
// it makes. internal/testenv makes the namespace and internal/diskstore owns
// the host roots; both are outside this rule.

// auditTestHostTempAllowance names a test file still allowed a host temp
// path, with the reason: none.
var auditTestHostTempAllowance = map[string]string{}

// auditTestTempCalls are the calls that create or name a temp location.
var auditTestTempCalls = map[string]bool{"os.MkdirTemp": true, "os.CreateTemp": true, "os.TempDir": true}

// auditTestHostTempLines returns the lines of a Go test file that reach the
// host temp root: an os.MkdirTemp/os.CreateTemp whose directory is a literal
// under /tmp or /var, and a package-level initializer that calls
// os.TempDir, os.MkdirTemp, os.CreateTemp or os.Getenv("TMPDIR") (it runs
// before TestMain, so before testenv points TMPDIR at the namespace).
func auditTestHostTempLines(fileSet *token.FileSet, file *ast.File) []int {
	var lines []int
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		name := ratchetQualifiedName(call.Fun)
		if name != "os.MkdirTemp" && name != "os.CreateTemp" {
			return true
		}
		if literal, ok := call.Args[0].(*ast.BasicLit); ok && literal.Kind == token.STRING {
			if value, err := strconv.Unquote(literal.Value); err == nil && (strings.HasPrefix(value, "/tmp") || strings.HasPrefix(value, "/var")) {
				lines = append(lines, fileSet.Position(call.Pos()).Line)
			}
		}
		return true
	})
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.VAR {
			continue
		}
		ast.Inspect(general, func(node ast.Node) bool {
			if _, literal := node.(*ast.FuncLit); literal {
				return false
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := ratchetQualifiedName(call.Fun)
			tmpdir := name == "os.Getenv" && len(call.Args) == 1 && auditTestStringArgument(call.Args[0]) == "TMPDIR"
			if auditTestTempCalls[name] || tmpdir {
				lines = append(lines, fileSet.Position(call.Pos()).Line)
			}
			return true
		})
	}
	return lines
}

func auditTestStringArgument(expression ast.Expr) string {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return ""
	}
	value, _ := strconv.Unquote(literal.Value)
	return value
}

// auditTestScriptTempPattern finds a TypeScript test's temp directory.
var auditTestScriptTempPattern = regexp.MustCompile(`\bmkdtemp(Sync)?\(`)

// auditTestScriptTempLines returns the lines of a TypeScript test that make a
// temp directory nothing removes: a file may make temp directories only in
// one helper that registers its removal with onTestFinished and rmSync.
func auditTestScriptTempLines(text string) []int {
	var lines []int
	for index, line := range strings.Split(text, "\n") {
		if auditTestScriptTempPattern.MatchString(line) {
			lines = append(lines, index+1)
		}
	}
	if len(lines) == 1 && strings.Contains(text, "onTestFinished(") && regexp.MustCompile(`\brmSync\(`).MatchString(text) {
		return nil
	}
	return lines
}

func TestAuditTestsWriteNoHostTemp(t *testing.T) {
	t.Parallel()
	_, module := verbRatchetRoots(t)
	var sites []ratchetSite
	walkRatchetFiles(t, module, []string{".git", "node_modules", "artifacts", "testdata", "bin"}, []string{"internal/testenv", "internal/diskstore"}, func(path, rel string) {
		switch {
		case strings.HasSuffix(rel, "_test.go"):
			fileSet := token.NewFileSet()
			parsed, err := parser.ParseFile(fileSet, path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", rel, err)
			}
			for _, line := range auditTestHostTempLines(fileSet, parsed) {
				sites = append(sites, ratchetSite{path: rel, line: line})
			}
		case strings.HasSuffix(rel, ".test.ts"), strings.HasSuffix(rel, ".test.tsx"), strings.HasSuffix(rel, ".test.mjs"):
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range auditTestScriptTempLines(string(data)) {
				sites = append(sites, ratchetSite{path: rel, line: line})
			}
		}
	})
	for _, site := range sites {
		if _, allowed := auditTestHostTempAllowance[site.path]; !allowed {
			t.Errorf("%s:%d reaches the host temp root; make it under t.TempDir() (Go) or in the file's one onTestFinished/rmSync helper (TypeScript)", site.path, site.line)
		}
	}
	for path := range auditTestHostTempAllowance {
		found := false
		for _, site := range sites {
			found = found || site.path == path
		}
		if !found {
			t.Errorf("stale allowance %s: it reaches no host temp root any more; remove it", path)
		}
	}
}

// The witness sees each way a test reaches the host temp root, and passes
// the namespace spellings.
func TestAuditTestHostTempWitnessSeesEachWay(t *testing.T) {
	t.Parallel()
	for source, want := range map[string]int{
		`package p; import "os"; func f() { os.MkdirTemp("/tmp", "x") }`:                     1,
		`package p; import "os"; func f() { os.CreateTemp("/var/tmp", "x") }`:                 1,
		`package p; import "os"; var d = os.TempDir()`:                                        1,
		`package p; import ("os"; "path/filepath"); var d = filepath.Join(os.TempDir(), "x")`: 1,
		`package p; import "os"; var d = os.Getenv("TMPDIR")`:                                 1,
		`package p; import "os"; var f = func() string { return os.TempDir() }`:               0,
		`package p; import "os"; func f() { os.MkdirTemp("", "x"); os.TempDir() }`:            0,
		`package p; import "os"; func f(t T) { os.MkdirTemp(t.TempDir(), "x") }`:              0,
	} {
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, "x_test.go", source, 0)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(auditTestHostTempLines(fileSet, parsed)); got != want {
			t.Errorf("%s: %d sites, want %d", source, got, want)
		}
	}
	helper := "import { mkdtempSync, rmSync } from \"node:fs\";\nfunction temp() {\n  const dir = mkdtempSync(\"x\");\n  onTestFinished(() => rmSync(dir, { recursive: true, force: true }));\n  return dir;\n}\n"
	for text, want := range map[string]int{
		helper: 0,
		"const dir = mkdtempSync(path.join(tmpdir(), \"metasystem-digest-\"));\n": 1,
		helper + "const other = mkdtempSync(\"y\");\n":                             2,
	} {
		if got := len(auditTestScriptTempLines(text)); got != want {
			t.Errorf("%q: %d sites, want %d", text, got, want)
		}
	}
}
