package proofrun

import (
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The frozen export is a copy of the live root, so an installed dependency
// tree would be exported into every frozen candidate. The guard cannot
// observe this walker — Freeze compares its three reads only with each other
// and nothing the guard runs reads the export's contents — so this fixture is
// the whole of its coverage (Sol's round 5, obligation 3). Without the
// node_modules arm of hardExcluded the two planted files are manifested.
func TestManifestExcludesDependencyTrees(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "internal", "ui", "web", "_app", "src", "main.tsx"), []byte("kept"), 0644)
	writeTestFile(t, filepath.Join(root, "internal", "ui", "web", "_app", "node_modules", "poison", "seam.go"), []byte("poison"), 0644)
	writeTestFile(t, filepath.Join(root, "internal", "ui", "web", "_app", "node_modules", "poison", "node_modules", "deeper", "seam.go"), []byte("poison"), 0644)

	m, err := readManifest(root)
	testutil.Require(t, "read manifest", err, nil)
	paths := make([]string, len(m.entries))
	for i, item := range m.entries {
		paths[i] = item.path
	}
	testutil.Expect(t, "manifested paths", paths, []string{"internal/ui/web/_app/src/main.tsx"})
}
