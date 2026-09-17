package batch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

func gatePatch(paths ...string) []byte {
	var patch strings.Builder
	for _, name := range paths {
		to := "b/" + name
		if strings.HasPrefix(name, "!") {
			name = strings.TrimPrefix(name, "!")
			to = "/dev/null"
		}
		fmt.Fprintf(&patch, "diff --git a/%s b/%s\n--- a/%s\n+++ %s\n@@ -1 +1 @@\n--- metasystem/not-a-header.go\n+++ metasystem/not-a-header.go\n", name, name, name, to)
	}
	return []byte(patch.String())
}
func readRepo(t *testing.T, path ...string) []byte {
	data, err := os.ReadFile(filepath.Join(append([]string{"..", "..", ".."}, path...)...))
	must(t, err)
	return data
}
func fixtureMap(t *testing.T) []byte {
	return readRepo(t, "scripts", "agents", "fixture-bed-groups.tsv")
}
func successfulGate(calls *[]gateStep) batchGateExec {
	return func(_ string, step gateStep) gateStepResult {
		*calls = append(*calls, step)
		return gateStepResult{RunID: fmt.Sprintf("self-%d", len(*calls))}
	}
}
func TestBatchJoinRunsItsOwnGate(t *testing.T) {
	patch := append(gatePatch("metasystem/internal/landing/batch/gate.go", "!metasystem/internal/refusal/register.go", "metasystem/scripts/agents/other-fixtures.sh"), []byte("diff --git a/metasystem/scripts/agents/runtime-hook-fixtures.sh b/metasystem/scripts/agents/runtime-hook-fixtures.sh\nold mode 100644\nnew mode 100755\ndiff --git a/metasystem/scripts/agents/land-fixtures.sh b/metasystem/scripts/agents/land-checks.sh\nsimilarity index 100%\nrename from metasystem/scripts/agents/land-fixtures.sh\nrename to metasystem/scripts/agents/land-checks.sh\ndiff --git a/metasystem/cmd/legacy/file.go b/metasystem/internal/landing/batch/renamed.go\nsimilarity index 100%\nrename from metasystem/cmd/legacy/file.go\nrename to metasystem/internal/landing/batch/renamed.go\n")...)
	groups := append(fixtureMap(t), []byte("scripts/agents/other-fixtures.sh\tsection/other-fixtures\n")...)
	var calls []gateStep
	var trees []string
	unit := Unit{Gate: []string{"supplied"}}
	execute := successfulGate(&calls)
	must(t, runJoinGate("unit-tree", patch, groups, &unit, func(tree string, step gateStep) gateStepResult {
		trees = append(trees, tree)
		return execute(tree, step)
	}))
	if !slices.Equal(unit.Gate, []string{"self-1", "self-2", "self-3", "self-4", "self-5", "self-6", "self-7"}) || len(calls) != 7 || !slices.Equal(trees, []string{"unit-tree", "unit-tree", "unit-tree", "unit-tree", "unit-tree", "unit-tree", "unit-tree"}) {
		t.Fatalf("gate runs=%v calls=%v trees=%v", unit.Gate, calls, trees)
	}
	if !slices.Equal(calls[0].Args, []string{"bash", "scripts/agents/go-gate.sh", "--fast"}) ||
		!slices.Equal(calls[1].Args, []string{"go", "test", "-count=1", "-timeout", "900s", "./cmd/..."}) ||
		!slices.Equal(calls[2].Args, []string{"go", "test", "-count=1", "-timeout", "900s", "./internal/..."}) ||
		!slices.Equal(calls[3].Args, []string{"go", "test", "-count=1", "-timeout", "900s", "./internal/landing/batch"}) ||
		!slices.Equal(calls[4].Args, []string{"bin/metasystem", "test", "run", "--root", ".", "--purpose", "diagnostic", "--groups", "section/land-fixtures", "--mode", "canary", "--tree", "unit-tree"}) ||
		!slices.Equal(calls[5].Args, []string{"bin/metasystem", "test", "run", "--root", ".", "--purpose", "diagnostic", "--groups", "section/other-fixtures", "--mode", "canary", "--tree", "unit-tree"}) ||
		!slices.Equal(calls[6].Args, []string{"bin/metasystem", "test", "run", "--root", ".", "--purpose", "diagnostic", "--groups", "section/supervision-and-census-fixtures", "--mode", "canary", "--tree", "unit-tree"}) {
		t.Fatalf("gate commands=%v", calls)
	}
}

func TestBatchJoinRefusesRedStep(t *testing.T) {
	unit := Unit{}
	err := runJoinGate("unit-tree", gatePatch("metasystem/internal/landing/batch/gate.go"), fixtureMap(t), &unit,
		func(_ string, step gateStep) gateStepResult {
			if strings.Contains(step.Name, "./internal/landing/batch") {
				return gateStepResult{RunID: "red-package", ExitCode: 1}
			}
			return gateStepResult{RunID: "green-fast"}
		})
	if err == nil || !strings.Contains(err.Error(), "BATCH_JOIN_GATE_RED") || !strings.Contains(err.Error(), "./internal/landing/batch") {
		t.Fatalf("red package refusal=%v", err)
	}
	err = runJoinGate("unit-tree", nil, fixtureMap(t), &unit, func(string, gateStep) gateStepResult { return gateStepResult{} })
	if err == nil || !strings.Contains(err.Error(), "returned no run id") {
		t.Fatalf("missing run refusal=%v", err)
	}
}

