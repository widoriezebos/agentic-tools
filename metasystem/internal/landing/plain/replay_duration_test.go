package plain

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestReplayAndFlakeRepeatWaitForLongCheck(t *testing.T) {
	t.Parallel()
	// Execute the default runner with a detached launcher that completes after
	// twenty virtual minutes. Keeping its actual body makes a timer added to
	// that runner observable without waiting for a real process for minutes.
	runner := longCheckRunner(t)
	for _, flake := range []bool{false, true} {
		t.Run(fmt.Sprintf("flake=%t", flake), func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			now := bedNow
			b.seams.Now = func() time.Time { return now }
			if !flake {
				b.seams.Judge = nil
			}
			replays := 0
			b.seams.Command = func(command *exec.Cmd) error {
				only := ""
				for _, entry := range command.Env {
					if strings.HasPrefix(entry, "LANDING_ONLY=") {
						only = strings.TrimPrefix(entry, "LANDING_ONLY=")
					}
				}
				if only != "" {
					replays++
					child := exec.Command(runner, "-test.run=^TestCheckRunner$", "-test.timeout=30m")
					output, err := child.CombinedOutput()
					if err != nil {
						return fmt.Errorf("isolated runner: %w: %s", err, output)
					}
					now = now.Add(20 * time.Minute)
				}
				return command.Run()
			}
			command := failedEvidenceReport()
			if flake {
				command = "if [ -z \"$LANDING_ONLY\" ]; then " + command + "; fi; " + passingEvidenceReport()
			}
			got := b.run(command)
			if flake {
				if got.Result != Green || replays != 2 || len(b.records) != 2 {
					t.Fatalf("long flake repeats lost their completed result: %+v cause=%+v replays=%d records=%d", got, got.Cause, replays, len(b.records))
				}
			} else if got.Result != Red || got.Cause == nil || got.Cause.Kind != "main" || replays != 1 || strings.Contains(got.Reason, "did not complete") {
				t.Fatalf("long classification replay lost its attribution: %+v cause=%+v replays=%d", got, got.Cause, replays)
			}
		})
	}
}

func longCheckRunner(t *testing.T) string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "prove.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var runner *ast.FuncLit
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "runCheck" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			assignment, ok := node.(*ast.AssignStmt)
			if ok && len(assignment.Lhs) == 1 && len(assignment.Rhs) == 1 {
				name, named := assignment.Lhs[0].(*ast.Ident)
				literal, callable := assignment.Rhs[0].(*ast.FuncLit)
				if named && callable && name.Name == "run" {
					runner = literal
				}
			}
			return true
		})
	}
	if runner == nil {
		t.Fatal("the default check runner was not found")
	}
	// Only substitute the detached process effect; timers retain Go's virtual clock.
	ast.Inspect(runner, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if ok {
			name, ok := selector.X.(*ast.Ident)
			if ok && name.Name == "gaterun" {
				name.Name = "stub"
			}
		}
		return true
	})
	var body bytes.Buffer
	if err := format.Node(&body, fset, runner); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	stubDir := filepath.Join(dir, "stub")
	if err := os.Mkdir(stubDir, 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module longcheck\n\ngo 1.27\n",
		"stub/launch.go": `package stub
import "time"
type DetachedLaunch struct { Argv, Env []string; Dir, Log string; Completed chan<- error }
func LaunchDetached(launch DetachedLaunch) (int64, error) {
    go func() { time.Sleep(20*time.Minute); launch.Completed <- nil }()
    return 1, nil
}
`,
		"runner_test.go": `package longcheck
import (
    "errors"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "testing"
    "testing/synctest"
    "time"
    "longcheck/stub"
)
var config = struct {
    CommittedLookup func(string, string) (string, bool, error)
    SettingValueProblem func(string, string) error
}{func(string, string) (string, bool, error) { return "1", true, nil }, func(string, string) error { return nil }}
var syscall = struct { Kill func(int, int) error; SIGKILL int }{func(int, int) error { return nil }, 9}
// The launcher seam replaces the detached process effect; the runner still owns its wait.
var seams = struct { launchCheck func(*exec.Cmd, []string, string, chan<- error) error }{
    func(command *exec.Cmd, env []string, log string, done chan<- error) error {
        _, err := stub.LaunchDetached(stub.DetachedLaunch{Argv: command.Args, Dir: command.Dir, Env: env, Log: log, Completed: done})
        return err
    },
}
var env []string
var dir, logPath, only = ".", "unused", "u/a"
var _ = []any{errors.Join, fmt.Errorf, os.ErrDeadlineExceeded, filepath.Join, strconv.ParseInt, time.After}
func TestCheckRunner(t *testing.T) {
    t.Parallel()
    synctest.Test(t, func(t *testing.T) {
        start := time.Now()
        run := ` + body.String() + `
        if err := run(exec.Command("unused")); err != nil {
            t.Fatalf("long isolated check was recorded lost-process: %v", err)
        }
        if elapsed := time.Since(start); elapsed != 20*time.Minute {
            t.Fatalf("runner returned before the check completed: %s", elapsed)
        }
    })
}
`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	binary := filepath.Join(dir, "runner")
	build := testenv.Go("test", "-p", "4", "-timeout", "30m", "-c", "-o", binary, ".")
	build.Dir = dir
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compile isolated runner: %v\n%s", err, output)
	}
	return binary
}
