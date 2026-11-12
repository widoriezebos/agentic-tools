package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func publicationEngineStub(t *testing.T) string {
	t.Helper()
	engine := filepath.Join(t.TempDir(), "metasystem")
	helmMust(t, testexec.WriteFile(engine, []byte("#!/bin/sh\ntest \"$*\" = 'test impact --publication-only' || exit 2\npwd\n"), 0700))
	return engine
}

// UnitImpact reads a real repository diff and plain.Git has no injected seam;
// markers observe the public command's prerequisite closure and execution order.
func TestUnitCheckRunsWhatPublicationRequires(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"cheap", "narrowed", "static-only", "lane", "manual", "cached", "cached-red", "cached-missing"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			root := impactAdapterBed(t)
			marker := filepath.Join(t.TempDir(), "groups")
			data, err := os.ReadFile(filepath.Join(root, "testing.json"))
			helmMust(t, err)
			var contract testpolicy.Contract
			helmMust(t, json.Unmarshal(data, &contract))
			prerequisite := contract.Groups[2]
			prerequisite.ID = "refusal-register-standard"
			contract.Groups[2].Requires = []string{prerequisite.ID}
			contract.Groups = append(contract.Groups, prerequisite)
			for i := range contract.Groups {
				group := &contract.Groups[i]
				group.Argv = []string{"/bin/sh", "-c", "printf '%s\\n' " + shellCommand([]string{group.ID}) + " >> " + shellCommand([]string{marker})}
				if mode == "cached-red" && group.ID == "fast-static-build" {
					group.Argv[2] += "; exit 1"
				}
			}
			if mode == "cached-missing" {
				contract.Groups = slices.DeleteFunc(contract.Groups, func(group testpolicy.Group) bool { return group.ID == "fast-static-build" })
			}
			data, err = json.Marshal(contract)
			helmMust(t, err)
			impactWrite(t, root, "testing.json", string(data))
			limit := 100
			if mode == "static-only" {
				limit = 1
			} else if mode == "narrowed" {
				limit = 40 // One selected package is cheap; adding the root makes it too wide.
			}
			impactWrite(t, root, "metasystem.conf", fmt.Sprintf("testing.contract=testing.json\nlanding.impact-max-share=%d\n", limit))
			impactWrite(t, root, "cmd/metasystem/value.go", "package main\nfunc PublicationValue() int { return 1 }\n")
			impactWrite(t, root, "cmd/metasystem/value_test.go", fmt.Sprintf("package main\nimport (\"testing\"; \"os\")\nfunc TestPublication(t *testing.T) { t.Parallel(); _ = PublicationValue(); f, err := os.OpenFile(%q, os.O_APPEND|os.O_WRONLY, 0600); if err != nil { t.Fatal(err) }; defer f.Close(); if _, err := f.WriteString(\"named\\n\"); err != nil { t.Fatal(err) } }\n", marker))
			impactWrite(t, root, "cmd/metasystem/noise_test.go", "package main\nimport \"testing\"\nfunc TestNoise(t *testing.T) { t.Parallel(); t.Fatal(\"unselected test ran\") }\n")
			testingFixtureGit(t, root, "init", "-q", "-b", "main")
			testingFixtureGit(t, root, "add", ".")
			testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "base")
			base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
			if mode == "narrowed" {
				impactWrite(t, root, "cmd/metasystem/value.go", "package main\nfunc PublicationValue() int { return 2 }\n")
			} else {
				impactWrite(t, root, "internal/launch/value.go", "package launch\nconst Value = true // edited\n")
			}
			testingFixtureGit(t, root, "add", ".")
			if strings.HasPrefix(mode, "cached") {
				identity, err := repairCheckIdentity(root, base)
				helmMust(t, err)
				_, err = repairCheckCache(root, identity, true)
				helmMust(t, err)
			}
			args := []string{"test", "impact", "--root", root, "--base", base, "--check"}
			if mode == "cheap" || mode == "lane" {
				args = args[:len(args)-1]
			}
			if mode == "manual" {
				b := newWorkBed(t)
				owners := b.workOwners()
				owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
				code, built := checkActBuild(t, b, owners, "work", "build", b.id, "manual", "--brief", b.brief("manual.md", "Build.\n"), "--lines", "5", "--by", "Wido", "--reason", "Run the manual check", "--check", "true")
				if code != 0 {
					t.Fatalf("manual build: exit=%d %+v", code, built)
				}
				plan, err := launch.ReadUnitPlan(resultData(t, built)["plan"].(string))
				if err != nil || plan.Check == nil || plan.Check.Audits != "metasystem test impact --publication-only" {
					t.Fatalf("manual check did not select the publication command: %v", err)
				}
				args = append(shellWords(plan.Check.Audits)[1:], "--root", root)
			}
			run := func(plan bool) (int, string) {
				t.Helper()
				command := exec.CommandContext(t.Context(), commandTestExecutable(t), args...)
				if plan {
					command.Args = append(command.Args, "--plan")
				}
				command.Dir = root
				scope := ""
				if mode == "lane" {
					scope = "impact"
				}
				command.Env = fixtureCommandEnvironment(t, "LANDING_ONLY=", "LANDING_PROOF_SCOPE="+scope, "LANDING_PROOF_BASE=")
				out, err := command.CombinedOutput()
				code := 0
				if err != nil {
					exit, ok := err.(*exec.ExitError)
					if !ok {
						t.Fatal(err)
					}
					code = exit.ExitCode()
				}
				return code, string(out)
			}
			if mode != "cached-missing" {
				code, out := run(true)
				if code != 0 || !strings.Contains(out, "required: fast-static-build\n") || strings.Contains(out, "selection: fast-static-build") {
					t.Fatalf("requirements plan: exit=%d %s", code, out)
				}
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatalf("plan ran a group: %v", err)
				}
			}
			code, out := run(false)
			if mode == "cached-missing" {
				if code != 1 || !strings.Contains(out, "the contract has no publication group fast-static-build; nothing was run") {
					t.Fatalf("missing requirement: exit=%d %s", code, out)
				}
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatalf("refused check ran a group: %v", err)
				}
				return
			}
			want := []string{"refusal-register-standard", "fast-static-build"}
			if mode != "manual" && mode != "static-only" {
				want = append(want, "canary")
			}
			if mode == "narrowed" {
				want = append(want, "named")
			}
			data, err = os.ReadFile(marker)
			wantCode := 0
			if mode == "cached-red" {
				wantCode = 1
			}
			if code != wantCode || err != nil || !slices.Equal(strings.Fields(string(data)), want) {
				t.Fatalf("exit=%d want=%d markers=%s want=%v error=%v\n%s", code, wantCode, data, want, err, out)
			}
		})
	}
}
