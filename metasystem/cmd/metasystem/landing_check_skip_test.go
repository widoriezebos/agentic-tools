package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// Git supplies the comparison tree and the real adapter selects the changed tests.
func TestLandingCheckCheapOrSkipGitAdapter(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"over-share", "cheap", "cmd-named", "static-red", "missing-static"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			root, _ := impactGitAdapterBed(t)
			data, err := os.ReadFile(filepath.Join(root, "testing.json"))
			helmMust(t, err)
			var contract testpolicy.Contract
			helmMust(t, json.Unmarshal(data, &contract))
			static := contract.Groups[0]
			static.ID, static.Kind, static.Phase = "fast-static-build", "static", "admission"
			static.Inputs = []string{"cmd/**"}
			static.Argv = []string{"/bin/sh", "-c", "printf static-ran > static-ran"}
			if mode == "static-red" {
				static.Argv = []string{"/bin/sh", "-c", "exit 7"}
			}
			if mode != "missing-static" {
				contract.Groups = append(contract.Groups, static)
			}
			data, err = json.Marshal(contract)
			helmMust(t, err)
			impactWrite(t, root, "testing.json", string(data))
			limit := 25
			if mode == "cheap" || mode == "cmd-named" {
				limit = 100
			}
			impactWrite(t, root, "metasystem.conf", fmt.Sprintf("testing.contract=testing.json\nlanding.impact-max-share=%d\n", limit))
			impactWrite(t, root, ".gitignore", "artifacts/\nstatic-ran\nselection-ran\n")
			impactWrite(t, root, "cmd/metasystem/main_test.go", "package main\nimport \"testing\"\nfunc TestCmd(t *testing.T) { t.Parallel() }\n")
			testingFixtureGit(t, root, "add", ".")
			testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "check contract")
			base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
			pkg := "internal/launch"
			if mode == "cmd-named" {
				pkg = "cmd/metasystem"
			}
			packageName := "launch"
			if pkg == "cmd/metasystem" {
				packageName = "main"
			}
			failure := "t.Fatal(\"expensive selection ran\")"
			if mode == "cheap" {
				failure = ""
			}
			impactWrite(t, root, pkg+"/selected_test.go", "package "+packageName+"\nimport (\"testing\"; \"os\")\nfunc TestSelection(t *testing.T) { t.Parallel(); if err := os.WriteFile(\"selection-ran\", []byte(\"ran\"), 0600); err != nil { t.Fatal(err) }; "+failure+" }\n")
			testingFixtureGit(t, root, "add", ".")
			data, err = os.ReadFile("testdata/landing-check/merge-fix.json")
			helmMust(t, err)
			var fix plain.Fix
			helmMust(t, json.Unmarshal(data, &fix))
			fix.State, fix.Commit = "resolving", base
			helmMust(t, plain.WriteFix(root, &fix))
			code, out, problem := impactPublic(t, root, "--base", base, "--check", "--plan", "--json")
			result, err := verbresult.Read([]byte(out), "test impact", code, problem)
			var plan struct {
				Check string `json:"check"`
			}
			want := "static-only"
			if mode == "cheap" {
				want = "impact"
			}
			if err != nil || code != 0 || json.Unmarshal(result.Data, &plan) != nil || plan.Check != want {
				t.Fatalf("plan decision: exit=%d out=%s stderr=%s error=%v", code, out, problem, err)
			}
			if _, err := os.Stat(filepath.Join(root, "static-ran")); !os.IsNotExist(err) {
				t.Fatalf("plan ran static: %v", err)
			}
			code, out, problem = impactPublic(t, root, "--base", base, "--check")
			wantCode := 0
			if mode == "static-red" || mode == "missing-static" {
				wantCode, want = 1, plain.Red
			}
			if mode == "cheap" {
				want = plain.Green
			}
			if code != wantCode {
				t.Fatalf("check exit=%d out=%s stderr=%s", code, out, problem)
			}
			recorded, err := plain.FixForAttempt(root, fix.Attempt)
			if err != nil || recorded.CheckResult != want || recorded.CheckMinutes <= 0 {
				t.Fatalf("fix check: %+v error=%v", recorded, err)
			}
			_, err = os.Stat(filepath.Join(root, pkg, "selection-ran"))
			if mode == "cheap" {
				helmMust(t, err)
			} else if !os.IsNotExist(err) || strings.Contains(out, "landing group unit/") {
				t.Fatalf("expensive selection executed: error=%v out=%s", err, out)
			}
			if want == "static-only" && !strings.Contains(out, "not cheap: static gate only; the batch proof covers the tree") {
				t.Fatalf("skip explanation missing: %s", out)
			}
			if mode == "static-red" || mode == "missing-static" {
				if _, err := os.Stat(filepath.Join(plain.Dir(root), "repair-check.json")); !os.IsNotExist(err) {
					t.Fatalf("failed static gate cached: %v", err)
				}
			}
			t.Logf("%s: exit=%d result=%s\n%s", mode, code, recorded.CheckResult, out)
		})
	}
}

