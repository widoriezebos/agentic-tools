package testenv

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestNoParallelTestMutatesProcessState: every test in a package shares one
// process, so a parallel test that writes process-wide state changes it
// under every other parallel test in the binary (flaky-test cluster F). The
// audit follows each parallel scope of every _test.go file (the statements
// after t.Parallel() in a test or subtest, with every closure in them)
// through the same package's test functions and methods it calls, and fails
// on a write it reaches to process-wide state: os.Setenv, os.Unsetenv,
// os.Clearenv, os.Chdir, or an assignment to a package-level variable (the
// package's own, or another package's through its name). A write inside a
// sync.Once Do or under a held lock is a serialized lazy cache, not a
// sharer. Give the code under test the value through a parameter or a
// per-test struct instead; TestMain and sequential tests may still set
// process state, since no parallel test runs beside them.
//
// A scope ends at an owned-child guard: the parent returns there and a
// re-exec of the test binary runs that one test alone, so what follows is
// that child's own process.
func TestNoParallelTestMutatesProcessState(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	writes, err := findParallelStateWrites(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, write := range writes {
		t.Errorf("%s:%d %s in %s, reached from the parallel scope %s; pass the value to the code under test instead of writing process-wide state",
			write.file, write.line, write.act, write.function, write.scope)
	}
}

type parallelStateWrite struct {
	file, function, scope, act string
	line                       int
}

// parallelStatePackage is one package's test code: the functions and
// methods its _test.go files declare, and the package-level variables of
// the package (test and non-test files alike).
type parallelStatePackage struct {
	fileSet   *token.FileSet
	functions map[string][]parallelStateFunction
	globals   map[string]bool
	files     []*parallelStateFile
}

type parallelStateFile struct {
	path    string
	syntax  *ast.File
	imports map[string]string
}

type parallelStateFunction struct {
	file *parallelStateFile
	name string
	body *ast.BlockStmt
}

func findParallelStateWrites(root string) ([]parallelStateWrite, error) {
	packages := map[string]*parallelStatePackage{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if path != root && (name == "testdata" || name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".") || name == "artifacts") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		fileSet := token.NewFileSet()
		syntax, err := parser.ParseFile(fileSet, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		key := filepath.Dir(path) + "\x00" + syntax.Name.Name
		pkg := packages[key]
		if pkg == nil {
			pkg = &parallelStatePackage{functions: map[string][]parallelStateFunction{}, globals: map[string]bool{}}
			packages[key] = pkg
		}
		for _, decl := range syntax.Decls {
			if general, ok := decl.(*ast.GenDecl); ok && general.Tok == token.VAR {
				for _, spec := range general.Specs {
					for _, ident := range spec.(*ast.ValueSpec).Names {
						pkg.globals[ident.Name] = true
					}
				}
			}
		}
		if !strings.HasSuffix(name, "_test.go") {
			return nil
		}
		// The test files are parsed again into one file set per package so
		// positions resolve; the non-test files only give their globals.
		if pkg.fileSet == nil {
			pkg.fileSet = token.NewFileSet()
		}
		syntax, err = parser.ParseFile(pkg.fileSet, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		file := &parallelStateFile{path: filepath.ToSlash(relative), syntax: syntax, imports: parallelStateImports(syntax)}
		pkg.files = append(pkg.files, file)
		for _, decl := range syntax.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			pkg.functions[function.Name.Name] = append(pkg.functions[function.Name.Name], parallelStateFunction{file: file, name: parallelStateFunctionName(function), body: function.Body})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var writes []parallelStateWrite
	for _, pkg := range packages {
		writes = append(writes, pkg.writes()...)
	}
	sort.Slice(writes, func(i, j int) bool {
		if writes[i].file != writes[j].file {
			return writes[i].file < writes[j].file
		}
		return writes[i].line < writes[j].line
	})
	return writes, nil
}

func parallelStateFunctionName(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) == 0 {
		return function.Name.Name
	}
	receiver := function.Recv.List[0].Type
	if star, ok := receiver.(*ast.StarExpr); ok {
		receiver = star.X
	}
	if index, ok := receiver.(*ast.IndexExpr); ok {
		receiver = index.X
	}
	if ident, ok := receiver.(*ast.Ident); ok {
		return ident.Name + "." + function.Name.Name
	}
	return function.Name.Name
}

// parallelStateImports maps each file-local package name to its path.
func parallelStateImports(file *ast.File) map[string]string {
	imports := map[string]string{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "_" || name == "." {
			continue
		}
		imports[name] = path
	}
	return imports
}

// writes walks every parallel scope of the package and what it calls.
func (pkg *parallelStatePackage) writes() []parallelStateWrite {
	var writes []parallelStateWrite
	seen := map[string]bool{}
	for _, file := range pkg.files {
		for _, decl := range file.syntax.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			for _, scope := range pkg.parallelScopes(file, function.Body) {
				scopeName := file.path + "#" + parallelStateFunctionName(function)
				visited := map[*ast.BlockStmt]bool{}
				var visit func(file *parallelStateFile, function string, enclosing ast.Node, nodes []ast.Node)
				visit = func(current *parallelStateFile, function string, enclosing ast.Node, nodes []ast.Node) {
					for _, node := range nodes {
						// The enclosing block stays at the bottom of the stack,
						// so a Lock() statement before a scope's statement is
						// seen.
						stack := []ast.Node{enclosing}
						ast.Inspect(node, func(node ast.Node) bool {
							if node == nil {
								stack = stack[:len(stack)-1]
								return true
							}
							stack = append(stack, node)
							found := pkg.writeAt(current, node)
							if len(found) != 0 && parallelStateSerialized(stack) {
								found = nil
							}
							for _, write := range found {
								write.file, write.function, write.scope = current.path, function, scopeName
								write.line = pkg.fileSet.Position(node.Pos()).Line
								key := fmt.Sprintf("%s:%d:%s", write.file, write.line, write.act)
								if !seen[key] {
									seen[key] = true
									writes = append(writes, write)
								}
							}
							call, ok := node.(*ast.CallExpr)
							if !ok {
								return true
							}
							for _, callee := range pkg.callees(current, call.Fun) {
								if visited[callee.body] {
									continue
								}
								visited[callee.body] = true
								visit(callee.file, callee.name, nil, []ast.Node{callee.body})
							}
							return true
						})
					}
				}
				visit(file, parallelStateFunctionName(function), scope.block, scope.statements)
			}
		}
	}
	return writes
}

// parallelScopes are the statements of body, and of every function literal
// in it, that run after that function's own t.Parallel() call, up to an
// owned-child guard: the statements after it run only in the re-exec child.
func (pkg *parallelStatePackage) parallelScopes(file *parallelStateFile, body *ast.BlockStmt) []parallelScope {
	var scopes []parallelScope
	var collect func(block *ast.BlockStmt)
	collect = func(block *ast.BlockStmt) {
		for index, statement := range block.List {
			if isParallelCall(statement) {
				var rest []ast.Node
				for _, later := range block.List[index+1:] {
					if pkg.ownedChildGuard(file, later) {
						break
					}
					rest = append(rest, later)
				}
				scopes = append(scopes, parallelScope{block: block, statements: rest})
				return
			}
		}
		// No t.Parallel() at this level: a subtest's function literal may
		// have its own.
		ast.Inspect(block, func(node ast.Node) bool {
			if literal, ok := node.(*ast.FuncLit); ok && literal.Body != block {
				collect(literal.Body)
				return false
			}
			return true
		})
	}
	collect(body)
	return scopes
}

// ownedChildGuard says whether statement ends the parent's run of the test
// and leaves the rest to a re-exec of the test binary that runs this one
// test alone (it names -test.run=): `if run(t, ...) { return }` where the
// package's run re-execs, or an if whose own body re-execs and returns.
func (pkg *parallelStatePackage) ownedChildGuard(file *parallelStateFile, statement ast.Stmt) bool {
	guard, ok := statement.(*ast.IfStmt)
	if !ok || guard.Else != nil || len(guard.Body.List) == 0 {
		return false
	}
	if _, ok := guard.Body.List[len(guard.Body.List)-1].(*ast.ReturnStmt); !ok {
		return false
	}
	if guard.Init == nil && parallelStateReexecs(guard.Body) {
		return true
	}
	call, ok := guard.Cond.(*ast.CallExpr)
	if !ok || guard.Init != nil || len(guard.Body.List) != 1 {
		return false
	}
	callees := pkg.callees(file, call.Fun)
	if len(callees) == 0 {
		return false
	}
	for _, callee := range callees {
		if !parallelStateReexecs(callee.body) {
			return false
		}
	}
	return true
}

// parallelStateReexecs says whether node names a -test.run= selection: it
// starts the test binary again for one test.
func parallelStateReexecs(node ast.Node) bool {
	reexec := false
	ast.Inspect(node, func(node ast.Node) bool {
		if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING && strings.Contains(literal.Value, "-test.run=") {
			reexec = true
		}
		return !reexec
	})
	return reexec
}

// parallelScope is the statements of block that run in parallel.
type parallelScope struct {
	block      *ast.BlockStmt
	statements []ast.Node
}

func isParallelCall(statement ast.Stmt) bool {
	expression, ok := statement.(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := expression.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Parallel" {
		return false
	}
	_, ok = selector.X.(*ast.Ident)
	return ok
}

// callees are the package's test functions and methods a call may reach:
// a plain name, or a method name on a receiver that is not a package.
func (pkg *parallelStatePackage) callees(file *parallelStateFile, fun ast.Expr) []parallelStateFunction {
	switch fun := fun.(type) {
	case *ast.Ident:
		var found []parallelStateFunction
		for _, function := range pkg.functions[fun.Name] {
			if !strings.Contains(function.name, ".") {
				found = append(found, function)
			}
		}
		return found
	case *ast.SelectorExpr:
		if ident, ok := fun.X.(*ast.Ident); ok {
			if _, isPackage := file.imports[ident.Name]; isPackage {
				return nil
			}
		}
		var found []parallelStateFunction
		for _, function := range pkg.functions[fun.Sel.Name] {
			if strings.Contains(function.name, ".") {
				found = append(found, function)
			}
		}
		return found
	case *ast.IndexExpr:
		return pkg.callees(file, fun.X)
	}
	return nil
}

// writeAt is the process-wide write node is, if any.
func (pkg *parallelStatePackage) writeAt(file *parallelStateFile, node ast.Node) []parallelStateWrite {
	switch node := node.(type) {
	case *ast.CallExpr:
		selector, ok := node.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok || file.imports[ident.Name] != "os" {
			return nil
		}
		switch selector.Sel.Name {
		case "Setenv", "Unsetenv", "Clearenv", "Chdir":
			return []parallelStateWrite{{act: "os." + selector.Sel.Name}}
		}
	case *ast.AssignStmt:
		if node.Tok == token.DEFINE {
			return nil
		}
		var writes []parallelStateWrite
		for _, target := range node.Lhs {
			if name, ok := pkg.globalTarget(file, target, node); ok {
				writes = append(writes, parallelStateWrite{act: "assignment to the package-level " + name})
			}
		}
		return writes
	case *ast.IncDecStmt:
		if name, ok := pkg.globalTarget(file, node.X, nil); ok {
			return []parallelStateWrite{{act: "assignment to the package-level " + name}}
		}
	}
	return nil
}

// globalTarget says whether target names a package-level variable: the
// package's own by a name no enclosing function declares, or another
// package's as name.Var, or a field or element of either.
func (pkg *parallelStatePackage) globalTarget(file *parallelStateFile, target ast.Expr, _ *ast.AssignStmt) (string, bool) {
	for {
		switch expression := target.(type) {
		case *ast.ParenExpr:
			target = expression.X
			continue
		case *ast.StarExpr:
			target = expression.X
			continue
		case *ast.IndexExpr:
			// An element write to a shared map or slice is a write too, but a
			// local map indexed by a global key is not; only follow X.
			target = expression.X
			continue
		case *ast.SelectorExpr:
			if ident, ok := expression.X.(*ast.Ident); ok {
				if _, isPackage := file.imports[ident.Name]; isPackage && !parallelStateShadowed(file.syntax, ident) {
					return ident.Name + "." + expression.Sel.Name, true
				}
			}
			target = expression.X
			continue
		case *ast.Ident:
			if expression.Name == "_" || !pkg.globals[expression.Name] {
				return "", false
			}
			if parallelStateShadowed(file.syntax, expression) {
				return "", false
			}
			return expression.Name, true
		}
		return "", false
	}
}

// parallelStateShadowed says whether a local declaration in a function
// enclosing ident declares its name: a parameter, a := or a var before it.
func parallelStateShadowed(file *ast.File, ident *ast.Ident) bool {
	shadowed := false
	ast.Inspect(file, func(node ast.Node) bool {
		if shadowed || node == nil {
			return false
		}
		if node.Pos() > ident.Pos() || node.End() < ident.Pos() {
			// Only enclosing nodes and declarations before ident matter.
			if node.End() < ident.Pos() {
				return false
			}
			return false
		}
		var fields []*ast.FieldList
		switch node := node.(type) {
		case *ast.FuncDecl:
			fields = append(fields, node.Recv, node.Type.Params, node.Type.Results)
		case *ast.FuncLit:
			fields = append(fields, node.Type.Params, node.Type.Results)
		case *ast.BlockStmt:
			for _, statement := range node.List {
				if statement.Pos() >= ident.Pos() {
					break
				}
				if parallelStateDeclares(statement, ident.Name) {
					shadowed = true
					return false
				}
			}
		case *ast.IfStmt:
			if node.Init != nil && parallelStateDeclares(node.Init, ident.Name) {
				shadowed = true
			}
		case *ast.ForStmt:
			if node.Init != nil && parallelStateDeclares(node.Init, ident.Name) {
				shadowed = true
			}
		case *ast.RangeStmt:
			if node.Tok == token.DEFINE {
				for _, expression := range []ast.Expr{node.Key, node.Value} {
					if local, ok := expression.(*ast.Ident); ok && local.Name == ident.Name {
						shadowed = true
					}
				}
			}
		case *ast.TypeSwitchStmt:
			if node.Assign != nil && parallelStateDeclares(node.Assign, ident.Name) {
				shadowed = true
			}
		}
		for _, list := range fields {
			if list == nil {
				continue
			}
			for _, field := range list.List {
				for _, name := range field.Names {
					if name.Name == ident.Name {
						shadowed = true
					}
				}
			}
		}
		return !shadowed
	})
	return shadowed
}

// parallelStateSerialized says whether the write at the top of stack runs
// one at a time: in a function literal handed to a Do call (sync.Once), or
// after a Lock() call statement that no Unlock() statement in the same block
// follows before it (a mutex-guarded lazy cache).
func parallelStateSerialized(stack []ast.Node) bool {
	for index := len(stack) - 1; index > 0; index-- {
		if stack[index-1] == nil {
			break
		}
		if literal, ok := stack[index].(*ast.FuncLit); ok {
			if call, ok := stack[index-1].(*ast.CallExpr); ok {
				if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Do" {
					for _, argument := range call.Args {
						if argument == literal {
							return true
						}
					}
				}
			}
		}
		block, ok := stack[index-1].(*ast.BlockStmt)
		if !ok {
			continue
		}
		locked := false
		for _, statement := range block.List {
			if statement == stack[index] {
				break
			}
			expression, ok := statement.(*ast.ExprStmt)
			if !ok {
				continue
			}
			call, ok := expression.X.(*ast.CallExpr)
			if !ok {
				continue
			}
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
				switch selector.Sel.Name {
				case "Lock":
					locked = true
				case "Unlock":
					locked = false
				}
			}
		}
		if locked {
			return true
		}
	}
	return false
}

