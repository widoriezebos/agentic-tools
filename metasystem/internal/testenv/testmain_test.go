package testenv

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
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
