package pathclass

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func oneRepositoryRootResult(t *testing.T, expected repositoryRootRequest, output []byte, resultErr error) repositoryRootRunner {
	t.Helper()
	expected.Args = append([]string(nil), expected.Args...)
	expected.Environment = append([]string(nil), expected.Environment...)
	output = append([]byte(nil), output...)
	var mu sync.Mutex
	calls := 0
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		if calls != 1 {
			t.Errorf("repository root runner received %d calls; want exactly one", calls)
		}
	})
	return func(request repositoryRootRequest) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls != 1 || !reflect.DeepEqual(request, expected) {
			t.Errorf("repository root request %d = %+v; want %+v", calls, request, expected)
			return nil, errors.New("unexpected repository root request")
		}
		return append([]byte(nil), output...), resultErr
	}
}

func TestDiscoverRepositoryRootWithRunner(t *testing.T) {
	installation := t.TempDir()
	steering := []string{
		"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_CEILING_DIRECTORIES",
		"GIT_DISCOVERY_ACROSS_FILESYSTEM", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES",
		"GIT_CONFIG", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM",
		"GIT_CONFIG_NOSYSTEM", "GIT_GRAFT_FILE", "GIT_SHALLOW_FILE", "GIT_REPLACE_REF_BASE",
		"GIT_IMPLICIT_WORK_TREE", "GIT_NO_REPLACE_OBJECTS", "GIT_PREFIX",
	}
	for _, name := range steering {
		t.Setenv(name, "hostile")
	}
	t.Setenv("PATHCLASS_SENTINEL", "retained")
	expected := repositoryRootRequest{Directory: installation, Args: []string{"rev-parse", "--show-toplevel"}, Environment: scrubGitSteering(os.Environ())}
	for _, entry := range expected.Environment {
		name, _, _ := strings.Cut(entry, "=")
		for _, forbidden := range steering {
			if name == forbidden {
				t.Fatalf("Git steering %s survived scrubbing", name)
			}
		}
	}
	if !containsEnvironment(expected.Environment, "PATHCLASS_SENTINEL=retained") {
		t.Fatal("unrelated environment entry was dropped")
	}
	for _, test := range []struct {
		name, output, want string
		err                error
	}{
		{name: "absolute output", output: "  " + installation + "\n", want: installation},
		{name: "relative output", output: "  .\n", want: mustWorkingDirectory(t)},
		{name: "command error", output: "fatal: no repository\n", err: errors.New("exit status 128"), want: "path class: installation is not inside a Git repository: fatal: no repository"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, err := discoverRepositoryRootWithRunner(installation, oneRepositoryRootResult(t, expected, []byte(test.output), test.err))
			if test.err != nil {
				if err == nil || err.Error() != test.want {
					t.Fatalf("error = %v; want %q", err, test.want)
				}
			} else if err != nil || root != test.want {
				t.Fatalf("root = %q, error = %v; want %q", root, err, test.want)
			}
		})
	}
	if _, err := discoverRepositoryRootWithRunner(installation, nil); err == nil || !strings.Contains(err.Error(), "runner is required") {
		t.Fatalf("nil runner error = %v", err)
	}
}

func containsEnvironment(environment []string, entry string) bool {
	for _, candidate := range environment {
		if candidate == entry {
			return true
		}
	}
	return false
}

func mustWorkingDirectory(t *testing.T) string {
	t.Helper()
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return directory
}

func TestLongestPrefixWins(t *testing.T) {
	manifest := parseTestManifest(t, `
install:memory/ record
install:memory/README.md behavior
install:plans/ record
install:plans/goals/ ledger
`)
	for _, test := range []struct {
		key       string
		wantClass Class
		wantRow   string
	}{
		{key: "memory/note.md", wantClass: Record, wantRow: "install:memory/"},
		{key: "memory/README.md", wantClass: Behavior, wantRow: "install:memory/README.md"},
		{key: "plans/design.md", wantClass: Record, wantRow: "install:plans/"},
		{key: "plans/goals/x.md", wantClass: Ledger, wantRow: "install:plans/goals/"},
	} {
		got := manifest.Resolve(Install, test.key)
		if got.Class != test.wantClass || got.Row != test.wantRow {
			t.Errorf("Resolve(install, %q) = %+v; want class %s row %s", test.key, got, test.wantClass, test.wantRow)
		}
	}
}

