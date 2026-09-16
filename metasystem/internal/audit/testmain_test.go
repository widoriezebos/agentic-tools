package audit

import (
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
	armHookSignalGuard()
	os.Exit(testenv.Main(m))
}
