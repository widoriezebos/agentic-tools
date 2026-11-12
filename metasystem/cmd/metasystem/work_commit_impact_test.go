package main

import (
	"bytes"
	"encoding/json"
	"errors"
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
			contract.Groups[2] = static
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
			impactWrite(t, c.worktree, "metasystem.conf", fmt.Sprintf("testing.contract=testing.json\nproof.cheap=%s\nproof.audits=true\nlanding.impact-max-share=%d\n", command, limit))
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
			staticCalls := 0
			owners.connection.rebaseGate = func(string) (string, error) { staticCalls++; return "static-green", nil }
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
			wantStatic := 0
			if strings.HasPrefix(mode, "declared") {
				wantStatic = 1
			}
			if staticCalls != wantStatic {
				t.Fatalf("separate static gate ran %d times, want %d", staticCalls, wantStatic)
			}
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
			if mode == "cheap" || mode == "cheap-flags" || mode == "cmd-named" {
				form = "impact: 1 tests of 1 packages + groups canary"
			}
			if mode == "declared" {
				form = "declared: ./check.sh"
			}
			if checked.Form != form+" + no audits" || checked.Minutes <= 0 || checked.DurationMS <= 0 || checked.Base != parent || checked.Tree == "" {
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
	share, cheap, full, reason, err := impactCost(module, module, plan.Text(), contract)
	helmMust(t, err)
	if plan.Check != "impact" || !cheap {
		t.Fatalf("named command selection was not cheap: share=%d full=%d reason=%q plan=%+v", share, full, reason, plan)
	}
	t.Logf("real lane fix share: %d%% of %d packages", share, full)
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
	if err != nil || got.Tree != checked.Tree || got.Base != checked.Base || got.Form == checked.Form || strings.Contains(output.String(), "check reused from the job") || !strings.Contains(output.String(), "landing group fast-static-build green") {
		t.Fatalf("check did not replace legacy evidence: got=%+v legacy=%+v err=%v output=%s", got, checked, err, output.String())
	}
}

// Real Git binds the declarations and the fresh check to the candidate's parent.
func TestWorkCommitRunsTheDeclaredAuditsGitAdapter(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"impact", "declared", "reused"} {
		for _, declaration := range []string{"proof.audits=exit 3\n", "proof.audits=true\n", "proof.audits=\n", "proof.audits=true\nproof.audits=exit 3\n", ""} {
			t.Run(path+"/"+declaration, func(t *testing.T) {
				t.Parallel()
				root, _ := impactGitAdapterBed(t)
				root, err := filepath.EvalSymlinks(root)
				helmMust(t, err)
				cheap := "metasystem test impact"
				if path == "declared" {
					cheap = "./check.sh"
					mark := "test -n \"$LANDING_PROOF_BASE\" && test -z \"$LANDING_ONLY\"\n"
					helmMust(t, testexec.WriteFile(filepath.Join(root, "check.sh"), []byte("#!/bin/sh\n"+mark), 0700))
				}
				impactWrite(t, root, "metasystem.conf", "testing.contract=testing.json\nlanding.impact-max-share=100\nproof.cheap="+cheap+"\n"+declaration)
				connectionGit(t, root, "add", ".")
				connectionGit(t, root, "commit", "-qm", "declarations")
				base := connectionGit(t, root, "rev-parse", "HEAD")
				impactWrite(t, root, "internal/launch/value.go", "package launch\nconst Value = true // unit\n")
				connectionGit(t, root, "add", ".")
				if path == "reused" {
					checked, err := repairCheckIdentity(root, base)
					helmMust(t, err)
					checked.Form, checked.Minutes = "impact: saved", 2.05
					_, err = repairCheckCache(root, checked, true)
					helmMust(t, err)
				}
				// Working configuration cannot replace the committed audit command.
				impactWrite(t, root, "metasystem.conf", "testing.contract=testing.json\nlanding.impact-max-share=100\nproof.cheap="+cheap+"\nproof.audits=true\n")
				var output bytes.Buffer
				checked, err := committedWorkCheck(root, base, &output, &output)
				if declaration == "proof.audits=true\n" || declaration == "" {
					if err != nil || checked.AuditMinutes < 0 || !strings.HasSuffix(checked.Form, " + no audits") {
						t.Fatalf("passing audits: %+v error=%v output=%s", checked, err, output.String())
					}
					if declaration == "" && !strings.Contains(output.String(), "no audits declared") {
						t.Fatalf("absent adopted declaration was not explained: %s", output.String())
					}
				} else {
					var failure *committedAuditFailure
					want := "proof.audits"
					if declaration == "proof.audits=exit 3\n" {
						want = "exit status 3"
					}
					if !errors.As(err, &failure) || !strings.Contains(err.Error(), want) {
						t.Fatalf("audit failure: %+v error=%v output=%s", checked, err, output.String())
					}
				}
				if path == "reused" && (declaration == "" || declaration == "proof.audits=true\n" || declaration == "proof.audits=exit 3\n") && (strings.Contains(output.String(), "check reused from the job") || !strings.Contains(output.String(), "landing group fast-static-build green")) {
					t.Fatalf("legacy evidence skipped the cheap check: %+v output=%s", checked, output.String())
				}
			})
		}
	}
}

// The public commit boundary must leave the branch unchanged on an audit failure.
func TestWorkCommitReportsTheDeclaredAuditsGitAdapter(t *testing.T) {
	t.Parallel()
	for _, audits := range []string{"exit 3", "test -n \"$LANDING_PROOF_BASE\" && test -z \"$LANDING_ONLY\""} {
		t.Run(audits, func(t *testing.T) {
			t.Parallel()
			c, owners := readPublicationAdapterBed(t)
			marker := filepath.Join(t.TempDir(), "audit-ran")
			impactWrite(t, c.worktree, "metasystem.conf", "proof.cheap=true\nproof.audits="+audits+" && touch "+shellCommand([]string{marker})+"\n")
			connectionGit(t, c.worktree, "add", ".")
			connectionGit(t, c.worktree, "commit", "-qm", "goal "+c.id+" units settings\n\nGoal-Unit: "+c.id+"/settings")
			parent := connectionGit(t, c.worktree, "rev-parse", "HEAD")
			impactWrite(t, c.worktree, "hand.txt", "unit\n")
			connectionGit(t, c.worktree, "add", "hand.txt")
			owners.connection.rebaseGate = func(string) (string, error) { return "static-green", nil }
			owners.connection.commitCheck = nil
			code, result := c.runJSON(owners, "work", "commit", c.id, "--work", "hand")
			if audits == "exit 3" {
				if code == 0 || !strings.HasPrefix(result.Summary, "the declared audits failed; nothing was committed") || connectionGit(t, c.worktree, "rev-parse", "HEAD") != parent || c.commits != 0 {
					t.Fatalf("audit red committed or mislabeled: exit=%d %+v", code, result)
				}
			} else if _, err := os.Stat(marker); code != 0 || err != nil || !strings.Contains(result.Summary, "(cheap ") || !strings.Contains(result.Summary, " minutes, audits ") || c.commits != 1 {
				t.Fatalf("audit success: exit=%d %+v marker=%v", code, result, err)
			}
		})
	}
}
