package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestLandingProveDepthShareBoundary(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, plan string
		share      int
		cheap      bool
	}{
		{"below", "selection: internal/a\nselection: internal/b\n", 40, true},
		{"at threshold", "selection: internal/a\n", 50, false},
		{"above", "selection: internal/a\nselection: internal/b\nselection: internal/c\n", 60, false},
		{"command whole", "selection: metasystem/cmd/metasystem\n", 20, false},
		{"command named", "selection: metasystem/cmd/metasystem=TestA\n", 20, false},
		{"named and static", "selection: fast-static-build\nselection: internal/a=TestA\n", 40, true},
		{"duplicate", "selection: internal/a\nselection: internal/a\n", 20, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			full := 5
			if row.name == "at threshold" {
				full = 2
			}
			contract := testpolicy.Contract{Groups: []testpolicy.Group{{ID: "fast-static-build", Adapter: "command"}}}
			share, cheap := impactDepth(row.plan, contract, depthTestMeasure(full, row.plan), 50)
			if share != row.share || cheap != row.cheap {
				t.Fatalf("share=%d cheap=%v", share, cheap)
			}
		})
	}
}

func depthGoals(t *testing.T, b *replayVerbBed, tiers ...int) {
	t.Helper()
	files := map[string]*goal.GoalFile{}
	gcliForgivingBoxed("a", "b")(files)
	for i, id := range []string{"a", "b"} {
		files[id].Tier = uint8(tiers[i])
		files[id].Risk.Severity = uint8(tiers[i])
		path := filepath.Join(b.install, "plans", "goals", id+".md")
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, goal.RenderFile(files[id]), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLandingProveBatchDepthReasonsAndStatus(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name                string
		tiers               []int
		plan, scope, reason string
	}{
		{"below", []int{2, 2}, "selection: internal/launch\n", "impact", "tiers 2,2; 12% of 8 packages"},
		{"at tier", []int{2, 3}, "", "full", "tier 3: full"},
		{"above tier", []int{3, 3}, "", "full", "tier 3: full"},
		{"share", []int{2, 2}, "selection: internal/a\nselection: internal/b\nselection: internal/launch\nselection: internal/goal/branch\nselection: cmd/metasystem=TestProcess\n", "full", "impact would cover 62% of 8 packages: full"},
		{"cmd", []int{2, 2}, "selection: cmd/metasystem\n", "full", "change selects cmd/metasystem: full"},
		{"cmd named only", []int{2, 2}, "selection: cmd/metasystem=TestProcess\n", "full", "change selects cmd/metasystem: full"},
		{"cmd named", []int{2, 2}, "selection: cmd/metasystem=TestProcess\nselection: internal/goal/branch\n", "full", "change selects cmd/metasystem: full"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := newMergeGateBed(t)
			b.prepareBatch(t)
			seedCurrentTrunkClock(t, b.install, laneTestNow)
			depthGoals(t, b, row.tiers...)
			if row.name == "above tier" {
				impactWrite(t, b.install, "metasystem.conf", "landing.full-from-tier=2\n")
			}
			// Three measured command groups and five test packages make the denominator explicit.
			fixture := impactAdapterBed(t)
			data, err := os.ReadFile(filepath.Join(fixture, "testing.json"))
			if err != nil {
				t.Fatal(err)
			}
			var contract testpolicy.Contract
			if err = json.Unmarshal(data, &contract); err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"extra-a", "extra-b"} {
				g := contract.Groups[0]
				g.ID = id
				contract.Groups = append(contract.Groups, g)
			}
			data, err = json.Marshal(contract)
			if err != nil {
				t.Fatal(err)
			}
			var depthTrees []string
			git := b.owners.landing.plainProve.Git
			b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
				result, err := git(dir, args...)
				if len(args) > 1 && args[0] == "worktree" && args[1] == "add" {
					if strings.HasPrefix(filepath.Base(args[3]), "metasystem-depth-") {
						scratch, err := diskstore.ProcessScratch()
						if err != nil || filepath.Dir(args[3]) != scratch {
							t.Fatalf("depth checkout outside process scratch: path=%s scratch=%s error=%v", args[3], scratch, err)
						}
						depthTrees = append(depthTrees, args[3])
					}
					install := filepath.Join(args[3], "metasystem")
					if err := os.MkdirAll(install, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("testing.contract=testing.json\n"), 0600); err != nil {
						t.Fatal(err)
					}
					impactWrite(t, install, "go.mod", "module example.invalid/depth\n\ngo 1.27\n")
					impactWrite(t, install, "internal/launch/value_test.go", "package launch\nimport \"testing\"\nfunc TestValue(t *testing.T) { t.Parallel() }\n")
					for _, pkg := range []string{"cmd/metasystem", "internal/goal/branch", "internal/a", "internal/b"} {
						impactWrite(t, install, pkg+"/value_test.go", "package fixture\nimport \"testing\"\nfunc TestValue(t *testing.T) { t.Parallel() }\n")
					}
					if err := os.WriteFile(filepath.Join(install, "testing.json"), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
				return result, err
			}
			b.owners.landing.plainProve.Command = func(command *exec.Cmd) error {
				if len(command.Args) > 1 && command.Args[1] == "test" {
					writeImpactPlanResult(t, command, "plan: base main\n"+row.plan)
					return nil
				}
				if got := commandEnv(command, "LANDING_PROOF_SCOPE"); got != row.scope {
					t.Fatalf("scope=%s", got)
				}
				want := "full-fixture"
				if row.scope == "impact" {
					want = "cheap-fixture"
				}
				if command.Args[2] != want {
					t.Fatalf("command=%v", command.Args)
				}
				fmt.Fprint(command.Stdout, "landing environment fixture toolchain\nlanding group fast-static-build green 1\nLANDING-CHECKED\t0\n")
				return nil
			}
			code, out := b.run(t, b.root, "prove", "--wait", "--json")
			var report struct{ Data plain.Result }
			if err := json.Unmarshal([]byte(out), &report); err != nil || code != 0 || report.Data.Scope != row.scope || report.Data.ScopeReason != row.reason {
				t.Fatalf("code=%d report=%s err=%v", code, out, err)
			}
			if row.tiers[0] == 2 && row.tiers[1] == 2 && len(depthTrees) == 0 {
				t.Fatal("the batch depth selection did not create its checkout")
			}
			for _, tree := range depthTrees {
				if _, err := os.Stat(tree); !os.IsNotExist(err) {
					t.Fatalf("depth checkout was not removed: path=%s error=%v", tree, err)
				}
			}
			if len(report.Data.Executions) != 1 || report.Data.Executions[0].DepthScope != row.scope || report.Data.Executions[0].DepthBase != "main" {
				t.Fatalf("batch depth decision was not retained: %+v", report.Data.Executions)
			}
			code, out = b.run(t, b.root, "status")
			want := "proving at " + row.scope + " depth: " + row.reason
			if code != 0 || !strings.Contains(out, want) || !strings.Contains(out, "wall time") || report.Data.Minutes == nil {
				t.Fatalf("status missing %q: %s", want, out)
			}
			t.Log(want)
			if row.scope == "impact" {
				previous := b.owners.landing.plainProve.Git
				pushed := false
				b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
					if strings.Join(args, " ") == "rev-parse --verify refs/remotes/origin/main^{commit}" {
						return "main", nil
					}
					if args[0] == "push" {
						pushed = true
						return "", nil
					}
					return previous(dir, args...)
				}
				code, out = b.run(t, b.root, "push", "--json")
				if code != 0 || !pushed {
					t.Fatalf("automatic impact green refused: %d %s", code, out)
				}
			}
		})
	}
}

