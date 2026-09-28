package stateroot

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOwnerRefusesWhenTheRepositoryBoundaryIsUnknown(t *testing.T) {
	t.Parallel()
	for _, template := range []bool{false, true} {
		mode := map[bool]string{false: "adopted", true: "template"}[template]
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			repo, install, resolver, top := vendoredShape(t, template)
			top.expectCanonical(install, "", errors.New("not in a repository"))
			owner, gotMode, err := resolver.Owner(filepath.Join(repo, "README.md"))
			if err == nil || owner != OwnerOutside || gotMode != mode {
				t.Fatalf("Owner() = %v, %q, %v; want refusal in %s mode", owner, gotMode, err, mode)
			}
			if template && !strings.Contains(err.Error(), "repository top unreadable") {
				t.Fatalf("template refusal does not name the unreadable repository top: %v", err)
			}
		})
	}
}

func TestOwnerRefusesAnExecutableOutsideAnInstallation(t *testing.T) {
	t.Parallel()
	executable := filepath.Join(t.TempDir(), "bin", "metasystem")
	resolver := NewResolver(func(string) (string, error) { t.Fatal("unexpected Git lookup"); return "", nil }, func() (string, error) { return executable, nil })
	owner, mode, err := resolver.Owner("README.md")
	if err == nil || owner != OwnerOutside || mode != "" || !strings.Contains(err.Error(), "is not installed at <installation>/bin/metasystem") {
		t.Fatalf("Owner() = %v, %q, %v; want uninstalled-executable refusal", owner, mode, err)
	}
}

func TestResolveLayoutRefusesPathsThatSelectNoInstallation(t *testing.T) {
	t.Parallel()
	resolver, top := resolverFixture(t, t.TempDir())
	if _, err := resolver.ResolveLayout("  "); err == nil || !strings.Contains(err.Error(), "repository path is required") {
		t.Fatalf("blank repository path = %v", err)
	}
	if _, err := resolver.ResolveLayout(filepath.Join(t.TempDir(), "absent")); err == nil || !strings.Contains(err.Error(), "inspect repository path") {
		t.Fatalf("absent repository path = %v", err)
	}
	unrepositoried := t.TempDir()
	unavailable := errors.New("not in a repository")
	top.expectCanonical(unrepositoried, "", unavailable)
	if _, err := resolver.ResolveLayout(unrepositoried); !errors.Is(err, unavailable) {
		t.Fatalf("path outside Git and any installation = %v; want repository failure", err)
	}
	bare := t.TempDir()
	top.expectCanonical(bare, bare, nil)
	if _, err := resolver.ResolveLayout(bare); err == nil || !strings.Contains(err.Error(), "selects neither a nested template nor an adopted installation") {
		t.Fatalf("repository without an installation = %v", err)
	}
}

func TestResolveLayoutFromAFileUsesItsDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeLayoutFile(t, filepath.Join(root, "metasystem.conf"), "metasystem.runtimes=claude\n")
	if err := os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "skills", "demo", "SKILL.md")
	writeLayoutFile(t, file, "demo\n")
	resolver, top := resolverFixture(t, root)
	top.expectCanonical(filepath.Dir(file), root, nil)
	layout, err := resolver.ResolveLayout(file)
	want, _ := filepath.EvalSymlinks(root)
	if err != nil || layout.InstallationRoot != want || layout.InstallationRel != "." || layout.Template {
		t.Fatalf("layout from file = %+v, %v; want adopted root %s", layout, err, want)
	}
}