func TestLandingCheckRealRecordShapes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	data, err := os.ReadFile("testdata/landing-check/repair-check.json")
	helmMust(t, err)
	var check repairCheck
	helmMust(t, json.Unmarshal(data, &check))
	if check.Tree == "" || check.Base == "" || check.DurationMS != 136997 || !strings.Contains(check.Reason, "60% of 170 packages") {
		t.Fatalf("real repair check lost fields: %+v", check)
	}
	_, err = repairCheckCache(root, check, true)
	helmMust(t, err)
	if reused, err := repairCheckCache(root, check, false); err != nil || !reused {
		t.Fatalf("real record cannot be reused: %v %v", reused, err)
	}
	check.Base = "another-base"
	if reused, _ := repairCheckCache(root, check, false); reused {
		t.Fatal("real record reused for another base")
	}
	data, err = os.ReadFile("testdata/landing-check/merge-fix.json")
	helmMust(t, err)
	var fix plain.Fix
	helmMust(t, json.Unmarshal(data, &fix))
	helmMust(t, plain.WriteFix(root, &fix))
	got, err := plain.ReadFix(root)
	if err != nil || got == nil || len(got.Paths) != 1 || len(got.Generated) != 1 || got.CheckResult != plain.Green || got.CheckMinutes != 2.283517503466667 {
		t.Fatalf("real fix cannot be read: %+v error=%v", got, err)
	}
}

// The real contract and ledger ride a Git archive, which excludes seat-local configuration.
// The real impact verb reads this repository's complete package graph and prints its plan.
func TestLandingCheckRealContractLedgerAndPlanGitAdapter(t *testing.T) {
	t.Parallel()
	module := realImpactArchive(t, "HEAD")
	root := filepath.Dir(module)
	data, err := os.ReadFile("../../testing.json")
	helmMust(t, err)
	contract, err := testpolicy.Decode(data)
	helmMust(t, err)
	if len(contract.Groups) < 20 {
		t.Fatalf("real contract was not loaded: %d groups", len(contract.Groups))
	}
	goals, err := filepath.Glob(filepath.Join("..", "..", "plans", "goals", "*.md"))
	helmMust(t, err)
	if len(goals) == 0 {
		t.Fatal("real goal ledger missing")
	}
	tiers := map[string]uint8{}
	for _, path := range goals {
		if filepath.Base(path) == "backlog.md" {
			continue
		}
		data, err := os.ReadFile(path)
		helmMust(t, err)
		file, problems := goal.ParseFile(data)
		if len(problems) != 0 || goal.GateTier(file) == 0 {
			t.Fatalf("real goal %s: %v", path, problems)
		}
		tiers[file.Id] = file.Tier
		impactWrite(t, module, "plans/goals/"+filepath.Base(path), string(data))
	}
	testingFixtureGit(t, root, "read-tree", "HEAD")
	base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	impactWrite(t, module, "cmd/metasystem/lane_speed_probe_test.go", "package main\nimport \"testing\"\nfunc TestLaneSpeedProbe(t *testing.T) { t.Parallel() }\n")
	code, out, problem := impactPublic(t, module, "--base", base, "--plan", "--json")
	result, err := verbresult.Read([]byte(out), "test impact", code, problem)
	var plan plain.ImpactPlan
	if err != nil || code != 0 || result.DecodeData(&plan) != nil {
		t.Fatalf("real plan: exit=%d out=%s stderr=%s error=%v", code, out, problem, err)
	}
	// The probe is selected by name inside the cmd/metasystem selection; other tests of this file
	// are selected with it once the file is committed, because they name the probe's path.
	probe := slices.ContainsFunc(plan.Selections, func(s string) bool {
		unit, names, ok := strings.Cut(s, "=")
		return ok && strings.HasSuffix(unit, "cmd/metasystem") && slices.Contains(strings.Split(names, ","), "TestLaneSpeedProbe")
	})
	if plan.Check != "static-only" || !probe {
		t.Fatalf("real plan: check=%q selections=%v", plan.Check, plan.Selections)
	}
	form, err := impactSelectionForm(module, contract, plan.Selections, nil)
	helmMust(t, err)
	for _, id := range []string{"policy-canary", "adapter-canary", "command-interface-smoke"} {
		if !slices.Contains(plan.Selections, id) || !strings.Contains(form, id) {
			t.Fatalf("real contract group %s missing from plan or check record: %v; %s", id, plan.Selections, form)
		}
	}
	// The selector consumes real ledger tiers while the queue and clock remain isolated.
	queueInstall := t.TempDir()
	members := []plain.GoalSHA{}
	for _, id := range slices.Sorted(maps.Keys(tiers)) {
		members = append(members, plain.GoalSHA{Goal: id, SHA: id + "-tip"})
		_, _, err := plain.HandIn(queueInstall, plain.Line{Goal: id, SHA: id + "-tip", At: laneTestNow.Format(time.RFC3339Nano)})
		helmMust(t, err)
	}
	notAncestor := exec.Command("/usr/bin/false").Run()
	selected, err := plain.SelectBatch(queueInstall, queueInstall, lane.Record{Root: queueInstall, Install: queueInstall, CustodyEpoch: 1}, plain.ProveSeams{
		Now:       func() time.Time { return laneTestNow },
		GoalTier:  func(_, id string) (uint8, error) { return tiers[id], nil },
		Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil },
		Git: func(_ string, args ...string) (string, error) {
			switch args[0] {
			case "fetch", "cat-file":
				return "", nil
			case "rev-parse":
				return "main", nil
			case "merge-base":
				return "", notAncestor
			default:
				return "", fmt.Errorf("unexpected selector Git: %v", args)
			}
		},
	})
	if err != nil || selected == nil || !selected.Open || selected.DepthClass != "full" || !slices.Equal(selected.Members, members) {
		t.Fatalf("real ledger selection: %+v error=%v", selected, err)
	}

	t.Logf("real contract: %d groups, %d ledger goals; plan:\n%s", len(contract.Groups), len(goals), plan.Text())
}

