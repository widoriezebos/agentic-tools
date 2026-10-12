package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// Git's candidate and parent must produce the same selection as the public impact plan.
func TestWorkCommitUnitImpactGitAdapter(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"cheap", "cheap-flags", "declared", "declared-red", "selected-red", "cmd-named", "over-share", "static-red"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			c, owners := readPublicationAdapterBed(t)
			fixture := impactAdapterBed(t)
			data, err := os.ReadFile(filepath.Join(fixture, "testing.json"))
			helmMust(t, err)
			var contract testpolicy.Contract
			helmMust(t, json.Unmarshal(data, &contract))
			static := contract.Groups[0]
			static.ID, static.Kind, static.Phase, static.Inputs = "fast-static-build", "static", "admission", []string{"cmd/**"}
			static.Argv = []string{"/bin/sh", "-c", `test -f go.mod && test -n "$LANDING_PROOF_BASE" && test -z "$LANDING_ONLY"`}
			if mode == "static-red" {
				static.Argv = []string{"/bin/sh", "-c", "exit 7"}
			}
			contract.Groups = append(contract.Groups, static)
			data, err = json.Marshal(contract)
			helmMust(t, err)
			impactWrite(t, c.worktree, "testing.json", string(data))
			impactWrite(t, c.worktree, "go.mod", "module example.invalid/impact\n\ngo 1.27\n")
			limit := 100
			if mode == "over-share" || mode == "static-red" {
				limit = 1
			}
			command := "metasystem test impact"
			if mode == "cheap-flags" {
				command += " --json"
			}
			marker := filepath.Join(t.TempDir(), "declared-ran")
			if strings.HasPrefix(mode, "declared") {
				command = "./check.sh"
				exit := "0"
				if mode == "declared-red" {
					exit = "7"
				}
				helmMust(t, testexec.WriteFile(filepath.Join(c.worktree, "check.sh"), []byte("#!/bin/sh\nprintf '%s' \"$LANDING_PROOF_BASE\" > "+shellCommand([]string{marker})+"\nexit "+exit+"\n"), 0700))
			}
			impactWrite(t, c.worktree, "metasystem.conf", fmt.Sprintf("testing.contract=testing.json\nproof.cheap=%s\nlanding.impact-max-share=%d\n", command, limit))
			pkg := "cmd/tool"
			if mode == "cmd-named" {
				pkg = "cmd/metasystem"
			}
			impactWrite(t, c.worktree, pkg+"/value.go", "package main\nfunc Value() bool { return true }\n")
			impactWrite(t, c.worktree, pkg+"/value_test.go", "package main\nimport \"testing\"\nfunc TestValue(t *testing.T) { t.Parallel(); if !Value() { t.Fatal(\"selected red\") } }\n")
			impactWrite(t, c.worktree, pkg+"/family_test.go", "package main\nimport \"testing\"\nfunc TestFamily(t *testing.T) { t.Parallel(); t.Fatal(\"family ran\") }\n")
			impactWrite(t, c.worktree, pkg+"/earlier.go", "package main\nfunc Earlier() bool { return false }\n")
			impactWrite(t, c.worktree, pkg+"/earlier_test.go", "package main\nimport \"testing\"\nfunc TestEarlier(t *testing.T) { t.Parallel(); t.Fatal(\"earlier unit ran\") }\n")
			connectionGit(t, c.worktree, "add", ".")
			connectionGit(t, c.worktree, "commit", "-qm", "goal "+c.id+" units settings\n\nGoal-Unit: "+c.id+"/settings")
			impactWrite(t, c.worktree, pkg+"/earlier.go", "package main\nfunc Earlier() bool { return true }\n")
			connectionGit(t, c.worktree, "add", ".")
			connectionGit(t, c.worktree, "commit", "-qm", "goal "+c.id+" units earlier\n\nGoal-Unit: "+c.id+"/earlier")
			parent := connectionGit(t, c.worktree, "rev-parse", "HEAD")
			value := "true // unit edit"
			if mode == "selected-red" {
				value = "false"
			}
			impactWrite(t, c.worktree, pkg+"/value.go", "package main\nfunc Value() bool { return "+value+"\n}\n")
			connectionGit(t, c.worktree, "add", ".")
			owners.connection.rebaseGate = func(string) (string, error) { return "static-green", nil }
			owners.connection.commitCheck = func(directory, base string) (repairCheck, error) {
				if base != parent {
					t.Fatalf("unit base %s != parent %s", base, parent)
				}
				code, out, problem := impactPublic(t, directory, "--base", base, "--plan", "--json")
				result, err := verbresult.Read([]byte(out), "test impact", code, problem)
				var plan plain.ImpactPlan
				if err != nil || code != 0 || result.DecodeData(&plan) != nil || !slices.Contains(plan.Selections, pkg+"=TestValue") || slices.ContainsFunc(plan.Selections, func(s string) bool {
					return strings.Contains(s, "TestFamily") || strings.Contains(s, "TestEarlier") || s == pkg
				}) {
					t.Fatalf("unit selection grew: exit=%d plan=%+v stderr=%s error=%v", code, plan, problem, err)
				}
				return committedWorkCheck(directory, base, &bytes.Buffer{}, &bytes.Buffer{})
			}
			if strings.HasPrefix(mode, "declared") {
				owners.connection.commitCheck = nil
			}
			code, result := c.runJSON(owners, "work", "commit", c.id, "--work", "value")
			if strings.HasPrefix(mode, "declared") {
				data, err := os.ReadFile(marker)
				if err != nil || string(data) != parent {
					t.Fatalf("declared script did not run against parent %s: %s %v", parent, data, err)
				}
			}
			red := mode == "selected-red" || mode == "static-red" || mode == "declared-red"
			if red {
				if code == 0 || c.commits != 0 || connectionGit(t, c.worktree, "rev-parse", "HEAD") != parent {
					t.Fatalf("red committed: exit=%d %+v", code, result)
				}
				return
			}
			if code != 0 || c.commits != 1 {
				t.Fatalf("unit commit exit=%d %+v", code, result)
			}
			paths, err := filepath.Glob(filepath.Join(c.root(), "artifacts/unit-checks/commits/*.json"))
			helmMust(t, err)
			if len(paths) != 1 {
				t.Fatalf("check records: %v", paths)
			}
			data, err = os.ReadFile(paths[0])
			helmMust(t, err)
			var checked repairCheck
			helmMust(t, json.Unmarshal(data, &checked))
			form := "static-only: selection not cheap"
			if mode == "cheap" || mode == "cheap-flags" {
				form = "impact: 1 tests of 1 packages + groups canary"
			}
			if mode == "declared" {
				form = "declared: ./check.sh"
			}
			if checked.Form != form || checked.Minutes <= 0 || checked.DurationMS <= 0 || checked.Base != parent || checked.Tree == "" {
				t.Fatalf("check record: %+v", checked)
			}
			t.Logf("%s: exit=%d %s; %.4f minutes", mode, code, checked.Form, checked.Minutes)
		})
	}
}

