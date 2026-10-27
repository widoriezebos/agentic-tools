package goadapter

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestUnitImpactNamesOnlyTestsTheGoToolBuildsHere(t *testing.T) {
	t.Parallel()
	other := "plan9"
	if runtime.GOOS == other {
		other = "windows"
	}
	for _, row := range []struct {
		name, path, constraint, setting string
		keep, lane, whole, cgoImport    bool
	}{
		{name: "constraint", constraint: "!" + runtime.GOOS},
		{name: "file name", path: "internal/direct/reader_" + other + "_test.go"},
		{name: "GOFLAGS tag enabled", constraint: "fixturetag", setting: "GOFLAGS=-tags=fixturetag", keep: true},
		{name: "GOFLAGS tag disabled", constraint: "fixturetag"},
		{name: "environment file", constraint: "fixturetag", setting: "GOENV", keep: true},
		{name: "cgo disabled", constraint: "cgo"},
		{name: "cgo enabled", constraint: "cgo", setting: "CGO_ENABLED=1", keep: true},
		{name: "cgo import disabled", cgoImport: true},
		{name: "cgo import enabled", cgoImport: true, setting: "CGO_ENABLED=1", keep: true},
		{name: "uncertain tool tag", constraint: "go1.999", keep: true},
		{name: "lane mode", path: "cmd/tool/excluded_test.go", constraint: "!" + runtime.GOOS, keep: true, lane: true},
		{name: "whole", path: "internal/launch/excluded_test.go", constraint: "!" + runtime.GOOS, whole: true},
		{name: "unresolved experiment", constraint: "!" + runtime.GOOS, setting: "GOEXPERIMENT=unknownfixture", keep: true},
		{name: "quoted tags", constraint: "!" + runtime.GOOS, setting: "GOFLAGS=-tags='fixturetag'", keep: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write := func(path, source string) {
				t.Helper()
				absolute := filepath.Join(root, path)
				if err := os.MkdirAll(filepath.Dir(absolute), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(absolute, []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
			}
			git := func(args ...string) string {
				t.Helper()
				command := exec.Command("git", append([]string{"-C", root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid"}, args...)...)
				command.Env = gittree.ScrubbedEnviron()
				output, err := command.CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v: %s", args, err, output)
				}
				return strings.TrimSpace(string(output))
			}
			reader := func(pkg, name string) string {
				return "package " + pkg + "_test\nimport (\"testing\"; \"example.invalid/impact/internal/launch\")\nfunc Test" + name + "(t *testing.T) { t.Parallel(); _ = launch.Value }\n"
			}
			write("go.mod", "module example.invalid/impact\n\ngo 1.27\n")
			write("internal/launch/value.go", "package launch\nconst Value = true\n")
			write("internal/direct/value.go", "package direct\nimport _ \"example.invalid/impact/internal/launch\"\n")
			write("cmd/tool/main.go", "package main\nimport _ \"example.invalid/impact/internal/launch\"\nfunc main() {}\n")
			write("internal/direct/reader_test.go", reader("direct", "Reader"))
			write("cmd/tool/reader_test.go", reader("main", "Reader"))
			path := row.path
			if path == "" {
				path = "internal/direct/extra_test.go"
			}
			pkg := "direct"
			if row.lane {
				pkg = "main"
			} else if row.whole {
				pkg = "launch"
			}
			source := reader(pkg, "Extra")
			if row.cgoImport {
				source = strings.Replace(source, "import (", "import \"C\"\nimport (", 1)
			}
			if row.constraint != "" {
				source = "//go:build " + row.constraint + "\n\n" + source
			}
			write(path, source)
			if row.name == "constraint" {
				write("internal/direct/host_test.go", "//go:build "+runtime.GOOS+"\n\n"+reader("direct", "Host"))
			}
			git("init", "-q", "-b", "main")
			git("add", ".")
			git("commit", "-qm", "base")
			base := git("rev-parse", "HEAD")
			write("internal/launch/value.go", "package launch\nconst Value = true // edited\n")
			environment := slices.DeleteFunc(os.Environ(), func(entry string) bool {
				name, _, _ := strings.Cut(entry, "=")
				return slices.Contains([]string{"GOFLAGS", "GOENV", "GOOS", "GOARCH", "CGO_ENABLED", "GOEXPERIMENT"}, name)
			})
			environment = append(environment, "GOENV=off", "CGO_ENABLED=0")
			if row.setting == "GOENV" {
				settings := filepath.Join(t.TempDir(), "goenv")
				if err := os.WriteFile(settings, []byte("GOFLAGS=-tags=fixturetag\n"), 0600); err != nil {
					t.Fatal(err)
				}
				environment = append(environment, "GOENV="+settings)
			} else if row.setting != "" {
				environment = append(environment, row.setting)
			}
			impact, err := unitImpact(root, base, row.lane, environment)
			if err != nil {
				t.Fatal(err)
			}
			if row.lane {
				want := []testpolicy.Group{
					{ID: "unit/cmd/tool", Adapter: "go", CWD: ".", Packages: []string{"./cmd/tool"}, Tests: json.RawMessage(`["TestExtra","TestReader"]`)},
					{ID: "unit/internal/direct", Adapter: "go", CWD: ".", Packages: []string{"./internal/direct"}, Tests: json.RawMessage(`"all"`)},
					{ID: "unit/internal/launch", Adapter: "go", CWD: ".", Packages: []string{"./internal/launch"}, Tests: json.RawMessage(`"all"`)},
				}
				if !reflect.DeepEqual(impact.Packages, want) || impact.NotRunnable != 0 {
					t.Fatalf("lane packages=%+v count=%d; want %+v and zero", impact.Packages, impact.NotRunnable, want)
				}
				return
			}
			want, count := `["TestReader"]`, 1
			if row.whole {
				want, count = `"all"`, 0
			} else if row.keep {
				want, count = `["TestExtra","TestReader"]`, 0
			} else if row.name == "constraint" {
				want = `["TestHost","TestReader"]`
			}
			id := "unit/" + filepath.ToSlash(filepath.Dir(path))
			index := slices.IndexFunc(impact.Packages, func(group testpolicy.Group) bool { return group.ID == id })
			if index < 0 || string(impact.Packages[index].Tests) != want || impact.NotRunnable != count {
				t.Fatalf("selection for %s: packages=%+v count=%d; want tests=%s count=%d", id, impact.Packages, impact.NotRunnable, want, count)
			}
		})
	}
}

func TestTestPackagePathsHonorsBuildTagsAndTestFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for path, source := range map[string]string{
		"go.mod":                     "module example.invalid/count\n\ngo 1.27\n",
		"internaltest/value.go":      "package internaltest\n",
		"internaltest/value_test.go": "package internaltest\nimport \"testing\"\nfunc TestValue(t *testing.T) { t.Parallel() }\n",
		"externaltest/value.go":      "package externaltest\n",
		"externaltest/value_test.go": "package externaltest_test\nimport \"testing\"\nfunc TestValue(t *testing.T) { t.Parallel() }\n",
		"untested/value.go":          "package untested\n",
		"tagged/value.go":            "package tagged\n",
		"tagged/value_test.go":       "//go:build count_one && count_two\n\npackage tagged\nimport \"testing\"\nfunc TestValue(t *testing.T) { t.Parallel() }\n",
	} {
		absolute := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(absolute), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct {
		name string
		tags []string
		want int
	}{
		{name: "default", want: 2},
		{name: "one tag", tags: []string{"count_one"}, want: 2},
		{name: "both tags", tags: []string{"count_one", "count_two"}, want: 3},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			packages, err := TestPackagePaths(root, row.tags)
			count := len(packages)
			if err != nil || count != row.want {
				t.Fatalf("test packages=%d error=%v; want %d", count, err, row.want)
			}
			want := []string{"externaltest", "internaltest"}
			if row.want == 3 {
				want = append(want, "tagged")
			}
			slices.Sort(packages)
			if !slices.Equal(packages, want) {
				t.Fatalf("paths=%v; want %v", packages, want)
			}
		})
	}
}

func TestTestPackagePathsReturnsDiscoveryFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("invalid module declaration\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if packages, err := TestPackagePaths(root, nil); err == nil || len(packages) != 0 {
		t.Fatalf("invalid module: packages=%v error=%v; want no packages and a discovery error", packages, err)
	}
}