func parallelStateDeclares(statement ast.Stmt, name string) bool {
	switch statement := statement.(type) {
	case *ast.AssignStmt:
		if statement.Tok != token.DEFINE {
			return false
		}
		for _, target := range statement.Lhs {
			if ident, ok := target.(*ast.Ident); ok && ident.Name == name {
				return true
			}
		}
	case *ast.DeclStmt:
		general, ok := statement.Decl.(*ast.GenDecl)
		if !ok {
			return false
		}
		for _, spec := range general.Specs {
			if value, ok := spec.(*ast.ValueSpec); ok {
				for _, ident := range value.Names {
					if ident.Name == name {
						return true
					}
				}
			}
		}
	}
	return false
}

// TestParallelStateWritesFindsEveryShape drives the detector over a
// fixture package: every write a parallel scope reaches is found, and the
// serialized, sequential, owned-child and shadowed forms are not.
func TestParallelStateWritesFindsEveryShape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		path := filepath.Join(root, "fixture", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("fixture.go", "package fixture\n\nvar seam = 1\n")
	write("fixture_test.go", `package fixture

import (
	"os"
	"os/exec"
	"sync"
	"testing"

	other "example.invalid/other"
)

var (
	cache    string
	once     sync.Once
	mu       sync.Mutex
	counter  int
)

func setHome(t *testing.T) { _ = os.Setenv("HOME", "x") }

func (b *bed) swap() { seam = 2 }

type bed struct{}

func runOwned(t *testing.T) bool { _ = exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$"); return true }

func TestMain(m *testing.M) { _ = os.Setenv("A", "b"); os.Exit(m.Run()) }

func TestSequential(t *testing.T) { _ = os.Unsetenv("A"); seam = 3 }

func TestHelperWrite(t *testing.T) {
	_ = os.Chdir("/")
	t.Parallel()
	setHome(t)
}

func TestMethodWrite(t *testing.T) {
	t.Parallel()
	(&bed{}).swap()
}

func TestForeignAndCounter(t *testing.T) {
	t.Parallel()
	other.Clock = nil
	counter++
}

func TestSubtest(t *testing.T) {
	t.Run("child", func(t *testing.T) {
		t.Parallel()
		os.Clearenv()
	})
}

func TestSerialized(t *testing.T) {
	t.Parallel()
	once.Do(func() { cache = "built" })
	mu.Lock()
	counter = 4
	mu.Unlock()
	seam := 5
	seam = 6
	_ = seam
}

func TestOwned(t *testing.T) {
	t.Parallel()
	if runOwned(t) {
		return
	}
	_ = os.Setenv("HOME", "child")
}

func TestOwnedInline(t *testing.T) {
	t.Parallel()
	if os.Getenv("CHILD") != "1" {
		_ = exec.Command(os.Args[0], "-test.run=^TestOwnedInline$").Run()
		return
	}
	seam = 7
}
`)
	writes, err := findParallelStateWrites(root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, write := range writes {
		got = append(got, fmt.Sprintf("%d %s %s", write.line, write.function, write.act))
	}
	want := []string{
		"19 setHome os.Setenv",
		"21 bed.swap assignment to the package-level seam",
		"44 TestForeignAndCounter assignment to the package-level other.Clock",
		"45 TestForeignAndCounter assignment to the package-level counter",
		"51 TestSubtest os.Clearenv",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("writes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
