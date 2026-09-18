package audit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const stopSurfaceListPath = "scripts/agents/stop-decision-surface.txt"

var completeStopGoPatternFragments = []string{
	"ShouldBlock",
	"BlockSource",
	"IdleRefusal",
	"CountSpent",
	`\\?"decision\\?"\s*:\s*\\?"(?:block|allow)\\?"`,
	`\[\s*"decision"\s*\]\s*(?:==|!=)\s*"(?:block|allow)"`,
}

var requiredRepositoryStopSurfaceAssertions = []struct {
	file  string
	text  string
	count int
}{
	{file: "cmd/metasystem/context_cost_test.go", text: "if code != 0 || problem != \"\" || decodeErr != nil || verdict.ShouldBlock ||"},
	{file: "cmd/metasystem/wait_verb_test.go", text: "if !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != \"unwatched-work\" ||"},
	{file: "cmd/metasystem/wait_verb_test.go", text: "if verdict.ShouldBlock || verdict.BlockSource != nil || verdict.IdleRefusal || verdict.CountSpent ||"},
	{file: "internal/report/stoppresentation_test.go", text: "if result.Control.ShouldBlock != input.Control.ShouldBlock || !sameStringPointer(result.Control.BlockSource, input.Control.BlockSource) {"},
	{file: "cmd/metasystem/wait_verb_test.go", text: "if err != nil || strings.Contains(string(hookOutput), `\"decision\":\"block\"`) || strings.Contains(string(hookOutput), \"needs supervision repair\") {"},
	{file: "cmd/metasystem/wait_verb_test.go", text: "if strings.Contains(liveHook.stdout, `\"decision\":\"block\"`) || artifactErr != nil || !strings.Contains(string(liveArtifact), \"WAITING: installed local build\") {"},
	{file: "cmd/metasystem/wait_verb_test.go", text: "if !strings.Contains(deadHook.stdout, `\"decision\":\"block\"`) || artifactErr != nil || strings.Contains(string(deadArtifact), \"WAITING: installed local build\") {"},
	{file: "cmd/metasystem/wait_verb_test.go", text: "if strings.Contains(humanHook.stdout, `\"decision\":\"block\"`) || artifactErr != nil || !strings.Contains(string(humanArtifact), \"WAITING: human answer to May the installed run stop?\") {"},
	{file: "cmd/metasystem/wait_verb_test.go", text: "if !strings.Contains(controlHook.stdout, `\"decision\":\"block\"`) {"},
	{file: "cmd/metasystem/wait_verb_test.go", text: "if strings.Contains(allowedHook.stdout, `\"decision\":\"block\"`) {", count: 2},
	{file: "cmd/metasystem/wait_verb_test.go", text: "if !strings.Contains(absentHook.stdout, `\"decision\":\"block\"`) {"},
	{file: "internal/stopreport/response_test.go", text: "payload = []byte(`{\"decision\":\"block\",\"reason\":\"visible\"}`)"},
	{file: "internal/adapter/stopoutput_test.go", text: "payloadObject = map[string]any{\"decision\": \"block\", \"reason\": presentation.HumanLine}"},
	{file: "internal/report/stopblock_test.go", text: "if b[\"decision\"] != \"block\" {"},
	{file: "internal/hooks/setup_test.go", text: "!strings.Contains(text, `hook-bootstrap-failed`) || strings.Contains(text, `\\\"decision\\\":\\\"block\\\"`) {"},
}

type stopSurfaceFixture struct {
	t    *testing.T
	root string
}

func newStopSurfaceFixture(t *testing.T, files []stopSurfaceFile, contents map[string]string, includeList bool) *stopSurfaceFixture {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	fixture := &stopSurfaceFixture{t: t, root: t.TempDir()}
	fixture.git("init", "-q", "-b", "main")
	fixture.git("config", "--local", "user.name", "Stop Surface Fixture")
	fixture.git("config", "--local", "user.email", "stop-surface@invalid")
	fixture.git("config", "--local", "commit.gpgsign", "false")
	fixture.write("go.mod", "module fixture\n\ngo 1.25\n")
	for path, content := range contents {
		fixture.write(path, content)
	}
	if includeList {
		fixture.write(stopSurfaceListPath, renderStopSurfaceList(files))
	}
	fixture.write("plans/goals/move-stop.md", "# Move Stop\n")
	fixture.commit("base")
	return fixture
}