func TestSettingsBatchDepthDefaults(t *testing.T) {
	t.Parallel()
	for key, want := range map[string]string{"landing.full-from-tier": "3", "landing.impact-max-share": "50"} {
		if got := config.MustDefault(key); got != want {
			t.Fatalf("%s=%s", key, got)
		}
		for _, setting := range config.CompiledSettings() {
			if setting.Key == key && setting.Meaning == "" {
				t.Fatalf("%s lacks help", key)
			}
		}
	}
}

func TestLandingStatusBatchDepthInFlight(t *testing.T) {
	t.Parallel()
	b := newMergeGateBed(t)
	b.prepareBatch(t)
	seedCurrentTrunkClock(t, b.install, laneTestNow)
	depthGoals(t, b, 3, 3)
	b.owners.landing.plainProve.Launch = func([]string, string, string) (int64, error) { return int64(os.Getpid()), nil }
	b.owners.landing.plainProve.Alive = func(plain.Running) bool { return true }
	code, out := b.run(t, b.root, "prove", "--json")
	if code != 0 {
		t.Fatalf("start=%d %s", code, out)
	}
	code, out = b.run(t, b.root, "status")
	if code != 0 || !strings.Contains(out, "proving at full depth: tier 3: full") {
		t.Fatalf("running status=%d %s", code, out)
	}
}

