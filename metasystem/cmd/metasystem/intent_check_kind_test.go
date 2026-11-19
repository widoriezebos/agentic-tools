package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
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
				if code == 0 || !strings.Contains(built.Summary, "check") || len(b.starter.launched()) != 0 {
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
			if setting == "proof.check-kind=bounded" {
				if code == 0 || len(b.starter.launched()) != 0 || len(acts) != 0 {
					t.Fatal("unavailable kind launched work or proposed approval")
				}
				return
			}
			if code == 0 || len(b.starter.launched()) != 0 || len(acts) != 1 || acts[0].Status != "proposed" || acts[0].Class != "declaration" {
				t.Fatal("changed kind or budget bypassed declaration admission")
			}
			if acts[0].AfterDeclaration.Values[5] != "9" {
				t.Fatal("declaration proposal lost the changed setting")
			}
		})
	}
}

func TestBoundedBuildRefusesBeforeSavingAndLegacyRecovers(t *testing.T) {
	t.Parallel()
	for _, inherited := range []bool{false, true} {
		t.Run(map[bool]string{false: "changed", true: "inherited"}[inherited], func(t *testing.T) {
			t.Parallel()
			body := "proof.cheap=printf cheap\nproof.audits=printf audits\nproof.deadline=15\nproof.full=true # full\n"
			b := declaredCheckBed(t, strings.Replace(body, "proof.full=true # full\n", "", 1))
			b.head = "candidate-commit"
			bounded := true
			hook := b.workOwnersHook
			b.workOwnersHook = func(work *intentWorkOwners) {
				hook(work)
				git := work.git
				work.git = func(root string, args ...string) ([]byte, error) {
					if len(args) == 2 && args[0] == "show" && (args[1] == "candidate-commit:metasystem.conf" || inherited && args[1] == "base-commit:metasystem.conf") {
						if bounded {
							return []byte(body + "proof.check-kind=bounded\n"), nil
						}
						return []byte(body + "proof.check-kind=legacy\n"), nil
					}
					return git(root, args...)
				}
			}
			brief := b.brief("recover-kind.md", "Build.\n")
			code, result, _ := b.work("work", "build", b.id, "recover-kind", "--brief", brief, "--lines", "100")
			if code == 0 || len(b.starter.launched()) != 0 {
				t.Fatal("bounded declaration launched a builder")
			}
			for _, pattern := range []string{
				filepath.Join(b.root(), "process", "declaration-*.json"),
				filepath.Join(b.root(), "process", "acts", "*.json"),
				filepath.Join(b.root(), "artifacts", "agents", "channel", "questions", "*.json"),
				filepath.Join(b.unitRoot, "*", "round-*", "plan.json"),
			} {
				matches, err := filepath.Glob(pattern)
				if err != nil || len(matches) != 0 {
					t.Errorf("bounded refusal saved state matching %s", pattern)
				}
			}
			if !strings.Contains(result.Summary, "bounded") {
				t.Fatal("refusal did not explain the unavailable kind")
			}
			bounded = false
			code, built, _ := b.work("work", "build", b.id, "recover-kind", "--brief", brief, "--lines", "100")
			if code != 0 || len(b.starter.launched()) == 0 {
				t.Fatalf("legacy did not recover without repair: %s", built.Summary)
			}
			run := resultData(t, built)["run"].(string)
			code, checked := declaredCheckAt(t, b, b.worktree, "proof", "test", "run", "--unit-run", run)
			if code != 0 || resultData(t, checked)["checkOutcome"] != string(launch.CheckPassed) {
				t.Fatal("recovered legacy check did not pass")
			}
		})
	}
}

func TestBoundedCarryFlagsUnitAndContinues(t *testing.T) {
	t.Parallel()
	for _, inherited := range []bool{false, true} {
		t.Run(map[bool]string{false: "changed", true: "inherited"}[inherited], func(t *testing.T) {
			t.Parallel()
			declaration := "proof.cheap=true\nproof.audits=true\nproof.deadline=15\nproof.check-kind=legacy\n"
			bounded := strings.Replace(declaration, "=legacy", "=bounded", 1)
			if inherited {
				declaration = bounded
			}
			b, owners, main, _ := declaredCarryBed(t, declaration, false, bounded+"proof.full=printf full-suite\n")
			// The second unit returns to the inherited declaration.
			writeUnitCarryFile(t, filepath.Join(b.worktree, "metasystem.conf"), declaration+"proof.full=printf full-suite\n")
			writeUnitCarryFile(t, filepath.Join(b.worktree, "second.go"), "second change\n")
			connectionGit(t, b.worktree, "add", "metasystem.conf", "second.go")
			unit, err := branch.CommitStaged(branch.CommitRequest{Repo: b.worktree, Remote: "origin", EndpointTip: main, GoalID: b.id, Unit: "u2", Kind: branch.Unit, OpID: "unit-two", CheckClaim: func() error { return nil }})
			if err != nil {
				t.Fatal(err)
			}
			digest, err := branch.UnitDigest(b.worktree, unit)
			if err != nil {
				t.Fatal(err)
			}
			readPath := "metasystem/records/misc/read-two.md"
			writeUnitCarryFile(t, filepath.Join(b.worktree, readPath), "Read "+unit+" change "+digest+"\nVerdict: LAND\n")
			_, _, err = branch.CommitRead(branch.CommitReadRequest{Repo: b.worktree, Remote: "origin", EndpointTip: main, GoalID: b.id, Unit: "u2", OpID: "read-two", ReaderRecord: readPath, CheckClaim: func() error { return nil }, GateRunID: "old-static-gate", GateTree: connectionGit(t, b.worktree, "rev-parse", unit+"^{tree}")})
			if err != nil {
				t.Fatal(err)
			}
			connectionGit(t, b.worktree, "push", "-q", "origin", "HEAD:goal/"+b.id)
			code, result := b.runJSON(owners, "work", "rebase", b.id)
			data, err := json.Marshal(result.Data)
			var rebased branch.RebaseResult
			if err != nil || json.Unmarshal(data, &rebased) != nil {
				t.Fatal("cannot read carry result")
			}
			if code != 0 || !slices.Contains(rebased.NeedsReview, "u1") || !strings.Contains(rebased.ReviewReasons["u1"], "bounded") {
				t.Fatalf("unavailable first unit stopped carry: code=%d summary=%s", code, result.Summary)
			}
			if inherited {
				if !slices.Equal(rebased.NeedsReview, []string{"u1", "u2"}) || !strings.Contains(rebased.ReviewReasons["u2"], "bounded") {
					t.Fatal("carry did not flag the second unavailable unit")
				}
			} else if !slices.Equal(rebased.Carried, []string{"u2"}) {
				t.Fatal("carry did not run the second unit's legacy check")
			}
			if acts := checkActs(t, b); len(acts) != 0 {
				t.Fatal("carry proposed approval for an unavailable kind")
			}
			snapshots, err := filepath.Glob(filepath.Join(b.root(), "artifacts", "unit-checks", "carry", "*", "subject.json"))
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range snapshots {
				data, err := os.ReadFile(path)
				var check launch.UnitCheck
				if err != nil || json.Unmarshal(data, &check) != nil {
					t.Fatal("cannot read carry snapshot")
				}
				if check.Declaration != nil && check.Declaration.EffectiveValues()[4] == "bounded" {
					t.Fatal("carry saved an unavailable declaration")
				}
			}
		})
	}
}
