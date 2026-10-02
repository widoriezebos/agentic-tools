package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// identityImportPath is the package whose host process-table readers the
// audit below confines.
const identityImportPath = "github.com/widoriezebos/agentic-tools/metasystem/internal/identity"

// hostProcessTableReaders are identity's readers of the whole host process
// table. Outside identity a scan reaches them through an
// identity.ProcessTable it is given (identity.KernelProcessTable at the
// production root), never by naming them: not as a call, and not as a
// function value a seam defaults to (a package variable a test swaps, or a
// dependency field a production path fills with the host's table).
var hostProcessTableReaders = map[string]bool{"AllPids": true, "TakeProcessCensus": true}

// kernelGroupReaders are the kernel's per-process group and session reads.
// One read of a named process is fine; a read inside a loop is a scan of a
// pid list, which reads the table through identity.ProcessTable's Group or
// Session instead, so a test can hand it exactly its own processes.
var kernelGroupReaders = map[string]bool{"Getpgid": true, "Getsid": true}

var kernelGroupReaderPackages = map[string]bool{"golang.org/x/sys/unix": true, "syscall": true}

// TestAuditHostProcessScansReadAGivenTable: a production scan that reads
// the host's process table itself cannot be handed a table, so its test
// runs against every process on the host, the parallel tests' included,
// and a pid reused into a group or session the test recorded decides its
// verdict (flaky-test cluster B). No production Go file outside
// internal/identity names identity.AllPids or identity.TakeProcessCensus,
// as a call or as a value, or reads a process group or session with
// unix/syscall Getpgid or Getsid inside a loop; the scan takes an
// identity.ProcessTable per call or per struct.
func TestAuditHostProcessScansReadAGivenTable(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
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
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if strings.HasPrefix(relative, "internal/identity/") {
			return nil
		}
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return err
		}
		for _, use := range hostProcessTableUses(fileSet, parsed) {
			if use.call {
				t.Errorf("%s:%d calls identity.%s; take an identity.ProcessTable from the caller (identity.KernelProcessTable at the production root) so a test can hand the scan its own processes",
					relative, use.line, use.reader)
			} else {
				t.Errorf("%s:%d uses identity.%s as a value; a seam defaulting to the host's table is swapped by tests or filled on a production path, so take an identity.ProcessTable from the caller (identity.KernelProcessTable at the production root) instead",
					relative, use.line, use.reader)
			}
		}
		for _, read := range loopedKernelGroupReads(fileSet, parsed) {
			t.Errorf("%s:%d reads %s inside a loop; a scan of pids reads groups and sessions through the identity.ProcessTable it is given",
				relative, read.line, read.reader)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the Go tree: %v", err)
	}
}

type hostProcessTableUse struct {
	reader string
	line   int
	call   bool
}

// importedAs is the name file imports path by, or "" when it does not.
func importedAs(file *ast.File, path, defaultName string) string {
	local := ""
	for _, spec := range file.Imports {
		if importPath, err := strconv.Unquote(spec.Path.Value); err != nil || importPath != path {
			continue
		}
		local = defaultName
		if spec.Name != nil {
			local = spec.Name.Name
		}
	}
	if local == "_" || local == "." {
		return ""
	}
	return local
}

// hostProcessTableUses lists every mention of identity's host-table readers
// in one file, through whatever name the file imports identity by, and
// whether the mention is a call or a value.
func hostProcessTableUses(fileSet *token.FileSet, file *ast.File) []hostProcessTableUse {
	local := importedAs(file, identityImportPath, "identity")
	if local == "" {
		return nil
	}
	called := map[*ast.SelectorExpr]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
				called[selector] = true
			}
		}
		return true
	})
	var uses []hostProcessTableUse
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == local && hostProcessTableReaders[selector.Sel.Name] {
			uses = append(uses, hostProcessTableUse{
				reader: selector.Sel.Name, line: fileSet.Position(selector.Pos()).Line, call: called[selector],
			})
		}
		return true
	})
	return uses
}

