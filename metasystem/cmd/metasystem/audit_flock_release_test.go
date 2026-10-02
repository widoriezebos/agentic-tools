package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// A flock belongs to the open file description, and a fork by any goroutine
// duplicates every descriptor, close-on-exec ones included, until the child
// execs. flock(2): the lock is "released either by an explicit LOCK_UN
// operation on any of these duplicate descriptors, or when all such
// descriptors have been closed". A lock released by Close alone is still held
// by a sibling's fork copy, and the next LOCK_NB reads it as held: the disk
// clean repeat once answered "another steward is trimming the engine Go cache
// now" (goal no-flaky-tests, cluster C). Every release of a lock this process
// took therefore unlocks explicitly before it closes.
//
// The audit is syntactic, per package. A lock target is the E of a
// Flock(int(E.Fd()), LOCK_EX or LOCK_SH) call. A Close of a lock target (a
// local the same function locks, or a field the package locks, matched by
// field name) must sit in a function that also unlocks that target
// (Flock(int(E.Fd()), LOCK_UN), or a call that hands it to a same-package
// function unlocking its parameter). Exempt are a Close in the failure branch
// of the acquiring Flock and a non-deferred Close of a local before its first
// acquisition: neither holds a lock.

// auditFlockReleaseAllowance names a function allowed to close a lock target
// without unlocking it, with the reason. Only locks meant to be inherited
// belong here: LOCK_UN would release every child that holds the description.
var auditFlockReleaseAllowance = map[string]string{
	"internal/diskstore/process.go:closeWriter":         "the process scratch writer lock is one description every child started through PrepareChild inherits; LOCK_UN would release the root under a live child (fork copies are waited out by WriterDrain)",
	"internal/proofrun/scratch.go:Cleanup":              "the scratch writer lock is one description every root-writing child inherits; LOCK_UN would release the root under a live worker (fork copies are waited out by scratchDrain)",
	"internal/testenv/testenv.go:startFixtureCustodian": "the custodian log lock is held by the custodian through its stderr, which inherits the description; LOCK_UN would release it while the custodian lives",
}

type flockSite struct {
	path     string
	function string
	line     int
	target   string
}

func (s flockSite) String() string {
	return fmt.Sprintf("%s:%d %s closes %s without LOCK_UN", s.path, s.line, s.function, s.target)
}

// flockOperation reports the operation of a unix/syscall Flock call and the
// lock target E of its int(E.Fd()) argument; ok is false for any other call.
func flockOperation(call *ast.CallExpr) (target ast.Expr, unlock bool, ok bool) {
	name := ratchetQualifiedName(call.Fun)
	if (name != "unix.Flock" && name != "syscall.Flock") || len(call.Args) != 2 {
		return nil, false, false
	}
	operation := types.ExprString(call.Args[1])
	unlock = strings.Contains(operation, "LOCK_UN")
	// Any other operation (LOCK_EX, LOCK_SH, a variable) is an acquisition.
	argument := call.Args[0]
	if conversion, isCall := argument.(*ast.CallExpr); isCall && len(conversion.Args) == 1 && types.ExprString(conversion.Fun) == "int" {
		argument = conversion.Args[0]
	}
	fd, isCall := argument.(*ast.CallExpr)
	if !isCall || len(fd.Args) != 0 {
		return nil, unlock, true
	}
	selector, isSelector := fd.Fun.(*ast.SelectorExpr)
	if !isSelector || selector.Sel.Name != "Fd" {
		return nil, unlock, true
	}
	return selector.X, unlock, true
}

// flockKey names a lock target: a field by its name (the package's other
// selectors of that name match it), anything else by its spelling.
func flockKey(target ast.Expr) string {
	if selector, ok := target.(*ast.SelectorExpr); ok {
		return "field " + selector.Sel.Name
	}
	return "local " + types.ExprString(target)
}

// flockFunction is one function declaration of a package and its file.
type flockFunction struct {
	path string
	decl *ast.FuncDecl
}

