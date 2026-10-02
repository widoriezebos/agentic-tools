package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// A process that stops itself with a process-directed signal
// (kill(getpid(), SIGSTOP), kill(0, SIGTSTP), ...) is not stopped when kill
// returns: the kernel may queue the stop for another thread, and the calling
// goroutine keeps running. TestSupervisorStoppedProcessIsReportedLeftAliveAndResumes
// failed 1-5 in 200 under load because its helper returned and exited 0
// before the stop was delivered (no-flaky-tests, 2026-10-02). The statement
// after a self-directed stop must therefore block on the observable outcome:
// a channel receive (the SIGCONT notification) or a select.

var auditSelfStopKills = map[string]bool{"syscall.Kill": true, "unix.Kill": true}

var auditSelfStopSignals = map[string]bool{"SIGSTOP": true, "SIGTSTP": true, "SIGTTIN": true, "SIGTTOU": true}

var auditSelfPIDCalls = map[string]bool{
	"os.Getpid": true, "syscall.Getpid": true, "unix.Getpid": true,
	"os.Getpgrp": true, "syscall.Getpgrp": true, "unix.Getpgrp": true,
}

// auditSelfStopTarget reports whether a kill target names this process or
// its own process group: getpid(), getpgrp(), -getpgrp(), or 0.
func auditSelfStopTarget(expression ast.Expr) bool {
	switch target := expression.(type) {
	case *ast.BasicLit:
		return target.Kind == token.INT && target.Value == "0"
	case *ast.UnaryExpr:
		return target.Op == token.SUB && auditSelfStopTarget(target.X)
	case *ast.ParenExpr:
		return auditSelfStopTarget(target.X)
	case *ast.CallExpr:
		if len(target.Args) == 1 && ratchetQualifiedName(target.Fun) == "int" {
			return auditSelfStopTarget(target.Args[0])
		}
		return len(target.Args) == 0 && auditSelfPIDCalls[ratchetQualifiedName(target.Fun)]
	}
	return false
}

// auditSelfStopIn reports whether a statement sends itself a stop signal.
func auditSelfStopIn(statement ast.Stmt) bool {
	found := false
	ast.Inspect(statement, func(node ast.Node) bool {
		if _, literal := node.(*ast.FuncLit); literal {
			return false
		}
		switch node.(type) {
		case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause:
			return false // nested statement lists are checked on their own
		}
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 || !auditSelfStopKills[ratchetQualifiedName(call.Fun)] {
			return true
		}
		signal, ok := call.Args[1].(*ast.SelectorExpr)
		if ok && auditSelfStopSignals[signal.Sel.Name] && auditSelfStopTarget(call.Args[0]) {
			found = true
		}
		return true
	})
	return found
}

// auditSelfStopWaits reports whether a statement blocks on an outcome: a
// receive, an assignment from a receive, or a select.
func auditSelfStopWaits(statement ast.Stmt) bool {
	receive := func(expression ast.Expr) bool {
		unary, ok := expression.(*ast.UnaryExpr)
		return ok && unary.Op == token.ARROW
	}
	switch statement := statement.(type) {
	case *ast.SelectStmt:
		return true
	case *ast.ExprStmt:
		return receive(statement.X)
	case *ast.AssignStmt:
		return len(statement.Rhs) == 1 && receive(statement.Rhs[0])
	}
	return false
}

// auditSelfStopLines returns the lines of self-directed stop signals whose
// next statement in the same list does not block on the outcome.
func auditSelfStopLines(fileSet *token.FileSet, file *ast.File) []int {
	var lines []int
	check := func(statements []ast.Stmt) {
		for index, statement := range statements {
			if !auditSelfStopIn(statement) {
				continue
			}
			if index+1 < len(statements) && auditSelfStopWaits(statements[index+1]) {
				continue
			}
			lines = append(lines, fileSet.Position(statement.Pos()).Line)
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.BlockStmt:
			check(node.List)
		case *ast.CaseClause:
			check(node.Body)
		case *ast.CommClause:
			check(node.Body)
		}
		return true
	})
	return lines
}

func TestAuditSelfDirectedStopIsFollowedByAWait(t *testing.T) {
	t.Parallel()
	_, module := verbRatchetRoots(t)
	walkRatchetFiles(t, module, []string{".git", "node_modules", "artifacts", "bin"}, nil, func(path, rel string) {
		if !strings.HasSuffix(rel, ".go") {
			return
		}
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		for _, line := range auditSelfStopLines(fileSet, parsed) {
			t.Errorf("%s:%d stops its own process and does not wait: kill returns before the stop is delivered, so block on the outcome next (a receive on a signal.Notify(SIGCONT) channel, or a select)", rel, line)
		}
	})
}

// The witness sees each self-directed stop without a wait and passes the
// waited and the not-self ones.
func TestAuditSelfStopWitnessSeesEachWay(t *testing.T) {
	t.Parallel()
	for source, want := range map[string]int{
		`package p; import ("os"; "syscall"); func f() { syscall.Kill(os.Getpid(), syscall.SIGSTOP) }`:                                                            1,
		`package p; import ("os"; "syscall"); func f() { _ = syscall.Kill(os.Getpid(), syscall.SIGTSTP); return }`:                                                1,
		`package p; import "golang.org/x/sys/unix"; func f() { unix.Kill(0, unix.SIGSTOP) }`:                                                                      1,
		`package p; import "syscall"; func f() { syscall.Kill(-syscall.Getpgrp(), syscall.SIGSTOP) }`:                                                             1,
		`package p; import ("os"; "syscall"); func f() { if err := syscall.Kill(os.Getpid(), syscall.SIGSTOP); err != nil {} }`:                                   1,
		`package p; import ("os"; "syscall"); func f(c chan int) { syscall.Kill(os.Getpid(), syscall.SIGSTOP); <-c }`:                                             0,
		`package p; import ("os"; "syscall"); func f(c chan int) { syscall.Kill(os.Getpid(), syscall.SIGSTOP); v := <-c; _ = v }`:                                 0,
		`package p; import ("os"; "syscall"); func f() { syscall.Kill(os.Getpid(), syscall.SIGSTOP); select {} }`:                                                 0,
		`package p; import ("os"; "syscall"); func f() { syscall.Kill(os.Getpid(), syscall.SIGTERM) }`:                                                            0,
		`package p; import "syscall"; func f(pid int) { syscall.Kill(pid, syscall.SIGSTOP) }`:                                                                     0,
		`package p; import ("os"; "syscall"); func f(m string, c chan int) { switch m { case "a": syscall.Kill(os.Getpid(), syscall.SIGSTOP); <-c; case "b": } }`: 0,
		`package p; import ("os"; "syscall"); func f(m string) { switch m { case "a": syscall.Kill(os.Getpid(), syscall.SIGSTOP); case "b": } }`:                  1,
	} {
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, "x.go", source, 0)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(auditSelfStopLines(fileSet, parsed)); got != want {
			t.Errorf("%s: %d sites, want %d", source, got, want)
		}
	}
}
