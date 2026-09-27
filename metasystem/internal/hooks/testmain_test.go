package hooks

import (
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
	if os.Getenv(hookWorkerModeEnv) == "1" {
		os.Exit(runHookTestWorker())
	}
	os.Exit(testenv.Main(m))
}
