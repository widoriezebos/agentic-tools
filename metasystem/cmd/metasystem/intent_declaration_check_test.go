package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func assertDeclaredRepairNameRefusal(t *testing.T, code int, result intentResult, command []string) {
	t.Helper()
	if code != 1 || !strings.Contains(result.Summary, "this terminal is enrolled for Wido, not W. Riezebos") {
		t.Fatalf("repair lost the enrolled-name mismatch: exit=%d result=%+v", code, result)
	}
	raw := fmt.Sprintf("%+v", result)
	if strings.Contains(result.Summary, "declaration baseline is unavailable") || strings.Contains(raw, "--by NAME") || result.Next != nil && slices.Equal(result.Next.Argv, append([]string{"metasystem"}, command...)) {
		t.Fatalf("mismatch offered an unavailable-declaration or looping remedy: %+v", result)
	}
	withJSON := slices.Insert(append([]string{"metasystem"}, command...), 3, "--json")
	remedy := shellCommand(withoutOption(withJSON, "by"))
	if (result.Next == nil || !slices.Equal(result.Next.Argv, withoutOption(withJSON, "by"))) && !strings.Contains(result.Summary, "run: "+remedy) {
		t.Fatalf("mismatch did not return the enrolled-name remedy %q: %+v", remedy, result)
	}
}

func TestDeclaredChangeHoldsAgentCheckRoute(t *testing.T) {
	t.Parallel()
	testDeclaredAgentCheckRoute(t, false)
}

func TestDeclaredChangeHoldsAgentCheckRouteAfterAdmission(t *testing.T) {
	t.Parallel()
	testDeclaredAgentCheckRoute(t, true)
}

func testDeclaredAgentCheckRoute(t *testing.T, afterAdmission bool) {
	t.Helper()
	for _, route := range []string{"build", "carry"} {
		t.Run(route, func(t *testing.T) {
			t.Parallel()
			b := newWorkBed(t)
			marker := filepath.Join(t.TempDir(), "changed-audit-started")
			declaration := func(cheap, audits, deadline string) string {
				return fmt.Sprintf("proof.cheap=%s\nproof.audits=%s\nproof.deadline=%s\nproof.full=printf full\n", cheap, audits, deadline)
			}
			main := declaration("printf baseline", "true", "15")
			if afterAdmission {
				main = declaration("true", "true", "15")
			}
			candidate := main
			b.head = strings.Repeat("c", 40)
			subject := branch.AttestationSubject{Commit: b.head, Tree: strings.Repeat("d", 40), Parent: "base-commit"}
			b.workOwnersHook = func(work *intentWorkOwners) {
				original := work.git
				work.git = func(root string, args ...string) ([]byte, error) {
					switch {
					case len(args) == 4 && args[0] == "show" && args[1] == "-s":
						return []byte("Goal-Unit: " + b.id + "/u1"), nil
					case len(args) == 2 && args[0] == "show" && strings.HasSuffix(args[1], ":metasystem.conf"):
						if args[1] == "base-commit:metasystem.conf" {
							return []byte(main), nil
						}
						return []byte(candidate), nil
					case len(args) == 2 && args[0] == "rev-parse" && strings.HasSuffix(args[1], "^{tree}"):
						return []byte(subject.Tree), nil
					case args[0] == "status":
						return nil, nil
					}
					return original(root, args...)
				}
			}
			owners := b.workOwners()
			owners.connection.readRepository = carryFullRepository{root: b.worktree, common: t.TempDir(), subject: subject}
			brief := b.brief("check-route.md", "Build.\n")
			if afterAdmission {
				args := []string{"work", "build", b.id, "--work", "initial", "--brief", brief, "--lines", "5"}
				if code, result := checkActBuild(t, b, owners, args...); code != 0 {
					t.Fatalf("first admitted build: %d %+v", code, result)
				}
				if route == "build" {
					checkNextWorktree(t, b)
				}
			}
			candidate = declaration("true", "printf changed > "+shellCommand([]string{marker}), "40")
			before := len(b.starter.launched())
			args := checkBuildArgs(b, "changed", brief, []string{"true"})
			if route == "carry" {
				args = []string{"work", "review", "--commit", subject.Commit, "--goal", b.id, "--check-only", "--check", "true"}
			}
			code, held := checkActBuild(t, b, owners, args...)
			if code != 1 || !strings.Contains(held.Summary, "the changed check awaits a person's exact command") || len(b.starter.launched()) != before {
				t.Fatalf("changed declarations escaped --check admission: exit=%d summary=%s launches=%v", code, held.Summary, b.starter.launched())
			}
			acts := checkActs(t, b)
			if len(acts) != 1 || acts[0].Class != "declaration" || acts[0].Status != "proposed" || acts[0].BeforeDeclaration.Values[1] != "true" || acts[0].AfterDeclaration.Values[1] != "printf changed > "+shellCommand([]string{marker}) || acts[0].AfterDeclaration.Values[2] != "40" {
				t.Fatalf("missing exact declaration proposal: %+v", acts)
			}
			question, err := channel.ReadQuestion(b.root(), acts[0].Question)
			if err != nil || question.State != "open" || !strings.Contains(question.Wants, "--act "+acts[0].ID) {
				t.Fatalf("missing person command: %+v %v", question, err)
			}
			if route == "build" && !slices.Contains(held.Next.Argv, acts[0].ID) {
				t.Fatalf("wrong printed person command: %+v", held.Next)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("changed audit started before approval: %v", err)
			}
			executions, err := filepath.Glob(filepath.Join(b.root(), "artifacts", "unit-checks", "carry", subject.Commit, "check-*"))
			if err != nil || len(executions) != 0 {
				t.Fatalf("carry children started before approval: %v %v", executions, err)
			}
			remedy := []string{"work", "review", "--commit", subject.Commit, "--goal", b.id, "--repo", b.root(), "--check-only", "--act", acts[0].ID}
			if route == "build" {
				remedy = held.Next.Argv[1:]
			}
			if question.Wants != shellCommand(append([]string{"metasystem"}, remedy...)) {
				t.Fatalf("person command differs from retained question: %q", question.Wants)
			}
			owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
			code, approved := checkActBuild(t, b, owners, remedy...)
			if code != 0 {
				t.Fatalf("printed person command failed: %d %s", code, approved.Summary)
			}
			question, err = channel.ReadQuestion(b.root(), acts[0].Question)
			if err != nil || question.State != "closed" {
				t.Fatalf("approved question remained open: %+v %v", question, err)
			}
			if route == "carry" {
				if _, err := os.Stat(marker); err != nil {
					t.Fatalf("approved audit was not executed: %v", err)
				}
				return
			}
			checkNextWorktree(t, b)
			owners.prove = fixedFixtureGoalAuthority
			code, reused := checkActBuild(t, b, owners, checkBuildArgs(b, "approved", brief, []string{"true"})...)
			if code != 0 {
				t.Fatalf("agent could not reuse admitted declaration: %d %s", code, reused.Summary)
			}
			plan, err := launch.ReadUnitPlan(resultData(t, reused)["plan"].(string))
			if err != nil || plan.Check.Audits != acts[0].AfterDeclaration.Values[1] || plan.Check.Minutes != 40 {
				t.Fatalf("agent did not freeze admitted audits and deadline: %+v %v", plan.Check, err)
			}
		})
	}
}
