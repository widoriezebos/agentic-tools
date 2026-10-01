package kernel

import (
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv(helperMode); mode != "" {
		os.Exit(runDetachedProveHelper(mode, os.Args[1:]))
	}
	os.Exit(testenv.Main(m))
}
