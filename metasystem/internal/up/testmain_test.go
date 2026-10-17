package up

import (
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
	if dispatchUpHelper() {
		os.Exit(0)
	}
	os.Exit(testenv.Main(m))
}
