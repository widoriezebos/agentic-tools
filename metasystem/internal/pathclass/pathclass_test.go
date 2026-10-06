package pathclass

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestLongestPrefixWins(t *testing.T) {
	t.Parallel()
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

	got := manifest.ResolveRepositoryPath(Template, stateroot.OwnerMetasystem, "metasystem", "metasystem/cmd/x.go")
	if got.Class != Behavior || got.Namespace != Install || got.Key != "cmd/x.go" {
		t.Fatalf("template installation path resolved as %+v; want install:cmd/x.go behavior", got)
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
		"docs/journey.md":                     Behavior,
		"docs/reviews/old.md":                 Behavior,
		"memory/README.md":                    Behavior,
		"memory/rulings.md":                   Record,
		"plans/README.md":                     Behavior,
		"plans/handoff-fixture-1.md":          Record,
		"plans/goals/x.md":                    Ledger,
		"records/README.md":                   Behavior,
		"records/goals/x.md":                  Ledger,
		"records/narrator-digest.log":         Record,
		"internal/pathclass/path-classes.txt": Behavior,
	} {
		if got := manifest.Class(path); got != want {
			t.Errorf("Class(%q) = %s; want %s", path, got, want)
		}
	}
}

func TestTemplateVersusAdoptedResolution(t *testing.T) {
	manifest := loadRepositoryManifest(t)
	templateAnswer := manifest.ResolveRepositoryPath(Template, stateroot.OwnerApp, "metasystem", "development/report.md")
	if templateAnswer.Class != Record || templateAnswer.Namespace != Repo {
		t.Fatalf("template development/report.md = %+v; want repo record", templateAnswer)
	}
	adoptedAnswer := manifest.ResolveRepositoryPath(Adopted, stateroot.OwnerApp, "metasystem", "development/report.md")
	if adoptedAnswer.Class != Outside {
		t.Fatalf("adopted development/report.md = %+v; want outside", adoptedAnswer)
	}
}

// The compiled-in manifest answers the template layout's classes: runtime,
// ledger, record and behavior rows under the installation, an unclassified
// application path at the repository top, and a behavior directory row
// explaining a file beneath it.
func TestCompiledManifestClassifiesTheTemplateLayout(t *testing.T) {
	t.Parallel()
	manifest, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path      string
		ownership stateroot.Ownership
		want      Class
	}{
		{"metasystem/internal/x.go", stateroot.OwnerMetasystem, Behavior},
		{"metasystem/internal/goal/txn.go", stateroot.OwnerMetasystem, Behavior},
		{"metasystem/records/misc/x.md", stateroot.OwnerMetasystem, Record},
		{"metasystem/plans/path-class-fixture.md", stateroot.OwnerMetasystem, Record},
		{"metasystem/plans/goals/x.md", stateroot.OwnerMetasystem, Ledger},
		{"metasystem/bin/metasystem", stateroot.OwnerMetasystem, Runtime},
		{"metasystem/artifacts/path-class-fixture.json", stateroot.OwnerMetasystem, Runtime},
		{"product.txt", stateroot.OwnerApp, Unclassified},
	} {
		if got := manifest.ResolveRepositoryPath(Template, test.ownership, "metasystem", test.path); got.Class != test.want || got.Mode != Template {
			t.Errorf("%s resolved as %+v; want %s in template mode", test.path, got, test.want)
		}
	}
	explained := manifest.ResolveRepositoryPath(Template, stateroot.OwnerMetasystem, "metasystem", "metasystem/docs/guide.md")
	if explained.Class != Behavior || explained.Row != "install:docs/" || explained.Namespace != Install || explained.Key != "docs/guide.md" {
		t.Fatalf("docs/guide.md explained as %+v; want behavior row=install:docs/ key=install:docs/guide.md", explained)
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

func TestRepositoryManifestClassifiesProofSourcesAndGeneratedReporter(t *testing.T) {
	t.Parallel()
	manifest := loadRepositoryManifest(t)
	for _, test := range []struct {
		path string
		want Class
	}{
		{"proof/full.sh", Behavior},
		{"proof/main.go", Behavior},
		{"proof/.full-reporter", Runtime},
	} {
		if got := manifest.Resolve(Install, test.path).Class; got != test.want {
			t.Errorf("Resolve(install, %q) = %s; want %s", test.path, got, test.want)
		}
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

func TestRefusalTextUsesUnclassifiedSentinel(t *testing.T) {
	const want = "path product.txt has no class in the engine's path-class policy (internal/pathclass/path-classes.txt); no classified ancestor"
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
	manifest, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

// The manifest is engine source compiled into the engine: Load returns the
// embedded bytes, which are the file at SourcePath, and no installation file
// is read.
func TestLoadIsTheCompiledInManifest(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(SourcePath)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(source, Source()) {
		t.Fatalf("the embedded manifest differs from %s", SourcePath)
	}
	manifest, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]Class{
		SourcePath:                           Behavior,
		"testing-coverage-floors.json":       Behavior,
		"testing-coverage-floors-linux.json": Behavior,
		"scripts/anything.sh":                Unclassified,
	} {
		if got := manifest.Class(path); got != want {
			t.Errorf("Class(%q) = %s; want %s", path, got, want)
		}
	}
	for _, floor := range []string{"internal/protocol/schemas/", SourcePath, "internal/landing/landing-classes.json"} {
		if !manifest.Floors[floor] {
			t.Errorf("%s is not a tier-1 floor", floor)
		}
	}
	for floor := range manifest.Floors {
		if strings.HasPrefix(floor, "scripts/") {
			t.Errorf("floor %s names the retired scripts tree", floor)
		}
	}
}