// flockHelpers are the package's functions that lock or unlock a
// parameter (a call passing a lock target there locks or unlocks it), and
// those returning a file they locked (a local assigned from one is locked).
type flockHelpers struct {
	acquirers, releasers map[string][]int
	returners            map[string]bool
}

func flockParameters(decl *ast.FuncDecl) []string {
	var parameters []string
	if decl.Type.Params == nil {
		return nil
	}
	for _, field := range decl.Type.Params.List {
		if len(field.Names) == 0 {
			parameters = append(parameters, "")
		}
		for _, name := range field.Names {
			parameters = append(parameters, name.Name)
		}
	}
	return parameters
}

func newFlockHelpers(functions []flockFunction) flockHelpers {
	helpers := flockHelpers{acquirers: map[string][]int{}, releasers: map[string][]int{}, returners: map[string]bool{}}
	for _, function := range functions {
		parameters := flockParameters(function.decl)
		ast.Inspect(function.decl, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			target, unlock, isFlock := flockOperation(call)
			if !isFlock || target == nil {
				return true
			}
			for index, name := range parameters {
				if name == "" || types.ExprString(target) != name {
					continue
				}
				if unlock {
					helpers.releasers[function.decl.Name.Name] = append(helpers.releasers[function.decl.Name.Name], index)
				} else {
					helpers.acquirers[function.decl.Name.Name] = append(helpers.acquirers[function.decl.Name.Name], index)
				}
			}
			return true
		})
	}
	// A returner's returned file is locked; returners compose, so the set
	// grows until it is stable.
	for changed := true; changed; {
		changed = false
		for _, function := range functions {
			name := function.decl.Name.Name
			if helpers.returners[name] || function.decl.Body == nil {
				continue
			}
			locked := helpers.lockedLocals(function.decl.Body)
			ast.Inspect(function.decl.Body, func(node ast.Node) bool {
				if _, literal := node.(*ast.FuncLit); literal {
					return false
				}
				statement, ok := node.(*ast.ReturnStmt)
				if !ok || len(statement.Results) == 0 || helpers.returners[name] {
					return true
				}
				if locked[flockKey(statement.Results[0])] || helpers.produces(statement.Results[0], locked) {
					helpers.returners[name] = true
					changed = true
				}
				return true
			})
		}
	}
	return helpers
}

// lockedLocals are the locals of body a lock call or a producer locked.
func (h flockHelpers) lockedLocals(body ast.Node) map[string]bool {
	locked := map[string]bool{}
	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.CallExpr:
			if target, unlock, isLock := h.lockCall(node); isLock && !unlock && target != nil {
				locked[flockKey(target)] = true
			}
		case *ast.AssignStmt:
			if len(node.Lhs) >= 1 && len(node.Rhs) == 1 {
				if call, ok := node.Rhs[0].(*ast.CallExpr); ok && ratchetQualifiedName(call.Fun) == "lock.File" {
					locked["held "+types.ExprString(node.Lhs[0])] = true
				}
				if h.produces(node.Rhs[0], locked) {
					locked[flockKey(node.Lhs[0])] = true
				}
			}
		}
		return true
	})
	return locked
}

// produces reports a call whose first result is a locked file: a
// same-package returner, or File() of an internal/lock hold.
func (h flockHelpers) produces(expression ast.Expr, locked map[string]bool) bool {
	call, ok := expression.(*ast.CallExpr)
	if !ok {
		return false
	}
	if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "File" && len(call.Args) == 0 {
		return locked["held "+types.ExprString(selector.X)]
	}
	return h.returners[ratchetLastName(call.Fun)]
}

// lockCall is flockOperation extended to the package's helpers.
func (h flockHelpers) lockCall(call *ast.CallExpr) (target ast.Expr, unlock bool, ok bool) {
	if target, unlock, ok := flockOperation(call); ok {
		return target, unlock, true
	}
	name := ratchetLastName(call.Fun)
	for _, index := range h.releasers[name] {
		if index < len(call.Args) {
			return call.Args[index], true, true
		}
	}
	for _, index := range h.acquirers[name] {
		if index < len(call.Args) {
			return call.Args[index], false, true
		}
	}
	return nil, false, false
}

