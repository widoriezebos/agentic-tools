package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
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
// production root), never by calling them.
var hostProcessTableReaders = map[string]bool{"AllPids": true, "TakeProcessCensus": true}

// TestAuditHostProcessScansReadAGivenTable: a production scan that calls
// the host's process table directly cannot be handed a table, so its test
// runs against every process on the host, the parallel tests' included,
// and a pid reused into a group or session the test recorded decides its
// verdict (flaky-test cluster B). No production Go file outside
// internal/identity calls identity.AllPids or identity.TakeProcessCensus;
// the scan takes an identity.ProcessTable per call or per struct.
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
		for _, call := range hostProcessTableCalls(fileSet, parsed) {
			t.Errorf("%s:%d calls identity.%s; take an identity.ProcessTable from the caller (identity.KernelProcessTable at the production root) so a test can hand the scan its own processes",
				relative, call.line, call.reader)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the Go tree: %v", err)
	}
}

type hostProcessTableCall struct {
	reader string
	line   int
}

// hostProcessTableCalls lists the calls of identity's host-table readers in
// one file, through whatever name the file imports identity by.
func hostProcessTableCalls(fileSet *token.FileSet, file *ast.File) []hostProcessTableCall {
	local := ""
	for _, spec := range file.Imports {
		if importPath, err := strconv.Unquote(spec.Path.Value); err != nil || importPath != identityImportPath {
			continue
		}
		local = "identity"
		if spec.Name != nil {
			local = spec.Name.Name
		}
	}
	if local == "" || local == "_" {
		return nil
	}
	var calls []hostProcessTableCall
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == local && hostProcessTableReaders[selector.Sel.Name] {
			calls = append(calls, hostProcessTableCall{reader: selector.Sel.Name, line: fileSet.Position(call.Pos()).Line})
		}
		return true
	})
	return calls
}

// The audit's reader finds a direct call through the default and a renamed
// import, and leaves a function value (a seam's production default) alone.
func TestAuditHostProcessScansFindsDirectCalls(t *testing.T) {
	t.Parallel()
	source := `package p

import (
	id "` + identityImportPath + `"
)

var seam = id.AllPids

func scan() {
	_, _ = id.AllPids()
	_, _ = id.TakeProcessCensus()
	_, _ = seam()
}
`
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, "p.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	calls := hostProcessTableCalls(fileSet, parsed)
	if len(calls) != 2 || calls[0].reader != "AllPids" || calls[0].line != 10 || calls[1].reader != "TakeProcessCensus" {
		t.Fatalf("calls = %+v; want AllPids at line 10 and TakeProcessCensus", calls)
	}
}
