package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

func commandSelectionFixture(t *testing.T, root string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "testing.json"))
	helmMust(t, err)
	var contract testpolicy.Contract
	helmMust(t, json.Unmarshal(data, &contract))
	static := contract.Groups[0]
	static.ID, static.Kind, static.Phase = "fast-static-build", "static", "admission"
	static.Inputs = []string{"cmd/**"}
	static.Argv = []string{"/bin/sh", "-c", "printf static-ran > static-ran"}
	contract.Groups = append(contract.Groups, static)
	data, err = json.Marshal(contract)
	helmMust(t, err)
	impactWrite(t, root, "testing.json", string(data))
	impactWrite(t, root, "metasystem.conf", "testing.contract=testing.json\nlanding.impact-max-share=100\n")
	impactWrite(t, root, ".gitignore", "artifacts/\nstatic-ran\nselection-ran\n")
	impactWrite(t, root, "cmd/metasystem/main_test.go", "package main\nimport (\"testing\"; \"os\")\nfunc TestSelection(t *testing.T) { t.Parallel(); if err := os.WriteFile(\"selection-ran\", []byte(\"ran\"), 0600); err != nil { t.Fatal(err) } }\n")
}

// The seat and committed check need Git's real comparison and staged-tree cache.
// The gate keeps its Git seam and reads the module from the check's worktree.
func TestNamedCommandSelectionIsCheapAtEveryEntrypoint(t *testing.T) {
	t.Parallel()
	for _, consumer := range []string{"seat", "committed", "gate"} {
		t.Run(consumer, func(t *testing.T) {
			t.Parallel()
			for _, whole := range []bool{false, true} {
				name, selection := "named", "cmd/metasystem=TestSelection"
				if whole {
					name, selection = "whole", "cmd/metasystem"
				}
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					if consumer == "gate" {
						commandSelectionGate(t, selection, whole)
						return
					}
					root, _ := impactGitAdapterBed(t)
					commandSelectionFixture(t, root)
					// The selected test is added after the comparison commit.
					if !whole {
						helmMust(t, os.Remove(filepath.Join(root, "cmd/metasystem/main_test.go")))
					}
					testingFixtureGit(t, root, "add", ".")
					testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "command selection contract")
					base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
					if !whole {
						impactWrite(t, root, "cmd/metasystem/selected_test.go", "package main\nimport (\"testing\"; \"os\")\nfunc TestSelection(t *testing.T) { t.Parallel(); if err := os.WriteFile(\"selection-ran\", []byte(\"ran\"), 0600); err != nil { t.Fatal(err) } }\n")
					}
					if whole {
						impactWrite(t, root, "cmd/metasystem/unreferenced.go", "package main\nconst Unreferenced = true\n")
					}
					testingFixtureGit(t, root, "add", ".")
					code, out, problem := impactPublic(t, root, "--base", base, "--check", "--plan", "--json")
					result, err := verbresult.Read([]byte(out), "test impact", code, problem)
					var plan plain.ImpactPlan
					want := "impact"
					if whole {
						want = "static-only"
					}
					if err != nil || code != 0 || result.DecodeData(&plan) != nil || plan.Check != want || !slices.Contains(plan.Selections, selection) {
						t.Fatalf("plan: exit=%d plan=%+v out=%s problem=%s error=%v", code, plan, out, problem, err)
					}
					if consumer == "seat" {
						code, out, problem = impactPublic(t, root, "--base", base, "--check")
						phrase := "check: impact"
						if whole {
							phrase = "not cheap: static gate only; the batch proof covers the tree"
						}
						if code != 0 || !strings.Contains(out, phrase) {
							t.Fatalf("seat: exit=%d out=%s problem=%s", code, out, problem)
						}
					} else {
						var output bytes.Buffer
						checked, err := committedImpactCheck(root, base, &output, &output)
						if err != nil || whole && checked.Form != "static-only: selection not cheap" || !whole && !strings.HasPrefix(checked.Form, "impact:") {
							t.Fatalf("committed check: %+v error=%v output=%s", checked, err, output.String())
						}
					}
					_, err = os.Stat(filepath.Join(root, "cmd/metasystem/selection-ran"))
					if whole && !os.IsNotExist(err) || !whole && err != nil {
						t.Fatalf("selection marker: whole=%v error=%v", whole, err)
					}
				})
			}
		})
	}
}