// The candidate and its real impact command must agree before work commit authorizes delivery.
func TestWorkCommitLaneFixCheckGitAdapter(t *testing.T) {
	t.Parallel()
	for _, staticRed := range []bool{false, true} {
		t.Run(fmt.Sprintf("static-red-%v", staticRed), func(t *testing.T) {
			t.Parallel()
			c, owners := readPublicationAdapterBed(t)
			fixture, _ := impactGitAdapterBed(t)
			for _, path := range []string{"go.mod", "testing.json", "internal/launch/value.go", "internal/launch/value_test.go", "cmd/metasystem/main.go"} {
				data, err := os.ReadFile(filepath.Join(fixture, path))
				helmMust(t, err)
				impactWrite(t, c.worktree, path, string(data))
			}
			data, err := os.ReadFile(filepath.Join(c.worktree, "testing.json"))
			helmMust(t, err)
			var contract testpolicy.Contract
			helmMust(t, json.Unmarshal(data, &contract))
			static := contract.Groups[0]
			static.ID, static.Kind, static.Inputs = "fast-static-build", "static", []string{"cmd/**"}
			if staticRed {
				static.Argv = []string{"/bin/sh", "-c", "exit 7"}
			}
			contract.Groups = append(contract.Groups, static)
			data, err = json.Marshal(contract)
			helmMust(t, err)
			impactWrite(t, c.worktree, "testing.json", string(data))
			impactWrite(t, c.worktree, "metasystem.conf", "testing.contract=testing.json\nproof.cheap=metasystem test impact\nlanding.impact-max-share=1\n")
			connectionGit(t, c.worktree, "add", ".")
			connectionGit(t, c.worktree, "commit", "-qm", "goal "+c.id+" units settings\n\nGoal-Unit: "+c.id+"/settings")
			parent := connectionGit(t, c.worktree, "rev-parse", "HEAD")
			owners.connection.laneFix = func(_, checkout, id string) *plain.Fix {
				if !sameCanonicalPath(checkout, c.worktree) {
					return nil
				}
				return &plain.Fix{Goal: id, Parent: parent, Round: 1}
			}
			owners.connection.commitCheck = nil
			owners.connection.rebaseGate = func(string) (string, error) { return "static-green", nil }
			impactWrite(t, c.worktree, "internal/launch/value.go", "package launch\nconst Value = false\n")
			connectionGit(t, c.worktree, "add", "internal/launch/value.go")
			code, output, problem := c.run(owners, "work", "commit", c.id, "--work", "lane-fix-1", "--json")
			var result intentResult
			helmMust(t, json.Unmarshal([]byte(output), &result))
			t.Logf("commit check output:\n%s", problem)
			if staticRed {
				if code == 0 || connectionGit(t, c.worktree, "rev-parse", "HEAD") != parent || c.commits != 0 {
					t.Fatalf("static red committed: exit=%d %+v", code, result)
				}
			} else if code != 0 || c.commits != 1 || connectionGit(t, c.worktree, "rev-parse", "HEAD^") != parent {
				t.Fatalf("expensive commit check was not skipped: exit=%d %+v\n%s", code, result, problem)
			}
		})
	}
}

func realImpactArchive(t *testing.T, ref string) string {
	t.Helper()
	root := t.TempDir()
	archive := filepath.Join(t.TempDir(), "module.zip")
	testingFixtureGit(t, "../../..", "clone", "--quiet", "--shared", "--no-checkout", ".", root)
	testingFixtureGit(t, root, "update-ref", "HEAD", ref)
	testingFixtureGit(t, "../../..", "archive", "--format=zip", "--output="+archive, ref, "metasystem")
	reader, err := zip.OpenReader(archive)
	helmMust(t, err)
	defer reader.Close()
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		input, err := file.Open()
		helmMust(t, err)
		data, err := io.ReadAll(input)
		helmMust(t, err)
		helmMust(t, input.Close())
		impactWrite(t, root, file.Name, string(data))
		helmMust(t, os.Chmod(filepath.Join(root, file.Name), file.Mode().Perm()))
	}
	return filepath.Join(root, "metasystem")
}
