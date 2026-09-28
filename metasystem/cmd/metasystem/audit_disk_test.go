package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// auditDiskTestMainAllowance names the one TestMain still allowed to allocate
// a temp entry before testenv's custodian exists: the wait candidate, which
// unit A9 of disk-lifetimes moves under custody and then removes from here.
var auditDiskTestMainAllowance = map[string]bool{"cmd/metasystem/ambient_controls_test.go": true}

const auditDiskTestenvImport = "github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"

// TestAuditDiskNoTestMainAllocatesBeforeTestenv refuses an os.MkdirTemp or
// os.CreateTemp call positioned before a TestMain's testenv.Main or
// MainWithSetup call (or anywhere in a TestMain that makes no such call): an
// entry made there lands in the host temp root, outside the namespace and
// before the fixture custodian exists (rule A6).
func TestAuditDiskNoTestMainAllocatesBeforeTestenv(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	allowed := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if path != root && (name == "testdata" || name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, "_test.go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return err
		}
		for _, line := range auditDiskTestMainEarlyTemps(fileSet, parsed, strings.HasPrefix(relative, "internal/testenv/")) {
			if auditDiskTestMainAllowance[relative] {
				allowed[relative] = true
				continue
			}
			t.Errorf("%s:%d allocates a temp entry in TestMain before testenv.Main/MainWithSetup; move it into MainWithSetup's setup", relative, line)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the Go tree: %v", err)
	}
	if !reflect.DeepEqual(allowed, auditDiskTestMainAllowance) {
		t.Errorf("used allowances = %v, want exactly %v; remove an allowance whose site has moved", allowed, auditDiskTestMainAllowance)
	}
}

// auditDiskTestMainEarlyTemps returns the lines of the os.MkdirTemp and
// os.CreateTemp calls positioned before the first testenv.Main/MainWithSetup
// call of each TestMain in file. inTestenv resolves the unqualified calls of
// package testenv itself.
func auditDiskTestMainEarlyTemps(fileSet *token.FileSet, file *ast.File, inTestenv bool) []int {
	osNames, testenvNames := map[string]bool{}, map[string]bool{}
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		local := filepath.Base(path)
		if spec.Name != nil {
			local = spec.Name.Name
		}
		switch path {
		case "os":
			osNames[local] = true
		case auditDiskTestenvImport:
			testenvNames[local] = true
		}
	}
	var lines []int
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil || function.Name.Name != "TestMain" || function.Body == nil {
			continue
		}
		mainAt, temps := token.NoPos, []token.Pos{}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			var qualifier, name string
			switch callee := call.Fun.(type) {
			case *ast.SelectorExpr:
				if ident, ok := callee.X.(*ast.Ident); ok {
					qualifier, name = ident.Name, callee.Sel.Name
				}
			case *ast.Ident:
				name = callee.Name
			}
			isMain := name == "Main" || name == "MainWithSetup"
			if isMain && (testenvNames[qualifier] || inTestenv && qualifier == "") {
				if mainAt == token.NoPos || call.Pos() < mainAt {
					mainAt = call.Pos()
				}
			}
			if osNames[qualifier] && (name == "MkdirTemp" || name == "CreateTemp") {
				temps = append(temps, call.Pos())
			}
			return true
		})
		for _, at := range temps {
			if mainAt == token.NoPos || at < mainAt {
				lines = append(lines, fileSet.Position(at).Line)
			}
		}
	}
	return lines
}

func TestAuditDiskTestMainWitnessSeesPositionNotSpelling(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		source string
		want   []int
	}{
		"before Main": {`package p
import ("os"; "testing"; "github.com/widoriezebos/agentic-tools/metasystem/internal/testenv")
func TestMain(m *testing.M) {
	_, _ = os.MkdirTemp("", "x")
	os.Exit(testenv.Main(m))
}`, []int{4}},
		"aliased before MainWithSetup": {`package p
import (sys "os"; "testing"; env "github.com/widoriezebos/agentic-tools/metasystem/internal/testenv")
func TestMain(m *testing.M) {
	_, _ = sys.CreateTemp("", "x")
	sys.Exit(env.MainWithSetup(m, nil))
}`, []int{4}},
		"inside setup": {`package p
import ("os"; "testing"; "github.com/widoriezebos/agentic-tools/metasystem/internal/testenv")
func TestMain(m *testing.M) {
	os.Exit(testenv.MainWithSetup(m, func() error { _, err := os.MkdirTemp("", "x"); return err }))
}`, nil},
		"no testenv call": {`package p
import ("os"; "testing")
func TestMain(m *testing.M) {
	_, _ = os.MkdirTemp("", "x")
	os.Exit(m.Run())
}`, []int{4}},
		"other function": {`package p
import ("os"; "testing")
func TestOther(t *testing.T) { _, _ = os.MkdirTemp("", "x") }`, nil},
	}
	for name, test := range cases {
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, name+".go", test.source, 0)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := auditDiskTestMainEarlyTemps(fileSet, parsed, false); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: early temp lines = %v, want %v", name, got, test.want)
		}
	}
}

// TestAuditNoGoSourceSpellsTheLowercaseStampSentinel holds the Go tree at zero
// lowercase spellings of the build-stamp record sentinel (disk-lifetimes rule
// A3). The sentinel exists in source only as enginebuild's uppercase constant,
// lowered at runtime, so the only lowercase sentinel a compiled engine carries
// is the record the linker placed there and ReadStamp cannot be misled by code.
func TestAuditNoGoSourceSpellsTheLowercaseStampSentinel(t *testing.T) {
	t.Parallel()
	repository, _ := verbRatchetRoots(t)
	sentinel := strings.ToLower("METASYSTEM-BUILD-STAMP")
	scanned := 0
	var sites []string
	walkRatchetFiles(t, repository, []string{".git", "node_modules", "artifacts"}, nil, func(path, rel string) {
		if !strings.HasSuffix(rel, ".go") {
			return
		}
		scanned++
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), sentinel) {
			sites = append(sites, rel)
		}
	})
	if scanned == 0 {
		t.Fatal("scanned no Go files; the walk no longer reaches the tree")
	}
	if len(sites) != 0 {
		t.Fatalf("Go sources spell the lowercase stamp sentinel (use enginebuild's uppercase constant): %v", sites)
	}
}