// auditFlockReleases returns every Close of a lock target in functions that
// do not unlock it.
func auditFlockReleases(fileSet *token.FileSet, functions []flockFunction) []flockSite {
	helpers := newFlockHelpers(functions)
	lockedFields := map[string]bool{}
	for _, function := range functions {
		if function.decl.Body == nil {
			continue
		}
		lockedLocals := helpers.lockedLocals(function.decl.Body)
		for key := range lockedLocals {
			if strings.HasPrefix(key, "field ") {
				lockedFields[key] = true
			}
		}
		// A locked local stored in a field makes the field a lock target.
		ast.Inspect(function.decl, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.AssignStmt:
				for index, left := range node.Lhs {
					if selector, ok := left.(*ast.SelectorExpr); ok && len(node.Rhs) == len(node.Lhs) && lockedLocals[flockKey(node.Rhs[index])] {
						lockedFields["field "+selector.Sel.Name] = true
					}
				}
			case *ast.KeyValueExpr:
				if key, ok := node.Key.(*ast.Ident); ok && lockedLocals[flockKey(node.Value)] {
					lockedFields["field "+key.Name] = true
				}
			}
			return true
		})
	}
	var sites []flockSite
	for _, function := range functions {
		sites = append(sites, auditFlockFunction(fileSet, function, helpers, lockedFields)...)
	}
	return sites
}