func renderStopSurfaceList(files []stopSurfaceFile) string {
	var lines []string
	for _, file := range files {
		lines = append(lines, file.Kind+"\t"+file.Path)
	}
	return strings.Join(lines, "\n") + "\n"
}

func (f *stopSurfaceFixture) write(path, content string) {
	f.t.Helper()
	absolute := filepath.Join(f.root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *stopSurfaceFixture) git(args ...string) string {
	f.t.Helper()
	command := exec.Command("git", append([]string{"-C", f.root}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	output, err := command.CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func (f *stopSurfaceFixture) commit(message string) string {
	f.t.Helper()
	f.git("add", "-A")
	f.git("commit", "-q", "-m", message)
	return f.git("rev-parse", "HEAD")
}

func (f *stopSurfaceFixture) audit(options StopSurfaceOptions) StopSurfaceResult {
	f.t.Helper()
	result, err := AuditStopDecisionSurface(f.root, options)
	if err != nil {
		f.t.Fatal(err)
	}
	return result
}

func (f *stopSurfaceFixture) declare(goal, reason string, moves []StopSurfaceLine) string {
	f.t.Helper()
	rows := declarationRows(moves)
	digest := stopMoveDigest(rows)
	path := filepath.ToSlash(filepath.Join(stopMoveDirectory, goal+"-"+digest[:12]+".txt"))
	f.write(path, "goal: "+goal+"\nreason: "+reason+"\nmoved:\n"+strings.Join(rows, "\n")+"\n")
	return path
}

func standardStopFiles() []stopSurfaceFile {
	return []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}, {Kind: "bed", Path: "scripts/agents/bed-fixtures.sh"}}
}

func goStopSurfaceFixture(packageName, body string) string {
	var source strings.Builder
	fmt.Fprintf(&source, "package %s\n\nfunc TestFixture() {\n", packageName)
	for _, line := range strings.Split(strings.TrimSuffix(body, "\n"), "\n") {
		if line != "" {
			source.WriteByte('\t')
			source.WriteString(line)
			source.WriteByte('\n')
		}
	}
	source.WriteString("}\n")
	return source.String()
}

func TestStopSurfaceReportsAdditions(t *testing.T) {
	fixture := newStopSurfaceFixture(t, standardStopFiles(), map[string]string{
		"a_test.go":                      goStopSurfaceFixture("fixture", ""),
		"scripts/agents/bed-fixtures.sh": "#!/usr/bin/env bash\n",
	}, true)
	fixture.write("a_test.go", goStopSurfaceFixture("fixture", "want := Verdict{\tShouldBlock:   true}\n"))
	fixture.write("scripts/agents/bed-fixtures.sh", "#!/usr/bin/env bash\nwant='{\"decision\":\"block\"}'\n")

	result := fixture.audit(StopSurfaceOptions{})
	if result.Refused() {
		t.Fatalf("additive surface refused: %+v", result)
	}
	want := []StopSurfaceLine{
		{File: "a_test.go", Line: "want := Verdict{ ShouldBlock: true}"},
		{File: "scripts/agents/bed-fixtures.sh", Line: `want='{"decision":"block"}'`},
	}
	if !slices.Equal(result.Added, want) {
		t.Fatalf("added = %#v, want %#v", result.Added, want)
	}
}

func TestStopSurfaceDiscoversOnlyFixtureBeds(t *testing.T) {
	fixture := newStopSurfaceFixture(t, nil, map[string]string{
		"scripts/agents/nested/base-fixtures.sh": "#!/usr/bin/env bash\n",
		"scripts/agents/emit.sh":                 "#!/usr/bin/env bash\n",
	}, false)
	fixture.write("scripts/agents/nested/base-fixtures.sh", "#!/usr/bin/env bash\nwant='{\"decision\":\"allow\"}'\n")
	fixture.write("scripts/agents/emit.sh", "#!/usr/bin/env bash\nprintf '{\"decision\":\"block\"}'\n")

	result := fixture.audit(StopSurfaceOptions{})
	want := []StopSurfaceLine{{File: "scripts/agents/nested/base-fixtures.sh", Line: `want='{"decision":"allow"}'`}}
	if result.Refused() || !slices.Equal(result.Added, want) {
		t.Fatalf("fixture-bed discovery result = %#v, want additions %#v", result, want)
	}
}

func TestStopSurfaceDiscoversAssertionsOutsideTheOldList(t *testing.T) {
	const path = "nested/new_decision_test.go"
	fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
		"a_test.go": goStopSurfaceFixture("fixture", ""),
	}, true)
	fixture.write(path, goStopSurfaceFixture("nested", "want := Verdict{ShouldBlock: true}\n"))

	added := fixture.audit(StopSurfaceOptions{})
	wantAdded := StopSurfaceLine{File: path, Line: "want := Verdict{ShouldBlock: true}"}
	if added.Refused() || !slices.Contains(added.Added, wantAdded) {
		t.Fatalf("unlisted addition result = %+v, want addition %+v", added, wantAdded)
	}

	fixture.commit("add assertion outside the old list")
	fixture.write(path, goStopSurfaceFixture("nested", ""))
	removed := fixture.audit(StopSurfaceOptions{})
	if !removed.Refused() || !slices.Contains(removed.Removed, wantAdded) {
		t.Fatalf("unlisted removal result = %+v, want refusal for %+v", removed, wantAdded)
	}

	fixture.write(path, goStopSurfaceFixture("nested", "want := Verdict{ShouldBlock: false}\n"))
	inverted := fixture.audit(StopSurfaceOptions{})
	wantInverted := StopSurfaceLine{File: path, Line: "want := Verdict{ShouldBlock: false}"}
	if !inverted.Refused() || !slices.Contains(inverted.Removed, wantAdded) || !slices.Contains(inverted.Added, wantInverted) {
		t.Fatalf("unlisted inversion result = %+v, want removal %+v and addition %+v", inverted, wantAdded, wantInverted)
	}
}