func commandSelectionGate(t *testing.T, selection string, whole bool) {
	t.Helper()
	b := newMergeGateBed(t)
	fixture := impactAdapterBed(t)
	commandSelectionFixture(t, fixture)
	impactWrite(t, fixture, "cmd/metasystem/unselected_test.go", "package main\nimport \"testing\"\nfunc TestUnselected(t *testing.T) { t.Parallel(); t.Fatal(\"unselected test ran\") }\n")
	impactWrite(t, b.install, "metasystem.conf", "testing.contract=testing.json\nlanding.impact-max-share=100\nproof.full=full-fixture\nproof.cheap=cheap-fixture\n")
	b.owners.landing.plainProve.ImpactCost = nil
	git := b.owners.landing.plainProve.Git
	b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		out, err := git(dir, args...)
		if err == nil && len(args) == 5 && args[0] == "worktree" && args[1] == "add" {
			module := filepath.Join(args[3], "metasystem")
			for _, path := range []string{"go.mod", "metasystem.conf", "testing.json", "internal/launch/value.go", "internal/launch/value_test.go", "cmd/metasystem/main.go", "cmd/metasystem/main_test.go", "cmd/metasystem/unselected_test.go"} {
				data, err := os.ReadFile(filepath.Join(fixture, path))
				helmMust(t, err)
				if strings.HasPrefix(path, "cmd/metasystem/") {
					data = bytes.ReplaceAll(data, []byte("package main"), []byte("package command"))
				}
				impactWrite(t, module, path, string(data))
			}
		}
		return out, err
	}
	command := b.owners.landing.plainProve.Command
	var static []string
	b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
		if len(cmd.Args) > 1 && cmd.Args[1] == "test" {
			writeImpactPlanResult(t, cmd, "plan: base fixture\nselection: "+selection+"\n")
			return nil
		}
		if commandEnv(cmd, "LANDING_ONLY") == "fast-static-build" {
			static = append(static, commandEnv(cmd, "LANDING_COMMIT"))
		}
		return command(cmd)
	}
	b.fail = func(cmd *exec.Cmd, only string) (string, error) {
		if only != "" {
			t.Fatalf("unexpected replay: %q", only)
		}
		// The command seam runs only the fixture tests named by its plan.
		pkg, names, named := strings.Cut(selection, "=")
		check := testenv.Go("test", "./"+pkg, "-count=1")
		if named {
			check.Args = append(check.Args, "-run", "^("+strings.ReplaceAll(names, ",", "|")+")$")
		}
		check.Dir, check.Env = cmd.Dir, cmd.Env
		output, err := check.CombinedOutput()
		if err != nil {
			return string(output), err
		}
		_, err = os.Stat(filepath.Join(cmd.Dir, "cmd/metasystem/selection-ran"))
		helmMust(t, err)
		b.runs[len(b.runs)-1] = commandEnv(cmd, "LANDING_COMMIT") + ":" + selection
		return string(output) + fmt.Sprintf("landing environment fixture toolchain\nlanding group unit/%s green 1\nLANDING-CHECKED\t0\n", pkg), nil
	}
	result := gateResult(t, b, 0)
	if whole {
		if result.Result != plain.Skipped || result.Static != plain.Green || result.Reason != "change selects cmd/metasystem whole; the batch check covers it" || len(b.runs) != 0 || !slices.Equal(static, []string{"merge-b"}) {
			t.Fatalf("whole gate: %+v static=%v runs=%v", result, static, b.runs)
		}
	} else if result.Result != plain.Green || result.Depth != "impact" || result.Static != plain.Green || !slices.Equal(b.runs, []string{"merge-a:" + selection, "merge-b:" + selection}) || !slices.Equal(static, []string{"merge-b", "merge-a"}) {
		t.Fatalf("named gate: %+v static=%v runs=%v", result, static, b.runs)
	}
}
