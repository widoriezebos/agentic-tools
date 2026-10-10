package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// writeImpactPlanResult makes a command fake answer with the plan protocol.
func writeImpactPlanResult(t *testing.T, command *exec.Cmd, text string) {
	t.Helper()
	plan := plain.ImpactPlan{Selections: []string{}}
	for _, line := range strings.Split(text, "\n") {
		if base, ok := strings.CutPrefix(line, "plan: base "); ok {
			plan.Base, plan.Subject, _ = strings.Cut(base, " (")
			plan.Subject = strings.TrimSuffix(plan.Subject, ")")
		}
		if selection, ok := strings.CutPrefix(line, "selection: "); ok {
			plan.Selections = append(plan.Selections, selection)
		}
	}
	if err := verbresult.Write(command.Stdout, verbresult.FromError("test impact", 0, nil, plan)); err != nil {
		t.Fatal(err)
	}
	ended := exec.Command("/usr/bin/true")
	if err := ended.Run(); err != nil {
		t.Fatal(err)
	}
	command.ProcessState = ended.ProcessState
}

// Git is the adapter boundary: its committed base and working files select the plan.
func TestTestImpactPlanJSONMatchesTextGitAdapter(t *testing.T) {
	t.Parallel()
	root, base := impactGitAdapterBed(t)
	impactWrite(t, root, "internal/launch/value.go", "package launch\nconst Value = false\n")
	code, text, problem := impactPublic(t, root, "--base", base, "--plan")
	if code != 0 {
		t.Fatalf("text plan: exit=%d stderr=%s", code, problem)
	}
	var first string
	for range 2 {
		code, out, problem := impactPublic(t, root, "--base", base, "--plan", "--json")
		result, err := verbresult.Read([]byte(out), "test impact", code, problem)
		if err != nil || code != 0 || result.Outcome != verbresult.Confirmed {
			t.Fatalf("JSON plan: exit=%d stdout=%s stderr=%s error=%v", code, out, problem, err)
		}
		var plan plain.ImpactPlan
		if err := result.DecodeData(&plan); err != nil || plan.Text() != text {
			t.Fatalf("plan differs: %+v error=%v text=%q", plan, err, text)
		}
		if first != "" && first != out {
			t.Fatalf("plan ordering changed: %q / %q", first, out)
		}
		first = out
	}
	// The real executable must produce the same single envelope without running tests.
	command := exec.Command(commandTestExecutable(t), "test", "impact", "--root", root, "--base", base, "--plan", "--json")
	command.Env = fixtureCommandEnvironment(t)
	command.Dir = root
	var problemBuffer bytes.Buffer
	command.Stderr = &problemBuffer
	result, err := verbresult.Run(command, "test impact")
	var plan plain.ImpactPlan
	if err != nil || result.Outcome != verbresult.Confirmed || result.DecodeData(&plan) != nil || plan.Text() != text || strings.Contains(problemBuffer.String(), "landing group ") {
		t.Fatalf("executable plan: %+v error=%v stderr=%s", result, err, &problemBuffer)
	}
}

func TestTestImpactPlanJSONFailureEnvelope(t *testing.T) {
	t.Parallel()
	root := impactAdapterBed(t)
	code, out, problem := impactPublic(t, root, "--base", "", "--plan", "--json")
	result, err := verbresult.Read([]byte(out), "test impact", code, problem)
	if err != nil || code != 2 || result.Outcome != verbresult.Refused || len(result.Data) != 0 || !strings.Contains(problem, "--base") {
		t.Fatalf("missing base: exit=%d result=%+v error=%v stderr=%s", code, result, err, problem)
	}
}