func auditFlockFunction(fileSet *token.FileSet, function flockFunction, helpers flockHelpers, lockedFields map[string]bool) []flockSite {
	body := function.decl.Body
	if body == nil {
		return nil
	}
	acquired := map[string][]token.Pos{} // key -> acquisition positions
	unlocked := map[string]bool{}
	// assignments are the error variables' assignments, each with the lock
	// target whose acquisition it holds ("" for any other call).
	type assignment struct {
		position token.Pos
		key      string
	}
	assignments := map[string][]assignment{}
	producerHolds := map[string]bool{}
	acquisitionKey := func(expression ast.Expr) string {
		if call, ok := expression.(*ast.CallExpr); ok {
			if target, unlock, isLock := helpers.lockCall(call); isLock && !unlock && target != nil {
				return flockKey(target)
			}
		}
		return ""
	}
	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.CallExpr:
			if target, unlock, isLock := helpers.lockCall(node); isLock && target != nil {
				if unlock {
					unlocked[flockKey(target)] = true
				} else {
					acquired[flockKey(target)] = append(acquired[flockKey(target)], node.Pos())
				}
			}
		case *ast.AssignStmt:
			if len(node.Rhs) == 1 && helpers.produces(node.Rhs[0], producerHolds) {
				acquired[flockKey(node.Lhs[0])] = append(acquired[flockKey(node.Lhs[0])], node.Pos())
			}
			if len(node.Rhs) == 1 {
				if call, ok := node.Rhs[0].(*ast.CallExpr); ok && ratchetQualifiedName(call.Fun) == "lock.File" {
					producerHolds["held "+types.ExprString(node.Lhs[0])] = true
				}
			}
			for index, left := range node.Lhs {
				ident, ok := left.(*ast.Ident)
				if !ok || ident.Name == "_" {
					continue
				}
				key := ""
				if len(node.Rhs) == len(node.Lhs) {
					key = acquisitionKey(node.Rhs[index])
				}
				assignments[ident.Name] = append(assignments[ident.Name], assignment{position: node.Pos(), key: key})
			}
		}
		return true
	})
	// decides returns the lock target whose acquisition an if statement
	// tests and whether its body is the success branch.
	decides := func(statement *ast.IfStmt) (key string, bodySucceeds bool) {
		condition := statement.Cond
		negated := false
		if unary, ok := condition.(*ast.UnaryExpr); ok && unary.Op == token.NOT {
			condition, negated = unary.X, true
		}
		if key := acquisitionKey(condition); key != "" {
			// if acquire(f) / if !acquire(f) on a bool acquirer.
			return key, !negated
		}
		binary, ok := condition.(*ast.BinaryExpr)
		if !ok || negated || (binary.Op != token.NEQ && binary.Op != token.EQL) {
			return "", false
		}
		ident, ok := binary.X.(*ast.Ident)
		if !ok {
			return "", false
		}
		if assign, ok := statement.Init.(*ast.AssignStmt); ok {
			// The init decides: its variable is the one tested.
			if len(assign.Rhs) == 1 && len(assign.Lhs) >= 1 && types.ExprString(assign.Lhs[0]) == ident.Name {
				return acquisitionKey(assign.Rhs[0]), binary.Op == token.EQL
			}
			return "", false
		}
		var nearest *assignment
		for index := range assignments[ident.Name] {
			candidate := assignments[ident.Name][index]
			if candidate.position < statement.Pos() && (nearest == nil || candidate.position > nearest.position) {
				nearest = &candidate
			}
		}
		if nearest == nil {
			return "", false
		}
		return nearest.key, binary.Op == token.EQL
	}
	// failures are the code reached only when an acquisition failed: its
	// failure branch, and after a success branch that returns, the rest of
	// the block. It holds nothing of that target.
	failures := map[ast.Node]string{}
	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.IfStmt:
			key, bodySucceeds := decides(node)
			switch {
			case key == "":
			case !bodySucceeds:
				failures[node.Body] = key
			case node.Else != nil:
				failures[node.Else] = key
			}
		case *ast.BlockStmt:
			for index, statement := range node.List {
				conditional, ok := statement.(*ast.IfStmt)
				if !ok || conditional.Else != nil || len(conditional.Body.List) == 0 {
					continue
				}
				if _, returns := conditional.Body.List[len(conditional.Body.List)-1].(*ast.ReturnStmt); !returns {
					continue
				}
				if key, bodySucceeds := decides(conditional); key != "" && bodySucceeds {
					for _, rest := range node.List[index+1:] {
						failures[rest] = key
					}
				}
			}
		}
		return true
	})
	var sites []flockSite
	var visit func(node ast.Node, exempt map[string]bool, deferred bool)
	visit = func(node ast.Node, exempt map[string]bool, deferred bool) {
		ast.Inspect(node, func(child ast.Node) bool {
			if child == nil || child == node {
				return true
			}
			if key, ok := failures[child]; ok {
				inner := map[string]bool{key: true}
				for name := range exempt {
					inner[name] = true
				}
				visit(child, inner, deferred)
				return false
			}
			switch child := child.(type) {
			case *ast.DeferStmt:
				visit(child, exempt, true)
				return false
			case *ast.CallExpr:
				selector, ok := child.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "Close" || len(child.Args) != 0 {
					return true
				}
				key := flockKey(selector.X)
				locked := lockedFields[key] || len(acquired[key]) > 0
				if !locked || unlocked[key] || exempt[key] {
					return true
				}
				if positions := acquired[key]; strings.HasPrefix(key, "local ") && !deferred && child.Pos() < positions[0] {
					return true
				}
				sites = append(sites, flockSite{path: function.path, function: function.decl.Name.Name, line: fileSet.Position(child.Pos()).Line, target: types.ExprString(selector.X)})
			}
			return true
		})
	}
	visit(body, map[string]bool{}, false)
	return sites
}

func flockFunctionsOf(path string, file *ast.File) []flockFunction {
	var functions []flockFunction
	for _, declaration := range file.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok {
			functions = append(functions, flockFunction{path: path, decl: function})
		}
	}
	return functions
}

// TestAuditEveryFlockReleaseUnlocks holds the rule above over every
// production package of the module.
func TestAuditEveryFlockReleaseUnlocks(t *testing.T) {
	t.Parallel()
	_, module := verbRatchetRoots(t)
	fileSet := token.NewFileSet()
	packages := map[string][]flockFunction{}
	walkRatchetFiles(t, module, []string{".git", "node_modules", "artifacts", "testdata", "bin"}, nil, func(path, rel string) {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return
		}
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		packages[filepath.Dir(rel)] = append(packages[filepath.Dir(rel)], flockFunctionsOf(rel, parsed)...)
	})
	allowed := map[string]bool{}
	var problems []string
	for _, functions := range packages {
		for _, site := range auditFlockReleases(fileSet, functions) {
			name := site.path + ":" + site.function
			if _, ok := auditFlockReleaseAllowance[name]; ok {
				allowed[name] = true
				continue
			}
			problems = append(problems, site.String()+"; unlock before Close (a fork copy keeps a lock released by Close alone)")
		}
	}
	sort.Strings(problems)
	for _, problem := range problems {
		t.Error(problem)
	}
	for name := range auditFlockReleaseAllowance {
		if !allowed[name] {
			t.Errorf("stale allowance %s: it closes no lock without LOCK_UN any more; remove it", name)
		}
	}
}

