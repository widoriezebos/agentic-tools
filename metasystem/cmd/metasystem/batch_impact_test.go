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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch/goadapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func batchPlanBed(t *testing.T) (string, impactSources) {
	t.Helper()
	root := impactAdapterBed(t)
	data, err := os.ReadFile(filepath.Join(root, "testing.json"))
	helmMust(t, err)
	var contract testpolicy.Contract
	helmMust(t, json.Unmarshal(data, &contract))
	for _, id := range append(testpolicy.BatchRosterIDs(), "verb-ratchet", "delegation-layering") {
		group := testpolicy.Group{ID: id, Kind: "unit", Adapter: "go", CWD: ".", Phase: "acceptance", EnvironmentMode: "inherit", Inputs: []string{"cmd/**"}, Platforms: []string{"any"}, TargetMS: 1000, Packages: []string{"internal/launch"}, Tests: json.RawMessage(`["TestValue"]`)}
		if id == "verb-ratchet" || id == "delegation-layering" {
			group.Phase = "admission"
			contract.Always.Canary = append(contract.Always.Canary, id)
		}
		contract.Groups = append(contract.Groups, group)
	}
	data, err = json.Marshal(contract)
	helmMust(t, err)
	impactWrite(t, root, "testing.json", string(data))
	sources := impactSources{
		git: func(_ string, args ...string) (string, error) {
			switch args[0] {
			case "rev-parse":
				return "batch-base", nil
			case "merge-base":
				return "", nil
			case "ls-files":
				return "", nil
			case "show":
				return "batch base subject", nil
			case "diff":
				if !slices.Equal(args, []string{"diff", "--name-only", "--no-renames", "--no-relative", "-z", "batch-base", "--", ":/"}) {
					t.Fatalf("incomplete batch diff: %v", args)
				}
				return "internal/launch/value.go\x00outside/readme.md\x00", nil
			default:
				t.Fatalf("unexpected Git operation %v", args)
				return "", nil
			}
		},
		impact: func(_ string, base string, whole bool) (goadapter.Impact, error) {
			if base != "batch-base" {
				t.Fatalf("base=%s", base)
			}
			tests := json.RawMessage(`["TestValue"]`)
			if whole {
				tests = json.RawMessage(`"all"`)
			}
			return goadapter.Impact{Paths: []string{"internal/launch/value.go"}, Packages: []testpolicy.Group{{ID: "unit/internal/launch", Adapter: "go", Packages: []string{"./internal/launch"}, Tests: tests}}}, nil
		},
		get: func(p config.GetParams) (string, int, error) {
			switch p.Key {
			case "proof.full":
				return "sh proof/full.sh", 0, nil
			case "proof.cheap":
				return "metasystem test impact", 0, nil
			case "landing.impact-max-share":
				return "50", 0, nil
			default:
				return p.Default, 0, nil
			}
		},
		getenv: func(key string) string {
			if key == "LANDING_PROOF_SCOPE" {
				return "impact"
			}
			return ""
		},
	}
	return root, sources
}