func TestRowKindsAreDistinctKeySpaces(t *testing.T) {
	manifest := parseTestManifest(t, `
install:metasystem runtime
install:cmd/ behavior
repo:metasystem record
`)
	if got := manifest.Resolve(Install, "metasystem"); got.Class != Runtime {
		t.Fatalf("install:metasystem = %+v; want runtime", got)
	}
	if got := manifest.Resolve(Repo, "metasystem"); got.Class != Record {
		t.Fatalf("repo:metasystem = %+v; want record", got)
	}

	repository := t.TempDir()
	installation := filepath.Join(repository, "metasystem")
	if err := os.MkdirAll(filepath.Join(installation, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := resolveAt(manifest, installation, repository, Template, stateroot.OwnerMetasystem, filepath.Join(installation, "cmd", "x.go"))
	if err != nil || got.Class != Behavior || got.Namespace != Install || got.Key != "cmd/x.go" {
		t.Fatalf("template installation path resolved as %+v, %v; want install:cmd/x.go behavior", got, err)
	}
}

func TestTierOneFloorsAreASeparateSet(t *testing.T) {
	manifest := parseTestManifest(t, `
install:internal/ behavior
floor:internal/landing/ tier-1-refused
floor:metasystem.conf tier-1-refused
`)
	for _, key := range []string{"internal/landing", "internal/landing/observe.go", "metasystem.conf"} {
		if !manifest.TierOneRefused(key) {
			t.Errorf("TierOneRefused(%q) = false; want true", key)
		}
	}
	if manifest.TierOneRefused("internal/goal/file.go") {
		t.Fatal("an unrelated behavior path was placed on the tier-1 floor")
	}
	if got := manifest.Class("internal/landing/observe.go"); got != Behavior {
		t.Fatalf("floor row changed the four-class answer to %s", got)
	}
}

func TestAdoptedModeAnswersOutside(t *testing.T) {
	repository := t.TempDir()
	manifest := parseTestManifest(t, "install:docs/ behavior\n")
	got, err := resolveAt(manifest, repository, repository, Adopted, stateroot.OwnerApp, "docs/application.md")
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != Outside || got.Mode != Adopted {
		t.Fatalf("application-owned docs/application.md resolved as %+v; want outside adopted", got)
	}
}

func TestManifestRejectsMalformedLines(t *testing.T) {
	for name, content := range map[string]string{
		"unknown row kind":        "other:docs/ behavior\n",
		"missing value":           "install:docs/\n",
		"extra value":             "install:docs/ behavior extra\n",
		"duplicate install key":   "install:docs/ behavior\ninstall:docs/ record\n",
		"duplicate repo key":      "repo:docs/ behavior\nrepo:docs/ record\n",
		"duplicate ownership key": "own:plans/x.md x\nown:plans/x.md y\n",
		"unknown class":           "install:docs/ evidence\n",
		"absolute path":           "install:/docs/ behavior\n",
		"parent segment":          "install:docs/../plans/ behavior\n",
		"glob":                    "install:docs/*.md behavior\n",
		"ownership directory":     "own:plans/x/ x\n",
		"ownership outside plans": "own:records/x.md x\n",
		"invalid goal id":         "own:plans/x.md Bad\n",
		"unknown floor rule":      "floor:internal/ unsafe\n",
		"duplicate floor":         "floor:internal/ tier-1-refused\nfloor:internal/ tier-1-refused\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(content)); err == nil {
				t.Fatalf("Parse accepted malformed manifest %q", content)
			}
		})
	}
	if _, err := Parse([]byte("install:docs/ behavior\nrepo:docs/ record\n")); err != nil {
		t.Fatalf("same key in distinct namespaces was rejected: %v", err)
	}
}

