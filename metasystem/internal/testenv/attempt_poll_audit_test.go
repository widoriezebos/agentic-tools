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

// TestNoTestPollsAnotherProcessByAttemptCount: no test, and no test-support
// file, looks at another process's work a fixed number of times and fails
// when the count runs out. A count of attempts is a wall-clock budget in
// disguise: each attempt takes as long as the host makes it, so the count
// expires before a slow process's event on a loaded host, or on a fast one
// whose attempts are quick (TestSetupChildIsEndedWhenTheBinaryIsKilled ran
// fifty starts on a Linux VM inside its custodian's 250ms poll). The test
// waits on the event instead (testenv.Await, bounded only by the test
// binary's deadline), then asserts once.
//
// The audit finds a for loop that both
//   - counts its attempts: a counter it declares or increments and uses for
//     nothing but comparisons, increments and report messages;
//   - fails on running out: an if comparing the counter (==, >= or >) whose
//     body fails, or, for a loop whose condition bounds the counter and which
//     breaks or returns on success, a failure right after the loop (a range
//     over an integer literal is such a loop too);
//   - and observes another process in its own body: it starts or waits for a
//     command, probes, signals or reaps a process, or reads a file system
//     entry another process writes (attemptPollObservations).
//
// A loop that only steps code in this process a counted number of times (a
// deterministic budget, such as a scan that must finish within N passes) is
// not flagged: its count is a property of the code, not of the host. An
// observation hidden behind a helper the loop calls is not seen; the audit
// names the shape where it is written out.
func TestNoTestPollsAnotherProcessByAttemptCount(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	support, err := walltimeSupportFiles(root)
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
		name := entry.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || !strings.HasSuffix(name, ".go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if !strings.HasSuffix(name, "_test.go") && !support[relative] {
			return nil
		}
		lines, err := attemptCountPolls(path, nil)
		if err != nil {
			return err
		}
		for _, line := range lines {
			found = append(found, relative+":"+line)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(found)
	for _, site := range found {
		t.Errorf("%s: a fixed number of attempts bounds a wait on another process; await its event with testenv.Await and assert once", site)
	}
}

// The detector flags attempt-count polls of another process and passes
// searches, deterministic in-process budgets and waits without a count.
func TestAttemptCountPollDetector(t *testing.T) {
	t.Parallel()
	source := `package p

func flagged(t T) {
	for attempt := 1; ; attempt++ { // 4
		if output, err := exec.Command("x").CombinedOutput(); err != nil {
			t.Fatal(output)
		}
		if _, err := os.Stat(home); os.IsNotExist(err) {
			break
		}
		if attempt == 50 {
			t.Fatalf("fifty starts kept %s", home)
		}
	}
	for i := 0; i < 100; i++ { // 15
		if _, state, _ := prober.Probe(pid); state == Dead {
			return
		}
	}
	t.Fatal("still alive")
	tries := 0
	for !gone() { // 22
		tries++
		_ = syscall.Kill(pid, 0)
		if tries > limit {
			t.Fatalf("after %d tries", tries)
		}
	}
	for range 20 { // 29
		if data, _ := os.ReadFile(path); len(data) > 0 {
			return
		}
	}
	t.Error("never written")
	for n := 0; n < max; n++ { // 35
		if _, err := os.Lstat(path); err == nil {
			break
		}
	}
	if !found {
		return errors.New("never appeared")
	}
}

func passed(t T, args []string) {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == "--dir" {
			break
		}
	}
	if dir == "" {
		t.Fatal("no dir")
	}
	for passes := 0; ; passes++ {
		if published, _ := index.Step(budget); published {
			break
		}
		if passes > 10 {
			t.Fatal("never completes")
		}
	}
	for stop := 1; stop <= 3; stop++ {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("stop %d", stop)
		}
		if stop == 1 {
			t.Fatal("first")
		}
	}
	for burst := 0; burst < 64; burst++ {
		if _, state, _ := prober.Probe(pid); state == Dead {
			return
		}
	}
	for i := 0; i < 3; i++ {
		if _, err := os.Stat(paths[i]); err != nil {
			break
		}
	}
	t.Fatal("indexed")
}
`
	lines, err := attemptCountPolls("detector_test.go", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"4", "15", "22", "29", "35"}
	if strings.Join(lines, ",") != strings.Join(want, ",") {
		t.Fatalf("flagged lines = %v, want %v", lines, want)
	}
}

// attemptPollObservations are the calls through which a loop body looks at
// another process: starting or waiting for a command, probing, signalling or
// reaping a process, or reading a file system entry another process writes.
var attemptPollObservations = map[string]bool{
	"Command": true, "CommandContext": true, "CombinedOutput": true, "Output": true, "Run": true, "Start": true,
	"Probe": true, "FixtureSurvivors": true, "Kill": true, "Signal": true, "SignalExact": true, "Wait4": true, "FindProcess": true,
	"Stat": true, "Lstat": true, "ReadFile": true, "ReadDir": true, "Readlink": true, "Open": true,
}

// attemptCountFailures are the calls that end a test or a helper in failure.
var attemptCountFailures = map[string]bool{
	"Fatal": true, "Fatalf": true, "Error": true, "Errorf": true, "Fail": true, "FailNow": true, "New": true,
}

// attemptCountPolls returns the lines of the loops in one file that poll
// another process with a fixed attempt count.
func attemptCountPolls(path string, source []byte) ([]string, error) {
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
		var statements []ast.Stmt
		switch block := node.(type) {
		case *ast.BlockStmt:
			statements = block.List
		case *ast.CaseClause:
			statements = block.Body
		case *ast.CommClause:
			statements = block.Body
		default:
			return true
		}
		for index, statement := range statements {
			failsAfter := index+1 < len(statements) && attemptCountFails(statements[index+1])
			if attemptCountPoll(statement, failsAfter) {
				lines = append(lines, strconv.Itoa(fileSet.Position(statement.Pos()).Line))
			}
		}
		return true
	})
	return lines, nil
}

