package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestIntentUnitCheckKindFreezeAndExecution(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, settings, kind, budget string }{
		{"absent", "", "legacy", "6"},
		{"explicit legacy", "proof.check-kind=legacy\nproof.check-budget=9\n", "legacy", "9"},
		{"bounded", "proof.check-kind=bounded\n", "bounded", "6"},
		{"invalid kind", "proof.check-kind=unknown\n", "unknown", "6"},
		{"invalid budget", "proof.check-budget=0\n", "legacy", "0"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := declaredCheckBed(t, "proof.cheap=printf cheap\nproof.audits=printf audits\nproof.deadline=15\n"+row.settings)
			code, built, _ := b.work("work", "build", b.id, "kind", "--brief", b.brief("kind.md", "Build.\n"), "--lines", "100")
			if row.kind != "legacy" || row.budget == "0" {
				if code == 0 || !strings.Contains(built.Summary, "proof.check-") || len(b.starter.launched()) != 0 {
					t.Fatalf("unsupported declaration launched work: code=%d summary=%s", code, built.Summary)
				}
				return
			}
			if code != 0 {
				t.Fatalf("legacy build failed: %s", built.Summary)
			}
			plan, err := launch.ReadUnitPlan(resultData(t, built)["plan"].(string))
			if err != nil || plan.Check.Declaration.EffectiveValues()[4] != row.kind || plan.Check.Declaration.EffectiveValues()[5] != row.budget {
				t.Fatal("kind and budget were not frozen with the round")
			}
			run := resultData(t, built)["run"].(string)
			code, result := declaredCheckAt(t, b, b.worktree, "proof", "test", "run", "--unit-run", run)
			if code != 0 || resultData(t, result)["checkOutcome"] != string(launch.CheckPassed) {
				t.Fatalf("legacy execution failed: %s", result.Summary)
			}
			execution := resultData(t, result)["execution"].(string)
			retained, err := launch.ReadCheckResult(execution)
			if err != nil || len(retained.Exits) != 2 || retained.Exits[0].Output != "cheap" || retained.Exits[1].Output != "audits" || retained.Check.Declaration.Values != plan.Check.Declaration.Values {
				t.Fatal("execution lost frozen settings or changed the legacy commands")
			}
			if _, err := os.Stat(filepath.Join(execution, "evidence.json")); err != nil {
				t.Fatal("legacy pass lost its coverage evidence")
			}
		})
	}
}

func TestDeclaredCheckKindAndBudgetChangesHoldBeforeBuild(t *testing.T) {
	t.Parallel()
	for _, setting := range []string{"proof.check-kind=bounded", "proof.check-budget=9"} {
		t.Run(setting, func(t *testing.T) {
			t.Parallel()
			body := "proof.cheap=printf cheap\nproof.audits=true\nproof.deadline=15\n"
			b := declaredCheckBed(t, body)
			b.head = "candidate-commit"
			hook := b.workOwnersHook
			b.workOwnersHook = func(work *intentWorkOwners) {
				hook(work)
				git := work.git
				work.git = func(root string, args ...string) ([]byte, error) {
					if slices.Equal(args, []string{"show", "candidate-commit:metasystem.conf"}) {
						return []byte(body + "proof.full=true # full\n" + setting + "\n"), nil
					}
					return git(root, args...)
				}
			}
			code, _, _ := b.work("work", "build", b.id, "changed-kind", "--brief", b.brief("changed-kind.md", "Build.\n"), "--lines", "100")
			acts := checkActs(t, b)
			if code == 0 || len(b.starter.launched()) != 0 || len(acts) != 1 || acts[0].Status != "proposed" || acts[0].Class != "declaration" {
				t.Fatal("changed kind or budget bypassed declaration admission")
			}
			index, want := 4, "bounded"
			if strings.HasPrefix(setting, "proof.check-budget") {
				index, want = 5, "9"
			}
			if acts[0].AfterDeclaration.Values[index] != want {
				t.Fatal("declaration proposal lost the changed setting")
			}
		})
	}
}