const (
	hookWireBlockAssertion = `if strings.Contains(out, "\"decision\":\"block\"") {}`
	hookWireAllowAssertion = `if strings.Contains(out, "\"decision\":\"allow\"") {}`
)

func TestStopSurfaceReportsHookWireDecisionAddition(t *testing.T) {
	const path = "nested/wire_decision_test.go"
	fixture := newStopSurfaceFixture(t, nil, map[string]string{
		path: goStopSurfaceFixture("nested", ""),
	}, false)
	fixture.write(path, goStopSurfaceFixture("nested", hookWireBlockAssertion))

	result := fixture.audit(StopSurfaceOptions{})
	want := StopSurfaceLine{File: path, Line: hookWireBlockAssertion}
	if result.Refused() || !slices.Equal(result.Added, []StopSurfaceLine{want}) {
		t.Fatalf("wire-decision addition result = %+v, want only %+v", result, want)
	}
}

func TestStopSurfaceRefusesHookWireDecisionRemoval(t *testing.T) {
	const path = "nested/wire_decision_test.go"
	fixture := newStopSurfaceFixture(t, nil, map[string]string{
		path: goStopSurfaceFixture("nested", hookWireBlockAssertion),
	}, false)
	fixture.write(path, goStopSurfaceFixture("nested", ""))

	result := fixture.audit(StopSurfaceOptions{})
	want := StopSurfaceLine{File: path, Line: hookWireBlockAssertion}
	if !result.Refused() || !slices.Equal(result.Removed, []StopSurfaceLine{want}) {
		t.Fatalf("wire-decision removal result = %+v, want refusal for %+v", result, want)
	}
}

func TestStopSurfaceRefusesHookWireDecisionInversion(t *testing.T) {
	const path = "nested/wire_decision_test.go"
	fixture := newStopSurfaceFixture(t, nil, map[string]string{
		path: goStopSurfaceFixture("nested", hookWireBlockAssertion),
	}, false)
	fixture.write(path, goStopSurfaceFixture("nested", hookWireAllowAssertion))

	result := fixture.audit(StopSurfaceOptions{})
	wantRemoved := StopSurfaceLine{File: path, Line: hookWireBlockAssertion}
	wantAdded := StopSurfaceLine{File: path, Line: hookWireAllowAssertion}
	if !result.Refused() || !slices.Equal(result.Removed, []StopSurfaceLine{wantRemoved}) ||
		!slices.Equal(result.Added, []StopSurfaceLine{wantAdded}) {
		t.Fatalf("wire-decision inversion result = %+v, want removal %+v and addition %+v", result, wantRemoved, wantAdded)
	}
}

