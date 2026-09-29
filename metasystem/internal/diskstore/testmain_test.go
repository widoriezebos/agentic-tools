package diskstore

import (
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
	if code, handled := processScratchHelper(); handled {
		os.Exit(code)
	}
	os.Exit(testenv.Main(m))
}
