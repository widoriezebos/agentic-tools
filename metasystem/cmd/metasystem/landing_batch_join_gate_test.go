package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

// The landing rehearsal sat in exec.Cmd.Wait for 53 minutes because the join
// gate ran its steps with no deadline. These cases pin the two ways a step can
// outlive its caller: the step itself never exits, and the step exits while a
// descendant keeps its output pipes open.

func TestBoundedGateCommandKillsAStepThatOutlivesItsBound(t *testing.T) {
	t.Parallel()
	bound := 300 * time.Millisecond
	started := time.Now()
	outcome := runBoundedGateCommand(t.TempDir(), os.Environ(), []string{"bash", "-c", "echo working; sleep 600"}, bound, time.Second)
	elapsed := time.Since(started)
	if !outcome.TimedOut {
		t.Fatalf("a step that sleeps 600s past a %s bound must report TimedOut, got %+v", bound, outcome)
	}
	if outcome.ExitCode == 0 {
		t.Fatalf("a timed out step must not report a passing exit code, got %d", outcome.ExitCode)
	}
	if !strings.Contains(string(outcome.Output), "working") {
		t.Fatalf("a timed out step must still carry what it printed, got %q", outcome.Output)
	}
	if elapsed > 30*bound {
		t.Fatalf("the bound did not stop the wait: returned after %s for a %s bound", elapsed, bound)
	}
}

func TestBoundedGateCommandDoesNotWaitOnADescendantHoldingThePipes(t *testing.T) {
	t.Parallel()
	grace := 300 * time.Millisecond
	started := time.Now()
	// bash exits at once; the backgrounded sleep inherits the output pipes, so
	// CombinedOutput blocks on the copy goroutines and never on the child.
	outcome := runBoundedGateCommand(t.TempDir(), os.Environ(), []string{"bash", "-c", "sleep 600 & echo done"}, 10*time.Minute, grace)
	elapsed := time.Since(started)
	if outcome.TimedOut {
		t.Fatalf("the step exited well inside its bound, so TimedOut must be false, got %+v", outcome)
	}
	if elapsed > 60*grace {
		t.Fatalf("a descendant holding the pipes blocked the gate: returned after %s for a %s grace", elapsed, grace)
	}
	if outcome.ExitCode != 0 {
		t.Fatalf("the step itself exited 0, so its verdict must survive the leak, got exit %d err %v", outcome.ExitCode, outcome.Err)
	}
	if !outcome.Leaked {
		t.Fatalf("a descendant outliving the step must be reported as Leaked, got %+v", outcome)
	}
}

func TestBoundedGateCommandKeepsTheStepsOwnVerdict(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name     string
		script   string
		exitCode int
		fails    bool
	}{
		{name: "passing step", script: "echo green", exitCode: 0, fails: false},
		{name: "failing step", script: "echo red >&2; exit 7", exitCode: 7, fails: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			outcome := runBoundedGateCommand(t.TempDir(), os.Environ(), []string{"bash", "-c", testCase.script}, time.Minute, time.Second)
			if outcome.TimedOut || outcome.Leaked {
				t.Fatalf("a prompt step must neither time out nor leak, got %+v", outcome)
			}
			if outcome.ExitCode != testCase.exitCode {
				t.Fatalf("exit code: want %d, got %d", testCase.exitCode, outcome.ExitCode)
			}
			if testCase.fails != (outcome.Err != nil) {
				t.Fatalf("failure reporting: want failure=%v, got err=%v", testCase.fails, outcome.Err)
			}
			if strings.TrimSpace(string(outcome.Output)) == "" {
				t.Fatalf("the step printed a line, so the outcome must carry it, got %q", outcome.Output)
			}
		})
	}
}

// A single fixed ceiling cannot serve this gate. A changed package declares
// -timeout 900s, while dependent packages, the aggregate step and the batchtest
// step all declare 40m. A ceiling generous enough for the long steps stops
// bounding the short ones, and one tuned to the short ones kills a long step
// that is still legitimately running. These cases pin the derivation instead.

func TestGateStepBoundIsDerivedFromTheStepsOwnTimeout(t *testing.T) {
	t.Parallel()
	margin := 5 * time.Minute
	fallback := 20 * time.Minute
	for _, testcase := range []struct {
		name string
		args []string
		want time.Duration
	}{
		{name: "separate word", args: []string{"go", "test", "-timeout", "40m", "./pkg"}, want: 45 * time.Minute},
		{name: "seconds", args: []string{"go", "test", "-timeout", "900s", "./pkg"}, want: 20 * time.Minute},
		{name: "equals form", args: []string{"go", "test", "-timeout=40m", "./pkg"}, want: 45 * time.Minute},
		{name: "double dash", args: []string{"go", "test", "--timeout=30m", "./pkg"}, want: 35 * time.Minute},
		{name: "no timeout falls back", args: []string{"go", "build", "./..."}, want: fallback},
		{name: "unparseable falls back", args: []string{"go", "test", "-timeout", "banana", "./pkg"}, want: fallback},
		{name: "trailing flag falls back", args: []string{"go", "test", "-timeout"}, want: fallback},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			if got := boundForGateStep(testcase.args, margin, fallback); got != testcase.want {
				t.Fatalf("bound for %v = %s, want %s", testcase.args, got, testcase.want)
			}
		})
	}
}

// The rule that matters is not any particular number: no step the production
// gate builds may be cut short by its own backstop. Driving the real step
// builders keeps this true when a step's timeout changes, and turns red the
// moment a fixed ceiling is reinstated below one of them.
func TestEveryProductionGateStepOutlivesItsOwnTimeout(t *testing.T) {
	t.Parallel()
	selection := batch.UnitPackages{
		Tree:       "tree",
		ModulePath: "example.com/m",
		Changed:    []string{"./cmd/metasystem"},
		Dependents: []string{"./internal/landing"},
	}
	steps := append(batch.JoinGatePackageSteps(selection), batch.AggregateUnitGateSteps(selection)...)
	if len(steps) < 4 {
		t.Fatalf("expected the production builders to yield package, dependent, batchtest and aggregate steps, got %d", len(steps))
	}
	checked := 0
	for _, step := range steps {
		declared, ok := declaredGateTimeout(step.Args)
		if !ok {
			continue
		}
		checked++
		bound := boundForGateStep(step.Args, joinGateStepMargin, joinGateStepFallbackBound)
		if bound <= declared {
			t.Fatalf("step %q declares -timeout %s but its gate bound is %s; the backstop would kill a step that was still running", step.Name, declared, bound)
		}
	}
	if checked == 0 {
		t.Fatal("no production gate step declared a timeout, so this test proved nothing")
	}
}

func declaredGateTimeout(args []string) (time.Duration, bool) {
	for i, arg := range args {
		if arg != "-timeout" || i+1 >= len(args) {
			continue
		}
		parsed, err := time.ParseDuration(args[i+1])
		if err != nil {
			return 0, false
		}
		return parsed, true
	}
	return 0, false
}
