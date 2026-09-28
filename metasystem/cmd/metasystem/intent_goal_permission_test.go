package main

// goal allow and goal disallow (verbs-match-intent, Wido 2026-09-28): a
// person allows a goal stop-test changes, which the sealed record keeps as
// "- StopSurface: moves" and the Stop decision surface audit reads; anyone
// disallows it. Rule H1: the agent's refusal and the audit's refusal both name
// the command a person runs.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/audit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestIntentGoalAllowAndDisallowStopTestChanges(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, makeQueued)
	bed.lineage = "m1"
	before := bed.publications()

	// An agent session is refused, and told the command a person runs.
	allow := []string{"goal", "allow", bedGoal, "stop-test-changes", "--reason", "the hook entry moved"}
	code, result := bed.runJSON(bed.owners(), allow...)
	bed.expectNoEffect(before, allow, code, result)
	if code != 1 || !strings.Contains(result.Summary, "allowing stop-test changes is a person's act") ||
		!strings.Contains(result.Decision, "metasystem goal allow "+bedGoal+" stop-test-changes --reason TEXT") {
		t.Fatalf("an agent's allow = %d %+v; want a person's-act refusal naming the command", code, result)
	}
	// The word itself is checked first: an unknown permission lists the known
	// ones, and an allow says why.
	for _, args := range [][]string{
		{"goal", "allow", bedGoal, "stop-surface", "--reason", "x"},
		{"goal", "allow", bedGoal, "--reason", "x"},
		{"goal", "allow", bedGoal, "stop-test-changes"},
		{"goal", "disallow", bedGoal, "anything"},
	} {
		code, result := bed.runJSON(bed.owners(), args...)
		bed.expectNoEffect(before, args, code, result)
		if code != 2 {
			t.Fatalf("%v = exit %d %+v; want 2", args, code, result)
		}
		if args[3] == "stop-surface" && !strings.Contains(result.Summary+result.Decision, "stop-test-changes") {
			t.Fatalf("an unknown permission does not list the known ones: %+v", result)
		}
	}
	if bed.goalFile(bedGoal).StopSurfaceMoves {
		t.Fatal("a refused allow recorded the permission")
	}

	// The person at the enrolled terminal allows it.
	bed.lineage = ""
	person := append(append([]string(nil), allow...), "--fixture-human-authority")
	code, result = bed.runJSON(bed.terminalOwners(), person...)
	if code != 0 || result.Outcome != intentConfirmed || bed.publications() != before+1 {
		t.Fatalf("the person's allow = %d %+v", code, result)
	}
	file := bed.goalFile(bedGoal)
	if !file.StopSurfaceMoves || !strings.HasPrefix(file.History[len(file.History)-1].Reason, "Allowed: stop-test-changes why=the hook entry moved") {
		t.Fatalf("the allow did not land as StopSurface: moves with its reason:\n%s", goal.RenderFile(file))
	}
	code, stdout, stderr := bed.run(bed.owners(), "goal", "show", bedGoal)
	if code != 0 || !strings.Contains(stdout, "Allowed: stop-test changes") {
		t.Fatalf("goal show does not say what the goal is allowed: %d %q %q", code, stdout, stderr)
	}

	// Anyone disallows it.
	bed.lineage = "m1"
	code, result = bed.runJSON(bed.owners(), "goal", "disallow", bedGoal, "stop-test-changes")
	if code != 0 || result.Outcome != intentConfirmed || bed.goalFile(bedGoal).StopSurfaceMoves {
		t.Fatalf("an agent's disallow = %d %+v", code, result)
	}
	if code, stdout, _ = bed.run(bed.owners(), "goal", "show", bedGoal); code != 0 || strings.Contains(stdout, "Allowed:") {
		t.Fatalf("goal show still lists a permission after the disallow: %q", stdout)
	}
}

// The audit that guards the Stop decision surface admits a declared move once
// a person has allowed the goal, reading the record goal allow wrote, and
// refuses it before, naming goal allow. Both seams are real: the goal record
// comes from the owner, the audit runs on a real Git checkout.
func TestStopSurfaceAuditAdmitsADeclarationAfterGoalAllow(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, makeQueued)
	path := "plans/goals/" + bedGoal + ".md"
	unallowed := append([]byte(nil), bed.repo.commit(bed.repo.accepted).files[path]...)
	code, result := bed.runJSON(bed.terminalOwners(), "goal", "allow", bedGoal, "stop-test-changes", "--reason", "the hook entry moved", "--fixture-human-authority")
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("allow = %d %+v", code, result)
	}
	allowed := bed.repo.commit(bed.repo.accepted).files[path]
	if !strings.Contains(string(allowed), "\n- StopSurface: moves\n") {
		t.Fatalf("the accepted record does not carry the allowance:\n%s", allowed)
	}

	root := t.TempDir()
	write := func(relative string, content []byte) {
		t.Helper()
		absolute := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		command.Env = environWithoutGitSteeringCLI()
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	assertion := "want := Verdict{ShouldBlock: true}"
	write("go.mod", []byte("module fixture\n\ngo 1.25\n"))
	write("a_test.go", []byte("package fixture\n\nfunc TestFixture() {\n\t"+assertion+"\n}\n"))
	write(path, unallowed)
	git("init", "-q")
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	write("a_test.go", []byte("package fixture\n\nfunc TestFixture() {\n}\n"))

	options := audit.StopSurfaceOptions{GoalRecord: goal.StopSurfaceGoalReader}
	_, err := audit.DeclareStopDecisionSurface(root, options, bedGoal, "the hook entry moved")
	if err == nil || !strings.Contains(err.Error(), "goal "+bedGoal+" is not allowed stop-test changes; a person runs: metasystem goal allow "+bedGoal+" stop-test-changes --reason") {
		t.Fatalf("a declaration before the allow = %v; want the refusal naming goal allow", err)
	}

	write(path, allowed)
	declaration, err := audit.DeclareStopDecisionSurface(root, options, bedGoal, "the hook entry moved")
	if err != nil {
		t.Fatalf("a declaration after the allow was refused: %v", err)
	}
	if !strings.HasPrefix(declaration, "docs/stop-decision-moves/"+bedGoal+"-") {
		t.Fatalf("declaration path = %q", declaration)
	}
	audited, err := audit.AuditStopDecisionSurface(root, options)
	if err != nil || audited.Refused() || len(audited.Moved) != 1 || audited.Moved[0].Goal != bedGoal {
		t.Fatalf("the audit after the allow = %+v %v; want one admitted move", audited, err)
	}
}