type kernelGroupRead struct {
	reader string
	line   int
}

// loopedKernelGroupReads lists the unix or syscall Getpgid and Getsid calls
// inside a for or range body of one file. A function literal inside the
// loop is still inside it: it runs per iteration.
func loopedKernelGroupReads(fileSet *token.FileSet, file *ast.File) []kernelGroupRead {
	locals := map[string]string{}
	for path := range kernelGroupReaderPackages {
		if local := importedAs(file, path, filepath.Base(path)); local != "" {
			locals[local] = path
		}
	}
	if len(locals) == 0 {
		return nil
	}
	var reads []kernelGroupRead
	seen := map[token.Pos]bool{}
	inspectBody := func(body *ast.BlockStmt) {
		ast.Inspect(body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok || locals[pkg.Name] == "" || !kernelGroupReaders[selector.Sel.Name] || seen[call.Pos()] {
				return true
			}
			seen[call.Pos()] = true
			reads = append(reads, kernelGroupRead{
				reader: pkg.Name + "." + selector.Sel.Name, line: fileSet.Position(call.Pos()).Line,
			})
			return true
		})
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch loop := node.(type) {
		case *ast.ForStmt:
			inspectBody(loop.Body)
		case *ast.RangeStmt:
			inspectBody(loop.Body)
		}
		return true
	})
	return reads
}

// The audit's readers find a call and a value of the host-table readers
// through the default and a renamed import, and a Getpgid or Getsid read in
// a loop body, while a single read outside any loop is left alone.
func TestAuditHostProcessScansFindsDirectCalls(t *testing.T) {
	t.Parallel()
	source := `package p

import (
	"syscall"

	id "` + identityImportPath + `"
	sys "golang.org/x/sys/unix"
)

var seam = id.AllPids

func scan() {
	_, _ = id.AllPids()
	_, _ = id.TakeProcessCensus()
	_, _ = seam()
	_, _ = sys.Getpgid(1)
	pids, _ := seam()
	for _, pid := range pids {
		_, _ = sys.Getpgid(int(pid))
		_ = func() { _, _ = syscall.Getsid(int(pid)) }
	}
	for index := 0; index < 2; index++ {
		_, _ = syscall.Getpgid(index)
	}
}
`
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, "p.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	uses := hostProcessTableUses(fileSet, parsed)
	if len(uses) != 3 ||
		uses[0] != (hostProcessTableUse{reader: "AllPids", line: 10, call: false}) ||
		uses[1] != (hostProcessTableUse{reader: "AllPids", line: 13, call: true}) ||
		uses[2] != (hostProcessTableUse{reader: "TakeProcessCensus", line: 14, call: true}) {
		t.Fatalf("uses = %+v; want the AllPids value at line 10, and the AllPids and TakeProcessCensus calls at 13 and 14", uses)
	}
	reads := loopedKernelGroupReads(fileSet, parsed)
	if len(reads) != 3 ||
		reads[0] != (kernelGroupRead{reader: "sys.Getpgid", line: 19}) ||
		reads[1] != (kernelGroupRead{reader: "syscall.Getsid", line: 20}) ||
		reads[2] != (kernelGroupRead{reader: "syscall.Getpgid", line: 23}) {
		t.Fatalf("looped reads = %+v; want sys.Getpgid at 19, syscall.Getsid at 20 and syscall.Getpgid at 23", reads)
	}
}

// processSeamMarkers are the names whose mention makes a package variable a
// process-table seam: identity's host-table readers and the table type; the
// kernel's per-process group and session reads count too (kernelGroupReaders).
// A one-pid identity.Prober seam is not a table: it answers for the pid a
// test names, never for another test's processes.
var processSeamMarkers = map[string]bool{
	"AllPids": true, "TakeProcessCensus": true, "KernelProcessTable": true, "ProcessTable": true,
}