func TestStopSurfaceIgnoresFileLevelDecisionMetadata(t *testing.T) {
	const path = "nested/metadata_test.go"
	fixture := newStopSurfaceFixture(t, nil, map[string]string{
		path: goStopSurfaceFixture("nested", ""),
	}, false)
	candidate := "package nested\n\n" +
		"var (\n\tstopMetadata = []Verdict{{ShouldBlock: false}}\n\twireMetadata = []string{`{\"decision\": \"block\"}`}\n)\n\n" +
		"const (\n\tShouldBlock = true\n)\n\n" +
		"func TestFixture() {\n\t" + hookWireAllowAssertion + "\n}\n"
	fixture.write(path, candidate)

	result := fixture.audit(StopSurfaceOptions{})
	want := StopSurfaceLine{File: path, Line: hookWireAllowAssertion}
	if result.Refused() || !slices.Equal(result.Added, []StopSurfaceLine{want}) {
		t.Fatalf("file-level metadata result = %+v, want only real assertion %+v", result, want)
	}
}

func TestStopSurfaceIgnoresCommentedDecisionText(t *testing.T) {
	const path = "nested/comment_test.go"
	fixture := newStopSurfaceFixture(t, nil, map[string]string{
		path: goStopSurfaceFixture("nested", ""),
	}, false)
	body := "// if verdict.ShouldBlock { panic(\"block\") }\n" + hookWireAllowAssertion
	fixture.write(path, goStopSurfaceFixture("nested", body))

	result := fixture.audit(StopSurfaceOptions{})
	want := StopSurfaceLine{File: path, Line: hookWireAllowAssertion}
	if result.Refused() || !slices.Equal(result.Added, []StopSurfaceLine{want}) {
		t.Fatalf("commented decision result = %+v, want only real assertion %+v", result, want)
	}
}

func TestStopSurfaceIgnoresSurfaceLineMetadata(t *testing.T) {
	const path = "nested/surface_line_metadata_test.go"
	fixture := newStopSurfaceFixture(t, nil, map[string]string{
		path: goStopSurfaceFixture("nested", ""),
	}, false)
	body := "want := []StopSurfaceLine{{File: \"other_test.go\", Line: `if strings.Contains(out, \"decision\":\"allow\") {}`}}\n" + hookWireBlockAssertion
	fixture.write(path, goStopSurfaceFixture("nested", body))

	result := fixture.audit(StopSurfaceOptions{})
	want := StopSurfaceLine{File: path, Line: hookWireBlockAssertion}
	if result.Refused() || !slices.Equal(result.Added, []StopSurfaceLine{want}) {
		t.Fatalf("surface-line metadata result = %+v, want only real assertion %+v", result, want)
	}
}

func TestStopSurfaceIgnoresEmbeddedPackageSource(t *testing.T) {
	const path = "nested/package_source_test.go"
	fixture := newStopSurfaceFixture(t, nil, map[string]string{
		path: goStopSurfaceFixture("nested", ""),
	}, false)
	body := `fixture.write(path, "package nested; var payload = \"decision\":\"allow\"")` + "\n" + hookWireBlockAssertion
	fixture.write(path, goStopSurfaceFixture("nested", body))

	result := fixture.audit(StopSurfaceOptions{})
	want := StopSurfaceLine{File: path, Line: hookWireBlockAssertion}
	if result.Refused() || !slices.Equal(result.Added, []StopSurfaceLine{want}) {
		t.Fatalf("embedded package source result = %+v, want only real assertion %+v", result, want)
	}
}

