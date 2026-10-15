package testenv

import (
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv/testdata/buildlock"
)

func TestMain(m *testing.M) {
	buildlock.Command()
	if os.Getenv(setupCustodySleeperEnv) != "" {
		setupCustodySleeper()
		os.Exit(0)
	}
	if os.Getenv(setupCustodyFixtureEnv) != "" {
		os.Exit(MainWithSetup(m, setupCustodyFixture))
	}
	if os.Getenv(mainWithSetupFixtureEnv) != "" {
		os.Exit(MainWithSetup(m, mainWithSetupFixture))
	}
	os.Exit(Main(m))
}