type processSeamAssignment struct {
	file, seam string
	line       int
}

// mentionsProcessSeam reports whether node names a process seam marker:
// identity's (unqualified inside identity itself) or unix/syscall Getpgid
// and Getsid.
func mentionsProcessSeam(node ast.Node, file *ast.File, insideIdentity bool) bool {
	identityLocal := importedAs(file, identityImportPath, "identity")
	kernel := map[string]bool{}
	for path := range kernelGroupReaderPackages {
		if local := importedAs(file, path, filepath.Base(path)); local != "" {
			kernel[local] = true
		}
	}
	found := false
	ast.Inspect(node, func(child ast.Node) bool {
		switch expression := child.(type) {
		case *ast.SelectorExpr:
			if pkg, ok := expression.X.(*ast.Ident); ok {
				if identityLocal != "" && pkg.Name == identityLocal && processSeamMarkers[expression.Sel.Name] ||
					kernel[pkg.Name] && kernelGroupReaders[expression.Sel.Name] {
					found = true
				}
			}
			return !found
		case *ast.Ident:
			if insideIdentity && processSeamMarkers[expression.Name] {
				found = true
			}
		}
		return !found
	})
	return found
}

// processSeamAssignments lists the assignments in one package's test files
// to a package-level variable its production files declare as a process
// seam: a variable whose type or value names a process-table reader, or
// whose type is a struct of the package that holds one. TestMain alone may
// set such a variable, once, before any test runs.
func processSeamAssignments(fileSet *token.FileSet, production, tests map[string]*ast.File, insideIdentity bool) []processSeamAssignment {
	seamTypes := map[string]bool{}
	for _, file := range production {
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}
			for _, spec := range general.Specs {
				if typeSpec := spec.(*ast.TypeSpec); mentionsProcessSeam(typeSpec.Type, file, insideIdentity) {
					seamTypes[typeSpec.Name.Name] = true
				}
			}
		}
	}
	seams := map[string]bool{}
	for _, file := range production {
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.VAR {
				continue
			}
			for _, spec := range general.Specs {
				valueSpec := spec.(*ast.ValueSpec)
				seam := false
				if valueSpec.Type != nil {
					if named, ok := valueSpec.Type.(*ast.Ident); ok && seamTypes[named.Name] || mentionsProcessSeam(valueSpec.Type, file, insideIdentity) {
						seam = true
					}
				}
				for _, value := range valueSpec.Values {
					if mentionsProcessSeam(value, file, insideIdentity) {
						seam = true
					}
				}
				for _, name := range valueSpec.Names {
					if seam {
						seams[name.Name] = true
					}
				}
			}
		}
	}
	if len(seams) == 0 {
		return nil
	}
	var found []processSeamAssignment
	names := make([]string, 0, len(tests))
	for name := range tests {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, declaration := range tests[name].Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil || function.Recv == nil && function.Name.Name == "TestMain" {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				assignment, ok := node.(*ast.AssignStmt)
				if !ok || assignment.Tok == token.DEFINE {
					return true
				}
				for _, target := range assignment.Lhs {
					root := target
					for {
						switch expression := root.(type) {
						case *ast.SelectorExpr:
							root = expression.X
							continue
						case *ast.IndexExpr:
							root = expression.X
							continue
						case *ast.ParenExpr:
							root = expression.X
							continue
						case *ast.StarExpr:
							root = expression.X
							continue
						}
						break
					}
					if identifier, ok := root.(*ast.Ident); ok && identifier.Obj == nil && seams[identifier.Name] {
						found = append(found, processSeamAssignment{file: name, seam: identifier.Name, line: fileSet.Position(identifier.Pos()).Line})
					}
				}
				return true
			})
		}
	}
	return found
}