// This saved lane fix is checked against the live testing contract and the real public plan.
func TestWorkCommitRealLaneFixImpactPlanGitAdapter(t *testing.T) {
	t.Parallel()
	var saved struct {
		Base, Commit, Package string
	}
	data, err := os.ReadFile("testdata/work-commit-impact/lane-fix.json")
	helmMust(t, err)
	helmMust(t, json.Unmarshal(data, &saved))
	base := saved.Base
	module := realImpactArchive(t, base)
	root := filepath.Dir(module)
	connectionGit(t, root, "read-tree", base)
	patch, err := os.ReadFile("testdata/work-commit-impact/lane-fix.patch")
	helmMust(t, err)
	path := filepath.Join(t.TempDir(), "fix.patch")
	helmMust(t, os.WriteFile(path, patch, 0600))
	connectionGit(t, root, "apply", path)
	// Contract certification deliberately reads the live file, never an install's local override.
	data, err = os.ReadFile("../../testing.json")
	helmMust(t, err)
	contract, err := testpolicy.Decode(data)
	helmMust(t, err)
	impactWrite(t, module, "testing.json", string(data))
	code, out, problem := impactPublic(t, module, "--base", base, "--plan", "--json")
	result, err := verbresult.Read([]byte(out), "test impact", code, problem)
	var plan plain.ImpactPlan
	if err != nil || code != 0 || result.DecodeData(&plan) != nil {
		t.Fatalf("lane fix plan exit=%d stderr=%s error=%v", code, problem, err)
	}
	selected := ""
	for _, selection := range plan.Selections {
		if strings.HasPrefix(selection, saved.Package+"=") {
			selected = selection
		}
		if selection == saved.Package {
			t.Fatal("lane fix selected the entire command package")
		}
	}
	_, names, named := strings.Cut(selected, "=")
	count := len(strings.Split(names, ","))
	family := map[string]bool{}
	for _, group := range contract.Groups {
		if !slices.ContainsFunc(group.Packages, func(p string) bool {
			return strings.TrimPrefix(strings.TrimPrefix(p, "./"), "metasystem/") == "cmd/metasystem"
		}) {
			continue
		}
		_, tests, err := testpolicy.GoTests(group)
		helmMust(t, err)
		for _, name := range tests {
			family[name] = true
		}
	}
	if !named || names == "" {
		t.Fatalf("lane fix %s did not select named command tests: %s", saved.Commit, selected)
	}
	if plan.Check != "static-only" {
		t.Fatalf("command selection was cheap: %+v", plan)
	}
	// The form is the actual selection size, not the real contract's command-family group.
	if len(contract.Groups) < 20 {
		t.Fatal("live contract was not loaded")
	}
	whole := testpolicy.Group{ID: "command-family", Packages: []string{"./cmd/metasystem"}, Tests: json.RawMessage(`"all"`)}
	form, err := impactSelectionForm(module, testpolicy.Contract{Groups: []testpolicy.Group{whole}}, []string{whole.ID}, map[string]string{whole.ID: "cmd/metasystem"})
	helmMust(t, err)
	var total int
	if _, err := fmt.Sscanf(form, "impact: %d tests of 1 packages", &total); err != nil || count >= total {
		t.Fatalf("selected %d of %d command tests: %s error=%v", count, total, form, err)
	}
	t.Logf("real lane fix: %d of %d command tests; %d contract family tests; check=%s", count, total, len(family), plan.Check)
}

func TestWorkCommitImpactCheckKeepsRecordedValuesGitAdapter(t *testing.T) {
	t.Parallel()
	root, base := impactGitAdapterBed(t)
	checked, err := repairCheckIdentity(root, base)
	helmMust(t, err)
	checked.Form, checked.Reason, checked.DurationMS, checked.Minutes = "impact: 7 tests of 2 packages", "saved unit selection", 123000, 2.05
	_, err = repairCheckCache(root, checked, true)
	helmMust(t, err)
	var output bytes.Buffer
	got, err := committedImpactCheck(root, base, &output, &output)
	if err != nil || got != checked || !strings.Contains(output.String(), "check reused from the job") {
		t.Fatalf("check recomputed recorded values: got=%+v want=%+v err=%v output=%s", got, checked, err, output.String())
	}
}