func TestStopSurfaceIgnoresEmbeddedEscapedNewlineSource(t *testing.T) {
	const path = "nested/newline_source_test.go"
	fixture := newStopSurfaceFixture(t, nil, map[string]string{
		path: goStopSurfaceFixture("nested", ""),
	}, false)
	body := `fixture.write(path, "var payload = \"decision\":\"allow\"\n")` + "\n" + hookWireBlockAssertion
	fixture.write(path, goStopSurfaceFixture("nested", body))

	result := fixture.audit(StopSurfaceOptions{})
	want := StopSurfaceLine{File: path, Line: hookWireBlockAssertion}
	if result.Refused() || !slices.Equal(result.Added, []StopSurfaceLine{want}) {
		t.Fatalf("embedded escaped-newline source result = %+v, want only real assertion %+v", result, want)
	}
}

func TestStopSurfaceRefusesAnUndeclaredRemoval(t *testing.T) {
	fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
		"a_test.go": goStopSurfaceFixture("fixture", "first := Verdict{ShouldBlock: true}\nsecond := Verdict{BlockSource: source}\n"),
	}, true)
	fixture.write("a_test.go", goStopSurfaceFixture("fixture", ""))

	result := fixture.audit(StopSurfaceOptions{})
	if !result.Refused() || len(result.Removed) != 2 {
		t.Fatalf("removal result = %+v", result)
	}
	report := fmt.Sprintf("%s\n%s", result.Removed[0].Line, result.Removed[1].Line)
	for _, line := range []string{"first := Verdict{ShouldBlock: true}", "second := Verdict{BlockSource: source}"} {
		if !strings.Contains(report, line) {
			t.Errorf("report does not name %q: %s", line, report)
		}
	}
}

func TestStopSurfaceRefusesAnInvertedDecision(t *testing.T) {
	tests := []struct {
		name string
		kind string
		old  string
		new  string
	}{
		{name: "expectation", kind: "go", old: "want := Verdict{ShouldBlock: true}\n", new: "want := Verdict{ShouldBlock: false}\n"},
		{name: "condition", kind: "go", old: "if !verdict.ShouldBlock { panic(\"allowed\") }\n", new: "if verdict.ShouldBlock { panic(\"blocked\") }\n"},
		{name: "bed", kind: "bed", old: "want='{\"decision\":\"block\"}'\n", new: "want='{\"decision\":\"allow\"}'\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := "a_test.go"
			if test.kind == "bed" {
				path = "scripts/agents/bed-fixtures.sh"
			}
			oldContent, newContent := test.old, test.new
			if test.kind == "go" {
				oldContent = goStopSurfaceFixture("fixture", oldContent)
				newContent = goStopSurfaceFixture("fixture", newContent)
			}
			fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: test.kind, Path: path}}, map[string]string{path: oldContent}, true)
			fixture.write(path, newContent)
			result := fixture.audit(StopSurfaceOptions{})
			if !result.Refused() || len(result.Removed) != 1 || len(result.Added) != 1 {
				t.Fatalf("inversion result = %+v", result)
			}
		})
	}
}

func TestStopSurfaceAdmitsADeclaredMove(t *testing.T) {
	t.Run("declared removal", func(t *testing.T) {
		fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
			"a_test.go": goStopSurfaceFixture("fixture", "want := Verdict{ShouldBlock: true}\n"),
		}, true)
		fixture.write("a_test.go", goStopSurfaceFixture("fixture", ""))
		fixture.declare("move-stop", "the Stop policy changed", []StopSurfaceLine{{File: "a_test.go", Line: "want := Verdict{ShouldBlock: true}"}})

		result := fixture.audit(StopSurfaceOptions{})
		if result.Refused() || len(result.Moved) != 1 || result.Moved[0].Goal != "move-stop" || len(result.Removed) != 0 {
			t.Fatalf("declared move result = %+v", result)
		}
	})

	t.Run("reorder", func(t *testing.T) {
		fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
			"a_test.go": goStopSurfaceFixture("fixture", "first := Verdict{ShouldBlock: true}\nsecond := Verdict{BlockSource: source}\n"),
		}, true)
		fixture.write("a_test.go", goStopSurfaceFixture("fixture", "second := Verdict{BlockSource: source}\nfirst := Verdict{ShouldBlock: true}\n"))
		result := fixture.audit(StopSurfaceOptions{})
		if result.Refused() || len(result.Added) != 0 || len(result.Moved) != 0 {
			t.Fatalf("reorder changed the surface: %+v", result)
		}
	})
}

