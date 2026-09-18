package identity_test

import (
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
	var declarations []testenv.Declaration
	if os.Getenv("FIXTURE_CUSTODIAN_WITNESS") != "" ||
		os.Getenv("FIXTURE_QUIET_CUSTODIAN_WITNESS") != "" ||
		os.Getenv("FIXTURE_PLATFORM_BINARY_WITNESS_MODE") != "" ||
		os.Getenv("FIXTURE_LAUNCHER_WITNESS_MODE") != "" {
		declarations = append(declarations,
			testenv.Declare(identity.FixtureCustodianPollEnv),
			testenv.Declare(identity.FixtureCustodianBoundEnv),
		)
	}
	os.Exit(testenv.Main(m, declarations...))
}
