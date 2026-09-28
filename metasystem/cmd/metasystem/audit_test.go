package main

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/audit"
)

func TestAuditStopDecisionSurfaceVerb(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rootArg := root + string(os.PathSeparator) + "."
	const base = "selected-base"
	added := audit.StopSurfaceResult{
		Base: base,
		Added: []audit.StopSurfaceLine{{
			File: "a_test.go", Line: "added := Verdict{BlockSource: source}",
		}},
	}
	removed := audit.StopSurfaceResult{
		Base: base,
		Removed: []audit.StopSurfaceLine{{
			File: "a_test.go", Line: "base := Verdict{ShouldBlock: true}",
		}},
	}
	auditResponses := []struct {
		result audit.StopSurfaceResult
		err    error
	}{{result: added}, {result: added}, {result: removed}, {err: errors.New("stop surface read failed")}}
	auditCalls, declareCalls := 0, 0
	dependencies := stopDecisionSurfaceDependencies{
		audit: func(gotRoot string, options audit.StopSurfaceOptions) (audit.StopSurfaceResult, error) {
			if gotRoot != root || options.Base != base || options.GoalRecord == nil {
				t.Errorf("audit forwarding: root %q, base %q, goal reader present %t", gotRoot, options.Base, options.GoalRecord != nil)
			}
			if auditCalls >= len(auditResponses) {
				t.Fatalf("unexpected audit call %d", auditCalls+1)
			}
			response := auditResponses[auditCalls]
			auditCalls++
			return response.result, response.err
		},
		declare: func(_ string, _ audit.StopSurfaceOptions, _, _ string) (string, error) {
			declareCalls++
			t.Fatal("audit mode called declaration")
			return "", nil
		},
	}
	invoke := func(args ...string) (int, string, string) {
		return captureCommandOutput(t, true, true, func() int {
			return runAuditStopDecisionSurfaceWith(args, dependencies)
		})
	}
	args := []string{"--root", rootArg, "--base", base}

	code, stdout, stderr := invoke(args...)
	if code != 0 || stderr != "" || auditCalls != 1 || declareCalls != 0 || !strings.Contains(stdout, "added: a_test.go: added := Verdict{BlockSource: source}") ||
		!strings.Contains(stdout, "stop decision surface: base selected-base; added 1, moved 0, removed 0") {
		t.Fatalf("additive verb = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}

	code, stdout, stderr = invoke(append(args, "--json")...)
	if code != 0 || stderr != "" || auditCalls != 2 || declareCalls != 0 {
		t.Fatalf("JSON verb = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &document); err != nil {
		t.Fatalf("decode JSON output %q: %v", stdout, err)
	}
	for _, field := range []string{"base", "added", "moved", "removed", "problems"} {
		if _, ok := document[field]; !ok {
			t.Errorf("JSON output lacks %q: %s", field, stdout)
		}
	}

	code, stdout, stderr = invoke(args...)
	if code != 1 || auditCalls != 3 || declareCalls != 0 || !strings.Contains(stdout, "removed 1") ||
		!strings.Contains(stderr, "removed: a_test.go: base := Verdict{ShouldBlock: true}") ||
		!strings.Contains(stderr, "--declare --goal") ||
		!strings.Contains(stderr, "a person allows the move with metasystem goal allow <goal-id> stop-test-changes --reason <text>") {
		t.Fatalf("refusing verb = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}

	for _, invalid := range [][]string{
		{"--root", rootArg, "extra"},
		{"--root", rootArg, "--declare", "--json"},
		{"--root", rootArg, "--goal", "move-stop"},
	} {
		code, _, stderr = invoke(invalid...)
		if code != 2 || !strings.Contains(stderr, "usage: metasystem internal audit stop-decision-surface") || auditCalls != 3 || declareCalls != 0 {
			t.Fatalf("invalid invocation %q = code %d, stderr %q, audit calls %d, declaration calls %d", invalid, code, stderr, auditCalls, declareCalls)
		}
	}

	code, stdout, stderr = invoke(args...)
	if code != 1 || stdout != "" || !strings.Contains(stderr, "stop surface read failed") {
		t.Fatalf("audit error = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if auditCalls != 4 || declareCalls != 0 {
		t.Fatalf("call counts: audit %d, declaration %d; want 4, 0", auditCalls, declareCalls)
	}
	code, stdout, stderr = captureCommandOutput(t, true, true, func() int {
		return runAuditStopDecisionSurfaceWith(args, stopDecisionSurfaceDependencies{})
	})
	if code != 1 || stdout != "" || !strings.Contains(stderr, "dependencies are required") {
		t.Fatalf("missing dependencies = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestAuditStopDecisionSurfaceRefusesRecordTheGoalParserRejects(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rootArg := root + string(os.PathSeparator) + "."
	const goalID = "move-stop"
	const declarationPath = "docs/stop-decision-moves/move-stop.txt"
	parserError := errors.New(`STOP_SURFACE_GOAL_REFUSED: unknown field "Unknown"`)
	auditCalls, declareCalls := 0, 0
	dependencies := stopDecisionSurfaceDependencies{
		declare: func(gotRoot string, options audit.StopSurfaceOptions, gotGoal, reason string) (string, error) {
			if gotRoot != root || options.Base != "" || options.GoalRecord == nil || gotGoal != goalID || reason != "r" {
				t.Errorf("declaration forwarding: root %q, base %q, goal reader present %t, goal %q, reason %q", gotRoot, options.Base, options.GoalRecord != nil, gotGoal, reason)
			}
			declareCalls++
			switch declareCalls {
			case 1:
				return declarationPath, nil
			case 2:
				return "", parserError
			default:
				t.Fatalf("unexpected declaration call %d", declareCalls)
				return "", nil
			}
		},
		audit: func(gotRoot string, options audit.StopSurfaceOptions) (audit.StopSurfaceResult, error) {
			if gotRoot != root || options.Base != "" || options.GoalRecord == nil {
				t.Errorf("audit forwarding: root %q, base %q, goal reader present %t", gotRoot, options.Base, options.GoalRecord != nil)
			}
			auditCalls++
			switch auditCalls {
			case 1:
				return audit.StopSurfaceResult{
					Base:     "base-commit",
					Removed:  []audit.StopSurfaceLine{{File: "a_test.go", Line: "want := Verdict{ShouldBlock: true}"}},
					Problems: []string{parserError.Error()},
				}, nil
			case 2:
				return audit.StopSurfaceResult{}, parserError
			default:
				t.Fatalf("unexpected audit call %d", auditCalls)
				return audit.StopSurfaceResult{}, nil
			}
		},
	}
	invoke := func(args ...string) (int, string, string) {
		return captureCommandOutput(t, true, true, func() int {
			return runAuditStopDecisionSurfaceWith(args, dependencies)
		})
	}
	declareArgs := []string{"--root", rootArg, "--declare", "--goal", goalID, "--reason", "r"}

	code, stdout, stderr := invoke(declareArgs...)
	if code != 0 || stderr != "" || stdout != declarationPath+"\n" || declareCalls != 1 || auditCalls != 0 {
		t.Fatalf("canonical declaration = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = invoke(declareArgs...)
	if code != 1 || stdout != "" || !strings.Contains(stderr, "STOP_SURFACE_GOAL_REFUSED") ||
		!strings.Contains(stderr, `unknown field "Unknown"`) || declareCalls != 2 || auditCalls != 0 {
		t.Fatalf("parser-rejected declaration = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}

	code, stdout, stderr = invoke("--root", rootArg)
	if code != 1 || !strings.Contains(stdout, "moved 0, removed 1") ||
		!strings.Contains(stderr, "STOP_SURFACE_GOAL_REFUSED") || !strings.Contains(stderr, `unknown field "Unknown"`) ||
		!strings.Contains(stderr, "removed: a_test.go: want := Verdict{ShouldBlock: true}") ||
		!strings.Contains(stderr, "--declare --goal") || auditCalls != 1 || declareCalls != 2 {
		t.Fatalf("parser-rejected audit = code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = invoke("--root", rootArg)
	if code != 1 || stdout != "" || !strings.Contains(stderr, "STOP_SURFACE_GOAL_REFUSED") ||
		!strings.Contains(stderr, `unknown field "Unknown"`) || auditCalls != 2 || declareCalls != 2 {
		t.Fatalf("audit error = code %d, stdout %q, stderr %q, audit calls %d, declaration calls %d", code, stdout, stderr, auditCalls, declareCalls)
	}
}
