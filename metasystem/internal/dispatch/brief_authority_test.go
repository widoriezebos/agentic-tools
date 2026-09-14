package dispatch

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBriefAuthorityRefusesMissingBasePathThenAdmitsCommittedPath(t *testing.T) {
	repo := newBriefAuthorityRepo(t)
	brief := writeBriefAuthorityFile(t, repo, "brief.md", "Working Mode: implement\nAuthority: records/two-bars/missing.md\n")

	err := ValidateBriefAuthority(brief, repo, repo)
	var refusal *BriefAuthorityRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("missing record error = %v, want typed BriefAuthorityRefusal", err)
	}
	if !reflect.DeepEqual(refusal.MissingPaths, []string{"records/two-bars/missing.md"}) ||
		!strings.Contains(err.Error(), "records/two-bars/missing.md") {
		t.Fatalf("missing record refusal = %+v, %q", refusal, err)
	}

	path := filepath.Join(repo, "records", "two-bars", "missing.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("landed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateBriefAuthority(brief, repo, repo); err == nil {
		t.Fatal("an uncommitted working-tree file was accepted as delegate base content")
	}
	briefAuthorityGit(t, repo, "add", "records/two-bars/missing.md")
	briefAuthorityGit(t, repo, "commit", "-q", "-m", "land authority")
	if err := ValidateBriefAuthority(brief, repo, repo); err != nil {
		t.Fatalf("committed authority path refused: %v", err)
	}
}

func TestBriefAuthoritySkipsTemplatesAndChecksArtifactsOnDisk(t *testing.T) {
	repo := newBriefAuthorityRepo(t)
	artifact := filepath.Join(repo, "artifacts", "agents", "jobs", "live.json")
	if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact, []byte("runtime state\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	brief := writeBriefAuthorityFile(t, repo, "brief.md", strings.Join([]string{
		"Working Mode: implement",
		"Skip records/**/*.md records/<name>.md records/${name}.md.",
		"Read `artifacts/agents/jobs/live.json` and `artifacts/agents/jobs/absent.json`.",
	}, "\n"))

	err := ValidateBriefAuthority(brief, repo, repo)
	var refusal *BriefAuthorityRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("missing artifact error = %v, want typed BriefAuthorityRefusal", err)
	}
	if !reflect.DeepEqual(refusal.MissingPaths, []string{"artifacts/agents/jobs/absent.json"}) {
		t.Fatalf("artifact and template extraction = %v", refusal.MissingPaths)
	}

	missing := filepath.Join(repo, "artifacts", "agents", "jobs", "absent.json")
	if err := os.WriteFile(missing, []byte("runtime state\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateBriefAuthority(brief, repo, repo); err != nil {
		t.Fatalf("on-disk artifact paths refused: %v", err)
	}
}

func TestBriefAuthorityAdmitsBriefWithNoCandidatePaths(t *testing.T) {
	repo := newBriefAuthorityRepo(t)
	brief := writeBriefAuthorityFile(t, repo, "brief.md", "Working Mode: implement\nExplain the change without citing repository inputs.\n")
	if err := ValidateBriefAuthority(brief, repo, repo); err != nil {
		t.Fatalf("zero-candidate brief refused: %v", err)
	}
}

