package hostturn

import (
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
	// The external host adapter fixture (external_host_test.go): this
	// binary run as an adapter executable answers one operation and exits.
	if os.Getenv("EXTERNAL_HOST_FIXTURE") == "1" {
		os.Exit(runExternalHostFixture())
	}
	os.Exit(testenv.Main(m))
}