func TestBatchPlanKeepsNamedSelectionRosterAndLargeCoverage(t *testing.T) {
	t.Parallel()
	root, sources := batchPlanBed(t)
	// Four of the six measured units are selected, above the seat limit.
	prior := sources.impact
	for _, pkg := range []string{"a", "b", "c"} {
		impactWrite(t, root, "internal/"+pkg+"/value_test.go", "package "+pkg+"\nimport \"testing\"\nfunc TestValue(t *testing.T) { t.Parallel() }\n")
	}
	sources.impact = func(root, base string, whole bool) (goadapter.Impact, error) {
		result, err := prior(root, base, whole)
		for _, pkg := range []string{"a", "b", "c"} {
			result.Packages = append(result.Packages, testpolicy.Group{ID: "unit/internal/" + pkg, Adapter: "go", Packages: []string{"./internal/" + pkg}, Tests: json.RawMessage(`"all"`)})
		}
		return result, err
	}
	plan, err := sources.batchPlan(root, "batch-base")
	helmMust(t, err)
	if plan.Scope != "impact" || plan.Purpose != "batch-impact" || plan.Policy == "" || plan.TestCount != 4 {
		t.Fatalf("plan scope=%s purpose=%s count=%d", plan.Scope, plan.Purpose, plan.TestCount)
	}
	for _, id := range append(testpolicy.BatchRosterIDs(), "verb-ratchet", "delegation-layering", "fast-static-build") {
		if !slices.ContainsFunc(plan.Groups, func(g testpolicy.Group) bool { return g.ID == id }) {
			t.Errorf("missing %s", id)
		}
	}
	i := slices.IndexFunc(plan.Groups, func(g testpolicy.Group) bool { return g.ID == "unit/internal/launch" })
	if i < 0 || string(plan.Groups[i].Tests) != `["TestValue"]` {
		t.Fatal("batch did not use named dependents")
	}
	var out, problem bytes.Buffer
	code := sources.run([]string{"--root", root, "--base", "batch-base", "--plan", "--json"}, &out, &problem, "")
	if code != 0 {
		t.Fatalf("unit plan failed: %s", problem.String())
	}
	if !strings.Contains(out.String(), `"check":"static-only"`) && !strings.Contains(out.String(), `"check": "static-only"`) {
		t.Fatalf("unit size boundary changed: %s", out.String())
	}
	for _, id := range append(testpolicy.BatchRosterIDs(), "verb-ratchet", "delegation-layering") {
		if strings.Contains(out.String(), id) {
			t.Errorf("unit plan acquired %s", id)
		}
	}
}

func TestBatchInstrumentTable(t *testing.T) {
	t.Parallel()
	paths := []string{"go.mod", "go.sum", "metasystem.conf", "proof/full.sh", "cmd/devgate/main.go", "internal/repoproof/full.go", "internal/proofrun/groups.go", "internal/testpolicy/contract.go", "internal/testenv/setup.go", "internal/testutil/fixture.go", "internal/testrun/contract.go", "cmd/metasystem/test_impact.go", "cmd/metasystem/batch_impact.go", "cmd/metasystem/test.go", "cmd/metasystem/publication_requirements.go", "cmd/metasystem/intent_landing_prove.go", "internal/landing/batch/goadapter/impact.go", "internal/gopackages/select.go", "internal/gittree/workspace.go", "internal/pathpattern/path.go", "internal/landing/plain/prove.go", "contracts/alternate.json"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			for _, prefix := range []string{"", "metasystem/"} {
				got := batchInstruments([]string{prefix + path}, prefix, prefix+"contracts/alternate.json")
				if !slices.Equal(got, []string{prefix + path}) {
					t.Fatalf("instrument %s=%v", prefix+path, got)
				}
			}
		})
	}
	if got := batchInstruments([]string{"docs/go.mod", "internal/proofrunner/file.go", "cmd/metasystem/intent_app.go"}, "", "contracts/alternate.json"); len(got) != 0 {
		t.Fatalf("ordinary files are instruments: %v", got)
	}
}

func TestBatchInstrumentMutationsDeletionsAndRenamesSelectRealFull(t *testing.T) {
	t.Parallel()
	for _, paths := range []string{"proof/full.sh\x00", "contracts/alternate.json\x00", "proof/full.sh\x00archive/full.sh\x00", "archive/full.sh\x00proof/full.sh\x00", "metasystem.conf\x00"} {
		t.Run(strings.ReplaceAll(paths, "\x00", "-"), func(t *testing.T) {
			t.Parallel()
			root, sources := batchPlanBed(t)
			impactWrite(t, root, "metasystem.conf", "testing.contract=contracts/alternate.json\n")
			// The contract is absent. Full selection must precede contract loading.
			prior := sources.git
			sources.git = func(root string, args ...string) (string, error) {
				if args[0] == "diff" {
					return paths, nil
				}
				return prior(root, args...)
			}
			sources.impact = func(string, string, bool) (goadapter.Impact, error) {
				t.Fatal("full plan widened impact instead of selecting proof.full")
				return goadapter.Impact{}, nil
			}
			plan, err := sources.batchPlan(root, "base")
			helmMust(t, err)
			if plan.Scope != "full" || plan.Command != "sh proof/full.sh" || len(plan.Groups) != 0 || len(plan.Instruments) == 0 {
				t.Fatalf("full plan=%+v", plan)
			}
		})
	}
}