// TestAuditNoTestAssignsAProcessSeam: a package variable holding a process
// table or a group or session reader that a test swaps is shared by every
// parallel test of the package, so one test's scripted processes decide
// another's verdict, and the race detector sees the write (flaky-test
// clusters B and F). A test hands its table per call or per struct;
// TestMain alone sets a package default, before any test runs.
func TestAuditNoTestAssignsAProcessSeam(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	type packageFiles struct{ production, tests map[string]*ast.File }
	packages := map[string]*packageFiles{}
	fileSet := token.NewFileSet()
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
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(filepath.Dir(relative)) + ":" + parsed.Name.Name
		files := packages[key]
		if files == nil {
			files = &packageFiles{production: map[string]*ast.File{}, tests: map[string]*ast.File{}}
			packages[key] = files
		}
		if strings.HasSuffix(name, "_test.go") {
			files.tests[relative] = parsed
		} else {
			files.production[relative] = parsed
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the Go tree: %v", err)
	}
	for key, files := range packages {
		insideIdentity := strings.HasPrefix(key, "internal/identity:")
		for _, assignment := range processSeamAssignments(fileSet, files.production, files.tests, insideIdentity) {
			t.Errorf("%s:%d assigns the package process seam %s; hand the test's table or group reader to the call or the struct instead (TestMain alone may set a package default)",
				assignment.file, assignment.line, assignment.seam)
		}
	}
}

// The seam audit finds an assignment to a package variable whose type is a
// struct holding a process table, to one defaulting to an identity reader
// (unqualified inside identity), and to one whose value reads Getpgid, in a
// test, a helper and a cleanup; it leaves TestMain, a local of the same
// name, and a variable with no process seam alone.
func TestAuditNoTestAssignsAProcessSeamFindsEverySwap(t *testing.T) {
	t.Parallel()
	production := `package p

import (
	id "` + identityImportPath + `"
	"golang.org/x/sys/unix"
)

type readers struct {
	table id.ProcessTable
	count int
}

var seams readers
var pids = id.AllPids
var scope = func(pid int) int { group, _ := unix.Getpgid(pid); return group }
var plain = 3
`
	tests := `package p

import "testing"

func TestMain(m *testing.M) { seams = readers{} }

func helper(t *testing.T) {
	previous := seams
	seams.table = nil
	t.Cleanup(func() { seams = previous })
}

func TestSwap(t *testing.T) {
	pids = nil
	scope = nil
	plain = 4
	seams := readers{}
	seams.count = 1
}
`
	identitySource := `package identity

func AllPids() ([]int64, error) { return nil, nil }

var survivorPids = AllPids
`
	identityTest := `package identity

import "testing"

func TestSurvivors(t *testing.T) { survivorPids = nil }
`
	fileSet := token.NewFileSet()
	parse := func(name, source string) *ast.File {
		parsed, err := parser.ParseFile(fileSet, name, source, 0)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	got := processSeamAssignments(fileSet, map[string]*ast.File{"p.go": parse("p.go", production)},
		map[string]*ast.File{"p_test.go": parse("p_test.go", tests)}, false)
	want := []processSeamAssignment{
		{file: "p_test.go", seam: "seams", line: 9}, {file: "p_test.go", seam: "seams", line: 10},
		{file: "p_test.go", seam: "pids", line: 14}, {file: "p_test.go", seam: "scope", line: 15},
	}
	if len(got) != len(want) {
		t.Fatalf("assignments = %+v; want %+v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("assignments = %+v; want %+v", got, want)
		}
	}
	inside := processSeamAssignments(fileSet, map[string]*ast.File{"identity.go": parse("identity.go", identitySource)},
		map[string]*ast.File{"identity_test.go": parse("identity_test.go", identityTest)}, true)
	if len(inside) != 1 || inside[0].seam != "survivorPids" || inside[0].line != 5 {
		t.Fatalf("identity assignments = %+v; want survivorPids at line 5", inside)
	}
}