// The witness sees each way a lock is released by Close alone, and passes
// each correct release.
func TestAuditFlockReleaseWitnessSeesEachWay(t *testing.T) {
	t.Parallel()
	const header = "package p\nimport (\"os\"; \"golang.org/x/sys/unix\"; \"syscall\")\n"
	for source, want := range map[string]int{
		// The trim shape: deferred Close, no unlock.
		`func f(path string) error { file, err := os.Open(path); if err != nil { return err }; defer file.Close(); if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil { return err }; return nil }`: 1,
		// A held field closed by a method of the type.
		`type L struct{ f *os.File }
func (l *L) take() error { return syscall.Flock(int(l.f.Fd()), syscall.LOCK_EX) }
func (l *L) Release() error { return l.f.Close() }`: 1,
		// A later error path closing a held lock.
		`func f(file *os.File) error { if err := unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil { return err }; if err := work(); err != nil { file.Close(); return err }; return nil }`: 1,
		// Correct: unlock before close.
		`func f(file *os.File) error { if err := unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil { file.Close(); return err }; defer func() { _ = unix.Flock(int(file.Fd()), unix.LOCK_UN); _ = file.Close() }(); return nil }`: 0,
		// Correct: the failure branch holds nothing; err assigned then checked.
		`func f(file *os.File) error { var err error; for { err = unix.Flock(int(file.Fd()), unix.LOCK_EX); if err != unix.EINTR { break } }; if err != nil { file.Close(); return err }; return nil }`: 0,
		// Correct: a same-package helper unlocks its parameter.
		`func release(file *os.File) { _ = unix.Flock(int(file.Fd()), unix.LOCK_UN); _ = file.Close() }
func f(file *os.File) { if err := unix.Flock(int(file.Fd()), unix.LOCK_SH); err != nil { file.Close(); return }; defer release(file) }`: 0,
		// Correct: a Close before the acquisition holds nothing.
		`func f(file *os.File) error { if err := file.Chmod(0); err != nil { file.Close(); return err }; return unix.Flock(int(file.Fd()), unix.LOCK_EX) }`: 0,
		// Correct: after a bool acquirer's success returns, the rest of the block failed.
		`func try(file *os.File) bool { return unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB) == nil }
func f(file *os.File) (*os.File, error) { if try(file) { return file, nil }; file.Close(); return nil, nil }`: 0,
		// Correct: err == nil returns the lock; what follows failed.
		`func f(file *os.File) (*os.File, error) { err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); if err == nil { return file, nil }; file.Close(); return nil, err }`: 0,
		// A returned lock closed by its caller without unlocking.
		`func take(path string) (*os.File, error) { file, _ := os.Open(path); if err := unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil { file.Close(); return nil, err }; return file, nil }
func g() { file, err := take("x"); if err != nil { return }; defer file.Close() }`: 1,
		// An internal/lock hold's raw file closed without unlocking.
		`func g() { held, err := lock.File("x", 0, lock.Exclusive); if err != nil { return }; file := held.File(); file.Close() }`: 1,
		// A file never locked is not a lock.
		`func f(file *os.File) error { return file.Close() }`: 0,
	} {
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, "x.go", header+source, 0)
		if err != nil {
			t.Fatalf("%s: %v", source, err)
		}
		if got := len(auditFlockReleases(fileSet, flockFunctionsOf("x.go", parsed))); got != want {
			t.Errorf("%s: %d sites, want %d", source, got, want)
		}
	}
}