func TestBriefAuthorityTreatsMetasystemDiffBoundaryExampleAsPrefixOnly(t *testing.T) {
	repo := newBriefAuthorityRepo(t)
	metasystemKeep := filepath.Join(repo, "metasystem", "internal", ".keep")
	if err := os.MkdirAll(filepath.Dir(metasystemKeep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metasystemKeep, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	briefAuthorityGit(t, repo, "add", "metasystem/internal/.keep")
	briefAuthorityGit(t, repo, "commit", "-q", "-m", "add nested installation")
	brief := writeBriefAuthorityFile(t, repo, "brief.md",
		"Working Mode: implement\ndiffBoundary example: [\"metasystem/internal/not-an-authority.go\"]\n")
	if err := ValidateBriefAuthority(brief, repo, repo); err != nil {
		t.Fatalf("repository-relative diffBoundary example was treated as cited authority: %v", err)
	}
}

func TestBriefAuthorityExemptsDeclaredOutputsUnlessTheyAreAlsoInputs(t *testing.T) {
	repo := newBriefAuthorityRepo(t)
	brief := writeBriefAuthorityFile(t, repo, "brief.md", strings.Join([]string{
		"Working Mode: implement",
		"# Workspace",
		"May-write: records/identity/epoch-drift-design.md",
		"May touch: records/identity/second-output.md",
		"Create `records/identity/third-output.md`.",
		"# Goal",
		"Produce the listed deliverables.",
	}, "\n"))
	if err := ValidateBriefAuthority(brief, repo, repo); err != nil {
		t.Fatalf("absent paths declared only as outputs were refused: %v", err)
	}

	brief = writeBriefAuthorityFile(t, repo, "brief.md", strings.Join([]string{
		"Working Mode: implement",
		"# Workspace",
		"May-write: records/identity/epoch-drift-design.md",
		"# Inputs",
		"Follow the authority in records/identity/epoch-drift-design.md.",
	}, "\n"))
	err := ValidateBriefAuthority(brief, repo, repo)
	var refusal *BriefAuthorityRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("path cited as both output and input error = %v, want typed BriefAuthorityRefusal", err)
	}
	if !reflect.DeepEqual(refusal.MissingPaths, []string{"records/identity/epoch-drift-design.md"}) {
		t.Fatalf("input citation did not win over output declaration: %v", refusal.MissingPaths)
	}
}

func TestBriefAuthorityBoundaryIsOutput(t *testing.T) {
	for _, tc := range []struct{ name, member string }{
		{"file", "docs/new.md"}, {"directory", "docs/new/"}, {"glob", "docs/*.md"}, {"special-name", "docs/a b.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireAuthorityInRepo(t, newBriefAuthorityRepo(t), boundedAuthority([]string{tc.member}), nil)
		})
	}
}

func TestBriefAuthorityIndentedBoundsExampleIsInert(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		missing    bool
	}{
		{"spaces", boundedAuthority([]string{}, ` Boundary: ["records/missing.md"]`), false},
		{"tabs", boundedAuthority([]string{}, "\tCeiling: records/missing.md"), false},
		{"headerless", "Working Mode: implement\n Boundary: records/missing.md", false},
		{"separate-input", boundedAuthority([]string{}, " Boundary: records/example.md", "Read records/missing.md"), true},
		{"quoted", boundedAuthority([]string{}, `> Boundary: ["records/missing.md"]`), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var want []string
			if tc.missing {
				want = []string{"records/missing.md"}
			}
			requireAuthorityInRepo(t, newBriefAuthorityRepo(t), tc.text, want)
		})
	}
}

func TestBriefAuthorityBoundedUsesTrunkTokenScanner(t *testing.T) {
	repo := newBriefAuthorityRepo(t)
	for _, tc := range []struct {
		name, line string
		want       []string
	}{
		{"backticked-path-line", "Read `docs/missing.md:426`.", []string{"docs/missing.md"}},
		{"command-with-arguments", "Run `scripts/agents/go-gate.sh --fast`.", []string{"scripts/agents/go-gate.sh"}},
		{"placeholder", "Read `records/<placeholder>.md`.", nil},
		{"glob", "Read `records/**/*.md`.", nil},
		{"quoted-sentence", `The note says "read docs/quoted-missing.md before continuing".`, []string{"docs/quoted-missing.md"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireAuthorityInRepo(t, repo, boundedAuthority([]string{}, tc.line), tc.want)
		})
	}
}

func boundedAuthority(boundary []string, lines ...string) string {
	encoded, _ := json.Marshal(boundary)
	return strings.Join(append([]string{"Working Mode: implement", "Boundary: " + string(encoded), "Ceiling: 1"}, lines...), "\n")
}

func requireAuthorityInRepo(t *testing.T, repo, content string, want []string) {
	t.Helper()
	err := ValidateBriefAuthority(writeBriefAuthorityFile(t, repo, "brief.md", content), repo, repo)
	var refusal *BriefAuthorityRefusal
	if want == nil && err != nil || want != nil && (!errors.As(err, &refusal) || !reflect.DeepEqual(refusal.MissingPaths, want)) {
		t.Fatalf("authority error = %#v, want missing paths %q", err, want)
	}
}

func newBriefAuthorityRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	briefAuthorityGit(t, repo, "init", "-q")
	briefAuthorityGit(t, repo, "config", "user.email", "test@example.invalid")
	briefAuthorityGit(t, repo, "config", "user.name", "Test")
	for _, directory := range []string{"records", "docs", "scripts"} {
		path := filepath.Join(repo, directory, ".keep")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	briefAuthorityGit(t, repo, "add", "records/.keep", "docs/.keep", "scripts/.keep")
	briefAuthorityGit(t, repo, "commit", "-q", "-m", "base")
	return repo
}

func writeBriefAuthorityFile(t *testing.T, directory, name, content string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func briefAuthorityGit(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}