// attemptCountPoll reports whether statement is a counted poll of another
// process; failsAfter says the statement after it fails.
func attemptCountPoll(statement ast.Stmt, failsAfter bool) bool {
	switch loop := statement.(type) {
	case *ast.RangeStmt:
		literal, ok := ast.Unparen(loop.X).(*ast.BasicLit)
		counted := ok && literal.Kind == token.INT
		if loop.Key != nil {
			if key, ok := loop.Key.(*ast.Ident); ok && key.Name != "_" && attemptCounterUsedOtherwise(loop.Body, key.Name) {
				counted = false
			}
		}
		return counted && failsAfter && attemptLoopExits(loop.Body) && attemptLoopObserves(loop.Body)
	case *ast.ForStmt:
		counter := attemptCounter(loop)
		if counter == "" || attemptCounterUsedOtherwise(loop.Body, counter) || !attemptLoopObserves(loop.Body) {
			return false
		}
		if attemptCountFailsOnExhaustion(loop, counter) {
			return true
		}
		return failsAfter && attemptConditionBounds(loop.Cond, counter) && attemptLoopExits(loop.Body)
	}
	return false
}

// attemptCounter names the loop's counter: the variable its init declares,
// else one its body increments.
func attemptCounter(loop *ast.ForStmt) string {
	if assign, ok := loop.Init.(*ast.AssignStmt); ok && len(assign.Lhs) == 1 {
		if identifier, ok := assign.Lhs[0].(*ast.Ident); ok {
			return identifier.Name
		}
	}
	for _, statement := range loop.Body.List {
		if increment, ok := statement.(*ast.IncDecStmt); ok && increment.Tok == token.INC {
			if identifier, ok := increment.X.(*ast.Ident); ok {
				return identifier.Name
			}
		}
	}
	return ""
}

func isAttemptCounter(expression ast.Expr, counter string) bool {
	identifier, ok := ast.Unparen(expression).(*ast.Ident)
	return ok && identifier.Name == counter
}

