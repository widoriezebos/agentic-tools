package testenv

import (
	"fmt"
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/durable"
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
	if os.Getenv(durabilityOptedFixtureEnv) != "" {
		// A process that already opted in: the harness's own opt-in refuses.
		if err := durable.UseKernelInThisTestProcess(); err != nil {
			os.Exit(3)
		}
		code := MainWithSetup(m, durabilityOptedFixtureSetup)
		// The namespace's cleanup restores the TMPDIR it replaced.
		fmt.Printf("tmpdir-after=%s\n", os.Getenv("TMPDIR"))
		os.Exit(code)
	}
	if os.Getenv(mainWithSetupFixtureEnv) != "" {
		os.Exit(MainWithSetup(m, mainWithSetupFixture))
	}
	os.Exit(Main(m))
}