func TestStopSurfaceRefusesAFalseDeclaration(t *testing.T) {
	removed := StopSurfaceLine{File: "a_test.go", Line: "want := Verdict{ShouldBlock: true}"}
	tests := []struct {
		name  string
		build func(*stopSurfaceFixture)
	}{
		{name: "declared line was not removed", build: func(f *stopSurfaceFixture) {
			f.declare("move-stop", "policy", []StopSurfaceLine{removed})
		}},
		{name: "removed line missing", build: func(f *stopSurfaceFixture) {
			f.write("a_test.go", goStopSurfaceFixture("fixture", ""))
			f.declare("move-stop", "policy", []StopSurfaceLine{{File: "a_test.go", Line: "other := Verdict{ShouldBlock: true}"}})
		}},
		{name: "declaration already in base", build: func(f *stopSurfaceFixture) {
			f.declare("move-stop", "policy", []StopSurfaceLine{removed})
			f.commit("stale declaration")
			f.write("a_test.go", goStopSurfaceFixture("fixture", ""))
		}},
		{name: "goal without ledger", build: func(f *stopSurfaceFixture) {
			f.write("a_test.go", goStopSurfaceFixture("fixture", ""))
			f.declare("missing-goal", "policy", []StopSurfaceLine{removed})
		}},
		{name: "empty reason", build: func(f *stopSurfaceFixture) {
			f.write("a_test.go", goStopSurfaceFixture("fixture", ""))
			rows := declarationRows([]StopSurfaceLine{removed})
			digest := stopMoveDigest(rows)
			f.write(filepath.ToSlash(filepath.Join(stopMoveDirectory, "move-stop-"+digest[:12]+".txt")),
				"goal: move-stop\nreason: \nmoved:\n"+strings.Join(rows, "\n")+"\n")
		}},
		{name: "digest mismatch", build: func(f *stopSurfaceFixture) {
			f.write("a_test.go", goStopSurfaceFixture("fixture", ""))
			f.write(filepath.ToSlash(filepath.Join(stopMoveDirectory, "move-stop-000000000000.txt")),
				"goal: move-stop\nreason: policy\nmoved:\na_test.go\twant := Verdict{ShouldBlock: true}\n")
		}},
		{name: "unknown key", build: func(f *stopSurfaceFixture) {
			f.write("a_test.go", goStopSurfaceFixture("fixture", ""))
			rows := declarationRows([]StopSurfaceLine{removed})
			digest := stopMoveDigest(rows)
			f.write(filepath.ToSlash(filepath.Join(stopMoveDirectory, "move-stop-"+digest[:12]+".txt")),
				"goal: move-stop\nreason: policy\nunknown: value\nmoved:\n"+strings.Join(rows, "\n")+"\n")
		}},
		{name: "missing moved section", build: func(f *stopSurfaceFixture) {
			f.write("a_test.go", goStopSurfaceFixture("fixture", ""))
			f.write(filepath.ToSlash(filepath.Join(stopMoveDirectory, "move-stop-000000000000.txt")),
				"goal: move-stop\nreason: policy\n")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
				"a_test.go": goStopSurfaceFixture("fixture", removed.Line+"\n"),
			}, true)
			test.build(fixture)
			if result := fixture.audit(StopSurfaceOptions{}); !result.Refused() {
				t.Fatalf("false declaration passed: %+v", result)
			}
		})
	}
}

func TestStopSurfaceListRemovalCountsAsRemoval(t *testing.T) {
	fixture := newStopSurfaceFixture(t, standardStopFiles(), map[string]string{
		"a_test.go":                      goStopSurfaceFixture("fixture", "want := Verdict{ShouldBlock: true}\n"),
		"scripts/agents/bed-fixtures.sh": "#!/usr/bin/env bash\n",
	}, true)
	fixture.write(stopSurfaceListPath, renderStopSurfaceList([]stopSurfaceFile{{Kind: "bed", Path: "scripts/agents/bed-fixtures.sh"}}))

	result := fixture.audit(StopSurfaceOptions{})
	if result.Refused() || len(result.Removed) != 0 {
		t.Fatalf("obsolete list changed discovered surface: %+v", result)
	}
}

