package batchowner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

// boardLaunchStore is the launch store the package's host observation
// counts: TestMain points the board's observation at it, under the owned
// TMPDIR, and a test writes the records it observes there.
var boardLaunchStore launch.Store

func TestMain(m *testing.M) {
	os.Exit(testenv.MainWithSetup(m, func() error {
		dir, err := os.MkdirTemp("", "batchowner-launch-")
		boardLaunchStore = launch.Store{Root: filepath.Join(dir, "launch")}
		LandingLaneHostWork = (&launch.Manager{Store: boardLaunchStore}).HostWork
		return err
	}))
}
