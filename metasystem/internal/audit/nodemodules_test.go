package audit

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The stop-surface scan lists the repository's files, so an installed
// dependency tree would contribute _test.go files it never wrote unless the
// scan itself leaves node_modules out (g1-s8 revision 5, the exclusion slice).
// The fixture has no ignore file: without the skip the two planted files are
// discovered.
func TestDiscoverStopSurfaceFilesSkipsDependencyTrees(t *testing.T) {
	t.Parallel()
	fixture := newStopSurfaceFixture(t, nil, map[string]string{
		"internal/ui/web/_app/src/kept_test.go":                                       "package poison\n",
		"internal/ui/web/_app/node_modules/poison/serial_test.go":                     "package poison\n",
		"internal/ui/web/_app/node_modules/poison/node_modules/deeper/serial_test.go": "package poison\n",
	}, false)

	files, err := discoverStopSurfaceFiles(fixture.root)
	testutil.Require(t, "discover error", err, nil)
	testutil.Expect(t, "discovered", files, []stopSurfaceFile{
		{Kind: "go", Path: "internal/ui/web/_app/src/kept_test.go"},
	})
}
