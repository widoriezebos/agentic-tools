package pathpattern

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// Expand is the declaration walker the proof contract reaches through its
// metasystem/internal/** inputs. It is covered by this fixture alone: a
// trailing /** parses into the subtree flag, so a live component-wildcard
// declaration is what would call Expand, and the contract declares none
// (Sol's round 5, obligation 2). Without the node_modules skip the second
// and third files below join the expansion.
func TestExpandSkipsDependencyTrees(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{
		"internal/ui/web/_app/src/main_test.go",
		"internal/ui/web/_app/node_modules/poison/x_test.go",
		"internal/ui/web/_app/node_modules/poison/node_modules/deeper/x_test.go",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		testutil.Require(t, "make "+name, os.MkdirAll(filepath.Dir(path), 0700), nil)
		testutil.Require(t, "write "+name, os.WriteFile(path, []byte(name), 0600), nil)
	}

	pattern, err := Parse("internal/*/web/_app/**")
	testutil.Require(t, "parse", err, nil)
	testutil.Require(t, "component wildcard", pattern.HasComponentWildcard(), true)

	matches, err := pattern.Expand(root)
	testutil.Expect(t, "expand error", err, nil)
	testutil.Expect(t, "expanded", matches, []string{"internal/ui/web/_app/src/main_test.go"})
}
