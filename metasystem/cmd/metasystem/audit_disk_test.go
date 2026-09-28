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

// auditDiskGoFiles parses every non-test Go file under the given
// directories of the module, relative to the module root.
func auditDiskGoFiles(t *testing.T, directories ...string) map[string]*ast.File {
	t.Helper()
	root := filepath.Join("..", "..")
	files := map[string]*ast.File{}
	for _, directory := range directories {
		err := filepath.WalkDir(filepath.Join(root, directory), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if name := entry.Name(); name == "testdata" || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			relative, _ := filepath.Rel(root, path)
			files[filepath.ToSlash(relative)] = parsed
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", directory, err)
		}
	}
	return files
}

// auditDiskGoCleanSites names every call or literal that assembles a `go
// clean` argv: the literal "clean" beside the literal "go" or one of Go's
// cache-clean flags.
func auditDiskGoCleanSites(file *ast.File) int {
	sites := 0
	check := func(elements []ast.Expr) {
		words := map[string]bool{}
		for _, element := range elements {
			if literal, ok := element.(*ast.BasicLit); ok && literal.Kind == token.STRING {
				if value, err := strconv.Unquote(literal.Value); err == nil {
					words[value] = true
				}
			}
		}
		if words["clean"] && (words["go"] || words["-cache"] || words["-testcache"] || words["-modcache"] || words["-fuzzcache"]) {
			sites++
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.CallExpr:
			check(value.Args)
		case *ast.CompositeLit:
			check(value.Elts)
		}
		return true
	})
	return sites
}

// The engine never runs `go clean` (disk-lifetimes rule A7): the steward's
// trimmer is the only thing that removes a cache entry.
func TestAuditDiskEngineNeverRunsGoClean(t *testing.T) {
	t.Parallel()
	for relative, file := range auditDiskGoFiles(t, "cmd", "internal") {
		if sites := auditDiskGoCleanSites(file); sites != 0 {
			t.Errorf("%s assembles %d `go clean` argv; the engine never runs go clean", relative, sites)
		}
	}
	for source, want := range map[string]int{
		`package p; import "os/exec"; func f() { exec.Command("go", "clean", "-cache") }`: 1,
		`package p; var argv = []string{"clean", "-testcache"}`:                             1,
		`package p; func f(run func(...string)) { run("git", "clean", "-fdq") }`:            0,
	} {
		parsed, err := parser.ParseFile(token.NewFileSet(), "fixture.go", source, 0)
		if err != nil {
			t.Fatal(err)
		}
		if got := auditDiskGoCleanSites(parsed); got != want {
			t.Errorf("witness over %q found %d, want %d", source, got, want)
		}
	}
}

// auditDiskSelectorCalls returns the qualifier.Name calls in file whose
// package is imported from importPath and whose name is in names.
func auditDiskSelectorCalls(fileSet *token.FileSet, file *ast.File, importPath string, names ...string) []string {
	locals := map[string]bool{}
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		if path != importPath {
			continue
		}
		local := filepath.Base(path)
		if spec.Name != nil {
			local = spec.Name.Name
		}
		locals[local] = true
	}
	var found []string
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if ident, ok := selector.X.(*ast.Ident); ok && locals[ident.Name] {
			for _, name := range names {
				if selector.Sel.Name == name {
					found = append(found, ident.Name+"."+name+" at line "+strconv.Itoa(fileSet.Position(selector.Pos()).Line))
				}
			}
		}
		return true
	})
	return found
}

// internal/gocache reads time only in its seam file (rule A9) and removes
// nothing by path (rule A12, DL3A-09): every removal is an Unlinkat or a
// directory Unlinkat relative to a verified shard handle.
func TestAuditDiskGocacheClockAndHandleRelativeRemoval(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "internal", "gocache")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, filepath.Join(root, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		checked++
		if name != "clock.go" {
			if found := auditDiskSelectorCalls(fileSet, parsed, "time", "Now", "Sleep", "Since", "Until", "After", "Tick", "NewTimer", "NewTicker"); len(found) != 0 {
				t.Errorf("internal/gocache/%s reads the wall clock outside clock.go: %v", name, found)
			}
		}
		removals := auditDiskSelectorCalls(fileSet, parsed, "os", "Remove", "RemoveAll")
		removals = append(removals, auditDiskSelectorCalls(fileSet, parsed, "golang.org/x/sys/unix", "Unlink", "Rmdir")...)
		removals = append(removals, auditDiskSelectorCalls(fileSet, parsed, "syscall", "Unlink", "Rmdir", "Unlinkat")...)
		if len(removals) != 0 {
			t.Errorf("internal/gocache/%s removes by path: %v", name, removals)
		}
	}
	if checked == 0 {
		t.Fatal("no internal/gocache source was checked")
	}
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, "fixture.go", `package p; import ("os"; t "time"); func f() { os.RemoveAll("x"); _ = t.Now() }`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(auditDiskSelectorCalls(fileSet, parsed, "os", "RemoveAll")) != 1 || len(auditDiskSelectorCalls(fileSet, parsed, "time", "Now")) != 1 {
		t.Fatal("the witness does not see an aliased or plain call")
	}
}
