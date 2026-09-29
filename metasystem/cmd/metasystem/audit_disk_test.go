package main

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// auditDiskTestMainAllowance names a TestMain still allowed to allocate a
// temp entry before testenv's custodian exists: none since the wait
// candidate moved under custody (disk-lifetimes A9).
var auditDiskTestMainAllowance = map[string]bool{}

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
		`package p; var argv = []string{"clean", "-testcache"}`:                           1,
		`package p; func f(run func(...string)) { run("git", "clean", "-fdq") }`:          0,
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

// internal/gocache reads no wall clock: every trimmer takes now from its
// caller (rule A9; the unused clock.go seam left under the dead-code
// check). It removes nothing by path (rule A12, DL3A-09): every removal is
// an Unlinkat or a directory Unlinkat relative to a verified shard handle.
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
		if found := auditDiskSelectorCalls(fileSet, parsed, "time", "Now", "Sleep", "Since", "Until", "After", "Tick", "NewTimer", "NewTicker"); len(found) != 0 {
			t.Errorf("internal/gocache/%s reads the wall clock: %v", name, found)
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

// Every engine site that starts a compile takes its cache paths from the
// authenticated domain (disk-lifetimes A8): no non-test source outside the
// cache packages resolves the cache from the inherited environment alone.
// testenv inherits by design (a test process is not an engine; it issues
// the context its children authenticate).
func TestAuditDiskEveryCompileSiteResolvesTheDomain(t *testing.T) {
	t.Parallel()
	allowed := map[string]bool{"internal/gocache": true, "internal/cachedomain": true, "internal/testenv": true}
	for relative, file := range auditDiskGoFiles(t, "cmd", "internal") {
		if allowed[filepath.ToSlash(filepath.Dir(relative))] {
			continue
		}
		found := auditDiskSelectorCalls(token.NewFileSet(), file, "github.com/widoriezebos/agentic-tools/metasystem/internal/gocache", "Carry", "Resolve", "ResolveUsing")
		if len(found) != 0 {
			t.Errorf("%s resolves the cache from the inherited environment: %v; use cachedomain.Carry or cachedomain.Resolve", relative, found)
		}
	}
}

/* ------------------------------------------ Part B U0: R1, R11, R13 -- */

// auditDiskEmptyTempCeiling is the number of os.MkdirTemp/os.CreateTemp calls
// in non-test Go whose directory argument is the empty string: each writes
// into the process's TMPDIR with no owner (Part B R1). The design counted 29
// at d4d124fb8; the tree at bd7e7d388 has 35. It is lowered at U1b-1 and is
// zero at U1b-2; it never rises.
const auditDiskEmptyTempCeiling = 35

// auditDiskTempDirCeiling is the number of os.TempDir() calls in non-test Go
// outside internal/diskstore and internal/testenv (Part B R1): 9 in the
// design at d4d124fb8, 11 at bd7e7d388; zero at U1b-2.
const auditDiskTempDirCeiling = 11

// auditDiskScriptMktempCeiling and auditDiskScriptVariableRmCeiling hold the
// committed shell files at zero mktemp lines without an engine-prefixed
// template (R1) and zero rm lines over a variable path (R11). Both were zero
// before Part B: the scripts U2b named were retired into Go.
const (
	auditDiskScriptMktempCeiling     = 0
	auditDiskScriptVariableRmCeiling = 0
)

// auditDiskOSNames returns the local names the file imports "os" under.
func auditDiskImportNames(file *ast.File, want string) map[string]bool {
	names := map[string]bool{}
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		if path != want {
			continue
		}
		local := filepath.Base(path)
		if spec.Name != nil {
			local = spec.Name.Name
		}
		names[local] = true
	}
	return names
}

// auditDiskEmptyTempLines returns the lines of os.MkdirTemp/os.CreateTemp
// calls whose first argument is the literal "" (or “).
func auditDiskEmptyTempLines(fileSet *token.FileSet, file *ast.File) []int {
	osNames := auditDiskImportNames(file, "os")
	var lines []int
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok || !osNames[ident.Name] || selector.Sel.Name != "MkdirTemp" && selector.Sel.Name != "CreateTemp" {
			return true
		}
		if literal, ok := call.Args[0].(*ast.BasicLit); ok && literal.Kind == token.STRING {
			if value, err := strconv.Unquote(literal.Value); err == nil && value == "" {
				lines = append(lines, fileSet.Position(call.Pos()).Line)
			}
		}
		return true
	})
	return lines
}

// auditDiskSelectorLines returns the lines where the file names pkg.name,
// called or taken as a value (a seam `now = time.Now` counts too).
func auditDiskSelectorLines(fileSet *token.FileSet, file *ast.File, pkg string, names ...string) []int {
	local := auditDiskImportNames(file, pkg)
	want := map[string]bool{}
	for _, name := range names {
		want[name] = true
	}
	var lines []int
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if ident, ok := selector.X.(*ast.Ident); ok && local[ident.Name] && want[selector.Sel.Name] {
			lines = append(lines, fileSet.Position(selector.Pos()).Line)
		}
		return true
	})
	return lines
}