func TestAbsentNamedPathsClassify(t *testing.T) {
	manifest := loadRepositoryManifest(t)
	for path, want := range map[string]Class{
		"go.work":                   Behavior,
		"go.work.sum":               Behavior,
		"plans/goals.md":            Ledger,
		"plans/goals-accepted.json": Ledger,
	} {
		if got := manifest.Class(path); got != want {
			t.Errorf("Class(%q) = %s; want %s", path, got, want)
		}
	}
}

func TestCompatibilityRows(t *testing.T) {
	manifest := loadRepositoryManifest(t)
	for path, want := range map[string]Class{
		"docs/journey.md":                 Behavior,
		"docs/reviews/old.md":             Behavior,
		"memory/README.md":                Behavior,
		"memory/rulings.md":               Record,
		"plans/README.md":                 Behavior,
		"plans/handoff-fixture-1.md":      Record,
		"plans/goals/x.md":                Ledger,
		"records/README.md":               Behavior,
		"records/goals/x.md":              Ledger,
		"records/narrator-digest.log":     Record,
		"scripts/agents/path-classes.txt": Behavior,
	} {
		if got := manifest.Class(path); got != want {
			t.Errorf("Class(%q) = %s; want %s", path, got, want)
		}
	}
}

func TestTemplateVersusAdoptedResolution(t *testing.T) {
	manifest := loadRepositoryManifest(t)
	repository := t.TempDir()
	installation := filepath.Join(repository, "metasystem")
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatal(err)
	}

	reportPath := filepath.Join(repository, "development", "report.md")
	templateAnswer, err := resolveAt(manifest, installation, repository, Template, stateroot.OwnerApp, reportPath)
	if err != nil || templateAnswer.Class != Record || templateAnswer.Namespace != Repo {
		t.Fatalf("template development/report.md = %+v, %v; want repo record", templateAnswer, err)
	}
	adoptedAnswer, err := resolveAt(manifest, installation, repository, Adopted, stateroot.OwnerApp, reportPath)
	if err != nil || adoptedAnswer.Class != Outside {
		t.Fatalf("adopted development/report.md = %+v, %v; want outside", adoptedAnswer, err)
	}
}

func TestRepositoryPathResolutionUsesModeAndLocation(t *testing.T) {
	manifest := loadRepositoryManifest(t)
	for _, test := range []struct {
		name      string
		mode      Mode
		ownership stateroot.Ownership
		prefix    string
		path      string
		wantClass Class
		wantSpace Namespace
	}{
		{name: "installation", mode: Template, ownership: stateroot.OwnerMetasystem, prefix: "metasystem", path: "metasystem/internal/goal/txn.go", wantClass: Behavior, wantSpace: Install},
		{name: "template repository record", mode: Template, ownership: stateroot.OwnerApp, prefix: "metasystem", path: "development/evidence-index.md", wantClass: Record, wantSpace: Repo},
		{name: "adopted application", mode: Adopted, ownership: stateroot.OwnerApp, prefix: "metasystem", path: "docs/application.md", wantClass: Outside},
		{name: "adopted root application", mode: Adopted, ownership: stateroot.OwnerApp, path: "docs/application.md", wantClass: Outside},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := manifest.ResolveRepositoryPath(test.mode, test.ownership, test.prefix, test.path)
			if got.Class != test.wantClass || got.Namespace != test.wantSpace || got.Mode != test.mode {
				t.Fatalf("ResolveRepositoryPath(%s, %q) = %+v; want class %s namespace %s", test.mode, test.path, got, test.wantClass, test.wantSpace)
			}
		})
	}
}