func TestLandingProveFullPackageMeasureExpandsPackages(t *testing.T) {
	t.Parallel()
	root := impactAdapterBed(t)
	impactWrite(t, root, "internal/extra/extra.go", "package extra\n")
	impactWrite(t, root, "internal/extra/extra_test.go", "package extra\nimport \"testing\"\nfunc TestExtra(t *testing.T) { t.Parallel() }\n")
	contract := testpolicy.Contract{Groups: []testpolicy.Group{{ID: "static"}, {ID: "section/check"}, {ID: "go-affected", Adapter: "go", PackageSelection: "affected-go"}}}
	measure, err := fullPackageMeasure(root, contract)
	count := len(measure)
	if err != nil || count != 4 {
		t.Fatalf("full group count=%d error=%v; want two declared groups and two test packages", count, err)
	}
}

func TestLandingDepthCountsEverySelectedPackage(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, plan string
		share      int
		cheap      bool
	}{
		{"process changes", "selection: cmd/metasystem=TestProcess,TestDeclaration\nselection: internal/goal/branch\nselection: internal/goal\n", 2, false},
		{"two small packages", "selection: internal/a=TestA\nselection: internal/b\n", 1, true},
		{"same package twice", "selection: internal/a=TestA\nselection: internal/a=TestB\nselection: internal/a\n", 0, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			share, cheap := impactDepth(row.plan, testpolicy.Contract{}, depthTestMeasure(110, row.plan), 50)
			if share != row.share || cheap != row.cheap {
				t.Fatalf("share=%d cheap=%v; want %d %v", share, cheap, row.share, row.cheap)
			}
			t.Logf("share=%d%% cheap=%v", share, cheap)
		})
	}
}

func TestLandingDepthFullSelectionHasSameDenominator(t *testing.T) {
	t.Parallel()
	root := impactAdapterBed(t)
	impactWrite(t, root, "internal/extra/extra_test.go", "package extra\nimport \"testing\"\nfunc TestExtra(t *testing.T) { t.Parallel() }\n")
	contract := testpolicy.Contract{Groups: []testpolicy.Group{
		{ID: "static", Adapter: "command"},
		{ID: "named", Adapter: "go", Packages: []string{"./internal/launch"}, Tests: json.RawMessage(`["TestValue"]`)},
		{ID: "all", Adapter: "go", PackageSelection: "affected-go"},
		{ID: "duplicate", Adapter: "go", Packages: []string{"internal/launch"}, Tests: json.RawMessage(`"all"`)},
	}}
	packages, err := fullPackageMeasure(root, contract)
	full := len(packages)
	if err != nil || full != 3 {
		t.Fatalf("full=%d err=%v; want two packages and one command group", full, err)
	}
	measure, err := fullPackageMeasure(root, contract)
	if err != nil {
		t.Fatal(err)
	}
	share, cheap := impactDepth("selection: static\nselection: named\nselection: duplicate\nselection: internal/extra=TestExtra\n", contract, measure, 50)
	if share != 100 || cheap {
		t.Fatalf("full selection share=%d cheap=%v", share, cheap)
	}
	share, cheap = impactDepth("selection: static\nselection: all\n", contract, measure, 50)
	if share != 100 || cheap {
		t.Fatalf("expanded full selection share=%d cheap=%v", share, cheap)
	}
}

