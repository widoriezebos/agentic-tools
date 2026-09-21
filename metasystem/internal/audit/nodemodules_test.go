package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The stop-surface scan walks the live tree by name, so an installed
// dependency tree would contribute _test.go files and scripts/agents
// fixture beds it never wrote (g1-s8 revision 5, the exclusion slice).
// Without the node_modules skip the two planted files are discovered.
func TestDiscoverStopSurfaceFilesSkipsDependencyTrees(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{
		"internal/ui/web/_app/src/kept_test.go",
		"internal/ui/web/_app/node_modules/poison/serial_test.go",
		"internal/ui/web/_app/node_modules/poison/node_modules/deeper/serial_test.go",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		testutil.Require(t, "make "+name, os.MkdirAll(filepath.Dir(path), 0o755), nil)
		testutil.Require(t, "write "+name, os.WriteFile(path, []byte("package poison\n"), 0o644), nil)
	}

	files, err := discoverStopSurfaceFiles(root)
	testutil.Require(t, "discover error", err, nil)
	testutil.Expect(t, "discovered", files, []stopSurfaceFile{
		{Kind: "go", Path: "internal/ui/web/_app/src/kept_test.go"},
	})
}