func TestStopSurfaceFirstLandingUsesTheCandidateList(t *testing.T) {
	fixture := newStopSurfaceFixture(t, nil, map[string]string{
		"a_test.go": goStopSurfaceFixture("fixture", "first := Verdict{ShouldBlock: true}\n"),
	}, false)
	fixture.write(stopSurfaceListPath, renderStopSurfaceList([]stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}))
	fixture.write("a_test.go", goStopSurfaceFixture("fixture", "first := Verdict{ShouldBlock: true}\nsecond := Verdict{BlockSource: source}\n"))

	result := fixture.audit(StopSurfaceOptions{})
	if result.Refused() || len(result.Added) != 1 || result.Added[0].Line != "second := Verdict{BlockSource: source}" {
		t.Fatalf("first landing result = %+v", result)
	}
}

func TestStopSurfaceResolvesItsBase(t *testing.T) {
	t.Run("explicit base wins", func(t *testing.T) {
		fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
			"a_test.go": goStopSurfaceFixture("fixture", "want := Verdict{ShouldBlock: true}\n"),
		}, true)
		base := fixture.git("rev-parse", "HEAD")
		fixture.write("a_test.go", goStopSurfaceFixture("fixture", "want := Verdict{ShouldBlock: false}\n"))
		fixture.commit("inversion")
		result := fixture.audit(StopSurfaceOptions{Base: base})
		if result.Base != base || !result.Refused() || len(result.Removed) != 1 {
			t.Fatalf("explicit base result = %+v", result)
		}
	})

	t.Run("origin main merge base includes branch commits", func(t *testing.T) {
		fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
			"a_test.go": goStopSurfaceFixture("fixture", ""),
		}, true)
		base := fixture.git("rev-parse", "HEAD")
		fixture.git("update-ref", "refs/remotes/origin/main", base)
		fixture.write("a_test.go", goStopSurfaceFixture("fixture", "first := Verdict{ShouldBlock: true}\n"))
		fixture.commit("first branch commit")
		fixture.write("a_test.go", goStopSurfaceFixture("fixture", "first := Verdict{ShouldBlock: true}\nsecond := Verdict{BlockSource: source}\n"))
		fixture.commit("second branch commit")
		result := fixture.audit(StopSurfaceOptions{})
		if result.Base != base || result.Refused() || len(result.Added) != 2 {
			t.Fatalf("merge-base result = %+v", result)
		}
	})

	t.Run("missing origin main falls back to HEAD", func(t *testing.T) {
		fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
			"a_test.go": goStopSurfaceFixture("fixture", "first := Verdict{ShouldBlock: true}\n"),
		}, true)
		head := fixture.git("rev-parse", "HEAD")
		fixture.write("a_test.go", goStopSurfaceFixture("fixture", "first := Verdict{ShouldBlock: true}\nsecond := Verdict{BlockSource: source}\n"))
		result := fixture.audit(StopSurfaceOptions{})
		if result.Base != head || result.Refused() || len(result.Added) != 1 {
			t.Fatalf("HEAD fallback result = %+v", result)
		}
	})
}

func TestStopSurfaceDeclareWritesAnAcceptedDeclaration(t *testing.T) {
	t.Run("writes an accepted declaration", func(t *testing.T) {
		fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
			"a_test.go": goStopSurfaceFixture("fixture", "want := Verdict{ShouldBlock: true}\n"),
		}, true)
		fixture.write("a_test.go", goStopSurfaceFixture("fixture", ""))
		path, err := DeclareStopDecisionSurface(fixture.root, StopSurfaceOptions{}, "move-stop", "the policy changed")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(path, stopMoveDirectory+"/move-stop-") {
			t.Fatalf("declaration path = %q", path)
		}
		result := fixture.audit(StopSurfaceOptions{})
		if result.Refused() || len(result.Moved) != 1 {
			t.Fatalf("generated declaration result = %+v", result)
		}
	})

	t.Run("refuses no removals", func(t *testing.T) {
		fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
			"a_test.go": goStopSurfaceFixture("fixture", "want := Verdict{ShouldBlock: true}\n"),
		}, true)
		if _, err := DeclareStopDecisionSurface(fixture.root, StopSurfaceOptions{}, "move-stop", "policy"); err == nil {
			t.Fatal("declaration passed without a removal")
		}
	})

	t.Run("refuses missing goal", func(t *testing.T) {
		fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
			"a_test.go": goStopSurfaceFixture("fixture", "want := Verdict{ShouldBlock: true}\n"),
		}, true)
		fixture.write("a_test.go", goStopSurfaceFixture("fixture", ""))
		if _, err := DeclareStopDecisionSurface(fixture.root, StopSurfaceOptions{}, "missing-goal", "policy"); err == nil {
			t.Fatal("declaration passed without a ledger goal")
		}
	})

	t.Run("refuses empty reason", func(t *testing.T) {
		fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
			"a_test.go": goStopSurfaceFixture("fixture", "want := Verdict{ShouldBlock: true}\n"),
		}, true)
		fixture.write("a_test.go", goStopSurfaceFixture("fixture", ""))
		if _, err := DeclareStopDecisionSurface(fixture.root, StopSurfaceOptions{}, "move-stop", ""); err == nil {
			t.Fatal("declaration passed with an empty reason")
		}
	})
}

