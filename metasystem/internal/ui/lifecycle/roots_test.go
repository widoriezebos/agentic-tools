package lifecycle

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// O3: the state root follows the installation's layout, so the lifecycle state
// sits beside the engine's own process control and never under the Git checkout
// of an adopted installation.
func TestO3RootsAcrossLayouts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// installation and state root, relative to the application repository.
		installation string
		selfHosted   bool
		stateRoot    string
	}{
		{name: "self-hosted", installation: "metasystem", selfHosted: true, stateRoot: "metasystem"},
		{name: "adopted vendored beneath the application", installation: filepath.Join("vendor", "metasystem"), stateRoot: "."},
		{name: "adopted at the application root", installation: ".", stateRoot: "."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			repository := rootsTestRepository(t, "application")
			installation := filepath.Join(repository, test.installation)
			testutil.Require(t, "create installation", os.MkdirAll(installation, 0o755), nil)
			if test.selfHosted {
				// The self-hosted layout is the nested metasystem/ directory
				// beside development/metasystem-design.md.
				design := filepath.Join(repository, "development", "metasystem-design.md")
				testutil.Require(t, "create development directory", os.MkdirAll(filepath.Dir(design), 0o755), nil)
				testutil.Require(t, "create design document", os.WriteFile(design, []byte("# design\n"), 0o644), nil)
			}
			wantStateRoot := filepath.Join(repository, test.stateRoot)

			// The checkout defaults to the Git top that contains the
			// installation, and naming it explicitly resolves the same.
			for _, repo := range []struct{ name, path string }{
				{"the default checkout", ""},
				{"the named checkout", repository},
				{"a directory inside the checkout", filepath.Join(repository, test.installation)},
			} {
				roots, err := ResolveRoots(repo.path, installation)

				testutil.Require(t, "resolve roots error from "+repo.name, err, nil)
				testutil.Expect(t, "checkout from "+repo.name, roots.Checkout, repository)
				testutil.Expect(t, "installation from "+repo.name, roots.Installation, filepath.Clean(installation))
				testutil.Expect(t, "state root from "+repo.name, roots.StateRoot, filepath.Clean(wantStateRoot))
				testutil.Expect(t, "state directory from "+repo.name, Dir(roots.StateRoot),
					filepath.Join(filepath.Clean(wantStateRoot), "artifacts", "agents", "ui"))
			}
		})
	}
}

// O4: a checkout the installation does not live in is refused before anything
// is created, and a path outside every checkout is refused with the same line.
func TestO4ResolveRootsRefusesForeignCheckout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		repo func(t *testing.T) string
	}{
		{
			name: "another checkout",
			repo: func(t *testing.T) string { return rootsTestRepository(t, "foreign") },
		},
		{
			name: "no checkout at all",
			repo: func(t *testing.T) string {
				outside := filepath.Join(t.TempDir(), "outside")
				testutil.Require(t, "create directory outside every checkout", os.MkdirAll(outside, 0o755), nil)
				canonical, err := filepath.EvalSymlinks(outside)
				testutil.Require(t, "canonical directory", err, nil)
				return canonical
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			repository := rootsTestRepository(t, "application")
			installation := filepath.Join(repository, "metasystem")
			testutil.Require(t, "create installation", os.MkdirAll(installation, 0o755), nil)
			foreign := test.repo(t)
			before := map[string][]string{
				repository: treeSnapshot(t, "application before", repository),
				foreign:    treeSnapshot(t, "foreign before", foreign),
			}

			roots, err := ResolveRoots(foreign, installation)

			var mismatch *RootsMismatchError
			testutil.Require(t, "mismatch error", errors.As(err, &mismatch), true)
			testutil.Expect(t, "refused roots", roots, Roots{})
			testutil.Expect(t, "mismatch checkout", mismatch.Checkout, foreign)
			testutil.Expect(t, "mismatch installation", mismatch.Installation, installation)
			testutil.Expect(t, "mismatch line", err.Error(),
				"the installation at "+installation+" does not serve the checkout at "+foreign)
			for _, walk := range []struct{ label, root string }{
				{"application after", repository},
				{"foreign after", foreign},
			} {
				testutil.Expect(t, "unchanged tree at "+walk.root,
					treeSnapshot(t, walk.label, walk.root), before[walk.root])
			}
		})
	}
}

// rootsTestRepository is a real Git repository, because only a Git top can be a
// checkout, and its path is canonical, because a temporary directory reaches it
// through a symbolic link.
func rootsTestRepository(t *testing.T, label string) string {
	t.Helper()

	repository := filepath.Join(t.TempDir(), label)
	output, err := exec.Command("git", "init", "-q", repository).CombinedOutput()
	testutil.Require(t, label+" git init: "+string(output), err, nil)
	canonical, err := filepath.EvalSymlinks(repository)
	testutil.Require(t, label+" canonical repository", err, nil)
	return canonical
}

// treeSnapshot lists every path beneath root, so a test can prove that a call
// created nothing anywhere rather than that one expected path is absent.
func treeSnapshot(t *testing.T, label, root string) []string {
	t.Helper()

	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		suffix := ""
		if entry.IsDir() {
			suffix = "/"
		}
		paths = append(paths, relative+suffix)
		return nil
	})
	testutil.Require(t, "walk "+label, err, nil)
	sort.Strings(paths)
	return paths
}
