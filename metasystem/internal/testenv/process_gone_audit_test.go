package testenv

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

// TestNoTestReadsAProcessGoneOnce: no test asserts that a process or a
// process group is gone with one kill(target, 0). A process another process
// ends goes in stages (its files close, it exits, its parent or init reaps
// it), and a zombie still answers kill(target, 0) (Darwin answers a group of
// zombies with EPERM). A test that saw an earlier stage (a pipe's EOF, a
// lock freed, a SIGKILL sent, a drain that counts zombies as gone) and then
// reads kill(target, 0) once fails whenever the reap is late. The test
// awaits the end instead: testenv.AwaitProcessTargetGone, or the exit of the
// child it Waits itself.
//
// The audit finds an if statement whose condition reads a kill(target, 0) as
// "still there": the call (or the error it returned in the if's own
// initializer) compared == nil, or not errors.Is(..., ESRCH). A for loop over
// the same read is a wait and is not flagged; neither is a check that the
// target is still alive (!= nil, errors.Is(..., ESRCH)).
func TestNoTestReadsAProcessGoneOnce(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && skippedDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), "_test.go") || strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "_") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		lines, err := oneShotGoneReads(path, nil)
		if err != nil {
			return err
		}
		for _, line := range lines {
			found = append(found, filepath.ToSlash(relative)+":"+line)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(found)
	for _, site := range found {
		t.Errorf("%s: a single kill(target, 0) asserts the target gone; await it with testenv.AwaitProcessTargetGone", site)
	}
}

// The detector flags the one-shot "still there" reads and passes waits and
// "still alive" checks.
func TestOneShotGoneReadsDetector(t *testing.T) {
	t.Parallel()
	source := `package p

import (
	"errors"
	"syscall"
	"golang.org/x/sys/unix"
)

func flagged(t T, pid int) {
	if err := unix.Kill(pid, 0); !errors.Is(err, unix.ESRCH) { // 1
		t.Fatal(err)
	}
	if syscall.Kill(-pid, 0) == nil { // 2
		t.Fatal()
	}
	if err := syscall.Kill(pid, 0); err == nil { // 3
		t.Fatal()
	}
	if ok || syscall.Kill(pid, 0) == nil { // 4
		t.Fatal()
	}
}

func passed(t T, pid int) {
	for unix.Kill(pid, 0) == nil {
	}
	if unix.Kill(pid, 0) != nil {
		t.Fatal("ended before the witness looked")
	}
	if err := unix.Kill(pid, 0); errors.Is(err, unix.ESRCH) {
		return
	}
	if err := unix.Kill(pid, 9); err == nil {
		return
	}
}
`
	lines, err := oneShotGoneReads("detector_test.go", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"10", "13", "16", "19"}
	if strings.Join(lines, ",") != strings.Join(want, ",") {
		t.Fatalf("flagged lines = %v, want %v", lines, want)
	}
}

// oneShotGoneReads returns the lines of the if statements in one file that
// read a kill(target, 0) as "still there".
func oneShotGoneReads(path string, source []byte) ([]string, error) {
	fileSet := token.NewFileSet()
	var file *ast.File
	var err error
	if source != nil {
		file, err = parser.ParseFile(fileSet, path, source, 0)
	} else {
		file, err = parser.ParseFile(fileSet, path, nil, 0)
	}
	if err != nil {
		return nil, err
	}
	var lines []string
	ast.Inspect(file, func(node ast.Node) bool {
		statement, ok := node.(*ast.IfStmt)
		if !ok {
			return true
		}
		killed := map[string]bool{}
		if assign, ok := statement.Init.(*ast.AssignStmt); ok && len(assign.Lhs) == 1 && len(assign.Rhs) == 1 && isKillZero(assign.Rhs[0]) {
			if identifier, ok := assign.Lhs[0].(*ast.Ident); ok {
				killed[identifier.Name] = true
			}
		}
		if readsStillThere(statement.Cond, killed) {
			lines = append(lines, strconv.Itoa(fileSet.Position(statement.Pos()).Line))
		}
		return true
	})
	return lines, nil
}

// readsStillThere reports a condition that holds while the target is still
// present: kill(target, 0) == nil, or !errors.Is(err, ESRCH) of such a read.
func readsStillThere(condition ast.Expr, killed map[string]bool) bool {
	found := false
	ast.Inspect(condition, func(node ast.Node) bool {
		switch expression := node.(type) {
		case *ast.BinaryExpr:
			if expression.Op == token.EQL && (isNil(expression.Y) && isKillRead(expression.X, killed) || isNil(expression.X) && isKillRead(expression.Y, killed)) {
				found = true
			}
		case *ast.UnaryExpr:
			if expression.Op != token.NOT {
				return true
			}
			call, ok := ast.Unparen(expression.X).(*ast.CallExpr)
			if !ok || len(call.Args) != 2 || !isSelector(call.Fun, "Is") || !isSelector(call.Args[1], "ESRCH") {
				return true
			}
			if isKillRead(call.Args[0], killed) {
				found = true
			}
		}
		return !found
	})
	return found
}

func isKillRead(expression ast.Expr, killed map[string]bool) bool {
	if identifier, ok := ast.Unparen(expression).(*ast.Ident); ok {
		return killed[identifier.Name]
	}
	return isKillZero(expression)
}

func isKillZero(expression ast.Expr) bool {
	call, ok := ast.Unparen(expression).(*ast.CallExpr)
	if !ok || len(call.Args) != 2 || !isSelector(call.Fun, "Kill") {
		return false
	}
	literal, ok := call.Args[1].(*ast.BasicLit)
	return ok && literal.Kind == token.INT && literal.Value == "0"
}

func isSelector(expression ast.Expr, name string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == name
}

func isNil(expression ast.Expr) bool {
	identifier, ok := expression.(*ast.Ident)
	return ok && identifier.Name == "nil"
}