func TestResolveSameFileThreeInputForms(t *testing.T) {
	manifest := loadRepositoryManifest(t)
	repository, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	installation := filepath.Join(repository, "metasystem")

	paths := []struct {
		key       string
		wantClass Class
	}{
		{key: "internal/goal/txn.go", wantClass: Behavior},
		{key: "plans/path-class-fixture.md", wantClass: Record},
		{key: "artifacts/path-class-fixture.json", wantClass: Runtime},
	}
	for _, test := range paths {
		absolute := filepath.Join(installation, filepath.FromSlash(test.key))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte("fixture\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	originalDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDirectory); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	for _, test := range paths {
		t.Run(test.key, func(t *testing.T) {
			forms := []struct {
				name      string
				directory string
				input     string
			}{
				{name: "installation relative", directory: installation, input: filepath.FromSlash(test.key)},
				{name: "repository relative", directory: repository, input: filepath.Join("metasystem", filepath.FromSlash(test.key))},
				{name: "absolute", directory: repository, input: filepath.Join(installation, filepath.FromSlash(test.key))},
			}
			for _, form := range forms {
				t.Run(form.name, func(t *testing.T) {
					if err := os.Chdir(form.directory); err != nil {
						t.Fatal(err)
					}
					got, err := resolveAt(manifest, installation, repository, Template, stateroot.OwnerMetasystem, form.input)
					if err != nil {
						t.Fatal(err)
					}
					if got.Class != test.wantClass || got.Namespace != Install || got.Key != test.key {
						t.Fatalf("resolveAt(%q) from %q = %+v; want %s at install:%s", form.input, form.directory, got, test.wantClass, test.key)
					}
				})
			}
		})
	}
}

func TestRepositoryManifestClassifiesEveryTrackedPath(t *testing.T) {
	installation, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	repository := filepath.Dir(installation)
	manifest := loadRepositoryManifest(t)
	probe := exec.Command("git", "-C", repository, "rev-parse", "--is-inside-work-tree")
	probeOutput, probeErr := probe.Output()
	if probeErr != nil || strings.TrimSpace(string(probeOutput)) != "true" {
		t.Skipf("tracked-path manifest check requires the template parent to be a Git work tree: %v", probeErr)
	}
	command := exec.Command("git", "-C", repository, "ls-files", "-z")
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	var unclassified []string
	for _, tracked := range strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00") {
		if tracked == "" {
			continue
		}
		if strings.HasPrefix(tracked, "metasystem/") {
			key := strings.TrimPrefix(tracked, "metasystem/")
			if manifest.Resolve(Install, key).Class == Unclassified {
				unclassified = append(unclassified, "install:"+key)
			}
			continue
		}
		if manifest.Resolve(Repo, tracked).Class == Unclassified {
			unclassified = append(unclassified, "repo:"+tracked)
		}
	}
	if len(unclassified) > 0 {
		t.Fatalf("tracked paths missing from path class manifest: %s", strings.Join(unclassified, ", "))
	}
}

func TestResolveAtAnswersOutsideAfterSuccessfulDiscovery(t *testing.T) {
	repository := t.TempDir()
	installation := filepath.Join(repository, "metasystem")
	manifest := parseTestManifest(t, "install:docs/ behavior\n")
	answer, err := resolveAt(manifest, installation, repository, Template, stateroot.OwnerOutside, filepath.Join(filepath.Dir(repository), "outside.md"))
	if err != nil || answer.Class != Outside || answer.Mode != Template {
		t.Fatalf("successful outside ownership resolved as %+v, %v; want outside template", answer, err)
	}
}

func TestResolvePathReportsMisinstalledEngine(t *testing.T) {
	answer, err := ResolvePath("outside.md")
	if err == nil || !strings.Contains(err.Error(), "not installed at <installation>/bin/metasystem") {
		t.Fatalf("misinstalled test engine resolved as %+v, %v; want the installation diagnostic", answer, err)
	}
}

func TestRefusalTextUsesUnclassifiedSentinel(t *testing.T) {
	const want = "path product.txt has no class in scripts/agents/path-classes.txt; no classified ancestor; add a row for product.txt or its directory to scripts/agents/path-classes.txt"
	if got := RefusalText("product.txt"); got != want {
		t.Fatalf("RefusalText(product.txt) = %q; want %q", got, want)
	}
}

func parseTestManifest(t *testing.T, content string) *Manifest {
	t.Helper()
	manifest, err := Parse([]byte(content))
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func loadRepositoryManifest(t *testing.T) *Manifest {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}
