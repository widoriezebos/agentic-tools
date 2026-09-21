package parallelratchet

import (
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// A dependency tree carries test files of its own; counting them would move
// the ratchet on a tree nobody in this repository wrote (g1-s8 revision 5,
// the exclusion slice). Without the node_modules skip the planted serial test
// is discovered and its package joins the inventory.
func TestScanParallelTestsSkipsDependencyTrees(t *testing.T) {
	t.Parallel()
	root := parallelFixtureModule(t)
	writeParallelFixture(t, filepath.Join(root, "kept_test.go"), `package fixture
import "testing"
func TestKept(t *testing.T) { t.Parallel() }
`)
	serial := `package poison
import "testing"
func TestPoison(t *testing.T) {}
`
	writeParallelFixture(t, filepath.Join(root, "internal/ui/web/_app/node_modules/poison/serial_test.go"), serial)
	writeParallelFixture(t, filepath.Join(root, "internal/ui/web/_app/node_modules/poison/node_modules/deeper/serial_test.go"), serial)

	inventory, err := ScanParallelTests(root)
	testutil.Require(t, "scan error", err, nil)
	testutil.Expect(t, "packages", inventory.Packages, []string{"example.test/fixture"})
	names := []string{}
	for _, test := range inventory.Tests {
		names = append(names, test.Test)
	}
	testutil.Expect(t, "tests", names, []string{"TestKept"})
}
