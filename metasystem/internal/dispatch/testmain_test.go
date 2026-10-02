package dispatch

import (
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

// inlineInputLimitEnv is the operator's packet-cap override. The
// composition tests compose against the real checkout, so an exported cap
// would decide their results; TestMain clears it once, before any test
// runs, and a test that needs a cap sets it sequentially (t.Setenv).
const inlineInputLimitEnv = "METASYSTEM_DISPATCH_MAX_INLINE_INPUT_KB"

func TestMain(m *testing.M) {
	if err := os.Unsetenv(inlineInputLimitEnv); err != nil {
		panic(err)
	}
	os.Exit(testenv.Main(m))
}