func TestLandingDepthExcludesPackagesWithoutTests(t *testing.T) {
	t.Parallel()
	root := impactAdapterBed(t)
	share, cheap, full, _, err := impactCost(root, root, "selection: cmd/metasystem\nselection: canary\nselection: internal/launch=TestValue\n", testpolicy.Contract{Groups: []testpolicy.Group{{ID: "canary", Adapter: "command"}}})
	if err != nil || full != 2 || share != 100 || cheap {
		t.Fatalf("share=%d cheap=%v full=%d err=%v", share, cheap, full, err)
	}
}

func depthTestMeasure(count int, plan string) map[string]bool {
	full := map[string]bool{}
	for _, line := range strings.Split(plan, "\n") {
		if item, ok := strings.CutPrefix(line, "selection: "); ok {
			pkg, _, _ := strings.Cut(item, "=")
			pkg = strings.TrimPrefix(pkg, "metasystem/")
			if pkg == "fast-static-build" {
				pkg = "group/" + pkg
			}
			full[pkg] = true
		}
	}
	for i := 0; len(full) < count; i++ {
		full[fmt.Sprintf("internal/filler%d", i)] = true
	}
	return full
}

func TestLandingDepthRealCanariesDoNotDecideCheapness(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contract testpolicy.Contract
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	canaries := "selection: policy-canary\nselection: adapter-canary\nselection: command-interface-smoke\n"
	small := "selection: internal/a=TestA\nselection: internal/b\n"
	pcc := "selection: cmd/metasystem=TestProcess,TestDeclaration\nselection: internal/goal/branch\nselection: internal/goal\n"
	full := depthTestMeasure(110, small+pcc)
	everything := canaries
	for pkg := range full {
		everything += "selection: " + pkg + "\n"
	}
	for _, row := range []struct {
		name, plan string
		share      int
		cheap      bool
	}{
		{"two small packages plus canaries", small + canaries, 1, true},
		{"process changes plus canaries", pcc + canaries, 2, false},
		{"everything", everything, 100, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			share, cheap := impactDepth(row.plan, contract, full, 50)
			if share != row.share || cheap != row.cheap {
				t.Fatalf("share=%d cheap=%v; want %d %v", share, cheap, row.share, row.cheap)
			}
			t.Logf("share=%d%% cheap=%v", share, cheap)
		})
	}
}

func TestLandingDepthMeasureExcludesNonGoCanaries(t *testing.T) {
	t.Parallel()
	root := impactAdapterBed(t)
	contract := testpolicy.Contract{Groups: []testpolicy.Group{
		{ID: "canary", Adapter: "command"}, {ID: "static", Adapter: "command"},
	}}
	contract.Always.Canary = []string{"canary"}
	full, err := fullPackageMeasure(root, contract)
	if err != nil {
		t.Fatal(err)
	}
	if len(full) != 2 || full["group/canary"] {
		t.Fatalf("measure includes canary: %v", full)
	}
	share, cheap := impactDepth("selection: canary\nselection: static\nselection: internal/launch\n", contract, full, 50)
	if share != 100 || cheap {
		t.Fatalf("everything: share=%d cheap=%v", share, cheap)
	}
	t.Logf("everything: share=%d%% cheap=%v", share, cheap)
}

func TestLandingDepthNamedCommandReasonWithRealCanaries(t *testing.T) {
	t.Parallel()
	contract, err := testpolicy.Load(filepath.Join("..", "..", "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	root := impactAdapterBed(t)
	plan := "selection: cmd/metasystem=TestProcess\n"
	for _, canary := range contract.Always.Canary {
		plan += "selection: " + canary + "\n"
	}
	module, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	share, cheap, full, reason, err := impactCost(root, module, plan, contract)
	if err != nil || cheap || reason != "change selects cmd/metasystem: full" {
		t.Fatalf("share=%d cheap=%v full=%d reason=%q error=%v", share, cheap, full, reason, err)
	}
	t.Logf("%d%% of %d packages; %s", share, full, reason)
}
