package audit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

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
	return []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}, {Kind: "bed", Path: "bed.sh"}}
}

func TestStopSurfaceReportsAdditions(t *testing.T) {
	fixture := newStopSurfaceFixture(t, standardStopFiles(), map[string]string{
		"a_test.go": "package fixture\n",
		"bed.sh":    "#!/usr/bin/env bash\n",
	}, true)
	fixture.write("a_test.go", "package fixture\nwant := Verdict{\tShouldBlock:   true}\n")
	fixture.write("bed.sh", "#!/usr/bin/env bash\nwant='{\"decision\":\"block\"}'\n")

	result := fixture.audit(StopSurfaceOptions{})
	if result.Refused() {
		t.Fatalf("additive surface refused: %+v", result)
	}
	want := []StopSurfaceLine{
		{File: "a_test.go", Line: "want := Verdict{ ShouldBlock: true}"},
		{File: "bed.sh", Line: `want='{"decision":"block"}'`},
	}
	if !slices.Equal(result.Added, want) {
		t.Fatalf("added = %#v, want %#v", result.Added, want)
	}
}

func TestStopSurfaceRefusesAnUndeclaredRemoval(t *testing.T) {
	fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
		"a_test.go": "first := Verdict{ShouldBlock: true}\nsecond := Verdict{BlockSource: source}\n",
	}, true)
	fixture.write("a_test.go", "package fixture\n")

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
				path = "bed.sh"
			}
			fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: test.kind, Path: path}}, map[string]string{path: test.old}, true)
			fixture.write(path, test.new)
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
			"a_test.go": "want := Verdict{ShouldBlock: true}\n",
		}, true)
		fixture.write("a_test.go", "package fixture\n")
		fixture.declare("move-stop", "the Stop policy changed", []StopSurfaceLine{{File: "a_test.go", Line: "want := Verdict{ShouldBlock: true}"}})

		result := fixture.audit(StopSurfaceOptions{})
		if result.Refused() || len(result.Moved) != 1 || result.Moved[0].Goal != "move-stop" || len(result.Removed) != 0 {
			t.Fatalf("declared move result = %+v", result)
		}
	})

	t.Run("reorder", func(t *testing.T) {
		fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
			"a_test.go": "first := Verdict{ShouldBlock: true}\nsecond := Verdict{BlockSource: source}\n",
		}, true)
		fixture.write("a_test.go", "second := Verdict{BlockSource: source}\nfirst := Verdict{ShouldBlock: true}\n")
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
			f.write("a_test.go", "package fixture\n")
			f.declare("move-stop", "policy", []StopSurfaceLine{{File: "a_test.go", Line: "other := Verdict{ShouldBlock: true}"}})
		}},
		{name: "declaration already in base", build: func(f *stopSurfaceFixture) {
			f.declare("move-stop", "policy", []StopSurfaceLine{removed})
			f.commit("stale declaration")
			f.write("a_test.go", "package fixture\n")
		}},
		{name: "goal without ledger", build: func(f *stopSurfaceFixture) {
			f.write("a_test.go", "package fixture\n")
			f.declare("missing-goal", "policy", []StopSurfaceLine{removed})
		}},
		{name: "empty reason", build: func(f *stopSurfaceFixture) {
			f.write("a_test.go", "package fixture\n")
			rows := declarationRows([]StopSurfaceLine{removed})
			digest := stopMoveDigest(rows)
			f.write(filepath.ToSlash(filepath.Join(stopMoveDirectory, "move-stop-"+digest[:12]+".txt")),
				"goal: move-stop\nreason: \nmoved:\n"+strings.Join(rows, "\n")+"\n")
		}},
		{name: "digest mismatch", build: func(f *stopSurfaceFixture) {
			f.write("a_test.go", "package fixture\n")
			f.write(filepath.ToSlash(filepath.Join(stopMoveDirectory, "move-stop-000000000000.txt")),
				"goal: move-stop\nreason: policy\nmoved:\na_test.go\twant := Verdict{ShouldBlock: true}\n")
		}},
		{name: "unknown key", build: func(f *stopSurfaceFixture) {
			f.write("a_test.go", "package fixture\n")
			rows := declarationRows([]StopSurfaceLine{removed})
			digest := stopMoveDigest(rows)
			f.write(filepath.ToSlash(filepath.Join(stopMoveDirectory, "move-stop-"+digest[:12]+".txt")),
				"goal: move-stop\nreason: policy\nunknown: value\nmoved:\n"+strings.Join(rows, "\n")+"\n")
		}},
		{name: "missing moved section", build: func(f *stopSurfaceFixture) {
			f.write("a_test.go", "package fixture\n")
			f.write(filepath.ToSlash(filepath.Join(stopMoveDirectory, "move-stop-000000000000.txt")),
				"goal: move-stop\nreason: policy\n")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
				"a_test.go": removed.Line + "\n",
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
		"a_test.go": "want := Verdict{ShouldBlock: true}\n",
		"bed.sh":    "#!/usr/bin/env bash\n",
	}, true)
	fixture.write(stopSurfaceListPath, renderStopSurfaceList([]stopSurfaceFile{{Kind: "bed", Path: "bed.sh"}}))

	result := fixture.audit(StopSurfaceOptions{})
	if !result.Refused() || len(result.Removed) != 1 || result.Removed[0].File != "a_test.go" {
		t.Fatalf("list removal result = %+v", result)
	}
}

