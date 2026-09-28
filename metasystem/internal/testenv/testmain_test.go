package testenv

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv(mainWithSetupFixtureEnv) != "" {
		os.Exit(MainWithSetup(m, mainWithSetupFixture))
	}
	os.Exit(Main(m))
}
