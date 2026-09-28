package testutil

import (
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
	if os.Getenv(fixtureBedEngineStubEnv) == "1" {
		os.Exit(runFixtureBedEngineStub(os.Args[1:], os.Stdout, os.Stderr))
	}
	os.Exit(testenv.Main(m))
}