func TestGoGateRunsTheStopDecisionSurfaceCheck(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "go-gate.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(content)
	hook := strings.Index(script, `audit hook-start-exits --root "$root"`)
	stop := strings.Index(script, `audit stop-decision-surface --root "$root"`)
	if hook < 0 || stop <= hook {
		t.Fatalf("Stop surface audit is not after the hook-start audit: hook=%d stop=%d", hook, stop)
	}
	for _, required := range []string{
		`gate_static_reds+=("Stop decision surface audit failed:`,
		`"$gate_hook_start_scope" == installation`,
		`git -C "$root" rev-parse --is-inside-work-tree`,
		`Stop decision surface audit not applicable`,
		`printf '%s\n' "$gate_stop_surface_out"`,
	} {
		if !strings.Contains(script[hook:], required) {
			t.Errorf("go gate lacks %q", required)
		}
	}
	if strings.Contains(script[hook:], "stop-decision-surface.txt") {
		t.Error("go gate still makes the audit depend on the removed include list")
	}
}

func TestStopSurfaceListOfThisRepositoryIsSound(t *testing.T) {
	root := filepath.Join("..", "..")
	files, err := discoverStopSurfaceFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	surface, err := extractStopSurface(files, func(path string) ([]byte, bool, error) {
		data, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		return data, readErr == nil, readErr
	})
	if err != nil {
		t.Fatal(err)
	}

	if got, want := stopGoPattern.String(), strings.Join(completeStopGoPatternFragments, "|"); got != want {
		t.Fatalf("Stop decision token pattern = %q, want complete set %q", got, want)
	}
	for _, fragment := range completeStopGoPatternFragments {
		pattern := regexp.MustCompile(fragment)
		found := false
		for line := range surface {
			if pattern.MatchString(line.Line) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("discovered repository surface has no extracted line for %s", fragment)
		}
	}

	discovered := map[string]bool{}
	for _, file := range files {
		discovered[file.Path] = true
	}
	for _, oldBed := range []string{
		"scripts/agents/runtime-hook-fixtures.sh",
		"scripts/agents/supervision-fixtures.sh",
		"scripts/agents/supervision-hook-fixtures.sh",
	} {
		if !discovered[oldBed] {
			t.Errorf("discovery omitted fixture bed %s", oldBed)
		}
	}

	for _, assertion := range requiredRepositoryStopSurfaceAssertions {
		content, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(assertion.file)))
		if readErr != nil {
			t.Errorf("read %s: %v", assertion.file, readErr)
			continue
		}
		lines := strings.Split(string(content), "\n")
		lineNumber := 0
		for index, line := range lines {
			if strings.Join(strings.Fields(line), " ") == assertion.text {
				lineNumber = index + 1
				break
			}
		}
		if lineNumber == 0 {
			t.Errorf("%s no longer contains %q", assertion.file, assertion.text)
			continue
		}
		wantCount := assertion.count
		if wantCount == 0 {
			wantCount = 1
		}
		if got := surface[stopSurfaceKey{File: assertion.file, Line: assertion.text}]; got != wantCount {
			t.Errorf("discovered surface count for %s:%d = %d, want %d: %s", assertion.file, lineNumber, got, wantCount, assertion.text)
		}
	}
}
