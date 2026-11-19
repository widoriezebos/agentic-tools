package atomicfile

import (
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
	if earlySyncBeforeMain() != nil {
		os.Exit(2)
	}
	os.Exit(testenv.Main(m))
}