func TestBatchJoinRefusesUnmappedFixtureBed(t *testing.T) {
	for _, path := range []string{"metasystem/scripts/agents/unmapped-fixtures.sh", "metasystem/scripts/root-fixtures.sh"} {
		var calls []gateStep
		err := runJoinGate("unit-tree", gatePatch(path), fixtureMap(t), &Unit{}, successfulGate(&calls))
		if err == nil || !strings.Contains(err.Error(), "BATCH_JOIN_GATE_RED") || !strings.Contains(err.Error(), strings.TrimPrefix(path, "metasystem/")) || len(calls) != 0 {
			t.Errorf("unmapped bed %s: refusal=%v calls=%v", path, err, calls)
		}
	}
}

func TestBatchJoinIgnoresSuppliedResults(t *testing.T) {
	tree := t.TempDir()
	prior := filepath.Join(tree, "artifacts", "agents", "proof-runs", "supplied.json")
	must(t, os.MkdirAll(filepath.Dir(prior), 0o755))
	must(t, os.WriteFile(prior, []byte(`{"runId":"supplied"}`), 0o644))
	var calls []gateStep
	unit := Unit{Gate: []string{"supplied"}}
	patch := gatePatch("metasystem/internal/landing/batch/gate.go", "metasystem/scripts/agents/runtime-hook-fixtures.sh")
	must(t, runJoinGate(tree, patch, fixtureMap(t), &unit, successfulGate(&calls)))
	if len(calls) != 3 || !slices.Equal(unit.Gate, []string{"self-1", "self-2", "self-3"}) {
		t.Fatalf("supplied result shortened or entered gate: calls=%d runs=%v", len(calls), unit.Gate)
	}
}

func TestFixtureGroupsForChangedBeds(t *testing.T) {
	groups := parseFixtureBedGroups(fixtureMap(t))
	cases := []struct {
		name, path string
		want       fixtureSelection
	}{
		{"shared bed", "metasystem/scripts/watch-background-jobs.sh", fixtureSelection{Groups: []string{"section/watch-background-jobs-fixtures", "section/workflow-tooling-fixtures"}}},
		{"scenario harness", "metasystem/scripts/agents/fixture-bed-scenarios.sh", fixtureSelection{CoveredByProof: true}},
		{"harness", "metasystem/scripts/agents/fixture-budget.sh", fixtureSelection{CoveredByProof: true}},
		{"assert harness", "metasystem/scripts/agents/fixture-assert.sh", fixtureSelection{CoveredByProof: true}},
		{"go only", "metasystem/internal/landing/batch/gate.go", fixtureSelection{}},
	}
	for _, test := range cases {
		if got := fixtureGroupsForChangedBeds(patchChange{test.path: false}, groups); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s selection=%v, want %v", test.name, got, test.want)
		}
	}
}

var sectionScriptPattern = regexp.MustCompile(`^scripts/(?:agents/)?[[:alnum:]_.-]+\.sh$`)

func scriptsInSection(source string) []string {
	var paths []string
	for _, line := range strings.Split(source, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		for _, field := range strings.Fields(line) {
			path := strings.TrimPrefix(strings.Trim(field, `"'();`), "$root/")
			if sectionScriptPattern.MatchString(path) {
				paths = append(paths, path)
			}
		}
	}
	return paths
}

func fixtureRowsFromSuite(suite []byte) []string {
	text := strings.ReplaceAll(string(suite), "\\\n", " ")
	rows := map[string]bool{}
	offset := 0
	for _, match := range regexp.MustCompile(`(?m)\brun_section[ \t]+(\S+-fixtures)[ \t]+\S+[ \t]+(\S+)[^\n]*`).FindAllStringSubmatch(text, -1) {
		at := offset + strings.Index(text[offset:], match[0])
		source := match[0]
		if start := strings.Index(text[:at], "\n"+match[2]+"() {"); start >= 0 {
			start += len(match[2]) + 5
			if end := strings.LastIndex(text[start:at], "\n}\n"); end >= 0 {
				source += text[start : start+end]
			}
		}
		offset = at + len(match[0])
		for _, script := range scriptsInSection(source) {
			if !isSharedFixtureHarness(script) {
				rows[script+"\tsection/"+match[1]] = true
			}
		}
	}
	result := make([]string, 0, len(rows))
	for row := range rows {
		result = append(result, row)
	}
	sort.Strings(result)
	return result
}

func TestFixtureBedGroupMapMatchesSections(t *testing.T) {
	want := fixtureRowsFromSuite(readRepo(t, "scripts", "validate-metasystem.sh"))
	mapPath := filepath.Join("..", "..", "..", "scripts", "agents", "fixture-bed-groups.tsv")
	if os.Getenv("UPDATE_FIXTURE_BED_GROUPS") == "1" {
		must(t, os.WriteFile(mapPath, []byte(strings.Join(want, "\n")+"\n"), 0o644))
	}
	got := strings.Split(strings.TrimSpace(string(fixtureMap(t))), "\n")
	sort.Strings(got)
	if !slices.Equal(got, want) {
		t.Fatalf("fixture bed map=%v, section beds=%v", got, want)
	}
	var contract struct {
		Groups []struct {
			ID string `json:"id"`
		} `json:"groups"`
	}
	must(t, json.Unmarshal(readRepo(t, "testing.json"), &contract))
	known := map[string]bool{}
	for _, group := range contract.Groups {
		known[group.ID] = true
	}
	for _, row := range want {
		if group := strings.SplitN(row, "\t", 2)[1]; !known[group] {
			t.Errorf("fixture group %s is absent from testing.json", group)
		}
	}
}