// auditDiskGoSites parses every non-test Go file of the module and returns
// the sites classify reports.
func auditDiskGoSites(t *testing.T, include func(rel string) bool, classify func(*token.FileSet, *ast.File) []int) []ratchetSite {
	t.Helper()
	_, module := verbRatchetRoots(t)
	var sites []ratchetSite
	walkRatchetFiles(t, module, []string{".git", "node_modules", "artifacts", "testdata", "bin"}, nil, func(path, rel string) {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || !include(rel) {
			return
		}
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		for _, line := range classify(fileSet, parsed) {
			sites = append(sites, ratchetSite{path: rel, line: line})
		}
	})
	return sites
}

func TestAuditDiskEmptyArgumentTempCalls(t *testing.T) {
	t.Parallel()
	sites := auditDiskGoSites(t, func(string) bool { return true }, auditDiskEmptyTempLines)
	checkVerbRatchet(t, "empty-argument os.MkdirTemp/os.CreateTemp calls in non-test Go", "auditDiskEmptyTempCeiling",
		len(sites), auditDiskEmptyTempCeiling, sites)
}

func TestAuditDiskTempDirReaders(t *testing.T) {
	t.Parallel()
	outside := func(rel string) bool {
		return !strings.HasPrefix(rel, "internal/diskstore/") && !strings.HasPrefix(rel, "internal/testenv/")
	}
	sites := auditDiskGoSites(t, outside, func(fileSet *token.FileSet, file *ast.File) []int {
		return auditDiskSelectorLines(fileSet, file, "os", "TempDir")
	})
	checkVerbRatchet(t, "os.TempDir() readers outside internal/diskstore and internal/testenv", "auditDiskTempDirCeiling",
		len(sites), auditDiskTempDirCeiling, sites)
}

// auditDiskSweeperCallees are the owners' retention files the sweeper
// calls into outside internal/diskstore; R13 binds them too.
var auditDiskSweeperCallees = map[string]bool{"internal/launch/retention.go": true, "internal/launch/unit_retention.go": true, "internal/proofrun/retention.go": true}

// TestAuditDiskStoreReadsNoWallClock is R13's witness for the registry and
// sweeper package: every instant is a parameter, so no file of
// internal/diskstore names time.Now or time.Sleep, not even as a seam.
func TestAuditDiskStoreReadsNoWallClock(t *testing.T) {
	t.Parallel()
	inStore := func(rel string) bool {
		return strings.HasPrefix(rel, "internal/diskstore/") || auditDiskSweeperCallees[rel]
	}
	scanned := 0
	sites := auditDiskGoSites(t, inStore, func(fileSet *token.FileSet, file *ast.File) []int {
		scanned++
		return auditDiskSelectorLines(fileSet, file, "time", "Now", "Sleep")
	})
	if scanned == 0 {
		t.Fatal("scanned no file of internal/diskstore")
	}
	if len(sites) != 0 {
		t.Fatalf("internal/diskstore reads the wall clock; take now as a parameter:\n%s", ratchetSiteList(sites))
	}
}

var (
	auditDiskMktempRE   = regexp.MustCompile(`(^|[\s;&|(` + "`" + `])mktemp(\s|$|\))`)
	auditDiskTemplateRE = regexp.MustCompile(`/metasystem-[A-Za-z0-9._-]*X{3,}`)
	auditDiskRmRE       = regexp.MustCompile(`(^\s*|[;&|(]\s*|\b(?:then|do|else)\s+)rm\s+([^#;&|)]*)`)
)

// auditDiskMktempUnprefixed reports a shell line that runs mktemp without an
// engine-prefixed template (a path whose last element starts metasystem- and
// ends in XXX…).
func auditDiskMktempUnprefixed(line string) bool {
	code, _, _ := strings.Cut(line, "#")
	return auditDiskMktempRE.MatchString(code) && !auditDiskTemplateRE.MatchString(code)
}

// auditDiskVariableRm reports a shell line whose rm names a path through a
// variable or a command substitution.
func auditDiskVariableRm(line string) bool {
	code, _, _ := strings.Cut(line, "#")
	for _, match := range auditDiskRmRE.FindAllStringSubmatch(code, -1) {
		if strings.ContainsAny(match[2], "$`") {
			return true
		}
	}
	return false
}

// auditDiskShellFiles lists the committed shell files under metasystem/: by
// extension, or by a shell shebang.
func auditDiskShellFiles(t *testing.T) map[string]string {
	t.Helper()
	_, module := verbRatchetRoots(t)
	files := map[string]string{}
	walkRatchetFiles(t, module, scriptRuleSkipNames, nil, func(path, rel string) {
		if strings.HasSuffix(rel, ".sh") || strings.HasSuffix(rel, ".bash") {
			files[rel] = path
			return
		}
		head := make([]byte, 64)
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		read, _ := file.Read(head)
		_ = file.Close()
		first, _, _ := strings.Cut(string(head[:read]), "\n")
		if strings.HasPrefix(first, "#!") && (strings.Contains(first, "sh") || strings.Contains(first, "bash")) {
			files[rel] = path
		}
	})
	return files
}

