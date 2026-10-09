package delegation

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
	os.Exit(testenv.MainWithSetup(m, func() error {
		// Admission reads the process's provider home. The test namespace
		// owns an empty provider history for all of its isolated checkouts.
		home, err := board.Home()
		if err != nil {
			return err
		}
		root := filepath.Dir(home)
		_, _, err = lane.Register(home, lane.Layout{Checkout: lane.CheckoutRoot(root), Install: lane.InstallRoot(root)}, "fixture", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		return err
	}))
}
