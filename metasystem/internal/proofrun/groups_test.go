package proofrun

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func namedGroupModule(t *testing.T, source string) (string, []string) {
	t.Helper()
	root := t.TempDir()
	for name, data := range map[string]string{
		"go.mod":          "module example.com/named\n\ngo 1.23\n",
		"present_test.go": source,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	tools := t.TempDir()
	if err := os.Symlink(goPath, filepath.Join(tools, "go")); err != nil {
		t.Fatal(err)
	}
	environment := TestingEnvironment(os.Environ(), map[string]string{
		"PATH": tools, "GOWORK": "off", "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOSUMDB": "off",
		"METASYSTEM_TESTING_WORKERS": "2", TestWorkersEnvironment: "2",
	})
	if _, err := ResolveTestingExecutable(context.Background(), root, environment, []string{"git"}); err == nil {
		t.Fatal("fixture has Git on PATH")
	}
	return root, environment
}

func TestANamedTestThatDidNotRunIsRed(t *testing.T) {
	t.Parallel()
	root, environment := namedGroupModule(t, "package named\nimport \"testing\"\nfunc TestPresent(t *testing.T) {}\n")
	group := testpolicy.Group{ID: "named", Adapter: "go", CWD: ".", Packages: []string{"."},
		Tests: json.RawMessage(`["TestPresent","TestAbsent"]`)}
	results, err := RunNamedGroups(context.Background(), root, testpolicy.Contract{SchemaVersion: 2, Groups: []testpolicy.Group{group}}, []string{"named"}, environment)
	if err != nil || len(results) != 1 {
		t.Fatalf("results %+v, error %v", results, err)
	}
	result := results[0]
	if result.Status != "red" || !slices.Contains(result.Reasons, "missing test TestAbsent") {
		t.Fatalf("absent test was not named as red: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(root, "artifacts")); !os.IsNotExist(err) {
		t.Fatalf("plain runner wrote proof artifacts: %v", err)
	}
}

func TestNamedGroupsReturnOnlyRedCommandOutput(t *testing.T) {
	t.Parallel()
	for _, adapter := range []string{"command", "section"} {
		t.Run(adapter, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			groups := []testpolicy.Group{
				{ID: "red", Adapter: adapter, CWD: ".", Format: "exit-status", Argv: []string{"/bin/sh", "-c", "printf 'red stdout\\n'; printf 'red stderr\\n' >&2; exit 1"}},
				{ID: "green", Adapter: adapter, CWD: ".", Format: "exit-status", Argv: []string{"/bin/sh", "-c", "printf 'green output\\n'"}},
				{ID: "bounded", Adapter: adapter, CWD: ".", Format: "exit-status", Argv: []string{"/bin/sh", "-c", "i=1; while [ $i -le 203 ]; do printf 'line %s\\n' $i; i=$((i+1)); done; exit 1"}},
			}
			results, err := RunNamedGroups(context.Background(), root, testpolicy.Contract{Groups: groups}, []string{"red", "green", "bounded"}, []string{"PATH=" + t.TempDir()})
			if err != nil || len(results) != 3 {
				t.Fatalf("results %+v, error %v", results, err)
			}
			if results[0].Status != "red" || results[0].Output != "red stdout\nred stderr\n" || results[1].Status != "green" || results[1].Output != "" {
				t.Fatalf("red diagnostics and silent green: %+v", results)
			}
			var tail strings.Builder
			tail.WriteString("[3 lines omitted]\n")
			for i := 4; i <= 203; i++ {
				fmt.Fprintf(&tail, "line %d\n", i)
			}
			if results[2].Status != "red" || results[2].Output != tail.String() {
				t.Fatalf("bounded diagnostics: %+v", results[2])
			}
		})
	}
}

func TestNamedGroupsReturnFailedGoOutput(t *testing.T) {
	t.Parallel()
	root, environment := namedGroupModule(t, `package named
import "testing"
func TestPresent(t *testing.T) { t.Fatal("failed test diagnostic") }
func TestGreen(t *testing.T) { t.Log("green test diagnostic") }
`)
	group := testpolicy.Group{ID: "go", Adapter: "go", CWD: ".", Packages: []string{"."}, Tests: json.RawMessage(`"all"`)}
	for _, buildFailed := range []bool{false, true} {
		if buildFailed {
			if err := os.WriteFile(filepath.Join(root, "broken.go"), []byte("package named\nvar broken = undefinedBuildDiagnostic\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		results, err := RunNamedGroups(context.Background(), root, testpolicy.Contract{SchemaVersion: 2, Groups: []testpolicy.Group{group}}, []string{"go"}, environment)
		if err != nil || len(results) != 1 || results[0].Status != "red" {
			t.Fatalf("results %+v, error %v", results, err)
		}
		want := "failed test diagnostic"
		if buildFailed {
			want = "undefinedBuildDiagnostic"
		}
		if !strings.Contains(results[0].Output, want) || strings.Contains(results[0].Output, "green test diagnostic") ||
			strings.Contains(results[0].Output, "PASS\n") || strings.Contains(results[0].Output, `"Action":`) {
			t.Fatalf("build failed %v, diagnostics %q", buildFailed, results[0].Output)
		}
	}
}

func TestNamedGroupsRunPrerequisitesAndCommands(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	command := func(id, script string) testpolicy.Group {
		return testpolicy.Group{ID: id, Adapter: "command", CWD: ".", Argv: []string{"/bin/sh", "-c", script}, Format: "exit-status"}
	}
	green := command("green", "exit 0")
	red := command("red", "exit 1")
	dependent := command("dependent", "echo ran > dependent")
	dependent.Requires = []string{"red"}
	template := command("template", "echo ran > template")
	template.PackageSelection = "changed-and-consumers"
	contract := testpolicy.Contract{Groups: []testpolicy.Group{green, red, dependent, template}}
	environment := []string{"PATH=" + t.TempDir()}
	results, err := RunNamedGroups(context.Background(), root, contract, []string{"green", "dependent", "red"}, environment)
	if err != nil || len(results) != 3 {
		t.Fatalf("results %+v, error %v", results, err)
	}
	for i, want := range []string{"green green", "red red", "dependent red"} {
		if got := results[i].ID + " " + results[i].Status; got != want || results[i].DurationMS < 0 {
			t.Errorf("result %d = %+v, want %s", i, results[i], want)
		}
	}
	if !strings.Contains(strings.Join(results[1].Reasons, " "), "exit status 1") ||
		!strings.Contains(strings.Join(results[2].Reasons, " "), "prerequisite red") {
		t.Fatalf("failure reasons %+v", results)
	}
	if _, err := os.Stat(filepath.Join(root, "dependent")); !os.IsNotExist(err) {
		t.Fatalf("dependent of red prerequisite ran: %v", err)
	}
	sentinel := command("sentinel", "echo ran > sentinel")
	contract.Groups = append(contract.Groups, sentinel)
	for _, ids := range [][]string{nil, {"sentinel", "unknown"}, {"sentinel", "template"}} {
		results, err := RunNamedGroups(context.Background(), root, contract, ids, environment)
		if err == nil || len(results) != 0 || len(ids) > 0 && !strings.Contains(err.Error(), ids[1]) {
			t.Fatalf("selection %v: results %+v, error %v", ids, results, err)
		}
		if _, err := os.Stat(filepath.Join(root, "sentinel")); !os.IsNotExist(err) {
			t.Fatal("refused selection ran a command")
		}
	}
	section := command("section", "exit 0")
	section.Adapter = "section"
	results, err = RunNamedGroups(context.Background(), root, testpolicy.Contract{Groups: []testpolicy.Group{section}}, []string{"section"}, environment)
	if err != nil || len(results) != 1 || results[0].Status != "green" {
		t.Fatalf("section results %+v, error %v", results, err)
	}
	if err := os.Mkdir(filepath.Join(root, "reports"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, verdict := range []struct{ xml, status, reason string }{
		{"<testcase classname='fixture' name='TestReport'/>", "green", ""},
		{"<testcase classname='fixture' name='TestReport'><failure/></testcase>", "red", "failed test fixture.TestReport"},
		{"<testcase classname='fixture' name='TestReport'><skipped/></testcase>", "red", "fixture.TestReport skipped"},
		{"", "red", "missing test fixture.TestReport"},
	} {
		report := command("report", "printf '%s' \"<testsuite>"+verdict.xml+"</testsuite>\" > reports/result.xml")
		report.Format, report.Reports = "junit-xml", []string{"reports"}
		report.ExpectedTests = []testpolicy.ExpectedTest{{Report: "reports/result.xml", Classname: "fixture", Name: "TestReport"}}
		results, err = RunNamedGroups(context.Background(), root, testpolicy.Contract{Groups: []testpolicy.Group{report}}, []string{"report"}, environment)
		if err != nil || len(results) != 1 || results[0].Status != verdict.status ||
			!strings.Contains(strings.Join(results[0].Reasons, " "), verdict.reason) {
			t.Fatalf("report %s: results %+v, error %v", verdict.xml, results, err)
		}
	}
}

func TestNamedGroupsGoOptionsAndDiagnosticRerun(t *testing.T) {
	t.Parallel()
	root, environment := namedGroupModule(t, `//go:build scoped

package named
import ("os"; "testing")
func TestPresent(t *testing.T) {
	if os.Getenv("GROUP_VALUE") != "group" || os.Getenv("BASE_ONLY") != "" { t.Fatal("wrong environment") }
	if _, err := os.Stat("first"); os.IsNotExist(err) {
		if err := os.WriteFile("first", []byte("failed"), 0600); err != nil { t.Fatal(err) }
		t.Fatal("first run fails")
	}
}
`)
	group := testpolicy.Group{ID: "options", Adapter: "go", CWD: ".", Packages: []string{"."}, BuildTags: []string{"scoped"},
		Tests: json.RawMessage(`"all"`), EnvironmentMode: "explicit", Shards: 2, Coverage: true, Env: map[string]string{"GROUP_VALUE": "group"}}
	for _, entry := range environment {
		name, value, _ := strings.Cut(entry, "=")
		group.Env[name] = value
	}
	// The worker allowance belongs to the runner, not the group declaration.
	delete(group.Env, TestWorkersEnvironment)
	environment = append(environment, "BASE_ONLY=should-not-reach-test")
	results, err := RunNamedGroups(context.Background(), root, testpolicy.Contract{SchemaVersion: 2, Groups: []testpolicy.Group{group}}, []string{"options"}, environment)
	if err != nil || len(results) != 1 {
		t.Fatalf("results %+v, error %v", results, err)
	}
	reasons := strings.Join(results[0].Reasons, "\n")
	if results[0].Status != "red" || !strings.Contains(reasons, "failed test example.com/named.TestPresent") ||
		!strings.Contains(reasons, "diagnostic rerun example.com/named.TestPresent: passed") {
		t.Fatalf("initial failure must stay red after diagnostic pass: %+v", results)
	}
	if !slices.ContainsFunc(results[0].Reasons, func(reason string) bool { return strings.HasPrefix(reason, "exit status ") }) {
		t.Fatalf("native exit not reported: %+v", results)
	}
	results, err = RunNamedGroups(context.Background(), root, testpolicy.Contract{SchemaVersion: 2, Groups: []testpolicy.Group{group}}, []string{"options"}, environment)
	if err != nil || len(results) != 1 || results[0].Status != "green" || results[0].Output != "" {
		t.Fatalf("passing tagged test must be green without coverage: %+v, %v", results, err)
	}
}