func TestAuditDiskScriptTemplatesAndRemovals(t *testing.T) {
	t.Parallel()
	var mktempSites, rmSites []ratchetSite
	files := auditDiskShellFiles(t)
	if len(files) == 0 {
		t.Fatal("found no committed shell file; the walk no longer reaches the tree")
	}
	for rel, path := range files {
		for index, line := range readRatchetLines(t, path) {
			if auditDiskMktempUnprefixed(line) {
				mktempSites = append(mktempSites, ratchetSite{path: rel, line: index + 1, text: strings.TrimSpace(line)})
			}
			if auditDiskVariableRm(line) {
				rmSites = append(rmSites, ratchetSite{path: rel, line: index + 1, text: strings.TrimSpace(line)})
			}
		}
	}
	checkVerbRatchet(t, "mktemp lines without an engine-prefixed template in committed shell files", "auditDiskScriptMktempCeiling",
		len(mktempSites), auditDiskScriptMktempCeiling, mktempSites)
	checkVerbRatchet(t, "rm lines over a variable path in committed shell files", "auditDiskScriptVariableRmCeiling",
		len(rmSites), auditDiskScriptVariableRmCeiling, rmSites)
}

// TestAuditDiskWitnessesSeeTheirSites is each witness's mutation: a site of
// each kind is found, and its compliant spelling is not.
func TestAuditDiskWitnessesSeeTheirSites(t *testing.T) {
	t.Parallel()
	source := `package p
import (sys "os"; clock "time")
func a() { _, _ = sys.MkdirTemp("", "x"); _, _ = sys.CreateTemp(` + "``" + `, "y") }
func b(dir string) { _, _ = sys.MkdirTemp(dir, "x"); _ = sys.TempDir() }
var now = clock.Now
func c() { clock.Sleep(1) }
`
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, "p.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := auditDiskEmptyTempLines(fileSet, parsed); !reflect.DeepEqual(got, []int{3, 3}) {
		t.Errorf("empty-argument temp lines = %v, want [3 3]", got)
	}
	if got := auditDiskSelectorLines(fileSet, parsed, "os", "TempDir"); !reflect.DeepEqual(got, []int{4}) {
		t.Errorf("TempDir lines = %v, want [4]", got)
	}
	if got := auditDiskSelectorLines(fileSet, parsed, "time", "Now", "Sleep"); !reflect.DeepEqual(got, []int{5, 6}) {
		t.Errorf("wall-clock lines = %v, want [5 6]", got)
	}
	for line, want := range map[string]bool{
		`tmp=$(mktemp -d)`:                     true,
		`tmp=$(mktemp)`:                        true,
		`f=$(mktemp "${TMPDIR:-/tmp}/x.XXXX")`: true,
		`d=$(mktemp -d "${METASYSTEM_SUITE_PROGRESS_TMP:-${TMPDIR:-/tmp}}/metasystem-bed.XXXXXX")`: false,
		`# mktemp -d is documented here`: false,
	} {
		if got := auditDiskMktempUnprefixed(line); got != want {
			t.Errorf("mktemp %q = %v, want %v", line, got, want)
		}
	}
	for line, want := range map[string]bool{
		`rm -rf "$dir"`:              true,
		`rm -f -- ${stage}/x`:        true,
		"rm -rf `pwd`/x":             true,
		`[ -f x ] && rm "$tmp"`:      true,
		`rm -f /tmp/literal.lock`:    false,
		`echo "remove with rm $x" #`: false,
		`firm "$x"`:                  false,
	} {
		if got := auditDiskVariableRm(line); got != want {
			t.Errorf("rm %q = %v, want %v", line, got, want)
		}
	}
}

// auditDiskSettingOwners are the only files that spell a disk-lifetime key:
// the table of 3.13 and the evidence root's one owner.
var auditDiskSettingOwners = map[string]bool{"internal/config/disksettings.go": true, "internal/config/evidenceroot.go": true}

// TestAuditDiskSettingsAreSpelledOnce is the settings witness's static half
// (Part B 3.13, R7): no production file but the table spells a key of 3.13,
// so every number lives in its one compiled default and is read through
// diskstore.LoadSettings.
func TestAuditDiskSettingsAreSpelledOnce(t *testing.T) {
	t.Parallel()
	keys := map[string]bool{}
	for _, row := range config.DiskSettings() {
		keys[row.Key] = true
	}
	sites := auditDiskGoSites(t, func(rel string) bool { return !auditDiskSettingOwners[rel] }, func(fileSet *token.FileSet, file *ast.File) []int {
		var lines []int
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			if value, err := strconv.Unquote(literal.Value); err == nil && keys[value] {
				lines = append(lines, fileSet.Position(literal.Pos()).Line)
			}
			return true
		})
		return lines
	})
	if len(sites) != 0 {
		t.Fatalf("disk-lifetime keys spelled outside their table; use the config constants:\n%s", ratchetSiteList(sites))
	}
}