// attemptCounterUsedOtherwise reports a use of the counter in the body other
// than a comparison, an increment or a report message: an index, an
// argument or a value, which makes the loop a walk over something.
func attemptCounterUsedOtherwise(body *ast.BlockStmt, counter string) bool {
	used := false
	ast.Inspect(body, func(node ast.Node) bool {
		switch expression := node.(type) {
		case *ast.BinaryExpr:
			if expression.Op.Precedence() == token.EQL.Precedence() && (isAttemptCounter(expression.X, counter) || isAttemptCounter(expression.Y, counter)) {
				return false
			}
		case *ast.IncDecStmt:
			if isAttemptCounter(expression.X, counter) {
				return false
			}
		case *ast.CallExpr:
			if selector, ok := ast.Unparen(expression.Fun).(*ast.SelectorExpr); ok && attemptCountFailures[selector.Sel.Name] {
				return false
			}
		case *ast.Ident:
			if expression.Name == counter {
				used = true
			}
		}
		return !used
	})
	return used
}

// attemptCountFailsOnExhaustion reports an if in the loop body that compares
// the counter against its last attempt and fails. A loop whose own condition
// bounds the counter compares it against another value only to single out
// one attempt, which is no exhaustion.
func attemptCountFailsOnExhaustion(loop *ast.ForStmt, counter string) bool {
	if attemptConditionBounds(loop.Cond, counter) {
		return false
	}
	for _, statement := range loop.Body.List {
		branch, ok := statement.(*ast.IfStmt)
		if !ok {
			continue
		}
		comparison, ok := ast.Unparen(branch.Cond).(*ast.BinaryExpr)
		if !ok || !isAttemptCounter(comparison.X, counter) {
			continue
		}
		if (comparison.Op == token.EQL || comparison.Op == token.GEQ || comparison.Op == token.GTR) && attemptCountFails(branch.Body) {
			return true
		}
	}
	return false
}

// attemptConditionBounds reports a loop condition counter < N or counter <= N
// with N a literal or a named value, not a length.
func attemptConditionBounds(condition ast.Expr, counter string) bool {
	comparison, ok := ast.Unparen(condition).(*ast.BinaryExpr)
	if !ok || (comparison.Op != token.LSS && comparison.Op != token.LEQ) || !isAttemptCounter(comparison.X, counter) {
		return false
	}
	switch ast.Unparen(comparison.Y).(type) {
	case *ast.BasicLit, *ast.Ident:
		return true
	}
	return false
}

// attemptLoopExits reports a break or return under an if at the top of the
// loop body: the loop ends on the observation, not only on the count.
func attemptLoopExits(body *ast.BlockStmt) bool {
	exits := false
	for _, statement := range body.List {
		branch, ok := statement.(*ast.IfStmt)
		if !ok {
			continue
		}
		ast.Inspect(branch.Body, func(node ast.Node) bool {
			switch exit := node.(type) {
			case *ast.FuncLit, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
				return false
			case *ast.BranchStmt:
				if exit.Tok == token.BREAK && exit.Label == nil {
					exits = true
				}
			case *ast.ReturnStmt:
				exits = true
			}
			return !exits
		})
	}
	return exits
}

// attemptLoopObserves reports a call in the loop body, outside a function
// literal, that looks at another process (attemptPollObservations).
func attemptLoopObserves(body *ast.BlockStmt) bool {
	observes := false
	ast.Inspect(body, func(node ast.Node) bool {
		switch expression := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			if selector, ok := ast.Unparen(expression.Fun).(*ast.SelectorExpr); ok && attemptPollObservations[selector.Sel.Name] {
				observes = true
			}
		}
		return !observes
	})
	return observes
}

// attemptCountFails reports a failure call in node, outside a function
// literal.
func attemptCountFails(node ast.Node) bool {
	fails := false
	ast.Inspect(node, func(node ast.Node) bool {
		switch expression := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			if selector, ok := ast.Unparen(expression.Fun).(*ast.SelectorExpr); ok && attemptCountFailures[selector.Sel.Name] {
				fails = true
			}
		}
		return !fails
	})
	return fails
}