func TestStopSurfaceFirstLandingUsesTheCandidateList(t *testing.T) {
	fixture := newStopSurfaceFixture(t, nil, map[string]string{
		"a_test.go": "first := Verdict{ShouldBlock: true}\n",
	}, false)
	fixture.write(stopSurfaceListPath, renderStopSurfaceList([]stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}))
	fixture.write("a_test.go", "first := Verdict{ShouldBlock: true}\nsecond := Verdict{BlockSource: source}\n")

	result := fixture.audit(StopSurfaceOptions{})
	if result.Refused() || len(result.Added) != 1 || result.Added[0].Line != "second := Verdict{BlockSource: source}" {
		t.Fatalf("first landing result = %+v", result)
	}
}

func TestStopSurfaceResolvesItsBase(t *testing.T) {
	t.Run("explicit base wins", func(t *testing.T) {
		fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
			"a_test.go": "want := Verdict{ShouldBlock: true}\n",
		}, true)
		base := fixture.git("rev-parse", "HEAD")
		fixture.write("a_test.go", "want := Verdict{ShouldBlock: false}\n")
		fixture.commit("inversion")
		result := fixture.audit(StopSurfaceOptions{Base: base})
		if result.Base != base || !result.Refused() || len(result.Removed) != 1 {
			t.Fatalf("explicit base result = %+v", result)
		}
	})

	t.Run("origin main merge base includes branch commits", func(t *testing.T) {
		fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
			"a_test.go": "package fixture\n",
		}, true)
		base := fixture.git("rev-parse", "HEAD")
		fixture.git("update-ref", "refs/remotes/origin/main", base)
		fixture.write("a_test.go", "package fixture\nfirst := Verdict{ShouldBlock: true}\n")
		fixture.commit("first branch commit")
		fixture.write("a_test.go", "package fixture\nfirst := Verdict{ShouldBlock: true}\nsecond := Verdict{BlockSource: source}\n")
		fixture.commit("second branch commit")
		result := fixture.audit(StopSurfaceOptions{})
		if result.Base != base || result.Refused() || len(result.Added) != 2 {
			t.Fatalf("merge-base result = %+v", result)
		}
	})

	t.Run("missing origin main falls back to HEAD", func(t *testing.T) {
		fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
			"a_test.go": "first := Verdict{ShouldBlock: true}\n",
		}, true)
		head := fixture.git("rev-parse", "HEAD")
		fixture.write("a_test.go", "first := Verdict{ShouldBlock: true}\nsecond := Verdict{BlockSource: source}\n")
		result := fixture.audit(StopSurfaceOptions{})
		if result.Base != head || result.Refused() || len(result.Added) != 1 {
			t.Fatalf("HEAD fallback result = %+v", result)
		}
	})
}

func TestStopSurfaceDeclareWritesAnAcceptedDeclaration(t *testing.T) {
	t.Run("writes an accepted declaration", func(t *testing.T) {
		fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
			"a_test.go": "want := Verdict{ShouldBlock: true}\n",
		}, true)
		fixture.write("a_test.go", "package fixture\n")
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
			"a_test.go": "want := Verdict{ShouldBlock: true}\n",
		}, true)
		if _, err := DeclareStopDecisionSurface(fixture.root, StopSurfaceOptions{}, "move-stop", "policy"); err == nil {
			t.Fatal("declaration passed without a removal")
		}
	})

	t.Run("refuses missing goal", func(t *testing.T) {
		fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
			"a_test.go": "want := Verdict{ShouldBlock: true}\n",
		}, true)
		fixture.write("a_test.go", "package fixture\n")
		if _, err := DeclareStopDecisionSurface(fixture.root, StopSurfaceOptions{}, "missing-goal", "policy"); err == nil {
			t.Fatal("declaration passed without a ledger goal")
		}
	})

	t.Run("refuses empty reason", func(t *testing.T) {
		fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "a_test.go"}}, map[string]string{
			"a_test.go": "want := Verdict{ShouldBlock: true}\n",
		}, true)
		fixture.write("a_test.go", "package fixture\n")
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
		`-f "$root/scripts/agents/stop-decision-surface.txt"`,
		`Stop decision surface audit not applicable`,
		`printf '%s\n' "$gate_stop_surface_out"`,
	} {
		if !strings.Contains(script[hook:], required) {
			t.Errorf("go gate lacks %q", required)
		}
	}
}

func TestStopSurfaceListOfThisRepositoryIsSound(t *testing.T) {
	root := filepath.Join("..", "..")
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(stopSurfaceListPath)))
	if err != nil {
		t.Fatal(err)
	}
	files, err := parseStopSurfaceList(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 13 {
		t.Fatalf("surface list has %d files, want 13", len(files))
	}
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil || !info.Mode().IsRegular() {
			t.Errorf("listed file %s is not a regular file", file.Path)
			continue
		}
		if file.Kind != "go" {
			continue
		}
		surface, err := extractStopSurface([]stopSurfaceFile{file}, func(path string) ([]byte, bool, error) {
			data, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
			return data, readErr == nil, readErr
		})
		if err != nil {
			t.Errorf("extract %s: %v", file.Path, err)
		} else if len(surface) == 0 {
			t.Errorf("listed Go test %s has no Stop surface lines", file.Path)
		}
	}
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	if unique := slices.Compact(sorted); len(unique) != len(paths) {
		t.Fatal("surface list has a duplicate path")
	}
}