func TestBatchPlanRejectsExecutionMissingRosterAndBadBase(t *testing.T) {
	t.Parallel()
	root, sources := batchPlanBed(t)
	for _, args := range [][]string{{"--consumer", "batch"}, {"--consumer", "batch", "--plan", "--check"}, {"--consumer", "batch", "--plan", "--publication-only"}, {"--consumer", "unknown", "--plan"}} {
		var out, problem bytes.Buffer
		if code := sources.run(append([]string{"--root", root, "--base", "base"}, args...), &out, &problem, ""); code != 2 {
			t.Fatalf("args=%v code=%d", args, code)
		}
	}
	if _, err := sources.batchPlan(root, ""); err == nil {
		t.Fatal("empty batch base accepted")
	}
	data, err := os.ReadFile(filepath.Join(root, "testing.json"))
	helmMust(t, err)
	impactWrite(t, root, "testing.json", strings.Replace(string(data), `"batch-audits"`, `"missing-batch-audits"`, 1))
	if _, err := sources.batchPlan(root, "base"); err == nil || !strings.Contains(err.Error(), "batch-audits") {
		t.Fatalf("missing roster error=%v", err)
	}
	sources.git = func(string, ...string) (string, error) { return "", fmt.Errorf("unreadable base") }
	if _, err := sources.batchPlan(root, "base"); err == nil {
		t.Fatal("unreadable base accepted")
	}
}

func TestBatchPlanPrintDoesNotRunOrWriteProof(t *testing.T) {
	t.Parallel()
	root, sources := batchPlanBed(t)
	marker := filepath.Join(root, "selection.json")
	impactWrite(t, root, "selection.json", "untouched")
	inherited := sources.getenv
	sources.getenv = func(key string) string {
		if key == "METASYSTEM_CHECK_SELECTION" {
			return marker
		}
		return inherited(key)
	}
	var out, problem bytes.Buffer
	code := sources.run([]string{"--root", root, "--base", "batch-base", "--consumer", "batch", "--plan", "--json"}, &out, &problem, "")
	if code != 0 {
		t.Fatalf("batch plan exit=%d: %s", code, problem.String())
	}
	var result struct {
		Summary string
		Data    batchImpactPlan
	}
	helmMust(t, json.Unmarshal(out.Bytes(), &result))
	if result.Summary != "batch proof proposal; no tests were run" || result.Data.Scope != "impact" || result.Data.TestCount != 1 {
		t.Fatalf("proposal=%s", out.String())
	}
	data, err := os.ReadFile(marker)
	helmMust(t, err)
	if string(data) != "untouched" {
		t.Fatal("planning wrote check evidence")
	}
	if _, err := os.Stat(plain.Dir(root)); !os.IsNotExist(err) {
		t.Fatalf("planning created proof state: %v", err)
	}
}

