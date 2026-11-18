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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch/goadapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestBatchPlanTouchedInputsIgnoreTemplateCoverageOnlyForBatch(t *testing.T) {
	t.Parallel()
	for _, template := range []bool{false, true} {
		for _, touched := range []bool{false, true} {
			t.Run(fmt.Sprintf("template=%t/touched=%t", template, touched), func(t *testing.T) {
				t.Parallel()
				root, sources := batchPlanBed(t)
				if template {
					install := filepath.Join(t.TempDir(), "metasystem")
					helmMust(t, os.Rename(root, install))
					root = install
					impactWrite(t, root, "metasystem.conf", "metasystem.template=true\ntesting.contract=testing.json\n")
				}
				data, err := os.ReadFile(filepath.Join(root, "testing.json"))
				helmMust(t, err)
				var contract testpolicy.Contract
				helmMust(t, json.Unmarshal(data, &contract))
				g := contract.Groups[len(contract.Groups)-1]
				g.ID, g.Phase, g.Inputs = "input-reader", "acceptance", []string{"docs/**"}
				contract.Groups = append(contract.Groups, g)
				tagged := g
				tagged.ID = "tagged-reader"
				tagged.BuildTags = []string{"batchtest"}
				tagged.Tests = json.RawMessage(`"all"`)
				contract.Groups = append(contract.Groups, tagged)
				selector := g
				selector.ID = "package-template"
				selector.PackageSelection = "changed-and-consumers"
				selector.Packages = nil
				selector.Tests = json.RawMessage(`"all"`)
				contract.Groups = append(contract.Groups, selector)
				section := g
				section.ID = "section/excluded"
				contract.Groups = append(contract.Groups, section)
				prefix := ""
				if template {
					prefix = "metasystem/"
					for i := range contract.Groups {
						contract.Groups[i].CWD = "metasystem"
						contract.Groups[i].Inputs = slices.Clone(contract.Groups[i].Inputs)
						for j := range contract.Groups[i].Inputs {
							contract.Groups[i].Inputs[j] = prefix + contract.Groups[i].Inputs[j]
						}
					}
				}
				data, err = json.Marshal(contract)
				helmMust(t, err)
				impactWrite(t, root, "testing.json", string(data))
				impactWrite(t, root, "internal/launch/tagged_test.go", "//go:build batchtest\n\npackage launch\nimport \"testing\"\nfunc TestTagged(t *testing.T) {}\n")
				path := "unrelated/readme.md"
				if touched {
					path = "docs/orchestration.md"
				}
				prior := sources.git
				sources.git = func(root string, args ...string) (string, error) {
					if args[0] == "diff" {
						return prefix + path + "\x00", nil
					}
					return prior(root, args...)
				}
				sources.impact = func(string, string, bool) (goadapter.Impact, error) {
					return goadapter.Impact{Paths: []string{path}}, nil
				}
				plan, err := sources.batchPlan(root, "batch-base")
				helmMust(t, err)
				selected := func(id string) bool {
					return slices.ContainsFunc(plan.Groups, func(g testpolicy.Group) bool { return g.ID == id })
				}
				if selected("input-reader") != touched || selected("tagged-reader") != touched || selected("package-template") || selected("section/excluded") {
					t.Fatalf("selected groups=%v", plan.Groups)
				}
				if touched {
					if got := plan.GroupTests["tagged-reader"]["internal/launch"]; !slices.Equal(got, []string{"TestTagged"}) {
						t.Fatalf("tagged selection=%v", got)
					}
					if plan.TestCount != 2 || plan.GroupInvocations <= plan.TestCount {
						t.Fatalf("distinct=%d group invocations=%d", plan.TestCount, plan.GroupInvocations)
					}
					if !strings.Contains(plan.text(), "group invocations:") {
						t.Fatal("text hides invocation count")
					}
				}
				for _, scope := range []string{"", "impact"} {
					sources.getenv = func(key string) string {
						if key == "LANDING_PROOF_SCOPE" {
							return scope
						}
						return ""
					}
					var out, problem bytes.Buffer
					code := sources.run([]string{"--root", root, "--base", "batch-base", "--plan", "--json"}, &out, &problem, "")
					if code != 0 {
						t.Fatalf("seat/gate scope %q: %s", scope, problem.String())
					}
					for _, id := range []string{"input-reader", "tagged-reader", "section/excluded"} {
						if strings.Contains(out.String(), id) {
							t.Fatalf("seat/gate acquired %s: %s", id, out.String())
						}
					}
				}
				// The gate's subprocess reader must continue to request the unit consumer.
				_, err = plain.ReadImpactPlan(plain.ProveSeams{Executable: func() (string, error) { return "fixture-engine", nil }, Command: func(cmd *exec.Cmd) error {
					if slices.Contains(cmd.Args, "--consumer") {
						t.Fatal("gate changed consumer")
					}
					code := sources.run(append(cmd.Args[3:], "--root", root), cmd.Stdout, cmd.Stderr, "")
					if code != 0 {
						t.Fatalf("gate reader exit=%d", code)
					}
					ended := exec.Command("/usr/bin/true")
					helmMust(t, ended.Run())
					cmd.ProcessState = ended.ProcessState
					return nil
				}}, root, "batch-base", "impact")
				helmMust(t, err)
			})
		}
	}
}
