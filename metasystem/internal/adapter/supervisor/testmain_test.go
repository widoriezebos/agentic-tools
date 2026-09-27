package supervisor

import (
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
	// Git is stubbed for the whole package, before any test runs and never
	// changed after: every fixture root is a temporary directory outside
	// any repository, and no test reads a real work tree. The stub is
	// constant, so parallel tests share no mutable fake.
	gitOutput = func(string, ...string) (string, bool) { return "", false }
	os.Exit(testenv.Main(m))
}