func TestBatchPlanLandingFrontDoorDoesNotAdmitProof(t *testing.T) {
	t.Parallel()
	b := newMergeGateBed(t)
	b.prepareBatch(t)
	batchPath := filepath.Join(plain.Dir(b.install), "batch.json")
	before, err := os.ReadFile(batchPath)
	helmMust(t, err)
	git := b.owners.landing.plainProve.Git
	b.owners.landing.plainProve.Git = func(root string, args ...string) (string, error) {
		switch args[0] {
		case "rev-parse":
			if args[len(args)-1] == "main^{commit}" {
				return "main", nil
			}
		case "merge-base":
			if args[len(args)-1] == "HEAD" {
				return "", nil
			}
		case "diff":
			return "proof/full.sh\x00", nil
		case "ls-files":
			return "", nil
		}
		return git(root, args...)
	}
	code, out := b.run(t, b.root, "prove", "--plan", "--json")
	var result struct {
		Outcome string
		Data    batchImpactPlan
	}
	helmMust(t, json.Unmarshal([]byte(out), &result))
	if code != 0 || result.Data.Scope != "full" || result.Data.Base != "main" || b.full != 0 || len(b.runs) != 0 {
		t.Fatalf("plan=%d %s, runs=%v", code, out, b.runs)
	}
	after, err := os.ReadFile(batchPath)
	helmMust(t, err)
	if !bytes.Equal(before, after) {
		t.Fatal("plan changed the batch record")
	}
	for _, flag := range []string{"--gate", "--impact", "--trunk", "--wait", "--fresh"} {
		code, _ := b.run(t, b.root, "prove", "--plan", flag, "--json")
		if code != 2 {
			t.Fatalf("execution flag %s with --plan exited %d", flag, code)
		}
	}
}

// Real Git is needed here to pin repository-relative names from a nested
// installation and both sides of a rename, rather than simulate its output.
func TestBatchInstrumentRepositoryDiffGitAdapter(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"mutation", "deletion", "rename", "alternate-contract"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			install := filepath.Join(root, "metasystem")
			impactWrite(t, install, "metasystem.conf", "metasystem.template=true\ntesting.contract=contracts/alternate.json\n")
			impactWrite(t, install, "proof/full.sh", "original")
			impactWrite(t, install, "contracts/alternate.json", "original")
			impactWrite(t, root, "outside.txt", "original")
			testingFixtureGit(t, root, "init", "-q", "-b", "main")
			testingFixtureGit(t, root, "config", "diff.relative", "true")
			testingFixtureGit(t, root, "add", ".")
			testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "base")
			base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
			switch mode {
			case "mutation":
				impactWrite(t, install, "proof/full.sh", "changed")
			case "deletion":
				helmMust(t, os.Remove(filepath.Join(install, "proof/full.sh")))
			case "rename":
				helmMust(t, os.Rename(filepath.Join(install, "proof/full.sh"), filepath.Join(root, "renamed.sh")))
			case "alternate-contract":
				helmMust(t, os.Remove(filepath.Join(install, "contracts/alternate.json")))
			}
			impactWrite(t, root, "outside.txt", "changed")
			sources := impactSources{git: plain.Git, get: func(p config.GetParams) (string, int, error) { return "sh proof/full.sh", 0, nil }}
			plan, err := sources.batchPlan(install, base)
			helmMust(t, err)
			if plan.Scope != "full" || !slices.Contains(plan.Paths, "outside.txt") {
				t.Fatalf("plan=%+v", plan)
			}
			if mode == "rename" && (!slices.Contains(plan.Paths, "metasystem/proof/full.sh") || !slices.Contains(plan.Paths, "renamed.sh")) {
				t.Fatalf("rename paths=%v", plan.Paths)
			}
		})
	}
}

func TestBatchPlanEmptyImpactStillSelectsRoster(t *testing.T) {
	t.Parallel()
	root, sources := batchPlanBed(t)
	sources.impact = func(string, string, bool) (goadapter.Impact, error) { return goadapter.Impact{}, nil }
	plan, err := sources.batchPlan(root, "batch-base")
	helmMust(t, err)
	for _, id := range testpolicy.BatchRosterIDs() {
		if !slices.ContainsFunc(plan.Groups, func(g testpolicy.Group) bool { return g.ID == id }) {
			t.Errorf("empty impact omits %s", id)
		}
	}
}
