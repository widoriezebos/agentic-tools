package stateroot

import (
	"os"
	"path/filepath"
	"testing"
)

// vendoredShape builds <repo>/vendor-app with a vendored metasystem
// installation and points the executable seam at its engine path.
func vendoredShape(t *testing.T, template bool) (repo, install string) {
	t.Helper()
	repo = t.TempDir()
	install = filepath.Join(repo, "metasystem")
	for _, d := range []string{"bin", "scripts/agents", "memory", "artifacts/agents"} {
		if err := os.MkdirAll(filepath.Join(install, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("metasystem.runtimes=\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if template {
		dev := filepath.Join(repo, "development")
		if err := os.MkdirAll(dev, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dev, "metasystem-design.md"), []byte("design\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	previousExec := executablePath
	executablePath = func() (string, error) {
		return filepath.Join(install, "bin", "metasystem"), nil
	}
	previousTop := repositoryTop
	repositoryTop = func(string) (string, error) { return repo, nil }
	t.Cleanup(func() { executablePath = previousExec; repositoryTop = previousTop })
	return repo, install
}

func TestOwnerFourAnswersInVendoredAdoptedShape(t *testing.T) {
	repo, _ := vendoredShape(t, false)
	cases := []struct {
		path string
		want Ownership
	}{
		{"metasystem/scripts/agents/dispatch.sh", OwnerMetasystem},
		{"metasystem", OwnerMetasystem},
		{"memory/rulings.md", OwnerApp},
		{"plans/goals/x.md", OwnerApp},
		{"artifacts/agents/steward/highwater.json", OwnerRuntime},
		{"metasystem/artifacts/agents/job.json", OwnerRuntime},
		{"bin/tool", OwnerRuntime},
		{"metasystem/bin/metasystem", OwnerRuntime},
	}
	for _, c := range cases {
		got, mode, err := Owner(c.path)
		if err != nil || got != c.want || mode != "adopted" {
			t.Fatalf("Owner(%q) = %v %q %v; want %v adopted", c.path, got, mode, err, c.want)
		}
	}
	if got, mode, err := Owner(filepath.Join(repo, "docs", "guide.md")); err != nil || got != OwnerApp || mode != "adopted" {
		t.Fatalf("absolute app path: %v %q %v", got, mode, err)
	}
	if got, _, err := Owner("/no/such/outside"); err == nil || got != OwnerOutside {
		t.Fatalf("outside path accepted: %v %v", got, err)
	}
	if got, _, err := Owner("../escape.txt"); err == nil || got != OwnerOutside {
		t.Fatalf("relative escape accepted: %v %v", got, err)
	}
}

func TestOwnerTemplateModeRidesAlong(t *testing.T) {
	_, _ = vendoredShape(t, true)
	got, mode, err := Owner("metasystem/memory/rulings.md")
	if err != nil || got != OwnerMetasystem || mode != "template" {
		t.Fatalf("template register: %v %q %v", got, mode, err)
	}
}

func TestOwnerSymlinkJudgedByEntryPath(t *testing.T) {
	repo, _ := vendoredShape(t, false)
	outside := t.TempDir()
	link := filepath.Join(repo, "docs-link")
	if err := os.MkdirAll(filepath.Join(repo, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	got, _, err := Owner("docs-link")
	if err != nil || got != OwnerApp {
		t.Fatalf("symlink entry ownership: %v %v", got, err)
	}
}
